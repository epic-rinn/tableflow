package identity

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"net/netip"
	"strconv"
	"time"

	"github.com/epic-rinn/tableflow/src/api/internal/platform/httpx"
)

// StaffCookie is host-only (__Host- prefix: Secure, Path=/, no Domain), so
// it is never sent to the customer PWA origin.
const StaffCookie = "__Host-tf_staff"

// HTTP exposes identity routes.
type HTTP struct {
	svc     *Service
	origins map[string]bool
	trusted []netip.Prefix
	logger  *slog.Logger
}

// NewHTTP builds handlers. adminOrigins are the exact origins allowed to
// send cookie-authenticated mutations.
func NewHTTP(svc *Service, adminOrigins []string, trusted []netip.Prefix, logger *slog.Logger) *HTTP {
	o := make(map[string]bool, len(adminOrigins))
	for _, v := range adminOrigins {
		o[v] = true
	}
	return &HTTP{svc: svc, origins: o, trusted: trusted, logger: logger}
}

// Routes returns path patterns and their method handlers.
func (h *HTTP) Routes() map[string]map[string]http.HandlerFunc {
	return map[string]map[string]http.HandlerFunc{
		"/api/v1/sessions/staff": {
			http.MethodPost: h.origin(h.login),
		},
		"/api/v1/sessions/current": {
			http.MethodGet:    h.staff(h.current),
			http.MethodDelete: h.origin(h.staff(h.logout)),
		},
		"/api/v1/staff/activate": {
			http.MethodPost: h.origin(h.activate),
		},
		"/api/v1/branches/{branch_id}/staff": {
			http.MethodGet:  h.staff(h.listStaff),
			http.MethodPost: h.origin(h.staff(h.invite)),
		},
		"/api/v1/staff/{staff_id}/activation": {
			http.MethodPost: h.origin(h.staff(h.reissue)),
		},
		"/api/v1/staff/{staff_id}/roles": {
			http.MethodPatch: h.origin(h.staff(h.setRoles)),
		},
		"/api/v1/staff/{staff_id}/deactivate": {
			http.MethodPost: h.origin(h.staff(h.deactivate)),
		},
	}
}

// origin rejects cross-site requests: the Origin header must be an allowed
// admin origin and, when browsers send Sec-Fetch-Site, it must be same-origin.
func (h *HTTP) origin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !h.origins[r.Header.Get("Origin")] {
			httpx.WriteError(w, r, http.StatusForbidden, "ORIGIN_REJECTED", "Request origin is not allowed")
			return
		}
		if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" {
			httpx.WriteError(w, r, http.StatusForbidden, "ORIGIN_REJECTED", "Request origin is not allowed")
			return
		}
		next(w, r)
	}
}

type principalKey struct{}

// PrincipalFrom returns the authenticated staff principal.
func PrincipalFrom(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalKey{}).(Principal)
	return p, ok
}

// staff authenticates the session cookie and marks the response private.
func (h *HTTP) staff(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		httpx.Private(w)
		c, err := r.Cookie(StaffCookie)
		if err != nil {
			httpx.WriteError(w, r, http.StatusUnauthorized, "UNAUTHENTICATED", "Sign in to continue")
			return
		}
		p, err := h.svc.Authenticate(r.Context(), c.Value)
		if err != nil {
			h.fail(w, r, err)
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), principalKey{}, p)))
	}
}

func (h *HTTP) setCookie(w http.ResponseWriter, value string, expires time.Time) {
	c := &http.Cookie{
		Name: StaffCookie, Value: value, Path: "/",
		Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode,
	}
	if value == "" {
		c.MaxAge = -1
	} else {
		c.Expires = expires.UTC()
	}
	http.SetCookie(w, c)
}

