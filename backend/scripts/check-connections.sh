#!/usr/bin/env bash
# make dev-up — 验证远端开发环境连通性
set -e

CONFIG="config/dev.env.yaml"
FAILED=0

if [ ! -f "$CONFIG" ]; then
  echo "ERROR: $CONFIG 不存在"
  echo "  cp config/dev.env.example.yaml config/dev.env.yaml"
  exit 1
fi

# 从 YAML 中用 grep+sed 提取简单 key: "value" 格式
_yaml_str() {
  grep -A 30 "^$1:" "$CONFIG" | grep "^\s*$2:" | head -1 \
    | sed "s/.*$2:[[:space:]]*//" | tr -d '"' | tr -d "'"
}

DSN=$(grep '^\s*dsn:' "$CONFIG" | head -1 | sed "s/.*dsn:[[:space:]]*//" | tr -d '"' | tr -d "'")
REDIS_ADDR=$(grep '^\s*addr:' "$CONFIG" | head -1 | sed "s/.*addr:[[:space:]]*//" | tr -d '"' | tr -d "'")
REDIS_PASS=$(grep '^\s*password:' "$CONFIG" | head -1 | sed "s/.*password:[[:space:]]*//" | tr -d '"' | tr -d "'")

echo "=== idcd dev-up: connection check ==="
echo ""

# ── PostgreSQL ────────────────────────────────────────────────
if command -v psql &>/dev/null; then
  printf "PostgreSQL ... "
  if psql "$DSN" -c "SELECT 1;" -t -A &>/dev/null; then
    echo "OK"
    TS=$(psql "$DSN" -c "SELECT extversion FROM pg_extension WHERE extname='timescaledb';" -t -A 2>/dev/null | tr -d ' ')
    if [ -n "$TS" ]; then
      echo "  TimescaleDB $TS ✓"
    else
      echo "  TimescaleDB: 未安装（需 root 权限在服务器上安装扩展）"
    fi
  else
    echo "FAILED"
    echo "  检查 config/dev.env.yaml → database.main.dsn"
    FAILED=1
  fi
else
  echo "PostgreSQL ... SKIP (psql 未安装: brew install postgresql)"
fi

echo ""

# ── Redis ─────────────────────────────────────────────────────
if command -v redis-cli &>/dev/null; then
  REDIS_HOST=$(echo "$REDIS_ADDR" | cut -d: -f1)
  REDIS_PORT=$(echo "$REDIS_ADDR" | cut -d: -f2)
  printf "Redis (%s) ... " "$REDIS_ADDR"
  RESULT=$(redis-cli -h "$REDIS_HOST" -p "$REDIS_PORT" -a "$REDIS_PASS" PING 2>/dev/null || echo "FAILED")
  if [ "$RESULT" = "PONG" ]; then
    echo "OK"
  else
    echo "FAILED ($RESULT)"
    echo "  检查 config/dev.env.yaml → redis"
    FAILED=1
  fi
else
  echo "Redis ... SKIP (redis-cli 未安装: brew install redis)"
fi

echo ""

if [ "$FAILED" = "1" ]; then
  echo "部分连接失败，请检查 config/dev.env.yaml"
  exit 1
else
  echo "✓ 所有连接正常，开发环境就绪"
fi
