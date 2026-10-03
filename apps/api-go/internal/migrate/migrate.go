package migrate

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Prisma's original SQL is copied byte-for-byte into this module. The checksums
// are compared with _prisma_migrations before an existing schema is adopted.
//
//go:embed migrations/prisma-baseline/*.sql
var baseline embed.FS

const ledger = `CREATE TABLE IF NOT EXISTS "_tassm_go_migrations" (
	"version" TEXT PRIMARY KEY,
	"checksum" TEXT NOT NULL,
	"applied_at" TIMESTAMPTZ NOT NULL DEFAULT now()
)`

type migration struct{ version, sql, checksum string }

func Run(ctx context.Context, pool *pgxpool.Pool, migrationsDir string, logger *slog.Logger) error {
	if logger == nil {
		logger = slog.Default()
	}
	migrations, err := loadMigrations(migrationsDir)
	if err != nil {
		return err
	}
	if len(migrations) == 0 {
		return errors.New("no SQL migrations found")
	}

	var prismaExists, goLedgerExists bool
	if err := pool.QueryRow(ctx, `SELECT to_regclass('public."_prisma_migrations"') IS NOT NULL`).Scan(&prismaExists); err != nil {
		return err
	}
	if err := pool.QueryRow(ctx, `SELECT to_regclass('public."_tassm_go_migrations"') IS NOT NULL`).Scan(&goLedgerExists); err != nil {
		return err
	}

	if prismaExists {
		if err := verifyPrismaLedger(ctx, pool, migrations); err != nil {
			return err
		}
		if goLedgerExists {
			if err := verifyGoLedger(ctx, pool, migrations, true); err != nil {
				return err
			}
		}
		if err := adoptPrismaLedger(ctx, pool, migrations); err != nil {
			return err
		}
		logger.Info("verified and adopted existing Prisma migration history", "migrations", len(migrations))
		return nil
	}
	if !goLedgerExists {
		var hasApplicationTables bool
		if err := pool.QueryRow(ctx, `SELECT EXISTS (
			SELECT 1 FROM pg_catalog.pg_tables
			WHERE schemaname = 'public' AND tablename NOT LIKE 'pg_%'
			AND tablename <> '_tassm_go_migrations'
		)`).Scan(&hasApplicationTables); err != nil {
			return err
		}
		if hasApplicationTables {
			return errors.New("database has application tables but no recognized Prisma or Go migration ledger; refusing to modify it")
		}
	} else if err := verifyGoLedger(ctx, pool, migrations, false); err != nil {
		return err
	}
	return applyPending(ctx, pool, migrations, logger)
}

