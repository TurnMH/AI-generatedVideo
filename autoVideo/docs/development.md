# 运行与联调

本页只保留交付副本运行、联调和基础排障所需内容。原来的开发规范、内部结构说明、路线图和历史排障记录已从交付文档中移除。

## 目录
- [环境要求](#环境要求)
- [本地启动](#本地启动)
- [数据库迁移](#数据库迁移)
- [手动联调](#手动联调)
- [常见问题](#常见问题)

---

## 环境要求

| 工具 | 说明 |
|------|------|
| Docker + Docker Compose | 负责 PostgreSQL / Redis / Kafka / MinIO |
| Go 1.22+ | 运行 Go 服务 |
| Node.js 20+ | 运行前端与 PM2 |
| golang-migrate（可选） | 执行数据库迁移 |

常用安装方式：

```bash
brew install go@1.22
brew install node@20
brew install golang-migrate
```

---

## 本地启动

### 推荐方式：从交付副本根目录统一启动

```bash
cd ..
./start.sh
```

这条链会：

1. 检查 Docker 是否就绪。
2. 拉起 PostgreSQL / Redis / Kafka 等基础设施。
3. 用 PM2 启动 gateway、各 Go 服务和前端。

停止命令：

```bash
cd ..
./stop.sh
```

### 启动前需要确认的文件

1. `config.local.yaml`
2. `services/gateway-service/config.local.yaml`
3. `frontend/.env.local`（缺失时 `scripts/dev.sh` 会从模板自动生成）

完整字段说明见 [env.md](env.md)。

### 快速验活

```bash
curl -s -o /dev/null -w '%{http_code}\n' http://localhost:3000
curl -s -o /dev/null -w '%{http_code}\n' http://localhost:8000/healthz
curl -s -o /dev/null -w '%{http_code}\n' http://localhost:8001/health
curl -s -o /dev/null -w '%{http_code}\n' http://localhost:9000/minio/health/live
```

前端返回 `200` 或 `307` 均可视为已在线。

---

## 数据库迁移

如果本机已安装 `golang-migrate`，可以执行：

```bash
make migrate-all
```

若未安装，`scripts/dev.sh` 会跳过自动迁移并给出提示。初始化数据库定义位于 `infra/init-db.sql`。

---

## 手动联调

仅在需要单独调试服务时使用。

### 启动基础设施

```bash
make infra-up
docker compose -f infra/docker-compose.yml ps
```

### 单独启动 gateway-service

```bash
cd services/gateway-service
go run ./cmd/main.go -config config.local.yaml
```

### 单独启动前端

```bash
cd frontend
cp .env.local.example .env.local
npm install
npm run dev
```

---

## 常见问题

### Docker daemon 未启动

```bash
docker info
```

如果失败，先启动 Docker Desktop 或 Docker 服务，再重新执行 `./start.sh`。

### 缺少配置文件

如果启动提示缺少 `config.local.yaml` 或 gateway 配置文件，请从模板复制后填写：

```bash
cp config.yaml config.local.yaml
cp services/gateway-service/config.yaml services/gateway-service/config.local.yaml
```

### gateway-service 无法绑定 8000

```bash
lsof -i:8000 -sTCP:LISTEN
```

确认旧进程退出后再重启。

### PostgreSQL 连接失败

```bash
docker ps | grep autovideo-postgres
psql -h localhost -U postgres -p 5432 postgres
```

### MinIO 上传后 URL 无法访问

本地联调时，确认 `CDN_BASE_URL` 与 MinIO 端口一致，并检查 bucket 是否允许公开读取。

### 前端请求不到后端

默认情况下：

1. `frontend/.env.local` 使用相对路径访问当前域名。
2. 开发模式下 `API_PROXY_TARGET` 默认转发到 `http://localhost:8000`。
3. 容器部署时 `API_PROXY_TARGET` 默认转发到 `http://gateway:8000`。

如果你改过域名或端口，请同步检查 `frontend/.env.local` 或 `infra/.env`。
