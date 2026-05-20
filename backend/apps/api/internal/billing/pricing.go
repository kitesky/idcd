package billing

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/kite365/idcd/lib/shared/idgen"
)

// PricingPool 是 Pricing 用到的 pgxpool 子集；*pgxpool.Pool 和 pgxmock 都满足。
type PricingPool interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// ItemKind 是付费品类。新增品类时只需要在这里加常量 + 同步 migration 00049 的 CHECK。
type ItemKind string

const (
	KindPlan            ItemKind = "plan"
	KindVerdictTemplate ItemKind = "verdict_template"
)

// PricingError 表示用户/调用方可恢复的定价错误（4xx 而非 5xx）。
type PricingError struct{ Reason string }

func (e *PricingError) Error() string { return "billing: pricing: " + e.Reason }

var (
	ErrUnknownItem     = &PricingError{Reason: "unknown pricing item"}
	ErrCouponInvalid   = &PricingError{Reason: "coupon code invalid or expired"}
	ErrCouponExhausted = &PricingError{Reason: "coupon usage limit reached"}
	ErrPromoNotForItem = &PricingError{Reason: "promotion not applicable to this item"}
)

// EffectivePriceResult 是 EffectivePrice 的返回。
// 调用方拿 FinalCents 去支付平台，BaseCents 用于 UI 显示"原价"。
// PromotionID 不为空表示命中了某条促销，订单持久化时应回写到对应表的 promotion_id
// 并调 IncrementPromotionUsage。
type EffectivePriceResult struct {
	BaseCents   int64
	FinalCents  int64
	Currency    string
	PromotionID string
}

// Pricing 是统一定价服务接口（plan / verdict / 未来证书 / API 加量包等共用）。
// 所有方法 ctx-aware；实现需保证并发安全。
type Pricing interface {
	BasePrice(ctx context.Context, kind ItemKind, itemKey string) (priceCents int64, currency string, err error)
	EffectivePrice(ctx context.Context, kind ItemKind, itemKey string, couponCode string) (*EffectivePriceResult, error)
	ValidItem(ctx context.Context, kind ItemKind, itemKey string) bool
	IncrementPromotionUsage(ctx context.Context, promotionID string) error
}

// ---- DB-backed implementation ----

type priceEntry struct {
	cents    int64
	currency string
}

// 用 string 拼 "kind|key" 作 map key 比组合结构体更简单且零开销。
func cacheKey(kind ItemKind, itemKey string) string { return string(kind) + "|" + itemKey }

type priceSnapshot struct {
	prices    map[string]priceEntry
	expiresAt time.Time
}

// DBPricing 从 pricing_items / pricing_promotions 表读，5min in-memory cache。
// admin 改价后等下次 cache 过期才生效；活动定时启停不需要秒级精度。
type DBPricing struct {
	pool     PricingPool
	cacheTTL time.Duration

	mu    sync.RWMutex
	cache *priceSnapshot
}

// NewDBPricing 构造一个默认 5min cache TTL 的 Pricing 实例。
func NewDBPricing(pool PricingPool) *DBPricing {
	return &DBPricing{pool: pool, cacheTTL: 5 * time.Minute}
}

// WithCacheTTL 覆盖默认 TTL（测试时改成 0 强制每次回库）。
func (p *DBPricing) WithCacheTTL(d time.Duration) *DBPricing {
	p.cacheTTL = d
	return p
}

// Invalidate 清掉 in-memory cache,下一次读会强制回库。
// admin 改价 / 改促销后调用,避免最长 cacheTTL 的 stale 窗口。
// 多实例部署下仅本进程生效;HA 集群另需 PubSub 广播。
func (p *DBPricing) Invalidate() {
	p.mu.Lock()
	p.cache = nil
	p.mu.Unlock()
}

