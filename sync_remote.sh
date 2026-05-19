#!/bin/bash
# 远程服务器代码同步脚本
# 使用方式: ./sync_remote.sh
# 策略: 以本地工作区为代码来源，通过 rsync 同步到 /home/autoVideo

set -euo pipefail

REMOTE_USER="root"
REMOTE_HOST="47.236.188.141"
REMOTE_PASS=""
SSHPASS="/opt/homebrew/bin/sshpass"
SSH_OPTS="-o ControlMaster=no -o ServerAliveInterval=30 -o StrictHostKeyChecking=no"
REMOTE_ROOT="/home/autoVideo"
REMOTE_WEB_ROOT="${REMOTE_ROOT}/web"

LOCAL_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOCAL_COMMIT_SHORT="$(git -C "$LOCAL_ROOT" rev-parse --short HEAD)"
LOCAL_AUTOVIDEO_ROOT="${LOCAL_ROOT}/autoVideo"
LOCAL_FRONTEND_OUT="${LOCAL_AUTOVIDEO_ROOT}/frontend/out"

ssh_exec() {
  $SSHPASS -p "$REMOTE_PASS" ssh $SSH_OPTS "$REMOTE_USER@$REMOTE_HOST" "$@"
}

if ! command -v git >/dev/null 2>&1; then
  echo "ERROR: 未检测到 git，请先安装 git 后重试"
  exit 1
fi

if ! command -v npm >/dev/null 2>&1; then
  echo "ERROR: 未检测到 npm，本地无法生成前端静态产物"
  exit 1
fi

echo "=== [1/3] 本地构建前端静态产物 ==="
(
  cd "$LOCAL_AUTOVIDEO_ROOT"
  bash scripts/export-frontend-static.sh --env=prod --build-only
)

echo ""
echo "=== [2/3] 同步本地工作区到远端 ==="
echo "将同步本地提交：${LOCAL_COMMIT_SHORT}"

$SSHPASS -p "$REMOTE_PASS" rsync -az --delete -e "ssh $SSH_OPTS" \
  --exclude '.git' \
  --exclude '.DS_Store' \
  --exclude 'node_modules' \
  --exclude 'autoVideo/frontend/node_modules' \
  --exclude 'autoVideo/frontend/.next' \
  --exclude 'autoVideo/frontend/out' \
  "$LOCAL_ROOT"/ "$REMOTE_USER@$REMOTE_HOST:$REMOTE_ROOT/"

echo ""
echo "=== [3/3] 远端重建并发布全量服务 ==="
ssh_exec "
set -euo pipefail
cd ${REMOTE_ROOT}/autoVideo
sudo -n env CI=true bash scripts/build.sh --env=prod --tag=latest --platform=linux/amd64
sudo -n env CI=true SKIP_FRONTEND_EXPORT=true bash scripts/deploy.sh --env=prod --tag=latest --skip-pull
"

ssh_exec "sudo -n mkdir -p ${REMOTE_WEB_ROOT}"
$SSHPASS -p "$REMOTE_PASS" rsync -az --delete -e "ssh $SSH_OPTS" \
  "$LOCAL_FRONTEND_OUT"/ "$REMOTE_USER@$REMOTE_HOST:$REMOTE_WEB_ROOT/"
ssh_exec "sudo -n chown -R www-data:www-data ${REMOTE_WEB_ROOT}"

echo ""
echo "=== 同步完成！$(date) ==="
