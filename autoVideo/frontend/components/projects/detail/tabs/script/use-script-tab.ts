'use client'

import React, { useState, useRef, useEffect, useMemo } from 'react'
import useSWR, { mutate as globalMutate } from 'swr'
import { projectAPI, assetAPI, storyboardAPI, modelAPI } from '@/lib/api'
import type { Project, Episode, Asset, Storyboard, Model } from '@/types'
import { useToast } from '@/components/ui/toast'
import { getProviderLabel, getRuntimeModelCapabilityLabels } from '@/lib/model-feasibility'
import { pickPreferredModel } from '@/lib/model-selection'
import { getProgressStallMeta, SCRIPT_PROGRESS_STALL_MS, getTimingSummary } from '@/lib/projects/utils'
import { recommendEpisodeCount } from '@/lib/projects/comic'
import { AUTO_EPISODE_SPLIT_HINT, prefersAutoEpisodeSplit } from '@/lib/projects/episode-split'
import { deriveVideoPipelineSnapshot } from '@/lib/projects/pipeline-status'
import { isCommentaryProject } from '@/lib/projects/commentary-project'
import { getSplitModelRemark, buildSplitModelSearchText, getSplitModelAvailabilityRank } from '@/lib/projects/models'
import type { StoryboardStatsData } from '@/lib/projects/workflow'
import { getApiErrorMessage } from '@/lib/projects/get-api-error-message'
import { resolveDraftTargetEpisodes } from '@/lib/projects/resolve-draft-target-episodes'
import { getProjectModelAvailability } from './model-availability'

