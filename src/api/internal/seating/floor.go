package seating

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/epic-rinn/tableflow/src/api/internal/access"
	"github.com/epic-rinn/tableflow/src/api/internal/identity"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/audit"
)

type lockedTable struct {
	id       string
	label    string
	capacity int
	needs    []string
	state    string
	active   bool
	version  int
}

func lockTable(ctx context.Context, tx pgx.Tx, id, branchID string) (lockedTable, error) {
	t := lockedTable{id: id}
	err := tx.QueryRow(ctx, q("table_lock"), id, branchID).Scan(&t.label, &t.capacity, &t.needs, &t.state, &t.active, &t.version)
	if errors.Is(err, pgx.ErrNoRows) {
		return t, ErrNotFound
	}
	return t, err
}

// fairness enforces "oldest compatible party first": serving someone else
// at this table while an older compatible party waits is a bypass that needs
// a manager and a reason, and is audited.
func fairness(ctx context.Context, tx pgx.Tx, actor identity.Principal, branchID string, joinOrder int64, table lockedTable,
	reason *string, subject, requestID string) error {
	var older bool
	if err := tx.QueryRow(ctx, q("older_compatible"), branchID, joinOrder, table.capacity, table.needs).Scan(&older); err != nil {
		return err
	}
	if !older {
		return nil
	}
	if reason == nil {
		return ErrBypass
	}
	if !actor.Has(identity.RoleManager) {
		return ErrForbidden
	}
	return audit.Record(ctx, tx, audit.Event{BranchID: branchID, ActorStaffID: &actor.StaffID, Action: "queue.bypass",
		ResourceType: "table", ResourceID: table.id, Reason: reason, RequestID: requestID,
		Details: map[string]any{"subject": subject}})
}

func normalizeReason(r *string) (*string, error) {
	if r == nil {
		return nil, nil
	}
	v, ok := validReason(strings.TrimSpace(*r))
	if !ok {
		return nil, &ValidationError{Fields: map[string]string{"override_reason": "must be 1–500 characters"}}
	}
	return &v, nil
}

// Call holds a table for a waiting ticket until a return deadline (QUE-003).
func (s *Service) Call(ctx context.Context, tx pgx.Tx, p identity.Principal, ticketID, tableID string, expectedVersion int,
	overrideReason *string, requestID string) (Ticket, error) {
	reason, err := normalizeReason(overrideReason)
	if err != nil {
		return Ticket{}, err
	}
	actor, err := s.staffActor(ctx, tx, p, p.BranchID, identity.RoleHost, identity.RoleManager)
	if err != nil {
		return Ticket{}, err
	}
	t, err := lockTicket(ctx, tx, ticketID)
	if err != nil || t.branchID != actor.BranchID {
		return Ticket{}, errOr(err, ErrNotFound)
	}
	if t.version != expectedVersion {
		return Ticket{}, ErrVersionConflict
	}
	if t.state != "waiting" {
		return Ticket{}, ErrTicketState
	}
	table, err := lockTable(ctx, tx, tableID, t.branchID)
	if err != nil {
		return Ticket{}, err
	}
	if !table.active || table.state != "available" {
		return Ticket{}, ErrTableUnavailable
	}
	if !compatible(table.capacity, table.needs, t.party, t.needs) {
		return Ticket{}, ErrTableIncompatible
	}
	if err := fairness(ctx, tx, actor, t.branchID, t.joinOrder, table, reason, "ticket:"+ticketID, requestID); err != nil {
		return Ticket{}, err
	}
	if _, err := tx.Exec(ctx, q("claim_insert"), tableID, t.branchID, ticketID, nil); err != nil {
		if isUniqueViolation(err) {
			return Ticket{}, ErrTableUnavailable
		}
		return Ticket{}, err
	}
	var tz string
	var holdMinutes int
	if err := tx.QueryRow(ctx, "SELECT timezone, call_hold_minutes FROM branches WHERE id = $1", t.branchID).Scan(&tz, &holdMinutes); err != nil {
		return Ticket{}, err
	}
	if err := execOne(ctx, tx, "ticket_call", ticketID, tableID, holdMinutes); err != nil {
		return Ticket{}, err
	}
	if err := execOne(ctx, tx, "table_set_state", tableID, "held"); err != nil {
		return Ticket{}, err
	}
	return ticketView(ctx, tx, ticketID)
}

