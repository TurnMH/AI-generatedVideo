import type { Asset } from '@/types'

export type StoryboardAssetReadiness = {
  ready: boolean
  blockingReason: string
  pending: number
  generating: number
  paused: number
  failed: number
}

/** Asset readiness for storyboard image generation within a scoped asset list. */
export function getStoryboardAssetReadiness(assets: Asset[]): StoryboardAssetReadiness {
  const visible = assets.filter((asset) => asset.name !== '__extracting__')
  const pending = visible.filter((asset) => asset.status === 'pending').length
  const generating = visible.filter((asset) => asset.status === 'generating').length
  const paused = visible.filter((asset) => asset.status === 'paused').length
  const failed = visible.filter((asset) => asset.status === 'failed').length

  // Assets are optional — allow storyboard generation when no assets exist.
  // Failed assets are a terminal state and do NOT block storyboard generation.
  const ready = visible.length === 0 || (pending === 0 && generating === 0 && paused === 0)
  const blockingReason = ready
    ? ''
    : `资源图尚未全部完成：待生成 ${pending}，生成中 ${generating}，已暂停 ${paused}，失败 ${failed}`

  return { ready, blockingReason, pending, generating, paused, failed }
}
