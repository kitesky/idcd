package handler

import (
	"context"

	"github.com/kite365/idcd/apps/api/internal/billing"
)

// pricingTestPrices mirrors the seed in
// lib/db/migrations/idcd_main/00049_pricing_tables.sql.
// Handler tests use fakePricing (below) to satisfy billing.Pricing without a
// real DB; tests that need promo / coupon failure paths should plug a richer
// mock at the call site.
var pricingTestPrices = map[billing.ItemKind]map[string]int64{
	billing.KindPlan: {
		"free":      0,
		"pro":       9900,
		"agent_pro": 29900,
		"team":      29900,
		"business":  99900,
	},
	billing.KindVerdictTemplate: {
		"sla":        29900,
		"incident":   19900,
		"compliance": 49900,
		"legal":      99900,
	},
}

type fakePricing struct{}

func (fakePricing) BasePrice(_ context.Context, kind billing.ItemKind, key string) (int64, string, error) {
	items, ok := pricingTestPrices[kind]
	if !ok {
		return 0, "", billing.ErrUnknownItem
	}
	cents, ok := items[key]
	if !ok {
		return 0, "", billing.ErrUnknownItem
	}
	return cents, "CNY", nil
}

func (fp fakePricing) EffectivePrice(ctx context.Context, kind billing.ItemKind, key string, _ string) (*billing.EffectivePriceResult, error) {
	cents, currency, err := fp.BasePrice(ctx, kind, key)
	if err != nil {
		return nil, err
	}
	return &billing.EffectivePriceResult{BaseCents: cents, FinalCents: cents, Currency: currency}, nil
}

func (fakePricing) ValidItem(_ context.Context, kind billing.ItemKind, key string) bool {
	items, ok := pricingTestPrices[kind]
	if !ok {
		return false
	}
	_, ok = items[key]
	return ok
}

func (fakePricing) IncrementPromotionUsage(_ context.Context, _ string) error { return nil }
