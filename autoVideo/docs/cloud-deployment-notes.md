# 云上迁移复盘与部署避坑指南

## 目的

这份文档用于记录本次 autoVideo 迁移到云服务器时遇到的典型问题、已经落地的修复改动，以及后续部署时必须执行的检查步骤，避免重复踩坑。

适用场景：

- 从本地开发环境迁移到单机 Docker Compose 云服务器
- 服务镜像已经构建，但容器内行为与本地源码不一致
- 多个 Go 微服务共用一份配置文件，并通过容器挂载覆盖默认配置

---

## 本次迁移涉及的关键改动

### 1. 为容器部署引入共享配置文件

新增共享配置文件：

- `config.local.yaml`
- `services/gateway-service/config.local.yaml`

目的：

- 把服务默认配置中的 `localhost` 统一替换为容器网络内可互通的服务名
- 避免每个服务单独维护一份生产配置
- 兼容旧镜像仍然依赖本地配置文件路径的问题

当前共享配置至少覆盖了这些关键地址：

- PostgreSQL: `postgres:5432`
- Redis: `redis:6379`
- Kafka: `kafka:9092`
- MinIO: `minio:9000`
- Gateway: `http://gateway:8000`
- Auth: `http://auth:8001`
- Project: `http://project:8002`
- Storage: `http://storage:8009`

### 2. 调整全量部署 Compose

主要修改文件：

- `infra/docker-compose.full.yml`

本次落地的关键变更：

- `x-go-service` 统一挂载 `../config.local.yaml:/app/config.yaml:ro`
- `project` 额外挂载 `../config.local.yaml:/etc/project-service/config.yaml:ro`
- `model` 额外挂载 `../config.local.yaml:/etc/model-service/config.yaml:ro`
- `gateway` 同时挂载：
  - `../services/gateway-service/config.local.yaml:/app/config.yaml:ro`
  - `../services/gateway-service/config.local.yaml:/app/config.local.yaml:ro`
- `auth` 健康检查从 `/healthz` 改为 `/health`
- `frontend` 端口从 `3000:3000` 改为 `3001:3000`
- Kafka / ZooKeeper 镜像切换到更稳定可拉取的镜像源

### 3. 调整 project-service 构建方式

主要修改文件：

- `services/project-service/Dockerfile`

关键变更：

- builder 基础镜像升级为 `golang:1.23-alpine`
- 显式设置：
  - `GOPROXY=https://goproxy.cn,direct`
  - `GOSUMDB=sum.golang.google.cn`

原因：

- 远端服务器构建时，`go mod download` 不能稳定依赖宿主机环境变量
- 仅在宿主机设置 Go 代理，不足以保证 Docker build 阶段成功下载依赖

### 4. 明确区分“空库初始化”和“业务迁移初始化”

这次模型列表为空，根因不是 PostgreSQL 没启动，也不是 `model_db` 没创建，而是把两层初始化混在了一起：

- `infra/init-db.sql` 只负责创建空数据库，例如 `auth_db`、`project_db`、`model_db`
- 各服务自己的表结构、索引、初始 seed 数据，来自 `services/*/migrations`
- 以模型服务为例，模型列表数据来自 `services/model-service/migrations`，不是来自 `infra/init-db.sql`

这意味着：

- 看到数据库已经存在，不代表业务表和初始数据已经存在
- `docker compose up` 成功，不代表服务依赖的迁移已经成功执行
- 如果迁移没跑，服务可能健康检查通过，但实际业务数据仍然为空

为避免再次出现“部署成功但数据未初始化”的假象，`scripts/deploy.sh` 已调整为：

- 服务器未安装 `golang-migrate` 时，直接终止部署
- 任一服务迁移执行失败时，直接终止部署
- 只有 `no change` 会被视为“已经是最新版本”并继续执行

---

## 迁移期间遇到的典型问题

下面记录本次实际踩到的坑，按“现象 -> 根因 -> 处理方式”整理。

### 1. 容器能启动，但服务还在访问 localhost

现象：

