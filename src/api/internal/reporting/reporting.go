// Package reporting serves manager daily summaries and the audit viewer
// (OPS-001/002, ADM-005). Reads only; five aggregate statements per report.
package reporting

import (
	"context"
	"embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"regexp"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/epic-rinn/tableflow/src/api/internal/identity"
	"github.com/epic-rinn/tableflow/src/api/internal/platform/bizdate"
)

//go:embed sql/*.sql
var sqlFiles embed.FS

func q(name string) string {
	b, err := sqlFiles.ReadFile("sql/" + name + ".sql")
	if err != nil {
		panic("reporting: missing SQL " + name)
	}
	return string(b)
}

// Errors mapped to HTTP responses.
var (
	ErrNotFound  = errors.New("not found")
	ErrForbidden = errors.New("forbidden")
)

// ValidationError carries per-field messages.
type ValidationError struct{ Fields map[string]string }

func (e *ValidationError) Error() string { return "validation failed" }

// Page bounds for the audit viewer.
const (
	pageDefault = 50
	pageMax     = 100
)

// Service implements reporting reads.
type Service struct{ pool *pgxpool.Pool }

// NewService builds the service.
func NewService(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

// managerRange authorizes a manager of branchID and parses the range in the
// branch timezone.
func (s *Service) managerRange(ctx context.Context, p identity.Principal, branchID, from, to string) (bizdate.Range, error) {
	if p.BranchID != branchID {
		return bizdate.Range{}, ErrNotFound
	}
	if !p.Has(identity.RoleManager) {
		return bizdate.Range{}, ErrForbidden
	}
	var tz string
	if err := s.pool.QueryRow(ctx, q("branch_timezone"), branchID).Scan(&tz); err != nil {
		return bizdate.Range{}, err
	}
	r, err := bizdate.Parse(tz, from, to)
	if errors.Is(err, bizdate.ErrRange) {
		return r, &ValidationError{Fields: map[string]string{"from": err.Error()}}
	}
	return r, err
}

// Amount is a count and total in satang.
type Amount struct {
	Count  int64 `json:"count"`
	Satang int64 `json:"satang"`
}

// Day is one business date (or the range totals when Date is empty).
type Day struct {
	Date              string            `json:"date,omitempty"`
	TicketsJoined     int64             `json:"tickets_joined"`
	TicketsSeated     int64             `json:"tickets_seated"`
	NoShows           int64             `json:"no_shows"`
	TicketsCancelled  int64             `json:"tickets_cancelled"`
	VisitsOpened      int64             `json:"visits_opened"`
	Sales             Amount            `json:"sales"`
	SalesByMethod     map[string]Amount `json:"sales_by_method"`
	Refunds           Amount            `json:"refunds"`
	RefundsByMethod   map[string]Amount `json:"refunds_by_method"`
	NetSatang         int64             `json:"net_satang"`
	MemberSettlements int64             `json:"member_settlements"`
	PointsEarned      int64             `json:"points_earned"`
	PointsReversed    int64             `json:"points_reversed"`
}

// Report is the daily summary of a range.
type Report struct {
	BranchID string `json:"branch_id"`
	Timezone string `json:"timezone"`
	From     string `json:"from"`
	To       string `json:"to"`
	Days     []Day  `json:"days"`
	Totals   Day    `json:"totals"`
}

func newDay(date string) *Day {
	return &Day{Date: date, SalesByMethod: map[string]Amount{}, RefundsByMethod: map[string]Amount{}}
}

func add(m map[string]Amount, k string, a Amount) {
	cur := m[k]
	m[k] = Amount{Count: cur.Count + a.Count, Satang: cur.Satang + a.Satang}
}

// Daily builds the per-day summary (OPS-002). Every date in the range is
// present, zero-filled; each metric is dated by its own event.
func (s *Service) Daily(ctx context.Context, p identity.Principal, branchID, from, to string) (Report, error) {
	r, err := s.managerRange(ctx, p, branchID, from, to)
	if err != nil {
		return Report{}, err
	}
	days := map[string]*Day{}
	for _, d := range r.Dates {
		days[d] = newDay(d)
	}
	tz := r.Location.String()
	// Each aggregate is scanned explicitly (typed columns, no reflection).
	if err := s.scanTickets(ctx, branchID, r, tz, days); err != nil {
		return Report{}, err
	}
	if err := s.scanSimple(ctx, "report_visits", branchID, r, tz, days, func(d *Day, n int64) { d.VisitsOpened = n }); err != nil {
		return Report{}, err
	}
	if err := s.scanMethods(ctx, "report_settlements", branchID, r, tz, days, true); err != nil {
		return Report{}, err
	}
	if err := s.scanMethods(ctx, "report_refunds", branchID, r, tz, days, false); err != nil {
		return Report{}, err
	}
	if err := s.scanLoyalty(ctx, branchID, r, tz, days); err != nil {
		return Report{}, err
	}
	rep := Report{BranchID: branchID, Timezone: tz, From: r.From, To: r.To, Totals: *newDay("")}
	t := &rep.Totals
	for _, date := range r.Dates {
		d := days[date]
		d.NetSatang = d.Sales.Satang - d.Refunds.Satang
		rep.Days = append(rep.Days, *d)
		t.TicketsJoined += d.TicketsJoined
		t.TicketsSeated += d.TicketsSeated
		t.NoShows += d.NoShows
		t.TicketsCancelled += d.TicketsCancelled
		t.VisitsOpened += d.VisitsOpened
		t.Sales = Amount{t.Sales.Count + d.Sales.Count, t.Sales.Satang + d.Sales.Satang}
		t.Refunds = Amount{t.Refunds.Count + d.Refunds.Count, t.Refunds.Satang + d.Refunds.Satang}
		for k, v := range d.SalesByMethod {
			add(t.SalesByMethod, k, v)
		}
		for k, v := range d.RefundsByMethod {
			add(t.RefundsByMethod, k, v)
		}
		t.NetSatang += d.NetSatang
		t.MemberSettlements += d.MemberSettlements
		t.PointsEarned += d.PointsEarned
		t.PointsReversed += d.PointsReversed
	}
	return rep, nil
}

func (s *Service) query(ctx context.Context, name, branchID string, r bizdate.Range, tz string) (pgx.Rows, error) {
	return s.pool.Query(ctx, q(name), branchID, r.Start, r.End, tz)
}

func (s *Service) scanTickets(ctx context.Context, branchID string, r bizdate.Range, tz string, days map[string]*Day) error {
	rows, err := s.query(ctx, "report_tickets", branchID, r, tz)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var date string
		var joined, seated, noShow, cancelled int64
		if err := rows.Scan(&date, &joined, &seated, &noShow, &cancelled); err != nil {
			return err
		}
		if d := days[date]; d != nil {
			d.TicketsJoined, d.TicketsSeated, d.NoShows, d.TicketsCancelled = joined, seated, noShow, cancelled
		}
	}
	return rows.Err()
}