// fail maps service errors to the common error envelope.
func (h *HTTP) fail(w http.ResponseWriter, r *http.Request, err error) {
	var ve *ValidationError
	var rl *RateLimitedError
	switch {
	case errors.As(err, &ve):
		httpx.ValidationError(w, r, ve.Fields)
	case errors.As(err, &rl):
		w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(rl.RetryAfter.Seconds()))))
		httpx.WriteError(w, r, http.StatusTooManyRequests, "RATE_LIMITED", "Too many attempts; try again later")
	case errors.Is(err, ErrUnauthenticated):
		h.setCookie(w, "", time.Time{})
		httpx.WriteError(w, r, http.StatusUnauthorized, "UNAUTHENTICATED", "Sign in to continue")
	case errors.Is(err, ErrInvalidCredentials):
		httpx.WriteError(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Email or password is incorrect")
	case errors.Is(err, ErrForbidden):
		httpx.WriteError(w, r, http.StatusForbidden, "FORBIDDEN", "Your role does not allow this action")
	case errors.Is(err, ErrNotFound):
		httpx.NotFound(w, r)
	case errors.Is(err, ErrVersionConflict):
		httpx.WriteError(w, r, http.StatusConflict, "VERSION_CONFLICT", "This record changed; refresh and try again")
	case errors.Is(err, ErrEmailTaken):
		httpx.WriteError(w, r, http.StatusConflict, "EMAIL_TAKEN", "A staff account already uses this email")
	case errors.Is(err, ErrLastManager):
		httpx.WriteError(w, r, http.StatusConflict, "LAST_MANAGER", "The branch must keep at least one active manager")
	case errors.Is(err, ErrAccountDisabled):
		httpx.WriteError(w, r, http.StatusConflict, "ACCOUNT_DISABLED", "This account is deactivated")
	case errors.Is(err, ErrNotInvited):
		httpx.WriteError(w, r, http.StatusConflict, "NOT_INVITED", "Only invited accounts can receive activation links")
	case errors.Is(err, ErrTokenInvalid):
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "TOKEN_INVALID", "This activation link is invalid or has expired")
	case errors.Is(err, ErrInvalidCursor):
		httpx.WriteError(w, r, http.StatusBadRequest, "INVALID_CURSOR", "Invalid page cursor")
	case errors.Is(err, context.Canceled):
		// Client went away; nothing useful to write.
	default:
		h.logger.Error("identity request failed", "request_id", httpx.RequestID(r.Context()), "error", err.Error())
		httpx.WriteError(w, r, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE", "Service temporarily unavailable")
	}
}

type identityBody struct {
	StaffID     string   `json:"staff_id"`
	BranchID    string   `json:"branch_id"`
	Email       string   `json:"email"`
	DisplayName string   `json:"display_name"`
	Roles       []string `json:"roles"`
	ExpiresAt   *string  `json:"session_expires_at,omitempty"`
}

func identityOf(p Principal) identityBody {
	return identityBody{StaffID: p.StaffID, BranchID: p.BranchID, Email: p.Email, DisplayName: p.DisplayName, Roles: p.Roles}
}

func (h *HTTP) login(w http.ResponseWriter, r *http.Request) {
	httpx.Private(w)
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	p, raw, expires, err := h.svc.Login(r.Context(), in.Email, in.Password, httpx.ClientIP(r, h.trusted))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	h.setCookie(w, raw, expires)
	body := identityOf(p)
	exp := expires.UTC().Format(time.RFC3339)
	body.ExpiresAt = &exp
	httpx.WriteJSON(w, http.StatusCreated, body)
}

func (h *HTTP) current(w http.ResponseWriter, r *http.Request) {
	p, _ := PrincipalFrom(r.Context())
	httpx.WriteJSON(w, http.StatusOK, identityOf(p))
}

func (h *HTTP) logout(w http.ResponseWriter, r *http.Request) {
	p, _ := PrincipalFrom(r.Context())
	if err := h.svc.Logout(r.Context(), p); err != nil {
		h.fail(w, r, err)
		return
	}
	h.setCookie(w, "", time.Time{})
	w.WriteHeader(http.StatusNoContent)
}

