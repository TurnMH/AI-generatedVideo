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

render_docker_local_config() {
  local source_file="${AUTOVIDEO_CONFIG_SOURCE_FILE:-$ROOT/config.local.yaml}"
  local output_file="${AUTOVIDEO_CONFIG_OUTPUT_FILE:-$ROOT/config.docker.local.yaml}"
  local python_bin="${AUTOVIDEO_PYTHON:-python3}"

  if ! command -v "$python_bin" >/dev/null 2>&1; then
    err "缺少 $python_bin，无法生成 $output_file"
  fi

  if ! "$python_bin" -c 'import yaml' >/dev/null 2>&1; then
    err "$python_bin 缺少 PyYAML，无法生成 $output_file"
  fi

  if [ ! -f "$source_file" ]; then
    err "缺少源配置：$source_file"
  fi

  log "生成 docker override 配置：$output_file"
  "$python_bin" - "$source_file" "$output_file" <<'PY'
import copy
import sys
from pathlib import Path

import yaml


REQUIRED_PATHS = [
  ("image-service", "models", "openai_base"),
  ("image-service", "models", "openai_keys"),
  ("video-service", "models"),
  ("video-service", "ffmpeg"),
  ("video-service", "concurrency"),
  ("storage-service", "storage", "cdn_base_url"),
  ("project-service", "llm", "base_url"),
  ("project-service", "llm", "api_key"),
  ("project-service", "llm", "model"),
  ("character-service", "llm", "base_url"),
  ("character-service", "llm", "api_key"),
  ("character-service", "llm", "model"),
  ("script-service", "llm", "openai", "base_url"),
  ("script-service", "llm", "openai", "api_key"),
  ("script-service", "llm", "openai", "model"),
]


def read_yaml(path: Path) -> dict:
  try:
    with path.open("r", encoding="utf-8") as handle:
      data = yaml.safe_load(handle)
  except FileNotFoundError:
    raise SystemExit(f"source config not found: {path}")
  except yaml.YAMLError as exc:
    raise SystemExit(f"failed to parse yaml from {path}: {exc}")
  if data is None:
    return {}
  if not isinstance(data, dict):
    raise SystemExit(f"unexpected yaml root in {path}: expected mapping")
  return data


def get_nested(data: dict, path: tuple[str, ...]):
  node = data
  for part in path:
    if not isinstance(node, dict) or part not in node:
      return None
    node = node[part]
  return node


def pick_sections(source: dict) -> dict:
  override: dict[str, dict] = {}

  selection_map: dict[str, tuple[str, ...]] = {
    "project-service": ("llm", "concurrency"),
    "script-service": ("llm",),
    "character-service": ("llm", "gemini", "claude", "qwen", "zhipu", "concurrency"),
    "image-service": ("models",),
    "video-service": ("models", "ffmpeg", "concurrency"),
    "storage-service": ("storage",),
  }

  for service_name, keys in selection_map.items():
    service_source = source.get(service_name)
    if not isinstance(service_source, dict):
      continue

    service_override: dict[str, object] = {}
    for key in keys:
      value = service_source.get(key)
      if value is not None:
        service_override[key] = copy.deepcopy(value)

    if service_override:
      override[service_name] = service_override

  root_llm: dict[str, object] = {}
  project_llm = source.get("project-service", {}).get("llm") if isinstance(source.get("project-service"), dict) else None
  script_llm = source.get("script-service", {}).get("llm") if isinstance(source.get("script-service"), dict) else None
  character_llm = source.get("character-service", {}).get("llm") if isinstance(source.get("character-service"), dict) else None

  if isinstance(script_llm, dict):
    for key in ("provider",):
      value = script_llm.get(key)
      if value is not None:
        root_llm[key] = copy.deepcopy(value)
    for key in ("openai", "claude", "qwen", "zhipu"):
      value = script_llm.get(key)
      if value is not None:
        root_llm[key] = copy.deepcopy(value)

  if isinstance(project_llm, dict):
    for key in ("base_url", "api_key", "model", "timeout", "fallback_base_url", "fallback_api_key", "fallback_model"):
      value = project_llm.get(key)
      if value is not None:
        root_llm[key] = copy.deepcopy(value)

  if isinstance(character_llm, dict):
    for key in ("base_url", "api_key", "model", "vision_model", "timeout"):
      value = character_llm.get(key)
      if value is not None:
        root_llm[key] = copy.deepcopy(value)

  if root_llm:
    override["llm"] = root_llm

  character_concurrency = source.get("character-service", {}).get("concurrency") if isinstance(source.get("character-service"), dict) else None
  if isinstance(character_concurrency, dict) and character_concurrency:
    override["concurrency"] = copy.deepcopy(character_concurrency)

  character_image = source.get("character-service", {}).get("image") if isinstance(source.get("character-service"), dict) else None
  default_model = character_image.get("default_model") if isinstance(character_image, dict) else None
  if isinstance(default_model, str) and default_model.strip():
    service_override = override.setdefault("character-service", {})
    image_override = service_override.setdefault("image", {})
    image_override["default_model"] = default_model.strip()

  return override


def validate_required_values(source: dict) -> None:
  missing: list[str] = []
  for path in REQUIRED_PATHS:
    value = get_nested(source, path)
    if value is None:
      missing.append(".".join(path))
      continue
    if isinstance(value, str) and not value.strip():
      missing.append(".".join(path))
      continue
    if isinstance(value, (list, tuple, dict)) and len(value) == 0:
      missing.append(".".join(path))
  if missing:
    joined = ", ".join(missing)
    raise SystemExit(f"missing required runtime LLM values in config.local.yaml: {joined}")


source_path = Path(sys.argv[1])
output_path = Path(sys.argv[2])
source = read_yaml(source_path)
validate_required_values(source)
override = pick_sections(source)

if not override:
  raise SystemExit("no override sections were selected from config.local.yaml")

output_path.parent.mkdir(parents=True, exist_ok=True)
with output_path.open("w", encoding="utf-8") as handle:
  yaml.safe_dump(override, handle, sort_keys=False, allow_unicode=True, default_flow_style=False)

print(f"rendered {output_path} from {source_path}")
PY
}

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

