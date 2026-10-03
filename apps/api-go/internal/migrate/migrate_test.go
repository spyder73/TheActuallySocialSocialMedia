package migrate

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestEmbeddedBaselineMatchesPrismaSQL(t *testing.T) {
	got, err := loadMigrations("")
	if err != nil {
		t.Fatal(err)
	}
	if len(onlyBaseline(got)) != 4 || len(got) != 5 {
		t.Fatalf("got %d migrations (%d baseline), want 5 (4 baseline)", len(got), len(onlyBaseline(got)))
	}
	for _, m := range got {
		if !m.baseline {
			continue
		}
		source := filepath.Join("..", "..", "..", "api", "prisma", "migrations", m.version, "migration.sql")
		want, err := os.ReadFile(source)
		if err != nil {
			t.Fatalf("read Prisma source %s: %v", source, err)
		}
		if string(want) != m.sql {
			t.Errorf("embedded migration %s differs from Prisma source", m.version)
		}
		sum := sha256.Sum256(want)
		if m.checksum != hex.EncodeToString(sum[:]) {
			t.Errorf("embedded checksum for %s does not match Prisma source", m.version)
		}
	}
}

func TestLoadsPrismaDirectoryLayout(t *testing.T) {
	dir := filepath.Join("..", "..", "..", "api", "prisma", "migrations")
	got, err := loadMigrations(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(onlyBaseline(got)) != 4 {
		t.Fatalf("got %d Prisma migrations, want 4", len(onlyBaseline(got)))
	}
}

// The test creates and drops a unique database using TEST_DATABASE_URL's
// credentials. That URL must point to a PostgreSQL database where CREATE/DROP
// DATABASE is permitted, never to production.
func TestMigrationIntegration(t *testing.T) {
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
	t.Cleanup(func() { _ = admin.Close(context.Background()) })
	var suffix [8]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		t.Fatal(err)
	}
	database := fmt.Sprintf("tassm_migrate_test_%x", suffix[:])
	quotedDatabase := (pgx.Identifier{database}).Sanitize()
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+quotedDatabase); err != nil {
		t.Fatalf("create isolated test database: %v", err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(context.Background(), "DROP DATABASE "+quotedDatabase+" WITH (FORCE)"); err != nil {
			t.Errorf("drop isolated test database: %v", err)
		}
	})
	target := cfg.ConnConfig.Copy()
	target.Database = database
	targetPoolCfg := cfg.Copy()
	targetPoolCfg.ConnConfig = target
	pool, err := pgxpool.NewWithConfig(ctx, targetPoolCfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	if err := Run(ctx, pool, "", logger); err != nil {
		t.Fatalf("fresh migration: %v", err)
	}
	if err := Run(ctx, pool, "", logger); err != nil {
		t.Fatalf("idempotent rerun: %v", err)
	}
	var applied int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM "_tassm_go_migrations"`).Scan(&applied); err != nil {
		t.Fatal(err)
	}
	if applied != 5 {
		t.Fatalf("Go ledger has %d migrations, want 5", applied)
	}

	changedDir := t.TempDir()
	if err := copyMigrations(changedDir); err != nil {
		t.Fatal(err)
	}
	changedSQL := filepath.Join(changedDir, "prisma-baseline", gotFirstVersion(t)+".sql")
	contents, err := os.ReadFile(changedSQL)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(changedSQL, append(contents, []byte("\n-- altered checksum\n")...), 0600); err != nil {
		t.Fatal(err)
	}
	if err := Run(ctx, pool, changedDir, logger); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("changed checksum should fail closed, got %v", err)
	}

	// Simulate a database created by the existing Prisma runner: retain its
	// schema, replace the Go ledger with verified Prisma ledger rows, then adopt.
	if _, err := pool.Exec(ctx, `DROP TABLE "AuthSession", "AccountToken";
 DROP INDEX "User_email_lower_key", "User_username_lower_key";
		ALTER TABLE "User" DROP CONSTRAINT "User_role_check", DROP COLUMN "role", DROP COLUMN "disabledAt";
		DROP TABLE "_tassm_go_migrations"; CREATE TABLE "_prisma_migrations" (
		migration_name TEXT NOT NULL, checksum TEXT NOT NULL, finished_at TIMESTAMPTZ, rolled_back_at TIMESTAMPTZ)`); err != nil {
		t.Fatal(err)
	}
	migrations, err := loadMigrations("")
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range onlyBaseline(migrations) {
		if _, err := pool.Exec(ctx, `INSERT INTO "_prisma_migrations" (migration_name, checksum, finished_at) VALUES ($1, $2, now())`, m.version, m.checksum); err != nil {
			t.Fatal(err)
		}
	}
	if err := Run(ctx, pool, "", logger); err != nil {
		t.Fatalf("Prisma baseline adoption: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM "_tassm_go_migrations"`).Scan(&applied); err != nil {
		t.Fatal(err)
	}
	if applied != 5 {
		t.Fatalf("adopted Go ledger has %d migrations, want 5", applied)
	}

	if _, err := pool.Exec(ctx, `DROP TABLE "_prisma_migrations"; DROP TABLE "_tassm_go_migrations"`); err != nil {
		t.Fatal(err)
	}
	if err := Run(ctx, pool, "", logger); err == nil || !strings.Contains(err.Error(), "no recognized") {
		t.Fatalf("populated database without a ledger should fail closed, got %v", err)
	}
}

func gotFirstVersion(t *testing.T) string {
	t.Helper()
	migrations, err := loadMigrations("")
	if err != nil {
		t.Fatal(err)
	}
	return migrations[0].version
}

func copyMigrations(destination string) error {
	migrations, err := loadMigrations("")
	if err != nil {
		return err
	}
	dir := filepath.Join(destination, "prisma-baseline")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	for _, m := range migrations {
		child := "go"
		if m.baseline {
			child = "prisma-baseline"
		}
		if err := os.MkdirAll(filepath.Join(destination, child), 0700); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(destination, child, m.version+".sql"), []byte(m.sql), 0600); err != nil {
			return err
		}
	}
	return nil
}
