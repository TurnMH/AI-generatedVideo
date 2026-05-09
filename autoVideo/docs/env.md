# 环境变量配置

> 提交到仓库的是模板文件 `config.yaml`；实际运行使用私有文件 `config.local.yaml`。  
> 网关同理：模板是 `services/gateway-service/config.yaml`，运行文件是 `services/gateway-service/config.local.yaml`。

> 每个服务通过 `config.local.yaml` 或环境变量配置。  
> 环境变量优先级高于配置文件，格式为 `{SERVICE_NAME_UPPER}_{KEY_PATH_UPPER}`。

> 当前运行期密钥约定：本地 `config.local.yaml` 是初始化 / 更新来源；`auth-service` 会把其中的系统运行 key 同步到 `system_api_keys`；业务服务运行时统一从 `auth-service` / 数据库读取活跃 key。

> 如果走 `infra/docker-compose.full.yml`，需要确保 `autoVideo/config.local.yaml` 已存在；compose 会把它挂载到 `auth-service` 容器，并通过 `AUTOVIDEO_CONFIG_FILE=/app/config.local.yaml` 让首启同步生效。

---

## 基础设施（所有服务共用）

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `POSTGRES_HOST` | `localhost` | PostgreSQL 主机 |
| `POSTGRES_PORT` | `5432` | PostgreSQL 端口 |
| `POSTGRES_USER` | `postgres` | 数据库用户 |
| `POSTGRES_PASSWORD` | `postgres` | 数据库密码 |
| `REDIS_ADDR` | `localhost:6379` | Redis 地址 |
| `KAFKA_BROKERS` | `localhost:9092` | Kafka broker 地址（逗号分隔）|
| `MINIO_ENDPOINT` | `localhost:9000` | MinIO 端点 |
| `MINIO_ACCESS_KEY` | `minioadmin` | MinIO Access Key |
| `MINIO_SECRET_KEY` | `minioadmin` | MinIO Secret Key |
| `CDN_BASE_URL` | `http://localhost:9000` | 文件 CDN 前缀 |

---

## gateway-service (port 8000)

统一路由、集中 JWT 鉴权、CORS、WebSocket 代理。

```yaml
# services/gateway-service/config.local.yaml
port: 8000

jwt:
  secret: "replace-with-shared-jwt-secret"   # 与 auth-service 保持一致

cors:
  allowed_origins:
    - "http://localhost:3000"
    - "http://localhost:8000"
    - "http://127.0.0.1:3000"

upstreams:
  auth:      "http://localhost:8001"
  project:   "http://localhost:8002"
  script:    "http://localhost:8003"
  character: "http://localhost:8004"
  image:     "http://localhost:8005"
  video:     "http://localhost:8006"
  task:      "http://localhost:8007"
  model:     "http://localhost:8008"
  storage:   "http://localhost:8009"
  minio:     "http://localhost:9000"

# 完整路由表见 services/gateway-service/config.yaml 模板（路由从上到下首次匹配）
routes:
  - prefix: "/api/v1/auth/"
    upstream: auth
    public: true
  # ...（其余路由见模板文件）
```

| 环境变量 | 说明 |
|----------|------|
| `GATEWAY_JWT_SECRET` | JWT 验证密钥，需与 auth-service 一致 |
| `GATEWAY_PORT` | 监听端口（默认 8000）|

**服务发现端点（内部专用，生产环境需防火墙保护）：**

| 端点 | 方法 | 说明 |
|------|------|------|
| `/_internal/register` | `POST` | 微服务注册/心跳（body: `{"name":"auth","addr":"http://host:port"}`）|
| `/_internal/services` | `GET` | 查看当前已注册的服务列表（含存活状态）|

**本地启动：**
```bash
# 方式一：直接 go run（开发调试）
make gateway

# 方式二：编译后运行（更快）
make gateway-run

# 方式三：随 goreman 启动（goreman start）
# Procfile 中已包含 gateway 条目
```

