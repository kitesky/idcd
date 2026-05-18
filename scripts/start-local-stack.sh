#!/usr/bin/env bash
# Bring up the FULL idcd stack locally so you can actually use the tools end-to-end.
# This is the answer to "submit a probe and never see a result":
#   web → api → gateway → agent → probe.results stream → aggregator → probe_task table → polled by frontend.
#
# Usage: bash scripts/start-local-stack.sh
# Stops everything on Ctrl+C.
#
# Prereqs:
#   - config/dev.env.yaml exists (cp from dev.env.example.yaml)
#   - config/aggregator.yaml exists (cp from aggregator.example.yaml + fill DSN/Redis)
#   - Postgres + Redis reachable per the configs above

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

C_RESET="\033[0m"
C_BOLD="\033[1m"
log() { printf "${C_BOLD}[stack]${C_RESET} %s\n" "$*"; }
die() { printf "${C_BOLD}[stack] ERROR:${C_RESET} %s\n" "$*" >&2; exit 1; }

[[ -f config/dev.env.yaml ]]    || die "config/dev.env.yaml missing"
[[ -f config/aggregator.yaml ]] || die "config/aggregator.yaml missing — copy from config/aggregator.example.yaml"

# Clean up old listeners.
for port in 3000 8080 8443 9091 9092 9099; do
  pids=$(lsof -ti ":$port" 2>/dev/null || true)
  [[ -n "$pids" ]] && { log "clearing port $port"; echo "$pids" | xargs kill 2>/dev/null || true; sleep 0.3; }
done

PIDS=()
cleanup() {
  log "stopping stack…"
  for pid in "${PIDS[@]-}"; do kill "$pid" 2>/dev/null || true; done
  sleep 1
  for pid in "${PIDS[@]-}"; do kill -9 "$pid" 2>/dev/null || true; done
}
trap cleanup EXIT INT TERM

# 1) API (8080)
log "starting API"
IDCD_CONFIG=config/dev.env.yaml go run ./apps/api/cmd/api 2>&1 | sed 's/^/[api]      /' &
PIDS+=($!)

# 2) Gateway (8443)
log "starting Gateway"
IDCD_CONFIG=config/dev.env.yaml go run ./apps/gateway/cmd/gateway 2>&1 | sed 's/^/[gateway]  /' &
PIDS+=($!)

# Wait for API + Gateway readiness.
for _ in {1..60}; do
  curl -s -o /dev/null -X OPTIONS http://localhost:8080/v1/auth/login -H "Origin: http://localhost:3000" -H "Access-Control-Request-Method: POST" && \
  curl -s -o /dev/null http://localhost:8443/health && break
  sleep 1
done

# 3) Enroll a local agent (one-time per session — writes /tmp/idcd-agent.yaml).
if [[ ! -f /tmp/idcd-agent.yaml ]]; then
  log "enrolling local agent…"
  ADMIN=$(awk -F'"' '/admin_token/{print $2; exit}' config/dev.env.yaml)
  TOK=$(curl -sS -X POST http://localhost:8080/internal/admin/nodes/enrollment-tokens \
        -H "X-Admin-Token: $ADMIN" -H "Content-Type: application/json" \
        -d '{"label":"local-stack","expires_in":"24h"}' | python3 -c 'import json,sys;print(json.load(sys.stdin)["data"]["token"])')
  CRED=$(curl -sS -X POST http://localhost:8080/v1/agent/enroll \
         -H "Content-Type: application/json" \
         -d "{\"token\":\"$TOK\",\"hostname\":\"$(hostname)\",\"os\":\"$(uname -s)\",\"arch\":\"$(uname -m)\"}")
  NODE_ID=$(echo "$CRED" | python3 -c 'import json,sys;print(json.load(sys.stdin)["data"]["node_id"])')
  SECRET=$(echo "$CRED" | python3 -c 'import json,sys;print(json.load(sys.stdin)["data"]["secret_key"])')
  mkdir -p /tmp/idcd-agent-data
  cat > /tmp/idcd-agent.yaml <<EOF
node_id: $NODE_ID
gateway_url: ws://localhost:8443/agent/ws
secret_key: $SECRET
data_dir: /tmp/idcd-agent-data
poll_interval: 5s
batch_size: 10
observability:
  prometheus_port: 9099
EOF
  log "agent enrolled: $NODE_ID"
fi

# 4) Agent
log "starting Agent"
IDCD_CONFIG=/tmp/idcd-agent.yaml go run ./apps/agent/cmd/agent 2>&1 | sed 's/^/[agent]    /' &
PIDS+=($!)

# 5) Aggregator
log "starting Aggregator"
AGGREGATOR_CONFIG=config/aggregator.yaml go run ./apps/aggregator/cmd/aggregator 2>&1 | sed 's/^/[agg]      /' &
PIDS+=($!)

# 6) Web
log "starting Web (3000)"
(cd apps/web && NODE_OPTIONS='--max-old-space-size=4096 --max-semi-space-size=128' pnpm next dev --turbopack -p 3000 2>&1 | sed 's/^/[web]      /') &
PIDS+=($!)

log "stack up — http://localhost:3000  |  Ctrl+C to stop"
wait