func (s *Service) scanSimple(ctx context.Context, name, branchID string, r bizdate.Range, tz string, days map[string]*Day, set func(*Day, int64)) error {
	rows, err := s.query(ctx, name, branchID, r, tz)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var date string
		var n int64
		if err := rows.Scan(&date, &n); err != nil {
			return err
		}
		if d := days[date]; d != nil {
			set(d, n)
		}
	}
	return rows.Err()
}

func (s *Service) scanMethods(ctx context.Context, name, branchID string, r bizdate.Range, tz string, days map[string]*Day, sales bool) error {
	rows, err := s.query(ctx, name, branchID, r, tz)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var date, method string
		var a Amount
		var members int64
		dest := []any{&date, &method, &a.Count, &a.Satang}
		if sales {
			dest = append(dest, &members)
		}
		if err := rows.Scan(dest...); err != nil {
			return err
		}
		d := days[date]
		if d == nil {
			continue
		}
		if sales {
			add(d.SalesByMethod, method, a)
			d.Sales = Amount{d.Sales.Count + a.Count, d.Sales.Satang + a.Satang}
			d.MemberSettlements += members
		} else {
			add(d.RefundsByMethod, method, a)
			d.Refunds = Amount{d.Refunds.Count + a.Count, d.Refunds.Satang + a.Satang}
		}
	}
	return rows.Err()
}

func (s *Service) scanLoyalty(ctx context.Context, branchID string, r bizdate.Range, tz string, days map[string]*Day) error {
	rows, err := s.query(ctx, "report_loyalty", branchID, r, tz)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var date string
		var earned, reversed int64
		if err := rows.Scan(&date, &earned, &reversed); err != nil {
			return err
		}
		if d := days[date]; d != nil {
			d.PointsEarned, d.PointsReversed = earned, reversed
		}
	}
	return rows.Err()
}

