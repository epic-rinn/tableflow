package seating

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"net/netip"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/epic-rinn/tableflow/src/api/internal/access"
	"github.com/epic-rinn/tableflow/src/api/internal/identity"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/httpx"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/idempotency"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/throttle"
)

// replayTTL covers the data-model rule "24 h and until the ticket/visit
// ends, whichever is later": tickets and visits end within a business day.
const replayTTL = 72 * time.Hour

// HTTP exposes seating routes.
type HTTP struct {
	svc       *Service
	store     *idempotency.Store
	staff     *identity.HTTP
	guests    *access.HTTP
	pwaOrigin func(http.HandlerFunc) http.HandlerFunc
	trusted   []netip.Prefix
	logger    *slog.Logger
}

// NewHTTP wires handlers; staff/guest middleware come from their modules.
func NewHTTP(svc *Service, store *idempotency.Store, staff *identity.HTTP, guests *access.HTTP, pwaOrigins []string,
	trusted []netip.Prefix, logger *slog.Logger) *HTTP {
	return &HTTP{svc: svc, store: store, staff: staff, guests: guests, pwaOrigin: httpx.OriginGuard(pwaOrigins), trusted: trusted, logger: logger}
}

// Routes returns path patterns and method handlers.
func (h *HTTP) Routes() map[string]map[string]http.HandlerFunc {
	st := func(f http.HandlerFunc) http.HandlerFunc { return h.staff.RequireStaff(f) }
	stw := func(f http.HandlerFunc) http.HandlerFunc { return h.staff.Origin(h.staff.RequireStaff(f)) }
	return map[string]map[string]http.HandlerFunc{
		"/api/v1/branches/{branch_id}/seating-groups": {http.MethodGet: st(h.getGroups), http.MethodPut: stw(h.putGroups)},
		"/api/v1/branches/{branch_id}/tables":         {http.MethodGet: st(h.getTables), http.MethodPost: stw(h.createTable)},
		"/api/v1/tables/{table_id}":                   {http.MethodPatch: stw(h.updateTable)},
		"/api/v1/tables/{table_id}/ready":             {http.MethodPost: stw(h.ready)},
		"/api/v1/branches/{branch_id}/queue-tickets": {
			http.MethodGet:  st(h.board),
			http.MethodPost: h.either(stw(h.join), h.pwaOrigin(h.guests.RequireAnonymous(h.join))),
		},
		"/api/v1/queue-tickets/{ticket_id}": {
			http.MethodGet: h.either(st(h.getTicket), h.guests.RequireGuest(access.KindQueue, h.getTicket)),
		},
		"/api/v1/queue-tickets/{ticket_id}/cancel": {
			http.MethodPost: h.either(stw(h.cancel), h.pwaOrigin(h.guests.RequireGuest(access.KindQueue, h.cancel))),
		},
		"/api/v1/queue-tickets/{ticket_id}/call":    {http.MethodPost: stw(h.call)},
		"/api/v1/queue-tickets/{ticket_id}/no-show": {http.MethodPost: stw(h.noShow)},
		"/api/v1/visits": {http.MethodPost: stw(h.seat)},
		"/api/v1/visits/{visit_id}": {
			http.MethodGet: h.either(st(h.getVisit), h.guests.RequireGuest(access.KindVisit, h.getVisit)),
		},
		"/api/v1/visits/{visit_id}/move":          {http.MethodPost: stw(h.move)},
		"/api/v1/visits/{visit_id}/depart":        {http.MethodPost: stw(h.depart)},
		"/api/v1/visits/{visit_id}/close-empty":   {http.MethodPost: stw(h.closeEmpty)},
		"/api/v1/visits/{visit_id}/rotate-access": {http.MethodPost: stw(h.rotate)},
	}
}

// either serves staff when a staff cookie is present (admin origin), else
// the guest/anonymous variant (PWA origin). Cookies are host-only per
// origin, so a browser only ever sends one kind.
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
	if an, ok := access.AnonymousFrom(r.Context()); ok {
		a.Anonymous = &an
	}
	return a
}

