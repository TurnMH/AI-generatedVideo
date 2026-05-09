import type { Model } from '@/types'
import { getProviderLabel, getRuntimeModelCapabilityLabels } from '@/lib/model-feasibility'

export function getSplitModelRemark(model: Model): string {
  if (model.description?.trim()) return model.description.trim()

  const notes: string[] = []
  const capabilities = new Set(getRuntimeModelCapabilityLabels(model))

  if ((model.context_window ?? 0) >= 64000) notes.push('支持长上下文，适合长剧本理解')
  if (capabilities.has('推理')) notes.push('擅长剧情推理与节奏判断')
  if (capabilities.has('信息抽取') || capabilities.has('结构化输出')) notes.push('适合结构化分集与关键词抽取')
  if (capabilities.has('视觉理解')) notes.push('可辅助处理图文混合输入')

  if (notes.length === 0) {
    notes.push('适合剧本理解、分集拆解与关键词辅助分析')
  }

  return notes.join('；')
}

export function buildSplitModelSearchText(model: Model): string {
  const capabilities = getRuntimeModelCapabilityLabels(model)
  const keywords: string[] = [
    model.name,
    model.model_key,
    model.provider,
    getProviderLabel(model.provider),
    model.description ?? '',
    ...capabilities,
    getSplitModelRemark(model),
    '剧本',
    '分集',
    '拆解',
    '剧情理解',
    '关键词',
  ]

  if ((model.context_window ?? 0) >= 64000) {
    keywords.push('长上下文', '长剧本', '长文本')
  }
  if (capabilities.includes('推理')) {
    keywords.push('思考', '分析')
  }
  if (capabilities.includes('信息抽取') || capabilities.includes('结构化输出')) {
    keywords.push('结构化', '抽取')
  }

  return keywords.join(' ').toLocaleLowerCase()
}

export function getSplitModelAvailabilityRank(model: Model, health: 'healthy' | 'unhealthy' | 'unknown') {
  if (!model.is_active) return 3
  if (health === 'healthy') return 0
  if (health === 'unknown') return 1
  if (health === 'unhealthy') return 2
  return 3
}

export function mapVideoModelToRuntimeKey(model?: Model): string | null {
  if (!model) return null
  const runtimeAlias = typeof (model as { runtime_alias?: string }).runtime_alias === 'string'
    ? (model as { runtime_alias?: string }).runtime_alias!.toLowerCase()
    : ''
  const text = `${model.model_key} ${model.name} ${model.provider} ${runtimeAlias}`.toLowerCase()
  if (text.includes('comfyui')) return 'comfyui-video'
  if (text.includes('sora')) return 'sora2'
  if (text.includes('veo') || text.includes('voe3.1')) return 'hubagi-voe3.1'
  if (text.includes('tc-gv')) return 'hubagi-TC-GV'
  if (text.includes('wan') || text.includes('dashscope')) return 'wan'
  return null
}

export function findPreferredVideoModelId(models: Model[], preferredRuntimeKeys: readonly string[]): number | undefined {
  for (const key of preferredRuntimeKeys) {
    const m = models.find((mm) => mapVideoModelToRuntimeKey(mm) === key)
    if (m) return m.id
  }
  return undefined
}
