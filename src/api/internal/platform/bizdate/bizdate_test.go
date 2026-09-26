package bizdate

import (
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	r, err := Parse("Asia/Bangkok", "2026-09-01", "2026-09-03")
	if err != nil {
		t.Fatal(err)
	}
	if got := r.Start.UTC().Format(time.RFC3339); got != "2026-08-31T17:00:00Z" {
		t.Fatalf("start %s", got)
	}
	if got := r.End.UTC().Format(time.RFC3339); got != "2026-09-03T17:00:00Z" {
		t.Fatalf("end %s", got)
	}
	if len(r.Dates) != 3 || r.Dates[2] != "2026-09-03" {
		t.Fatalf("dates %v", r.Dates)
	}
	if _, err := Parse("Asia/Bangkok", "2026-09-01", "2026-10-01"); err != nil { // 31 days
		t.Fatalf("31 days rejected: %v", err)
	}
	for _, c := range [][2]string{{"2026-09-01", "2026-10-02"}, {"2026-09-02", "2026-09-01"}, {"2026-9-1", "2026-09-01"}, {"", ""}} {
		if _, err := Parse("Asia/Bangkok", c[0], c[1]); err != ErrRange {
			t.Fatalf("%v accepted", c)
		}
	}
}
