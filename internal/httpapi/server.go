// Package httpapi wires the HTTP server, scheduler, and graceful shutdown.
package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cobanov/terminal-army-go/internal/auth"
	"github.com/cobanov/terminal-army-go/internal/config"
	"github.com/cobanov/terminal-army-go/internal/scheduler"
	"github.com/cobanov/terminal-army-go/internal/store"
	"github.com/cobanov/terminal-army-go/internal/svc"
	"github.com/cobanov/terminal-army-go/internal/ws"
)

func Run(ctx context.Context) error {
	svc.SetStartTime(time.Now())

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	logger := newLogger(cfg)
	slog.SetDefault(logger)

	pool, err := store.OpenPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	queries := store.New(pool)
	tokens := auth.NewSigner(cfg.JWTSecret, cfg.JWTTTL)
	app := svc.NewApp(cfg, pool, queries, tokens)
	hub := ws.NewHub()
	app.Events = hub

	handler := newRouter(app, hub)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		// ReadTimeout bounds slow-body (slowloris) requests. WriteTimeout is
		// intentionally left unset: the /ws endpoint hijacks the connection for
		// long-lived streaming and a global write deadline would sever it.
		ReadTimeout: 15 * time.Second,
		IdleTimeout: 60 * time.Second,
	}

	sched := scheduler.New(app, cfg.SchedulerTick)
	schedCtx, schedCancel := context.WithCancel(ctx)
	defer schedCancel()
	go sched.Run(schedCtx)

	go hub.Run(schedCtx)

	sigCtx, stopSig := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stopSig()

	errCh := make(chan error, 1)
	go func() {
		logger.Info("http server listening", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case <-sigCtx.Done():
		logger.Info("shutdown signal received")
	case err := <-errCh:
		return err
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

func newLogger(cfg *config.Config) *slog.Logger {
	var lvl slog.Level
	switch cfg.LogLevel {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	opts := &slog.HandlerOptions{Level: lvl}
	if cfg.LogFormat == "json" {
		return slog.New(slog.NewJSONHandler(os.Stdout, opts))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, opts))
}
