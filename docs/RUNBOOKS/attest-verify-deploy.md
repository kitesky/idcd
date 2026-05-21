# attest-verify 部署 Runbook

> 适用 S2 首次生产部署 attest-verify 独立自验服务（DECISIONS.md D6）。
> 关联：`docs/prd/ENG-REVIEW-REPORT.md` D6、`backend/apps/attest-verify/README.md`、
> `backend/infra/docker/docker-compose.prod.yml`。

---

## 0. 前置条件

- `attest-server`（apps/attest cmd/server）已在目标环境运行，且 `POST /verify` 可从此服务访问。
- 数据库已执行 `backend/lib/db/migrations/idcd_attest/00008_self_verify_log.sql` 迁移。
- 生产环境已有 `idcd_attest` schema（由 00001_create_schema.sql 创建）。

---

## 1. 数据库迁移

```bash
# 确认 self_verify_log 表存在
psql "$ATTEST_DB_DSN" -c "\d idcd_attest.self_verify_log"
# 期望列: id, record_id, verified_at, status, latency_ms, error, created_at
```

如尚未执行迁移，使用 goose：

```bash
GOOSE_DRIVER=postgres GOOSE_DBSTRING="$ATTEST_DB_DSN" \
  goose -dir backend/lib/db/migrations/idcd_attest up
```

---

## 2. 配置文件 `/opt/idcd/config/attest-verify.env`

> 此服务使用 **独立** 环境变量前缀 `ATTEST_VERIFIER_`，与 attest-server 的 `ATTEST_` 前缀完全分离。

新建文件，仅 root + idcd 可读（`chmod 600`）：

```bash
# === 基础 ===
ATTEST_VERIFIER_ENV=production
ATTEST_VERIFIER_LOG_LEVEL=info
ATTEST_VERIFIER_BIND_ADDR=:8090

# === 数据库（与 attest-server 共用同一 DB 实例，不同连接池）===
# 复用 attest-server 的同一 PostgreSQL DSN；两个服务读写不同的表
ATTEST_VERIFIER_DB_DSN=postgresql://idcd:CHANGE_ME@DB_HOST:5432/idcd

# === D6 核心：验证端点必须是公开 URL ===
# 不能填 localhost 或内网 IP — 必须走与外部第三方相同的代码路径
ATTEST_VERIFIER_VERIFY_ENDPOINT=https://attest.idcd.com/verify

# === 轮询参数 ===
ATTEST_VERIFIER_POLL_INTERVAL=5m   # 每 5 分钟一次轮询
ATTEST_VERIFIER_BATCH_SIZE=20      # 每次最多处理 20 条记录
```

---

## 3. Docker Compose 服务

`attest-verify` 已加入 `backend/infra/docker/docker-compose.prod.yml`。

启动命令：

```bash
docker compose -f /opt/idcd/docker-compose.prod.yml up -d attest-verify
```

验证健康：

```bash
# liveness
curl -sf http://localhost:8090/healthz

# readiness（需要 DB 连通）
curl -sf http://localhost:8090/readyz
```

---

## 4. 验证独立性（D6 审计项）

**必须在 staging 执行，截图留存：**

```bash
# 1. 验证两个 container 在不同进程
docker ps --format "table {{.Names}}\t{{.Image}}\t{{.Ports}}"
# 期望：idcd-attest-verify 和 idcd-attest-server 各自独立行

# 2. 验证 attest-verify 的网络调用走公开接口
docker exec idcd-attest-verify \
  wget -qO- http://localhost:8090/healthz
# 期望：ok

# 3. 确认 self_verify_log 有写入
psql "$ATTEST_DB_DSN" -c \
  "SELECT status, count(*) FROM idcd_attest.self_verify_log GROUP BY status;"
# 期望：pass N 条（N 取决于已有报告数量）

# 4. 查看日志（独立容器输出）
docker logs idcd-attest-verify --tail=50
# 期望：attest-verifier poller starting ... endpoint=https://attest.idcd.com/verify
```

---

## 5. 监控告警

在监控系统中添加以下规则：

| 告警名称 | 条件 | 级别 |
|---|---|---|
| `attest_verify_fail_rate_high` | `self_verify_log` 中 5 分钟内 fail+error > 3 条 | P1 |
| `attest_verify_no_activity` | 1 小时内 `self_verify_log` 无新写入且 `attestation_record` 有新 s3_archived 记录 | P2 |
| `attest_verify_container_down` | `/healthz` 连续 3 次失败 | P0 |

---

## 6. 回滚

attest-verify 是独立只读服务（不修改 attest-server 的任何表），回滚安全：

```bash
docker compose -f /opt/idcd/docker-compose.prod.yml stop attest-verify
docker compose -f /opt/idcd/docker-compose.prod.yml rm -f attest-verify
```

回滚不影响 attest-server、证书服务或 API。

---

## 7. 常见问题

**问：self_verify_log 有 error 记录，原因是 "status 502: Bad Gateway"**
答：attest.idcd.com nginx 层出现问题，与 attest-verify 无关。检查 attest-server /healthz 及 nginx 日志。

**问：self_verify_log 一直没有新行**
答：先检查 `attestation_record` 是否有 `action='s3_archived' AND status='success'` 的记录；若无，说明 Generator 未完成归档步骤，与 attest-verify 无关。

**问：verify endpoint 返回 valid=false**
答：说明 attest-server 的公开 /verify 端点判定签名无效。这是真实的签名验证失败，需要排查 attest-server 的 KMS 配置或 PDF 完整性。
