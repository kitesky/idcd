#!/usr/bin/env bash
# D1: 禁止跨 schema FOREIGN KEY REFERENCES
# CI 如发现 REFERENCES other_schema.table 格式则报错

MIGRATION_DIR="packages/db/migrations"
FAILED=0

if [ ! -d "$MIGRATION_DIR" ]; then
  echo "migration 目录不存在，跳过"
  exit 0
fi

echo "D1 检查：跨 schema REFERENCES..."

# REFERENCES 后紧跟 schema.table 格式（跨 schema FK）
if grep -rn --include="*.sql" -E \
  'REFERENCES[[:space:]]+[a-z_][a-z_0-9]*\.[a-z_][a-z_0-9]*[[:space:]]*\(' \
  "$MIGRATION_DIR" 2>/dev/null; then
  echo ""
  echo "ERROR: 发现跨 schema FOREIGN KEY（D1 违规）"
  echo "规则：不同 schema 之间禁止写 FK，改用 Repository 应用层 join"
  FAILED=1
fi

if [ "$FAILED" = "0" ]; then
  echo "OK — 未发现跨 schema REFERENCES"
fi

exit "$FAILED"
