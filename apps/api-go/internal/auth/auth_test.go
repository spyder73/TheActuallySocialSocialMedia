package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spyder73/TheActuallySocialSocialMedia/apps/api-go/internal/migrate"
	"golang.org/x/crypto/argon2"
)

func TestPasswordHashBoundsAndNodeCompatibility(t *testing.T) {
	encoded, err := hashPassword("a sufficiently long pilot password")
	if err != nil {
		t.Fatal(err)
	}
	if !verifyPassword(encoded, "a sufficiently long pilot password") {
		t.Fatal("new hash did not verify")
	}
	if verifyPassword(encoded, "another password") {
		t.Fatal("wrong password verified")
	}
	// node-argon2 defaults to m=65536,t=3,p=4; those existing hashes remain usable.
	salt := []byte("existing-node-salt")
	legacy := argon2.IDKey([]byte("legacy account password"), salt, 3, 64*1024, 4, 32)
	phc := fmt.Sprintf("$argon2id$v=19$m=65536,t=3,p=4$%s$%s", base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(legacy))
	if !verifyPassword(phc, "legacy account password") {
		t.Fatal("existing Node argon2id hash did not verify")
	}
	if verifyPassword(strings.Replace(phc, "m=65536", "m=999999999", 1), "legacy account password") {
		t.Fatal("unbounded memory parameter was accepted")
	}
	if verifyPassword(phc, strings.Repeat("x", 1025)) {
		t.Fatal("oversized password was accepted")
	}
}

func TestOriginMiddlewareRequiresExactOrigin(t *testing.T) {
	h := OriginMiddleware("https://app.example.test")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	for _, origin := range []string{"", "null", "https://evil.example.test", "https://app.example.test.evil"} {
		r := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != http.StatusForbidden {
			t.Errorf("origin %q got %d, want 403", origin, w.Code)
		}
	}
	r := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	r.Header.Set("Origin", "https://app.example.test")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusNoContent {
		t.Fatalf("allowed origin got %d", w.Code)
	}
}

