// Package database creates the shared PostgreSQL connection pool.
package database

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Options bounds pool size and per-session timeouts.
type Options struct {
	MaxConns         int32
	StatementTimeout time.Duration
	LockTimeout      time.Duration
	IdleInTxTimeout  time.Duration
}

// NewPool builds a pool without connecting eagerly, so the process can start
// and report not-ready while PostgreSQL is unavailable.
func NewPool(ctx context.Context, databaseURL string, opts Options) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		// pgx parse errors can include the connection string; do not wrap them.
		return nil, fmt.Errorf("parse DATABASE_URL: invalid connection string")
	}
	cfg.MaxConns = opts.MaxConns
	cfg.MinConns = 0
	cfg.MaxConnIdleTime = 5 * time.Minute
	cfg.MaxConnLifetime = time.Hour
	cfg.HealthCheckPeriod = time.Minute

	params := cfg.ConnConfig.RuntimeParams
	params["application_name"] = "tableflow-api"
	params["statement_timeout"] = millis(opts.StatementTimeout)
	params["lock_timeout"] = millis(opts.LockTimeout)
	params["idle_in_transaction_session_timeout"] = millis(opts.IdleInTxTimeout)
	params["timezone"] = "UTC"

	return pgxpool.NewWithConfig(ctx, cfg)
}

func millis(d time.Duration) string {
	return strconv.FormatInt(d.Milliseconds(), 10)
}
