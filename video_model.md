# dumps1 视频模型整理（第一版，仅视频）

> 目标目录：`/Users/hh/Desktop/claw/fengxi/dumps1`
> 输出文件：`/Users/hh/Desktop/Project/AI-generatedVideo/video_model.md`
> 本版范围：**只整理视频模型**，优先收口：
> 1. 模型名 / 别名
> 2. 支持的任务类型
> 3. 调用路径 / URL / base_url / endpoint
> 4. dump 中可见的真实运行命中

---

## 1. 主证据源

### A. `192.168.5.231_video_user_3306_limited.sql`
用于看**模型登记态 / 产品态**：

- `config_model`：模型总表
- `config_task_type`：任务类型表
- `config_task_type_model`：任务类型 ↔ 模型矩阵
- `config_model_channel`：模型渠道 / `endpoint` / `base_url`

### B. `192.168.5.231_ai_video_3306_limited.sql`
用于看**路由态 / 运行态**：

- `video_provider_config`：视频 provider 路由 / URL / adapter
- `video_script_model`：自动流 / 分镜视频模型
- `video_generation_task`：真实任务命中

---

## 2. 先说结论

1. **真实任务命中最强的是 `vidu / ds-video-1.0`**。
   - `reference2video`: 148
   - `img2video`: 22
   - `upscale2video`: 21

2. dump 中还能看到少量历史命中：
   - `zhipu / ds-video-2.0 / img2video`
   - `qwen / ds-video-3.0 / img2video`

3. `config_model` 里已经登记了很多新视频模型，例如：
   - `doubao-seedance-1-5-pro-251215`
   - `wan2.6-r2v`
   - `kling-v2-6`
   - `MiniMax-Hailuo-2.3`
   - `sora2`
   - `TC-GV`
   但**登记 ≠ 真实跑过**，是否真正命中仍要看 `video_generation_task`。

4. `video_provider_config` 能直接看出系统实际把请求打到哪：
   - `https://api.vidu.cn/ent/v2`
   - `https://ark.cn-beijing.volces.com/api/v3/contents/generations/tasks`
   - `https://www.sophnet.com/api/open-apis/projects/easyllms/videogenerator/volces/tasks`
   - `https://hubagi.cn/api/v1/video/generations`
   - `https://api.gaga.art/v1/generations`
   - `vod.tencentcloudapi.com`
   - `https://vod.bj.baidubce.com`
   - 多条 `sora-2` 代理入口

5. 有一类高风险混淆项必须单独看：
   - `model_view = doubao-seedance`
   - 但 `model = doubao-seedream-4-0-250828`
   - 且 `generate_type = text2Img`

   这类**更像图像链，不应直接当视频链**。

---

## 3. 真实运行命中（来自 `video_generation_task`）

| provider/company | model | generate_type | 命中次数 | 说明 |
|---|---|---:|---:|---|
| vidu | ds-video-1.0 | reference2video | 148 | dump 中最强真实命中 |
| vidu | ds-video-1.0 | img2video | 22 | dump 中最强真实命中 |
| vidu | ds-video-1.0 | upscale2video | 21 | dump 中最强真实命中 |
| zhipu | ds-video-2.0 | img2video | 7 | 历史少量命中 |
| qwen | ds-video-3.0 | img2video | 2 | 历史少量命中 |

这张表的意义：
- **优先级最高**，因为它最接近真实运行态。
- 如果某模型只出现在 `config_model`，但没在这里命中，就不能说它已经被该系统大规模真实使用。

---

## 4. 视频任务类型字典（来自 `config_task_type`）

| task_type_id | type_code | old_code | 中文说明 |
|---:|---|---|---|
| 1 | VIDEO_REFERENCE | reference2video | 融合生成视频 |
| 2 | VIDEO_IMAGE | img2video | 图片生成视频 |
| 3 | VIDEO_BEGIN_END | startEnd2video | 首尾帧生成视频 |
| 4 | VIDEO_UPSCALE | upscale | 视频高清转码 |
| 9 | VIDEO_MODIFY | editvideo | 编辑视频生成 |
| 10 | VIDEO_AUDIO | lipVideo | 对口型视频生成 |
| 12 | VIDEO_STORY | storyboard2video | 分镜生成视频 |
| 20 | VIDEO_ERASE | VIDEO_ERASE | 视频字幕擦除 |
| 22 | VIDEO_CLIP | VIDEO_CLIP | 视频剪辑 |

