#!/usr/bin/env bash
# =============================================================
# scripts/deploy.sh — autoVideo 线上一键部署
# 用法：bash scripts/deploy.sh [--env prod|staging] [--tag v1.0.0] [--skip-pull]
# =============================================================
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
source "$ROOT/scripts/config.sh"

ENV="$DEFAULT_ENV"
TAG="$DEFAULT_TAG"
SKIP_PULL=false

for arg in "$@"; do
  case $arg in
    --env=*)   ENV="${arg#*=}" ;;
    --tag=*)   TAG="${arg#*=}" ;;
    --skip-pull) SKIP_PULL=true ;;
  esac
done

log()  { echo -e "\033[1;35m[deploy:$ENV]\033[0m $*"; }
ok()   { echo -e "\033[1;32m[ok]\033[0m $*"; }
warn() { echo -e "\033[1;33m[warn]\033[0m $*"; }
err()  { echo -e "\033[1;31m[error]\033[0m $*" >&2; exit 1; }

rebuild_project_image() {
  local image="autovideo/project:${TAG}"
  local context="services/project-service"

  if [ ! -f "$context/Dockerfile" ]; then
    warn "未找到 $context/Dockerfile，跳过 project 显式重建"
    return
  fi

  log "显式重建 project 镜像（compose 为 image-only 服务）..."
  docker build -t "$image" "$context"
}

# ── 检查 .env 文件 ────────────────────────────────────────────
ENV_FILE="infra/.env.${ENV}"
if [ ! -f "$ENV_FILE" ]; then
  if [ -f "infra/.env" ]; then
    ENV_FILE="infra/.env"
    warn "使用 infra/.env（未找到 infra/.env.${ENV}）"
  else
    err "缺少环境变量文件：$ENV_FILE"
    err "请先：cp infra/.env.example infra/.env && 填入真实值"
  fi
fi

log "使用配置：$ENV_FILE，镜像标签：$TAG"

if ! command -v migrate >/dev/null 2>&1; then
  err "golang-migrate 未安装，终止部署"
fi

# ── 拉取最新代码（可选，CI 环境下通常已完成）──────────────────
if [ "${CI:-false}" = "false" ]; then
  log "拉取最新代码..."
  git pull origin main
fi

# ── 停止旧服务（保留基础设施）───────────────────────────────────
COMPOSE_FULL="$COMPOSE_FULL_FILE"
COMPOSE_INFRA="$COMPOSE_FILE"

if [ -f "$COMPOSE_FULL" ]; then
  log "停止旧应用服务..."
  docker compose -f "$COMPOSE_FULL" --env-file "$ENV_FILE" \
    stop auth project script character image frame-extractor video whisper-sidecar task model storage \
    2>/dev/null || true
fi

# ── 启动/更新基础设施（幂等）────────────────────────────────────
log "确保基础设施运行中..."
docker compose -f "$COMPOSE_INFRA" --env-file "$ENV_FILE" up -d

log "等待 PostgreSQL 就绪..."
until docker exec "$POSTGRES_CONTAINER" pg_isready -U postgres -q 2>/dev/null; do
  sleep 3
done
ok "PostgreSQL ✓"

# ── 数据库迁移 ────────────────────────────────────────────────
log "执行数据库迁移..."
# 从 env 文件读取密码
PG_PASS=$(grep POSTGRES_PASSWORD "$ENV_FILE" | cut -d= -f2 | tr -d '"' | tr -d ' ')
for svc_dir in services/*/migrations; do
  [ -d "$svc_dir" ] || continue
  svc_name=$(echo "$svc_dir" | sed 's|.*/\([^/]*\)/migrations|\1|' | sed 's/-service//')
  DB_URL="postgres://postgres:${PG_PASS}@localhost:5432/${svc_name}_db?sslmode=disable"

  migrate_output=""
  if migrate_output=$(migrate -path "$svc_dir" -database "$DB_URL" up 2>&1); then
    ok "migrate $svc_name ✓"
    continue
  fi

  if printf '%s' "$migrate_output" | grep -qi "no change"; then
    ok "migrate $svc_name 已是最新"
    continue
  fi

  printf '%s\n' "$migrate_output" >&2
  err "migrate $svc_name 失败"
done

# ── 拉取最新镜像并启动全量服务 ───────────────────────────────────
if [ -f "$COMPOSE_FULL" ]; then
  if [ "$SKIP_PULL" = true ]; then
    warn "跳过拉取镜像，直接使用服务器本地镜像（tag=$TAG）"
  else
    log "拉取最新镜像（tag=$TAG）..."
    AUTOVIDEO_TAG="$TAG" docker compose -f "$COMPOSE_FULL" --env-file "$ENV_FILE" pull
  fi

  rebuild_project_image

  log "启动全量服务..."
  AUTOVIDEO_TAG="$TAG" docker compose -f "$COMPOSE_FULL" --env-file "$ENV_FILE" up -d --remove-orphans

  log "强制刷新 project 容器以应用最新镜像..."
  AUTOVIDEO_TAG="$TAG" docker compose -f "$COMPOSE_FULL" --env-file "$ENV_FILE" up -d --no-deps --force-recreate project

  # 等待 Gateway 就绪
  log "等待 API Gateway 响应..."
  RETRY=0
  until curl -sf http://localhost:8000/healthz >/dev/null 2>&1; do
    RETRY=$((RETRY+1))
    [ $RETRY -gt 30 ] && err "API Gateway 30s 内未就绪"
    sleep 2
  done
  ok "API Gateway ✓ → http://localhost:8000"

  log "发布前端静态文件..."
  bash "$ROOT/scripts/export-frontend-static.sh" --env="$ENV"

  if docker ps -a --format '{{.Names}}' | grep -Fxq autovideo-frontend; then
    log "移除旧的 frontend 容器..."
    docker rm -f autovideo-frontend >/dev/null 2>&1 || true
  fi
else
  warn "未找到 $COMPOSE_FULL，仅启动了基础设施"
  warn "请先运行 bash scripts/build.sh 并生成 docker-compose.full.yml"
fi

# ── 清理旧镜像 ────────────────────────────────────────────────
log "清理悬空镜像..."
docker image prune -f >/dev/null 2>&1 || true

# ── 部署结果 ──────────────────────────────────────────────────
echo ""
ok "=== 部署完成 ==="
echo ""
docker compose -f "${COMPOSE_FULL:-$COMPOSE_INFRA}" --env-file "$ENV_FILE" ps --format "table {{.Name}}\t{{.Status}}\t{{.Ports}}" 2>/dev/null || \
docker compose -f "$COMPOSE_INFRA" ps
echo ""
ok "前端:    https://10003.klyhtest.com"
ok "Gateway: http://$(hostname -I | awk '{print $1}'):8000"
ok "MinIO:   http://$(hostname -I | awk '{print $1}'):9001"
