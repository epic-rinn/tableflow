// Package app wires configuration, the database pool, and HTTP routes.
package app

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/epic-rinn/tableflow/src/api/internal/platform/config"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/database"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/health"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/httpx"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/server"
)

// NewHandler returns the API's root handler. Only implemented routes are
// registered; everything else is a JSON 404.
func NewHandler(logger *slog.Logger, health *health.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/health/live", httpx.Method(http.MethodGet, health.Live))
	mux.HandleFunc("/api/v1/health/ready", httpx.Method(http.MethodGet, health.Ready))
	mux.HandleFunc("/", httpx.NotFound)
	return httpx.Middleware(logger, mux)
}

// Run serves the API on ln until ctx is cancelled, then drains requests and
// closes the database pool.
func Run(ctx context.Context, cfg config.Config, logger *slog.Logger, ln net.Listener) error {
	pool, err := database.NewPool(ctx, cfg.DatabaseURL, database.Options{
		MaxConns:         cfg.DBMaxConns,
		StatementTimeout: cfg.StatementTimeout,
		LockTimeout:      cfg.LockTimeout,
		IdleInTxTimeout:  cfg.IdleInTxTimeout,
	})
	if err != nil {
		return err
	}
	h := NewHandler(logger, health.New(pool, cfg.ReadinessTimeout, logger))
	logger.Info("api listening", "addr", ln.Addr().String())
	err = server.Serve(ctx, ln, h, server.Timeouts{
		ReadHeader: cfg.ReadHeaderTimeout,
		Read:       cfg.ReadTimeout,
		Write:      cfg.WriteTimeout,
		Idle:       cfg.IdleTimeout,
		Shutdown:   cfg.ShutdownTimeout,
	})
	// Close only after in-flight requests have drained (or the drain timed out).
	closePool(pool, logger, poolCloseTimeout)
	if err != nil {
		return fmt.Errorf("api: %w", err)
	}
	logger.Info("api shutdown complete")
	return nil
}

// poolCloseTimeout bounds pool shutdown: pgxpool.Close waits for every
// connection to terminate, which can block for many seconds when PostgreSQL
// is unresponsive. The OS releases any remaining sockets at process exit.
const poolCloseTimeout = 3 * time.Second

func closePool(pool interface{ Close() }, logger *slog.Logger, timeout time.Duration) {
	done := make(chan struct{})
	go func() {
		pool.Close()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(timeout):
		logger.Warn("database pool close timed out", "timeout", timeout.String())
	}
}
