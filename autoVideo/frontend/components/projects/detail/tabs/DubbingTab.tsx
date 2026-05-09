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


export function DubbingTab({ projectId, project, mutateProject, episodeId }: { projectId: number; project: Project; mutateProject: () => void ; episodeId?: number }) {
  const { toast } = useToast()
  const dubSharedEpisode = useProjectEpisodeFilter()
  type VoiceOverride = {
    voice_model?: string
    voice_rate?: string
    voice_pitch?: string
    voice_volume?: string
  }
  const [batchDubbing, setBatchDubbing] = useState(false)
  const [batchSubtitle, setBatchSubtitle] = useState(false)
  const [voiceModel, setVoiceModel] = useState('default')
  const [voiceRate, setVoiceRate] = useState('+0%')
  const [voicePitch, setVoicePitch] = useState('+0Hz')
  const [voiceVolume, setVoiceVolume] = useState('+0%')
  const [expandedEp, setExpandedEp] = useState<number | null>(() => {
    const n = Number(dubSharedEpisode.value)
    return dubSharedEpisode.value !== 'all' && !Number.isNaN(n) ? n : null
  })
  useEffect(() => {
    dubSharedEpisode.setValue(expandedEp == null ? 'all' : String(expandedEp))
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [expandedEp])
  const [dubbingDrafts, setDubbingDrafts] = useState<Record<number, string>>({})
  const [subtitleDrafts, setSubtitleDrafts] = useState<Record<number, string>>({})
  const [episodeVoiceOverrides, setEpisodeVoiceOverrides] = useState<Record<number, VoiceOverride>>({})
  const [subtitleTexts, setSubtitleTexts] = useState<Record<number, string>>({})
  const [loadingSubtitle, setLoadingSubtitle] = useState<number | null>(null)
  const [retryingTaskIds, setRetryingTaskIds] = useState<number[]>([])
  const [retryingGroup, setRetryingGroup] = useState<'dubbing' | 'subtitle' | null>(null)
  const [aggregatingDialogues, setAggregatingDialogues] = useState<Record<number, boolean>>({})

  // Dynamically fetch voice list; fall back to static list if API unavailable
  const { data: voicesDataDub } = useSWR(
    'voices',
    () => dubbingAPI.listVoices().then((r) => (r as { data?: { voices?: { key: string; label: string }[] } }).data?.voices ?? null),
    { revalidateOnFocus: false, revalidateOnReconnect: false }
  )
  const VOICE_OPTIONS = [
    { value: 'auto', label: '自动按人物分配' },
    ...(voicesDataDub ?? FALLBACK_VOICE_OPTIONS).map((v) => {
      const key = (v as { key?: string }).key ?? (v as { value?: string }).value ?? ''
      return { value: key, label: v.label }
    }),
  ]
  const VOICE_RATE_OPTIONS = [
    { value: '-30%', label: '慢 -30%' },
    { value: '-15%', label: '慢 -15%' },
    { value: '+0%', label: '正常' },
    { value: '+15%', label: '快 +15%' },
    { value: '+30%', label: '快 +30%' },
  ]
  const VOICE_PITCH_OPTIONS = [
    { value: '-10Hz', label: '低 -10Hz' },
    { value: '-5Hz', label: '低 -5Hz' },
    { value: '+0Hz', label: '正常' },
    { value: '+5Hz', label: '高 +5Hz' },
    { value: '+10Hz', label: '高 +10Hz' },
  ]
  const VOICE_VOLUME_OPTIONS = [
    { value: '-20%', label: '低 -20%' },
    { value: '-10%', label: '低 -10%' },
    { value: '+0%', label: '正常' },
    { value: '+10%', label: '高 +10%' },
    { value: '+20%', label: '高 +20%' },
  ]

  const { data: episodesData, isLoading } = useSWR(
    ['episodes', projectId],
    () => projectAPI.listEpisodes(projectId) as unknown as Promise<{ data: Episode[] }>
  )
  const episodes = ((episodesData as { data?: Episode[] })?.data ?? [])
    .slice()
    .sort((a, b) => a.episode_number - b.episode_number)

  // When called from EpisodeWorkspace with a specific episodeId, show only that episode
  const isSingleEpisodeMode = !!episodeId
  const displayedEpisodes = isSingleEpisodeMode
    ? episodes.filter(ep => ep.id === episodeId)
    : episodes

  // Poll dubbing tasks from a single shared source
  const { data: tasksData, mutate: mutateTasks } = useSWR(
    ['dubbing-tasks', projectId],
    () => dubbingAPI.listTasks(projectId).then(r => r.data ?? []),
    {
      refreshInterval: (data) => {
        if (!project || (!project.enable_dubbing && !project.enable_subtitle)) return 0
        const currentTasks = Array.isArray(data) ? data as DubbingTask[] : []
        if (currentTasks.some((task) => task.status === 'processing' || task.status === 'pending')) return 3000
        return 10000
      },
    }
  )
  const dubbingTaskList: DubbingTask[] = (tasksData as DubbingTask[] | undefined) ?? []

  const getTaskSubmitError = (err: unknown, label: '配音' | '字幕') => {
    const status = (err as { response?: { status?: number } })?.response?.status
    if (status === 409) {
      return `当前集已有进行中的${label}任务`
    }
    if (status === 503) {
      return `${label}服务暂时不可用，请稍后重试`
    }
    return `${label}提交失败`
  }

  // Build a lookup: episodeId -> latest task per type
  const taskByEpisode = useMemo(() => {
    const map: Record<number, { dubbing?: DubbingTask; subtitle?: DubbingTask }> = {}
    for (const t of dubbingTaskList) {
      if (!map[t.episode_id]) map[t.episode_id] = {}
      const existing = map[t.episode_id][t.task_type as 'dubbing' | 'subtitle']
      if (!existing || new Date(t.created_at) > new Date(existing.created_at)) {
        map[t.episode_id][t.task_type as 'dubbing' | 'subtitle'] = t
      }
    }
    return map
  }, [dubbingTaskList])

  // Compute stats from deduplicated map (each episode counted once per type)
  const latestTasks = useMemo(() =>
    Object.values(taskByEpisode).flatMap(v => [v.dubbing, v.subtitle].filter(Boolean)) as DubbingTask[],
    [taskByEpisode]
  )
  const dubbingDone = Object.values(taskByEpisode).filter(v => v.dubbing?.status === 'succeeded').length
  const subtitleDone = Object.values(taskByEpisode).filter(v => v.subtitle?.status === 'succeeded').length
  const processingTasks = latestTasks.filter(t => t.status === 'processing' || t.status === 'pending')
  const hasProcessing = processingTasks.length > 0
  const dubbingProcessingTasks = processingTasks.filter((t) => t.task_type === 'dubbing')
  const subtitleProcessingTasks = processingTasks.filter((t) => t.task_type === 'subtitle')

  const getEpisodeBaseText = (ep: Episode) => ep.script_excerpt || ep.title || ''
  const getDubbingSubmitText = (ep: Episode) => (dubbingDrafts[ep.id] ?? getEpisodeBaseText(ep)).trim()
  const getSubtitleSubmitText = (ep: Episode) => (subtitleDrafts[ep.id] ?? getEpisodeBaseText(ep)).trim()
  const getEpisodeVoiceOptions = (episodeId: number) => {
    const override = episodeVoiceOverrides[episodeId]
    return {
      voice_model: override?.voice_model || voiceModel,
      voice_rate: override?.voice_rate || voiceRate,
      voice_pitch: override?.voice_pitch || voicePitch,
      voice_volume: override?.voice_volume || voiceVolume,
    }
  }
  const hasEpisodeVoiceOverride = (episodeId: number) => {
    const override = episodeVoiceOverrides[episodeId]
    if (!override) return false
    return Boolean(override.voice_model || override.voice_rate || override.voice_pitch || override.voice_volume)
  }
  const updateEpisodeVoiceOverride = (episodeId: number, key: keyof VoiceOverride, value: string) => {
    setEpisodeVoiceOverrides((prev) => {
      const merged = {
        ...prev[episodeId],
        [key]: value,
      }
      const normalized: VoiceOverride = {
        voice_model: merged.voice_model && merged.voice_model !== voiceModel ? merged.voice_model : undefined,
        voice_rate: merged.voice_rate && merged.voice_rate !== voiceRate ? merged.voice_rate : undefined,
        voice_pitch: merged.voice_pitch && merged.voice_pitch !== voicePitch ? merged.voice_pitch : undefined,
        voice_volume: merged.voice_volume && merged.voice_volume !== voiceVolume ? merged.voice_volume : undefined,
      }
      if (!normalized.voice_model && !normalized.voice_rate && !normalized.voice_pitch && !normalized.voice_volume) {
        const { [episodeId]: _removed, ...rest } = prev
        return rest
      }
      return { ...prev, [episodeId]: normalized }
    })
  }
  const resetEpisodeVoiceOverride = (episodeId: number) => {
    setEpisodeVoiceOverrides((prev) => {
      const { [episodeId]: _removed, ...rest } = prev
      return rest
    })
  }
  const formatVoiceSettings = (task?: DubbingTask) => {
    if (!task) return ''
    return [task.voice_rate || '+0%', task.voice_pitch || '+0Hz', task.voice_volume || '+0%'].join(' / ')
  }

  // Aggregate storyboard dialogues for an episode when episode has no script text
  const handleAggregateDialogues = async (episodeId: number) => {
    setAggregatingDialogues((prev) => ({ ...prev, [episodeId]: true }))
    try {
      const res = await storyboardAPI.listAll(projectId, { episode_id: episodeId }) as { data?: { id: number; sequence_number: number; dialogue?: string }[] }
      const dialogues = (res?.data ?? [])
        .sort((a, b) => a.sequence_number - b.sequence_number)
        .map((sb) => sb.dialogue || '')
        .filter(Boolean)
      if (dialogues.length === 0) {
        toast({ title: '当前集暂无分镜台词，请先生成分镜', variant: 'destructive' })
        return
      }
      const aggregated = dialogues.join('\n')
      setDubbingDrafts((prev) => ({ ...prev, [episodeId]: aggregated }))
      setSubtitleDrafts((prev) => ({ ...prev, [episodeId]: aggregated }))
      toast({ title: `已从 ${dialogues.length} 个分镜提取台词`, variant: 'success' })
    } catch {
      toast({ title: '提取分镜台词失败', variant: 'destructive' })
    } finally {
      setAggregatingDialogues((prev) => ({ ...prev, [episodeId]: false }))
    }
  }
  const stalledDubbingTasks = dubbingProcessingTasks.filter((task) => task.status === 'processing' && getProgressStallMeta(task.updated_at))
  const stalledSubtitleTasks = subtitleProcessingTasks.filter((task) => task.status === 'processing' && getProgressStallMeta(task.updated_at))
  const handleRetryTask = async (task: DubbingTask, label: '配音' | '字幕') => {
    const episode = episodes.find((ep) => ep.id === task.episode_id)
    const fallbackText = episode
      ? (task.task_type === 'subtitle' ? getSubtitleSubmitText(episode) : getDubbingSubmitText(episode))
      : undefined
    setRetryingTaskIds((prev) => prev.includes(task.id) ? prev : [...prev, task.id])
    try {
      await dubbingAPI.retryTask(projectId, task.id, fallbackText)
      toast({ title: `第 ${episode?.episode_number ?? '?'} 集${label}任务已重新拉起`, variant: 'success' })
      mutateTasks()
      mutateProject()
    } catch (err) {
      const status = (err as { response?: { status?: number } })?.response?.status
      toast({ title: status === 409 ? `${label}任务当前已有活跃任务，无需重复重试` : `${label}任务重试失败`, variant: 'destructive' })
    } finally {
      setRetryingTaskIds((prev) => prev.filter((id) => id !== task.id))
    }
  }
  const handleRetryTaskGroup = async (group: DubbingTask[], label: '配音' | '字幕') => {
    if (group.length === 0) return
    const groupKey = group[0]?.task_type === 'subtitle' ? 'subtitle' : 'dubbing'
    setRetryingGroup(groupKey)
    const uniqueTasks = group.filter((task, index, list) =>
      list.findIndex((item) => item.episode_id === task.episode_id && item.task_type === task.task_type) === index
    )
    setRetryingTaskIds((prev) => Array.from(new Set([...prev, ...uniqueTasks.map((task) => task.id)])))
    let succeeded = 0
    let conflicts = 0
    let failed = 0
    try {
      const res = await dubbingAPI.retryTasksBatch(
        projectId,
        uniqueTasks.map((task) => {
          const episode = episodes.find((ep) => ep.id === task.episode_id)
          const fallbackText = episode
            ? (task.task_type === 'subtitle' ? getSubtitleSubmitText(episode) : getDubbingSubmitText(episode))
            : undefined
          return { task_id: task.id, text: fallbackText }
        })
      ) as unknown as { data?: { retried?: number; conflicts?: number; failed?: number } }
      succeeded = res?.data?.retried ?? 0
      conflicts = res?.data?.conflicts ?? 0
      failed = res?.data?.failed ?? 0
    } catch (error) {
      const status = (error as { response?: { status?: number } })?.response?.status
      if (status === 409) {
        conflicts = uniqueTasks.length
      } else {
        failed = uniqueTasks.length
      }
    } finally {
      setRetryingTaskIds((prev) => prev.filter((id) => !uniqueTasks.some((task) => task.id === id)))
    }
    if (succeeded > 0) {
      toast({ title: `已重新拉起 ${succeeded} 个${label}任务`, variant: 'success' })
      mutateTasks()
      mutateProject()
      if (conflicts > 0) {
        toast({ title: `${conflicts} 个${label}任务已有活跃任务，已跳过重复重试`, variant: 'default' })
      }
      if (failed > 0) {
        toast({ title: `${failed} 个${label}任务重试失败`, variant: 'destructive' })
      }
    } else {
      toast({ title: conflicts > 0 ? `${label}任务已有活跃任务，未重复重试` : `${label}任务重试失败`, variant: conflicts > 0 ? 'default' : 'destructive' })
    }
    setRetryingGroup(null)
  }

  const handleEnableFeatures = async (dubbing: boolean, subtitle: boolean) => {
    try {
      await projectAPI.update(projectId, { enable_dubbing: dubbing, enable_subtitle: subtitle } as Partial<Project>)
      toast({ title: '功能已启用', variant: 'success' })
      mutateProject()
    } catch {
      toast({ title: '启用失败', variant: 'destructive' })
    }
  }

  if (!project.enable_dubbing && !project.enable_subtitle) {
    return (
      <div className="flex flex-col items-center justify-center py-16">
        <Mic className="mb-3 h-12 w-12 text-surface-300" />
        <p className="mb-1 text-base font-medium text-surface-500">配音和字幕功能未启用</p>
        <p className="mb-4 text-sm text-surface-400">点击下方按钮开启</p>
        <div className="flex gap-3">
          <Button variant="outline" onClick={() => handleEnableFeatures(true, false)} title="为集数启用 AI 配音功能">
            <Mic className="mr-1.5 h-4 w-4" /> 启用配音
          </Button>
          <Button variant="outline" onClick={() => handleEnableFeatures(false, true)} title="为集数启用字幕生成功能">
            <FileText className="mr-1.5 h-4 w-4" /> 启用字幕
          </Button>
          <Button onClick={() => handleEnableFeatures(true, true)} title="同时启用配音和字幕功能">
            启用全部
          </Button>
        </div>
      </div>
    )
  }

  if (isLoading) return <TabSkeleton />

  const handleGenerateAllDubbing = async () => {
    setBatchDubbing(true)
    try {
      const eligible = episodes.filter(ep => {
        if (!(ep.script_excerpt || ep.title)) return false
        const existing = taskByEpisode[ep.id]?.dubbing
        return !(existing && (existing.status === 'succeeded' || existing.status === 'processing' || existing.status === 'pending'))
      })
      if (eligible.length === 0) {
        toast({ title: '没有可提交的配音集数', variant: 'default' })
        return
      }

      const res = await dubbingAPI.generateBatch(
        projectId,
        eligible.map((ep) => {
          const episodeVoiceOptions = getEpisodeVoiceOptions(ep.id)
          return {
            episode_id: ep.id,
            text: getDubbingSubmitText(ep),
            voice_model: episodeVoiceOptions.voice_model,
            voice_rate: episodeVoiceOptions.voice_rate,
            voice_pitch: episodeVoiceOptions.voice_pitch,
            voice_volume: episodeVoiceOptions.voice_volume,
          }
        })
      ) as unknown as { data?: { created?: number; conflicts?: number; failed?: number } }

      const submitted = res?.data?.created ?? 0
      const conflicts = res?.data?.conflicts ?? 0
      const failed = res?.data?.failed ?? 0
      toast({
        title: failed > 0
          ? `已提交 ${submitted} 集配音，${conflicts} 集跳过，${failed} 集失败`
          : conflicts > 0
            ? `已提交 ${submitted} 集配音，${conflicts} 集跳过`
            : `已提交 ${submitted} 集配音任务`,
        variant: failed > 0 ? 'default' : 'success',
      })
      mutateTasks()
    } catch (err) {
      toast({ title: getTaskSubmitError(err, '配音'), variant: 'destructive' })
    } finally {
      setBatchDubbing(false)
    }
  }

  const handleGenerateAllSubtitle = async () => {
    setBatchSubtitle(true)
    try {
      const eligible = episodes.filter(ep => {
        if (!(ep.script_excerpt || ep.title)) return false
        const existing = taskByEpisode[ep.id]?.subtitle
        return !(existing && (existing.status === 'succeeded' || existing.status === 'processing' || existing.status === 'pending'))
      })
      if (eligible.length === 0) {
        toast({ title: '没有可提交的字幕集数', variant: 'default' })
        return
      }

      const res = await dubbingAPI.generateSubtitleBatch(
        projectId,
        eligible.map((ep) => {
          const episodeVoiceOptions = getEpisodeVoiceOptions(ep.id)
          return {
            episode_id: ep.id,
            text: getSubtitleSubmitText(ep),
            voice_model: episodeVoiceOptions.voice_model,
            voice_rate: episodeVoiceOptions.voice_rate,
            voice_pitch: episodeVoiceOptions.voice_pitch,
            voice_volume: episodeVoiceOptions.voice_volume,
          }
        })
      ) as unknown as { data?: { created?: number; conflicts?: number; failed?: number } }

      const submitted = res?.data?.created ?? 0
      const conflicts = res?.data?.conflicts ?? 0
      const failed = res?.data?.failed ?? 0
      toast({
        title: failed > 0
          ? `已提交 ${submitted} 集字幕，${conflicts} 集跳过，${failed} 集失败`
          : conflicts > 0
            ? `已提交 ${submitted} 集字幕，${conflicts} 集跳过`
            : `已提交 ${submitted} 集字幕任务`,
        variant: failed > 0 ? 'default' : 'success',
      })
      mutateTasks()
    } catch (err) {
      toast({ title: getTaskSubmitError(err, '字幕'), variant: 'destructive' })
    } finally {
      setBatchSubtitle(false)
    }
  }

  // Unique episodes with deduplicated dubbing counts
  const uniqueDubEpisodes = new Set(dubbingTaskList.filter(t => t.task_type === 'dubbing' && t.status === 'succeeded').map(t => t.episode_id))
  const uniqueSubEpisodes = new Set(dubbingTaskList.filter(t => t.task_type === 'subtitle' && t.status === 'succeeded').map(t => t.episode_id))
  const autoVoiceEnabled = voiceModel === 'auto'

  return (
    <div className="space-y-6">
      {/* Action bar — hidden in single-episode mode since batch ops don't apply */}
      {!isSingleEpisodeMode && (
      <div className="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-surface-200 bg-surface-50 px-4 py-3">
        <div className="flex items-center gap-2 text-sm text-surface-600">
          <Mic className="h-4 w-4" />
          <span>共 {episodes.length} 集</span>
          {project.enable_dubbing && <span className="rounded bg-blue-100 px-1.5 py-0.5 text-[11px] text-blue-700">配音 {uniqueDubEpisodes.size}/{episodes.length}</span>}
          {project.enable_subtitle && <span className="rounded bg-green-100 px-1.5 py-0.5 text-[11px] text-green-700">字幕 {uniqueSubEpisodes.size}/{episodes.length}</span>}
          {hasProcessing && (
            <span className="flex items-center gap-1 rounded bg-amber-100 px-1.5 py-0.5 text-[11px] text-amber-700">
              <Loader2 className="h-3 w-3 animate-spin" /> {processingTasks.length} 个任务处理中
            </span>
          )}
          {(stalledDubbingTasks.length > 0 || stalledSubtitleTasks.length > 0) && (
            <span className="rounded bg-red-100 px-1.5 py-0.5 text-[11px] text-red-700">
              检测到进度停滞
            </span>
          )}
        </div>
        <div className="flex flex-wrap items-center gap-2">
          {(project.enable_dubbing || project.enable_subtitle) && (
            <>
              <select
                value={voiceModel}
                onChange={(e) => setVoiceModel(e.target.value)}
                className="h-8 rounded-md border border-surface-200 bg-white px-2 text-xs"
                title="选择音色或自动按人物分配"
              >
                {VOICE_OPTIONS.map(v => (
                  <option key={v.value} value={v.value}>{v.label}</option>
                ))}
              </select>
              <select
                value={voiceRate}
                onChange={(e) => setVoiceRate(e.target.value)}
                className="h-8 rounded-md border border-surface-200 bg-white px-2 text-xs"
                title="选择语速"
              >
                {VOICE_RATE_OPTIONS.map(v => (
                  <option key={v.value} value={v.value}>{v.label}</option>
                ))}
              </select>
              <select
                value={voicePitch}
                onChange={(e) => setVoicePitch(e.target.value)}
                className="h-8 rounded-md border border-surface-200 bg-white px-2 text-xs"
                title="选择音调"
              >
                {VOICE_PITCH_OPTIONS.map(v => (
                  <option key={v.value} value={v.value}>{v.label}</option>
                ))}
              </select>
              <select
                value={voiceVolume}
                onChange={(e) => setVoiceVolume(e.target.value)}
                className="h-8 rounded-md border border-surface-200 bg-white px-2 text-xs"
                title="选择音量"
              >
                {VOICE_VOLUME_OPTIONS.map(v => (
                  <option key={v.value} value={v.value}>{v.label}</option>
                ))}
              </select>
            </>
          )}
          {project.enable_dubbing && (
            <Button size="sm" variant="outline" onClick={handleGenerateAllDubbing} disabled={batchDubbing} title="为所有集数批量生成配音">
              {batchDubbing ? <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" /> : <Mic className="mr-1.5 h-3.5 w-3.5" />}
              一键生成配音
            </Button>
          )}
          {project.enable_subtitle && (
            <Button size="sm" variant="outline" onClick={handleGenerateAllSubtitle} disabled={batchSubtitle} title="为所有集数批量生成字幕">
              {batchSubtitle ? <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" /> : <FileText className="mr-1.5 h-3.5 w-3.5" />}
              一键生成字幕
            </Button>
          )}
        </div>
      </div>
      )}

      {autoVoiceEnabled && (
        <div className="rounded-lg border border-violet-200 bg-violet-50 px-4 py-3 text-xs text-violet-700">
          已启用人物自动配音：文案中使用 <span className="font-medium">角色名：台词</span>、<span className="font-medium">旁白：内容</span> 这类写法时，会按人物自动分配不同声线；未标注的内容按旁白处理。
        </div>
      )}

      {/* Global progress — show when any tasks are processing */}
      {hasProcessing && (
        <div className="grid gap-3 md:grid-cols-2">
          {[
            { key: 'dubbing', label: '配音生成进度', color: 'blue', tasks: dubbingProcessingTasks },
            { key: 'subtitle', label: '字幕生成进度', color: 'green', tasks: subtitleProcessingTasks },
          ].map((group) => {
            if (group.tasks.length === 0) return null
            const totalChunks = group.tasks.reduce((sum, task) => sum + Math.max(task.chunks_total, 1), 0)
            const doneChunks = group.tasks.reduce((sum, task) => sum + task.chunks_done, 0)
            const pct = totalChunks > 0 ? Math.max((doneChunks / totalChunks) * 100, 8) : 8
            const barClass = group.color === 'blue' ? 'bg-blue-500' : 'bg-green-500'
            const chipClass = group.color === 'blue'
              ? 'bg-blue-100 text-blue-700'
              : 'bg-green-100 text-green-700'
            const stalledTasks = group.key === 'dubbing' ? stalledDubbingTasks : stalledSubtitleTasks
            return (
              <div key={group.key} className="rounded-lg border border-surface-200 bg-white px-4 py-3">
                <div className="mb-2 flex items-center justify-between text-xs">
                  <div className="flex items-center gap-2">
                    <Loader2 className={`h-3.5 w-3.5 animate-spin ${group.color === 'blue' ? 'text-blue-500' : 'text-green-500'}`} />
                    <span className="font-medium text-surface-700">{group.label}</span>
                  </div>
                  <span className={`rounded px-1.5 py-0.5 text-[10px] ${chipClass}`}>
                    {doneChunks}/{totalChunks} · {group.tasks.length} 个任务
                  </span>
                </div>
                <div className="h-2 overflow-hidden rounded-full bg-surface-100">
                  <div className={`h-full rounded-full transition-all duration-300 ${barClass}`} style={{ width: `${pct}%` }} />
                </div>
                {stalledTasks.length > 0 && (
                  <div className="mt-2 rounded-md border border-red-200 bg-red-50 px-2.5 py-2 text-[11px] text-red-700">
                    <div className="flex flex-wrap items-center justify-between gap-2">
                      <span>{group.label}存在长时间未更新的任务，可能卡住了。</span>
                      <Button
                        size="sm"
                        variant="outline"
                        className="h-6 border-red-300 bg-white px-2 text-[11px] text-red-700 hover:bg-red-100"
                        onClick={() => handleRetryTaskGroup(stalledTasks, group.key === 'dubbing' ? '配音' : '字幕')}
                        disabled={retryingGroup === group.key}
                      >
                        {retryingGroup === group.key ? <Loader2 className="mr-1 h-3 w-3 animate-spin" /> : <RefreshCw className="mr-1 h-3 w-3" />}
                        全部重试
                      </Button>
                    </div>
                  </div>
                )}
                <div className="mt-2 flex flex-wrap gap-2 text-[11px] text-surface-500">
                  {group.tasks.slice(0, 4).map((task) => (
                    <span key={task.id}>
                      第{episodes.find((e) => e.id === task.episode_id)?.episode_number ?? '?'}集
                      {task.chunks_total > 0 ? ` (${task.chunks_done}/${task.chunks_total})` : ' (等待中)'}
                    </span>
                  ))}
                  {group.tasks.length > 4 ? <span>+{group.tasks.length - 4} 更多</span> : null}
                </div>
              </div>
            )
          })}
        </div>
      )}

      {/* Per-episode list */}
      {displayedEpisodes.length === 0 ? (
        <p className="py-12 text-center text-sm text-surface-400">暂无分集</p>
      ) : (
        <div className="space-y-3">
          {displayedEpisodes.map((ep) => {
            const dubTask = taskByEpisode[ep.id]?.dubbing
            const subTask = taskByEpisode[ep.id]?.subtitle
            const episodeVoiceOptions = getEpisodeVoiceOptions(ep.id)
            const hasOverride = hasEpisodeVoiceOverride(ep.id)
            return (
              <div key={ep.id} className="rounded-lg border p-4">
                <div className="mb-3 flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    <span className="text-sm font-semibold">第 {ep.episode_number} 集</span>
                    <span className="text-xs text-surface-500">{ep.title}</span>
                  </div>
                  <div className="flex items-center gap-2">
                    <span className={`rounded px-2 py-0.5 text-[10px] ${hasOverride ? 'bg-violet-100 text-violet-700' : 'bg-surface-100 text-surface-500'}`}>
                      {hasOverride ? '已覆盖本集参数' : '继承全局参数'}
                    </span>
                    {hasOverride && (
                      <button
                        type="button"
                        className="text-[10px] text-violet-600 hover:underline"
                        onClick={() => resetEpisodeVoiceOverride(ep.id)}
                      >
                        恢复全局参数
                      </button>
                    )}
                  </div>
                </div>

                <div className="mb-3 grid grid-cols-2 gap-2 rounded-md border border-dashed border-surface-200 bg-surface-50 p-3 md:grid-cols-4">
                  <select
                    value={episodeVoiceOptions.voice_model}
                    onChange={(e) => updateEpisodeVoiceOverride(ep.id, 'voice_model', e.target.value)}
                    className="h-8 rounded-md border border-surface-200 bg-white px-2 text-xs"
                    title="本集音色"
                  >
                    {VOICE_OPTIONS.map(v => (
                      <option key={v.value} value={v.value}>{v.label}</option>
                    ))}
                  </select>
                  <select
                    value={episodeVoiceOptions.voice_rate}
                    onChange={(e) => updateEpisodeVoiceOverride(ep.id, 'voice_rate', e.target.value)}
                    className="h-8 rounded-md border border-surface-200 bg-white px-2 text-xs"
                    title="本集语速"
                  >
                    {VOICE_RATE_OPTIONS.map(v => (
                      <option key={v.value} value={v.value}>{v.label}</option>
                    ))}
                  </select>
                  <select
                    value={episodeVoiceOptions.voice_pitch}
                    onChange={(e) => updateEpisodeVoiceOverride(ep.id, 'voice_pitch', e.target.value)}
                    className="h-8 rounded-md border border-surface-200 bg-white px-2 text-xs"
                    title="本集音调"
                  >
                    {VOICE_PITCH_OPTIONS.map(v => (
                      <option key={v.value} value={v.value}>{v.label}</option>
                    ))}
                  </select>
                  <select
                    value={episodeVoiceOptions.voice_volume}
                    onChange={(e) => updateEpisodeVoiceOverride(ep.id, 'voice_volume', e.target.value)}
                    className="h-8 rounded-md border border-surface-200 bg-white px-2 text-xs"
                    title="本集音量"
                  >
                    {VOICE_VOLUME_OPTIONS.map(v => (
                      <option key={v.value} value={v.value}>{v.label}</option>
                    ))}
                  </select>
                </div>

                <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
                  {/* Dubbing */}
                  {project.enable_dubbing && (
                    <div className="rounded-md bg-blue-50 px-3 py-2">
                      <div className="flex items-center justify-between">
                        <div className="flex items-center gap-2">
                          <Mic className="h-3.5 w-3.5 text-blue-500" />
                          <span className="text-xs font-medium text-blue-700">配音</span>
                          {dubTask?.status === 'succeeded' ? (
                            <span className="rounded bg-blue-100 px-1.5 py-0.5 text-[10px] text-blue-600">可播放</span>
                          ) : dubTask?.status === 'processing' || dubTask?.status === 'pending' ? (
                            <span className="flex items-center gap-1 rounded bg-amber-100 px-1.5 py-0.5 text-[10px] text-amber-600">
                              <Loader2 className="h-2.5 w-2.5 animate-spin" />
                              {dubTask.chunks_total > 0 ? `${dubTask.chunks_done}/${dubTask.chunks_total}` : '等待中'}
                            </span>
                          ) : dubTask?.status === 'failed' ? (
                            <span className="rounded bg-red-100 px-1.5 py-0.5 text-[10px] text-red-600" title={dubTask.error_msg}>失败</span>
                          ) : (
                            <span className="text-[10px] text-surface-400">待生成</span>
                          )}
                        </div>
                        <div className="flex items-center gap-1">
                          {dubTask?.audio_url && (
                            <Button size="sm" variant="ghost" className="h-7 w-7 p-0" onClick={() => setExpandedEp(expandedEp === ep.id ? null : ep.id)} title="展开预览">
                              <Eye className="h-3.5 w-3.5" />
                            </Button>
                          )}
                          <Button
                            size="sm"
                            variant="ghost"
                            className="h-7 px-2 text-xs"
                            disabled={dubTask?.status === 'processing' || dubTask?.status === 'pending' || batchDubbing || !getDubbingSubmitText(ep)}
                            onClick={async () => {
                              try {
                                await dubbingAPI.generate(projectId, ep.id, getDubbingSubmitText(ep), episodeVoiceOptions.voice_model, episodeVoiceOptions)
                                toast({ title: `第 ${ep.episode_number} 集配音已提交` })
                                mutateTasks()
                              } catch (err) {
                                toast({ title: getTaskSubmitError(err, '配音'), variant: 'destructive' })
                              }
                            }}
                            title={!getDubbingSubmitText(ep) ? '请输入配音文本' : (dubTask?.audio_url ? '重新生成本集配音' : '为本集生成配音')}
                          >
                            {dubTask?.status === 'processing' || dubTask?.status === 'pending'
                              ? <Loader2 className="h-3.5 w-3.5 animate-spin" />
                              : <Sparkles className="mr-1 h-3.5 w-3.5" />}
                            {dubTask?.status !== 'processing' && dubTask?.status !== 'pending' && (dubTask?.audio_url ? '重新生成' : '生成')}
                          </Button>
                        </div>
                      </div>
                      {/* Progress bar for this episode */}
                      {(dubTask?.status === 'processing' || dubTask?.status === 'pending') && (
                        <div className="mt-2 h-1.5 overflow-hidden rounded-full bg-blue-100">
                          <div
                            className="h-full rounded-full bg-blue-400 transition-all duration-300"
                            style={{ width: `${dubTask.chunks_total > 0 ? Math.max((dubTask.chunks_done / dubTask.chunks_total) * 100, dubTask.status === 'pending' ? 8 : 0) : 8}%` }}
                          />
                        </div>
                      )}
                      {dubTask?.status === 'processing' && getProgressStallMeta(dubTask.updated_at) && (
                        <div className="mt-2 flex flex-wrap items-center justify-between gap-2 rounded-md border border-red-200 bg-red-50 px-2.5 py-2 text-[11px] text-red-700">
                          <span>当前配音任务长时间未更新，可能卡住了。</span>
                          <Button size="sm" variant="outline" className="h-6 border-red-300 bg-white px-2 text-[11px] text-red-700 hover:bg-red-100" onClick={() => handleRetryTask(dubTask, '配音')} disabled={retryingTaskIds.includes(dubTask.id)}>
                            {retryingTaskIds.includes(dubTask.id) ? <Loader2 className="mr-1 h-3 w-3 animate-spin" /> : <RefreshCw className="mr-1 h-3 w-3" />}
                            重试
                          </Button>
                        </div>
                      )}
                      {dubTask?.status === 'pending' && getPendingQueueMeta(dubTask.updated_at) && (
                        <div className="mt-2 rounded-md border border-amber-200 bg-amber-50 px-2.5 py-2 text-[11px] text-amber-700">
                          当前配音任务仍在排队等待处理，并非异常卡住。
                        </div>
                      )}
                      <div className="mt-2 space-y-2">
                        {dubTask && (
                          <p className="text-[10px] text-surface-400">
                            当前参数：{VOICE_OPTIONS.find(v => v.value === dubTask.voice_model)?.label || '默认音色'} · {formatVoiceSettings(dubTask)}
                          </p>
                        )}
                        <div className="flex items-center justify-between">
                          <p className="text-[10px] text-surface-400">配音文案（可调整后重新生成）</p>
                          <div className="flex items-center gap-2">
                            {!getEpisodeBaseText(ep) && (
                              <button
                                type="button"
                                className="text-[10px] text-violet-500 hover:underline disabled:opacity-50"
                                disabled={aggregatingDialogues[ep.id]}
                                onClick={() => handleAggregateDialogues(ep.id)}
                                title="从该集分镜台词中聚合文本"
                              >
                                {aggregatingDialogues[ep.id] ? '提取中...' : '从分镜台词聚合'}
                              </button>
                            )}
                            <button
                              type="button"
                              className="text-[10px] text-blue-500 hover:underline"
                              onClick={() => setDubbingDrafts((prev) => ({ ...prev, [ep.id]: getEpisodeBaseText(ep) }))}
                            >
                              恢复原文
                            </button>
                          </div>
                        </div>
                        <Textarea
                          value={dubbingDrafts[ep.id] ?? getEpisodeBaseText(ep)}
                          onChange={(e) => setDubbingDrafts((prev) => ({ ...prev, [ep.id]: e.target.value }))}
                          rows={3}
                          className="min-h-[72px] border-blue-100 bg-white/80 text-xs"
                          placeholder="输入本集配音文案"
                        />
                      </div>
                      {/* Inline audio player */}
                      {dubTask?.audio_url && expandedEp === ep.id && (
                        <div className="mt-2">
                          <audio controls className="w-full h-8" src={dubTask.audio_url} />
                          <a href={dubTask.audio_url} target="_blank" rel="noopener noreferrer" className="mt-1 inline-flex items-center gap-1 text-[10px] text-blue-500 hover:underline">
                            <Download className="h-3 w-3" /> 下载音频
                          </a>
                        </div>
                      )}
                    </div>
                  )}

                  {/* Subtitle */}
                  {project.enable_subtitle && (
                    <div className="rounded-md bg-green-50 px-3 py-2">
                      <div className="flex items-center justify-between">
                        <div className="flex items-center gap-2">
                          <FileText className="h-3.5 w-3.5 text-green-500" />
                          <span className="text-xs font-medium text-green-700">字幕</span>
                          {subTask?.status === 'succeeded' ? (
                            <span className="rounded bg-green-100 px-1.5 py-0.5 text-[10px] text-green-600">已生成</span>
                          ) : subTask?.status === 'processing' || subTask?.status === 'pending' ? (
                            <span className="flex items-center gap-1 rounded bg-amber-100 px-1.5 py-0.5 text-[10px] text-amber-600">
                              <Loader2 className="h-2.5 w-2.5 animate-spin" />
                              {subTask.chunks_total > 0 ? `${subTask.chunks_done}/${subTask.chunks_total}` : '等待中'}
                            </span>
                          ) : subTask?.status === 'failed' ? (
                            <span className="rounded bg-red-100 px-1.5 py-0.5 text-[10px] text-red-600" title={subTask.error_msg}>失败</span>
                          ) : (
                            <span className="text-[10px] text-surface-400">待生成</span>
                          )}
                        </div>
                        <div className="flex items-center gap-1">
                          {subTask?.subtitle_url && (
                            <Button size="sm" variant="ghost" className="h-7 w-7 p-0" onClick={async () => {
                              if (expandedEp === ep.id && subtitleTexts[ep.id]) {
                                setExpandedEp(null)
                                return
                              }
                              setExpandedEp(ep.id)
                              if (!subtitleTexts[ep.id] && subTask.subtitle_url) {
                                setLoadingSubtitle(ep.id)
                                try {
                                  const resp = await fetch(subTask.subtitle_url)
                                  if (!resp.ok) throw new Error(`HTTP ${resp.status}`)
                                  const text = await resp.text()
                                  setSubtitleTexts(prev => ({ ...prev, [ep.id]: text }))
                                } catch (e) {
                                  console.error('[subtitle] fetch failed', e)
                                  setSubtitleTexts(prev => ({ ...prev, [ep.id]: '字幕加载失败，请刷新重试' }))
                                }
                                finally { setLoadingSubtitle(null) }
                              }
                            }} title="查看字幕">
                              <Eye className="h-3.5 w-3.5" />
                            </Button>
                          )}
                          <Button
                            size="sm"
                            variant="ghost"
                            className="h-7 px-2 text-xs"
                            disabled={subTask?.status === 'processing' || subTask?.status === 'pending' || batchSubtitle || !getSubtitleSubmitText(ep)}
                            onClick={async () => {
                              try {
                                await dubbingAPI.generateSubtitle(projectId, ep.id, getSubtitleSubmitText(ep), episodeVoiceOptions)
                                toast({ title: `第 ${ep.episode_number} 集字幕已提交` })
                                mutateTasks()
                              } catch (err) {
                                toast({ title: getTaskSubmitError(err, '字幕'), variant: 'destructive' })
                              }
                            }}
                            title={!getSubtitleSubmitText(ep) ? '请输入字幕文本' : (subTask?.subtitle_url ? '重新生成本集字幕' : '为本集生成字幕')}
                          >
                            {subTask?.status === 'processing' || subTask?.status === 'pending'
                              ? <Loader2 className="h-3.5 w-3.5 animate-spin" />
                              : <Sparkles className="mr-1 h-3.5 w-3.5" />}
                            {subTask?.status !== 'processing' && subTask?.status !== 'pending' && (subTask?.subtitle_url ? '重新生成' : '生成')}
                          </Button>
                        </div>
                      </div>
                      {/* Progress bar for subtitle */}
                      {(subTask?.status === 'processing' || subTask?.status === 'pending') && (
                        <div className="mt-2 h-1.5 overflow-hidden rounded-full bg-green-100">
                          <div
                            className="h-full rounded-full bg-green-400 transition-all duration-300"
                            style={{ width: `${subTask.chunks_total > 0 ? Math.max((subTask.chunks_done / subTask.chunks_total) * 100, subTask.status === 'pending' ? 8 : 0) : 8}%` }}
                          />
                        </div>
                      )}
                      {subTask?.status === 'processing' && getProgressStallMeta(subTask.updated_at) && (
                        <div className="mt-2 flex flex-wrap items-center justify-between gap-2 rounded-md border border-red-200 bg-red-50 px-2.5 py-2 text-[11px] text-red-700">
                          <span>当前字幕任务长时间未更新，可能卡住了。</span>
                          <Button size="sm" variant="outline" className="h-6 border-red-300 bg-white px-2 text-[11px] text-red-700 hover:bg-red-100" onClick={() => handleRetryTask(subTask, '字幕')} disabled={retryingTaskIds.includes(subTask.id)}>
                            {retryingTaskIds.includes(subTask.id) ? <Loader2 className="mr-1 h-3 w-3 animate-spin" /> : <RefreshCw className="mr-1 h-3 w-3" />}
                            重试
                          </Button>
                        </div>
                      )}
                      {subTask?.status === 'pending' && getPendingQueueMeta(subTask.updated_at) && (
                        <div className="mt-2 rounded-md border border-amber-200 bg-amber-50 px-2.5 py-2 text-[11px] text-amber-700">
                          当前字幕任务仍在排队等待处理，并非异常卡住。
                        </div>
                      )}
                      {subTask && (
                        <p className="mt-2 text-[10px] text-surface-400">
                          当前参数：{VOICE_OPTIONS.find(v => v.value === subTask.voice_model)?.label || '默认音色'} · {formatVoiceSettings(subTask)}
                        </p>
                      )}
                      <div className="mt-2 space-y-2">
                        <div className="flex items-center justify-between">
                          <p className="text-[10px] text-surface-400">字幕文案（可调整后重新生成）</p>
                          <div className="flex items-center gap-2">
                            {!getEpisodeBaseText(ep) && (
                              <button
                                type="button"
                                className="text-[10px] text-violet-500 hover:underline disabled:opacity-50"
                                disabled={aggregatingDialogues[ep.id]}
                                onClick={() => handleAggregateDialogues(ep.id)}
                                title="从该集分镜台词中聚合文本"
                              >
                                {aggregatingDialogues[ep.id] ? '提取中...' : '从分镜台词聚合'}
                              </button>
                            )}
                            <button
                              type="button"
                              className="text-[10px] text-green-500 hover:underline"
                              onClick={() => setSubtitleDrafts((prev) => ({ ...prev, [ep.id]: getEpisodeBaseText(ep) }))}
                            >
                              恢复原文
                            </button>
                          </div>
                        </div>
                        <Textarea
                          value={subtitleDrafts[ep.id] ?? getEpisodeBaseText(ep)}
                          onChange={(e) => setSubtitleDrafts((prev) => ({ ...prev, [ep.id]: e.target.value }))}
                          rows={3}
                          className="min-h-[72px] border-green-100 bg-white/80 text-xs"
                          placeholder="输入本集字幕文案"
                        />
                      </div>
                      {/* Inline subtitle preview */}
                      {subTask?.subtitle_url && expandedEp === ep.id && (
                        <div className="mt-2">
                          {loadingSubtitle === ep.id ? (
                            <div className="flex items-center gap-1.5 text-[11px] text-surface-400"><Loader2 className="h-3 w-3 animate-spin" /> 加载中...</div>
                          ) : subtitleTexts[ep.id] ? (
                            <pre className="max-h-40 overflow-y-auto rounded bg-white/80 p-2 text-[11px] leading-relaxed text-surface-600 whitespace-pre-wrap">{subtitleTexts[ep.id]}</pre>
                          ) : null}
                          <a href={subTask.subtitle_url} target="_blank" rel="noopener noreferrer" className="mt-1 inline-flex items-center gap-1 text-[10px] text-green-500 hover:underline">
                            <Download className="h-3 w-3" /> 下载字幕 (VTT)
                          </a>
                        </div>
                      )}
                    </div>
                  )}
                </div>

                {/* Script preview */}
                {ep.script_excerpt && (
                  <div className="mt-2 rounded-md bg-surface-50 px-3 py-2">
                    <p className="line-clamp-2 text-[11px] text-surface-500">{ep.script_excerpt}</p>
                  </div>
                )}
              </div>
            )
          })}
        </div>
      )}
    </div>
  )
}
