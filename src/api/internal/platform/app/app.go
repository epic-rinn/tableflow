// Package app wires configuration, the database pool, and HTTP routes.
package app

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"net"
	"net/http"
	"time"

	"github.com/epic-rinn/tableflow/src/api/internal/identity"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/config"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/database"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/health"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/httpx"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/server"
)

// Routes maps each implemented path pattern to its method handlers. The
// OpenAPI contract test compares this set with the documented operations.
func Routes(health *health.Handler, id *identity.HTTP) map[string]map[string]http.HandlerFunc {
	routes := map[string]map[string]http.HandlerFunc{
		"/api/v1/health/live":  {http.MethodGet: health.Live},
		"/api/v1/health/ready": {http.MethodGet: health.Ready},
	}
	if id != nil {
		maps.Copy(routes, id.Routes())
	}
	return routes
}

// NewHandler returns the API's root handler. Only implemented routes are
// registered; everything else is a JSON 404.
func NewHandler(logger *slog.Logger, routes map[string]map[string]http.HandlerFunc) http.Handler {
	mux := http.NewServeMux()
	for pattern, methods := range routes {
		mux.HandleFunc(pattern, httpx.Methods(methods))
	}
	mux.HandleFunc("/", httpx.NotFound)
	return httpx.Middleware(logger, mux)
}

// hashConcurrency bounds simultaneous argon2id operations (~19 MiB each).
const hashConcurrency = 4

func purgeIdentity(ctx context.Context, svc *identity.Service, logger *slog.Logger) {
	t := time.NewTicker(time.Hour)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if n, err := svc.Purge(ctx); err != nil && ctx.Err() == nil {
				logger.Warn("identity purge failed", "error", err.Error())
			} else if n > 0 {
				logger.Info("identity purge", "deleted", n)
			}
		}
	}
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
	svc := identity.NewService(pool, identity.NewHasher(hashConcurrency), cfg.StaffSessionIdle, cfg.StaffSessionAbsolute)
	idHTTP := identity.NewHTTP(svc, cfg.AdminOrigins, cfg.TrustedProxies, logger)
	h := NewHandler(logger, Routes(health.New(pool, cfg.ReadinessTimeout, logger), idHTTP))
	go purgeIdentity(ctx, svc, logger)
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
