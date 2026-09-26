package billing

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/epic-rinn/tableflow/src/api/internal/access"
	"github.com/epic-rinn/tableflow/src/api/internal/identity"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/httpx"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/idempotency"
)

// replayTTL: a settlement retry after response loss happens within the
// business day; keep results longer to cover a next-day reconciliation.
const replayTTL = 72 * time.Hour

// HTTP exposes billing, settlement and receipt routes.
type HTTP struct {
	svc    *Service
	pool   *pgxpool.Pool
	store  *idempotency.Store
	staff  *identity.HTTP
	guests *access.HTTP
	logger *slog.Logger
}

// NewHTTP wires handlers.
func NewHTTP(svc *Service, pool *pgxpool.Pool, store *idempotency.Store, staff *identity.HTTP, guests *access.HTTP, logger *slog.Logger) *HTTP {
	return &HTTP{svc: svc, pool: pool, store: store, staff: staff, guests: guests, logger: logger}
}

// Routes returns path patterns and method handlers. No route accepts a
// guest credential for settlement or refunds (BIL-A4).
func (h *HTTP) Routes() map[string]map[string]http.HandlerFunc {
	st := func(f http.HandlerFunc) http.HandlerFunc { return h.staff.RequireStaff(f) }
	stw := func(f http.HandlerFunc) http.HandlerFunc { return h.staff.Origin(h.staff.RequireStaff(f)) }
	return map[string]map[string]http.HandlerFunc{
		"/api/v1/branches/{branch_id}/charge-policy": {http.MethodGet: st(h.policy), http.MethodPut: stw(h.setPolicy)},
		"/api/v1/visits/{visit_id}/bill": {
			http.MethodGet: h.either(st(h.bill), h.guests.RequireGuest(access.KindVisit, h.bill)),
		},
		"/api/v1/bills/resolve":                        {http.MethodPost: stw(h.resolve)},
		"/api/v1/visits/{visit_id}/settlement/begin":   {http.MethodPost: stw(h.begin)},
		"/api/v1/visits/{visit_id}/settlement/reopen":  {http.MethodPost: stw(h.reopen)},
		"/api/v1/visits/{visit_id}/settlement/confirm": {http.MethodPost: stw(h.confirm)},
		"/api/v1/branches/{branch_id}/settlements":     {http.MethodGet: st(h.receipts)},
		"/api/v1/settlements/{settlement_id}":          {http.MethodGet: st(h.receipt)},
		"/api/v1/settlements/{settlement_id}/refund":   {http.MethodPost: stw(h.refund)},
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

type conflictBody struct {
	Error      httpx.ErrorDetail `json:"error"`
	Bill       *Bill             `json:"bill,omitempty"`
	Lines      []BillLine        `json:"lines,omitempty"`
	Settlement *SettlementRef    `json:"settlement,omitempty"`
}

func (h *HTTP) fail(w http.ResponseWriter, r *http.Request, err error) {
	detail := func(code, msg string) httpx.ErrorDetail {
		return httpx.ErrorDetail{Code: code, Message: msg, RequestID: httpx.RequestID(r.Context()), Fields: map[string]string{}}
	}
	var ve *ValidationError
	var bc *BillConflictError
	var ue *UnresolvedError
	var ap *AlreadyPaidError
	var ar *AlreadyRefundedError
	switch {
	case errors.As(err, &ve):
		httpx.ValidationError(w, r, ve.Fields)
	case errors.As(err, &bc):
		httpx.WriteJSON(w, http.StatusConflict, conflictBody{Error: detail("BILL_VERSION_CONFLICT", "The bill changed; review the refreshed bill"), Bill: &bc.Bill})
	case errors.As(err, &ue):
		httpx.WriteJSON(w, http.StatusConflict, conflictBody{Error: detail("UNRESOLVED_LINES", "Some items are not served, rejected or cancelled yet"), Lines: ue.Lines})
	case errors.As(err, &ap):
		d := detail("ALREADY_PAID", "This bill is already paid")
		d.Fields["receipt_reference"] = ap.Settlement.ReceiptReference
		httpx.WriteJSON(w, http.StatusConflict, conflictBody{Error: d, Settlement: &ap.Settlement})
	case errors.As(err, &ar):
		httpx.WriteError(w, r, http.StatusConflict, "ALREADY_REFUNDED", "This settlement already has a refund")
	case errors.Is(err, identity.ErrUnauthenticated):
		h.staff.Fail(w, r, err)
	case errors.Is(err, idempotency.ErrConflict):
		httpx.WriteError(w, r, http.StatusConflict, "IDEMPOTENCY_CONFLICT", "This request key was already used for a different request")
	case errors.Is(err, ErrNotFound):
		httpx.NotFound(w, r)
	case errors.Is(err, ErrForbidden):
		httpx.WriteError(w, r, http.StatusForbidden, "FORBIDDEN", "Your role does not allow this action")
	case errors.Is(err, ErrVersionConflict):
		httpx.WriteError(w, r, http.StatusConflict, "VERSION_CONFLICT", "This changed meanwhile; refresh and try again")
	case errors.Is(err, ErrVisitState):
		httpx.WriteError(w, r, http.StatusConflict, "VISIT_STATE_CONFLICT", "This visit is not in a state that allows this")
	case errors.Is(err, ErrAmountMismatch):
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "AMOUNT_MISMATCH", "The recorded amount must equal the bill total")
	case errors.Is(err, context.Canceled):
	default:
		h.logger.Error("billing request failed", "request_id", httpx.RequestID(r.Context()), "error", err.Error())
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

// mutate runs an idempotent staff command. Roles are checked from the
// freshly authenticated session before a stored result can be replayed, so
// a demoted cashier cannot replay a confirmation (HTTP contract).
func (h *HTTP) mutate(w http.ResponseWriter, r *http.Request, roles []string, op string, body any, fn func(ctx context.Context, tx pgx.Tx, p identity.Principal) (int, any, error)) {
	p, _ := identity.PrincipalFrom(r.Context())
	if !slices.ContainsFunc(roles, p.Has) {
		h.fail(w, r, ErrForbidden)
		return
	}
	idempotency.Mutate(w, r, h.pool, h.store, "staff:"+p.StaffID, op, replayTTL, body,
		func(ctx context.Context, tx pgx.Tx) (int, any, error) { return fn(ctx, tx, p) }, h.fail)
}

func (h *HTTP) policy(w http.ResponseWriter, r *http.Request) {
	branch, ok := pathID(w, r, "branch_id")
	if !ok {
		return
	}
	p, _ := identity.PrincipalFrom(r.Context())
	v, err := h.svc.ChargePolicy(r.Context(), p, branch)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.Private(w)
	httpx.WriteJSON(w, http.StatusOK, v)
}

func (h *HTTP) setPolicy(w http.ResponseWriter, r *http.Request) {
	branch, ok := pathID(w, r, "branch_id")
	if !ok {
		return
	}
	var in PolicyIn
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	p, _ := identity.PrincipalFrom(r.Context())
	var v PolicyView
	err := pgx.BeginFunc(r.Context(), h.pool, func(tx pgx.Tx) error {
		var err error
		v, err = h.svc.SetChargePolicy(r.Context(), tx, p, branch, in, httpx.RequestID(r.Context()))
		return err
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.Private(w)
	httpx.WriteJSON(w, http.StatusOK, v)
}

func (h *HTTP) bill(w http.ResponseWriter, r *http.Request) {
	visit, ok := pathID(w, r, "visit_id")
	if !ok {
		return
	}
	var rd Reader
	if p, ok := identity.PrincipalFrom(r.Context()); ok {
		rd.Staff = &p
	}
	if g, ok := access.GuestFrom(r.Context()); ok {
		rd.Guest = &g
	}
	b, err := h.svc.Bill(r.Context(), rd, visit)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.Private(w)
	httpx.WriteJSON(w, http.StatusOK, b)
}

func (h *HTTP) resolve(w http.ResponseWriter, r *http.Request) {
	var in struct {
		DiningToken string `json:"dining_token"`
	}
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	p, _ := identity.PrincipalFrom(r.Context())
	b, err := h.svc.Resolve(r.Context(), p, in.DiningToken)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.Private(w)
	httpx.WriteJSON(w, http.StatusOK, b)
}

type versionIn struct {
	ExpectedVersion *int    `json:"expected_version"`
	Reason          *string `json:"reason,omitempty"`
}

func (h *HTTP) versionBody(w http.ResponseWriter, r *http.Request) (versionIn, bool) {
	var in versionIn
	if !httpx.DecodeJSON(w, r, &in) {
		return in, false
	}
	if in.ExpectedVersion == nil {
		httpx.ValidationError(w, r, map[string]string{"expected_version": "is required"})
		return in, false
	}
	return in, true
}

func (h *HTTP) begin(w http.ResponseWriter, r *http.Request) {
	visit, ok := pathID(w, r, "visit_id")
	if !ok {
		return
	}
	in, ok := h.versionBody(w, r)
	if !ok {
		return
	}
	h.mutate(w, r, cashierRoles, "settlement.begin", in, func(ctx context.Context, tx pgx.Tx, p identity.Principal) (int, any, error) {
		b, err := h.svc.BeginSettlement(ctx, tx, p, visit, *in.ExpectedVersion, httpx.RequestID(ctx))
		return http.StatusOK, b, err
	})
}

func (h *HTTP) reopen(w http.ResponseWriter, r *http.Request) {
	visit, ok := pathID(w, r, "visit_id")
	if !ok {
		return
	}
	in, ok := h.versionBody(w, r)
	if !ok {
		return
	}
	reason := ""
	if in.Reason != nil {
		reason = *in.Reason
	}
	h.mutate(w, r, cashierRoles, "settlement.reopen", in, func(ctx context.Context, tx pgx.Tx, p identity.Principal) (int, any, error) {
		b, err := h.svc.ReopenSettlement(ctx, tx, p, visit, *in.ExpectedVersion, reason, httpx.RequestID(ctx))
		return http.StatusOK, b, err
	})
}

func (h *HTTP) confirm(w http.ResponseWriter, r *http.Request) {
	visit, ok := pathID(w, r, "visit_id")
	if !ok {
		return
	}
	var in ConfirmIn
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	h.mutate(w, r, cashierRoles, "settlement.confirm", in, func(ctx context.Context, tx pgx.Tx, p identity.Principal) (int, any, error) {
		s, err := h.svc.ConfirmSettlement(ctx, tx, p, visit, in, httpx.RequestID(ctx))
		return http.StatusCreated, s, err
	})
}

func (h *HTTP) receipts(w http.ResponseWriter, r *http.Request) {
	branch, ok := pathID(w, r, "branch_id")
	if !ok {
		return
	}
	limit := 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > pageMax {
			httpx.ValidationError(w, r, map[string]string{"limit": "must be 1–100"})
			return
		}
		limit = n
	}
	p, _ := identity.PrincipalFrom(r.Context())
	page, err := h.svc.Receipts(r.Context(), p, branch, r.URL.Query().Get("receipt_reference"), r.URL.Query().Get("cursor"), limit)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.Private(w)
	httpx.WriteJSON(w, http.StatusOK, page)
}

func (h *HTTP) receipt(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "settlement_id")
	if !ok {
		return
	}
	p, _ := identity.PrincipalFrom(r.Context())
	rc, err := h.svc.Receipt(r.Context(), p, id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.Private(w)
	httpx.WriteJSON(w, http.StatusOK, rc)
}

func (h *HTTP) refund(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "settlement_id")
	if !ok {
		return
	}
	var in RefundIn
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	h.mutate(w, r, []string{identity.RoleManager}, "settlement.refund", in, func(ctx context.Context, tx pgx.Tx, p identity.Principal) (int, any, error) {
		rc, err := h.svc.RecordRefund(ctx, tx, p, id, in, httpx.RequestID(ctx))
		return http.StatusCreated, rc, err
	})
}
