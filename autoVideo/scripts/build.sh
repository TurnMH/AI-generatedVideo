#!/usr/bin/env bash
# =============================================================
# scripts/build.sh — 构建所有服务的 Docker 镜像
# 用法：bash scripts/build.sh [--push] [--tag v1.0.0] [--env prod|staging]
#       --push   构建后推送到镜像仓库
#       --tag    镜像标签（默认 latest）
# =============================================================
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
source "$ROOT/scripts/config.sh"

PUSH=false
TAG="$DEFAULT_TAG"
REGISTRY="$DEFAULT_REGISTRY"
ENV="$DEFAULT_ENV"
PLATFORM=""

for arg in "$@"; do
  case $arg in
    --push)         PUSH=true ;;
    --tag=*)        TAG="${arg#*=}" ;;
    --registry=*)   REGISTRY="${arg#*=}" ;;
    --env=*)        ENV="${arg#*=}" ;;
    --platform=*)   PLATFORM="${arg#*=}" ;;
  esac
done

log()  { echo -e "\033[1;36m[build]\033[0m $*"; }
ok()   { echo -e "\033[1;32m[ok]\033[0m $*"; }
err()  { echo -e "\033[1;31m[error]\033[0m $*" >&2; exit 1; }

command -v docker >/dev/null 2>&1 || err "需要 Docker"
[ -n "$PLATFORM" ] && docker buildx version >/dev/null 2>&1 || true
[ -n "$PLATFORM" ] && docker buildx version >/dev/null 2>&1 || err "指定 --platform 时需要 Docker buildx"

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

FRONTEND_NEXT_PUBLIC_API_URL="$(read_env_value NEXT_PUBLIC_API_URL /)"
FRONTEND_NEXT_PUBLIC_WS_URL="$(read_env_value NEXT_PUBLIC_WS_URL '')"
FRONTEND_API_PROXY_TARGET="$(read_env_value API_PROXY_TARGET http://gateway:8000)"

FAILED=()

for entry in "${SERVICE_LIST[@]}"; do
  name="${entry%%:*}"
  dir="${entry#*:}"
  if [ ! -f "$dir/Dockerfile" ]; then
    log "跳过 $name（无 Dockerfile）"
    continue
  fi

  IMAGE="autovideo/${name}:${TAG}"
  [ -n "$REGISTRY" ] && IMAGE="${REGISTRY}/autovideo-${name}:${TAG}"

  log "构建 $name → $IMAGE"
  if [ -n "$PLATFORM" ]; then
    BUILD_CMD=(docker buildx build --platform "$PLATFORM" --load -t "$IMAGE" "$dir" --build-arg BUILD_DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)")
  else
    BUILD_CMD=(docker build -t "$IMAGE" "$dir" --build-arg BUILD_DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)")
  fi

  if "${BUILD_CMD[@]}" 2>&1 | tail -3; then
    ok "$name ✓"
    if [ "$PUSH" = true ]; then
      log "推送 $IMAGE..."
      docker push "$IMAGE"
    fi
  else
    echo -e "\033[1;31m[error]\033[0m $name 构建失败"
    FAILED+=("$name")
  fi
done

# ── 前端静态导出 ────────────────────────────────────────────────
log "frontend 已切换为静态部署，跳过 Docker 镜像构建"
log "如需发布前端，请运行: bash scripts/export-frontend-static.sh --env=${ENV}"

# ── 结果汇总 ──────────────────────────────────────────────────
echo ""
if [ ${#FAILED[@]} -eq 0 ]; then
  ok "全部镜像构建成功 🎉"
  echo ""
  docker images | grep "autovideo" | head -20
else
  err "以下服务构建失败：${FAILED[*]}"
fi