- `model-service`、`storage-service`、`project-service` 等容器启动后仍然尝试连接 `localhost`
- 日志里出现数据库、Redis、Auth、MinIO 连接拒绝

根因：

- 默认 `config.yaml` 面向本地开发，地址写的是 `localhost`
- 容器内的 `localhost` 是容器自身，不是其他服务
- 服务虽然启动了，但实际并没有读取到容器专用配置

处理方式：

- 新增 `config.local.yaml` 作为容器共享配置
- 在 Compose 中把共享配置挂载到各服务实际会读取的路径
- 对旧镜像额外兼容 `/etc/<service>/config.yaml` 这类搜索路径

结论：

- 云上容器部署不能复用“默认本地配置”
- 只要日志里还出现 `localhost`，优先怀疑配置未生效，而不是服务本身有 bug

### 2. 改了配置文件，但容器仍然使用旧配置

现象：

- 本地文件已修改，但容器日志仍表现为旧行为
- `project-service` 和 `gateway` 一度仍然读取旧地址

根因：

- 旧镜像的配置搜索路径与当前源码预期不同
- 部分服务并不只读取 `/app/config.yaml`
- `gateway` 旧镜像会查找 `config.local.yaml`
- `project-service`、`model-service` 旧镜像还依赖 `/etc/<service>/config.yaml`

处理方式：

- `gateway` 同时挂载 `/app/config.yaml` 和 `/app/config.local.yaml`
- `project` / `model` 同时挂载 `/app/config.yaml` 和 `/etc/<service>/config.yaml`
- 必须用 `docker inspect` 验证 mount 是否真的生效

结论：

- 不要假设“挂了一个配置路径就一定被读取”
- 对老镜像，必须确认：工作目录、配置搜索路径、最终 mount 点

### 3. auth 一直不 healthy

现象：

- Compose 卡在依赖等待
- `auth` 服务实际已经启动，但状态不是 healthy

根因：

- 健康检查访问了错误路径 `/healthz`
- 实际可用接口是 `/health`

处理方式：

- 将 `infra/docker-compose.full.yml` 中 `auth` healthcheck 改为：
  `wget -qO- http://localhost:8001/health`

结论：

- 依赖链异常时，先验证 healthcheck 地址是否和真实接口一致
- 服务“运行中”不代表 Compose 会把它判定为 healthy

### 4. frontend 无法启动或端口冲突

现象：

- 前端容器起不来
- 宿主机 `3000` 端口已经被其他服务占用

根因：

- 服务器上已有 FRP 或其他常驻进程占用 `3000`

处理方式：

- 将前端改为宿主机 `3001:3000`

结论：

- 云服务器上不要默认假设 `3000` 一定空闲
- 部署前先检查宿主机端口占用

### 5. MinIO / Kafka / ZooKeeper 镜像拉取慢或失败

现象：

- 镜像拉取缓慢
- 某些官方或第三方镜像源返回 403 或超时

根因：

- 服务器网络环境对部分镜像源不稳定
- 过度依赖单一镜像仓库

处理方式：

- 使用更稳定的镜像地址
- Kafka / ZooKeeper 改为可用镜像组合
- 预先准备 Docker 镜像加速源

结论：

- 云上第一次部署，镜像源往往比代码本身更容易成为阻塞点
- 基础设施镜像要优先验证可拉取性

### 6. project-service 远端源码看起来是新的，但容器跑的还是旧逻辑

现象：

- 远端文件已经更新
- `project-service` 容器日志却仍出现旧逻辑，例如加载运行时 API Key 的行为

根因：

- 远端源码与本地源码一度漂移
- 镜像没有基于最新源码重新构建
- 旧镜像缓存掩盖了真实问题

处理方式：

- 用 `rsync` 明确同步 `services/project-service/` 到远端
- 立刻回读远端 `cmd/main.go`，确认旧逻辑确实消失
- 执行 `docker build --no-cache -t autovideo/project:latest .`
- 再执行 `docker compose ... up -d --force-recreate project`

结论：

- “服务器上的文件看起来对了” 不等于 “运行中的容器已经是新产物”
- 代码路径变更后，必须做源码同步、无缓存构建、强制重建三连验证

