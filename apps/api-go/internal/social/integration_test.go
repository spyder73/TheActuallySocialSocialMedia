package social

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spyder73/TheActuallySocialSocialMedia/apps/api-go/internal/migrate"
)

// This test follows the repository's TEST_DATABASE_URL convention: the URL
// must identify a PostgreSQL database whose credentials may create databases.
func TestSocialIntegration(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	base, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	admin, err := pgx.ConnectConfig(ctx, base.ConnConfig.Copy())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close(context.Background()) })
	var suffix [8]byte
	if _, err = rand.Read(suffix[:]); err != nil {
		t.Fatal(err)
	}
	db := fmt.Sprintf("tassm_social_test_%s", hex.EncodeToString(suffix[:]))
	quoted := (pgx.Identifier{db}).Sanitize()
	if _, err = admin.Exec(ctx, "CREATE DATABASE "+quoted); err != nil {
		t.Fatalf("create isolated test database: %v", err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(context.Background(), "DROP DATABASE "+quoted+" WITH (FORCE)"); err != nil {
			t.Errorf("drop isolated test database: %v", err)
		}
	})
	cfg := base.Copy()
	target := cfg.ConnConfig.Copy()
	target.Database = db
	cfg.ConnConfig = target
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err = migrate.Run(ctx, pool, "", nil); err != nil {
		t.Fatalf("migrate isolated database: %v", err)
	}
	for _, u := range []struct{ id, name string }{{"user-a", "alice"}, {"user-b", "bob"}, {"user-c", "carol"}} {
		if _, err = pool.Exec(ctx, `INSERT INTO "User"(id,email,username,"passwordHash") VALUES($1,$2,$3,'test')`, u.id, u.name+"@example.test", u.name); err != nil {
			t.Fatal(err)
		}
	}
	r := chi.NewRouter()
	New(pool).Register(r, func(req *http.Request) string { return req.Header.Get("X-Test-User") })
	request := func(user, method, path string, body any) *httptest.ResponseRecorder {
		t.Helper()
		var payload []byte
		if body != nil {
			payload, err = json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
		}
		req := httptest.NewRequest(method, path, bytesReader(payload))
		req.Header.Set("X-Test-User", user)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	post := func(user, visibility, text string) string {
		t.Helper()
		w := request(user, http.MethodPost, "/posts", map[string]any{"text": text, "visibility": visibility})
		if w.Code != 201 {
			t.Fatalf("create post: %d %s", w.Code, w.Body.String())
		}
		var res struct {
			Post Post `json:"post"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
			t.Fatal(err)
		}
		return res.Post.ID
	}
	closeID := post("user-a", "close_friends", "for close friends")
	if w := request("user-b", http.MethodGet, "/posts/"+closeID, nil); w.Code != 404 {
		t.Fatalf("close-friends post visible before adding friend: %d", w.Code)
	}
	if w := request("user-a", http.MethodPut, "/relationships/close_friend/user-b", nil); w.Code != 204 {
		t.Fatalf("add close friend: %d", w.Code)
	}
	if w := request("user-b", http.MethodGet, "/posts/"+closeID, nil); w.Code != 200 {
		t.Fatalf("close-friends post hidden from listed friend: %d %s", w.Code, w.Body.String())
	}
	if w := request("user-c", http.MethodGet, "/posts/"+closeID, nil); w.Code != 404 {
		t.Fatalf("close-friends post visible to other user: %d", w.Code)
	}
	// Older or imported data can contain a public reply chain below a private
	// ancestor. Access must remain restricted at every depth.
	if _, err = pool.Exec(ctx, `INSERT INTO "Post"(id,"authorId",text,visibility,"parentPostId") VALUES('middle-post','user-b','middle','public',$1),('deep-post','user-c','deep','public','middle-post')`, closeID); err != nil {
		t.Fatal(err)
	}
	for _, postID := range []string{"middle-post", "deep-post"} {
		if w := request("user-c", http.MethodGet, "/posts/"+postID, nil); w.Code != 404 {
			t.Fatalf("private ancestor leaked through %s: %d", postID, w.Code)
		}
		if w := request("user-c", http.MethodGet, "/posts/"+postID+"/comments", nil); w.Code != 404 {
			t.Fatalf("private ancestor comments leaked through %s: %d", postID, w.Code)
		}
	}
	feed := request("user-c", http.MethodGet, "/feed", nil)
	if feed.Code != 200 || strings.Contains(feed.Body.String(), "middle-post") || strings.Contains(feed.Body.String(), "deep-post") {
		t.Fatalf("feed exposed nested replies to a private ancestor: %d %s", feed.Code, feed.Body.String())
	}
	publicID := post("user-a", "public", "public")
	if w := request("user-b", http.MethodDelete, "/posts/"+publicID, nil); w.Code != 404 {
		t.Fatalf("non-owner delete returned %d", w.Code)
	}
	if w := request("user-a", http.MethodPut, "/relationships/block/user-c", nil); w.Code != 204 {
		t.Fatalf("block: %d", w.Code)
	}
	if w := request("user-c", http.MethodGet, "/posts/"+publicID, nil); w.Code != 404 {
		t.Fatalf("blocked user read post returned %d", w.Code)
	}
	// A block initiated in the opposite direction also hides both parties' posts.
	bobPost := post("user-b", "public", "bob public")
	if w := request("user-b", http.MethodPut, "/relationships/block/user-a", nil); w.Code != 204 {
		t.Fatalf("reverse block: %d", w.Code)
	}
	if w := request("user-a", http.MethodGet, "/posts/"+bobPost, nil); w.Code != 404 {
		t.Fatalf("reverse-blocked post returned %d", w.Code)
	}
	if w := request("user-b", http.MethodGet, "/posts/"+publicID, nil); w.Code != 404 {
		t.Fatalf("reverse-blocked post returned %d", w.Code)
	}
	cw := request("user-a", http.MethodPost, "/conversations", map[string]any{"memberIds": []string{"user-b"}})
	if cw.Code != 400 {
		t.Fatalf("conversation creation with blocked member returned %d; want 400", cw.Code)
	}
	if w := request("user-a", http.MethodDelete, "/posts/"+publicID, nil); w.Code != 204 {
		t.Fatalf("owner delete returned %d", w.Code)
	}
	if w := request("user-a", http.MethodGet, "/posts/"+publicID, nil); w.Code != 404 {
		t.Fatalf("deleted post still visible: %d", w.Code)
	}
	// A separate unblocked pair verifies conversation membership on read and write.
	cw = request("user-b", http.MethodPost, "/conversations", map[string]any{"memberIds": []string{"user-c"}, "name": "test"})
	if cw.Code != 201 {
		t.Fatalf("create conversation: %d %s", cw.Code, cw.Body.String())
	}
	var convRes struct {
		Conversation Conversation `json:"conversation"`
	}
	if err = json.Unmarshal(cw.Body.Bytes(), &convRes); err != nil {
		t.Fatal(err)
	}
	msg := request("user-b", http.MethodPost, "/conversations/"+convRes.Conversation.ID+"/messages", map[string]string{"text": "hello"})
	if msg.Code != 201 {
		t.Fatalf("member message create: %d %s", msg.Code, msg.Body.String())
	}
	if w := request("user-a", http.MethodGet, "/conversations/"+convRes.Conversation.ID+"/messages", nil); w.Code != 404 {
		t.Fatalf("non-member message read returned %d", w.Code)
	}
	if w := request("user-a", http.MethodPost, "/conversations/"+convRes.Conversation.ID+"/messages", map[string]string{"text": "intrusion"}); w.Code != 404 {
		t.Fatalf("non-member message write returned %d", w.Code)
	}
	if w := request("user-b", http.MethodPut, "/relationships/block/user-c", nil); w.Code != 204 {
		t.Fatalf("block conversation member: %d", w.Code)
	}
	if w := request("user-b", http.MethodPost, "/conversations/"+convRes.Conversation.ID+"/messages", map[string]string{"text": "blocked"}); w.Code != 403 {
		t.Fatalf("bilaterally blocked message write returned %d; want 403", w.Code)
	}
	if w := request("user-c", http.MethodGet, "/conversations/"+convRes.Conversation.ID+"/messages", nil); w.Code != 403 {
		t.Fatalf("bilaterally blocked message read returned %d; want 403", w.Code)
	}
}

type byteReader struct {
	b []byte
	i int
}

func bytesReader(b []byte) *byteReader { return &byteReader{b: b} }
func (r *byteReader) Read(p []byte) (int, error) {
	if r.i >= len(r.b) {
		return 0, io.EOF
	}
	n := copy(p, r.b[r.i:])
	r.i += n
	return n, nil
}
