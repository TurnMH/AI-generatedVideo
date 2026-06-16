import type { Asset } from '@/types'

export type AssetExtractionState = {
  inProgress: boolean
  failed: boolean
  errorMessage: string
  modelName?: string
  episodeId?: number
}

export type AssetExtractionStatusResponse = {
  in_progress?: boolean
  failed?: boolean
  error_message?: string
  model_name?: string
  episode_id?: number
}

export function parseExtractionStatusResponse(payload?: AssetExtractionStatusResponse | null): AssetExtractionState {
  if (!payload) {
    return { inProgress: false, failed: false, errorMessage: '' }
  }
  if (payload.in_progress) {
    return { inProgress: true, failed: false, errorMessage: '' }
  }
  if (payload.failed) {
    return {
      inProgress: false,
      failed: true,
      errorMessage: String(payload.error_message || '资源提取失败，请切换模型后重试'),
      modelName: typeof payload.model_name === 'string' ? payload.model_name : undefined,
      episodeId: typeof payload.episode_id === 'number' ? payload.episode_id : undefined,
    }
  }
  return { inProgress: false, failed: false, errorMessage: '' }
}

function readExtractionMetadata(asset: Asset): Record<string, unknown> {
  const metadata = asset.metadata
  if (!metadata || typeof metadata !== 'object' || Array.isArray(metadata)) return {}
  return metadata as Record<string, unknown>
}

export function pickExtractionSentinel(assets: Asset[], episodeId?: number): Asset | undefined {
  const sentinels = assets.filter((asset) => asset.name === '__extracting__')
  if (sentinels.length === 0) return undefined

  const scoped = episodeId != null
    ? sentinels.filter((asset) => (asset.episode_ids ?? []).includes(episodeId))
    : sentinels
  const pool = scoped.length > 0 ? scoped : sentinels

  const failed = pool.find((asset) => asset.status === 'failed')
  if (failed) return failed
  const extracting = pool.find((asset) => asset.status === 'extracting')
  if (extracting) return extracting
  return [...pool].sort((left, right) => right.id - left.id)[0]
}

export function parseAssetExtractionState(assets: Asset[], episodeId?: number): AssetExtractionState {
  const sentinel = pickExtractionSentinel(assets, episodeId)
  if (!sentinel) {
    return { inProgress: false, failed: false, errorMessage: '' }
  }
  if (sentinel.status === 'extracting') {
    return { inProgress: true, failed: false, errorMessage: '' }
  }
  if (sentinel.status === 'failed') {
    const metadata = readExtractionMetadata(sentinel)
    return {
      inProgress: false,
      failed: true,
      errorMessage: String(metadata.extraction_error || sentinel.description || '资源提取失败，请切换模型后重试'),
      modelName: typeof metadata.model_name === 'string' ? metadata.model_name : undefined,
      episodeId: typeof metadata.episode_id === 'number' ? metadata.episode_id : sentinel.episode_ids?.[0],
    }
  }
  return { inProgress: false, failed: false, errorMessage: '' }
}

export function resolveExtractionModelName(
  selectedModelKey: string | undefined,
  projectTextModelId: number | undefined,
  textModels: Array<{ id: number; model_key?: string; name?: string }>,
): string | undefined {
  if (selectedModelKey?.trim()) return selectedModelKey.trim()
  if (projectTextModelId) {
    const matched = textModels.find((model) => model.id === projectTextModelId)
    const key = matched?.model_key?.trim()
    if (key) return key
  }
  return undefined
}
