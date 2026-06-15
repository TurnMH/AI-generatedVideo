import type { Model } from '@/types'
import { mapVideoModelToRuntimeKey } from '@/lib/projects/models'
import { getClipDurationOptionsForModelFamily } from '@/lib/projects/video-clip-duration'

export type VideoModelParamOption = { value: string; label: string }

export type VideoModelStatusItem = {
  key: string
  label?: string
  available?: boolean
  native_audio?: boolean
  params?: {
    key: string
    label: string
    default?: string
    values?: VideoModelParamOption[]
  }[]
}

export function buildVideoModelStatusMap(items: VideoModelStatusItem[] = []): Map<string, VideoModelStatusItem> {
  return new Map(items.map((item) => [item.key, item]))
}

export function parseVideoModelStatusItems(payload: unknown): VideoModelStatusItem[] {
  const root = payload as {
    data?: { models?: VideoModelStatusItem[] } | VideoModelStatusItem[]
    models?: VideoModelStatusItem[]
  }
  if (Array.isArray(root.models)) return root.models
  if (Array.isArray(root.data)) return root.data
  if (root.data && typeof root.data === 'object' && Array.isArray(root.data.models)) {
    return root.data.models
  }
  return []
}

export function getParamOptionsFromStatus(
  statusMap: Map<string, VideoModelStatusItem>,
  generatorKey: string,
  paramKey: string,
): VideoModelParamOption[] {
  const item = statusMap.get(generatorKey)
  const param = item?.params?.find((entry) => entry.key === paramKey)
  return param?.values ?? []
}

export function resolveVideoGeneratorKey(
  model: Model | undefined,
  statusMap: Map<string, VideoModelStatusItem>,
): string {
  if (!model) return 'default'

  const directCandidates = [
    mapVideoModelToRuntimeKey(model),
    model.model_key?.trim(),
    typeof (model as { runtime_alias?: string }).runtime_alias === 'string'
      ? (model as { runtime_alias?: string }).runtime_alias!.trim()
      : '',
  ].filter(Boolean) as string[]

  for (const candidate of directCandidates) {
    if (statusMap.has(candidate)) return candidate
  }

  const text = `${model.model_key} ${model.name} ${model.provider}`.toLowerCase()
  for (const [key] of statusMap) {
    if (text.includes(key.toLowerCase())) return key
  }

  const aliasRules: Array<{ pattern: RegExp; keys: string[] }> = [
    { pattern: /kling|aiping|vclm|tencent-vclm|hubagi-tc/, keys: ['kling', 'aiping', 'tencent-vclm', 'hubagi-TC-GV'] },
    { pattern: /vidu/, keys: ['vidu', 'vidu-mix', 'vidu-offpeak', 'vidu-mix-offpeak'] },
    { pattern: /veo|voe3\.1|voe/, keys: ['hubagi-voe3.1'] },
    { pattern: /sora/, keys: ['sora2'] },
    { pattern: /doubao|seedance/, keys: ['doubao', 'doubao-seedance'] },
    { pattern: /suanneng|算能/, keys: ['suanneng'] },
    { pattern: /wan|dashscope|通义/, keys: ['wan', 'wan-t2v'] },
    { pattern: /comfyui/, keys: ['comfyui-video'] },
    { pattern: /minmax|hailuo|海螺/, keys: ['minmax'] },
    { pattern: /gaga/, keys: ['gaga'] },
    { pattern: /cogvideo/, keys: ['cogvideo'] },
    { pattern: /baidu|bce/, keys: ['baidu-bce'] },
  ]

  for (const rule of aliasRules) {
    if (!rule.pattern.test(text)) continue
    for (const key of rule.keys) {
      if (statusMap.has(key)) return key
    }
  }

  return directCandidates[0] || 'default'
}

export function resolveClipDurationOptions(
  model: Model | undefined,
  statusMap: Map<string, VideoModelStatusItem>,
): number[] {
  const generatorKey = resolveVideoGeneratorKey(model, statusMap)
  const fromApi = getParamOptionsFromStatus(statusMap, generatorKey, 'duration')
    .map((option) => Number(option.value))
    .filter((value) => Number.isFinite(value) && value > 0)

  if (fromApi.length > 0) {
    return Array.from(new Set(fromApi)).sort((a, b) => a - b)
  }

  return getClipDurationOptionsForModelFamily(generatorKey)
}

export function snapDurationToSupportedOptions(duration: number, options: number[]): number {
  if (!options.length) return Math.max(1, duration)
  if (options.includes(duration)) return duration
  return options.reduce((best, candidate) =>
    Math.abs(candidate - duration) < Math.abs(best - duration) ? candidate : best,
  options[0])
}

export function formatClipDurationOptionsHint(
  model: Model | undefined,
  statusMap: Map<string, VideoModelStatusItem>,
  options: number[],
): string | undefined {
  if (!model || options.length === 0) return undefined
  const generatorKey = resolveVideoGeneratorKey(model, statusMap)
  const statusItem = statusMap.get(generatorKey)
  const param = statusItem?.params?.find((entry) => entry.key === 'duration')
  const labels = (param?.values ?? [])
    .filter((entry) => options.includes(Number(entry.value)))
    .map((entry) => entry.label || `${entry.value}秒`)
  const modelLabel = statusItem?.label?.trim() || model.name
  if (labels.length > 0) {
    return `当前视频模型「${modelLabel}」支持：${labels.join(' / ')}`
  }
  return `当前视频模型「${modelLabel}」支持：${options.map((value) => `${value}秒`).join(' / ')}`
}