// This integration test requires TEST_DATABASE_URL credentials allowed to create/drop databases.
func TestInvitationSessionResetAndReplayIntegration(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	admin, err := pgx.ConnectConfig(ctx, cfg.ConnConfig.Copy())
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(ctx)
	var suffix [8]byte
	if _, err = rand.Read(suffix[:]); err != nil {
		t.Fatal(err)
	}
	db := fmt.Sprintf("tassm_auth_test_%x", suffix[:])
	quoted := (pgx.Identifier{db}).Sanitize()
	if _, err = admin.Exec(ctx, "CREATE DATABASE "+quoted); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupConn, e := pgx.Connect(context.Background(), dsn)
		if e != nil {
			t.Errorf("connect to clean test database: %v", e)
			return
		}
		defer cleanupConn.Close(context.Background())
		if _, e := cleanupConn.Exec(context.Background(), "DROP DATABASE "+quoted+" WITH (FORCE)"); e != nil {
			t.Errorf("drop test database: %v", e)
		}
	})
	target := cfg.ConnConfig.Copy()
	target.Database = db
	poolCfg := cfg.Copy()
	poolCfg.ConnConfig = target
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err = migrate.Run(ctx, pool, "", nil); err != nil {
		t.Fatal(err)
	}
	svc := New(pool, Options{AllowedOrigins: []string{"http://localhost:18080"}, LocalHTTP: true})
	handler := OriginMiddleware("http://localhost:18080")(svc.Handler())
	inviteToken := "invite-token-for-test"
	inviteHash := sha256.Sum256([]byte(inviteToken))
	if _, err = pool.Exec(ctx, `INSERT INTO "AccountToken"("id","tokenHash","purpose","email","inviteRole","expiresAt") VALUES ('invite-1',$1,'invite','pilot@example.test','admin',now()+interval '1 hour')`, inviteHash[:]); err != nil {
		t.Fatal(err)
	}
	register := request(handler, http.MethodPost, "/auth/register", `{"token":"invite-token-for-test","email":"PILOT@example.test","username":"pilot_user","password":"a strong pilot password","displayName":"Pilot"}`, "")
	if register.Code != http.StatusOK {
		t.Fatalf("registration: %d %s", register.Code, register.Body.String())
	}
	cookie := register.Result().Cookies()[0]
	if cookie.Name != cookieName || !cookie.HttpOnly || cookie.Secure || cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("cookie flags are wrong: %#v", cookie)
	}
	var role string
	if err = pool.QueryRow(ctx, `SELECT "role" FROM "User" WHERE "email"='pilot@example.test'`).Scan(&role); err != nil || role != "admin" {
		t.Fatalf("CLI invite role not preserved: %q %v", role, err)
	}
	var stored []byte
	if err = pool.QueryRow(ctx, `SELECT "tokenHash" FROM "AccountToken" WHERE "id"='invite-1'`).Scan(&stored); err != nil || string(stored) != string(inviteHash[:]) {
		t.Fatalf("token hash mismatch: %v", err)
	}
	if replay := request(handler, http.MethodPost, "/auth/register", `{"token":"invite-token-for-test","email":"pilot@example.test","username":"other_user","password":"a strong pilot password"}`, ""); replay.Code != http.StatusBadRequest {
		t.Fatalf("replayed invite got %d", replay.Code)
	}
	me := requestWithCookie(handler, http.MethodGet, "/auth/me", "", cookie)
	if me.Code != http.StatusOK || !strings.Contains(me.Body.String(), `"role":"admin"`) {
		t.Fatalf("authenticated /me response %d %s", me.Code, me.Body.String())
	}
	logout := requestWithCookie(handler, http.MethodPost, "/auth/logout", "{}", cookie)
	if logout.Code != http.StatusOK {
		t.Fatalf("logout status %d", logout.Code)
	}
	me = requestWithCookie(handler, http.MethodGet, "/auth/me", "", cookie)
	if me.Code != http.StatusUnauthorized {
		t.Fatalf("revoked session /me got %d", me.Code)
	}
	login := request(handler, http.MethodPost, "/auth/login", `{"email":"pilot@example.test","password":"a strong pilot password"}`, "")
	if login.Code != http.StatusOK {
		t.Fatalf("login got %d %s", login.Code, login.Body.String())
	}
	oldCookie := login.Result().Cookies()[0]
	resetToken := "reset-token-for-test"
	resetHash := sha256.Sum256([]byte(resetToken))
	if _, err = pool.Exec(ctx, `INSERT INTO "AccountToken"("id","tokenHash","purpose","email","userId","expiresAt") SELECT 'reset-1',$1,'reset',"email","id",now()+interval '1 hour' FROM "User" WHERE "email"='pilot@example.test'`, resetHash[:]); err != nil {
		t.Fatal(err)
	}
	reset := request(handler, http.MethodPost, "/auth/reset", `{"token":"reset-token-for-test","password":"a newer pilot password"}`, "")
	if reset.Code != http.StatusOK {
		t.Fatalf("reset got %d %s", reset.Code, reset.Body.String())
	}
	me = requestWithCookie(handler, http.MethodGet, "/auth/me", "", oldCookie)
	if me.Code != http.StatusUnauthorized {
		t.Fatalf("reset left session valid, /me got %d", me.Code)
	}
	if replay := request(handler, http.MethodPost, "/auth/reset", `{"token":"reset-token-for-test","password":"a different pilot password"}`, ""); replay.Code != http.StatusBadRequest {
		t.Fatalf("replayed reset got %d", replay.Code)
	}
	login = request(handler, http.MethodPost, "/auth/login", `{"email":"pilot@example.test","password":"a newer pilot password"}`, "")
	if login.Code != http.StatusOK {
		t.Fatalf("new password login got %d", login.Code)
	}
	if bad := request(handler, http.MethodPost, "/auth/login", `{"email":"nobody@example.test","password":"a strong pilot password"}`, ""); bad.Code != http.StatusUnauthorized {
		t.Fatalf("unknown email login got %d", bad.Code)
	}
}
func request(h http.Handler, method, path, body, origin string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if origin == "" {
		origin = "http://localhost:18080"
	}
	r.Header.Set("Origin", origin)
	r.RemoteAddr = "192.0.2.1:4312"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func requestWithCookie(h http.Handler, method, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.AddCookie(cookie)
	if method != http.MethodGet {
		r.Header.Set("Origin", "http://localhost:18080")
	}
	r.RemoteAddr = "192.0.2.1:4312"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
