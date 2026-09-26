// Package menu implements branch menus: categories → items → option groups →
// options, with Thai/English names, satang prices, sold-out toggles and
// revisions. Entities are retired, never deleted, so order snapshots and
// references stay valid.
package menu

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/epic-rinn/tableflow/src/api/internal/identity"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/audit"
)

//go:embed sql/*.sql
var sqlFiles embed.FS

func q(name string) string {
	b, err := sqlFiles.ReadFile("sql/" + name + ".sql")
	if err != nil {
		panic("menu: missing SQL " + name)
	}
	return string(b)
}

// Bounds (pilot limits; the HTTP contract caps the menu at 500 items).
const (
	MaxBody          = 1 << 20
	maxCategories    = 50
	maxItems         = 500
	maxGroupsPerItem = 10
	maxOptions       = 20
	maxPrice         = 10_000_000
	maxNameRunes     = 80
)

// Errors mapped to HTTP responses.
var (
	ErrNotFound        = errors.New("not found")
	ErrForbidden       = errors.New("forbidden")
	ErrVersionConflict = errors.New("version conflict")
)

// ValidationError carries per-field messages.
type ValidationError struct{ Fields map[string]string }

func (e *ValidationError) Error() string { return "validation failed" }

// Option is one choice in a group; its delta adds to the item price.
type Option struct {
	ID               string `json:"id"`
	NameTH           string `json:"name_th"`
	NameEN           string `json:"name_en"`
	PriceDeltaSatang int64  `json:"price_delta_satang"`
}

// OptionGroup bounds how many options may be chosen (min 0 = optional).
type OptionGroup struct {
	ID         string   `json:"id"`
	NameTH     string   `json:"name_th"`
	NameEN     string   `json:"name_en"`
	MinChoices int      `json:"min_choices"`
	MaxChoices int      `json:"max_choices"`
	Options    []Option `json:"options"`
}

// Item is a menu item.
type Item struct {
	ID           string        `json:"id"`
	NameTH       string        `json:"name_th"`
	NameEN       string        `json:"name_en"`
	PriceSatang  int64         `json:"price_satang"`
	SoldOut      bool          `json:"sold_out"`
	Version      int           `json:"version"`
	OptionGroups []OptionGroup `json:"option_groups"`
	changedRev   int
	categoryID   string
}

// Category groups items.
type Category struct {
	ID     string `json:"id"`
	NameTH string `json:"name_th"`
	NameEN string `json:"name_en"`
	Items  []Item `json:"items"`
}

// Menu is the full active menu of a branch.
type Menu struct {
	BranchID   string     `json:"branch_id"`
	Revision   int        `json:"revision"`
	Currency   string     `json:"currency"`
	Categories []Category `json:"categories"`
	ServerTime time.Time  `json:"server_time"`
}

// Service implements menu use cases.
type Service struct {
	pool  *pgxpool.Pool
	staff *identity.Service
}

// NewService builds the service.
func NewService(pool *pgxpool.Pool, staff *identity.Service) *Service {
	return &Service{pool: pool, staff: staff}
}

type querier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

