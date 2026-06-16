'use client'

import { useEffect, useMemo, useRef, useState } from 'react'
import useSWR, { mutate as globalMutate } from 'swr'
import { assetAPI, modelAPI, projectAPI, storyboardAPI } from '@/lib/api'
import { useToast } from '@/components/ui/toast'
import type { Asset, Episode, Model, Project, Storyboard } from '@/types'
import { parseAssetExtractionState, parseExtractionStatusResponse, resolveExtractionModelName, type AssetExtractionStatusResponse } from '@/lib/projects/asset-extraction'
import { pickPreferredModel } from '@/lib/model-selection'
import { getEpisodeWorkspaceLabels } from '@/lib/projects/episode-workspace-labels'
import {
  computeEpisodeAssetStats,
  computeEpisodeStoryboardStats,
  computeSerialStoryboardStats,
} from '@/lib/projects/episode-workspace-stats'
import { computeEpisodePipelineStatus } from '@/lib/projects/episode-workspace-pipeline-status'
import { computeEpisodeWorkflowSteps, type WorkflowStepKey } from '@/lib/projects/episode-workspace-workflow-steps'

export type EpisodeWorkspaceTab = WorkflowStepKey

export function useEpisodeWorkspaceData({
  projectId,
  episodeId,
  episode,
  project,
  initialTab = 'assets',
  initialAwaitingAutoStoryboard = false,
  autoPipelineActive = false,
}: {
  projectId: number
  episodeId: number
  episode?: Episode
  project: Project
  initialTab?: EpisodeWorkspaceTab
  initialAwaitingAutoStoryboard?: boolean
  autoPipelineActive?: boolean
}) {
  const { toast } = useToast()
  const isSerial = project.project_type === 'video_serial'
  const labels = getEpisodeWorkspaceLabels(isSerial)

  const [activeTab, setActiveTab] = useState<EpisodeWorkspaceTab>(initialTab)
  const [isExtracting, setIsExtracting] = useState(false)
  const [isExtractingStoryboards, setIsExtractingStoryboards] = useState(false)
  const [awaitingAutoStoryboard, setAwaitingAutoStoryboard] = useState(initialAwaitingAutoStoryboard)
  const autoSwitchedRef = useRef(false)

  const [generateTrigger, setGenerateTrigger] = useState(0)
  const [regenerateTrigger, setRegenerateTrigger] = useState(0)
  const [sbGenerateTrigger, setSbGenerateTrigger] = useState(0)
  const [sbRegenerateTrigger, setSbRegenerateTrigger] = useState(0)
  const [sbPauseTrigger, setSbPauseTrigger] = useState(0)
  const [sbResumeTrigger, setSbResumeTrigger] = useState(0)
  const [sbAuditTrigger, setSbAuditTrigger] = useState(0)
  const [autoMatchingVoices, setAutoMatchingVoices] = useState(false)
  const [pausingGeneration, setPausingGeneration] = useState(false)
  const [resumingGeneration, setResumingGeneration] = useState(false)
  const [extractionModelKey, setExtractionModelKey] = useState('')
  const extractionFailureNotifiedRef = useRef(false)

  const { data: textModelsData } = useSWR(
    ['episode-workspace-text-models', projectId],
    () => modelAPI.list({ type: 'llm', sort_by: 'priority' }) as unknown as Promise<{ data: Model[] }>,
  )
  const textModels = (textModelsData as { data?: Model[] })?.data?.filter((model) => model.is_active || model.id === project.text_model_id) ?? []
  const preferredExtractionModel = pickPreferredModel(textModels)
  const effectiveExtractionModelKey = extractionModelKey || preferredExtractionModel?.model_key || preferredExtractionModel?.name || ''

  const { data: assetsData, mutate: mutateAssets } = useSWR(
    ['episode-workspace-assets', projectId, episodeId],
    () => assetAPI.list(projectId, { episode_id: episodeId }) as unknown as Promise<{ data: Asset[] }>,
    {
      refreshInterval: (data) => {
        const items = (data as { data?: Asset[] })?.data ?? []
        const state = parseAssetExtractionState(items, episodeId)
        return awaitingAutoStoryboard || state.inProgress || state.failed || items.some((asset) => ['extracting', 'pending', 'generating'].includes(asset.status)) ? 3000 : 0
      },
    },
  )
  const episodeAssets = assetsData?.data ?? []

  const { data: extractionStatusData } = useSWR(
    ['assets-extraction-status', projectId, episodeId],
    () => assetAPI.getExtractionStatus(projectId, { episode_id: episodeId }) as unknown as Promise<{ data: AssetExtractionStatusResponse }>,
    {
      refreshInterval: (data) => {
        const state = parseExtractionStatusResponse((data as { data?: AssetExtractionStatusResponse } | undefined)?.data)
        return state.inProgress || state.failed ? 3000 : 15000
      },
    },
  )

  const { data: storyboardsData, mutate: mutateStoryboards } = useSWR(
    ['episode-workspace-storyboards', projectId, episodeId],
    () => storyboardAPI.listAll(projectId, { episode_id: episodeId }) as Promise<{ data: Storyboard[] }>,
    {
      refreshInterval: (data) => {
        const items = (data as { data?: Storyboard[] })?.data ?? []
        return awaitingAutoStoryboard || items.some((sb) => ['pending', 'generating', 'paused'].includes(sb.status)) ? 3000 : 0
      },
    },
  )
  const episodeStoryboards = storyboardsData?.data ?? []

  const assetStats = useMemo(() => computeEpisodeAssetStats(episodeAssets, episodeId), [episodeAssets, episodeId])
  const extractionStateFromList = useMemo(() => parseAssetExtractionState(episodeAssets, episodeId), [episodeAssets, episodeId])
  const extractionStateFromApi = useMemo(
    () => parseExtractionStatusResponse((extractionStatusData as { data?: AssetExtractionStatusResponse } | undefined)?.data),
    [extractionStatusData],
  )
  const extractionState = useMemo(() => {
    if (extractionStateFromApi.failed || extractionStateFromApi.inProgress) return extractionStateFromApi
    if (extractionStateFromList.failed || extractionStateFromList.inProgress) return extractionStateFromList
    return extractionStateFromApi
  }, [extractionStateFromApi, extractionStateFromList])
  const storyboardStats = useMemo(() => computeEpisodeStoryboardStats(episodeStoryboards), [episodeStoryboards])
  const serialStoryboardStats = useMemo(() => computeSerialStoryboardStats(episodeStoryboards), [episodeStoryboards])
  const hasRenderableStoryboard = isSerial ? serialStoryboardStats.firstClipReady > 0 : storyboardStats.completed > 0

  const pipelineStatus = useMemo(
    () => computeEpisodePipelineStatus({
      assetStats,
      storyboardStats,
      serialStoryboardStats,
      awaitingAutoStoryboard,
      isExtractingStoryboards,
      episodeStatus: episode?.status,
      isSerial,
      storyboardStageLabel: labels.storyboardStageLabel,
    }),
    [assetStats, storyboardStats, serialStoryboardStats, awaitingAutoStoryboard, isExtractingStoryboards, episode?.status, isSerial, labels.storyboardStageLabel],
  )

  const workflowSteps = useMemo(
    () => computeEpisodeWorkflowSteps({
      project,
      assetStats,
      storyboardStats,
      serialStoryboardStats,
      awaitingAutoStoryboard,
      isExtracting,
      isExtractingStoryboards,
      episodeStatus: episode?.status,
      storyboardStageLabel: labels.storyboardStageLabel,
      isSerial,
      hasRenderableStoryboard,
    }),
    [project, assetStats, storyboardStats, serialStoryboardStats, awaitingAutoStoryboard, isExtracting, isExtractingStoryboards, episode?.status, labels.storyboardStageLabel, isSerial, hasRenderableStoryboard],
  )

  useEffect(() => {
    autoSwitchedRef.current = false
    setAwaitingAutoStoryboard(initialAwaitingAutoStoryboard || autoPipelineActive)
    setActiveTab(initialTab)
    setGenerateTrigger(0)
    setRegenerateTrigger(0)
    setSbGenerateTrigger(0)
    setSbRegenerateTrigger(0)
    setSbPauseTrigger(0)
    setSbResumeTrigger(0)
    setSbAuditTrigger(0)
    extractionFailureNotifiedRef.current = false
  }, [episodeId, initialTab, initialAwaitingAutoStoryboard, autoPipelineActive])

  useEffect(() => {
    if (!extractionState.failed) {
      extractionFailureNotifiedRef.current = false
      return
    }
    if (extractionFailureNotifiedRef.current) return
    extractionFailureNotifiedRef.current = true
    toast({
      title: '资源提取失败',
      description: extractionState.errorMessage,
      variant: 'destructive',
    })
  }, [extractionState.failed, extractionState.errorMessage, toast])

  useEffect(() => {
    if (!autoPipelineActive) return
    setAwaitingAutoStoryboard(true)
    autoSwitchedRef.current = false
  }, [autoPipelineActive, episodeId])

  useEffect(() => {
    const storyboardStarted = episode?.status === 'scene_splitting' || storyboardStats.total > 0 || storyboardStats.active > 0
    if (!awaitingAutoStoryboard || !storyboardStarted) return

    setAwaitingAutoStoryboard(false)
    setActiveTab('storyboard')
    if (!autoSwitchedRef.current) {
      autoSwitchedRef.current = true
      toast({
        title: '已自动开启本集镜头拆分',
        description: `「自动处理本集」已接管后续流程，当前已切换到${labels.storyboardWorkspaceLabel}，资源图会在后台继续生成。`,
        variant: 'success',
      })
    }
  }, [awaitingAutoStoryboard, episode?.status, storyboardStats.total, storyboardStats.active, toast, labels.storyboardWorkspaceLabel])

  const handleExtractAssets = async (modelName?: string) => {
    setIsExtracting(true)
    setActiveTab('assets')
    extractionFailureNotifiedRef.current = false
    const resolvedModel = resolveExtractionModelName(
      modelName ?? effectiveExtractionModelKey,
      project.text_model_id,
      textModels,
    )
    try {
      await assetAPI.extractEpisode(projectId, episodeId, resolvedModel ? { modelName: resolvedModel } : undefined)
      await mutateAssets()
      void globalMutate(['stepper-episodes', projectId])
      void globalMutate(['project', projectId])
      toast({
        title: '任务已提交',
        description: resolvedModel
          ? `正在使用 ${resolvedModel} 提取本集资源…`
          : '正在提取本集资源条目，完成后可在资源页手动生成图片，或通过左侧「自动处理本集」衔接后续流程。',
      })
    } catch (error: unknown) {
      const err = error as { response?: { data?: { error?: string } } }
      toast({
        title: '提取失败',
        description: err?.response?.data?.error || '服务器发生错误',
        variant: 'destructive',
      })
    } finally {
      setIsExtracting(false)
    }
  }

  const handleExtractStoryboards = () => {
    setIsExtractingStoryboards(true)
    setAwaitingAutoStoryboard(false)
    setActiveTab('storyboard')
    const assetsReady = assetStats.total > 0 && assetStats.active === 0 && !assetStats.extracting
    ;(projectAPI.extractEpisodeStoryboards(projectId, episodeId, assetsReady) as Promise<unknown>)
      .then(() => {
        void mutateStoryboards()
        void globalMutate(['stepper-episodes', projectId])
        void globalMutate(['project', projectId])
        setTimeout(() => setIsExtractingStoryboards(false), 5000)
      })
      .catch((error: unknown) => {
        setIsExtractingStoryboards(false)
        const err = error as { response?: { data?: { error?: string; message?: string } } }
        toast({
          title: '本集分镜提取失败',
          description: err?.response?.data?.error || err?.response?.data?.message || '服务器发生错误',
          variant: 'destructive',
        })
      })
    toast({
      title: '镜头拆分已开始',
      description: isSerial ? '正在为本集拆分镜头并生成场景分组，可在下方镜头工作台查看进度。' : '正在为本集拆分镜头条目，可在下方镜头工作台查看进度。',
      variant: 'success',
    })
  }

  const handleAutoMatchVoices = async () => {
    setAutoMatchingVoices(true)
    try {
      const res = await assetAPI.autoMatchVoices(projectId) as unknown as { data?: { updated?: number } }
      const updated = res?.data?.updated ?? 0
      toast({
        title: updated > 0 ? `自动匹配完成，已为 ${updated} 个人物分配音色` : '所有人物已有音色绑定，无需匹配',
        variant: updated > 0 ? 'success' : 'default',
      })
      mutateAssets()
    } catch {
      toast({ title: '自动匹配音色失败', variant: 'destructive' })
    } finally {
      setAutoMatchingVoices(false)
    }
  }

  const handlePauseGeneration = async () => {
    setPausingGeneration(true)
    try {
      const res = await assetAPI.pauseGeneration(projectId) as unknown as { data?: { paused?: number } }
      toast({ title: `已暂停资源生成（${res?.data?.paused ?? 0} 项）`, variant: 'success' })
      mutateAssets()
    } catch {
      toast({ title: '暂停资源生成失败', variant: 'destructive' })
    } finally {
      setPausingGeneration(false)
    }
  }

  const handleResumeGeneration = async () => {
    setResumingGeneration(true)
    try {
      const res = await assetAPI.resumeGeneration(projectId) as unknown as { data?: { triggered?: number } }
      const triggered = res?.data?.triggered ?? 0
      toast({ title: triggered > 0 ? `已继续资源生成（${triggered} 项）` : '当前没有已暂停的资源', variant: triggered > 0 ? 'success' : 'default' })
      mutateAssets()
    } catch {
      toast({ title: '继续资源生成失败', variant: 'destructive' })
    } finally {
      setResumingGeneration(false)
    }
  }

  const resourceButtonDisabled = isExtracting || assetStats.extracting
  const storyboardButtonDisabled = isExtractingStoryboards || assetStats.extracting || assetStats.extractionFailed || awaitingAutoStoryboard
    || episode?.status === 'scene_splitting' || storyboardStats.active > 0

  const episodeSummary = episode?.summary?.trim() || '暂无单集摘要，可先从资源提取开始推进当前集制作。'
  const updatedAtLabel = episode?.updated_at
    ? new Date(episode.updated_at).toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
    : '—'

  return {
    isSerial,
    labels,
    activeTab,
    setActiveTab,
    workflowSteps,
    pipelineStatus,
    assetStats,
    storyboardStats,
    episodeSummary,
    updatedAtLabel,
    resourceButtonDisabled,
    storyboardButtonDisabled,
    isExtractingStoryboards,
    awaitingAutoStoryboard,
    generateTrigger,
    setGenerateTrigger,
    regenerateTrigger,
    setRegenerateTrigger,
    sbGenerateTrigger,
    setSbGenerateTrigger,
    sbRegenerateTrigger,
    setSbRegenerateTrigger,
    sbPauseTrigger,
    setSbPauseTrigger,
    sbResumeTrigger,
    setSbResumeTrigger,
    sbAuditTrigger,
    setSbAuditTrigger,
    autoMatchingVoices,
    pausingGeneration,
    resumingGeneration,
    handleExtractAssets,
    handleExtractStoryboards,
    handleAutoMatchVoices,
    handlePauseGeneration,
    handleResumeGeneration,
    extractionState,
    textModels,
    extractionModelKey,
    setExtractionModelKey,
    effectiveExtractionModelKey,
  }
}
