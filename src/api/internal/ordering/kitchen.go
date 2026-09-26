package ordering

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"github.com/epic-rinn/tableflow/src/api/internal/identity"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/audit"
)

// transition is one allowed line state change (ORD-004/007).
type transition struct {
	roles     []string
	reason    bool // reason required
	financial bool // removes the charge: only while the visit is open
	audited   bool
}

var matrix = map[[2]string]transition{
	{"submitted", "accepted"}:  {roles: []string{identity.RoleKitchen, identity.RoleManager}},
	{"submitted", "rejected"}:  {roles: []string{identity.RoleKitchen, identity.RoleManager}, reason: true, financial: true},
	{"accepted", "preparing"}:  {roles: []string{identity.RoleKitchen, identity.RoleManager}},
	{"preparing", "ready"}:     {roles: []string{identity.RoleKitchen, identity.RoleManager}},
	{"ready", "served"}:        {roles: []string{identity.RoleKitchen, identity.RoleHost, identity.RoleManager}},
	{"submitted", "cancelled"}: {roles: []string{identity.RoleHost, identity.RoleManager}, reason: true, financial: true},
	{"accepted", "cancelled"}:  {roles: []string{identity.RoleHost, identity.RoleManager}, reason: true, financial: true},
	{"preparing", "cancelled"}: {roles: []string{identity.RoleManager}, reason: true, financial: true, audited: true},
	{"ready", "cancelled"}:     {roles: []string{identity.RoleManager}, reason: true, financial: true, audited: true},
	{"served", "cancelled"}:    {roles: []string{identity.RoleManager}, reason: true, financial: true, audited: true},
}

// Transition moves a line through the kitchen workflow (staff only).
func (s *Service) Transition(ctx context.Context, tx pgx.Tx, p identity.Principal, lineID string, expectedVersion int, to string, reason *string, requestID string) (Line, error) {
	actor, err := s.staff.Revalidate(ctx, tx, p)
	if err != nil {
		return Line{}, err
	}
	var visitID string
	if err := tx.QueryRow(ctx, q("line_peek"), lineID).Scan(&visitID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Line{}, ErrNotFound
		}
		return Line{}, err
	}
	var branchID, visitState string
	if err := tx.QueryRow(ctx, q("visit_lock"), visitID).Scan(&branchID, &visitState); err != nil {
		return Line{}, err
	}
	if branchID != actor.BranchID {
		return Line{}, ErrNotFound
	}
	var lineBranch, lineVisit, state string
	var version int
	var total int64
	if err := tx.QueryRow(ctx, q("line_lock"), lineID).Scan(&lineBranch, &lineVisit, &state, &version, &total); err != nil {
		return Line{}, err
	}
	if version != expectedVersion {
		return Line{}, ErrVersionConflict
	}
	t, ok := matrix[[2]string{state, to}]
	if !ok {
		return Line{}, ErrLineState
	}
	if !slices.ContainsFunc(t.roles, actor.Has) {
		return Line{}, ErrForbidden
	}
	var r *string
	if t.reason {
		if reason == nil || strings.TrimSpace(*reason) == "" || utf8.RuneCountInString(*reason) > 500 {
			return Line{}, &ValidationError{Fields: map[string]string{"reason": "a reason of 1–500 characters is required"}}
		}
		v := strings.TrimSpace(*reason)
		r = &v
	}
	if t.financial && visitState != "open" {
		return Line{}, ErrVisitState
	}
	line, err := scanLine(tx.QueryRow(ctx, q("line_transition"), lineID, to, r))
	if err != nil {
		return Line{}, err
	}
	if t.financial {
		if _, err := tx.Exec(ctx, q("bump_bill"), visitID); err != nil {
			return Line{}, err
		}
	}
	if t.audited {
		if err := audit.Record(ctx, tx, audit.Event{BranchID: branchID, ActorStaffID: &actor.StaffID, Action: "order_line.cancelled_late",
			ResourceType: "order_line", ResourceID: lineID, Reason: r, RequestID: requestID,
			Details: map[string]any{"from": state, "amount_satang": total}}); err != nil {
			return Line{}, err
		}
	}
	return line, nil
}

