import { modelAPI } from '@/lib/api'
import type { Model } from '@/types'

type ModelHealth = 'healthy' | 'unhealthy' | 'unknown'

function getHealthRank(model: Model, healthMap?: Record<string, ModelHealth>): number {
  const health = healthMap?.[model.name] ?? model.health_status ?? 'unknown'
  if (!model.is_active) return 3
  if (health === 'healthy') return 0
  if (health === 'unknown') return 1
  return 2
}

export function pickPreferredModel(
  models: Model[],
  healthMap?: Record<string, ModelHealth>
): Model | undefined {
  if (models.length === 0) return undefined

  return [...models].sort((left, right) => {
    const healthDelta = getHealthRank(left, healthMap) - getHealthRank(right, healthMap)
    if (healthDelta !== 0) return healthDelta
    if (left.is_default !== right.is_default) return left.is_default ? -1 : 1
    if (left.priority !== right.priority) return left.priority - right.priority
    return left.name.localeCompare(right.name, 'zh-CN')
  })[0]
}

export async function fetchModelIdentity(modelId?: number): Promise<{
  modelKey?: string
  modelLabel?: string
}> {
  if (!modelId) return {}

  try {
    const res = await modelAPI.get(modelId) as unknown as { data?: Model }
    const model = res?.data
    const modelKey = typeof model?.model_key === 'string' ? model.model_key.trim() : ''
    const modelLabel = typeof model?.name === 'string' ? model.name.trim() : ''

    return {
      modelKey: modelKey || undefined,
      modelLabel: modelLabel || undefined,
    }
  } catch {
    return {}
  }
}
