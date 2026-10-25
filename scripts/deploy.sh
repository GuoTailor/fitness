#!/usr/bin/env bash
# 一键部署：本机构建（跨平台交叉编译）→ 打包 → 上传 → 远端重启
# 用法：./scripts/deploy.sh user@host [/opt/fitness]
# 前置：本机 go/node；远端已按 server/deploy/ 配好 nginx + systemd；
#       生产配置与密钥在远端 /opt/fitness/configs/（不入包，不覆盖）。
set -euo pipefail

TARGET="${1:?用法: ./scripts/deploy.sh user@host [/opt/fitness]}"
REMOTE_DIR="${2:-/opt/fitness}"
STAGE=$(mktemp -d)
trap 'rm -rf "$STAGE"' EXIT

echo "==> 1/5 构建后端（linux/amd64）"
cd "$(dirname "$0")/../server"
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o "$STAGE/fitness-server" ./cmd/server

echo "==> 2/5 构建管理后台"
cd ../admin-web
npm ci --silent
npm run build --silent
mkdir -p "$STAGE/admin"
cp -r dist/* "$STAGE/admin/"

echo "==> 3/5 构建 H5 用户端（client/，产物为 history 路由静态站）"
cd ../client
npm ci --silent
npm run build:h5 --silent
mkdir -p "$STAGE/web"
cp -r dist/build/h5/* "$STAGE/web/"

echo "==> 4/5 打包上传"
tar -czf "$STAGE/pkg.tgz" -C "$STAGE" fitness-server admin web
scp "$STAGE/pkg.tgz" "$TARGET:/tmp/fitness-pkg.tgz"

echo "==> 5/5 远端切换（配置/密钥不动）"
ssh "$TARGET" bash -s <<REMOTE
set -euo pipefail
cd /tmp && rm -rf fitness-pkg && mkdir fitness-pkg
tar -xzf fitness-pkg.tgz -C fitness-pkg
install -m 0755 fitness-pkg/fitness-server $REMOTE_DIR/fitness-server.new
rm -rf /var/www/fitness-admin && cp -r fitness-pkg/admin /var/www/fitness-admin
rm -rf /var/www/fitness-web && cp -r fitness-pkg/web /var/www/fitness-web
systemctl stop fitness-server
mv $REMOTE_DIR/fitness-server.new $REMOTE_DIR/fitness-server
systemctl start fitness-server
sleep 2
systemctl is-active fitness-server && curl -sf http://127.0.0.1:8080/healthz
rm -rf fitness-pkg fitness-pkg.tgz
REMOTE

echo
echo "部署完成：API https://your-domain.com/healthz · H5 https://your-domain.com/ · 后台 /admin/"
