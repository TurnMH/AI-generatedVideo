'use client'

import React, { useState, useRef, useEffect, useMemo } from 'react'
import { useParams, useRouter } from 'next/navigation'
import useSWR, { mutate as globalMutate } from 'swr'
import {
  ArrowLeft,
  FileText,
  Image,
  LayoutGrid,
  Mic,
  Video,
  Upload,
  RefreshCw,
  Trash2,
  Lock,
  Unlock,
  Play,
  Pause,
  Download,
  Send,
  X,
  ChevronLeft,
  ChevronRight,
  HardDrive,
  Eye,
  Sparkles,
  Ban,
  MessageSquare,
  CheckCircle2,
  AlertCircle,
  Clock,
  Loader2,
  Plus,
  Search,
  ChevronDown,
  Film,
  Star,
  BookOpen,
  Pencil,
  RotateCcw,
} from 'lucide-react'
import { projectAPI, assetAPI, storyboardAPI, storageAPI, videoAPI, dubbingAPI, modelAPI, utilsAPI, type DubbingTask } from '@/lib/api'
import { ProductionSkillsPanel } from '@/components/skills/ProductionSkillsPanel'
import type {
  Project,
  Episode,
  Asset,
  AssetType,
  AssetStatus,
  Storyboard,
  StoryboardVersion,
  Video as VideoType,
  StorageDetails,
  StorageFile,
  ChatMessage,
  Model,
} from '@/types'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Progress } from '@/components/ui/progress'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Label } from '@/components/ui/label'
import { Separator } from '@/components/ui/separator'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { ZoomableImage, ZoomBadge } from '@/components/ui/image-lightbox'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
} from '@/components/ui/dropdown-menu'
import { LoadingSpinner } from '@/components/common/LoadingSpinner'
import { useToast } from '@/components/ui/toast'
import { Switch } from '@/components/ui/switch'
import { format } from 'date-fns'
import { getProviderLabel, getRuntimeModelCapabilityLabels } from '@/lib/model-feasibility'
import { pickPreferredModel } from '@/lib/model-selection'
import {
  normalizeVideoStylePreset,
  VIDEO_STYLE_COMPACT_OPTIONS,
  VIDEO_STYLE_FILTERS,
  VIDEO_STYLE_PRESETS,
  VIDEO_STYLE_MODE_META,
  VIDEO_MOTION_OPTIONS,
  VIDEO_MODEL_SELECTION_HINTS,
  VIDEO_GENERATION_PRESETS,
  type VideoGenerationPreset,
  type VideoModelCapability,
  type VideoStylePreset,
  type VideoStyleMode,
} from '@/lib/video-style-config'
import { buildImageModelOption, buildVideoModelOption, buildVideoModelCapability, dedupeModels } from '@/lib/model-display'

import { formatBytes, formatDuration, parseTimestampMs, formatRuntimeDuration, getElapsedTimeLabel, getEstimatedRemainingLabel, getTimingSummary, getEarliestTimestamp, getProgressStallMeta, getPendingQueueMeta, formatChatTimestamp, SCRIPT_PROGRESS_STALL_MS, TASK_PROGRESS_STALL_MS } from '@/lib/projects/utils'
import { FALLBACK_VOICE_OPTIONS, GENERATION_STAGE_HINTS } from '@/lib/projects/constants'
import { STATUS_MAP } from '@/lib/projects/status'
import type { LegacyChatMessage } from '@/lib/projects/chat'
import { getChatRole, getChatContent, getChatImageUrl, getChatImageModel } from '@/lib/projects/chat'
import { COMIC_STYLE_PRESETS, splitEpisodeIntoComicPanels, recommendEpisodeCount } from '@/lib/projects/comic'
import type { ComicStylePresetKey, EpisodeCountRecommendation } from '@/lib/projects/comic'
import { getAssetGeneratedImages, getSelectedGeneratedImageUrl, getAssetGenerationProgress, getGenerationStageHint, getGenerationEtaLabel, getGenerationElapsedLabel } from '@/lib/projects/assets'
import type { AssetImageVersion, AssetGenerationProgress } from '@/lib/projects/assets'
import { getSplitModelRemark, buildSplitModelSearchText, getSplitModelAvailabilityRank, mapVideoModelToRuntimeKey, findPreferredVideoModelId } from '@/lib/projects/models'
import { useProjectEpisodeFilter, ProjectEpisodeFilterContext } from '@/lib/projects/episode-filter'
import type { StoryboardStatsData, StepAssetStats, StepStoryboardStats, StepDubbingStats, StepVideoStats, WorkflowStepKey, WorkflowStepView, WorkflowStepStatus } from '@/lib/projects/workflow'
import { buildWorkflowSteps, getDisplayedEpisodeCount, toPercent, getIssueStepIndex, WORKFLOW_STEPS } from '@/lib/projects/workflow'
import { StatusBadge, VideoTaskStatusBadge } from '@/components/projects/detail/StatusBadge'
import { TabSkeleton } from '@/components/projects/detail/TabSkeleton'
import { CharacterPanelStrip } from '@/components/projects/detail/CharacterPanelStrip'
import { EpisodeStoryboardList } from '@/components/projects/detail/EpisodeStoryboardList'

type TabKey = WorkflowStepKey

function resolveDraftTargetEpisodes(
  targetEpisodes: number,
  recommendation: EpisodeCountRecommendation | null | undefined,
  hasGeneratedEpisodes: boolean
): string {
  if (
    recommendation &&
    !hasGeneratedEpisodes &&
    (targetEpisodes <= 0 || (targetEpisodes === 1 && recommendation.count !== 1))
  ) {
    return String(recommendation.count)
  }

  return targetEpisodes > 0 ? String(targetEpisodes) : ''
}


