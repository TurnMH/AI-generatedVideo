/**
 * model-display.ts
 * Adapters that convert backend Model objects to UI display shapes used by
 * resource generation, storyboard, and video tabs.
 *
 * The backend Model list is the source of truth for WHICH models are shown.
 * This file provides icon / default-prompt / capability extras that are not
 * stored in the database.
 */

import type { Model } from '@/types'
import type { VideoModelCapability } from './video-style-config'

// ─── Icon lookup per model_key ────────────────────────────────────────────────

const IMAGE_MODEL_ICON_MAP: Record<string, string> = {
  'gpt-image-1': '🤖',
  'gpt-image-1.5': '🤖',
  'dalle': '🎨',
  'dall-e-3': '🎨',
  'cogview-4': '🧠',
  'cogview-3-plus': '🧠',
  'doubao-image': '🎯',
  'doubao-seedream-3-0-t2i': '🎯',
  'wanx2.1-t2i-plus': '🖼️',
  'wanx2.1-t2i-turbo': '⚡',
  'tongyi': '🖼️',
  'wanx-v1': '🖼️',
  'sdxl': '🖥️',
  'qianfan-flux.1-schnell': '⚡',
  'qianfan-stable-diffusion-xl': '🖥️',
  'gemini-3.1-flash-image': '🌟',
  'banana2.1': '🍌',
  'banana2.0': '🍌',
  'xingrong2.5': '⭐',
  'wan2.5-i2i-preview': '🎬',
  'baidu-img': '🔴',
}

const VIDEO_MODEL_ICON_MAP: Record<string, string> = {
  'comfyui-video': '🖥️',
  'wan': '🎬',
  'kling': '🎞️',
  'aiping': '🎞️',
  'tencent-vclm': '📡',
  'hubagi-TC-GV': '🚀',
  'hubagi-voe3.1': '⚡',
  'sora2': '🎥',
  'doubao': '🎯',
  'doubao-seedance': '✨',
  'vidu': '🎭',
  'vidu-mix': '🎭',
  'vidu-offpeak': '🌙',
  'vidu-mix-offpeak': '🌙',
  'suanneng': '🌟',
  'gaga': '🎪',
  'baidu-bce': '🔴',
}

// ─── Default prompts per model_key ────────────────────────────────────────────

const IMAGE_MODEL_DEFAULT_PROMPT_MAP: Record<string, string> = {
  'gpt-image-1': 'ultra-high resolution, photorealistic detail, cinematic lighting, sharp focus, professional concept art, no text, no watermark',
  'dalle': 'masterpiece quality, ultra-detailed, professional concept art, clean background, no text, no watermark',
  'dall-e-3': 'high quality digital art, detailed illustration, vibrant colors, clean composition, no text, no watermark',
  'cogview-4': '高清精细，专业设定图，光影真实，构图清晰，细节丰富，无文字无水印',
  'cogview-3-plus': '精细刻画，高质量原画，构图稳定，色彩丰富，无文字无水印',
  'wanx2.1-t2i-plus': '高清，超精细，专业概念插画，构图完整，光影丰富，无文字无水印',
  'tongyi': '高清，精美插画，古典东方风格，构图精良，无文字无水印',
  'wanx-v1': '高清，精美插画，古典东方风格，构图精良，无文字无水印',
  'wanx2.1-t2i-turbo': '清晰构图，色彩饱满，概念设定图，无文字无水印',
  'sdxl': 'masterpiece, best quality, ultra detailed, sharp focus, 8k resolution, no text, no watermark',
}

// ─── Video model capability extras per model_key ──────────────────────────────

type VideoCapabilityExtras = Partial<
  Pick<VideoModelCapability, 'audioSupport' | 'aspectRatio' | 'resolution' | 'multiVariant' | 'clipDuration' | 'note' | 'bestFor' | 'supportsStartEnd' | 'supportsReference'>
>

