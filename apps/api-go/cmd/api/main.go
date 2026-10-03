package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spyder73/TheActuallySocialSocialMedia/apps/api-go/internal/auth"
	"github.com/spyder73/TheActuallySocialSocialMedia/apps/api-go/internal/config"
	"github.com/spyder73/TheActuallySocialSocialMedia/apps/api-go/internal/httpapi"
	"github.com/spyder73/TheActuallySocialSocialMedia/apps/api-go/internal/social"
)

func main() {
	if len(os.Args) == 2 && os.Args[1] == "healthcheck" {
		if err := healthcheck(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) > 1 {
		fmt.Fprintln(os.Stderr, "usage: api [healthcheck]")
		os.Exit(2)
	}
	if err := run(); err != nil {
		slog.Error("API stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if err := config.ValidateOrigin(cfg.AppOrigin, cfg.LocalHTTP); err != nil {
		return err
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return errors.New("invalid database connection configuration")
	}
	defer pool.Close()

	accounts := auth.New(pool, auth.Options{AllowedOrigins: []string{cfg.AppOrigin}, LocalHTTP: cfg.LocalHTTP})
	router := httpapi.NewRouter(pool, logger, func(r chi.Router) {
		r.Route("/api", func(api chi.Router) {
			api.Use(auth.OriginMiddleware(cfg.AppOrigin))
			api.Use(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Cache-Control", "no-store")
					next.ServeHTTP(w, r)
				})
			})
			api.Mount("/auth", http.StripPrefix("/api", accounts.Handler()))
			api.Group(func(protected chi.Router) {
				protected.Use(accounts.Middleware)
				social.New(pool).Register(protected, auth.UserID)
			})
		})
	})
	server := &http.Server{
		Addr: cfg.HTTPAddr, Handler: router,
		ReadHeaderTimeout: cfg.ReadTimeout, ReadTimeout: cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout, IdleTimeout: cfg.IdleTimeout,
	}
	serveErr := make(chan error, 1)
	go func() {
		logger.Info("API listening", "addr", cfg.HTTPAddr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- err
		}
		close(serveErr)
	}()
	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			_ = server.Close()
			return fmt.Errorf("graceful HTTP shutdown: %w", err)
		}
		logger.Info("API stopped")
		return nil
	}
}

func healthcheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:8080/api/healthz", nil)
	if err != nil {
		return err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("healthcheck returned HTTP %d", res.StatusCode)
	}
	return nil
}
