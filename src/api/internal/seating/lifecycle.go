package seating

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/epic-rinn/tableflow/src/api/internal/access"
	"github.com/epic-rinn/tableflow/src/api/internal/identity"
	"github.com/epic-rinn/tableflow/src/api/internal/ordering"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/audit"
)

type lockedVisit struct {
	branchID string
	tableID  string
	state    string
	party    int
	needs    []string
	version  int
}

// lockVisitWithTables locks the visit's current table (and extra tables) in
// ID order, then the visit, and verifies the visit did not move in between.
// Tables are locked before visits (data-model lock order); a visit that
// moved concurrently yields ErrVersionConflict for the client to retry.
func lockVisitWithTables(ctx context.Context, tx pgx.Tx, visitID, branchID string, extra ...string) (lockedVisit, map[string]lockedTable, error) {
	var peekBranch, peekTable string
	err := tx.QueryRow(ctx, q("visit_peek"), visitID).Scan(&peekBranch, &peekTable)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && peekBranch != branchID) {
		return lockedVisit{}, nil, ErrNotFound
	}
	if err != nil {
		return lockedVisit{}, nil, err
	}
	ids := append([]string{peekTable}, extra...)
	slices.Sort(ids)
	ids = slices.Compact(ids)
	tables := map[string]lockedTable{}
	for _, id := range ids {
		t, err := lockTable(ctx, tx, id, branchID)
		if err != nil {
			return lockedVisit{}, nil, err
		}
		tables[id] = t
	}
	var v lockedVisit
	if err := tx.QueryRow(ctx, q("visit_lock"), visitID).Scan(&v.branchID, &v.tableID, &v.state, &v.party, &v.needs, &v.version); err != nil {
		return lockedVisit{}, nil, err
	}
	if v.tableID != peekTable {
		return lockedVisit{}, nil, ErrVersionConflict
	}
	return v, tables, nil
}

func (s *Service) floorActor(ctx context.Context, tx pgx.Tx, p identity.Principal) (identity.Principal, error) {
	return s.staffActor(ctx, tx, p, p.BranchID, identity.RoleHost, identity.RoleManager)
}

// Move transfers a visit's claim to another table, keeping the visit, its
// access and (later) orders and bill; the old table needs cleaning (SEA-003).
func (s *Service) Move(ctx context.Context, tx pgx.Tx, p identity.Principal, visitID, tableID string, expectedVersion, expectedTableVersion int) (Visit, error) {
	actor, err := s.floorActor(ctx, tx, p)
	if err != nil {
		return Visit{}, err
	}
	v, tables, err := lockVisitWithTables(ctx, tx, visitID, actor.BranchID, tableID)
	if err != nil {
		return Visit{}, err
	}
	if v.version != expectedVersion {
		return Visit{}, ErrVersionConflict
	}
	if v.state != "open" && v.state != "settling" && v.state != "paid" {
		return Visit{}, ErrVisitState
	}
	if tableID == v.tableID {
		return Visit{}, &ValidationError{Fields: map[string]string{"table_id": "the party is already at this table"}}
	}
	dest := tables[tableID]
	if dest.version != expectedTableVersion {
		return Visit{}, ErrVersionConflict
	}
	if !dest.active || dest.state != "available" {
		return Visit{}, ErrTableUnavailable
	}
	if !compatible(dest.capacity, dest.needs, v.party, v.needs) {
		return Visit{}, ErrTableIncompatible
	}
	if err := execOne(ctx, tx, "claim_move", visitID, tableID, v.tableID); err != nil {
		return Visit{}, ErrTableUnavailable
	}
	for _, step := range []struct {
		name string
		args []any
	}{
		{"visit_move", []any{visitID, tableID}},
		{"table_set_state", []any{v.tableID, "cleaning"}},
		{"table_set_state", []any{tableID, "occupied"}},
	} {
		if err := execOne(ctx, tx, step.name, step.args...); err != nil {
			return Visit{}, err
		}
	}
	return visitView(ctx, tx, visitID)
}

// endVisit releases the claim, sends the table to cleaning, ends the visit
// and revokes its dining access.
func endVisit(ctx context.Context, tx pgx.Tx, visitID, tableID, state string, reason *string) error {
	if _, err := tx.Exec(ctx, q("claim_delete"), tableID); err != nil {
		return err
	}
	if err := execOne(ctx, tx, "table_set_state", tableID, "cleaning"); err != nil {
		return err
	}
	if err := execOne(ctx, tx, "visit_end", visitID, state, reason); err != nil {
		return err
	}
	if err := access.RevokeCapability(ctx, tx, access.KindVisit, visitID); err != nil && !errors.Is(err, access.ErrNoCapability) {
		return err
	}
	return nil
}

