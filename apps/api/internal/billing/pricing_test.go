package billing

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sqlSelectItems = "SELECT kind, item_key, price_cents, currency FROM pricing_items"

func seedItemsRows() *pgxmock.Rows {
	return pgxmock.NewRows([]string{"kind", "item_key", "price_cents", "currency"}).
		AddRow("plan", "free", int64(0), "CNY").
		AddRow("plan", "pro", int64(9900), "CNY").
		AddRow("plan", "agent_pro", int64(29900), "CNY").
		AddRow("plan", "team", int64(29900), "CNY").
		AddRow("plan", "business", int64(99900), "CNY").
		AddRow("verdict_template", "sla", int64(29900), "CNY").
		AddRow("verdict_template", "incident", int64(19900), "CNY").
		AddRow("verdict_template", "compliance", int64(49900), "CNY").
		AddRow("verdict_template", "legal", int64(99900), "CNY")
}

func newPricing(t *testing.T) (*DBPricing, pgxmock.PgxPoolIface) {
	t.Helper()
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	t.Cleanup(mock.Close)
	return NewDBPricing(mock), mock
}

func TestDBPricing_BasePrice_Plan(t *testing.T) {
	p, mock := newPricing(t)
	mock.ExpectQuery(sqlSelectItems).WillReturnRows(seedItemsRows())

	cents, cur, err := p.BasePrice(context.Background(), KindPlan, "pro")
	require.NoError(t, err)
	assert.Equal(t, int64(9900), cents)
	assert.Equal(t, "CNY", cur)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDBPricing_BasePrice_VerdictTemplate(t *testing.T) {
	p, mock := newPricing(t)
	mock.ExpectQuery(sqlSelectItems).WillReturnRows(seedItemsRows())

	cents, _, err := p.BasePrice(context.Background(), KindVerdictTemplate, "legal")
	require.NoError(t, err)
	assert.Equal(t, int64(99900), cents)
}

func TestDBPricing_BasePrice_Unknown(t *testing.T) {
	p, mock := newPricing(t)
	mock.ExpectQuery(sqlSelectItems).WillReturnRows(seedItemsRows())

	_, _, err := p.BasePrice(context.Background(), KindPlan, "never_heard")
	assert.ErrorIs(t, err, ErrUnknownItem)
}

func TestDBPricing_BasePrice_KindMismatch(t *testing.T) {
	// 同一个 item_key 在不同 kind 下的语义不同：'pro' 是 plan，不是 verdict_template
	p, mock := newPricing(t)
	mock.ExpectQuery(sqlSelectItems).WillReturnRows(seedItemsRows())

	_, _, err := p.BasePrice(context.Background(), KindVerdictTemplate, "pro")
	assert.ErrorIs(t, err, ErrUnknownItem)
}

func TestDBPricing_ValidItem(t *testing.T) {
	p, mock := newPricing(t)
	mock.ExpectQuery(sqlSelectItems).WillReturnRows(seedItemsRows())

	assert.True(t, p.ValidItem(context.Background(), KindPlan, "pro"))
	assert.True(t, p.ValidItem(context.Background(), KindVerdictTemplate, "sla"))
	assert.False(t, p.ValidItem(context.Background(), KindPlan, "nope"))
}

func TestDBPricing_CacheHitsOnSecondCall(t *testing.T) {
	p, mock := newPricing(t)
	mock.ExpectQuery(sqlSelectItems).WillReturnRows(seedItemsRows())

	_, _, err := p.BasePrice(context.Background(), KindPlan, "pro")
	require.NoError(t, err)
	_, _, err = p.BasePrice(context.Background(), KindVerdictTemplate, "sla")
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDBPricing_CacheReloadsAfterTTL(t *testing.T) {
	p, mock := newPricing(t)
	p.WithCacheTTL(0)
	mock.ExpectQuery(sqlSelectItems).WillReturnRows(seedItemsRows())
	mock.ExpectQuery(sqlSelectItems).WillReturnRows(seedItemsRows())

	_, _, err := p.BasePrice(context.Background(), KindPlan, "pro")
	require.NoError(t, err)
	_, _, err = p.BasePrice(context.Background(), KindPlan, "pro")
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

// ---- EffectivePrice ----

func TestEffectivePrice_NoCoupon_NoPromo_ReturnsBase(t *testing.T) {
	p, mock := newPricing(t)
	mock.ExpectQuery(sqlSelectItems).WillReturnRows(seedItemsRows())
	mock.ExpectQuery("FROM pricing_promotions").
		WithArgs(pgxmock.AnyArg(), "plan", "pro").
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "applies_kind", "applies_key", "discount_type", "value", "max_uses", "used_count",
		}))

	res, err := p.EffectivePrice(context.Background(), KindPlan, "pro", "")
	require.NoError(t, err)
	assert.Equal(t, int64(9900), res.BaseCents)
	assert.Equal(t, int64(9900), res.FinalCents)
	assert.Empty(t, res.PromotionID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEffectivePrice_FreePlan_IgnoresCoupon(t *testing.T) {
	p, mock := newPricing(t)
	mock.ExpectQuery(sqlSelectItems).WillReturnRows(seedItemsRows())

	res, err := p.EffectivePrice(context.Background(), KindPlan, "free", "ANYCODE")
	require.NoError(t, err)
	assert.Equal(t, int64(0), res.FinalCents)
	assert.Empty(t, res.PromotionID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEffectivePrice_AutoPromo_PicksBest(t *testing.T) {
	p, mock := newPricing(t)
	mock.ExpectQuery(sqlSelectItems).WillReturnRows(seedItemsRows())
	promoRows := pgxmock.NewRows([]string{
		"id", "applies_kind", "applies_key", "discount_type", "value", "max_uses", "used_count",
	}).
		AddRow("promo_a", strPtr("plan"), strPtr("pro"), "percent", int64(10), nil, int64(0)).      // 9900 - 990 = 8910
		AddRow("promo_b", strPtr("plan"), strPtr("pro"), "override", int64(900), nil, int64(0)).    // 900（最低）
		AddRow("promo_c", strPtr("plan"), nil, "amount", int64(500), nil, int64(0))                 // 9900 - 500 = 9400（plan 全场）
	mock.ExpectQuery("FROM pricing_promotions").
		WithArgs(pgxmock.AnyArg(), "plan", "pro").
		WillReturnRows(promoRows)

	res, err := p.EffectivePrice(context.Background(), KindPlan, "pro", "")
	require.NoError(t, err)
	assert.Equal(t, int64(900), res.FinalCents)
	assert.Equal(t, "promo_b", res.PromotionID)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestEffectivePrice_CrossKindGlobalPromo(t *testing.T) {
	// 全平台通用 8 折（applies_kind=NULL）应作用于 verdict_template
	p, mock := newPricing(t)
	mock.ExpectQuery(sqlSelectItems).WillReturnRows(seedItemsRows())
	mock.ExpectQuery("FROM pricing_promotions").
		WithArgs(pgxmock.AnyArg(), "verdict_template", "sla").
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "applies_kind", "applies_key", "discount_type", "value", "max_uses", "used_count",
		}).AddRow("promo_global", nil, nil, "percent", int64(20), nil, int64(0)))

	res, err := p.EffectivePrice(context.Background(), KindVerdictTemplate, "sla", "")
	require.NoError(t, err)
	assert.Equal(t, int64(29900), res.BaseCents)
	assert.Equal(t, int64(23920), res.FinalCents) // 29900 - 5980
	assert.Equal(t, "promo_global", res.PromotionID)
}

func TestEffectivePrice_Coupon_Valid_PercentDiscount(t *testing.T) {
	p, mock := newPricing(t)
	mock.ExpectQuery(sqlSelectItems).WillReturnRows(seedItemsRows())
	mock.ExpectQuery("FROM pricing_promotions\\s+WHERE coupon_code").
		WithArgs("WELCOME10", pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "applies_kind", "applies_key", "discount_type", "value", "max_uses", "used_count",
		}).AddRow("promo_w", strPtr("plan"), strPtr("pro"), "percent", int64(10), nil, int64(0)))

	res, err := p.EffectivePrice(context.Background(), KindPlan, "pro", "WELCOME10")
	require.NoError(t, err)
	assert.Equal(t, int64(8910), res.FinalCents)
	assert.Equal(t, "promo_w", res.PromotionID)
}

