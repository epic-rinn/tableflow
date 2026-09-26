package seating

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/epic-rinn/tableflow/src/api/internal/identity"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/audit"
)

// Group is a party-size band.
type Group struct {
	ID       string `json:"id,omitempty"`
	Label    string `json:"label"`
	MinParty int    `json:"min_party"`
	MaxParty int    `json:"max_party"`
}

// Claim describes who holds a table.
type Claim struct {
	Kind          string  `json:"kind"` // hold | visit
	QueueTicketID *string `json:"queue_ticket_id,omitempty"`
	DisplayNumber *int    `json:"display_number,omitempty"`
	VisitID       *string `json:"visit_id,omitempty"`
}

// Table is a dining table with its current claim.
type Table struct {
	ID       string   `json:"id"`
	Label    string   `json:"label"`
	Capacity int      `json:"capacity"`
	Needs    []string `json:"needs"`
	State    string   `json:"state"`
	Active   bool     `json:"active"`
	Version  int      `json:"version"`
	Claim    *Claim   `json:"claim"`
}

func scanTable(row pgx.Row) (Table, error) {
	var t Table
	var ticketID, visitID *string
	var display *int
	if err := row.Scan(&t.ID, &t.Label, &t.Capacity, &t.Needs, &t.State, &t.Active, &t.Version, &ticketID, &display, &visitID); err != nil {
		return Table{}, err
	}
	switch {
	case ticketID != nil:
		t.Claim = &Claim{Kind: "hold", QueueTicketID: ticketID, DisplayNumber: display}
	case visitID != nil:
		t.Claim = &Claim{Kind: "visit", VisitID: visitID}
	}
	return t, nil
}

// Groups returns the branch's current seating groups (any staff role).
func (s *Service) Groups(ctx context.Context, p identity.Principal, branchID string) ([]Group, error) {
	if p.BranchID != branchID {
		return nil, ErrNotFound
	}
	return currentGroups(ctx, s.pool, branchID)
}

func currentGroups(ctx context.Context, db interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}, branchID string) ([]Group, error) {
	rows, err := db.Query(ctx, q("groups_current"), branchID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Group, error) {
		var g Group
		err := r.Scan(&g.ID, &g.Label, &g.MinParty, &g.MaxParty)
		return g, err
	})
}

// validateGroups requires contiguous, non-overlapping bands starting at 1.
func validateGroups(in []Group) ([]Group, error) {
	if len(in) == 0 || len(in) > 10 {
		return nil, &ValidationError{Fields: map[string]string{"groups": "provide 1–10 groups"}}
	}
	next := 1
	out := make([]Group, 0, len(in))
	for i, g := range in {
		g.Label = strings.TrimSpace(g.Label)
		if n := len([]rune(g.Label)); n < 1 || n > 40 {
			return nil, &ValidationError{Fields: map[string]string{fmt.Sprintf("groups[%d].label", i): "must be 1–40 characters"}}
		}
		if g.MinParty != next || g.MaxParty < g.MinParty || g.MaxParty > maxParty {
			return nil, &ValidationError{Fields: map[string]string{fmt.Sprintf("groups[%d]", i): fmt.Sprintf("must start at %d and end between min_party and %d", next, maxParty)}}
		}
		next = g.MaxParty + 1
		out = append(out, Group{Label: g.Label, MinParty: g.MinParty, MaxParty: g.MaxParty})
	}
	return out, nil
}

// ReplaceGroups sets new bands (manager). Refused while tickets are active so
// every waiting party's group position stays meaningful.
func (s *Service) ReplaceGroups(ctx context.Context, p identity.Principal, branchID string, groups []Group, requestID string) ([]Group, error) {
	valid, err := validateGroups(groups)
	if err != nil {
		return nil, err
	}
	var out []Group
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		actor, err := s.staffActor(ctx, tx, p, branchID, identity.RoleManager)
		if err != nil {
			return err
		}
		var id string
		if err := tx.QueryRow(ctx, q("branch_update_lock"), branchID).Scan(&id); err != nil {
			return err
		}
		var active bool
		if err := tx.QueryRow(ctx, q("queue_active_exists"), branchID).Scan(&active); err != nil {
			return err
		}
		if active {
			return ErrQueueActive
		}
		if _, err := tx.Exec(ctx, q("groups_retire"), branchID); err != nil {
			return err
		}
		for _, g := range valid {
			if _, err := tx.Exec(ctx, q("group_insert"), branchID, g.Label, g.MinParty, g.MaxParty); err != nil {
				return err
			}
		}
		if err := audit.Record(ctx, tx, audit.Event{BranchID: branchID, ActorStaffID: &actor.StaffID, Action: "seating_groups.replaced",
			ResourceType: "branch", ResourceID: branchID, RequestID: requestID, Details: map[string]any{"groups": valid}}); err != nil {
			return err
		}
		out, err = currentGroups(ctx, tx, branchID)
		return err
	})
	return out, err
}

// Tables returns the whole floor with claims (any staff role).
func (s *Service) Tables(ctx context.Context, p identity.Principal, branchID string) ([]Table, error) {
	if p.BranchID != branchID {
		return nil, ErrNotFound
	}
	rows, err := s.pool.Query(ctx, q("tables_board"), branchID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Table, error) { return scanTable(r) })
}

