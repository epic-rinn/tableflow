package idempotency

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/epic-rinn/tableflow/src/api/internal/platform/httpx"
)

// Mutate runs fn and its idempotency record in one transaction and writes
// the (possibly replayed) response. The key comes from the Idempotency-Key
// header; the request hash covers the path and the canonical body. Failed
// commands roll back and store nothing; fail maps their errors.
func Mutate(w http.ResponseWriter, r *http.Request, pool *pgxpool.Pool, store *Store, scope, op string, ttl time.Duration, body any,
	fn func(ctx context.Context, tx pgx.Tx) (int, any, error), fail func(http.ResponseWriter, *http.Request, error)) {
	httpx.Private(w)
	key, ok := ParseKey(r.Header.Get(Header))
	if !ok {
		httpx.WriteError(w, r, http.StatusBadRequest, "IDEMPOTENCY_KEY_REQUIRED", "Send a UUID Idempotency-Key header")
		return
	}
	canonical, err := json.Marshal(body)
	if err != nil {
		fail(w, r, err)
		return
	}
	var res Result
	err = pgx.BeginFunc(r.Context(), pool, func(tx pgx.Tx) error {
		var err error
		res, err = store.Execute(r.Context(), tx, scope, op, key, RequestHash([]byte(r.URL.Path), canonical), ttl,
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
		fail(w, r, err)
		return
	}
	if res.Replayed {
		w.Header().Set("Idempotency-Replayed", "true")
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(res.Status)
	_, _ = w.Write(append(res.Body, '\n'))
}
