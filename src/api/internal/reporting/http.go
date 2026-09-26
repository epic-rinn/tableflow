package reporting

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/epic-rinn/tableflow/src/api/internal/identity"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/httpx"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/idempotency"
)

// HTTP exposes manager reports and the audit viewer.
type HTTP struct {
	svc    *Service
	staff  *identity.HTTP
	logger *slog.Logger
}

// NewHTTP wires handlers.
func NewHTTP(svc *Service, staff *identity.HTTP, logger *slog.Logger) *HTTP {
	return &HTTP{svc: svc, staff: staff, logger: logger}
}

// Routes returns path patterns and method handlers.
func (h *HTTP) Routes() map[string]map[string]http.HandlerFunc {
	st := func(f http.HandlerFunc) http.HandlerFunc { return h.staff.RequireStaff(f) }
	return map[string]map[string]http.HandlerFunc{
		"/api/v1/branches/{branch_id}/reports/daily": {http.MethodGet: st(h.daily)},
		"/api/v1/branches/{branch_id}/audit-events":  {http.MethodGet: st(h.audit)},
	}
}

func (h *HTTP) fail(w http.ResponseWriter, r *http.Request, err error) {
	var ve *ValidationError
	switch {
	case errors.As(err, &ve):
		httpx.ValidationError(w, r, ve.Fields)
	case errors.Is(err, ErrNotFound):
		httpx.NotFound(w, r)
	case errors.Is(err, ErrForbidden):
		httpx.WriteError(w, r, http.StatusForbidden, "FORBIDDEN", "Your role does not allow this action")
	case errors.Is(err, context.Canceled):
	default:
		h.logger.Error("reporting request failed", "request_id", httpx.RequestID(r.Context()), "error", err.Error())
		httpx.WriteError(w, r, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE", "Service temporarily unavailable")
	}
}

func branch(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.PathValue("branch_id")
	if _, ok := idempotency.ParseKey(id); !ok {
		httpx.NotFound(w, r)
		return "", false
	}
	return id, true
}

func (h *HTTP) daily(w http.ResponseWriter, r *http.Request) {
	b, ok := branch(w, r)
	if !ok {
		return
	}
	p, _ := identity.PrincipalFrom(r.Context())
	rep, err := h.svc.Daily(r.Context(), p, b, r.URL.Query().Get("from"), r.URL.Query().Get("to"))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.Private(w)
	httpx.WriteJSON(w, http.StatusOK, rep)
}

func (h *HTTP) audit(w http.ResponseWriter, r *http.Request) {
	b, ok := branch(w, r)
	if !ok {
		return
	}
	qs := r.URL.Query()
	limit := 0
	if raw := qs.Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > pageMax {
			httpx.ValidationError(w, r, map[string]string{"limit": "must be 1–100"})
			return
		}
		limit = n
	}
	p, _ := identity.PrincipalFrom(r.Context())
	page, err := h.svc.Audit(r.Context(), p, b, qs.Get("from"), qs.Get("to"), qs.Get("action"), qs.Get("cursor"), limit)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.Private(w)
	httpx.WriteJSON(w, http.StatusOK, page)
}
