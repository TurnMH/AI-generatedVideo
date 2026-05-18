#!/usr/bin/env bash
# =============================================================
# autoVideo 项目公共配置
# 由 scripts/dev.sh、build.sh、deploy.sh 共同引用
# =============================================================

COMPOSE_FILE="infra/docker-compose.yml"
COMPOSE_FULL_FILE="infra/docker-compose.full.yml"
MIGRATIONS_DIR="infra/migrations"

POSTGRES_CONTAINER="autovideo-postgres"
REDIS_CONTAINER="autovideo-redis"

DEFAULT_TAG="latest"
DEFAULT_ENV="prod"
DEFAULT_REGISTRY=""

# 服务列表（名称:目录，bash 3.2 兼容）
SERVICE_LIST=(
  "gateway:services/gateway-service"
  "auth:services/auth-service"
  "project:services/project-service"
  "script:services/script-service"
  "character:services/character-service"
  "image:services/image-service"
  "frame-extractor:services/frame-extractor-service"
  "video:services/video-service"
  "whisper-sidecar:services/whisper-sidecar"
  "task:services/task-service"
  "model:services/model-service"
  "storage:services/storage-service"
)

# 所有服务名
SERVICE_NAMES=""
for _entry in "${SERVICE_LIST[@]}"; do
  SERVICE_NAMES="${SERVICE_NAMES} ${_entry%%:*}"
done
SERVICE_NAMES="${SERVICE_NAMES# }"