func errOr(err, fallback error) error {
	if err != nil {
		return err
	}
	return fallback
}

// NoShow ends a called ticket and releases its hold. Deadlines never do this
// automatically; staff decide, avoiding races with arriving guests (QUE-004).
func (s *Service) NoShow(ctx context.Context, tx pgx.Tx, p identity.Principal, ticketID string, expectedVersion int, reason, requestID string) (Ticket, error) {
	r, ok := validReason(strings.TrimSpace(reason))
	if !ok {
		return Ticket{}, &ValidationError{Fields: map[string]string{"reason": "must be 1–500 characters"}}
	}
	actor, err := s.staffActor(ctx, tx, p, p.BranchID, identity.RoleHost, identity.RoleManager)
	if err != nil {
		return Ticket{}, err
	}
	t, err := lockTicket(ctx, tx, ticketID)
	if err != nil || t.branchID != actor.BranchID {
		return Ticket{}, errOr(err, ErrNotFound)
	}
	if t.version != expectedVersion {
		return Ticket{}, ErrVersionConflict
	}
	if t.state != "called" {
		return Ticket{}, ErrTicketState
	}
	if err := releaseHold(ctx, tx, t.branchID, t.calledTable); err != nil {
		return Ticket{}, err
	}
	if err := execOne(ctx, tx, "ticket_end", ticketID, "no_show"); err != nil {
		return Ticket{}, err
	}
	if err := access.ExpireCapability(ctx, tx, access.KindQueue, ticketID, trackingAfterEnd); err != nil {
		return Ticket{}, err
	}
	if err := audit.Record(ctx, tx, audit.Event{BranchID: t.branchID, ActorStaffID: &actor.StaffID, Action: "queue.no_show",
		ResourceType: "queue_ticket", ResourceID: ticketID, Reason: &r, RequestID: requestID}); err != nil {
		return Ticket{}, err
	}
	return ticketView(ctx, tx, ticketID)
}

// Visit is the dine-in visit view (no member data).
type Visit struct {
	ID         string    `json:"id"`
	BranchID   string    `json:"branch_id"`
	State      string    `json:"state"`
	PartySize  int       `json:"party_size"`
	Needs      []string  `json:"needs"`
	Table      TableRef  `json:"table"`
	Version    int       `json:"version"`
	OpenedAt   time.Time `json:"opened_at"`
	ServerTime time.Time `json:"server_time"`
}

// TableRef identifies a table for display.
type TableRef struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

func visitView(ctx context.Context, db interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, id string) (Visit, error) {
	var v Visit
	err := db.QueryRow(ctx, q("visit_view"), id).
		Scan(&v.ID, &v.BranchID, &v.State, &v.PartySize, &v.Needs, &v.Table.ID, &v.Table.Label, &v.Version, &v.OpenedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Visit{}, ErrNotFound
	}
	v.ServerTime = time.Now().UTC()
	return v, err
}

// GetVisit returns a visit to its dining guests or to staff of its branch.
func (s *Service) GetVisit(ctx context.Context, actor Actor, id string) (Visit, error) {
	v, err := visitView(ctx, s.pool, id)
	if err != nil {
		return Visit{}, err
	}
	switch {
	case actor.Staff != nil && actor.Staff.BranchID == v.BranchID:
		return v, nil
	case actor.Guest != nil && actor.Guest.Kind == access.KindVisit && actor.Guest.ResourceID == v.ID:
		return v, nil
	}
	return Visit{}, ErrNotFound
}

// SeatInput seats a called or waiting ticket, or a walk-in party.
type SeatInput struct {
	BranchID             string   `json:"branch_id"`
	TableID              string   `json:"table_id"`
	QueueTicketID        *string  `json:"queue_ticket_id"`
	PartySize            *int     `json:"party_size"`
	Needs                []string `json:"needs"`
	ExpectedTableVersion int      `json:"expected_table_version"`
	OverrideReason       *string  `json:"override_reason"`
}

// SeatResult carries the visit and its one-time dining token.
type SeatResult struct {
	Visit  Visit `json:"visit"`
	Dining struct {
		Token string `json:"token"`
	} `json:"dining"`
}

