import { useMemo, useState } from 'react'
import useSWR from 'swr'
import { assetAPI, projectAPI, storyboardAPI } from '@/lib/api'
import { buildEpisodeWorkspaceMeta, countVisibleAssets, isAssetExtractionRunning } from '@/lib/projects/build-episode-workspace-meta'
import { prefersAutoEpisodeSplit } from '@/lib/projects/episode-split'
import { deriveVideoPipelineSnapshot, type VideoPipelineStageView } from '@/lib/projects/pipeline-status'
import { isStoryboardImageActive } from '@/lib/projects/storyboard-status'
import { buildProjectOverview } from '@/lib/projects/workflow'
import type { Episode, Project } from '@/types'

export type ProjectDetailVariant = 'standard' | 'serial'

export function useProjectDetailData(projectId: number, enabled: boolean) {
  const [isExtractingStoryboards, setIsExtractingStoryboards] = useState(false)
  const [isExtractingAssets, setIsExtractingAssets] = useState(false)
  const [isGeneratingProjectImages, setIsGeneratingProjectImages] = useState(false)

  const { data, isLoading, mutate: mutateProject } = useSWR(
    enabled ? ['project', projectId] : null,
    () => projectAPI.get(projectId) as unknown as Promise<{ data: Project }>,
    { refreshInterval: 3000 },
  )
  const project = data?.data

  const { data: episodesData } = useSWR(
    enabled && project ? ['stepper-episodes', projectId] : null,
    () => projectAPI.listEpisodes(projectId) as unknown as Promise<{ data: Episode[] }>,
    { refreshInterval: 5000 },
  )
  const episodes = useMemo(() => episodesData?.data ?? [], [episodesData])

  const { data: assetsCountData, mutate: mutateAssetsCount } = useSWR(
    enabled && project ? ['stepper-assets', projectId] : null,
    () => assetAPI.list(projectId) as unknown as Promise<{ data: any[] }>,
    { refreshInterval: 5000 },
  )
  const stepperAssetsRaw = useMemo(
    () => (assetsCountData as { data?: any[] })?.data ?? [],
    [assetsCountData],
  )

  const { data: storyboardsData, mutate: mutateStoryboardsData } = useSWR(
    enabled && project ? ['stepper-storyboards', projectId] : null,
    () => storyboardAPI.listAll(projectId) as Promise<{ data: any[] }>,
    { refreshInterval: 5000 },
  )
  const stepperStoryboardsRaw = useMemo(
    () => (storyboardsData as { data?: any[] })?.data ?? [],
    [storyboardsData],
  )

  const episodeWorkspaceMeta = useMemo(
    () => buildEpisodeWorkspaceMeta(episodes, stepperAssetsRaw, stepperStoryboardsRaw),
    [episodes, stepperAssetsRaw, stepperStoryboardsRaw],
  )

  const projectSharedAssetCount = useMemo(
    () => countVisibleAssets(stepperAssetsRaw),
    [stepperAssetsRaw],
  )

  const hasUsableScriptContent = useMemo(() => {
    const projectScriptText = project?.script_text?.trim() ?? ''
    if (projectScriptText) return true
    return episodes.some((episode) => Boolean(
      episode.optimized_text?.trim()
      || episode.script_excerpt?.trim()
      || episode.summary?.trim(),
    ))
  }, [episodes, project?.script_text])

  const assetExtractionRunning = useMemo(
    () => isAssetExtractionRunning(stepperAssetsRaw),
    [stepperAssetsRaw],
  )

  const pipelineSnapshot = useMemo(() => deriveVideoPipelineSnapshot({
    project: (project ?? { status: 'draft', progress: undefined }) as Pick<Project, 'status' | 'progress'>,
    episodes,
    assetExtracting: assetExtractionRunning || isExtractingAssets,
    storyboardGenerating: isExtractingStoryboards,
  }), [assetExtractionRunning, episodes, isExtractingAssets, isExtractingStoryboards, project])

  const projectOverview = useMemo(() => {
    if (!project) {
      return {
        workflowSteps: [] as ReturnType<typeof buildProjectOverview>['workflowSteps'],
        notices: [] as ReturnType<typeof buildProjectOverview>['notices'],
        nextAction: {
          title: '请先加载项目数据',
          description: '项目数据加载完成后，可继续查看剧本大纲与分集工作台。',
          cta: '稍候',
          type: 'noop' as const,
          tab: 'assets' as const,
          disabled: true,
        },
        stats: {
          episodeReady: 0,
          episodeFailed: 0,
          assetTotal: 0,
          assetCompleted: 0,
          assetActive: 0,
          assetFailed: 0,
          storyboardTotal: 0,
          storyboardCompleted: 0,
          storyboardActive: 0,
          storyboardFailed: 0,
        },
      }
    }

    const episodeReady = episodes.filter((ep) => ep.status === 'scene_ready').length
    const episodeFailed = episodes.filter((ep) => ep.status === 'failed').length
    const episodeSplitting = episodes.filter((ep) => ep.status === 'scene_splitting').length
    const stepperAssetsVisible = stepperAssetsRaw.filter((a) => a?.name !== '__extracting__' && a?.status !== 'extracting')
    const assetTotal = stepperAssetsVisible.length
    const assetCompleted = stepperAssetsVisible.filter((asset) => asset?.status === 'completed').length
    const assetActive = stepperAssetsVisible.filter((asset) => ['pending', 'generating', 'paused'].includes(asset?.status)).length
    const assetFailed = stepperAssetsVisible.filter((asset) => asset?.status === 'failed' || asset?.status === 'qa_failed').length
    const storyboardTotal = stepperStoryboardsRaw.length
    const storyboardCompleted = stepperStoryboardsRaw.filter((sb) => sb?.status === 'completed' && sb?.image_url).length
    const storyboardActive = stepperStoryboardsRaw.filter((sb) => isStoryboardImageActive(sb?.status)).length
    const storyboardFailed = stepperStoryboardsRaw.filter((sb) => sb?.status === 'failed').length
    const sceneGroupCount = new Set(stepperStoryboardsRaw.map((sb) => String(sb?.scene_group_key || '')).filter(Boolean)).size
    const serialFirstClipTotal = stepperStoryboardsRaw.filter((sb) => sb?.scene_group_key && sb?.is_scene_first_clip).length
    const serialFirstClipReady = stepperStoryboardsRaw.filter((sb) => sb?.scene_group_key && sb?.is_scene_first_clip && sb?.status === 'completed' && sb?.image_url).length

    const firstAssetFailureEpisodeId = episodes.find((ep) => (episodeWorkspaceMeta.get(ep.id)?.assetFailed ?? 0) > 0)?.id ?? null
    const firstStoryboardFailureEpisodeId = episodes.find((ep) => episodeWorkspaceMeta.get(ep.id)?.storyboardFailed)?.id ?? null
    const firstStoryboardActiveEpisodeId = episodes.find((ep) => episodeWorkspaceMeta.get(ep.id)?.storyboardGenerating || ep.status === 'scene_splitting')?.id ?? null
    const firstStoryboardReadyEpisodeId = episodes.find((ep) => (episodeWorkspaceMeta.get(ep.id)?.storyboardTotal ?? 0) > 0)?.id ?? null
    const firstStoryboardImageReadyEpisodeId = episodes.find((ep) => stepperStoryboardsRaw.some((sb) => Number(sb?.episode_id) === ep.id && sb?.status === 'completed' && sb?.image_url))?.id ?? null
    const firstAssetReadyEpisodeId = episodes.find((ep) => (episodeWorkspaceMeta.get(ep.id)?.assetTotal ?? 0) > 0)?.id ?? null

    const overview = buildProjectOverview({
      project,
      episodeCount: episodes.length,
      episodeSplitting,
      assetTotal,
      assetCompleted,
      assetActive,
      assetFailed,
      storyboardTotal,
      storyboardImageReady: storyboardCompleted,
      storyboardActive,
      storyboardFailed,
      isExtractingStoryboards,
      sceneGroupCount,
      serialFirstClipTotal,
      serialFirstClipReady,
      firstAssetFailureEpisodeId,
      firstStoryboardFailureEpisodeId,
      firstStoryboardActiveEpisodeId,
      firstStoryboardReadyEpisodeId,
      firstStoryboardImageReadyEpisodeId,
      firstAssetReadyEpisodeId,
    })

    return {
      ...overview,
      stats: {
        episodeReady,
        episodeFailed,
        assetTotal,
        assetCompleted,
        assetActive,
        assetFailed,
        storyboardTotal,
        storyboardCompleted,
        storyboardActive,
        storyboardFailed,
      },
    }
  }, [project, episodes, stepperAssetsRaw, stepperStoryboardsRaw, episodeWorkspaceMeta, isExtractingStoryboards])

  const projectProgress = project?.progress
  const projectTargetEpisodes = project?.target_episodes ?? 0
  const structuredPhaseLabel = projectProgress?.phase_label?.trim()
  const structuredNextStep = projectProgress?.next_step?.trim()
  const projectSplitTotal = projectProgress?.episode_split?.total ?? 0
  const projectSplitCompleted = projectProgress?.episode_split?.completed ?? 0

  const projectSplitTitle = structuredPhaseLabel
    || projectProgress?.message?.trim()
    || (projectSplitTotal > 0
      ? `正在生成分集结构（${projectSplitCompleted}/${projectSplitTotal}）`
      : prefersAutoEpisodeSplit(project)
        ? '正在按剧本内容自动拆分分集'
        : projectTargetEpisodes > 0
          ? `正在按目标 ${projectTargetEpisodes} 集拆分剧本`
          : '正在拆分剧本并生成分集结构')

  const projectSplitDescription = pipelineSnapshot.isActive
    ? pipelineSnapshot.activeDetail
    : structuredNextStep
      ? `${structuredNextStep}${projectProgress?.current_episode && projectProgress?.total_episodes ? ` 当前进度 ${projectProgress.current_episode}/${projectProgress.total_episodes}。` : ''}`
      : projectSplitTotal > 0
        ? `系统正在分析剧本、提取关键词并生成分集。当前已识别 ${projectSplitCompleted}/${projectSplitTotal} 个分集草稿，完成后左侧分集列表会自动出现。`
        : pipelineSnapshot.episodeSplitDone
          ? pipelineSnapshot.nextStepHint
          : '系统正在分析剧本内容并自动生成分集。生成完成后，左侧会自动出现分集列表。'

  const projectAssetEntries = stepperAssetsRaw.filter(
    (asset) => asset?.name !== '__extracting__' && asset?.status !== 'extracting',
  )
  const projectAssetCompleted = projectAssetEntries.filter((asset) => asset?.status === 'completed').length
  const projectAssetActive = isExtractingAssets
    || assetExtractionRunning
    || projectAssetEntries.some((asset) => ['pending', 'generating', 'paused'].includes(asset?.status))
  const projectAssetDone = projectAssetEntries.length > 0 && !projectAssetActive

  const projectControlStages: VideoPipelineStageView[] = useMemo(() => pipelineSnapshot.stages.map((stage) => {
    if (stage.key !== 'assets') return stage
    return {
      ...stage,
      status: projectAssetActive ? 'running' : projectAssetDone ? 'done' : stage.status,
      detail: projectAssetActive
        ? (projectAssetEntries.length > 0
          ? `已完成 ${projectAssetCompleted}/${projectAssetEntries.length} 项资源，剩余资源仍在处理中`
          : stage.detail)
        : projectAssetDone
          ? `已准备 ${projectAssetEntries.length} 项资源，可继续后续镜头处理`
          : stage.detail,
      progress: projectAssetDone
        ? 1
        : projectAssetEntries.length > 0
          ? projectAssetCompleted / Math.max(projectAssetEntries.length, 1)
          : projectAssetActive
            ? 0.2
            : stage.progress,
    }
  }), [pipelineSnapshot.stages, projectAssetActive, projectAssetDone, projectAssetEntries.length, projectAssetCompleted])

  const projectControlDoneCount = projectControlStages.filter((stage) => stage.status === 'done').length
  const projectControlOverallProgress = Math.round(
    (projectControlStages.reduce((sum, stage) => sum + Math.min(Math.max(stage.progress, 0), 1), 0) / projectControlStages.length) * 100,
  )
  const projectControlCurrentStage = projectControlStages.find((stage) => stage.status === 'running')

  const firstEpisodeAutoActive = pipelineSnapshot.isActive
    && (pipelineSnapshot.phase === 'script_prepping' || pipelineSnapshot.phase === 'scene_splitting')

  return {
    project,
    episodes,
    isLoading,
    mutateProject,
    mutateAssetsCount,
    mutateStoryboardsData,
    stepperAssetsRaw,
    stepperStoryboardsRaw,
    episodeWorkspaceMeta,
    projectSharedAssetCount,
    hasUsableScriptContent,
    assetExtractionRunning,
    pipelineSnapshot,
    projectOverview,
    projectProgress,
    structuredPhaseLabel,
    structuredNextStep,
    projectSplitTotal,
    projectSplitCompleted,
    projectSplitTitle,
    projectSplitDescription,
    projectControlStages,
    projectControlDoneCount,
    projectControlOverallProgress,
    projectControlCurrentStage,
    firstEpisodeAutoActive,
    isExtractingStoryboards,
    setIsExtractingStoryboards,
    isExtractingAssets,
    setIsExtractingAssets,
    isGeneratingProjectImages,
    setIsGeneratingProjectImages,
  }
}
