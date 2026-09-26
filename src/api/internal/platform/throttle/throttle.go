// Package throttle implements fixed-window attempt limits in PostgreSQL so
// limits hold across API replicas and restarts.
package throttle

import (
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"net/netip"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed sql/*.sql
var sqlFiles embed.FS

func q(name string) string {
	b, err := sqlFiles.ReadFile("sql/" + name + ".sql")
	if err != nil {
		panic("throttle: missing SQL " + name)
	}
	return string(b)
}

// RateLimitedError reports when the caller may retry.
type RateLimitedError struct{ RetryAfter time.Duration }

func (e *RateLimitedError) Error() string { return "rate limited" }

// Limiter counts attempts per bucket.
type Limiter struct{ pool *pgxpool.Pool }

// New returns a limiter backed by pool.
func New(pool *pgxpool.Pool) *Limiter { return &Limiter{pool: pool} }

// Hit records an attempt and fails once more than limit attempts occurred in
// the current window.
func (l *Limiter) Hit(ctx context.Context, bucket string, limit int, window time.Duration) error {
	var attempts int
	var remaining float64
	if err := l.pool.QueryRow(ctx, q("hit"), bucket, window).Scan(&attempts, &remaining); err != nil {
		return fmt.Errorf("throttle: %w", err)
	}
	if attempts > limit {
		return &RateLimitedError{RetryAfter: max(time.Duration(remaining*float64(time.Second)), time.Second)}
	}
	return nil
}

// Clear resets a bucket (e.g. after a successful login).
func (l *Limiter) Clear(ctx context.Context, bucket string) error {
	_, err := l.pool.Exec(ctx, q("clear"), bucket)
	return err
}

// Purge deletes buckets whose window ended more than a day ago.
func (l *Limiter) Purge(ctx context.Context) (int64, error) {
	tag, err := l.pool.Exec(ctx, q("purge"))
	return tag.RowsAffected(), err
}

// HashedKey builds a bucket name without storing the raw value (e.g. email).
func HashedKey(kind, value string) string {
	sum := sha256.Sum256([]byte(value))
	return kind + ":" + hex.EncodeToString(sum[:16])
}

// IPKey builds a per-client-address bucket name.
func IPKey(kind string, ip netip.Addr) string {
	if !ip.IsValid() {
		return kind + ":unknown"
	}
	return kind + ":" + ip.String()
}
