# cert-svc 部署 Runbook

> 适用 S2 W8 收尾后的首次生产部署。后续版本升级走 CI/CD `deploy.yml` 自动化即可。
> 关联：`docs/prd/20-free-cert.md`、`backend/infra/docker/docker-compose.prod.yml`、`backend/apps/cert-svc/`

---

## 0. 前置条件

- prod server 已有 `idcd-api / idcd-gateway / idcd-notifier` 运行中
- 已申请 Cloudflare API Token（Zone:DNS:Edit 权限），且 idcd 业务域 zone 已托管
- 已申请 ZeroSSL EAB（可选，S2 多 CA 路由用）
- 已申请阿里云 KMS（可选，国内主路径；首发可暂用 envmaster）

---

## 1. 数据库 migration

S2 W8 push 到 main 后 CI 的 `migrate` job 会自动执行：

```sql
-- backend/lib/db/migrations/idcd_main/00042_cert_init.sql
-- 创建 cert.* schema 下 8 张表
```

人工验证：

```bash
psql "$PROD_DB_DSN" -c "\dt cert.*"
# 期望 8 行：domains / dns_credentials / acme_accounts / orders /
#         order_events / certs / renewal_jobs / audit_logs
```

---

## 2. 配置文件 `/opt/idcd/config/cert-svc.env`

> ⚠️ **不需要起新的 PG / Redis 实例**。cert-svc 与 apps/api、apps/aggregator、apps/notifier 共用同一套 Postgres + Redis：
> - **DB**：同一个 `idcd` 库；cert 表都在独立 `cert.*` schema 下（`00042_cert_init.sql`），不会跟 public schema 现存表冲突；跨 schema 不写 FK（D1 决策），走应用层 join
> - **Redis**：同一个实例；cert 用 `cert:order_events` / `cert:notifications` 两个 stream，跟 `monitor:*` / `alert:*` 互不影响
>
> 所以下面 `CERT_DB_DSN / CERT_REDIS_ADDR` 的**值**就是 apps/api 在 `prod.env.yaml` 里 `database.dsn` / `redis.addr` 的值，原样复制即可。`CERT_JWT_SECRET / CERT_ADMIN_TOKEN` 同理——**复用 apps/api 同名密钥**才能让用户 session / admin Bearer token 在两个服务之间透传。

新建文件，**仅 root + idcd 可读 (chmod 600)**：

```bash
# === 基础 ===
CERT_ENV=production
CERT_LOG_LEVEL=info
CERT_SVC_PORT=8080
CERT_SVC_METRICS_PORT=9090

# === 数据存储（与 apps/api 同一个实例，schema 隔离）===
# 直接 copy /opt/idcd/config/prod.env.yaml 里的 database.dsn 和 redis.addr
CERT_DB_DSN=postgres://idcd:<pw>@<pg-host>:5432/idcd?sslmode=require
CERT_REDIS_ADDR=<redis-host>:6379
CERT_REDIS_URL=redis://<redis-host>:6379/0

# === 鉴权（必须与 apps/api 同值，否则 session / admin 调用穿不过来）===
CERT_JWT_SECRET=<copy apps/api 的 auth.jwt.secret>
CERT_ADMIN_TOKEN=<copy apps/api 的 server.admin_token>

# === 私钥加密 ===
# S2 上线首发可走 envmaster（已知短板，但允许 MVP）：
CERT_VAULT_BACKEND=envmaster
CERT_MASTER_KEY=<base64(32B random)>
# 切真 KMS（推荐 ≤ 1 个月内完成）：
# CERT_VAULT_BACKEND=alikms
# CERT_ALIKMS_REGION_ID=cn-hangzhou
# CERT_ALIKMS_ACCESS_KEY_ID=<RAM 子账号 AK>
# CERT_ALIKMS_ACCESS_KEY_SECRET=<RAM 子账号 SK>
# CERT_ALIKMS_KEY_ID=<KMS CMK ID>

# === 下载链接签名 ===
CERT_DOWNLOAD_SECRET=<base64(32B random)>

# === ACME ===
CERT_ACME_ACCOUNT_EMAIL=acme@idcd.com
CERT_LE_ENV=production  # 调试期用 staging

# === 多 CA（可选，S2）===
# CERT_ZEROSSL_EAB_KID=<ZeroSSL 控制台获取>
# CERT_ZEROSSL_EAB_HMAC_KEY=<同上>
# CERT_BUYPASS_ENV=production
```

生成随机 secret：

```bash
openssl rand -base64 32  # MASTER_KEY / DOWNLOAD_SECRET / ADMIN_TOKEN
```

---

## 3. nginx 反代

`/opt/idcd/nginx/nginx.conf` 加两段 location（在现有 `/api/v1/*` 转发 gateway 的块里）：

