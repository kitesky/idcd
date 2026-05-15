#!/usr/bin/env bash
# idcd Agent Installer
# Usage: curl -fsSL https://get.idcd.com/agent | IDCD_TOKEN=ent_xxx bash
#
# Required:
#   IDCD_TOKEN   — one-time enrollment token from idcd console
#
# Optional (override defaults):
#   IDCD_API_BASE   — API server base URL       (default: https://api.idcd.com)
#   AGENT_VERSION   — binary version to install  (default: latest)
#   AGENT_NODE_NAME — human label for this node  (shown in console)

set -euo pipefail

# ── defaults ──────────────────────────────────────────────────────────────────
IDCD_API_BASE="${IDCD_API_BASE:-https://api.idcd.com}"
AGENT_VERSION="${AGENT_VERSION:-latest}"
AGENT_NODE_NAME="${AGENT_NODE_NAME:-}"
INSTALL_BIN="/usr/local/bin/idcd-agent"
INSTALL_CTL="/usr/local/bin/idcd-agent-ctl"
CONFIG_DIR="/etc/idcd-agent"
DATA_DIR="/var/lib/idcd-agent"
LOG_DIR="/var/log/idcd-agent"
SERVICE_USER="idcd-agent"
SERVICE_FILE="/etc/systemd/system/idcd-agent.service"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# ── colours ───────────────────────────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; BLUE='\033[0;34m'; NC='\033[0m'
log()  { printf "${GREEN}[idcd]${NC} %s\n" "$*"; }
info() { printf "${BLUE}[info]${NC} %s\n" "$*"; }
warn() { printf "${YELLOW}[warn]${NC} %s\n" "$*"; }
die()  { printf "${RED}[error]${NC} %s\n" "$*" >&2; exit 1; }

# ── preflight checks ──────────────────────────────────────────────────────────
[[ -z "${IDCD_TOKEN:-}" ]] && die "IDCD_TOKEN is required. Get one at https://app.idcd.com/nodes/new"
[[ $EUID -ne 0 ]] && die "Run as root: sudo IDCD_TOKEN=... bash install.sh"
[[ "$(uname -s)" != "Linux" ]] && die "Only Linux is supported"

command -v systemctl &>/dev/null || die "systemd is required"

# ── detect architecture ───────────────────────────────────────────────────────
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)        ARCH_SLUG="amd64" ;;
  aarch64|arm64) ARCH_SLUG="arm64" ;;
  *)             die "Unsupported architecture: $ARCH" ;;
esac

# ── download helper ───────────────────────────────────────────────────────────
http_get() {
  local url="$1" out="$2"
  if command -v curl &>/dev/null; then
    curl -fsSL -o "$out" "$url"
  elif command -v wget &>/dev/null; then
    wget -qO "$out" "$url"
  else
    die "curl or wget is required"
  fi
}

# ── json extract (jq preferred, python3 fallback) ─────────────────────────────
json_field() {
  local json="$1" field="$2"
  if command -v jq &>/dev/null; then
    echo "$json" | jq -r ".data.${field} // empty"
  else
    echo "$json" | python3 -c \
      "import sys,json; d=json.load(sys.stdin).get('data',{}); print(d.get('${field}',''))" 2>/dev/null || true
  fi
}

# ── download binary ───────────────────────────────────────────────────────────
log "Downloading idcd Agent (arch=${ARCH_SLUG}, version=${AGENT_VERSION})..."

if [[ "$AGENT_VERSION" == "latest" ]]; then
  BINARY_URL="${IDCD_API_BASE}/releases/agent/latest/idcd-agent-linux-${ARCH_SLUG}"
else
  BINARY_URL="${IDCD_API_BASE}/releases/agent/${AGENT_VERSION}/idcd-agent-linux-${ARCH_SLUG}"
fi

TMP_BIN=$(mktemp)
trap 'rm -f "$TMP_BIN"' EXIT

http_get "$BINARY_URL" "$TMP_BIN" || die "Failed to download binary from $BINARY_URL"
chmod +x "$TMP_BIN"

# Sanity-check: should be an ELF binary
file "$TMP_BIN" 2>/dev/null | grep -q ELF || warn "Downloaded file does not look like an ELF binary — proceeding anyway"

# ── system user & directories ─────────────────────────────────────────────────
if ! id "$SERVICE_USER" &>/dev/null; then
  log "Creating system user: $SERVICE_USER"
  useradd --system --no-create-home --shell /usr/sbin/nologin "$SERVICE_USER"
fi

for dir in "$CONFIG_DIR" "$DATA_DIR" "$LOG_DIR"; do
  mkdir -p "$dir"
  chown "$SERVICE_USER:$SERVICE_USER" "$dir"
done

# ── enrollment: exchange token for node credentials ───────────────────────────
log "Enrolling node with idcd control plane..."

HOSTNAME_VAL=$(hostname -f 2>/dev/null || hostname)
KERNEL_VAL=$(uname -r)
AGENT_VER=$("$TMP_BIN" --version 2>/dev/null | head -1 || echo "unknown")
NODE_NAME="${AGENT_NODE_NAME:-${HOSTNAME_VAL}}"

# Use jq to safely encode all fields — prevents JSON injection if hostname or
# version strings contain double-quotes, backslashes, or other special characters.
if command -v jq &>/dev/null; then
  ENROLL_JSON=$(jq -n \
    --arg token   "$IDCD_TOKEN" \
    --arg host    "$HOSTNAME_VAL" \
    --arg arch    "$ARCH_SLUG" \
    --arg kernel  "$KERNEL_VAL" \
    --arg version "$AGENT_VER" \
    --arg label   "$NODE_NAME" \
    '{token:$token,hostname:$host,arch:$arch,os:"linux",kernel:$kernel,version:$version,label:$label}')
