// Package social implements the authenticated social API against the Prisma
// schema used by the existing application.
package social

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const pageSize = 25

var errBlockedConversation = errors.New("conversation blocked")

type Service struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

type User struct {
	ID          string  `json:"id"`
	Username    string  `json:"username"`
	DisplayName *string `json:"displayName"`
	Bio         *string `json:"bio"`
}
type Post struct {
	ID           string    `json:"id"`
	Text         *string   `json:"text"`
	Visibility   string    `json:"visibility"`
	CreatedAt    time.Time `json:"createdAt"`
	Author       User      `json:"author"`
	ParentPostID *string   `json:"parentPostId"`
	CommentCount int       `json:"commentCount"`
}
type Comment struct {
	ID        string    `json:"id"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"createdAt"`
	Author    User      `json:"author"`
}
type Conversation struct {
	ID      string  `json:"id"`
	Name    *string `json:"name"`
	Members []User  `json:"members"`
}
type Message struct {
	ID        string    `json:"id"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"createdAt"`
	Sender    User      `json:"sender"`
}

func (s *Service) Register(r chi.Router, userID func(*http.Request) string) {
	r.Get("/feed", s.auth(userID, s.feed))
	r.Get("/posts/{id}", s.auth(userID, s.getPost))
	r.Post("/posts", s.auth(userID, s.createPost))
	r.Delete("/posts/{id}", s.auth(userID, s.deletePost))
	r.Get("/posts/{id}/comments", s.auth(userID, s.getComments))
	r.Post("/posts/{id}/comments", s.auth(userID, s.createComment))
	r.Get("/members", s.auth(userID, s.members))
	r.Patch("/profile", s.auth(userID, s.patchProfile))
	r.Get("/relationships", s.auth(userID, s.relationships))
	r.Put("/relationships/{kind}/{id}", s.auth(userID, s.putRelationship))
	r.Delete("/relationships/{kind}/{id}", s.auth(userID, s.deleteRelationship))
	r.Post("/reports", s.auth(userID, s.report))
	r.Get("/conversations", s.auth(userID, s.conversations))
	r.Post("/conversations", s.auth(userID, s.createConversation))
	r.Get("/conversations/{id}/messages", s.auth(userID, s.getMessages))
	r.Post("/conversations/{id}/messages", s.auth(userID, s.createMessage))
}

type handler func(http.ResponseWriter, *http.Request, string)

func (s *Service) auth(getID func(*http.Request) string, h handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := ""
		if getID != nil {
			id = getID(r)
		}
		if id == "" {
			writeError(w, 401, "unauthorized")
			return
		}
		h(w, r, id)
	}
}
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		writeError(w, 400, "invalid JSON body")
		return false
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		writeError(w, 400, "invalid JSON body")
		return false
	}
	return true
}
func newID() (string, error) {
	var b [16]byte
	if _, e := rand.Read(b[:]); e != nil {
		return "", e
	}
	return hex.EncodeToString(b[:]), nil
}
func idOrError(w http.ResponseWriter) (string, bool) {
	id, e := newID()
	if e != nil {
		writeError(w, 500, "internal server error")
		return "", false
	}
	return id, true
}
func (s *Service) user(ctx context.Context, id string) (User, error) {
	var u User
	e := s.pool.QueryRow(ctx, `SELECT id,username,"displayName",bio FROM "User" WHERE id=$1 AND "disabledAt" IS NULL`, id).Scan(&u.ID, &u.Username, &u.DisplayName, &u.Bio)
	return u, e
}
func (s *Service) blocked(ctx context.Context, a, b string) (bool, error) {
	var yes bool
	e := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM "BlockedUser" WHERE ("blockerId"=$1 AND "blockedId"=$2) OR ("blockerId"=$2 AND "blockedId"=$1))`, a, b).Scan(&yes)
	return yes, e
}
func (s *Service) canSeePost(ctx context.Context, viewer, post string) (bool, error) {
	var yes bool
	e := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM "Post" target WHERE target.id=$2) AND NOT EXISTS (
		WITH RECURSIVE ancestors AS (
			SELECT p.id,p."authorId",p.visibility,p."parentPostId",0 AS depth FROM "Post" p WHERE p.id=$2
			UNION ALL
			SELECT parent.id,parent."authorId",parent.visibility,parent."parentPostId",a.depth+1 FROM ancestors a JOIN "Post" parent ON parent.id=a."parentPostId" WHERE a.depth<100
		)
		SELECT 1 FROM ancestors a JOIN "User" au ON au.id=a."authorId" WHERE au."disabledAt" IS NOT NULL OR a.depth=100 AND a."parentPostId" IS NOT NULL OR
			NOT (a.visibility='public' OR a."authorId"=$1 OR (a.visibility='close_friends' AND EXISTS(SELECT 1 FROM "CloseFriend" cf WHERE cf."ownerId"=a."authorId" AND cf."friendId"=$1))) OR
			EXISTS(SELECT 1 FROM "BlockedUser" b WHERE (b."blockerId"=$1 AND b."blockedId"=a."authorId") OR (b."blockerId"=a."authorId" AND b."blockedId"=$1))
	)`, viewer, post).Scan(&yes)
	return yes, e
}

