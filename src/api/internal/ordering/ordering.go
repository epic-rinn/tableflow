// Package ordering implements shared-visit orders with immutable line
// snapshots, the kitchen line workflow and assistance requests.
//
// Lock order: acting staff rows (FOR SHARE) → visit (FOR UPDATE) → guest
// session/capability (FOR SHARE, children of the visit) → menu items
// (FOR SHARE, ID order) → order lines / assistance rows.
package ordering

import (
	"context"
	"embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/epic-rinn/tableflow/src/api/internal/access"
	"github.com/epic-rinn/tableflow/src/api/internal/identity"
)

//go:embed sql/*.sql
var sqlFiles embed.FS

func q(name string) string {
	b, err := sqlFiles.ReadFile("sql/" + name + ".sql")
	if err != nil {
		panic("ordering: missing SQL " + name)
	}
	return string(b)
}

// Bounds (ORD-001).
const (
	MaxBody      = 64 << 10
	maxLines     = 50
	maxQuantity  = 20
	maxNoteRunes = 500
	pageDefault  = 25
	pageMax      = 100
)

// Errors mapped to HTTP responses.
var (
	ErrNotFound        = errors.New("not found")
	ErrForbidden       = errors.New("forbidden")
	ErrVersionConflict = errors.New("version conflict")
	ErrVisitState      = errors.New("visit does not accept this")
	ErrLineState       = errors.New("line state conflict")
)

// MenuChangedError lists items that changed since the guest's menu revision.
type MenuChangedError struct{ ItemIDs []string }

func (e *MenuChangedError) Error() string { return "menu changed" }

// ValidationError carries per-field messages.
type ValidationError struct{ Fields map[string]string }

func (e *ValidationError) Error() string { return "validation failed" }

// Actor issues an ordering command: staff or a dining guest.
type Actor struct {
	Staff *identity.Principal
	Guest *access.Guest
}

// Service implements ordering use cases.
type Service struct {
	pool  *pgxpool.Pool
	staff *identity.Service
}

// NewService builds the service.
func NewService(pool *pgxpool.Pool, staff *identity.Service) *Service {
	return &Service{pool: pool, staff: staff}
}

// lockVisit authorizes actor for visitID and locks the visit row. Staff are
// re-validated first (principal rows first); guests after the visit lock
// (their capability is a child of the visit). staffRoles restricts staff.
func (s *Service) lockVisit(ctx context.Context, tx pgx.Tx, actor Actor, visitID string, staffRoles ...string) (branchID, state string, staff *identity.Principal, err error) {
	if actor.Staff != nil {
		p, err := s.staff.Revalidate(ctx, tx, *actor.Staff)
		if err != nil {
			return "", "", nil, err
		}
		if !slices.ContainsFunc(staffRoles, p.Has) {
			return "", "", nil, ErrForbidden
		}
		staff = &p
	}
	err = tx.QueryRow(ctx, q("visit_lock"), visitID).Scan(&branchID, &state)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", nil, ErrNotFound
	}
	if err != nil {
		return "", "", nil, err
	}
	switch {
	case staff != nil:
		if staff.BranchID != branchID {
			return "", "", nil, ErrNotFound
		}
	case actor.Guest != nil:
		g, err := access.RevalidateGuest(ctx, tx, *actor.Guest)
		if err != nil {
			return "", "", nil, err
		}
		if g.Kind != access.KindVisit || g.ResourceID != visitID {
			return "", "", nil, ErrNotFound
		}
	default:
		return "", "", nil, ErrForbidden
	}
	return branchID, state, staff, nil
}

// readable checks that actor may read visitID (no locks).
func (s *Service) readable(ctx context.Context, actor Actor, visitID string) error {
	var branch string
	err := s.pool.QueryRow(ctx, q("visit_branch"), visitID).Scan(&branch)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	switch {
	case actor.Staff != nil && actor.Staff.BranchID == branch:
		return nil
	case actor.Guest != nil && actor.Guest.Kind == access.KindVisit && actor.Guest.ResourceID == visitID:
		return nil
	}
	return ErrNotFound
}

// --- submission ----------------------------------------------------------

// LineIn is one requested line.
type LineIn struct {
	ItemID    string   `json:"item_id"`
	Quantity  int      `json:"quantity"`
	OptionIDs []string `json:"option_ids"`
	Note      *string  `json:"note"`
}

// OrderIn is a submission.
type OrderIn struct {
	MenuRevision int      `json:"menu_revision"`
	Lines        []LineIn `json:"lines"`
}

