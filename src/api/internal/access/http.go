package access

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

// Cookie names: distinct per principal kind; host-only on the PWA origin.
const (
	GuestCookie     = "__Host-tf_guest"
	AnonymousCookie = "__Host-tf_anon"
)

// HTTP exposes guest/anonymous routes and middleware for domain modules.
type HTTP struct {
	svc     *Service
	origin  func(http.HandlerFunc) http.HandlerFunc
	trusted []netip.Prefix
	logger  *slog.Logger
}

// NewHTTP builds handlers; pwaOrigins may send guest/anonymous mutations.
func NewHTTP(svc *Service, pwaOrigins []string, trusted []netip.Prefix, logger *slog.Logger) *HTTP {
	return &HTTP{svc: svc, origin: httpx.OriginGuard(pwaOrigins), trusted: trusted, logger: logger}
}

// Routes returns path patterns and method handlers.
func (h *HTTP) Routes() map[string]map[string]http.HandlerFunc {
	return map[string]map[string]http.HandlerFunc{
		"/api/v1/sessions/anonymous":  {http.MethodPost: h.origin(h.anonymous)},
		"/api/v1/sessions/capability": {http.MethodPost: h.origin(h.exchange)},
		"/api/v1/sessions/guest":      {http.MethodGet: h.RequireGuest("", h.current)},
	}
}

type guestKey struct{}
type anonymousKey struct{}

// GuestFrom returns the authenticated guest set by RequireGuest.
func GuestFrom(ctx context.Context) (Guest, bool) {
	g, ok := ctx.Value(guestKey{}).(Guest)
	return g, ok
}

// AnonymousFrom returns the session set by RequireAnonymous.
func AnonymousFrom(ctx context.Context) (Anonymous, bool) {
	a, ok := ctx.Value(anonymousKey{}).(Anonymous)
	return a, ok
}

// RequireGuest authenticates the guest cookie (optionally of one kind) and
// marks the response private. Mutations must also pass the Origin guard
// and re-validate with RevalidateGuest in their transaction.
func (h *HTTP) RequireGuest(kind string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		httpx.Private(w)
		c, err := r.Cookie(GuestCookie)
		if err != nil {
			httpx.WriteError(w, r, http.StatusUnauthorized, "UNAUTHENTICATED", "Scan the QR code again to continue")
			return
		}
		g, err := h.svc.AuthenticateGuest(r.Context(), c.Value)
		if err == nil && kind != "" && g.Kind != kind {
			err = ErrUnauthenticated
		}
		if err != nil {
			h.fail(w, r, err)
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), guestKey{}, g)))
	}
}

// RequireAnonymous authenticates the anonymous bootstrap cookie.
func (h *HTTP) RequireAnonymous(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		httpx.Private(w)
		c, err := r.Cookie(AnonymousCookie)
		if err != nil {
			httpx.WriteError(w, r, http.StatusUnauthorized, "UNAUTHENTICATED", "Reload the page to continue")
			return
		}
		a, err := h.svc.AuthenticateAnonymous(r.Context(), c.Value)
		if errors.Is(err, ErrUnauthenticated) {
			// Clear only the anonymous cookie; a guest session is unaffected.
			setCookie(w, AnonymousCookie, "", time.Time{})
			httpx.WriteError(w, r, http.StatusUnauthorized, "UNAUTHENTICATED", "Reload the page to continue")
			return
		}
		if err != nil {
			h.fail(w, r, err)
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), anonymousKey{}, a)))
	}
}

func setCookie(w http.ResponseWriter, name, value string, expires time.Time) {
	c := &http.Cookie{
		Name: name, Value: value, Path: "/",
		// Lax: a QR scan is a cross-site top-level navigation that must still
		// carry the session; mutations are protected by the Origin guard.
		Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode,
	}
	if value == "" {
		c.MaxAge = -1
	} else {
		c.Expires = expires.UTC()
	}
	http.SetCookie(w, c)
}

func (h *HTTP) fail(w http.ResponseWriter, r *http.Request, err error) {
	var rl *throttle.RateLimitedError
	switch {
	case errors.As(err, &rl):
		httpx.RateLimited(w, r, int(math.Ceil(rl.RetryAfter.Seconds())))
	case errors.Is(err, ErrTokenInvalid):
		httpx.WriteError(w, r, http.StatusUnprocessableEntity, "TOKEN_INVALID", "This QR code is no longer valid; ask staff for a new one")
	case errors.Is(err, ErrUnauthenticated):
		setCookie(w, GuestCookie, "", time.Time{})
		httpx.WriteError(w, r, http.StatusUnauthorized, "UNAUTHENTICATED", "Scan the QR code again to continue")
	case errors.Is(err, context.Canceled):
	default:
		h.logger.Error("access request failed", "request_id", httpx.RequestID(r.Context()), "error", err.Error())
		httpx.WriteError(w, r, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE", "Service temporarily unavailable")
	}
}

type anonymousBody struct {
	ExpiresAt time.Time `json:"expires_at"`
}

func (h *HTTP) anonymous(w http.ResponseWriter, r *http.Request) {
	httpx.Private(w)
	existing := ""
	if c, err := r.Cookie(AnonymousCookie); err == nil {
		existing = c.Value
	}
	a, raw, created, err := h.svc.StartAnonymous(r.Context(), existing, httpx.ClientIP(r, h.trusted))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	status := http.StatusOK
	if created {
		setCookie(w, AnonymousCookie, raw, a.ExpiresAt)
		status = http.StatusCreated
	}
	httpx.WriteJSON(w, status, anonymousBody{ExpiresAt: a.ExpiresAt.UTC()})
}

func (h *HTTP) exchange(w http.ResponseWriter, r *http.Request) {
	httpx.Private(w)
	var in struct {
		Token string `json:"token"`
		Kind  string `json:"kind"`
	}
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	if in.Kind != KindQueue && in.Kind != KindVisit {
		httpx.ValidationError(w, r, map[string]string{"kind": "must be queue or visit"})
		return
	}
	g, raw, err := h.svc.Exchange(r.Context(), in.Token, in.Kind, httpx.ClientIP(r, h.trusted))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	setCookie(w, GuestCookie, raw, g.ExpiresAt)
	g.ExpiresAt = g.ExpiresAt.UTC()
	httpx.WriteJSON(w, http.StatusCreated, g)
}

func (h *HTTP) current(w http.ResponseWriter, r *http.Request) {
	g, _ := GuestFrom(r.Context())
	g.ExpiresAt = g.ExpiresAt.UTC()
	httpx.WriteJSON(w, http.StatusOK, g)
}
