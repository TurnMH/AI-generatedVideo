# video 交付副本说明

这份 `video/` 目录是可直接交付、可直接上传服务器的整理副本。目标只有三个：

1. 配置来源清楚。
2. 启动入口清楚。
3. 不再混入开发过程中的日志、草稿和临时文件。

## 当前默认运行方式

默认按这条链运行：

1. `./start.sh` 负责启动基础设施。
2. 业务服务和前端由 PM2 守护。
3. Docker Compose 主要负责 PostgreSQL、Redis、Kafka、MinIO。

因此，当前主配置源不是容器 `.env`，而是本地运行配置文件。

## 关键文件

| 文件 | 用途 |
|------|------|
| `autoVideo/config.local.yaml` | 应用服务主配置，放数据库、JWT、渠道 key 等真实值 |
| `autoVideo/services/gateway-service/config.local.yaml` | gateway 实际运行配置 |
| `autoVideo/frontend/.env.local` | 前端 API / WebSocket 地址 |
| `startup.config.json` | 启动链统一配置，集中定义路径、端口和 PM2 进程 |
| `autoVideo/infra/.env.example` | 全容器部署时的变量模板 |

说明：

1. `config.yaml` 和 `services/gateway-service/config.yaml` 现在都只是模板。
2. `frontend/.env.local` 缺失时，`scripts/dev.sh` 会从模板自动生成。
3. 用户侧 BYOK 仍然存数据库，不在这些启动配置文件里。

## 最短交付流程

### 方案 A：直接上传到服务器运行

这是当前最推荐的路径，也最接近你现在本机的运行方式。

1. 上传整个 `video/` 目录到服务器。
2. 服务器安装 `Docker`、`Node.js`、`npm`、`pm2`。
3. 填写：
   - `autoVideo/config.local.yaml`
   - `autoVideo/services/gateway-service/config.local.yaml`
4. 启动：`./start.sh`
5. 停止：`./stop.sh`

补充说明：

1. 这条路径下，运行期密钥的源头仍是 `autoVideo/config.local.yaml`。
2. `auth-service` 启动时会把这里面的系统运行 key 同步进数据库。
3. 业务服务运行时统一从 `auth-service` / 数据库取活跃 key，不再直接把配置文件当成运行期取钥源。

示例上传命令：

```bash
scp -r video user@server:/opt/
# 或
rsync -av video/ user@server:/opt/video/
```

### 方案 B：全容器部署

仅在你准备走完整镜像和 `docker compose.full.yml` 时使用。

1. 复制 `autoVideo/config.yaml` 为 `autoVideo/config.local.yaml`，填入真实数据库、JWT 和运行期 key。
2. 复制 `autoVideo/services/gateway-service/config.yaml` 为 `autoVideo/services/gateway-service/config.local.yaml`。
3. 复制 `autoVideo/infra/.env.example` 为 `autoVideo/infra/.env`，填入容器环境变量。
4. 构建镜像：`cd autoVideo && bash scripts/build.sh --env=prod --tag=latest`
5. 部署：`cd autoVideo && bash scripts/deploy.sh --env=prod --tag=latest --skip-pull`

这条路径下，frontend 构建参数和运行参数优先读取 `infra/.env.prod` 或 `infra/.env`。

补充说明：

1. `docker-compose.full.yml` 现在会把 `autoVideo/config.local.yaml` 挂载到 `auth-service` 容器内。
2. 容器首启时，`auth-service` 会先用这个文件把运行期 key 同步到数据库。
3. 如果你后续修改了 `config.local.yaml` 并希望业务立即使用新 key，建议重启 `auth`、`project`、`script`、`character`、`image`、`video` 这几个服务。

## 本地启动与访问

本地直接启动：

```bash
./start.sh
```

停止：

```bash
./stop.sh
```

常用入口：

| 服务 | 地址 |
|------|------|
| 前端 | http://localhost:3000 |
| Gateway | http://localhost:8000 |
| MinIO 控制台 | http://localhost:9001 |

## 交付副本内保留的文档

| 文件 | 作用 |
|------|------|
| `autoVideo/README.md` | 项目最小运行说明 |
| `autoVideo/docs/env.md` | 配置项与模板说明 |
| `autoVideo/docs/development.md` | 运行与联调说明 |
| `autoVideo/docs/api.md` | 常用接口速查 |

## 一句话结论

如果只记一件事：这份交付副本里，应用服务看 `autoVideo/config.local.yaml`，网关看 `services/gateway-service/config.local.yaml`，基础设施由 Docker 起，服务进程由 PM2 守护。