const postSelect = `SELECT p.id,p.text,p.visibility,p."createdAt",p."parentPostId",u.id,u.username,u."displayName",u.bio,(SELECT count(*) FROM "Comment" c JOIN "User" cu ON cu.id=c."authorId" WHERE c."postId"=p.id AND cu."disabledAt" IS NULL AND NOT EXISTS(SELECT 1 FROM "BlockedUser" b WHERE (b."blockerId"=$1 AND b."blockedId"=c."authorId") OR (b."blockerId"=c."authorId" AND b."blockedId"=$1))) FROM "Post" p JOIN "User" u ON u.id=p."authorId"`

func scanPost(row pgx.Row) (Post, error) {
	var p Post
	var u User
	e := row.Scan(&p.ID, &p.Text, &p.Visibility, &p.CreatedAt, &p.ParentPostID, &u.ID, &u.Username, &u.DisplayName, &u.Bio, &p.CommentCount)
	p.Author = u
	return p, e
}
func (s *Service) feed(w http.ResponseWriter, r *http.Request, uid string) {
	cur, err := decodeCursor(r.URL.Query().Get("cursor"))
	if err != nil {
		writeError(w, 400, "invalid cursor")
		return
	}
	q := postSelect + ` WHERE u."disabledAt" IS NULL AND NOT EXISTS(
		WITH RECURSIVE ancestors AS (
			SELECT p0.id,p0."authorId",p0.visibility,p0."parentPostId",0 AS depth FROM "Post" p0 WHERE p0.id=p.id
			UNION ALL
			SELECT parent.id,parent."authorId",parent.visibility,parent."parentPostId",a.depth+1 FROM ancestors a JOIN "Post" parent ON parent.id=a."parentPostId" WHERE a.depth<100
		)
		SELECT 1 FROM ancestors a JOIN "User" au ON au.id=a."authorId" WHERE au."disabledAt" IS NOT NULL OR a.depth=100 AND a."parentPostId" IS NOT NULL OR
			NOT (a.visibility='public' OR a."authorId"=$1 OR (a.visibility='close_friends' AND EXISTS(SELECT 1 FROM "CloseFriend" cf WHERE cf."ownerId"=a."authorId" AND cf."friendId"=$1))) OR
			EXISTS(SELECT 1 FROM "BlockedUser" b WHERE (b."blockerId"=$1 AND b."blockedId"=a."authorId") OR (b."blockerId"=a."authorId" AND b."blockedId"=$1))
	)`
	args := []any{uid}
	if cur != nil {
		q += ` AND (p."createdAt",p.id)<($2,$3)`
		args = append(args, cur.at, cur.id)
	}
	q += ` ORDER BY p."createdAt" DESC,p.id DESC LIMIT 26`
	rows, e := s.pool.Query(r.Context(), q, args...)
	if e != nil {
		writeError(w, 500, "internal server error")
		return
	}
	defer rows.Close()
	posts := []Post{}
	for rows.Next() {
		p, e := scanPost(rows)
		if e != nil {
			writeError(w, 500, "internal server error")
			return
		}
		posts = append(posts, p)
	}
	if rows.Err() != nil {
		writeError(w, 500, "internal server error")
		return
	}
	var next *string
	if len(posts) > pageSize {
		posts = posts[:pageSize]
		c := encodeCursor(posts[len(posts)-1].CreatedAt, posts[len(posts)-1].ID)
		next = &c
	}
	writeJSON(w, 200, map[string]any{"posts": posts, "nextCursor": next})
}