// load reads the active menu in four statements.
func load(ctx context.Context, db querier, branchID string) (Menu, error) {
	m := Menu{BranchID: branchID, Currency: "THB", Categories: []Category{}}
	err := db.QueryRow(ctx, q("menu_revision"), branchID).Scan(&m.Revision)
	if errors.Is(err, pgx.ErrNoRows) {
		m.Revision = 1 // branch without menu edits yet
	} else if err != nil {
		return Menu{}, err
	}
	rows, err := db.Query(ctx, q("categories"), branchID)
	if err != nil {
		return Menu{}, err
	}
	cats, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Category, error) {
		c := Category{Items: []Item{}}
		return c, r.Scan(&c.ID, &c.NameTH, &c.NameEN)
	})
	if err != nil {
		return Menu{}, err
	}
	rows, err = db.Query(ctx, q("items"), branchID)
	if err != nil {
		return Menu{}, err
	}
	items, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Item, error) {
		it := Item{OptionGroups: []OptionGroup{}}
		return it, r.Scan(&it.ID, &it.categoryID, &it.NameTH, &it.NameEN, &it.PriceSatang, &it.SoldOut, &it.Version, &it.changedRev)
	})
	if err != nil {
		return Menu{}, err
	}
	groups := map[string][]OptionGroup{} // item → groups in order
	rows, err = db.Query(ctx, q("groups_options"), branchID)
	if err != nil {
		return Menu{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var g OptionGroup
		var itemID string
		var oid, oth, oen *string
		var delta *int64
		if err := rows.Scan(&g.ID, &itemID, &g.NameTH, &g.NameEN, &g.MinChoices, &g.MaxChoices, &oid, &oth, &oen, &delta); err != nil {
			return Menu{}, err
		}
		list := groups[itemID]
		if n := len(list); n == 0 || list[n-1].ID != g.ID {
			g.Options = []Option{}
			list = append(list, g)
		}
		if oid != nil {
			list[len(list)-1].Options = append(list[len(list)-1].Options, Option{ID: *oid, NameTH: *oth, NameEN: *oen, PriceDeltaSatang: *delta})
		}
		groups[itemID] = list
	}
	if err := rows.Err(); err != nil {
		return Menu{}, err
	}
	index := map[string]int{}
	for i, c := range cats {
		index[c.ID] = i
	}
	for _, it := range items {
		if gs, ok := groups[it.ID]; ok {
			it.OptionGroups = gs
		}
		if i, ok := index[it.categoryID]; ok {
			cats[i].Items = append(cats[i].Items, it)
		}
	}
	m.Categories = cats
	m.ServerTime = time.Now().UTC()
	return m, nil
}

// Get returns the public active menu (no authentication; no private data).
func (s *Service) Get(ctx context.Context, branchID string) (Menu, error) {
	var exists bool
	if err := s.pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM branches WHERE id = $1)", branchID).Scan(&exists); err != nil {
		return Menu{}, err
	}
	if !exists {
		return Menu{}, ErrNotFound
	}
	return load(ctx, s.pool, branchID)
}

// --- replacement -------------------------------------------------------

// OptionIn, GroupIn, ItemIn, CategoryIn describe the desired menu; an empty
// ID creates an entity, a known ID updates it, and omitted ones retire.
type OptionIn struct {
	ID               string `json:"id"`
	NameTH           string `json:"name_th"`
	NameEN           string `json:"name_en"`
	PriceDeltaSatang int64  `json:"price_delta_satang"`
}

type GroupIn struct {
	ID         string     `json:"id"`
	NameTH     string     `json:"name_th"`
	NameEN     string     `json:"name_en"`
	MinChoices int        `json:"min_choices"`
	MaxChoices int        `json:"max_choices"`
	Options    []OptionIn `json:"options"`
}

type ItemIn struct {
	ID           string    `json:"id"`
	NameTH       string    `json:"name_th"`
	NameEN       string    `json:"name_en"`
	PriceSatang  int64     `json:"price_satang"`
	OptionGroups []GroupIn `json:"option_groups"`
}

type CategoryIn struct {
	ID     string   `json:"id"`
	NameTH string   `json:"name_th"`
	NameEN string   `json:"name_en"`
	Items  []ItemIn `json:"items"`
}

func validName(s string) bool {
	n := utf8.RuneCountInString(strings.TrimSpace(s))
	return n >= 1 && n <= maxNameRunes
}