func (a Actor) scope() string {
	switch {
	case a.Staff != nil:
		return "staff:" + a.Staff.StaffID
	case a.Guest != nil:
		return "guest:" + a.Guest.SessionID
	case a.Anonymous != nil:
		return "anon:" + a.Anonymous.ID
	}
	return ""
}

// mutate runs fn and its idempotency record in one transaction, replaying
// the committed response for a repeated key. Failed commands store nothing.
func (h *HTTP) mutate(w http.ResponseWriter, r *http.Request, op string, body any,
	fn func(ctx context.Context, tx pgx.Tx) (int, any, error)) {
	httpx.Private(w)
	key, ok := idempotency.ParseKey(r.Header.Get(idempotency.Header))
	if !ok {
		httpx.WriteError(w, r, http.StatusBadRequest, "IDEMPOTENCY_KEY_REQUIRED", "Send a UUID Idempotency-Key header")
		return
	}
	canonical, err := json.Marshal(body)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	scope := actorFrom(r).scope()
	var res idempotency.Result
	err = pgx.BeginFunc(r.Context(), h.svc.pool, func(tx pgx.Tx) error {
		var err error
		res, err = h.store.Execute(r.Context(), tx, scope, op, key, idempotency.RequestHash([]byte(r.URL.Path), canonical), replayTTL,
			func() (int, []byte, error) {
				status, out, err := fn(r.Context(), tx)
				if err != nil {
					return 0, nil, err
				}
				b, err := json.Marshal(out)
				return status, b, err
			})
		return err
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if res.Replayed {
		w.Header().Set("Idempotency-Replayed", "true")
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(res.Status)
	_, _ = w.Write(append(res.Body, '\n'))
}

func (h *HTTP) fail(w http.ResponseWriter, r *http.Request, err error) {
	var ve *ValidationError
	var rl *throttle.RateLimitedError
	conflict := func(code, msg string) { httpx.WriteError(w, r, http.StatusConflict, code, msg) }
	switch {
	case errors.As(err, &ve):
		httpx.ValidationError(w, r, ve.Fields)
	case errors.As(err, &rl):
		httpx.RateLimited(w, r, int(math.Ceil(rl.RetryAfter.Seconds())))
	case errors.Is(err, identity.ErrUnauthenticated):
		h.staff.Fail(w, r, err)
	case errors.Is(err, access.ErrUnauthenticated):
		httpx.WriteError(w, r, http.StatusUnauthorized, "UNAUTHENTICATED", "Scan the QR code again to continue")
	case errors.Is(err, idempotency.ErrConflict):
		conflict("IDEMPOTENCY_CONFLICT", "This request key was already used for a different request")
	case errors.Is(err, ErrNotFound):
		httpx.NotFound(w, r)
	case errors.Is(err, ErrForbidden):
		httpx.WriteError(w, r, http.StatusForbidden, "FORBIDDEN", "Your role does not allow this action")
	case errors.Is(err, ErrVersionConflict):
		conflict("VERSION_CONFLICT", "This changed meanwhile; refresh and try again")
	case errors.Is(err, ErrTableUnavailable):
		conflict("TABLE_UNAVAILABLE", "The table is not available; refresh the board")
	case errors.Is(err, ErrTicketState):
		conflict("TICKET_STATE_CONFLICT", "The ticket can no longer do that; refresh the board")
	case errors.Is(err, ErrVisitState):
		conflict("VISIT_STATE_CONFLICT", "The visit can no longer do that; refresh")
	case errors.Is(err, ErrBypass):
		conflict("BYPASS_REQUIRES_OVERRIDE", "An older compatible party is waiting; a manager must give a reason to skip it")
	case errors.Is(err, ErrQueueActive):
		conflict("QUEUE_ACTIVE", "Seating groups can change only when nobody is waiting or called")
	case errors.Is(err, ErrLabelTaken):
		conflict("LABEL_TAKEN", "Another table already uses this label")
	case errors.Is(err, ErrTooManyTables):
		conflict("TABLE_LIMIT", "The branch already has the maximum number of tables")
	case errors.Is(err, ErrTableIncompatible):
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "TABLE_INCOMPATIBLE", "The table does not fit this party")
	case errors.Is(err, ErrPartyNeedsStaff):
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "PARTY_NEEDS_STAFF", "Please ask a staff member to add a party of this size")
	case errors.Is(err, context.Canceled):
	default:
		h.logger.Error("seating request failed", "request_id", httpx.RequestID(r.Context()), "error", err.Error())
		httpx.WriteError(w, r, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE", "Service temporarily unavailable")
	}
}

func pathID(w http.ResponseWriter, r *http.Request, name string) (string, bool) {
	id := r.PathValue(name)
	if _, ok := idempotency.ParseKey(id); !ok { // canonical UUID check
		httpx.NotFound(w, r)
		return "", false
	}
	return id, true
}

func principal(r *http.Request) identity.Principal {
	p, _ := identity.PrincipalFrom(r.Context())
	return p
}

// --- configuration ---

func (h *HTTP) getGroups(w http.ResponseWriter, r *http.Request) {
	branch, ok := pathID(w, r, "branch_id")
	if !ok {
		return
	}
	groups, err := h.svc.Groups(r.Context(), principal(r), branch)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"groups": groups})
}

