#!/usr/bin/env bash
# 项目公共配置，由 start.sh / stop.sh 共同引用

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
STARTUP_CONFIG="$ROOT/startup.config.json"

if [ ! -f "$STARTUP_CONFIG" ]; then
	echo "[config] 缺少启动配置文件: $STARTUP_CONFIG" >&2
	exit 1
fi

STARTUP_EXPORTS="$(node - "$ROOT" "$STARTUP_CONFIG" <<'EOF'
const path = require("path")

const root = process.argv[2]
const configPath = process.argv[3]
const config = require(configPath)

const projectDir = path.join(root, config.projectDir)
const runDir = path.join(projectDir, config.runDir)
const ports = (config.ports || []).join(" ")
const pm2Config = path.join(root, config.pm2Config)
const appConfig = path.join(root, config.sharedConfig)
const gatewayConfig = path.join(root, config.gatewayConfig)

process.stdout.write([
	`PROJECT_DIR=${JSON.stringify(projectDir)}`,
	`RUN_DIR=${JSON.stringify(runDir)}`,
	`PORTS=${JSON.stringify(ports)}`,
	`PM2_CONFIG=${JSON.stringify(pm2Config)}`,
	`APP_CONFIG=${JSON.stringify(appConfig)}`,
	`GATEWAY_CONFIG=${JSON.stringify(gatewayConfig)}`,
].join("\n"))
EOF
)"

eval "$STARTUP_EXPORTS"

PID_FILE="$RUN_DIR/dev.pid"
LOG_FILE="$RUN_DIR/dev.log"
COMPOSE_BASE="$PROJECT_DIR/infra/docker-compose.yml"
COMPOSE_OVERRIDE="$RUN_DIR/docker-compose.override.yml"

DOCKER_CMD=(docker)
DOCKER_ACCESS_MODE="direct"

if command -v docker >/dev/null 2>&1; then
	if docker info >/dev/null 2>&1; then
		DOCKER_CMD=(docker)
		DOCKER_ACCESS_MODE="direct"
	elif command -v sudo >/dev/null 2>&1 && sudo -n docker info >/dev/null 2>&1; then
		DOCKER_CMD=(sudo docker)
		DOCKER_ACCESS_MODE="sudo"
	else
		DOCKER_ACCESS_MODE="unavailable"
	fi
else
	DOCKER_ACCESS_MODE="missing"
fi

docker_cmd() {
	"${DOCKER_CMD[@]}" "$@"
}