func TestEffectivePrice_Coupon_Invalid_ReturnsError(t *testing.T) {
	p, mock := newPricing(t)
	mock.ExpectQuery(sqlSelectItems).WillReturnRows(seedItemsRows())
	mock.ExpectQuery("FROM pricing_promotions\\s+WHERE coupon_code").
		WithArgs("WRONG", pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "applies_kind", "applies_key", "discount_type", "value", "max_uses", "used_count",
		})) // empty → QueryRow.Scan 返回 pgx.ErrNoRows

	_, err := p.EffectivePrice(context.Background(), KindPlan, "pro", "WRONG")
	assert.ErrorIs(t, err, ErrCouponInvalid)
}

func TestEffectivePrice_Coupon_WrongKind_ReturnsError(t *testing.T) {
	// promo 限定 verdict_template 但用户用在 plan 上
	p, mock := newPricing(t)
	mock.ExpectQuery(sqlSelectItems).WillReturnRows(seedItemsRows())
	mock.ExpectQuery("FROM pricing_promotions\\s+WHERE coupon_code").
		WithArgs("VERDICTONLY", pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "applies_kind", "applies_key", "discount_type", "value", "max_uses", "used_count",
		}).AddRow("promo_v", strPtr("verdict_template"), nil, "percent", int64(50), nil, int64(0)))

	_, err := p.EffectivePrice(context.Background(), KindPlan, "pro", "VERDICTONLY")
	assert.ErrorIs(t, err, ErrPromoNotForItem)
}