const VIDEO_MODEL_CAPABILITY_EXTRAS: Record<string, VideoCapabilityExtras> = {
  'comfyui-video': {
    audioSupport: 'dubbed',
    aspectRatio: 'unsupported',
    resolution: 'unsupported',
    multiVariant: 'unsupported',
    clipDuration: '由本地工作流决定，建议与配音时长保持一致',
    note: '适合接本地 Wan、AnimateDiff 或自定义视频节点；需先在服务端配置 ComfyUI URL 与工作流模板。',
    bestFor: ['anime-2d', 'anime-3d'],
    supportsStartEnd: false,
    supportsReference: false,
  },
  'wan': {
    audioSupport: 'dubbed',
    aspectRatio: 'unsupported',
    resolution: 'unsupported',
    multiVariant: 'unsupported',
    clipDuration: '当前接入固定单段约 5 秒',
    note: '支持生成后自动合成项目配音；当前接入未开放宽高比、分辨率与多候选配置。',
    bestFor: ['anime-2d'],
    supportsStartEnd: false,
    supportsReference: true,
  },
  'kling': {
    audioSupport: 'dubbed',
    aspectRatio: 'supported',
    resolution: 'unsupported',
    multiVariant: 'unsupported',
    clipDuration: '单段约 5 秒',
    note: '可灵视频生成，支持宽高比选择（16:9 / 9:16 / 1:1）与标准/专业模式切换；角色动作与面部连贯性好。支持首尾帧生视频与融合生视频（Omni模式）。',
    bestFor: ['anime-2d', 'live-action-short'],
    supportsStartEnd: true,
    supportsReference: true,
  },
  'aiping': {
    audioSupport: 'dubbed',
    aspectRatio: 'supported',
    resolution: 'unsupported',
    multiVariant: 'unsupported',
    clipDuration: '单段约 5 秒',
    note: '可灵 K3 高并发通道，与 kling 共享参数支持（宽高比、模式），适合批量生产场景。支持首尾帧生视频。',
    bestFor: ['anime-2d', 'live-action-short'],
    supportsStartEnd: true,
    supportsReference: false,
  },
  'hubagi-TC-GV': {
    audioSupport: 'dubbed',
    aspectRatio: 'unsupported',
    resolution: 'unsupported',
    multiVariant: 'unsupported',
    clipDuration: '当前接入默认单段约 5 秒',
    note: '生成速度快，适合三渲二风格与量产试片；音频通过后续配音合成追加。支持首尾帧生视频。',
    bestFor: ['anime-3d'],
    supportsStartEnd: true,
    supportsReference: false,
  },
  'hubagi-voe3.1': {
    audioSupport: 'dubbed',
    aspectRatio: 'unsupported',
    resolution: 'unsupported',
    multiVariant: 'unsupported',
    clipDuration: '当前接入默认单段约 5 秒',
    note: '当前接入聚焦单图单视频结果，不开放候选数量与画幅参数；人物表演和情绪表现力强。',
    bestFor: ['live-action-short'],
    supportsStartEnd: false,
    supportsReference: false,
  },
  'sora2': {
    audioSupport: 'dubbed',
    aspectRatio: 'supported',
    resolution: 'supported',
    multiVariant: 'supported',
    clipDuration: '当前接入单段展示约 10 秒',
    note: '支持宽高比与分辨率（480p/720p/1080p）选择，以及多候选结果；时长最长，适合大场面与史诗镜头。',
    bestFor: ['live-action-film'],
    supportsStartEnd: false,
    supportsReference: false,
  },
  'vidu': {
    audioSupport: 'dubbed',
    aspectRatio: 'supported',
    resolution: 'supported',
    multiVariant: 'unsupported',
    clipDuration: '单段 3～8 秒',
    note: '支持宽高比与分辨率（360p/720p/1080p）参数；动作流畅，适合动漫与真实系场景。支持首尾帧生视频与融合生视频。',
    bestFor: ['anime-2d', 'anime-3d', 'live-action-short'],
    supportsStartEnd: true,
    supportsReference: true,
  },
  'doubao-seedance': {
    audioSupport: 'native',
    aspectRatio: 'supported',
    resolution: 'unsupported',
    multiVariant: 'unsupported',
    clipDuration: '单段约 5 秒',
    note: '豆包 SeedDream 模型，支持宽高比选择与原生环境音频一体生成（非语音配音）；动态感强，适合氛围感镜头。支持首尾帧生视频。',
    bestFor: ['anime-2d', 'anime-3d', 'live-action-short'],
    supportsStartEnd: true,
    supportsReference: false,
  },
  'doubao': {
    audioSupport: 'native',
    aspectRatio: 'supported',
    resolution: 'supported',
    multiVariant: 'unsupported',
    clipDuration: '单段约 5 秒',
    note: '豆包 V4.0（星光）视频通道；支持首尾帧生视频、融合生视频（reference_image）与原生音频生成。分辨率支持 pro-720/pro-480/fast-720/fast-480。',
    bestFor: ['anime-2d', 'live-action-short'],
    supportsStartEnd: true,
    supportsReference: true,
  },
  'tencent-vclm': {
    audioSupport: 'dubbed',
    aspectRatio: 'unsupported',
    resolution: 'supported',
    multiVariant: 'unsupported',
    clipDuration: '单段约 5 秒',
    note: '腾讯 VCLM 直连接口，支持分辨率选择（480p/720p/1080p）；图生视频，稳定性好。',
    bestFor: ['live-action-short', 'live-action-film'],
    supportsStartEnd: false,
    supportsReference: false,
  },
  'suanneng': {
    audioSupport: 'dubbed',
    aspectRatio: 'supported',
    resolution: 'unsupported',
    multiVariant: 'unsupported',
    clipDuration: '单段约 5 秒',
    note: 'Sophnet Seedance 1.5 Pro（星光2.5渠道，算能 SophNet），接口与豆包 ARK 完全兼容，content 数组格式；支持宽高比选择。',
    bestFor: ['anime-2d', 'anime-3d'],
    supportsStartEnd: false,
    supportsReference: false,
  },
  'vidu-mix': {
    audioSupport: 'dubbed',
    aspectRatio: 'supported',
    resolution: 'supported',
    multiVariant: 'unsupported',
    clipDuration: '单段 3～8 秒',
    note: '生数科技 Vidu Q3 Mix（星辰3.1渠道），速度快，支持宽高比与分辨率（360p/720p/1080p）参数；融合生视频效果好。',
    bestFor: ['anime-2d', 'anime-3d', 'live-action-short'],
    supportsStartEnd: false,
    supportsReference: true,
  },
  'vidu-offpeak': {
    audioSupport: 'dubbed',
    aspectRatio: 'supported',
    resolution: 'supported',
    multiVariant: 'unsupported',
    clipDuration: '单段 3～8 秒',
    note: 'Vidu Q3 Pro 低峰时段专用key（vidu_offpeak_key），生成能力与 vidu 完全相同，低峰时段成本更低。',
    bestFor: ['anime-2d', 'anime-3d', 'live-action-short'],
    supportsStartEnd: true,
    supportsReference: true,
  },
  'vidu-mix-offpeak': {
    audioSupport: 'dubbed',
    aspectRatio: 'supported',
    resolution: 'supported',
    multiVariant: 'unsupported',
    clipDuration: '单段 3～8 秒',
    note: 'Vidu Q3 Mix 低峰时段专用key（vidu_offpeak_key），速度快，能力与 vidu-mix 相同，低峰时段成本更低。',
    bestFor: ['anime-2d', 'anime-3d'],
    supportsStartEnd: false,
    supportsReference: true,
  },
  'baidu-bce': {
    audioSupport: 'dubbed',
    aspectRatio: 'supported',
    resolution: 'supported',
    multiVariant: 'unsupported',
    clipDuration: '单段约 5 秒',
    note: '百度 BCE 图生视频，BCE-AUTH-V1 HMAC签名，V20=720p；支持宽高比选择，稳定出片。',
    bestFor: ['live-action-short'],
    supportsStartEnd: false,
    supportsReference: false,
  },
}

