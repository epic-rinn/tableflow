// Package bizdate converts business-date ranges (calendar dates in a
// branch's timezone) into instant ranges for reporting queries (OPS-002).
package bizdate

import (
	"errors"
	"time"
	_ "time/tzdata" // branch timezones resolve without system tzdata
)

// MaxDays caps interactive report and detail ranges (OPS-002).
const MaxDays = 31

// ErrRange describes an invalid or oversized range.
var ErrRange = errors.New("date range must be YYYY-MM-DD, from ≤ to, at most 31 days")

// Range is an inclusive business-date range and its half-open instant span.
type Range struct {
	Location   *time.Location
	From, To   string    // YYYY-MM-DD, inclusive
	Start, End time.Time // [Start, End)
	Dates      []string  // every date in the range
}

// Parse validates from/to in tz. Midnight is taken in the branch timezone,
// so a Bangkok business day is 17:00Z–17:00Z.
func Parse(tz, from, to string) (Range, error) {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return Range{}, err
	}
	f, err1 := time.ParseInLocation(time.DateOnly, from, loc)
	t, err2 := time.ParseInLocation(time.DateOnly, to, loc)
	if err1 != nil || err2 != nil || t.Before(f) {
		return Range{}, ErrRange
	}
	r := Range{Location: loc, From: from, To: to, Start: f}
	for d := f; !d.After(t); d = d.AddDate(0, 0, 1) {
		r.Dates = append(r.Dates, d.Format(time.DateOnly))
		if len(r.Dates) > MaxDays {
			return Range{}, ErrRange
		}
	}
	r.End = t.AddDate(0, 0, 1)
	return r, nil
}
