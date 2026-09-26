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

	"github.com/epic-rinn/tableflow/src/api/internal/access"
	"github.com/epic-rinn/tableflow/src/api/internal/identity"
	"github.com/epic-rinn/tableflow/src/api/internal/members"
	"github.com/epic-rinn/tableflow/src/api/internal/menu"
	"github.com/epic-rinn/tableflow/src/api/internal/ordering"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/config"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/database"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/health"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/httpx"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/idempotency"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/mail"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/password"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/server"
	"github.com/epic-rinn/tableflow/src/api/internal/seating"
)

// RouteProvider is a module exposing its HTTP routes.
type RouteProvider interface {
	Routes() map[string]map[string]http.HandlerFunc
}

// Routes maps each implemented path pattern to its method handlers. The
// OpenAPI contract test compares this set with the documented operations.
func Routes(health *health.Handler, providers ...RouteProvider) map[string]map[string]http.HandlerFunc {
	routes := map[string]map[string]http.HandlerFunc{
		"/api/v1/health/live":  {http.MethodGet: health.Live},
		"/api/v1/health/ready": {http.MethodGet: health.Ready},
	}
	for _, p := range providers {
		maps.Copy(routes, p.Routes())
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

// purgeLoop runs hourly maintenance deletes (expired sessions, throttle
// buckets, idempotency records); each purge is bounded.
func purgeLoop(ctx context.Context, logger *slog.Logger, purges map[string]func(context.Context) (int64, error)) {
	t := time.NewTicker(time.Hour)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			for name, purge := range purges {
				if n, err := purge(ctx); err != nil && ctx.Err() == nil {
					logger.Warn("purge failed", "purge", name, "error", err.Error())
				} else if n > 0 {
					logger.Info("purge", "purge", name, "deleted", n)
				}
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
	hasher := password.New(hashConcurrency) // shared bound across staff and members
	svc := identity.NewService(pool, hasher, cfg.StaffSessionIdle, cfg.StaffSessionAbsolute)
	idHTTP := identity.NewHTTP(svc, cfg.AdminOrigins, cfg.TrustedProxies, logger)
	accSvc := access.NewService(pool)
	accHTTP := access.NewHTTP(accSvc, cfg.PWAOrigins, cfg.TrustedProxies, logger)
	outbox := mail.NewQueue(mail.SMTP{Addr: cfg.SMTPAddr, From: cfg.MailFrom, Timeout: 10 * time.Second}, 100, 10*time.Second, logger)
	go outbox.Run(ctx, 2)
	memSvc := members.NewService(pool, hasher, outbox, cfg.PWAPublicURL)
	memHTTP := members.NewHTTP(memSvc, cfg.PWAOrigins, cfg.TrustedProxies, logger)
	store, err := idempotency.NewStore(cfg.DataKey)
	if err != nil {
		return err
	}
	seatHTTP := seating.NewHTTP(seating.NewService(pool, svc), store, idHTTP, accHTTP, cfg.PWAOrigins, cfg.TrustedProxies, logger)
	menuHTTP := menu.NewHTTP(menu.NewService(pool, svc), idHTTP, logger)
	orderHTTP := ordering.NewHTTP(ordering.NewService(pool, svc), pool, store, idHTTP, accHTTP, cfg.PWAOrigins, logger)
	h := NewHandler(logger, Routes(health.New(pool, cfg.ReadinessTimeout, logger), idHTTP, accHTTP, memHTTP, seatHTTP, menuHTTP, orderHTTP))
	go purgeLoop(ctx, logger, map[string]func(context.Context) (int64, error){
		"identity":    svc.Purge,
		"access":      accSvc.Purge,
		"members":     memSvc.Purge,
		"idempotency": func(ctx context.Context) (int64, error) { return idempotency.Purge(ctx, pool) },
	})
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
