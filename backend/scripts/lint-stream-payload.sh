#!/usr/bin/env bash
# P0-4 (ARCHITECTURE-REVIEW-2026-05-21.md):
# 跨 stream 边界禁止用 map[string]any 自由发挥字段名。
#
# 新代码必须改用 stream.Client.{AddProbeResultTyped,AddMonitorEventTyped}
# 配合 backend/lib/shared/contracts 中的 ProbeResult / MonitorEvent 强类型。
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
  echo "选项 1 (推荐 - 新代码): 改用 AddProbeResultTyped / AddMonitorEventTyped"
  echo "  c.AddProbeResultTyped(ctx, contracts.ProbeResult{TaskID: \"...\", ...})"
  echo ""
  echo "选项 2 (旧代码渐进迁移): 在调用上一行加注释"
  echo "  // LINT-IGNORE: stream-payload-legacy"
  echo "  c.AddProbeResult(ctx, taskID, nodeID, payload)"
  echo ""
  echo "详见 backend/lib/shared/contracts/doc.go 和 docs/prd/ARCHITECTURE-REVIEW-2026-05-21.md P0-4"
  exit 1
fi

echo "OK — 所有非 Typed 调用都已加 LINT-IGNORE: stream-payload-legacy 注释"