// Depart ends a paid visit (SEA-004): paid parties keep their table until
// staff record departure.
func (s *Service) Depart(ctx context.Context, tx pgx.Tx, p identity.Principal, visitID string, expectedVersion int) (Visit, error) {
	actor, err := s.floorActor(ctx, tx, p)
	if err != nil {
		return Visit{}, err
	}
	v, _, err := lockVisitWithTables(ctx, tx, visitID, actor.BranchID)
	if err != nil {
		return Visit{}, err
	}
	if v.version != expectedVersion {
		return Visit{}, ErrVersionConflict
	}
	if v.state != "paid" {
		return Visit{}, ErrVisitState
	}
	if err := endVisit(ctx, tx, visitID, v.tableID, "departed", nil); err != nil {
		return Visit{}, err
	}
	return visitView(ctx, tx, visitID)
}

// CloseEmpty closes an open visit without chargeable orders, with an audited
// reason (SEA-004); visits with chargeable order lines are refused.
func (s *Service) CloseEmpty(ctx context.Context, tx pgx.Tx, p identity.Principal, visitID string, expectedVersion int, reason, requestID string) (Visit, error) {
	r, ok := validReason(strings.TrimSpace(reason))
	if !ok {
		return Visit{}, &ValidationError{Fields: map[string]string{"reason": "must be 1–500 characters"}}
	}
	actor, err := s.floorActor(ctx, tx, p)
	if err != nil {
		return Visit{}, err
	}
	v, _, err := lockVisitWithTables(ctx, tx, visitID, actor.BranchID)
	if err != nil {
		return Visit{}, err
	}
	if v.version != expectedVersion {
		return Visit{}, ErrVersionConflict
	}
	if v.state != "open" {
		return Visit{}, ErrVisitState
	}
	// SEA-004: only visits without chargeable orders close empty. The visit
	// row is locked, so no order can be added concurrently.
	if n, err := ordering.ChargeableLines(ctx, tx, visitID); err != nil {
		return Visit{}, err
	} else if n > 0 {
		return Visit{}, ErrVisitHasOrders
	}
	if err := endVisit(ctx, tx, visitID, v.tableID, "closed", &r); err != nil {
		return Visit{}, err
	}
	if err := audit.Record(ctx, tx, audit.Event{BranchID: v.branchID, ActorStaffID: &actor.StaffID, Action: "visit.closed_empty",
		ResourceType: "visit", ResourceID: visitID, Reason: &r, RequestID: requestID}); err != nil {
		return Visit{}, err
	}
	return visitView(ctx, tx, visitID)
}

// Ready makes a cleaned, unclaimed table available again.
func (s *Service) Ready(ctx context.Context, tx pgx.Tx, p identity.Principal, tableID string, expectedVersion int) (Table, error) {
	actor, err := s.floorActor(ctx, tx, p)
	if err != nil {
		return Table{}, err
	}
	t, err := lockTable(ctx, tx, tableID, actor.BranchID)
	if err != nil {
		return Table{}, err
	}
	if t.version != expectedVersion {
		return Table{}, ErrVersionConflict
	}
	var claimed bool
	if err := tx.QueryRow(ctx, q("claim_exists"), tableID).Scan(&claimed); err != nil {
		return Table{}, err
	}
	if t.state != "cleaning" || claimed {
		return Table{}, ErrTableUnavailable
	}
	if err := execOne(ctx, tx, "table_set_state", tableID, "available"); err != nil {
		return Table{}, err
	}
	return scanTable(tx.QueryRow(ctx, q("table_get"), tableID))
}

// RotateAccess replaces the dining QR; every guest session derived from the
// old one stops working (ACC-001, ACC-A2).
func (s *Service) RotateAccess(ctx context.Context, tx pgx.Tx, p identity.Principal, visitID string, expectedVersion int, reason, requestID string) (SeatResult, error) {
	r, ok := validReason(strings.TrimSpace(reason))
	if !ok {
		return SeatResult{}, &ValidationError{Fields: map[string]string{"reason": "must be 1–500 characters"}}
	}
	actor, err := s.floorActor(ctx, tx, p)
	if err != nil {
		return SeatResult{}, err
	}
	var v lockedVisit
	err = tx.QueryRow(ctx, q("visit_lock"), visitID).Scan(&v.branchID, &v.tableID, &v.state, &v.party, &v.needs, &v.version)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && v.branchID != actor.BranchID) {
		return SeatResult{}, ErrNotFound
	}
	if err != nil {
		return SeatResult{}, err
	}
	if v.version != expectedVersion {
		return SeatResult{}, ErrVersionConflict
	}
	// Only before payment: payment revokes dining access (BIL-006), and a new
	// QR must never restore it afterwards (BIL-008).
	if v.state != "open" && v.state != "settling" {
		return SeatResult{}, ErrVisitState
	}
	token, _, err := access.RotateCapability(ctx, tx, access.KindVisit, visitID)
	if err != nil {
		return SeatResult{}, err
	}
	if err := execOne(ctx, tx, "visit_bump", visitID); err != nil {
		return SeatResult{}, err
	}
	if err := audit.Record(ctx, tx, audit.Event{BranchID: v.branchID, ActorStaffID: &actor.StaffID, Action: "visit.access_rotated",
		ResourceType: "visit", ResourceID: visitID, Reason: &r, RequestID: requestID}); err != nil {
		return SeatResult{}, err
	}
	var res SeatResult
	res.Dining.Token = token
	res.Visit, err = visitView(ctx, tx, visitID)
	return res, err
}