func validateTree(cats []CategoryIn) error {
	fields := map[string]string{}
	add := func(path, msg string) {
		if len(fields) < 20 {
			fields[path] = msg
		}
	}
	if len(cats) > maxCategories {
		add("categories", fmt.Sprintf("at most %d categories", maxCategories))
	}
	items := 0
	for ci, c := range cats {
		cp := fmt.Sprintf("categories[%d]", ci)
		if !validName(c.NameTH) || !validName(c.NameEN) {
			add(cp+".name", "Thai and English names must be 1–80 characters")
		}
		items += len(c.Items)
		for ii, it := range c.Items {
			ip := fmt.Sprintf("%s.items[%d]", cp, ii)
			if !validName(it.NameTH) || !validName(it.NameEN) {
				add(ip+".name", "Thai and English names must be 1–80 characters")
			}
			if it.PriceSatang < 0 || it.PriceSatang > maxPrice {
				add(ip+".price_satang", "must be 0–10,000,000 satang")
			}
			if len(it.OptionGroups) > maxGroupsPerItem {
				add(ip+".option_groups", fmt.Sprintf("at most %d groups", maxGroupsPerItem))
			}
			for gi, g := range it.OptionGroups {
				gp := fmt.Sprintf("%s.option_groups[%d]", ip, gi)
				if !validName(g.NameTH) || !validName(g.NameEN) {
					add(gp+".name", "Thai and English names must be 1–80 characters")
				}
				n := len(g.Options)
				if n == 0 || n > maxOptions {
					add(gp+".options", fmt.Sprintf("1–%d options", maxOptions))
				}
				if g.MinChoices < 0 || g.MaxChoices < 1 || g.MinChoices > g.MaxChoices || g.MaxChoices > n {
					add(gp+".choices", "need 0 ≤ min ≤ max, 1 ≤ max ≤ number of options")
				}
				for oi, o := range g.Options {
					op := fmt.Sprintf("%s.options[%d]", gp, oi)
					if !validName(o.NameTH) || !validName(o.NameEN) {
						add(op+".name", "Thai and English names must be 1–80 characters")
					}
					if o.PriceDeltaSatang < 0 || o.PriceDeltaSatang > maxPrice {
						add(op+".price_delta_satang", "must be 0–10,000,000 satang")
					}
				}
			}
		}
	}
	if items > maxItems {
		add("items", fmt.Sprintf("at most %d items", maxItems))
	}
	if len(fields) > 0 {
		return &ValidationError{Fields: fields}
	}
	return nil
}

// signature captures everything that changes a charge: price, groups
// (bounds) and options (deltas). Name changes are deliberately excluded.
func signatureOf(price int64, groups []OptionGroup) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%d|", price)
	for _, g := range groups {
		fmt.Fprintf(&b, "g%s:%d-%d|", g.ID, g.MinChoices, g.MaxChoices)
		for _, o := range g.Options {
			fmt.Fprintf(&b, "o%s:%d|", o.ID, o.PriceDeltaSatang)
		}
	}
	return b.String()
}

func signatureIn(it ItemIn) (string, bool) {
	var b strings.Builder
	fmt.Fprintf(&b, "%d|", it.PriceSatang)
	complete := true
	for _, g := range it.OptionGroups {
		if g.ID == "" {
			complete = false
		}
		fmt.Fprintf(&b, "g%s:%d-%d|", g.ID, g.MinChoices, g.MaxChoices)
		for _, o := range g.Options {
			if o.ID == "" {
				complete = false
			}
			fmt.Fprintf(&b, "o%s:%d|", o.ID, o.PriceDeltaSatang)
		}
	}
	return b.String(), complete
}

