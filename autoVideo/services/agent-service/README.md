# agent-service

最小可用版 AI Orchestrator / Agent Service。

## 当前能力

- 提供统一 Agent HTTP 入口
- 暴露可用 tool 列表
- 根据用户目标生成结构化 plan
- 执行 plan
- 已接入部分真实下游调用，失败时自动回退 stub
- 支持 step 依赖校验
- 支持把前序 step 输出注入后续 step 输入

## 当前 tool 状态

### 已接真实 / 内建 tool
- `project.get`
- `script.generate`
- `dubbing.generate`
- `shot_plan.generate`（内建本地规划 tool）
- `image.generate_batch`
- `video.generate_batch`
- `task.get_status`

其中 `script.generate` 当前调用：
- `POST /api/v1/script-library/generate`（script-service）

并会自动做最小字段适配：
- `goal -> premise`
- `constraints -> requirements(JSON string)`
- 默认 `mode=script`

同时 agent-service 会从 script 结果里尽量提取结构化信息：
- `script_storyboard`
- `script_dialogues`
- `script_structured_content`

当前优先来源：
- `outline[]`
- 其次 `content`

### `dubbing.generate`

当前调用：
- `POST /api/v1/dubbing/generate`

会自动从以下字段推导配音分段：
- `script_dialogues`
- `script_content`
- `subtitle_text`
- `source_text`

并尽量返回：
- `audio_url`
- `subtitle_timeline`
- `narration_segments`
- `dubbing`

当下游不可用时仍会回退到 stub/fallback。

## API

- `GET /health`
- `GET /api/v1/agent/tools`
- `POST /api/v1/agent/plans`
- `POST /api/v1/agent/plans/execute`
- `GET /api/v1/agent/executions/:id`
- `POST /api/v1/agent/executions/:id/retry`
- `POST /api/v1/agent/executions/:id/resume`
- `POST /api/v1/agent/executions/:id/replay-from/:stepId`
- `GET /api/v1/agent/plans/:id/executions`

execution store 默认使用内存；开启 Redis 后可跨服务重启保留 execution record。

`GET /api/v1/agent/plans/:id/executions` 支持查询参数：
- `status=succeeded|failed|running`
- `limit=20`

## 示例：生成计划

```bash
curl -X POST http://localhost:8012/api/v1/agent/plans \
  -H 'Content-Type: application/json' \
  -d '{
    "goal": "生成一个30秒剧情短片",
    "project_id": 1,
    "episode_id": 1,
    "user_id": 100,
    "constraints": {
      "duration_sec": 30,
      "style": "赛博朋克",
      "quality_priority": "high"
    }
  }'
```

## 示例：执行计划

`ExecutePlan` 现在支持：

- `depends_on`：依赖 step 成功后才执行
- `use_result_fields`：将前序 step 输出映射到当前 step 输入

映射语法示例：

- `step-1`：注入整个 step-1 输出
- `step-1.project`：注入 step-1 输出里的 `project`
- `step-2.submission`：注入 step-2 输出里的 `submission`

例如：

```json
{
  "plan": {
    "plan_id": "demo",
    "steps": [
      {
        "id": "step-1",
        "tool": "project.get",
        "input": {
          "project_id": 1,
          "user_id": 100,
          "authorization": "Bearer <token>"
        }
      },
      {
        "id": "step-2",
        "tool": "script.generate",
        "depends_on": ["step-1"],
        "use_result_fields": {
          "project_context": "step-1.project"
        },
        "input": {
          "goal": "生成一个30秒剧情短片"
        }
      }
    ]
  }
}
```

执行响应中会返回：

- `execution_id`
- `execution_meta`
- `resolved_input`
- `dependency_info`

同时服务会在内存中保存 execution record，可通过：

- `GET /api/v1/agent/executions/:id`
- `POST /api/v1/agent/executions/:id/retry`
- `POST /api/v1/agent/executions/:id/resume`
- `POST /api/v1/agent/executions/:id/replay-from/:stepId`
- `GET /api/v1/agent/plans/:id/executions`

查询执行状态、历史；其中：
- 历史列表接口支持按 `status` 过滤，并用 `limit` 控制返回条数
- `retry`：基于原始 plan 全量重跑
- `resume`：复用原 execution 中已成功的前序 step，只从失败/中断位置继续跑
- `replay-from/:stepId`：从指定 step 开始重跑，之前的成功 step 作为依赖输入复用，之后的 downstream 全部重跑

