.PHONY: dev-setup dev-up test seed lint build clean help

# ── 颜色输出 ─────────────────────────────────────────────────
BOLD  := $(shell tput bold 2>/dev/null || echo "")
RESET := $(shell tput sgr0 2>/dev/null || echo "")
GREEN := $(shell tput setaf 2 2>/dev/null || echo "")
YELLOW:= $(shell tput setaf 3 2>/dev/null || echo "")

# ── 帮助 ─────────────────────────────────────────────────────
help:
	@echo "$(BOLD)idcd Makefile$(RESET)"
	@echo ""
	@echo "  $(GREEN)make dev-setup$(RESET)   安装 Go 工具链 + Node 依赖"
	@echo "  $(GREEN)make dev-up$(RESET)      验证远端 DB / Redis 连通性"
	@echo "  $(GREEN)make test$(RESET)        运行全量测试 (Go + TS)"
	@echo "  $(GREEN)make seed$(RESET)        初始化开发数据"
	@echo "  $(GREEN)make lint$(RESET)        全量 lint (golangci + eslint + D1/D-Concern1 规则)"
	@echo "  $(GREEN)make build$(RESET)       构建所有二进制"
	@echo "  $(GREEN)make clean$(RESET)       清理构建产物"

# ── 开发环境 ─────────────────────────────────────────────────
dev-setup:
	@echo "$(BOLD)→ Go tools$(RESET)"
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/pressly/goose/v3/cmd/goose@latest
	@echo "$(BOLD)→ Node dependencies$(RESET)"
	cd apps/web && pnpm install
	@echo ""
	@echo "$(GREEN)✓ Setup done.$(RESET)"
	@echo "  如还没有 config/dev.env.yaml，执行："
	@echo "    cp config/dev.env.example.yaml config/dev.env.yaml"
	@echo "  然后填入连接信息。"

dev-up:
	@bash scripts/check-connections.sh

# ── 测试 ─────────────────────────────────────────────────────
test:
	go test -p 4 ./... -count=1 -timeout 120s
	cd apps/web && pnpm test --passWithNoTests 2>/dev/null || true

# ── 数据填充 ─────────────────────────────────────────────────
seed:
	@if [ ! -f scripts/seed/main.go ]; then \
	  echo "$(YELLOW)seed 脚本尚未实现 (待 A3 数据库层完成后添加)$(RESET)"; \
	else \
	  go run scripts/seed/main.go; \
	fi

# ── Lint ─────────────────────────────────────────────────────
lint: lint-go lint-sql lint-attestation lint-ts

lint-go:
	@echo "$(BOLD)→ golangci-lint$(RESET)"
	golangci-lint run ./...

lint-sql:
	@echo "$(BOLD)→ cross-schema FK check (D1)$(RESET)"
	@bash scripts/lint-cross-schema-fk.sh

lint-attestation:
	@echo "$(BOLD)→ attestation words check (D-Concern1)$(RESET)"
	@bash scripts/lint-attestation-words.sh

lint-ts:
	@echo "$(BOLD)→ ESLint + tsc$(RESET)"
	cd apps/web && pnpm lint 2>/dev/null || true

# ── 构建 ─────────────────────────────────────────────────────
BUILD_DIR := bin
LDFLAGS   := -s -w -X main.version=$(shell cat VERSION)

build: build-api build-scheduler build-gateway build-agent build-aggregator build-notifier

build-api:
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/api ./apps/api/cmd/api

build-scheduler:
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/scheduler ./apps/scheduler/cmd/scheduler

build-gateway:
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/gateway ./apps/gateway/cmd/gateway

build-agent:
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/agent ./apps/agent/cmd/agent
	@echo "Cross-compile linux/amd64:"
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/agent-linux-amd64 ./apps/agent/cmd/agent
	@echo "Cross-compile linux/arm64:"
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/agent-linux-arm64 ./apps/agent/cmd/agent

build-aggregator:
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/aggregator ./apps/aggregator/cmd/aggregator

build-notifier:
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/notifier ./apps/notifier/cmd/notifier

# ── 清理 ─────────────────────────────────────────────────────
clean:
	rm -rf $(BUILD_DIR)
	cd apps/web && rm -rf .next dist 2>/dev/null || true

# ── DB 迁移快捷命令 ────────────────────────────────────────────
# 多 schema 共用一个 DB：每个 schema 必须用独立的 goose 版本表，
# 否则版本号会跨 schema 撞车（attest 的 v1-6 会被 main 的 v1-45 “占用”
# 而被 goose 误判已 apply）。
_DSN := $(shell python3 -c "import yaml; c=yaml.safe_load(open('config/dev.env.yaml')); print(c['database']['main']['dsn'])" 2>/dev/null)
_GOOSE := go run github.com/pressly/goose/v3/cmd/goose@latest
_MIG_DIR_MAIN := lib/db/migrations/idcd_main
_MIG_DIR_ATTEST := lib/db/migrations/idcd_attest

migrate-up: migrate-main-up migrate-attest-up

migrate-down: migrate-attest-down migrate-main-down

migrate-status:
	@echo "== idcd_main =="
	$(_GOOSE) -dir $(_MIG_DIR_MAIN) postgres "$(_DSN)" status
	@echo "== idcd_attest =="
	$(_GOOSE) -table goose_attest_version -dir $(_MIG_DIR_ATTEST) postgres "$(_DSN)" status

migrate-main-up:
	$(_GOOSE) -dir $(_MIG_DIR_MAIN) postgres "$(_DSN)" up

migrate-main-down:
	$(_GOOSE) -dir $(_MIG_DIR_MAIN) postgres "$(_DSN)" down

migrate-attest-up:
	$(_GOOSE) -table goose_attest_version -dir $(_MIG_DIR_ATTEST) postgres "$(_DSN)" up

migrate-attest-down:
	$(_GOOSE) -table goose_attest_version -dir $(_MIG_DIR_ATTEST) postgres "$(_DSN)" down

sqlc-gen:
	sqlc generate -f packages/db/sqlc.yaml