func TestEffectivePrice_Coupon_WrongKey_ReturnsError(t *testing.T) {
	p, mock := newPricing(t)
	mock.ExpectQuery(sqlSelectItems).WillReturnRows(seedItemsRows())
	mock.ExpectQuery("FROM pricing_promotions\\s+WHERE coupon_code").
		WithArgs("TEAMONLY", pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "applies_kind", "applies_key", "discount_type", "value", "max_uses", "used_count",
		}).AddRow("promo_t", strPtr("plan"), strPtr("team"), "percent", int64(20), nil, int64(0)))

	_, err := p.EffectivePrice(context.Background(), KindPlan, "pro", "TEAMONLY")
	assert.ErrorIs(t, err, ErrPromoNotForItem)
}

func TestEffectivePrice_Coupon_Exhausted(t *testing.T) {
	p, mock := newPricing(t)
	mock.ExpectQuery(sqlSelectItems).WillReturnRows(seedItemsRows())
	mock.ExpectQuery("FROM pricing_promotions\\s+WHERE coupon_code").
		WithArgs("LIMITED", pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{
			"id", "applies_kind", "applies_key", "discount_type", "value", "max_uses", "used_count",
		}).AddRow("promo_l", strPtr("plan"), strPtr("pro"), "percent", int64(50), i64Ptr(100), int64(100)))

	_, err := p.EffectivePrice(context.Background(), KindPlan, "pro", "LIMITED")
	assert.ErrorIs(t, err, ErrCouponExhausted)
}

func TestApplyPromo_EdgeCases(t *testing.T) {
	// applyPromo 输出不应低于 0
	assert.Equal(t, int64(0), applyPromo(100, "amount", 500))
	assert.Equal(t, int64(0), applyPromo(100, "percent", 200))
	assert.Equal(t, int64(0), applyPromo(100, "override", 0))
	assert.Equal(t, int64(50), applyPromo(100, "percent", 50))
	assert.Equal(t, int64(100), applyPromo(100, "unknown_type", 0))
}

