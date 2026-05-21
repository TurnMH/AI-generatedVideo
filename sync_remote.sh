#!/bin/bash
# 远程服务器代码同步脚本
# 使用方式: ./sync_remote.sh
# 策略: 以本地工作区为代码来源，通过 rsync 同步到 /home/autoVideo

set -euo pipefail

REMOTE_USER="root"
REMOTE_HOST="47.236.188.141"
REMOTE_PASS='BsMD@!T8&8$$j#jJ'
SSHPASS="/opt/homebrew/bin/sshpass"
SSH_OPTS="-o ControlMaster=no -o ServerAliveInterval=30 -o StrictHostKeyChecking=no"
REMOTE_ROOT="/home/autoVideo"
REMOTE_WEB_ROOT="${REMOTE_ROOT}/web"
REMOTE_WEB_STAGING="${REMOTE_ROOT}/web.staging"
REMOTE_WEB_PREVIOUS="${REMOTE_ROOT}/web.previous"
REMOTE_SITE_HOST="10003.klyhtest.com"

LOCAL_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOCAL_COMMIT_SHORT="$(git -C "$LOCAL_ROOT" rev-parse --short HEAD)"
LOCAL_AUTOVIDEO_ROOT="${LOCAL_ROOT}/autoVideo"
LOCAL_FRONTEND_OUT="${LOCAL_AUTOVIDEO_ROOT}/frontend/out"

ssh_exec() {
  $SSHPASS -p "$REMOTE_PASS" ssh $SSH_OPTS "$REMOTE_USER@$REMOTE_HOST" "$@"
}

ensure_local_frontend_out() {
  if [ ! -f "$LOCAL_FRONTEND_OUT/index.html" ] || [ ! -f "$LOCAL_FRONTEND_OUT/404.html" ]; then
    echo "ERROR: 本地静态产物缺失 index.html 或 404.html，请重新构建前端"
    exit 1
  fi
}

publish_remote_frontend() {
  echo ""
  echo "=== [4/4] 原子发布远端前端静态文件 ==="

  ssh_exec "rm -rf ${REMOTE_WEB_STAGING} ${REMOTE_WEB_PREVIOUS} && mkdir -p ${REMOTE_WEB_STAGING}"

  $SSHPASS -p "$REMOTE_PASS" rsync -az --delete -e "ssh $SSH_OPTS" \
    "$LOCAL_FRONTEND_OUT"/ "$REMOTE_USER@$REMOTE_HOST:$REMOTE_WEB_STAGING/"

  ssh_exec "test -f ${REMOTE_WEB_STAGING}/index.html && test -f ${REMOTE_WEB_STAGING}/404.html"

  ssh_exec "
set -euo pipefail
if [ -e ${REMOTE_WEB_ROOT} ]; then
  mv ${REMOTE_WEB_ROOT} ${REMOTE_WEB_PREVIOUS}
fi
mv ${REMOTE_WEB_STAGING} ${REMOTE_WEB_ROOT}
chown -R www-data:www-data ${REMOTE_WEB_ROOT}
"

  if ! ssh_exec "
set -euo pipefail
code=\$(curl -sk -o /dev/null -w '%{http_code}' -H 'Host: ${REMOTE_SITE_HOST}' https://127.0.0.1/)
echo \"远端首页状态码: \$code\"
[ \"\$code\" = \"200\" ]
"; then
    echo "ERROR: 远端首页校验失败，开始回滚前端静态文件"
    ssh_exec "
set -euo pipefail
rm -rf ${REMOTE_WEB_ROOT}
if [ -e ${REMOTE_WEB_PREVIOUS} ]; then
  mv ${REMOTE_WEB_PREVIOUS} ${REMOTE_WEB_ROOT}
  chown -R www-data:www-data ${REMOTE_WEB_ROOT}
fi
rm -rf ${REMOTE_WEB_STAGING}
"
    exit 1
  fi

  ssh_exec "rm -rf ${REMOTE_WEB_PREVIOUS}"
}

render_local_docker_override() {
  local temp_override
  temp_override="$(mktemp)"
  python3 "$LOCAL_AUTOVIDEO_ROOT/scripts/render-docker-local-config.py" \
    --source "$LOCAL_AUTOVIDEO_ROOT/config.local.yaml" \
    --output "$temp_override" >/dev/null
  rm -f "$temp_override"
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
  render_local_docker_override
  bash scripts/export-frontend-static.sh --env=prod --build-only
)
ensure_local_frontend_out

echo ""
echo "=== [2/4] 同步本地工作区到远端 ==="
echo "将同步本地提交：${LOCAL_COMMIT_SHORT}"

$SSHPASS -p "$REMOTE_PASS" rsync -az --delete -e "ssh $SSH_OPTS" \
  --exclude '.git' \
  --exclude '.DS_Store' \
  --exclude 'node_modules' \
  --exclude 'web' \
  --exclude 'web/**' \
  --exclude 'web.staging' \
  --exclude 'web.staging/**' \
  --exclude 'web.previous' \
  --exclude 'web.previous/**' \
  --exclude 'autoVideo/frontend/node_modules' \
  --exclude 'autoVideo/frontend/.next' \
  --exclude 'autoVideo/frontend/out' \
  --exclude 'autoVideo/config.docker.local.yaml' \
  "$LOCAL_ROOT"/ "$REMOTE_USER@$REMOTE_HOST:$REMOTE_ROOT/"

echo ""
echo "=== [3/4] 远端重建并发布全量服务 ==="
ssh_exec "
set -euo pipefail
cd ${REMOTE_ROOT}/autoVideo
sudo -n env CI=true bash scripts/build.sh --env=prod --tag=latest --platform=linux/amd64
sudo -n env CI=true SKIP_FRONTEND_EXPORT=true bash scripts/deploy.sh --env=prod --tag=latest --skip-pull
"

publish_remote_frontend

echo ""
echo "=== 同步完成！$(date) ==="
