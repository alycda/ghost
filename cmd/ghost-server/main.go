// ghost-server serves the Ghost API for one Postgres cluster. See
// internal/server for what that means and internal/server/config.go for the
// GHOST_SERVER_* environment it reads.
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/timescale/ghost/internal/log"
	"github.com/timescale/ghost/internal/server"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "ghost-server:", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	logger := log.New(os.Stderr)

	cfg, err := server.ConfigFromEnv()
	if err != nil {
		return err
	}
	srv, err := server.New(ctx, cfg, logger)
	if err != nil {
		return err
	}
	defer srv.Close()

	httpServer := &http.Server{
		Addr:              cfg.Listen,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()

	logger.Info("listening", "addr", cfg.Listen, "space", cfg.SpaceID,
		"public", fmt.Sprintf("%s:%d", cfg.PublicHost, cfg.PublicPort))
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