### 7. project-service 构建卡在 go mod download

现象：

- `docker build` 长时间停留在 `RUN go mod download`

根因：

- Docker build 阶段未继承稳定的 Go 代理配置
- 宿主机环境变量不足以保证容器内依赖下载成功

处理方式：

- 在 `services/project-service/Dockerfile` 内显式设置：
  - `GOPROXY=https://goproxy.cn,direct`
  - `GOSUMDB=sum.golang.google.cn`

结论：

- 对远端构建不稳定的 Go 服务，不要只配宿主机代理
- 需要把关键代理配置写进 Dockerfile，保证构建可重复

### 8. 第一次请求 healthz 失败，第二次成功

现象：

- 容器刚启动时，请求 `/healthz` 出现 `connection reset by peer`
- 随后再次请求恢复正常并返回 200

根因：

- 服务进程已启动，但 HTTP 监听或依赖初始化仍在完成中

处理方式：

- 重建后不要只看第一次探测结果
- 至少重试一次，并结合容器日志判断是否是短暂启动波动

结论：

- 刚启动阶段的瞬时失败不一定代表真正异常
- 但如果连续重试仍失败，就应回到日志与配置排查

### 9. 模型迁移后列表为空，或线上模型与本地不一致

现象：

- 前端模型列表为空，或者线上可见模型明显少于本地
- 容器和数据库都在运行，但模型选择器没有可用项

根因：

- `infra/init-db.sql` 只负责创建空数据库，不负责导入模型目录数据
- `model-service` 的 migrations 或 seed 没有执行，或者线上没有同步本地已整理过的模型数据
- “模型目录可展示” 和 “模型实际可调用” 是两回事，不能只看数据库里有没有行

处理方式：

- 先执行 `services/model-service/migrations`，确认模型表和基础 seed 已落地
- 如果线上需要与本地保持同一批模型目录，额外同步本地模型数据或导出的 SQL，而不是只重启服务
- 迁移完成后，至少检查一次模型总数、默认模型、图片模型数、视频模型数，而不是只打开前端页面肉眼判断

结论：

- “数据库已创建” 不等于 “模型目录已迁移完成”
- 下次迁移模型时，必须把“库结构迁移”和“模型数据迁移”拆成两个检查项

### 10. 图片生成全部失败

现象：

- 页面能正常触发图片任务，但任务持续失败
- `image-service` 日志出现类似 `dalle: no api key configured` 的报错
- 部分请求未显式传模型名时，线上会落到本地默认链路，但服务器并没有对应的本地生成环境

根因：

- 线上缺失 `image-service.models.*` 配置，导致远端 provider 没有注册成功
- 图片链路里“空模型”“未知模型”“项目默认模型”并不总是落到同一个 provider，不能靠猜测判断最终调用的是谁
- 线上没有 `comfyui` 时，本地 `sdxl` 默认链路不可用

处理方式：

- 在共享配置里显式补齐 `image-service.models` 下的 key、base、model 等配置
- 重启 `image-service` 后，立即校验 provider 注册结果和 `model-status`，确认远端模型真正可用
- 生产环境不要依赖“空模型自动兜底”，而要让业务入口传明确模型，或者至少明确一个线上可用的默认模型
- 每次迁移后，都要实际发起一张测试图，确认从任务创建到资源回填全链路成功

结论：

- 图片生成问题大多不是前端按钮问题，而是运行时 provider 配置没有落地
- 下次迁移图片能力时，必须同时检查“配置已加载”“provider 已注册”“真实生成成功”三件事

### 11. 模型聊天报错，但模型列表看起来正常

现象：

- 前端能看到聊天模型，但实际对话报错，或者多个模型轮流失败
- 同一套模型目录里，有的模型能显示却不能真正聊天

根因：

- 模型目录数据和聊天运行时配置不是同一条链路，`model_db` 有记录不代表聊天服务已有可用的 `base_url`、API Key 和默认模型
- `script-service`、`auth-service` 的运行时 key 同步依赖共享配置；如果 provider、base URL、key 或 model 没同步到线上，聊天仍会失败
- 部分模型当前只是“可展示目录”或“配置占位”，并不代表下游服务已经按 `model_key` 全链路接入