type cursor struct {
	at time.Time
	id string
}

func encodeCursor(t time.Time, id string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(t.UTC().Format(time.RFC3339Nano) + "\n" + id))
}
func decodeCursor(v string) (*cursor, error) {
	if v == "" {
		return nil, nil
	}
	b, e := base64.RawURLEncoding.DecodeString(v)
	if e != nil {
		return nil, e
	}
	p := strings.SplitN(string(b), "\n", 2)
	if len(p) != 2 || p[1] == "" {
		return nil, errors.New("malformed cursor")
	}
	t, e := time.Parse(time.RFC3339Nano, p[0])
	if e != nil {
		return nil, e
	}
	return &cursor{t, p[1]}, nil
}
func (s *Service) getPost(w http.ResponseWriter, r *http.Request, uid string) {
	ok, e := s.canSeePost(r.Context(), uid, chi.URLParam(r, "id"))
	if e != nil {
		writeError(w, 500, "internal server error")
		return
	}
	if !ok {
		writeError(w, 404, "post not found")
		return
	}
	p, e := scanPost(s.pool.QueryRow(r.Context(), postSelect+` WHERE p.id=$2`, uid, chi.URLParam(r, "id")))
	if e != nil {
		writeError(w, 500, "internal server error")
		return
	}
	writeJSON(w, 200, map[string]any{"post": p})
}
func (s *Service) createPost(w http.ResponseWriter, r *http.Request, uid string) {
	var in struct {
		Text       string  `json:"text"`
		Visibility string  `json:"visibility"`
		Parent     *string `json:"parentPostId"`
	}
	if !decode(w, r, &in) {
		return
	}
	in.Text = strings.TrimSpace(in.Text)
	if len([]rune(in.Text)) == 0 || len([]rune(in.Text)) > 5000 || (in.Visibility != "public" && in.Visibility != "close_friends") {
		writeError(w, 400, "invalid post")
		return
	}
	if in.Parent != nil {
		ok, e := s.canSeePost(r.Context(), uid, *in.Parent)
		if e != nil {
			writeError(w, 500, "internal server error")
			return
		}
		if !ok {
			writeError(w, 404, "parent post not found")
			return
		}
	}
	id, ok := idOrError(w)
	if !ok {
		return
	}
	if _, e := s.pool.Exec(r.Context(), `INSERT INTO "Post"(id,"authorId",text,visibility,"parentPostId") VALUES($1,$2,$3,$4,$5)`, id, uid, in.Text, in.Visibility, in.Parent); e != nil {
		writeError(w, 500, "internal server error")
		return
	}
	p, e := scanPost(s.pool.QueryRow(r.Context(), postSelect+` WHERE p.id=$2`, uid, id))
	if e != nil {
		writeError(w, 500, "internal server error")
		return
	}
	writeJSON(w, 201, map[string]any{"post": p})
}
func (s *Service) deletePost(w http.ResponseWriter, r *http.Request, uid string) {
	tag, e := s.pool.Exec(r.Context(), `DELETE FROM "Post" WHERE id=$1 AND "authorId"=$2`, chi.URLParam(r, "id"), uid)
	if e != nil {
		writeError(w, 500, "internal server error")
		return
	}
	if tag.RowsAffected() == 0 {
		writeError(w, 404, "post not found")
		return
	}
	w.WriteHeader(204)
}
func (s *Service) getComments(w http.ResponseWriter, r *http.Request, uid string) {
	post := chi.URLParam(r, "id")
	ok, e := s.canSeePost(r.Context(), uid, post)
	if e != nil {
		writeError(w, 500, "internal server error")
		return
	}
	if !ok {
		writeError(w, 404, "post not found")
		return
	}
	rows, e := s.pool.Query(r.Context(), `SELECT c.id,c.text,c."createdAt",u.id,u.username,u."displayName",u.bio FROM "Comment" c JOIN "User" u ON u.id=c."authorId" WHERE c."postId"=$1 AND u."disabledAt" IS NULL AND NOT EXISTS(SELECT 1 FROM "BlockedUser" b WHERE (b."blockerId"=$2 AND b."blockedId"=c."authorId") OR (b."blockerId"=c."authorId" AND b."blockedId"=$2)) ORDER BY c."createdAt" ASC,c.id ASC LIMIT 500`, post, uid)
	if e != nil {
		writeError(w, 500, "internal server error")
		return
	}
	defer rows.Close()
	out := []Comment{}
	for rows.Next() {
		var c Comment
		if e = rows.Scan(&c.ID, &c.Text, &c.CreatedAt, &c.Author.ID, &c.Author.Username, &c.Author.DisplayName, &c.Author.Bio); e != nil {
			writeError(w, 500, "internal server error")
			return
		}
		out = append(out, c)
	}
	if rows.Err() != nil {
		writeError(w, 500, "internal server error")
		return
	}
	if rows.Err() != nil {
		writeError(w, 500, "internal server error")
		return
	}
	writeJSON(w, 200, map[string]any{"comments": out})
}
func (s *Service) createComment(w http.ResponseWriter, r *http.Request, uid string) {
	var in struct {
		Text string `json:"text"`
	}
	if !decode(w, r, &in) {
		return
	}
	in.Text = strings.TrimSpace(in.Text)
	if in.Text == "" || len([]rune(in.Text)) > 2000 {
		writeError(w, 400, "invalid comment")
		return
	}
	post := chi.URLParam(r, "id")
	ok, e := s.canSeePost(r.Context(), uid, post)
	if e != nil {
		writeError(w, 500, "internal server error")
		return
	}
	if !ok {
		writeError(w, 404, "post not found")
		return
	}
	id, ok := idOrError(w)
	if !ok {
		return
	}
	var c Comment
	e = s.pool.QueryRow(r.Context(), `INSERT INTO "Comment"(id,"postId","authorId",text) VALUES($1,$2,$3,$4) RETURNING id,text,"createdAt"`, id, post, uid, in.Text).Scan(&c.ID, &c.Text, &c.CreatedAt)
	if e != nil {
		writeError(w, 500, "internal server error")
		return
	}
	c.Author, e = s.user(r.Context(), uid)
	if e != nil {
		writeError(w, 500, "internal server error")
		return
	}
	writeJSON(w, 201, map[string]any{"comment": c})
}
func (s *Service) members(w http.ResponseWriter, r *http.Request, uid string) {
	rows, e := s.pool.Query(r.Context(), `SELECT u.id,u.username,u."displayName",u.bio FROM "User" u WHERE u.id<>$1 AND u."disabledAt" IS NULL AND NOT EXISTS(SELECT 1 FROM "BlockedUser" b WHERE (b."blockerId"=$1 AND b."blockedId"=u.id) OR (b."blockerId"=u.id AND b."blockedId"=$1)) ORDER BY u.username LIMIT 200`, uid)
	if e != nil {
		writeError(w, 500, "internal server error")
		return
	}
	defer rows.Close()
	out := []User{}
	for rows.Next() {
		var u User
		if e = rows.Scan(&u.ID, &u.Username, &u.DisplayName, &u.Bio); e != nil {
			writeError(w, 500, "internal server error")
			return
		}
		out = append(out, u)
	}
	if rows.Err() != nil {
		writeError(w, 500, "internal server error")
		return
	}
	writeJSON(w, 200, map[string]any{"users": out})
}
func (s *Service) patchProfile(w http.ResponseWriter, r *http.Request, uid string) {
	var in struct {
		DisplayName *string `json:"displayName"`
		Bio         *string `json:"bio"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.DisplayName == nil || in.Bio == nil || len([]rune(*in.DisplayName)) > 80 || len([]rune(*in.Bio)) > 500 {
		writeError(w, 400, "invalid profile")
		return
	}
	var u User
	e := s.pool.QueryRow(r.Context(), `UPDATE "User" SET "displayName"=$2,bio=$3 WHERE id=$1 RETURNING id,username,"displayName",bio`, uid, strings.TrimSpace(*in.DisplayName), strings.TrimSpace(*in.Bio)).Scan(&u.ID, &u.Username, &u.DisplayName, &u.Bio)
	if e != nil {
		writeError(w, 404, "user not found")
		return
	}
	writeJSON(w, 200, map[string]any{"user": u})
}
func (s *Service) relationships(w http.ResponseWriter, r *http.Request, uid string) {
	out := map[string][]string{"following": {}, "closeFriends": {}, "blocked": {}}
	for name, q := range map[string]string{"following": `SELECT "followeeId" FROM "Follow" WHERE "followerId"=$1 ORDER BY "followeeId"`, "closeFriends": `SELECT "friendId" FROM "CloseFriend" WHERE "ownerId"=$1 ORDER BY "friendId"`, "blocked": `SELECT "blockedId" FROM "BlockedUser" WHERE "blockerId"=$1 ORDER BY "blockedId"`} {
		rows, e := s.pool.Query(r.Context(), q, uid)
		if e != nil {
			writeError(w, 500, "internal server error")
			return
		}
		for rows.Next() {
			var id string
			if e = rows.Scan(&id); e != nil {
				rows.Close()
				writeError(w, 500, "internal server error")
				return
			}
			out[name] = append(out[name], id)
		}
		rows.Close()
	}
	writeJSON(w, 200, out)
}
func relationship(kind string) (table, from, to string, ok bool) {
	switch kind {
	case "follow":
		return `Follow`, `followerId`, `followeeId`, true
	case "close_friend":
		return `CloseFriend`, `ownerId`, `friendId`, true
	case "block":
		return `BlockedUser`, `blockerId`, `blockedId`, true
	}
	return "", "", "", false
}
func (s *Service) changeRelationship(w http.ResponseWriter, r *http.Request, uid string, del bool) {
	kind, target := chi.URLParam(r, "kind"), chi.URLParam(r, "id")
	table, from, to, ok := relationship(kind)
	if !ok || target == uid {
		writeError(w, 400, "invalid relationship")
		return
	}
	if _, e := s.user(r.Context(), target); e != nil {
		writeError(w, 404, "user not found")
		return
	}
	if !del && kind != "block" {
		blocked, e := s.blocked(r.Context(), uid, target)
		if e != nil {
			writeError(w, 500, "internal server error")
			return
		}
		if blocked {
			writeError(w, 409, "relationship blocked")
			return
		}
	}
	q := fmt.Sprintf(`INSERT INTO "%s"(id,"%s","%s") VALUES($1,$2,$3) ON CONFLICT("%s","%s") DO NOTHING`, table, from, to, from, to)
	if del {
		q = fmt.Sprintf(`DELETE FROM "%s" WHERE "%s"=$1 AND "%s"=$2`, table, from, to)
		if _, e := s.pool.Exec(r.Context(), q, uid, target); e != nil {
			writeError(w, 500, "internal server error")
			return
		}
	} else {
		id, e := newID()
		if e != nil {
			writeError(w, 500, "internal server error")
			return
		}
		if kind == "block" {
			tx, beginErr := s.pool.Begin(r.Context())
			if beginErr != nil {
				writeError(w, 500, "internal server error")
				return
			}
			defer tx.Rollback(r.Context())
			if _, e = tx.Exec(r.Context(), q, id, uid, target); e == nil {
				_, e = tx.Exec(r.Context(), `DELETE FROM "Follow" WHERE ("followerId"=$1 AND "followeeId"=$2) OR ("followerId"=$2 AND "followeeId"=$1)`, uid, target)
			}
			if e == nil {
				_, e = tx.Exec(r.Context(), `DELETE FROM "CloseFriend" WHERE ("ownerId"=$1 AND "friendId"=$2) OR ("ownerId"=$2 AND "friendId"=$1)`, uid, target)
			}
			if e == nil {
				e = tx.Commit(r.Context())
			}
			if e != nil {
				writeError(w, 500, "internal server error")
				return
			}
		} else if _, e = s.pool.Exec(r.Context(), q, id, uid, target); e != nil {
			writeError(w, 500, "internal server error")
			return
		}
	}
	w.WriteHeader(204)
}
func (s *Service) putRelationship(w http.ResponseWriter, r *http.Request, uid string) {
	s.changeRelationship(w, r, uid, false)
}
func (s *Service) deleteRelationship(w http.ResponseWriter, r *http.Request, uid string) {
	s.changeRelationship(w, r, uid, true)
}
func (s *Service) report(w http.ResponseWriter, r *http.Request, uid string) {
	var in struct {
		TargetType string `json:"targetType"`
		TargetID   string `json:"targetId"`
		Reason     string `json:"reason"`
	}
	if !decode(w, r, &in) {
		return
	}
	in.Reason = strings.TrimSpace(in.Reason)
	if (in.TargetType != "post" && in.TargetType != "user") || in.TargetID == "" || in.Reason == "" || len([]rune(in.Reason)) > 1000 {
		writeError(w, 400, "invalid report")
		return
	}
	if in.TargetType == "post" {
		ok, e := s.canSeePost(r.Context(), uid, in.TargetID)
		if e != nil {
			writeError(w, 500, "internal server error")
			return
		}
		if !ok {
			writeError(w, 404, "target not found")
			return
		}
	} else if _, e := s.user(r.Context(), in.TargetID); e != nil {
		writeError(w, 404, "target not found")
		return
	}
	id, ok := idOrError(w)
	if !ok {
		return
	}
	_, e := s.pool.Exec(r.Context(), `INSERT INTO "Report"(id,"reporterId","targetType","targetId",reason) VALUES($1,$2,$3,$4,$5)`, id, uid, in.TargetType, in.TargetID, in.Reason)
	if e != nil {
		writeError(w, 500, "internal server error")
		return
	}
	w.WriteHeader(204)
}
func (s *Service) conversations(w http.ResponseWriter, r *http.Request, uid string) {
	rows, e := s.pool.Query(r.Context(), `SELECT c.id,c.name FROM "Conversation" c JOIN "ConversationMember" m ON m."conversationId"=c.id WHERE m."userId"=$1 AND NOT EXISTS(SELECT 1 FROM "ConversationMember" other JOIN "BlockedUser" b ON ((b."blockerId"=$1 AND b."blockedId"=other."userId") OR (b."blockerId"=other."userId" AND b."blockedId"=$1)) WHERE other."conversationId"=c.id AND other."userId"<>$1) ORDER BY c."createdAt" DESC,c.id DESC LIMIT 200`, uid)
	if e != nil {
		writeError(w, 500, "internal server error")
		return
	}
	defer rows.Close()
	ids := []Conversation{}
	for rows.Next() {
		var c Conversation
		if e = rows.Scan(&c.ID, &c.Name); e != nil {
			writeError(w, 500, "internal server error")
			return
		}
		ids = append(ids, c)
	}
	if rows.Err() != nil {
		writeError(w, 500, "internal server error")
		return
	}
	rows.Close()
	for i := range ids {
		if s.loadMembers(r.Context(), ids[i].ID, &ids[i].Members) != nil {
			writeError(w, 500, "internal server error")
			return
		}
	}
	writeJSON(w, 200, map[string]any{"conversations": ids})
}
func (s *Service) loadMembers(ctx context.Context, cid string, dst *[]User) error {
	rows, e := s.pool.Query(ctx, `SELECT u.id,u.username,u."displayName",u.bio FROM "ConversationMember" m JOIN "User" u ON u.id=m."userId" WHERE m."conversationId"=$1 AND u."disabledAt" IS NULL ORDER BY u.username`, cid)
	if e != nil {
		return e
	}
	defer rows.Close()
	*dst = []User{}
	for rows.Next() {
		var u User
		if e = rows.Scan(&u.ID, &u.Username, &u.DisplayName, &u.Bio); e != nil {
			return e
		}
		*dst = append(*dst, u)
	}
	return rows.Err()
}
func (s *Service) createConversation(w http.ResponseWriter, r *http.Request, uid string) {
	var in struct {
		MemberIDs []string `json:"memberIds"`
		Name      *string  `json:"name"`
	}
	if !decode(w, r, &in) {
		return
	}
	if len(in.MemberIDs) > 49 || (in.Name != nil && len([]rune(*in.Name)) > 100) {
		writeError(w, 400, "invalid conversation")
		return
	}
	seen := map[string]bool{uid: true}
	ids := []string{uid}
	for _, id := range in.MemberIDs {
		if seen[id] {
			writeError(w, 400, "invalid conversation members")
			return
		}
		seen[id] = true
		if _, e := s.user(r.Context(), id); e != nil {
			writeError(w, 400, "invalid conversation members")
			return
		}
		blocked, e := s.blocked(r.Context(), uid, id)
		if e != nil {
			writeError(w, 500, "internal server error")
			return
		}
		if blocked {
			writeError(w, 400, "invalid conversation members")
			return
		}
		ids = append(ids, id)
	}
	if len(ids) < 2 {
		writeError(w, 400, "conversation needs another member")
		return
	}
	id, ok := idOrError(w)
	if !ok {
		return
	}
	tx, e := s.pool.Begin(r.Context())
	if e != nil {
		writeError(w, 500, "internal server error")
		return
	}
	defer tx.Rollback(r.Context())
	_, e = tx.Exec(r.Context(), `INSERT INTO "Conversation"(id,name) VALUES($1,$2)`, id, in.Name)
	for _, member := range ids {
		if e == nil {
			mid, er := newID()
			if er != nil {
				e = er
			} else {
				_, e = tx.Exec(r.Context(), `INSERT INTO "ConversationMember"(id,"conversationId","userId") VALUES($1,$2,$3)`, mid, id, member)
			}
		}
	}
	if e == nil {
		e = tx.Commit(r.Context())
	}
	if e != nil {
		writeError(w, 500, "internal server error")
		return
	}
	c := Conversation{ID: id, Name: in.Name}
	if e = s.loadMembers(r.Context(), id, &c.Members); e != nil {
		writeError(w, 500, "internal server error")
		return
	}
	writeJSON(w, 201, map[string]any{"conversation": c})
}
func (s *Service) checkConversation(ctx context.Context, cid, uid string) error {
	var member bool
	if e := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM "ConversationMember" WHERE "conversationId"=$1 AND "userId"=$2)`, cid, uid).Scan(&member); e != nil {
		return e
	}
	if !member {
		return pgx.ErrNoRows
	}
	var blocked bool
	e := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM "ConversationMember" m JOIN "BlockedUser" b ON ((b."blockerId"=$2 AND b."blockedId"=m."userId") OR (b."blockerId"=m."userId" AND b."blockedId"=$2)) WHERE m."conversationId"=$1 AND m."userId"<>$2)`, cid, uid).Scan(&blocked)
	if e != nil {
		return e
	}
	if blocked {
		return errBlockedConversation
	}
	return nil
}
func (s *Service) getMessages(w http.ResponseWriter, r *http.Request, uid string) {
	cid := chi.URLParam(r, "id")
	if e := s.checkConversation(r.Context(), cid, uid); e != nil {
		if errors.Is(e, errBlockedConversation) {
			writeError(w, 403, "conversation blocked")
		} else if errors.Is(e, pgx.ErrNoRows) {
			writeError(w, 404, "conversation not found")
		} else {
			writeError(w, 500, "internal server error")
		}
		return
	}
	cur, e := decodeCursor(r.URL.Query().Get("cursor"))
	if e != nil {
		writeError(w, 400, "invalid cursor")
		return
	}
	q := `SELECT m.id,m.text,m."createdAt",u.id,u.username,u."displayName",u.bio FROM "Message" m JOIN "User" u ON u.id=m."senderId" WHERE m."conversationId"=$1 AND u."disabledAt" IS NULL`
	args := []any{cid}
	if cur != nil {
		q += ` AND (m."createdAt",m.id)<($2,$3)`
		args = append(args, cur.at, cur.id)
	}
	q += ` ORDER BY m."createdAt" DESC,m.id DESC LIMIT 26`
	rows, e := s.pool.Query(r.Context(), q, args...)
	if e != nil {
		writeError(w, 500, "internal server error")
		return
	}
	defer rows.Close()
	out := []Message{}
	for rows.Next() {
		var m Message
		if e = rows.Scan(&m.ID, &m.Text, &m.CreatedAt, &m.Sender.ID, &m.Sender.Username, &m.Sender.DisplayName, &m.Sender.Bio); e != nil {
			writeError(w, 500, "internal server error")
			return
		}
		out = append(out, m)
	}
	if rows.Err() != nil {
		writeError(w, 500, "internal server error")
		return
	}
	var next *string
	if len(out) > pageSize {
		out = out[:pageSize]
		v := encodeCursor(out[len(out)-1].CreatedAt, out[len(out)-1].ID)
		next = &v
	}
	writeJSON(w, 200, map[string]any{"messages": out, "nextCursor": next})
}
func (s *Service) createMessage(w http.ResponseWriter, r *http.Request, uid string) {
	var in struct {
		Text string `json:"text"`
	}
	if !decode(w, r, &in) {
		return
	}
	in.Text = strings.TrimSpace(in.Text)
	if in.Text == "" || len([]rune(in.Text)) > 4000 {
		writeError(w, 400, "invalid message")
		return
	}
	cid := chi.URLParam(r, "id")
	if e := s.checkConversation(r.Context(), cid, uid); e != nil {
		if errors.Is(e, errBlockedConversation) {
			writeError(w, 403, "conversation blocked")
		} else if errors.Is(e, pgx.ErrNoRows) {
			writeError(w, 404, "conversation not found")
		} else {
			writeError(w, 500, "internal server error")
		}
		return
	}
	id, ok := idOrError(w)
	if !ok {
		return
	}
	var m Message
	e := s.pool.QueryRow(r.Context(), `INSERT INTO "Message"(id,"conversationId","senderId",text) VALUES($1,$2,$3,$4) RETURNING id,text,"createdAt"`, id, cid, uid, in.Text).Scan(&m.ID, &m.Text, &m.CreatedAt)
	if e != nil {
		writeError(w, 500, "internal server error")
		return
	}
	m.Sender, e = s.user(r.Context(), uid)
	if e != nil {
		writeError(w, 500, "internal server error")
		return
	}
	writeJSON(w, 201, map[string]any{"message": m})
}
