# idcd 配置参考

> P1-8 配置回迁（env → YAML）完成后的配置文档。
> **SSOT**：`backend/config/dev.env.example.yaml`（开发）和 `backend/config/prod.env.example.yaml`（生产）。

---

## 总览

idcd 使用两层配置：

| 层级 | 优先级 | 说明 |
|------|--------|------|
| 环境变量（`CERT_*` / `ATTEST_*` / `IDCD_*`） | **最高** | 用于 secret 注入、CI/CD 覆盖 |
| YAML 文件（`cert_svc:` / `attest:` 段） | 中 | 非 secret 运行配置，版本可控 |
| 硬编码默认值（Go 代码内） | 最低 | 本地开发 fallback |

### YAML 文件路径

| 服务 | 指定路径 env var | 次级 fallback | 默认 |
|------|-----------------|---------------|------|
| api / notifier / gateway / scheduler | `IDCD_CONFIG` | — | `config/dev.env.yaml` |
| aggregator | `AGGREGATOR_CONFIG` | — | `config/aggregator.yaml` |
| cert-svc | `CERT_SVC_CONFIG` | `IDCD_CONFIG` | `config/dev.env.yaml` |
| attest | `ATTEST_CONFIG` | `IDCD_CONFIG` | `config/dev.env.yaml` |

> **注意**：YAML 文件不存在时服务静默跳过（不报错），完全依靠环境变量。

---

## cert-svc 配置（`cert_svc:` 段）

### env → YAML 映射表

| 旧环境变量 | YAML 字段 | 是否 secret | 默认值 |
|-----------|-----------|------------|--------|
| `CERT_SVC_PORT` | `cert_svc.port` | 否 | 8080 |
| `CERT_SVC_METRICS_PORT` | `cert_svc.metrics_port` | 否 | 9090 |
| `CERT_ENV` | `cert_svc.env` | 否 | `development` |
| `CERT_LE_ENV` | `cert_svc.le_env` | 否 | `staging` |
| `CERT_LOG_LEVEL` | `cert_svc.log_level` | 否 | `info` |
| `CERT_ACME_ACCOUNT_EMAIL` | `cert_svc.acme_account_email` | 否 | `acme@idcd.local` |
| `CERT_DB_DSN` | `cert_svc.database.dsn` | 否* | 本地 PG |
| `CERT_REDIS_ADDR` | `cert_svc.redis.addr` | 否 | `localhost:6379` |
| `CERT_REDIS_PASSWORD` | `cert_svc.redis.password` | 否* | `""` |
| `CERT_REDIS_DB` | `cert_svc.redis.db` | 否 | `0` |
| `CERT_REDIS_MASTER_NAME` | `cert_svc.redis.master_name` | 否 | `""` |
| `CERT_REDIS_SENTINEL_ADDRS` | `cert_svc.redis.sentinel_addrs` | 否 | `[]` |
| `CERT_REDIS_SENTINEL_PASSWORD` | `cert_svc.redis.sentinel_password` | 否* | `""` |
| `CERT_VAULT_BACKEND` | `cert_svc.vault.backend` | 否 | `envmaster` |
| `CERT_ALIKMS_REGION_ID` | `cert_svc.vault.alikms.region_id` | 否 | `""` |
| `CERT_ALIKMS_ACCESS_KEY_ID` | `cert_svc.vault.alikms.access_key_id` | 否* | `""` |
| `CERT_ALIKMS_ACCESS_KEY_SECRET` | `cert_svc.vault.alikms.access_key_secret` | **是** | `""` |
| `CERT_ALIKMS_KEY_ID` | `cert_svc.vault.alikms.key_id` | 否 | `""` |
| `CERT_AWSKMS_REGION` | `cert_svc.vault.awskms.region` | 否 | `""` |
| `CERT_AWSKMS_ACCESS_KEY_ID` | `cert_svc.vault.awskms.access_key_id` | 否* | `""` |
| `CERT_AWSKMS_SECRET_ACCESS_KEY` | `cert_svc.vault.awskms.secret_access_key` | **是** | `""` |
| `CERT_AWSKMS_KEY_ID` | `cert_svc.vault.awskms.key_id` | 否 | `""` |
| `CERT_HASHIVAULT_ADDRESS` | `cert_svc.vault.hashivault.address` | 否 | `""` |
| `CERT_HASHIVAULT_TOKEN` | `cert_svc.vault.hashivault.token` | **是** | `""` |
| `CERT_HASHIVAULT_NAMESPACE` | `cert_svc.vault.hashivault.namespace` | 否 | `""` |
| `CERT_HASHIVAULT_KEY_NAME` | `cert_svc.vault.hashivault.key_name` | 否 | `""` |
| `CERT_HASHIVAULT_MOUNT_PATH` | `cert_svc.vault.hashivault.mount_path` | 否 | `"transit"` |
| `CERT_ZEROSSL_EAB_KID` | `cert_svc.zerossl_eab_kid` | 否* | `""` |
| `CERT_ZEROSSL_EAB_HMAC_KEY` | `cert_svc.zerossl_eab_hmac_key` | **是** | `""` |
| `CERT_BUYPASS_ENV` | `cert_svc.buypass_env` | 否 | `""` |

