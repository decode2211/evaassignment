// Entry point: wires config, DB, router, and handles graceful shutdown.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/decode2211/evaassignment/internal/auth"
	"github.com/decode2211/evaassignment/internal/config"
	"github.com/decode2211/evaassignment/internal/handlers"
	"github.com/decode2211/evaassignment/internal/store"
	"github.com/decode2211/evaassignment/web"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg := config.Load()

	db, err := store.New(cfg.DBPath)
	if err != nil {
		slog.Error("failed to open database", "error", err, "path", cfg.DBPath)
		os.Exit(1)
	}
	defer db.Close()

	h := handlers.New(db, cfg.JWTSecret)
	mw := auth.NewMiddleware(cfg.JWTSecret, db)
	router := handlers.NewRouter(h, mw, web.FS)

	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router,
		// Bound connection lifetimes so a slow client can't hold one open forever.
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		slog.Info("server starting", "port", cfg.Port, "db_path", cfg.DBPath)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		slog.Error("server failed to start", "error", err)
		os.Exit(1)
	case sig := <-stop:
		slog.Info("shutdown signal received", "signal", sig.String())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		slog.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}
	slog.Info("server stopped cleanly")
}
