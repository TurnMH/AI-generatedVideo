const path = require("path")
const startupConfig = require("./startup.config.json")

const resolvePath = (relativePath) => path.join(__dirname, relativePath)
const sharedConfigPath = resolvePath(startupConfig.sharedConfig)
const gatewayConfigPath = resolvePath(startupConfig.gatewayConfig)

module.exports = {
  apps: startupConfig.apps.map((app) => ({
    ...app,
    args:
      app.name === "gateway-service"
        ? `run ./cmd/main.go -config ${gatewayConfigPath}`
        : app.args,
    cwd: resolvePath(app.cwd),
    script: app.script.includes("/") ? resolvePath(app.script) : app.script,
    env: {
      AUTOVIDEO_CONFIG_FILE: sharedConfigPath,
      AUTOVIDEO_GATEWAY_CONFIG_FILE: gatewayConfigPath,
      ...(app.env || {}),
    },
  })),
}
