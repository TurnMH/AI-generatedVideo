#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/config.sh"

if [ ! -d "$PROJECT_DIR" ] || [ ! -f "$PROJECT_DIR/scripts/dev.sh" ]; then
  echo "[start] 未找到 autoVideo 项目或 scripts/dev.sh"
  exit 1
fi

if [ ! -f "$APP_CONFIG" ]; then
  echo "[start] 未找到共享运行配置文件: $APP_CONFIG"
  exit 1
fi

if [ ! -f "$GATEWAY_CONFIG" ]; then
  echo "[start] 未找到网关配置文件: $GATEWAY_CONFIG"
  exit 1
fi

mkdir -p "$RUN_DIR"

if [ -f "$PID_FILE" ]; then
  OLD_PID="$(cat "$PID_FILE" 2>/dev/null || true)"
  if [ -n "${OLD_PID:-}" ] && kill -0 "$OLD_PID" 2>/dev/null; then
    echo "[start] 已在运行，PID=$OLD_PID"
    echo "[start] 日志: $LOG_FILE"
    exit 0
  fi
  rm -f "$PID_FILE"
fi

# macOS: 优先使用 GNU grep（避免 grep -P 兼容问题）
if command -v ggrep >/dev/null 2>&1; then
  export PATH="$(dirname "$(command -v ggrep)"):$PATH"
fi

# 尝试拉起 Docker Desktop（若已启动不会有副作用）
if [ "$(uname -s)" = "Darwin" ]; then
  open -a Docker >/dev/null 2>&1 || true
fi

ZOOKEEPER_IMAGE="${AUTOVIDEO_ZOOKEEPER_IMAGE:-docker.m.daocloud.io/confluentinc/cp-zookeeper:7.5.0}"
KAFKA_IMAGE="${AUTOVIDEO_KAFKA_IMAGE:-docker.m.daocloud.io/confluentinc/cp-kafka:7.5.0}"

if [ "$DOCKER_ACCESS_MODE" = "missing" ]; then
  echo "[start] 未检测到 Docker，请先安装 Docker 后重试"
  exit 1
fi

if [ "$DOCKER_ACCESS_MODE" = "sudo" ]; then
  echo "[start] 当前用户无 Docker 直连权限，已自动切换为 sudo docker"
fi

echo "[start] 等待 Docker daemon 就绪..."
for _ in $(seq 1 60); do
  if docker_cmd info >/dev/null 2>&1; then
    break
  fi
  sleep 2
done
if ! docker_cmd info >/dev/null 2>&1; then
  echo "[start] Docker 不可用：请确认 Docker daemon 已启动，且当前用户具备 docker 或 sudo docker 权限"
  exit 1
fi

cat > "$COMPOSE_OVERRIDE" <<EOF
services:
  zookeeper:
    image: ${ZOOKEEPER_IMAGE}
    environment:
      ZOOKEEPER_CLIENT_PORT: 2181
      ZOOKEEPER_TICK_TIME: 2000
    healthcheck:
      test: ["CMD-SHELL", "echo srvr | nc localhost 2181 | grep Mode"]
      interval: 10s
      timeout: 5s
      retries: 10

  kafka:
    image: ${KAFKA_IMAGE}
    depends_on:
      zookeeper:
        condition: service_healthy
    environment:
      KAFKA_BROKER_ID: 1
      KAFKA_ZOOKEEPER_CONNECT: zookeeper:2181
      KAFKA_LISTENER_SECURITY_PROTOCOL_MAP: PLAINTEXT:PLAINTEXT,PLAINTEXT_HOST:PLAINTEXT
      KAFKA_ADVERTISED_LISTENERS: PLAINTEXT://kafka:29092,PLAINTEXT_HOST://localhost:9092
      KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR: 1
      KAFKA_AUTO_CREATE_TOPICS_ENABLE: "true"
      KAFKA_LISTENERS: PLAINTEXT://0.0.0.0:29092,PLAINTEXT_HOST://0.0.0.0:9092
      KAFKA_INTER_BROKER_LISTENER_NAME: PLAINTEXT
    healthcheck:
      test: ["CMD-SHELL", "kafka-topics --bootstrap-server localhost:9092 --list >/dev/null 2>&1"]
      interval: 15s
      timeout: 10s
      retries: 10
EOF

echo "[start] 启动基础设施..."
docker_cmd compose -f "$COMPOSE_BASE" -f "$COMPOSE_OVERRIDE" up -d >/dev/null

echo "[start] 等待 PostgreSQL/Redis/Kafka 就绪..."
for _ in $(seq 1 40); do
  if docker_cmd exec autovideo-postgres pg_isready -U postgres -q >/dev/null 2>&1; then
    break
  fi
  sleep 2
done
for _ in $(seq 1 40); do
  if docker_cmd exec autovideo-redis redis-cli ping 2>/dev/null | grep -q PONG; then
    break
  fi
  sleep 2
done
for _ in $(seq 1 60); do
  if docker_cmd exec autovideo-kafka kafka-topics --bootstrap-server localhost:9092 --list >/dev/null 2>&1; then
    break
  fi
  sleep 3
done

echo "[start] 启动中，请稍候..."
# 检查并确保 pm2 已安装
if ! command -v pm2 >/dev/null 2>&1; then
  echo "[start] 未检测到 pm2，正在自动全局安装 pm2..."
  npm install -g pm2
fi

# Kill any stale processes still holding service ports (e.g. after a crash with no PID file).
for port in $PORTS; do
  pid="$(lsof -ti tcp:"$port" 2>/dev/null || true)"
  if [ -n "$pid" ]; then
    echo "[start] 释放端口 $port (PID $pid)"
    kill "$pid" 2>/dev/null || true
  fi
done

# 使用 PM2 启动项目
echo "[start] 使用 PM2 启动微服务集群..."
pm2 start "$PM2_CONFIG"

# PM2 会自己管理后台驻留，我们可以直接清理掉旧的PID逻辑
rm -f "$PID_FILE"

echo ""
echo "=================================================="
echo "[start] 🚀 所有服务已由 PM2 后台启动成功！"
echo ""
echo " 常用 PM2 命令指南:"
echo " - 查看所有服务状态: pm2 list"
echo " - 查看所有聚合日志: pm2 logs"
echo " - 仅查看视频服务日志: pm2 logs video-service"
echo " - 单独重启剧本服务: pm2 restart script-service"
echo " - 停止所有服务:     ./stop.sh  (或 pm2 stop all)"
echo "=================================================="
exit 0
