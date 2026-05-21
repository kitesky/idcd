#!/usr/bin/env bash
# P0-4 (ARCHITECTURE-REVIEW-2026-05-21.md):
# 跨 stream 边界禁止用 map[string]any 自由发挥字段名。
#
# 新代码必须改用以下 typed API + lib/shared/contracts 强类型 payload:
#   * stream.Client.AddProbeResultTyped       (W1)
#   * stream.Client.AddMonitorEventTyped      (W1)
#   * stream.Client.AddCertNotificationTyped  (W2)
#   * stream.Client.AddRefundInitiateTyped    (W3 — 钱相关 / D5 退款入口)
#   * stream.Client.AddRefundRetryTyped       (W3 — 钱相关 / D5 retry ladder)
#
# 受保护的 stream 名 (出现在 XAdd Stream 参数里就报警):
#   * "probe.results"          (W1)
#   * "monitor.events"         (W1)
#   * "cert:notifications"     (W2 — 钱相关 / 合规相关)
#   * "refund_initiate_queue"  (W3 — Self-Verify → refund-worker)
#   * "refund_retry_queue"     (W3 — PaymentHub webhook → refund-worker)
#
# 旧代码渐进迁移: 在调用上一行加注释 `// LINT-IGNORE: stream-payload-legacy`,
# 脚本会跳过, CI 不阻塞。每次迁移一个 caller 就删掉注释。
#
# 退出码: 0 = 通过, 1 = 发现违规

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "P0-4 检查: 禁用裸 map[string]any 跨 stream 边界..."

# 找出 AddProbeResult/AddMonitorEvent 的调用 (不带 Typed 后缀, 不在 stream/ 包自身, 不是测试)
# grep 输出格式: file:line:content
candidates=$(
  grep -rn -E '\.(AddProbeResult|AddMonitorEvent)\(' \
    --include='*.go' \
    --exclude-dir='lib/shared/stream' \
    --exclude-dir='lib/shared/contracts' \
    "$REPO_ROOT" 2>/dev/null \
  | grep -v -E 'AddProbeResultTyped|AddMonitorEventTyped' \
  | grep -v '_test\.go:' \
  || true
)

# 额外检查 cert:notifications (W2) — 该流没有非 Typed 的 stream.Client 方法,
# 旧调用方直接 rdb.XAdd(Stream: "cert:notifications", ...). 直接 grep raw XAdd
# 对该 stream 的引用, 排除 stream.Client.AddCertNotificationTyped 自己内部的
# 那次调用 (走的是 c.Add() 间接 XAdd, 不会匹配字符串)。
cert_candidates=$(
  grep -rn -F 'cert:notifications' \
    --include='*.go' \
    --exclude-dir='lib/shared/stream' \
    --exclude-dir='lib/shared/contracts' \
    "$REPO_ROOT" 2>/dev/null \
  | grep -E 'XAdd\(' \
  | grep -v 'AddCertNotificationTyped' \
  | grep -v '_test\.go:' \
  || true
)
if [ -n "$cert_candidates" ]; then
  candidates="${candidates}${cert_candidates}"$'\n'
fi

# 额外检查 refund_initiate_queue + refund_retry_queue (W3) — 类似 W2,
# 这两条流没有非 Typed 的 stream.Client 方法。任何 raw rdb.XAdd 对这两条
# 流的引用一律触发, 除了 lint-ignore 注释或 AddRefund*Typed 包装。
refund_candidates=$(
  grep -rn -E -F -e 'refund_initiate_queue' -e 'refund_retry_queue' \
    --include='*.go' \
    --exclude-dir='lib/shared/stream' \
    --exclude-dir='lib/shared/contracts' \
    "$REPO_ROOT" 2>/dev/null \
  | grep -E 'XAdd\(' \
  | grep -v -E 'AddRefundInitiateTyped|AddRefundRetryTyped' \
  | grep -v '_test\.go:' \
  || true
)
if [ -n "$refund_candidates" ]; then
  candidates="${candidates}${refund_candidates}"$'\n'
fi

if [ -z "$candidates" ]; then
  echo "OK — 未发现非 Typed 调用"
  exit 0
fi

# 对每个候选 hit, 看前 5 行内是否有 LINT-IGNORE 注释 (允许中间夹 TODO / 解释注释)
violations=""
while IFS= read -r line; do
  [ -z "$line" ] && continue
  file=$(echo "$line" | cut -d: -f1)
  lineno=$(echo "$line" | cut -d: -f2)
  start=$((lineno - 5))
  if [ "$start" -lt 1 ]; then start=1; fi
  end=$((lineno - 1))
  if [ "$end" -lt 1 ]; then
    violations="$violations$line"$'\n'
    continue
  fi
  context=$(sed -n "${start},${end}p" "$file" 2>/dev/null || echo "")
  if echo "$context" | grep -q 'LINT-IGNORE: stream-payload-legacy'; then
    continue  # whitelisted
  fi
  violations="$violations$line"$'\n'
done <<< "$candidates"

if [ -n "$(echo -n "$violations")" ]; then
  echo ""
  echo "ERROR: 发现非 Typed stream 调用 (P0-4 违规):"
  echo "$violations"
  echo ""
  echo "选项 1 (推荐 - 新代码): 改用 AddProbeResultTyped / AddMonitorEventTyped / AddCertNotificationTyped"
  echo "  c.AddProbeResultTyped(ctx, contracts.ProbeResult{TaskID: \"...\", ...})"
  echo "  c.AddCertNotificationTyped(ctx, contracts.CertNotificationEvent{EventType: \"cert.issued\", ...})"
  echo ""
  echo "选项 2 (旧代码渐进迁移): 在调用上一行加注释"
  echo "  // LINT-IGNORE: stream-payload-legacy"
  echo "  c.AddProbeResult(ctx, taskID, nodeID, payload)"
  echo ""
  echo "详见 backend/lib/shared/contracts/doc.go 和 docs/prd/ARCHITECTURE-REVIEW-2026-05-21.md P0-4"
  exit 1
fi

echo "OK — 所有非 Typed 调用都已加 LINT-IGNORE: stream-payload-legacy 注释"