func TestPromoApplies(t *testing.T) {
	cases := []struct {
		name    string
		row     promotionRow
		kind    ItemKind
		key     string
		applies bool
	}{
		{"global", promotionRow{appliesKind: nil, appliesKey: nil}, KindPlan, "pro", true},
		{"kind_match_key_null", promotionRow{appliesKind: strPtr("plan"), appliesKey: nil}, KindPlan, "pro", true},
		{"kind_mismatch", promotionRow{appliesKind: strPtr("verdict_template"), appliesKey: nil}, KindPlan, "pro", false},
		{"exact_match", promotionRow{appliesKind: strPtr("plan"), appliesKey: strPtr("pro")}, KindPlan, "pro", true},
		{"exact_kind_wrong_key", promotionRow{appliesKind: strPtr("plan"), appliesKey: strPtr("team")}, KindPlan, "pro", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.applies, promoApplies(&c.row, c.kind, c.key))
		})
	}
}

// ---- IncrementPromotionUsage ----

func TestIncrementPromotionUsage_OK(t *testing.T) {
	p, mock := newPricing(t)
	mock.ExpectExec("UPDATE pricing_promotions SET used_count").
		WithArgs("promo_x").
		WillReturnResult(pgxmock.NewResult("UPDATE", 1))

	require.NoError(t, p.IncrementPromotionUsage(context.Background(), "promo_x"))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestIncrementPromotionUsage_EmptyIDNoop(t *testing.T) {
	p, _ := newPricing(t)
	require.NoError(t, p.IncrementPromotionUsage(context.Background(), ""))
}

func TestIncrementPromotionUsage_DBError(t *testing.T) {
	p, mock := newPricing(t)
	mock.ExpectExec("UPDATE pricing_promotions SET used_count").
		WithArgs("promo_x").
		WillReturnError(errors.New("conn refused"))

	err := p.IncrementPromotionUsage(context.Background(), "promo_x")
	assert.Error(t, err)
}

// 验证 increment SQL 带 `max_uses IS NULL OR used_count < max_uses` 守卫,
// 防止 EffectivePrice→IncrementPromotionUsage 之间的 TOCTOU 让 used_count 超额。
func TestIncrementPromotionUsage_GuardsMaxUses(t *testing.T) {
	p, mock := newPricing(t)
	// pgxmock.ExpectExec uses regex; match the guard clause.
	mock.ExpectExec(`UPDATE pricing_promotions[\s\S]+used_count < max_uses`).
		WithArgs("promo_capped").
		WillReturnResult(pgxmock.NewResult("UPDATE", 0)) // already at limit → no rows affected
	require.NoError(t, p.IncrementPromotionUsage(context.Background(), "promo_capped"))
	require.NoError(t, mock.ExpectationsWereMet())
}

// ---- Invalidate ----

func TestInvalidate_ForcesReloadOnNextRead(t *testing.T) {
	p, mock := newPricing(t)
	// 默认 5min TTL,但 Invalidate 后应强制回库二次。
	mock.ExpectQuery(sqlSelectItems).WillReturnRows(seedItemsRows())
	mock.ExpectQuery(sqlSelectItems).WillReturnRows(seedItemsRows())

	_, _, err := p.BasePrice(context.Background(), KindPlan, "pro")
	require.NoError(t, err)

	p.Invalidate()

	_, _, err = p.BasePrice(context.Background(), KindPlan, "pro")
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

// ---- helpers ----

func strPtr(s string) *string { return &s }
func i64Ptr(i int64) *int64   { return &i }

func TestNewDBPricing_DefaultCacheTTL(t *testing.T) {
	p := NewDBPricing(nil)
	assert.Equal(t, 5*time.Minute, p.cacheTTL)
}
