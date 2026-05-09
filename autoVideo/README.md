# autoVideo — AI 剧本生成漫画/图片/视频平台

> **一句话描述**：输入剧本，全自动输出分集漫画分镜 + 连贯视频。

[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](https://go.dev)
[![Next.js](https://img.shields.io/badge/Next.js-14-black?logo=next.js)](https://nextjs.org)
[![License](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

---

## 目录

- [功能概览](#功能概览)
- [架构图](#架构图)
- [快速开始](#快速开始)
- [服务一览](#服务一览)
- [环境变量](docs/env.md)
- [开发指南](docs/development.md)
- [API 文档](docs/api.md)

补充说明：

- [交付副本说明](../README.md)

---

## 功能概览

| 功能 | 说明 |
|------|------|
| 📝 剧本上传 | 支持 txt/docx/pdf，自动解析 |
| 🤖 LLM 分析 | GPT-4o / Claude / Qwen / GLM-5 自动分集、提取场景/角色/情感 |
| 🎨 图像生成 | SDXL / Flux / GPT-Image / 通义万象，多模型路由 |
| 🎬 视频合成 | Kling / Wan2.1 / CogVideoX / Vidu / Doubao / Sora2 图生视频 + FFmpeg 拼接 |
| 🎙️ AI 配音 | TTS 多角色配音（按剧本标注自动分配声线）+ Whisper 字幕生成 |
| 🔄 全自动流水线 | 一键 → 8 阶段自动成片，断点续传 |
| 👤 角色一致性 | 风格预置 + IP-Adapter + 固定 seed |
| ⚡ 实时进度 | WebSocket 推送，任务队列可视化 |
| 🔑 BYOK | 用户自带 API Key，AES-256-GCM 加密存储 |

---

## 架构图

```
┌─────────────────────────────────────────────┐
│           前端工作台 (Next.js 14)             │
│  项目管理 / 分镜看板 / 生成中心 / 视频工作台   │
└──────────────────┬──────────────────────────┘
                   │ HTTP / WebSocket
┌──────────────────▼──────────────────────────┐
│       API Gateway — gateway-service          │
│     Go 实现，本地运行 (port 8000)             │
│   统一路由 / 集中 JWT 鉴权 / CORS / WS 代理   │
└──┬──────┬──────┬──────┬──────┬──────┬───────┘
   │      │      │      │      │      │
 auth  project script char  image  video  ...
 8001   8002   8003  8004  8005   8006

         ↕ Kafka 异步消息
┌─────────────────────────────────────────────┐
│   task-service (8007) — 任务状态机 + WS推送   │
└─────────────────────────────────────────────┘
         ↕ 存储
  PostgreSQL · Redis · MinIO · Kafka
```

---

## 快速开始

### 前置要求

| 工具 | 版本 |
|------|------|
| Docker + Docker Compose | 24+ |
| Go | 1.22+ |
| Node.js | 20+ |
| Make | 任意版本 |

### 1. 配置文件

实际运行时请填写这些本地文件：

1. `config.local.yaml`
2. `services/gateway-service/config.local.yaml`
3. `frontend/.env.local`（没有时可从 `frontend/.env.local.example` 复制，`scripts/dev.sh` 也会自动生成）

模板与字段说明见 [docs/env.md](docs/env.md)。

### 2. 启动

交付副本推荐从上层 `video/` 目录统一启动：

```bash
cd ..
./start.sh
```

如果只在当前目录联调，也可以直接执行：

```bash
bash scripts/dev.sh
```

### 3. 停止

```bash
cd ..
./stop.sh
```

### 4. 访问平台

| 服务 | 地址 | 说明 |
|------|------|------|
| 前端工作台 | http://localhost:3000 | 主界面 |
| Gateway | http://localhost:8000 | API 与 WebSocket 入口 |
| MinIO 控制台 | http://localhost:9001 | 对象存储（admin/minioadmin） |

---

## 服务一览

| 服务 | 端口 | 数据库 | 说明 |
|------|------|--------|------|
| gateway-service | 8000 | — | Go API Gateway，统一路由 / 集中 JWT 鉴权 / CORS / WS 代理 |
| auth-service | 8001 | auth_db | JWT登录 / OAuth2 / RBAC / BYOK |
| project-service | 8002 | project_db | 剧本项目 / 分集 / 版本快照 |
| script-service | 8003 | script_db | 剧本解析 / LLM分析 / 场景提取 |
| character-service | 8004 | character_db | 角色资产 / 风格预置 / 参考图 |
| image-service | 8005 | image_db | 多模型图像生成 / 批量并发 |
| video-service | 8006 | video_db | 图生视频 / FFmpeg合成 / AI配音 / 字幕 |
| task-service | 8007 | task_db | 异步任务队列 / WebSocket推送 |
| model-service | 8008 | model_db | 模型注册 / 路由 / 用量计费 |
| storage-service | 8009 | — | MinIO对象存储代理 / CDN签发 |
| whisper-sidecar | — | — | Whisper 字幕识别边车服务（Python） |

---

---

## License

MIT © autoVideo Team