如果走 Docker Compose 全量部署，gateway 容器会挂载 `services/gateway-service/config.local.yaml`。

---

## auth-service (port 8001)

```yaml
# config.local.yaml
http:
  port: 8001

db:
  dsn: "host=localhost user=postgres password=postgres dbname=auth_db sslmode=disable"

redis:
  addr: "localhost:6379"
  password: ""
  db: 0

jwt:
  access_secret: "your-access-secret-min-32-chars"
  refresh_secret: "your-refresh-secret-min-32-chars"
  access_ttl: 15          # 分钟
  refresh_ttl: 7          # 天

encryption_key: "32-byte-hex-key-for-aes-256-gcm"   # 用于 BYOK Key 加密
```

| 环境变量 | 说明 |
|----------|------|
| `AUTH_JWT_ACCESS_SECRET` | JWT access token 签名密钥（≥32字符）|
| `AUTH_JWT_REFRESH_SECRET` | JWT refresh token 签名密钥（≥32字符）|
| `AUTH_ENCRYPTION_KEY` | BYOK Key 加密密钥（32字节 hex）|
| `GATEWAY_ADDR` | Gateway 地址，用于服务注册（默认 `http://localhost:8000`）|
| `GATEWAY_SELF_ADDR` | 本服务自身地址（默认 `http://localhost:8001`）|

---

## script-service (port 8003)

```yaml
http:
  port: 8003

db:
  dsn: "host=localhost user=postgres password=postgres dbname=script_db sslmode=disable"

kafka:
  brokers: ["localhost:9092"]
  producer_topic: "script.analyze.result"
  consumer_topic: "script.analyze.request"

jwt:
  secret: "your-jwt-secret"     # 与 auth-service 保持一致

llm:
  provider: "openai"             # openai / claude / qwen / zhipu
  openai:
    base_url: "https://poloai.top/v1"
    api_key: "sk-..."
    model: "gpt-5.4"
  claude:
    base_url: "https://api.anthropic.com"
    api_key: "sk-ant-..."
    model: "claude-sonnet-4-6"
  zhipu:
    base_url: "https://open.bigmodel.cn/api/paas/v4"
    api_key: "..."
    model: "glm-5"

storage:
  base_url: "http://localhost:8009"   # storage-service 地址
```

| 环境变量 | 说明 |
|----------|------|
| `SCRIPT_LLM_OPENAI_API_KEY` | OpenAI 兼容 API Key |
| `SCRIPT_LLM_OPENAI_MODEL` | 模型名（当前默认 `gpt-5.4`）|
| `SCRIPT_LLM_PROVIDER` | LLM 提供商：openai / claude / qwen / zhipu |
| `SCRIPT_GATEWAY_ADDR` | Gateway 地址，用于服务注册（默认 `http://localhost:8000`）|
| `SCRIPT_GATEWAY_SELF_ADDR` | 本服务自身地址（默认 `http://localhost:8003`）|

---

## character-service (port 8004)

```yaml
http:
  port: 8004

grpc_port: 9004

db:
  dsn: "host=localhost user=postgres password=postgres dbname=character_db sslmode=disable"

jwt:
  secret: "your-jwt-secret"

storage:
  base_url: "http://localhost:8008"
```

| 环境变量 | 说明 |
|----------|------|
| `CHARACTER_GATEWAY_ADDR` | Gateway 地址，用于服务注册（默认 `http://localhost:8000`）|
| `CHARACTER_GATEWAY_SELF_ADDR` | 本服务自身地址（默认 `http://localhost:8004`）|

---

## image-service (port 8005)

