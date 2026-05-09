/**
 * video-style-config.ts
 * Single source of truth for video style presets, model options, motion modes,
 * and generation presets. Import from here instead of defining locally.
 */

// ── Canonical style keys ──────────────────────────────────────────────────────

export const CANONICAL_VIDEO_STYLE_KEYS = [
  'anime-2d',
  'anime-3d',
  'live-action-film',
  'live-action-short',
] as const

export type VideoStyleKey = (typeof CANONICAL_VIDEO_STYLE_KEYS)[number]

// ── Types ─────────────────────────────────────────────────────────────────────

export type VideoMotionKey = 'gentle' | 'dynamic' | 'cinematic'

export type VideoStyleMode = '2维动漫' | '3维动漫' | '真人电影' | '真人短剧'

export type VideoGenerationPreset = {
  key: string
  label: string
  hint: string
  model: string
  style: string
  motion: VideoMotionKey
  tone: VideoStyleMode
}

export type VideoModelCapability = {
  key: string
  label: string
  icon: string
  desc: string
  provider: string
  audioSupport: 'dubbed' | 'native' | 'none'
  aspectRatio: 'supported' | 'unsupported'
  resolution: 'supported' | 'unsupported'
  multiVariant: 'supported' | 'unsupported'
  clipDuration: string
  note: string
  tags: string[]
  speed: 'fast' | 'medium' | 'slow'
  quality: 'high' | 'standard'
  bestFor: string[]
  supportsStartEnd: boolean  // 首尾帧生视频
  supportsReference: boolean // 融合生视频 (reference2video)
}

export type VideoStylePreset = {
  key: string
  label: string
  desc: string
  tags: string[]
  category: '动画' | '真人' | '艺术化'
  modeTags: VideoStyleMode[]
  families: string[]
  recommendedModels: string[]
  bestFor: string
  texture: string
}

// ── Legacy alias → canonical key mapping ─────────────────────────────────────

export const VIDEO_STYLE_LEGACY_ALIAS_MAP: Record<string, string> = {
  anime: 'anime-2d',
  'comic-dynamic': 'anime-2d',
  'guofeng-myth': 'anime-2d',
  'ink-poetry': 'anime-2d',
  'fantasy-dream': 'anime-3d',
  'cinematic-epic': 'live-action-film',
  'realistic-drama': 'live-action-short',
  'fashion-commercial': 'live-action-short',
  'documentary-natural': 'live-action-short',
  'vintage-film': 'live-action-film',
  'sci-fi-neon': 'live-action-film',
  'suspense-dark': 'live-action-film',
  'warm-healing': 'live-action-short',
  'urban-romance': 'live-action-short',
}

export function normalizeVideoStylePreset(value?: string): VideoStyleKey {
  const trimmed = value?.trim() ?? ''
  if (!trimmed) return 'anime-2d'
  if ((CANONICAL_VIDEO_STYLE_KEYS as readonly string[]).includes(trimmed)) {
    return trimmed as VideoStyleKey
  }
  return (VIDEO_STYLE_LEGACY_ALIAS_MAP[trimmed] ?? 'anime-2d') as VideoStyleKey
}

/** Returns true if the given style key (canonical or alias) maps to a live-action variant. */
export function isLiveActionStyle(value?: string): boolean {
  const canonical = normalizeVideoStylePreset(value)
  return canonical === 'live-action-film' || canonical === 'live-action-short'
}

// ── Style compact options (for selects / badges) ─────────────────────────────

export const VIDEO_STYLE_COMPACT_OPTIONS = [
  { key: 'anime-2d', label: '2维动漫', tone: '2维动漫', hint: '平面线条和番剧感更强，适合连续剧情与角色演出' },
  { key: 'anime-3d', label: '3维动漫', tone: '3维动漫', hint: '更强调体积感、材质和三渲二镜头层次' },
  { key: 'live-action-film', label: '真人电影', tone: '真人电影', hint: '写实电影光影和真实大场面更强' },
  { key: 'live-action-short', label: '真人短剧', tone: '真人短剧', hint: '更贴近短剧对白、人物表演和近景情绪戏' },
] as const

// ── Motion options ────────────────────────────────────────────────────────────

export const VIDEO_MOTION_OPTIONS = [
  { key: 'gentle', label: '柔和', desc: '适合对白、治愈、氛围镜头' },
  { key: 'dynamic', label: '动感', desc: '适合动作、追逐、冲突场景' },
  { key: 'cinematic', label: '电影感', desc: '适合预告、史诗、大片镜头语言' },
] as const

