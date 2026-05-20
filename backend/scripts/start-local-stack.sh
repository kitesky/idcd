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
# 8082 = cert-svc HTTP API, 9094 = cert-svc /metrics.
for port in 3000 8080 8082 8443 9091 9092 9094 9099; do
  pids=$(lsof -ti ":$port" 2>/dev/null || true)
  [[ -n "$pids" ]] && { log "clearing port $port"; echo "$pids" | xargs kill 2>/dev/null || true; sleep 0.3; }
done

# Also kill orphaned component binaries that aren't listening on a public port.
# `go run` forks a build-cache binary as a child; killing the wrapper leaks the
# child as an orphan. Agent has no public listener besides 9099, so if 9099
# hadn't bound yet during the port sweep above the orphan survives, registers
# on the gateway under the same node_id, and triggers a connection-replacement
# storm that defers every probe result by up to wstimeouts.PingInterval (54 s).
log "clearing orphaned component binaries"
pkill -9 -f "go-build.*/(api|gateway|agent|aggregator|cert-svc|cert-worker|cert/server|cert/worker)$" 2>/dev/null || true
pkill -9 -f "exe/(api|gateway|agent|aggregator|server|worker)$" 2>/dev/null || true
sleep 0.3

PIDS=()
cleanup() {
  log "stopping stack…"
  for pid in "${PIDS[@]-}"; do kill "$pid" 2>/dev/null || true; done
  sleep 1
  for pid in "${PIDS[@]-}"; do kill -9 "$pid" 2>/dev/null || true; done
  # Belt-and-suspenders: orphan go-build binaries don't share the wrapper pid.
  pkill -9 -f "go-build.*/(api|gateway|agent|aggregator|cert-svc|cert-worker|cert/server|cert/worker)$" 2>/dev/null || true
  pkill -9 -f "exe/(api|gateway|agent|aggregator|server|worker)$" 2>/dev/null || true
}
trap cleanup EXIT INT TERM

# 1) API (8080)
log "starting API"
IDCD_CONFIG=config/dev.env.yaml go run ./apps/api/cmd/api 2>&1 | sed 's/^/[api]      /' &
PIDS+=($!)

# 2) Gateway (8443)
# Pin a per-host consumer group so this dev gateway gets its own copy of the
# probe.tasks stream instead of sharing `gateway-dispatch` with whatever
# prod/staging gateways are also pointed at the shared dev Redis. Without
# this, redis load-balances each task between consumers — half of every
# tool-page probe lands on a remote gateway that has no record of the local
# agent's ws conn and stalls in PEL for 60s+ before XAutoClaim hands it back.
GATEWAY_DISPATCH_GROUP="gateway-dispatch-dev-$(hostname -s)"
log "starting Gateway (dispatch group=$GATEWAY_DISPATCH_GROUP)"
IDCD_CONFIG=config/dev.env.yaml GATEWAY_DISPATCH_GROUP="$GATEWAY_DISPATCH_GROUP" go run ./apps/gateway/cmd/gateway 2>&1 | sed 's/^/[gateway]  /' &
PIDS+=($!)

# Wait for API + Gateway readiness.
for _ in {1..60}; do
  curl -s -o /dev/null -X OPTIONS http://localhost:8080/v1/auth/login -H "Origin: http://localhost:3000" -H "Access-Control-Request-Method: POST" && \
  curl -s -o /dev/null http://localhost:8443/health && break
  sleep 1
done

# 3) Enroll a local agent (one-time per session — writes /tmp/idcd-agent.yaml).
# If the yaml already exists but is missing fields added in newer revisions,
# patch them in-place so an old session's yaml doesn't silently strand the
# new feature (e.g. geoip_db_path).
if [[ -f /tmp/idcd-agent.yaml ]]; then
  if [[ -f "$ROOT/apps/agent/data/GeoLite2-City.mmdb" ]] && ! grep -q "^geoip_db_path:" /tmp/idcd-agent.yaml; then
    echo "geoip_db_path: \"$ROOT/apps/agent/data/GeoLite2-City.mmdb\"" >> /tmp/idcd-agent.yaml
    log "patched existing /tmp/idcd-agent.yaml with geoip_db_path"
  fi
