package billing

import "testing"

// TestEligibleSpendFixtures (LOY-004): hand-computed examples.
func TestEligibleSpendFixtures(t *testing.T) {
	excl := Policy{TaxMode: TaxExclusive, TaxBP: 700, ServiceBP: 1000}
	incl := Policy{TaxMode: TaxInclusive, TaxBP: 700, ServiceBP: 1000}
	cases := []struct {
		name     string
		gross    int64
		p        Policy
		discount int
		eligible int64
		points   int64
	}{
		// Exclusive: eligible = net food subtotal; service and tax excluded.
		{"exclusive", 1234500, excl, 0, 1234500, 123},
		// Silver 3%: 12345.00 − 370.35 = 11974.65 → 119 points.
		{"exclusive discount", 1234500, excl, 300, 1197465, 119},
		// Inclusive: 10000.00 × 700/10700 = 654.205… → 654.21 food tax; 9345.79 → 93 points.
		{"inclusive", 1000000, incl, 0, 934579, 93},
		// Inclusive after Gold 5%: net 9500.00; tax 621.495… → 621.50; 8878.50 → 88.
		{"inclusive discount", 1000000, incl, 500, 887850, 88},
		{"below one point", 9999, excl, 0, 9999, 0},
	}
	for _, c := range cases {
		totals := Calculate(c.gross, c.p, c.discount)
		e := EligibleSpend(totals, c.p)
		if e != c.eligible || Points(e, PilotLoyalty.SatangPerPoint) != c.points {
			t.Errorf("%s: eligible %d points %d, want %d / %d", c.name, e, Points(e, PilotLoyalty.SatangPerPoint), c.eligible, c.points)
		}
	}
}

func TestTiers(t *testing.T) {
	p := PilotLoyalty
	for q, want := range map[int64]string{0: TierBase, 499999: TierBase, 500000: TierSilver, 1499999: TierSilver, 1500000: TierGold} {
		if got := p.Tier(q); got != want {
			t.Errorf("tier(%d) = %s, want %s", q, got, want)
		}
	}
	if p.DiscountBP(TierSilver) != 300 || p.DiscountBP(TierGold) != 500 || p.DiscountBP(TierBase) != 0 {
		t.Fatal("discounts")
	}
	if n, th := p.NextTier(600000); n != TierGold || th != 1500000 {
		t.Fatalf("next tier %s %d", n, th)
	}
}
