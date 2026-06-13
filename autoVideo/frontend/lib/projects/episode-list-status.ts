import type { Episode } from '@/types'

export type EpisodeWorkspaceMeta = {
  assetTotal: number
  assetCompleted: number
  assetFailed: number
  assetExtracting?: boolean
  assetGenerating?: boolean
  storyboardTotal: number
  storyboardCompleted: number
  storyboardGenerating?: boolean
  storyboardFailed?: boolean
}

export type EpisodePhaseView = {
  label: string
  className: string
}

export function deriveEpisodePhase(ep: Episode, meta: EpisodeWorkspaceMeta | undefined): EpisodePhaseView {
  const assetTotal = meta?.assetTotal ?? 0
  const storyboardTotal = meta?.storyboardTotal ?? 0
  const isAssetExtracting = meta?.assetExtracting ?? false
  const isAssetGenerating = meta?.assetGenerating ?? false
  const isStoryboardGenerating = meta?.storyboardGenerating ?? false
  const hasStoryboardFailure = meta?.storyboardFailed ?? false
  const hasAssetFailure = (meta?.assetFailed ?? 0) > 0

  if (ep.status === 'failed' || (storyboardTotal > 0 && hasStoryboardFailure)) {
    return { label: '异常待处理', className: 'text-red-600' }
  }
  if (isStoryboardGenerating || ep.status === 'scene_splitting') {
    return {
      label: ep.status === 'scene_splitting' ? '分镜拆分中' : '分镜出图中',
      className: 'text-blue-600',
    }
  }
  if (storyboardTotal > 0) {
    return { label: '分镜已就绪', className: 'text-violet-600' }
  }
  if (isAssetExtracting) {
    return { label: '资源提取中', className: 'text-amber-600' }
  }
  if (isAssetGenerating) {
    return { label: '资源生成中', className: 'text-blue-600' }
  }
  if (assetTotal > 0) {
    return { label: '待拆分分镜', className: 'text-primary-600' }
  }
  if (hasAssetFailure) {
    return { label: '资源异常', className: 'text-red-600' }
  }
  return { label: '待自动处理', className: 'text-surface-400' }
}

export type EpisodeBadgeTone = 'red' | 'blue' | 'violet' | 'muted' | 'green' | 'yellow'

export type EpisodeBadgeView = {
  label: string
  tone: EpisodeBadgeTone
  spinning?: boolean
}

const badgeClass: Record<EpisodeBadgeTone, string> = {
  red: 'bg-red-100 text-red-700',
  blue: 'bg-blue-100 text-blue-700',
  violet: 'bg-violet-100 text-violet-700',
  muted: 'bg-surface-100 text-surface-500',
  green: 'bg-green-100 text-green-800',
  yellow: 'bg-yellow-100 text-yellow-800',
}

export function episodeBadgeClassName(tone: EpisodeBadgeTone): string {
  return badgeClass[tone]
}

export function deriveEpisodeAssetBadge(
  ep: Episode,
  meta: EpisodeWorkspaceMeta | undefined,
  projectSharedAssetCount: number,
): EpisodeBadgeView {
  const assetTotal = meta?.assetTotal ?? 0
  const assetCompleted = meta?.assetCompleted ?? 0
  const storyboardTotal = meta?.storyboardTotal ?? 0
  const isAssetExtracting = meta?.assetExtracting ?? false
  const isAssetGenerating = meta?.assetGenerating ?? false
  const hasAssetFailure = (meta?.assetFailed ?? 0) > 0

  if (isAssetExtracting) {
    return { label: '资源提取中', tone: 'yellow', spinning: true }
  }
  if (assetTotal === 0) {
    if (storyboardTotal > 0 && projectSharedAssetCount > 0) {
      return { label: `共用项目资源 ${projectSharedAssetCount}`, tone: 'muted' }
    }
    return { label: '暂无资源', tone: 'muted' }
  }
  if (isAssetGenerating) {
    return { label: `资源生成中 ${assetCompleted}/${assetTotal}`, tone: 'blue', spinning: true }
  }
  if (hasAssetFailure) {
    return { label: `资源异常 ${assetCompleted}/${assetTotal}`, tone: 'red' }
  }
  return { label: `资源就绪 ${assetCompleted}/${assetTotal}`, tone: 'green' }
}

export function deriveEpisodeStoryboardBadge(
  ep: Episode,
  meta: EpisodeWorkspaceMeta | undefined,
  labels: { splitting: string; generating: string } = { splitting: '分镜拆分中', generating: '分镜出图中' },
): EpisodeBadgeView {
  const storyboardTotal = meta?.storyboardTotal ?? 0
  const storyboardCompleted = meta?.storyboardCompleted ?? 0
  const isAssetExtracting = meta?.assetExtracting ?? false
  const isAssetGenerating = meta?.assetGenerating ?? false
  const isStoryboardGenerating = meta?.storyboardGenerating ?? false
  const hasStoryboardFailure = meta?.storyboardFailed ?? false

  if (ep.status === 'failed' && storyboardTotal === 0) {
    return { label: '分镜异常', tone: 'red' }
  }
  if (isStoryboardGenerating || ep.status === 'scene_splitting') {
    const progress = storyboardTotal > 0 ? ` ${storyboardCompleted}/${storyboardTotal}` : ''
    return {
      label: `${ep.status === 'scene_splitting' ? labels.splitting : labels.generating}${progress}`,
      tone: 'blue',
      spinning: true,
    }
  }
  if (storyboardTotal > 0 && hasStoryboardFailure) {
    return { label: `分镜异常 ${storyboardCompleted}/${storyboardTotal}`, tone: 'red' }
  }
  if (storyboardTotal > 0) {
    return { label: `分镜就绪 ${storyboardCompleted}/${storyboardTotal}`, tone: 'violet' }
  }
  if (isAssetExtracting || isAssetGenerating) {
    return { label: '等待资源完成', tone: 'muted' }
  }
  if ((meta?.assetTotal ?? 0) > 0) {
    return { label: '待拆分分镜', tone: 'blue' }
  }
  return { label: '待自动处理', tone: 'muted' }
}