### 保留 env-only（不在 YAML）

| 环境变量 | 用途 |
|---------|------|
| `CERT_JWT_SECRET` | api 颁发 JWT 的校验密钥 |
| `CERT_MASTER_KEY` | envmaster vault 的主密钥（base64） |
| `CERT_DOWNLOAD_SECRET` | W5 下载令牌 HMAC 密钥（base64） |
| `CERT_ADMIN_TOKEN` | `/v1/admin/cert/*` Bearer 令牌 |

---

## attest 配置（`attest:` 段）

### env → YAML 映射表

| 旧环境变量 | YAML 字段 | 是否 secret | 默认值 |
|-----------|-----------|------------|--------|
| `ATTEST_PORT` | `attest.port` | 否 | 8080 |
| `ATTEST_ENV` | `attest.env` | 否 | `development` |
| `ATTEST_LOG_LEVEL` | `attest.log_level` | 否 | `info` |
| `ATTEST_DB_DSN` | `attest.database.dsn` | 否* | `""` |
| `ATTEST_REDIS_ADDR` | `attest.redis.addr` | 否 | `""` |
| `ATTEST_REDIS_PASSWORD` | `attest.redis.password` | 否* | `""` |
| `ATTEST_REDIS_DB` | `attest.redis.db` | 否 | `0` |
| `ATTEST_REDIS_MASTER_NAME` | `attest.redis.master_name` | 否 | `""` |
| `ATTEST_REDIS_SENTINEL_ADDRS` | `attest.redis.sentinel_addrs` | 否 | `[]` |
| `ATTEST_REDIS_SENTINEL_PASSWORD` | `attest.redis.sentinel_password` | 否* | `""` |
| `ATTEST_SIGN_BACKEND` | `attest.sign_backend` | 否 | `""` |
| `ATTEST_AWSKMS_REGION` | `attest.awskms.region` | 否 | `""` |
| `ATTEST_AWSKMS_ACCESS_KEY_ID` | `attest.awskms.access_key_id` | 否* | `""` |
| `ATTEST_AWSKMS_SECRET_ACCESS_KEY` | `attest.awskms.secret_access_key` | **是** | `""` |
| `ATTEST_AWSKMS_KEY_ID` | `attest.awskms.key_id` | 否 | `""` |
| `ATTEST_AWSKMS_ALGORITHM` | `attest.awskms.algorithm` | 否 | `ECDSA_SHA_256` |
| `ATTEST_ALIKMS_REGION_ID` | `attest.alikms.region_id` | 否 | `""` |
| `ATTEST_ALIKMS_ACCESS_KEY_ID` | `attest.alikms.access_key_id` | 否* | `""` |
| `ATTEST_ALIKMS_ACCESS_KEY_SECRET` | `attest.alikms.access_key_secret` | **是** | `""` |
| `ATTEST_ALIKMS_KEY_ID` | `attest.alikms.key_id` | 否 | `""` |
| `ATTEST_ALIKMS_ALGORITHM` | `attest.alikms.algorithm` | 否 | `ECDSA_SHA_256` |
| `ATTEST_LOCAL_KEY_PATH` | `attest.local_key_path` | 否 | `""` |
| `ATTEST_LOCAL_ALGORITHM` | `attest.local_algorithm` | 否 | `RSASSA_PKCS1_V1_5_SHA_256` |
| `ATTEST_TSA_PROVIDERS` | `attest.tsa.providers` | 否 | `[digicert, globalsign]` |
| `ATTEST_S3_BUCKET` | `attest.s3.bucket` | 否 | `""` |
| `ATTEST_S3_REGION` | `attest.s3.region` | 否 | `""` |
| `ATTEST_S3_ENDPOINT` | `attest.s3.endpoint` | 否 | `""` |
| `ATTEST_S3_OBJECT_LOCK_MODE` | `attest.s3.object_lock_mode` | 否 | `COMPLIANCE` |
| `ATTEST_S3_OBJECT_LOCK_DAYS` | `attest.s3.object_lock_days` | 否 | `3650` |
| `ATTEST_S3_KEY_PREFIX` | `attest.s3.key_prefix` | 否 | `""` |
| `ATTEST_ARCHIVER_BACKEND` | `attest.archiver_backend` | 否 | `local` |
| `ATTEST_LOCAL_ARCHIVE_DIR` | `attest.local_archive_dir` | 否 | `/var/lib/attest/archive` |
| `ATTEST_VERIFY_ENDPOINT` | `attest.verify_endpoint` | 否 | `""` |
| `ATTEST_REFUND_INITIATE_STREAM` | `attest.refund.initiate_stream` | 否 | `refund_initiate_queue` |
| `ATTEST_REFUND_RETRY_STREAM` | `attest.refund.retry_stream` | 否 | `refund_retry_queue` |
| `ATTEST_REFUND_DELAY_ZONE` | `attest.refund.delay_zone_key` | 否 | `refund_delay_zone` |
| `ATTEST_REFUND_GROUP` | `attest.refund.group` | 否 | `attest-refund-worker` |
| `ATTEST_REFUND_CONSUMER` | `attest.refund.consumer` | 否 | `attest-refund-worker-1` |
| `ATTEST_REFUND_NOTIFIER_REDIS_ADDR` | `attest.refund.notifier_addr` | 否* | `""` |
| `ATTEST_REFUND_NOTIFIER_QUEUE` | `attest.refund.notifier_queue` | 否 | `billing` |

