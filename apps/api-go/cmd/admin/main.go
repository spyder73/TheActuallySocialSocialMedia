// Command admin performs one-time account administration actions against PostgreSQL.
package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spyder73/TheActuallySocialSocialMedia/apps/api-go/internal/config"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		slog.Error("admin command failed", "error", err)
		os.Exit(1)
	}
}
func run(args []string) error {
	if len(args) < 1 {
		return errors.New("usage: admin invite [--admin] EMAIL | reset USER | disable USER | reports | remove-post POST_ID | sessions revoke USER | sessions revoke-all")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return errors.New("invalid database configuration")
	}
	defer pool.Close()
	if err = pool.Ping(ctx); err != nil {
		return errors.New("database connection failed")
	}
	switch args[0] {
	case "invite":
		adminInvite := len(args) == 3 && args[1] == "--admin"
		if (!adminInvite && len(args) != 2) || (adminInvite && len(args) != 3) {
			return errors.New("usage: admin invite [--admin] EMAIL")
		}
		emailArg := args[len(args)-1]
		return issueInvite(ctx, pool, emailArg, adminInvite)
	case "reset":
		if len(args) != 2 {
			return errors.New("usage: admin reset USER")
		}
		return issueReset(ctx, pool, args[1])
	case "disable":
		if len(args) != 2 {
			return errors.New("usage: admin disable USER")
		}
		return disable(ctx, pool, args[1])
	case "reports":
		if len(args) != 1 {
			return errors.New("usage: admin reports")
		}
		return listReports(ctx, pool)
	case "remove-post":
		if len(args) != 2 {
			return errors.New("usage: admin remove-post POST_ID")
		}
		return removePost(ctx, pool, args[1])
	case "sessions":
		if len(args) == 2 && args[1] == "revoke-all" {
			return revokeAll(ctx, pool)
		}
		if len(args) != 3 || args[1] != "revoke" {
			return errors.New("usage: admin sessions revoke USER | sessions revoke-all")
		}
		return revokeSessions(ctx, pool, args[2])
	default:
		return errors.New("unknown admin command")
	}
}
func issueInvite(ctx context.Context, pool *pgxpool.Pool, email string, adminInvite bool) error {
	email = strings.ToLower(strings.TrimSpace(email))
	if len(email) < 3 || len(email) > 254 || strings.Count(email, "@") != 1 || strings.ContainsAny(email, " \t\r\n") {
		return errors.New("invalid email")
	}
	origin, err := appOrigin()
	if err != nil {
		return err
	}
	token, err := randomToken()
	if err != nil {
		return err
	}
	id, err := randomToken()
	if err != nil {
		return err
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(7247202602)`); err != nil {
		return err
	}
	role := "user"
	if adminInvite {
		var pendingAdmin bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM "AccountToken" WHERE "purpose"='invite' AND "inviteRole"='admin' AND "usedAt" IS NULL AND "expiresAt">now())`).Scan(&pendingAdmin); err != nil {
			return err
		}
		var admins bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM "User" WHERE "role"='admin')`).Scan(&admins); err != nil {
			return err
		}
		if pendingAdmin || admins {
			return errors.New("an admin or pending admin invitation already exists")
		}
		role = "admin"
	}
	if _, err = tx.Exec(ctx, `UPDATE "AccountToken" SET "usedAt"=now() WHERE "purpose"='invite' AND lower("email")=$1 AND "usedAt" IS NULL`, email); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO "AccountToken" ("id","tokenHash","purpose","email","inviteRole","expiresAt") VALUES ($1,$2,'invite',$3,$4,$5)`, id, digest(token), email, role, time.Now().Add(7*24*time.Hour))
	if err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	fmt.Printf("Invitation created for %s (role %s), expires in 7 days. One-time link: %s/register#token=%s\n", email, role, origin, token)
	return nil
}
func issueReset(ctx context.Context, pool *pgxpool.Pool, user string) error {
	origin, err := appOrigin()
	if err != nil {
		return err
	}
	token, err := randomToken()
	if err != nil {
		return err
	}
	id, err := randomToken()
	if err != nil {
		return err
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var userID, email string
	err = tx.QueryRow(ctx, `SELECT "id","email" FROM "User" WHERE "disabledAt" IS NULL AND ("id"=$1 OR lower("email")=lower($1) OR lower("username")=lower($1))`, strings.TrimSpace(user)).Scan(&userID, &email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errors.New("user not found")
		}
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE "AccountToken" SET "usedAt"=now() WHERE "purpose"='reset' AND "userId"=$1 AND "usedAt" IS NULL`, userID); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO "AccountToken" ("id","tokenHash","purpose","email","userId","expiresAt") VALUES ($1,$2,'reset',$3,$4,$5)`, id, digest(token), email, userID, time.Now().Add(time.Hour))
	if err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	fmt.Printf("Password reset link for %s expires in 1 hour: %s/reset#token=%s\n", email, origin, token)
	return nil
}
func disable(ctx context.Context, pool *pgxpool.Pool, user string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	tag, err := tx.Exec(ctx, `UPDATE "User" SET "disabledAt"=COALESCE("disabledAt",now()) WHERE "id"=$1 OR lower("email")=lower($1) OR lower("username")=lower($1)`, strings.TrimSpace(user))
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return errors.New("user not found or ambiguous")
	}
	if _, err = tx.Exec(ctx, `UPDATE "AuthSession" SET "revokedAt"=now() WHERE "userId"=(SELECT "id" FROM "User" WHERE "id"=$1 OR lower("email")=lower($1) OR lower("username")=lower($1)) AND "revokedAt" IS NULL`, strings.TrimSpace(user)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func revokeSessions(ctx context.Context, pool *pgxpool.Pool, user string) error {
	var id string
	err := pool.QueryRow(ctx, `SELECT "id" FROM "User" WHERE "id"=$1 OR lower("email")=lower($1) OR lower("username")=lower($1)`, strings.TrimSpace(user)).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return errors.New("user not found")
	}
	if err != nil {
		return err
	}
	_, err = pool.Exec(ctx, `UPDATE "AuthSession" SET "revokedAt"=now() WHERE "userId"=$1 AND "revokedAt" IS NULL`, id)
	return err
}
func listReports(ctx context.Context, pool *pgxpool.Pool) error {
	type report struct {
		ID               string    `json:"id"`
		TargetType       string    `json:"targetType"`
		TargetID         string    `json:"targetId"`
		Reason           string    `json:"reason"`
		CreatedAt        time.Time `json:"createdAt"`
		ReporterID       string    `json:"reporterId"`
		ReporterEmail    string    `json:"reporterEmail"`
		ReporterUsername string    `json:"reporterUsername"`
	}
	rows, err := pool.Query(ctx, `SELECT r."id",r."targetType",r."targetId",left(r."reason",2000),r."createdAt",u."id",u."email",u."username" FROM "Report" r JOIN "User" u ON u."id"=r."reporterId" ORDER BY r."createdAt" DESC LIMIT 100`)
	if err != nil {
		return err
	}
	defer rows.Close()
	result := make([]report, 0, 100)
	for rows.Next() {
		var v report
		if err = rows.Scan(&v.ID, &v.TargetType, &v.TargetID, &v.Reason, &v.CreatedAt, &v.ReporterID, &v.ReporterEmail, &v.ReporterUsername); err != nil {
			return err
		}
		result = append(result, v)
	}
	if err = rows.Err(); err != nil {
		return err
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(true)
	return enc.Encode(result)
}
func removePost(ctx context.Context, pool *pgxpool.Pool, id string) error {
	tag, err := pool.Exec(ctx, `DELETE FROM "Post" WHERE "id"=$1`, strings.TrimSpace(id))
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return errors.New("post not found")
	}
	return nil
}
func revokeAll(ctx context.Context, pool *pgxpool.Pool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `UPDATE "AuthSession" SET "revokedAt"=now() WHERE "revokedAt" IS NULL`); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE "AccountToken" SET "usedAt"=now() WHERE "usedAt" IS NULL`); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
func digest(token string) []byte { s := sha256.Sum256([]byte(token)); return s[:] }
func appOrigin() (string, error) {
	raw := strings.TrimSpace(os.Getenv("APP_ORIGIN"))
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || u.User != nil || u.Path != "" && u.Path != "/" || u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("APP_ORIGIN must be a complete http(s) origin")
	}
	return strings.ToLower(u.Scheme + "://" + u.Host), nil
}