// snapshot 返回当前 cache；过期/未初始化则回库 reload。
func (p *DBPricing) snapshot(ctx context.Context) (*priceSnapshot, error) {
	p.mu.RLock()
	cur := p.cache
	p.mu.RUnlock()
	if cur != nil && time.Now().Before(cur.expiresAt) {
		return cur, nil
	}

	rows, err := p.pool.Query(ctx, `SELECT kind, item_key, price_cents, currency FROM pricing_items`)
	if err != nil {
		return nil, fmt.Errorf("billing/pricing: load pricing_items: %w", err)
	}
	defer rows.Close()

	next := &priceSnapshot{
		prices:    make(map[string]priceEntry),
		expiresAt: time.Now().Add(p.cacheTTL),
	}
	for rows.Next() {
		var kind, key string
		var entry priceEntry
		if err := rows.Scan(&kind, &key, &entry.cents, &entry.currency); err != nil {
			return nil, fmt.Errorf("billing/pricing: scan pricing_items: %w", err)
		}
		next.prices[cacheKey(ItemKind(kind), key)] = entry
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("billing/pricing: iterate pricing_items: %w", err)
	}

	p.mu.Lock()
	p.cache = next
	p.mu.Unlock()
	return next, nil
}

// BasePrice 返回某商品的基础价；不存在返回 ErrUnknownItem。
func (p *DBPricing) BasePrice(ctx context.Context, kind ItemKind, itemKey string) (int64, string, error) {
	snap, err := p.snapshot(ctx)
	if err != nil {
		return 0, "", err
	}
	e, ok := snap.prices[cacheKey(kind, itemKey)]
	if !ok {
		return 0, "", ErrUnknownItem
	}
	return e.cents, e.currency, nil
}

// ValidItem 判断 (kind, itemKey) 是否数据库认识的商品。
func (p *DBPricing) ValidItem(ctx context.Context, kind ItemKind, itemKey string) bool {
	snap, err := p.snapshot(ctx)
	if err != nil {
		return false
	}
	_, ok := snap.prices[cacheKey(kind, itemKey)]
	return ok
}

// promotionRow 是 pricing_promotions 单行投影。
type promotionRow struct {
	id           string
	appliesKind  *string
	appliesKey   *string
	discountType string
	value        int64
	maxUses      *int64
	usedCount    int64
}

// EffectivePrice 计算最终价。匹配规则：
//   - 用户提供 couponCode（trim 后非空）：必须命中该 code 的 active + 时段内 + 未超用量；
//     code 命中后检查适用范围：
//       applies_kind=NULL → 全平台通用
//       applies_kind=kind, applies_key=NULL → 该品类全场
//       applies_kind=kind, applies_key=itemKey → 单品命中
//   - 用户未提供 code：自动应用一条 active + 时段内 + coupon_code IS NULL + 适用范围命中 + 未超用量；
//     若同时多条命中取最优（最终价最低）；都未命中则返回基础价
//   - 基础价为 0（如 free plan）永远返回 0，promo 不适用
func (p *DBPricing) EffectivePrice(ctx context.Context, kind ItemKind, itemKey string, couponCode string) (*EffectivePriceResult, error) {
	baseCents, currency, err := p.BasePrice(ctx, kind, itemKey)
	if err != nil {
		return nil, err
	}
	res := &EffectivePriceResult{BaseCents: baseCents, FinalCents: baseCents, Currency: currency}
	if baseCents == 0 {
		return res, nil
	}

	coupon := strings.TrimSpace(couponCode)

	if coupon != "" {
		row, err := p.lookupCouponPromo(ctx, coupon)
		if err != nil {
			return nil, err
		}
		if row == nil {
			return nil, ErrCouponInvalid
		}
		if !promoApplies(row, kind, itemKey) {
			return nil, ErrPromoNotForItem
		}
		if row.maxUses != nil && row.usedCount >= *row.maxUses {
			return nil, ErrCouponExhausted
		}
		res.FinalCents = applyPromo(baseCents, row.discountType, row.value)
		res.PromotionID = row.id
		return res, nil
	}

	rows, err := p.listAutoPromos(ctx, kind, itemKey)
	if err != nil {
		return nil, err
	}
	best := baseCents
	bestID := ""
	for _, r := range rows {
		if r.maxUses != nil && r.usedCount >= *r.maxUses {
			continue
		}
		cand := applyPromo(baseCents, r.discountType, r.value)
		if cand < best {
			best = cand
			bestID = r.id
		}
	}
	res.FinalCents = best
	res.PromotionID = bestID
	return res, nil
}