处理方式：

- 迁移聊天能力时，分开验证三层：模型目录、运行时配置、真实对话请求
- 检查 `script-service.llm.*` 和运行时 key 同步是否完整，尤其是 provider、base URL、API Key、默认 model
- 不要把“模型显示在下拉框里”当作验收标准，必须实际发一条聊天请求，看返回是否成功
- 对仅做目录展示的模型，要明确标记为不可直接用于生产聊天，避免前端把它们当成可用模型暴露出去

结论：

- “模型可见” 不等于 “模型可聊”
- 下次迁移聊天能力时，必须把目录迁移、密钥迁移、聊天实测拆开验收

### 12. Nginx 指向错误，导致命中错页或详情页打不开

现象：

- 打开 `/projects/<id>` 进入的是项目列表页，而不是详情页
- 打开 `/video-serial/<id>` 看到的是视频列表页，或者命中了错误的静态页面
- `/projects/new`、`/video-serial/new` 这类固定路由在上线后异常

根因：

- 前端采用静态导出后，动态详情页依赖 `__dynamic__` 壳页和 Nginx rewrite 配合工作
- 如果 Nginx 使用了宽泛的 catch-all 规则把整个目录都重写到 `index.html`，就会把本该命中的详情页、创建页一并打坏
- 只看 HTML 标题不够，错误页面也可能复用相同布局和标题

处理方式：

- 只保留显式的 `__dynamic__` rewrite 规则，不要对 `/projects` 或 `/video-serial` 做泛化兜底
- 为 `/projects/new`、`/video-serial/new`、`/video-serial/quick` 这类固定静态路由保留排除规则
- 验证是否命中正确页面时，不要只看标题，要看实际加载的 chunk 或页面行为
- 每次改 Nginx 后，至少回归四个路径：列表页、创建页、普通详情页、串行详情页

结论：

- 静态导出的动态路由问题，本质上是 Nginx 指向问题，不是前端路由库本身失效
- 下次上线前，必须把 rewrite 规则和真实页面路径逐条对照验证

---

## 已确认有效的部署检查顺序

后续部署建议严格按下面顺序执行，不要跳步。

### 1. 先确认源码与服务器一致

建议：

```bash
rsync -az --delete ./services/project-service/ user@server:/path/to/autoVideo/services/project-service/
ssh user@server "sed -n '1,80p' /path/to/autoVideo/services/project-service/cmd/main.go"
```

目标：

- 确认远端确实拿到了本地最新源码
- 尤其是启动入口、Dockerfile、配置加载代码

### 2. 渲染 Compose，确认最终配置

建议：

```bash
docker compose -f infra/docker-compose.full.yml --env-file infra/.env config
```

重点检查：

- volume 挂载是否存在
- healthcheck 是否是正确地址
- 镜像 tag 是否符合预期
- 环境变量是否被正确注入

### 3. 用 docker inspect 验证容器真实挂载

建议重点看：

- `Mounts`
- `Config.Image`
- `State`
- `WorkingDir`

典型用途：

- 确认 `config.local.yaml` 真的进了容器
- 确认容器正在运行的镜像是不是你刚构建的那个

### 4. 单服务无缓存构建

当出现以下任一情况时，直接使用 `--no-cache`：

- 启动入口改过
- Dockerfile 改过
- 配置加载逻辑改过
- 远端行为明显与源码不一致

示例：

```bash
cd services/project-service
docker build --no-cache -t autovideo/project:latest .
```

### 5. 单服务强制重建，不要一上来全量重启

建议：

```bash
docker compose -f infra/docker-compose.full.yml --env-file infra/.env up -d --force-recreate project
```

原因：

- 单点验证更容易定位问题
- 避免多个服务同时波动，增加噪音

### 6. 先看日志，再看健康检查

建议顺序：

```bash
docker logs --tail 100 autovideo-project
wget -S -O- http://127.0.0.1:8002/healthz
```

原因：

