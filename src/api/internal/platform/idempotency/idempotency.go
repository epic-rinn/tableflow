// Package idempotency stores committed mutation results so a retried request
// with the same scoped key replays the original response instead of
// repeating its effects.
//
// Callers must authenticate and authorise the request before Execute, and
// include the principal in scope, so a replay is never served to someone who
// could not have made the original request.
package idempotency

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"embed"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed sql/*.sql
var sqlFiles embed.FS

func q(name string) string {
	b, err := sqlFiles.ReadFile("sql/" + name + ".sql")
	if err != nil {
		panic("idempotency: missing SQL " + name)
	}
	return string(b)
}

// Header is the request header carrying the client's key.
const Header = "Idempotency-Key"

// DefaultTTL is the minimum retention required by the data model.
const DefaultTTL = 24 * time.Hour

// ErrConflict: the key was already used with a different request.
var ErrConflict = errors.New("idempotency key reused with a different request")

// Result is a committed (or replayed) response.
type Result struct {
	Status   int
	Body     []byte
	Replayed bool
}

// Store seals stored responses with AES-256-GCM.
type Store struct{ aead cipher.AEAD }

// NewStore builds a store from a 32-byte key.
func NewStore(key []byte) (*Store, error) {
	if len(key) != 32 {
		return nil, errors.New("idempotency: key must be 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Store{aead: aead}, nil
}

// ParseKey validates a client key (canonical UUID) and returns it.
func ParseKey(raw string) (string, bool) {
	if len(raw) != 36 {
		return "", false
	}
	for i, c := range raw {
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return "", false
			}
		default:
			if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
				return "", false
			}
		}
	}
	return raw, true
}

// RequestHash digests the parts that define "the same request" (route
// parameters and canonical body), length-prefixed to avoid ambiguity.
func RequestHash(parts ...[]byte) []byte {
	h := sha256.New()
	for _, p := range parts {
		fmt.Fprintf(h, "%d:", len(p))
		h.Write(p)
	}
	return h.Sum(nil)
}

func aad(scope, operation, key string) []byte {
	return []byte(scope + "\x00" + operation + "\x00" + key)
}

func (s *Store) seal(scope, operation, key string, body []byte) []byte {
	nonce := make([]byte, s.aead.NonceSize())
	_, _ = rand.Read(nonce)
	return s.aead.Seal(nonce, nonce, body, aad(scope, operation, key))
}

func (s *Store) open(scope, operation, key string, sealed []byte) ([]byte, error) {
	n := s.aead.NonceSize()
	if len(sealed) < n {
		return nil, errors.New("idempotency: sealed response too short")
	}
	return s.aead.Open(nil, sealed[:n], sealed[n:], aad(scope, operation, key))
}

// Execute runs fn at most once per (scope, operation, key) inside tx. fn's
// effects and the stored result commit or roll back together with tx. If the
// key was committed before, fn is not called: the original result is
// replayed, or ErrConflict returned when reqHash differs.
//
// fn returns a status/body to store (including business rejections, which
// then replay identically) or an error, after which the caller must roll
// back so no record remains.
func (s *Store) Execute(ctx context.Context, tx pgx.Tx, scope, operation, key string, reqHash []byte, ttl time.Duration,
	fn func() (int, []byte, error)) (Result, error) {
	var id string
	err := tx.QueryRow(ctx, q("claim"), scope, operation, key, reqHash, ttl).Scan(&id)
	if err == nil {
		status, body, err := fn()
		if err != nil {
			return Result{}, err
		}
		if status < 200 || status > 599 {
			return Result{}, fmt.Errorf("idempotency: invalid status %d", status)
		}
		tag, err := tx.Exec(ctx, q("complete"), id, status, s.seal(scope, operation, key, body))
		if err != nil {
			return Result{}, err
		}
		if tag.RowsAffected() != 1 {
			return Result{}, errors.New("idempotency: claim lost")
		}
		return Result{Status: status, Body: body}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Result{}, fmt.Errorf("idempotency claim: %w", err)
	}
	var storedHash, sealed []byte
	var status int
	if err := tx.QueryRow(ctx, q("lookup"), scope, operation, key).Scan(&storedHash, &status, &sealed); err != nil {
		return Result{}, fmt.Errorf("idempotency lookup: %w", err)
	}
	if !bytes.Equal(storedHash, reqHash) {
		return Result{}, ErrConflict
	}
	if status == 0 {
		// Only possible if a caller committed after fn failed; never replay it.
		return Result{}, errors.New("idempotency: incomplete record committed")
	}
	body, err := s.open(scope, operation, key, sealed)
	if err != nil {
		return Result{}, err
	}
	return Result{Status: status, Body: body, Replayed: true}, nil
}

// Purge deletes expired records in bounded batches.
func Purge(ctx context.Context, pool *pgxpool.Pool) (int64, error) {
	tag, err := pool.Exec(ctx, q("purge"))
	return tag.RowsAffected(), err
}
