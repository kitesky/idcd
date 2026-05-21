# attest 服务部署计划

> 生成日期:2026-05-21
> 起源:架构审查 (`docs/prd/ARCHITECTURE-REVIEW-2026-05-21.md`) P0-3 项 — "attest 服务的 prod 部署位置未核实"
> 核实人:Claude(对 SG staging 节点 `43.134.175.79` 现场 SSH 核实)
> 状态:**S1 阶段故意未部署**,代码 + DB schema 已就绪,本文档记录 S2 启用前需完成的部署事项

---

## 1. S1 现状核实(2026-05-21 现场)

### 1.1 SG staging 节点(`43.134.175.79`)

| 检查项 | 结果 |
|---|---|
| Docker container | 10 个健康 service:`api / aggregator / notifier / gateway / scheduler / mcp / cert-svc / cert-worker / cert-renewer / nginx`(以及 1 个 `idcd-agent-staging`)— **无 attest 容器** |
| `/opt/idcd/docker-compose.prod.yml` services | **不含 attest 服务定义** |
| nginx 路由(`/opt/idcd/nginx/nginx.conf`) | 4 个 `server_name`:`api.idcd.com / admin.idcd.com / mcp.idcd.com / cname.idcd.com` — **无 `attest.idcd.com` server 块** |
| DNS 解析 | `attest.idcd.com` → **NXDOMAIN**(域名未注册解析) |

### 1.2 已就绪的部分

| 项 | 路径 / 证据 |
|---|---|
| 源代码 | `backend/apps/attest/{cmd,internal}/`,3 个 binary(`server / generator / refund-worker`),7.2k 行 |
| 共享库 | `backend/lib/attest/{sign,record,pdfsign,...}/` 完整 |
| DB schema | `deploy.sh:53` goose migrate `idcd_attest` schema(`backend/lib/db/migrations/idcd_attest/`)— PG 库已建,迁移已跑 |
| 测试覆盖 | attest 包测试覆盖率 124%(架构审查"已确认健康项") |
| 架构决策 | D4 (Verdict WAL) + D5 (Refund retry) + D6 (Self-verify 独立) + D11 (12h Shamir SOP) 全部在 `docs/prd/DECISIONS.md` 已锁定 |

### 1.3 与 D6 决策的对应关系

`docs/ARCHITECTURE.md` §3.4 要求 attest 实施"物理隔离 4 层":不同进程 + 不同 VPC subnet + 独立 KMS 客户端 + 公开 verify 路径。**S1 不部署 attest 完全符合 D6**,因为这一阶段没有需要 attest 服务的业务流(无 Evidence pipeline / 无 Verdict 签发)。

---

## 2. S2 启用条件(何时需要部署)

attest 服务在以下里程碑达到时启用:

- **M5-M8 Evidence pipeline 上线**:Verdict 签发 → KMS sign → TSA timestamp → S3 WORM 存档 → attestation_record WAL 完成
- 客户开始使用 Verdict 报告并需要公开验签链(`https://attest.idcd.com/verify`)
- D11 KMS 应急 SOP 需要 attest server 提供 sign 接口给 Shamir 重组流程

详 `docs/ARCHITECTURE.md` §4.2 S2 部署。

---

## 3. S2 上线前置清单(部署前必须完成)

### 3.1 基础设施

- [ ] **采购独立云主机**(D6 独立 VPC subnet 要求):
  - 主 attest VPC subnet:`attest-generator`(跑 server + generator + refund-worker)
  - 副 attest VPC subnet:`attest-verifier`(跑 Self-verify worker,见 D6)
  - 内部 RPC 防火墙阻断:只允许出站到 KMS / TSA / S3 / PG,**禁止入站除 :443 verify 接口**
- [ ] **域名注册 + DNS**:
  - `attest.idcd.com` A 记录指向 attest-generator
  - 内部 `attest-verifier.internal` (CNAME 或 Route53 私有 zone,不暴露公网)
- [ ] **TLS 证书**:
  - `attest.idcd.com` 用 Let's Encrypt(沿用 cert-svc / cert-renewer 链路)
- [ ] **KMS 配置**:
  - attest-generator 独立 KMS 客户端 + 独立 IAM role
  - attest-verifier 独立 KMS 客户端 + **read-only** GetPublicKey 权限(D6 要求)
  - Shamir 持有人 5 人名单 + 联系方式登记(Pre-5 决策已锁定 "创始人担 + Emergency Contact List")

### 3.2 代码 / 构建

- [ ] **Dockerfile**:`backend/apps/attest/Dockerfile`(若不存在则建)— 复用其他 service 的 alpine 3.22 multi-stage 模板(参考 `backend/apps/api/Dockerfile`)
- [ ] **docker-compose.attest.yml**:新增独立 compose 文件(**不**合并进 `docker-compose.prod.yml`,以保持物理隔离)
  - service:`attest-server`(:8081) / `attest-generator` / `attest-refund-worker`
  - 同 compose 文件下可加 `attest-verifier` service,但**部署到不同主机**(运维约定)
- [ ] **配置文件**:`/opt/idcd-attest/config.yaml`(独立路径,不混在 `/opt/idcd/` 下)
  - DB DSN(连接 idcd_attest schema)
  - KMS endpoint + region + IAM role
  - TSA endpoint
  - S3 WORM bucket