```nginx
# 用户 cert API（gateway 已支持 /v1/cert/* + /v1/admin/cert/* 反代）
# 现有的 /api/v1/ → gateway:8443 转发已经覆盖，无需新增 location
# 仅确认 location /api/v1/ 段已存在即可
```

`/v1/admin/cert/*` 建议**额外加 IP 白名单**（只允许办公网 / VPN 出口 IP）：

```nginx
location /api/v1/admin/cert/ {
    allow 1.2.3.4/32;        # 办公 VPN 出口
    deny all;
    proxy_pass https://gateway:8443;
    # ... 其它转发头同 /api/v1/
}
```

reload：

```bash
nginx -t && nginx -s reload
```

---

## 4. 启动 cert-svc / cert-worker / cert-renewer

```bash
cd /opt/idcd
docker compose pull cert-svc cert-worker cert-renewer
docker compose up -d cert-svc
# 等 healthy 再起 worker / renewer
timeout 60 bash -c 'until docker compose ps cert-svc | grep -q "healthy"; do sleep 2; done'
docker compose up -d cert-worker cert-renewer

# 验证
curl -sf http://127.0.0.1:8086/health && echo OK
curl -sf http://127.0.0.1:9096/metrics | head -20
docker compose logs --tail=30 cert-svc cert-worker cert-renewer
```

---

## 5. 冒烟测试

5.1 **登录后台调一笔订单**：浏览器登录 → `/app/cert/new` → 填测试域名（先用 CERT_LE_ENV=staging 跑 LE staging endpoint，避免吃 prod 配额）→ Cloudflare 自动模式 → 等待 60-180s → 检查订单页状态 `issued` → 点下载 → 确认拿到 fullchain + privkey。

5.2 **Admin 后台**：

```bash
curl -H "Authorization: Bearer $CERT_ADMIN_TOKEN" \
     https://api.idcd.com/api/v1/admin/cert/ca-quota
```

期望返回三家 CA 的当周使用率。

5.3 **Prometheus 抓取**：在 Prometheus `prometheus.yml` 加 target：

```yaml
- job_name: cert-svc
  static_configs:
    - targets: ['cert-svc-host:9096']
  scrape_interval: 30s
```

---

## 6. 切真 KMS（首次部署后 ≤ 1 个月）

`envmaster` 的已知短板是 master key 跟进程同主机，主机被攻破即明文私钥泄露。生产应在首批商业用户上线前切到 KMS：

1. 阿里云控制台创建 CMK（用途 `ENCRYPT/DECRYPT`，对称 AES_256）
2. 创建 RAM 子账号，授权 `AliyunKMSCryptoUserAccess`
3. 用新账号 + KMS 加密 envmaster 时期的 master key → 写一个一次性迁移脚本：
   - 读 `cert.dns_credentials` 所有 `encrypted_blob`
   - envmaster 解密 → alikms 加密 → 覆盖写回
   - 同上处理 `cert.certs` 私钥
4. 更新 `cert-svc.env`：切换 `CERT_VAULT_BACKEND=alikms`，删 `CERT_MASTER_KEY`
5. `docker compose up -d cert-svc cert-worker cert-renewer` 重启

⚠️ 迁移前**先冻结新订单**（admin 后台改成维护模式或直接 nginx 503）。

---

## 7. 监控告警接线

`/metrics` 暴露 6 个指标：

| 指标 | 告警阈值 |
|---|---|
| `cert_orders_total{status="failed"}` 5min 增量 | > 10 → P2 |
| `cert_ca_quota_used{ca="lets-encrypt"}` | > 0.8 → P2（提前切 CA） |
| `cert_queue_depth{queue="cert:orders"}` | > 100 持续 5min → P1 |
| `cert_acme_errors_total{error_type="rate_limited"}` 1h 增量 | > 5 → P2 |
| `cert_renewal_jobs_total{status="failed"}` 24h | > 5 → P1（影响存量用户续期） |
| `cert_order_duration_seconds` P95 | > 600s → P3（DNS 传播慢） |

Alertmanager 路由按 `severity` 转 Slack / 钉钉。

---

## 8. 回滚

```bash
# 暂停三个容器，保留数据
docker compose stop cert-svc cert-worker cert-renewer

# 回滚 image
docker compose pull cert-svc:main-<旧 SHA>
docker compose up -d cert-svc cert-worker cert-renewer
```

DB migration 不可单步回滚（cert schema 只增不减）；如必须回滚 schema：

```bash
goose -dir backend/lib/db/migrations/idcd_main postgres "$PROD_DB_DSN" down-to 41
```

⚠️ 这会丢失 cert 数据；仅紧急情况使用。

---

## 9. 故障联系

- P0 / P1：创始人手机（7×24）
- P2 / P3：研发群 + Jira
- 业务方求救：`abuse@idcd.com` / `support@idcd.com`
