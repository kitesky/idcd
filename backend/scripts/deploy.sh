#!/usr/bin/env bash
# idcd 部署脚本 — rsync 源码 → ssh 跑 migration + docker compose build/up → 烟测
#
# 用法:
#   bash scripts/deploy.sh [HOST]
#
# 默认 HOST = 43.134.175.79 (staging)。
# 假设目标机已跑过 infra/scripts/server-init.sh,/opt/idcd/{config,nginx,src} 已就绪。
#
# 必须本地预先准备:
#   /opt/idcd/config/prod.env.yaml  + cert-svc.env (在 staging 机上)
#   /opt/idcd/.env (GHCR_OWNER=local)              (在 staging 机上)
#
# 退出码:0 全 OK / 非 0 表示某 step 失败,看终端打印的 step 编号。

set -euo pipefail

HOST="${1:-43.134.175.79}"
USER="${DEPLOY_USER:-root}"
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
REMOTE_SRC="/opt/idcd/src"
REMOTE_STACK="/opt/idcd"

cyan()  { printf "\033[36m%s\033[0m\n" "$*"; }
green() { printf "\033[32m%s\033[0m\n" "$*"; }
red()   { printf "\033[31m%s\033[0m\n" "$*" >&2; }

step() { cyan "▶ [$1/$STEPS] $2"; }

STEPS=7

# ── 0. 预检 ───────────────────────────────────────────────────
cyan "═══ idcd 部署 → ${USER}@${HOST} ═══"
ssh -o ConnectTimeout=5 "${USER}@${HOST}" 'command -v docker >/dev/null && command -v goose >/dev/null' \
  || { red "目标机缺 docker 或 goose,先跑 infra/scripts/server-init.sh"; exit 1; }
ssh "${USER}@${HOST}" 'test -f /opt/idcd/config/prod.env.yaml && test -f /opt/idcd/config/cert-svc.env' \
  || { red "目标机缺 /opt/idcd/config/{prod.env.yaml,cert-svc.env},先准备"; exit 2; }

# ── 1. rsync 源码 ────────────────────────────────────────────
step 1 "rsync 源码到 ${HOST}:${REMOTE_SRC}"
rsync -az --delete \
  --exclude='.git' --exclude='node_modules' \
  --exclude='dist' --exclude='build' --exclude='.claude' --exclude='.idea' --exclude='.vscode' \
  --exclude='*.log' --exclude='__pycache__' --exclude='coverage' \
  "${REPO_ROOT}/" "${USER}@${HOST}:${REMOTE_SRC}/"

# ── 2. 取 DSN 跑 migration ────────────────────────────────────
step 2 "提取 DSN 并跑 goose migration (idcd_main + idcd_attest)"
DSN=$(ssh "${USER}@${HOST}" "grep -E '^  dsn:' /opt/idcd/config/prod.env.yaml | head -1 | sed 's/^  dsn: \"\\(.*\\)\"/\\1/'")
[[ -z "$DSN" ]] && { red "无法从 prod.env.yaml 读取 database.dsn"; exit 3; }
ssh "${USER}@${HOST}" "cd ${REMOTE_SRC} && \
  goose -dir lib/db/migrations/idcd_main postgres '${DSN}' up && \
  goose -table goose_attest_version -dir lib/db/migrations/idcd_attest postgres '${DSN}' up"

# ── 3. build 6 个 image (如果有 build override) ──────────────
step 3 "docker compose build (如果 docker-compose.build.yml 存在)"
ssh "${USER}@${HOST}" "cd ${REMOTE_STACK} && \
  if [ -f docker-compose.build.yml ]; then \
    docker compose -f docker-compose.prod.yml -f docker-compose.build.yml build; \
  else \
    docker compose -f docker-compose.prod.yml pull; \
  fi"

# ── 4. compose up -d ─────────────────────────────────────────
step 4 "docker compose up -d"
ssh "${USER}@${HOST}" "cd ${REMOTE_STACK} && docker compose -f docker-compose.prod.yml up -d"

# ── 5. 等 api / cert-svc healthy ─────────────────────────────
step 5 "等 api + cert-svc healthcheck pass (max 60s)"
ssh "${USER}@${HOST}" 'for i in $(seq 1 30); do
  api=$(docker inspect -f "{{.State.Health.Status}}" idcd-api 2>/dev/null || echo "missing")
  cert=$(docker inspect -f "{{.State.Health.Status}}" idcd-cert-svc 2>/dev/null || echo "missing")
  echo "[$i/30] api=$api cert-svc=$cert"
  [[ "$api" == "healthy" && "$cert" == "healthy" ]] && exit 0
  sleep 2
done; exit 1' || { red "api 或 cert-svc 30 次后仍不 healthy,看日志:"; \
    ssh "${USER}@${HOST}" 'docker compose -f /opt/idcd/docker-compose.prod.yml logs --tail=30 api cert-svc'; exit 5; }

# ── 6. 烟测关键 endpoint ──────────────────────────────────────
step 6 "烟测 /health + /healthz + /v1/cert (经 api 反代)"
ssh "${USER}@${HOST}" 'set -e
  echo "  api    /health:        $(curl -fsS http://127.0.0.1:8080/health | head -c 60)"
  echo "  cert   /healthz:       $(curl -fsS http://127.0.0.1:8086/healthz | head -c 60)"
  echo "  api → cert /v1/cert/ca-status (proxy): $(curl -s -o /dev/null -w "%{http_code}" http://127.0.0.1:8080/v1/cert/ca-status)"
'

# ── 7. 状态总览 ───────────────────────────────────────────────
step 7 "状态总览"
ssh "${USER}@${HOST}" 'docker compose -f /opt/idcd/docker-compose.prod.yml ps'

green "✅ 部署完成 → https://${HOST}/ (或 https://staging.idcd.com 等 DNS 生效)"