func (h *HTTP) putGroups(w http.ResponseWriter, r *http.Request) {
	branch, ok := pathID(w, r, "branch_id")
	if !ok {
		return
	}
	var in struct {
		Groups []Group `json:"groups"`
	}
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	groups, err := h.svc.ReplaceGroups(r.Context(), principal(r), branch, in.Groups, httpx.RequestID(r.Context()))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"groups": groups})
}

func (h *HTTP) getTables(w http.ResponseWriter, r *http.Request) {
	branch, ok := pathID(w, r, "branch_id")
	if !ok {
		return
	}
	tables, err := h.svc.Tables(r.Context(), principal(r), branch)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": tables, "server_time": time.Now().UTC()})
}

type tableBody struct {
	Label    string   `json:"label"`
	Capacity int      `json:"capacity"`
	Needs    []string `json:"needs"`
}

func (h *HTTP) createTable(w http.ResponseWriter, r *http.Request) {
	branch, ok := pathID(w, r, "branch_id")
	if !ok {
		return
	}
	var in tableBody
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	t, err := h.svc.CreateTable(r.Context(), principal(r), branch, TableInput{Label: in.Label, Capacity: in.Capacity, Needs: in.Needs, Active: true},
		httpx.RequestID(r.Context()))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, t)
}

func (h *HTTP) updateTable(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "table_id")
	if !ok {
		return
	}
	var in struct {
		tableBody
		Active          *bool `json:"active"`
		ExpectedVersion *int  `json:"expected_version"`
	}
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	if in.ExpectedVersion == nil || in.Active == nil {
		httpx.ValidationError(w, r, map[string]string{"expected_version": "expected_version and active are required"})
		return
	}
	t, err := h.svc.UpdateTable(r.Context(), principal(r), id, *in.ExpectedVersion,
		TableInput{Label: in.Label, Capacity: in.Capacity, Needs: in.Needs, Active: *in.Active}, httpx.RequestID(r.Context()))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, t)
}

type versionBody struct {
	ExpectedVersion *int `json:"expected_version"`
}

func requireVersion(w http.ResponseWriter, r *http.Request, v *int) bool {
	if v == nil {
		httpx.ValidationError(w, r, map[string]string{"expected_version": "is required"})
		return false
	}
	return true
}