// ─── Speed / quality helpers ──────────────────────────────────────────────────

function mapSpeed(rating: string): 'fast' | 'medium' | 'slow' {
  if (rating === 'fast') return 'fast'
  if (rating === 'slow') return 'slow'
  return 'medium'
}

function mapQuality(rating: string, tags: string[]): 'high' | 'standard' {
  if (rating === 'quality') return 'high'
  if (tags.some((t) => t === '高质量' || t === 'high-quality' || t === 'high_quality')) return 'high'
  return 'standard'
}

// ─── Exported types ───────────────────────────────────────────────────────────

export type ImageModelOption = {
  key: string
  label: string
  provider: string
  icon: string
  desc: string
  tags: string[]
  speed: 'fast' | 'medium' | 'slow'
  quality: 'high' | 'standard'
  defaultPrompt: string
  failureReason?: string
}

export type VideoModelOption = {
  key: string
  label: string
  provider: string
  icon: string
  desc: string
  tags: string[]
  speed: 'fast' | 'medium' | 'slow'
  quality: 'high' | 'standard'
  failureReason?: string
}

// ─── Dedup helper ─────────────────────────────────────────────────────────────

/** Remove duplicate models by model_key, keeping the first occurrence. */
export function dedupeModels(models: Model[]): Model[] {
  const seen = new Set<string>()
  return models.filter((m) => {
    if (seen.has(m.model_key)) return false
    seen.add(m.model_key)
    return true
  })
}

