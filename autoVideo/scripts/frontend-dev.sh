#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FRONTEND_DIR="$ROOT/frontend"
PORT="${1:-3000}"

find_existing_frontend_pid() {
  lsof -t -nP -iTCP:"$PORT" -sTCP:LISTEN 2>/dev/null | head -n1 || true
}

is_next_frontend_process() {
  local pid="$1"
  local cmd
  cmd="$(ps -p "$pid" -o command= 2>/dev/null || true)"
  [[ "$cmd" == *"next-server"* || "$cmd" == *"next dev"* ]]
}

pid="$(find_existing_frontend_pid)"
if [[ -n "$pid" ]]; then
  if is_next_frontend_process "$pid"; then
    echo "[frontend] detected existing Next dev on port $PORT (PID=$pid), reusing it"
    while kill -0 "$pid" 2>/dev/null; do
      sleep 5
    done
    echo "[frontend] previous Next dev exited, starting a new one on port $PORT"
  else
    cmd="$(ps -p "$pid" -o command= 2>/dev/null || true)"
    echo "[frontend] port $PORT is already in use by another process: ${cmd:-PID $pid}" >&2
    exit 1
  fi
fi

cd "$FRONTEND_DIR"
exec npm run dev -- -p "$PORT"
