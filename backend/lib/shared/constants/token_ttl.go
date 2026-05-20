package constants

import "time"

// MCP token 各类型有效期 (对应 D2 决策).
//
// D2 决策摘要: Token 90d 上限, 无永久 token. Personal 24h / Workspace 90d /
// Service 90d auto_renewal. MCP units 与 API 配额完全独立池.
const (
	// MCPTokenPersonalTTL 是 MCP personal token 默认有效期.
	// 用于开发人员临时登录, 24h 后必须刷新.
	MCPTokenPersonalTTL = 24 * time.Hour

	// MCPTokenWorkspaceTTL 是 MCP workspace 集成 token 有效期上限.
	// 工作区集成自动续期, 但单次最多 90 天.
	MCPTokenWorkspaceTTL = 90 * 24 * time.Hour

	// MCPTokenServiceTTL 是 MCP service-to-service token 有效期上限.
	// auto_renewal 启用, 但单次签发不得超过 90 天.
	MCPTokenServiceTTL = 90 * 24 * time.Hour
)
