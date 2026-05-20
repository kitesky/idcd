// Package pagination centralises the page-size contract that every idcd HTTP
// list endpoint uses.
//
// 背景
//
// 之前 cert-svc / api 各自的 handler 都把分页默认值和上限写死成
// `default=20, max=100`（admin 是 `default=50`），同一个数字在 7+ 处出现。
// 一旦产品想把 max 从 100 抬到 200，或者想给 admin 一个不同的 default，就要
// 在每个文件里逐个改且容易漏。
//
// 现在所有 handler 都通过 pagination.Clamp / pagination.ClampWith 读取这套
// 常量，业务方调整只改本文件即可。
//
// 调整指引
//
//   - DefaultPageSize / MaxPageSize 是对**公开 API**的承诺，调整需同步更新
//     文档（apps/web/app/docs/api/pagination.mdx 等）以及客户端 SDK 默认值。
//   - AdminDefaultPageSize 仅影响 admin 后台 UX，可以更激进地调大。
//   - RepoMaxPageSize 是 repository 层的硬上限，handler 在传值前必须先
//     Clamp 到 MaxPageSize；这层只是兜底，防止内部脚本不小心传超大值
//     导致 PG 慢查询 / 内存占用飙升。
package pagination

const (
	// DefaultPageSize is the page size returned when the client does not
	// pass ?limit= (or passes a non-positive value).
	DefaultPageSize = 20

	// MaxPageSize is the upper bound a public API client may request on
	// a single page. Larger values are silently clamped down.
	// 想抬高需同步评估: DB 慢查询、网关响应体大小、移动端流量。
	MaxPageSize = 100

	// AdminDefaultPageSize is the default page size used by admin-only
	// list endpoints. Admins typically want to scan more rows at once
	// (auditing, troubleshooting) so the default is higher than the
	// public default.
	AdminDefaultPageSize = 50

	// RepoMaxPageSize is the hard cap enforced inside repository layers
	// (raw SQL). Handlers must clamp public input to MaxPageSize before
	// calling into the repo; this larger bound only exists so internal
	// jobs (migrations, exports) can pull bigger batches deliberately.
	RepoMaxPageSize = 200
)

// Clamp normalises a client-supplied limit to the public API contract:
//
//   - limit <= 0      → DefaultPageSize
//   - limit > MaxPage → MaxPageSize
//   - otherwise        → limit unchanged
//
// 用于所有公开 list 接口的 handler。
func Clamp(limit int) int {
	return ClampWith(limit, DefaultPageSize, MaxPageSize)
}

// ClampWith is Clamp with caller-provided default / max.
//
// 仅在端点有**正式记录**的不同分页契约时才用，比如 admin 端点用
//
//	pagination.ClampWith(raw, pagination.AdminDefaultPageSize, pagination.MaxPageSize)
//
// 或 repository 层用
//
//	pagination.ClampWith(raw, pagination.AdminDefaultPageSize, pagination.RepoMaxPageSize)
//
// 不要把这个函数当成"绕过 Clamp 的逃生口"。
func ClampWith(limit, defaultSize, maxSize int) int {
	if limit <= 0 {
		return defaultSize
	}
	if limit > maxSize {
		return maxSize
	}
	return limit
}