---

## 5. 视频模型主表（只保留视频相关）

> 来源：`config_model + config_task_type_model`
> 说明：这一节是“模型登记态”。

| model_id | model_code | 对外别名 | family | 状态 | 支持的视频任务类型 | 备注 |
|---:|---|---|---|---|---|---|
| 102 | MiniMax-Hailuo-02 | xingdou-1.0 / 星斗 1.0 | minmax | 启用 | `img2video`、`startEnd2video` | 擅长打斗和大动作 |
| 103 | MiniMax-Hailuo-2.3 | xingdou-2.0 / 星斗 2.0 | minmax | 启用 | `img2video` | 升级打斗及运动表现力，增强质感 |
| 104 | P | xingyu-1.0 / 星语 1.0 | baidu | 启用 | `lipVideo` | 文字对口型 |
| 105 | P2 | xingyu-1.5 / 星语 1.5 | kling | 启用 | `lipVideo` | 对口型路线 |
| 107 | doubao-seedance-1-5-pro-251215 | xingguang-2.5 / 星光 2.5 | doubao | 启用 | **未在任务矩阵中绑定视频任务** | 声画同出，多人对白对口型强 |
| 111 | baidu-video-1.0 | xingchen-1.0 / 星辰 1.0 | baidu | 启用 | `upscale` | 视频高清转码 |
| 112 | cogvideox-2 | xingguang-1.0 / 星光 1.0 | zhipu | 启用 | 未在任务矩阵中绑定视频任务 | 历史视频模型登记 |
| 113 | cogvideox-3 | xingguang-2.0 / 星光 2.0 | zhipu | 启用 | 未在任务矩阵中绑定视频任务 | 历史视频模型登记 |
| 115 | doubao-seedance-1-0-lite-i2v-250428 | xinghuo-1.0 / 星火 1.0 | doubao | 禁用 | 未在任务矩阵中绑定视频任务 | 旧版 i2v |
| 116 | doubao-seedance-1-0-pro-250528 | xinghuo-2.0 / 星火 2.0 | doubao | 启用 | `img2video` | Doubao 视频 |
| 117 | doubao-seedance-1-0-pro-fast-251015 | xinghuo-2.0-discount / 星火-折扣 | doubao | 启用 | `img2video` | Doubao 折扣视频线 |
| 118 | Seedance-1.5-Pro | xingguang-2.5-sn / 星光 2.5 | suanneng | 启用 | `img2video`、`startEnd2video` | 算能视频线 |
| 121 | gaga-1 | xingdian-2.0 / 星点 2.0 | gaga | 启用 | `img2video` | 声画同出，性价比型 |
| 127 | kling-v2-5-turbo | xinglan-2.0 / 星澜 2.0 | kling | 启用 | `img2video` | Kling 视频 |
| 128 | kling-v2-6 | xinglan-2.5 / 星澜 2.5 | kling | 启用 | `img2video` | 声画同出 |
| 134 | sora2 | xingxiu-2.0 / 星梭 2.0 | sora2 | 禁用 | `img2video` | Sora2 路线 |
| 135 | v5.5 | xinghui-2.5 / 星辉 2.5 | aishi | 禁用 | `img2video`、`startEnd2video` | 多镜头叙事 |
| 136 | TC-GV | xingwei-3.1 / 星威 3.1 | hubagi | 禁用 | `img2video` | 测试版 |
| 137 | vidu2.0 | xingchen-1.0 / 星辰 1.0 | vidu | 禁用 | `reference2video`、`storyboard2video` | 早期 Vidu |
| 139 | viduq1 | xingchen-2.0 / 星辰 2.0 | vidu | 启用 | `reference2video`、`storyboard2video` | Vidu |
| 140 | viduq2 | xingchen-2.5 / 星辰 2.5 | vidu | 启用 | `reference2video`、`storyboard2video` | Vidu |
| 141 | viduq2-pro | xingchen-2.5 / 星辰 2.5 | vidu | 启用 | `img2video`、`startEnd2video` | Vidu |
| 142 | viduq3-pro | xingchen-2.6 / 星辰 3.0 | vidu | 启用 | `img2video`、`startEnd2video` | 声画同出，多人对白多镜头 |
| 143 | wan2.2-i2v-plus | xinghai-2.0 / 星海 2.0 | wanx | 启用 | `img2video` | Wan |
| 145 | wan2.6-i2v | xinghai-2.5 / 星海 2.5 | wanx | 启用 | `img2video` | 声画同出 |
| 146 | wan2.6-r2v | xinghe-2.5 / 星河 2.5beta | wanx | 启用 | `reference2video` | Wan R2V |
| 163 | Kling-3.0-Omni | xinghe-3.0 / 星河 3.0 | kling | 启用 | `reference2video` | 多模态输入 |
| 164 | Kling-3.0 | xinghe-3.0 / 星河 3.0 | kling | 启用 | `img2video`、`startEnd2video` | Kling 3.0 |
| 165 | v5.6 | xinghui-2.6 / 星辉 2.6 | aishi | 启用 | `img2video`、`startEnd2video`、`reference2video` | 爱诗视频 |
| 167 | viduq3 | xingyan-3.0 / 星焰 3.0 | vidu | 启用 | `reference2video` | Vidu |
| 170 | kling-video-o1 | xingyi-1.0 / 星移 1.0 | kling | 启用 | `editvideo` | 视频编辑 |
| 171 | V4.0 | V4.0 | doubao | 启用 | `reference2video` | 声画同出，偏真人 |
| 172 | xingguang-3.0 | 星光 3.0 | doubao | 启用 | `reference2video` | Doubao 新视频线 |
| 173 | smarterase | quzimu-1.0 / 去字幕 1.0 | tencent_mps | 启用 | `VIDEO_ERASE` | 视频字幕擦除 |
| 175 | viduq3-mix | xingchen-3.1 / 星辰 3.1 | vidu | 禁用 | `img2video`、`startEnd2video`、`reference2video` | Vidu mix |
| 176 | v6 | xinghui-3.0 / 星辉 3.0 | aishi | 启用 | `img2video`、`startEnd2video` | 爱诗视频 |
| 177 | c1 | xinghui-3.0 / 星辉 3.0 | aishi | 启用 | `reference2video` | 爱诗视频 |
| 178 | happyhorse-1.0-i2v | xingchi-3.0 / 星驰 3.0 | wanx | 启用 | `img2video` | Wan 特惠线 |
| 179 | wan2.7-r2v | xinghe-2.7 / 星河 2.7 | wanx | 启用 | `reference2video` | Wan 新版 R2V |
| 180 | gemini-3.1-pro-preview | xingwen-2.0 / 星文 2.0 | gemini | 启用 | `VIDEO_CLIP` | 视频剪辑 |
| 184 | happyhorse-1.0-r2v | xingchi-3.0 / 星驰 3.0 | wanx | 启用 | `reference2video` | Wan 特惠线 |
| 185 | smarterase2.0 | quzimu-2.0 / 去字幕 2.0 | tencent_mps | 启用 | `VIDEO_ERASE` | 大模型去字幕 |
| 186 | global.anthropic.claude-opus-4-6-v1 | xingwen-2.0 / 星文 2.0 | claude | 启用 | `VIDEO_CLIP` | 视频剪辑 |

