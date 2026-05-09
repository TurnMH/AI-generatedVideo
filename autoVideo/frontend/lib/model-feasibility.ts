import type { APIKey, Model, ModelType, SystemAPIKey } from '@/types'
import type { IntegrationStatus, OfficialModelCatalogItem } from '@/lib/official-model-catalog'

export type ValidationStatus = 'verified' | 'ready' | 'partial' | 'blocked'

export interface ChannelCoverage {
  provider: string
  label: string
  scope_match: 'exact' | 'provider'
  kind: 'system' | 'user'
  base_url?: string
}

export interface ModelFeasibilityAssessment {
  item: OfficialModelCatalogItem
  product_key: string
  product_label: string
  status: ValidationStatus
  reasons: string[]
  coverage: ChannelCoverage[]
  has_exact_scope: boolean
  has_provider_coverage: boolean
  is_imported: boolean
  health_status: 'healthy' | 'unhealthy' | 'unknown'
}

export interface ProductFeasibilityAssessment {
  product_key: string
  product_label: string
  provider: string
  provider_label: string
  type: ModelType
  status: ValidationStatus
  model_count: number
  imported_count: number
  verified_count: number
  ready_count: number
  partial_count: number
  blocked_count: number
  native_count: number
  channels: string[]
  models: ModelFeasibilityAssessment[]
}

const CAPABILITY_LABELS: Record<string, string> = {
  'text-generation': '文本生成',
  reasoning: '推理',
  vision: '视觉理解',
  'function-calling': '函数调用',
  classification: '分类',
  extraction: '信息抽取',
  'text-to-image': '文生图',
  'image-to-image': '图生图',
  consistency: '一致性生成',
  'text-to-video': '文生视频',
  'image-to-video': '图生视频',
  'synced-audio': '同步音频',
  'text-to-speech': '文本转语音',
  'speech-to-text': '语音转文本',
  multilingual: '多语种',
  'multi-speaker': '多角色语音',
  'low-latency': '低延迟',
  'json-output': '结构化输出',
}

type APIKeyRecord = APIKey | SystemAPIKey

const PROVIDER_LABELS: Record<string, string> = {
  // Proxy / bridge channels
  easyart: '星启 easyart',
  wcnbai: 'wcnbai',
  aiping: 'aiping.cn · 可灵高并发',
  hubagi: 'hubagi.cn',
  custom: '自定义',
  unknown: '未知来源',
  // International AI
  openai: 'OpenAI',
  anthropic: 'Anthropic',
  deepseek: 'DeepSeek',
  elevenlabs: 'ElevenLabs',
  google: 'Google',
  // Chinese AI / Cloud
  dashscope: '阿里云 DashScope',
  aliyun: '阿里云 DashScope',
  baidu: '百度 Baidu',
  kuaishou: '快手 Kling',
  kling: '快手 Kling',
  bytedance: '字节跳动 ByteDance',
  zhipu: '智谱 GLM',
  tencent: '腾讯 Tencent',
  vidu: '生数科技 Vidu',
  sophnet: '算能 SophNet',
  gaga: 'Gaga',
}

const BRIDGE_SCOPE_ONLY_PROVIDERS = new Set(['easyart', 'wcnbai', 'custom'])

function normalizeModelKey(value?: string | null) {
  return (value ?? '').trim().toLowerCase()
}

function parseModelScope(scope?: string | null) {
  return (scope ?? '')
    .split(',')
    .map((item) => normalizeModelKey(item))
    .filter(Boolean)
}

function providerLabel(provider: string) {
  return PROVIDER_LABELS[provider] ?? provider
}

function keyKindLabel(kind: 'system' | 'user') {
  return kind === 'system' ? '系统渠道' : '用户渠道'
}

function findScopeMatch(key: APIKeyRecord, item: OfficialModelCatalogItem) {
  const scope = parseModelScope(key.model_scope)
  if (scope.length === 0) return false
  const candidates = new Set([normalizeModelKey(item.model_key), normalizeModelKey(item.runtime_alias)])
  for (const candidate of candidates) {
    if (candidate && scope.includes(candidate)) {
      return true
    }
  }
  return false
}

