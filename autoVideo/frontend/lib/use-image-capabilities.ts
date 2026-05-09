import useSWR from 'swr'
import { modelAPI } from '@/lib/api'
import type {
  ImageModelCapability,
  ImageModelCapabilitiesResponse,
  ImageRatioIntent,
  ImageSizeMode,
  TaskTypeCapability,
} from '@/types'

export function useImageCapabilities() {
  const { data, isLoading, error } = useSWR(
    'image-model-capabilities',
    () => modelAPI.imageCapabilities() as unknown as Promise<{ data: ImageModelCapabilitiesResponse }>,
    { revalidateOnFocus: false, dedupingInterval: 60_000 }
  )

  const resp = (data as { data?: ImageModelCapabilitiesResponse })?.data
  const models: ImageModelCapability[] = resp?.models ?? []
  const taskTypes: TaskTypeCapability[] = resp?.task_type_capabilities ?? []

  return { models, taskTypes, isLoading, error }
}

/** Given a model key, find its capability entry */
export function findModelCapability(
  models: ImageModelCapability[],
  modelKey: string
): ImageModelCapability | undefined {
  return models.find((m) => m.key === modelKey)
}

/** Given a task type, find its preferred ratio intent */
export function findTaskTypeRatio(
  taskTypes: TaskTypeCapability[],
  taskType: string
): ImageRatioIntent {
  return taskTypes.find((t) => t.task_type === taskType)?.ratio_intent ?? 'square'
}

/** Get the recommended default size for a model + task type combination */
export function getRecommendedSize(
  cap: ImageModelCapability | undefined,
  ratioIntent: ImageRatioIntent
): { width: number; height: number } {
  if (!cap) return { width: 1024, height: 1024 }

  let sizeStr: string
  switch (ratioIntent) {
    case 'portrait':
      sizeStr = cap.default_portrait || cap.default_square || '1024x1024'
      break
    case 'landscape':
      sizeStr = cap.default_landscape || cap.default_square || '1024x1024'
      break
    default:
      sizeStr = cap.default_square || '1024x1024'
  }

  const [w, h] = sizeStr.split('x').map(Number)
  return { width: w || 1024, height: h || 1024 }
}

/** Check if a given width x height is valid for a model */
export function validateSize(
  cap: ImageModelCapability | undefined,
  width: number,
  height: number
): { valid: boolean; reason?: string } {
  if (!cap) return { valid: true }

  const mode: ImageSizeMode = cap.size_mode

  if (mode === 'passthrough') return { valid: true }

  if (mode === 'enum_size') {
    const sizeStr = `${width}x${height}`
    if (cap.allowed_sizes && cap.allowed_sizes.length > 0) {
      if (!cap.allowed_sizes.includes(sizeStr)) {
        return {
          valid: false,
          reason: `${cap.provider_key} 当前仅支持: ${cap.allowed_sizes.join(', ')}`,
        }
      }
    }
    return { valid: true }
  }

  // arbitrary_wh
  const issues: string[] = []
  if (cap.min_width && width < cap.min_width) issues.push(`宽度不能小于 ${cap.min_width}`)
  if (cap.max_width && width > cap.max_width) issues.push(`宽度不能大于 ${cap.max_width}`)
  if (cap.min_height && height < cap.min_height) issues.push(`高度不能小于 ${cap.min_height}`)
  if (cap.max_height && height > cap.max_height) issues.push(`高度不能大于 ${cap.max_height}`)
  if (cap.require_multiple && cap.require_multiple > 1) {
    if (width % cap.require_multiple !== 0) issues.push(`宽度需为 ${cap.require_multiple} 的倍数`)
    if (height % cap.require_multiple !== 0) issues.push(`高度需为 ${cap.require_multiple} 的倍数`)
  }

  return issues.length > 0 ? { valid: false, reason: issues.join('；') } : { valid: true }
}

/** Human-readable labels */
export const SIZE_MODE_LABELS: Record<ImageSizeMode, string> = {
  arbitrary_wh: '自由宽高',
  enum_size: '固定尺寸',
  passthrough: '透传',
}

export const VERIFICATION_LABELS: Record<string, string> = {
  verified: '已核实',
  partial: '部分核实',
  assumed: '待核实',
}

export const VERIFICATION_COLORS: Record<string, string> = {
  verified: 'bg-emerald-100 text-emerald-700',
  partial: 'bg-amber-100 text-amber-700',
  assumed: 'bg-surface-100 text-surface-500',
}

export const RATIO_INTENT_LABELS: Record<ImageRatioIntent, string> = {
  square: '方图',
  portrait: '竖图',
  landscape: '横图',
}