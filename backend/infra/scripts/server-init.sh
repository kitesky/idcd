#!/usr/bin/env bash
# idcd 服务器首次初始化脚本
#
# 目标: 在一台纯净 Ubuntu 22.04 / 24.04 上,把 idcd 全栈跑起来所需的
# 系统环境一次性配齐 — 不引入宝塔等面板,纯 apt + ufw + fail2ban + docker。
#
# 用法 (以 root 执行):
#   bash server-init.sh
#
# 幂等: 重复运行只会 reapply 配置,不会重建 docker / 重置防火墙规则。
#
# 后续手动 step 在脚本结尾打印。

set -euo pipefail

# ── 可配置参数 ────────────────────────────────────────────────────
DEPLOY_USER="${DEPLOY_USER:-deploy}"
SSH_PORT="${SSH_PORT:-22}"
GATEWAY_PORT="${GATEWAY_PORT:-8443}"  # backend/apps/gateway agent mTLS WSS

# 确认以 root 跑
if [[ "${EUID}" -ne 0 ]]; then
  echo "ERROR: 必须以 root 运行 (sudo bash $0)" >&2
  exit 1
fi

# 检测发行版,只支持 Ubuntu
if ! grep -qi "ubuntu" /etc/os-release 2>/dev/null; then
  echo "ERROR: 只测试过 Ubuntu 22.04 / 24.04,你的系统是:" >&2
  cat /etc/os-release | head -3 >&2
  exit 1
fi

echo "==> [1/7] apt update + 装基础工具"
export DEBIAN_FRONTEND=noninteractive
apt-get update -y
apt-get install -y \
  ca-certificates curl wget gnupg lsb-release \
  openssl jq git vim htop ncdu \
  ufw fail2ban \
  postgresql-client-16 redis-tools

echo "==> [2/7] 装 Docker Engine + Compose plugin (官方仓库)"
if ! command -v docker >/dev/null 2>&1; then
  install -m 0755 -d /etc/apt/keyrings
  curl -fsSL https://download.docker.com/linux/ubuntu/gpg | \
    gpg --dearmor -o /etc/apt/keyrings/docker.gpg
  chmod a+r /etc/apt/keyrings/docker.gpg

  echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] \
https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo "${VERSION_CODENAME}") stable" \
    > /etc/apt/sources.list.d/docker.list

  apt-get update -y
  apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
  systemctl enable --now docker
else
  echo "    docker 已存在,跳过"
fi
docker --version
docker compose version

echo "==> [3/7] 装 goose (DB migration 工具)"
if ! command -v goose >/dev/null 2>&1; then
  # 二进制安装,避免拉 Go toolchain
  GOOSE_VER="v3.22.1"
  ARCH=$(uname -m); case "$ARCH" in x86_64) ARCH=x86_64 ;; aarch64) ARCH=arm64 ;; esac
  curl -fsSL "https://github.com/pressly/goose/releases/download/${GOOSE_VER}/goose_linux_${ARCH}" \
    -o /usr/local/bin/goose
  chmod +x /usr/local/bin/goose
fi
goose -version

echo "==> [4/7] 部署目录结构 (/opt/idcd)"
mkdir -p /opt/idcd/config
mkdir -p /opt/idcd/nginx/ssl
mkdir -p /opt/idcd/nginx/conf.d
mkdir -p /opt/idcd/certs           # gateway mTLS CA
mkdir -p /opt/idcd/data/postgres   # 全栈模式 PG 数据 (云托管时忽略)
mkdir -p /opt/idcd/data/redis      # 同上
mkdir -p /opt/idcd/data/prom       # Prometheus tsdb
mkdir -p /opt/idcd/data/grafana
mkdir -p /opt/idcd/data/loki
mkdir -p /opt/idcd/backup
chmod 750 /opt/idcd /opt/idcd/config /opt/idcd/certs

echo "==> [5/7] 创建 deploy 用户 (CI/CD ssh 进来用)"
if ! id "$DEPLOY_USER" &>/dev/null; then
  useradd -m -s /bin/bash "$DEPLOY_USER"