```yaml
http:
  port: 8005

db:
  dsn: "host=localhost user=postgres password=postgres dbname=image_db sslmode=disable"

kafka:
  brokers: ["localhost:9092"]
  consumer_group: "image-service"
  consumer_topic: "image.generate.request"
  producer_topic: "image.generate.result"

jwt:
  secret: "your-jwt-secret"

storage:
  base_url: "http://localhost:8008"

models:
  comfyui_url: "http://localhost:8188"    # ComfyUI 本地服务
  comfyui_urls: ["http://localhost:8188", "http://localhost:8189"] # 多节点本地 ComfyUI
  comfyui_workflow: "sdxl_anime.json"     # 可配置 ComfyUI 工作流模板
  replicate_key: "r8_..."                 # Replicate API Key（Flux）
  openai_key: "sk-..."                    # OpenAI 兼容 API Key（gpt-image-1.5 / DALL-E）
  openai_base: "https://api.ppchat.vip/v1"
  tongyi_key: "sk-..."                    # 阿里云 DashScope Key
  dashscope_base: "https://dashscope.aliyuncs.com/api/v1"
```

| 环境变量 | 说明 |
|----------|------|
| `IMAGE_MODELS_REPLICATE_KEY` | Replicate API Key（Flux 图像生成）|
| `IMAGE_MODELS_OPENAI_KEY` | OpenAI 兼容 API Key（gpt-image-1.5 / DALL-E）|
| `IMAGE_MODELS_TONGYI_KEY` | 阿里云 DashScope Key（通义万象）|
| `IMAGE_MODELS_COMFYUI_URL` | ComfyUI 服务地址（SDXL 本地部署）|
| `IMAGE_MODELS_COMFYUI_URLS` | 多个 ComfyUI 地址，逗号分隔后可轮询扩容|
| `IMAGE_MODELS_COMFYUI_WORKFLOW` | ComfyUI 工作流模板路径或原始 JSON 字符串|
| `GATEWAY_ADDR` | Gateway 地址，用于服务注册（默认 `http://localhost:8000`）|
| `GATEWAY_SELF_ADDR` | 本服务自身地址（默认 `http://localhost:8005`）|

---

## video-service (port 8006)

```yaml
http:
  port: 8006

db:
  dsn: "host=localhost user=postgres password=postgres dbname=video_db sslmode=disable"

kafka:
  brokers: ["localhost:9092"]
  consumer_group: "video-service"
  consumer_topic: "video.generate.request"
  producer_topic: "video.generate.result"

jwt:
  secret: "your-jwt-secret"

storage:
  base_url: "http://localhost:8008"

models:
  kling_key: "..."                        # Kling API Key（Tencent VOD 渠道）
  kling_secret: ""                        # Tencent Cloud SecretKey（TC3-HMAC 签名用）
  kling_base: "https://vod.tencentcloudapi.com"
  wan_key: "..."                          # 阿里云 DashScope Key（Wan2.1）
  wan_base: "https://dashscope.aliyuncs.com"
  sora2_key: "..."                        # Sora-2 代理 Key
  sora2_base: "http://..."
  hubagi_key: "..."                       # HuBagi API Key（TC-GV / Veo3.1）
  hubagi_base: "https://hubagi.cn/api/v1"
  veo_key: "..."                          # HuBagi Veo3.1 Key
  veo_base: "https://www.hubagi.cn/api/v1"
  doubao_key: "..."                       # 字节 Doubao V4.0 Key
  doubao_base: "https://ark.cn-beijing.volces.com/api/v3/contents/generations/tasks"
  vidu_key: "..."                         # Vidu 官方 Key（正常）
  vidu_offpeak_key: "..."                 # Vidu 官方 Key（闲时优先）
  vidu_base: "https://api.vidu.cn/ent/v2"
  suanneng_key: "..."                     # Suanneng Seedance-1.5 Key
  suanneng_base: "https://www.sophnet.com/api/open-apis/..."
  gaga_key: "..."                         # Gaga Art Key
  gaga_base: "https://api.gaga.art/v1/generations"
  replicate_key: "r8_..."                 # Replicate Key（CogVideoX）

ffmpeg:
  temp_dir: "/tmp/autovideo"
  bin: "ffmpeg"                           # ffmpeg 可执行文件路径
```

