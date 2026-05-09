# API 速查

> Base URL: `http://localhost:8000`
> 认证方式: `Authorization: Bearer <access_token>`
> 通用响应: `{"code": 200, "message": "ok", "data": {...}}`

这份文档只保留交付副本里最常用的接口。未列出的边缘接口、内部调试接口和细节字段，以服务实现为准。

---

## 认证

### 登录

`POST /api/v1/auth/login/password`

```json
{"email": "alice@example.com", "password": "Secure1234!"}
```

### 刷新 token

`POST /api/v1/auth/token/refresh`

```json
{"refresh_token": "eyJhbGci..."}
```

### 当前用户

`GET /api/v1/auth/me`

### BYOK 管理

`GET /api/v1/auth/api-keys`

`POST /api/v1/auth/api-keys`

`DELETE /api/v1/auth/api-keys/:id`

新增 key 示例：

```json
{
  "provider": "openai",
  "key_alias": "My OpenAI Key",
  "api_key": "sk-..."
}
```

---

## 项目

### 项目列表

`GET /api/v1/projects`

常用查询参数：`keyword`、`status`、`page`、`page_size`

### 创建项目

`POST /api/v1/projects`

```json
{"title": "穿越异世界", "description": "一个关于...的故事"}
```

### 项目详情

`GET /api/v1/projects/:id`

### 更新项目

`PUT /api/v1/projects/:id`

### 删除项目

`DELETE /api/v1/projects/:id`

---

## 剧本

### 上传剧本

`POST /api/v1/scripts/upload`

`multipart/form-data` 字段：

| 字段 | 说明 |
|------|------|
| `file` | 剧本文件 |
| `project_id` | 关联项目 ID |
| `title` | 剧本标题 |

### 查询剧本

`GET /api/v1/scripts/:id`

### 触发分析

`POST /api/v1/scripts/:id/analyze`

可选请求体示例：

```json
{
  "split_keywords": ["第1集", "第2集"],
  "character_keywords": ["主角", "反派"]
}
```

---

## 图像生成

### 提交任务

`POST /api/v1/images/generate`

```json
{
  "project_id": 1,
  "episode_id": 1,
  "scene_id": 12,
  "prompt": "夜晚，雨中的东京街道，霓虹灯反射，动漫风",
  "style_preset": "anime",
  "model_name": "sdxl"
}
```

### 查询任务

`GET /api/v1/images/tasks/:id`

---

## 视频生成

### 提交任务

`POST /api/v1/videos/generate`

```json
{
  "project_id": 1,
  "episode_id": 1,
  "image_urls": [
    "https://cdn.../scene1.jpg",
    "https://cdn.../scene2.jpg"
  ],
  "style_preset": "anime",
  "motion_mode": "gentle",
  "model_name": "kling"
}
```

### 查询任务

`GET /api/v1/videos/tasks/:id`

### 合成成片

`POST /api/v1/videos/tasks/:id/compose`

### 下载成片

`GET /api/v1/videos/:id/download`

### 配音与字幕

常用接口：

`POST /api/v1/projects/:pid/dubbing/generate-batch`

`GET /api/v1/projects/:pid/dubbing/tasks`

`POST /api/v1/projects/:pid/subtitle/generate`

---

## 任务与进度

### 创建异步任务

`POST /api/v1/tasks`

```json
{
  "task_type": "script_analyze",
  "payload": {"script_id": 1},
  "priority": 0
}
```

### 查询任务

`GET /api/v1/tasks/:id`

### 取消任务

`POST /api/v1/tasks/:id/cancel`

---

## WebSocket

### 单任务进度

`WS /ws/tasks/:task_id`

### 项目级进度

`WS /ws/projects/:project_id`

消息格式示例：

```json
{
  "type": "progress",
  "task_id": 100,
  "progress": 75,
  "message": "生成中 75%",
  "status": "running"
}
```

---

## 常见错误码

| 错误码 | 含义 | 处理建议 |
|--------|------|--------|
| 4001 | Token 无效或过期 | 调用 `/auth/token/refresh` |
| 4002 | Token 已吊销 | 重新登录 |
| 4003 | 权限不足 | 检查当前账号 |
| 4004 | 资源不存在 | 检查 ID |
| 4220 | 参数错误 | 检查请求体或 query |
| 4290 | 操作冲突 | 等待当前任务完成 |
| 5000 | 内部服务错误 | 查看服务日志 |
| 5001 | 第三方 AI 服务错误 | 检查 API Key 和额度 |
| 5002 | 存储服务错误 | 检查 MinIO 与 CDN 配置 |