else
  # Fallback: sanitize values by stripping characters unsafe in JSON strings.
  _sanitize() { printf '%s' "$1" | tr -d '"\\'; }
  ENROLL_JSON=$(printf '{"token":"%s","hostname":"%s","arch":"%s","os":"linux","kernel":"%s","version":"%s","label":"%s"}' \
    "$(_sanitize "$IDCD_TOKEN")" "$(_sanitize "$HOSTNAME_VAL")" "$(_sanitize "$ARCH_SLUG")" \
    "$(_sanitize "$KERNEL_VAL")" "$(_sanitize "$AGENT_VER")" "$(_sanitize "$NODE_NAME")")
fi

if command -v curl &>/dev/null; then
  ENROLL_RESP=$(curl -fsSL -X POST \
    -H "Content-Type: application/json" \
    -d "$ENROLL_JSON" \
    "${IDCD_API_BASE}/v1/agent/enroll") || die "Enrollment request failed. Verify IDCD_TOKEN and network connectivity."
else
  ENROLL_RESP=$(wget -qO- --post-data="$ENROLL_JSON" \
    --header="Content-Type: application/json" \
    "${IDCD_API_BASE}/v1/agent/enroll") || die "Enrollment request failed."
fi

NODE_ID=$(json_field "$ENROLL_RESP" "node_id")
SECRET_KEY=$(json_field "$ENROLL_RESP" "secret_key")
GATEWAY_URL=$(json_field "$ENROLL_RESP" "gateway_url")

[[ -z "$NODE_ID" || "$NODE_ID" == "null" ]] && \
  die "Enrollment failed — server rejected token (expired or already used)"
[[ -z "$SECRET_KEY" || "$SECRET_KEY" == "null" ]] && \
  die "Enrollment failed — no secret_key returned"
GATEWAY_URL="${GATEWAY_URL:-wss://gateway.idcd.com}"

info "Enrolled: node_id=${NODE_ID}"

# ── write config ──────────────────────────────────────────────────────────────
log "Writing config to ${CONFIG_DIR}/config.yaml"
cat > "${CONFIG_DIR}/config.yaml" <<YAML
# idcd Agent configuration — managed by installer, edit with care
node_id: "${NODE_ID}"
gateway_url: "${GATEWAY_URL}"
secret_key: "${SECRET_KEY}"
data_dir: "${DATA_DIR}"
poll_interval: "30s"
batch_size: 100
observability:
  telemetry:
    enabled: false
    sampling_rate: 0.1
YAML
chmod 600 "${CONFIG_DIR}/config.yaml"
chown "${SERVICE_USER}:${SERVICE_USER}" "${CONFIG_DIR}/config.yaml"

# ── install binary ────────────────────────────────────────────────────────────
log "Installing binary: ${INSTALL_BIN}"
mv "$TMP_BIN" "$INSTALL_BIN"
trap - EXIT
chown root:root "$INSTALL_BIN"

# ── install management CLI ────────────────────────────────────────────────────
if [[ -f "${SCRIPT_DIR}/idcd-agent-ctl" ]]; then
  cp "${SCRIPT_DIR}/idcd-agent-ctl" "$INSTALL_CTL"
  chmod +x "$INSTALL_CTL"
  log "Management tool installed: idcd-agent-ctl"
fi

# ── systemd service ───────────────────────────────────────────────────────────
log "Installing systemd service"
cat > "$SERVICE_FILE" <<SERVICE
[Unit]
Description=idcd Agent Node
Documentation=https://idcd.com/docs/agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=${SERVICE_USER}
Group=${SERVICE_USER}
ExecStart=${INSTALL_BIN}
Restart=always
RestartSec=5
StartLimitBurst=5
StartLimitIntervalSec=60

Environment=IDCD_CONFIG=${CONFIG_DIR}/config.yaml
Environment=AGENT_DATA_DIR=${DATA_DIR}

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=${DATA_DIR} ${LOG_DIR} /usr/local/bin
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
RestrictRealtime=true
LockPersonality=true
MemoryDenyWriteExecute=true
RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6

# ICMP ping needs CAP_NET_RAW
AmbientCapabilities=CAP_NET_RAW
CapabilityBoundingSet=CAP_NET_RAW

# Resource limits
MemoryMax=512M
TasksMax=100

# Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=idcd-agent

[Install]
WantedBy=multi-user.target
SERVICE

systemctl daemon-reload
systemctl enable idcd-agent
systemctl start idcd-agent

# ── verify ────────────────────────────────────────────────────────────────────
sleep 2
if systemctl is-active --quiet idcd-agent; then
  log "✓ Agent is running"
  echo ""
  printf "${GREEN}Installation complete!${NC}\n"
  printf "  node_id   : %s\n" "$NODE_ID"
  printf "  gateway   : %s\n" "$GATEWAY_URL"
  printf "  status    : %s\n" "$(systemctl is-active idcd-agent)"
  echo ""
  log "Useful commands:"
  log "  idcd-agent-ctl status    # Service status + uptime"
  log "  idcd-agent-ctl logs      # Recent log lines"
  log "  idcd-agent-ctl upgrade   # Upgrade to latest version"
  log "  idcd-agent-ctl restart   # Restart the service"
else
  warn "Service may not have started correctly."
  warn "Check: journalctl -u idcd-agent -n 50"
  exit 1
fi
