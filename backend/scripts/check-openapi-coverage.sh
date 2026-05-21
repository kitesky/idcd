#!/usr/bin/env bash
# P0-5 (ARCHITECTURE-REVIEW-2026-05-21.md):
# OpenAPI spec ↔ 实际 chi router 注册对账。
#
# spec 是手写的 (docs/prd/16-api-spec.yaml, 3.8k 行), 代码没用 oapi-codegen
# 也没用 swag。本脚本是一道弱契约闸: 验证每个 spec path+method 在
# backend/apps/{api,attest} 的 chi router 注册里能找到 (cert-svc 通过
# api gateway 的 r.Handle("/v1/cert/*", proxy) wildcard 已覆盖)。
# 反向也查: 代码注册了但 spec 没写的, 同样报错。
#
# 走 AST 解析 chi.Router 调用 (Get / Post / Put / Patch / Delete / Route / Handle),
# 不需要启动服务跑请求 — 那是更深一层 contract test, 留给 A 方案。
#
# Baseline: backend/scripts/openapi-coverage/baseline.txt 记录当前已知 drift。
# 脚本只在 baseline 之外发现新 drift 时才失败。修了 drift 之后跑:
#
#   bash backend/scripts/check-openapi-coverage.sh --write-baseline
#
# 来收紧 baseline。最终目标是把 baseline 清空。
#
# 退出码:
#   0 = 通过 (无 drift 或全部 drift 已在 baseline 内)
#   1 = 出现新 drift (PR 该修)
#   2 = 脚本自身错误 (spec / helper 找不到)
#
# CI: 已挂到 .github/workflows/ci.yml 的 schema-lint job。

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
BACKEND_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
REPO_ROOT="$(cd "$BACKEND_DIR/.." && pwd)"

SPEC="${SPEC:-$REPO_ROOT/docs/prd/16-api-spec.yaml}"
HELPER_DIR="$SCRIPT_DIR/openapi-coverage"
BASELINE="${BASELINE:-$HELPER_DIR/baseline.txt}"

if [ ! -f "$SPEC" ]; then
  echo "ERROR: spec not found at $SPEC" >&2
  exit 2
fi
if [ ! -d "$HELPER_DIR" ]; then
  echo "ERROR: helper module missing at $HELPER_DIR" >&2
  exit 2
fi

# --write-baseline (or env WRITE_BASELINE=1): snapshot current drift to
# baseline.txt and exit 0. Use after intentionally accepting more debt
# OR after reducing drift in a follow-up PR.
WRITE_FLAG=""
if [ "${1:-}" = "--write-baseline" ] || [ "${WRITE_BASELINE:-}" = "1" ]; then
  WRITE_FLAG="-write-baseline"
fi

# cert-svc is intentionally NOT in the scan roots: its routes are
# served behind the api gateway's r.Handle("/v1/cert/*", proxy)
# wildcard, which the helper records as a wildcard mount. Adding
# cert-svc here would double-count + emit orphan paths because
# cert-svc's chi.Router("/v1/cert", ...) closure delegates to
# helper funcs (mountOrders / mountCerts / ...) whose bodies live
# in different files, breaking single-function prefix tracking.
#
# GOWORK=off keeps this helper out of backend/go.work (the helper
# is stdlib-only, no module conflict needed).
cd "$HELPER_DIR"
exec env GOWORK=off go run . \
  -spec "$SPEC" \
  -root "$REPO_ROOT/backend/apps/api" \
  -root "$REPO_ROOT/backend/apps/attest" \
  -baseline "$BASELINE" \
  $WRITE_FLAG