// ── Style mode meta (display colors/hints per category) ──────────────────────

export const VIDEO_STYLE_MODE_META: Record<VideoStyleMode, { label: string; hint: string; className: string }> = {
  '2维动漫': {
    label: '2维动漫',
    hint: '平面线条、番剧感和二维角色表现更强',
    className: 'border-violet-200 bg-violet-50 text-violet-700',
  },
  '3维动漫': {
    label: '3维动漫',
    hint: '体积感、材质和三渲二镜头层次更强',
    className: 'border-fuchsia-200 bg-fuchsia-50 text-fuchsia-700',
  },
  '真人电影': {
    label: '真人电影',
    hint: '真实场景和电影光影更强',
    className: 'border-rose-200 bg-rose-50 text-rose-700',
  },
  '真人短剧': {
    label: '真人短剧',
    hint: '人物对白、近景情绪戏和短剧节奏更强',
    className: 'border-emerald-200 bg-emerald-50 text-emerald-700',
  },
}

// ── Full style presets (style library / panel) ────────────────────────────────

export const VIDEO_STYLE_PRESETS: VideoStylePreset[] = [
  {
    key: 'anime-2d',
    label: '2维动漫',
    desc: '平面线条、番剧感叙事和二维角色演出更强。',
    tags: ['番剧', '角色演出', '线条'],
    category: '动画',
    modeTags: ['2维动漫'],
    families: ['动画', '剧情'],
    recommendedModels: ['wan', 'hubagi-TC-GV'],
    bestFor: '二次元剧情、角色连续表演',
    texture: '干净线条 / 平面上色',
  },
  {
    key: 'anime-3d',
    label: '3维动漫',
    desc: '更强调体积感、材质和三渲二式镜头层次。',
    tags: ['3D', '体积感', '材质'],
    category: '动画',
    modeTags: ['3维动漫'],
    families: ['动画', '3D'],
    recommendedModels: ['hubagi-TC-GV', 'wan'],
    bestFor: 'CG 动漫、角色动作和空间调度',
    texture: '体积光 / 三维材质',
  },
  {
    key: 'live-action-film',
    label: '真人电影',
    desc: '写实电影光影、真实场景和更强银幕感。',
    tags: ['电影感', '写实', '大片'],
    category: '真人',
    modeTags: ['真人电影'],
    families: ['真人', '电影'],
    recommendedModels: ['sora2', 'hubagi-voe3.1'],
    bestFor: '写实大片、宏大场景、电影化镜头',
    texture: '电影光影 / 真实场景',
  },
  {
    key: 'live-action-short',
    label: '真人短剧',
    desc: '更贴近短剧叙事、人物对白和近景情绪推进。',
    tags: ['短剧', '对白', '人物表演'],
    category: '真人',
    modeTags: ['真人短剧'],
    families: ['真人', '短剧'],
    recommendedModels: ['hubagi-voe3.1', 'sora2'],
    bestFor: '对白戏、情绪戏、人物关系推进',
    texture: '近景表演 / 真实人物',
  },
]

export const VIDEO_STYLE_FILTERS = ['全部', '2维动漫', '3维动漫', '真人电影', '真人短剧', '动画', '真人'] as const

// ── Model selection hints ─────────────────────────────────────────────────────

export const VIDEO_MODEL_SELECTION_HINTS: Record<string, string> = {
  'comfyui-video': '本地 ComfyUI 图生视频，适合已部署本地视频工作流时稳定出片',
  wan: '阿里云 wanx2.1-i2v-turbo，动漫叙事首选，角色连贯性好',
  kling: '快手可灵 v3.0 直连通道，运动流畅，角色表现力强，支持首尾帧与融合生视频',
  aiping: '快手可灵 K3 高并发通道（via aiping.cn），与 kling 能力相同，适合批量出片',
  'tencent-vclm': '腾讯 VCLM 可灵直连（TC3-HMAC-SHA256），图生视频稳定，速度优先',
  'hubagi-TC-GV': '⚠️ 已停用：HubAGI余额不足，TC-GV视频无法生成',
  'hubagi-voe3.1': '⚠️ 已停用：HubAGI/VoE余额不足，voe3.1视频无法生成',
  sora2: '⚠️ 已停用：Sora-2代理无分组权限（503 no channel）',
  doubao: '字节跳动豆包 V4.0 星光渠道，支持首尾帧、融合生视频与原生音频',
  'doubao-seedance': '字节跳动 Seedance 1.5-Pro 星图渠道，T2V/I2V，支持首尾帧与原生环境音频',
  vidu: '生数科技 Vidu Q3 Pro 星成渠道，动作流畅，支持首尾帧与融合生视频',
  'vidu-mix': '生数科技 Vidu Q3 Mix 星辰渠道，更快速，支持融合生视频',
  'vidu-offpeak': 'Vidu Q3 Pro 低峰时段key，能力同 vidu，低峰时段成本更低',
  'vidu-mix-offpeak': 'Vidu Q3 Mix 低峰时段key，速度快，低峰时段成本更低',
  suanneng: '算能 SophNet Seedance 1.5 Pro 星光渠道，兼容豆包 ARK 接口',
  gaga: 'Gaga-1 星点渠道，需先上传资产取 id，适合快速测试',
  'baidu-bce': '百度 BCE 图生视频，BCE-AUTH-V1签名，V20=720p，稳定出片',
}

