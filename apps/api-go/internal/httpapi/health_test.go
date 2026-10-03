package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakePinger struct{ err error }

func (f fakePinger) Ping(context.Context) error { return f.err }

func TestLivenessDoesNotDependOnDatabase(t *testing.T) {
	r := NewRouter(nil, slog.New(slog.NewTextHandler(httptest.NewRecorder(), nil)))
	res := httptest.NewRecorder()
	r.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/healthz", nil))
	if res.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", res.Code)
	}
	if res.Header().Get("X-Request-ID") == "" {
		t.Fatal("missing request ID")
	}
}

func TestReadinessReflectsDatabasePing(t *testing.T) {
	tests := []struct {
		name string
		db   Pinger
		want int
	}{
		{name: "ready", db: fakePinger{}, want: http.StatusOK},
		{name: "failed ping", db: fakePinger{err: errors.New("offline")}, want: http.StatusServiceUnavailable},
		{name: "missing db", db: nil, want: http.StatusServiceUnavailable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewRouter(tt.db, slog.New(slog.NewTextHandler(httptest.NewRecorder(), nil)))
			res := httptest.NewRecorder()
			r.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/api/readyz", nil))
			if res.Code != tt.want {
				t.Fatalf("status=%d, want %d", res.Code, tt.want)
			}
		})
	}
}