- 日志能直接反映服务读取了哪份配置、访问了哪个地址
- 健康检查只告诉你“好/坏”，不能解释为什么坏

### 7. 最后再执行全量部署脚本

`scripts/deploy.sh` 适合作为收尾步骤，不适合作为第一诊断入口。

原因：

- 脚本会同时处理多个步骤，输出噪音大
- 当某个服务仍有单点问题时，很容易把真正根因淹没掉
- `--skip-pull` 场景下，脚本默认依赖服务器本地已有镜像

建议：

- 先把异常服务单独修好
- 再用 `bash scripts/deploy.sh --env=prod --tag=latest --skip-pull` 做收口部署

---

## 后续部署必须注意的要点

### 1. 本地开发默认配置不能直接拿去跑容器

凡是写着 `localhost` 的配置，都要默认视为“仅限本地开发”。

容器内部应该优先使用：

- `postgres`
- `redis`
- `kafka`
- `minio`
- `auth`
- `project`
- `storage`
- `gateway`

### 2. 不要只改文件，不验证生效路径

必须同时验证三件事：

- `docker compose config` 的渲染结果
- `docker inspect` 的实际挂载结果
- 容器日志里的真实访问地址

只改仓库文件，不足以证明线上行为已经变化。

### 3. 不要把“镜像已重建”和“容器已更新”混为一谈

镜像构建成功后，还要确认：

- 运行中的容器是否已经替换为新镜像
- `docker inspect autovideo-project --format '{{.Image}} {{.Config.Image}}'` 是否符合预期
- 容器日志是否已经消失旧逻辑特征

### 4. 旧镜像兼容问题要留冗余挂载

对于配置加载历史不一致的服务，短期内不要急着删兼容 mount。

当前至少需要保留兼容挂载的服务：

- `project`
- `model`
- `gateway`

### 5. 健康检查地址要和真实接口保持一致

任何服务迁移后第一步都要确认：

- healthcheck 用的路径是不是存在
- 端口是不是正确
- 返回 200 的接口是不是那个路径

### 6. 云上端口使用必须先盘点

尤其要提前检查：

- `3000`
- `8000`
- `8001`
- `8002`
- `9000`
- `9001`

如果宿主机已有 FRP、Nginx、面板或其他业务进程，直接套本地端口映射很容易冲突。

### 7. 构建代理配置要写进 Dockerfile，而不是只配宿主机

特别是 Go 服务：

- 远端网络不稳定时，Docker build 的可重复性高于宿主机手工环境
- 能写进 Dockerfile 的代理配置，优先写进 Dockerfile

### 8. 生产密钥不要继续和部署文档混放

后续建议：

- `infra/.env` 只保存基础设施与部署级变量
- 业务 API Key 拆到专门的私有配置文件或密钥管理系统
- 不要把真实密钥提交到公开仓库

---

## 当前云端常用运维命令

服务器当前项目目录：

```bash
/home/ubuntu/AI-generatedVideo/autoVideo
```

当前全量编排入口：

```bash
docker compose -f infra/docker-compose.full.yml --env-file infra/.env
```

建议先进入项目目录再执行：

```bash
cd /home/ubuntu/AI-generatedVideo/autoVideo
```

### 1. 查看当前怎么运行

查看所有容器状态：

```bash
sudo docker compose -f infra/docker-compose.full.yml --env-file infra/.env ps
```

查看某个容器是否在运行：

```bash
sudo docker ps --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}' | grep autovideo
```

查看 compose 渲染后的最终配置：

```bash
sudo docker compose -f infra/docker-compose.full.yml --env-file infra/.env config
```

### 2. 启动和停止

全量启动：

```bash
sudo docker compose -f infra/docker-compose.full.yml --env-file infra/.env up -d
```

只启动单个服务：

```bash
sudo docker compose -f infra/docker-compose.full.yml --env-file infra/.env up -d project
```

停止全量服务：

```bash
sudo docker compose -f infra/docker-compose.full.yml --env-file infra/.env stop
```

停止单个服务：

```bash
sudo docker compose -f infra/docker-compose.full.yml --env-file infra/.env stop project
```