// Replace atomically sets the branch menu to cats (manager).
func (s *Service) Replace(ctx context.Context, p identity.Principal, branchID string, expectedRevision int, cats []CategoryIn, requestID string) (Menu, error) {
	if err := validateTree(cats); err != nil {
		return Menu{}, err
	}
	var out Menu
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		actor, err := s.staff.Revalidate(ctx, tx, p)
		if err != nil {
			return err
		}
		if actor.BranchID != branchID {
			return ErrNotFound
		}
		if !actor.Has(identity.RoleManager) {
			return ErrForbidden
		}
		if _, err := tx.Exec(ctx, q("ensure_menu"), branchID); err != nil {
			return err
		}
		var rev int
		if err := tx.QueryRow(ctx, q("lock_menu"), branchID).Scan(&rev); err != nil {
			return err
		}
		if rev != expectedRevision {
			return ErrVersionConflict
		}
		newRev := rev + 1
		if _, err := tx.Exec(ctx, q("items_lock"), branchID); err != nil {
			return err
		}
		current, err := load(ctx, tx, branchID)
		if err != nil {
			return err
		}
		if err := apply(ctx, tx, branchID, current, cats, newRev); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, q("bump_menu"), branchID, newRev); err != nil {
			return err
		}
		if err := audit.Record(ctx, tx, audit.Event{BranchID: branchID, ActorStaffID: &actor.StaffID, Action: "menu.replaced",
			ResourceType: "menu", ResourceID: branchID, RequestID: requestID, Details: map[string]any{"revision": newRev}}); err != nil {
			return err
		}
		out, err = load(ctx, tx, branchID)
		return err
	})
	return out, err
}

// apply writes the desired tree. Every referenced ID must already exist in
// this branch under the same kind of parent (groups and options may not move
// between items or groups; items may move between categories).
func apply(ctx context.Context, tx pgx.Tx, branchID string, current Menu, cats []CategoryIn, newRev int) error {
	curCats := map[string]bool{}
	curItems := map[string]Item{}
	groupOwner := map[string]string{}  // group → item
	optionOwner := map[string]string{} // option → group
	for _, c := range current.Categories {
		curCats[c.ID] = true
		for _, it := range c.Items {
			curItems[it.ID] = it
			for _, g := range it.OptionGroups {
				groupOwner[g.ID] = it.ID
				for _, o := range g.Options {
					optionOwner[o.ID] = g.ID
				}
			}
		}
	}
	bad := func(path string) error {
		return &ValidationError{Fields: map[string]string{path: "unknown or misplaced id"}}
	}
	keepCats, keepItems, keepGroups, keepOptions := []string{}, []string{}, []string{}, []string{}
	seen := map[string]bool{}
	for ci, c := range cats {
		cp := fmt.Sprintf("categories[%d]", ci)
		catID := c.ID
		if catID == "" {
			if err := tx.QueryRow(ctx, q("category_insert"), branchID, strings.TrimSpace(c.NameTH), strings.TrimSpace(c.NameEN), ci).Scan(&catID); err != nil {
				return err
			}
		} else {
			if !curCats[catID] || seen[catID] {
				return bad(cp + ".id")
			}
			if _, err := tx.Exec(ctx, q("category_update"), catID, branchID, strings.TrimSpace(c.NameTH), strings.TrimSpace(c.NameEN), ci); err != nil {
				return err
			}
		}
		seen[catID] = true
		keepCats = append(keepCats, catID)
		for ii, it := range c.Items {
			ip := fmt.Sprintf("%s.items[%d]", cp, ii)
			itemID := it.ID
			if itemID == "" {
				if err := tx.QueryRow(ctx, q("item_insert"), branchID, catID, strings.TrimSpace(it.NameTH), strings.TrimSpace(it.NameEN),
					it.PriceSatang, ii, newRev).Scan(&itemID); err != nil {
					return err
				}
			} else {
				old, ok := curItems[itemID]
				if !ok || seen[itemID] {
					return bad(ip + ".id")
				}
				changed := old.changedRev
				if sig, complete := signatureIn(it); !complete || sig != signatureOf(old.PriceSatang, old.OptionGroups) {
					changed = newRev
				}
				if _, err := tx.Exec(ctx, q("item_update"), itemID, branchID, catID, strings.TrimSpace(it.NameTH), strings.TrimSpace(it.NameEN),
					it.PriceSatang, ii, changed); err != nil {
					return err
				}
			}
			seen[itemID] = true
			keepItems = append(keepItems, itemID)
			for gi, g := range it.OptionGroups {
				gp := fmt.Sprintf("%s.option_groups[%d]", ip, gi)
				groupID := g.ID
				if groupID == "" {
					if err := tx.QueryRow(ctx, q("group_insert"), branchID, itemID, strings.TrimSpace(g.NameTH), strings.TrimSpace(g.NameEN),
						g.MinChoices, g.MaxChoices, gi).Scan(&groupID); err != nil {
						return err
					}
				} else {
					if groupOwner[groupID] != itemID || seen[groupID] {
						return bad(gp + ".id")
					}
					if _, err := tx.Exec(ctx, q("group_update"), groupID, branchID, strings.TrimSpace(g.NameTH), strings.TrimSpace(g.NameEN),
						g.MinChoices, g.MaxChoices, gi); err != nil {
						return err
					}
				}
				seen[groupID] = true
				keepGroups = append(keepGroups, groupID)
				for oi, o := range g.Options {
					if o.ID == "" {
						if _, err := tx.Exec(ctx, q("option_insert"), branchID, groupID, strings.TrimSpace(o.NameTH), strings.TrimSpace(o.NameEN),
							o.PriceDeltaSatang, oi); err != nil {
							return err
						}
						continue
					}
					if optionOwner[o.ID] != groupID || seen[o.ID] {
						return bad(fmt.Sprintf("%s.options[%d].id", gp, oi))
					}
					if _, err := tx.Exec(ctx, q("option_update"), o.ID, branchID, strings.TrimSpace(o.NameTH), strings.TrimSpace(o.NameEN),
						o.PriceDeltaSatang, oi); err != nil {
						return err
					}
					seen[o.ID] = true
					keepOptions = append(keepOptions, o.ID)
				}
			}
		}
	}
	// New options are kept implicitly (they were just inserted); retire only
	// options that existed before and are no longer listed.
	var newOptionIDs []string
	rows, err := tx.Query(ctx, "SELECT id FROM menu_options WHERE branch_id = $1 AND retired_at IS NULL", branchID)
	if err != nil {
		return err
	}
	all, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return err
	}
	for _, id := range all {
		if _, existed := optionOwner[id]; !existed {
			newOptionIDs = append(newOptionIDs, id)
		}
	}
	keepOptions = append(keepOptions, newOptionIDs...)
	for _, step := range []struct {
		name string
		args []any
	}{
		{"options_retire", []any{branchID, keepOptions}},
		{"groups_retire", []any{branchID, keepGroups}},
		{"items_retire", []any{branchID, keepItems, newRev}},
		{"categories_retire", []any{branchID, keepCats}},
	} {
		if _, err := tx.Exec(ctx, q(step.name), step.args...); err != nil {
			return err
		}
	}
	return nil
}

