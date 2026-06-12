import type { Episode, Project, ProjectProgress } from '@/types'

export type VideoPipelinePhase =
  | 'episode_splitting'
  | 'script_prepping'
  | 'scene_splitting'
  | 'asset_working'
  | 'idle'

export type VideoPipelineStageKey = 'split' | 'assets' | 'storyboard'

export type VideoPipelineStageView = {
  key: VideoPipelineStageKey
  label: string
  status: 'pending' | 'running' | 'done'
  detail: string
  progress: number
}

export type VideoPipelineSnapshot = {
  phase: VideoPipelinePhase
  isActive: boolean
  activeLabel: string
  activeDetail: string
  episodeSplitDone: boolean
  scriptPrepActive: boolean
  sceneSplitActive: boolean
  assetWorkActive: boolean
  stages: VideoPipelineStageView[]
  nextStepHint: string
}

type PipelineInput = {
  project: Pick<Project, 'status' | 'progress'>
  episodes: Episode[]
  episodeGenerating?: boolean
  assetExtracting?: boolean
  assetGenerating?: boolean
  storyboardGenerating?: boolean
}

function progressOf(project: Pick<Project, 'progress'>): ProjectProgress | undefined {
  return project.progress
}

export function isEpisodeSplitting(input: PipelineInput): boolean {
  const { project, episodes, episodeGenerating } = input
  const stage = progressOf(project)?.stage
  if (episodeGenerating) return true
  if (episodes.length > 0) return false
  return (
    project.status === 'script_processing'
    || stage === 'episode_splitting'
  )
}

export function isScriptPrepping(input: PipelineInput): boolean {
  const { project, episodes } = input
  if (episodes.length === 0) return false
  return progressOf(project)?.stage === 'script_prepping'
}

export function isSceneSplitting(input: PipelineInput): boolean {
  const { project, episodes, storyboardGenerating } = input
  const stage = progressOf(project)?.stage
  if (stage === 'scene_splitting') return true
  if (project.status === 'storyboard_generating' || storyboardGenerating) return true
  return episodes.some((ep) => ep.status === 'scene_splitting')
}

export function isAssetWorking(input: PipelineInput): boolean {
  const { project, assetExtracting, assetGenerating } = input
  if (assetExtracting || assetGenerating) return true
  return project.status === 'asset_generating'
}

export function isPipelineActive(input: PipelineInput): boolean {
  return (
    isEpisodeSplitting(input)
    || isScriptPrepping(input)
    || isSceneSplitting(input)
    || isAssetWorking(input)
  )
}

function sceneReadyCount(episodes: Episode[]): number {
  return episodes.filter((ep) => ep.status === 'scene_ready' || ep.status === 'done').length
}

function sceneSplitDone(project: Pick<Project, 'progress'>, episodes: Episode[]): boolean {
  if (episodes.length === 0) return false
  const sceneProgress = progressOf(project)?.scene_split
  if (sceneProgress?.status === 'done') return true
  return sceneReadyCount(episodes) >= episodes.length
}

function episodeSplitDone(project: Pick<Project, 'progress'>, episodes: Episode[]): boolean {
  if (episodes.length === 0) return false
  const episodeProgress = progressOf(project)?.episode_split
  if (episodeProgress?.status === 'done') return true
  return episodes.length > 0 && progressOf(project)?.stage !== 'episode_splitting'
}

function formatProgressMessage(progress?: ProjectProgress): string | null {
  const message = progress?.message?.trim()
  if (message) return message
  const phaseLabel = progress?.phase_label?.trim()
  if (phaseLabel) return phaseLabel
  return null
}

