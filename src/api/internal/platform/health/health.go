// Package health implements liveness and readiness endpoints.
package health

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/epic-rinn/tableflow/src/api/internal/platform/httpx"
)

// Pinger is satisfied by *pgxpool.Pool.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Handler serves health routes.
type Handler struct {
	db      Pinger
	timeout time.Duration
	logger  *slog.Logger
	group   singleflight.Group
}

// New returns health handlers that bound each database probe by timeout.
func New(db Pinger, timeout time.Duration, logger *slog.Logger) *Handler {
	return &Handler{db: db, timeout: timeout, logger: logger}
}

type liveBody struct {
	Status string `json:"status"`
}

type readyBody struct {
	Status string            `json:"status"`
	Checks map[string]string `json:"checks"`
}

// Live reports process liveness without touching dependencies.
func (h *Handler) Live(w http.ResponseWriter, _ *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, liveBody{Status: "ok"})
}

// Ready reports whether PostgreSQL answers within the timeout. Concurrent
// probes share one in-flight ping, so readiness uses at most one connection.
func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	ch := h.group.DoChan("database", func() (any, error) {
		// Detached from any single caller so one disconnect cannot fail others.
		ctx, cancel := context.WithTimeout(context.Background(), h.timeout)
		defer cancel()
		return nil, h.db.Ping(ctx)
	})
	var err error
	select {
	case res := <-ch:
		err = res.Err
	case <-r.Context().Done():
		return
	}
	if err != nil {
		// Driver errors can contain host names; log them, never return them.
		h.logger.Warn("readiness check failed",
			"request_id", httpx.RequestID(r.Context()), "check", "database", "error", err.Error())
		httpx.WriteError(w, r, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE", "Service is not ready")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, readyBody{Status: "ready", Checks: map[string]string{"database": "ok"}})
}