export function useScriptTab({
  projectId,
  project,
  mutateProject,
  onAutoStoryboardQueued,
}: {
  projectId: number
  project: Project
  mutateProject: () => void
  onAutoStoryboardQueued?: () => void
}) {
  const { toast } = useToast()
  const usesAutoEpisodeSplit = prefersAutoEpisodeSplit(project)
  const commentaryProject = isCommentaryProject(project)
  const fileRef = useRef<HTMLInputElement>(null)
  const [selectedEpisode, setSelectedEpisode] = useState<Episode | null>(null)
  const [assetGenerating, setAssetGenerating] = useState(false)
  const [episodeGenerating, setEpisodeGenerating] = useState(false)
  const [showRegenerateDialog, setShowRegenerateDialog] = useState(false)
  const [showCreateEpisodeDialog, setShowCreateEpisodeDialog] = useState(false)
  const [showScriptPreviewDialog, setShowScriptPreviewDialog] = useState(false)
  const [showSplitAdvancedSettings, setShowSplitAdvancedSettings] = useState(() => !project.text_model_id)
  const [creatingEpisode, setCreatingEpisode] = useState(false)
  const [savingSplitModel, setSavingSplitModel] = useState(false)
  const [savingImageModel, setSavingImageModel] = useState(false)
  const [storyboardDispatching, setStoryboardDispatching] = useState(false)
  const [episodeStoryboardDispatching, setEpisodeStoryboardDispatching] = useState<number | null>(null)
  const [autoStoryboardAfterSplit, setAutoStoryboardAfterSplit] = useState(true)
  const [splitModelSearch, setSplitModelSearch] = useState('')
  const [draftSplitModelId, setDraftSplitModelId] = useState<string>(project.text_model_id ? String(project.text_model_id) : '')
  const [draftTargetEpisodes, setDraftTargetEpisodes] = useState<string>(() =>
    resolveDraftTargetEpisodes(project.target_episodes, recommendEpisodeCount(project.script_text?.trim() ?? ''), false)
  )
  const [splitSettingsDirty, setSplitSettingsDirty] = useState(false)
  const [kwSplitKeywords, setKwSplitKeywords] = useState('')
  const [kwCharacters, setKwCharacters] = useState('')
  const [kwLocations, setKwLocations] = useState('')
  const [kwEvents, setKwEvents] = useState('')
  const [kwProps, setKwProps] = useState('')
  const [manualEpisodeNumber, setManualEpisodeNumber] = useState('')
  const [manualEpisodeTitle, setManualEpisodeTitle] = useState('')
  const [manualEpisodeSummary, setManualEpisodeSummary] = useState('')
  const [manualEpisodeContent, setManualEpisodeContent] = useState('')
  const [editingEpisode, setEditingEpisode] = useState(false)
  const [editEpisodeTitle, setEditEpisodeTitle] = useState('')
  const [editEpisodeSummary, setEditEpisodeSummary] = useState('')
  const [editEpisodeContent, setEditEpisodeContent] = useState('')
  const [savingEpisodeEdit, setSavingEpisodeEdit] = useState(false)
  const [polishingEpisode, setPolishingEpisode] = useState(false)
  const [deletingEpisodeId, setDeletingEpisodeId] = useState<number | null>(null)
  const [episodeDeleteTarget, setEpisodeDeleteTarget] = useState<Episode | null>(null)
  const [extractingEpisodeAssets, setExtractingEpisodeAssets] = useState<number | null>(null)
  const [generatingEpisodeAssets, setGeneratingEpisodeAssets] = useState<number | null>(null)
  // Script optimize + AI review states
  const [optimizingEpisode, setOptimizingEpisode] = useState<number | null>(null)
  const [reviewingEpisode, setReviewingEpisode] = useState<number | null>(null)
  const [autoOptimizingEpisode, setAutoOptimizingEpisode] = useState<number | null>(null)
  const [applyingOptimized, setApplyingOptimized] = useState<number | null>(null)
  const [batchOptimizing, setBatchOptimizing] = useState(false)
  const [batchReviewing, setBatchReviewing] = useState(false)
  // Tracks when the last extraction was started; used to enforce a grace period in the
  // assetsCleared check so we don't prematurely reset assetGenerating during the brief
  // window between DeleteByProjectID completing and the sentinel being created.
  const extractionStartedAtRef = React.useRef<number | null>(null)
  const autoOptimizePollingRef = React.useRef<ReturnType<typeof setInterval> | null>(null)

  // Track asset extraction progress
  const { data: extractAssetsData, mutate: mutateExtractAssets } = useSWR(
    ['extract-assets', projectId],
    () => assetAPI.list(projectId) as unknown as Promise<{ data: Asset[] }>,
    {
      refreshInterval: (data) => {
        if (assetGenerating) return 3000
        const assets = (data as { data?: Asset[] })?.data ?? []
        // Poll during extraction (sentinel) or image generation
        return assets.some((a) => a.status === 'extracting' || a.status === 'generating') ? 3000 : 0
      },
    }
  )
  const extractAssetsRaw = (extractAssetsData as { data?: Asset[] })?.data ?? []
  const extractAssets = extractAssetsRaw.filter((a) => a.name !== '__extracting__')
  const extractTotal = extractAssets.length

  // Phase 1: Extraction (LLM reads script → creates assets in "pending" status)
  const extractionInProgress = extractAssetsRaw.some((a) => a.status === 'extracting')
  const extractionDone = extractTotal > 0 && !extractionInProgress
  const extractedGeneratingCount = extractAssets.filter((asset) => asset.status === 'generating').length
  const extractedPendingCount = extractAssets.filter((asset) => asset.status === 'pending').length
  const extractedPausedCount = extractAssets.filter((asset) => asset.status === 'paused').length
  const extractedFailedCount = extractAssets.filter((asset) => asset.status === 'failed').length
  // Assets are optional — allow storyboard generation when no assets exist.
  // Failed assets are a terminal state and do NOT block storyboard generation
  // (mirrors backend ensureProjectAssetsReady in storyboard_handler.go).
  const storyboardAssetsReady =
    extractTotal === 0 ||
    (!extractionInProgress && extractedPendingCount === 0 && extractedGeneratingCount === 0 && extractedPausedCount === 0)
  const storyboardAssetBlockingReason = extractionInProgress
    ? '资源仍在提取中，请先完成资源提取'
    : extractedPendingCount > 0 || extractedGeneratingCount > 0 || extractedPausedCount > 0
      ? `资源图尚未全部完成：待生成 ${extractedPendingCount}，生成中 ${extractedGeneratingCount}，已暂停 ${extractedPausedCount}，失败 ${extractedFailedCount}`
      : ''
  const scriptText = project.script_text?.trim() ?? ''
  const hasScriptText = scriptText.length > 0
  const scriptPreview = hasScriptText ? scriptText.slice(0, 1200) : ''
  const recommendedEpisodeCount = useMemo(() => recommendEpisodeCount(scriptText), [scriptText])

  // Phase 2: Image generation (pending → generating → completed/failed) — tracked in AssetsTab

  const { data: textModelsData, isLoading: textModelsLoading } = useSWR(
    ['project-text-models', projectId],
    () => modelAPI.list({ type: 'llm', sort_by: 'priority' }) as unknown as Promise<{ data: Model[] }>
  )
  const { data: textModelHealthData } = useSWR(
    ['project-text-model-health', projectId],
    () => modelAPI.health() as unknown as Promise<{ data: Record<string, 'healthy' | 'unhealthy' | 'unknown'> }>
  )
  const { data: imageModelsData, isLoading: imageModelsLoading } = useSWR(
    ['project-image-models', projectId],
    () => modelAPI.list({ type: 'image', sort_by: 'priority' }) as unknown as Promise<{ data: Model[] }>
  )
  const allTextModels: Model[] = (textModelsData as { data?: Model[] })?.data ?? []
  const allImageModels: Model[] = (imageModelsData as { data?: Model[] })?.data ?? []
  const projectImageModelKey = allImageModels.find(m => m.id === project.image_model_id)?.model_key ?? ''
  const textModelHealthMap = (textModelHealthData as { data?: Record<string, 'healthy' | 'unhealthy' | 'unknown'> })?.data ?? {}
  const splitModels = allTextModels
    .filter((model) => model.is_active || model.id === project.text_model_id)
    .sort((left, right) => {
      const leftHealth = textModelHealthMap[left.name] ?? left.health_status ?? 'unknown'
      const rightHealth = textModelHealthMap[right.name] ?? right.health_status ?? 'unknown'
      const availabilityDelta = getSplitModelAvailabilityRank(left, leftHealth) - getSplitModelAvailabilityRank(right, rightHealth)
      if (availabilityDelta !== 0) return availabilityDelta
      if (left.is_default !== right.is_default) return left.is_default ? -1 : 1
      if (left.priority !== right.priority) return left.priority - right.priority
      return left.name.localeCompare(right.name, 'zh-CN')
    })
  const defaultSplitModel = pickPreferredModel(splitModels, textModelHealthMap)
  const selectedSplitModelId = Number(draftSplitModelId)
  const selectedSplitModel = allTextModels.find((model) => model.id === selectedSplitModelId)
  const effectiveSplitModel = selectedSplitModel ?? defaultSplitModel ?? null
  const splitModelCapabilities = effectiveSplitModel ? getRuntimeModelCapabilityLabels(effectiveSplitModel) : []
  const selectedSplitModelRemark = effectiveSplitModel ? getSplitModelRemark(effectiveSplitModel) : ''
  const selectedSplitModelProvider = effectiveSplitModel ? getProviderLabel(effectiveSplitModel.provider) : null
  const imageModels = allImageModels
    .filter((model) => model.is_active || model.failure_reason || model.id === project.image_model_id)
    .sort((left, right) => {
      const leftHealth = textModelHealthMap[left.name] ?? left.health_status ?? 'unknown'
      const rightHealth = textModelHealthMap[right.name] ?? right.health_status ?? 'unknown'
      const availabilityDelta = getSplitModelAvailabilityRank(left, leftHealth) - getSplitModelAvailabilityRank(right, rightHealth)
      if (availabilityDelta !== 0) return availabilityDelta
      if (left.is_default !== right.is_default) return left.is_default ? -1 : 1
      if (left.priority !== right.priority) return left.priority - right.priority
      return left.name.localeCompare(right.name, 'zh-CN')
    })
  const selectedImageModel = allImageModels.find((model) => model.id === project.image_model_id)
  const selectedProjectImageModelName = selectedImageModel?.name
  const selectedImageModelCapabilities = selectedImageModel ? getRuntimeModelCapabilityLabels(selectedImageModel) : []
  const selectedImageModelProvider = selectedImageModel ? getProviderLabel(selectedImageModel.provider) : null
  const selectedSplitModelAvailability = effectiveSplitModel ? getProjectModelAvailability(effectiveSplitModel, textModelHealthMap) : null
  const selectedImageModelAvailability = selectedImageModel ? getProjectModelAvailability(selectedImageModel, textModelHealthMap) : null
  const parsedTargetEpisodes = Number.parseInt(draftTargetEpisodes, 10)
  const hasValidTargetEpisodes = Number.isFinite(parsedTargetEpisodes) && parsedTargetEpisodes >= 1 && parsedTargetEpisodes <= 200
  const splitConfigReady = !!effectiveSplitModel && (usesAutoEpisodeSplit || hasValidTargetEpisodes)
  const shouldShowSplitSearch = splitModels.length > 8
  const filteredSplitModels = useMemo(() => {
    const keyword = splitModelSearch.trim().toLocaleLowerCase()
    if (!keyword) return splitModels
    return splitModels.filter((model) => buildSplitModelSearchText(model).includes(keyword))
  }, [splitModels, splitModelSearch])

  const shouldPollEpisodes = episodeGenerating
    || project.status === 'script_processing'
    || ['episode_splitting', 'script_prepping', 'scene_splitting'].includes(project.progress?.stage ?? '')

  const { data: episodesData, isLoading: episodesLoading, mutate: mutateEpisodes } = useSWR(
    ['episodes', projectId],
    () => projectAPI.listEpisodes(projectId) as unknown as Promise<{ data: Episode[] }>,
    {
      refreshInterval: (data) => {
        if (shouldPollEpisodes) return 3000
        const eps = (data as { data?: Episode[] })?.data ?? []
        if (eps.some((ep) => ep.optimize_status === 'optimizing' || ep.optimize_status === '' || ep.review_status === 'reviewing')) return 3000
        return 0
      },
    }
  )
  const episodes = (episodesData as { data?: Episode[] })?.data ?? []

  const pipeline = useMemo(
    () => deriveVideoPipelineSnapshot({
      project,
      episodes,
      episodeGenerating,
      assetExtracting: extractionInProgress,
      assetGenerating,
      storyboardGenerating: storyboardDispatching || episodeStoryboardDispatching !== null,
    }),
    [
      project,
      episodes,
      episodeGenerating,
      extractionInProgress,
      assetGenerating,
      storyboardDispatching,
      episodeStoryboardDispatching,
    ],
  )

  const isProcessing = episodeGenerating || pipeline.isActive
  const hasExistingEpisodes = episodes.length > 0
  const splitInProgress = episodeGenerating || (!hasExistingEpisodes && pipeline.isActive)
  const nextManualEpisodeNumber = useMemo(
    () => episodes.reduce((maxValue, episode) => Math.max(maxValue, episode.episode_number), 0) + 1,
    [episodes]
  )
  const parsedManualEpisodeNumber = Number(manualEpisodeNumber)
  const manualEpisodeNumberValid = Number.isInteger(parsedManualEpisodeNumber) && parsedManualEpisodeNumber > 0
  const manualEpisodeNumberTaken = manualEpisodeNumberValid && episodes.some((episode) => episode.episode_number === parsedManualEpisodeNumber)

  // Fetch storyboards to show status in ScriptTab
  const { data: scriptTabSbData, mutate: mutateScriptTabSb } = useSWR(
    episodes.length > 0 ? ['script-tab-storyboards', projectId] : null,
    () => storyboardAPI.list(projectId, { page_size: 100 }) as unknown as Promise<{ data: Storyboard[] | { items: Storyboard[] } }>,
    { refreshInterval: shouldPollEpisodes || pipeline.isActive ? 5000 : 0 }
  )
  const scriptTabSbRaw = (scriptTabSbData as { data?: Storyboard[] | { items?: Storyboard[] } })?.data
  const scriptTabStoryboards: Storyboard[] = Array.isArray(scriptTabSbRaw) ? scriptTabSbRaw : (scriptTabSbRaw as { items?: Storyboard[] })?.items ?? []
  const { data: scriptTabStoryboardStatsRaw, mutate: mutateScriptTabStoryboardStats } = useSWR(
    episodes.length > 0 ? ['script-tab-storyboard-stats', projectId] : null,
    () => storyboardAPI.stats(projectId) as unknown as Promise<{ data: StoryboardStatsData }>,
    {
      refreshInterval: (data) => {
        if (isProcessing || storyboardDispatching) return 5000
        const stats = (data as { data?: StoryboardStatsData })?.data
        return stats && stats.generating > 0 ? 5000 : 0
      },
    }
  )
  const scriptTabStoryboardStats: StoryboardStatsData =
    (scriptTabStoryboardStatsRaw as { data?: StoryboardStatsData })?.data
    ?? { total: 0, pending: 0, generating: 0, paused: 0, completed: 0, failed: 0, voided: 0 }
  const startableStoryboardCount = scriptTabStoryboardStats.pending + scriptTabStoryboardStats.failed
  const pausedStoryboardCount = scriptTabStoryboardStats.paused
  const scriptProgressStalled = getProgressStallMeta(project.progress?.updated_at, SCRIPT_PROGRESS_STALL_MS)
  const storyboardSplitTiming = getTimingSummary(
    project.progress?.started_at ?? project.progress?.updated_at,
    project.progress?.scene_split?.total
      ? (project.progress.scene_split.completed ?? 0) / Math.max(project.progress.scene_split.total, 1)
      : 0,
    Date.now()
  )
  const splitProgressSummary = project.progress?.message
    || (project.progress?.stage === 'episode_splitting'
      ? `AI 正在识别集数边界与情节节点${project.progress?.episode_split?.total ? `（${project.progress.episode_split.completed}/${project.progress.episode_split.total} 集）` : '…'}`
      : 'AI 正在分析剧本结构，自动识别分集边界与情节节点…')
  const splitProgressPercent = project.progress?.episode_split?.total
    ? Math.min(100, ((project.progress.episode_split.completed ?? 0) / Math.max(project.progress.episode_split.total, 1)) * 100)
    : 0
  const scenePreppingCount = episodes.filter((ep) => ep.status === 'script_prepping').length
  const sceneSplittingCount = episodes.filter((ep) => ep.status === 'scene_splitting').length
  const sceneReadyCount = episodes.filter((ep) => ep.status === 'scene_ready' || ep.status === 'done').length
  const sceneProcessingSummary = (() => {
    if (scenePreppingCount > 0 || sceneSplittingCount > 0) {
      const parts: string[] = []
      if (scenePreppingCount > 0) parts.push(`${scenePreppingCount} 集优化提示词`)
      if (sceneSplittingCount > 0) parts.push(`${sceneSplittingCount} 集分镜拆分中`)
      if (sceneReadyCount > 0) parts.push(`${sceneReadyCount} 集已就绪`)
      return parts.join(' · ')
    }
    if (project.progress?.scene_split) {
      const done = project.progress.scene_split.completed ?? 0
      const total = project.progress.scene_split.total ?? episodes.length
      if (done >= total && scriptTabStoryboards.length < total) {
        return `分镜拆分完成，正在审查与精修提示词（${scriptTabStoryboards.length}/${total * 4} 个分镜已写入）`
      }
      return `分镜格式化进度：${done}/${total} 集`
    }
    return `已格式化 ${scriptTabStoryboards.length} 个分镜，正在继续处理剩余集数`
  })()
  const sceneProcessingProgress = project.progress?.scene_split?.total
    ? Math.min(100, ((project.progress.scene_split.completed ?? 0) / Math.max(project.progress.scene_split.total, 1)) * 100)
    : episodes.length > 0
      ? Math.min(100, (scriptTabStoryboards.length / Math.max(episodes.length * 4, 1)) * 100)
      : 0

  // Per-episode storyboard generation trigger
  const handleStartEpisodeStoryboard = async (episodeId: number) => {
    setEpisodeStoryboardDispatching(episodeId)
    try {
      const res = await storyboardAPI.generateAll(projectId, episodeId, projectImageModelKey) as unknown as { data?: { triggered?: number } }
      const triggered = res?.data?.triggered ?? 0
      mutateScriptTabSb()
      mutateScriptTabStoryboardStats()
      if (triggered > 0) {
        toast({ title: `第 ${episodeId} 集分镜已启动 ${triggered} 个任务`, variant: 'success' })
      } else {
        toast({ title: '当前集暂无待启动的分镜', variant: 'default' })
      }
    } catch {
      toast({ title: '分镜启动失败', variant: 'destructive' })
    } finally {
      setEpisodeStoryboardDispatching(null)
    }
  }

  const handleStartStoryboard = React.useCallback(async (options?: { silentNoop?: boolean; successTitle?: string }) => {
    if (!storyboardAssetsReady) {
      toast({ title: storyboardAssetBlockingReason || '请先完成资源图生成后再开始分镜', variant: 'destructive' })
      return
    }
    setStoryboardDispatching(true)
    try {
      const res = await storyboardAPI.generateAll(projectId, undefined, projectImageModelKey) as unknown as { data?: { triggered?: number } }
      const triggered = res?.data?.triggered ?? 0
      mutateScriptTabSb()
      mutateScriptTabStoryboardStats()
      globalMutate(['project', projectId])

      if (triggered > 0) {
        toast({ title: options?.successTitle ?? `手动开始分镜，已提交 ${triggered} 个任务`, variant: 'success' })
      } else if (!options?.silentNoop) {
        toast({ title: '当前没有待启动的分镜', variant: 'default' })
      }
    } catch {
      toast({ title: options?.successTitle ? '分镜启动失败，请手动重试' : '手动开始分镜失败', variant: 'destructive' })
    } finally {
      setStoryboardDispatching(false)
    }
  }, [mutateScriptTabSb, mutateScriptTabStoryboardStats, projectId, storyboardAssetBlockingReason, storyboardAssetsReady, toast])

  // Clear local generating flag once project transitions OUT of script_processing
  // (i.e., it must have entered script_processing first before we clear)
  const [wasProcessing, setWasProcessing] = React.useState(false)
  React.useEffect(() => {
    if (project.status === 'script_processing') {
      setWasProcessing(true)
    }
    if (episodeGenerating && wasProcessing && project.status !== 'script_processing') {
      setEpisodeGenerating(false)
      setWasProcessing(false)
      mutateEpisodes()
      mutateScriptTabSb()
      mutateScriptTabStoryboardStats()
    }
  }, [
    episodeGenerating,
    wasProcessing,
    project.status,
    mutateEpisodes,
    mutateScriptTabSb,
    mutateScriptTabStoryboardStats,
  ])

  // Clear asset generating flag once extraction completes or assets are cleared.
  // Grace period: don't apply assetsCleared within 8s of starting to avoid a race
  // where DeleteByProjectID finishes before the extraction sentinel is created.
  React.useEffect(() => {
    if (assetGenerating) {
      const msSinceStart = extractionStartedAtRef.current ? Date.now() - extractionStartedAtRef.current : Infinity
      const inGracePeriod = msSinceStart < 8000
      const assetsCleared = !inGracePeriod && extractTotal === 0 && !extractAssetsRaw.some((a) => a.status === 'extracting')
      const extractionComplete = extractTotal > 0 && !extractionInProgress
      if (assetsCleared || extractionComplete) {
        setAssetGenerating(false)
        extractionStartedAtRef.current = null
      }
    }
  }, [assetGenerating, extractTotal, extractionInProgress, extractAssetsRaw])

  React.useEffect(() => {
    if (!usesAutoEpisodeSplit || project.target_episodes <= 0) return
    void projectAPI.update(projectId, { target_episodes: 0 } as Partial<Project>).then(() => {
      mutateProject()
      globalMutate(['project', projectId])
    })
  }, [usesAutoEpisodeSplit, project.target_episodes, projectId, mutateProject])

  React.useEffect(() => {
    if (splitSettingsDirty) return
    const preferredId = project.text_model_id || defaultSplitModel?.id
    if (!preferredId) return
    const nextId = String(preferredId)
    if (draftSplitModelId !== nextId) {
      setDraftSplitModelId(nextId)
    }
  }, [project.text_model_id, defaultSplitModel?.id, splitSettingsDirty, draftSplitModelId])

  React.useEffect(() => {
    if (splitSettingsDirty || usesAutoEpisodeSplit) return
    setDraftTargetEpisodes(resolveDraftTargetEpisodes(project.target_episodes, recommendedEpisodeCount, episodes.length > 0))
  }, [episodes.length, project.target_episodes, project.text_model_id, recommendedEpisodeCount, splitSettingsDirty, usesAutoEpisodeSplit])

  // Poll every 3s while auto-optimize-review is running (backend is async)
  React.useEffect(() => {
    if (autoOptimizingEpisode !== null) {
      autoOptimizePollingRef.current = setInterval(() => { mutateEpisodes() }, 3000)
    }
    return () => {
      if (autoOptimizePollingRef.current) {
        clearInterval(autoOptimizePollingRef.current)
        autoOptimizePollingRef.current = null
      }
    }
  }, [autoOptimizingEpisode, mutateEpisodes])

  // Detect auto-optimize-review completion and clear spinner
  React.useEffect(() => {
    if (autoOptimizingEpisode === null) return
    const ep = episodes.find((e) => e.id === autoOptimizingEpisode)
    if (!ep) return
    if (ep.optimize_status === 'failed') {
      setAutoOptimizingEpisode(null)
      toast({ title: 'AI 优化失败，请重试', variant: 'destructive' })
      return
    }
    const reviewSettled = ep.review_status === 'done' || ep.review_status === 'failed' || (!ep.review_status && ep.optimize_status === 'done')
    if (ep.optimize_status === 'done' && reviewSettled) {
      setAutoOptimizingEpisode(null)
      if (selectedEpisode?.id === ep.id) setSelectedEpisode(ep)
      mutateEpisodes()
      toast({ title: 'AI 一键优化完成', description: '已完成转剧本格式 + AI 审查，如有不足已自动修复，可在详情中确认应用', variant: 'success' })
    }
  }, [episodes, autoOptimizingEpisode, selectedEpisode])

  const persistSplitSettings = async (options?: { successTitle?: string; silent?: boolean }) => {
    if (!effectiveSplitModel) {
      toast({ title: '请先选择分集模型', variant: 'destructive' })
      return false
    }
    if (!usesAutoEpisodeSplit && !hasValidTargetEpisodes) {
      toast({ title: '请填写 1-200 的目标分集数', variant: 'destructive' })
      return false
    }

    const nextTargetEpisodes = usesAutoEpisodeSplit ? 0 : parsedTargetEpisodes
    const shouldUpdate =
      project.text_model_id !== effectiveSplitModel.id ||
      project.target_episodes !== nextTargetEpisodes

    if (!shouldUpdate) {
      return true
    }

    setSavingSplitModel(true)
    try {
      await projectAPI.update(projectId, {
        text_model_id: effectiveSplitModel.id,
        target_episodes: nextTargetEpisodes,
      } as Partial<Project>)
      setSplitSettingsDirty(false)
      if (options?.successTitle) {
        toast({ title: options.successTitle, variant: 'success' })
      }
      mutateProject()
      globalMutate(['project', projectId])
      return true
    } catch {
      if (!options?.silent) {
        toast({ title: options?.successTitle ? '分集配置更新失败' : '分集配置保存失败', variant: 'destructive' })
      }
      return false
    } finally {
      setSavingSplitModel(false)
    }
  }

  const handleUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    try {
      await projectAPI.uploadScript(projectId, file)
      const nextProject = await mutateProject()
      const uploadedProject = (nextProject as { data?: Project } | undefined)?.data
      const uploadedRecommendation = recommendEpisodeCount(uploadedProject?.script_text?.trim() ?? '')
      if (!splitSettingsDirty && !usesAutoEpisodeSplit) {
        setDraftTargetEpisodes(resolveDraftTargetEpisodes(uploadedProject?.target_episodes ?? 0, uploadedRecommendation, false))
      }
      globalMutate(['project', projectId])

      const shouldAutoStart = splitConfigReady && !splitSettingsDirty
      if (shouldAutoStart) {
    setEpisodeGenerating(true)
    setAssetGenerating(false)
    await projectAPI.generateEpisodes(projectId, undefined, { autoStoryboard: autoStoryboardAfterSplit })
    toast({
      title: '上传成功，已自动开始分集',
      description: autoStoryboardAfterSplit
        ? (commentaryProject
          ? '系统将基于上传原文自动提取资源并拆分分镜，不会进行 AI 润色优化。'
          : '系统会自动润色优化第 1 集示范剧本（仅文本），资源与分镜请在单集列表手动启动。')
        : '分集完成后可继续手动推进后续流程。',
      variant: 'success',
    })
    if (autoStoryboardAfterSplit) onAutoStoryboardQueued?.()
    mutateEpisodes()
    mutateExtractAssets()
    globalMutate(['project', projectId])
    setTimeout(() => globalMutate(['project', projectId]), 1500)
    setTimeout(() => globalMutate(['project', projectId]), 4000)
    return
  }

    toast({
      title: '上传成功，请选择模型后手动开始分集',
      description: usesAutoEpisodeSplit
        ? (uploadedRecommendation
          ? `将按剧本自动分集，预计约 ${uploadedRecommendation.count} 集（${uploadedRecommendation.reason}）`
          : AUTO_EPISODE_SPLIT_HINT)
        : (uploadedRecommendation ? `已推荐 ${uploadedRecommendation.count} 个分集，${uploadedRecommendation.reason}` : '可按需要手动填写目标分集数'),
      variant: 'success',
    })
    } catch {
      toast({ title: '上传失败', variant: 'destructive' })
    } finally {
    setEpisodeGenerating(false)
    }
    if (fileRef.current) fileRef.current.value = ''
  }

  const handleGenerateEpisodes = async () => {
    if (!project.script_file_url && !hasScriptText) {
      toast({ title: '请先上传剧本文件', variant: 'destructive' })
      return
    }
    if (!splitConfigReady) {
      toast({
        title: !effectiveSplitModel ? '请先选择分集模型' : usesAutoEpisodeSplit ? '分集配置未就绪' : '请填写 1-200 的目标分集数',
        description: !effectiveSplitModel && splitModels.length === 0 ? '当前没有可用的文本模型，请先在模型管理里启用至少一个 LLM。' : undefined,
        variant: 'destructive',
      })
      return
    }

    const parseTags = (s: string) => s.split(/[,，、\n]/).map(t => t.trim()).filter(Boolean)
    const splitKws = kwSplitKeywords.split(/[\n]/).map(t => t.trim()).filter(Boolean)
    const keywords = {
      characters: parseTags(kwCharacters),
      locations: parseTags(kwLocations),
      events: parseTags(kwEvents),
      props: parseTags(kwProps),
      split_keywords: splitKws,
    }
    const hasKeywords = Object.values(keywords).some(a => a.length > 0)

    try {
      const saved = await persistSplitSettings({ silent: true })
      if (!saved) {
        toast({ title: '分集配置保存失败', description: '请检查分集模型是否可用，或在「分集高级设置」中重新选择模型。', variant: 'destructive' })
        return
      }
    } catch {
      toast({ title: '分集配置保存失败', variant: 'destructive' })
      return
    }

    setShowRegenerateDialog(false)
    setEpisodeGenerating(true)
    setAssetGenerating(false) // Reset stale extraction state

    // Clean up existing assets (including any in-progress extractions)
    try {
      await assetAPI.deleteAll(projectId)
      mutateExtractAssets()
    } catch {
      // Non-fatal — backend regenerate also cleans episodes/storyboards
    }

    try {
      await projectAPI.generateEpisodes(projectId, hasKeywords ? keywords : undefined, {
        autoStoryboard: autoStoryboardAfterSplit,
        ...(hasExistingEpisodes ? { rebuild: true } : {}),
      })
      toast({
        title: autoStoryboardAfterSplit
          ? (commentaryProject
            ? '重新分集已启动：将基于原文自动提取资源并拆分分镜'
            : '重新分集已启动：将自动润色优化第 1 集示范剧本（仅文本）')
          : '重新分集已启动：旧分集与旧分镜将按新配置重建',
        variant: 'success',
      })
      if (autoStoryboardAfterSplit) onAutoStoryboardQueued?.()
      mutateEpisodes()
      mutateExtractAssets()
      mutateScriptTabSb()
      globalMutate(['project', projectId])
      // Backend goroutine may not have set script_processing yet — re-fetch after short delays
      setTimeout(() => globalMutate(['project', projectId]), 1500)
      setTimeout(() => globalMutate(['project', projectId]), 4000)
    } catch (err) {
      const status = (err as { response?: { status?: number } })?.response?.status
      const message = getApiErrorMessage(err)
      if (status === 409) {
        toast({
          title: '已有进行中的分集任务',
          description: project.progress?.message || message || '当前项目正在拆分集数，请稍候，页面会自动刷新进度。',
          variant: 'default',
        })
      } else {
        toast({
          title: '分集启动失败',
          description: message || '请稍后重试',
          variant: 'destructive',
        })
      }
      setEpisodeGenerating(false)
    }
  }

  const handleRetryStalledScript = async () => {
    if (!splitConfigReady) {
      toast({ title: !effectiveSplitModel ? '请先选择分集模型' : usesAutoEpisodeSplit ? '分集配置未就绪' : '请填写 1-200 的目标分集数', variant: 'destructive' })
      return
    }

    const parseTags = (s: string) => s.split(/[,，、\n]/).map(t => t.trim()).filter(Boolean)
    const splitKws = kwSplitKeywords.split(/[\n]/).map(t => t.trim()).filter(Boolean)
    const keywords = {
      characters: parseTags(kwCharacters),
      locations: parseTags(kwLocations),
      events: parseTags(kwEvents),
      props: parseTags(kwProps),
      split_keywords: splitKws,
    }
    const hasKeywords = Object.values(keywords).some(a => a.length > 0)

    const saved = await persistSplitSettings({ silent: true })
    if (!saved) {
      return
    }

    setEpisodeGenerating(true)
    try {
      await projectAPI.generateEpisodes(projectId, hasKeywords ? keywords : undefined, { force: true, autoStoryboard: autoStoryboardAfterSplit })
      toast({ title: '已尝试重新拉起剧本拆分任务', variant: 'success' })
      if (autoStoryboardAfterSplit) onAutoStoryboardQueued?.()
      mutateEpisodes()
      mutateScriptTabSb()
      globalMutate(['project', projectId])
      setTimeout(() => globalMutate(['project', projectId]), 1500)
      setTimeout(() => globalMutate(['project', projectId]), 4000)
    } catch (err) {
      const status = (err as { response?: { status?: number } })?.response?.status
      toast({ title: status === 409 ? '当前剧本任务尚未满足强制重试条件' : '重新拉起失败', variant: 'destructive' })
      setEpisodeGenerating(false)
    }
  }

  const handleOpenRegenerate = () => {
    if (!project.script_file_url && !hasScriptText) {
      toast({ title: '请先上传剧本文件', variant: 'destructive' })
      return
    }
    if (!effectiveSplitModel && defaultSplitModel?.id) {
      setDraftSplitModelId(String(defaultSplitModel.id))
    }
    if (!splitConfigReady) {
      toast({
        title: !effectiveSplitModel ? '请先选择分集模型' : usesAutoEpisodeSplit ? '分集配置未就绪' : '请填写 1-200 的目标分集数',
        description: !effectiveSplitModel && splitModels.length === 0 ? '当前没有可用的文本模型，请先在模型管理里启用至少一个 LLM。' : undefined,
        variant: 'destructive',
      })
      return
    }
    if (episodeGenerating) {
      toast({ title: '分集任务已在提交中，请稍候', variant: 'default' })
      return
    }

    // Pre-fill from existing keyword library
    const kw = project.keyword_library
    if (kw) {
      setKwSplitKeywords(kw.split_keywords?.join('\n') ?? '')
      setKwCharacters(kw.characters?.join('、') ?? '')
      setKwLocations(kw.locations?.join('、') ?? '')
      setKwEvents(kw.events?.join('、') ?? '')
      setKwProps(kw.props?.join('、') ?? '')
    }
    setShowRegenerateDialog(true)
  }

  const handleOpenCreateEpisode = () => {
    setManualEpisodeNumber(String(nextManualEpisodeNumber))
    setManualEpisodeTitle(`第 ${nextManualEpisodeNumber} 集`)
    setManualEpisodeSummary('')
    setManualEpisodeContent('')
    setShowCreateEpisodeDialog(true)
  }

  const handleCreateEpisode = async () => {
    if (!manualEpisodeNumberValid) {
      toast({ title: '请填写大于 0 的分集序号', variant: 'destructive' })
      return
    }
    if (manualEpisodeNumberTaken) {
      toast({ title: `第 ${parsedManualEpisodeNumber} 集已存在，请换一个序号`, variant: 'destructive' })
      return
    }

    const title = manualEpisodeTitle.trim() || `第 ${parsedManualEpisodeNumber} 集`
    const payload = {
      episode_number: parsedManualEpisodeNumber,
      title,
      summary: manualEpisodeSummary.trim() || undefined,
      script_excerpt: manualEpisodeContent.trim() || undefined,
    }
    console.log('[createEpisode] payload:', JSON.stringify(payload))
    setCreatingEpisode(true)
    try {
      const created = await projectAPI.createEpisode(projectId, payload) as unknown as { data?: Episode }
      console.log('[createEpisode] response:', JSON.stringify(created))

      if (!usesAutoEpisodeSplit) {
        const nextEpisodeTarget = Math.max(project.target_episodes || 0, episodes.length + 1, parsedManualEpisodeNumber)
        if (nextEpisodeTarget !== project.target_episodes) {
          await projectAPI.update(projectId, { target_episodes: nextEpisodeTarget } as Partial<Project>)
        }
        setDraftTargetEpisodes(String(nextEpisodeTarget))
        setSplitSettingsDirty(false)
      }
      setShowCreateEpisodeDialog(false)
      mutateEpisodes()
      mutateProject()
      globalMutate(['project', projectId])

      if (created?.data) {
        setSelectedEpisode(created.data)
      }
      toast({ title: `已手动创建第 ${parsedManualEpisodeNumber} 集`, variant: 'success' })
    } catch (error) {
      const message = (error as { response?: { data?: { message?: string } } })?.response?.data?.message
      toast({ title: message || '手动创建分集失败', variant: 'destructive' })
    } finally {
      setCreatingEpisode(false)
    }
  }

  const handleOpenEditEpisode = (ep: Episode) => {
    setEditEpisodeTitle(ep.title ?? '')
    setEditEpisodeSummary(ep.summary ?? '')
    setEditEpisodeContent(ep.script_excerpt ?? '')
    setEditingEpisode(true)
  }

  const handleSaveEpisodeEdit = async () => {
    console.log('[saveEpisodeEdit] called, selectedEpisode:', selectedEpisode?.id, 'editEpisodeContent length:', editEpisodeContent.length)
    if (!selectedEpisode) return
    setSavingEpisodeEdit(true)
    const payload = {
      title: editEpisodeTitle.trim() || selectedEpisode.title,
      summary: editEpisodeSummary.trim() || undefined,
      script_excerpt: editEpisodeContent.trim(),
    }
    console.log('[saveEpisodeEdit] payload:', JSON.stringify(payload))
    try {
      const res = await projectAPI.updateEpisode(projectId, selectedEpisode.id, payload as Partial<Episode>)
      console.log('[saveEpisodeEdit] response:', JSON.stringify(res))
      const updated = (res as { data?: Episode })?.data
      console.log('[saveEpisodeEdit] updated episode:', JSON.stringify(updated))
      if (updated) setSelectedEpisode(updated)
      mutateEpisodes()
      setEditingEpisode(false)
      toast({ title: '分集信息已保存', variant: 'success' })
    } catch {
      toast({ title: '保存失败，请重试', variant: 'destructive' })
    } finally {
      setSavingEpisodeEdit(false)
    }
  }

  const handlePolishEpisode = async () => {
    if (!selectedEpisode) return
    setPolishingEpisode(true)
    try {
      const res = await fetch(`/api/v1/projects/${projectId}/episodes/${selectedEpisode.id}/polish`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json', ...(typeof window !== 'undefined' && localStorage.getItem('auth_token') ? { Authorization: `Bearer ${localStorage.getItem('auth_token')}` } : {}) },
      })
      if (!res.ok) throw new Error(await res.text())
      const json = await res.json()
      const updated: Episode = json?.data ?? json
      setSelectedEpisode(updated)
      mutateEpisodes()
      toast({ title: 'AI 润色完成', description: '标题、摘要和内容已更新', variant: 'success' })
    } catch {
      toast({ title: 'AI 润色失败，请重试', variant: 'destructive' })
    } finally {
      setPolishingEpisode(false)
    }
  }

  const handleOptimizeEpisode = async (ep: Episode) => {
    setOptimizingEpisode(ep.id)
    try {
      const res = await projectAPI.optimizeEpisode(projectId, ep.id)
      const raw = res as unknown as { data?: Episode }
      const updated: Episode = raw?.data ?? (res as unknown as Episode)
      if (selectedEpisode?.id === ep.id) setSelectedEpisode(updated)
      mutateEpisodes()
      toast({ title: '剧本格式化完成', description: '已生成优化后的剧本格式内容，可在详情中查看并确认应用', variant: 'success' })
    } catch {
      toast({ title: '格式转化失败，请重试', variant: 'destructive' })
    } finally {
      setOptimizingEpisode(null)
    }
  }

  const handleApplyOptimizedText = async (ep: Episode) => {
    setApplyingOptimized(ep.id)
    try {
      const res = await projectAPI.applyOptimizedText(projectId, ep.id)
      const raw = res as unknown as { data?: Episode }
      const updated: Episode = raw?.data ?? (res as unknown as Episode)
      if (selectedEpisode?.id === ep.id) setSelectedEpisode(updated)
      mutateEpisodes()
      toast({ title: '已应用优化内容', description: '优化后的剧本格式已替换原有正文', variant: 'success' })
    } catch {
      toast({ title: '应用失败，请重试', variant: 'destructive' })
    } finally {
      setApplyingOptimized(null)
    }
  }

  const handleReviewEpisode = async (ep: Episode) => {
    setReviewingEpisode(ep.id)
    try {
      const res = await projectAPI.reviewEpisode(projectId, ep.id)
      const raw = res as unknown as { data?: Episode }
      const updated: Episode = raw?.data ?? (res as unknown as Episode)
      if (selectedEpisode?.id === ep.id) setSelectedEpisode(updated)
      mutateEpisodes()
      toast({ title: 'AI 审查完成', description: '已生成一致性与质量审查报告', variant: 'success' })
    } catch {
      toast({ title: 'AI 审查失败，请重试', variant: 'destructive' })
    } finally {
      setReviewingEpisode(null)
    }
  }

  const handleAutoOptimizeReview = async (ep: Episode) => {
    setAutoOptimizingEpisode(ep.id)
    try {
      await projectAPI.autoOptimizeReview(projectId, ep.id)
      // 202 Accepted — backend processes async; spinner kept until polling detects completion
      toast({ title: 'AI 一键优化已启动', description: '正在后台处理，完成后状态将自动更新' })
    } catch {
      setAutoOptimizingEpisode(null)
      toast({ title: '一键优化启动失败，请重试', variant: 'destructive' })
    }
  }

  const handleBatchOptimize = async () => {
    setBatchOptimizing(true)
    try {
      await projectAPI.batchOptimize(projectId)
      toast({ title: '批量格式转化已启动', description: '后台正在处理所有分集，完成后状态将自动更新', variant: 'success' })
    } catch {
      toast({ title: '批量格式转化启动失败', variant: 'destructive' })
    } finally {
      setBatchOptimizing(false)
    }
  }

  const handleBatchReview = async () => {
    setBatchReviewing(true)
    try {
      await projectAPI.batchReview(projectId)
      toast({ title: '批量 AI 审查已启动', description: '后台正在审查所有分集，完成后状态将自动更新', variant: 'success' })
    } catch {
      toast({ title: '批量 AI 审查启动失败', variant: 'destructive' })
    } finally {
      setBatchReviewing(false)
    }
  }

  const handleSplitModelChange = async (value: string) => {
    setDraftSplitModelId(value)
    setSplitSettingsDirty(true)
  }

  const handleImageModelChange = async (value: string) => {
    const nextModelId = Number(value)
    if (!Number.isFinite(nextModelId) || nextModelId === project.image_model_id) return

    setSavingImageModel(true)
    try {
      await projectAPI.update(projectId, { image_model_id: nextModelId } as Partial<Project>)
      toast({ title: '资源图片模型已更新', variant: 'success' })
      mutateProject()
      globalMutate(['project', projectId])
    } catch {
      toast({ title: '资源图片模型更新失败', variant: 'destructive' })
    } finally {
      setSavingImageModel(false)
    }
  }

  const handleStartAssetExtraction = async () => {
    setAssetGenerating(true)
    extractionStartedAtRef.current = Date.now()
    try {
      await assetAPI.extract(projectId)
      mutateExtractAssets()
      globalMutate(['project', projectId])
      toast({ title: '手动提取已启动，旧资源会先清除再重建', variant: 'success' })
      // Poll several times to bridge the delete→sentinel creation race window
      // (backend deletes old assets then creates sentinel asynchronously)
      setTimeout(() => mutateExtractAssets(), 1000)
      setTimeout(() => mutateExtractAssets(), 2500)
      setTimeout(() => mutateExtractAssets(), 5000)
    } catch {
      toast({ title: '资源提取失败', variant: 'destructive' })
      mutateExtractAssets()
      setAssetGenerating(false)
      extractionStartedAtRef.current = null
    }
  }

  const handleExtractEpisodeAssets = async (episodeId: number, episodeNum: number) => {
    setExtractingEpisodeAssets(episodeId)
    try {
      await assetAPI.extractEpisode(projectId, episodeId)
      mutateExtractAssets()
      toast({ title: `第 ${episodeNum} 集资源提取已启动`, variant: 'success' })
      // Poll a few extra times so the sentinel is caught even if the first refresh races ahead
      setTimeout(() => mutateExtractAssets(), 1000)
      setTimeout(() => mutateExtractAssets(), 2500)
    } catch {
      toast({ title: `第 ${episodeNum} 集资源提取失败`, variant: 'destructive' })
    } finally {
      setExtractingEpisodeAssets(null)
    }
  }

  const handleAutoStartEpisodeAssets = async (episodeId: number, episodeNum: number) => {
    setExtractingEpisodeAssets(episodeId)
    try {
      await assetAPI.extractEpisode(projectId, episodeId)
      mutateExtractAssets()
      setTimeout(() => mutateExtractAssets(), 1000)
      setTimeout(() => mutateExtractAssets(), 2500)

      setGeneratingEpisodeAssets(episodeId)
      await assetAPI.generateAll(projectId, episodeId, selectedProjectImageModelName)
      mutateExtractAssets()
      toast({ title: `第 ${episodeNum} 集已自动开始提取并生成`, variant: 'success' })
    } catch {
      toast({ title: `第 ${episodeNum} 集自动提取生成失败`, variant: 'destructive' })
    } finally {
      setExtractingEpisodeAssets(null)
      setGeneratingEpisodeAssets(null)
    }
  }

  const handleGenerateEpisodeAssetsFromScript = async (episodeId: number, episodeNum: number) => {
    setGeneratingEpisodeAssets(episodeId)
    try {
      await assetAPI.generateAll(projectId, episodeId, selectedProjectImageModelName)
      mutateExtractAssets()
      toast({ title: `第 ${episodeNum} 集资源图生成已启动`, variant: 'success' })
    } catch {
      toast({ title: `第 ${episodeNum} 集资源图生成失败`, variant: 'destructive' })
    } finally {
      setGeneratingEpisodeAssets(null)
    }
  }

  const handleDeleteEpisode = async () => {
    if (!episodeDeleteTarget) return
    setDeletingEpisodeId(episodeDeleteTarget.id)
    try {
      await projectAPI.deleteEpisode(projectId, episodeDeleteTarget.id)
      toast({ title: `第 ${episodeDeleteTarget.episode_number} 集已删除`, variant: 'success' })
      mutateEpisodes()
      mutateProject()
    } catch {
      toast({ title: '删除失败，请稍后重试', variant: 'destructive' })
    } finally {
      setDeletingEpisodeId(null)
      setEpisodeDeleteTarget(null)
    }
  }


  return {
    fileRef,
    episodesLoading,
    hasScriptText,
    scriptText,
    episodes,
    pipeline,
    splitInProgress,
    splitConfigReady,
    effectiveSplitModel,
    splitModels,
    textModelsLoading,
    usesAutoEpisodeSplit,
    splitSettingsDirty,
    setSplitSettingsDirty,
    selectedSplitModelAvailability,
    selectedSplitModelProvider,
    hasValidTargetEpisodes,
    parsedTargetEpisodes,
    recommendedEpisodeCount,
    draftTargetEpisodes,
    setDraftTargetEpisodes,
    showSplitAdvancedSettings,
    setShowSplitAdvancedSettings,
    savingSplitModel,
    isProcessing,
    shouldShowSplitSearch,
    splitModelSearch,
    setSplitModelSearch,
    draftSplitModelId,
    filteredSplitModels,
    splitModelCapabilities,
    selectedSplitModelRemark,
    textModelHealthMap,
    splitProgressSummary,
    splitProgressPercent,
    sceneReadyCount,
    sceneProcessingSummary,
    sceneProcessingProgress,
    scriptTabStoryboards,
    extractionInProgress,
    assetGenerating,
    extractionDone,
    extractTotal,
    episodeStoryboardDispatching,
    deletingEpisodeId,
    extractingEpisodeAssets,
    generatingEpisodeAssets,
    autoOptimizingEpisode,
    optimizingEpisode,
    reviewingEpisode,
    applyingOptimized,
    showRegenerateDialog,
    setShowRegenerateDialog,
    showCreateEpisodeDialog,
    setShowCreateEpisodeDialog,
    showScriptPreviewDialog,
    setShowScriptPreviewDialog,
    selectedEpisode,
    setSelectedEpisode,
    editingEpisode,
    setEditingEpisode,
    editEpisodeTitle,
    setEditEpisodeTitle,
    editEpisodeSummary,
    setEditEpisodeSummary,
    editEpisodeContent,
    setEditEpisodeContent,
    polishingEpisode,
    savingEpisodeEdit,
    episodeDeleteTarget,
    setEpisodeDeleteTarget,
    kwSplitKeywords,
    setKwSplitKeywords,
    kwCharacters,
    setKwCharacters,
    kwLocations,
    setKwLocations,
    kwEvents,
    setKwEvents,
    kwProps,
    setKwProps,
    autoStoryboardAfterSplit,
    setAutoStoryboardAfterSplit,
    nextManualEpisodeNumber,
    manualEpisodeNumber,
    setManualEpisodeNumber,
    manualEpisodeNumberTaken,
    parsedManualEpisodeNumber,
    manualEpisodeTitle,
    setManualEpisodeTitle,
    manualEpisodeSummary,
    setManualEpisodeSummary,
    manualEpisodeContent,
    setManualEpisodeContent,
    creatingEpisode,
    handleUpload,
    handleOpenRegenerate,
    handleGenerateEpisodes,
    handleOpenCreateEpisode,
    handleCreateEpisode,
    handleOpenEditEpisode,
    handleSaveEpisodeEdit,
    handlePolishEpisode,
    handleOptimizeEpisode,
    handleApplyOptimizedText,
    handleReviewEpisode,
    handleAutoOptimizeReview,
    handleSplitModelChange,
    handleStartAssetExtraction,
    handleExtractEpisodeAssets,
    handleAutoStartEpisodeAssets,
    handleGenerateEpisodeAssetsFromScript,
    handleDeleteEpisode,
    handleRetryStalledScript,
    handleStartEpisodeStoryboard,
  }
}
