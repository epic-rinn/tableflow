package ordering

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/epic-rinn/tableflow/src/api/internal/access"
	"github.com/epic-rinn/tableflow/src/api/internal/identity"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/httpx"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/idempotency"
)

// replayTTL: orders and kitchen commands end within a business day.
const replayTTL = 72 * time.Hour

// HTTP exposes ordering, kitchen and assistance routes.
type HTTP struct {
	svc       *Service
	pool      *pgxpool.Pool
	store     *idempotency.Store
	staff     *identity.HTTP
	guests    *access.HTTP
	pwaOrigin func(http.HandlerFunc) http.HandlerFunc
	logger    *slog.Logger
}

// NewHTTP wires handlers.
func NewHTTP(svc *Service, pool *pgxpool.Pool, store *idempotency.Store, staff *identity.HTTP, guests *access.HTTP, pwaOrigins []string, logger *slog.Logger) *HTTP {
	return &HTTP{svc: svc, pool: pool, store: store, staff: staff, guests: guests, pwaOrigin: httpx.OriginGuard(pwaOrigins), logger: logger}
}

// Routes returns path patterns and method handlers.
func (h *HTTP) Routes() map[string]map[string]http.HandlerFunc {
	st := func(f http.HandlerFunc) http.HandlerFunc { return h.staff.RequireStaff(f) }
	stw := func(f http.HandlerFunc) http.HandlerFunc { return h.staff.Origin(h.staff.RequireStaff(f)) }
	guestRead := func(f http.HandlerFunc) http.HandlerFunc { return h.guests.RequireGuest(access.KindVisit, f) }
	guestWrite := func(f http.HandlerFunc) http.HandlerFunc {
		return h.pwaOrigin(h.guests.RequireGuest(access.KindVisit, f))
	}
	return map[string]map[string]http.HandlerFunc{
		"/api/v1/visits/{visit_id}/orders": {
			http.MethodGet:  h.either(st(h.orders), guestRead(h.orders)),
			http.MethodPost: h.either(stw(h.submit), guestWrite(h.submit)),
		},
		"/api/v1/branches/{branch_id}/kitchen-lines": {http.MethodGet: st(h.kitchen)},
		"/api/v1/order-lines/{line_id}/transition":   {http.MethodPost: stw(h.transition)},
		"/api/v1/branches/{branch_id}/assistance":    {http.MethodGet: st(h.assistanceBoard)},
		"/api/v1/assistance/{request_id}/transition": {http.MethodPost: stw(h.assistanceTransition)},
		"/api/v1/visits/{visit_id}/assistance": {
			http.MethodGet:  h.either(st(h.visitAssistance), guestRead(h.visitAssistance)),
			http.MethodPost: h.either(stw(h.raise), guestWrite(h.raise)),
		},
	}
}

// either serves staff when a staff cookie is present, else the guest route.
func (h *HTTP) either(staff, guest http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := r.Cookie(identity.StaffCookie); err == nil {
			staff(w, r)
			return
		}
		guest(w, r)
	}
}

func actorFrom(r *http.Request) Actor {
	var a Actor
	if p, ok := identity.PrincipalFrom(r.Context()); ok {
		a.Staff = &p
	}
	if g, ok := access.GuestFrom(r.Context()); ok {
		a.Guest = &g
	}
	return a
}

func (a Actor) scope() string {
	if a.Staff != nil {
		return "staff:" + a.Staff.StaffID
	}
	if a.Guest != nil {
		return "guest:" + a.Guest.SessionID
	}
	return ""
}

func (h *HTTP) mutate(w http.ResponseWriter, r *http.Request, op string, body any, fn func(ctx context.Context, tx pgx.Tx) (int, any, error)) {
	idempotency.Mutate(w, r, h.pool, h.store, actorFrom(r).scope(), op, replayTTL, body, fn, h.fail)
}

