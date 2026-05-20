// Package constants is the SSOT for cross-service business-semantic constants.
//
// 设计动机 (ARCHITECTURE-REVIEW-2026-05-21.md P1-9):
//
// 业务常量散落在十几个 Go 文件里 — Token TTL 在 auth handler / mcp server /
// 后台清理 job 各写一次 `90 * 24 * time.Hour`, 想把 90 天调成 60 天得 grep
// 全代码库。这个包把所有"对应 docs/prd/DECISIONS.md 中某条 D 决策"的常量
// 集中托管, 改值时先改 DECISIONS.md, 然后所有引用处自动跟随。
//
// # 文件分类
//
//   - sla.go         Verdict / 客服 SLA 阶梯 (D12)
//   - token_ttl.go   MCP token 各类型有效期 (D2)
//   - retention.go   数据保留期 (probe_result 热/冷存档, cert order, 等)
//   - timeout.go     杂项业务 timeout (WebAuthn challenge, stream claim, monitor flap)
//
// # 命名约定
//
//   - 时长一律 time.Duration, 不要 int 秒
//   - 字段名含单位 (... TTL / ... Timeout / ... Retention), 不留歧义
//   - 显式给出每个常量对应的 D 决策编号, 改值前先回溯文档
//
// # 这个包不做的事
//
// 不主动 grep 替换现有硬编码字面值, 那是另一个 PR 的事。本包只是把常量库
// 搭起来供未来使用 + 新代码强制走这里。
package constants
