package seating

import (
	"context"
	"encoding/base64"
	"errors"
	"math"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/epic-rinn/tableflow/src/api/internal/access"
	"github.com/epic-rinn/tableflow/src/api/internal/identity"
)

// Actor is whoever issues a queue command: staff, a guest session derived
// from a queue capability, or an anonymous bootstrap session (join only).
type Actor struct {
	Staff     *identity.Principal
	Guest     *access.Guest
	Anonymous *access.Anonymous
}

// Ticket is the queue ticket view. Guests only ever see their own ticket.
type Ticket struct {
	ID            string     `json:"id"`
	BranchID      string     `json:"branch_id"`
	DisplayNumber int        `json:"display_number"`
	BusinessDate  string     `json:"business_date"`
	State         string     `json:"state"`
	PartySize     int        `json:"party_size"`
	Needs         []string   `json:"needs"`
	Source        string     `json:"source"`
	Group         *Group     `json:"seating_group"`
	PartiesAhead  *int       `json:"parties_ahead"`
	CalledUntil   *time.Time `json:"called_until"`
	CalledTable   *string    `json:"called_table_label"`
	Overdue       bool       `json:"overdue"`
	Version       int        `json:"version"`
	CreatedAt     time.Time  `json:"created_at"`
	ServerTime    time.Time  `json:"server_time"`
	joinOrder     int64
}

func scanTicketBase(row interface{ Scan(...any) error }, t *Ticket, extra ...any) error {
	var gid, glabel *string
	dest := []any{&t.ID, &t.BranchID, &t.DisplayNumber, &t.BusinessDate, &t.State, &t.PartySize, &t.Needs, &t.Source,
		&gid, &glabel, &t.CalledUntil, &t.CalledTable, &t.Version, &t.CreatedAt}
	if err := row.Scan(append(dest, extra...)...); err != nil {
		return err
	}
	if gid != nil {
		t.Group = &Group{ID: *gid, Label: *glabel}
	}
	now := time.Now().UTC()
	t.ServerTime = now
	t.Overdue = t.State == "called" && t.CalledUntil != nil && t.CalledUntil.Before(now)
	return nil
}

func ticketView(ctx context.Context, db interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, id string) (Ticket, error) {
	var t Ticket
	var ahead *int
	err := scanTicketBase(db.QueryRow(ctx, q("ticket_view"), id), &t, &ahead)
	if errors.Is(err, pgx.ErrNoRows) {
		return Ticket{}, ErrNotFound
	}
	t.PartiesAhead = ahead
	return t, err
}

// JoinInput is a queue join request.
type JoinInput struct {
	PartySize int      `json:"party_size"`
	Needs     []string `json:"needs"`
}

// JoinResult carries the ticket and its one-time tracking token.
type JoinResult struct {
	Ticket   Ticket `json:"ticket"`
	Tracking struct {
		Token string `json:"token"`
	} `json:"tracking"`
}

// CheckJoinLimits throttles guest joins per anonymous session and address
// (outside the transaction, so attempts count even when a join fails).
func (s *Service) CheckJoinLimits(ctx context.Context, a access.Anonymous, ipKey string) error {
	if err := s.limiter.Hit(ctx, "queue-join:anon:"+a.ID, joinsPerSession, joinWindow); err != nil {
		return err
	}
	return s.limiter.Hit(ctx, ipKey, joinsPerIP, joinWindow)
}