func (h *HTTP) activate(w http.ResponseWriter, r *http.Request) {
	httpx.Private(w)
	var in struct {
		Token       string `json:"token"`
		Password    string `json:"password"`
		DisplayName string `json:"display_name"`
	}
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	err := h.svc.Activate(r.Context(), in.Token, in.Password, in.DisplayName, httpx.ClientIP(r, h.trusted), httpx.RequestID(r.Context()))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "active"})
}

type staffPage struct {
	Items      []Staff `json:"items"`
	NextCursor *string `json:"next_cursor"`
	ServerTime string  `json:"server_time"`
}

func (h *HTTP) listStaff(w http.ResponseWriter, r *http.Request) {
	p, _ := PrincipalFrom(r.Context())
	branchID := r.PathValue("branch_id")
	if !isUUID(branchID) {
		httpx.NotFound(w, r)
		return
	}
	limit := 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > staffPageMax {
			httpx.ValidationError(w, r, map[string]string{"limit": "must be 1–100"})
			return
		}
		limit = n
	}
	items, next, err := h.svc.ListStaff(r.Context(), p, branchID, r.URL.Query().Get("cursor"), limit)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	page := staffPage{Items: items, ServerTime: time.Now().UTC().Format(time.RFC3339)}
	if next != "" {
		page.NextCursor = &next
	}
	httpx.WriteJSON(w, http.StatusOK, page)
}

type invitation struct {
	Staff      Staff      `json:"staff"`
	Activation Activation `json:"activation"`
}

func (h *HTTP) invite(w http.ResponseWriter, r *http.Request) {
	p, _ := PrincipalFrom(r.Context())
	branchID := r.PathValue("branch_id")
	if !isUUID(branchID) {
		httpx.NotFound(w, r)
		return
	}
	var in struct {
		Email       string   `json:"email"`
		DisplayName string   `json:"display_name"`
		Roles       []string `json:"roles"`
	}
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	st, act, err := h.svc.Invite(r.Context(), p, branchID, in.Email, in.DisplayName, in.Roles, httpx.RequestID(r.Context()))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, invitation{Staff: st, Activation: act})
}

func (h *HTTP) staffID(w http.ResponseWriter, r *http.Request) (string, bool) {
	id := r.PathValue("staff_id")
	if !isUUID(id) {
		httpx.NotFound(w, r)
		return "", false
	}
	return id, true
}

func (h *HTTP) reissue(w http.ResponseWriter, r *http.Request) {
	p, _ := PrincipalFrom(r.Context())
	id, ok := h.staffID(w, r)
	if !ok {
		return
	}
	act, err := h.svc.ReissueActivation(r.Context(), p, id, httpx.RequestID(r.Context()))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, act)
}

func (h *HTTP) setRoles(w http.ResponseWriter, r *http.Request) {
	p, _ := PrincipalFrom(r.Context())
	id, ok := h.staffID(w, r)
	if !ok {
		return
	}
	var in struct {
		ExpectedVersion *int     `json:"expected_version"`
		Roles           []string `json:"roles"`
	}
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	if in.ExpectedVersion == nil {
		httpx.ValidationError(w, r, map[string]string{"expected_version": "is required"})
		return
	}
	st, err := h.svc.SetRoles(r.Context(), p, id, *in.ExpectedVersion, in.Roles, httpx.RequestID(r.Context()))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, st)
}

func (h *HTTP) deactivate(w http.ResponseWriter, r *http.Request) {
	p, _ := PrincipalFrom(r.Context())
	id, ok := h.staffID(w, r)
	if !ok {
		return
	}
	var in struct {
		ExpectedVersion *int   `json:"expected_version"`
		Reason          string `json:"reason"`
	}
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	if in.ExpectedVersion == nil {
		httpx.ValidationError(w, r, map[string]string{"expected_version": "is required"})
		return
	}
	st, err := h.svc.Deactivate(r.Context(), p, id, *in.ExpectedVersion, in.Reason, httpx.RequestID(r.Context()))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, st)
}
