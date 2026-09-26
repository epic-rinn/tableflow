package identity

import (
	"context"
	"errors"
	"net/netip"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/epic-rinn/tableflow/src/api/internal/platform/dbtest"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/password"
)

const testPassword = "correct horse battery staple"

// twoManagers returns a service and authenticated principals for managers A and B.
func twoManagers(t *testing.T) (*Service, *pgxpool.Pool, Principal, Principal) {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbtest.NewMigratedDatabase(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	svc := NewService(pool, password.New(2), time.Hour, 12*time.Hour)
	branch, _, act, err := svc.Bootstrap(ctx, "Main", "a@example.com", "A")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Activate(ctx, act.Token, testPassword, "", mustAddr(), "t"); err != nil {
		t.Fatal(err)
	}
	a, _, _, err := svc.Login(ctx, "a@example.com", testPassword, mustAddr())
	if err != nil {
		t.Fatal(err)
	}
	_, inv, err := svc.Invite(ctx, a, branch, "b@example.com", "B", []string{RoleManager}, "t")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Activate(ctx, inv.Token, testPassword, "", mustAddr(), "t"); err != nil {
		t.Fatal(err)
	}
	b, _, _, err := svc.Login(ctx, "b@example.com", testPassword, mustAddr())
	if err != nil {
		t.Fatal(err)
	}
	return svc, pool, a, b
}

func version(t *testing.T, pool *pgxpool.Pool, id string) int {
	t.Helper()
	var v int
	if err := pool.QueryRow(context.Background(), "SELECT version FROM staff_accounts WHERE id = $1", id).Scan(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

// TestInFlightMutationAfterRevocationFails (ACC-A3): A authenticated at
// request start, then B revoked A before A's transaction began. A's
// mutation must fail on re-validation, not run with stale authority.
func TestInFlightMutationAfterRevocationFails(t *testing.T) {
	svc, pool, a, b := twoManagers(t)
	ctx := context.Background()
	if _, err := svc.SetRoles(ctx, b, a.StaffID, version(t, pool, a.StaffID), []string{RoleHost}, "t"); err != nil {
		t.Fatalf("B demotes A: %v", err)
	}
	// A still holds a Principal from its earlier authentication.
	_, err := svc.SetRoles(ctx, a, b.StaffID, version(t, pool, b.StaffID), []string{RoleHost}, "t")
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("stale principal mutation: %v", err)
	}
	if got := version(t, pool, b.StaffID); got != 2 {
		t.Fatalf("B was modified by a revoked session (version %d)", got)
	}
	_, _, err = svc.Invite(ctx, a, a.BranchID, "c@example.com", "C", []string{RoleHost}, "t")
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("stale principal invite: %v", err)
	}
}

// TestRevocationWaitsForInFlightMutation (ACC-A3): while A's transaction
// holds its re-validated session (FOR SHARE), B's revocation of A blocks
// until A commits, so A's already-authorised write is never interleaved
// with a half-applied revocation.
func TestRevocationWaitsForInFlightMutation(t *testing.T) {
	svc, pool, a, b := twoManagers(t)
	ctx := context.Background()
	aVersion := version(t, pool, a.StaffID)

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := svc.revalidate(ctx, tx, a); err != nil {
		t.Fatalf("revalidate A: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		_, err := svc.SetRoles(ctx, b, a.StaffID, aVersion, []string{RoleHost}, "t")
		done <- err
	}()
	select {
	case err := <-done:
		t.Fatalf("revocation did not wait for the in-flight transaction: %v", err)
	case <-time.After(300 * time.Millisecond):
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("revocation after commit: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("revocation still blocked after commit")
	}
	if _, err := svc.revalidate(ctx, mustTx(t, pool), a); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("A still valid after revocation: %v", err)
	}
}

func mustTx(t *testing.T, pool *pgxpool.Pool) pgx.Tx {
	t.Helper()
	tx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	return tx
}

func mustAddr() netip.Addr { return netip.MustParseAddr("192.0.2.1") }

// TestAdminLockOrderPreventsDeadlock: two managers acting on each other at
// the same moment. Each transaction pauses after re-validating its actor;
// the branch lock must stop the second from reaching that point, so the
// pair serialises (one succeeds, the other sees its revocation) instead of
// deadlocking on crossed account locks.
func TestAdminLockOrderPreventsDeadlock(t *testing.T) {
	svc, _, a, b := twoManagers(t)
	ctx := context.Background()
	ready := make(chan string, 2)
	release := make(chan struct{})
	run := func(actor Principal, target string) error {
		return svc.adminTx(ctx, actor, actor.BranchID, func(tx pgx.Tx, _ Principal) error {
			ready <- actor.StaffID
			<-release
			if _, err := lockTarget(ctx, tx, target, actor.BranchID, nil); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, q("revoke_account_sessions"), target); err != nil {
				return err
			}
			return execOne(ctx, tx, q("bump_account"), target)
		})
	}
	errs := make(chan error, 2)
	go func() { errs <- run(a, b.StaffID) }()
	go func() { errs <- run(b, a.StaffID) }()

	<-ready
	select {
	case <-ready:
		t.Error("both transactions passed actor re-validation concurrently (no serialising lock)")
	case <-time.After(300 * time.Millisecond):
	}
	close(release)
	var ok, revoked int
	for range 2 {
		err := <-errs
		switch {
		case err == nil:
			ok++
		case errors.Is(err, ErrUnauthenticated):
			revoked++
		default:
			t.Errorf("unexpected error (deadlock?): %v", err)
		}
	}
	if ok != 1 || revoked != 1 {
		t.Fatalf("ok=%d revoked=%d, want 1 and 1", ok, revoked)
	}
}
