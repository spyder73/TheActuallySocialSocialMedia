// Package auth implements invitation-only account creation and opaque database sessions.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/argon2"
)

const (
	cookieName = "tassm_session"
	sessionTTL = 30 * 24 * time.Hour
	maxBody    = 16 << 10
)

var (
	usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9_]{3,30}$`)
	passwordSlots   = make(chan struct{}, 2)
)

const dummyPasswordHash = "$argon2id$v=19$m=65536,t=3,p=2$AAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"

// User is the small public identity attached to authenticated requests.
type User struct {
	ID          string  `json:"id"`
	Email       string  `json:"email"`
	Username    string  `json:"username"`
	DisplayName *string `json:"displayName"`
	Bio         *string `json:"bio"`
	Role        string  `json:"role"`
}

type Options struct {
	// AllowedOrigins must contain the exact browser origin, including scheme and port.
	AllowedOrigins []string
	// LocalHTTP explicitly enables a non-Secure cookie for local HTTP development.
	LocalHTTP       bool
	CookieName      string
	SessionLifetime time.Duration
}

type Service struct {
	pool    *pgxpool.Pool
	opts    Options
	origins map[string]struct{}
	limits  *limiter
}

// New constructs the auth service. Cookies are Secure unless LocalHTTP is explicitly true.
func New(pool *pgxpool.Pool, opts Options) *Service {
	if opts.CookieName == "" {
		opts.CookieName = cookieName
	}
	if opts.SessionLifetime <= 0 {
		opts.SessionLifetime = sessionTTL
	}
	origins := make(map[string]struct{}, len(opts.AllowedOrigins))
	for _, origin := range opts.AllowedOrigins {
		if normalized, err := normalizeOrigin(origin); err == nil {
			origins[normalized] = struct{}{}
		}
	}
	return &Service{pool: pool, opts: opts, origins: origins, limits: newLimiter()}
}

// Handler serves routes relative to /api, e.g. mount it at /api to expose /api/auth/login.
func (s *Service) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /auth/me", s.me)
	mux.HandleFunc("POST /auth/login", s.login)
	mux.HandleFunc("POST /auth/logout", s.logout)
	mux.HandleFunc("POST /auth/register", s.register)
	mux.HandleFunc("POST /auth/reset", s.reset)
	return mux
}

type contextKey struct{}

// UserFromContext returns the authenticated user populated by Middleware.
func UserFromContext(ctx context.Context) (User, bool) {
	u, ok := ctx.Value(contextKey{}).(User)
	return u, ok
}

// UserID returns the authenticated user's ID, or an empty string when absent.
func UserID(r *http.Request) string {
	u, _ := UserFromContext(r.Context())
	return u.ID
}

// Middleware resolves a database session and adds the user to request context.
func (s *Service) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, err := r.Cookie(s.opts.CookieName)
		if err == nil && len(token.Value) <= 128 {
			hash := tokenDigest(token.Value)
			var u User
			err = s.pool.QueryRow(r.Context(), `SELECT u."id", u."email", u."username", u."displayName", u."bio", u."role"
				FROM "AuthSession" s JOIN "User" u ON u."id"=s."userId"
				WHERE s."tokenHash"=$1 AND s."revokedAt" IS NULL AND s."expiresAt">now() AND u."disabledAt" IS NULL`, hash).
				Scan(&u.ID, &u.Email, &u.Username, &u.DisplayName, &u.Bio, &u.Role)
			if err == nil {
				r = r.WithContext(context.WithValue(r.Context(), contextKey{}, u))
			}
		}
		next.ServeHTTP(w, r)
	})
}

// OriginMiddleware strictly validates Origin on every state-changing API request.
func OriginMiddleware(allowedOrigins ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, raw := range allowedOrigins {
		if origin, err := normalizeOrigin(raw); err == nil {
			allowed[origin] = struct{}{}
		}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions {
				origin, err := normalizeOrigin(r.Header.Get("Origin"))
				if err != nil {
					writeError(w, http.StatusForbidden, "invalid origin")
					return
				}
				if _, ok := allowed[origin]; !ok {
					writeError(w, http.StatusForbidden, "origin not allowed")
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (s *Service) me(w http.ResponseWriter, r *http.Request) {
	token, err := r.Cookie(s.opts.CookieName)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	hash := tokenDigest(token.Value)
	var u User
	err = s.pool.QueryRow(r.Context(), `SELECT u."id",u."email",u."username",u."displayName",u."bio",u."role"
		FROM "AuthSession" s JOIN "User" u ON u."id"=s."userId"
		WHERE s."tokenHash"=$1 AND s."revokedAt" IS NULL AND s."expiresAt">now() AND u."disabledAt" IS NULL`, hash).
		Scan(&u.ID, &u.Email, &u.Username, &u.DisplayName, &u.Bio, &u.Role)
	if err != nil {
		clearCookie(w, s)
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"user": u})
}

func (s *Service) login(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decode(w, r, &in) {
		return
	}
	email := normalizeEmail(in.Email)
	ip := clientIP(r)
	if len(in.Password) > 1024 || !s.limits.allow("ip|"+ip, 60, time.Minute) || !s.limits.allow(ip+"|"+email, 8, time.Minute) {
		writeError(w, http.StatusTooManyRequests, "try again later")
		return
	}
	var id, encoded string
	err := s.pool.QueryRow(r.Context(), `SELECT "id","passwordHash" FROM "User" WHERE lower("email")=$1 AND "disabledAt" IS NULL`, email).Scan(&id, &encoded)
	valid := err == nil && verifyPassword(encoded, in.Password)
	if errors.Is(err, pgx.ErrNoRows) {
		_ = verifyPassword(dummyPasswordHash, in.Password)
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	if !valid {
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}
	s.createSession(w, r, id, encoded)
}

func (s *Service) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(s.opts.CookieName); err == nil && len(c.Value) <= 128 {
		if _, err := s.pool.Exec(r.Context(), `UPDATE "AuthSession" SET "revokedAt"=now() WHERE "tokenHash"=$1 AND "revokedAt" IS NULL`, tokenDigest(c.Value)); err != nil {
			writeError(w, http.StatusInternalServerError, "server error")
			return
		}
	}
	clearCookie(w, s)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Service) register(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Token       string `json:"token"`
		Email       string `json:"email"`
		Username    string `json:"username"`
		Password    string `json:"password"`
		DisplayName string `json:"displayName"`
	}
	if !decode(w, r, &in) {
		return
	}
	email, username := normalizeEmail(in.Email), strings.TrimSpace(in.Username)
	if len(in.Token) > 128 || !validEmail(email) || !usernamePattern.MatchString(username) || !validPassword(in.Password) || len(in.DisplayName) > 100 {
		writeError(w, http.StatusBadRequest, "invalid registration details")
		return
	}
	ip := clientIP(r)
	if !s.limits.allow("register-ip|"+ip, 10, time.Hour) || !s.limits.allow("register|"+ip+"|"+email, 5, time.Hour) {
		writeError(w, http.StatusTooManyRequests, "try again later")
		return
	}
	hash, err := hashPassword(in.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	defer tx.Rollback(r.Context())
	var tokenID, tokenEmail, inviteRole string
	err = tx.QueryRow(r.Context(), `SELECT "id","email","inviteRole" FROM "AccountToken" WHERE "tokenHash"=$1 AND "purpose"='invite' AND "usedAt" IS NULL AND "expiresAt">now() FOR UPDATE`, tokenDigest(in.Token)).Scan(&tokenID, &tokenEmail, &inviteRole)
	if err != nil || normalizeEmail(tokenEmail) != email {
		writeError(w, http.StatusBadRequest, "invalid or expired invitation")
		return
	}
	id, err := randomToken(18)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	var display any
	if strings.TrimSpace(in.DisplayName) != "" {
		display = strings.TrimSpace(in.DisplayName)
	}
	_, err = tx.Exec(r.Context(), `INSERT INTO "User" ("id","email","username","passwordHash","displayName","role") VALUES ($1,$2,$3,$4,$5,$6)`, id, email, username, hash, display, inviteRole)
	if err != nil {
		writeError(w, http.StatusConflict, "email or username is already in use")
		return
	}
	if _, err = tx.Exec(r.Context(), `UPDATE "AccountToken" SET "usedAt"=now(),"userId"=$2 WHERE "id"=$1`, tokenID, id); err != nil {
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	s.createSession(w, r, id, hash)
}

func (s *Service) reset(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Token    string `json:"token"`
		Password string `json:"password"`
	}
	if !decode(w, r, &in) {
		return
	}
	if len(in.Token) > 128 || !validPassword(in.Password) {
		writeError(w, http.StatusBadRequest, "invalid reset details")
		return
	}
	if !s.limits.allow(clientIP(r)+"|reset", 10, time.Hour) {
		writeError(w, http.StatusTooManyRequests, "try again later")
		return
	}
	hash, err := hashPassword(in.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	defer tx.Rollback(r.Context())
	var tokenID, userID string
	err = tx.QueryRow(r.Context(), `SELECT "id","userId" FROM "AccountToken" WHERE "tokenHash"=$1 AND "purpose"='reset' AND "usedAt" IS NULL AND "expiresAt">now() AND "userId" IS NOT NULL FOR UPDATE`, tokenDigest(in.Token)).Scan(&tokenID, &userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid or expired reset token")
		return
	}
	if _, err = tx.Exec(r.Context(), `UPDATE "AccountToken" SET "usedAt"=now() WHERE "id"=$1`, tokenID); err != nil {
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	updated, updateErr := tx.Exec(r.Context(), `UPDATE "User" SET "passwordHash"=$2 WHERE "id"=$1 AND "disabledAt" IS NULL`, userID, hash)
	if updateErr != nil {
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	if updated.RowsAffected() != 1 {
		writeError(w, http.StatusBadRequest, "invalid or expired reset token")
		return
	}
	if _, err = tx.Exec(r.Context(), `UPDATE "AuthSession" SET "revokedAt"=now() WHERE "userId"=$1 AND "revokedAt" IS NULL`, userID); err != nil {
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	clearCookie(w, s)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Service) createSession(w http.ResponseWriter, r *http.Request, userID, expectedHash string) {
	token, err := randomToken(32)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	id, err := randomToken(18)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	expires := time.Now().Add(s.opts.SessionLifetime)
	tx, err := s.pool.Begin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	defer tx.Rollback(r.Context())
	var u User
	var currentHash string
	err = tx.QueryRow(r.Context(), `SELECT "id","email","username","displayName","bio","role","passwordHash" FROM "User" WHERE "id"=$1 AND "disabledAt" IS NULL FOR UPDATE`, userID).Scan(&u.ID, &u.Email, &u.Username, &u.DisplayName, &u.Bio, &u.Role, &currentHash)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}
	if currentHash != expectedHash {
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}
	if _, err = tx.Exec(r.Context(), `INSERT INTO "AuthSession" ("id","tokenHash","userId","expiresAt") VALUES ($1,$2,$3,$4)`, id, tokenDigest(token), userID, expires); err != nil {
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "server error")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: s.opts.CookieName, Value: token, Path: "/", HttpOnly: true, Secure: !s.opts.LocalHTTP, SameSite: http.SameSiteLaxMode, Expires: expires, MaxAge: int(time.Until(expires).Seconds())})
	writeJSON(w, http.StatusOK, map[string]any{"user": u})
}

func (s *Service) SetCookieForToken(w http.ResponseWriter, token string, expires time.Time) {
	http.SetCookie(w, &http.Cookie{Name: s.opts.CookieName, Value: token, Path: "/", HttpOnly: true, Secure: !s.opts.LocalHTTP, SameSite: http.SameSiteLaxMode, Expires: expires})
}
func clearCookie(w http.ResponseWriter, s *Service) {
	http.SetCookie(w, &http.Cookie{Name: s.opts.CookieName, Value: "", Path: "/", HttpOnly: true, Secure: !s.opts.LocalHTTP, SameSite: http.SameSiteLaxMode, MaxAge: -1, Expires: time.Unix(1, 0)})
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBody)
	defer r.Body.Close()
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return false
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return false
	}
	return true
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
func tokenDigest(token string) []byte { sum := sha256.Sum256([]byte(token)); return sum[:] }
func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
func validPassword(s string) bool { return len(s) >= 12 && len(s) <= 1024 }
func validEmail(s string) bool {
	return len(s) >= 3 && len(s) <= 254 && strings.Count(s, "@") == 1 && !strings.ContainsAny(s, " \t\r\n")
}
func normalizeEmail(s string) string { return strings.ToLower(strings.TrimSpace(s)) }
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}
func normalizeOrigin(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" || u.User != nil || u.Path != "" && u.Path != "/" || u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("invalid origin")
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return "", fmt.Errorf("invalid origin scheme")
	}
	return strings.ToLower(u.Scheme + "://" + u.Host), nil
}

func hashPassword(password string) (string, error) {
	select {
	case passwordSlots <- struct{}{}:
	case <-time.After(2 * time.Second):
		return "", errors.New("password hashing busy")
	}
	defer func() { <-passwordSlots }()
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	derived := argon2.IDKey([]byte(password), salt, 3, 64*1024, 2, 32)
	return fmt.Sprintf("$argon2id$v=19$m=65536,t=3,p=2$%s$%s", base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(derived)), nil
}

// verifyPassword accepts Node argon2's PHC argon2id encoding while rejecting
// unbounded parameters before allocating memory.
func verifyPassword(encoded, password string) bool {
	if len(password) > 1024 || len(encoded) > 256 {
		return false
	}
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return false
	}
	var memory uint32
	var iterations uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &threads); err != nil || memory < 8*1024 || memory > 256*1024 || iterations < 1 || iterations > 6 || threads < 1 || threads > 4 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) < 8 || len(salt) > 64 {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(want) < 16 || len(want) > 64 {
		return false
	}
	select {
	case passwordSlots <- struct{}{}:
	case <-time.After(2 * time.Second):
		return false
	}
	defer func() { <-passwordSlots }()
	got := argon2.IDKey([]byte(password), salt, iterations, memory, threads, uint32(len(want)))
	return subtleCompare(got, want)
}
func subtleCompare(a, b []byte) bool {
	return subtle.ConstantTimeCompare(a, b) == 1
}

type limiter struct {
	mu      sync.Mutex
	entries map[string]window
}
type window struct {
	count int
	until time.Time
}

func newLimiter() *limiter { return &limiter{entries: make(map[string]window)} }
func (l *limiter) allow(key string, max int, period time.Duration) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.entries) > 10000 {
		for k, v := range l.entries {
			if now.After(v.until) {
				delete(l.entries, k)
			}
		}
		if len(l.entries) > 10000 {
			return false
		}
	}
	v := l.entries[key]
	if now.After(v.until) {
		v = window{until: now.Add(period)}
	}
	v.count++
	l.entries[key] = v
	return v.count <= max
}