// ── Generation presets (canonical model + style + motion combos) ──────────────

export const VIDEO_GENERATION_PRESETS: VideoGenerationPreset[] = [
  { key: 'anime-2d', label: '2维动漫', hint: '适合番剧叙事、稳定角色和二维线条演出', model: 'wan', style: 'anime-2d', motion: 'gentle', tone: '2维动漫' },
  { key: 'anime-3d', label: '3维动漫', hint: '适合更强体积感、材质和三渲二镜头层次', model: 'doubao', style: 'anime-3d', motion: 'dynamic', tone: '3维动漫' },
  { key: 'live-action-film', label: '真人电影', hint: '适合写实电影感、真实场景和宏大镜头', model: 'doubao-seedance', style: 'live-action-film', motion: 'cinematic', tone: '真人电影' },
  { key: 'live-action-short', label: '真人短剧', hint: '适合对白戏、人物表演和近景情绪推进', model: 'vidu', style: 'live-action-short', motion: 'gentle', tone: '真人短剧' },
]

// ── Video model capabilities ──────────────────────────────────────────────────

export const VIDEO_MODEL_OPTIONS_VT: VideoModelCapability[] = [
  {
    key: 'comfyui-video',
    label: 'ComfyUI Video',
    icon: '🖥️',
    desc: '本地图生视频 · 自建工作流，无网络限制，无费用',
    provider: '本地部署 / ComfyUI',
    audioSupport: 'dubbed',
    aspectRatio: 'unsupported',
    resolution: 'unsupported',
    multiVariant: 'unsupported',
    clipDuration: '由本地工作流决定，建议与配音时长保持一致',
    note: '适合接本地 Wan、AnimateDiff 或自定义视频节点；需先在服务端配置 ComfyUI URL 与工作流模板。',
    tags: ['本地', '无费用', '自定义工作流'],
    speed: 'fast',
    quality: 'standard',
    bestFor: ['anime-2d', 'anime-3d'],
    supportsStartEnd: false,
    supportsReference: false,
  },
  {
    key: 'wan',
    label: 'wanx2.1-i2v-turbo',
    icon: '🎬',
    desc: '二维动漫叙事首选，角色连贯性好 · 图生视频',
    provider: '阿里云 DashScope · Wan2.1 Turbo',
    audioSupport: 'dubbed',
    aspectRatio: 'unsupported',
    resolution: 'unsupported',
    multiVariant: 'unsupported',
    clipDuration: '当前接入固定单段约 5 秒',
    note: '支持生成后自动合成项目配音；当前接入未开放宽高比、分辨率与多候选配置。',
    tags: ['动漫', '叙事稳定', '角色连贯'],
    speed: 'medium',
    quality: 'standard',
    bestFor: ['anime-2d'],
    supportsStartEnd: false,
    supportsReference: true,
  },
  {
    key: 'kling',
    label: 'Kling v3.0',
    icon: '🎞️',
    desc: '可灵图生视频 v3.0，运动流畅，人物连贯性强',
    provider: '快手 Kling · v3.0 官方直连',
    audioSupport: 'dubbed',
    aspectRatio: 'supported',
    resolution: 'unsupported',
    multiVariant: 'unsupported',
    clipDuration: '单段约 5 秒',
    note: '可灵 v3.0 直连，支持宽高比（16:9/9:16/1:1）与标准/专业模式；角色动作连贯性好。支持首尾帧生视频与融合生视频（Omni 模式）。',
    tags: ['动漫', '真人', '角色连贯', '首尾帧'],
    speed: 'medium',
    quality: 'high',
    bestFor: ['anime-2d', 'live-action-short'],
    supportsStartEnd: true,
    supportsReference: true,
  },
  {
    key: 'aiping',
    label: 'Kling K3 高并发',
    icon: '🎞️',
    desc: '可灵 K3 高并发代理通道，适合批量出片',
    provider: 'aiping.cn · 快手 Kling K3 代理',
    audioSupport: 'dubbed',
    aspectRatio: 'supported',
    resolution: 'unsupported',
    multiVariant: 'unsupported',
    clipDuration: '单段约 5 秒',
    note: '可灵 K3 高并发通道，与 kling 共享参数支持（宽高比、模式），适合批量生产场景。支持首尾帧生视频。',
    tags: ['动漫', '真人', '批量生产', '高并发'],
    speed: 'fast',
    quality: 'high',
    bestFor: ['anime-2d', 'live-action-short'],
    supportsStartEnd: true,
    supportsReference: false,
  },
  {
    key: 'hubagi-TC-GV',
    label: '可灵直连 (VCLM)',
    icon: '🚀',
    desc: '高速出片 · 腾讯 VCLM 可灵直连，三维动漫与快速试片首选',
    provider: '腾讯 VCLM · via hubagi.cn',
    audioSupport: 'dubbed',
    aspectRatio: 'unsupported',
    resolution: 'unsupported',
    multiVariant: 'unsupported',
    clipDuration: '当前接入默认单段约 5 秒',
    note: '腾讯 VCLM 可灵直连（vclm.tencentcloudapi.com），生成速度快，适合三渲二风格与量产试片。支持首尾帧生视频。',
    tags: ['快速', '3D动漫', '量产适合'],
    speed: 'fast',
    quality: 'standard',
    bestFor: ['anime-3d'],
    supportsStartEnd: true,
    supportsReference: false,
  },
  {
    key: 'hubagi-voe3.1',
    label: 'Veo 3.1',
    icon: '⚡',
    desc: '真人短剧/人物表演 · 动作真实感与情绪表现力强',
    provider: 'Google Veo 3.1 · via hubagi.cn',
    audioSupport: 'dubbed',
    aspectRatio: 'unsupported',
    resolution: 'unsupported',
    multiVariant: 'unsupported',
    clipDuration: '当前接入默认单段约 5 秒',
    note: '当前接入聚焦单图单视频结果，不开放候选数量与画幅参数；人物表演和情绪表现力强。',
    tags: ['真人短剧', '人物表演', '情绪感强'],
    speed: 'medium',
    quality: 'high',
    bestFor: ['live-action-short'],
    supportsStartEnd: false,
    supportsReference: false,
  },
  {
    key: 'sora2',
    label: 'Sora 2',
    icon: '🎥',
    desc: '电影级质感 · 写实宏大场面首选，片段时长最长',
    provider: 'OpenAI · Sora 2',
    audioSupport: 'dubbed',
    aspectRatio: 'supported',
    resolution: 'supported',
    multiVariant: 'supported',
    clipDuration: '当前接入单段展示约 10 秒',
    note: '支持宽高比与分辨率（480p/720p/1080p）选择及多候选结果；时长最长，适合大场面与史诗镜头。',
    tags: ['电影感', '写实', '史诗场面', '长片段'],
    speed: 'slow',
    quality: 'high',
    bestFor: ['live-action-film'],
    supportsStartEnd: false,
    supportsReference: false,
  },
  {
    key: 'doubao',
    label: '豆包 V4.0 (星光)',
    icon: '🎯',
    desc: '字节跳动豆包 V4.0 — 支持首尾帧、融合生视频与原生音频',
    provider: '字节跳动 ByteDance · Doubao V4.0',
    audioSupport: 'native',
    aspectRatio: 'supported',
    resolution: 'supported',
    multiVariant: 'unsupported',
    clipDuration: '单段约 5 秒',
    note: '豆包 V4.0（星光3.0）视频渠道；支持首尾帧生视频、融合生视频（reference_image）与原生音频生成。分辨率支持 pro-720/pro-480/fast-720/fast-480。',
    tags: ['原生音频', '首尾帧', '融合生视频'],
    speed: 'medium',
    quality: 'high',
    bestFor: ['anime-2d', 'live-action-short'],
    supportsStartEnd: true,
    supportsReference: true,
  },
  {
    key: 'doubao-seedance',
    label: 'SeedDream 4.0 (星图)',
    icon: '✨',
    desc: '字节跳动 SeedDream 4.0 — 动态感强，支持原生环境音频',
    provider: '字节跳动 ByteDance · SeedDream 4.0',
    audioSupport: 'native',
    aspectRatio: 'supported',
    resolution: 'unsupported',
    multiVariant: 'unsupported',
    clipDuration: '单段约 5 秒',
    note: '豆包 SeedDream 4.0（星图渠道），支持宽高比选择与原生环境音频一体生成（非语音配音）；动态感强，适合氛围感镜头。支持首尾帧生视频。',
    tags: ['原生音频', '首尾帧', '氛围感'],
    speed: 'medium',
    quality: 'standard',
    bestFor: ['anime-2d', 'anime-3d', 'live-action-short'],
    supportsStartEnd: true,
    supportsReference: false,
  },
  {
    key: 'vidu',
    label: 'Vidu Q3 Pro (星成)',
    icon: '🎭',
    desc: '生数科技 Vidu Q3 Pro — 动作流畅，支持首尾帧与融合生视频',
    provider: '生数科技 Vidu · Q3 Pro',
    audioSupport: 'dubbed',
    aspectRatio: 'supported',
    resolution: 'supported',
    multiVariant: 'unsupported',
    clipDuration: '单段 3～8 秒',
    note: '生数科技 Vidu Q3 Pro（星成2.6渠道），支持宽高比与分辨率（360p/720p/1080p）参数；动作流畅，适合动漫与真实系场景。支持首尾帧生视频与融合生视频。',
    tags: ['首尾帧', '融合生视频', '动作流畅'],
    speed: 'medium',
    quality: 'high',
    bestFor: ['anime-2d', 'anime-3d', 'live-action-short'],
    supportsStartEnd: true,
    supportsReference: true,
  },
  {
    key: 'vidu-mix',
    label: 'Vidu Q3 Mix (星辰)',
    icon: '🎭',
    desc: '生数科技 Vidu Q3 Mix — 更快速，支持融合生视频',
    provider: '生数科技 Vidu · Q3 Mix',
    audioSupport: 'dubbed',
    aspectRatio: 'supported',
    resolution: 'supported',
    multiVariant: 'unsupported',
    clipDuration: '单段 3～8 秒',
    note: '生数科技 Vidu Q3 Mix（星辰3.1渠道），速度快，支持宽高比与分辨率参数；融合生视频效果好。',
    tags: ['融合生视频', '快速', '多风格'],
    speed: 'fast',
    quality: 'standard',
    bestFor: ['anime-2d', 'anime-3d', 'live-action-short'],
    supportsStartEnd: false,
    supportsReference: true,
  },
  {
    key: 'suanneng',
    label: 'Seedance 1.5 Pro (星光)',
    icon: '🌟',
    desc: '算能 SophNet Seedance 1.5 Pro — 兼容豆包 ARK 接口',
    provider: '算能 SophNet · Seedance 1.5 Pro',
    audioSupport: 'dubbed',
    aspectRatio: 'supported',
    resolution: 'unsupported',
    multiVariant: 'unsupported',
    clipDuration: '单段约 5 秒',
    note: 'Sophnet Seedance 1.5 Pro（星光2.5渠道），接口与豆包 ARK 完全兼容，content 数组格式；支持宽高比选择。',
    tags: ['ARK兼容', '叙事稳定'],
    speed: 'medium',
    quality: 'standard',
    bestFor: ['anime-2d', 'anime-3d'],
    supportsStartEnd: false,
    supportsReference: false,
  },
  {
    key: 'gaga',
    label: 'Gaga-1 (星点)',
    icon: '🎪',
    desc: 'Gaga 图生视频 — 需先上传资产取 id，适合快速测试',
    provider: 'Gaga · Gaga-1',
    audioSupport: 'dubbed',
    aspectRatio: 'supported',
    resolution: 'unsupported',
    multiVariant: 'unsupported',
    clipDuration: '单段约 5 秒',
    note: 'Gaga-1（星点2.0渠道），需先 POST /v1/assets 上传图片取得 Long 类型 asset_id，再传入 source.content（不能传 URL 字符串）；支持宽高比选择。',
    tags: ['快速测试', '低成本'],
    speed: 'fast',
    quality: 'standard',
    bestFor: ['anime-2d'],
    supportsStartEnd: false,
    supportsReference: false,
  },
]