function hasProviderCoverage(key: APIKeyRecord, item: OfficialModelCatalogItem) {
  if (normalizeModelKey(key.provider) === normalizeModelKey(item.provider)) {
    return true
  }

  if (BRIDGE_SCOPE_ONLY_PROVIDERS.has(normalizeModelKey(key.provider))) {
    return false
  }

  return false
}

function deriveProduct(item: OfficialModelCatalogItem) {
  const key = item.model_key.toLowerCase()
  const name = item.name.toLowerCase()

  if (item.provider === 'openai') {
    if (key.startsWith('gpt-5.4')) return { product_key: 'openai-gpt-5.4', product_label: 'GPT-5.4 系列' }
    if (key.startsWith('gpt-5')) return { product_key: 'openai-gpt-5', product_label: 'GPT-5 系列' }
    if (key.startsWith('gpt-4.1') || key.startsWith('gpt-4o')) return { product_key: 'openai-gpt-4.x', product_label: 'GPT-4.x / 4o 系列' }
    if (key.includes('image')) return { product_key: 'openai-gpt-image', product_label: 'GPT Image 系列' }
    if (key.startsWith('sora-2')) return { product_key: 'openai-sora-2', product_label: 'Sora 2 系列' }
    if (key.includes('tts') || key.includes('transcribe') || key.includes('realtime')) {
      return { product_key: 'openai-audio', product_label: 'OpenAI Audio / Realtime' }
    }
    return { product_key: 'openai-other', product_label: 'OpenAI 其他产品' }
  }

  if (item.provider === 'anthropic') {
    if (name.includes('opus 4.6')) return { product_key: 'anthropic-opus-4.6', product_label: 'Claude Opus 4.6' }
    if (name.includes('sonnet 4.6')) return { product_key: 'anthropic-sonnet-4.6', product_label: 'Claude Sonnet 4.6' }
    if (name.includes('haiku 4.5')) return { product_key: 'anthropic-haiku-4.5', product_label: 'Claude Haiku 4.5' }
    if (name.includes('opus 4') || name.includes('sonnet 4')) return { product_key: 'anthropic-claude-4', product_label: 'Claude 4 历史快照' }
    if (name.includes('3.7')) return { product_key: 'anthropic-claude-3.7', product_label: 'Claude 3.7 系列' }
    if (name.includes('3.5')) return { product_key: 'anthropic-claude-3.5', product_label: 'Claude 3.5 系列' }
    return { product_key: 'anthropic-legacy', product_label: 'Claude 其他历史系列' }
  }

  if (item.provider === 'deepseek') {
    if (key.includes('reasoner')) return { product_key: 'deepseek-reasoner', product_label: 'DeepSeek Reasoner' }
    return { product_key: 'deepseek-chat', product_label: 'DeepSeek Chat' }
  }

  if (item.provider === 'dashscope') {
    if (key.startsWith('qwen3')) return { product_key: 'dashscope-qwen3', product_label: 'Qwen3 系列' }
    if (key.startsWith('qwen-max')) return { product_key: 'dashscope-qwen-max', product_label: 'Qwen Max 系列' }
    if (key.startsWith('qwen-turbo')) return { product_key: 'dashscope-qwen-turbo', product_label: 'Qwen Turbo 系列' }
    if (key.startsWith('qvq') || key.includes('vl')) return { product_key: 'dashscope-qwen-vision', product_label: 'Qwen 视觉系列' }
    if (key.includes('omni')) return { product_key: 'dashscope-qwen-omni', product_label: 'Qwen Omni 系列' }
    if (key.startsWith('wan') || key.startsWith('wanx')) return { product_key: 'dashscope-wanx', product_label: 'Wan / WanX 系列' }
    if (key.startsWith('cosyvoice')) return { product_key: 'dashscope-cosyvoice', product_label: 'CosyVoice 系列' }
    if (key.startsWith('sambert')) return { product_key: 'dashscope-sambert', product_label: 'Sambert 系列' }
    if (key.startsWith('paraformer')) return { product_key: 'dashscope-paraformer', product_label: 'Paraformer 系列' }
    return { product_key: 'dashscope-other', product_label: 'DashScope 其他产品' }
  }

  if (item.provider === 'elevenlabs') {
    if (key.includes('flash')) return { product_key: 'elevenlabs-flash', product_label: 'Eleven Flash 系列' }
    if (key.includes('turbo')) return { product_key: 'elevenlabs-turbo', product_label: 'Eleven Turbo 系列' }
    if (key.includes('multilingual')) return { product_key: 'elevenlabs-multilingual', product_label: 'Eleven Multilingual 系列' }
    return { product_key: 'elevenlabs-v3', product_label: 'Eleven v3 系列' }
  }

  if (item.provider === 'kling') {
    if (key.includes('2.1')) return { product_key: 'kling-2.1', product_label: 'Kling 2.1' }
    return { product_key: 'kling-1.6', product_label: 'Kling 1.6' }
  }

  return { product_key: `${item.provider}-${item.model_key}`, product_label: item.name }
}