---

## 6. 自动流 / 分镜视频模型（`video_script_model`）

> 这一组最适合看：**自动流实际暴露了哪些视频模型，以及它们走什么 path / base_url**。

| id | model_name | model_view | 厂商模型 | generate_type | duration | resolution | aspect_ratio_list | endpoint(path) | base_url | 状态 |
|---:|---|---|---|---|---:|---|---|---|---|---|
| 1 | 星河3.0 | xinghe-3.0 | Kling-3.0-Omni | reference2video | 15 | 1080P-有声 | 16:9,9:16,1:1 | / | - | 启用 |
| 2 | 星河2.5 | xinghe-2.5 | wan2.6-r2v | reference2video | 5 | 1080P(单镜头) | 16:9,9:16,1:1 | / |  | 启用 |
| 3 | 星辰2.5 | xingchen-2.5 | viduq2 | reference2video | 4 | 1080p | 16:9,9:16,1:1 | / | - | 启用 |
| 4 | 星辰2.0 | xingchen-2.0 | ds-video-1.2 | reference2video | 5 | 1080p | 16:9,9:16,1:1 | /v2/aigc/image_to_video | https://vod.bj.baidubce.com | 启用 |
| 5 | 星辰1.0 | xingchen-1.0 | ds-video-1.0 | reference2video | 4 | 720p | 16:9,9:16,1:1 | /v2/aigc/image_to_video | https://vod.bj.baidubce.com | 启用 |
| 7 | 星辉2.6 | xinghui-2.6 | v5.6 | reference2video | 5 | 720p-有声 | 16:9,9:16,4:3,3:4,1:1 | / | - | 启用 |
| 8 | 星辰3.0 | xingchen-3.0 | viduq3 | reference2video | 8 | 1080p | 16:9,9:16,1:1 | / | - | 启用 |
| 10 | V4.0 | V4.0 | V4.0 | reference2video | 5 | 720p | 16:9,9:16,4:3,3:4,1:1 | / | - | 启用 |