- [ ] **secrets**:走 Vault(参考 cert-svc Vault 集成,见 `backend/apps/cert-svc/cmd/server/main.go:334`)

### 3.3 网络 / nginx

- [ ] 在 attest-generator 主机部署独立 nginx(或复用主 idcd-nginx 加新 server_name 块,看运维选择)
- [ ] nginx 配置 `attest.idcd.com` server 块,`location /verify` proxy 到 `attest-server:8081/verify`
- [ ] 防火墙规则:`attest.idcd.com:443` 公网开放;其他端口全部内网

### 3.4 SOP / 演练

- [ ] **D11 12h Shamir 演练**(必须在 S2 上线**前**完成 1 次)— Pre-4 决策"S2 上线前必演练 12h 路径"
  - 5 个 Shamir 持有人手动重组 KMS key
  - 记录耗时基线
  - 演练记录入 `docs/runbooks/`
- [ ] **D12 Emergency Contact List** 制定 — Pre-5 决策"S2 上线前制定 Backup 联系人名单"
- [ ] **3 档 SLA SOP**:Verdict 失败纯自动 / 1h 仅 P0 / 24h 常规 — 客服流程 + 工单系统对接

### 3.5 监控 / 告警

- [ ] Prometheus 指标接入(P1-11 Phase 1 已埋点):
  - `idcd_attest_kms_sign_attempts_total` / `kms_sign_duration_seconds` / `kms_sign_retries_total`
  - `idcd_attest_refund_retry_queue_length` (gauge, D5 退款失败监控)
  - `idcd_attest_verdict_records_total{outcome}`
- [ ] Grafana dashboard(Phase 2 工作)
- [ ] AlertManager 规则:KMS sign 失败率 > 10% / refund_retry_queue_length > 10(对应 D5 P0)

---

## 4. 部署步骤(S2 启用日)

1. **数据库准备**:
   ```bash
   # idcd_attest schema 已在 deploy.sh 跑过 goose migrate,确认即可
   goose -table goose_attest_version -dir backend/lib/db/migrations/idcd_attest postgres "$DSN" status
   ```
2. **secrets / KMS 准备**:Vault 写入 attest 独立 IAM credentials + Shamir 切片(冷启动一次性写入)
3. **镜像构建 + 推送**:
   ```bash
   docker build -t idcd-attest:v1 -f backend/apps/attest/Dockerfile backend/
   docker push <registry>/idcd-attest:v1
   ```
4. **attest-generator 主机部署**:
   ```bash
   ssh attest-generator-host
   sudo mkdir -p /opt/idcd-attest && cd /opt/idcd-attest
   # 部署 docker-compose.attest.yml + config.yaml
   sudo docker compose -f docker-compose.attest.yml up -d
   ```
5. **attest-verifier 主机部署**:
   - 同上,但**不同主机 / 不同 VPC subnet**
   - 配置只读 KMS 客户端
6. **nginx 切流**:
   - `attest.idcd.com` DNS 切到 attest-generator 公网 IP
   - 等 LE 证书签发
   - 验证 `curl -I https://attest.idcd.com/verify` 200
7. **D6 验证**:
   - `docker ps` on attest-generator 和 attest-verifier:容器 ID 不同
   - 网络拓扑展示两个 subnet
   - 运行时 OpenTelemetry trace 显示 attest-verifier **仅调** `https://attest.idcd.com/verify` 公开接口,**无**内部 RPC

---

## 5. 验证(部署后冒烟)

- [ ] `curl https://attest.idcd.com/verify` 返回非 5xx
- [ ] 跑 attest 测试用例(从 staging 触发 1 个 Verdict 签发流):
  - KMS sign 成功
  - TSA timestamp 拿到
  - S3 PutObject 成功
  - attestation_record 写入 WAL
  - Self-verify 在另一台机器独立验签通过
- [ ] Prometheus `/metrics` 含 attest 业务指标(P1-11 已埋)
- [ ] Self-verify 走 verify 公开接口的 OpenTelemetry trace 可见

---

## 6. 回滚

- attest 故障 → 切 LB 把 `attest.idcd.com` 回滚到 503 维护页(不影响主业务,因 S1/S2 早期 attest 在关键路径外)
- Verdict 签发失败已经有 D12 SLA SOP 兜底(纯自动 / 1h P0 / 24h 常规)

---

## 7. 后续 follow-up(本计划落地后)

- D11 Backup HSM 加速通道:**S4 启用**(Pre-4 已决,接受 SLA 偶尔滑至 24h+ 现实风险)
- attest 多区(D6 + S3 部署 multi-region):S3 阶段(M9-M14)
- 商业客户接入 attest API:需要 OpenAPI spec(`docs/prd/16-api-spec.yaml`)补 `/v1/attest/verify` 端点定义(目前 spec-有/代码-缺 38 项之一,见 P0-5 baseline)

---

## 8. 参考

- `docs/prd/ARCHITECTURE-REVIEW-2026-05-21.md` P0-3
- `docs/ARCHITECTURE.md` §3.4 D6 Self-Verify Worker 独立部署 + §4.2 S2 部署
- `docs/prd/DECISIONS.md` D4 / D5 / D6 / D11 / D12
- `docs/prd/14-tech-architecture.md`(PRD 级)
- `docs/prd/18-evidence-and-attestation.md`(若存在,详 Evidence pipeline)