// SetAvailability toggles sold-out (manager or kitchen). It bumps the menu
// revision and the item's changed_revision, so carts built before the change
// are rejected for this item only.
func (s *Service) SetAvailability(ctx context.Context, p identity.Principal, itemID string, expectedVersion int, soldOut bool) (Item, error) {
	var out Item
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		actor, err := s.staff.Revalidate(ctx, tx, p)
		if err != nil {
			return err
		}
		if !actor.Has(identity.RoleManager) && !actor.Has(identity.RoleKitchen) {
			return ErrForbidden
		}
		if _, err := tx.Exec(ctx, q("ensure_menu"), actor.BranchID); err != nil {
			return err
		}
		var rev int
		if err := tx.QueryRow(ctx, q("lock_menu"), actor.BranchID).Scan(&rev); err != nil {
			return err
		}
		var branch string
		var version int
		err = tx.QueryRow(ctx, q("item_lock"), itemID).Scan(&branch, &version)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && branch != actor.BranchID) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if version != expectedVersion {
			return ErrVersionConflict
		}
		if _, err := tx.Exec(ctx, q("item_availability"), itemID, soldOut, rev+1); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, q("bump_menu"), actor.BranchID, rev+1); err != nil {
			return err
		}
		out.OptionGroups = []OptionGroup{}
		return tx.QueryRow(ctx, q("item_get"), itemID).Scan(&out.ID, &out.NameTH, &out.NameEN, &out.PriceSatang, &out.SoldOut, &out.Version)
	})
	return out, err
}