// Join creates a ticket and its tracking capability (QUE-001) in tx.
func (s *Service) Join(ctx context.Context, tx pgx.Tx, actor Actor, branchID string, in JoinInput) (JoinResult, error) {
	source := defaultJoinSource
	if actor.Staff != nil {
		if _, err := s.staffActor(ctx, tx, *actor.Staff, branchID, identity.RoleHost, identity.RoleManager); err != nil {
			return JoinResult{}, err
		}
		source = "staff"
	} else if actor.Anonymous == nil {
		return JoinResult{}, ErrForbidden
	}
	fields := map[string]string{}
	if in.PartySize < 1 || in.PartySize > maxParty {
		fields["party_size"] = "must be 1–50"
	}
	needs, ok := normalizeNeeds(in.Needs)
	if !ok {
		fields["needs"] = "allowed: accessible, high_chair"
	}
	if len(fields) > 0 {
		return JoinResult{}, &ValidationError{Fields: fields}
	}
	var tz string
	var holdMinutes int
	err := tx.QueryRow(ctx, q("branch_share"), branchID).Scan(&tz, &holdMinutes)
	if errors.Is(err, pgx.ErrNoRows) {
		return JoinResult{}, ErrNotFound
	}
	if err != nil {
		return JoinResult{}, err
	}
	var groupID *string
	var gid string
	err = tx.QueryRow(ctx, q("group_for_party"), branchID, in.PartySize).Scan(&gid)
	switch {
	case err == nil:
		groupID = &gid
	case errors.Is(err, pgx.ErrNoRows):
		if source != "staff" {
			return JoinResult{}, ErrPartyNeedsStaff
		}
	default:
		return JoinResult{}, err
	}
	var businessDate time.Time
	var number int
	if err := tx.QueryRow(ctx, q("counter_next"), branchID, tz).Scan(&businessDate, &number); err != nil {
		return JoinResult{}, err
	}
	var id string
	if err := tx.QueryRow(ctx, q("ticket_insert"), branchID, businessDate, number, in.PartySize, needs, groupID, source).Scan(&id); err != nil {
		return JoinResult{}, err
	}
	token, err := access.IssueCapability(ctx, tx, branchID, access.KindQueue, id, nil)
	if err != nil {
		return JoinResult{}, err
	}
	var res JoinResult
	res.Tracking.Token = token
	res.Ticket, err = ticketView(ctx, tx, id)
	return res, err
}

// GetTicket returns a ticket to its own guest or to staff of its branch;
// anyone else gets ErrNotFound.
func (s *Service) GetTicket(ctx context.Context, actor Actor, id string) (Ticket, error) {
	t, err := ticketView(ctx, s.pool, id)
	if err != nil {
		return Ticket{}, err
	}
	switch {
	case actor.Staff != nil && actor.Staff.BranchID == t.BranchID:
		return t, nil
	case actor.Guest != nil && actor.Guest.Kind == access.KindQueue && actor.Guest.ResourceID == t.ID:
		return t, nil
	}
	return Ticket{}, ErrNotFound
}

// BoardPage is one page of active tickets.
type BoardPage struct {
	Items      []Ticket  `json:"items"`
	NextCursor *string   `json:"next_cursor"`
	ServerTime time.Time `json:"server_time"`
}

// Board lists waiting/called tickets oldest first (host, manager).
func (s *Service) Board(ctx context.Context, p identity.Principal, branchID, state, cursor string, limit int) (BoardPage, error) {
	if p.BranchID != branchID {
		return BoardPage{}, ErrNotFound
	}
	if !p.Has(identity.RoleHost) && !p.Has(identity.RoleManager) {
		return BoardPage{}, ErrForbidden
	}
	var stateArg *string
	if state != "" {
		if state != "waiting" && state != "called" {
			return BoardPage{}, &ValidationError{Fields: map[string]string{"state": "must be waiting or called"}}
		}
		stateArg = &state
	}
	if limit <= 0 {
		limit = boardPageDefault
	}
	limit = min(limit, boardPageMax)
	after := int64(0)
	if cursor != "" {
		b, err := base64.RawURLEncoding.DecodeString(cursor)
		if err != nil {
			return BoardPage{}, errInvalidCursor
		}
		after, err = strconv.ParseInt(string(b), 10, 64)
		if err != nil || after < 0 {
			return BoardPage{}, errInvalidCursor
		}
	}
	rows, err := s.pool.Query(ctx, q("queue_board"), branchID, stateArg, after, limit+1)
	if err != nil {
		return BoardPage{}, err
	}
	items, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Ticket, error) {
		var t Ticket
		err := scanTicketBase(r, &t, &t.joinOrder)
		return t, err
	})
	if err != nil {
		return BoardPage{}, err
	}
	page := BoardPage{Items: items, ServerTime: time.Now().UTC()}
	if len(items) > limit {
		page.Items = items[:limit]
		next := base64.RawURLEncoding.EncodeToString([]byte(strconv.FormatInt(page.Items[limit-1].joinOrder, 10)))
		page.NextCursor = &next
	}
	return page, nil
}

