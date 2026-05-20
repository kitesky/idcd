#!/usr/bin/env bash
# D-Concern1: 禁止在拨测模块（非 Evidence 模块）使用伪 attestation 营销词
# 避免把普通拨测结果包装成"区块链认证/防篡改"等误导性描述

# 让脚本独立于调用者 cwd: cd 到 backend root (scripts 的上一层)
cd "$(cd "$(dirname "$0")/.." && pwd)" || exit 1

PROBE_DIRS="apps/api apps/agent apps/gateway apps/aggregator apps/scheduler"
EVIDENCE_DIRS="apps/verifier"  # 这些目录允许使用 attestation 词汇

# 禁用词（不区分大小写）
FORBIDDEN='(blockchain[\-_ ]?verified|tamper[\-_ ]?proof|certified[\-_ ]?result|notarized|不可篡改证明|区块链认证拨测)'

FAILED=0

echo "D-Concern1 检查：probe 模块 attestation 滥用词..."

for DIR in $PROBE_DIRS; do
  [ -d "$DIR" ] || continue
  MATCHES=$(grep -rn --include="*.go" -iE "$FORBIDDEN" "$DIR" 2>/dev/null || true)
  if [ -n "$MATCHES" ]; then
    echo "$MATCHES"
    echo ""
    echo "ERROR: $DIR 中发现禁用 attestation 词（D-Concern1 违规）"
    echo "这些词只能出现在 Evidence 模块（$EVIDENCE_DIRS）"
    FAILED=1
  fi
done

if [ "$FAILED" = "0" ]; then
  echo "OK — 未发现 attestation 滥用词"
fi

exit "$FAILED"
