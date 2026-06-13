import type { Model } from '@/types'

export function getProjectModelAvailability(
  model: Model,
  textModelHealthMap: Record<string, 'healthy' | 'unhealthy' | 'unknown'>,
) {
  const health = textModelHealthMap[model.name] ?? model.health_status ?? 'unknown'
  if (!model.is_active) return { label: '未启用', color: 'bg-zinc-100 text-zinc-700' }
  if (health === 'healthy') return { label: '可用', color: 'bg-emerald-100 text-emerald-800' }
  if (health === 'unhealthy') return { label: '连接异常', color: 'bg-red-100 text-red-800' }
  return { label: '已启用', color: 'bg-blue-100 text-blue-800' }
}