// Seat creates a visit, its dining capability and the table claim in one
// transaction (SEA-001/002).
func (s *Service) Seat(ctx context.Context, tx pgx.Tx, p identity.Principal, in SeatInput, requestID string) (SeatResult, error) {
	reason, err := normalizeReason(in.OverrideReason)
	if err != nil {
		return SeatResult{}, err
	}
	actor, err := s.staffActor(ctx, tx, p, in.BranchID, identity.RoleHost, identity.RoleManager)
	if err != nil {
		return SeatResult{}, err
	}
	party, needs := 0, []string{}
	joinOrder := int64(maxJoinOrder)
	var ticket *lockedTicket
	if in.QueueTicketID != nil {
		t, err := lockTicket(ctx, tx, *in.QueueTicketID)
		if err != nil || t.branchID != in.BranchID {
			return SeatResult{}, errOr(err, ErrNotFound)
		}
		switch {
		case t.state == "called" && t.calledTable != nil && *t.calledTable == in.TableID:
		case t.state == "waiting":
		default:
			return SeatResult{}, ErrTicketState
		}
		ticket, party, needs, joinOrder = &t, t.party, t.needs, t.joinOrder
	} else {
		if in.PartySize == nil || *in.PartySize < 1 || *in.PartySize > maxParty {
			return SeatResult{}, &ValidationError{Fields: map[string]string{"party_size": "required for walk-ins; 1–50"}}
		}
		n, ok := normalizeNeeds(in.Needs)
		if !ok {
			return SeatResult{}, &ValidationError{Fields: map[string]string{"needs": "allowed: accessible, high_chair"}}
		}
		party, needs = *in.PartySize, n
	}
	table, err := lockTable(ctx, tx, in.TableID, in.BranchID)
	if err != nil {
		return SeatResult{}, err
	}
	if table.version != in.ExpectedTableVersion {
		return SeatResult{}, ErrVersionConflict
	}
	heldForTicket := ticket != nil && ticket.state == "called"
	if heldForTicket {
		if table.state != "held" {
			return SeatResult{}, ErrTableUnavailable
		}
	} else if !table.active || table.state != "available" {
		return SeatResult{}, ErrTableUnavailable
	}
	if !compatible(table.capacity, table.needs, party, needs) {
		return SeatResult{}, ErrTableIncompatible
	}
	if !heldForTicket {
		subject := "walk-in"
		if ticket != nil {
			subject = "ticket:" + *in.QueueTicketID
		}
		if err := fairness(ctx, tx, actor, in.BranchID, joinOrder, table, reason, subject, requestID); err != nil {
			return SeatResult{}, err
		}
	}
	var visitID string
	if err := tx.QueryRow(ctx, q("visit_insert"), in.BranchID, in.TableID, in.QueueTicketID, party, needs).Scan(&visitID); err != nil {
		if isUniqueViolation(err) {
			return SeatResult{}, ErrTicketState
		}
		return SeatResult{}, err
	}
	if heldForTicket {
		if err := execOne(ctx, tx, "claim_to_visit", in.TableID, visitID, *in.QueueTicketID); err != nil {
			return SeatResult{}, ErrTableUnavailable
		}
	} else if _, err := tx.Exec(ctx, q("claim_insert"), in.TableID, in.BranchID, nil, visitID); err != nil {
		if isUniqueViolation(err) {
			return SeatResult{}, ErrTableUnavailable
		}
		return SeatResult{}, err
	}
	if err := execOne(ctx, tx, "table_set_state", in.TableID, "occupied"); err != nil {
		return SeatResult{}, err
	}
	if ticket != nil {
		if err := execOne(ctx, tx, "ticket_end", *in.QueueTicketID, "seated"); err != nil {
			return SeatResult{}, err
		}
		if err := access.ExpireCapability(ctx, tx, access.KindQueue, *in.QueueTicketID, trackingAfterEnd); err != nil {
			return SeatResult{}, err
		}
	}
	token, err := access.IssueCapability(ctx, tx, in.BranchID, access.KindVisit, visitID, nil)
	if err != nil {
		return SeatResult{}, err
	}
	var res SeatResult
	res.Dining.Token = token
	res.Visit, err = visitView(ctx, tx, visitID)
	return res, err
}
