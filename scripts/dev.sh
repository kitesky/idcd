#!/usr/bin/env bash
# 本地开发一键启动脚本。Ctrl+C 退出时自动停止所有子进程。
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CONFIG="$ROOT/config/dev.env.yaml"

# ── 颜色 ──────────────────────────────────────────────────────
C_RESET="\033[0m"
C_BOLD="\033[1m"
C_DIM="\033[2m"
C_WEB="\033[36m"    # cyan
C_API="\033[32m"    # green
C_GW="\033[33m"     # yellow
C_ERR="\033[31m"    # red

log()  { printf "${C_BOLD}[dev]${C_RESET} %s\n" "$*"; }
die()  { printf "${C_ERR}[dev] ERROR: %s${C_RESET}\n" "$*" >&2; exit 1; }

# ── 前置检查 ───────────────────────────────────────────────────
[[ -f "$CONFIG" ]] || die "缺少 $CONFIG，请先复制 config/dev.env.example.yaml 并填写连接信息"
command -v go   >/dev/null 2>&1 || die "未找到 go"
command -v pnpm >/dev/null 2>&1 || die "未找到 pnpm"

# 清理残留的本项目开发进程（杀掉占用 8080/8081/3000 的进程）
for port in 8080 8081 3000; do
  pids=$(lsof -ti ":$port" 2>/dev/null || true)
  if [[ -n "$pids" ]]; then
    log "清理端口 $port 上的残留进程 (PID: $pids)"
    echo "$pids" | xargs kill 2>/dev/null || true
    sleep 0.3
  fi
done

# ── 进程组管理 ────────────────────────────────────────────────
PIDS=()

cleanup() {
  echo ""
  log "正在停止所有服务…"
  for pid in "${PIDS[@]-}"; do
    kill "$pid" 2>/dev/null || true
  done
  # 等待子进程退出（最多 4 秒）
  local waited=0
  while [[ $waited -lt 4 ]]; do
    local alive=0
    for pid in "${PIDS[@]-}"; do
      kill -0 "$pid" 2>/dev/null && alive=1 && break
    done
    [[ $alive -eq 0 ]] && break
    sleep 1
    (( waited++ )) || true
  done
  # 强杀残留
  for pid in "${PIDS[@]-}"; do
    kill -9 "$pid" 2>/dev/null || true
  done
  log "全部服务已停止"
}

trap cleanup EXIT INT TERM

# ── 带颜色前缀的流水线 ─────────────────────────────────────────
prefix_log() {
  local label="$1" color="$2"
  while IFS= read -r line; do
    printf "${color}[%-7s]${C_RESET} %s\n" "$label" "$line"
  done
}

# ── 启动 Next.js web ─────────────────────────────────────────
log "启动 ${C_WEB}Next.js web${C_RESET}  → http://localhost:3000"
(
  cd "$ROOT/apps/web"
  pnpm dev 2>&1
) | prefix_log "web" "$C_WEB" &
PIDS+=($!)

# ── 启动 Go API ───────────────────────────────────────────────
log "启动 ${C_API}Go API${C_RESET}       → http://localhost:8080  ${C_DIM}(debug 日志已开启)${C_RESET}"
(
  cd "$ROOT"
  go run ./apps/api/cmd/api 2>&1
) | prefix_log "api" "$C_API" &
PIDS+=($!)

# ── 启动 Go Gateway ──────────────────────────────────────────
log "启动 ${C_GW}Go Gateway${C_RESET}   → :8081"
(
  cd "$ROOT"
  go run ./apps/gateway/cmd/gateway 2>&1
) | prefix_log "gateway" "$C_GW" &
PIDS+=($!)

echo ""
log "所有服务已启动 │ 按 ${C_BOLD}Ctrl+C${C_RESET} 停止"
echo ""

# ── 监控：任意进程退出则停止全部（bash 3.2 兼容）───────────────
while true; do
  for pid in "${PIDS[@]}"; do
    if ! kill -0 "$pid" 2>/dev/null; then
      log "${C_ERR}服务 PID $pid 意外退出，正在关闭其余进程…${C_RESET}"
      exit 1
    fi
  done
  sleep 2
done
