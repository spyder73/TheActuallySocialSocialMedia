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

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spyder73/TheActuallySocialSocialMedia/apps/api-go/internal/config"
	"github.com/spyder73/TheActuallySocialSocialMedia/apps/api-go/internal/httpapi"
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
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return errors.New("invalid database connection configuration")
	}
	defer pool.Close()

	server := &http.Server{
		Addr: cfg.HTTPAddr, Handler: httpapi.NewRouter(pool, logger),
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