export function deriveVideoPipelineSnapshot(input: PipelineInput): VideoPipelineSnapshot {
  const { project, episodes } = input
  const progress = progressOf(project)
  const splitDone = episodeSplitDone(project, episodes)
  const scriptPrepActive = isScriptPrepping(input)
  const sceneSplitActive = isSceneSplitting(input)
  const assetWorkActive = isAssetWorking(input)
  const episodeSplitActive = isEpisodeSplitting(input)
  const storyboardDone = sceneSplitDone(project, episodes)

  let phase: VideoPipelinePhase = 'idle'
  if (episodeSplitActive) phase = 'episode_splitting'
  else if (scriptPrepActive) phase = 'script_prepping'
  else if (sceneSplitActive) phase = 'scene_splitting'
  else if (assetWorkActive) phase = 'asset_working'

  const progressMessage = formatProgressMessage(progress)
  const nextStepHint = progress?.next_step?.trim()
    || (splitDone && !storyboardDone ? '分集已完成。系统已自动处理第 1 集作为示范，其余分集请在左侧列表点击「自动处理」。' : '上传剧本后会自动开始分集。')

  const episodeSplitTotal = progress?.episode_split?.total ?? episodes.length
  const episodeSplitCompleted = progress?.episode_split?.completed ?? (splitDone ? episodes.length : 0)
  const sceneSplitTotal = Math.max(progress?.scene_split?.total ?? 0, episodes.length)
  const sceneSplitCompleted = Math.max(progress?.scene_split?.completed ?? 0, sceneReadyCount(episodes))

  const splitStage: VideoPipelineStageView = {
    key: 'split',
    label: '剧本分集',
    status: episodeSplitActive || scriptPrepActive ? 'running' : splitDone ? 'done' : 'pending',
    detail: episodeSplitActive
      ? (progressMessage || (episodeSplitTotal > 0
        ? `正在识别分集结构（${episodeSplitCompleted}/${episodeSplitTotal}）`
        : '正在分析剧本结构与章节边界'))
      : scriptPrepActive
        ? (progressMessage || `分集已完成（${episodes.length} 集），正在润色剧本并自动准备后续流程`)
        : splitDone
          ? `已生成 ${episodes.length} 集，可在下方查看与编辑`
          : '上传剧本后自动拆分',
    progress: episodeSplitActive
      ? (episodeSplitTotal > 0 ? episodeSplitCompleted / Math.max(episodeSplitTotal, 1) : 0.35)
      : scriptPrepActive
        ? Math.max(0.6, sceneSplitTotal > 0 ? sceneSplitCompleted / Math.max(sceneSplitTotal, 1) : 0.6)
        : splitDone
          ? 1
          : 0,
  }

  const assetsStage: VideoPipelineStageView = {
    key: 'assets',
    label: '资源提取',
    status: assetWorkActive ? 'running' : 'pending',
    detail: assetWorkActive
      ? (progressMessage || '正在识别角色、场景、道具等资源')
      : splitDone
        ? '可在剧本页手动提取，或进入各集工作台继续处理'
        : '等待分集完成',
    progress: assetWorkActive ? 0.35 : 0,
  }

  const storyboardStage: VideoPipelineStageView = {
    key: 'storyboard',
    label: '分镜拆分',
    status: sceneSplitActive ? 'running' : storyboardDone ? 'done' : 'pending',
    detail: sceneSplitActive
      ? (progressMessage || (sceneSplitTotal > 0
        ? `正在拆分镜头（${sceneSplitCompleted}/${sceneSplitTotal} 集）`
        : '正在为各集拆分镜头序列'))
      : storyboardDone
        ? `已完成 ${sceneReadyCount(episodes)}/${episodes.length} 集分镜拆分`
        : splitDone
          ? '分集已完成，可在各集点击「生成分镜」开始拆分'
          : '等待分集完成',
    progress: storyboardDone
      ? 1
      : sceneSplitActive && sceneSplitTotal > 0
        ? sceneSplitCompleted / Math.max(sceneSplitTotal, 1)
        : sceneSplitActive
          ? 0.25
          : 0,
  }

  let activeLabel = '当前无进行中的自动任务'
  let activeDetail = nextStepHint

  if (episodeSplitActive) {
    activeLabel = '剧本分集中'
    activeDetail = splitStage.detail
  } else if (scriptPrepActive) {
    activeLabel = '剧本润色与自动准备中'
    activeDetail = splitStage.detail
  } else if (sceneSplitActive) {
    activeLabel = '分镜拆分中'
    activeDetail = storyboardStage.detail
  } else if (assetWorkActive) {
    activeLabel = '资源处理中'
    activeDetail = assetsStage.detail
  } else if (splitDone) {
    activeLabel = '分集已完成'
    activeDetail = nextStepHint
  }

  return {
    phase,
    isActive: isPipelineActive(input),
    activeLabel,
    activeDetail,
    episodeSplitDone: splitDone,
    scriptPrepActive,
    sceneSplitActive,
    assetWorkActive,
    stages: [splitStage, assetsStage, storyboardStage],
    nextStepHint,
  }
}
