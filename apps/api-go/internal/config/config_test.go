package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func env(values map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) { v, ok := values[key]; return v, ok }
}

func TestDatabaseURLFileAndExclusiveSetting(t *testing.T) {
	dir := t.TempDir()
	secret := filepath.Join(dir, "database-url")
	if err := os.WriteFile(secret, []byte("postgres://user:pass@localhost/app\n"), 0600); err != nil {
		t.Fatal(err)
	}
	c, err := LoadFrom(env(map[string]string{"DATABASE_URL_FILE": secret, "LISTEN_ADDR": ":8080", "MIGRATIONS_DIR": "/app/migrations"}))
	if err != nil {
		t.Fatal(err)
	}
	if c.DatabaseURL != "postgres://user:pass@localhost/app" || c.HTTPAddr != ":8080" || c.MigrationsDir != "/app/migrations" {
		t.Fatalf("unexpected config: %+v", c)
	}
	if _, err := LoadFrom(env(map[string]string{"DATABASE_URL": "postgres://localhost/db", "DATABASE_URL_FILE": secret})); err == nil {
		t.Fatal("expected mutually exclusive setting error")
	}
}

func TestLoadDefaultsAndRequiredDatabase(t *testing.T) {
	if _, err := LoadFrom(env(map[string]string{})); err == nil || !strings.Contains(err.Error(), "DATABASE_URL") {
		t.Fatalf("expected missing DATABASE_URL error, got %v", err)
	}
	c, err := LoadFrom(env(map[string]string{"DATABASE_URL": "postgres://user:pass@localhost/db"}))
	if err != nil {
		t.Fatal(err)
	}
	if c.HTTPAddr != ":8080" || c.ReadTimeout != 10*time.Second {
		t.Fatalf("unexpected defaults: %+v", c)
	}
}

func TestLoadRejectsInvalidDatabaseAndTimeout(t *testing.T) {
	for _, values := range []map[string]string{
		{"DATABASE_URL": "http://localhost/db"},
		{"DATABASE_URL": "postgres://localhost/db", "HTTP_READ_TIMEOUT": "soon"},
		{"DATABASE_URL": "postgres://localhost/db", "HTTP_IDLE_TIMEOUT": "0s"},
	} {
		if _, err := LoadFrom(env(values)); err == nil {
			t.Fatalf("expected invalid config error for %#v", values)
		}
	}
}
