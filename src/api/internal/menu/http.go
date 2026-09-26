package menu

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/epic-rinn/tableflow/src/api/internal/identity"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/httpx"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/idempotency"
)

// HTTP exposes menu routes.
type HTTP struct {
	svc    *Service
	staff  *identity.HTTP
	logger *slog.Logger
}

// NewHTTP builds handlers.
func NewHTTP(svc *Service, staff *identity.HTTP, logger *slog.Logger) *HTTP {
	return &HTTP{svc: svc, staff: staff, logger: logger}
}

// Routes returns path patterns and method handlers.
func (h *HTTP) Routes() map[string]map[string]http.HandlerFunc {
	stw := func(f http.HandlerFunc) http.HandlerFunc { return h.staff.Origin(h.staff.RequireStaff(f)) }
	return map[string]map[string]http.HandlerFunc{
		"/api/v1/branches/{branch_id}/menu": {
			http.MethodGet: h.get,
			http.MethodPut: stw(h.replace),
		},
		"/api/v1/menu-items/{item_id}/availability": {http.MethodPatch: stw(h.availability)},
	}
}

func validID(id string) bool {
	_, ok := idempotency.ParseKey(id) // canonical UUID
	return ok
}

func (h *HTTP) fail(w http.ResponseWriter, r *http.Request, err error) {
	var ve *ValidationError
	switch {
	case errors.As(err, &ve):
		httpx.ValidationError(w, r, ve.Fields)
	case errors.Is(err, identity.ErrUnauthenticated):
		h.staff.Fail(w, r, err)
	case errors.Is(err, ErrNotFound):
		httpx.NotFound(w, r)
	case errors.Is(err, ErrForbidden):
		httpx.WriteError(w, r, http.StatusForbidden, "FORBIDDEN", "Your role does not allow this action")
	case errors.Is(err, ErrVersionConflict):
		httpx.WriteError(w, r, http.StatusConflict, "VERSION_CONFLICT", "The menu changed meanwhile; reload and try again")
	case errors.Is(err, context.Canceled):
	default:
		h.logger.Error("menu request failed", "request_id", httpx.RequestID(r.Context()), "error", err.Error())
		httpx.WriteError(w, r, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE", "Service temporarily unavailable")
	}
}

// get serves the public menu. It carries no private data but stays
// no-store in MVP (HTTP contract) so sold-out changes are seen at once.
func (h *HTTP) get(w http.ResponseWriter, r *http.Request) {
	branch := r.PathValue("branch_id")
	if !validID(branch) {
		httpx.NotFound(w, r)
		return
	}
	var category *string
	if c := r.URL.Query().Get("category_id"); c != "" {
		if !validID(c) {
			httpx.ValidationError(w, r, map[string]string{"category_id": "must be a category ID"})
			return
		}
		category = &c
	}
	m, err := h.svc.Get(r.Context(), branch, category)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	// The menu is large and holds no secrets, so it is safe to compress
	// (BREACH needs a secret next to attacker input; other routes stay raw).
	if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		body, err := json.Marshal(m)
		if err != nil {
			h.fail(w, r, err)
			return
		}
		h := w.Header()
		h.Set("Content-Type", "application/json; charset=utf-8")
		h.Set("Cache-Control", "no-store")
		h.Set("Content-Encoding", "gzip")
		h.Add("Vary", "Accept-Encoding")
		w.WriteHeader(http.StatusOK)
		gz := gzip.NewWriter(w)
		_, _ = gz.Write(append(body, '\n'))
		_ = gz.Close()
		return
	}
	httpx.WriteJSON(w, http.StatusOK, m)
}

func (h *HTTP) replace(w http.ResponseWriter, r *http.Request) {
	branch := r.PathValue("branch_id")
	if !validID(branch) {
		httpx.NotFound(w, r)
		return
	}
	var in struct {
		ExpectedRevision *int         `json:"expected_revision"`
		Categories       []CategoryIn `json:"categories"`
	}
	if !httpx.DecodeJSONLimit(w, r, &in, MaxBody) {
		return
	}
	if in.ExpectedRevision == nil {
		httpx.ValidationError(w, r, map[string]string{"expected_revision": "is required"})
		return
	}
	for _, c := range in.Categories { // reject malformed IDs before SQL
		ids := []string{c.ID}
		for _, it := range c.Items {
			ids = append(ids, it.ID)
			for _, g := range it.OptionGroups {
				ids = append(ids, g.ID)
				for _, o := range g.Options {
					ids = append(ids, o.ID)
				}
			}
		}
		for _, id := range ids {
			if id != "" && !validID(id) {
				httpx.ValidationError(w, r, map[string]string{"id": "ids must be UUIDs or empty for new entries"})
				return
			}
		}
	}
	p, _ := identity.PrincipalFrom(r.Context())
	m, err := h.svc.Replace(r.Context(), p, branch, *in.ExpectedRevision, in.Categories, httpx.RequestID(r.Context()))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.Private(w)
	httpx.WriteJSON(w, http.StatusOK, m)
}

func (h *HTTP) availability(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("item_id")
	if !validID(id) {
		httpx.NotFound(w, r)
		return
	}
	var in struct {
		ExpectedVersion *int  `json:"expected_version"`
		SoldOut         *bool `json:"sold_out"`
	}
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	if in.ExpectedVersion == nil || in.SoldOut == nil {
		httpx.ValidationError(w, r, map[string]string{"expected_version": "expected_version and sold_out are required"})
		return
	}
	p, _ := identity.PrincipalFrom(r.Context())
	it, err := h.svc.SetAvailability(r.Context(), p, id, *in.ExpectedVersion, *in.SoldOut)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, it)
}