| 环境变量 | 说明 |
|----------|------|
| `VIDEO_MODELS_KLING_KEY` | Kling API Key（Tencent VOD 渠道）|
| `VIDEO_MODELS_KLING_SECRET` | Tencent Cloud SecretKey（TC3-HMAC 签名用）|
| `VIDEO_MODELS_WAN_KEY` | 阿里云 DashScope Key（Wan2.1）|
| `VIDEO_MODELS_SORA2_KEY` | Sora-2 代理 API Key |
| `VIDEO_MODELS_HUBAGI_KEY` | HuBagi API Key（TC-GV）|
| `VIDEO_MODELS_VEO_KEY` | HuBagi Veo3.1 API Key |
| `VIDEO_MODELS_DOUBAO_KEY` | 字节火山引擎 Doubao V4.0 Key |
| `VIDEO_MODELS_VIDU_KEY` | Vidu 官方 API Key |
| `VIDEO_MODELS_VIDU_OFFPEAK_KEY` | Vidu 闲时 API Key |
| `VIDEO_MODELS_SUANNENG_KEY` | Suanneng Seedance Key |
| `VIDEO_MODELS_GAGA_KEY` | Gaga Art API Key |
| `VIDEO_MODELS_REPLICATE_KEY` | Replicate API Key（CogVideoX）|
| `VIDEO_FFMPEG_TEMP_DIR` | FFmpeg 临时文件目录（默认 /tmp/autovideo）|
| `VIDEO_SERVICE_GATEWAY_ADDR` | Gateway 地址，用于服务注册（默认 `http://localhost:8000`）|
| `VIDEO_SERVICE_GATEWAY_SELF_ADDR` | 本服务自身地址（默认 `http://localhost:8006`）|

---

---

## frontend (port 3000)

> 提交到仓库的是模板文件 `frontend/.env.local.example`；实际本地运行使用 `frontend/.env.local`。  
> `scripts/dev.sh` 会在缺失时自动从模板生成本地文件。

```bash
# frontend/.env.local
NEXT_PUBLIC_API_URL=/                        # 默认走当前访问域名
NEXT_PUBLIC_WS_URL=                          # 留空时自动跟随当前访问域名（ws/wss）
API_PROXY_TARGET=http://localhost:8000      # 当前域名下的 /api 与 /ws 由前端转发到 gateway
```

生产环境下使用 Docker Compose 构建 frontend 镜像时，建议保持：

```bash
# infra/.env
NEXT_PUBLIC_API_URL=/
NEXT_PUBLIC_WS_URL=
API_PROXY_TARGET=http://gateway:8000        # frontend 容器通过 compose 内部网络访问 gateway
```

`scripts/build.sh` 在未显式导出这些变量时，会优先读取 `infra/.env.<env>`，其次读取 `infra/.env`，再回退到以上默认值。

---

## 生产环境建议

```bash
# 所有 JWT Secret 应使用随机生成的强密钥
openssl rand -hex 32

# 加密密钥（32字节）
openssl rand -hex 16

# 示例生产配置（通过 K8s Secret 或 Vault 注入）
AUTH_JWT_ACCESS_SECRET=$(openssl rand -hex 32)
AUTH_JWT_REFRESH_SECRET=$(openssl rand -hex 32)
AUTH_ENCRYPTION_KEY=$(openssl rand -hex 16)
```

---

## 快速获取 API Keys

| 服务 | 申请地址 |
|------|--------|
| OpenAI | https://platform.openai.com/api-keys |
| Claude | https://console.anthropic.com |
| Replicate | https://replicate.com/account/api-tokens |
| 阿里云 DashScope | https://dashscope.console.aliyun.com |
| 快手可灵 | https://klingai.com/developer |
| 火山引擎（Wan2.1）| https://console.volcengine.com |