// KitchenLine is an active line with table context.
type KitchenLine struct {
	ID         string       `json:"id"`
	OrderID    string       `json:"order_id"`
	VisitID    string       `json:"visit_id"`
	TableLabel string       `json:"table_label"`
	NameTH     string       `json:"name_th"`
	NameEN     string       `json:"name_en"`
	Options    []OptionSnap `json:"options"`
	Quantity   int          `json:"quantity"`
	Note       *string      `json:"note"`
	State      string       `json:"state"`
	Version    int          `json:"version"`
	CreatedAt  time.Time    `json:"created_at"`
}

// KitchenPage is a page of active lines.
type KitchenPage struct {
	Items      []KitchenLine `json:"items"`
	NextCursor *string       `json:"next_cursor"`
	ServerTime time.Time     `json:"server_time"`
}

// Kitchen lists active lines oldest first (kitchen, host, manager).
func (s *Service) Kitchen(ctx context.Context, p identity.Principal, branchID, state, after string, limit int) (KitchenPage, error) {
	if p.BranchID != branchID {
		return KitchenPage{}, ErrNotFound
	}
	if !p.Has(identity.RoleKitchen) && !p.Has(identity.RoleHost) && !p.Has(identity.RoleManager) {
		return KitchenPage{}, ErrForbidden
	}
	var st *string
	if state != "" {
		if !slices.Contains([]string{"submitted", "accepted", "preparing", "ready"}, state) {
			return KitchenPage{}, &ValidationError{Fields: map[string]string{"state": "must be an active state"}}
		}
		st = &state
	}
	if limit <= 0 {
		limit = 50
	}
	limit = min(limit, pageMax)
	c := cursor{T: time.Unix(0, 0).UTC(), ID: "00000000-0000-0000-0000-000000000000", V: branchID}
	if after != "" {
		b, err := base64.RawURLEncoding.DecodeString(after)
		if err != nil || json.Unmarshal(b, &c) != nil || c.V != branchID {
			return KitchenPage{}, &ValidationError{Fields: map[string]string{"cursor": "invalid cursor"}}
		}
	}
	rows, err := s.pool.Query(ctx, q("kitchen_board"), branchID, st, c.T, c.ID, limit+1)
	if err != nil {
		return KitchenPage{}, err
	}
	items, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (KitchenLine, error) {
		var k KitchenLine
		var opts []byte
		if err := r.Scan(&k.ID, &k.OrderID, &k.VisitID, &k.TableLabel, &k.NameTH, &k.NameEN, &opts, &k.Quantity, &k.Note, &k.State, &k.Version, &k.CreatedAt); err != nil {
			return k, err
		}
		k.Options = []OptionSnap{}
		return k, json.Unmarshal(opts, &k.Options)
	})
	if err != nil {
		return KitchenPage{}, err
	}
	page := KitchenPage{Items: items, ServerTime: time.Now().UTC()}
	if len(items) > limit {
		page.Items = items[:limit]
		last := page.Items[limit-1]
		b, _ := json.Marshal(cursor{T: last.CreatedAt, ID: last.ID, V: branchID})
		next := base64.RawURLEncoding.EncodeToString(b)
		page.NextCursor = &next
	}
	return page, nil
}

// --- assistance (ORD-006) -------------------------------------------------

// Assistance is a help/allergy/checkout request.
type Assistance struct {
	ID             string     `json:"id"`
	VisitID        string     `json:"visit_id"`
	TableLabel     string     `json:"table_label"`
	Topic          string     `json:"topic"`
	Note           *string    `json:"note"`
	State          string     `json:"state"`
	Version        int        `json:"version"`
	CreatedAt      time.Time  `json:"created_at"`
	AcknowledgedAt *time.Time `json:"acknowledged_at"`
	ResolvedAt     *time.Time `json:"resolved_at"`
}