### 保留 env-only（不在 YAML）

| 环境变量 | 用途 |
|---------|------|
| `ATTEST_PAYMENT_HUB_WEBHOOK_SECRET` | PaymentHub webhook HMAC secret |

---

## 验证结果

```bash
# Before（cert-svc）：os.Getenv 调用数
grep -c 'os.Getenv' backend/apps/cert-svc/internal/config/config.go
# Before: ~30  →  After: 0（全部通过 YAML 或常量读取）

# Before（attest）：os.Getenv 调用数
grep -c 'os.Getenv' backend/apps/attest/internal/config/config.go
# Before: ~25  →  After: 0（全部通过 YAML 或常量读取）
```

> `os.Getenv` 仍出现在 config.go 的环境变量层读取逻辑中，但它们是**覆盖**层（env > YAML > default），不再是 sole source of truth。

---

## 迁移指南（生产部署）

1. 复制 `backend/config/prod.env.example.yaml` 到部署路径（如 `/opt/idcd/config/prod.env.yaml`）
2. 填入 `cert_svc:` 和 `attest:` 段的真实值（DSN、KMS 凭证等）
3. **保留** secret 类通过环境变量注入（见"保留 env-only"表）
4. 启动服务：
   ```bash
   CERT_SVC_CONFIG=/opt/idcd/config/prod.env.yaml ./cert-svc
   ATTEST_CONFIG=/opt/idcd/config/prod.env.yaml ./attest-server
   # 或复用 IDCD_CONFIG
   IDCD_CONFIG=/opt/idcd/config/prod.env.yaml ./cert-svc
   ```
5. 验证：`grep -c 'CHANGE_ME' /opt/idcd/config/prod.env.yaml` → 应为 0

---

## 标注说明

`否*` = 含有敏感信息但在 YAML 中以 placeholder 形式存在；建议通过环境变量或 secret manager 覆盖，不要在版本控制的 YAML 中存放真实值。
