#!/usr/bin/env bash
# mem-guard.sh — macOS 内存压力监控 + 自动杀孤儿 node 进程
# 适配 macOS 26+：使用 kern.memorystatus_level (0-100) 替代已损坏的 vm.memory_pressure
#
# 运行方式: bash scripts/mem-guard.sh [--dry-run]
#
# 阈值（memorystatus_level 越低 = 越危险）:
#   level < 30  (~5GB 压力)  → 警告：杀旧 node worker（存活 > 120s 的非关键进程）
#   level < 15  (~2.5GB 压力)→ 危机：激进清理 + 杀多余 gopls
#   free  < 800 MB (兜底)    → 无论 level 多少都触发危机模式

set -euo pipefail

DRY_RUN=false
[[ "${1:-}" == "--dry-run" ]] && DRY_RUN=true

ts() { date '+%H:%M:%S'; }
log() { echo "[mem-guard $(ts)] $*"; }

# ── 不能杀的进程名关键词 ─────────────────────────────────────────
KEEP_PATTERNS=(
  "next dev"
  "next-server"
  "shadcn-mcp"
  "playwright-mcp"
  "context7-mcp"
  "mcp-server-github"
  "mcp-server-memory"
  "mcp-server-sequential"
  "Code Helper"
  "code-helper"
  "electron"
  "vitest"
  "jest"
)

# ── 内存指标读取 ──────────────────────────────────────────────────
memorystatus_level() {
  sysctl -n kern.memorystatus_level 2>/dev/null || echo 100
}

free_mb() {
  vm_stat | awk '
    /page size of ([0-9]+) bytes/ { size = $8 }
    /Pages free:/      { free = $3+0 }
    /Pages inactive:/  { inactive = $3+0 }
    END { printf "%.0f", (free + inactive) * size / 1024 / 1024 }
  '
}

# ── 判断是否保留 ──────────────────────────────────────────────────
is_keep() {
  local cmdline="$1"
  for pat in "${KEEP_PATTERNS[@]}"; do
    [[ "$cmdline" == *"$pat"* ]] && return 0
  done
  return 1
}

# ── 列出可杀的 node PID（按存活时长降序，最老的在前）───────────────
killable_node_pids() {
  local min_age="${1:-120}"
  ps -eo pid,etimes,command 2>/dev/null \
    | awk '$2+0 >= '"$min_age"' && ($3 ~ /^node$/ || $3 ~ /\/node$/) {
        print $1, $2, substr($0, index($0,$3))
      }' \
    | sort -k2 -rn \
    | while IFS= read -r line; do
        local pid cmd
        pid=$(awk '{print $1}' <<<"$line")
        cmd=$(awk '{$1=$2=""; gsub(/^ +/,""); print}' <<<"$line")
        is_keep "$cmd" && continue
        echo "$pid $cmd"
      done
}

do_kill() {
  local sig="$1"; shift
  local killed=0
  for entry in "$@"; do
    local pid cmd rss
    pid=$(awk '{print $1}' <<<"$entry")
    cmd=$(awk '{$1=""; gsub(/^ /,""); print}' <<<"$entry")
    rss=$(ps -o rss= -p "$pid" 2>/dev/null | awk '{printf "%.0f",$1/1024}') || rss="?"
    echo "[mem-guard $(ts)] KILL-${sig} pid=${pid} rss=${rss}MB  ${cmd:0:72}"
    [[ "$DRY_RUN" == false ]] && kill "-${sig}" "$pid" 2>/dev/null || true
    (( killed++ )) || true
  done
  echo "[mem-guard $(ts)] => 共杀 ${killed} 个 (SIG${sig})"
}

# ── 主循环 ───────────────────────────────────────────────────────
LEVEL=$(memorystatus_level)
FREE=$(free_mb)
echo "[mem-guard $(ts)] 启动 dry_run=${DRY_RUN}  level=${LEVEL}/100  free=${FREE}MB"
echo "[mem-guard $(ts)] 阈值: level<30→警告杀旧workers  level<15→危机激进清理  free<800MB→兜底危机"

prev_level="$LEVEL"

while true; do
  sleep 5

  LEVEL=$(memorystatus_level)
  FREE=$(free_mb)

  # level 变化超过 5 或进入压力区才打印（避免每秒微波动刷屏）
  diff=$(( LEVEL - prev_level ))
  [[ "$diff" -lt 0 ]] && diff=$(( -diff ))
  if [[ "$diff" -ge 5 ]] || [[ "$LEVEL" -lt 40 ]]; then
    echo "[mem-guard $(ts)] level=${LEVEL}/100  free=${FREE}MB"
    prev_level="$LEVEL"
  fi

  # ── 充裕区，跳过 ───────────────────────────────────────────────
  [[ "$LEVEL" -ge 40 ]] && [[ "$FREE" -gt 3000 ]] && continue

  # ── 警告区：level<30 或 free<2000 ──────────────────────────────
  if [[ "$LEVEL" -lt 30 ]] || [[ "$FREE" -lt 2000 ]]; then
    echo "[mem-guard $(ts)] ⚠️  警告 level=${LEVEL} free=${FREE}MB — 清理旧 node workers (>120s)"
    mapfile -t victims < <(killable_node_pids 120 | head -30)
    if [[ ${#victims[@]} -eq 0 ]]; then
      echo "[mem-guard $(ts)] 没有符合条件的旧 node 进程"
    else
      do_kill TERM "${victims[@]}"
    fi
  fi

  # ── 危机区：level<15 或 free<800 ──────────────────────────────
  if [[ "$LEVEL" -lt 15 ]] || [[ "$FREE" -lt 800 ]]; then
    echo "[mem-guard $(ts)] 🔴 危机 level=${LEVEL} free=${FREE}MB — 激进清理"
    mapfile -t victims < <(killable_node_pids 30)
    [[ ${#victims[@]} -gt 0 ]] && do_kill KILL "${victims[@]}"

    # 杀多余 gopls（保留 pid 最大即最新的）
    mapfile -t gprocs < <(pgrep -x gopls 2>/dev/null | sort -n || true)
    if [[ ${#gprocs[@]} -gt 1 ]]; then
      echo "[mem-guard $(ts)] 杀多余 gopls (${#gprocs[@]} 实例，保留最新)"
      for (( i=0; i<${#gprocs[@]}-1; i++ )); do
        echo "[mem-guard $(ts)] kill gopls pid=${gprocs[$i]}"
        [[ "$DRY_RUN" == false ]] && kill -TERM "${gprocs[$i]}" 2>/dev/null || true
      done
    fi
    echo "[mem-guard $(ts)] ⚡ 建议立即重启 pnpm dev"
  fi

done