function integrationReason(status: IntegrationStatus) {
  switch (status) {
    case 'native':
      return '仓库内已存在明确调用路径。'
    case 'config_only':
      return '当前可入库管理，但业务链路尚未完全按 model_key 消费。'
    case 'catalog_only':
      return '官方目录已收录，但项目还缺少对应适配器。'
    default:
      return '接入状态未知。'
  }
}

function deriveStatus(item: OfficialModelCatalogItem, coverage: ChannelCoverage[], imported: boolean, healthStatus: 'healthy' | 'unhealthy' | 'unknown') {
  const hasExactScope = coverage.some((entry) => entry.scope_match === 'exact')
  const hasProviderCoverage = coverage.some((entry) => entry.scope_match === 'provider')

  if (imported && healthStatus === 'healthy') {
    return { status: 'verified' as const, hasExactScope, hasProviderCoverage }
  }

  if (item.integration_status === 'native' && (hasExactScope || hasProviderCoverage)) {
    return { status: 'ready' as const, hasExactScope, hasProviderCoverage }
  }

  if (imported || hasExactScope || hasProviderCoverage) {
    return { status: 'partial' as const, hasExactScope, hasProviderCoverage }
  }

  return { status: 'blocked' as const, hasExactScope, hasProviderCoverage }
}

function collectCoverage(item: OfficialModelCatalogItem, systemKeys: SystemAPIKey[], userKeys: APIKey[]) {
  const coverage: ChannelCoverage[] = []

  for (const key of systemKeys) {
    if (!key.is_active) continue
    if (findScopeMatch(key, item)) {
      coverage.push({
        provider: key.provider,
        label: `${providerLabel(key.provider)} · ${keyKindLabel('system')}`,
        scope_match: 'exact',
        kind: 'system',
        base_url: key.base_url,
      })
      continue
    }
    if (hasProviderCoverage(key, item)) {
      coverage.push({
        provider: key.provider,
        label: `${providerLabel(key.provider)} · ${keyKindLabel('system')}`,
        scope_match: 'provider',
        kind: 'system',
        base_url: key.base_url,
      })
    }
  }

  for (const key of userKeys) {
    if (!key.is_active) continue
    if (findScopeMatch(key, item)) {
      coverage.push({
        provider: key.provider,
        label: `${providerLabel(key.provider)} · ${keyKindLabel('user')}`,
        scope_match: 'exact',
        kind: 'user',
        base_url: key.base_url,
      })
      continue
    }
    if (hasProviderCoverage(key, item)) {
      coverage.push({
        provider: key.provider,
        label: `${providerLabel(key.provider)} · ${keyKindLabel('user')}`,
        scope_match: 'provider',
        kind: 'user',
        base_url: key.base_url,
      })
    }
  }

  return coverage
}

function unique<T>(items: T[]) {
  return Array.from(new Set(items))
}

