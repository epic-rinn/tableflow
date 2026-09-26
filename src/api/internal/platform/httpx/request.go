package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
)

// MaxJSONBody bounds request bodies for ordinary JSON endpoints.
const MaxJSONBody = 16 << 10

// DecodeJSON reads exactly one JSON object into dst, rejecting other media
// types, unknown fields, trailing data and bodies over MaxJSONBody. On
// failure it writes the error response and returns false.
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mt != "application/json" {
		WriteError(w, r, http.StatusUnsupportedMediaType, "UNSUPPORTED_MEDIA_TYPE", "Content-Type must be application/json")
		return false
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, MaxJSONBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			WriteError(w, r, http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "Request body is too large")
			return false
		}
		WriteError(w, r, http.StatusBadRequest, "MALFORMED_REQUEST", "Request body is not valid JSON for this operation")
		return false
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		WriteError(w, r, http.StatusBadRequest, "MALFORMED_REQUEST", "Request body must contain one JSON object")
		return false
	}
	return true
}

// ValidationError writes a 422 with per-field messages.
func ValidationError(w http.ResponseWriter, r *http.Request, fields map[string]string) {
	WriteJSON(w, http.StatusUnprocessableEntity, ErrorBody{Error: ErrorDetail{
		Code:      "VALIDATION_FAILED",
		Message:   "Some fields are invalid",
		RequestID: RequestID(r.Context()),
		Fields:    fields,
	}})
}

// ClientIP returns the connecting address, or, when that address is a
// trusted proxy, the right-most X-Forwarded-For entry that is not itself
// trusted. Arbitrary clients cannot spoof their address this way.
func ClientIP(r *http.Request, trusted []netip.Prefix) netip.Addr {
	remote, err := netip.ParseAddrPort(r.RemoteAddr)
	if err != nil {
		return netip.Addr{}
	}
	addr := remote.Addr().Unmap()
	if !isTrusted(addr, trusted) {
		return addr
	}
	hops := strings.Split(strings.Join(r.Header.Values("X-Forwarded-For"), ","), ",")
	for i := len(hops) - 1; i >= 0; i-- {
		hop, err := netip.ParseAddr(strings.TrimSpace(hops[i]))
		if err != nil {
			return addr // malformed chain: fall back to the proxy itself
		}
		hop = hop.Unmap()
		if !isTrusted(hop, trusted) {
			return hop
		}
	}
	return addr
}

func isTrusted(a netip.Addr, trusted []netip.Prefix) bool {
	for _, p := range trusted {
		if p.Contains(a) {
			return true
		}
	}
	return false
}

// OriginGuard rejects cross-site requests: the Origin header must be one of
// allowed and, when browsers send Sec-Fetch-Site, it must be same-origin.
func OriginGuard(allowed []string) func(http.HandlerFunc) http.HandlerFunc {
	set := make(map[string]bool, len(allowed))
	for _, o := range allowed {
		set[o] = true
	}
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			site := r.Header.Get("Sec-Fetch-Site")
			if !set[r.Header.Get("Origin")] || (site != "" && site != "same-origin") {
				WriteError(w, r, http.StatusForbidden, "ORIGIN_REJECTED", "Request origin is not allowed")
				return
			}
			next(w, r)
		}
	}
}

// RateLimited writes a 429 with a Retry-After header in whole seconds.
func RateLimited(w http.ResponseWriter, r *http.Request, seconds int) {
	w.Header().Set("Retry-After", strconv.Itoa(max(seconds, 1)))
	WriteError(w, r, http.StatusTooManyRequests, "RATE_LIMITED", "Too many attempts; try again later")
}
