#!/bin/bash
# 远程服务器代码同步脚本
# 使用方式: ./sync_remote.sh
# 策略: 以本地工作区为准推送到远程，同时保留远程环境专属配置文件

set -euo pipefail

REMOTE_USER="ubuntu"
REMOTE_HOST="118.89.83.96"
REMOTE_PASS="asdf#234@!"
SSHPASS="/opt/homebrew/bin/sshpass"
SSH_OPTS="-o ControlMaster=no -o ServerAliveInterval=30 -o StrictHostKeyChecking=no"
REMOTE_ROOT="/home/ubuntu/AI-generatedVideo"
RSYNC_RSH="ssh ${SSH_OPTS}"

ssh_exec() {
  $SSHPASS -p "$REMOTE_PASS" ssh $SSH_OPTS "$REMOTE_USER@$REMOTE_HOST" "$@"
}

if ! command -v rsync >/dev/null 2>&1; then
  echo "ERROR: 未检测到 rsync，请先安装 rsync 后重试"
  exit 1
fi

echo "=== [1/4] 本地代码推送到远程（保留远程环境文件） ==="
ssh_exec "mkdir -p ${REMOTE_ROOT}"
$SSHPASS -p "$REMOTE_PASS" rsync -az --delete \
  --exclude '.git/' \
  --exclude '.DS_Store' \
  --exclude 'autoVideo/frontend/node_modules/' \
  --exclude 'autoVideo/frontend/.next/' \
  --exclude 'autoVideo/run/' \
  --exclude 'autoVideo/tmp/' \
  --exclude 'autoVideo/config.docker.local.yaml' \
  --exclude 'autoVideo/config.local.yaml' \
  --exclude 'autoVideo/services/gateway-service/config.local.yaml' \
  --exclude 'autoVideo/frontend/.env.local' \
  --exclude 'autoVideo/infra/.env' \
  -e "$RSYNC_RSH" \
  ./ "$REMOTE_USER@$REMOTE_HOST:${REMOTE_ROOT}/"
echo "代码推送完成"

echo ""
echo "=== [2/4] 远程代码状态 ==="
ssh_exec "
cd ${REMOTE_ROOT}
git rev-parse --short HEAD 2>/dev/null || true
git status --short
"

echo ""
echo "=== [3/4] 重建 Docker 镜像 ==="
ssh_exec "
set -euo pipefail
cd ${REMOTE_ROOT}/autoVideo
echo '--- 构建 auth-service ---'
sudo docker build -t autovideo/auth:latest services/auth-service 2>&1 | tail -5
echo '--- 构建 project-service ---'
sudo docker build -t autovideo/project:latest services/project-service 2>&1 | tail -5
echo '--- 构建 script-service ---'
sudo docker build -t autovideo/script:latest services/script-service 2>&1 | tail -5
echo '--- 构建 character-service ---'
sudo docker build -t autovideo/character:latest services/character-service 2>&1 | tail -5
echo '--- 构建 image-service ---'
sudo docker build -t autovideo/image:latest services/image-service 2>&1 | tail -5
echo '--- 构建 video-service ---'
sudo docker build -t autovideo/video:latest services/video-service 2>&1 | tail -5
echo '--- 构建 task-service ---'
sudo docker build -t autovideo/task:latest services/task-service 2>&1 | tail -5
echo '--- 构建 model-service ---'
sudo docker build -t autovideo/model:latest services/model-service 2>&1 | tail -5
echo '--- 构建 storage-service ---'
sudo docker build -t autovideo/storage:latest services/storage-service 2>&1 | tail -5
"

echo ""
echo "=== [4/4] 重启容器 ==="
ssh_exec "
cd ${REMOTE_ROOT}/autoVideo/infra
sudo docker compose -f docker-compose.full.yml up --force-recreate --no-build --no-deps -d auth project script character image video task model storage
sleep 3
sudo docker ps | grep -E 'autovideo-(auth|project|script|character|image|video|task|model|storage)'
"

echo ""
echo "=== 同步完成！$(date) ==="
