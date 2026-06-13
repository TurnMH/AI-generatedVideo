import type { Project } from '@/types'
import type { EpisodeAssetStats, EpisodeStoryboardStats, SerialStoryboardStats } from './episode-workspace-stats'

export type WorkflowStepStatus = 'done' | 'current' | 'pending' | 'failed' | 'skipped'
export type WorkflowStepKey = 'assets' | 'storyboard' | 'dubbing' | 'video'

export type WorkflowStep = {
  key: WorkflowStepKey
  label: string
  status: WorkflowStepStatus
  statusLabel: string
  hint: string
}

export function computeEpisodeWorkflowSteps(params: {
  project: Pick<Project, 'enable_dubbing' | 'enable_subtitle'>
  assetStats: EpisodeAssetStats
  storyboardStats: EpisodeStoryboardStats
  serialStoryboardStats: SerialStoryboardStats
  awaitingAutoStoryboard: boolean
  isExtracting: boolean
  isExtractingStoryboards: boolean
  episodeStatus?: string
  storyboardStageLabel: string
  isSerial: boolean
  hasRenderableStoryboard: boolean
}): WorkflowStep[] {
  const {
    project,
    assetStats,
    storyboardStats,
    serialStoryboardStats,
    awaitingAutoStoryboard,
    isExtracting,
    isExtractingStoryboards,
    episodeStatus,
    storyboardStageLabel,
    isSerial,
    hasRenderableStoryboard,
  } = params

  const dubbingEnabled = project.enable_dubbing || project.enable_subtitle

  const assetStepStatus: WorkflowStepStatus = assetStats.failed > 0 && assetStats.completed === 0 && !assetStats.extracting && assetStats.active === 0
    ? 'failed'
    : assetStats.extracting || assetStats.active > 0 || isExtracting
      ? 'current'
      : assetStats.total > 0 && assetStats.failed === 0
        ? 'done'
        : assetStats.total > 0
          ? 'current'
          : 'pending'

  const storyboardStepStatus: WorkflowStepStatus = storyboardStats.failed > 0 && storyboardStats.completed === 0 && storyboardStats.active === 0 && storyboardStats.pending === 0
    ? 'failed'
    : awaitingAutoStoryboard || isExtractingStoryboards || episodeStatus === 'scene_splitting' || storyboardStats.active > 0
      ? 'current'
      : storyboardStats.total > 0 && storyboardStats.pending === 0 && storyboardStats.failed === 0
        ? 'done'
        : 'pending'

  return [
    {
      key: 'assets',
      label: '资源提取',
      status: assetStepStatus,
      statusLabel: assetStats.extracting || isExtracting
        ? '提取中'
        : assetStats.active > 0
          ? '生成中'
          : assetStepStatus === 'done'
            ? '已完成'
            : assetStepStatus === 'failed'
              ? '异常'
              : '待开始',
      hint: assetStats.extracting || isExtracting
        ? '提取中...'
        : assetStats.active > 0
          ? `生成中 ${assetStats.completed}/${assetStats.total || '?'}`
          : assetStats.failed > 0
            ? `${assetStats.completed}/${assetStats.total}，失败 ${assetStats.failed}`
            : assetStats.total > 0
              ? `${assetStats.completed}/${assetStats.total} 个资源`
              : '尚未开始',
    },
    {
      key: 'storyboard',
      label: storyboardStageLabel,
      status: storyboardStepStatus,
      statusLabel: awaitingAutoStoryboard
        ? '排队中'
        : isExtractingStoryboards || episodeStatus === 'scene_splitting'
          ? '拆分中'
          : storyboardStats.generating > 0 || storyboardStats.paused > 0
            ? (isSerial ? '首帧生成中' : '出图中')
            : storyboardStepStatus === 'done'
              ? '已完成'
              : storyboardStepStatus === 'failed'
                ? '异常'
                : storyboardStats.total > 0
                  ? (isSerial ? '待首帧' : '待出图')
                  : assetStats.total > 0
                    ? '待开始'
                    : '待资源',
      hint: awaitingAutoStoryboard
        ? '资源条目识别后自动开启'
        : isExtractingStoryboards || episodeStatus === 'scene_splitting'
          ? '正在拆分镜头条目'
          : storyboardStats.generating > 0 || storyboardStats.paused > 0
            ? isSerial
              ? `场景组 ${serialStoryboardStats.sceneGroups || 0}，首帧 ${serialStoryboardStats.firstClipReady}/${serialStoryboardStats.firstClipTotal || 0}`
              : `出图中 ${storyboardStats.completed}/${storyboardStats.total || '?'}`
            : storyboardStats.failed > 0
              ? `${storyboardStats.completed}/${storyboardStats.total}，失败 ${storyboardStats.failed}`
              : storyboardStats.total > 0
                ? isSerial
                  ? `${storyboardStats.total} 条镜头，待生成首帧`
                  : `${storyboardStats.total} 条镜头，待生成图片`
                : assetStats.total > 0
                  ? '可手动启动'
                  : '依赖资源结果',
    },
    {
      key: 'dubbing',
      label: '语音合成',
      status: dubbingEnabled ? 'pending' : 'skipped',
      statusLabel: !dubbingEnabled
        ? '未启用'
        : storyboardStats.total > 0
          ? '可开始'
          : '待分镜',
      hint: !dubbingEnabled
        ? '项目未启用'
        : storyboardStats.total > 0
          ? '分镜已就绪'
          : '需先完成分镜',
    },
    {
      key: 'video',
      label: '视频成片',
      status: 'pending',
      statusLabel: hasRenderableStoryboard
        ? '可开始'
        : '待前序',
      hint: hasRenderableStoryboard
        ? isSerial ? '首帧已就绪，可继续串行视频' : '分镜图片已就绪，可继续成片'
        : '需先完成前序步骤',
    },
  ]
}

export const workflowStatusLabel: Record<WorkflowStepStatus, string> = {
  done: '已完成',
  current: '处理中',
  pending: '待开始',
  failed: '异常',
  skipped: '未启用',
}

export const workflowStatusClass: Record<WorkflowStepStatus, string> = {
  done: 'border-emerald-200 bg-emerald-50 text-emerald-700',
  current: 'border-blue-200 bg-blue-50 text-blue-700',
  pending: 'border-surface-200 bg-surface-50 text-surface-600',
  failed: 'border-red-200 bg-red-50 text-red-700',
  skipped: 'border-surface-200 bg-surface-100 text-surface-400',
}