function buildReasons(item: OfficialModelCatalogItem, status: ValidationStatus, coverage: ChannelCoverage[], imported: boolean, healthStatus: 'healthy' | 'unhealthy' | 'unknown') {
  const reasons = [integrationReason(item.integration_status)]

  const exactCoverage = coverage.filter((entry) => entry.scope_match === 'exact').map((entry) => entry.label)
  if (exactCoverage.length > 0) {
    reasons.push(`已在渠道模型范围中命中：${unique(exactCoverage).join('、')}。`)
  }

  const providerCoverage = coverage.filter((entry) => entry.scope_match === 'provider').map((entry) => entry.label)
  if (providerCoverage.length > 0) {
    reasons.push(`当前存在同供应商渠道：${unique(providerCoverage).join('、')}。`)
  }

  if (imported) {
    reasons.push('模型已入库，可直接参与当前平台模型管理。')
  }

  if (healthStatus === 'healthy') {
    reasons.push('已存在健康检查通过记录。')
  } else if (healthStatus === 'unhealthy') {
    reasons.push('已入库，但最近一次健康检查失败。')
  } else if (imported) {
    reasons.push('已入库，但暂无健康检查结论。')
  }

  if (status === 'blocked') {
    reasons.push('当前项目看不到对应可用渠道或运行实例，落地前需补充配置/适配。')
  }

  return reasons
}

export function getValidationStatusMeta(status: ValidationStatus) {
  switch (status) {
    case 'verified':
      return { label: '已验证', color: 'bg-emerald-100 text-emerald-800' }
    case 'ready':
      return { label: '可接入', color: 'bg-blue-100 text-blue-800' }
    case 'partial':
      return { label: '部分可行', color: 'bg-amber-100 text-amber-800' }
    case 'blocked':
      return { label: '待补齐', color: 'bg-zinc-100 text-zinc-700' }
    default:
      return { label: '未知', color: 'bg-zinc-100 text-zinc-700' }
  }
}

export function buildModelFeasibilityAssessments(params: {
  catalog: OfficialModelCatalogItem[]
  runtimeModels: Model[]
  healthMap: Record<string, 'healthy' | 'unhealthy' | 'unknown'>
  systemKeys: SystemAPIKey[]
  userKeys: APIKey[]
}) {
  const { catalog, runtimeModels, healthMap, systemKeys, userKeys } = params

  return catalog.map((item) => {
    const { product_key, product_label } = deriveProduct(item)
    const existing = runtimeModels.find((model) => model.provider === item.provider && model.model_key === item.model_key)
    const healthStatus = existing ? (healthMap[existing.name] ?? existing.health_status ?? 'unknown') : 'unknown'
    const coverage = collectCoverage(item, systemKeys, userKeys)
    const { status, hasExactScope, hasProviderCoverage } = deriveStatus(item, coverage, Boolean(existing), healthStatus)

    return {
      item,
      product_key,
      product_label,
      status,
      reasons: buildReasons(item, status, coverage, Boolean(existing), healthStatus),
      coverage,
      has_exact_scope: hasExactScope,
      has_provider_coverage: hasProviderCoverage,
      is_imported: Boolean(existing),
      health_status: healthStatus,
    } satisfies ModelFeasibilityAssessment
  })
}

function aggregateProductStatus(models: ModelFeasibilityAssessment[]): ValidationStatus {
  const total = models.length
  const verified = models.filter((item) => item.status === 'verified').length
  const ready = models.filter((item) => item.status === 'ready').length
  const blocked = models.filter((item) => item.status === 'blocked').length

  if (verified === total) return 'verified'
  if (blocked === total) return 'blocked'
  if (verified + ready === total) return 'ready'
  return 'partial'
}