func (h *HTTP) fail(w http.ResponseWriter, r *http.Request, err error) {
	var ve *ValidationError
	var mc *MenuChangedError
	switch {
	case errors.As(err, &ve):
		httpx.ValidationError(w, r, ve.Fields)
	case errors.As(err, &mc):
		fields := map[string]string{}
		for _, id := range mc.ItemIDs {
			fields[id] = "changed, sold out or removed"
		}
		httpx.WriteJSON(w, http.StatusConflict, httpx.ErrorBody{Error: httpx.ErrorDetail{Code: "MENU_CHANGED",
			Message:   "Some items changed since you loaded the menu. Refresh and review your order; nothing was ordered.",
			RequestID: httpx.RequestID(r.Context()), Fields: fields}})
	case errors.Is(err, identity.ErrUnauthenticated):
		h.staff.Fail(w, r, err)
	case errors.Is(err, access.ErrUnauthenticated):
		httpx.WriteError(w, r, http.StatusUnauthorized, "UNAUTHENTICATED", "Scan the table QR code again to continue")
	case errors.Is(err, idempotency.ErrConflict):
		httpx.WriteError(w, r, http.StatusConflict, "IDEMPOTENCY_CONFLICT", "This request key was already used for a different request")
	case errors.Is(err, ErrNotFound):
		httpx.NotFound(w, r)
	case errors.Is(err, ErrForbidden):
		httpx.WriteError(w, r, http.StatusForbidden, "FORBIDDEN", "Your role does not allow this action")
	case errors.Is(err, ErrVersionConflict):
		httpx.WriteError(w, r, http.StatusConflict, "VERSION_CONFLICT", "This changed meanwhile; refresh and try again")
	case errors.Is(err, ErrVisitState):
		httpx.WriteError(w, r, http.StatusConflict, "VISIT_STATE_CONFLICT", "This visit no longer accepts that change")
	case errors.Is(err, ErrLineState):
		httpx.WriteError(w, r, http.StatusConflict, "STATE_CONFLICT", "That step is not allowed from the current state")
	case errors.Is(err, context.Canceled):
	default:
		h.logger.Error("ordering request failed", "request_id", httpx.RequestID(r.Context()), "error", err.Error())
		httpx.WriteError(w, r, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE", "Service temporarily unavailable")
	}
}

func pathID(w http.ResponseWriter, r *http.Request, name string) (string, bool) {
	id := r.PathValue(name)
	if _, ok := idempotency.ParseKey(id); !ok {
		httpx.NotFound(w, r)
		return "", false
	}
	return id, true
}

func limitParam(w http.ResponseWriter, r *http.Request) (int, bool) {
	raw := r.URL.Query().Get("limit")
	if raw == "" {
		return 0, true
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > pageMax {
		httpx.ValidationError(w, r, map[string]string{"limit": "must be 1–100"})
		return 0, false
	}
	return n, true
}

func (h *HTTP) submit(w http.ResponseWriter, r *http.Request) {
	visit, ok := pathID(w, r, "visit_id")
	if !ok {
		return
	}
	var in OrderIn
	if !httpx.DecodeJSONLimit(w, r, &in, MaxBody) {
		return
	}
	for _, l := range in.Lines {
		if _, ok := idempotency.ParseKey(l.ItemID); !ok {
			httpx.ValidationError(w, r, map[string]string{"lines": "item_id and option_ids must be IDs"})
			return
		}
		for _, o := range l.OptionIDs {
			if _, ok := idempotency.ParseKey(o); !ok {
				httpx.ValidationError(w, r, map[string]string{"lines": "item_id and option_ids must be IDs"})
				return
			}
		}
	}
	actor := actorFrom(r)
	h.mutate(w, r, "orders.submit", in, func(ctx context.Context, tx pgx.Tx) (int, any, error) {
		o, err := h.svc.Submit(ctx, tx, actor, visit, in)
		return http.StatusCreated, o, err
	})
}

func (h *HTTP) orders(w http.ResponseWriter, r *http.Request) {
	visit, ok := pathID(w, r, "visit_id")
	if !ok {
		return
	}
	limit, ok := limitParam(w, r)
	if !ok {
		return
	}
	page, err := h.svc.Orders(r.Context(), actorFrom(r), visit, r.URL.Query().Get("cursor"), limit)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.Private(w)
	httpx.WriteJSON(w, http.StatusOK, page)
}

func (h *HTTP) kitchen(w http.ResponseWriter, r *http.Request) {
	branch, ok := pathID(w, r, "branch_id")
	if !ok {
		return
	}
	limit, ok := limitParam(w, r)
	if !ok {
		return
	}
	p, _ := identity.PrincipalFrom(r.Context())
	page, err := h.svc.Kitchen(r.Context(), p, branch, r.URL.Query().Get("state"), r.URL.Query().Get("cursor"), limit)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, page)
}

func (h *HTTP) transition(w http.ResponseWriter, r *http.Request) {
	line, ok := pathID(w, r, "line_id")
	if !ok {
		return
	}
	var in struct {
		ExpectedVersion *int    `json:"expected_version"`
		ToState         string  `json:"to_state"`
		Reason          *string `json:"reason"`
	}
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	if in.ExpectedVersion == nil {
		httpx.ValidationError(w, r, map[string]string{"expected_version": "is required"})
		return
	}
	p, _ := identity.PrincipalFrom(r.Context())
	h.mutate(w, r, "order_lines.transition", in, func(ctx context.Context, tx pgx.Tx) (int, any, error) {
		l, err := h.svc.Transition(ctx, tx, p, line, *in.ExpectedVersion, in.ToState, in.Reason, httpx.RequestID(ctx))
		return http.StatusOK, l, err
	})
}

func (h *HTTP) raise(w http.ResponseWriter, r *http.Request) {
	visit, ok := pathID(w, r, "visit_id")
	if !ok {
		return
	}
	var in struct {
		Topic string  `json:"topic"`
		Note  *string `json:"note"`
	}
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	actor := actorFrom(r)
	h.mutate(w, r, "assistance.raise", in, func(ctx context.Context, tx pgx.Tx) (int, any, error) {
		a, created, err := h.svc.RaiseAssistance(ctx, tx, actor, visit, in.Topic, in.Note)
		status := http.StatusOK
		if created {
			status = http.StatusCreated
		}
		return status, a, err
	})
}

type assistanceList struct {
	Items      []Assistance `json:"items"`
	ServerTime time.Time    `json:"server_time"`
}

func (h *HTTP) visitAssistance(w http.ResponseWriter, r *http.Request) {
	visit, ok := pathID(w, r, "visit_id")
	if !ok {
		return
	}
	items, err := h.svc.VisitAssistance(r.Context(), actorFrom(r), visit)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, assistanceList{Items: items, ServerTime: time.Now().UTC()})
}

func (h *HTTP) assistanceBoard(w http.ResponseWriter, r *http.Request) {
	branch, ok := pathID(w, r, "branch_id")
	if !ok {
		return
	}
	p, _ := identity.PrincipalFrom(r.Context())
	items, err := h.svc.AssistanceBoard(r.Context(), p, branch)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, assistanceList{Items: items, ServerTime: time.Now().UTC()})
}

func (h *HTTP) assistanceTransition(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "request_id")
	if !ok {
		return
	}
	var in struct {
		ExpectedVersion *int   `json:"expected_version"`
		ToState         string `json:"to_state"`
	}
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	if in.ExpectedVersion == nil {
		httpx.ValidationError(w, r, map[string]string{"expected_version": "is required"})
		return
	}
	p, _ := identity.PrincipalFrom(r.Context())
	h.mutate(w, r, "assistance.transition", in, func(ctx context.Context, tx pgx.Tx) (int, any, error) {
		a, err := h.svc.TransitionAssistance(ctx, tx, p, id, *in.ExpectedVersion, in.ToState)
		return http.StatusOK, a, err
	})
}
