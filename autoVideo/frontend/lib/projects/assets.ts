import type { Asset } from '@/types'
import { getElapsedTimeLabel, getEstimatedRemainingLabel } from './utils'
import { GENERATION_STAGE_HINTS } from './constants'

export type AssetImageVersion = {
  url: string
  prompt?: string
  created_at?: string
  source?: string
  model_name?: string
}

export type AssetGenerationProgress = {
  stage: string
  label: string
  detail?: string
  percent: number
  task_id?: number
  model_name?: string
  started_at?: string
  updated_at?: string
}

export function getAssetGeneratedImages(asset: Asset): AssetImageVersion[] {
  const metadata = asset.metadata ?? {}
  const rawVersions = Array.isArray(metadata.generated_images) ? metadata.generated_images : []
  const versions: AssetImageVersion[] = []
  rawVersions.forEach((item) => {
    if (!item || typeof item !== 'object') return
    const record = item as Record<string, unknown>
    const url = typeof record.url === 'string' ? record.url.trim() : ''
    if (!url) return
    versions.push({
      url,
      prompt: typeof record.prompt === 'string' ? record.prompt : undefined,
      created_at: typeof record.created_at === 'string' ? record.created_at : undefined,
      source: typeof record.source === 'string' ? record.source : undefined,
      model_name: typeof record.model_name === 'string' ? record.model_name : undefined,
    })
  })

  const primaryImage = asset.image_url?.trim()
  if (primaryImage && !versions.some((item) => item.url === primaryImage)) {
    versions.unshift({
      url: primaryImage,
      prompt: asset.prompt_used || undefined,
      created_at: asset.updated_at,
      source: 'current',
    })
  }

  return versions
}

export function getSelectedGeneratedImageUrl(asset: Asset): string {
  const metadata = asset.metadata ?? {}
  const preferred = typeof metadata.selected_generated_image_url === 'string' ? metadata.selected_generated_image_url.trim() : ''
  if (preferred) return preferred

  const primaryImage = asset.image_url?.trim()
  if (primaryImage) return primaryImage

  return getAssetGeneratedImages(asset)[0]?.url ?? ''
}

export function getAssetGenerationProgress(asset: Asset): AssetGenerationProgress | null {
  const metadata = asset.metadata ?? {}
  const raw = metadata.generation_progress
  if (!raw || typeof raw !== 'object') return null

  const record = raw as Record<string, unknown>
  const label = typeof record.label === 'string' ? record.label.trim() : ''
  if (!label) return null

  const percentValue = typeof record.percent === 'number' ? record.percent : Number(record.percent ?? 0)

  return {
    stage: typeof record.stage === 'string' ? record.stage : 'processing',
    label,
    detail: typeof record.detail === 'string' ? record.detail : undefined,
    percent: Number.isFinite(percentValue) ? Math.max(0, Math.min(100, Math.round(percentValue))) : 0,
    task_id: typeof record.task_id === 'number' ? record.task_id : undefined,
    model_name: typeof record.model_name === 'string' ? record.model_name : undefined,
    started_at: typeof record.started_at === 'string' ? record.started_at : undefined,
    updated_at: typeof record.updated_at === 'string' ? record.updated_at : undefined,
  }
}

export function getGenerationStageHint(progress: AssetGenerationProgress | null, tick: number): string {
  if (!progress) return ''
  const messages = GENERATION_STAGE_HINTS[progress.stage] ?? GENERATION_STAGE_HINTS.processing
  if (messages.length === 0) return progress.detail || ''
  return messages[tick % messages.length] ?? messages[0]
}

export function getGenerationEtaLabel(progress: AssetGenerationProgress | null, nowMs: number): string | null {
  if (!progress || progress.percent >= 100) return null
  return getEstimatedRemainingLabel(progress.started_at ?? progress.updated_at, progress.percent / 100, nowMs)
}

export function getGenerationElapsedLabel(progress: AssetGenerationProgress | null, nowMs: number): string | null {
  if (!progress) return null
  return getElapsedTimeLabel(progress.started_at ?? progress.updated_at, nowMs)
}