export function buildProductFeasibilityAssessments(models: ModelFeasibilityAssessment[]) {
  const grouped = new Map<string, ProductFeasibilityAssessment>()

  for (const model of models) {
    const existing = grouped.get(model.product_key)
    if (!existing) {
      grouped.set(model.product_key, {
        product_key: model.product_key,
        product_label: model.product_label,
        provider: model.item.provider,
        provider_label: model.item.provider_label,
        type: model.item.type,
        status: model.status,
        model_count: 1,
        imported_count: model.is_imported ? 1 : 0,
        verified_count: model.status === 'verified' ? 1 : 0,
        ready_count: model.status === 'ready' ? 1 : 0,
        partial_count: model.status === 'partial' ? 1 : 0,
        blocked_count: model.status === 'blocked' ? 1 : 0,
        native_count: model.item.integration_status === 'native' ? 1 : 0,
        channels: unique(model.coverage.map((entry) => entry.label)),
        models: [model],
      })
      continue
    }

    existing.model_count += 1
    existing.imported_count += model.is_imported ? 1 : 0
    existing.verified_count += model.status === 'verified' ? 1 : 0
    existing.ready_count += model.status === 'ready' ? 1 : 0
    existing.partial_count += model.status === 'partial' ? 1 : 0
    existing.blocked_count += model.status === 'blocked' ? 1 : 0
    existing.native_count += model.item.integration_status === 'native' ? 1 : 0
    existing.channels = unique([...existing.channels, ...model.coverage.map((entry) => entry.label)])
    existing.models.push(model)
  }

  return Array.from(grouped.values())
    .map((entry) => ({
      ...entry,
      status: aggregateProductStatus(entry.models),
      channels: entry.channels.sort((left, right) => left.localeCompare(right, 'zh-CN')),
      models: [...entry.models].sort((left, right) => left.item.name.localeCompare(right.item.name, 'zh-CN')),
    }))
    .sort((left, right) => {
      if (left.status !== right.status) {
        const order = { verified: 0, ready: 1, partial: 2, blocked: 3 }
        return order[left.status] - order[right.status]
      }
      if (left.provider_label !== right.provider_label) {
        return left.provider_label.localeCompare(right.provider_label, 'zh-CN')
      }
      return left.product_label.localeCompare(right.product_label, 'zh-CN')
    })
}

export function getProviderLabel(provider: string) {
  return providerLabel(provider)
}

export function getModelCapabilityLabels(item: OfficialModelCatalogItem) {
  const labels = item.capability_tags
    .map((tag) => CAPABILITY_LABELS[tag] ?? tag)
    .filter(Boolean)

  if (item.type === 'image' && labels.length === 0) labels.unshift('图像生成')
  if (item.type === 'video' && labels.length === 0) labels.unshift('视频生成')
  if (item.type === 'audio' && labels.length === 0) labels.unshift('语音生成')
  if (item.type === 'llm' && labels.length === 0) labels.unshift('文本生成')

  return unique(labels)
}

export function getRuntimeModelCapabilityLabels(model: Pick<Model, 'type' | 'capability_tags'>) {
  const labels = (model.capability_tags ?? [])
    .map((tag) => CAPABILITY_LABELS[tag] ?? tag)
    .filter(Boolean)

  if (model.type === 'image' && labels.length === 0) labels.unshift('图像生成')
  if (model.type === 'video' && labels.length === 0) labels.unshift('视频生成')
  if (model.type === 'audio' && labels.length === 0) labels.unshift('语音生成')
  if (model.type === 'llm' && labels.length === 0) labels.unshift('文本生成')

  return unique(labels)
}

export function getAssessmentAvailabilityLabels(assessment: ModelFeasibilityAssessment) {
  const labels: string[] = []

  if (assessment.item.integration_status === 'native') labels.push('原生链路')
  if (assessment.is_imported) labels.push('已入库')
  if (assessment.health_status === 'healthy') labels.push('健康通过')
  if (assessment.health_status === 'unhealthy') labels.push('健康异常')
  if (assessment.has_exact_scope) labels.push('精确命中渠道')
  else if (assessment.has_provider_coverage) labels.push('同供应商可用')
  if (assessment.item.runtime_alias) labels.push(`运行别名 ${assessment.item.runtime_alias}`)

  return unique(labels)
}

export function getProductCapabilityLabels(assessment: ProductFeasibilityAssessment) {
  return unique(
    assessment.models.flatMap((model) => getModelCapabilityLabels(model.item))
  )
}

export function getProductAvailabilityLabels(assessment: ProductFeasibilityAssessment) {
  const labels: string[] = []

  if (assessment.verified_count > 0) labels.push(`已验证 ${assessment.verified_count}`)
  if (assessment.native_count > 0) labels.push(`原生 ${assessment.native_count}`)
  if (assessment.imported_count > 0) labels.push(`已入库 ${assessment.imported_count}`)
  if (assessment.channels.length > 0) labels.push(`命中渠道 ${assessment.channels.length}`)
  if (assessment.blocked_count > 0) labels.push(`待补齐 ${assessment.blocked_count}`)

  return unique(labels)
}
