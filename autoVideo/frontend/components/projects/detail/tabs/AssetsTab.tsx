'use client'

import React, { useState, useRef, useEffect, useMemo } from 'react'
import { useParams, useRouter } from 'next/navigation'
import useSWR, { mutate as globalMutate } from 'swr'
import {
  ArrowLeft,
  FileText,
  Image,
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
  User,
  Layers,
  Package,
  Images,
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
import { Tooltip, TooltipTrigger, TooltipContent } from '@/components/ui/tooltip'
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
import { EpisodeStoryboardList } from '@/components/projects/detail/EpisodeStoryboardList'

type TabKey = WorkflowStepKey

function buildAssetStoryboardUsageMap(storyboards: Storyboard[]): Map<number, Storyboard[]> {
  const map = new Map<number, Storyboard[]>()
  storyboards.forEach((storyboard) => {
    ;(storyboard.asset_ids ?? []).forEach((assetId) => {
      const current = map.get(assetId) ?? []
      current.push(storyboard)
      map.set(assetId, current)
    })
  })
  return map
}


export function AssetsTab({ projectId, project, episodeId, onExtractEpisodeAssets, isExtractingEpisodeAssets, generateTrigger, regenerateTrigger, hideActionBar }: { projectId: number; project: Project; episodeId?: number; onExtractEpisodeAssets?: () => void; isExtractingEpisodeAssets?: boolean; generateTrigger?: number; regenerateTrigger?: number; hideActionBar?: boolean }) {
  const { toast } = useToast()
  const sharedEpisode = useProjectEpisodeFilter()
  const [filter, setFilter] = useState<AssetType | 'all'>('all')
  const [statusFilter, setStatusFilter] = useState<AssetStatus | 'all'>('all')

  React.useEffect(() => {
    if (episodeId !== undefined) {
      setEpisodeFilter(episodeId)
    }
  }, [episodeId])

  const [episodeFilter, setEpisodeFilter] = useState<number | 'all'>(() =>
    sharedEpisode.value === 'all' ? 'all' : Number(sharedEpisode.value) || 'all'
  )
  useEffect(() => {
    sharedEpisode.setValue(episodeFilter === 'all' ? 'all' : String(episodeFilter))
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [episodeFilter])
  const [keyword, setKeyword] = useState('')
  const [selectedAsset, setSelectedAsset] = useState<Asset | null>(null)
  const [chatInput, setChatInput] = useState('')
  const [chatLoading, setChatLoading] = useState(false)
  const [selectingImageUrl, setSelectingImageUrl] = useState<string | null>(null)
  const [progressTick, setProgressTick] = useState(0)
  const uploadRef = useRef<HTMLInputElement>(null)
  const chatListRef = useRef<HTMLDivElement>(null)
  const chatBottomRef = useRef<HTMLDivElement>(null)
  const shouldStickChatToBottomRef = useRef(true)
  const [showCreateDialog, setShowCreateDialog] = useState(false)
  const [createForm, setCreateForm] = useState({ type: 'character' as AssetType, name: '', description: '' })
  const [createLoading, setCreateLoading] = useState(false)
  const [batchGeneratingTarget, setBatchGeneratingTarget] = useState<'all' | number | null>(null)
  const [pausingGeneration, setPausingGeneration] = useState(false)
  const [resumingGeneration, setResumingGeneration] = useState(false)
  const [selectedChatModelId, setSelectedChatModelId] = useState(project.text_model_id ? String(project.text_model_id) : '')
  const [page, setPage] = useState(1)
  const pageSize = 50
  const [imageModelAvailability, setImageModelAvailability] = useState<Record<string, boolean>>({})
  const [showBatchGenDialog, setShowBatchGenDialog] = useState(false)
  const [batchGenTarget, setBatchGenTarget] = useState<{
    kind: 'all' | 'episode' | 'asset' | 'failed'
    episodeId?: number
    assetIds?: number[]
    assetLabel?: string
    force: boolean
  }>({ kind: 'all', force: false })
  const [batchGenModels, setBatchGenModels] = useState<string[]>([])
  const [batchGenUserExtra, setBatchGenUserExtra] = useState('')
  const [batchGenTranslating, setBatchGenTranslating] = useState(false)
  const [batchGenRunning, setBatchGenRunning] = useState(false)
  const [batchGenForce, setBatchGenForce] = useState(false)

  React.useEffect(() => {
    assetAPI.modelStatus().then((res) => {
      const map: Record<string, boolean> = {}
      // interceptor returns res.data (HTTP body) directly — response is { models: [...] } not { data: { models: [...] } }
      const models: { key: string; available: boolean }[] = (res as any)?.models ?? (res as any)?.data?.models ?? []
      models.forEach((m) => { map[m.key] = m.available })
      setImageModelAvailability(map)
    }).catch((e) => { console.warn('[modelStatus] endpoint unavailable', e) })
  }, [])

  // Build query params for paginated fetch
  const queryParams = {
    page,
    page_size: pageSize,
    ...(filter !== 'all' ? { type: filter } : {}),
    ...(statusFilter !== 'all' ? { status: statusFilter } : {}),
    ...(episodeFilter !== 'all' ? { episode_id: episodeFilter } : {}),
    ...(keyword.trim() ? { keyword: keyword.trim() } : {}),
  }

  const { data: assetsData, isLoading, mutate: mutateAssets } = useSWR(
    ['assets', projectId, page, filter, statusFilter, episodeFilter, keyword],
    () => assetAPI.listPaginated(projectId, queryParams) as unknown as Promise<{ data: { items: Asset[]; total: number; page: number; page_size: number } }>,
    {
      refreshInterval: (data) => {
        const items = (data as { data?: { items?: Asset[] } })?.data?.items ?? []
        return items.some((a) => a.status === 'generating') ? 3000 : 0
      },
    }
  )
  const paginatedData = (assetsData as { data?: { items?: Asset[]; total?: number } })?.data
  const allAssets = paginatedData?.items ?? []
  const totalAssets = paginatedData?.total ?? 0
  const totalPages = Math.max(1, Math.ceil(totalAssets / pageSize))

  // Also fetch full stats (lightweight count query) for the stats bar
  const { data: allAssetsForStats } = useSWR(
    ['assets-stats', projectId],
    () => assetAPI.list(projectId) as unknown as Promise<{ data: Asset[] }>,
    { refreshInterval: (data) => {
        const items = (data as { data?: Asset[] } | undefined)?.data ?? []
        const active = items.some((a) => a.status === 'generating' || a.status === 'pending' || a.status === 'extracting')
        return active ? 10000 : 60000
      } }
  )
  const statsAssetsRaw = (allAssetsForStats as { data?: Asset[] })?.data ?? []
  const statsAssets = statsAssetsRaw.filter((asset) => asset.name !== '__extracting__' && asset.status !== 'extracting')
  const extractionInProgress = statsAssetsRaw.some((asset) => asset.status === 'extracting')

  const { data: episodesData } = useSWR(
    ['episodes-for-assets', projectId],
    () => projectAPI.listEpisodes(projectId) as unknown as Promise<{ data: Episode[] }>
  )
  const episodes = (episodesData as { data?: Episode[] })?.data ?? []
  const orderedEpisodes = [...episodes].sort((a, b) => a.episode_number - b.episode_number)
  const episodeNumberById = React.useMemo(() => {
    const map = new Map<number, number>()
    orderedEpisodes.forEach((episode) => {
      map.set(episode.id, episode.episode_number)
    })
    return map
  }, [orderedEpisodes])
  const { data: storyboardLinksData } = useSWR(
    ['asset-storyboard-links', projectId],
    () => storyboardAPI.listAll(projectId) as unknown as Promise<{ data: Storyboard[] }>,
    {
      refreshInterval: (data) => {
        const items = (data as { data?: Storyboard[] } | undefined)?.data ?? []
        return items.some((sb) => sb.status === 'pending' || sb.status === 'generating') ? 10000 : 0
      },
    }
  )
  const storyboardLinks = React.useMemo(
    () => ((storyboardLinksData as { data?: Storyboard[] })?.data ?? []).filter((sb) => (sb.asset_ids ?? []).length > 0),
    [storyboardLinksData]
  )
  const { data: imageModelsData } = useSWR(
    ['asset-image-models', projectId],
    () => modelAPI.list({ type: 'image', sort_by: 'priority' }) as unknown as Promise<{ data: Model[] }>
  )
  const { data: textModelsData } = useSWR(
    ['asset-text-models', projectId],
    () => modelAPI.list({ type: 'llm', sort_by: 'priority' }) as unknown as Promise<{ data: Model[] }>
  )
  const imageModels = (imageModelsData as { data?: Model[] })?.data ?? []
  const chatModels = ((textModelsData as { data?: Model[] })?.data ?? [])
    .filter((model) => model.is_active || model.id === project.text_model_id)
    .sort((left, right) => {
      if (left.is_default !== right.is_default) return left.is_default ? -1 : 1
      if (left.priority !== right.priority) return left.priority - right.priority
      return left.name.localeCompare(right.name, 'zh-CN')
    })
  const selectedProjectImageModel = imageModels.find((model) => model.id === project.image_model_id)
  const selectedProjectImageModelName = selectedProjectImageModel?.name
  const selectedProjectImageModelLabel = selectedProjectImageModel?.name ?? '系统默认图片模型'
  const selectedChatModel = chatModels.find((model) => String(model.id) === selectedChatModelId) ?? null
  const selectedChatModelName = selectedChatModel?.name
  const selectedChatModelLabel = selectedChatModel?.name ?? '系统默认文本模型'

  // Dynamically fetch voice list; fall back to static list if API unavailable
  const { data: voicesData } = useSWR(
    'voices',
    () => dubbingAPI.listVoices().then((r) => (r as { data?: { voices?: { key: string; label: string }[] } }).data?.voices ?? null),
    { revalidateOnFocus: false, revalidateOnReconnect: false }
  )
  const ASSET_VOICE_OPTIONS = [
    { value: '', label: '未绑定音色' },
    ...(voicesData ?? FALLBACK_VOICE_OPTIONS).map((v) => {
      const key = (v as { key?: string }).key ?? (v as { value?: string }).value ?? ''
      return { value: key, label: v.label }
    }),
  ]

  React.useEffect(() => {
    const currentExists = chatModels.some((model) => String(model.id) === selectedChatModelId)
    if (selectedChatModelId && currentExists) return
    const fallbackId = project.text_model_id
      ? String(project.text_model_id)
      : chatModels.length > 0
        ? String((chatModels.find((model) => model.is_default) ?? chatModels[0]).id)
        : ''
    if (fallbackId) setSelectedChatModelId(fallbackId)
  }, [chatModels, project.text_model_id, selectedChatModelId])

  // Only show episodes that have at least one associated asset
  const episodesWithAssets = React.useMemo(() => {
    const assetEpIds = new Set(statsAssets.flatMap((a) => a.episode_ids ?? []))
    return [...episodes]
      .filter((ep) => assetEpIds.has(ep.id))
      .sort((a, b) => a.episode_number - b.episode_number)
  }, [episodes, statsAssets])
  const currentEpisode = episodeFilter === 'all' ? null : orderedEpisodes.find((ep) => ep.id === episodeFilter) ?? null
  const scopedStoryboards = React.useMemo(
    () => episodeFilter === 'all' ? storyboardLinks : storyboardLinks.filter((sb) => sb.episode_id === episodeFilter),
    [episodeFilter, storyboardLinks]
  )
  const assetStoryboardUsageMap = React.useMemo(() => buildAssetStoryboardUsageMap(storyboardLinks), [storyboardLinks])
  const scopedAssetStoryboardUsageMap = React.useMemo(() => buildAssetStoryboardUsageMap(scopedStoryboards), [scopedStoryboards])
  const scopedStatsAssets = React.useMemo(
    () =>
      episodeFilter === 'all'
        ? statsAssets
        : statsAssets.filter((asset) => (asset.episode_ids ?? []).includes(episodeFilter)),
    [episodeFilter, statsAssets]
  )
  const statsTotal = scopedStatsAssets.length
  const usedAssetCount = scopedStatsAssets.filter((asset) => (scopedAssetStoryboardUsageMap.get(asset.id)?.length ?? 0) > 0).length
  const assetScopeLabel = currentEpisode ? `第 ${currentEpisode.episode_number} 集` : '全部资源'

  const assets = allAssets

  // Reset page when filter changes
  React.useEffect(() => { setPage(1) }, [filter, statusFilter, episodeFilter, keyword])

  // Keep detail panel in sync when assets auto-refresh
  React.useEffect(() => {
    if (selectedAsset) {
      const updated = allAssets.find((a) => a.id === selectedAsset.id)
      if (updated) setSelectedAsset(updated)
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [allAssets])

  React.useEffect(() => {
    shouldStickChatToBottomRef.current = true
  }, [selectedAsset?.id])

  React.useEffect(() => {
    if (selectedAsset?.status !== 'generating') return

    const timer = window.setInterval(() => {
      setProgressTick((value) => value + 1)
    }, 3000)

    return () => window.clearInterval(timer)
  }, [selectedAsset?.status])

  const counts = {
    character: scopedStatsAssets.filter((a) => a.type === 'character').length,
    scene: scopedStatsAssets.filter((a) => a.type === 'scene').length,
    prop: scopedStatsAssets.filter((a) => a.type === 'prop').length,
  }
  const completedCount = scopedStatsAssets.filter((a) => a.status === 'completed').length
  const generatingCount = scopedStatsAssets.filter((a) => a.status === 'generating').length
  const pendingCount = scopedStatsAssets.filter((a) => a.status === 'pending').length
  const pausedCount = scopedStatsAssets.filter((a) => a.status === 'paused').length
  const failedCount = scopedStatsAssets.filter((a) => a.status === 'failed' || a.status === 'qa_failed').length
  const nowMs = Date.now()
  const generatingAssetProgresses = scopedStatsAssets
    .filter((asset) => asset.status === 'generating')
    .map((asset) => getAssetGenerationProgress(asset))
    .filter((progress): progress is AssetGenerationProgress => Boolean(progress))
  const aggregateGeneratingProgress = generatingAssetProgresses.reduce((sum, progress) => sum + Math.max(0, Math.min(100, progress.percent)), 0)
  const completionRate = statsTotal > 0 ? Math.round((completedCount / statsTotal) * 100) : 0
  const imageProgressRate = statsTotal > 0
    ? Math.round((((completedCount * 100) + aggregateGeneratingProgress) / statsTotal) )
    : 0
  const isGenerating = generatingCount > 0
  const isGenerationPaused = pausedCount > 0 && generatingCount === 0
  const hasImageWorkRemaining = statsTotal > 0 && completedCount + failedCount < statsTotal
  const describedCount = scopedStatsAssets.filter((a) => (a.description ?? '').trim().length > 0).length
  const missingDescriptionCount = Math.max(statsTotal - describedCount, 0)
  const descriptionCompletionRate = statsTotal > 0 ? Math.round((describedCount / statsTotal) * 100) : 0
  const assetsGenerationStartedAt = getEarliestTimestamp(
    generatingAssetProgresses.map((progress) => progress.started_at ?? progress.updated_at)
  )
  const assetsGenerationTiming = isGenerating
    ? getTimingSummary(assetsGenerationStartedAt, imageProgressRate / 100, nowMs)
    : null

  const openBatchGenDialog = (target: {
    kind: 'all' | 'episode' | 'asset' | 'failed'
    episodeId?: number
    assetIds?: number[]
    assetLabel?: string
    force: boolean
  }) => {
    setBatchGenTarget(target)
    setBatchGenForce(target.force)
    setBatchGenModels(selectedProjectImageModelName ? [selectedProjectImageModelName] : [])
    setBatchGenUserExtra('')
    setShowBatchGenDialog(true)
  }

  const openSingleAssetGenerateDialog = (asset: Asset) => {
    const force = asset.status === 'failed' || asset.status === 'qa_failed' || asset.status === 'paused' || asset.status === 'completed'
    openBatchGenDialog({
      kind: 'asset',
      assetIds: [asset.id],
      assetLabel: asset.name,
      force,
    })
  }

  const handleGenerateAll = () => openBatchGenDialog(episodeId !== undefined ? { kind: 'episode', episodeId, force: false } : { kind: 'all', force: false })
  const handleRegenerateAll = () => openBatchGenDialog(episodeId !== undefined ? { kind: 'episode', episodeId, force: true } : { kind: 'all', force: true })

  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(() => { if ((generateTrigger ?? 0) > 0) handleGenerateAll() }, [generateTrigger])
  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(() => { if ((regenerateTrigger ?? 0) > 0) handleRegenerateAll() }, [regenerateTrigger])

  const executeBatchGenerate = async () => {
    setBatchGenRunning(true)
    try {
      const extra = batchGenUserExtra.trim()
      const selectedModels = MODEL_OPTIONS.filter((model) => batchGenModels.includes(model.key))
      const selectedModelKeys = selectedModels.map((model) => model.key)
      const promptSuffixes = Object.fromEntries(
        selectedModels.map((model) => [
          model.key,
          [model.defaultPrompt?.trim(), extra].filter(Boolean).join(' ').trim(),
        ]),
      )
      const singleModelName = selectedModels.length === 1 ? selectedModels[0]?.key : undefined
      const singlePromptSuffix = selectedModels.length === 1
        ? promptSuffixes[selectedModels[0].key] || undefined
        : (selectedModels.length === 0 && extra ? extra : undefined)
      const generateOpts = {
        modelName: singleModelName,
        modelNames: selectedModels.length > 1 ? selectedModelKeys : undefined,
        promptSuffix: singlePromptSuffix,
        promptSuffixes: selectedModels.length > 1 ? promptSuffixes : undefined,
        force: batchGenTarget.force || undefined,
      }

      let triggered = 0
      if (batchGenTarget.kind === 'asset') {
        const response = await assetAPI.generateBatch(projectId, batchGenTarget.assetIds ?? [], generateOpts) as unknown as { data?: { triggered?: number } }
        triggered = response?.data?.triggered ?? 0
      } else if (batchGenTarget.kind === 'failed') {
        const failedAssetIds = scopedStatsAssets.filter((asset) => asset.status === 'failed').map((asset) => asset.id)
        if (failedAssetIds.length === 0) {
          toast({ title: episodeFilter === 'all' ? '当前没有失败资源' : '当前集没有失败资源', variant: 'default' })
          setShowBatchGenDialog(false)
          return
        }
        const response = await assetAPI.generateBatch(projectId, failedAssetIds, generateOpts) as unknown as { data?: { triggered?: number } }
        triggered = response?.data?.triggered ?? 0
      } else {
        const episodeId = batchGenTarget.kind === 'episode' ? batchGenTarget.episodeId : undefined
        const response = await assetAPI.generateAll(
          projectId,
          episodeId,
          singleModelName,
          singlePromptSuffix,
          batchGenTarget.force || undefined,
          selectedModels.length > 1 ? { modelNames: selectedModelKeys, promptSuffixes } : undefined,
        ) as unknown as { data?: { triggered?: number } }
        triggered = response?.data?.triggered ?? 0
      }

      const selectedLabels = selectedModels.map((model) => model.label)
      const modelDescription = selectedLabels.length === 0
        ? undefined
        : selectedLabels.length === 1
          ? `使用模型：${selectedLabels[0]}`
          : `使用模型：${selectedLabels.join('、')}`
      const targetLabel = (() => {
        if (batchGenTarget.kind === 'asset') return `资源「${batchGenTarget.assetLabel ?? '未命名资源'}」`
        if (batchGenTarget.kind === 'episode') {
          const episodeId = batchGenTarget.episodeId
          return `第 ${orderedEpisodes.find((episode) => episode.id === episodeId)?.episode_number ?? episodeId} 集资源`
        }
        if (batchGenTarget.kind === 'failed') return episodeFilter === 'all' ? '失败资源' : '当前集失败资源'
        return batchGenTarget.force ? '全部重新资源' : '全部资源'
      })()
      toast({
        title: triggered > 0
          ? `${targetLabel}已启动（${triggered} 项）`
          : (batchGenTarget.kind === 'episode' ? '当前集没有待生成资源' : batchGenTarget.kind === 'asset' ? '当前资源暂无可生成任务' : '当前没有待生成资源'),
        description: triggered > 0 ? modelDescription : undefined,
        variant: triggered > 0 ? 'success' : 'default',
      })
      mutateAssets()
      setShowBatchGenDialog(false)
    } catch {
      toast({ title: '批量生成失败', variant: 'destructive' })
    } finally {
      setBatchGenRunning(false)
    }
  }

  const handleResetAsset = async (id: number) => {
    try {
      await assetAPI.reset(projectId, id)
      toast({ title: '已重置为待生成', variant: 'success' })
      mutateAssets()
    } catch {
      toast({ title: '重置失败', variant: 'destructive' })
    }
  }

  const handleRetryAllFailed = () => openBatchGenDialog({ kind: 'failed', force: false, episodeId: episodeFilter === 'all' ? undefined : episodeFilter })

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

  const [autoMatchingVoices, setAutoMatchingVoices] = useState(false)
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



  const handleDelete = async (id: number) => {
    try {
      await assetAPI.delete(projectId, id)
      toast({ title: '已删除', variant: 'success' })
      mutateAssets()
      if (selectedAsset?.id === id) setSelectedAsset(null)
    } catch {
      toast({ title: '删除失败', variant: 'destructive' })
    }
  }

  const handleToggleLock = async (asset: Asset) => {
    try {
      await assetAPI.update(projectId, asset.id, { is_locked: !asset.is_locked })
      toast({ title: asset.is_locked ? '已解锁' : '已锁定', variant: 'success' })
      mutateAssets()
    } catch {
      toast({ title: '操作失败', variant: 'destructive' })
    }
  }

  const handleUploadReplace = async (id: number, file: File) => {
    try {
      await assetAPI.upload(projectId, id, file)
      toast({ title: '替换成功', variant: 'success' })
      mutateAssets()
    } catch {
      toast({ title: '上传失败', variant: 'destructive' })
    }
  }

  const handleChat = async () => {
    if (!selectedAsset || !chatInput.trim()) return
    const outgoingMessage = chatInput.trim()
    const optimisticMessage: ChatMessage = {
      role: 'user',
      content: outgoingMessage,
      timestamp: new Date().toISOString(),
    }

    setSelectedAsset({
      ...selectedAsset,
      agent_history: [...(selectedAsset.agent_history ?? []), optimisticMessage],
    })
    setChatInput('')
    setChatLoading(true)
    try {
      const generationModel = selectedProjectImageModelName
      const imageModelOption = MODEL_OPTIONS.find((m) => m.key === generationModel || m.label === generationModel)
      const skillContext = imageModelOption?.defaultPrompt?.trim() || undefined

      const res = await assetAPI.chat(projectId, selectedAsset.id, outgoingMessage, selectedChatModelName, skillContext) as unknown as { data: Asset }
      const nextAsset = res.data ?? selectedAsset

      const generateWithChatUpdate = assetAPI.generateBatch(projectId, [nextAsset.id], {
        modelName: generationModel,
        promptSuffix: skillContext,
        force: nextAsset.status === 'failed' || nextAsset.status === 'qa_failed' || nextAsset.status === 'completed' || nextAsset.status === 'generating',
      })

      await generateWithChatUpdate

      setSelectedAsset({
        ...nextAsset,
        status: 'generating',
      })
      toast({ title: '已提交修改，正在生成新图', variant: 'success' })
      mutateAssets()
    } catch {
      toast({ title: '发送失败', variant: 'destructive' })
      setSelectedAsset(selectedAsset)
    }
    setChatLoading(false)
  }

  const handleSelectGeneratedImage = async (asset: Asset, imageUrl: string) => {
    const normalizedUrl = imageUrl.trim()
    if (!normalizedUrl) return

    const nextMetadata = {
      ...(asset.metadata ?? {}),
      selected_generated_image_url: normalizedUrl,
    }

    // If the asset is in a non-completed state (e.g. failed after a chat-modify),
    // selecting an existing generated image implicitly marks it as completed so
    // the storyboard tab is not blocked by this asset.
    const shouldResetStatus = asset.status !== 'completed'

    setSelectingImageUrl(normalizedUrl)
    try {
      const updatePayload: Partial<Asset> = { image_url: normalizedUrl, metadata: nextMetadata }
      if (shouldResetStatus) updatePayload.status = 'completed'

      const res = await assetAPI.update(projectId, asset.id, updatePayload) as unknown as { data: Asset }

      const updatedAsset = (res.data ?? {
        ...asset,
        ...updatePayload,
      }) as Asset

      setSelectedAsset(updatedAsset)
      mutateAssets()
      toast({ title: '已切换展示图片', variant: 'success' })
    } catch {
      toast({ title: '切换图片失败', variant: 'destructive' })
    } finally {
      setSelectingImageUrl(null)
    }
  }

  const handleDeleteGeneratedImage = async (asset: Asset, imageUrl: string) => {
    const currentImages: Array<{ url: string; created_at?: string }> =
      (asset.metadata?.generated_images as Array<{ url: string; created_at?: string }>) ?? []
    const nextImages = currentImages.filter((img) => img.url !== imageUrl)
    const currentSelected = asset.metadata?.selected_generated_image_url as string | undefined
    const nextSelected = currentSelected === imageUrl
      ? (nextImages[nextImages.length - 1]?.url ?? '')
      : (currentSelected ?? '')
    const nextMetadata = {
      ...(asset.metadata ?? {}),
      generated_images: nextImages,
      selected_generated_image_url: nextSelected,
    }
    try {
      const res = await assetAPI.update(projectId, asset.id, {
        metadata: nextMetadata,
        image_url: nextSelected,
      })
      if (res.data) {
        setSelectedAsset(res.data as Asset)
        mutateAssets()
      }
      toast({ title: '已删除该候选图片', variant: 'success' })
    } catch {
      toast({ title: '删除失败', variant: 'destructive' })
    }
  }

  const handleClearAllGeneratedImages = async (asset: Asset) => {
    const nextMetadata = {
      ...(asset.metadata ?? {}),
      generated_images: [],
      selected_generated_image_url: '',
    }
    try {
      const res = await assetAPI.update(projectId, asset.id, {
        metadata: nextMetadata,
        image_url: '',
        status: 'pending',
      })
      if (res.data) {
        setSelectedAsset(res.data as Asset)
        mutateAssets()
      }
      toast({ title: '已清除所有历史生成图片', variant: 'success' })
    } catch {
      toast({ title: '清除失败', variant: 'destructive' })
    }
  }

  const handleCreateAsset = async () => {
    if (!createForm.name.trim()) {
      toast({ title: '请输入资源名称', variant: 'destructive' })
      return
    }
    setCreateLoading(true)
    try {
      await assetAPI.create(projectId, {
        type: createForm.type,
        name: createForm.name.trim(),
        description: createForm.description.trim(),
        is_manual: true,
      })
      toast({ title: '资源创建成功', variant: 'success' })
      mutateAssets()
      setShowCreateDialog(false)
      setCreateForm({ type: 'character', name: '', description: '' })
    } catch {
      toast({ title: '创建失败', variant: 'destructive' })
    }
    setCreateLoading(false)
  }

  const TYPE_LABELS: Record<AssetType, string> = { character: '人物', scene: '场景', prop: '物品', image: '图片' }
  const selectedAssetPreviewUrl = selectedAsset ? getSelectedGeneratedImageUrl(selectedAsset) : ''
  const selectedAssetLinkedStoryboards = selectedAsset ? (assetStoryboardUsageMap.get(selectedAsset.id) ?? []) : []
  const selectedAssetScopedLinkedStoryboards = selectedAsset ? (scopedAssetStoryboardUsageMap.get(selectedAsset.id) ?? []) : []
  const formatStoryboardReference = React.useCallback((storyboard: Storyboard) => {
    const episodeNumber = storyboard.episode_id ? episodeNumberById.get(storyboard.episode_id) : undefined
    return episodeNumber ? `第${episodeNumber}集 · #${storyboard.sequence_number}` : `#${storyboard.sequence_number}`
  }, [episodeNumberById])
  const selectedAssetGeneratedImages = selectedAsset ? getAssetGeneratedImages(selectedAsset) : []
  const selectedAssetGenerationProgress = selectedAsset ? getAssetGenerationProgress(selectedAsset) : null
  const selectedAssetProgressHint = getGenerationStageHint(selectedAssetGenerationProgress, progressTick)
  const selectedAssetEtaLabel = getGenerationEtaLabel(selectedAssetGenerationProgress, nowMs)
  const selectedAssetElapsedLabel = getGenerationElapsedLabel(selectedAssetGenerationProgress, nowMs)
  const selectedAssetTimingLabel = [selectedAssetElapsedLabel, selectedAssetEtaLabel].filter(Boolean).join(' · ')

  React.useEffect(() => {
    if (!selectedAsset || !shouldStickChatToBottomRef.current) return
    chatBottomRef.current?.scrollIntoView({ block: 'end' })
  }, [
    selectedAsset?.id,
    selectedAsset?.status,
    selectedAsset?.agent_history?.length,
    selectedAssetGenerationProgress?.percent,
    selectedAssetGenerationProgress?.stage,
  ])

  const handleChatListScroll = () => {
    const el = chatListRef.current
    if (!el) return
    const distanceToBottom = el.scrollHeight - el.scrollTop - el.clientHeight
    shouldStickChatToBottomRef.current = distanceToBottom <= 96
  }

  const STATUS_FILTERS: { key: AssetStatus | 'all'; label: string; color?: string }[] = [
    { key: 'all', label: '全部状态' },
    { key: 'completed', label: '已完成' },
    { key: 'generating', label: '生成中', color: 'text-yellow-600' },
    { key: 'paused', label: '已暂停', color: 'text-amber-700' },
    { key: 'pending', label: '待生成', color: 'text-surface-400' },
    { key: 'failed', label: '失败', color: 'text-red-500' },
  ]

  // Build from API data + any generator keys discovered dynamically from image-service
  // that have no DB entry yet (so newly-configured generators appear without needing a migration).
  const MODEL_OPTIONS = useMemo(() => {
    const dbModels = imageModels.filter((m) => m.is_active)
    const dbKeys = new Set(dbModels.map((m) => m.model_key))
    // Also include inactive models that have a failure_reason so they display with a warning
    const brokenModels = imageModels.filter((m) => !m.is_active && m.failure_reason)
    const allDbKeys = new Set([...dbModels, ...brokenModels].map((m) => m.model_key))
    const synthetic: Model[] = Object.keys(imageModelAvailability)
      .filter((key) => !allDbKeys.has(key) && !dbKeys.has(key))
      .map((key) => ({
        id: 0, name: key, model_key: key, type: 'image' as const,
        provider: 'unknown', speed_rating: 'balanced' as const,
        capability_tags: ['text-to-image'], supports_consistency: false,
        consistency_method: 'none' as const, supported_ratios: ['1:1'],
        is_active: true, is_default: false,
        cost_per_unit: 0, unit: 'image', priority: 1,
        created_at: '', updated_at: '',
      }))
    return dedupeModels([...dbModels, ...synthetic, ...brokenModels]).map(buildImageModelOption)
  }, [imageModels, imageModelAvailability])

  const formatErrorMsg = (msg: string): string => {
    if (!msg) return '生成失败'
    if (msg.includes('moderation') || msg.includes('safety')) return '内容审核被拒（安全策略限制）'
    if (msg.includes('429') || msg.includes('RateLimit') || msg.includes('rate_limit')) return 'API 限流，请稍后重试'
    if (msg.includes('timed out') || msg.includes('timeout') || msg.includes('deadline exceeded')) return '请求超时，请重试'
    if (msg.includes('connection refused') || msg.includes('no such host')) return '服务不可用'
    if (msg.includes('insufficient_quota') || msg.includes('billing')) return 'API 额度不足'
    if (msg.length > 60) return msg.slice(0, 57) + '…'
    return msg
  }
  const TYPE_COLORS: Record<AssetType, string> = {
    character: 'bg-purple-100 text-purple-800',
    scene: 'bg-primary-100 text-primary-800',
    prop: 'bg-orange-100 text-orange-800',
    image: 'bg-sky-100 text-sky-800',
  }
  const TYPE_PLACEHOLDER_BG: Record<AssetType, string> = {
    character: 'bg-purple-50',
    scene: 'bg-primary-50',
    prop: 'bg-orange-50',
    image: 'bg-sky-50',
  }
  const TYPE_PLACEHOLDER_ICON_COLOR: Record<AssetType, string> = {
    character: 'text-purple-300',
    scene: 'text-primary-300',
    prop: 'text-orange-300',
    image: 'text-sky-300',
  }
  const TYPE_PLACEHOLDER_ICONS: Record<AssetType, React.ReactNode> = {
    character: <User className="h-10 w-10" />,
    scene: <Layers className="h-10 w-10" />,
    prop: <Package className="h-10 w-10" />,
    image: <Images className="h-10 w-10" />,
  }
  const TYPE_BADGE_ICONS: Record<AssetType, React.ReactNode> = {
    character: <User className="h-2.5 w-2.5" />,
    scene: <Layers className="h-2.5 w-2.5" />,
    prop: <Package className="h-2.5 w-2.5" />,
    image: <Images className="h-2.5 w-2.5" />,
  }

  if (isLoading) return <TabSkeleton />

  return (
    <div className="relative">
      {/* Action toolbar — hidden when parent (EpisodeWorkspace) renders the buttons in the sidebar */}
      {!hideActionBar ? (
        <div className="mb-3 flex flex-wrap items-center gap-2">
          {extractionInProgress && (
            <span className="inline-flex items-center gap-1 text-xs text-primary-600">
              <Loader2 className="h-3.5 w-3.5 animate-spin" /> 提取中
            </span>
          )}
          <div className="ml-auto flex flex-wrap gap-2">
            <Button size="sm" variant="outline" onClick={handleAutoMatchVoices} disabled={autoMatchingVoices} title="根据人物姓名自动为未绑定音色的人物资源分配音色">
              {autoMatchingVoices ? <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" /> : <Mic className="mr-1.5 h-3.5 w-3.5" />}
              自动匹配音色
            </Button>
            {failedCount > 0 && (
              <Button size="sm" variant="outline" onClick={handleRetryAllFailed} title="统一进入模型确认弹窗后重试当前范围内所有失败资源">
                <RefreshCw className="mr-1.5 h-3.5 w-3.5" />
                重试失败 ({failedCount})
              </Button>
            )}
            {pausedCount > 0 ? (
              <Button size="sm" variant="outline" onClick={handleResumeGeneration} disabled={resumingGeneration} title="继续项目下所有已暂停的资源图生成">
                {resumingGeneration ? <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" /> : <Play className="mr-1.5 h-3.5 w-3.5" />}
                继续生成 ({pausedCount})
              </Button>
            ) : (generatingCount > 0 || pendingCount > 0) ? (
              <Button size="sm" variant="outline" onClick={handlePauseGeneration} disabled={pausingGeneration} title="暂停整个项目的资源图生成队列；已在执行中的个别任务可能自然完成">
                {pausingGeneration ? <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" /> : <Pause className="mr-1.5 h-3.5 w-3.5" />}
                暂停生成
              </Button>
            ) : null}
            <Button size="sm" onClick={handleGenerateAll} title="为所有待生成的资源一键生成图片">
              <Sparkles className="mr-1.5 h-3.5 w-3.5" />
              一键生成全部
            </Button>
            <Button size="sm" variant="outline" onClick={handleRegenerateAll} title="强制重新生成，包含已完成的资源图片">
              <RotateCcw className="mr-1.5 h-3.5 w-3.5" />
              全部重新生成
            </Button>
          </div>
        </div>
      ) : failedCount > 0 ? (
        <div className="mb-3 flex items-center justify-end">
          <Button size="sm" variant="outline" onClick={handleRetryAllFailed} title="统一进入模型确认弹窗后重试当前范围内所有失败资源">
            <RefreshCw className="mr-1.5 h-3.5 w-3.5" />
            重试失败 ({failedCount})
          </Button>
        </div>
      ) : null}

      {/* Filter buttons */}
      <div className="mb-4 flex flex-wrap items-center gap-2">
        <div className="relative mr-2 min-w-[220px] flex-1 sm:max-w-xs">
          <Search className="pointer-events-none absolute left-3 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-surface-400" />
          <Input
            value={keyword}
            onChange={(e) => setKeyword(e.target.value)}
            placeholder="搜索资源名称 / 描述"
            className="h-8 pl-8 text-xs"
          />
        </div>
        {([['all', '全部'], ['character', '人物'], ['scene', '场景'], ['prop', '物品']] as const).map(([k, label]) => (
          <Button key={k} size="sm" variant={filter === k ? 'default' : 'outline'} onClick={() => setFilter(k as AssetType | 'all')}>
            {label}{k !== 'all' && <span className="ml-1 opacity-60">{counts[k]}</span>}
          </Button>
        ))}
        <Separator orientation="vertical" className="mx-1 h-5" />
        {STATUS_FILTERS.map(({ key, label, color }) => {
          const count = key === 'all' ? statsTotal
            : key === 'failed' ? failedCount
            : key === 'completed' ? completedCount
            : key === 'generating' ? generatingCount
            : key === 'paused' ? pausedCount
            : pendingCount
          return (
            <Button
              key={key}
              size="sm"
              variant={statusFilter === key ? 'default' : 'outline'}
              className={statusFilter !== key && color ? color : ''}
              onClick={() => setStatusFilter(key as AssetStatus | 'all')}
            >
              {label}{count > 0 && ` (${count})`}
            </Button>
          )
        })}
        <div className="ml-auto">
          <Button size="sm" variant="outline" onClick={() => setShowCreateDialog(true)} title="手动创建新的资源条目">
            <Plus className="mr-1.5 h-3.5 w-3.5" />
            手动创建
          </Button>
        </div>
      </div>

      {/* Asset grid */}
      {assets.length === 0 ? (
        <p className="py-12 text-center text-sm text-surface-400">
          {(filter !== 'all' || statusFilter !== 'all' || episodeFilter !== 'all' || keyword.trim()) ? (
            <>
              当前筛选条件下无资源。
              <button className="ml-1 text-primary-500 hover:underline" onClick={() => { setFilter('all'); setStatusFilter('all'); setEpisodeFilter('all'); setKeyword('') }}>清除筛选</button>
            </>
          ) : (
            <>
              暂无资源。请先上传剧本并生成集数，然后手动提取角色、场景、道具。<br />
              <span className="text-xs text-surface-300">提取完成后可点击「一键生成全部」进行图像生成。</span>
            </>
          )}
        </p>
      ) : (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
          {assets.map((asset) => {
            const linkedStoryboards = scopedAssetStoryboardUsageMap.get(asset.id) ?? []
            const assetProgress = getAssetGenerationProgress(asset)
            const rawGeneratedImages = Array.isArray(asset.metadata?.generated_images)
              ? (asset.metadata!.generated_images as Array<{ url?: string; model_name?: string }>)
              : []
            // Per-model completion counts (from saved generated_images)
            const modelImageCounts: Record<string, number> = {}
            for (const img of rawGeneratedImages) {
              const m = img.model_name?.trim() || '未知模型'
              modelImageCounts[m] = (modelImageCounts[m] ?? 0) + 1
            }
            const modelEntries = Object.entries(modelImageCounts)
            const hasMultiModelData = modelEntries.length > 1 || (modelEntries.length === 1 && asset.status === 'generating' && assetProgress?.model_name && assetProgress.model_name !== modelEntries[0]?.[0])
            return (
            <Card key={asset.id} className="group relative flex h-full flex-col overflow-hidden border-surface-200 bg-white/95 shadow-sm transition-all hover:-translate-y-0.5 hover:shadow-md">
              <div
                className="relative aspect-[4/3] cursor-pointer overflow-hidden bg-surface-100"
                onClick={() => setSelectedAsset(asset)}
              >
                {asset.image_url ? (
                  <img src={asset.image_url} alt={asset.name} className="h-full w-full object-cover transition-transform group-hover:scale-105" />
                ) : (
                  <div className={`flex h-full flex-col items-center justify-center gap-1.5 ${TYPE_PLACEHOLDER_BG[asset.type]}`}>
                    <span className={TYPE_PLACEHOLDER_ICON_COLOR[asset.type]}>{TYPE_PLACEHOLDER_ICONS[asset.type]}</span>
                    <span className="max-w-[80%] truncate text-[11px] font-medium text-surface-400">{asset.name}</span>
                  </div>
                )}
                <div className="absolute inset-x-0 top-0 flex items-start justify-between gap-2 bg-gradient-to-b from-black/45 via-black/10 to-transparent p-3">
                  <div className="flex flex-wrap items-center gap-1.5">
                    <span className={`inline-flex items-center gap-0.5 rounded-full px-1.5 py-0.5 text-[10px] font-medium ${TYPE_COLORS[asset.type]}`}>
                      {TYPE_BADGE_ICONS[asset.type]}
                      {TYPE_LABELS[asset.type]}
                    </span>
                    <StatusBadge status={asset.status} />
                  </div>
                  {asset.is_locked && (
                    <span className="rounded-full bg-amber-100/95 p-1 text-amber-700">
                      <Lock className="h-3.5 w-3.5" />
                    </span>
                  )}
                </div>
                {/* Per-asset generation progress bar */}
                {asset.status === 'generating' && assetProgress && (
                  <div className="absolute inset-x-0 bottom-0 bg-black/40 px-2 pb-1.5 pt-1">
                    <div className="flex items-center justify-between">
                      <span className="text-[10px] text-white/90 truncate max-w-[70%]">
                        {assetProgress.model_name ? `${assetProgress.model_name}` : assetProgress.label}
                      </span>
                      <span className="text-[10px] font-semibold text-white">{assetProgress.percent}%</span>
                    </div>
                    <div className="mt-0.5 h-1 w-full overflow-hidden rounded-full bg-white/20">
                      <div
                        className="h-full rounded-full bg-blue-400 transition-all duration-500"
                        style={{ width: `${assetProgress.percent}%` }}
                      />
                    </div>
                  </div>
                )}
                {rawGeneratedImages.length > 1 && asset.status !== 'generating' && (
                  <div className="absolute left-2 bottom-2">
                    <span className="inline-flex items-center gap-1 rounded-full bg-black/50 px-1.5 py-0.5 text-[10px] font-medium text-white backdrop-blur-sm">
                      <Images className="h-3 w-3" />
                      {rawGeneratedImages.length}
                    </span>
                  </div>
                )}
                {asset.image_url && asset.status !== 'generating' && (
                  <div className="absolute right-2 bottom-2 opacity-0 transition-opacity group-hover:opacity-100">
                    <ZoomBadge src={asset.image_url} alt={asset.name} />
                  </div>
                )}
                {/* Multi-model hover tooltip */}
                {(hasMultiModelData || (asset.status === 'generating' && assetProgress)) && (
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <div className="absolute left-0 top-0 h-full w-full" />
                    </TooltipTrigger>
                    <TooltipContent side="right" className="max-w-[200px] space-y-1 p-2 text-[11px]">
                      {modelEntries.length > 0 && (
                        <>
                          <p className="font-semibold text-white/70 mb-1">已生成图像</p>
                          {modelEntries.map(([model, count]) => (
                            <div key={model} className="flex items-center justify-between gap-3">
                              <span className="truncate text-white/90">{model}</span>
                              <span className="shrink-0 rounded bg-white/20 px-1">{count} 张</span>
                            </div>
                          ))}
                        </>
                      )}
                      {asset.status === 'generating' && assetProgress?.model_name && (
                        <div className="flex items-center gap-1.5 border-t border-white/20 pt-1 mt-1">
                          <Loader2 className="h-3 w-3 animate-spin shrink-0" />
                          <span className="truncate text-amber-300">{assetProgress.model_name} {assetProgress.percent}%</span>
                        </div>
                      )}
                    </TooltipContent>
                  </Tooltip>
                )}
              </div>
              <CardContent className="flex flex-1 flex-col p-4">
                <div className="min-h-0 flex-1">
                  <div className="mb-2 flex items-start justify-between gap-2">
                    <div className="min-w-0">
                      <p className="truncate text-sm font-semibold text-surface-900">{asset.name}</p>
                      <p className="mt-1 line-clamp-2 text-xs leading-5 text-surface-500">
                        {asset.description || '暂无描述，可通过右侧对话继续补充和修改。'}
                      </p>
                    </div>
                  </div>

                  <div className="flex flex-wrap items-center gap-1.5">
                    {episodes.length > 0 && (asset.episode_ids ?? []).length > 0 ? (
                      <span className="inline-flex rounded-full bg-surface-100 px-1.5 py-0.5 text-[10px] text-surface-500">
                        {(asset.episode_ids ?? []).map((eid) => {
                          const ep = episodes.find((e) => e.id === eid)
                          return ep ? `第${ep.episode_number}集` : null
                        }).filter(Boolean).join('、') || ''}
                      </span>
                    ) : (
                      <span className="inline-flex rounded-full bg-surface-100 px-1.5 py-0.5 text-[10px] text-surface-400">
                        未关联集数
                      </span>
                    )}
                    {linkedStoryboards.length > 0 ? (
                      <>
                        <span className="inline-flex rounded-full bg-emerald-50 px-1.5 py-0.5 text-[10px] text-emerald-700">
                          已用于 {linkedStoryboards.length} 条分镜
                        </span>
                        {linkedStoryboards.slice(0, 2).map((storyboard) => (
                          <span key={storyboard.id} className="inline-flex rounded-full border border-emerald-100 bg-white px-1.5 py-0.5 text-[10px] text-emerald-700">
                            {formatStoryboardReference(storyboard)}
                          </span>
                        ))}
                        {linkedStoryboards.length > 2 && (
                          <span className="inline-flex rounded-full bg-surface-100 px-1.5 py-0.5 text-[10px] text-surface-500">
                            +{linkedStoryboards.length - 2}
                          </span>
                        )}
                      </>
                    ) : (
                      <span className="inline-flex rounded-full bg-surface-100 px-1.5 py-0.5 text-[10px] text-surface-400">
                        未用于分镜
                      </span>
                    )}
                    {asset.is_manual && (
                      <Badge variant="outline" className="text-[10px]">
                        手动
                      </Badge>
                    )}
                  </div>
                </div>

                {asset.type === 'character' && (
                  <div className="mt-2 flex items-center gap-1.5" onClick={(e) => e.stopPropagation()}>
                    <Mic className="h-3 w-3 flex-shrink-0 text-surface-400" />
                    <select
                      value={asset.voice_model ?? ''}
                      onChange={async (e) => {
                        await assetAPI.update(projectId, asset.id, { voice_model: e.target.value })
                        mutateAssets()
                      }}
                      className="flex-1 rounded border border-surface-200 bg-white px-1.5 py-0.5 text-[10px] text-surface-700 focus:outline-none focus:ring-1 focus:ring-primary-400"
                      title="绑定配音音色（配音模式为「自动按人物分配」时生效）"
                    >
                      {ASSET_VOICE_OPTIONS.map(v => (
                        <option key={v.value} value={v.value}>{v.label}</option>
                      ))}
                    </select>
                  </div>
                )}
                {(asset.status === 'failed' || asset.status === 'qa_failed') && asset.error_msg && (
                  <div className="mt-3 flex items-start gap-1 rounded-md border border-red-100 bg-red-50 px-2 py-1.5">
                    <AlertCircle className="mt-0.5 h-3 w-3 flex-shrink-0 text-red-400" />
                    <p className="line-clamp-2 text-[10px] text-red-500" title={asset.error_msg}>{formatErrorMsg(asset.error_msg)}</p>
                  </div>
                )}
                {(asset.status === 'failed' || asset.status === 'qa_failed' || asset.status === 'pending' || asset.status === 'paused') && (
                  <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                      <Button size="sm" variant="outline" className="mt-3 h-8 w-full text-[11px]" onClick={(e) => e.stopPropagation()}>
                        <RefreshCw className="mr-1 h-3 w-3" />
                        {(asset.status === 'failed' || asset.status === 'qa_failed') ? '重新生成' : asset.status === 'paused' ? '继续' : '生成'}
                        <ChevronDown className="ml-1 h-3 w-3" />
                      </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end" onClick={(e) => e.stopPropagation()} className="w-64">
                      <DropdownMenuLabel className="text-[10px] text-surface-400">统一进入确认弹窗</DropdownMenuLabel>
                      <DropdownMenuSeparator />
                      <DropdownMenuItem className="cursor-pointer px-3 py-2" onClick={() => openSingleAssetGenerateDialog(asset)}>
                        <div>
                          <span className="text-xs font-medium">选择模型并确认</span>
                          <p className="text-[10px] text-surface-400">进入统一弹窗后可选择一个或多个模型，再手动确认生成</p>
                        </div>
                      </DropdownMenuItem>

                    </DropdownMenuContent>
                  </DropdownMenu>
                )}
                <div className="mt-3 flex gap-1.5 border-t border-surface-100 pt-3">
                  <Button size="sm" variant="ghost" className="h-8 flex-1 justify-center" onClick={() => setSelectedAsset(asset)} title="查看">
                    <Eye className="mr-1.5 h-3.5 w-3.5" />
                    查看
                  </Button>
                  {asset.status === 'completed' ? (
                    <DropdownMenu>
                      <DropdownMenuTrigger asChild>
                        <Button size="sm" variant="ghost" className="h-8 flex-1 justify-center" title="重新生成" onClick={(e) => e.stopPropagation()}>
                          <RefreshCw className="mr-1.5 h-3.5 w-3.5" />
                          重新生成
                          <ChevronDown className="ml-1 h-3 w-3" />
                        </Button>
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align="end" onClick={(e) => e.stopPropagation()} className="w-64">
                        <DropdownMenuLabel className="text-[10px] text-surface-400">统一进入确认弹窗</DropdownMenuLabel>
                        <DropdownMenuSeparator />
                        <DropdownMenuItem className="cursor-pointer px-3 py-2" onClick={() => openSingleAssetGenerateDialog(asset)}>
                          <div>
                            <span className="text-xs font-medium">选择模型并确认</span>
                            <p className="text-[10px] text-surface-400">进入统一弹窗后可选择一个或多个模型，再手动确认重新生成</p>
                          </div>
                        </DropdownMenuItem>
                        <DropdownMenuSeparator />
                        <DropdownMenuItem className="cursor-pointer px-3 py-2 text-amber-700 focus:text-amber-700" onClick={() => handleResetAsset(asset.id)}>
                          <RotateCcw className="mr-2 h-3.5 w-3.5" />
                          <div>
                            <span className="text-xs font-medium">重置图片</span>
                            <p className="text-[10px] text-surface-400">清除已生成图片，恢复为待生成</p>
                          </div>
                        </DropdownMenuItem>
                      </DropdownMenuContent>
                    </DropdownMenu>
                  ) : (
                    <Button size="sm" variant="ghost" className="h-8 flex-1 justify-center" onClick={() => openSingleAssetGenerateDialog(asset)} title="生成">
                      <Sparkles className="mr-1.5 h-3.5 w-3.5" />
                      生成
                    </Button>
                  )}
                  <Button size="sm" variant="ghost" className="h-8 px-2" onClick={() => handleToggleLock(asset)} title={asset.is_locked ? '解锁' : '锁定'}>
                    {asset.is_locked ? <Unlock className="h-3.5 w-3.5" /> : <Lock className="h-3.5 w-3.5" />}
                  </Button>
                  <Button size="sm" variant="ghost" className="h-8 px-2 text-red-500 hover:text-red-700" onClick={() => handleDelete(asset.id)} title="删除">
                    <Trash2 className="h-3.5 w-3.5" />
                  </Button>
                </div>
              </CardContent>
            </Card>
            )
          })}
        </div>
      )}

      {/* Pagination controls */}
      {totalPages > 1 && (
        <div className="flex items-center justify-between rounded-lg border border-surface-200 bg-surface-50 px-4 py-3">
          <span className="text-xs text-surface-500">
            共 {totalAssets} 项，第 {page}/{totalPages} 页
          </span>
          <div className="flex items-center gap-1">
            <Button size="sm" variant="outline" className="h-7 px-2 text-xs" disabled={page <= 1} onClick={() => setPage(1)} title="跳转到第一页">
              首页
            </Button>
            <Button size="sm" variant="outline" className="h-7 px-2 text-xs" disabled={page <= 1} onClick={() => setPage(p => Math.max(1, p - 1))} title="上一页">
              上一页
            </Button>
            {/* Page number buttons */}
            {(() => {
              const pages: number[] = []
              const start = Math.max(1, page - 2)
              const end = Math.min(totalPages, page + 2)
              for (let i = start; i <= end; i++) pages.push(i)
              return pages.map(p => (
                <Button key={p} size="sm" variant={p === page ? 'default' : 'outline'} className="h-7 w-7 p-0 text-xs" onClick={() => setPage(p)} title={`跳转到第 ${p} 页`}>
                  {p}
                </Button>
              ))
            })()}
            <Button size="sm" variant="outline" className="h-7 px-2 text-xs" disabled={page >= totalPages} onClick={() => setPage(p => Math.min(totalPages, p + 1))} title="下一页">
              下一页
            </Button>
            <Button size="sm" variant="outline" className="h-7 px-2 text-xs" disabled={page >= totalPages} onClick={() => setPage(totalPages)} title="跳转到最后一页">
              末页
            </Button>
          </div>
        </div>
      )}

      {/* Asset detail slide-out panel */}
      {selectedAsset && (
        <div className="fixed inset-0 z-40 flex justify-end">
          <div className="absolute inset-0 bg-black/30" onClick={() => setSelectedAsset(null)} />
          <div className="relative z-50 flex h-full w-full max-w-6xl flex-col bg-white shadow-xl xl:max-w-7xl">
            <div className="flex items-center justify-between border-b bg-white/95 px-5 py-4 backdrop-blur-sm">
              <div className="min-w-0">
                <h3 className="truncate text-lg font-semibold">{selectedAsset.name}</h3>
                <p className="mt-1 text-xs text-surface-400">资源详情、修改记录与对话调优</p>
              </div>
              <Button size="sm" variant="ghost" onClick={() => setSelectedAsset(null)}>
                <X className="h-4 w-4" />
              </Button>
            </div>
            <div className="grid min-h-0 flex-1 lg:grid-cols-[minmax(0,1.05fr)_minmax(460px,0.95fr)]">
              <div className="flex min-h-0 flex-col border-b border-surface-200 bg-surface-50/60 p-5 lg:border-b-0 lg:border-r">
                <div className="mb-3 flex items-center justify-between">
                  <h4 className="text-sm font-semibold text-surface-700">资源预览</h4>
                  <div className="flex items-center gap-2">
                    <StatusBadge status={selectedAsset.status} />
                  </div>
                </div>

                <div className="flex-1 overflow-y-auto pr-1">
                  {selectedAssetPreviewUrl ? (
                    <div className="rounded-2xl border border-surface-200 bg-white p-3 shadow-sm">
                      <div className="relative">
                        <ZoomableImage src={selectedAssetPreviewUrl} alt={selectedAsset.name} className="w-full rounded-xl border border-surface-100" />
                        {selectedAsset.status === 'generating' && (
                          <div className="absolute inset-x-3 bottom-3 rounded-xl bg-black/65 px-3 py-2 text-xs text-white shadow-lg backdrop-blur-sm">
                            <div className="flex items-center gap-2">
                              <Loader2 className="h-3.5 w-3.5 animate-spin" />
                              <span>{selectedAssetGenerationProgress?.label ?? '正在生成新版本，当前先展示已选图片'}</span>
                            </div>
                            {selectedAssetGenerationProgress ? (
                              <div className="mt-2 space-y-1.5">
                                <Progress value={selectedAssetGenerationProgress.percent} className="h-1.5 bg-white/20" />
                                <div className="flex items-center justify-between gap-2 text-[10px] text-white/80">
                                  <span>{selectedAssetTimingLabel || selectedAssetProgressHint || selectedAssetGenerationProgress.detail || '生成完成后会自动加入候选图片列表。'}</span>
                                  <span>{selectedAssetGenerationProgress.percent}%</span>
                                </div>
                                {(selectedAssetProgressHint || selectedAssetGenerationProgress.detail) && (
                                  <div className="text-[10px] text-white/65">
                                    {selectedAssetProgressHint || selectedAssetGenerationProgress.detail}
                                  </div>
                                )}
                              </div>
                            ) : null}
                          </div>
                        )}
                      </div>

                      <div className="mt-3 flex flex-wrap items-center gap-2">
                        <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${TYPE_COLORS[selectedAsset.type]}`}>
                          {TYPE_LABELS[selectedAsset.type]}
                        </span>
                        {selectedAsset.is_locked ? (
                          <Badge variant="outline" className="text-amber-700">已锁定</Badge>
                        ) : null}
                      </div>
                    </div>
                  ) : selectedAsset.status === 'generating' ? (
                    <div className="flex h-full min-h-[360px] flex-col items-center justify-center rounded-2xl border border-dashed border-primary-200 bg-white/90 p-6 text-center">
                      <Loader2 className="h-10 w-10 animate-spin text-primary-500" />
                      <p className="mt-4 text-sm font-medium text-surface-800">{selectedAssetGenerationProgress?.label ?? '图片资源生成中'}</p>
                      <p className="mt-2 max-w-sm text-xs leading-6 text-surface-500">
                        {selectedAssetTimingLabel || selectedAssetProgressHint || selectedAssetGenerationProgress?.detail || '生成完成后会在左侧自动展示最新图片，右侧聊天记录仍可继续查看和追加修改要求。'}
                      </p>
                      <div className="mt-4 w-full max-w-sm space-y-2">
                        <Progress value={selectedAssetGenerationProgress?.percent ?? 12} className="h-2" />
                        <div className="flex items-center justify-between text-[11px] text-surface-500">
                          <span>{selectedAssetGenerationProgress?.model_name || selectedProjectImageModelLabel}</span>
                          <span>{selectedAssetGenerationProgress?.percent ?? 12}%</span>
                        </div>
                        {selectedAssetProgressHint ? (
                          <p className="text-[11px] text-surface-500">{selectedAssetProgressHint}</p>
                        ) : null}
                        {selectedAssetGenerationProgress?.task_id ? (
                          <p className="text-[10px] text-surface-400">任务 #{selectedAssetGenerationProgress.task_id}</p>
                        ) : null}
                      </div>
                    </div>
                  ) : (
                    <div className="flex h-full min-h-[360px] flex-col items-center justify-center rounded-2xl border border-dashed border-surface-200 bg-white/80 p-6 text-center">
                      <Image className="h-10 w-10 text-surface-300" />
                      <p className="mt-4 text-sm font-medium text-surface-700">暂无图片资源</p>
                      <p className="mt-2 max-w-sm text-xs leading-6 text-surface-400">
                        你可以先在右侧通过聊天修改描述，再触发图片生成；生成结果会显示在这里。
                      </p>
                    </div>
                  )}

                  <div className="mt-4 space-y-3 pb-3">
                    {selectedAssetGeneratedImages.length > 0 && (
                      <div className="rounded-xl border border-surface-200 bg-white p-4 shadow-sm">
                        <div className="flex items-center justify-between gap-3">
                          <div>
                            <p className="text-xs font-medium text-surface-500">候选图片</p>
                            <p className="mt-1 text-[11px] text-surface-400">保留历史生成结果，可手动切换当前展示图。</p>
                          </div>
                          <div className="flex items-center gap-2">
                            <Badge variant="outline">{selectedAssetGeneratedImages.length} 张</Badge>
                            <button
                              type="button"
                              className="flex items-center gap-1 rounded px-1.5 py-1 text-[11px] text-red-500 hover:bg-red-50"
                              onClick={() => handleClearAllGeneratedImages(selectedAsset)}
                              title="清除所有历史生成图片，重置为待生成状态"
                            >
                              <Trash2 className="h-3 w-3" />
                              清除全部
                            </button>
                          </div>
                        </div>
                        <div className="mt-3 grid grid-cols-3 gap-2">
                          {selectedAssetGeneratedImages.map((image) => {
                            const isSelected = image.url === selectedAssetPreviewUrl
                            const isSwitching = selectingImageUrl === image.url

                            return (
                              <div key={image.url} className="group relative">
                                <button
                                  type="button"
                                  className={`relative overflow-hidden rounded-xl border bg-surface-50 transition w-full ${
                                    isSelected ? 'border-primary-500 ring-2 ring-primary-200' : 'border-surface-200 hover:border-primary-300'
                                  }`}
                                  onClick={() => handleSelectGeneratedImage(selectedAsset, image.url)}
                                  disabled={isSwitching}
                                  title={image.created_at ? `生成时间：${formatChatTimestamp(image.created_at)}` : '候选图片'}
                                >
                                  <img src={image.url} alt="" className="aspect-square w-full object-cover" />
                                  {image.model_name && (
                                    <span className="absolute left-1 top-1 rounded bg-black/60 px-1 py-0 text-[9px] text-white" title={`由 ${image.model_name} 生成`}>
                                      {image.model_name}
                                    </span>
                                  )}
                                  <div className="absolute inset-x-0 bottom-0 bg-gradient-to-t from-black/70 via-black/25 to-transparent px-2 py-1.5 text-left text-[10px] text-white">
                                    <div className="flex items-center justify-between gap-2">
                                      <span>{isSelected ? '当前展示' : '点击切换'}</span>
                                      {isSwitching ? <Loader2 className="h-3 w-3 animate-spin" /> : null}
                                    </div>
                                  </div>
                                </button>
                                <button
                                  type="button"
                                  className="absolute right-1 top-1 hidden rounded-full bg-black/60 p-0.5 text-white hover:bg-red-600 group-hover:flex"
                                  onClick={(e) => { e.stopPropagation(); handleDeleteGeneratedImage(selectedAsset, image.url) }}
                                  title="删除此候选图片"
                                >
                                  <X className="h-3 w-3" />
                                </button>
                              </div>
                            )
                          })}
                        </div>
                      </div>
                    )}

                    <div className="rounded-xl border border-surface-200 bg-white p-4 shadow-sm">
                      <div className="flex flex-wrap items-center gap-2">
                        <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${TYPE_COLORS[selectedAsset.type]}`}>
                          {TYPE_LABELS[selectedAsset.type]}
                        </span>
                        {selectedAsset.is_locked && (
                          <span className="flex items-center gap-1 text-xs text-amber-600">
                            <Lock className="h-3 w-3" /> 已锁定
                          </span>
                        )}
                        {episodes.length > 0 && (selectedAsset.episode_ids ?? []).length > 0 ? (
                          <Badge variant="outline">
                            {(selectedAsset.episode_ids ?? []).map((eid) => {
                              const ep = episodes.find((e) => e.id === eid)
                              return ep ? `第${ep.episode_number}集` : null
                            }).filter(Boolean).join('、')}
                          </Badge>
                        ) : null}
                      </div>
                      <p className="mt-3 text-xs font-medium text-surface-500">当前描述</p>
                      <p className="mt-1 text-sm leading-6 text-surface-700">{selectedAsset.description || '暂无描述'}</p>
                      <p className="mt-3 text-xs font-medium text-surface-500">分镜引用</p>
                      {(episodeFilter === 'all' ? selectedAssetLinkedStoryboards : selectedAssetScopedLinkedStoryboards).length > 0 ? (
                        <>
                          <div className="mt-2 flex flex-wrap gap-1.5">
                            {(episodeFilter === 'all' ? selectedAssetLinkedStoryboards : selectedAssetScopedLinkedStoryboards).map((storyboard) => (
                              <span
                                key={storyboard.id}
                                className="inline-flex rounded-full border border-emerald-100 bg-emerald-50 px-2 py-0.5 text-[11px] text-emerald-700"
                              >
                                {formatStoryboardReference(storyboard)}
                              </span>
                            ))}
                          </div>
                          {episodeFilter !== 'all' && selectedAssetLinkedStoryboards.length !== selectedAssetScopedLinkedStoryboards.length && (
                            <p className="mt-2 text-[11px] text-surface-400">
                              当前仅展示第 {currentEpisode?.episode_number ?? episodeFilter} 集内的引用；整个项目共有 {selectedAssetLinkedStoryboards.length} 条分镜使用该资源。
                            </p>
                          )}
                        </>
                      ) : selectedAssetLinkedStoryboards.length > 0 && episodeFilter !== 'all' ? (
                        <p className="mt-1 text-xs text-surface-400">
                          当前集未引用该资源；整个项目共有 {selectedAssetLinkedStoryboards.length} 条分镜使用它。
                        </p>
                      ) : (
                        <p className="mt-1 text-xs text-surface-400">尚未被任何分镜引用</p>
                      )}
                    </div>

                  </div>
                </div>
                <div className="border-t border-surface-200 bg-white/95 p-4 backdrop-blur-sm">
                  <input ref={uploadRef} type="file" accept="image/*" className="hidden" onChange={(e) => {
                    const f = e.target.files?.[0]
                    if (f) handleUploadReplace(selectedAsset.id, f)
                    if (uploadRef.current) uploadRef.current.value = ''
                  }} />
                  <div className="rounded-xl border border-surface-200 bg-surface-50/80 p-3 shadow-sm">
                    <div className="mb-2">
                      <p className="text-xs font-medium text-surface-700">图片上传与替换</p>
                      <p className="mt-1 text-[11px] text-surface-400">左侧操作区固定可见，方便随时上传新图覆盖当前展示。</p>
                    </div>
                    <Button size="sm" variant="outline" className="h-10 w-full bg-white" onClick={() => uploadRef.current?.click()}>
                      <Upload className="mr-1.5 h-3.5 w-3.5" />
                      上传替换图片
                    </Button>
                  </div>
                </div>
              </div>

              <div className="flex min-h-0 flex-col overflow-hidden bg-white">
                <div className="min-h-0 overflow-y-auto border-b p-5">
                  <div className="mb-5 space-y-3 text-sm">
                    {(selectedAsset.status === 'failed' || selectedAsset.status === 'qa_failed') && selectedAsset.error_msg && (
                      <div className="rounded-md border border-red-200 bg-red-50 p-3">
                        <div className="mb-1 flex items-center gap-1.5">
                          <AlertCircle className="h-3.5 w-3.5 text-red-500" />
                          <span className="text-xs font-medium text-red-700">
                            {selectedAsset.status === 'qa_failed' ? 'AI 质检未通过（已自动重试）' : '生成失败'}
                          </span>
                        </div>
                        <p className="text-xs text-red-600">{selectedAsset.error_msg}</p>
                      </div>
                    )}
                    {(selectedAsset.status === 'pending' || selectedAsset.status === 'failed' || selectedAsset.status === 'qa_failed' || selectedAsset.status === 'paused') && (
                      <div className="rounded-md border border-surface-200 bg-surface-50 p-3">
                        <p className="mb-2.5 text-xs font-medium text-surface-600">
                          {(selectedAsset.status === 'failed' || selectedAsset.status === 'qa_failed') ? '统一弹窗重新生成' : selectedAsset.status === 'paused' ? '统一弹窗继续' : '统一弹窗生成'}
                        </p>
                        <Button variant="outline" className="w-full justify-center bg-white" onClick={() => openSingleAssetGenerateDialog(selectedAsset)}>
                          <Sparkles className="mr-1.5 h-3.5 w-3.5" />
                          打开模型确认弹窗
                        </Button>
                        <p className="mt-2 text-[11px] text-surface-400">
                          先在统一弹窗里选择一个或多个模型，再手动确认生成，避免不同入口触发方式不一致。
                        </p>
                      </div>
                    )}
                    {selectedAsset.type === 'character' && (
                      <div className="rounded-xl border border-primary-100 bg-primary-50/60 p-4">
                        <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                          <div>
                            <p className="text-xs font-medium text-surface-700">人物资源图生成</p>
                            <p className="mt-1 text-xs text-surface-500">
                              自动生成四视图人物资源图（头像特写 + 正面全身 + 90°侧面全身 + 背面全身合成一张），供分镜及后续生图模型识别角色。
                            </p>
                          </div>
                          <Badge variant="outline" className="w-fit">
                            {selectedProjectImageModelLabel}
                          </Badge>
                        </div>
                        <div className="mt-3">
                          <Button
                            size="sm"
                            onClick={() => openSingleAssetGenerateDialog(selectedAsset)}
                          >
                            <Sparkles className="mr-1.5 h-3.5 w-3.5" />
                            生成人物资源图
                          </Button>
                        </div>
                      </div>
                    )}
                    {selectedAsset.consistency_ref && Object.keys(selectedAsset.consistency_ref).length > 0 && (
                      <div className="rounded-xl bg-surface-50 p-3">
                        <p className="mb-1 text-xs font-medium text-surface-500">一致性参考</p>
                        <pre className="text-xs text-surface-600">{JSON.stringify(selectedAsset.consistency_ref, null, 2)}</pre>
                      </div>
                    )}
                  </div>
                </div>

                <div className="flex min-h-0 flex-1 flex-col p-5">
                  <div className="mb-3 flex items-center justify-between">
                    <h4 className="text-sm font-semibold text-surface-700">AI 对话记录</h4>
                    <span className="text-[11px] text-surface-400">
                      {selectedAsset.agent_history?.length ?? 0} 条消息
                    </span>
                  </div>
                  <div
                    ref={chatListRef}
                    onScroll={handleChatListScroll}
                    className="min-h-0 flex-1 space-y-3 overflow-y-auto rounded-xl border border-surface-200 bg-surface-50/70 p-3 pr-2"
                  >
                    {selectedAsset.status === 'generating' && selectedAssetGenerationProgress && (
                      <div className="rounded-2xl border border-primary-200 bg-primary-50/80 px-3 py-3 text-xs text-primary-900 shadow-sm">
                        <div className="flex items-center justify-between gap-3">
                          <div className="flex items-center gap-2 font-medium">
                            <Loader2 className="h-3.5 w-3.5 animate-spin" />
                            <span>{selectedAssetGenerationProgress.label}</span>
                          </div>
                          <span>{selectedAssetGenerationProgress.percent}%</span>
                        </div>
                        <Progress value={selectedAssetGenerationProgress.percent} className="mt-2 h-1.5" />
                        <div className="mt-2 flex flex-wrap items-center justify-between gap-2 text-[11px] text-primary-700/80">
                          <span>{selectedAssetTimingLabel || selectedAssetGenerationProgress.detail || '新图生成完成后会自动展示在左侧。'}</span>
                          {selectedAssetGenerationProgress.task_id ? <span>任务 #{selectedAssetGenerationProgress.task_id}</span> : null}
                        </div>
                        {selectedAssetProgressHint ? (
                          <p className="mt-1 text-[11px] text-primary-700/70">{selectedAssetProgressHint}</p>
                        ) : null}
                      </div>
                    )}
                    {selectedAsset.status === 'generating' && (
                      <div className="flex justify-start">
                        <div className="max-w-[88%] rounded-2xl border border-surface-200 bg-white px-3 py-2.5 text-xs text-surface-700 shadow-sm">
                          <div className="mb-1 flex items-center gap-2 text-[10px] text-surface-400">
                            <Loader2 className="h-3 w-3 animate-spin" />
                            <span>AI 正在生成待返回图片</span>
                          </div>
                          <div className="overflow-hidden rounded-xl border border-dashed border-primary-200 bg-primary-50/60">
                            {selectedAssetPreviewUrl ? (
                              <div className="relative">
                                <img src={selectedAssetPreviewUrl} alt="" className="h-36 w-full object-cover opacity-75" />
                                <div className="absolute inset-0 flex items-center justify-center bg-surface-950/35">
                                  <div className="rounded-full bg-white/90 px-3 py-1 text-[11px] font-medium text-surface-700 shadow-sm">
                                    新版本生成中
                                  </div>
                                </div>
                              </div>
                            ) : (
                              <div className="flex h-36 w-full flex-col items-center justify-center gap-2">
                                <Image className="h-8 w-8 text-primary-300" />
                                <span className="text-[11px] text-surface-500">待生成图片将在这里返回</span>
                              </div>
                            )}
                          </div>
                          <div className="mt-2 space-y-1.5 leading-5">
                            <div className="flex items-center justify-between gap-2 text-[11px]">
                              <span>{selectedAssetTimingLabel || selectedAssetProgressHint || '当前资源图片仍在生成中。'}</span>
                              <span>{selectedAssetGenerationProgress?.percent ?? 12}%</span>
                            </div>
                            <Progress value={selectedAssetGenerationProgress?.percent ?? 12} className="h-1.5" />
                            <div className="flex flex-wrap items-center justify-between gap-2 text-[10px] text-surface-400">
                              <span>{selectedAssetGenerationProgress?.detail || '生成完成后会自动显示在左侧预览区域，并加入候选图片列表。'}</span>
                              <span>{selectedAssetGenerationProgress?.model_name || selectedProjectImageModelLabel}</span>
                            </div>
                          </div>
                        </div>
                      </div>
                    )}
                    {(!selectedAsset.agent_history || selectedAsset.agent_history.length === 0) ? (
                      <p className="py-4 text-center text-xs text-surface-400">暂无对话记录</p>
                    ) : (
                      selectedAsset.agent_history.map((rawMsg, i) => {
                        const msg = rawMsg as LegacyChatMessage
                        const role = getChatRole(msg)
                        const content = getChatContent(msg)
                        const imageUrl = getChatImageUrl(msg)
                        const imageModel = getChatImageModel(msg)

                        return (
                          <div key={i} className={`flex ${role === 'user' ? 'justify-end' : 'justify-start'}`}>
                            <div className={`max-w-[88%] rounded-2xl px-3 py-2.5 text-xs shadow-sm ${
                              role === 'user'
                                ? 'bg-primary-500 text-white'
                                : 'border border-surface-200 bg-white text-surface-700'
                            }`}>
                              <div className={`mb-1 flex items-center justify-between gap-3 text-[10px] ${
                                role === 'user' ? 'text-primary-100' : 'text-surface-400'
                              }`}>
                                <span>{role === 'user' ? '你的修改要求' : 'AI 回应'}</span>
                                <span>{formatChatTimestamp(msg.timestamp)}</span>
                              </div>
                              <div className="whitespace-pre-wrap leading-5">
                                {content}
                              </div>
                              {imageUrl && (
                                <div className="relative mt-2">
                                  <img src={imageUrl} alt="" className="max-w-full rounded-lg border border-black/5" />
                                  {imageModel && (
                                    <span className="absolute left-1.5 top-1.5 rounded bg-black/60 px-1.5 py-0.5 text-[9px] text-white" title={`由 ${imageModel} 生成`}>
                                      🤖 {imageModel}
                                    </span>
                                  )}
                                </div>
                              )}
                            </div>
                          </div>
                        )
                      })
                    )}
                    <div ref={chatBottomRef} />
                  </div>
                </div>

                {/* Chat input */}
                <div className="border-t bg-white/95 p-4 backdrop-blur-sm">
                  <div className="rounded-xl border border-surface-200 bg-surface-50/70 p-3">
                    <div className="mb-2 flex items-center justify-between">
                      <div>
                        <p className="text-sm font-medium text-surface-800">对话修改</p>
                        <p className="text-[11px] text-surface-400">告诉 AI 你想怎么改人物设定、场景描述或道具细节。</p>
                      </div>
                      <div className="min-w-[200px]">
                        <Select value={selectedChatModelId} onValueChange={setSelectedChatModelId}>
                          <SelectTrigger className="h-9 bg-white text-xs">
                            <SelectValue placeholder="选择对话模型" />
                          </SelectTrigger>
                          <SelectContent>
                            {(() => {
                              const isMultimodal = (m: Model) => (m.capability_tags ?? []).some((t) => ['vision', 'vision-understanding', 'multimodal', 'image-understanding'].includes(t))
                              const mm = chatModels.filter(isMultimodal)
                              const txt = chatModels.filter((m) => !isMultimodal(m))
                              return (
                                <>
                                  {mm.length > 0 && (
                                    <>
                                      <div className="px-2 py-1 text-[10px] font-semibold uppercase text-surface-400">👁️ 多模态（支持图像输入）</div>
                                      {mm.map((model) => (
                                        <SelectItem key={model.id} value={String(model.id)}>
                                          👁️ {model.name}
                                        </SelectItem>
                                      ))}
                                    </>
                                  )}
                                  {txt.length > 0 && (
                                    <>
                                      <div className="px-2 py-1 text-[10px] font-semibold uppercase text-surface-400">💬 纯文本</div>
                                      {txt.map((model) => (
                                        <SelectItem key={model.id} value={String(model.id)}>
                                          💬 {model.name}
                                        </SelectItem>
                                      ))}
                                    </>
                                  )}
                                </>
                              )
                            })()}
                          </SelectContent>
                        </Select>
                        <p className="mt-1 text-right text-[10px] text-surface-400">{selectedChatModelLabel}</p>
                      </div>
                    </div>
                    <Textarea
                      value={chatInput}
                      onChange={(e) => setChatInput(e.target.value)}
                      placeholder="例如：保留人物服饰不变，补充正面表情更坚定；场景增加傍晚逆光和云层层次。"
                      className="min-h-[110px] resize-none bg-white"
                      onKeyDown={(e) => {
                        if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
                          e.preventDefault()
                          handleChat()
                        }
                      }}
                    />
                    <div className="mt-3 flex items-center justify-between gap-3">
                      <span className="text-[11px] text-surface-400">按 `Ctrl/Cmd + Enter` 快速发送</span>
                      <Button onClick={handleChat} disabled={chatLoading || !chatInput.trim()}>
                        {chatLoading ? (
                          <Loader2 className="mr-1.5 h-4 w-4 animate-spin" />
                        ) : (
                          <Send className="mr-1.5 h-4 w-4" />
                        )}
                        发送修改要求
                      </Button>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Batch generate dialog */}
      <Dialog open={showBatchGenDialog} onOpenChange={setShowBatchGenDialog}>
          <DialogContent className="max-w-lg flex flex-col max-h-[90vh]">
            <DialogHeader>
              <DialogTitle>
              {batchGenTarget.kind === 'asset'
                ? `${batchGenTarget.force ? '重新生成' : '生成'}资源：${batchGenTarget.assetLabel ?? '未命名资源'}`
                : batchGenTarget.kind === 'failed'
                  ? (episodeFilter === 'all' ? '重试全部失败资源' : `重试第 ${orderedEpisodes.find((e) => e.id === episodeFilter)?.episode_number ?? episodeFilter} 集失败资源`)
                  : batchGenTarget.force
                    ? (batchGenTarget.kind === 'all'
                        ? '全部重新生成资源'
                        : `重新生成第 ${orderedEpisodes.find((e) => e.id === batchGenTarget.episodeId)?.episode_number ?? batchGenTarget.episodeId} 集资源`)
                    : (batchGenTarget.kind === 'all'
                        ? '一键生成全部资源'
                        : `生成第 ${orderedEpisodes.find((e) => e.id === batchGenTarget.episodeId)?.episode_number ?? batchGenTarget.episodeId} 集资源`)}
              </DialogTitle>
            </DialogHeader>
          <div className="flex-1 overflow-y-auto space-y-4 pt-2 pr-1">
            {batchGenForce && (
              <div className="flex items-start gap-2 rounded-md border border-amber-200 bg-amber-50 px-3 py-2">
                <AlertCircle className="mt-0.5 h-3.5 w-3.5 flex-shrink-0 text-amber-600" />
                <p className="text-[11px] text-amber-700">
                  {batchGenTarget.kind === 'asset'
                    ? <>将先清除资源 <strong>{batchGenTarget.assetLabel ?? '未命名资源'}</strong> 当前已生成图片，再按所选模型重新生成，当前展示图会被新候选图覆盖。</>
                    : batchGenTarget.kind === 'all'
                      ? <>将先清除当前范围内已完成资源的图片，再对 <strong>全部资源</strong>（含已完成）重新生成，原图将被覆盖。</>
                      : <>将先清除当前集已完成资源的图片，再对本集 <strong>全部资源</strong>（含已完成）重新生成，原图将被覆盖。</>}
                </p>
              </div>
            )}
            {/* Model selector */}
            <div className="space-y-3">
              <Label className="text-xs font-medium">生成模型（可多选）</Label>
              {(
                [
                  { label: '🌐 多模态推荐', filter: (m: typeof MODEL_OPTIONS[0]) => m.tags.includes('多模态') },
                  { label: '🎨 高质量文生图', filter: (m: typeof MODEL_OPTIONS[0]) => m.tags.includes('高质量') && !m.tags.includes('多模态') },
                  { label: '⚡ 高速 / 低成本', filter: (m: typeof MODEL_OPTIONS[0]) => !m.tags.includes('多模态') && !m.tags.includes('高质量') && !m.tags.includes('本地') },
                  { label: '🖥️ 本地部署', filter: (m: typeof MODEL_OPTIONS[0]) => m.tags.includes('本地') },
                ] as Array<{ label: string; filter: (m: typeof MODEL_OPTIONS[0]) => boolean }>
              ).map(({ label: sectionLabel, filter }) => {
                const models = MODEL_OPTIONS.filter(filter)
                if (models.length === 0) return null
                return (
                  <div key={sectionLabel}>
                    <p className="mb-1.5 text-[10px] font-semibold uppercase tracking-wide text-surface-400">{sectionLabel}</p>
                    <div className="grid gap-2 sm:grid-cols-2">
                      {models.map((m) => {
                        const avail = imageModelAvailability[m.key]
                        const selected = batchGenModels.includes(m.key)
                        const broken = !!m.failureReason
                        return (
                          <button
                            key={m.key}
                            type="button"
                            title={broken ? `已停用：${m.failureReason}` : undefined}
                            onClick={() => {
                              if (avail === false || broken) return
                              setBatchGenModels((current) => (
                                selected
                                  ? current.filter((item) => item !== m.key)
                                  : [...current, m.key]
                              ))
                            }}
                            className={`flex items-start gap-2 rounded-lg border p-2.5 text-left transition-colors ${
                              broken
                                ? 'cursor-not-allowed border-red-200 bg-red-50 opacity-60'
                                : selected
                                ? 'border-primary-400 bg-primary-50 ring-1 ring-primary-400'
                                : avail === false
                                ? 'border-surface-200 bg-surface-50 opacity-50'
                                : 'border-surface-200 bg-white hover:border-surface-300'
                            }`}
                          >
                            <span className="mt-0.5 text-base">{m.icon}</span>
                            <div className="min-w-0 flex-1">
                              <div className="flex flex-wrap items-center gap-1">
                                <span className="text-xs font-semibold">{m.label}</span>
                                {!broken && m.speed === 'fast' && <span className="rounded bg-green-100 px-1 text-[9px] text-green-700">⚡ 快</span>}
                                {!broken && m.quality === 'high' && <span className="rounded bg-blue-100 px-1 text-[9px] text-blue-700">★ 高质</span>}
                                {!broken && avail === true && <span className="rounded bg-emerald-100 px-1 text-[9px] text-emerald-700">● 可用</span>}
                                {!broken && avail === false && <span className="rounded bg-red-100 px-1 text-[9px] text-red-600">● 未配置</span>}
                                {broken && <span className="rounded bg-red-100 px-1 text-[9px] text-red-600">⚠ 已停用</span>}
                              </div>
                              <p className="text-[10px] text-surface-400">{getProviderLabel(m.provider)}</p>
                              {broken ? (
                                <p className="mt-0.5 text-[9px] leading-snug text-red-400">{m.failureReason}</p>
                              ) : (
                                <p className="mt-0.5 text-[9px] leading-snug text-surface-500">{m.desc}</p>
                              )}
                            </div>
                          </button>
                        )
                      })}
                    </div>
                  </div>
                )
              })}
              {batchGenModels.length === 0 ? (
                <p className="text-[11px] text-surface-400">未选择将使用项目默认图片模型：{selectedProjectImageModelLabel}</p>
              ) : (
                <p className="text-[11px] text-surface-400">已选 {batchGenModels.length} 个模型；每个模型都会为每项资源各生成一版候选图。</p>
              )}
            </div>

            {/* Prompt area: two zones */}
            <div className="space-y-3">
              {/* Zone 1: Model default prompt (read-only reference) */}
              <div className="space-y-1.5">
                <Label className="text-xs font-medium">模型默认提示词 <span className="text-surface-400 font-normal">（只读参考）</span></Label>
                {batchGenModels.length > 0 ? (
                  <div className="space-y-2">
                    {MODEL_OPTIONS.filter((model) => batchGenModels.includes(model.key)).map((model) => (
                      <div key={model.key} className="rounded-md border border-dashed border-surface-200 bg-surface-50 px-3 py-2">
                        <p className="text-[11px] font-medium text-surface-600">{model.label}</p>
                        <p className="mt-1 text-[11px] leading-relaxed text-surface-500 font-mono break-all">{model.defaultPrompt}</p>
                      </div>
                    ))}
                  </div>
                ) : (
                  <div className="rounded-md border border-dashed border-surface-200 bg-surface-50 px-3 py-2">
                    <p className="text-[11px] text-surface-400 italic">选择模型后将自动显示推荐提示词</p>
                  </div>
                )}
              </div>

              {/* Zone 2: User custom extra prompt with auto-translate */}
              <div className="space-y-1.5">
                <div className="flex items-center justify-between">
                  <Label className="text-xs font-medium">
                    自定义追加内容
                    <span className="ml-1.5 text-surface-400 font-normal text-[10px]">（输入中文将自动翻译并优化为英文提示词）</span>
                  </Label>
                  {batchGenTranslating && (
                    <span className="flex items-center gap-1 text-[10px] text-blue-500">
                      <Loader2 className="h-3 w-3 animate-spin" />优化中…
                    </span>
                  )}
                </div>
                <Textarea
                  value={batchGenUserExtra}
                  onChange={(e) => {
                    const val = e.target.value
                    setBatchGenUserExtra(val)
                    // Debounce: auto-translate+optimize if Chinese detected
                    if (typeof window !== 'undefined') {
                      clearTimeout((window as Window & { _batchTransTimer?: ReturnType<typeof setTimeout> })._batchTransTimer)
                      ;(window as Window & { _batchTransTimer?: ReturnType<typeof setTimeout> })._batchTransTimer = setTimeout(async () => {
                        if (!/[\u4e00-\u9fff]/.test(val) || !val.trim()) return
                        setBatchGenTranslating(true)
                        try {
                          const res = await utilsAPI.translatePrompt(val) as unknown as { translated?: string }
                          const optimized = res?.translated
                          if (optimized && optimized !== val) setBatchGenUserExtra(optimized)
                        } catch { /* silent fail */ } finally {
                          setBatchGenTranslating(false)
                        }
                      }, 900)
                    }
                  }}
                  placeholder="输入风格要求、特殊指令…（支持中文，停止输入后自动翻译并优化）"
                  className={`h-20 resize-none text-xs transition-colors ${batchGenTranslating ? 'border-blue-300 bg-blue-50/30' : ''}`}
                />
                <p className="text-[11px] text-surface-400">
                  内容将拼接到默认提示词末尾。停止输入约 1 秒后自动翻译并优化为英文。
                </p>
              </div>
            </div>

          </div>
          <div className="flex justify-end gap-2 pt-3 border-t mt-1">
            <Button variant="outline" onClick={() => setShowBatchGenDialog(false)}>取消</Button>
            <Button onClick={executeBatchGenerate} disabled={batchGenRunning}>
              {batchGenRunning ? <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" /> : <Sparkles className="mr-1.5 h-3.5 w-3.5" />}
              开始生成
            </Button>
          </div>
        </DialogContent>
      </Dialog>

      {/* Manual create asset dialog */}
      <Dialog open={showCreateDialog} onOpenChange={setShowCreateDialog}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>手动创建资源</DialogTitle>
          </DialogHeader>
          <div className="space-y-4 pt-2">
            <div className="space-y-2">
              <Label>资源类型</Label>
              <Select value={createForm.type} onValueChange={(v: string) => setCreateForm(f => ({ ...f, type: v as AssetType }))}>
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="character">人物</SelectItem>
                  <SelectItem value="scene">场景</SelectItem>
                  <SelectItem value="prop">道具</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <Label>名称</Label>
              <Input
                placeholder="例：孙悟空、花果山、金箍棒"
                value={createForm.name}
                onChange={(e: React.ChangeEvent<HTMLInputElement>) => setCreateForm(f => ({ ...f, name: e.target.value }))}
              />
            </div>
            <div className="space-y-2">
              <Label>描述 / 关键词</Label>
              <Textarea
                placeholder="详细描述外观特征、环境氛围或物品属性，可直接用作图像生成提示词"
                rows={4}
                value={createForm.description}
                onChange={(e: React.ChangeEvent<HTMLTextAreaElement>) => setCreateForm(f => ({ ...f, description: e.target.value }))}
              />
            </div>
            <div className="flex justify-end gap-2 pt-2">
              <Button variant="outline" onClick={() => setShowCreateDialog(false)}>取消</Button>
              <Button onClick={handleCreateAsset} disabled={createLoading}>
                {createLoading && <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />}
                创建
              </Button>
            </div>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  )
}
