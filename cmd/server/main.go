// Command server runs the http1.1 demo application.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/uig27055/http1.1/internal/app"
	"github.com/uig27055/http1.1/internal/config"
	"github.com/uig27055/http1.1/internal/notes"
	"github.com/uig27055/http1.1/internal/server"
)

func main() {
	// SIGTERM is what `docker stop` sends; SIGINT is Ctrl+C.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := run(ctx, os.LookupEnv, os.Stdout)
	stop()

	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// run is the testable entry point: it serves until ctx is cancelled and
// then shuts down gracefully.
func run(ctx context.Context, lookupEnv func(string) (string, bool), logOut io.Writer) error {
	cfg, err := config.Load(lookupEnv)
	if err != nil {
		return err
	}
	logger := slog.New(slog.NewJSONHandler(logOut, &slog.HandlerOptions{Level: cfg.SlogLevel()}))

	srv := server.New(app.New(notes.NewMemoryStore(), logger).Handler())
	srv.Logger = logger
	srv.ReadTimeout = cfg.ReadTimeout
	srv.WriteTimeout = cfg.WriteTimeout
	srv.IdleTimeout = cfg.IdleTimeout
	srv.Limits.MaxBodyBytes = cfg.MaxBodyBytes

	ln, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return err
	}
	logger.Info("server listening", "addr", ln.Addr().String())

	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()

	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
	}

	logger.Info("shutting down", "timeout", cfg.ShutdownTimeout)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	if err := <-serveErr; !errors.Is(err, server.ErrServerClosed) {
		return err
	}
	logger.Info("server stopped")
	return nil
}
