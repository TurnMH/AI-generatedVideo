#!/usr/bin/env bash
set -euo pipefail

source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/config.sh"

if command -v pm2 >/dev/null 2>&1; then
  echo "[stop] 使用 PM2 停止所有服务..."
  pm2 stop "$PM2_CONFIG" >/dev/null 2>&1 || true
  pm2 delete "$PM2_CONFIG" >/dev/null 2>&1 || true
fi

if [ -f "$PID_FILE" ]; then
  rm -f "$PID_FILE"
fi

# 清理常用服务端口占用（精确按端口 kill）
for p in $PORTS; do
  P="$(lsof -t -nP -iTCP:$p -sTCP:LISTEN | head -n1 || true)"
  if [ -n "${P:-}" ]; then
    echo "[stop] kill port=$p pid=$P"
    kill "$P" 2>/dev/null || true
  fi
done

sleep 1
for p in $PORTS; do
  P="$(lsof -t -nP -iTCP:$p -sTCP:LISTEN | head -n1 || true)"
  if [ -n "${P:-}" ]; then
    kill -9 "$P" 2>/dev/null || true
  fi
done

echo "[stop] 应用进程已清理"

if [ "$DOCKER_ACCESS_MODE" != "missing" ] && docker_cmd info >/dev/null 2>&1; then
    if [ -f "$COMPOSE_OVERRIDE" ]; then
      docker_cmd compose -f "$COMPOSE_BASE" -f "$COMPOSE_OVERRIDE" down >/dev/null 2>&1 || true
    else
      docker_cmd compose -f "$COMPOSE_BASE" down >/dev/null 2>&1 || true
    fi
    echo "[stop] 基础设施已停止"
fi