// ─── Builders ─────────────────────────────────────────────────────────────────

export function buildImageModelOption(model: Model): ImageModelOption {
  const tags = model.capability_tags ?? []
  return {
    key: model.model_key,
    label: model.name,
    provider: model.provider,
    icon: IMAGE_MODEL_ICON_MAP[model.model_key] ?? '🖼️',
    desc: model.description ?? '',
    tags,
    speed: mapSpeed(model.speed_rating),
    quality: mapQuality(model.speed_rating, tags),
    defaultPrompt:
      (model.config?.defaultPrompt as string | undefined) ??
      IMAGE_MODEL_DEFAULT_PROMPT_MAP[model.model_key] ??
      '',
    failureReason: model.failure_reason,
  }
}

export function buildVideoModelOption(model: Model): VideoModelOption {
  const tags = model.capability_tags ?? []
  return {
    key: model.model_key,
    label: model.name,
    provider: model.provider,
    icon: VIDEO_MODEL_ICON_MAP[model.model_key] ?? '🎬',
    desc: model.description ?? '',
    tags,
    speed: mapSpeed(model.speed_rating),
    quality: mapQuality(model.speed_rating, tags),
    failureReason: model.failure_reason,
  }
}

export function buildVideoModelCapability(model: Model): VideoModelCapability {
  const tags = model.capability_tags ?? []
  const extras = VIDEO_MODEL_CAPABILITY_EXTRAS[model.model_key] ?? {}
  return {
    key: model.model_key,
    label: model.name,
    icon: VIDEO_MODEL_ICON_MAP[model.model_key] ?? '🎬',
    desc: model.description ?? '',
    provider: model.provider,
    audioSupport: extras.audioSupport ?? 'dubbed',
    aspectRatio: extras.aspectRatio ?? 'unsupported',
    resolution: extras.resolution ?? 'unsupported',
    multiVariant: extras.multiVariant ?? 'unsupported',
    clipDuration: extras.clipDuration ?? '',
    note: extras.note ?? '',
    tags,
    speed: mapSpeed(model.speed_rating),
    quality: mapQuality(model.speed_rating, tags),
    bestFor: extras.bestFor ?? [],
    supportsStartEnd: extras.supportsStartEnd ?? false,
    supportsReference: extras.supportsReference ?? false,
  }
}