ensure_kafka_topics() {
  local kafka_container="autovideo-kafka"
  local -a topics=(
    "storyboard.generate.request"
    "storyboard.generate.result"
    "script.analyze.request"
    "script.analyze.result"
    "script.quick_generate.request"
    "script.quick_generate.result"
    "asset.generate.request"
    "asset.generate.result"
    "image.generate.request"
    "image.generate.result"
    "video.generate.request"
    "video.generate.result"
    "music.generate.request"
    "music.generate.result"
    "task.completed"
    "task.failed"
    "task.progress"
  )

  log "等待 Kafka 就绪..."
  until docker exec "$kafka_container" kafka-topics --bootstrap-server localhost:9092 --list >/dev/null 2>&1; do
    sleep 3
  done
  ok "Kafka ✓"

  log "检查并创建必需的 Kafka topics..."
  for topic in "${topics[@]}"; do
    docker exec "$kafka_container" kafka-topics \
      --bootstrap-server localhost:9092 \
      --create \
      --if-not-exists \
      --topic "$topic" \
      --partitions 1 \
      --replication-factor 1 >/dev/null
  done
  ok "Kafka topics ✓"
}

if [[ "${BASH_SOURCE[0]}" != "$0" ]]; then
  return 0
fi

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

# ── 生成 docker override 配置，补齐运行时 LLM 密钥 ─────────────────────
render_docker_local_config

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

ensure_kafka_topics

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

  # Gateway 只在启动时读取 config.local.yaml；配置变更后必须重建容器，否则
  # /storyboards/:id/dubbing 等 pattern 路由会继续走 project-service 并返回 404。
  log "强制刷新 gateway 容器以加载最新路由配置..."
  AUTOVIDEO_TAG="$TAG" docker compose -f "$COMPOSE_FULL" --env-file "$ENV_FILE" up -d --no-deps --force-recreate gateway

  # 等待 Gateway 就绪
  log "等待 API Gateway 响应..."
  RETRY=0
  until curl -sf http://localhost:8000/healthz >/dev/null 2>&1; do
    RETRY=$((RETRY+1))
    [ $RETRY -gt 30 ] && err "API Gateway 30s 内未就绪"
    sleep 2
  done
  ok "API Gateway ✓ → http://localhost:8000"

  if [ "${SKIP_FRONTEND_EXPORT:-false}" = "true" ]; then
    warn "跳过前端静态文件发布（由外部同步流程负责）"
  else
    log "发布前端静态文件..."
    bash "$ROOT/scripts/export-frontend-static.sh" --env="$ENV"
  fi

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