### 自动流里能直接读出的映射

- `xinghe-3.0` → `Kling-3.0-Omni` → `reference2video`
- `xinghe-2.5` → `wan2.6-r2v` → `reference2video`
- `xingchen-2.5` → `viduq2` → `reference2video`
- `xingchen-2.0` → `ds-video-1.2`
  - `base_url = https://vod.bj.baidubce.com`
  - `endpoint = /v2/aigc/image_to_video`
- `xingchen-1.0` → `ds-video-1.0`
  - `base_url = https://vod.bj.baidubce.com`
  - `endpoint = /v2/aigc/image_to_video`
- `xinghui-2.6` → `v5.6` → `reference2video`
- `xingchen-3.0` → `viduq3` → `reference2video`
- `V4.0` → `V4.0` → `reference2video`

---

## 7. Provider 路由 / URL / 调用路径（`video_provider_config`）

> 这一节是“系统到底往哪打请求”的主证据。

| id | subgroup | provider_name | model_view | model | generate_type | enable | endpoint/url | adapter | note |
|---:|---|---|---|---|---|---|---|---|---|
| 204 | video | 薛总 | voe3.1 | voe3.1 | img2video | 启用 | https://www.hubagi.cn/api/v1/video/generations |  |  |
| 301 | sora2 | polo | sora2 | sora-2 | img2video | 禁用 | https://poloai.top/v1/videos | OpenAIVideoGenerateService |  |
| 305 | sora2 | aipg | sora2 | sora-2 | img2video | 禁用 | https://api.easyart.cc/v1/videos |  |  |
| 306 | sora2 | vip1 | sora2 | sora-2 | img2video | 启用 | http://38.46.221.145:8868/v1/videos |  |  |
| 307 | sora2 | vip2 | sora2 | sora-2 | img2video | 禁用 | http://38.46.221.145:8868/v1/videos |  |  |
| 308 | sora2 | 盛总 | sora2 | sora-2 | img2video | 禁用 | http://104.243.37.148:3000/v1/videos |  |  |
| 309 | sora2 | dyuapi | sora2 | sora-2 | img2video | 禁用 | https://api.dyuapi.com/v1/videos |  |  |
| 311 | vidu | 薛总 |  |  |  | 禁用 | https://hubagi.cn/api/v1/video/vidu |  | Vidu 代理入口 |
| 312 | vidu | vidu官方 |  |  |  | 启用 | https://api.vidu.cn/ent/v2 |  | 官方入口 |
| 313 | vidu-offpeak | vidu官方 | any | any | any | 启用 | https://api.vidu.cn/ent/v2 |  | 生数错峰 |
| 315 | gaga | gaga | xingdian2.0 | gaga-1 | img2video | 启用 | https://api.gaga.art/v1/generations |  |  |
| 316 | hubagi | hubagi | xingwei-3.1 | TC-GV | reference2video | 启用 | https://hubagi.cn/api/v1/video/generations |  |  |
| 317 | vidu | vidu官方-3pro | xingcheng-2.6 | viduq3-pro |  | 启用 | https://api.vidu.cn/ent/v2 |  | 官方 3pro |
| 319 | veo | 薛总 | xingwei-3.1 | voe3.1 | startEnd2video | 启用 | https://www.hubagi.cn/api/v1/video/generations |  |  |
| 320 | vidu | vidu官方-3pro | xingcheng-2.6 | viduq3-pro |  | 启用 | https://api.vidu.cn/ent/v2 |  | 官方 3pro |
| 321 | suanneng | suanneng | xingguang-2.5 | Seedance-1.5-Pro | any | 启用 | https://www.sophnet.com/api/open-apis/projects/easyllms/videogenerator/volces/tasks |  | 算能 |
| 342 | kling | tx | xinghe-3.0 | Kling-3.0 | img2video | 启用 | vod.tencentcloudapi.com |  | 腾讯云 |
| 343 | kling | tx | xinghe-3.0 | Kling-3.0 | startEnd2Video | 启用 | vod.tencentcloudapi.com |  | 腾讯云 |
| 344 | kling | tx | xinghe-3.0-Omni | Kling-3.0-Omni | reference2video | 启用 | vod.tencentcloudapi.com |  | 腾讯云 |
| 345 | kling | baidu | xinghe-3.0-Omni | Kling-3.0-Omni | reference2video | 禁用 | https://vod.bj.baidubce.com |  | 百度 |
| 346 | kling | baidu | xinghe-3.0 | Kling-3.0 | startEnd2Video | 启用 | https://vod.bj.baidubce.com |  | 百度 |
| 347 | kling | baidu | xinghe-3.0 | Kling-3.0 | img2video | 启用 | https://vod.bj.baidubce.com |  | 百度 |
| 349 | kling | aiping | xinghe-3.0-Omni | Kling-3.0-Omni | reference2video | 禁用 | https://aiping.cn/api/v1/videos |  | 爱平 |
| 350 | kling | aiping | xinghe-3.0 | Kling-3.0 | startEnd2Video | 禁用 | https://aiping.cn/api/v1/videos |  | 爱平 |
| 351 | kling | aiping | xinghe-3.0 | Kling-3.0 | img2video | 禁用 | https://aiping.cn/api/v1/videos |  | 爱平 |
| 356 | doubao | doubao | V4.0 | V4.0 | reference2video | 启用 | https://ark.cn-beijing.volces.com/api/v3/contents/generations/tasks |  | Doubao Ark |
| 358 | doubao | doubao | xingguang-3.0 | V4.0 | reference2video | 启用 | https://ark.cn-beijing.volces.com/api/v3/contents/generations/tasks |  | Doubao Ark |
| 360 | vidu | vidu官方-mix | xingchen-3.1 | viduq3-mix |  | 启用 | https://api.vidu.cn/ent/v2 |  | Vidu mix |

