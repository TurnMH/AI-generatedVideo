import type { Episode } from '@/types'
import type { EpisodeAssetStats, EpisodeStoryboardStats, SerialStoryboardStats } from './episode-workspace-stats'

export type PipelineTone = 'amber' | 'blue' | 'green' | 'violet' | 'slate'

export type EpisodePipelineStatus = {
  tone: PipelineTone
  title: string
  description: string
}

export function computeEpisodePipelineStatus(params: {
  assetStats: EpisodeAssetStats
  storyboardStats: EpisodeStoryboardStats
  serialStoryboardStats: SerialStoryboardStats
  awaitingAutoStoryboard: boolean
  isExtractingStoryboards: boolean
  episodeStatus?: Episode['status']
  isSerial: boolean
  storyboardStageLabel: string
}): EpisodePipelineStatus {
  const {
    assetStats,
    storyboardStats,
    serialStoryboardStats,
    awaitingAutoStoryboard,
    isExtractingStoryboards,
    episodeStatus,
    isSerial,
    storyboardStageLabel,
  } = params

  if (assetStats.extracting) {
    return {
      tone: 'amber',
      title: '资源提取中',
      description: '系统正在分析本集角色、场景与道具，请稍候。',
    }
  }
  if (awaitingAutoStoryboard && storyboardStats.total === 0 && episodeStatus !== 'scene_splitting') {
    return {
      tone: 'blue',
      title: '自动处理进行中，等待镜头拆分',
      description: isSerial
        ? '已通过「自动处理本集」启动，系统会继续拆分镜头并生成串行场景分组。'
        : '已通过「自动处理本集」启动，系统会继续拆分镜头条目。',
    }
  }
  if (isExtractingStoryboards || episodeStatus === 'scene_splitting') {
    return {
      tone: 'blue',
      title: isSerial ? '镜头拆分中，等待首帧生成' : '镜头拆分中，等待出图',
      description: isSerial
        ? `当前已识别 ${serialStoryboardStats.sceneGroups || 0} 个场景组，拆分完成后会继续进入首帧生成。`
        : '当前正在拆分镜头条目，拆分完成后即可启动分镜图片生成。',
    }
  }
  if (storyboardStats.generating > 0 || storyboardStats.paused > 0) {
    return {
      tone: 'blue',
      title: isSerial ? '首帧生成中' : '分镜图片生成中',
      description: isSerial
        ? `当前已识别 ${serialStoryboardStats.sceneGroups || 0} 个场景组，首帧就绪 ${serialStoryboardStats.firstClipReady}/${serialStoryboardStats.firstClipTotal || 0}。`
        : `当前集已出图 ${storyboardStats.completed}/${storyboardStats.total || 0}，可继续在镜头工作台查看新增结果。`,
    }
  }
  if (storyboardStats.total > 0 && storyboardStats.pending > 0 && storyboardStats.completed === 0) {
    return {
      tone: 'violet',
      title: isSerial ? '镜头已拆分，待生成首帧' : '镜头已拆分，待生成图片',
      description: isSerial
        ? `当前集已拆分 ${storyboardStats.total} 条镜头，场景组 ${serialStoryboardStats.sceneGroups || 0}，尚未开始首帧生成。`
        : `当前集已拆分 ${storyboardStats.total} 条镜头，尚未开始分镜图片生成。`,
    }
  }
  if (storyboardStats.total > 0) {
    return {
      tone: 'green',
      title: isSerial ? '镜头已拆分' : '镜头与图片已就绪',
      description: isSerial
        ? `当前集已拆分 ${storyboardStats.total} 条镜头，场景组 ${serialStoryboardStats.sceneGroups || 0}，首帧 ${serialStoryboardStats.firstClipReady}/${serialStoryboardStats.firstClipTotal || 0}。`
        : `当前集已累计 ${storyboardStats.completed}/${storyboardStats.total} 条可用分镜图片。`,
    }
  }
  if (assetStats.active > 0) {
    return {
      tone: 'blue',
      title: '资源生成中',
      description: `当前集已完成 ${assetStats.completed}/${assetStats.total} 个资源，正在生成剩余资源。`,
    }
  }
  if (assetStats.total > 0) {
    return {
      tone: 'violet',
      title: '资源已就绪',
      description: `当前集已识别 ${assetStats.completed}/${assetStats.total} 个资源，可直接进入${storyboardStageLabel}。`,
    }
  }
  return {
    tone: 'slate',
    title: '等待开始',
    description: `你可以先提取本集资源，或在左侧单集列表点击「自动处理本集」衔接${storyboardStageLabel}。`,
  }
}

export const pipelineToneBadgeClass: Record<PipelineTone, string> = {
  amber: 'border-amber-200 bg-amber-100 text-amber-800',
  blue: 'border-blue-200 bg-blue-100 text-blue-800',
  green: 'border-emerald-200 bg-emerald-100 text-emerald-800',
  violet: 'border-violet-200 bg-violet-100 text-violet-800',
  slate: 'border-surface-200 bg-surface-100 text-surface-700',
}

export const pipelineToneLabel: Record<PipelineTone, string> = {
  amber: '处理中',
  blue: '进行中',
  green: '已就绪',
  violet: '可继续',
  slate: '待开始',
}

export const pipelineToneDotClass: Record<PipelineTone, string> = {
  amber: 'bg-amber-500',
  blue: 'bg-blue-500',
  green: 'bg-emerald-500',
  violet: 'bg-violet-500',
  slate: 'bg-surface-400',
}