func (h *HTTP) ready(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "table_id")
	if !ok {
		return
	}
	var in versionBody
	if !httpx.DecodeJSON(w, r, &in) || !requireVersion(w, r, in.ExpectedVersion) {
		return
	}
	h.mutate(w, r, "tables.ready", in, func(ctx context.Context, tx pgx.Tx) (int, any, error) {
		t, err := h.svc.Ready(ctx, tx, principal(r), id, *in.ExpectedVersion)
		return http.StatusOK, t, err
	})
}

// --- queue ---

func (h *HTTP) join(w http.ResponseWriter, r *http.Request) {
	branch, ok := pathID(w, r, "branch_id")
	if !ok {
		return
	}
	var in JoinInput
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	actor := actorFrom(r)
	ip := throttle.IPKey("queue-join:ip", httpx.ClientIP(r, h.trusted))
	h.mutate(w, r, "queue.join", in, func(ctx context.Context, tx pgx.Tx) (int, any, error) {
		// Only first executions count toward the limit: retries of a
		// committed key replay without reaching this function.
		if actor.Anonymous != nil {
			if err := h.svc.CheckJoinLimits(ctx, *actor.Anonymous, ip); err != nil {
				return 0, nil, err
			}
		}
		res, err := h.svc.Join(ctx, tx, actor, branch, in)
		return http.StatusCreated, res, err
	})
}

func (h *HTTP) board(w http.ResponseWriter, r *http.Request) {
	branch, ok := pathID(w, r, "branch_id")
	if !ok {
		return
	}
	limit := 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > boardPageMax {
			httpx.ValidationError(w, r, map[string]string{"limit": "must be 1–100"})
			return
		}
		limit = n
	}
	page, err := h.svc.Board(r.Context(), principal(r), branch, r.URL.Query().Get("state"), r.URL.Query().Get("cursor"), limit)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, page)
}

func (h *HTTP) getTicket(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "ticket_id")
	if !ok {
		return
	}
	t, err := h.svc.GetTicket(r.Context(), actorFrom(r), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, t)
}

func (h *HTTP) cancel(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "ticket_id")
	if !ok {
		return
	}
	var in versionBody
	if !httpx.DecodeJSON(w, r, &in) || !requireVersion(w, r, in.ExpectedVersion) {
		return
	}
	actor := actorFrom(r)
	h.mutate(w, r, "queue.cancel", in, func(ctx context.Context, tx pgx.Tx) (int, any, error) {
		t, err := h.svc.Cancel(ctx, tx, actor, id, *in.ExpectedVersion)
		return http.StatusOK, t, err
	})
}

func (h *HTTP) call(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "ticket_id")
	if !ok {
		return
	}
	var in struct {
		TableID         string  `json:"table_id"`
		ExpectedVersion *int    `json:"expected_version"`
		OverrideReason  *string `json:"override_reason"`
	}
	if !httpx.DecodeJSON(w, r, &in) || !requireVersion(w, r, in.ExpectedVersion) {
		return
	}
	if _, ok := idempotency.ParseKey(in.TableID); !ok {
		httpx.ValidationError(w, r, map[string]string{"table_id": "must be a table ID"})
		return
	}
	h.mutate(w, r, "queue.call", in, func(ctx context.Context, tx pgx.Tx) (int, any, error) {
		t, err := h.svc.Call(ctx, tx, principal(r), id, in.TableID, *in.ExpectedVersion, in.OverrideReason, httpx.RequestID(ctx))
		return http.StatusOK, t, err
	})
}

func (h *HTTP) noShow(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "ticket_id")
	if !ok {
		return
	}
	var in struct {
		ExpectedVersion *int   `json:"expected_version"`
		Reason          string `json:"reason"`
	}
	if !httpx.DecodeJSON(w, r, &in) || !requireVersion(w, r, in.ExpectedVersion) {
		return
	}
	h.mutate(w, r, "queue.no_show", in, func(ctx context.Context, tx pgx.Tx) (int, any, error) {
		t, err := h.svc.NoShow(ctx, tx, principal(r), id, *in.ExpectedVersion, in.Reason, httpx.RequestID(ctx))
		return http.StatusOK, t, err
	})
}

