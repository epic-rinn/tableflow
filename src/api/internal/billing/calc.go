package billing

// Tax modes (BIL-004).
const (
	TaxExclusive = "exclusive"
	TaxInclusive = "inclusive"
)

// Policy is one charge-policy version. Version 0 is the unconfigured
// default (exclusive, 0%, 0%); rates are basis points (1/100 of a percent).
type Policy struct {
	Version    int    `json:"version"`
	TaxMode    string `json:"tax_mode"`
	TaxBP      int    `json:"tax_bp"`
	ServiceBP  int    `json:"service_bp"`
	Configured bool   `json:"configured"`
}

// Totals are the aggregate charges of one bill, all in satang.
type Totals struct {
	GrossSatang    int64 `json:"gross_satang"`
	DiscountBP     int   `json:"discount_bp"`
	DiscountSatang int64 `json:"discount_satang"`
	NetSatang      int64 `json:"net_satang"`
	ServiceSatang  int64 `json:"service_satang"`
	TaxSatang      int64 `json:"tax_satang"`
	TotalSatang    int64 `json:"total_satang"`
}

// roundHalfUp returns n/d rounded half up for n >= 0, d > 0.
func roundHalfUp(n, d int64) int64 { return (2*n + d) / (2 * d) }

// Calculate is the canonical bill calculation (BIL-003/004): integer satang,
// each aggregate charge rounded half up exactly once, never per item.
// discountBP is the single eligible tier discount (0 for non-members until
// loyalty tasks supply one); every item is discount-eligible in MVP.
//
//	net     = gross − round(gross × discount)
//	service = round(net × service)
//	exclusive: tax = round((net + service) × tax), total = net + service + tax
//	inclusive: tax = round((net + service) × tax / (1 + tax)), total = net + service
//
// Inputs are bounded (rates ≤ 10000 bp; bill gross far below 10^14 satang),
// so no intermediate product overflows int64.
func Calculate(gross int64, p Policy, discountBP int) Totals {
	t := Totals{GrossSatang: gross, DiscountBP: discountBP}
	t.DiscountSatang = roundHalfUp(gross*int64(discountBP), 10000)
	t.NetSatang = gross - t.DiscountSatang
	t.ServiceSatang = roundHalfUp(t.NetSatang*int64(p.ServiceBP), 10000)
	base := t.NetSatang + t.ServiceSatang
	if p.TaxMode == TaxInclusive {
		t.TaxSatang = roundHalfUp(base*int64(p.TaxBP), 10000+int64(p.TaxBP))
		t.TotalSatang = base
	} else {
		t.TaxSatang = roundHalfUp(base*int64(p.TaxBP), 10000)
		t.TotalSatang = base + t.TaxSatang
	}
	return t
}