注意：当前默认 persistence 为内存版，服务重启后 execution 记录会丢失；若启用 Redis，可保留 execution record 到 TTL 过期。`resume` / `replay-from` 目前依赖线性 step 顺序复用结果，还不支持任意 DAG 节点级恢复。

并且当前 plan 已扩展为两条模式：

- 默认模式：
  - `project.get -> script.generate -> image.generate_batch -> video.generate_batch`
- `commentary_comic` 模式：
  - `project.get -> script.generate -> dubbing.generate -> shot_plan.generate -> image.generate_batch -> video.generate_batch`

其中：
- `script.generate` 会额外提取：
  - `script_storyboard`
  - `script_dialogues`
  - `script_structured_content`
- `shot_plan.generate` 会把 scene 级 storyboard 扩展成 shot 级结构，输出：
  - `shot_plan`
  - `shots`
- 每个 shot 当前会附带：
  - `panel_type`
  - `camera`
  - `visual_focus`
  - `character_pose`
  - `emotion`
  - `transition`
  - `duration_sec`
  - `subtitle_candidate`
  - `character_name`
  - `character_focus`
  - `costume_hint`
  - `location_tag`
- `image.generate_batch` 会调用 `POST /api/v1/images/generate` 多次提交 storyboard 图像任务
- 可选等待图片任务完成并轮询 `GET /api/v1/images/tasks/:id`
- 默认模式从 `step-2.script_storyboard` 提取 prompt 列表
- `commentary_comic` 模式从 `step-4.shots` 提取 shot prompt，并自动把 `panel_type / camera / visual_focus / character_pose / emotion / transition / subtitle_candidate / duration_sec` 拼入图片 prompt
- `commentary_comic` 模式下，`video.generate_batch` 会把以下字段同步写入 `episodes[*]`：
  - `audio_url`
  - `subtitle_timeline`
  - `scene_descriptions`
  - `dialogues`
  - `subtitle_text`
  - `image_urls`
  - `camera_movements`
  - `moods`
  - `transition_plan`
  - `character_focus`
  - `costume_hints`
  - `location_tags`
- `video-service` 当前对这些字段的消费状态：
  - `audio_url`：已用于最终视频配音/音轨合成
  - `subtitle_timeline`：已在 compose 阶段消费，用于最终字幕烧录
  - `scene_descriptions` / `dialogues`：已用于 clip prompt、字幕 fallback 与 narration 上下文
  - `camera_movements` / `moods`：已用于 clip prompt 与转场语义规划
  - `transition_plan`：handler 层会桥接到 `transition_notes`，并参与 prompt / continuity 提示
  - `character_focus`：handler 层会桥接到 `scene_characters`，并参与角色参考图过滤；同时保留原字段进入 prompt
  - `costume_hints` / `location_tags`：已进入 render_config，并参与 clip prompt / continuity prompt 组装
  - `image_urls`：已作为 batch 生成输入直接提交给 video-service
- 字段 shape 约定：
  - `audio_url` / `subtitle_text`：字符串
  - `subtitle_timeline`：`[]object`，推荐每项包含 `text`, `start_sec`, `end_sec`
  - `scene_descriptions` / `dialogues` / `image_urls` / `camera_movements` / `moods` / `transition_plan` / `character_focus` / `costume_hints` / `location_tags`：按 shot / clip 对齐的 `[]string`

便于调试 agent 编排行为。

## 当前设计定位

本服务先只做：

1. Orchestrator Agent
2. Plan Schema
3. Tool Registry
4. 部分真实 tool + stub fallback
5. 基础依赖执行器

后续建议迭代：

- 把当前启发式 `shot_plan.generate` 升级为 LLM/规则混合版
- 增加镜头节奏控制、转场模板、分镜密度策略
- 加入 recovery agent
- 加入 storyboard agent / character agent
- 做多 Agent supervisor 协调

## 配置示例

可在统一配置中增加：

```yaml
agent-service:
  http:
    port: 8012
  gateway:
    addr: http://localhost:8000
    self_addr: http://localhost:8012
  agent:
    default_model: gpt-4.1-mini
    system_prompt: "You are an orchestration agent for AI video production."
  redis:
    enabled: true
    addr: localhost:6379
    password: ""
    db: 0
    key_prefix: agent:execution:
    ttl_seconds: 172800
  services:
    project: http://localhost:8007
    script: http://localhost:8003
    image: http://localhost:8005
    video: http://localhost:8006
    dubbing: http://localhost:8006
    task: http://localhost:8008
```
