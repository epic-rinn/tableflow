// Package httpx contains shared HTTP middleware and response helpers.
package httpx

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"runtime/debug"
	"slices"
	"strings"
	"time"
)

// RequestIDHeader carries the correlation ID in requests and responses.
const RequestIDHeader = "X-Request-ID"

type ctxKey struct{}

// RequestID returns the request's correlation ID, or "" outside middleware.
func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(ctxKey{}).(string)
	return id
}

// ErrorBody is the common API error envelope from the HTTP contract.
type ErrorBody struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail describes one error with a stable code.
type ErrorDetail struct {
	Code      string            `json:"code"`
	Message   string            `json:"message"`
	RequestID string            `json:"request_id"`
	Fields    map[string]string `json:"fields"`
}

// WriteJSON writes v as JSON with the given status. API responses are never
// stored by browsers or shared caches unless a route opts in explicitly.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	h := w.Header()
	h.Set("Content-Type", "application/json; charset=utf-8")
	if h.Get("Cache-Control") == "" {
		h.Set("Cache-Control", "no-store")
	}
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// WriteError writes the common error envelope.
func WriteError(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	WriteJSON(w, status, ErrorBody{Error: ErrorDetail{
		Code:      code,
		Message:   message,
		RequestID: RequestID(r.Context()),
		Fields:    map[string]string{},
	}})
}

// NotFound is the JSON fallback for unknown routes.
func NotFound(w http.ResponseWriter, r *http.Request) {
	WriteError(w, r, http.StatusNotFound, "NOT_FOUND", "Resource not found")
}

// Method restricts a handler to one HTTP method with a JSON 405 response.
func Method(method string, h http.HandlerFunc) http.HandlerFunc {
	return Methods(map[string]http.HandlerFunc{method: h})
}

// Methods dispatches by HTTP method with a JSON 405 response for others.
// HEAD is served by the GET handler when one exists.
func Methods(handlers map[string]http.HandlerFunc) http.HandlerFunc {
	allowed := make([]string, 0, len(handlers))
	for m := range handlers {
		allowed = append(allowed, m)
	}
	slices.Sort(allowed)
	allow := strings.Join(allowed, ", ")
	return func(w http.ResponseWriter, r *http.Request) {
		method := r.Method
		if method == http.MethodHead {
			method = http.MethodGet
		}
		h, ok := handlers[method]
		if !ok {
			w.Header().Set("Allow", allow)
			WriteError(w, r, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
			return
		}
		h(w, r)
	}
}

// Private marks a response as private and never stored by any cache.
func Private(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "private, no-store")
}

// validRequestID accepts short opaque IDs from a trusted proxy or client
// without allowing log injection or unbounded header reuse.
func validRequestID(id string) bool {
	if id == "" || len(id) > 64 {
		return false
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '-', c == '_', c == '.':
		default:
			return false
		}
	}
	return true
}

func newRequestID() string {
	var b [16]byte
	_, _ = rand.Read(b[:]) // crypto/rand.Read never returns an error.
	return hex.EncodeToString(b[:])
}

type recorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (r *recorder) WriteHeader(status int) {
	if r.status == 0 {
		r.status = status
	}
	r.ResponseWriter.WriteHeader(status)
}

func (r *recorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(b)
	r.bytes += n
	return n, err
}

func (r *recorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

// Middleware assigns request IDs, sets baseline headers, recovers panics, and
// writes one structured access log per request. Query strings are never logged.
func Middleware(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		id := r.Header.Get(RequestIDHeader)
		if !validRequestID(id) {
			id = newRequestID()
		}
		r = r.WithContext(context.WithValue(r.Context(), ctxKey{}, id))
		h := w.Header()
		h.Set(RequestIDHeader, id)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")

		rec := &recorder{ResponseWriter: w}
		defer func() {
			if p := recover(); p != nil {
				if p == http.ErrAbortHandler {
					panic(p)
				}
				logger.Error("panic serving request",
					"request_id", id, "panic", p, "stack", string(debug.Stack()))
				if rec.status == 0 {
					WriteError(rec, r, http.StatusInternalServerError, "INTERNAL", "Internal server error")
				}
			}
			if rec.status == 0 {
				rec.status = http.StatusOK // net/http's implicit status
			}
			// ServeMux records the matched pattern on this request value.
			route := r.Pattern
			if route == "" {
				route = "unmatched"
			}
			logger.Info("request",
				"request_id", id,
				"method", r.Method,
				"route", route,
				"status", rec.status,
				"bytes", rec.bytes,
				"duration_ms", float64(time.Since(start).Microseconds())/1000,
			)
		}()
		next.ServeHTTP(rec, r)
	})
}