// promoApplies 检查促销的适用范围是否覆盖请求的 (kind, itemKey)。
func promoApplies(row *promotionRow, kind ItemKind, itemKey string) bool {
	if row.appliesKind == nil {
		return true // 全平台通用
	}
	if *row.appliesKind != string(kind) {
		return false
	}
	if row.appliesKey == nil {
		return true // 该品类全场
	}
	return *row.appliesKey == itemKey
}

func (p *DBPricing) lookupCouponPromo(ctx context.Context, code string) (*promotionRow, error) {
	now := time.Now().UTC()
	row := p.pool.QueryRow(ctx, `
		SELECT id, applies_kind, applies_key, discount_type, value, max_uses, used_count
		FROM pricing_promotions
		WHERE coupon_code = $1
		  AND active = TRUE
		  AND start_at <= $2 AND end_at > $2
	`, code, now)
	var r promotionRow
	if err := row.Scan(&r.id, &r.appliesKind, &r.appliesKey, &r.discountType, &r.value, &r.maxUses, &r.usedCount); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("billing/pricing: lookup coupon: %w", err)
	}
	return &r, nil
}

// listAutoPromos 列出对 (kind, itemKey) 当前生效且无需 coupon 的促销。
// SQL 用 applies_kind IS NULL OR applies_kind = $1 覆盖跨品类，applies_key 同理。
func (p *DBPricing) listAutoPromos(ctx context.Context, kind ItemKind, itemKey string) ([]promotionRow, error) {
	now := time.Now().UTC()
	rows, err := p.pool.Query(ctx, `
		SELECT id, applies_kind, applies_key, discount_type, value, max_uses, used_count
		FROM pricing_promotions
		WHERE coupon_code IS NULL
		  AND active = TRUE
		  AND start_at <= $1 AND end_at > $1
		  AND (applies_kind IS NULL OR applies_kind = $2)
		  AND (applies_key  IS NULL OR applies_key  = $3)
	`, now, string(kind), itemKey)
	if err != nil {
		return nil, fmt.Errorf("billing/pricing: list auto promos: %w", err)
	}
	defer rows.Close()
	out := make([]promotionRow, 0, 4)
	for rows.Next() {
		var r promotionRow
		if err := rows.Scan(&r.id, &r.appliesKind, &r.appliesKey, &r.discountType, &r.value, &r.maxUses, &r.usedCount); err != nil {
			return nil, fmt.Errorf("billing/pricing: scan auto promo: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// applyPromo 在 baseCents 上应用一条 promo，返回最终价（不低于 0）。
func applyPromo(baseCents int64, discountType string, value int64) int64 {
	var out int64
	switch discountType {
	case "percent":
		// value 是百分比折扣（如 10 = 减 10%）
		out = baseCents - baseCents*value/100
	case "amount":
		out = baseCents - value
	case "override":
		out = value
	default:
		return baseCents
	}
	if out < 0 {
		return 0
	}
	return out
}

// IncrementPromotionUsage 订单创建成功后调用,原子 +1 used_count。
// WHERE 条件 `max_uses IS NULL OR used_count < max_uses` 防止超额累加 ——
// EffectivePrice 与本调用之间存在 TOCTOU 窗口,高并发下两个请求都可能通过
// 用量检查再都来 +1。WHERE 守卫确保 used_count 永不超过 max_uses,代价是
// RowsAffected=0 时本次支付不计数(已成功支付仍生效;用户拿到了优惠价就是赚
// 的,DB 一致性优先于精确计数)。
//
// 不阻塞主流程:返回 error 调用方应只 log,不回滚已成功的支付。
func (p *DBPricing) IncrementPromotionUsage(ctx context.Context, promotionID string) error {
	if promotionID == "" {
		return nil
	}
	_, err := p.pool.Exec(ctx, `
		UPDATE pricing_promotions
		SET used_count = used_count + 1, updated_at = NOW()
		WHERE id = $1
		  AND (max_uses IS NULL OR used_count < max_uses)
	`, promotionID)
	if err != nil {
		return fmt.Errorf("billing/pricing: increment usage %s: %w", promotionID, err)
	}
	return nil
}

// NewPromotionID 生成 promo_xxx 前缀的 ID（admin CRUD 使用）。
func NewPromotionID() string { return idgen.New("promo_") }
