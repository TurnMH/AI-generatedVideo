#!/bin/bash
# 远程服务器代码同步脚本
# 使用方式: ./sync_remote.sh
# 策略: 以 Git 提交为唯一代码来源，要求本地提交已 push 到 origin

set -euo pipefail

REMOTE_USER="ubuntu"
REMOTE_HOST="118.89.83.96"
REMOTE_PASS="asdf#234@!"
SSHPASS="/opt/homebrew/bin/sshpass"
SSH_OPTS="-o ControlMaster=no -o ServerAliveInterval=30 -o StrictHostKeyChecking=no"
REMOTE_ROOT="/home/ubuntu/AI-generatedVideo"

LOCAL_BRANCH="$(git rev-parse --abbrev-ref HEAD)"
LOCAL_COMMIT="$(git rev-parse HEAD)"
LOCAL_COMMIT_SHORT="$(git rev-parse --short HEAD)"
ORIGIN_URL="$(git remote get-url origin)"

ssh_exec() {
  $SSHPASS -p "$REMOTE_PASS" ssh $SSH_OPTS "$REMOTE_USER@$REMOTE_HOST" "$@"
}

if ! command -v git >/dev/null 2>&1; then
  echo "ERROR: 未检测到 git，请先安装 git 后重试"
  exit 1
fi

echo "=== [1/4] 校验当前提交已推送到 origin ==="
ORIGIN_BRANCH_COMMIT="$(git ls-remote origin "refs/heads/${LOCAL_BRANCH}" | awk '{print $1}')"
if [ -z "${ORIGIN_BRANCH_COMMIT:-}" ]; then
  echo "ERROR: origin 上未找到分支 ${LOCAL_BRANCH}"
  exit 1
fi
if [ "$ORIGIN_BRANCH_COMMIT" != "$LOCAL_COMMIT" ]; then
  echo "ERROR: 当前提交 ${LOCAL_COMMIT_SHORT} 尚未 push 到 origin/${LOCAL_BRANCH}"
  echo "请先执行 git push，再运行 ./sync_remote.sh"
  exit 1
fi
echo "将部署提交：${LOCAL_COMMIT_SHORT} (${LOCAL_BRANCH})"

echo ""
echo "=== [2/4] 远程切换到指定 Git 提交 ==="
ssh_exec "
set -euo pipefail
if [ ! -d ${REMOTE_ROOT}/.git ]; then
  git clone --branch '${LOCAL_BRANCH}' '${ORIGIN_URL}' '${REMOTE_ROOT}'
fi
cd ${REMOTE_ROOT}
git remote set-url origin '${ORIGIN_URL}'
git fetch origin '${LOCAL_BRANCH}' --depth=1
git checkout '${LOCAL_BRANCH}'
git reset --hard '${LOCAL_COMMIT}'
git rev-parse --short HEAD
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