// --- audit viewer ------------------------------------------------------------

// Detail keys the application writes into audit events. Anything else is
// dropped before display, so a future mistake cannot surface a secret.
var detailAllowlist = []string{
	"amount_satang", "bill_version", "external_reference", "from", "gold_discount_bp", "gold_threshold", "groups",
	"member_id", "method", "points_earned", "receipt_reference", "refund_id", "revision", "roles", "satang_per_point",
	"service_bp", "silver_discount_bp", "silver_threshold", "snapshot_id", "subject", "tax_bp", "tax_mode",
	"total_satang", "version", "visit_id", "table_id", "ticket_id", "to", "to_state", "item_id", "sold_out",
}

var actionPattern = regexp.MustCompile(`^[a-z_]+(\.[a-z_]+)?$`)

// AuditEvent is one redacted audit row.
type AuditEvent struct {
	ID           string         `json:"id"`
	OccurredAt   time.Time      `json:"occurred_at"`
	Actor        string         `json:"actor"`
	Action       string         `json:"action"`
	ResourceType string         `json:"resource_type"`
	ResourceID   *string        `json:"resource_id"`
	Reason       *string        `json:"reason"`
	RequestID    string         `json:"request_id"`
	Details      map[string]any `json:"details"`
}

// AuditPage is a keyset page, newest first.
type AuditPage struct {
	Items      []AuditEvent `json:"items"`
	NextCursor *string      `json:"next_cursor"`
}

type cursor struct {
	T  time.Time `json:"t"`
	ID string    `json:"i"`
	B  string    `json:"b"`
}

func actorName(staff *string, action string) string {
	switch {
	case staff != nil:
		return *staff
	case action == "visit.member_claimed":
		return "Member"
	}
	return "System"
}

// Audit pages a branch's audit events (manager only, OPS-001).
func (s *Service) Audit(ctx context.Context, p identity.Principal, branchID, from, to, action, after string, limit int) (AuditPage, error) {
	r, err := s.managerRange(ctx, p, branchID, from, to)
	if err != nil {
		return AuditPage{}, err
	}
	var act *string
	if action != "" {
		if !actionPattern.MatchString(action) {
			return AuditPage{}, &ValidationError{Fields: map[string]string{"action": "must be an action like settlement or settlement.refunded"}}
		}
		act = &action
	}
	if limit <= 0 {
		limit = pageDefault
	}
	limit = min(limit, pageMax)
	c := cursor{T: r.End, ID: "ffffffff-ffff-ffff-ffff-ffffffffffff", B: branchID}
	if after != "" {
		b, err := base64.RawURLEncoding.DecodeString(after)
		if err != nil || json.Unmarshal(b, &c) != nil || c.B != branchID {
			return AuditPage{}, &ValidationError{Fields: map[string]string{"cursor": "invalid cursor"}}
		}
	}
	rows, err := s.pool.Query(ctx, q("audit_page"), branchID, r.Start, r.End, act, c.T, c.ID, limit+1)
	if err != nil {
		return AuditPage{}, err
	}
	items, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (AuditEvent, error) {
		var e AuditEvent
		var raw []byte
		var staff *string
		if err := row.Scan(&e.ID, &e.OccurredAt, &e.Action, &e.ResourceType, &e.ResourceID, &e.Reason, &e.RequestID, &raw, &staff); err != nil {
			return e, err
		}
		e.Actor = actorName(staff, e.Action)
		all := map[string]any{}
		_ = json.Unmarshal(raw, &all)
		e.Details = map[string]any{}
		for k, v := range all {
			if slices.Contains(detailAllowlist, k) {
				e.Details[k] = v
			}
		}
		return e, nil
	})
	if err != nil {
		return AuditPage{}, err
	}
	page := AuditPage{Items: items}
	if len(items) > limit {
		page.Items = items[:limit]
		last := page.Items[limit-1]
		b, _ := json.Marshal(cursor{T: last.OccurredAt, ID: last.ID, B: branchID})
		next := base64.RawURLEncoding.EncodeToString(b)
		page.NextCursor = &next
	}
	return page, nil
}
