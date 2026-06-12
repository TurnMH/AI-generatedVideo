import type { Episode } from '@/types'

export function getFirstEpisodeNumber(episodes: Episode[]): number | null {
  if (episodes.length === 0) return null
  return episodes.reduce((minValue, episode) => {
    if (episode.episode_number <= 0) return minValue
    return minValue === null ? episode.episode_number : Math.min(minValue, episode.episode_number)
  }, null as number | null)
}

export function isEpisodePipelineComplete(episode: Episode, storyboardTotal: number): boolean {
  return storyboardTotal > 0 || episode.status === 'scene_ready' || episode.status === 'done'
}

export function isEpisodePipelineFailed(input: {
  episode: Episode
  storyboardTotal: number
  hasStoryboardFailure?: boolean
  hasAssetFailure?: boolean
}): boolean {
  const { episode, storyboardTotal, hasStoryboardFailure, hasAssetFailure } = input
  if (episode.status === 'failed') return true
  if (episode.optimize_status === 'failed') return true
  if (episode.review_status === 'failed') return true
  if (hasStoryboardFailure) return true
  if (hasAssetFailure && storyboardTotal === 0) return true
  return false
}

export function isEpisodePipelineRunning(input: {
  episode: Episode
  storyboardTotal: number
  firstEpisodeNumber?: number | null
  assetExtracting?: boolean
  assetGenerating?: boolean
  storyboardGenerating?: boolean
  autoPreparingEpisodeId?: number | null
  firstEpisodeAutoActive?: boolean
}): boolean {
  const {
    episode,
    autoPreparingEpisodeId,
    firstEpisodeAutoActive,
    firstEpisodeNumber,
    assetExtracting,
    assetGenerating,
    storyboardGenerating,
  } = input

  if (autoPreparingEpisodeId === episode.id) return true
  if (firstEpisodeAutoActive && firstEpisodeNumber != null && episode.episode_number === firstEpisodeNumber) return true
  return Boolean(
    assetExtracting
    || assetGenerating
    || storyboardGenerating
    || episode.status === 'scene_splitting'
    || episode.status === 'script_prepping'
    || episode.optimize_status === 'optimizing'
    || episode.review_status === 'reviewing',
  )
}

export function getEpisodePipelineRunningLabel(input: {
  episode: Episode
  assetExtracting?: boolean
  assetGenerating?: boolean
  storyboardGenerating?: boolean
}): string {
  const { episode, assetExtracting, assetGenerating, storyboardGenerating } = input
  if (episode.optimize_status === 'optimizing' || episode.status === 'script_prepping') return '剧本润色中…'
  if (episode.review_status === 'reviewing') return '剧本审查中…'
  if (assetExtracting) return '资源提取中…'
  if (assetGenerating) return '资源生成中…'
  if (episode.status === 'scene_splitting' || storyboardGenerating) return '分镜拆分中…'
  return '自动处理中…'
}

export type EpisodeAutoPipelineAction =
  | { type: 'hidden' }
  | { type: 'start' }
  | { type: 'running'; label: string }
  | { type: 'success'; label: string }
  | { type: 'retry'; label: string; reason?: string }

export function resolveEpisodeAutoPipelineAction(input: {
  episode: Episode
  storyboardTotal: number
  firstEpisodeNumber?: number | null
  autoPreparingEpisodeId?: number | null
  firstEpisodeAutoActive?: boolean
  assetExtracting?: boolean
  assetGenerating?: boolean
  storyboardGenerating?: boolean
  hasStoryboardFailure?: boolean
  hasAssetFailure?: boolean
  wasAutoAttempted?: boolean
}): EpisodeAutoPipelineAction {
  const running = isEpisodePipelineRunning(input)
  const complete = isEpisodePipelineComplete(input.episode, input.storyboardTotal)
  const failed = isEpisodePipelineFailed(input)

  if (running) {
    return {
      type: 'running',
      label: getEpisodePipelineRunningLabel(input),
    }
  }

  if (complete) {
    if (!input.wasAutoAttempted) {
      return { type: 'hidden' }
    }
    return {
      type: 'success',
      label: input.storyboardTotal > 0
        ? `自动处理已完成（${input.storyboardTotal} 个分镜）`
        : '自动处理已完成',
    }
  }

  if (failed || (input.wasAutoAttempted && !complete)) {
    return {
      type: 'retry',
      label: '重试自动生成',
      reason: failed ? '上次自动处理未完成或出现异常' : '上次自动处理未生成分镜，可重试',
    }
  }

  const isFirstEpisode = input.firstEpisodeNumber != null && input.episode.episode_number === input.firstEpisodeNumber
  if (isFirstEpisode && input.firstEpisodeAutoActive) {
    return {
      type: 'running',
      label: getEpisodePipelineRunningLabel(input),
    }
  }

  return { type: 'start' }
}

/** @deprecated use resolveEpisodeAutoPipelineAction */
export function shouldShowEpisodeAutoButton(input: {
  episode: Episode
  episodes: Episode[]
  storyboardTotal: number
  firstEpisodeNumber?: number | null
  autoPreparingEpisodeId?: number | null
  firstEpisodeAutoActive?: boolean
  assetExtracting?: boolean
  assetGenerating?: boolean
  storyboardGenerating?: boolean
}): boolean {
  const action = resolveEpisodeAutoPipelineAction({ ...input, wasAutoAttempted: false })
  return action.type === 'start' || action.type === 'retry'
}