fi
usermod -aG docker "$DEPLOY_USER"
mkdir -p "/home/$DEPLOY_USER/.ssh"
chmod 700 "/home/$DEPLOY_USER/.ssh"
touch "/home/$DEPLOY_USER/.ssh/authorized_keys"
chmod 600 "/home/$DEPLOY_USER/.ssh/authorized_keys"
chown -R "$DEPLOY_USER:$DEPLOY_USER" "/home/$DEPLOY_USER/.ssh" /opt/idcd

echo "==> [6/7] UFW 防火墙 (deny in default, 只开必要端口)"
ufw --force reset >/dev/null
ufw default deny incoming
ufw default allow outgoing
ufw allow "${SSH_PORT}/tcp" comment "SSH"
ufw allow 80/tcp comment "HTTP (nginx)"
ufw allow 443/tcp comment "HTTPS (nginx)"
ufw allow "${GATEWAY_PORT}/tcp" comment "agent gateway mTLS WSS"
ufw --force enable
ufw status verbose | sed 's/^/    /'

echo "==> [7/7] fail2ban (SSH 暴力破解防护)"
cat > /etc/fail2ban/jail.d/sshd.local <<EOF
[sshd]
enabled = true
port    = ${SSH_PORT}
maxretry = 5
findtime = 10m
bantime  = 1h
EOF
systemctl enable --now fail2ban
systemctl restart fail2ban

echo ""
echo "════════════════════════════════════════════════════════════════"
echo " ✅ 系统初始化完成。后续手动 step:"
echo "════════════════════════════════════════════════════════════════"
echo ""
echo "  1) 把你的 SSH 公钥加到 /home/$DEPLOY_USER/.ssh/authorized_keys"
echo "     (CI/CD 用 deploy 用户登录;root 维持原 ssh key)"
echo ""
echo "  2) 准备 /opt/idcd/config/prod.env.yaml"
echo "     模板: config/prod.env.example.yaml"
echo "     ⚠️ 必填:database.dsn / redis / jwt.secret / encryption.field_key /"
echo "            payment / server.admin_token / cert_svc_url=http://cert-svc:8080"
echo ""
echo "  3) 准备 /opt/idcd/config/cert-svc.env"
echo "     模板: backend/infra/docker/cert-svc.env.example"
echo "     ⚠️ 严禁复用 dev 全零 MASTER_KEY"
echo "     生成: openssl rand -base64 32"
echo ""
echo "  4) 把 nginx 配置 + TLS 证书放好"
echo "     /opt/idcd/nginx/nginx.conf            (从 backend/infra/nginx/nginx.conf 拷贝)"
echo "     /opt/idcd/nginx/conf.d/admin-allowlist.conf  (运维 IP allow 段)"
echo "     /opt/idcd/nginx/ssl/api.idcd.com.crt + .key"
echo ""
echo "  5) 准备 docker compose 入口"
echo "     mkdir /opt/idcd/stack && cd /opt/idcd/stack"
echo "     scp docker-compose.prod.yml + docker-compose.core.yml 过来"
echo "     echo 'GHCR_OWNER=你的GitHub用户名' > .env"
echo "     # 同时建议把 cert-svc.env 等敏感 env 加 600 权限:"
echo "     chmod 600 /opt/idcd/config/*.yaml /opt/idcd/config/cert-svc.env"
echo ""
echo "  6) 跑 migration (deploy.sh 会自动跑,但首次可以手动验证):"
echo "     goose -dir lib/db/migrations/idcd_main postgres \"\${DB_DSN}\" up"
echo ""
echo "  7) 启动栈:"
echo "     cd /opt/idcd/stack"
echo "     docker compose -f docker-compose.core.yml up -d   # 仅本机自托管 PG/Redis 时"
echo "     docker compose -f docker-compose.prod.yml pull"
echo "     docker compose -f docker-compose.prod.yml up -d"
echo ""
echo "  8) 烟测:"
echo "     curl -fsS http://127.0.0.1:8080/health      # api"
echo "     curl -fsS http://127.0.0.1:8086/healthz     # cert-svc (compose 映射)"
echo ""
