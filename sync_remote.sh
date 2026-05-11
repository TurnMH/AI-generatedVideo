#!/bin/bash
# 远程服务器代码同步脚本
# 使用方式: ./sync_remote.sh
# 前提: 本地 VPN 已开启，代理端口 7897

set -e

REMOTE_USER="ubuntu"
REMOTE_HOST="118.89.83.96"
REMOTE_PASS="asdf#234@!"
LOCAL_PROXY_PORT="7897"
REMOTE_PROXY_PORT="17897"
SSHPASS="/opt/homebrew/bin/sshpass"
SSH_OPTS="-o ControlMaster=no -o ServerAliveInterval=30 -o StrictHostKeyChecking=no"

ssh_exec() {
  $SSHPASS -p "$REMOTE_PASS" ssh $SSH_OPTS "$REMOTE_USER@$REMOTE_HOST" "$@"
}

echo "=== [1/4] 建立 SSH 反向隧道（本地代理 → 远程 $REMOTE_PROXY_PORT）==="
$SSHPASS -p "$REMOTE_PASS" ssh $SSH_OPTS -N \
  -R "${REMOTE_PROXY_PORT}:127.0.0.1:${LOCAL_PROXY_PORT}" \
  "$REMOTE_USER@$REMOTE_HOST" &
TUNNEL_PID=$!
sleep 3
if ! kill -0 $TUNNEL_PID 2>/dev/null; then
  echo "ERROR: 隧道建立失败，请确认 VPN 已开启且代理端口为 $LOCAL_PROXY_PORT"
  exit 1
fi
echo "隧道已建立 (PID: $TUNNEL_PID)"

# 确保退出时关闭隧道
trap "kill $TUNNEL_PID 2>/dev/null; echo '隧道已关闭'" EXIT

echo ""
echo "=== [2/4] git pull 最新代码 ==="
ssh_exec "
cd /home/ubuntu/AI-generatedVideo
http_proxy=http://127.0.0.1:${REMOTE_PROXY_PORT} https_proxy=http://127.0.0.1:${REMOTE_PROXY_PORT} \
  git pull origin main --depth=1 2>&1
echo 'git pull 完成'
"

echo ""
echo "=== [3/4] 重建 Docker 镜像 ==="
ssh_exec "
cd /home/ubuntu/AI-generatedVideo/autoVideo
echo '--- 构建 image-service ---'
sudo docker build -t autovideo/image:latest -f services/image-service/Dockerfile . 2>&1 | tail -5
echo '--- 构建 video-service ---'
sudo docker build -t autovideo/video:latest -f services/video-service/Dockerfile . 2>&1 | tail -5
"

echo ""
echo "=== [4/4] 重启容器 ==="
ssh_exec "
cd /home/ubuntu/AI-generatedVideo/autoVideo/infra
sudo docker compose -f docker-compose.full.yml up --force-recreate --no-build --no-deps -d image video
sleep 3
sudo docker ps | grep -E 'autovideo-image|autovideo-video'
"

echo ""
echo "=== 同步完成！$(date) ==="
