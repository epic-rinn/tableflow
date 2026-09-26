package billing

import "testing"

// TestCalculateFixtures (BIL-A2): hand-computed examples, integer only.
func TestCalculateFixtures(t *testing.T) {
	excl := Policy{TaxMode: TaxExclusive, TaxBP: 700, ServiceBP: 1000}
	incl := Policy{TaxMode: TaxInclusive, TaxBP: 700, ServiceBP: 1000}
	cases := []struct {
		name     string
		gross    int64
		p        Policy
		discount int
		want     Totals
	}{
		// 100.00 THB, 10% service, 7% VAT added: 10000 + 1000 = 11000; tax 770.
		{"exclusive", 10000, excl, 0, Totals{10000, 0, 0, 10000, 1000, 770, 11770}},
		// Inclusive: 11000 × 700/10700 = 719.626… → 720, not added.
		{"inclusive", 10000, incl, 0, Totals{10000, 0, 0, 10000, 1000, 720, 11000}},
		// Fractional: 12345 × 10% = 1234.5 → 1235; (12345+1235)=13580 × 7% = 950.6 → 951.
		{"exclusive half up", 12345, excl, 0, Totals{12345, 0, 0, 12345, 1235, 951, 14531}},
		// 13580 × 700/10700 = 888.411… → 888.
		{"inclusive fractional", 12345, incl, 0, Totals{12345, 0, 0, 12345, 1235, 888, 13580}},
		// Discount 5% of 12345 = 617.25 → 617; net 11728; service 1172.8 → 1173;
		// base 12901 × 7% = 903.07 → 903.
		{"discount", 12345, excl, 500, Totals{12345, 500, 617, 11728, 1173, 903, 13804}},
		// Exactly half a satang rounds up: 50 × 1% = 0.5 → 1.
		{"half", 50, Policy{TaxMode: TaxExclusive, ServiceBP: 100}, 0, Totals{50, 0, 0, 50, 1, 0, 51}},
		{"unconfigured", 9999, Policy{TaxMode: TaxExclusive}, 0, Totals{9999, 0, 0, 9999, 0, 0, 9999}},
		{"empty", 0, excl, 0, Totals{}},
	}
	for _, c := range cases {
		if got := Calculate(c.gross, c.p, c.discount); got != c.want {
			t.Errorf("%s: got %+v want %+v", c.name, got, c.want)
		}
	}
}

// TestPerAggregateNotPerItem: three 33.33 items at 10% service round once
// (9999 × 10% = 999.9 → 1000), whereas per-item rounding would give 999.
func TestPerAggregateNotPerItem(t *testing.T) {
	got := Calculate(3*3333, Policy{TaxMode: TaxExclusive, ServiceBP: 1000}, 0)
	if got.ServiceSatang != 1000 {
		t.Fatalf("service = %d, want 1000", got.ServiceSatang)
	}
}

func TestLargeBillNoOverflow(t *testing.T) {
	gross := int64(1_000_000_000_00) // 1 billion THB
	got := Calculate(gross, Policy{TaxMode: TaxInclusive, TaxBP: 10000, ServiceBP: 10000}, 10000)
	if got.TotalSatang != 0 || got.DiscountSatang != gross {
		t.Fatalf("%+v", got)
	}
}
