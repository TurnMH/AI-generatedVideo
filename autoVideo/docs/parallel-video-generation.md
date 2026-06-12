# 并行视频生成流程

本文梳理 autoVideo 项目中**非口播广告**场景的并行视频生成链路，覆盖前端提交、Kafka 调度、clip 并发、合成上传各阶段。

## 1. 总览

```
前端 VideoTab / StoryboardTab
  → POST /api/v1/projects/:pid/videos/generate(-batch)
  → video-service CreateTask + DispatchTask
  → Kafka topic: video.generate.request
  → KafkaConsumer（max_kafka_tasks 并发任务）
  → ProcessTask
      ├─ 准备角色参考图 / 资产锚点 / Motion LLM 优化
      ├─ 创建 VideoClip 记录
      ├─ 并行 clip 生成（max_clips 信号量）
      ├─ 失败 clip 自动重试 1 次
      ├─ 部分成功可合成（并行模式默认开启）
      └─ FFmpeg 拼接 → 配音/字幕 → 上传 MinIO
```

串行模式（`video_serial` / `serial_scene=true`）在**场景组内**按序生成、组间并行，见第 4 节。

## 2. 前端触发

| 入口 | API | 说明 |
|------|-----|------|
| 单集生成 | `POST .../videos/generate` | `VideoTab.handleGenerateEpisode` |
| 多集批量 | `POST .../videos/generate-batch` | `VideoTab.handleGenerateAll` |
| 分镜页快捷生成 | `POST .../videos/generate` | `StoryboardTab` |

提交 payload 核心字段：

- `image_urls`：与分镜一一对应的首帧图（串行模式允许非首帧为空）
- `scene_descriptions` / `dialogues` / `durations` / `camera_movements` 等 per-clip 元数据
- `serial_scene` + `scene_group_keys`：串行场景链
- `render_config.allow_incomplete_compose`：是否允许跳过失败 clip 继续合成

前端已改为复用 SWR 缓存的 `storyboards-for-video` 数据，避免每次生成重复 `listAll`。

## 3. 任务调度层

### 3.1 任务创建

`video_handler.GenerateProjectVideo` / `GenerateProjectVideosBatch` 写入 `video_tasks` 表，状态 `pending`，并调用 `DispatchTask` 发布 Kafka 消息。

### 3.2 Kafka 消费

文件：`video-service/internal/service/kafka_consumer.go`

- 读取消息后立即 commit，任务在内存队列等待执行槽
- `max_kafka_tasks`（默认 5，可在 `config.local.yaml` 调整）限制**同时处理的视频任务数**
- 单任务超时 3 小时，服务重启不取消已在飞的 clip 生成

### 3.3 服务重启恢复

`ResumePendingTasks` 会重新分发 DB 中仍为 `pending` 且未入队的任务。

## 4. ProcessTask：并行 vs 串行

文件：`video-service/internal/service/video_service.go` → `ProcessTask`

### 4.1 并行模式（默认）

条件：`serial_scene=false`，或串行链未满足（`scene_group_keys` 长度不匹配等）。

```
for each clip in task:
  acquire semaphore(max_clips)
  buildAndGenClip(clip, maxAttempts=6)
  release semaphore
```

**并发上限 `clipConcurrencySlots`**（按模型动态）：

| 模型/生成器 | 并发 clip 数 |
|-------------|-------------|
| ComfyUI 本地 | `local_max_clips`（默认 1） |
| 腾讯 VCLM | 1 |
| RunningHub | min(max_clips, 2) |
| MiniMax | min(max_clips, 4) |
| 其他云端 API | `max_clips`（默认 3，配置可调到 5–8） |

### 4.2 串行模式

条件：`serial_scene=true` 且 `scene_group_keys` 与 clip 数一致。

```
按 scene_group_key 分组
  组间：并行（受 clipConcurrencySlots 限制）
  组内：严格顺序
    clip[n] 成功后 → frame-extractor 提取末帧 → 作为 clip[n+1] 首帧
    任一环失败 → 后续 clip 标记链断，组内停止
```

## 5. 失败恢复与部分合成

### 5.1 clip 级自动重试

首轮生成结束后，对 `status=failed` 的 clip **自动再试 1 次**：

- 并行：失败 clip 并发重试
- 串行：从组内首个失败 clip 起，沿链重跑

底层仍使用 `generateClipWithRetry`（瞬态错误指数退避，最多 3–6 次 API 尝试）。

### 5.2 部分成功可合成

并行模式**默认** `allow_incomplete_compose=true`：

- 只要有 ≥1 个成功 clip，就继续 FFmpeg 合成
- `render_config.partial_compose=true` 与 `failed_clip_count` 写入任务元数据
- 串行模式保持严格：需显式设置 `allow_incomplete_compose=true` 才允许部分合成

手动修复：前端支持单 clip 重试（`POST .../clips/:cid/retry`）。

## 6. 合成与后处理

成功 clip 按 `clip_order` 收集 URL 后：

1. `ConcatClipsWithTransitionPlan`（支持转场计划）
2. 分 clip 配音合成 `tryPerClipAudioCompose`（优先，避免音画漂移）
3. 或整轨配音 `AddAudio` + VTT/文本字幕
4. 可选 BGM
5. 上传 storage-service → `result_url`

## 7. 分镜完成后自动触发视频（非广告）

文件：`project-service/internal/service/auto_video_trigger.go`

当分镜 Kafka 消费者完成一张图后，会异步检查：

- 非 `ad-workbench` 标签项目
- `storyboard_config.auto_generate_video` 未关闭（默认 true）
- 无 pending / generating / failed 分镜
- 尚未触发过（`auto_video_triggered=false`）

满足条件则调用 `POST .../videos/generate-batch`，并将项目状态置为 `video_generating`。

口播广告工作台（`style_tags: ['ad-workbench']`）**不参与**此自动链路。

## 8. 推荐配置

`config.local.yaml`：

```yaml
video-service:
  concurrency:
    max_clips: 5
    max_kafka_tasks: 8
  frame_extractor:
    base_url: "http://localhost:8010"  # 串行模式强烈建议启用
```

项目级 `storyboard_config`：

```json
{
  "auto_generate_video": true,
  "video_model": "kling",
  "style_preset": "anime-2d",
  "motion_mode": "gentle",
  "duration": 5
}
```

关闭自动视频：`"auto_generate_video": false`