func verifyGoLedger(ctx context.Context, pool *pgxpool.Pool, migrations []migration, requireAll bool) error {
	rows, err := pool.Query(ctx, `SELECT version, checksum FROM "_tassm_go_migrations"`)
	if err != nil {
		return fmt.Errorf("read Go migration ledger: %w", err)
	}
	defer rows.Close()
	expected := make(map[string]string, len(migrations))
	for _, m := range migrations {
		expected[m.version] = m.checksum
	}
	seen := make(map[string]bool, len(migrations))
	for rows.Next() {
		var version, checksum string
		if err := rows.Scan(&version, &checksum); err != nil {
			return err
		}
		want, ok := expected[version]
		if !ok || want != checksum {
			return fmt.Errorf("Go migration ledger entry %q is unknown or has a different checksum", version)
		}
		seen[version] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if requireAll {
		for version := range expected {
			if !seen[version] {
				return fmt.Errorf("Go migration ledger is missing baseline entry %q", version)
			}
		}
	}
	return nil
}

func loadMigrations(dir string) ([]migration, error) {
	var source fs.FS = baseline
	root := "migrations/prisma-baseline"
	if dir != "" {
		if st, err := os.Stat(dir); err != nil {
			return nil, fmt.Errorf("MIGRATIONS_DIR: %w", err)
		} else if !st.IsDir() {
			return nil, fmt.Errorf("MIGRATIONS_DIR %q is not a directory", dir)
		}
		source, root = os.DirFS(dir), "."
	}
	var paths []string
	err := fs.WalkDir(source, root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("read migrations: %w", err)
	}
	sort.Strings(paths)
	result := make([]migration, 0, len(paths))
	for _, path := range paths {
		contents, err := fs.ReadFile(source, path)
		if err != nil {
			return nil, err
		}
		version := strings.TrimSuffix(filepath.Base(path), ".sql")
		if filepath.Base(path) == "migration.sql" {
			version = filepath.Base(filepath.Dir(path))
		}
		if version == "" || strings.ContainsAny(version, " \t\n") {
			return nil, fmt.Errorf("invalid migration filename %q", path)
		}
		sum := sha256.Sum256(contents)
		result = append(result, migration{version: version, sql: string(contents), checksum: hex.EncodeToString(sum[:])})
	}
	for i := 1; i < len(result); i++ {
		if result[i-1].version == result[i].version {
			return nil, fmt.Errorf("duplicate migration version %q", result[i].version)
		}
	}
	return result, nil
}

func verifyPrismaLedger(ctx context.Context, pool *pgxpool.Pool, migrations []migration) error {
	rows, err := pool.Query(ctx, `SELECT migration_name, checksum, finished_at IS NOT NULL, rolled_back_at IS NOT NULL
		FROM "_prisma_migrations" ORDER BY migration_name`)
	if err != nil {
		return fmt.Errorf("read Prisma migration ledger: %w", err)
	}
	defer rows.Close()
	byVersion := make(map[string]migration, len(migrations))
	for _, m := range migrations {
		byVersion[m.version] = m
	}
	seen := make(map[string]bool, len(migrations))
	for rows.Next() {
		var name, checksum string
		var finished, rolledBack bool
		if err := rows.Scan(&name, &checksum, &finished, &rolledBack); err != nil {
			return err
		}
		if rolledBack {
			continue
		}
		m, ok := byVersion[name]
		if !ok {
			return fmt.Errorf("Prisma ledger contains migration %q that is not packaged by this Go service; refusing adoption", name)
		}
		if !finished {
			return fmt.Errorf("Prisma migration %q is incomplete; refusing adoption", name)
		}
		if checksum != m.checksum {
			return fmt.Errorf("Prisma migration %q checksum differs from packaged SQL; refusing adoption", name)
		}
		if seen[name] {
			return fmt.Errorf("Prisma ledger contains duplicate active migration %q", name)
		}
		seen[name] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, m := range migrations {
		if !seen[m.version] {
			return fmt.Errorf("Prisma ledger is missing completed migration %q; refusing adoption", m.version)
		}
	}
	return nil
}

func adoptPrismaLedger(ctx context.Context, pool *pgxpool.Pool, migrations []migration) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(7247202601)`); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, ledger); err != nil {
		return err
	}
	for _, m := range migrations {
		if _, err := tx.Exec(ctx, `INSERT INTO "_tassm_go_migrations" (version, checksum) VALUES ($1, $2)
			ON CONFLICT (version) DO UPDATE SET checksum = EXCLUDED.checksum
			WHERE "_tassm_go_migrations".checksum = EXCLUDED.checksum`, m.version, m.checksum); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func applyPending(ctx context.Context, pool *pgxpool.Pool, migrations []migration, logger *slog.Logger) error {
	for _, m := range migrations {
		tx, err := pool.Begin(ctx)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(7247202601)`); err == nil {
			_, err = tx.Exec(ctx, ledger)
		}
		var checksum string
		if err == nil {
			queryErr := tx.QueryRow(ctx, `SELECT checksum FROM "_tassm_go_migrations" WHERE version=$1`, m.version).Scan(&checksum)
			if queryErr == nil {
				if checksum != m.checksum {
					err = fmt.Errorf("migration %q checksum changed; refusing to continue", m.version)
				}
			} else if errors.Is(queryErr, pgx.ErrNoRows) {
				if _, err = tx.Exec(ctx, m.sql); err == nil {
					_, err = tx.Exec(ctx, `INSERT INTO "_tassm_go_migrations" (version, checksum) VALUES ($1, $2)`, m.version, m.checksum)
				}
			} else {
				err = queryErr
			}
		}
		if err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("migration %s: %w", m.version, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit migration %s: %w", m.version, err)
		}
		logger.Info("migration applied or verified", "version", m.version)
	}
	return nil
}