---

## 8. 按产品线收口

### 8.1 Vidu 系

#### 模型侧
- `vidu2.0`
- `viduq1`
- `viduq2`
- `viduq2-pro`
- `viduq3`
- `viduq3-pro`
- `viduq3-mix`

#### 任务类型
- `reference2video`
- `img2video`
- `startEnd2video`
- `storyboard2video`
- `upscale2video`（从真实任务命中可见）

#### URL / 路径
- `https://api.vidu.cn/ent/v2`
- `https://hubagi.cn/api/v1/video/vidu`

#### 真实命中
- `vidu / ds-video-1.0 / reference2video`
- `vidu / ds-video-1.0 / img2video`
- `vidu / ds-video-1.0 / upscale2video`

---

### 8.2 Doubao / Seedance / Ark 系

#### 模型侧
- `doubao-seedance-1-5-pro-251215`
- `doubao-seedance-1-0-lite-i2v-250428`
- `doubao-seedance-1-0-pro-250528`
- `doubao-seedance-1-0-pro-fast-251015`
- `V4.0`
- `xingguang-3.0`
- `Seedance-1.5-Pro`（suanneng 线）

#### 任务类型
- `img2video`
- `reference2video`
- `startEnd2video`

#### URL / 路径
- `https://ark.cn-beijing.volces.com/api/v3/contents/generations/tasks`
- `https://www.sophnet.com/api/open-apis/projects/easyllms/videogenerator/volces/tasks`

#### 备注
- `doubao-seedance-1-5-pro-251215` 已登记，但在这份 dump 的任务矩阵中没有强绑定信息。
- `suanneng` 路由里 `model = Seedance-1.5-Pro`，走 Sophnet 视频生成入口。

---

### 8.3 Kling 系

#### 模型侧
- `kling-v2-5-turbo`
- `kling-v2-6`
- `Kling-3.0`
- `Kling-3.0-Omni`
- `kling-video-o1`