// TableInput is a create/update request.
type TableInput struct {
	Label    string
	Capacity int
	Needs    []string
	Active   bool
}

func validateTable(in TableInput) (TableInput, error) {
	fields := map[string]string{}
	in.Label = strings.TrimSpace(in.Label)
	if n := len([]rune(in.Label)); n < 1 || n > 20 {
		fields["label"] = "must be 1–20 characters"
	}
	if in.Capacity < 1 || in.Capacity > maxParty {
		fields["capacity"] = fmt.Sprintf("must be 1–%d", maxParty)
	}
	needs, ok := normalizeNeeds(in.Needs)
	if !ok {
		fields["needs"] = "allowed: accessible, high_chair"
	}
	in.Needs = needs
	if len(fields) > 0 {
		return in, &ValidationError{Fields: fields}
	}
	return in, nil
}

// CreateTable adds a table (manager).
func (s *Service) CreateTable(ctx context.Context, p identity.Principal, branchID string, in TableInput, requestID string) (Table, error) {
	in, err := validateTable(in)
	if err != nil {
		return Table{}, err
	}
	var t Table
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		actor, err := s.staffActor(ctx, tx, p, branchID, identity.RoleManager)
		if err != nil {
			return err
		}
		var n int
		if err := tx.QueryRow(ctx, "SELECT count(*) FROM dining_tables WHERE branch_id = $1", branchID).Scan(&n); err != nil {
			return err
		}
		if n >= maxTables {
			return ErrTooManyTables
		}
		var id string
		err = tx.QueryRow(ctx, q("table_insert"), branchID, in.Label, in.Capacity, in.Needs).Scan(&id)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrLabelTaken
		}
		if err != nil {
			return err
		}
		if err := audit.Record(ctx, tx, audit.Event{BranchID: branchID, ActorStaffID: &actor.StaffID, Action: "table.created",
			ResourceType: "table", ResourceID: id, RequestID: requestID}); err != nil {
			return err
		}
		t, err = scanTable(tx.QueryRow(ctx, q("table_get"), id))
		return err
	})
	return t, err
}

// UpdateTable edits a table (manager). A claimed table cannot be deactivated
// or changed so that its current party no longer fits.
func (s *Service) UpdateTable(ctx context.Context, p identity.Principal, tableID string, expectedVersion int, in TableInput, requestID string) (Table, error) {
	in, err := validateTable(in)
	if err != nil {
		return Table{}, err
	}
	var t Table
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		actor, err := s.staffActor(ctx, tx, p, p.BranchID, identity.RoleManager)
		if err != nil {
			return err
		}
		var label, state string
		var capacity, version int
		var needs []string
		var active bool
		err = tx.QueryRow(ctx, q("table_lock"), tableID, actor.BranchID).Scan(&label, &capacity, &needs, &state, &active, &version)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if version != expectedVersion {
			return ErrVersionConflict
		}
		var claimed bool
		if err := tx.QueryRow(ctx, q("claim_exists"), tableID).Scan(&claimed); err != nil {
			return err
		}
		if claimed || state != "available" {
			if !in.Active || in.Capacity < capacity || !containsAll(in.Needs, needs) {
				return ErrTableUnavailable
			}
		}
		var taken bool
		if err := tx.QueryRow(ctx, q("table_label_taken"), actor.BranchID, in.Label, tableID).Scan(&taken); err != nil {
			return err
		}
		if taken {
			return ErrLabelTaken
		}
		if err := execOne(ctx, tx, "table_update", tableID, in.Label, in.Capacity, in.Needs, in.Active); err != nil {
			return err
		}
		if err := audit.Record(ctx, tx, audit.Event{BranchID: actor.BranchID, ActorStaffID: &actor.StaffID, Action: "table.updated",
			ResourceType: "table", ResourceID: tableID, RequestID: requestID}); err != nil {
			return err
		}
		t, err = scanTable(tx.QueryRow(ctx, q("table_get"), tableID))
		return err
	})
	return t, err
}

func containsAll(have, want []string) bool {
	for _, w := range want {
		found := false
		for _, h := range have {
			if h == w {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// DefaultGroups are the pilot's editable party-size bands.
var DefaultGroups = []Group{{Label: "1–2", MinParty: 1, MaxParty: 2}, {Label: "3–4", MinParty: 3, MaxParty: 4}, {Label: "5–6", MinParty: 5, MaxParty: 6}}

// InsertDefaultGroups gives a newly bootstrapped branch the default bands
// when it has none.
func InsertDefaultGroups(ctx context.Context, pool interface {
	Begin(context.Context) (pgx.Tx, error)
}, branchID string) error {
	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		existing, err := currentGroups(ctx, tx, branchID)
		if err != nil || len(existing) > 0 {
			return err
		}
		for _, g := range DefaultGroups {
			if _, err := tx.Exec(ctx, q("group_insert"), branchID, g.Label, g.MinParty, g.MaxParty); err != nil {
				return err
			}
		}
		return nil
	})
}
