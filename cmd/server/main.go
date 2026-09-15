// This is the entry point of the whole application - the file that runs
// when you execute the compiled program. Its job is purely "wiring": read
// configuration, open the database, build the HTTP router, and start
// listening for requests, then shut down cleanly when asked to stop. All
// the actual business logic lives in the internal/ packages, not here.
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
		// These timeouts stop a slow or malicious client from tying up a
		// connection (and a goroutine) forever.
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Run the server in the background so the main goroutine is free to
	// wait for a shutdown signal instead.
	serverErr := make(chan error, 1)
	go func() {
		slog.Info("server starting", "port", cfg.Port, "db_path", cfg.DBPath)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	// Listen for Ctrl+C (SIGINT) or a termination request from the OS or
	// container runtime (SIGTERM), so we can shut down gracefully instead
	// of dropping in-flight requests when the process is stopped.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		slog.Error("server failed to start", "error", err)
		os.Exit(1)
	case sig := <-stop:
		slog.Info("shutdown signal received", "signal", sig.String())
	}

	// Give in-flight requests up to 10 seconds to finish before forcing
	// the shutdown, rather than cutting every open connection immediately.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		slog.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}
	slog.Info("server stopped cleanly")
}