var errInvalidCursor = &ValidationError{Fields: map[string]string{"cursor": "invalid cursor"}}

type lockedTicket struct {
	branchID    string
	state       string
	party       int
	needs       []string
	calledTable *string
	version     int
	joinOrder   int64
}

func lockTicket(ctx context.Context, tx pgx.Tx, id string) (lockedTicket, error) {
	var t lockedTicket
	err := tx.QueryRow(ctx, q("ticket_lock"), id).Scan(&t.branchID, &t.state, &t.party, &t.needs, &t.calledTable, &t.version, &t.joinOrder)
	if errors.Is(err, pgx.ErrNoRows) {
		return t, ErrNotFound
	}
	return t, err
}

// authorizeStaff re-validates a staff actor (host/manager) before any
// domain lock (principal rows first).
func (s *Service) authorizeStaff(ctx context.Context, tx pgx.Tx, actor Actor) (*identity.Principal, error) {
	if actor.Staff == nil {
		return nil, nil
	}
	p, err := s.staffActor(ctx, tx, *actor.Staff, actor.Staff.BranchID, identity.RoleHost, identity.RoleManager)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// authorizeGuestForTicket re-validates a guest AFTER the ticket row is
// locked: the capability is a child of its ticket, and seating/cancel update
// it after locking the ticket, so the ticket must always be locked first.
func authorizeGuestForTicket(ctx context.Context, tx pgx.Tx, actor Actor, ticketID string) error {
	if actor.Guest == nil {
		return ErrForbidden
	}
	g, err := access.RevalidateGuest(ctx, tx, *actor.Guest)
	if err != nil {
		return err
	}
	if g.Kind != access.KindQueue || g.ResourceID != ticketID {
		return ErrNotFound
	}
	return nil
}

// releaseHold deletes a called ticket's claim and frees its table. The
// ticket row must already be locked (ticket → table order).
func releaseHold(ctx context.Context, tx pgx.Tx, branchID string, tableID *string) error {
	if tableID == nil {
		return nil
	}
	var label, state string
	var capacity, version int
	var needs []string
	var active bool
	if err := tx.QueryRow(ctx, q("table_lock"), *tableID, branchID).Scan(&label, &capacity, &needs, &state, &active, &version); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, q("claim_delete"), *tableID); err != nil {
		return err
	}
	return execOne(ctx, tx, "table_set_state", *tableID, "available")
}

// Cancel ends a waiting or called ticket (QUE-004), releasing any hold.
func (s *Service) Cancel(ctx context.Context, tx pgx.Tx, actor Actor, ticketID string, expectedVersion int) (Ticket, error) {
	staff, err := s.authorizeStaff(ctx, tx, actor)
	if err != nil {
		return Ticket{}, err
	}
	t, err := lockTicket(ctx, tx, ticketID)
	if err != nil {
		return Ticket{}, err
	}
	if staff != nil && staff.BranchID != t.branchID {
		return Ticket{}, ErrNotFound
	}
	if staff == nil {
		if err := authorizeGuestForTicket(ctx, tx, actor, ticketID); err != nil {
			return Ticket{}, err
		}
	}
	if t.version != expectedVersion {
		return Ticket{}, ErrVersionConflict
	}
	if t.state != "waiting" && t.state != "called" {
		return Ticket{}, ErrTicketState
	}
	if err := releaseHold(ctx, tx, t.branchID, t.calledTable); err != nil {
		return Ticket{}, err
	}
	if err := execOne(ctx, tx, "ticket_end", ticketID, "cancelled"); err != nil {
		return Ticket{}, err
	}
	if err := access.ExpireCapability(ctx, tx, access.KindQueue, ticketID, trackingAfterEnd); err != nil {
		return Ticket{}, err
	}
	return ticketView(ctx, tx, ticketID)
}

// maxJoinOrder stands in for "a walk-in joined after everyone".
const maxJoinOrder = math.MaxInt64