#### 任务类型
- `img2video`
- `startEnd2video`
- `reference2video`
- `editvideo`

#### URL / 路径
- `vod.tencentcloudapi.com`
- `https://vod.bj.baidubce.com`
- `https://aiping.cn/api/v1/videos`

---

### 8.4 Wan / 千问 系

#### 模型侧
- `wan2.2-i2v-plus`
- `wan2.6-i2v`
- `wan2.6-r2v`
- `wan2.7-r2v`
- `happyhorse-1.0-i2v`
- `happyhorse-1.0-r2v`

#### 任务类型
- `img2video`
- `reference2video`

#### 真实命中
- `qwen / ds-video-3.0 / img2video`（少量）

---

### 8.5 MiniMax / Hailuo 系
a
#### 模型侧
- `MiniMax-Hailuo-02`
- `MiniMax-Hailuo-2.3`

#### 任务类型
- `img2video`
- `startEnd2video`

#### 备注
- 当前 dump 中看得到模型登记与任务矩阵，但还没看到很强的真实任务命中。

---

### 8.6 Sora2 系

#### 模型侧
- `sora2`

#### 任务类型
- `img2video`

#### URL / 路径
- `https://poloai.top/v1/videos`
- `https://api.easyart.cc/v1/videos`
- `http://38.46.221.145:8868/v1/videos`
- `http://104.243.37.148:3000/v1/videos`
- `https://api.dyuapi.com/v1/videos`

---

### 8.7 Hubagi / VEO / Gaga 系

#### 模型侧
- `TC-GV`
- `voe3.1`
- `gaga-1`

#### 任务类型
- `reference2video`
- `img2video`
- `startEnd2video`

#### URL / 路径
- `https://hubagi.cn/api/v1/video/generations`
- `https://api.gaga.art/v1/generations`

---

## 9. 高风险混淆点

### 9.1 `doubao-seedance` 名字混淆
以下条目不能直接算视频：

| id | model_view | model | generate_type | endpoint |
|---:|---|---|---|---|
| 90 | doubao-seedance | doubao-seedream-4-0-250828 |  | https://ark.cn-beijing.volces.com |
| 91 | doubao-seedance | doubao-seedream-4-0-250828 |  | https://ark.cn-beijing.volces.com |
| 92 | doubao-seedance | doubao-seedream-4-0-250828 | text2Img | https://ark.cn-beijing.volces.com/api/v3/images/generations |
| 93 | doubao-seedance | doubao-seedream-4-0-250828 | text2Img | https://api.chatfire.cn/v1/images/generations |
| 94 | doubao-seedance | seedream-4-hd | text2Img | https://newgg.aionline.fun/v1/images/generations |

结论：
- 这批更像是**图像链 / 星图链**，不是视频链。
- 不能因为 `model_view` 叫 `doubao-seedance`，就直接推断它们是视频生成配置。

### 9.2 配置态 ≠ 运行态
必须分三层看：

1. **配置态**：`config_model`
2. **路由态**：`video_provider_config` / `video_script_model`
3. **运行态**：`video_generation_task`

最终判断“这模型是否真的在用”，优先级应是：

`video_generation_task` > `video_provider_config` > `video_script_model` > `config_model`

---

## 10. 当前最值得优先盯的几条视频线

### 第一优先：Vidu 线
因为它在 dump 中有最明显真实命中。

### 第二优先：Doubao / Seedance / Suanneng 线
因为模型登记和 provider 路由都很多，但混淆也最大，值得继续核清“哪个是视频、哪个其实是图像”。

### 第三优先：Kling / Wan 线
因为自动流模型里已经能看到比较清楚的 `reference2video` 绑定关系。

---

## 11. 下一步建议

如果继续做第二版，建议直接往下面三个方向深挖：

1. **每个视频模型对应的真实 provider/channel**
   - 把 `config_model_channel` 和 `video_provider_config` 做更严谨映射

2. **每条视频线的真实调用 path**
   - 例如 `Ark / Vidu / 腾讯云 Kling / 百度 / Sophnet / Hubagi`

3. **把“视频模型 / 图像模型 / 文本模型”彻底拆开**
   - 避免 `doubao-seedance` / `seedream` 这类名称混淆