### 3. 重启

重启全量服务：

```bash
sudo docker compose -f infra/docker-compose.full.yml --env-file infra/.env restart
```

重启单个服务：

```bash
sudo docker compose -f infra/docker-compose.full.yml --env-file infra/.env restart gateway
sudo docker compose -f infra/docker-compose.full.yml --env-file infra/.env restart project
```

如果要强制按最新镜像和最新挂载重新创建容器，不要只用 restart，而要用：

```bash
sudo docker compose -f infra/docker-compose.full.yml --env-file infra/.env up -d --force-recreate project
```

### 4. 查看日志

查看单个服务最近 100 行日志：

```bash
sudo docker logs --tail 100 autovideo-project
sudo docker logs --tail 100 autovideo-gateway
sudo docker logs --tail 100 autovideo-auth
```

持续跟日志：

```bash
sudo docker logs -f autovideo-project
```

查看 compose 聚合日志：

```bash
sudo docker compose -f infra/docker-compose.full.yml --env-file infra/.env logs --tail 100
sudo docker compose -f infra/docker-compose.full.yml --env-file infra/.env logs -f gateway project
```

### 5. 查看健康状态

直接打健康检查：

```bash
wget -S -O- http://127.0.0.1:8000/healthz
wget -S -O- http://127.0.0.1:8001/health
wget -S -O- http://127.0.0.1:8002/healthz
```

查看容器 health 状态：

```bash
sudo docker inspect autovideo-project --format '{{json .State.Health}}'
sudo docker inspect autovideo-auth --format '{{json .State.Health}}'
sudo docker inspect autovideo-gateway --format '{{json .State.Health}}'
```

### 6. 进入容器或看容器详情

进入容器：

```bash
sudo docker exec -it autovideo-project sh
sudo docker exec -it autovideo-gateway sh
```

查看容器真实镜像、挂载、工作目录：

```bash
sudo docker inspect autovideo-project
sudo docker inspect autovideo-gateway
```

只看镜像和运行状态：

```bash
sudo docker inspect autovideo-project --format '{{.Image}} {{.Config.Image}} {{.State.Status}}'
```

### 7. 服务更新后的标准操作

如果只是改了配置文件：

```bash
sudo docker compose -f infra/docker-compose.full.yml --env-file infra/.env up -d --force-recreate gateway project model
```

如果改了某个服务源码：

```bash
cd /home/ubuntu/AI-generatedVideo/autoVideo/services/project-service
sudo docker build --no-cache -t autovideo/project:latest .

cd /home/ubuntu/AI-generatedVideo/autoVideo
sudo docker compose -f infra/docker-compose.full.yml --env-file infra/.env up -d --force-recreate project
```

如果要做收口部署：

```bash
cd /home/ubuntu/AI-generatedVideo/autoVideo
sudo bash scripts/deploy.sh --env=prod --tag=latest --skip-pull
```

但本次迁移过程中，更推荐优先使用“单服务 build + 单服务 recreate”，不要一开始就直接跑全量部署脚本。

---

## 建议保留的排障命令

```bash
# 1. 渲染最终 compose
docker compose -f infra/docker-compose.full.yml --env-file infra/.env config

# 2. 查看容器状态
docker compose -f infra/docker-compose.full.yml --env-file infra/.env ps

# 3. 看单服务最近日志
docker logs --tail 100 autovideo-project

# 4. 查看容器真实镜像与挂载
docker inspect autovideo-project

# 5. 单服务强制重建
docker compose -f infra/docker-compose.full.yml --env-file infra/.env up -d --force-recreate project

# 6. 单服务无缓存构建
cd services/project-service
docker build --no-cache -t autovideo/project:latest .

# 7. 健康检查
wget -S -O- http://127.0.0.1:8002/healthz
wget -S -O- http://127.0.0.1:8000/healthz
wget -S -O- http://127.0.0.1:8001/health
```

---

## 一句话原则

云上迁移时，优先怀疑“配置未生效、镜像未更新、挂载路径不对、健康检查地址错误”，不要第一时间把问题归因到业务代码。
