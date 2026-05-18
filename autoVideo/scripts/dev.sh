#!/usr/bin/env bash
# =============================================================
# scripts/dev.sh — autoVideo 本地开发一键启动
# 用法：bash scripts/dev.sh [--skip-infra] [--skip-migrate]
# =============================================================
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
source "$ROOT/scripts/config.sh"

CONFIG_FILE="${AUTOVIDEO_CONFIG_FILE:-$ROOT/config.local.yaml}"
GATEWAY_CONFIG_FILE="${AUTOVIDEO_GATEWAY_CONFIG_FILE:-$ROOT/services/gateway-service/config.local.yaml}"
export AUTOVIDEO_CONFIG_FILE="$CONFIG_FILE"
export AUTOVIDEO_GATEWAY_CONFIG_FILE="$GATEWAY_CONFIG_FILE"

SKIP_INFRA=false
SKIP_MIGRATE=false

for arg in "$@"; do
  case $arg in
    --skip-infra)    SKIP_INFRA=true ;;
    --skip-migrate)  SKIP_MIGRATE=true ;;
  esac
done

log()  { echo -e "\033[1;32m[dev]\033[0m $*"; }
warn() { echo -e "\033[1;33m[warn]\033[0m $*"; }
err()  { echo -e "\033[1;31m[error]\033[0m $*" >&2; exit 1; }

# ── 环境检查 ─────────────────────────────────────────────────
command -v docker   >/dev/null 2>&1 || err "需要 Docker，请先安装"
command -v go       >/dev/null 2>&1 || err "需要 Go 1.22+，请先安装"
command -v node     >/dev/null 2>&1 || err "需要 Node.js 20+，请先安装"

GO_VERSION=$(go version | sed -E 's/.*go([0-9]+\.[0-9]+).*/\1/')
NODE_VERSION=$(node -e "process.stdout.write(process.version.slice(1).split('.')[0])")
log "Go $GO_VERSION | Node $NODE_VERSION"

# ── Go 代理（国内加速）───────────────────────────────────────
if ! go env GOPROXY | grep -q "goproxy.cn"; then
  warn "设置 GOPROXY=https://goproxy.cn,direct"
  go env -w GOPROXY=https://goproxy.cn,direct
fi

# ── 确保 ~/go/bin 在 PATH 中（goreman 等工具）────────────────
export PATH="$HOME/go/bin:$PATH"

# ── 启动基础设施 ──────────────────────────────────────────────
if [ "$SKIP_INFRA" = false ]; then
  log "启动中间件（PostgreSQL / Redis / Kafka / MinIO / Monitoring）..."
  docker compose -f "$COMPOSE_FILE" up -d

  log "等待服务健康..."
  sleep 15

  # 验证 PostgreSQL
  until docker exec "$POSTGRES_CONTAINER" pg_isready -U postgres -q; do
    warn "等待 PostgreSQL..."
    sleep 3
  done
  log "PostgreSQL ✓"

  # 验证 Redis
  until docker exec "$REDIS_CONTAINER" redis-cli ping | grep -q PONG; do
    warn "等待 Redis..."
    sleep 2
  done
  log "Redis ✓"
fi

# ── 数据库迁移 ────────────────────────────────────────────────
if [ "$SKIP_MIGRATE" = false ]; then
  if command -v migrate >/dev/null 2>&1; then
    log "执行数据库迁移..."
    make migrate-all
    log "数据库迁移完成 ✓"
  else
    warn "golang-migrate 未安装，跳过迁移"
    warn "安装命令：go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest"
  fi
fi

# ── 前端依赖 ──────────────────────────────────────────────────
if [ ! -d "frontend/node_modules" ]; then
  log "安装前端依赖..."
  (cd frontend && npm install)
fi

# ── 配置文件检查 ──────────────────────────────────────────────
if [ ! -f "$CONFIG_FILE" ]; then
  warn "缺少共享配置文件：$CONFIG_FILE，请参考 autoVideo/docs/env.md 填入 API Key"
fi

if [ ! -f "$GATEWAY_CONFIG_FILE" ]; then
  warn "缺少网关配置文件：$GATEWAY_CONFIG_FILE"
fi

if [ ! -f "frontend/.env.local" ]; then
  cp frontend/.env.local.example frontend/.env.local
  warn "已创建 frontend/.env.local，如需自定义请编辑"
fi

# ── 启动所有服务 ──────────────────────────────────────────────
log "启动所有后端服务 + 前端..."

if command -v goreman >/dev/null 2>&1; then
  log "使用 goreman 启动（Procfile）"
  exec goreman start
else
  warn "goreman 未安装，改为后台启动各服务"
  warn "安装命令：go install github.com/mattn/goreman@latest"

  PIDS=()
  for svc in services/*/; do
    svc_name=$(basename "$svc")
    log "启动 $svc_name..."
    if [ "$svc_name" = "gateway-service" ]; then
      # gateway-service 需要 -config 参数，且优先使用编译好的二进制
      if [ -f "$svc/gateway-service" ]; then
        (cd "$svc" && ./gateway-service -config "$GATEWAY_CONFIG_FILE" 2>&1 | sed "s/^/[$svc_name] /") &
      else
        (cd "$svc" && go run ./cmd/main.go -config "$GATEWAY_CONFIG_FILE" 2>&1 | sed "s/^/[$svc_name] /") &
      fi
    elif [ "$svc_name" = "whisper-sidecar" ]; then
      # Python 服务单独处理
      if [ -f "$svc/main.py" ]; then
        (cd "$svc" && WHISPER_MODEL_SIZE="${WHISPER_MODEL_SIZE:-small}" PORT=8093 python main.py 2>&1 | sed "s/^/[$svc_name] /") &
      fi
    else
      (cd "$svc" && go run ./cmd/main.go 2>&1 | sed "s/^/[$svc_name] /") &
    fi
    PIDS+=($!)
  done

  log "启动前端..."
  (cd frontend && npm run dev 2>&1 | sed 's/^/[frontend] /') &
  PIDS+=($!)

  log ""
  log "所有服务已启动，按 Ctrl+C 停止"
  log "前端: http://localhost:3000"
  log "Gateway: http://localhost:8000"

  # Ctrl+C 时清理所有子进程
  trap 'log "停止所有服务..."; kill "${PIDS[@]}" 2>/dev/null; exit 0' INT TERM
  wait
fi
