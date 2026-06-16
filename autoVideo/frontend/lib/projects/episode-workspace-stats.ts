import type { Asset, Storyboard } from '@/types'
import { parseAssetExtractionState } from '@/lib/projects/asset-extraction'

export type EpisodeAssetStats = {
  total: number
  completed: number
  active: number
  paused: number
  generating: number
  failed: number
  extracting: boolean
  extractionFailed: boolean
  extractionError: string
}

export type EpisodeStoryboardStats = {
  total: number
  completed: number
  pending: number
  active: number
  paused: number
  generating: number
  failed: number
}

export type SerialStoryboardStats = {
  sceneGroups: number
  firstClipTotal: number
  firstClipReady: number
}

export function computeEpisodeAssetStats(episodeAssets: Asset[], episodeId?: number): EpisodeAssetStats {
  const extractionState = parseAssetExtractionState(episodeAssets, episodeId)
  const extracting = extractionState.inProgress
  const visibleAssets = episodeAssets.filter((asset) => asset.name !== '__extracting__' && asset.status !== 'extracting')
  const completed = visibleAssets.filter((asset) => asset.status === 'completed').length
  const paused = visibleAssets.filter((asset) => asset.status === 'paused').length
  const generating = visibleAssets.filter((asset) => asset.status === 'generating' || asset.status === 'pending').length
  const failed = visibleAssets.filter((asset) => asset.status === 'failed' || asset.status === 'qa_failed').length
  return {
    total: visibleAssets.length,
    completed,
    active: paused + generating,
    paused,
    generating,
    failed,
    extracting,
    extractionFailed: extractionState.failed,
    extractionError: extractionState.errorMessage,
  }
}

export function computeEpisodeStoryboardStats(episodeStoryboards: Storyboard[]): EpisodeStoryboardStats {
  const completed = episodeStoryboards.filter((sb) => sb.status === 'completed' && sb.image_url).length
  const paused = episodeStoryboards.filter((sb) => sb.status === 'paused').length
  const pending = episodeStoryboards.filter((sb) => sb.status === 'pending').length
  const generating = episodeStoryboards.filter((sb) => sb.status === 'generating').length
  const failed = episodeStoryboards.filter((sb) => sb.status === 'failed').length
  return {
    total: episodeStoryboards.length,
    completed,
    pending,
    active: paused + generating,
    paused,
    generating,
    failed,
  }
}

export function computeSerialStoryboardStats(episodeStoryboards: Storyboard[]): SerialStoryboardStats {
  const sceneGroups = new Set(
    episodeStoryboards.map((storyboard) => storyboard.scene_group_key).filter((groupKey): groupKey is string => Boolean(groupKey)),
  ).size
  const firstClips = episodeStoryboards.filter((storyboard) => storyboard.scene_group_key && storyboard.is_scene_first_clip)
  const firstClipReady = firstClips.filter((storyboard) => storyboard.status === 'completed' && storyboard.image_url).length
  return {
    sceneGroups,
    firstClipTotal: firstClips.length,
    firstClipReady,
  }
}