func scanAssistance(r interface{ Scan(...any) error }) (Assistance, error) {
	var a Assistance
	err := r.Scan(&a.ID, &a.VisitID, &a.TableLabel, &a.Topic, &a.Note, &a.State, &a.Version, &a.CreatedAt, &a.AcknowledgedAt, &a.ResolvedAt)
	return a, err
}

// RaiseAssistance creates an outstanding request or returns the existing
// one for the same topic (created=false).
func (s *Service) RaiseAssistance(ctx context.Context, tx pgx.Tx, actor Actor, visitID, topic string, note *string) (Assistance, bool, error) {
	if !slices.Contains([]string{"help", "allergy", "checkout"}, topic) {
		return Assistance{}, false, &ValidationError{Fields: map[string]string{"topic": "must be help, allergy or checkout"}}
	}
	if note != nil {
		n := strings.TrimSpace(*note)
		if utf8.RuneCountInString(n) > maxNoteRunes {
			return Assistance{}, false, &ValidationError{Fields: map[string]string{"note": "at most 500 characters"}}
		}
		note = &n
		if n == "" {
			note = nil
		}
	}
	branchID, state, staff, err := s.lockVisit(ctx, tx, actor, visitID, identity.RoleHost, identity.RoleManager)
	if err != nil {
		return Assistance{}, false, err
	}
	if state != "open" && state != "paid" {
		return Assistance{}, false, ErrVisitState
	}
	by := "guest"
	if staff != nil {
		by = "staff"
	}
	var id string
	created := true
	err = tx.QueryRow(ctx, q("assistance_insert"), branchID, visitID, topic, note, by).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		created = false
		err = tx.QueryRow(ctx, q("assistance_outstanding"), visitID, topic).Scan(&id)
	}
	if err != nil {
		return Assistance{}, false, err
	}
	a, err := scanAssistance(tx.QueryRow(ctx, q("assistance_get"), id))
	return a, created, err
}

// VisitAssistance lists a visit's recent requests (its guests or staff).
func (s *Service) VisitAssistance(ctx context.Context, actor Actor, visitID string) ([]Assistance, error) {
	if err := s.readable(ctx, actor, visitID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, q("assistance_for_visit"), visitID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Assistance, error) { return scanAssistance(r) })
}

// AssistanceBoard lists outstanding requests (host, manager).
func (s *Service) AssistanceBoard(ctx context.Context, p identity.Principal, branchID string) ([]Assistance, error) {
	if p.BranchID != branchID {
		return nil, ErrNotFound
	}
	if !p.Has(identity.RoleHost) && !p.Has(identity.RoleManager) {
		return nil, ErrForbidden
	}
	rows, err := s.pool.Query(ctx, q("assistance_board"), branchID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Assistance, error) { return scanAssistance(r) })
}

// TransitionAssistance acknowledges or resolves a request (host, manager).
func (s *Service) TransitionAssistance(ctx context.Context, tx pgx.Tx, p identity.Principal, id string, expectedVersion int, to string) (Assistance, error) {
	actor, err := s.staff.Revalidate(ctx, tx, p)
	if err != nil {
		return Assistance{}, err
	}
	if !actor.Has(identity.RoleHost) && !actor.Has(identity.RoleManager) {
		return Assistance{}, ErrForbidden
	}
	var branchID, visitID, state string
	var version int
	err = tx.QueryRow(ctx, q("assistance_lock"), id).Scan(&branchID, &visitID, &state, &version)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && branchID != actor.BranchID) {
		return Assistance{}, ErrNotFound
	}
	if err != nil {
		return Assistance{}, err
	}
	if version != expectedVersion {
		return Assistance{}, ErrVersionConflict
	}
	ok := (state == "open" && (to == "acknowledged" || to == "resolved")) || (state == "acknowledged" && to == "resolved")
	if !ok {
		return Assistance{}, ErrLineState
	}
	if _, err := tx.Exec(ctx, q("assistance_transition"), id, to, actor.StaffID); err != nil {
		return Assistance{}, err
	}
	return scanAssistance(tx.QueryRow(ctx, q("assistance_get"), id))
}
