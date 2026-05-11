import { FileText, Image, LayoutGrid, Mic, Video } from 'lucide-react'
import type { Project } from '@/types'

export const WORKFLOW_STEPS = [
  { key: 'script',     label: '剧本',   Icon: FileText   },
  { key: 'assets',     label: '资源',   Icon: Image      },
  { key: 'storyboard', label: '分镜',   Icon: LayoutGrid },
  { key: 'dubbing',    label: '配音/字幕', Icon: Mic      },
  { key: 'video',      label: '视频',   Icon: Video      },
] as const

export type WorkflowStepKey = (typeof WORKFLOW_STEPS)[number]['key']
export type WorkflowStepStatus = 'done' | 'current' | 'pending' | 'failed' | 'paused' | 'skipped'

export type StoryboardStatsData = {
  total: number
  pending: number
  generating: number
  paused: number
  completed: number
  failed: number
  voided: number
}

export type StepAssetStats = {
  total: number
  completed: number
  pending: number
  extracting: number
  generating: number
  failed: number
}

export type StepStoryboardStats = {
  total: number
  completed: number
  pending: number
  paused: number
  generating: number
  failed: number
}

export type StepDubbingStats = {
  total: number
  completed: number
  processing: number
  failed: number
}

export type StepVideoStats = {
  total: number
  completed: number
  pending: number
  processing: number
  failed: number
  paused: number
}

export type WorkflowStepView = {
  key: WorkflowStepKey
  label: string
  Icon: (typeof WORKFLOW_STEPS)[number]['Icon']
  status: WorkflowStepStatus
  processing: boolean
  subLabel: string | null
  progress: number | null
}

export type ProjectOverviewStepStatus = 'done' | 'current' | 'pending' | 'failed' | 'skipped'

export type ProjectOverviewStepView = {
  key: 'script' | 'assets' | 'scene-groups' | 'storyboard-split' | 'storyboard-images' | 'video'
  label: string
  status: ProjectOverviewStepStatus
  hint: string
}

export type EpisodeWorkspaceTab = 'assets' | 'storyboard' | 'dubbing' | 'video'

export type ProjectOverviewNotice = {
  tone: 'blue' | 'red'
  title: string
  description: string
}

export type ProjectOverviewAction = {
  title: string
  description: string
  cta: string
  type: 'extract_assets' | 'extract_storyboards' | 'select_episode' | 'noop'
  tab?: EpisodeWorkspaceTab
  episodeId?: number | null
  disabled?: boolean
}

export type BuildProjectOverviewInput = {
  project: Project
  episodeCount: number
  episodeSplitting: number
  assetTotal: number
  assetCompleted: number
  assetActive: number
  assetFailed: number
  storyboardTotal: number
  storyboardImageReady: number
  storyboardActive: number
  storyboardFailed: number
  isExtractingStoryboards: boolean
  sceneGroupCount?: number
  serialFirstClipTotal?: number
  serialFirstClipReady?: number
  firstAssetFailureEpisodeId?: number | null
  firstStoryboardFailureEpisodeId?: number | null
  firstStoryboardActiveEpisodeId?: number | null
  firstStoryboardReadyEpisodeId?: number | null
  firstStoryboardImageReadyEpisodeId?: number | null
  firstAssetReadyEpisodeId?: number | null
}