export function ScriptTab({
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
  const getApiErrorMessage = (error: unknown) => {
    const response = (error as { response?: { data?: { message?: string; error?: string } } })?.response?.data
    return response?.message || response?.error || (error as { message?: string })?.message || ''
  }
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

  // Derive processing state from project status OR local trigger
  const isProcessing = project.status === 'script_processing' || episodeGenerating

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
  const splitModelCapabilities = selectedSplitModel ? getRuntimeModelCapabilityLabels(selectedSplitModel) : []
  const selectedSplitModelRemark = selectedSplitModel ? getSplitModelRemark(selectedSplitModel) : ''
  const selectedSplitModelProvider = selectedSplitModel ? getProviderLabel(selectedSplitModel.provider) : null
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
  const getProjectModelAvailability = (model: Model) => {
    const health = textModelHealthMap[model.name] ?? model.health_status ?? 'unknown'
    if (!model.is_active) return { label: '未启用', color: 'bg-zinc-100 text-zinc-700' }
    if (health === 'healthy') return { label: '可用', color: 'bg-emerald-100 text-emerald-800' }
    if (health === 'unhealthy') return { label: '连接异常', color: 'bg-red-100 text-red-800' }
    return { label: '已启用', color: 'bg-blue-100 text-blue-800' }
  }
  const selectedSplitModelAvailability = selectedSplitModel ? getProjectModelAvailability(selectedSplitModel) : null
  const selectedImageModelAvailability = selectedImageModel ? getProjectModelAvailability(selectedImageModel) : null
  const parsedTargetEpisodes = Number.parseInt(draftTargetEpisodes, 10)
  const hasValidTargetEpisodes = Number.isFinite(parsedTargetEpisodes) && parsedTargetEpisodes >= 1 && parsedTargetEpisodes <= 200
  const splitConfigReady = !!selectedSplitModel && hasValidTargetEpisodes
  const shouldShowSplitSearch = splitModels.length > 8
  const filteredSplitModels = useMemo(() => {
    const keyword = splitModelSearch.trim().toLocaleLowerCase()
    if (!keyword) return splitModels
    return splitModels.filter((model) => buildSplitModelSearchText(model).includes(keyword))
  }, [splitModels, splitModelSearch])

  const { data: episodesData, isLoading: episodesLoading, mutate: mutateEpisodes } = useSWR(
    ['episodes', projectId],
    () => projectAPI.listEpisodes(projectId) as unknown as Promise<{ data: Episode[] }>,
    {
      refreshInterval: (data) => {
        if (isProcessing) return 3000
        const eps = (data as { data?: Episode[] })?.data ?? []
        if (eps.some((ep) => ep.optimize_status === 'optimizing' || ep.optimize_status === '' || ep.review_status === 'reviewing')) return 3000
        return 0
      },
    }
  )
  const episodes = (episodesData as { data?: Episode[] })?.data ?? []
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
    { refreshInterval: isProcessing ? 5000 : 0 }
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
    if (splitSettingsDirty) return
    setDraftSplitModelId(project.text_model_id ? String(project.text_model_id) : '')
    setDraftTargetEpisodes(resolveDraftTargetEpisodes(project.target_episodes, recommendedEpisodeCount, episodes.length > 0))
  }, [episodes.length, project.target_episodes, project.text_model_id, recommendedEpisodeCount, splitSettingsDirty])

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
    if (!selectedSplitModel) {
      toast({ title: '请先选择分集模型', variant: 'destructive' })
      return false
    }
    if (!hasValidTargetEpisodes) {
      toast({ title: '请填写 1-200 的目标分集数', variant: 'destructive' })
      return false
    }

    const nextTargetEpisodes = parsedTargetEpisodes
    const shouldUpdate =
      project.text_model_id !== selectedSplitModel.id ||
      project.target_episodes !== nextTargetEpisodes

    if (!shouldUpdate) {
      return true
    }

    setSavingSplitModel(true)
    try {
      await projectAPI.update(projectId, {
        text_model_id: selectedSplitModel.id,
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
      if (!splitSettingsDirty) {
        setDraftTargetEpisodes(resolveDraftTargetEpisodes(uploadedProject?.target_episodes ?? 0, uploadedRecommendation, false))
      }
      toast({
        title: '上传成功，请选择模型后手动开始分集',
        description: uploadedRecommendation ? `已推荐 ${uploadedRecommendation.count} 个分集，${uploadedRecommendation.reason}` : '可按需要手动填写目标分集数',
        variant: 'success',
      })
      globalMutate(['project', projectId])
    } catch {
      toast({ title: '上传失败', variant: 'destructive' })
    }
    if (fileRef.current) fileRef.current.value = ''
  }

  const handleGenerateEpisodes = async () => {
    if (!project.script_file_url && !hasScriptText) {
      toast({ title: '请先上传剧本文件', variant: 'destructive' })
      return
    }
    if (!splitConfigReady) {
      toast({ title: !selectedSplitModel ? '请先选择分集模型' : '请填写 1-200 的目标分集数', variant: 'destructive' })
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
        return
      }
    } catch {
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
      await projectAPI.generateEpisodes(projectId, hasKeywords ? keywords : undefined, { autoStoryboard: autoStoryboardAfterSplit })
      toast({
        title: autoStoryboardAfterSplit ? '自动分集已启动，分镜需等待资源图全部完成后再开始' : '自动分集已启动，完成后可手动开始分镜',
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
      toast({ title: !selectedSplitModel ? '请先选择分集模型' : '请填写 1-200 的目标分集数', variant: 'destructive' })
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
    if (!splitConfigReady) {
      toast({ title: !selectedSplitModel ? '请先选择分集模型' : '请填写 1-200 的目标分集数', variant: 'destructive' })
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

      const nextEpisodeTarget = Math.max(project.target_episodes || 0, episodes.length + 1, parsedManualEpisodeNumber)
      if (nextEpisodeTarget !== project.target_episodes) {
        await projectAPI.update(projectId, { target_episodes: nextEpisodeTarget } as Partial<Project>)
      }

      setDraftTargetEpisodes(String(nextEpisodeTarget))
      setSplitSettingsDirty(false)
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
    try {
      await assetAPI.extractEpisode(projectId, episodeId)
      mutateExtractAssets()
      toast({ title: `第 ${episodeNum} 集资源提取已启动`, variant: 'success' })
      // Poll a few extra times so the sentinel is caught even if the first refresh races ahead
      setTimeout(() => mutateExtractAssets(), 1000)
      setTimeout(() => mutateExtractAssets(), 2500)
    } catch {
      toast({ title: `第 ${episodeNum} 集资源提取失败`, variant: 'destructive' })
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

  if (episodesLoading) return <TabSkeleton />

  return (
    <div className="space-y-6">
      {/* Script file info */}
      <Card>
        <CardHeader className="pb-3">
          <div className="flex items-center justify-between">
            <CardTitle className="text-base">剧本文件</CardTitle>
            <div className="flex gap-2">
              <input ref={fileRef} type="file" accept=".txt,.pdf,.docx,.md" className="hidden" onChange={handleUpload} />
              <Button size="sm" variant="outline" onClick={() => fileRef.current?.click()} title="上传新版本的剧本文件">
                <Upload className="mr-1.5 h-3.5 w-3.5" />
                上传新版本
              </Button>
            </div>
          </div>
        </CardHeader>
        <CardContent>
          {project.script_file_url ? (
            <div className="space-y-4">
              <div className="flex flex-wrap items-center justify-between gap-3 text-sm text-surface-600">
                <div className="flex flex-wrap items-center gap-4">
                  <span className="flex items-center gap-1.5">
                    <FileText className="h-4 w-4 text-primary-500" />
                    {project.script_file_url.split('/').pop() || '剧本文件'}
                  </span>
                  <span>{formatBytes(project.script_file_size || 0)}</span>
                  <span>上传于 {format(new Date(project.updated_at), 'yyyy-MM-dd HH:mm')}</span>
                </div>
                <div className="flex flex-wrap gap-2">
                  {hasScriptText && (
                    <Button size="sm" variant="outline" onClick={() => setShowScriptPreviewDialog(true)} title="查看剧本全文">
                      <Eye className="mr-1.5 h-3.5 w-3.5" />
                      查看全文
                    </Button>
                  )}
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => window.open(project.script_file_url, '_blank', 'noopener,noreferrer')}
                    title="打开原始剧本文件"
                  >
                    <Download className="mr-1.5 h-3.5 w-3.5" />
                    打开原文件
                  </Button>
                </div>
              </div>


            </div>
          ) : (
            <p className="text-sm text-surface-400">尚未上传剧本文件</p>
          )}
        </CardContent>
      </Card>

      {/* Episode config & list */}
      <Card>
        <CardHeader className="pb-3">
          <div className="flex items-center justify-between">
            <CardTitle className="text-base">分集列表</CardTitle>
            <div className="flex items-center gap-2">
              <Button
                size="sm"
                variant="outline"
                onClick={handleOpenCreateEpisode}
                title="手动补充一集，可直接用于资源提取、分镜和视频流程"
              >
                <Plus className="mr-1.5 h-3.5 w-3.5" />
                手动创建分集
              </Button>
              <Button
                size="sm"
                variant={episodes.length > 0 ? 'outline' : 'default'}
                onClick={handleOpenRegenerate}
                disabled={isProcessing || !splitConfigReady || (!project.script_file_url && !hasScriptText)}
                title={isProcessing
                  ? (project.progress?.message || '当前已有分集任务进行中，请等待当前任务完成')
                  : (episodes.length > 0 ? '按当前手动配置重新自动分集' : '按当前手动配置开始自动分集')}
              >
                {isProcessing ? (
                  <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
                ) : (
                  <RefreshCw className="mr-1.5 h-3.5 w-3.5" />
                )}
                {isProcessing ? '分集进行中' : (episodes.length > 0 ? 'AI 重新分集' : 'AI 开始分集')}
              </Button>
            </div>
          </div>
        </CardHeader>
        <CardContent>
          <div className="mb-4 rounded-xl border border-primary-100 bg-primary-50/50 p-4">
            <div>
              <div className="rounded-lg border border-white/80 bg-white/80 p-4 shadow-sm">
                {textModelsLoading ? (
                  <div className="h-10 animate-pulse rounded-md bg-surface-100" />
                ) : splitModels.length === 0 ? (
                  <p className="text-xs text-surface-400">暂无可选文本模型</p>
                ) : (
                  <div className="space-y-4">
                    <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
                      <div className="min-w-0 flex-1 space-y-3">
                        <div className="flex flex-wrap items-center gap-2">
                          <p className="text-sm font-semibold text-surface-900">分集高级设置</p>
                          <Badge variant="outline" className="text-[11px]">
                            {splitSettingsDirty ? '待应用' : '按当前配置执行'}
                          </Badge>
                          {selectedSplitModelAvailability ? (
                            <span className={`rounded-full px-2 py-0.5 text-[10px] font-medium ${selectedSplitModelAvailability.color}`}>
                              {selectedSplitModelAvailability.label}
                            </span>
                          ) : null}
                        </div>
                        <p className="text-xs leading-5 text-surface-500">
                          这一块默认收起。只有在需要更换分集模型、调整目标集数或手动覆盖推荐值时，再展开修改即可。
                        </p>

                        <div className="grid gap-2 md:grid-cols-3">
                          <div className="rounded-xl border border-surface-200 bg-surface-50/70 px-3 py-2">
                            <p className="text-[11px] text-surface-400">当前模型</p>
                            <p className="truncate text-sm font-medium text-surface-900">{selectedSplitModel?.name || '未选择分集模型'}</p>
                            <p className="truncate text-[11px] text-surface-500">{selectedSplitModelProvider || '展开后可切换模型'}</p>
                          </div>
                          <div className="rounded-xl border border-surface-200 bg-surface-50/70 px-3 py-2">
                            <p className="text-[11px] text-surface-400">目标分集数</p>
                            <p className="text-sm font-medium text-surface-900">{hasValidTargetEpisodes ? `${parsedTargetEpisodes} 集` : '未设置'}</p>
                            <p className="text-[11px] text-surface-500">启动自动分集时会按这里的集数执行</p>
                          </div>
                          <div className="rounded-xl border border-surface-200 bg-surface-50/70 px-3 py-2">
                            <p className="text-[11px] text-surface-400">智能建议</p>
                            <p className="text-sm font-medium text-surface-900">
                              {recommendedEpisodeCount ? `推荐 ${recommendedEpisodeCount.count} 集` : '暂无推荐'}
                            </p>
                            <p className="truncate text-[11px] text-surface-500">{recommendedEpisodeCount?.reason || '上传并解析剧本后自动计算推荐值'}</p>
                          </div>
                        </div>
                      </div>

                      <div className="flex shrink-0 flex-wrap items-center gap-2">
                        {recommendedEpisodeCount && draftTargetEpisodes !== String(recommendedEpisodeCount.count) ? (
                          <Button
                            type="button"
                            size="sm"
                            variant="outline"
                            onClick={() => {
                              setDraftTargetEpisodes(String(recommendedEpisodeCount.count))
                              setSplitSettingsDirty(true)
                            }}
                            disabled={savingSplitModel || isProcessing}
                          >
                            采用推荐
                          </Button>
                        ) : null}
                        <Button
                          type="button"
                          size="sm"
                          variant="outline"
                          onClick={() => setShowSplitAdvancedSettings((value) => !value)}
                        >
                          <ChevronDown className={`mr-1.5 h-3.5 w-3.5 transition-transform ${showSplitAdvancedSettings ? 'rotate-180' : ''}`} />
                          {showSplitAdvancedSettings ? '收起设置' : '调整设置'}
                        </Button>
                      </div>
                    </div>

                    {showSplitAdvancedSettings ? (
                      <div className="space-y-4 border-t border-surface-100 pt-4">
                        {shouldShowSplitSearch ? (
                          <div className="space-y-1.5">
                            <div className="flex items-center justify-between gap-2">
                              <Label className="text-xs font-medium text-surface-700">搜索筛选</Label>
                              <span className="text-[11px] text-surface-400">支持中文搜索</span>
                            </div>
                            <div className="relative">
                              <Search className="pointer-events-none absolute left-3 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-surface-400" />
                              <Input
                                value={splitModelSearch}
                                onChange={(event) => setSplitModelSearch(event.target.value)}
                                placeholder="搜索模型、供应商、能力或备注"
                                className="bg-white pl-8 pr-8"
                                disabled={savingSplitModel || isProcessing}
                              />
                              {splitModelSearch ? (
                                <button
                                  type="button"
                                  className="absolute right-2 top-1/2 -translate-y-1/2 rounded p-1 text-surface-400 transition hover:bg-surface-100 hover:text-surface-600"
                                  onClick={() => setSplitModelSearch('')}
                                  disabled={savingSplitModel || isProcessing}
                                  aria-label="清空模型搜索"
                                >
                                  <X className="h-3.5 w-3.5" />
                                </button>
                              ) : null}
                            </div>
                          </div>
                        ) : null}

                        <div className="space-y-1.5">
                          <Label className="text-xs font-medium text-surface-700">选择模型</Label>
                          <Select
                            value={draftSplitModelId}
                            onValueChange={handleSplitModelChange}
                            disabled={savingSplitModel || isProcessing}
                          >
                            <SelectTrigger className="h-auto min-h-12 bg-white py-2.5">
                              {selectedSplitModel ? (
                                <div className="min-w-0 flex-1 text-left">
                                  <p className="truncate text-sm font-medium text-surface-900">{selectedSplitModel.name}</p>
                                  <p className="truncate text-[11px] text-surface-400">{selectedSplitModelRemark}</p>
                                </div>
                              ) : (
                                <span className="truncate text-surface-400">手动选择分集模型</span>
                              )}
                            </SelectTrigger>
                            <SelectContent>
                              {filteredSplitModels.length > 0 ? (
                                filteredSplitModels.map((model) => {
                                  const availability = getProjectModelAvailability(model)
                                  const remark = getSplitModelRemark(model)
                                  return (
                                    <SelectItem
                                      key={model.id}
                                      value={model.id.toString()}
                                      textValue={`${model.name} ${getProviderLabel(model.provider)} ${remark}`}
                                    >
                                      <div className="flex max-w-[360px] flex-col gap-1 py-0.5">
                                        <div className="flex min-w-0 items-center gap-2">
                                          <span className="truncate font-medium">{model.name}</span>
                                          <span className="text-xs text-surface-400">({getProviderLabel(model.provider)})</span>
                                          <span className={`rounded-full px-1.5 py-0.5 text-[10px] font-medium ${availability.color}`}>
                                            {availability.label}
                                          </span>
                                          {model.is_default ? (
                                            <Badge variant="outline" className="px-1 py-0 text-[10px]">
                                              默认
                                            </Badge>
                                          ) : null}
                                        </div>
                                        <p className="truncate text-[11px] leading-4 text-surface-500">{remark}</p>
                                      </div>
                                    </SelectItem>
                                  )
                                })
                              ) : (
                                <div className="px-3 py-2 text-xs text-surface-400">
                                  未找到匹配的模型，请尝试搜索模型名、供应商或“推理 / 长上下文”等中文关键词
                                </div>
                              )}
                            </SelectContent>
                          </Select>
                          {splitModelCapabilities.length > 0 ? (
                            <p className="text-[11px] leading-4 text-surface-500">
                              能力标签：{splitModelCapabilities.join(' · ')}
                            </p>
                          ) : null}
                        </div>

                        <div className="space-y-1.5">
                          <Label className="text-xs font-medium text-surface-700">目标分集数</Label>
                          <Input
                            type="number"
                            min={1}
                            max={200}
                            step={1}
                            value={draftTargetEpisodes}
                            onChange={(event) => {
                              setDraftTargetEpisodes(event.target.value.replace(/[^\d]/g, ''))
                              setSplitSettingsDirty(true)
                            }}
                            placeholder="填写需要拆分的分集数量，例如 12"
                            disabled={savingSplitModel || isProcessing}
                            className="bg-white"
                          />
                          {recommendedEpisodeCount ? (
                            <p className="text-[11px] leading-4 text-emerald-700">
                              推荐 {recommendedEpisodeCount.count} 段（{recommendedEpisodeCount.reason}）
                              {draftTargetEpisodes === String(recommendedEpisodeCount.count) ? '，当前已采用推荐值。' : '。'}
                            </p>
                          ) : null}
                        </div>
                      </div>
                    ) : null}
                  </div>
                )}
              </div>
            </div>
          </div>

          {isProcessing && episodes.length === 0 ? (
            <div className="rounded-xl border border-primary-100 bg-gradient-to-b from-primary-50/80 to-white px-5 py-5">
              <div className="flex items-start gap-3">
                <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-primary-100">
                  <Loader2 className="h-4 w-4 animate-spin text-primary-600" />
                </div>
                <div className="min-w-0 flex-1">
                  <div className="flex flex-wrap items-center gap-2">
                    <p className="text-sm font-semibold text-primary-900">分集生成进行中</p>
                    <span className="rounded-full bg-white px-2 py-0.5 text-[10px] font-medium text-primary-600 shadow-sm">
                      总控进度已同步到上方
                    </span>
                    {project.progress?.episode_split?.total ? (
                      <span className="rounded-full bg-primary-100 px-2 py-0.5 text-[10px] font-medium text-primary-700">
                        {project.progress.episode_split.completed ?? 0}/{project.progress.episode_split.total} 集
                      </span>
                    ) : null}
                  </div>
                  <p className="mt-1 text-sm leading-6 text-primary-800">{splitProgressSummary}</p>
                  <p className="mt-1 text-xs leading-5 text-primary-600">
                    分集、资源提取、分镜格式化等进度已统一汇总到上方“剧本大纲与项目总控”，这里仅保留当前阶段摘要。分集完成后会自动出现在下方列表。
                  </p>
                  {splitProgressPercent > 0 ? (
                    <div className="mt-3 h-1.5 w-full overflow-hidden rounded-full bg-primary-100">
                      <div className="h-full rounded-full bg-primary-500 transition-all duration-700" style={{ width: `${splitProgressPercent}%` }} />
                    </div>
                  ) : null}
                </div>
              </div>

              {scriptProgressStalled && (
                <div className="mt-4 rounded-lg border border-amber-200 bg-amber-50 px-4 py-2.5 text-xs text-amber-700">
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <span>剧本解析超过 2 分钟无进展，可能已卡住。</span>
                    <Button size="sm" variant="outline" className="h-7 border-amber-300 bg-white text-amber-700 hover:bg-amber-100" onClick={handleRetryStalledScript}>
                      <RefreshCw className="mr-1 h-3 w-3" />
                      重新拉起
                    </Button>
                  </div>
                </div>
              )}
            </div>
          ) : isProcessing && episodes.length > 0 ? (
            <div className="space-y-4">
              <div className="rounded-xl border border-violet-200 bg-gradient-to-r from-violet-50 to-purple-50 px-4 py-3.5">
                <div className="flex items-start gap-3">
                  <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-violet-100">
                    <Loader2 className="h-4 w-4 animate-spin text-violet-600" />
                  </div>
                  <div className="min-w-0 flex-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <p className="text-sm font-semibold text-violet-900">分镜序列格式化进行中</p>
                      <span className="inline-flex items-center gap-1 rounded-full bg-white px-2 py-0.5 text-[10px] font-medium text-violet-600 shadow-sm">
                        总控进度已同步到上方
                      </span>
                      <span className="inline-flex items-center gap-1 rounded-full bg-violet-100 px-2 py-0.5 text-[10px] font-medium text-violet-600">
                        {sceneReadyCount}/{Math.max(episodes.length, 1)} 集就绪
                      </span>
                    </div>
                    <p className="mt-1 text-sm leading-6 text-violet-800">
                      {project.progress?.message || `分集已完成（${episodes.length} 集），AI 正在逐集拆分分镜序列…`}
                    </p>
                    <p className="mt-1 text-xs text-violet-600">{sceneProcessingSummary}</p>
                    {storyboardSplitTiming ? (
                      <p className="mt-1 text-[11px] text-violet-400">{storyboardSplitTiming}</p>
                    ) : null}
                    <div className="mt-2 h-1.5 w-full overflow-hidden rounded-full bg-violet-200">
                      <div className="h-full rounded-full bg-violet-500 transition-all duration-700" style={{ width: `${sceneProcessingProgress}%` }} />
                    </div>
                    {scriptProgressStalled && (
                      <div className="mt-2 rounded-lg border border-amber-200 bg-amber-50/80 px-3 py-2 text-xs text-amber-800">
                        <div className="flex flex-wrap items-center justify-between gap-2">
                          <span>分镜格式化进度长时间未更新，可能已卡住。</span>
                          <Button size="sm" variant="outline" className="h-7 border-amber-300 bg-white text-amber-700 hover:bg-amber-100" onClick={handleRetryStalledScript}>
                            <RefreshCw className="mr-1 h-3 w-3" />
                            重新拉起
                          </Button>
                        </div>
                      </div>
                    )}
                  </div>
                </div>
              </div>
              <div className="grid grid-cols-1 gap-3 md:grid-cols-2 lg:grid-cols-3">
                {episodes.map((ep) => {
                  const epSbCount = scriptTabStoryboards.filter(sb => sb.episode_id === ep.id).length
                  const statusBadge = ep.status === 'script_prepping' ? (
                    <Badge variant="default" className="gap-1 bg-violet-500 hover:bg-violet-500"><Loader2 className="h-3 w-3 animate-spin" />优化中</Badge>
                  ) : ep.status === 'scene_splitting' ? (
                    <Badge variant="default" className="gap-1"><Loader2 className="h-3 w-3 animate-spin" />拆分中</Badge>
                  ) : ep.status === 'scene_ready' || ep.status === 'done' ? (
                    <Badge variant="success">{epSbCount} 个分镜</Badge>
                  ) : ep.status === 'failed' ? (
                    <Badge variant="destructive">失败</Badge>
                  ) : ep.status === 'pending' ? (
                    <Badge variant="secondary">等待中</Badge>
                  ) : epSbCount > 0 ? (
                    <Badge variant="success">{epSbCount} 个分镜</Badge>
                  ) : (
                    <Badge variant="secondary">等待分镜</Badge>
                  )
                  return (
                    <div
                      key={ep.id}
                      className="relative cursor-pointer rounded-lg border p-4 transition-colors hover:bg-surface-50"
                      onClick={() => setSelectedEpisode(ep)}
                    >
                      {(ep.status === 'scene_splitting' || ep.status === 'script_prepping') && (
                        <div className="absolute right-3 top-3">
                          <Loader2 className="h-3.5 w-3.5 animate-spin text-amber-400" />
                        </div>
                      )}
                      <div className="mb-2 flex items-center justify-between">
                        <span className="text-sm font-semibold">第 {ep.episode_number} 集</span>
                        <div className="flex items-center gap-2">
                          {statusBadge}
                        </div>
                      </div>
                      <p className="mb-1 text-sm font-medium text-surface-800">{ep.title}</p>
                      <p className="mb-2 line-clamp-2 text-xs text-surface-500">{ep.summary || ep.script_excerpt?.slice(0, 80)}</p>
                      <div className="flex items-center justify-between">
                        <div className="flex gap-3 text-xs text-surface-400">
                          <span>{ep.word_count ? `${ep.word_count} 字` : null}</span>
                          <span>{ep.estimated_duration ? `~${formatDuration(ep.estimated_duration)}` : null}</span>
                        </div>
                        {/* Per-episode generate storyboard button */}
                        <div className="flex items-center gap-1">
                          <Button
                            size="sm"
                            variant="ghost"
                            className="h-6 px-2 text-xs text-primary-600 hover:bg-primary-50"
                            onClick={(e) => { e.stopPropagation(); void handleStartEpisodeStoryboard(ep.id) }}
                            disabled={episodeStoryboardDispatching === ep.id}
                            title="为本集生成分镜"
                          >
                            {episodeStoryboardDispatching === ep.id ? (
                              <Loader2 className="mr-1 h-3 w-3 animate-spin" />
                            ) : (
                              <LayoutGrid className="mr-1 h-3 w-3" />
                            )}
                            生成分镜
                          </Button>
                          <Button
                            size="sm"
                            variant="ghost"
                            className="h-6 w-6 p-0 text-red-400 hover:bg-red-50 hover:text-red-600"
                            onClick={(e) => { e.stopPropagation(); setEpisodeDeleteTarget(ep) }}
                            disabled={deletingEpisodeId === ep.id}
                            title="删除本集"
                          >
                            {deletingEpisodeId === ep.id ? (
                              <Loader2 className="h-3 w-3 animate-spin" />
                            ) : (
                              <Trash2 className="h-3 w-3" />
                            )}
                          </Button>
                        </div>
                      </div>
                    </div>
                  )
                })}
              </div>
            </div>
          ) : episodes.length === 0 ? (
            <div className="py-6 text-center text-sm text-surface-400">
              暂无分集，点击右上角「手动创建分集」添加，或上传剧本后 AI 自动分集。
            </div>
          ) : (
            <>
              {/* Asset extraction trigger — top position */}
              {/* Batch script formatting progress banner */}
          {(() => {
            const formattingCount = episodes.filter((ep) => ep.optimize_status === 'optimizing').length
            const formattedCount = episodes.filter((ep) => ep.optimize_status === 'done').length
            const pendingCount = episodes.filter((ep) => (ep.optimize_status ?? '') === '').length
            const isAutoFormatting = formattingCount > 0 || (formattedCount > 0 && formattedCount < episodes.length && pendingCount > 0)
            if (!isAutoFormatting && formattingCount === 0) return null
            const progressPct = episodes.length > 0 ? Math.round((formattedCount / episodes.length) * 100) : 0
            return (
              <div className="mb-4 rounded-xl border border-violet-200 bg-gradient-to-r from-violet-50 to-purple-50/60 px-4 py-3">
                <div className="flex items-center gap-3">
                  <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-violet-100">
                    <Loader2 className="h-4 w-4 animate-spin text-violet-600" />
                  </div>
                  <div className="flex-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <p className="text-sm font-semibold text-violet-900">AI 自动处理中 · 分镜序列格式化</p>
                      <span className="rounded-full bg-violet-100 px-2 py-0.5 text-[10px] font-medium text-violet-600">
                        {formattedCount}/{episodes.length} 集
                      </span>
                    </div>
                    <p className="mt-0.5 text-xs text-violet-600">
                      {formattingCount > 0 ? `当前 ${formattingCount} 集正在格式化` : `剩余 ${pendingCount} 集待处理`}，格式化完成后即可进入各集工作台进行资源生成与出图
                    </p>
                    <div className="mt-1.5 h-1.5 w-full overflow-hidden rounded-full bg-violet-200">
                      <div
                        className="h-full rounded-full bg-violet-500 transition-all duration-700"
                        style={{ width: `${progressPct}%` }}
                      />
                    </div>
                  </div>
                </div>
              </div>
            )
          })()}

          {['draft', 'script_processing', 'script_ready', 'asset_generating'].includes(project.status) && (
                <div className={`mb-4 rounded-lg border px-4 py-3 ${
                  extractionInProgress || assetGenerating
                    ? 'border-yellow-200 bg-yellow-50'
                    : extractionDone
                      ? 'border-green-200 bg-green-50'
                      : 'border-blue-100 bg-primary-50'
                }`}>
                  <div className="flex flex-col gap-4">
                    <div className="flex items-center justify-between">
                      <div className="flex-1">
                        {extractionInProgress || assetGenerating ? (
                          <>
                          <div className="flex items-center gap-2">
                            <Loader2 className="h-4 w-4 animate-spin text-yellow-600" />
                            <p className="text-sm font-semibold text-yellow-800">AI 自动处理中 · 资源提取</p>
                            <span className="rounded-full bg-yellow-100 px-2 py-0.5 text-[10px] font-medium text-yellow-700">步骤 2/3</span>
                          </div>
                          <p className="mt-1 text-xs text-yellow-700">
                            正在从剧本文本中识别并提取全部角色、场景、道具资源，提取完成后将自动开始分镜序列格式化（步骤 3）
                          </p>
                        </>
                      ) : extractionDone ? (
                        <>
                          <div className="flex items-center gap-2">
                            <CheckCircle2 className="h-4 w-4 text-green-600" />
                            <p className="text-sm font-medium text-green-800">资源提取完成</p>
                          </div>
                          <p className="mt-1 text-xs text-green-600">
                            共提取 {extractTotal} 项资源，可前往「资源」标签页查看与生成图像
                          </p>
                        </>
                      ) : (
                        <>
                          <p className="text-sm font-medium text-primary-800">分集完成，下一步：提取资源</p>
                          <p className="text-xs text-primary-600">手动提取前会先清除旧资源，再重新识别角色、场景、道具等内容</p>
                        </>
                        )}
                      </div>
                      <Button
                        size="sm"
                        onClick={() => handleStartAssetExtraction()}
                        disabled={assetGenerating}
                        className="ml-4 shrink-0"
                        title={extractionDone ? '清除旧资源后重新提取角色、场景、道具等资源' : extractionInProgress ? '重置提取状态并重新开始' : '手动提取角色、场景、道具等资源'}
                      >
                        {assetGenerating ? (
                          <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
                        ) : (
                          <Sparkles className="mr-1.5 h-3.5 w-3.5" />
                        )}
                        {extractionDone ? '清除后重新提取' : extractionInProgress ? '重置并重新提取' : '手动开始提取'}
                      </Button>
                    </div>


                  </div>
                </div>
              )}

              {/* Keyword Library display */}
              {project.keyword_library && (project.keyword_library.characters?.length > 0 || project.keyword_library.locations?.length > 0) && (
                <div className="mb-4 rounded-lg border border-purple-100 bg-purple-50 px-4 py-3">
                  <p className="mb-2 text-xs font-semibold text-purple-700">📚 关键词库</p>
                  <div className="space-y-1.5">
                    {project.keyword_library.characters?.length > 0 && (
                      <div className="flex flex-wrap gap-1">
                        <span className="text-[11px] text-purple-500 shrink-0">人物：</span>
                        {project.keyword_library.characters.slice(0, 20).map(k => (
                          <span key={k} className="rounded bg-purple-100 px-1.5 py-0.5 text-[10px] text-purple-700">{k}</span>
                        ))}
                      </div>
                    )}
                    {project.keyword_library.locations?.length > 0 && (
                      <div className="flex flex-wrap gap-1">
                        <span className="text-[11px] text-purple-500 shrink-0">地点：</span>
                        {project.keyword_library.locations.slice(0, 20).map(k => (
                          <span key={k} className="rounded bg-primary-100 px-1.5 py-0.5 text-[10px] text-primary-700">{k}</span>
                        ))}
                      </div>
                    )}
                    {project.keyword_library.events?.length > 0 && (
                      <div className="flex flex-wrap gap-1">
                        <span className="text-[11px] text-purple-500 shrink-0">事件：</span>
                        {project.keyword_library.events.slice(0, 15).map(k => (
                          <span key={k} className="rounded bg-orange-100 px-1.5 py-0.5 text-[10px] text-orange-700">{k}</span>
                        ))}
                      </div>
                    )}
                  </div>
                </div>
              )}

              <div className="grid grid-cols-1 gap-3 md:grid-cols-2 lg:grid-cols-3">
                {episodes.map((ep) => (
                  <div key={ep.id} className="rounded-lg border p-4 transition-colors hover:bg-surface-50">
                    <div className="mb-2 flex items-center justify-between">
                      <span className="text-sm font-semibold">第 {ep.episode_number} 集</span>
                      <div className="flex items-center gap-2">
                        <StatusBadge status={ep.status} />
                        {ep.optimize_status === 'optimizing' && (
                          <span className="inline-flex items-center gap-1 rounded-full bg-amber-100 px-2 py-0.5 text-[10px] text-amber-700">
                            <Loader2 className="h-2.5 w-2.5 animate-spin" />格式化中
                          </span>
                        )}
                        {ep.optimize_status === 'done' && (
                          <span className="inline-flex items-center gap-1 rounded-full bg-green-100 px-2 py-0.5 text-[10px] text-green-700">
                            <CheckCircle2 className="h-2.5 w-2.5" />已格式化
                          </span>
                        )}
                        {(ep.script_excerpt || ep.summary) && (
                          <Button
                            size="sm"
                            variant="ghost"
                            className="h-6 px-2 text-xs text-primary-600 hover:text-primary-800"
                            onClick={() => setSelectedEpisode(ep)}
                            title="查看本集详情"
                          >
                            <Eye className="mr-1 h-3 w-3" />
                            查看详情
                          </Button>
                        )}
                      </div>
                    </div>
                    <p className="mb-1 text-sm font-medium text-surface-800">{ep.title}</p>
                    <p className="mb-2 line-clamp-2 text-xs text-surface-500">{ep.summary}</p>
                    {ep.keywords?.length > 0 && (
                      <div className="mb-2 flex flex-wrap gap-1">
                        {ep.keywords.slice(0, 6).map(k => (
                          <span key={k} className="rounded bg-surface-100 px-1.5 py-0.5 text-[10px] text-surface-600">{k}</span>
                        ))}
                      </div>
                    )}
                    <div className="flex items-center justify-between text-xs text-surface-400">
                      <div className="flex gap-3">
                        <span>{ep.word_count} 字</span>
                        <span>~{formatDuration(ep.estimated_duration)}</span>
                      </div>
                      <div className="flex gap-1">
                        <Button
                          size="sm"
                          variant="ghost"
                          className="h-6 px-2 text-xs text-orange-500 hover:text-orange-700"
                          onClick={() => handleExtractEpisodeAssets(ep.id, ep.episode_number)}
                          title="从本集剧本中提取角色、场景等资源"
                        >
                          <Sparkles className="mr-1 h-3 w-3" />
                          提取资源
                        </Button>
                        <Button
                          size="sm"
                          variant="ghost"
                          className="h-6 px-2 text-xs text-blue-500 hover:text-blue-700"
                          onClick={() => handleGenerateEpisodeAssetsFromScript(ep.id, ep.episode_number)}
                          disabled={generatingEpisodeAssets === ep.id}
                          title="生成本集所有待处理资源的图片"
                        >
                          {generatingEpisodeAssets === ep.id ? (
                            <Loader2 className="mr-1 h-3 w-3 animate-spin" />
                          ) : (
                            <Image className="mr-1 h-3 w-3" />
                          )}
                          按集生成
                        </Button>
                      </div>
                    </div>
                    {/* Script optimize + review mini-bar */}
                    <div className="mt-2 flex items-center gap-1 border-t border-surface-100 pt-2">
                      <Button
                        size="sm"
                        variant="ghost"
                        className="h-6 px-2 text-xs text-purple-600 hover:bg-purple-50 hover:text-purple-800"
                        onClick={() => { handleAutoOptimizeReview(ep); setSelectedEpisode(ep) }}
                        disabled={autoOptimizingEpisode === ep.id || optimizingEpisode === ep.id || reviewingEpisode === ep.id}
                        title="AI 自动完成：转剧本格式 → 审查 → 弥补不足"
                      >
                        {autoOptimizingEpisode === ep.id ? <Loader2 className="mr-1 h-3 w-3 animate-spin" /> : <Sparkles className="mr-1 h-3 w-3" />}
                        AI 一键优化
                      </Button>
                      <Button
                        size="sm"
                        variant="ghost"
                        className="h-6 px-2 text-xs text-amber-600 hover:bg-amber-50 hover:text-amber-800"
                        onClick={() => handleOptimizeEpisode(ep)}
                        disabled={optimizingEpisode === ep.id || autoOptimizingEpisode === ep.id}
                        title="将本集小说原文转化为标准剧本格式"
                      >
                        {optimizingEpisode === ep.id ? <Loader2 className="mr-1 h-3 w-3 animate-spin" /> : null}
                        转剧本格式
                        {ep.optimize_status === 'done' && <span className="ml-1 rounded bg-amber-100 px-1 text-[9px] text-amber-700">✓</span>}
                      </Button>
                      {ep.optimize_status === 'done' && ep.optimized_text && (
                        <Button
                          size="sm"
                          variant="ghost"
                          className="h-6 px-2 text-xs text-amber-500 hover:bg-amber-50"
                          onClick={() => handleApplyOptimizedText(ep)}
                          disabled={applyingOptimized === ep.id}
                          title="将优化后的剧本格式应用为正式内容"
                        >
                          {applyingOptimized === ep.id ? <Loader2 className="mr-1 h-3 w-3 animate-spin" /> : null}
                          确认应用
                        </Button>
                      )}
                      <Button
                        size="sm"
                        variant="ghost"
                        className="h-6 px-2 text-xs text-green-600 hover:bg-green-50 hover:text-green-800"
                        onClick={() => { handleReviewEpisode(ep); setSelectedEpisode(ep) }}
                        disabled={reviewingEpisode === ep.id || autoOptimizingEpisode === ep.id}
                        title="AI 审查本集剧本质量与一致性"
                      >
                        {reviewingEpisode === ep.id ? <Loader2 className="mr-1 h-3 w-3 animate-spin" /> : null}
                        AI 审查
                        {ep.review_status === 'done' && <span className="ml-1 rounded bg-green-100 px-1 text-[9px] text-green-700">✓</span>}
                      </Button>
                    </div>
                  </div>
                ))}
              </div>


            </>
          )}
        </CardContent>
      </Card>

      {/* Production skills annotation panel — 影视部门标注技能 */}
      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-base flex items-center gap-2">
            <span>🎬</span> 影视部门标注技能
          </CardTitle>
        </CardHeader>
        <CardContent>
          <ProductionSkillsPanel projectId={projectId} />
        </CardContent>
      </Card>

      {/* Regenerate dialog with keyword input */}
      <Dialog open={showRegenerateDialog} onOpenChange={setShowRegenerateDialog}>
        <DialogContent className="max-w-lg">
          <DialogHeader>
            <DialogTitle>重新自动分集</DialogTitle>
          </DialogHeader>
          <div className="space-y-3 pt-2">
            <div className="rounded-md bg-amber-50 border border-amber-200 p-3">
              <p className="text-xs text-amber-800 font-medium">⚠️ 重新生成将清除以下已有数据：</p>
              <ul className="text-xs text-amber-700 mt-1 list-disc list-inside space-y-0.5">
                <li>所有剧本分集</li>
                <li>所有分镜片段</li>
                <li>所有已提取的资源（包括提取中的）</li>
                <li>所有已生成的视频</li>
              </ul>
              <p className="text-xs text-amber-600 mt-1">锁定的资源不受影响。</p>
            </div>
            <p className="text-xs text-surface-500">
              输入关键词可帮助 AI 更精准地拆分和理解内容。留空则由 AI 自动提取。
            </p>
            <div className="rounded-md border border-primary-100 bg-primary-50/60 p-3">
              <div className="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
                <div>
                  <p className="text-xs font-medium text-surface-800">本次分集使用模型</p>
                  <p className="mt-1 text-xs text-surface-500">
                    这里展示你手动选择的分集模型与目标分集数，确认后才会开始本次自动分集。
                  </p>
                </div>
                {selectedSplitModel ? (
                  <div className="flex flex-wrap items-center gap-2">
                    <Badge variant="secondary">{selectedSplitModel.name}</Badge>
                    {selectedSplitModelAvailability ? (
                      <Badge className={selectedSplitModelAvailability.color}>{selectedSplitModelAvailability.label}</Badge>
                    ) : null}
                  </div>
                ) : (
                  <Badge variant="outline">尚未选择分集模型</Badge>
                )}
              </div>
              <div className="mt-2 flex flex-wrap items-center gap-2">
                <Badge variant="outline">目标分集数：{hasValidTargetEpisodes ? parsedTargetEpisodes : '未填写'}</Badge>
              </div>
              {selectedSplitModel ? (
                <>
                  <div className="mt-2 flex flex-wrap gap-1">
                    {splitModelCapabilities.map((label) => (
                      <Badge key={label} variant="outline" className="text-[11px] font-normal">
                        {label}
                      </Badge>
                    ))}
                  </div>
                  <p className="mt-2 text-xs leading-5 text-surface-500">
                    {selectedSplitModelRemark}
                  </p>
                </>
              ) : null}
            </div>
            <div className="space-y-2">
              <Label className="text-xs font-medium text-red-700">✂️ 分集关键字（每行一个，在原文中出现的位置将作为分集边界）</Label>
              <Textarea
                placeholder={"第一回 灵根育孕源流出 心性修持大道生\n第二回 悟彻菩提真妙理 断魔归本合元神\n第三回 四海千山皆拱伏 九幽十类尽除名"}
                value={kwSplitKeywords}
                onChange={(e: React.ChangeEvent<HTMLTextAreaElement>) => setKwSplitKeywords(e.target.value)}
                rows={3}
                className="text-xs"
              />
            </div>
            <div className="border-t pt-3">
              <p className="text-xs text-surface-400 mb-2">以下关键词用于辅助 AI 理解内容（可选）：</p>
            </div>
            <div className="space-y-2">
              <Label className="text-xs font-medium text-purple-700">👤 人物</Label>
              <Input
                placeholder="孙悟空、唐僧、猪八戒、沙悟净"
                value={kwCharacters}
                onChange={(e: React.ChangeEvent<HTMLInputElement>) => setKwCharacters(e.target.value)}
              />
            </div>
            <div className="space-y-2">
              <Label className="text-xs font-medium text-primary-700">📍 地点</Label>
              <Input
                placeholder="花果山、东海龙宫、五行山"
                value={kwLocations}
                onChange={(e: React.ChangeEvent<HTMLInputElement>) => setKwLocations(e.target.value)}
              />
            </div>
            <div className="space-y-2">
              <Label className="text-xs font-medium text-orange-700">⚡ 事件</Label>
              <Input
                placeholder="大闹天宫、三打白骨精、西天取经"
                value={kwEvents}
                onChange={(e: React.ChangeEvent<HTMLInputElement>) => setKwEvents(e.target.value)}
              />
            </div>
            <div className="space-y-2">
              <Label className="text-xs font-medium text-green-700">🔧 道具</Label>
              <Input
                placeholder="金箍棒、紧箍咒、芭蕉扇"
                value={kwProps}
                onChange={(e: React.ChangeEvent<HTMLInputElement>) => setKwProps(e.target.value)}
              />
            </div>
            <div className="rounded-md border border-surface-200 bg-surface-50 p-3">
              <div className="flex items-center justify-between gap-3">
                <div>
                  <p className="text-sm font-medium text-surface-800">分集完成后延后开始分镜</p>
                  <p className="mt-1 text-xs text-surface-500">
                    资源图必须全部完成后才能开始分镜图像生成；开启后也不会跳过这一步校验。
                  </p>
                </div>
                <Switch checked={autoStoryboardAfterSplit} onCheckedChange={setAutoStoryboardAfterSplit} />
              </div>
            </div>
            <div className="flex justify-end gap-2 pt-3">
              <Button variant="outline" onClick={() => setShowRegenerateDialog(false)}>取消</Button>
              <Button onClick={handleGenerateEpisodes}>
                <RefreshCw className="mr-1.5 h-3.5 w-3.5" />
                开始自动分集
              </Button>
            </div>
          </div>
        </DialogContent>
      </Dialog>

      <Dialog open={showCreateEpisodeDialog} onOpenChange={setShowCreateEpisodeDialog}>
        <DialogContent className="max-w-lg">
          <DialogHeader>
            <DialogTitle>手动创建分集</DialogTitle>
          </DialogHeader>
          <div className="space-y-4 pt-2">
            <div className="rounded-md border border-primary-100 bg-primary-50/60 p-3">
              <p className="text-sm font-medium text-surface-800">适合补充插叙、番外、加更，或在没有剧本拆分结果时先手动搭建分集结构。</p>
              <p className="mt-1 text-xs text-surface-500">
                建议序号：第 {nextManualEpisodeNumber} 集。创建后可直接用于资源提取、从分集创建分镜，以及后续按集生成视频。
              </p>
            </div>
            <div className="space-y-2">
              <Label>分集序号</Label>
              <Input
                type="number"
                min={1}
                step={1}
                value={manualEpisodeNumber}
                onChange={(event) => setManualEpisodeNumber(event.target.value.replace(/[^\d]/g, ''))}
                placeholder="例如 1"
              />
              {manualEpisodeNumberTaken ? (
                <p className="text-xs text-red-500">第 {parsedManualEpisodeNumber} 集已存在，请换一个未占用的分集序号。</p>
              ) : null}
            </div>
            <div className="space-y-2">
              <Label>分集标题</Label>
              <Input
                value={manualEpisodeTitle}
                onChange={(event) => setManualEpisodeTitle(event.target.value)}
                placeholder="例如：初入花果山"
              />
            </div>
            <div className="space-y-2">
              <Label>分集摘要</Label>
              <Textarea
                value={manualEpisodeSummary}
                onChange={(event) => setManualEpisodeSummary(event.target.value)}
                rows={3}
                placeholder="简要描述这一集的主要剧情、人物和场景，方便后续提取资源与生成分镜。"
              />
            </div>
            <div className="space-y-2">
              <Label>
                分集正文内容
                <span className="ml-1 text-xs font-normal text-surface-400">（可选）分镜生成时会优先使用此内容</span>
              </Label>
              <Textarea
                value={manualEpisodeContent}
                onChange={(event) => setManualEpisodeContent(event.target.value)}
                rows={5}
                placeholder="粘贴或输入本集的具体剧本/小说原文，AI 将基于此内容拆分分镜。不填则使用摘要。"
              />
            </div>
            <div className="flex justify-end gap-2 pt-2">
              <Button variant="outline" onClick={() => setShowCreateEpisodeDialog(false)} disabled={creatingEpisode}>
                取消
              </Button>
              <Button onClick={handleCreateEpisode} disabled={creatingEpisode}>
                {creatingEpisode ? <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" /> : <Plus className="mr-1.5 h-3.5 w-3.5" />}
                创建分集
              </Button>
            </div>
          </div>
        </DialogContent>
      </Dialog>

      <Dialog open={showScriptPreviewDialog} onOpenChange={setShowScriptPreviewDialog}>
        <DialogContent className="max-w-5xl">
          <DialogHeader>
            <DialogTitle>剧本全文</DialogTitle>
          </DialogHeader>
          <div className="space-y-3">
            <div className="flex flex-wrap items-center justify-between gap-2 text-xs text-surface-500">
              <span>{project.script_file_url.split('/').pop() || '剧本文件'}</span>
              <span>{scriptText.length} 字</span>
            </div>
            <div className="max-h-[70vh] overflow-auto rounded-lg border bg-surface-50 p-4">
              <pre className="whitespace-pre-wrap break-words text-sm leading-6 text-surface-700">
                {scriptText}
              </pre>
            </div>
          </div>
        </DialogContent>
      </Dialog>

      {/* Episode content dialog with storyboard list */}
      <Dialog open={!!selectedEpisode} onOpenChange={(open) => { if (!open) { setSelectedEpisode(null); setEditingEpisode(false) } }}>
        <DialogContent className="max-w-4xl">
          <DialogHeader>
            <div className="flex items-center justify-between gap-3 pr-8">
              <DialogTitle>
                第 {selectedEpisode?.episode_number} 集 · {editingEpisode ? (editEpisodeTitle || selectedEpisode?.title) : selectedEpisode?.title}
              </DialogTitle>
              {!editingEpisode ? (
                <div className="flex gap-2 shrink-0">
                  <Button size="sm" variant="outline" onClick={handlePolishEpisode} disabled={polishingEpisode}>
                    {polishingEpisode ? <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" /> : <Sparkles className="mr-1.5 h-3.5 w-3.5" />}
                    AI 润色
                  </Button>
                  <Button size="sm" variant="outline" className="shrink-0" onClick={() => selectedEpisode && handleOpenEditEpisode(selectedEpisode)}>
                    <Pencil className="mr-1.5 h-3.5 w-3.5" />
                    编辑
                  </Button>
                </div>
              ) : (
                <div className="flex gap-2 shrink-0">
                  <Button type="button" size="sm" variant="outline" onClick={() => setEditingEpisode(false)} disabled={savingEpisodeEdit}>取消</Button>
                  <Button type="button" size="sm" onClick={handleSaveEpisodeEdit} disabled={savingEpisodeEdit}>
                    {savingEpisodeEdit ? <Loader2 className="mr-1 h-3 w-3 animate-spin" /> : null}保存
                  </Button>
                 </div>
              )}
            </div>
          </DialogHeader>
          <div className="grid grid-cols-1 gap-6 md:grid-cols-2">
            {/* Left: episode content */}
            <div className="space-y-3">
              {editingEpisode ? (
                <>
                  <div className="space-y-1.5">
                    <Label className="text-xs">标题</Label>
                    <Input value={editEpisodeTitle} onChange={(e) => setEditEpisodeTitle(e.target.value)} placeholder="分集标题" />
                  </div>
                  <div className="space-y-1.5">
                    <Label className="text-xs">剧情摘要</Label>
                    <Textarea value={editEpisodeSummary} onChange={(e) => setEditEpisodeSummary(e.target.value)} rows={3} placeholder="简要描述这一集的主要剧情" />
                  </div>
                  <div className="space-y-1.5">
                    <Label className="text-xs">
                      正文内容
                      <span className="ml-1 font-normal text-surface-400">（分镜生成时优先使用）</span>
                    </Label>
                    <Textarea value={editEpisodeContent} onChange={(e) => setEditEpisodeContent(e.target.value)} rows={8} placeholder="粘贴或输入本集的具体剧本/小说原文，AI 将基于此内容拆分分镜。不填则使用摘要。" className="font-mono text-xs" />
                  </div>
                </>
              ) : (
                <>
                  {selectedEpisode?.summary && (
                    <div className="rounded-md bg-surface-50 px-4 py-3">
                      <p className="mb-1 text-xs font-semibold uppercase tracking-wide text-surface-400">剧情摘要</p>
                      <p className="text-sm text-surface-700">{selectedEpisode.summary}</p>
                    </div>
                  )}
                  {selectedEpisode?.script_excerpt ? (
                    <div className="rounded-md bg-surface-50 px-4 py-3">
                      <p className="mb-1 text-xs font-semibold uppercase tracking-wide text-surface-400">本集原文内容</p>
                      <div className="max-h-96 overflow-y-auto">
                        <p className="whitespace-pre-wrap text-sm leading-relaxed text-surface-700">
                          {selectedEpisode.script_excerpt}
                        </p>
                      </div>
                    </div>
                  ) : (
                    <div className="rounded-md border border-dashed border-surface-200 px-4 py-3 text-center">
                      <p className="text-xs text-surface-400">暂无正文内容，点击「编辑」可添加剧本原文，提升分镜生成质量。</p>
                    </div>
                  )}
                  <div className="flex gap-4 text-xs text-surface-400">
                    <span>{selectedEpisode?.word_count} 字</span>
                    <span>~{formatDuration(selectedEpisode?.estimated_duration ?? 0)}</span>
                  </div>

                  {/* Optimize section */}
                  <div className="rounded-md border border-amber-200 bg-amber-50 px-3 py-3">
                    <div className="mb-2 flex items-center justify-between">
                      <p className="text-xs font-semibold text-amber-800">剧本格式优化</p>
                      <div className="flex items-center gap-2">
                        {selectedEpisode?.optimize_status === 'done' && (
                          <span className="rounded bg-amber-200 px-1.5 py-0.5 text-[10px] text-amber-800">已优化</span>
                        )}
                        {selectedEpisode?.optimize_status === 'optimizing' && (
                          <span className="rounded bg-amber-100 px-1.5 py-0.5 text-[10px] text-amber-600">优化中...</span>
                        )}
                        <Button
                          size="sm"
                          variant="outline"
                          className="h-6 border-purple-300 px-2 text-xs text-purple-700 hover:bg-purple-100"
                          onClick={() => selectedEpisode && handleAutoOptimizeReview(selectedEpisode)}
                          disabled={autoOptimizingEpisode === selectedEpisode?.id || optimizingEpisode === selectedEpisode?.id}
                          title="AI 自动完成：转剧本格式 → 审查 → 弥补不足"
                        >
                          {autoOptimizingEpisode === selectedEpisode?.id ? <Loader2 className="mr-1 h-3 w-3 animate-spin" /> : <Sparkles className="mr-1 h-3 w-3" />}
                          AI 一键优化
                        </Button>
                        <Button
                          size="sm"
                          variant="outline"
                          className="h-6 border-amber-300 px-2 text-xs text-amber-700 hover:bg-amber-100"
                          onClick={() => selectedEpisode && handleOptimizeEpisode(selectedEpisode)}
                          disabled={optimizingEpisode === selectedEpisode?.id || autoOptimizingEpisode === selectedEpisode?.id}
                        >
                          {optimizingEpisode === selectedEpisode?.id ? <Loader2 className="mr-1 h-3 w-3 animate-spin" /> : null}
                          转剧本格式
                        </Button>
                      </div>
                    </div>
                    {selectedEpisode?.optimize_status === 'done' && selectedEpisode.optimized_text ? (
                      <>
                        <div className="grid grid-cols-2 gap-2">
                          <div>
                            <p className="mb-1 text-[10px] font-medium text-surface-500">原文</p>
                            <div className="max-h-40 overflow-y-auto rounded bg-white px-2 py-2">
                              <p className="whitespace-pre-wrap font-mono text-[11px] leading-relaxed text-surface-600">
                                {selectedEpisode.original_excerpt || selectedEpisode.script_excerpt}
                              </p>
                            </div>
                          </div>
                          <div>
                            <p className="mb-1 text-[10px] font-medium text-amber-700">优化后</p>
                            <div className="max-h-40 overflow-y-auto rounded bg-amber-50/60 px-2 py-2 border border-amber-200">
                              <p className="whitespace-pre-wrap font-mono text-[11px] leading-relaxed text-amber-900">
                                {selectedEpisode.optimized_text}
                              </p>
                            </div>
                          </div>
                        </div>
                        <div className="mt-2 flex justify-end">
                          <Button
                            size="sm"
                            className="h-7 bg-amber-500 px-3 text-xs text-white hover:bg-amber-600"
                            onClick={() => selectedEpisode && handleApplyOptimizedText(selectedEpisode)}
                            disabled={applyingOptimized === selectedEpisode?.id}
                          >
                            {applyingOptimized === selectedEpisode?.id ? <Loader2 className="mr-1 h-3 w-3 animate-spin" /> : null}
                            确认应用优化内容
                          </Button>
                        </div>
                      </>
                    ) : (
                      <p className="text-[11px] text-amber-600">点击"转剧本格式"将小说原文转化为标准剧本格式（场景标题、动作描述、台词等）。</p>
                    )}
                  </div>

                  {/* Review section */}
                  <div className="rounded-md border border-green-200 bg-green-50 px-3 py-3">
                    <div className="mb-2 flex items-center justify-between">
                      <p className="text-xs font-semibold text-green-800">AI 质量审查</p>
                      <div className="flex items-center gap-2">
                        {selectedEpisode?.review_status === 'done' && (
                          <span className="rounded bg-green-200 px-1.5 py-0.5 text-[10px] text-green-800">已审查</span>
                        )}
                        {selectedEpisode?.review_status === 'reviewing' && (
                          <span className="rounded bg-green-100 px-1.5 py-0.5 text-[10px] text-green-600">审查中...</span>
                        )}
                        <Button
                          size="sm"
                          variant="outline"
                          className="h-6 border-green-300 px-2 text-xs text-green-700 hover:bg-green-100"
                          onClick={() => selectedEpisode && handleReviewEpisode(selectedEpisode)}
                          disabled={reviewingEpisode === selectedEpisode?.id}
                        >
                          {reviewingEpisode === selectedEpisode?.id ? <Loader2 className="mr-1 h-3 w-3 animate-spin" /> : null}
                          AI 审查
                        </Button>
                      </div>
                    </div>
                    {selectedEpisode?.review_status === 'done' && selectedEpisode.review_result ? (
                      <div className="space-y-2">
                        {/* Score bars */}
                        {(() => {
                          const s = selectedEpisode.review_result.score
                          const dims = [
                            { label: '完整度', val: s.completeness },
                            { label: '连贯性', val: s.integrity },
                            { label: '一致性', val: s.consistency },
                            { label: '衔接性', val: s.transitions },
                            { label: '台词质量', val: s.dialog_quality },
                          ]
                          return (
                            <div className="space-y-1">
                              {dims.map(d => (
                                <div key={d.label} className="flex items-center gap-2">
                                  <span className="w-14 shrink-0 text-[10px] text-surface-600">{d.label}</span>
                                  <div className="h-1.5 flex-1 rounded-full bg-surface-200">
                                    <div
                                      className={`h-1.5 rounded-full ${d.val >= 80 ? 'bg-green-400' : d.val >= 60 ? 'bg-yellow-400' : 'bg-red-400'}`}
                                      style={{ width: `${d.val}%` }}
                                    />
                                  </div>
                                  <span className="w-7 shrink-0 text-right text-[10px] font-medium text-surface-700">{d.val}</span>
                                </div>
                              ))}
                            </div>
                          )
                        })()}
                        {selectedEpisode.review_result.overall && (
                          <p className="text-[11px] text-surface-700">{selectedEpisode.review_result.overall}</p>
                        )}
                        {/* Issues */}
                        {selectedEpisode.review_result.issues?.length > 0 && (
                          <div className="space-y-1">
                            {selectedEpisode.review_result.issues.map((issue, i) => (
                              <div key={i} className={`rounded px-2 py-1.5 text-[11px] ${issue.severity === 'critical' ? 'bg-red-50 text-red-800' : issue.severity === 'warning' ? 'bg-yellow-50 text-yellow-800' : 'bg-blue-50 text-blue-800'}`}>
                                <span className="font-medium">[{issue.type}]</span> {issue.description}
                                {issue.suggestion && <p className="mt-0.5 opacity-80">→ {issue.suggestion}</p>}
                              </div>
                            ))}
                          </div>
                        )}
                      </div>
                    ) : (
                      <p className="text-[11px] text-green-600">点击"AI 审查"分析本集剧本的完整度、一致性、台词质量及情节衔接。</p>
                    )}
                  </div>
                </>
              )}
            </div>
            {/* Right: storyboard list for this episode */}
            <div>
              <p className="mb-2 text-xs font-semibold uppercase tracking-wide text-surface-400">分镜列表</p>
              <EpisodeStoryboardList projectId={projectId} episodeId={selectedEpisode?.id ?? 0} />
            </div>
          </div>
        </DialogContent>
      </Dialog>

      {/* Episode delete confirmation */}
      <AlertDialog open={!!episodeDeleteTarget} onOpenChange={(open) => { if (!open) setEpisodeDeleteTarget(null) }}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>确认删除分集</AlertDialogTitle>
            <AlertDialogDescription>
              确定要删除第 {episodeDeleteTarget?.episode_number} 集「{episodeDeleteTarget?.title}」吗？此操作不可恢复，该集的分镜等关联数据也将被删除。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={!!deletingEpisodeId}>取消</AlertDialogCancel>
            <AlertDialogAction
              className="bg-red-500 hover:bg-red-600"
              disabled={!!deletingEpisodeId}
              onClick={handleDeleteEpisode}
            >
              {deletingEpisodeId ? '删除中...' : '确认删除'}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
