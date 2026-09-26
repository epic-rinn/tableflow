package members

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"net/netip"
	"time"

	"github.com/epic-rinn/tableflow/src/api/internal/platform/httpx"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/throttle"
)

// MemberCookie is host-only on the PWA origin and coexists with guest and
// anonymous cookies.
const MemberCookie = "__Host-tf_member"

// HTTP exposes member routes.
type HTTP struct {
	svc     *Service
	origin  func(http.HandlerFunc) http.HandlerFunc
	trusted []netip.Prefix
	logger  *slog.Logger
}

// NewHTTP builds handlers; pwaOrigins may send member mutations.
func NewHTTP(svc *Service, pwaOrigins []string, trusted []netip.Prefix, logger *slog.Logger) *HTTP {
	return &HTTP{svc: svc, origin: httpx.OriginGuard(pwaOrigins), trusted: trusted, logger: logger}
}

// Routes returns path patterns and method handlers.
func (h *HTTP) Routes() map[string]map[string]http.HandlerFunc {
	return map[string]map[string]http.HandlerFunc{
		"/api/v1/members":                        {http.MethodPost: h.origin(h.signup)},
		"/api/v1/members/verification":           {http.MethodPost: h.origin(h.resend)},
		"/api/v1/members/verify":                 {http.MethodPost: h.origin(h.verify)},
		"/api/v1/members/me":                     {http.MethodGet: h.RequireMember(h.me)},
		"/api/v1/members/password-reset/request": {http.MethodPost: h.origin(h.resetRequest)},
		"/api/v1/members/password-reset/confirm": {http.MethodPost: h.origin(h.resetConfirm)},
		"/api/v1/sessions/member": {
			http.MethodPost:   h.origin(h.login),
			http.MethodDelete: h.origin(h.RequireMember(h.logout)),
		},
	}
}

type memberKey struct{}

// MemberFrom returns the member set by RequireMember.
func MemberFrom(ctx context.Context) (Member, bool) {
	m, ok := ctx.Value(memberKey{}).(Member)
	return m, ok
}

// RequireMember authenticates the member cookie and marks the response private.
func (h *HTTP) RequireMember(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		httpx.Private(w)
		c, err := r.Cookie(MemberCookie)
		if err != nil {
			httpx.WriteError(w, r, http.StatusUnauthorized, "UNAUTHENTICATED", "Sign in to continue")
			return
		}
		m, err := h.svc.Authenticate(r.Context(), c.Value)
		if err != nil {
			h.fail(w, r, err)
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), memberKey{}, m)))
	}
}

func setCookie(w http.ResponseWriter, value string, expires time.Time) {
	c := &http.Cookie{Name: MemberCookie, Value: value, Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode}
	if value == "" {
		c.MaxAge = -1
	} else {
		c.Expires = expires.UTC()
	}
	http.SetCookie(w, c)
}

func (h *HTTP) fail(w http.ResponseWriter, r *http.Request, err error) {
	var ve *ValidationError
	var rl *throttle.RateLimitedError
	switch {
	case errors.As(err, &ve):
		httpx.ValidationError(w, r, ve.Fields)
	case errors.As(err, &rl):
		httpx.RateLimited(w, r, int(math.Ceil(rl.RetryAfter.Seconds())))
	case errors.Is(err, ErrUnauthenticated):
		setCookie(w, "", time.Time{})
		httpx.WriteError(w, r, http.StatusUnauthorized, "UNAUTHENTICATED", "Sign in to continue")
	case errors.Is(err, ErrInvalidCredentials):
		httpx.WriteError(w, r, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Email or password is incorrect")
	case errors.Is(err, ErrTokenInvalid):
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "TOKEN_INVALID", "This link is invalid or has expired; request a new one")
	case errors.Is(err, context.Canceled):
	default:
		h.logger.Error("member request failed", "request_id", httpx.RequestID(r.Context()), "error", err.Error())
		httpx.WriteError(w, r, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE", "Service temporarily unavailable")
	}
}

var checkEmail = map[string]string{"status": "check_email"}

func (h *HTTP) signup(w http.ResponseWriter, r *http.Request) {
	httpx.Private(w)
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Locale   string `json:"locale"`
	}
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	if err := h.svc.Signup(r.Context(), in.Email, in.Password, in.Locale, httpx.ClientIP(r, h.trusted)); err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusAccepted, checkEmail)
}

func (h *HTTP) resend(w http.ResponseWriter, r *http.Request) {
	httpx.Private(w)
	var in struct {
		Email string `json:"email"`
	}
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	if err := h.svc.ResendVerification(r.Context(), in.Email, httpx.ClientIP(r, h.trusted)); err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusAccepted, checkEmail)
}

func (h *HTTP) verify(w http.ResponseWriter, r *http.Request) {
	httpx.Private(w)
	var in struct {
		Token string `json:"token"`
	}
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	if err := h.svc.Verify(r.Context(), in.Token, httpx.ClientIP(r, h.trusted)); err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "verified"})
}

func (h *HTTP) resetRequest(w http.ResponseWriter, r *http.Request) {
	httpx.Private(w)
	var in struct {
		Email string `json:"email"`
	}
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	if err := h.svc.RequestReset(r.Context(), in.Email, httpx.ClientIP(r, h.trusted)); err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusAccepted, checkEmail)
}

func (h *HTTP) resetConfirm(w http.ResponseWriter, r *http.Request) {
	httpx.Private(w)
	var in struct {
		Token       string `json:"token"`
		NewPassword string `json:"new_password"`
	}
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	if err := h.svc.ConfirmReset(r.Context(), in.Token, in.NewPassword, httpx.ClientIP(r, h.trusted)); err != nil {
		h.fail(w, r, err)
		return
	}
	setCookie(w, "", time.Time{}) // every session was revoked
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "password_reset"})
}

type memberBody struct {
	Member
	SessionExpiresAt *string `json:"session_expires_at,omitempty"`
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
	m, raw, expires, err := h.svc.Login(r.Context(), in.Email, in.Password, httpx.ClientIP(r, h.trusted))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	setCookie(w, raw, expires)
	exp := expires.UTC().Format(time.RFC3339)
	httpx.WriteJSON(w, http.StatusCreated, memberBody{Member: m, SessionExpiresAt: &exp})
}

func (h *HTTP) me(w http.ResponseWriter, r *http.Request) {
	m, _ := MemberFrom(r.Context())
	httpx.WriteJSON(w, http.StatusOK, memberBody{Member: m})
}

func (h *HTTP) logout(w http.ResponseWriter, r *http.Request) {
	m, _ := MemberFrom(r.Context())
	if err := h.svc.Logout(r.Context(), m); err != nil {
		h.fail(w, r, err)
		return
	}
	setCookie(w, "", time.Time{})
	w.WriteHeader(http.StatusNoContent)
}
