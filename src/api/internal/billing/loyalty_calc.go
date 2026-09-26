package billing

// Tiers (LOY-003). Base has no discount.
const (
	TierBase   = "base"
	TierSilver = "silver"
	TierGold   = "gold"
)

// LoyaltyPolicy is one loyalty-policy version. Version 0 is the pilot default
// from specs/product/mvp.md, shown as not configured.
type LoyaltyPolicy struct {
	Version          int   `json:"version"`
	SatangPerPoint   int64 `json:"satang_per_point"`
	SilverThreshold  int64 `json:"silver_threshold_satang"`
	SilverDiscountBP int   `json:"silver_discount_bp"`
	GoldThreshold    int64 `json:"gold_threshold_satang"`
	GoldDiscountBP   int   `json:"gold_discount_bp"`
	Configured       bool  `json:"configured"`
}

// PilotLoyalty: 1 point per THB 100; Silver at THB 5,000 (3%), Gold at
// THB 15,000 (5%) — editable pilot defaults, not industry standards.
var PilotLoyalty = LoyaltyPolicy{SatangPerPoint: 10000, SilverThreshold: 500000, SilverDiscountBP: 300, GoldThreshold: 1500000, GoldDiscountBP: 500}

// Tier returns the tier earned by cumulative qualifying spend.
func (p LoyaltyPolicy) Tier(qualifying int64) string {
	switch {
	case qualifying >= p.GoldThreshold:
		return TierGold
	case qualifying >= p.SilverThreshold:
		return TierSilver
	}
	return TierBase
}

// DiscountBP is the single tier discount (no stacking).
func (p LoyaltyPolicy) DiscountBP(tier string) int {
	switch tier {
	case TierGold:
		return p.GoldDiscountBP
	case TierSilver:
		return p.SilverDiscountBP
	}
	return 0
}

// NextTier returns the next tier and its threshold, or "" at the top.
func (p LoyaltyPolicy) NextTier(qualifying int64) (string, int64) {
	switch p.Tier(qualifying) {
	case TierBase:
		return TierSilver, p.SilverThreshold
	case TierSilver:
		return TierGold, p.GoldThreshold
	}
	return "", 0
}

// EligibleSpend (LOY-004): post-discount food subtotal excluding service
// charge and tax. For tax-inclusive prices the food tax is extracted from the
// net food subtotal, rounded half up once; service-charge tax is never
// subtracted. Every menu item is food in MVP.
func EligibleSpend(t Totals, p Policy) int64 {
	net := t.GrossSatang - t.DiscountSatang
	if p.TaxMode == TaxInclusive && p.TaxBP > 0 {
		return net - roundHalfUp(net*int64(p.TaxBP), 10000+int64(p.TaxBP))
	}
	return net
}

// Points is floor(eligible / satang per point).
func Points(eligible, satangPerPoint int64) int64 {
	if satangPerPoint <= 0 || eligible <= 0 {
		return 0
	}
	return eligible / satangPerPoint
}
