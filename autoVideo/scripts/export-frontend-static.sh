#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
source "$ROOT/scripts/config.sh"

ENV="$DEFAULT_ENV"
TARGET_DIR="${FRONTEND_STATIC_DIR:-/home/autoVideo/web}"
SKIP_INSTALL=false

for arg in "$@"; do
  case $arg in
    --env=*) ENV="${arg#*=}" ;;
    --target=*) TARGET_DIR="${arg#*=}" ;;
    --skip-install) SKIP_INSTALL=true ;;
  esac
done

log()  { echo -e "\033[1;36m[frontend-static]\033[0m $*"; }
ok()   { echo -e "\033[1;32m[ok]\033[0m $*"; }
err()  { echo -e "\033[1;31m[error]\033[0m $*" >&2; exit 1; }

command -v npm >/dev/null 2>&1 || err "需要 npm"

ENV_FILE="infra/.env.${ENV}"
if [ ! -f "$ENV_FILE" ] && [ -f "infra/.env" ]; then
  ENV_FILE="infra/.env"
fi

read_env_value() {
  local key="$1"
  local fallback="${2:-}"

  if [ -n "${!key:-}" ]; then
    printf '%s' "${!key}"
    return
  fi

  if [ -f "$ENV_FILE" ]; then
    local line
    line="$(grep -E "^${key}=" "$ENV_FILE" | tail -n 1 || true)"
    if [ -n "$line" ]; then
      printf '%s' "${line#*=}"
      return
    fi
  fi

  printf '%s' "$fallback"
}

export NEXT_PUBLIC_API_URL="$(read_env_value NEXT_PUBLIC_API_URL /)"
export NEXT_PUBLIC_WS_URL="$(read_env_value NEXT_PUBLIC_WS_URL '')"
export API_PROXY_TARGET="$(read_env_value API_PROXY_TARGET http://127.0.0.1:8000)"
export NEXT_TELEMETRY_DISABLED=1

if [ "$SKIP_INSTALL" = false ] && [ ! -d "frontend/node_modules" ]; then
  log "安装前端依赖..."
  (cd frontend && npm ci --prefer-offline)
fi

log "构建前端静态导出..."
(cd frontend && npm run build)

STAGING_DIR="${TARGET_DIR}.staging"
log "同步静态文件到 $TARGET_DIR"
sudo rm -rf "$STAGING_DIR"
sudo mkdir -p "$STAGING_DIR"

if command -v rsync >/dev/null 2>&1; then
  sudo rsync -a --delete frontend/out/ "$STAGING_DIR/"
else
  sudo find "$STAGING_DIR" -mindepth 1 -maxdepth 1 -exec rm -rf {} +
  sudo cp -a frontend/out/. "$STAGING_DIR/"
fi

sudo mkdir -p "$(dirname "$TARGET_DIR")"
sudo rm -rf "$TARGET_DIR"
sudo mv "$STAGING_DIR" "$TARGET_DIR"
sudo chown -R www-data:www-data "$TARGET_DIR"

ok "前端静态文件已发布到 $TARGET_DIR"