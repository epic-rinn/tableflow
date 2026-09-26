package idempotency_test

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/epic-rinn/tableflow/src/api/internal/platform/dbtest"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/idempotency"
)

const key = "3b241101-e2bb-4255-8caf-4136c566a962"

func setup(t *testing.T) (*pgxpool.Pool, *idempotency.Store) {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), dbtest.NewMigratedDatabase(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	// A stand-in business effect: one row per executed request.
	if _, err := pool.Exec(context.Background(), "CREATE TABLE effects (id serial PRIMARY KEY, note text NOT NULL)"); err != nil {
		t.Fatal(err)
	}
	store, err := idempotency.NewStore(bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatal(err)
	}
	return pool, store
}

// do performs one idempotent "create effect" in its own transaction.
func do(ctx context.Context, pool *pgxpool.Pool, store *idempotency.Store, scope, note string, fail error, hold time.Duration) (idempotency.Result, error) {
	var res idempotency.Result
	err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		var err error
		res, err = store.Execute(ctx, tx, scope, "effects.create", key, idempotency.RequestHash([]byte(note)), time.Hour,
			func() (int, []byte, error) {
				var id int
				if err := tx.QueryRow(ctx, "INSERT INTO effects (note) VALUES ($1) RETURNING id", note).Scan(&id); err != nil {
					return 0, nil, err
				}
				time.Sleep(hold)
				if fail != nil {
					return 0, nil, fail
				}
				return 201, []byte(`{"note":"` + note + `"}`), nil
			})
		return err
	})
	return res, err
}

func count(t *testing.T, pool *pgxpool.Pool, sql string) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), sql).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// TestIdempotencyConcurrentSameKey: parallel same-key/same-body requests on
// independent connections commit exactly one effect; all return its result.
func TestIdempotencyConcurrentSameKey(t *testing.T) {
	pool, store := setup(t)
	ctx := context.Background()
	const n = 10
	start := make(chan struct{})
	var wg sync.WaitGroup
	results := make([]idempotency.Result, n)
	errs := make([]error, n)
	for i := range n {
		wg.Go(func() {
			<-start
			results[i], errs[i] = do(ctx, pool, store, "guest:s1", "a", nil, 50*time.Millisecond)
		})
	}
	close(start)
	wg.Wait()
	fresh := 0
	for i := range n {
		if errs[i] != nil {
			t.Fatalf("request %d: %v", i, errs[i])
		}
		if results[i].Status != 201 || string(results[i].Body) != `{"note":"a"}` {
			t.Fatalf("request %d result %+v", i, results[i])
		}
		if !results[i].Replayed {
			fresh++
		}
	}
	if fresh != 1 {
		t.Fatalf("%d requests executed, want 1", fresh)
	}
	if got := count(t, pool, "SELECT count(*) FROM effects"); got != 1 {
		t.Fatalf("%d effects committed", got)
	}
}

// TestIdempotencyConflictReplayRollback: replay after response loss, a
// different body conflicts, a failed attempt leaves no record, and the same
// key in another scope is independent.
func TestIdempotencyConflictReplayRollback(t *testing.T) {
	pool, store := setup(t)
	ctx := context.Background()

	if _, err := do(ctx, pool, store, "guest:s1", "a", errors.New("boom"), 0); err == nil {
		t.Fatal("expected failure")
	}
	if got := count(t, pool, "SELECT count(*) FROM idempotency_requests"); got != 0 {
		t.Fatalf("failed attempt left %d records", got)
	}
	if got := count(t, pool, "SELECT count(*) FROM effects"); got != 0 {
		t.Fatal("failed attempt left an effect")
	}

	first, err := do(ctx, pool, store, "guest:s1", "a", nil, 0)
	if err != nil || first.Replayed {
		t.Fatalf("first: %+v %v", first, err)
	}
	again, err := do(ctx, pool, store, "guest:s1", "a", nil, 0)
	if err != nil || !again.Replayed || again.Status != first.Status || !bytes.Equal(again.Body, first.Body) {
		t.Fatalf("replay: %+v %v", again, err)
	}
	if _, err := do(ctx, pool, store, "guest:s1", "b", nil, 0); !errors.Is(err, idempotency.ErrConflict) {
		t.Fatalf("different body: %v", err)
	}
	other, err := do(ctx, pool, store, "guest:s2", "b", nil, 0)
	if err != nil || other.Replayed {
		t.Fatalf("other scope: %+v %v", other, err)
	}
	if got := count(t, pool, "SELECT count(*) FROM effects"); got != 2 {
		t.Fatalf("effects = %d, want 2", got)
	}
}

// TestIdempotencyRolledBackOwnerLetsWaiterProceed: a waiter blocked on an
// in-flight claim executes itself when the owner rolls back.
func TestIdempotencyRolledBackOwnerLetsWaiterProceed(t *testing.T) {
	pool, store := setup(t)
	ctx := context.Background()
	ownerErr := make(chan error, 1)
	go func() {
		_, err := do(ctx, pool, store, "guest:s1", "a", errors.New("late failure"), 300*time.Millisecond)
		ownerErr <- err
	}()
	time.Sleep(100 * time.Millisecond)
	res, err := do(ctx, pool, store, "guest:s1", "a", nil, 0)
	if err != nil || res.Replayed || res.Status != 201 {
		t.Fatalf("waiter: %+v %v", res, err)
	}
	if err := <-ownerErr; err == nil {
		t.Fatal("owner should have failed")
	}
	if got := count(t, pool, "SELECT count(*) FROM effects"); got != 1 {
		t.Fatalf("effects = %d", got)
	}
}

// TestIdempotencyResponseEncrypted: stored responses are sealed, bound to
// their scope/operation/key, and unreadable with another key.
func TestIdempotencyResponseEncrypted(t *testing.T) {
	pool, store := setup(t)
	ctx := context.Background()
	if _, err := do(ctx, pool, store, "guest:s1", "secret-qr-token", nil, 0); err != nil {
		t.Fatal(err)
	}
	var stored []byte
	if err := pool.QueryRow(ctx, "SELECT response FROM idempotency_requests").Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(stored, []byte("secret-qr-token")) {
		t.Fatal("response stored in plaintext")
	}
	other, _ := idempotency.NewStore(bytes.Repeat([]byte{8}, 32))
	err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		_, err := other.Execute(ctx, tx, "guest:s1", "effects.create", key, idempotency.RequestHash([]byte("secret-qr-token")), time.Hour,
			func() (int, []byte, error) { t.Fatal("must not execute"); return 0, nil, nil })
		return err
	})
	if err == nil {
		t.Fatal("wrong key opened the stored response")
	}
}

func TestParseKey(t *testing.T) {
	for raw, ok := range map[string]bool{
		key:                                    true,
		"3B241101-E2BB-4255-8CAF-4136C566A962": true,
		"":                                     false,
		"not-a-uuid":                           false,
		"3b241101e2bb42558caf4136c566a962":     false,
		"3b241101-e2bb-4255-8caf-4136c566a96z": false,
	} {
		if _, got := idempotency.ParseKey(raw); got != ok {
			t.Errorf("ParseKey(%q) = %v", raw, got)
		}
	}
}