// OptionSnap is an immutable option snapshot on a line.
type OptionSnap struct {
	GroupTH          string `json:"group_name_th"`
	GroupEN          string `json:"group_name_en"`
	OptionID         string `json:"option_id"`
	NameTH           string `json:"name_th"`
	NameEN           string `json:"name_en"`
	PriceDeltaSatang int64  `json:"price_delta_satang"`
}

// Line is an order line view.
type Line struct {
	ID              string       `json:"id"`
	OrderID         string       `json:"order_id"`
	ItemID          string       `json:"item_id"`
	NameTH          string       `json:"name_th"`
	NameEN          string       `json:"name_en"`
	Options         []OptionSnap `json:"options"`
	UnitPriceSatang int64        `json:"unit_price_satang"`
	Quantity        int          `json:"quantity"`
	LineTotalSatang int64        `json:"line_total_satang"`
	Note            *string      `json:"note"`
	State           string       `json:"state"`
	Chargeable      bool         `json:"chargeable"`
	Reason          *string      `json:"reason"`
	Version         int          `json:"version"`
}

// Order is a confirmed order with its lines.
type Order struct {
	ID        string    `json:"id"`
	VisitID   string    `json:"visit_id"`
	Actor     string    `json:"actor"`
	CreatedAt time.Time `json:"created_at"`
	Lines     []Line    `json:"lines"`
}

func chargeable(state string) bool { return state != "rejected" && state != "cancelled" }

func scanLine(r interface{ Scan(...any) error }) (Line, error) {
	var l Line
	var opts []byte
	if err := r.Scan(&l.ID, &l.OrderID, &l.ItemID, &l.NameTH, &l.NameEN, &opts, &l.UnitPriceSatang, &l.Quantity, &l.Note, &l.State, &l.Reason, &l.Version); err != nil {
		return Line{}, err
	}
	l.Options = []OptionSnap{}
	if err := json.Unmarshal(opts, &l.Options); err != nil {
		return Line{}, err
	}
	l.LineTotalSatang = l.UnitPriceSatang * int64(l.Quantity)
	l.Chargeable = chargeable(l.State)
	return l, nil
}

func validateOrder(in OrderIn) error {
	fields := map[string]string{}
	if in.MenuRevision < 1 {
		fields["menu_revision"] = "is required"
	}
	if len(in.Lines) < 1 || len(in.Lines) > maxLines {
		fields["lines"] = fmt.Sprintf("send 1–%d lines", maxLines)
	}
	for i, l := range in.Lines {
		if l.Quantity < 1 || l.Quantity > maxQuantity {
			fields[fmt.Sprintf("lines[%d].quantity", i)] = "must be 1–20"
		}
		if l.Note != nil && utf8.RuneCountInString(*l.Note) > maxNoteRunes {
			fields[fmt.Sprintf("lines[%d].note", i)] = "at most 500 characters"
		}
		if len(l.OptionIDs) > 200 {
			fields[fmt.Sprintf("lines[%d].option_ids", i)] = "too many options"
		}
		if len(fields) > 20 {
			break
		}
	}
	if len(fields) > 0 {
		return &ValidationError{Fields: fields}
	}
	return nil
}

type menuItem struct {
	nameTH, nameEN string
	price          int64
	soldOut        bool
	changedRev     int
	retired        bool
	groups         []menuGroup
}

type menuGroup struct {
	id, nameTH, nameEN string
	min, max           int
	options            map[string]OptionSnap
}