fi
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
  GEOIP_PATH=""
  if [[ -f "$ROOT/apps/agent/data/GeoLite2-City.mmdb" ]]; then
    GEOIP_PATH="$ROOT/apps/agent/data/GeoLite2-City.mmdb"
  fi
  cat > /tmp/idcd-agent.yaml <<EOF
node_id: $NODE_ID
gateway_url: ws://localhost:8443/agent/ws
secret_key: $SECRET
data_dir: /tmp/idcd-agent-data
poll_interval: 5s
batch_size: 10
geoip_db_path: "$GEOIP_PATH"
observability:
  prometheus_port: 9099
EOF
  log "agent enrolled: $NODE_ID${GEOIP_PATH:+ (geoip enabled)}"
fi

# 4) Agent
log "starting Agent"
IDCD_CONFIG=/tmp/idcd-agent.yaml go run ./apps/agent/cmd/agent 2>&1 | sed 's/^/[agent]    /' &
PIDS+=($!)

# 5) Aggregator
log "starting Aggregator"
AGGREGATOR_CONFIG=config/aggregator.yaml go run ./apps/aggregator/cmd/aggregator 2>&1 | sed 's/^/[agg]      /' &
PIDS+=($!)

# 6) cert-svc + cert-worker
# cert-svc reads CERT_* env vars directly (it doesn't share lib/shared/config).
# We mirror the relevant values from dev.env.yaml so the same DB / Redis / JWT
# secret is shared with apps/api — that's what lets a browser session signed
# in via /v1/auth/login reach /v1/cert/* through the apps/api reverse-proxy.
#
# CERT_MASTER_KEY is a dev-only AES-256 master key (base64 32 bytes). Same
# value across server + worker so envelope-encrypted DNS credentials written
# by one process decrypt cleanly in the other. Production switches to KMS
# via CERT_VAULT_BACKEND=alikms (D-FC-04).
#
# CERT_DOWNLOAD_SECRET signs the one-shot download URL the cert detail page
# embeds. base64-encoded random bytes, any stable dev value works.
export CERT_DB_DSN="postgresql://idcd_dev:idcd_dev@8.163.70.123:5432/idcd_dev"
export CERT_REDIS_ADDR="8.163.70.123:6379"
export CERT_REDIS_PASSWORD="Year2025"
export CERT_REDIS_DB="0"
export CERT_JWT_SECRET="dev_jwt_secret_please_change_in_staging"
export CERT_LE_ENV="staging"
# LE staging refuses to register accounts with a non-public TLD (e.g. .local),
# even though dev never sends mail. Use a real public suffix.
export CERT_ACME_ACCOUNT_EMAIL="acme-dev@idcd.com"
export CERT_ENV="development"
export CERT_LOG_LEVEL="info"
export CERT_SVC_PORT="8082"
export CERT_SVC_METRICS_PORT="9094"
# 32 zero bytes base64 — dev only. openssl rand -base64 32 for fresh.
export CERT_MASTER_KEY="AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
# 32 zero bytes base64 — dev only.
export CERT_DOWNLOAD_SECRET="AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
ADMIN_TOK=$(awk -F'"' '/admin_token/{print $2; exit}' config/dev.env.yaml)
export CERT_ADMIN_TOKEN="$ADMIN_TOK"

log "starting cert-svc (:8082, LE=$CERT_LE_ENV)"
go run ./apps/cert-svc/cmd/server 2>&1 | sed 's/^/[cert-svc] /' &
PIDS+=($!)

log "starting cert-worker"
go run ./apps/cert-svc/cmd/worker 2>&1 | sed 's/^/[cert-wkr] /' &
PIDS+=($!)

# Wait for cert-svc readiness so the API reverse-proxy doesn't 502 the first
# request through.
for _ in {1..60}; do
  curl -s -o /dev/null http://localhost:8082/healthz && break
  sleep 1
done

# 7) Web
log "starting Web (3000)"
(cd ../frontend && NODE_OPTIONS='--max-old-space-size=4096 --max-semi-space-size=128' pnpm next dev --turbopack -p 3000 2>&1 | sed 's/^/[web]      /') &
PIDS+=($!)

log "stack up — http://localhost:3000  |  Ctrl+C to stop"
wait
