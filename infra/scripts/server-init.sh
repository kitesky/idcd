#!/bin/bash
# 服务器首次初始化脚本
# 在目标服务器上以 root 运行一次: bash server-init.sh
set -e

DEPLOY_USER="${DEPLOY_USER:-deploy}"

echo "==> 创建部署目录结构"
mkdir -p /opt/idcd/config
mkdir -p /opt/idcd/nginx/ssl
mkdir -p /opt/idcd/certs

echo "==> 创建专用部署用户（无 sudo）"
if ! id "$DEPLOY_USER" &>/dev/null; then
  useradd -m -s /bin/bash "$DEPLOY_USER"
fi
usermod -aG docker "$DEPLOY_USER"

echo "==> 设置目录权限"
chown -R "$DEPLOY_USER":"$DEPLOY_USER" /opt/idcd

echo "==> 配置 SSH 公钥（粘贴 GitHub Actions 的 SSH 公钥）"
mkdir -p /home/"$DEPLOY_USER"/.ssh
chmod 700 /home/"$DEPLOY_USER"/.ssh
echo "# 在此粘贴 GitHub Actions SSH 公钥（对应 DEPLOY_SSH_KEY secret）" \
  >> /home/"$DEPLOY_USER"/.ssh/authorized_keys
chmod 600 /home/"$DEPLOY_USER"/.ssh/authorized_keys
chown -R "$DEPLOY_USER":"$DEPLOY_USER" /home/"$DEPLOY_USER"/.ssh

echo ""
echo "==> 完成。接下来："
echo "  1. 手动编辑 /home/$DEPLOY_USER/.ssh/authorized_keys，粘贴公钥"
echo "  2. 将 prod.env.yaml 上传到 /opt/idcd/config/prod.env.yaml"
echo "  3. 将 SSL 证书放到 /opt/idcd/nginx/ssl/"
echo "  4. 在 docker-compose.prod.yml 同目录创建 .env 文件，填写 GHCR_OWNER=你的GitHub用户名"
echo "  5. 首次手动运行: cd /opt/idcd && docker compose pull && docker compose up -d"