export function buildProjectOverview({
  project,
  episodeCount,
  episodeSplitting,
  assetTotal,
  assetCompleted,
  assetActive,
  assetFailed,
  storyboardTotal,
  storyboardImageReady,
  storyboardActive,
  storyboardFailed,
  isExtractingStoryboards,
  sceneGroupCount = 0,
  serialFirstClipTotal = 0,
  serialFirstClipReady = 0,
  firstAssetFailureEpisodeId = null,
  firstStoryboardFailureEpisodeId = null,
  firstStoryboardActiveEpisodeId = null,
  firstStoryboardReadyEpisodeId = null,
  firstStoryboardImageReadyEpisodeId = null,
  firstAssetReadyEpisodeId = null,
}: BuildProjectOverviewInput): {
  workflowSteps: ProjectOverviewStepView[]
  notices: ProjectOverviewNotice[]
  nextAction: ProjectOverviewAction
} {
  const isSerial = project.project_type === 'video_serial'
  const dubbingEnabled = project.enable_dubbing || project.enable_subtitle
  const hasAssets = assetTotal > 0
  const hasStoryboards = storyboardTotal > 0
  const hasStoryboardImages = storyboardImageReady > 0
  const hasSerialFirstClips = serialFirstClipReady > 0
  const hasRenderableStoryboard = hasStoryboardImages || (isSerial && hasSerialFirstClips)

  const workflowSteps: ProjectOverviewStepView[] = isSerial
    ? [
        {
          key: 'script',
          label: '剧本拆分',
          status: episodeCount > 0 ? 'done' : project.progress?.stage === 'episode_splitting' ? 'current' : 'pending',
          hint: episodeCount > 0 ? `已生成 ${episodeCount} 集` : project.target_episodes > 0 ? `目标 ${project.target_episodes} 集` : '等待拆分',
        },
        {
          key: 'assets',
          label: '资源提取',
          status: assetFailed > 0 && assetCompleted === 0 && assetActive === 0 ? 'failed' : hasAssets && assetActive === 0 && assetFailed === 0 ? 'done' : assetActive > 0 || hasAssets ? 'current' : episodeCount > 0 ? 'pending' : 'pending',
          hint: hasAssets ? `已完成 ${assetCompleted}/${assetTotal}` : episodeCount > 0 ? '待开始' : '依赖分集结果',
        },
        {
          key: 'scene-groups',
          label: '场景分组',
          status: sceneGroupCount > 0 ? 'done' : hasStoryboards ? 'current' : episodeCount > 0 ? 'pending' : 'pending',
          hint: sceneGroupCount > 0 ? `已识别 ${sceneGroupCount} 个场景组` : hasStoryboards ? '待核查串行分组' : '串行模式专属',
        },
        {
          key: 'storyboard-split',
          label: '镜头拆分',
          status: storyboardFailed > 0 && storyboardTotal === 0 && storyboardActive === 0 ? 'failed' : hasStoryboards && storyboardActive === 0 ? 'done' : storyboardActive > 0 || episodeSplitting > 0 || hasStoryboards || isExtractingStoryboards ? 'current' : episodeCount > 0 ? 'pending' : 'pending',
          hint: hasStoryboards ? `已拆分 ${storyboardTotal} 条镜头` : episodeCount > 0 ? '待开始' : '依赖分集结果',
        },
        {
          key: 'storyboard-images',
          label: '首帧准备',
          status: storyboardFailed > 0 && serialFirstClipReady === 0 && storyboardActive === 0 ? 'failed' : serialFirstClipTotal > 0 && serialFirstClipReady === serialFirstClipTotal && storyboardActive === 0 && storyboardFailed === 0 ? 'done' : storyboardActive > 0 || serialFirstClipReady > 0 || serialFirstClipTotal > 0 ? 'current' : hasStoryboards ? 'pending' : 'pending',
          hint: serialFirstClipTotal > 0 ? `首帧 ${serialFirstClipReady}/${serialFirstClipTotal}` : hasStoryboards ? '待生成场景首帧' : '需先完成镜头拆分',
        },
        {
          key: 'video',
          label: '串行视频',
          status: 'pending',
          hint: hasRenderableStoryboard ? '可开始串行视频链' : '需先准备场景首帧',
        },
      ]
    : [
        {
          key: 'script',
          label: '剧本拆分',
          status: episodeCount > 0 ? 'done' : project.progress?.stage === 'episode_splitting' || project.status === 'script_processing' ? 'current' : 'pending',
          hint: episodeCount > 0 ? `已生成 ${episodeCount} 集` : project.target_episodes > 0 ? `目标 ${project.target_episodes} 集` : '等待拆分',
        },
        {
          key: 'assets',
          label: '资源提取',
          status: assetFailed > 0 && assetCompleted === 0 && assetActive === 0 ? 'failed' : hasAssets && assetActive === 0 && assetFailed === 0 ? 'done' : assetActive > 0 || hasAssets ? 'current' : episodeCount > 0 ? 'pending' : 'pending',
          hint: hasAssets ? `已完成 ${assetCompleted}/${assetTotal}` : episodeCount > 0 ? '待开始' : '依赖分集结果',
        },
        {
          key: 'storyboard-split',
          label: '镜头拆分',
          status: storyboardFailed > 0 && storyboardTotal === 0 && storyboardActive === 0 ? 'failed' : hasStoryboards && storyboardActive === 0 ? 'done' : storyboardActive > 0 || episodeSplitting > 0 || hasStoryboards || isExtractingStoryboards ? 'current' : episodeCount > 0 ? 'pending' : 'pending',
          hint: hasStoryboards ? `已拆分 ${storyboardTotal} 条镜头` : episodeCount > 0 ? '待开始' : '依赖分集结果',
        },
        {
          key: 'storyboard-images',
          label: '分镜图片',
          status: storyboardFailed > 0 && storyboardImageReady === 0 && storyboardActive === 0 ? 'failed' : hasStoryboards && storyboardImageReady === storyboardTotal && storyboardActive === 0 && storyboardFailed === 0 ? 'done' : storyboardActive > 0 || storyboardImageReady > 0 || hasStoryboards ? 'current' : episodeCount > 0 ? 'pending' : 'pending',
          hint: hasStoryboards ? `已出图 ${storyboardImageReady}/${storyboardTotal}` : episodeCount > 0 ? '待开始' : '依赖分集结果',
        },
        {
          key: 'video',
          label: '视频成片',
          status: 'pending',
          hint: hasRenderableStoryboard ? '可开始生成视频' : '需先完成分镜图片',
        },
      ]

  const notices: ProjectOverviewNotice[] = []
  if (project.progress?.stage === 'episode_splitting') {
    notices.push({
      tone: 'blue',
      title: '系统正在拆分分集',
      description: '当前仍在根据剧本生成分集结构，请稍候查看最新结果。',
    })
  }
  if (assetActive > 0) {
    notices.push({
      tone: 'blue',
      title: '资源阶段进行中',
      description: `当前有 ${assetActive} 个资源仍在提取或生成，无需重复提交。`,
    })
  }
  if (episodeSplitting > 0 || storyboardActive > 0 || isExtractingStoryboards) {
    notices.push({
      tone: 'blue',
      title: isSerial ? '系统正在推进镜头与首帧阶段' : '系统正在推进镜头与出图阶段',
      description: `当前有 ${episodeSplitting || storyboardActive || 1} 个分集仍在处理中，可稍后刷新查看。`,
    })
  }
  if (assetFailed > 0) {
    notices.push({
      tone: 'red',
      title: '项目存在资源异常',
      description: `当前共有 ${assetFailed} 个资源失败，建议定位到相关分集优先处理。`,
    })
  }
  if (storyboardFailed > 0) {
    notices.push({
      tone: 'red',
      title: isSerial ? '项目存在镜头或首帧异常' : '项目存在分镜异常',
      description: `当前共有 ${storyboardFailed} 条分镜失败，建议定位到相关分集核查。`,
    })
  }

  let nextAction: ProjectOverviewAction
  if (firstStoryboardFailureEpisodeId) {
    nextAction = {
      title: isSerial ? '优先处理镜头或首帧异常' : '优先处理分镜异常',
      description: '项目内已有失败项，建议先进入对应分集工作台查看失败原因。',
      cta: '定位问题分集',
      type: 'select_episode',
      tab: 'storyboard',
      episodeId: firstStoryboardFailureEpisodeId,
    }
  } else if (firstAssetFailureEpisodeId) {
    nextAction = {
      title: '优先处理资源异常分集',
      description: '项目内已有资源失败项，建议先进入对应分集工作台核查。',
      cta: '定位问题分集',
      type: 'select_episode',
      tab: 'assets',
      episodeId: firstAssetFailureEpisodeId,
    }
  } else if (episodeCount === 0) {
    nextAction = {
      title: '先完成剧本拆分',
      description: '请先在下方剧本大纲区域继续生成分集，后续才能进入资源与镜头阶段。',
      cta: '查看下方剧本区',
      type: 'noop',
    }
  } else if (!hasAssets && assetActive === 0) {
    nextAction = {
      title: '建议先提取项目资源',
      description: '资源提取是镜头拆分与出图的统一起点，完成后系统会自动推进后续阶段。',
      cta: '开始资源提取',
      type: 'extract_assets',
    }
  } else if (assetActive > 0) {
    nextAction = {
      title: '等待资源处理完成',
      description: '当前资源正在提取或生成中，可先进入相关分集查看进度。',
      cta: '查看资源进度',
      type: 'select_episode',
      tab: 'assets',
      episodeId: firstAssetReadyEpisodeId,
      disabled: firstAssetReadyEpisodeId == null,
    }
  } else if (!hasStoryboards && !isExtractingStoryboards) {
    nextAction = {
      title: isSerial ? '建议开始镜头拆分与场景分组' : '建议开始镜头拆分',
      description: isSerial ? '当前已有资源，可为整个项目统一拆分镜头并生成串行场景分组。' : '当前已有资源，可为整个项目统一拆分镜头条目。',
      cta: isSerial ? '开始镜头拆分' : '开始镜头拆分',
      type: 'extract_storyboards',
    }
  } else if (isSerial && hasStoryboards && sceneGroupCount === 0) {
    nextAction = {
      title: '先核查场景分组',
      description: '串行模式依赖场景分组来衔接末帧链路，建议先进入镜头工作台检查分组结果。',
      cta: '查看场景分组',
      type: 'select_episode',
      tab: 'storyboard',
      episodeId: firstStoryboardReadyEpisodeId ?? firstAssetReadyEpisodeId,
    }
  } else if (isSerial && hasStoryboards && serialFirstClipReady === 0) {
    nextAction = {
      title: '先准备场景首帧',
      description: '串行模式只需准备每个场景组的首帧图片，后续镜头会沿用前一片段末帧。',
      cta: '进入镜头工作台',
      type: 'select_episode',
      tab: 'storyboard',
      episodeId: firstStoryboardReadyEpisodeId ?? firstAssetReadyEpisodeId,
    }
  } else if (!isSerial && hasStoryboards && storyboardImageReady === 0) {
    nextAction = {
      title: '先生成分镜图片',
      description: '镜头条目已经拆分完成，下一步需要批量生成分镜图片后再进入视频阶段。',
      cta: '进入分镜图片',
      type: 'select_episode',
      tab: 'storyboard',
      episodeId: firstStoryboardReadyEpisodeId ?? firstAssetReadyEpisodeId,
    }
  } else if (firstStoryboardActiveEpisodeId) {
    nextAction = {
      title: isSerial ? '查看首帧或镜头处理中分集' : '查看分镜处理中分集',
      description: '项目内已有分集进入处理中，建议优先查看当前最活跃的工作台。',
      cta: '打开处理中分集',
      type: 'select_episode',
      tab: 'storyboard',
      episodeId: firstStoryboardActiveEpisodeId,
    }
  } else if (hasRenderableStoryboard) {
    nextAction = dubbingEnabled
      ? {
          title: '可以继续处理配音或字幕',
          description: '当前已有可用于成片的画面内容，可进入单集工作台继续处理语音与字幕。',
          cta: '进入配音工作台',
          type: 'select_episode',
          tab: 'dubbing',
          episodeId: firstStoryboardImageReadyEpisodeId ?? firstStoryboardReadyEpisodeId ?? firstAssetReadyEpisodeId,
        }
      : {
          title: isSerial ? '可以进入串行视频阶段' : '可以进入视频成片',
          description: '当前已有可用于成片的画面内容，可直接进入视频阶段继续制作。',
          cta: '进入视频工作台',
          type: 'select_episode',
          tab: 'video',
          episodeId: firstStoryboardImageReadyEpisodeId ?? firstStoryboardReadyEpisodeId ?? firstAssetReadyEpisodeId,
        }
  } else {
    nextAction = {
      title: '先进入某一集开始处理',
      description: '可以先打开单集工作台，逐集管理资源、镜头、出图与成片流程。',
      cta: '打开第一集',
      type: 'select_episode',
      tab: 'assets',
      episodeId: firstAssetReadyEpisodeId ?? firstStoryboardReadyEpisodeId,
      disabled: episodeCount === 0,
    }
  }

  return { workflowSteps, notices, nextAction }
}