// Submit validates and records one order atomically (ORD-001/002/003).
func (s *Service) Submit(ctx context.Context, tx pgx.Tx, actor Actor, visitID string, in OrderIn) (Order, error) {
	if err := validateOrder(in); err != nil {
		return Order{}, err
	}
	branchID, state, staff, err := s.lockVisit(ctx, tx, actor, visitID, identity.RoleHost, identity.RoleManager)
	if err != nil {
		return Order{}, err
	}
	if state != "open" {
		return Order{}, ErrVisitState
	}
	var current int
	if err := tx.QueryRow(ctx, "SELECT coalesce((SELECT revision FROM menus WHERE branch_id = $1), 1)", branchID).Scan(&current); err != nil {
		return Order{}, err
	}
	if in.MenuRevision > current {
		return Order{}, &ValidationError{Fields: map[string]string{"menu_revision": "is newer than the current menu"}}
	}
	ids := []string{}
	for _, l := range in.Lines {
		if !slices.Contains(ids, l.ItemID) {
			ids = append(ids, l.ItemID)
		}
	}
	slices.Sort(ids)
	items := map[string]*menuItem{}
	rows, err := tx.Query(ctx, q("items_for_order"), branchID, ids)
	if err != nil {
		return Order{}, err
	}
	for rows.Next() {
		var id string
		it := &menuItem{}
		if err := rows.Scan(&id, &it.nameTH, &it.nameEN, &it.price, &it.soldOut, &it.changedRev, &it.retired); err != nil {
			rows.Close()
			return Order{}, err
		}
		items[id] = it
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return Order{}, err
	}
	rows, err = tx.Query(ctx, q("groups_for_items"), ids)
	if err != nil {
		return Order{}, err
	}
	for rows.Next() {
		var gid, itemID, gth, gen string
		var gmin, gmax int
		var oid, oth, oen *string
		var delta *int64
		if err := rows.Scan(&gid, &itemID, &gth, &gen, &gmin, &gmax, &oid, &oth, &oen, &delta); err != nil {
			rows.Close()
			return Order{}, err
		}
		it := items[itemID]
		if it == nil {
			continue
		}
		idx := slices.IndexFunc(it.groups, func(g menuGroup) bool { return g.id == gid })
		if idx < 0 {
			it.groups = append(it.groups, menuGroup{id: gid, nameTH: gth, nameEN: gen, min: gmin, max: gmax, options: map[string]OptionSnap{}})
			idx = len(it.groups) - 1
		}
		if oid != nil {
			it.groups[idx].options[*oid] = OptionSnap{GroupTH: gth, GroupEN: gen, OptionID: *oid, NameTH: *oth, NameEN: *oen, PriceDeltaSatang: *delta}
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return Order{}, err
	}

	var changed []string
	fields := map[string]string{}
	type prepared struct {
		name   [2]string
		opts   []byte
		unit   int64
		qty    int
		note   *string
		itemID string
	}
	lines := make([]prepared, 0, len(in.Lines))
	for i, l := range in.Lines {
		it := items[l.ItemID]
		if it == nil {
			fields[fmt.Sprintf("lines[%d].item_id", i)] = "unknown item"
			continue
		}
		if it.retired || it.soldOut || it.changedRev > in.MenuRevision {
			if !slices.Contains(changed, l.ItemID) {
				changed = append(changed, l.ItemID)
			}
			continue
		}
		unit := it.price
		snaps := []OptionSnap{}
		chosen := map[string]int{} // group → count
		seenOpt := map[string]bool{}
		for _, oid := range l.OptionIDs {
			if seenOpt[oid] {
				fields[fmt.Sprintf("lines[%d].option_ids", i)] = "duplicate option"
				break
			}
			seenOpt[oid] = true
			found := false
			for _, g := range it.groups {
				if o, ok := g.options[oid]; ok {
					chosen[g.id]++
					unit += o.PriceDeltaSatang
					snaps = append(snaps, o)
					found = true
					break
				}
			}
			if !found {
				fields[fmt.Sprintf("lines[%d].option_ids", i)] = "option does not belong to this item"
			}
		}
		for _, g := range it.groups {
			if c := chosen[g.id]; c < g.min || c > g.max {
				fields[fmt.Sprintf("lines[%d].option_ids", i)] = fmt.Sprintf("choose %d–%d for %s", g.min, g.max, g.nameEN)
			}
		}
		b, err := json.Marshal(snaps)
		if err != nil {
			return Order{}, err
		}
		var note *string
		if l.Note != nil {
			if n := strings.TrimSpace(*l.Note); n != "" {
				note = &n
			}
		}
		lines = append(lines, prepared{name: [2]string{it.nameTH, it.nameEN}, opts: b, unit: unit, qty: l.Quantity, note: note, itemID: l.ItemID})
	}
	if len(changed) > 0 {
		slices.Sort(changed)
		return Order{}, &MenuChangedError{ItemIDs: changed}
	}
	if len(fields) > 0 {
		return Order{}, &ValidationError{Fields: fields}
	}

	kind, staffID, guestSession := "guest", (*string)(nil), (*string)(nil)
	if staff != nil {
		kind, staffID = "staff", &staff.StaffID
	} else {
		guestSession = &actor.Guest.SessionID
	}
	o := Order{VisitID: visitID, Actor: kind}
	if err := tx.QueryRow(ctx, q("order_insert"), branchID, visitID, kind, staffID, guestSession, in.MenuRevision).Scan(&o.ID, &o.CreatedAt); err != nil {
		return Order{}, err
	}
	var itemIDs, nameTH, nameEN, opts []string
	var notes []*string
	var units []int64
	var qtys []int32
	for _, l := range lines {
		itemIDs = append(itemIDs, l.itemID)
		nameTH = append(nameTH, l.name[0])
		nameEN = append(nameEN, l.name[1])
		opts = append(opts, string(l.opts))
		units = append(units, l.unit)
		qtys = append(qtys, int32(l.qty))
		notes = append(notes, l.note)
	}
	if _, err := tx.Exec(ctx, q("lines_insert"), branchID, o.ID, visitID, itemIDs, nameTH, nameEN, opts, units, qtys, notes); err != nil {
		return Order{}, err
	}
	if _, err := tx.Exec(ctx, q("bump_bill"), visitID); err != nil {
		return Order{}, err
	}
	rows, err = tx.Query(ctx, q("lines_for_orders"), []string{o.ID})
	if err != nil {
		return Order{}, err
	}
	o.Lines, err = pgx.CollectRows(rows, func(r pgx.CollectableRow) (Line, error) { return scanLine(r) })
	return o, err
}

// --- reads ---------------------------------------------------------------

// OrdersPage is a page of confirmed orders plus visit-wide totals.
type OrdersPage struct {
	Items            []Order   `json:"items"`
	NextCursor       *string   `json:"next_cursor"`
	ChargeableSatang int64     `json:"chargeable_total_satang"`
	ChargeableLines  int       `json:"chargeable_lines"`
	ServerTime       time.Time `json:"server_time"`
}

type cursor struct {
	T  time.Time `json:"t"`
	ID string    `json:"i"`
	V  string    `json:"v"`
}

// Orders lists a visit's confirmed orders (its guests or branch staff);
// three statements regardless of page size (ORD-005).
func (s *Service) Orders(ctx context.Context, actor Actor, visitID, after string, limit int) (OrdersPage, error) {
	if err := s.readable(ctx, actor, visitID); err != nil {
		return OrdersPage{}, err
	}
	if limit <= 0 {
		limit = pageDefault
	}
	limit = min(limit, pageMax)
	c := cursor{T: time.Unix(0, 0).UTC(), ID: "00000000-0000-0000-0000-000000000000", V: visitID}
	if after != "" {
		b, err := base64.RawURLEncoding.DecodeString(after)
		if err != nil || json.Unmarshal(b, &c) != nil || c.V != visitID {
			return OrdersPage{}, &ValidationError{Fields: map[string]string{"cursor": "invalid cursor"}}
		}
	}
	rows, err := s.pool.Query(ctx, q("orders_page"), visitID, c.T, c.ID, limit+1)
	if err != nil {
		return OrdersPage{}, err
	}
	orders, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Order, error) {
		o := Order{VisitID: visitID, Lines: []Line{}}
		return o, r.Scan(&o.ID, &o.Actor, &o.CreatedAt)
	})
	if err != nil {
		return OrdersPage{}, err
	}
	page := OrdersPage{Items: orders, ServerTime: time.Now().UTC()}
	if len(orders) > limit {
		page.Items = orders[:limit]
		last := page.Items[limit-1]
		b, _ := json.Marshal(cursor{T: last.CreatedAt, ID: last.ID, V: visitID})
		next := base64.RawURLEncoding.EncodeToString(b)
		page.NextCursor = &next
	}
	ids := make([]string, len(page.Items))
	index := map[string]int{}
	for i, o := range page.Items {
		ids[i] = o.ID
		index[o.ID] = i
	}
	if len(ids) > 0 {
		rows, err = s.pool.Query(ctx, q("lines_for_orders"), ids)
		if err != nil {
			return OrdersPage{}, err
		}
		lines, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Line, error) { return scanLine(r) })
		if err != nil {
			return OrdersPage{}, err
		}
		for _, l := range lines {
			page.Items[index[l.OrderID]].Lines = append(page.Items[index[l.OrderID]].Lines, l)
		}
	}
	err = s.pool.QueryRow(ctx, q("visit_totals"), visitID).Scan(&page.ChargeableSatang, &page.ChargeableLines)
	return page, err
}

// ChargeableLines reports whether a visit has any chargeable line; used by
// close-empty (SEA-004) inside the caller's transaction.
func ChargeableLines(ctx context.Context, tx pgx.Tx, visitID string) (int, error) {
	var total int64
	var n int
	err := tx.QueryRow(ctx, q("visit_totals"), visitID).Scan(&total, &n)
	return n, err
}