// --- visits ---

func (h *HTTP) seat(w http.ResponseWriter, r *http.Request) {
	var in SeatInput
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	_, okB := idempotency.ParseKey(in.BranchID)
	_, okT := idempotency.ParseKey(in.TableID)
	okQ := in.QueueTicketID == nil
	if !okQ {
		_, okQ = idempotency.ParseKey(*in.QueueTicketID)
	}
	if !okB || !okT || !okQ {
		httpx.ValidationError(w, r, map[string]string{"ids": "branch_id, table_id and queue_ticket_id must be IDs"})
		return
	}
	h.mutate(w, r, "visits.seat", in, func(ctx context.Context, tx pgx.Tx) (int, any, error) {
		res, err := h.svc.Seat(ctx, tx, principal(r), in, httpx.RequestID(ctx))
		return http.StatusCreated, res, err
	})
}

func (h *HTTP) getVisit(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "visit_id")
	if !ok {
		return
	}
	v, err := h.svc.GetVisit(r.Context(), actorFrom(r), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, v)
}

func (h *HTTP) move(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "visit_id")
	if !ok {
		return
	}
	var in struct {
		TableID              string `json:"table_id"`
		ExpectedVersion      *int   `json:"expected_version"`
		ExpectedTableVersion *int   `json:"expected_table_version"`
	}
	if !httpx.DecodeJSON(w, r, &in) || !requireVersion(w, r, in.ExpectedVersion) || !requireVersion(w, r, in.ExpectedTableVersion) {
		return
	}
	if _, ok := idempotency.ParseKey(in.TableID); !ok {
		httpx.ValidationError(w, r, map[string]string{"table_id": "must be a table ID"})
		return
	}
	h.mutate(w, r, "visits.move", in, func(ctx context.Context, tx pgx.Tx) (int, any, error) {
		v, err := h.svc.Move(ctx, tx, principal(r), id, in.TableID, *in.ExpectedVersion, *in.ExpectedTableVersion)
		return http.StatusOK, v, err
	})
}

func (h *HTTP) depart(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "visit_id")
	if !ok {
		return
	}
	var in versionBody
	if !httpx.DecodeJSON(w, r, &in) || !requireVersion(w, r, in.ExpectedVersion) {
		return
	}
	h.mutate(w, r, "visits.depart", in, func(ctx context.Context, tx pgx.Tx) (int, any, error) {
		v, err := h.svc.Depart(ctx, tx, principal(r), id, *in.ExpectedVersion)
		return http.StatusOK, v, err
	})
}

type reasonBody struct {
	ExpectedVersion *int   `json:"expected_version"`
	Reason          string `json:"reason"`
}

func (h *HTTP) closeEmpty(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "visit_id")
	if !ok {
		return
	}
	var in reasonBody
	if !httpx.DecodeJSON(w, r, &in) || !requireVersion(w, r, in.ExpectedVersion) {
		return
	}
	h.mutate(w, r, "visits.close_empty", in, func(ctx context.Context, tx pgx.Tx) (int, any, error) {
		v, err := h.svc.CloseEmpty(ctx, tx, principal(r), id, *in.ExpectedVersion, in.Reason, httpx.RequestID(ctx))
		return http.StatusOK, v, err
	})
}

func (h *HTTP) rotate(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "visit_id")
	if !ok {
		return
	}
	var in reasonBody
	if !httpx.DecodeJSON(w, r, &in) || !requireVersion(w, r, in.ExpectedVersion) {
		return
	}
	h.mutate(w, r, "visits.rotate_access", in, func(ctx context.Context, tx pgx.Tx) (int, any, error) {
		res, err := h.svc.RotateAccess(ctx, tx, principal(r), id, *in.ExpectedVersion, in.Reason, httpx.RequestID(ctx))
		return http.StatusOK, res, err
	})
}
