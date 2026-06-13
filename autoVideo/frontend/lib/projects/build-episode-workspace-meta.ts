import { isStoryboardImageActive } from '@/lib/projects/storyboard-status'
import type { Episode } from '@/types'
import type { EpisodeWorkspaceMeta } from '@/lib/projects/episode-list-status'

type AssetRow = {
  name?: string
  status?: string
  episode_ids?: unknown[]
}

type StoryboardRow = {
  episode_id?: unknown
  status?: string
  image_url?: string | null
}

export function buildEpisodeWorkspaceMeta(
  episodes: Episode[],
  assets: AssetRow[],
  storyboards: StoryboardRow[],
): Map<number, EpisodeWorkspaceMeta> {
  const meta = new Map<number, EpisodeWorkspaceMeta>()

  for (const ep of episodes) {
    meta.set(ep.id, {
      assetTotal: 0,
      assetCompleted: 0,
      assetFailed: 0,
      assetExtracting: false,
      assetGenerating: false,
      storyboardTotal: 0,
      storyboardCompleted: 0,
      storyboardGenerating: false,
      storyboardFailed: false,
    })
  }

  for (const asset of assets) {
    if (asset?.name === '__extracting__' || asset?.status === 'extracting') continue
    const episodeIds = Array.isArray(asset?.episode_ids)
      ? asset.episode_ids.map((id) => Number(id)).filter((id) => Number.isFinite(id))
      : []
    for (const eid of episodeIds) {
      const current = meta.get(eid)
      if (!current) continue
      current.assetTotal += 1
      if (asset?.status === 'completed') current.assetCompleted += 1
      if (asset?.status === 'failed' || asset?.status === 'qa_failed') current.assetFailed += 1
      if (asset?.status === 'pending' || asset?.status === 'generating' || asset?.status === 'paused') {
        current.assetGenerating = true
      }
    }
  }

  for (const sb of storyboards) {
    const eid = Number(sb?.episode_id)
    if (!Number.isFinite(eid) || eid <= 0) continue
    const current = meta.get(eid)
    if (!current) continue
    current.storyboardTotal += 1
    if (sb?.status === 'completed' && sb?.image_url) current.storyboardCompleted += 1
    if (isStoryboardImageActive(sb?.status)) current.storyboardGenerating = true
    if (sb?.status === 'failed') current.storyboardFailed = true
  }

  return meta
}

export function countVisibleAssets(assets: AssetRow[]): number {
  return assets.filter((asset) => asset?.name !== '__extracting__' && asset?.status !== 'extracting').length
}

export function isAssetExtractionRunning(assets: AssetRow[]): boolean {
  return assets.some((asset) => asset?.name === '__extracting__' || asset?.status === 'extracting')
}