export function toPercent(completed: number, total: number): number | null {
  if (total <= 0) return null
  return Math.max(0, Math.min(100, Math.round((completed / total) * 100)))
}

export function getIssueStepIndex(steps: WorkflowStepView[]): number {
  let activeIndex = -1
  for (let i = steps.length - 1; i >= 0; i -= 1) {
    if (steps[i].status === 'current' || steps[i].status === 'failed') {
      activeIndex = i
      break
    }
  }
  if (activeIndex >= 0) return activeIndex

  const firstIncomplete = steps.findIndex((step) => step.status !== 'done' && step.status !== 'skipped')
  if (firstIncomplete >= 0) return firstIncomplete

  return steps.length - 1
}

export function getDisplayedEpisodeCount(project: Project, episodeCount: number): number {
  const splitTotal = project.progress?.episode_split?.total ?? 0
  if (project.progress?.stage === 'episode_splitting') {
    if (splitTotal > 0) return splitTotal
    if (project.target_episodes > 0) return project.target_episodes
  }
  if (project.progress?.stage === 'scene_splitting' && splitTotal > 0) {
    return splitTotal
  }
  return episodeCount
}

export function buildWorkflowSteps({
  project,
  episodeCount = 0,
  assetStats,
  storyboardStats,
  dubbingStats,
  videoStats,
}: {
  project: Project
  episodeCount?: number
  assetStats?: StepAssetStats
  storyboardStats?: StepStoryboardStats
  dubbingStats?: StepDubbingStats
  videoStats?: StepVideoStats
}): WorkflowStepView[] {
  const progress = project.progress
  const scriptStage = progress?.stage
  const episodeSplitTotal = progress?.episode_split?.total ?? 0
  const episodeSplitCompleted = progress?.episode_split?.completed ?? 0
  const sceneSplitTotal = progress?.scene_split?.total ?? 0
  const sceneSplitCompleted = progress?.scene_split?.completed ?? 0
  const displayedEpisodeCount = getDisplayedEpisodeCount(project, episodeCount)
  const scriptPreparing = scriptStage === 'script_prepping'
  const scriptRunning = scriptStage === 'episode_splitting' || scriptPreparing || (project.status === 'script_processing' && !scriptStage)
  const scriptDone = displayedEpisodeCount > 0 && scriptStage !== 'episode_splitting' && scriptStage !== 'script_prepping'

  const assets = assetStats ?? {
    total: 0,
    completed: 0,
    pending: 0,
    extracting: 0,
    generating: 0,
    failed: 0,
  }
  const assetsProcessing = assets.extracting + assets.generating
  const assetsDone = assets.total > 0 && assets.extracting === 0
  const assetsFailed = assets.failed > 0 && assets.extracting === 0 && assets.total === 0
  const assetsStarted = assets.total > 0 || assetsProcessing > 0 || ['asset_generating', 'asset_ready', 'storyboard_generating', 'storyboard_ready', 'video_generating', 'completed'].includes(project.status)

  const storyboards = storyboardStats ?? {
    total: 0,
    completed: 0,
    pending: 0,
    paused: 0,
    generating: 0,
    failed: 0,
  }
  const storyboardRunning = storyboards.generating > 0 || project.status === 'storyboard_generating' || progress?.stage === 'scene_splitting'
  const storyboardReadyToStart = storyboards.pending
  const storyboardPaused = storyboards.paused
  const storyboardRetryNeeded = storyboards.failed
  const storyboardDone = storyboards.total > 0
    && storyboards.generating === 0
    && storyboardReadyToStart === 0
    && storyboardPaused === 0
    && storyboardRetryNeeded === 0
  const storyboardFailed = storyboardRetryNeeded > 0
    && storyboards.generating === 0
    && storyboards.completed === 0
    && storyboardReadyToStart === 0
    && storyboardPaused === 0
  const storyboardStarted = storyboards.total > 0 || storyboardRunning || ['storyboard_ready', 'video_generating', 'completed'].includes(project.status)

  const dubbingEnabled = project.enable_dubbing || project.enable_subtitle
  const dubbing = dubbingStats ?? {
    total: 0,
    completed: 0,
    processing: 0,
    failed: 0,
  }
  const dubbingDone = dubbingEnabled && dubbing.total > 0 && dubbing.completed === dubbing.total
  const dubbingFailed = dubbingEnabled && dubbing.failed > 0 && dubbing.processing === 0
  const dubbingStarted = dubbingEnabled && (dubbing.total > 0 || dubbing.processing > 0 || project.status === 'video_generating' || project.status === 'completed')

  const videos = videoStats ?? {
    total: 0,
    completed: 0,
    pending: 0,
    processing: 0,
    failed: 0,
    paused: 0,
  }
  const videoRunning = videos.processing > 0 || project.status === 'video_generating'
  const videoPaused = videos.paused > 0 && videos.processing === 0 && videos.failed === 0 && !videoRunning
  const videoDone = project.status === 'completed' || (videos.total > 0 && videos.completed === videos.total)
  const videoFailed = videos.failed > 0 && videos.processing === 0
  const videoStarted = videos.total > 0 || videoRunning || project.status === 'completed'

  const steps: WorkflowStepView[] = WORKFLOW_STEPS.map((step) => {
    switch (step.key) {
      case 'script':
        return {
          ...step,
          status: scriptRunning ? 'current' : scriptDone ? 'done' : 'current',
          processing: scriptRunning,
          subLabel: scriptRunning
            ? scriptPreparing
              ? '自动润色中'
              : episodeSplitTotal > 0
              ? `${episodeSplitCompleted}/${episodeSplitTotal}`
              : project.target_episodes > 0
                ? `0/${project.target_episodes}`
                : project.script_file_url
                  ? '拆分中'
                  : '未上传'
            : scriptDone
              ? `${displayedEpisodeCount} 集`
              : project.script_file_url
                ? '待拆分'
                : '未上传',
          progress: scriptRunning
            ? scriptPreparing
              ? toPercent(sceneSplitCompleted, sceneSplitTotal || displayedEpisodeCount)
              : toPercent(episodeSplitCompleted, episodeSplitTotal)
            : scriptDone
              ? 100
              : project.script_file_url
                ? 0
                : null,
        }

      case 'assets':
        return {
          ...step,
          status: assetsDone
            ? 'done'
            : assetsFailed
              ? 'failed'
              : assetsProcessing > 0
                ? 'current'
                : assetsStarted && scriptDone
                  ? 'current'
                  : 'pending',
          processing: assets.extracting > 0,
          subLabel: assets.extracting > 0
            ? '提取中'
            : assets.total > 0
              ? `已提取 ${assets.total} 项`
              : scriptDone
                ? '待提取'
                : null,
          progress: assets.extracting > 0
            ? null
            : assets.total > 0
              ? 100
              : null,
        }

      case 'storyboard':
        return {
          ...step,
          status: storyboardDone
            ? 'done'
            : storyboardFailed
              ? 'failed'
              : storyboardRunning || (storyboardStarted && scriptDone)
                ? 'current'
                : 'pending',
          processing: storyboardRunning,
          subLabel: progress?.stage === 'scene_splitting' && sceneSplitTotal > 0
            ? `拆分 ${sceneSplitCompleted}/${sceneSplitTotal} 集`
            : storyboards.total > 0
              ? [
                  `已建 ${storyboards.total} 个`,
                  storyboardReadyToStart > 0 ? `待生成 ${storyboardReadyToStart}` : null,
                  storyboardPaused > 0 ? `已暂停 ${storyboardPaused}` : null,
                  storyboardRetryNeeded > 0 ? `失败 ${storyboardRetryNeeded}` : null,
                ].filter(Boolean).join(' · ')
              : scriptDone
                ? '待生成'
                : null,
          progress: progress?.stage === 'scene_splitting'
            ? toPercent(sceneSplitCompleted, sceneSplitTotal)
            : storyboards.total > 0
              ? storyboardDone
                ? 100
                : toPercent(storyboards.completed, storyboards.total)
              : null,
        }


      case 'dubbing':
        return {
          ...step,
          status: !dubbingEnabled
            ? 'skipped'
            : dubbingDone
              ? 'done'
              : dubbingFailed
                ? 'failed'
                : dubbing.processing > 0
                  ? 'current'
                  : dubbingStarted && storyboardStarted
                    ? 'current'
                    : 'pending',
          processing: dubbing.processing > 0,
          subLabel: !dubbingEnabled
            ? '未启用'
            : dubbing.total > 0
              ? `${dubbing.completed}/${dubbing.total}`
              : storyboardDone
                ? '待生成'
                : null,
          progress: !dubbingEnabled
            ? null
            : dubbing.total > 0
              ? toPercent(dubbing.completed, dubbing.total)
              : null,
        }

      case 'video':
        return {
          ...step,
          status: videoDone
            ? 'done'
            : videoFailed
              ? 'failed'
              : videoPaused
                ? 'paused'
                : videoRunning
                  ? 'current'
                  : videoStarted
                    ? 'current'
                    : 'pending',
          processing: videoRunning,
          subLabel: videos.total > 0
            ? [
                `${videos.completed}/${videos.total}`,
                videos.paused > 0 ? `已暂停 ${videos.paused}` : null,
                videos.failed > 0 ? `失败 ${videos.failed}` : null,
              ].filter(Boolean).join(' · ')
            : storyboardDone
              ? '待生成'
              : null,
          progress: videos.total > 0 ? toPercent(videos.completed, videos.total) : null,
        }
    }
  })

  const issueStepIndex = getIssueStepIndex(steps)

  if (project.status === 'failed' && issueStepIndex >= 0 && steps[issueStepIndex].status !== 'done' && steps[issueStepIndex].status !== 'skipped') {
    steps[issueStepIndex] = { ...steps[issueStepIndex], status: 'failed', processing: false }
  } else if (project.status === 'paused' && issueStepIndex >= 0 && steps[issueStepIndex].status !== 'done' && steps[issueStepIndex].status !== 'skipped' && steps[issueStepIndex].status !== 'failed') {
    steps[issueStepIndex] = { ...steps[issueStepIndex], status: 'paused', processing: false }
  }

  return steps
}
