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
  Bot,
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
import type { ImageModelOption } from '@/lib/model-display'

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
import { SerialSceneGroups } from '@/components/projects/serial/SerialSceneGroups'

type TabKey = WorkflowStepKey

// ── Reusable image model dropdown content ────────────────────────────────────
function ImageModelDropdownContent({
  options,
  availability,
  onSelect,
  label = '选择生成模型',
  align = 'end',
  showTags = false,
  stopPropagation = false,
}: {
  options: ImageModelOption[]
  availability: Record<string, boolean>
  onSelect: (key: string) => void
  label?: string
  align?: 'end' | 'start'
  showTags?: boolean
  stopPropagation?: boolean
}) {
  return (
    <DropdownMenuContent
      align={align}
      className="w-72 max-h-[70vh] overflow-y-auto"
      onClick={stopPropagation ? (e) => e.stopPropagation() : undefined}
    >
      <DropdownMenuLabel className="text-[10px] text-surface-400">{label}</DropdownMenuLabel>
      <DropdownMenuSeparator />
      {options.map((m, idx) => {
        const avail = availability[m.key]
        const broken = !!m.failureReason
        return (
          <DropdownMenuItem
            key={m.key}
            disabled={broken}
            title={broken ? `已停用：${m.failureReason}` : undefined}
            className={`cursor-pointer px-3 py-2 ${avail === false ? 'opacity-50' : ''} ${broken ? 'cursor-not-allowed opacity-40' : ''}`}
            onClick={() => broken ? undefined : onSelect(m.key)}
          >
            <div className="flex w-full items-start gap-2">
              <div className="mt-0.5 flex flex-col items-center gap-0.5">
                <span className="text-sm">{m.icon}</span>
                <span className="rounded-full bg-surface-200 px-1 text-[8px] text-surface-500 font-bold">#{idx + 1}</span>
              </div>
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-1.5 flex-wrap">
                  <span className="text-xs font-semibold">{m.label}</span>
                  {!broken && m.speed === 'fast' && <span className="rounded bg-green-100 px-1 py-0 text-[9px] text-green-700">⚡ 快</span>}
                  {!broken && m.quality === 'high' && <span className="rounded bg-blue-100 px-1 py-0 text-[9px] text-blue-700">★ 高质</span>}
                  {!broken && avail === true && <span className="rounded bg-emerald-100 px-1 py-0 text-[9px] text-emerald-700">● 可用</span>}
                  {!broken && avail === false && <span className="rounded bg-red-100 px-1 py-0 text-[9px] text-red-600">● 未配置</span>}
                  {broken && <span className="rounded bg-red-100 px-1 py-0 text-[9px] text-red-600">⚠ 已停用</span>}
                </div>
                <p className="text-[10px] text-surface-400 leading-none mt-0.5">{getProviderLabel(m.provider)}</p>
                {broken ? (
                  <p className="mt-0.5 text-[9px] text-red-400 leading-tight">{m.failureReason}</p>
                ) : (
                  <>
                    <p className="mt-0.5 text-[10px] text-surface-500 leading-tight">{m.desc}</p>
                    {showTags && (
                      <div className="mt-1 flex flex-wrap gap-1">
                        {m.tags.map(t => (
                          <span key={t} className="rounded-full bg-surface-100 px-1.5 py-0 text-[9px] text-surface-500">{t}</span>
                        ))}
                      </div>
                    )}
                  </>
                )}
              </div>
            </div>
          </DropdownMenuItem>
        )
      })}
    </DropdownMenuContent>
  )
}

export function StoryboardTab({ projectId, project, episodeId, onExtractStoryboards, isExtractingStoryboards, awaitingAutoStoryboard, storyboardButtonDisabled, hideActionBar, sbGenerateTrigger, sbRegenerateTrigger, sbPauseTrigger, sbResumeTrigger, sbAuditTrigger }: { projectId: number; project: Project ; episodeId?: number; onExtractStoryboards?: () => void; isExtractingStoryboards?: boolean; awaitingAutoStoryboard?: boolean; storyboardButtonDisabled?: boolean; hideActionBar?: boolean; sbGenerateTrigger?: number; sbRegenerateTrigger?: number; sbPauseTrigger?: number; sbResumeTrigger?: number; sbAuditTrigger?: number }) {
  const { toast } = useToast()
  const isSerial = project.project_type === 'video_serial'
  const storyboardItemLabel = isSerial ? '镜头' : '分镜'
  const storyboardImageLabel = isSerial ? '首帧图片' : '分镜图片'
  const storyboardGenerateLabel = isSerial ? '首帧生成' : '分镜图片生成'
  const extractStoryboardLabel = isSerial ? '提取当前集镜头并分组' : '提取当前集镜头'
  const storyboardVideoLabel = isSerial ? '串行视频' : '视频'
  const { data: sbImageModelsData } = useSWR(
    ['storyboard-image-models', projectId],
    () => modelAPI.list({ type: 'image', sort_by: 'priority' }) as unknown as Promise<{ data: Model[] }>
  )
  const sbImageModels: Model[] = (sbImageModelsData as { data?: Model[] })?.data ?? []
  const sbProjectImageModelKey = sbImageModels.find(m => m.id === project.image_model_id)?.model_key ?? ''
  const { data: sbVideoModelsData } = useSWR(
    ['storyboard-video-models', projectId],
    () => modelAPI.list({ type: 'video', sort_by: 'priority' }) as unknown as Promise<{ data: Model[] }>
  )
  const sbVideoModels: Model[] = (sbVideoModelsData as { data?: Model[] })?.data ?? []
  // vtVideoModelOptions shared between video tab and storyboard dialog
  const vtVideoModelOptions = useMemo(
    () => dedupeModels(sbVideoModels.filter((m) => m.is_active)).map(buildVideoModelCapability),
    [sbVideoModels]
  )
  const sbSharedEpisode = useProjectEpisodeFilter()
  const [episodeFilter, setEpisodeFilter] = useState<string>(() => episodeId ? String(episodeId) : sbSharedEpisode.value)
  useEffect(() => {
    sbSharedEpisode.setValue(episodeFilter)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [episodeFilter])
  const [statusFilter, setStatusFilter] = useState<string>('all')
  const [keyword, setKeyword] = useState('')
  const [selectedSb, setSelectedSb] = useState<Storyboard | null>(null)
  const [versionIdx, setVersionIdx] = useState(0)
  const [chatInput, setChatInput] = useState('')
  const [chatLoading, setChatLoading] = useState(false)
  const [pausingGeneration, setPausingGeneration] = useState(false)
  const [resumingGeneration, setResumingGeneration] = useState(false)
  const [isAuditingContinuity, setIsAuditingContinuity] = useState(false)
  const [sbDescLang, setSbDescLang] = useState<'zh' | 'en'>('zh')
  const chatListRef = useRef<HTMLDivElement>(null)
  const chatBottomRef = useRef<HTMLDivElement>(null)
  const shouldStickChatToBottomRef = useRef(true)
  const [sbPage, setSbPage] = useState(1)
  const sbPageSize = 50
  const [imageModelAvailability, setImageModelAvailability] = useState<Record<string, boolean>>({})
  const [videoModelAvailability, setVideoModelAvailability] = useState<Record<string, boolean>>({})
  // ── Shared video model params state (used by storyboard dialog & video tab) ──
  const [videoModelParams, setVideoModelParams] = useState<Record<string, { key: string; label: string; default: string; values: { value: string; label: string }[] }[]>>({})
  const [videoParamSelections, setVideoParamSelections] = useState<Record<string, Record<string, string>>>({})
  const getModelParam = (modelKey: string, paramKey: string): string => {
    const sel = videoParamSelections[modelKey] ?? {}
    if (sel[paramKey]) return sel[paramKey]
    const param = (videoModelParams[modelKey] ?? []).find((p) => p.key === paramKey)
    return param?.default ?? ''
  }
  const setModelParam = (modelKey: string, paramKey: string, value: string) => {
    setVideoParamSelections((prev) => ({
      ...prev,
      [modelKey]: { ...(prev[modelKey] ?? {}), [paramKey]: value },
    }))
  }
  // Transition effect for storyboard episode video generation
  const [selectedEpisodeTransition, setSelectedEpisodeTransition] = useState('dissolve')
  const [selectedEpisodeTransitionDuration, setSelectedEpisodeTransitionDuration] = useState('0.5')
  React.useEffect(() => {
    assetAPI.modelStatus().then((res) => {
      const map: Record<string, boolean> = {}
      const models: { key: string; available: boolean }[] = (res as any)?.models ?? (res as any)?.data?.models ?? []
      models.forEach((m) => { map[m.key] = m.available })
      setImageModelAvailability(map)
    }).catch((e) => { console.warn('[assetAPI.modelStatus]', e) })
    videoAPI.modelStatus().then((res) => {
      const avail: Record<string, boolean> = {}
      const params: Record<string, { key: string; label: string; default: string; values: { value: string; label: string }[] }[]> = {}
      const vmodels = (res as any)?.models ?? (res as any)?.data?.models ?? []
      vmodels.forEach((m: { key: string; available: boolean; params?: { key: string; label: string; default: string; values: { value: string; label: string }[] }[] }) => {
        avail[m.key] = m.available
        if (m.params && m.params.length > 0) params[m.key] = m.params
      })
      setVideoModelAvailability(avail)
      setVideoModelParams(params)
    }).catch((e) => { console.warn('[videoAPI.modelStatus]', e) })
  }, [])
  useEffect(() => { setSbPage(1) }, [episodeFilter, statusFilter, keyword])

  // ── Stats polling for progress bar ──
  type StatsData = { total: number; pending: number; generating: number; paused: number; completed: number; failed: number; voided: number }
  const { data: statsRaw, mutate: mutateStats } = useSWR(
    ['storyboard-stats', projectId],
    () => storyboardAPI.stats(projectId) as unknown as Promise<{ data: StatsData }>,
    { refreshInterval: (data) => {
        const s = (data as { data?: StatsData } | undefined)?.data
        if (!s) return 5000
        return (s.generating > 0 || s.pending > 0) ? 5000 : 30000
      } }
  )
  const stats: StatsData = (statsRaw as { data?: StatsData })?.data ?? { total: 0, pending: 0, generating: 0, paused: 0, completed: 0, failed: 0, voided: 0 }
  const isActive = stats.generating > 0

  const { data: storyboardAssetsRaw } = useSWR(
    ['storyboard-assets', projectId],
    () => assetAPI.list(projectId) as unknown as Promise<{ data: Asset[] }>,
    { refreshInterval: isActive ? 5000 : 30000 }
  )
  const storyboardAssets = ((storyboardAssetsRaw as { data?: Asset[] })?.data ?? []).filter((asset) => asset.name !== '__extracting__')
  // Scope the readiness check to the selected episode only (if one is selected),
  // so that other episodes' incomplete assets don't block the current episode's storyboard generation.
  const scopedStoryboardAssets = episodeFilter === 'all'
    ? storyboardAssets
    : storyboardAssets.filter((asset) => (asset.episode_ids ?? []).includes(Number(episodeFilter)))
  const storyboardAssetsPending = scopedStoryboardAssets.filter((asset) => asset.status === 'pending').length
  const storyboardAssetsGenerating = scopedStoryboardAssets.filter((asset) => asset.status === 'generating').length
  const storyboardAssetsPaused = scopedStoryboardAssets.filter((asset) => asset.status === 'paused').length
  const storyboardAssetsFailed = scopedStoryboardAssets.filter((asset) => asset.status === 'failed').length
  // Assets are optional — allow storyboard generation when no assets exist.
  // Failed assets are a terminal state and do NOT block storyboard generation
  // (mirrors backend ensureProjectAssetsReady in storyboard_handler.go);
  // otherwise a single failed asset would lock the user out of regenerating.
  const storyboardAssetsReady = scopedStoryboardAssets.length === 0 ||
    (storyboardAssetsPending === 0 && storyboardAssetsGenerating === 0 && storyboardAssetsPaused === 0)
  const storyboardAssetsBlockingReason = storyboardAssetsReady
    ? ''
    : `资源图尚未全部完成：待生成 ${storyboardAssetsPending}，生成中 ${storyboardAssetsGenerating}，已暂停 ${storyboardAssetsPaused}，失败 ${storyboardAssetsFailed}`
  const storyboardGenerateBlockedText = storyboardAssetsBlockingReason || `请先完成资源图生成后再开始${storyboardGenerateLabel}`
  const storyboardResumeBlockedText = storyboardAssetsBlockingReason || `请先完成资源图生成后再继续${storyboardGenerateLabel}`

  const { data: episodesData } = useSWR(
    ['episodes', projectId],
    () => projectAPI.listEpisodes(projectId) as unknown as Promise<{ data: Episode[] }>
  )
  const episodes = (episodesData as { data?: Episode[] })?.data ?? []

  type SbListResp = { data: Storyboard[] | { items: Storyboard[] }; page_info?: { page: number; page_size: number; total: number } }
  const { data: sbData, isLoading, mutate: mutateSb } = useSWR(
    ['storyboards', projectId, episodeFilter, statusFilter, keyword, sbPage],
    () => {
      const params: { episode_id?: number; status?: string; keyword?: string; page?: number; page_size?: number } = { page: sbPage, page_size: sbPageSize }
      if (episodeFilter !== 'all') params.episode_id = Number(episodeFilter)
      if (statusFilter !== 'all') params.status = statusFilter
      if (keyword.trim()) params.keyword = keyword.trim()
      return storyboardAPI.list(projectId, { ...params, include_versions: true }) as unknown as Promise<SbListResp>
    },
    {
      refreshInterval: () => {
        if (project.status === 'script_processing') return 3000
        if (isActive) return 5000
        const raw = (sbData as SbListResp)?.data
        const sbs: Storyboard[] = Array.isArray(raw) ? raw : (raw as { items?: Storyboard[] })?.items ?? []
        return sbs.some((s) => s.status === 'generating') ? 3000 : 0
      },
    }
  )
  // Per-episode completed counts — fetched independently of UI filters so the
  // "generate video for episode" button isn't incorrectly disabled by status/page filters.
  const { data: epStatsRaw } = useSWR(
    stats.total > 0 ? ['storyboard-episode-stats', projectId] : null,
    () => storyboardAPI.episodeStats(projectId) as unknown as Promise<{ data: Record<string, number> }>,
    { refreshInterval: isActive ? 5000 : 0 }
  )
  const episodeCompletedMap = useMemo(() => {
    const raw = (epStatsRaw as { data?: Record<string, number> })?.data ?? {}
    const m = new Map<number, number>()
    for (const [k, v] of Object.entries(raw)) m.set(Number(k), v)
    return m
  }, [epStatsRaw])
  const rawSb = (sbData as SbListResp)?.data
  const storyboards: Storyboard[] = Array.isArray(rawSb) ? rawSb : (rawSb as { items?: Storyboard[] })?.items ?? []
  const sbTotal = (sbData as SbListResp)?.page_info?.total ?? storyboards.length
  const sbTotalPages = Math.max(1, Math.ceil(sbTotal / sbPageSize))
  const selectedStoryboardVersion = selectedSb?.versions?.[versionIdx]
  const selectedStoryboardPreviewUrl = selectedStoryboardVersion?.image_url || selectedSb?.image_url || ''
  const selectedStoryboardMessageCount = selectedSb?.agent_history?.length ?? 0

  React.useEffect(() => {
    if (!selectedSb) return
    const updated = storyboards.find((sb) => sb.id === selectedSb.id)
    if (updated) setSelectedSb(updated)
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [storyboards])

  React.useEffect(() => {
    shouldStickChatToBottomRef.current = true
  }, [selectedSb?.id])

  React.useEffect(() => {
    if (!selectedSb || !shouldStickChatToBottomRef.current) return
    chatBottomRef.current?.scrollIntoView({ block: 'end' })
  }, [
    selectedSb?.id,
    selectedSb?.status,
    selectedSb?.agent_history?.length,
    selectedStoryboardPreviewUrl,
    chatLoading,
  ])

  React.useEffect(() => {
    if (!selectedSb) return
    const versions = selectedSb.versions ?? []
    if (versions.length === 0) {
      if (versionIdx !== 0) setVersionIdx(0)
      return
    }
    if (versionIdx > versions.length - 1) {
      setVersionIdx(versions.length - 1)
    }
  }, [selectedSb, versionIdx])

  const handleChatListScroll = () => {
    const el = chatListRef.current
    if (!el) return
    const distanceToBottom = el.scrollHeight - el.scrollTop - el.clientHeight
    shouldStickChatToBottomRef.current = distanceToBottom <= 96
  }

  const handleCreateFromEpisodes = async () => {
    try {
      for (const ep of episodes) {
        await storyboardAPI.create(projectId, {
          episode_id: ep.id,
          sequence_number: ep.episode_number,
          scene_description: ep.summary || ep.title,
          duration: ep.estimated_duration || 4,
        })
      }
      toast({ title: `已创建 ${episodes.length} 条${storyboardItemLabel}`, variant: 'success' })
      mutateSb()
    } catch {
      toast({ title: '创建失败', variant: 'destructive' })
    }
  }

  // ── Per-storyboard voice generation ──
  const [sbVoiceScope, setSbVoiceScope] = useState<'single' | 'episode'>('single')
  const [sbVoiceModel, setSbVoiceModel] = useState('default')
  const [sbVoiceRate, setSbVoiceRate] = useState('+0%')
  const [sbVoicePitch, setSbVoicePitch] = useState('+0Hz')
  const [sbVoiceVolume, setSbVoiceVolume] = useState('+0%')
  const [generatingSbVoice, setGeneratingSbVoice] = useState(false)

  const { data: voicesDataSb } = useSWR(
    'voices',
    () => dubbingAPI.listVoices().then((r) => (r as { data?: { voices?: { key: string; label: string }[] } }).data?.voices ?? null),
    { revalidateOnFocus: false, revalidateOnReconnect: false }
  )
  const SB_VOICE_OPTIONS = [
    { value: 'default', label: '默认音色' },
    { value: 'auto', label: '自动按人物分配' },
    ...(voicesDataSb ?? FALLBACK_VOICE_OPTIONS).map((v) => {
      const key = (v as { key?: string }).key ?? (v as { value?: string }).value ?? ''
      return { value: key, label: v.label }
    }),
  ]

  // Poll per-storyboard dubbing tasks to show status/audio inline
  const { data: sbTasksData, mutate: mutateStoryboardTasks } = useSWR(
    project.enable_dubbing ? ['storyboard-dubbing-tasks', projectId] : null,
    () => dubbingAPI.listStoryboardTasks(projectId).then(r => r.data ?? []),
    {
      refreshInterval: (data) => {
        const tasks = Array.isArray(data) ? data as DubbingTask[] : []
        return tasks.some(t => t.status === 'processing' || t.status === 'pending') ? 3000 : 15000
      },
    }
  )
  const storyboardTaskMap = useMemo(() => {
    const map = new Map<number, DubbingTask>()
    for (const t of (sbTasksData as DubbingTask[] | undefined) ?? []) {
      if (t.storyboard_id != null) map.set(t.storyboard_id, t)
    }
    return map
  }, [sbTasksData])

  const handleSbGenerateVoice = async () => {
    if (!selectedSb || !selectedSb.episode_id) return
    setGeneratingSbVoice(true)
    try {
      if (sbVoiceScope === 'single') {
        const text = selectedSb.dialogue?.trim() || ''
        if (!text) {
          toast({ title: '该分镜暂无台词，无法生成语音', variant: 'destructive' })
          return
        }
        await dubbingAPI.generateForStoryboard(projectId, selectedSb.id, selectedSb.episode_id, text, sbVoiceModel, {
          voice_rate: sbVoiceRate,
          voice_pitch: sbVoicePitch,
          voice_volume: sbVoiceVolume,
        })
        mutateStoryboardTasks()
        toast({ title: '单帧语音任务已提交', variant: 'success' })
      } else {
        const allSbsRes = await storyboardAPI.listAll(projectId, { episode_id: selectedSb.episode_id }) as { data?: Storyboard[] }
        const allDialogues = (allSbsRes?.data ?? [])
          .sort((a, b) => a.sequence_number - b.sequence_number)
          .map((sb) => sb.dialogue || '')
          .filter(Boolean)
        const text = allDialogues.join('\n')
        if (!text.trim()) {
          toast({ title: '当前集暂无台词，无法生成语音', variant: 'destructive' })
          return
        }
        await dubbingAPI.generate(projectId, selectedSb.episode_id, text, sbVoiceModel, {
          voice_rate: sbVoiceRate,
          voice_pitch: sbVoicePitch,
          voice_volume: sbVoiceVolume,
        })
        toast({ title: '全集语音任务已提交', variant: 'success' })
      }
    } catch (err) {
      const status = (err as { response?: { status?: number } })?.response?.status
      toast({
        title: status === 409 ? '当前分镜/集已有进行中的语音任务' : '语音生成提交失败',
        variant: 'destructive',
      })
    } finally {
      setGeneratingSbVoice(false)
    }
  }

  // ── Generate video for a single episode ──
  const [generatingVideoEps, setGeneratingVideoEps] = useState<Set<number>>(new Set())
  const [videoDialogEpisodeId, setVideoDialogEpisodeId] = useState<number | null>(null)

  // Build from API data — only active video models, ordered by backend priority
  const VIDEO_MODEL_OPTIONS = useMemo(
    () => dedupeModels([
      ...sbVideoModels.filter((m) => m.is_active && m.model_key),
      ...sbVideoModels.filter((m) => !m.is_active && m.failure_reason && m.model_key),
    ]).map(buildVideoModelOption),
    [sbVideoModels]
  )
  const VIDEO_STYLE_LABELS: Record<string, string> = {
    'anime-2d': '2维动漫',
    'anime-3d': '3维动漫',
    'live-action-film': '真人电影',
    'live-action-short': '真人短剧',
  }
  const VIDEO_MOTION_LABELS = {
    gentle: '柔和',
    dynamic: '动感',
    cinematic: '电影感',
  } as const
  const VIDEO_FRAME_SIZE_OPTIONS = [
    { key: 'portrait-9-16', label: '竖屏 9:16', desc: '适合短视频、手机全屏与人物主体。' },
    { key: 'landscape-16-9', label: '横屏 16:9', desc: '适合剧情镜头、横版预告与通用视频。' },
    { key: 'square-1-1', label: '方形 1:1', desc: '适合封面感画面与居中构图。' },
    { key: 'ultrawide-21-9', label: '宽银幕 21:9', desc: '适合史诗场景与电影化大场面。' },
  ] as const
  const VIDEO_SUBJECT_SIZE_OPTIONS = [
    { key: 'close-up', label: '特写 / 大主体', desc: '人物或主体更大，强调表情与细节。' },
    { key: 'medium-shot', label: '中景 / 平衡', desc: '主体与环境平衡，更适合常规叙事。' },
    { key: 'wide-shot', label: '远景 / 大场景', desc: '主体相对更小，突出场景与空间关系。' },
  ] as const
  const VIDEO_CLARITY_OPTIONS = [
    { key: 'standard', label: '标准清晰', desc: '细节自然，适合常规生成。' },
    { key: 'high', label: '高清细节', desc: '更清楚的边缘与材质层次。' },
    { key: 'ultra', label: '超清锐利', desc: '尽量强化精细纹理和清晰度。' },
  ] as const
  const [selectedEpisodeVideoModel, setSelectedEpisodeVideoModel] = useState<string>('wan')
  const [selectedEpisodeVideoStyle, setSelectedEpisodeVideoStyle] = useState('anime-2d')
  const [selectedEpisodeVideoMotionMode, setSelectedEpisodeVideoMotionMode] = useState<keyof typeof VIDEO_MOTION_LABELS>('gentle')
  const [selectedEpisodeVideoFrameSize, setSelectedEpisodeVideoFrameSize] = useState<(typeof VIDEO_FRAME_SIZE_OPTIONS)[number]['key']>('landscape-16-9')
  const [selectedEpisodeVideoSubjectSize, setSelectedEpisodeVideoSubjectSize] = useState<(typeof VIDEO_SUBJECT_SIZE_OPTIONS)[number]['key']>('medium-shot')
  const [selectedEpisodeVideoClarity, setSelectedEpisodeVideoClarity] = useState<(typeof VIDEO_CLARITY_OPTIONS)[number]['key']>('high')
  const selectedEpisodeVideoModelMeta = vtVideoModelOptions.find((item) => item.key === selectedEpisodeVideoModel) ?? vtVideoModelOptions[0] ?? { key: '', label: '', desc: '', icon: '', provider: '', audioSupport: 'none' as const, aspectRatio: 'unsupported' as const, resolution: 'unsupported' as const, multiVariant: 'unsupported' as const, clipDuration: '', note: '', tags: [], speed: 'medium' as const, quality: 'standard' as const, bestFor: [] }
  const selectedEpisodeVideoStyleMeta = VIDEO_STYLE_COMPACT_OPTIONS.find((item) => item.key === selectedEpisodeVideoStyle) ?? VIDEO_STYLE_COMPACT_OPTIONS[0]
  const selectedEpisodeVideoStyleLabel = VIDEO_STYLE_LABELS[selectedEpisodeVideoStyle] ?? selectedEpisodeVideoStyle
  const selectedEpisodeVideoMotionLabel = VIDEO_MOTION_LABELS[selectedEpisodeVideoMotionMode] ?? selectedEpisodeVideoMotionMode
  const selectedEpisodeVideoModeLabel = project.video_mode === 'api_generation' ? 'API生成' : '逐帧动画'
  const selectedEpisodeVideoFrameSizeMeta = VIDEO_FRAME_SIZE_OPTIONS.find((item) => item.key === selectedEpisodeVideoFrameSize) ?? VIDEO_FRAME_SIZE_OPTIONS[0]
  const selectedEpisodeVideoSubjectSizeMeta = VIDEO_SUBJECT_SIZE_OPTIONS.find((item) => item.key === selectedEpisodeVideoSubjectSize) ?? VIDEO_SUBJECT_SIZE_OPTIONS[0]
  const selectedEpisodeVideoClarityMeta = VIDEO_CLARITY_OPTIONS.find((item) => item.key === selectedEpisodeVideoClarity) ?? VIDEO_CLARITY_OPTIONS[0]
  const selectedVideoDialogEpisode = videoDialogEpisodeId ? episodes.find((episode) => episode.id === videoDialogEpisodeId) ?? null : null
  const episodeVideoModelStorageKey = `project-video-model-selection:${projectId}`
  const episodeVideoStyleStorageKey = `project-video-style-selection:${projectId}`
  const episodeVideoMotionStorageKey = `project-video-motion-selection:${projectId}`
  const projectConfiguredVideoStyle = normalizeVideoStylePreset(
    typeof project.storyboard_config?.style_preset === 'string' ? project.storyboard_config.style_preset : ''
  )
  const projectConfiguredVideoMotion =
    typeof project.storyboard_config?.motion_mode === 'string' ? project.storyboard_config.motion_mode : ''

  const syncEpisodeVideoSelections = () => {
    if (typeof window === 'undefined') return
    try {
      const savedModel = window.localStorage.getItem(episodeVideoModelStorageKey)
      if (savedModel && vtVideoModelOptions.some((item) => item.key === savedModel)) {
        setSelectedEpisodeVideoModel(savedModel)
      }
      const savedStyle = normalizeVideoStylePreset(window.localStorage.getItem(episodeVideoStyleStorageKey) ?? '')
      if (VIDEO_STYLE_COMPACT_OPTIONS.some((style) => style.key === savedStyle)) {
        setSelectedEpisodeVideoStyle(savedStyle)
      } else if (VIDEO_STYLE_COMPACT_OPTIONS.some((style) => style.key === projectConfiguredVideoStyle)) {
        setSelectedEpisodeVideoStyle(projectConfiguredVideoStyle)
      }
      const savedMotion = window.localStorage.getItem(episodeVideoMotionStorageKey)
      if (savedMotion && savedMotion in VIDEO_MOTION_LABELS) {
        setSelectedEpisodeVideoMotionMode(savedMotion as keyof typeof VIDEO_MOTION_LABELS)
      } else if (projectConfiguredVideoMotion && projectConfiguredVideoMotion in VIDEO_MOTION_LABELS) {
        setSelectedEpisodeVideoMotionMode(projectConfiguredVideoMotion as keyof typeof VIDEO_MOTION_LABELS)
      }
    } catch {}
  }

  useEffect(() => {
    syncEpisodeVideoSelections()
  }, [projectConfiguredVideoMotion, projectConfiguredVideoStyle, projectId])

  useEffect(() => {
    if (typeof window === 'undefined') return
    try {
      window.localStorage.setItem(episodeVideoModelStorageKey, selectedEpisodeVideoModel)
      window.localStorage.setItem(episodeVideoStyleStorageKey, selectedEpisodeVideoStyle)
      window.localStorage.setItem(episodeVideoMotionStorageKey, selectedEpisodeVideoMotionMode)
    } catch {}
  }, [
    episodeVideoModelStorageKey,
    episodeVideoMotionStorageKey,
    episodeVideoStyleStorageKey,
    selectedEpisodeVideoModel,
    selectedEpisodeVideoMotionMode,
    selectedEpisodeVideoStyle,
  ])

  const applyEpisodeVideoPreset = (presetKey: string) => {
    const preset = VIDEO_GENERATION_PRESETS.find((item) => item.key === presetKey)
    if (!preset) return
    if (vtVideoModelOptions.some((item) => item.key === preset.model)) {
      setSelectedEpisodeVideoModel(preset.model)
    }
    setSelectedEpisodeVideoStyle(preset.style)
    setSelectedEpisodeVideoMotionMode(preset.motion)
  }

  const handleGenerateVideoByEpisode = async (
    episodeId: number,
    options?: {
      modelName?: string
      frameSize?: (typeof VIDEO_FRAME_SIZE_OPTIONS)[number]['key']
      subjectSize?: (typeof VIDEO_SUBJECT_SIZE_OPTIONS)[number]['key']
      clarity?: (typeof VIDEO_CLARITY_OPTIONS)[number]['key']
    }
  ) => {
    const completedSbs = ((await storyboardAPI.listAll(projectId, { episode_id: episodeId, status: 'completed' })) as { data?: Storyboard[] }).data ?? []
    const isSerialProject = project.project_type === 'video_serial'
    const sortedSbs = completedSbs
      .filter((sb) => sb.image_url || (isSerialProject && sb.scene_group_key))
      .sort((a, b) => a.sequence_number - b.sequence_number)
    const imageUrls = sortedSbs.map((sb) => sb.image_url)
    // Prefer LLM-refined prompt_used; fall back to raw scene_description for older storyboards.
    const sceneDescriptions = sortedSbs.map((sb) => sb.prompt_used || sb.scene_description || '')
    const dialogues = sortedSbs.map((sb) => sb.dialogue || '')
    const durations = sortedSbs.map((sb) => sb.duration || 0)
    const cameraMovements = sortedSbs.map((sb) => sb.camera_movement || '')
    const moods = sortedSbs.map((sb) => sb.mood || '')
    const sceneCharacters = sortedSbs.map((sb) => sb.characters || [])
    const sceneAssetIds = sortedSbs.map((sb) => sb.asset_ids || [])
    const sceneDescription = sceneDescriptions.filter(Boolean).join(' ')
    const sceneGroupKeys = sortedSbs.map((sb) => sb.scene_group_key || '')
    const isSerialScene = isSerialProject || sceneGroupKeys.some(Boolean)

    if (!imageUrls.some(Boolean)) {
      toast({ title: isSerialProject ? '此集暂无可用首帧图片' : '此集暂无已完成的分镜图片', variant: 'destructive' })
      return
    }

    setGeneratingVideoEps(prev => new Set(prev).add(episodeId))
    try {
      const modelName = options?.modelName || 'wan'
      await videoAPI.generate(projectId, {
        episode_id: episodeId,
        image_urls: imageUrls,
        scene_descriptions: sceneDescriptions,
        dialogues: dialogues.some(Boolean) ? dialogues : undefined,
        durations: durations.some(Boolean) ? durations : undefined,
        camera_movements: cameraMovements.some(Boolean) ? cameraMovements : undefined,
        moods: moods.some(Boolean) ? moods : undefined,
        scene_characters: sceneCharacters.some((arr) => arr.length > 0) ? sceneCharacters : undefined,
        scene_asset_ids: sceneAssetIds.some((arr) => arr.length > 0) ? sceneAssetIds : undefined,
        model_name: modelName,
        style_preset: selectedEpisodeVideoStyle,
        motion_mode: selectedEpisodeVideoMotionMode,
        video_mode: project.video_mode,
        scene_description: sceneDescription || undefined,
        clip_duration_sec: (() => {
          const durSel = videoParamSelections[modelName]?.duration
          if (durSel) return parseFloat(durSel)
          return project.storyboard_config?.duration || 5
        })(),
        serial_scene: isSerialScene || undefined,
        scene_group_keys: isSerialScene && sceneGroupKeys.some(Boolean) ? sceneGroupKeys : undefined,
        render_config: {
          frame_size: options?.frameSize || selectedEpisodeVideoFrameSize,
          subject_size: options?.subjectSize || selectedEpisodeVideoSubjectSize,
          clarity: options?.clarity || selectedEpisodeVideoClarity,
          transition: selectedEpisodeTransition === 'none' ? undefined : selectedEpisodeTransition,
          transition_duration: selectedEpisodeTransition !== 'none' ? parseFloat(selectedEpisodeTransitionDuration) : undefined,
          ...(videoParamSelections[modelName] ?? {}),
        },
      })
      const ep = episodes.find(e => e.id === episodeId)
      const label = vtVideoModelOptions.find(m => m.key === modelName)?.label || modelName
      const frameLabel = VIDEO_FRAME_SIZE_OPTIONS.find((item) => item.key === (options?.frameSize || selectedEpisodeVideoFrameSize))?.label ?? (options?.frameSize || selectedEpisodeVideoFrameSize)
      const sizeLabel = VIDEO_SUBJECT_SIZE_OPTIONS.find((item) => item.key === (options?.subjectSize || selectedEpisodeVideoSubjectSize))?.label ?? (options?.subjectSize || selectedEpisodeVideoSubjectSize)
      const clarityLabel = VIDEO_CLARITY_OPTIONS.find((item) => item.key === (options?.clarity || selectedEpisodeVideoClarity))?.label ?? (options?.clarity || selectedEpisodeVideoClarity)
      toast({
        title: `第 ${ep?.episode_number ?? '?'} 集${storyboardVideoLabel}生成已启动（${label}）`,
        description: `${selectedEpisodeVideoStyleLabel} / ${selectedEpisodeVideoMotionLabel} / ${frameLabel} / ${sizeLabel} / ${clarityLabel} · ${isSerialProject ? `${sceneGroupKeys.filter(Boolean).length} 个场景组` : `${imageUrls.filter(Boolean).length} 张图`}`,
        variant: 'success',
      })
      return true
    } catch {
      toast({ title: '视频生成失败', variant: 'destructive' })
      return false
    } finally {
      setGeneratingVideoEps(prev => { const s = new Set(prev); s.delete(episodeId); return s })
    }
  }

  const openEpisodeVideoDialog = (episodeId: number) => {
    syncEpisodeVideoSelections()
    setVideoDialogEpisodeId(episodeId)
  }

  const handleConfirmEpisodeVideoGeneration = async () => {
    if (!videoDialogEpisodeId) return
    const ok = await handleGenerateVideoByEpisode(videoDialogEpisodeId, {
      modelName: selectedEpisodeVideoModel,
      frameSize: selectedEpisodeVideoFrameSize,
      subjectSize: selectedEpisodeVideoSubjectSize,
      clarity: selectedEpisodeVideoClarity,
    })
    if (ok) {
      setVideoDialogEpisodeId(null)
    }
  }

  // ── Generate videos for ALL episodes at once ──
  const [generatingAllVideos, setGeneratingAllVideos] = useState(false)

  const handleGenerateAllEpisodeVideos = async (modelName: string) => {
    const completedSbs = ((await storyboardAPI.listAll(projectId, { status: 'completed' })) as { data?: Storyboard[] }).data ?? []
    const isSerialProject = project.project_type === 'video_serial'
    const eligibleSbs = completedSbs
      .filter((sb) => sb.image_url || (isSerialProject && sb.scene_group_key))
      .sort((a, b) => a.sequence_number - b.sequence_number)
    if (eligibleSbs.length === 0 || !eligibleSbs.some((sb) => sb.image_url)) {
      toast({ title: isSerialProject ? '暂无可用场景首帧，请先完成首帧准备' : '暂无已完成的分镜图片，请先生成分镜图片', variant: 'destructive' })
      return
    }
    const byEpisode = new Map<number, string[]>()
    const byEpisodeDesc = new Map<number, string[]>()
    const byEpisodeDialogue = new Map<number, string[]>()
    const byEpisodeDuration = new Map<number, number[]>()
    const byEpisodeCamera = new Map<number, string[]>()
    const byEpisodeMood = new Map<number, string[]>()
    const byEpisodeChars = new Map<number, string[][]>()
    const byEpisodeAssetIds = new Map<number, number[][]>()
    const byEpisodeSceneGroupKeys = new Map<number, string[]>()
    for (const sb of eligibleSbs) {
      const epId = sb.episode_id ?? 0
      if (epId === 0) continue
      if (!byEpisode.has(epId)) {
        byEpisode.set(epId, []); byEpisodeDesc.set(epId, []); byEpisodeDialogue.set(epId, [])
        byEpisodeDuration.set(epId, []); byEpisodeCamera.set(epId, []); byEpisodeMood.set(epId, [])
        byEpisodeChars.set(epId, [])
        byEpisodeAssetIds.set(epId, [])
        byEpisodeSceneGroupKeys.set(epId, [])
      }
      byEpisode.get(epId)!.push(sb.image_url)
      byEpisodeDesc.get(epId)!.push(sb.prompt_used || sb.scene_description || '')
      byEpisodeDialogue.get(epId)!.push(sb.dialogue || '')
      byEpisodeDuration.get(epId)!.push(sb.duration || 0)
      byEpisodeCamera.get(epId)!.push(sb.camera_movement || '')
      byEpisodeMood.get(epId)!.push(sb.mood || '')
      byEpisodeChars.get(epId)!.push(sb.characters || [])
      byEpisodeAssetIds.get(epId)!.push(sb.asset_ids || [])
      byEpisodeSceneGroupKeys.get(epId)!.push(sb.scene_group_key || '')
    }
    if (byEpisode.size === 0) {
      toast({ title: `没有分配到集数的已完成${storyboardItemLabel}`, variant: 'destructive' })
      return
    }
    setGeneratingAllVideos(true)
    try {
      const isSerialScene = isSerialProject || eligibleSbs.some((sb) => sb.scene_group_key)
      const episodeBatch = Array.from(byEpisode.entries()).map(([epId, urls]) => {
        const dlgs = byEpisodeDialogue.get(epId) ?? []
        const durs = byEpisodeDuration.get(epId) ?? []
        const cams = byEpisodeCamera.get(epId) ?? []
        const mds = byEpisodeMood.get(epId) ?? []
        const chars = byEpisodeChars.get(epId) ?? []
        const assetIds = byEpisodeAssetIds.get(epId) ?? []
        const descs = byEpisodeDesc.get(epId) ?? []
        const sceneGroupKeys = byEpisodeSceneGroupKeys.get(epId) ?? []
        return {
          episode_id: epId,
          image_urls: urls,
          scene_descriptions: descs,
          dialogues: dlgs.some(Boolean) ? dlgs : undefined,
          durations: durs.some(Boolean) ? durs : undefined,
          camera_movements: cams.some(Boolean) ? cams : undefined,
          moods: mds.some(Boolean) ? mds : undefined,
          scene_characters: chars.some((arr) => arr.length > 0) ? chars : undefined,
          scene_asset_ids: assetIds.some((arr) => arr.length > 0) ? assetIds : undefined,
          scene_description: descs.filter(Boolean).join(' ') || undefined,
          scene_group_keys: isSerialScene && sceneGroupKeys.some(Boolean) ? sceneGroupKeys : undefined,
        }
      })
      await videoAPI.generateBatch(projectId, { episodes: episodeBatch, model_name: modelName, serial_scene: isSerialScene || undefined })
      toast({ title: `已启动 ${episodeBatch.length} 集${storyboardVideoLabel}生成（共 ${isSerialProject ? `${eligibleSbs.filter((sb) => sb.scene_group_key).length} 个场景链` : `${eligibleSbs.filter((sb) => sb.image_url).length} 张图`})`, variant: 'success' })
    } catch {
      toast({ title: `批量${storyboardVideoLabel}生成失败`, variant: 'destructive' })
    } finally {
      setGeneratingAllVideos(false)
    }
  }

  const handleGenerate = async (id: number) => {
    if (!storyboardAssetsReady) {
      toast({ title: storyboardGenerateBlockedText, variant: 'destructive' })
      return
    }
    try {
      await storyboardAPI.generate(projectId, id, sbProjectImageModelKey)
      toast({ title: `${storyboardGenerateLabel}已启动`, variant: 'success' })
      mutateSb()
    } catch {
      toast({ title: '生成失败', variant: 'destructive' })
    }
  }

  const handleForceGenerateEpisode = async (modelKey?: string) => {
    if (!storyboardAssetsReady) {
      toast({ title: storyboardGenerateBlockedText, variant: 'destructive' })
      return
    }
    const selectedEpisodeId = episodeFilter !== 'all' ? Number(episodeFilter) : undefined
    if (!selectedEpisodeId) return
    if (!window.confirm(`这将清除本集所有已生成的${storyboardImageLabel}并重新生成，确认继续？`)) return
    try {
      const usedModel = modelKey || sbProjectImageModelKey
      const res = await storyboardAPI.generateAll(projectId, selectedEpisodeId, usedModel, true) as unknown as { data: { triggered: number } }
      const n = res?.data?.triggered ?? 0
      toast({
        title: n > 0 ? `当前集重新${storyboardGenerateLabel}已启动，共 ${n} 个` : `没有可重新生成的${storyboardImageLabel}`,
        variant: n > 0 ? 'success' : 'default',
      })
      mutateStats()
      mutateSb()
    } catch {
      toast({ title: '重新生成失败', variant: 'destructive' })
    }
  }

  const handleGenerateAll = async (modelKey?: string) => {
    if (!storyboardAssetsReady) {
      toast({ title: storyboardGenerateBlockedText, variant: 'destructive' })
      return
    }
    try {
      const selectedEpisodeId = episodeFilter !== 'all' ? Number(episodeFilter) : undefined
      const usedModel = modelKey || sbProjectImageModelKey
      const res = await storyboardAPI.generateAll(projectId, selectedEpisodeId, usedModel) as unknown as { data: { triggered: number } }
      const n = res?.data?.triggered ?? 0
      toast({
        title: n > 0
          ? (selectedEpisodeId ? `当前集${storyboardGenerateLabel}已启动，共 ${n} 个` : `批量${storyboardGenerateLabel}已启动，共 ${n} 个`)
          : (selectedEpisodeId ? `当前集没有可生成的${storyboardImageLabel}` : `没有可生成的${storyboardImageLabel}`),
        variant: n > 0 ? 'success' : 'default',
      })
      mutateStats()
      mutateSb()
    } catch {
      toast({ title: '批量生成失败', variant: 'destructive' })
    }
  }

  const handleVoid = async (id: number) => {
    try {
      await storyboardAPI.void(projectId, id)
      toast({ title: '已作废', variant: 'success' })
      mutateSb()
      if (selectedSb?.id === id) setSelectedSb(null)
    } catch {
      toast({ title: '操作失败', variant: 'destructive' })
    }
  }

  const handleDelete = async (id: number) => {
    if (!window.confirm('确认永久删除该分镜？此操作不可撤销。')) return
    try {
      await storyboardAPI.delete(projectId, id)
      toast({ title: '已删除', variant: 'success' })
      mutateSb()
      mutateStats()
      if (selectedSb?.id === id) setSelectedSb(null)
    } catch {
      toast({ title: '删除失败', variant: 'destructive' })
    }
  }

  // Build from API data — only active image models, ordered by backend priority
  const SB_MODEL_OPTIONS = useMemo(
    () => dedupeModels([
      ...sbImageModels.filter((m) => m.is_active && m.model_key),
      ...sbImageModels.filter((m) => !m.is_active && m.failure_reason && m.model_key),
    ]).map(buildImageModelOption),
    [sbImageModels]
  )

  const formatSbErrorMsg = (msg: string): string => {
    if (!msg) return '生成失败'
    if (msg.includes('moderation') || msg.includes('content_policy'))
      return '内容审核未通过 — 建议换用通义万相'
    if (msg.includes('timeout') || msg.includes('deadline'))
      return '生成超时 — 服务繁忙，请稍后重试'
    if (msg.includes('rate') || msg.includes('429'))
      return '请求过于频繁 — 请稍后重试'
    if (msg.includes('unreachable') || msg.includes('connection'))
      return '服务不可达 — 请检查服务状态'
    if (msg.includes('upload') || msg.includes('storage'))
      return '图片上传失败 — 存储服务异常'
    if (msg.length > 80) return msg.substring(0, 77) + '...'
    return msg
  }

  const handleRetry = async (id: number, modelName?: string) => {
    if (!storyboardAssetsReady) {
      toast({ title: storyboardGenerateBlockedText, variant: 'destructive' })
      return
    }
    try {
      await storyboardAPI.retry(projectId, id, modelName)
      const label = modelName ? SB_MODEL_OPTIONS.find(m => m.key === modelName)?.label || modelName : '默认'
      toast({ title: `使用 ${label} 重新生成已启动`, variant: 'success' })
      mutateSb()
      mutateStats()
    } catch {
      toast({ title: '重试失败', variant: 'destructive' })
    }
  }

  const handleRetryAllFailed = async (modelName?: string) => {
    if (!storyboardAssetsReady) {
      toast({ title: storyboardGenerateBlockedText, variant: 'destructive' })
      return
    }
    try {
      const selectedEpisodeId = episodeFilter !== 'all' ? Number(episodeFilter) : undefined
      const res = await storyboardAPI.retryFailed(projectId, modelName, selectedEpisodeId) as unknown as { data: { retried: number } }
      const n = res?.data?.retried ?? 0
      const label = modelName ? SB_MODEL_OPTIONS.find(m => m.key === modelName)?.label || modelName : '默认'
      toast({
        title: n > 0
          ? (selectedEpisodeId ? `当前集已启动 ${n} 个失败${storyboardItemLabel}重试 (${label})` : `批量重试 ${n} 个失败${storyboardItemLabel} (${label})`)
          : (selectedEpisodeId ? `当前集没有失败${storyboardItemLabel}` : `当前没有失败${storyboardItemLabel}`),
        variant: n > 0 ? 'success' : 'default',
      })
      mutateSb()
      mutateStats()
    } catch {
      toast({ title: '批量重试失败', variant: 'destructive' })
    }
  }

  const handlePauseGeneration = async () => {
    setPausingGeneration(true)
    try {
      const selectedEpisodeId = episodeFilter !== 'all' ? Number(episodeFilter) : undefined
      const res = await storyboardAPI.pauseGeneration(projectId, selectedEpisodeId) as unknown as { data?: { paused?: number } }
      const paused = res?.data?.paused ?? 0
      toast({
        title: selectedEpisodeId ? `已暂停当前集${storyboardGenerateLabel}（${paused} 项）` : `已暂停${storyboardGenerateLabel}（${paused} 项）`,
        variant: 'success',
      })
      mutateStats()
      mutateSb()
    } catch {
      toast({ title: `暂停${storyboardGenerateLabel}失败`, variant: 'destructive' })
    } finally {
      setPausingGeneration(false)
    }
  }

  const handleResumeGeneration = async () => {
    if (!storyboardAssetsReady) {
      toast({ title: storyboardResumeBlockedText, variant: 'destructive' })
      return
    }
    setResumingGeneration(true)
    try {
      const selectedEpisodeId = episodeFilter !== 'all' ? Number(episodeFilter) : undefined
      const res = await storyboardAPI.resumeGeneration(projectId, selectedEpisodeId) as unknown as { data?: { triggered?: number } }
      const triggered = res?.data?.triggered ?? 0
      toast({
        title: triggered > 0
          ? (selectedEpisodeId ? `已继续当前集${storyboardGenerateLabel}（${triggered} 项）` : `已继续${storyboardGenerateLabel}（${triggered} 项）`)
          : (selectedEpisodeId ? `当前集没有已暂停的${storyboardItemLabel}` : `当前没有已暂停的${storyboardItemLabel}`),
        variant: triggered > 0 ? 'success' : 'default',
      })
      mutateStats()
      mutateSb()
    } catch {
      toast({ title: `继续${storyboardGenerateLabel}失败`, variant: 'destructive' })
    } finally {
      setResumingGeneration(false)
    }
  }

  const handleAuditContinuity = async () => {
    setIsAuditingContinuity(true)
    try {
      const selectedEpisodeId = episodeFilter !== 'all' ? Number(episodeFilter) : undefined
      const res = await storyboardAPI.auditContinuity(projectId, selectedEpisodeId) as unknown as { data?: { total_patched?: number } }
      const n = res?.data?.total_patched ?? 0
      toast({
        title: n > 0 ? `AI 已补全 ${n} 条${storyboardItemLabel}缺失信息` : `检查完成，未发现需要补全的${storyboardItemLabel}信息`,
        variant: n > 0 ? 'success' : 'default',
      })
      mutateSb()
    } catch {
      toast({ title: 'AI 补全失败', variant: 'destructive' })
    } finally {
      setIsAuditingContinuity(false)
    }
  }

  // ── 侧边栏触发器响应 ────────────────────────────────────────────
  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(() => { if (sbGenerateTrigger) handleGenerateAll(undefined) }, [sbGenerateTrigger])
  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(() => { if (sbRegenerateTrigger) handleForceGenerateEpisode(undefined) }, [sbRegenerateTrigger])
  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(() => { if (sbPauseTrigger) handlePauseGeneration() }, [sbPauseTrigger])
  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(() => { if (sbResumeTrigger) handleResumeGeneration() }, [sbResumeTrigger])
  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(() => { if (sbAuditTrigger) handleAuditContinuity() }, [sbAuditTrigger])

  const handleSwitchVersion = async (sbId: number, versionId: number) => {
    try {
      await storyboardAPI.switchVersion(projectId, sbId, versionId)
      toast({ title: '版本已切换', variant: 'success' })
      mutateSb()
    } catch {
      toast({ title: '切换失败', variant: 'destructive' })
    }
  }

  const handleChat = async () => {
    if (!selectedSb || !chatInput.trim()) return
    const message = chatInput.trim()
    const previousStoryboard = selectedSb
    const optimisticMessage: ChatMessage = {
      role: 'user',
      content: message,
      timestamp: new Date().toISOString(),
    }

    setSelectedSb({
      ...selectedSb,
      agent_history: [...(selectedSb.agent_history ?? []), optimisticMessage],
    })
    setChatInput('')
    setChatLoading(true)
    try {
      const res = await storyboardAPI.chat(projectId, selectedSb.id, message) as unknown as { data: Storyboard }
      if (res.data) setSelectedSb(res.data)
      mutateSb()
    } catch {
      setSelectedSb(previousStoryboard)
      setChatInput(message)
      toast({ title: '发送失败', variant: 'destructive' })
    } finally {
      setChatLoading(false)
    }
  }

  if (isLoading) return <TabSkeleton />

  return (
    <div className="relative">
      {/* ── Progress bar — shown when generating or when stats are meaningful ── */}
      {stats.total > 0 && (
        <div className={`mb-4 rounded-lg border px-4 py-3 ${isActive ? 'border-blue-200 bg-blue-50' : 'border-surface-200 bg-surface-50'}`}>
          <div className="mb-2 flex items-center justify-between">
            <div className="flex items-center gap-2">
              {isActive ? <Loader2 className="h-4 w-4 animate-spin text-blue-600" /> : stats.paused > 0 ? <Pause className="h-4 w-4 text-yellow-700" /> : null}
              <span className={`text-sm font-medium ${isActive ? 'text-blue-800' : 'text-surface-700'}`}>
                {isActive ? `${storyboardGenerateLabel}中...` : stats.paused > 0 ? `${storyboardGenerateLabel}已暂停` : `${storyboardGenerateLabel}进度`}
              </span>
            </div>
            <span className="text-sm font-semibold text-surface-700">
              {stats.completed}/{stats.total}
            </span>
          </div>
          <Progress value={stats.total > 0 ? (stats.completed / stats.total) * 100 : 0} className="mb-2 h-2" />
          <div className="flex flex-wrap gap-3 text-xs">
            <span className="flex items-center gap-1 text-green-600">
              <CheckCircle2 className="h-3 w-3" /> 已完成 {stats.completed}
            </span>
            {stats.generating > 0 && (
              <span className="flex items-center gap-1 text-blue-600">
                <Loader2 className="h-3 w-3 animate-spin" /> 生成中 {stats.generating}
              </span>
            )}
            {stats.paused > 0 && (
              <span className="flex items-center gap-1 text-yellow-700">
                <Pause className="h-3 w-3" /> 已暂停 {stats.paused}
              </span>
            )}
            {stats.pending > 0 && (
              <span className="flex items-center gap-1 text-surface-500">
                <Clock className="h-3 w-3" /> 待生成 {stats.pending}
              </span>
            )}
            {stats.failed > 0 && (
              <span
                className="flex cursor-pointer items-center gap-1 text-red-500 hover:underline"
                onClick={() => setStatusFilter('failed')}
              >
                <AlertCircle className="h-3 w-3" /> 失败 {stats.failed}
              </span>
            )}
            {stats.voided > 0 && (
              <span className="flex items-center gap-1 text-surface-400">
                <Ban className="h-3 w-3" /> 已作废 {stats.voided}
              </span>
            )}
          </div>
        </div>
      )}
      {/* ── 串行场景分组 — 仅在串行项目中显示 ─────────────────────── */}
      {project.project_type === 'video_serial' && (
        <div className="mb-4 rounded-xl border border-indigo-200 bg-indigo-50/30 p-4">
          <SerialSceneGroups projectId={projectId} episodeId={episodeId} />
        </div>
      )}
      <div className="mb-4 flex flex-wrap items-center gap-3">
        <div className="relative min-w-[220px] flex-1 sm:max-w-xs">
          <Search className="pointer-events-none absolute left-3 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-surface-400" />
          <Input
            value={keyword}
            onChange={(e) => setKeyword(e.target.value)}
            placeholder="搜索场景 / 地点 / 台词 / 角色"
            className="pl-8"
          />
        </div>
        <Select value={statusFilter} onValueChange={setStatusFilter}>
          <SelectTrigger className="w-32">
            <SelectValue placeholder="状态" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">全部状态</SelectItem>
            <SelectItem value="pending">待生成</SelectItem>
            <SelectItem value="generating">生成中</SelectItem>
            <SelectItem value="paused">已暂停</SelectItem>
            <SelectItem value="completed">已完成</SelectItem>
            <SelectItem value="failed">失败</SelectItem>
            <SelectItem value="voided">已作废</SelectItem>
          </SelectContent>
        </Select>
        <div className="ml-auto flex flex-wrap items-center gap-2">
          {episodeId !== undefined && onExtractStoryboards && (
            <Button
              onClick={onExtractStoryboards}
              disabled={storyboardButtonDisabled}
              variant="outline"
              size="sm"
              className="border-primary-200 bg-primary-50 text-primary-700 hover:bg-primary-100 hover:text-primary-800 disabled:opacity-60"
              title={awaitingAutoStoryboard ? '资源提取完成后会自动发起镜头拆分' : extractStoryboardLabel}
            >
              {isExtractingStoryboards ? <RefreshCw className="mr-1.5 h-3.5 w-3.5 animate-spin" /> : <LayoutGrid className="mr-1.5 h-3.5 w-3.5" />}
              {awaitingAutoStoryboard ? '等待自动拆分镜头' : isExtractingStoryboards ? '镜头拆分中…' : extractStoryboardLabel}
            </Button>
          )}
          {!hideActionBar && (
            <>
              <span className="text-[10px] font-medium text-surface-400">{storyboardImageLabel}</span>
              <Button
                size="sm"
                variant="outline"
                onClick={handleAuditContinuity}
                disabled={isAuditingContinuity}
                title="AI 检查并补全缺失的角色、地点和描述信息"
              >
                {isAuditingContinuity ? (
                  <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
                ) : (
                  <Bot className="mr-1.5 h-3.5 w-3.5" />
                )}
                AI 补全缺失信息
              </Button>
              <Button
                size="sm"
                variant="outline"
                onClick={handlePauseGeneration}
                disabled={pausingGeneration || (stats.generating + stats.pending === 0)}
                title={`暂停整个项目的${storyboardImageLabel}生成；已在执行中的个别任务可能自然完成`}
              >
                {pausingGeneration ? (
                  <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
                ) : (
                  <Pause className="mr-1.5 h-3.5 w-3.5" />
                )}
                暂停生成
              </Button>
              <Button
                size="sm"
                variant="outline"
                onClick={handleResumeGeneration}
                disabled={resumingGeneration || stats.paused === 0}
                title={`继续项目下所有已暂停的${storyboardImageLabel}生成`}
              >
                {resumingGeneration ? (
                  <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
                ) : (
                  <Play className="mr-1.5 h-3.5 w-3.5" />
                )}
                继续生成
              </Button>
            </>
          )}
          {stats.failed > 0 && (
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button size="sm" variant="outline" className="border-red-200 text-red-600 hover:bg-red-50" title={`选择模型重试所有失败的${storyboardItemLabel}`}>
                  <RefreshCw className="mr-1.5 h-3.5 w-3.5" />
                  重试失败 ({stats.failed})
                </Button>
              </DropdownMenuTrigger>
              <ImageModelDropdownContent
                options={SB_MODEL_OPTIONS}
                availability={imageModelAvailability}
                onSelect={handleRetryAllFailed}
                showTags
              />
            </DropdownMenu>
          )}
          {!hideActionBar && (
            <>
              {episodeFilter !== 'all' && (storyboardAssetsReady || isActive || stats.paused > 0) && (
            <>
              {/* 生成本集分镜 — 带模型选择 */}
              {isActive ? (
                <Button size="sm" variant="outline" disabled>
                  <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
                  生成中...
                </Button>
              ) : (
                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <Button
                      size="sm"
                      variant="outline"
                      disabled={!storyboardAssetsReady}
                      title={!storyboardAssetsReady ? storyboardAssetsBlockingReason || '请先完成资源图生成' : `选择模型生成本集所有待处理的${storyboardImageLabel}`}
                    >
                      <Sparkles className="mr-1.5 h-3.5 w-3.5" />
                      {`生成本集${storyboardImageLabel}`}
                    </Button>
                  </DropdownMenuTrigger>
                  <ImageModelDropdownContent
                    options={SB_MODEL_OPTIONS}
                    availability={imageModelAvailability}
                    onSelect={(key) => handleGenerateAll(key)}
                  />
                </DropdownMenu>
              )}
              {/* 重新生成本集 — 带模型选择 */}
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button size="sm" variant="outline" disabled={isActive} title={isActive ? `${storyboardGenerateLabel}进行中，请等待或先暂停` : `重置本集所有${storyboardImageLabel}并重新生成（包括已完成的）`}>
                    <RotateCcw className="mr-1.5 h-3.5 w-3.5" />
                    重新生成本集
                  </Button>
                </DropdownMenuTrigger>
                <ImageModelDropdownContent
                  options={SB_MODEL_OPTIONS}
                  availability={imageModelAvailability}
                  onSelect={handleForceGenerateEpisode}
                  label="选择生成模型（含已完成）"
                />
              </DropdownMenu>
            </>
          )}
          {episodeFilter === 'all' && (
            isActive ? (
              <Button size="sm" disabled>
                <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
                生成中...
              </Button>
            ) : (
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button
                    size="sm"
                    disabled={!storyboardAssetsReady}
                    title={!storyboardAssetsReady ? storyboardAssetsBlockingReason || '请先完成资源图生成' : `选择模型一键生成所有待处理的${storyboardImageLabel}`}
                  >
                    <Sparkles className="mr-1.5 h-3.5 w-3.5" />
                    {`一键生成${storyboardImageLabel}`}
                  </Button>
                </DropdownMenuTrigger>
                <ImageModelDropdownContent
                  options={SB_MODEL_OPTIONS}
                  availability={imageModelAvailability}
                  onSelect={(key) => handleGenerateAll(key)}
                />
              </DropdownMenu>
            )
          )}
            </>
          )}
          {/* ── 分镜视频生成 ── */}
          {stats.completed > 0 && (
            <>
              <span className="h-5 w-px bg-surface-200" />
              <span className="text-[10px] font-medium text-green-700">视频</span>
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button size="sm" variant="outline" className="border-green-200 text-green-700 hover:bg-green-50" disabled={generatingAllVideos} title={`选择模型为所有已完成${isSerial ? '场景首帧' : '分镜图片'}批量生成${storyboardVideoLabel}`}>
                    {generatingAllVideos ? (
                      <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
                    ) : (
                      <Video className="mr-1.5 h-3.5 w-3.5" />
                    )}
                    {`一键生成${storyboardVideoLabel}`}
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end" className="w-80">
                  <DropdownMenuLabel className="text-[10px] text-surface-400">选择视频生成模型</DropdownMenuLabel>
                  <DropdownMenuSeparator />
                  {vtVideoModelOptions.map((m, idx) => {
                    const avail = videoModelAvailability[m.key]
                    return (
                    <DropdownMenuItem key={m.key} className={`cursor-pointer px-3 py-2 ${avail === false ? 'opacity-50' : ''}`} onClick={() => handleGenerateAllEpisodeVideos(m.key)}>
                      <div className="flex w-full items-start gap-2">
                        <div className="mt-0.5 flex flex-col items-center gap-0.5">
                          <span className="text-sm">{m.icon}</span>
                          <span className="rounded-full bg-surface-200 px-1 text-[8px] text-surface-500 font-bold">#{idx + 1}</span>
                        </div>
                        <div className="flex-1 min-w-0">
                          <div className="flex items-center gap-1.5 flex-wrap">
                            <span className="text-xs font-semibold">{m.label}</span>
                            {m.speed === 'fast' && <span className="rounded bg-green-100 px-1 py-0 text-[9px] text-green-700">⚡ 快</span>}
                            {m.quality === 'high' && <span className="rounded bg-purple-100 px-1 py-0 text-[9px] text-purple-700">★ 高质</span>}
                            {avail === true && <span className="rounded bg-emerald-100 px-1 py-0 text-[9px] text-emerald-700">● 可用</span>}
                            {avail === false && <span className="rounded bg-red-100 px-1 py-0 text-[9px] text-red-600">● 未配置</span>}
                          </div>
                          <p className="text-[10px] text-surface-400 leading-none mt-0.5">{getProviderLabel(m.provider)}</p>
                          <p className="mt-0.5 text-[10px] text-surface-500 leading-tight">{m.desc}</p>
                          <div className="mt-1 flex flex-wrap gap-1">
                            {m.tags.map(t => (
                              <span key={t} className="rounded-full bg-surface-100 px-1.5 py-0 text-[9px] text-surface-500">{t}</span>
                            ))}
                          </div>
                        </div>
                      </div>
                    </DropdownMenuItem>
                    )
                  })}
                </DropdownMenuContent>
              </DropdownMenu>
            </>
          )}
        </div>
      </div>
      {!storyboardAssetsReady && (
        <div className="mb-4 rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-xs text-amber-800">
          {storyboardAssetsBlockingReason}
        </div>
      )}

      <Dialog open={videoDialogEpisodeId !== null} onOpenChange={(open) => { if (!open) setVideoDialogEpisodeId(null) }}>
        <DialogContent className="flex flex-col gap-0 overflow-hidden p-0 sm:max-w-2xl">
          <DialogHeader className="shrink-0 border-b border-surface-100 px-6 py-4">
            <DialogTitle>
              {selectedVideoDialogEpisode
                ? `第 ${selectedVideoDialogEpisode.episode_number} 集 · 当前集生成视频`
                : '当前集生成视频'}
            </DialogTitle>
          </DialogHeader>
          <div className="min-h-0 flex-1 space-y-4 overflow-y-auto px-6 py-4">
            <div className="rounded-lg border border-violet-200 bg-violet-50 px-3 py-3">
              <div className="flex items-center justify-between gap-2">
                <div>
                  <p className="text-xs font-semibold text-violet-800">快速质感预设</p>
                  <p className="text-[11px] text-violet-600">一键切换模型 + 风格 + 运动模式。写实大片优先“真人电影”，对白戏和人物关系优先“真人短剧”。</p>
                </div>
                <Badge variant="outline" className="border-violet-200 bg-white text-[10px] text-violet-700">
                  当前：{selectedEpisodeVideoModelMeta.label} / {selectedEpisodeVideoStyleMeta.label}
                </Badge>
              </div>
              <div className="mt-3 flex flex-wrap gap-2">
                {VIDEO_GENERATION_PRESETS.map((preset) => {
                  const active =
                    selectedEpisodeVideoModel === preset.model &&
                    selectedEpisodeVideoStyle === preset.style &&
                    selectedEpisodeVideoMotionMode === preset.motion
                  return (
                    <button
                      key={preset.key}
                      type="button"
                      onClick={() => applyEpisodeVideoPreset(preset.key)}
                      className={`rounded-lg border px-3 py-2 text-left transition-colors ${
                        active
                          ? 'border-violet-300 bg-white text-violet-700 shadow-sm'
                          : 'border-violet-200/70 bg-white/70 text-surface-600 hover:border-violet-300 hover:bg-white'
                      }`}
                    >
                      <div className="flex items-center gap-2">
                        <span className="text-xs font-semibold">{preset.label}</span>
                        <span className="rounded-full bg-violet-100 px-2 py-0.5 text-[10px] text-violet-700">{preset.tone}</span>
                      </div>
                      <p className="mt-1 text-[11px] leading-5">{preset.hint}</p>
                    </button>
                  )
                })}
              </div>
            </div>
            <div className="rounded-lg border border-surface-200 bg-surface-50 px-3 py-3">
              <p className="text-xs font-semibold text-surface-700">当前已选视频配置</p>
              <div className="mt-2 flex flex-wrap gap-2 text-[11px]">
                <span className="rounded-full border border-surface-200 bg-white px-2.5 py-1 text-surface-600">
                  风格：{selectedEpisodeVideoStyleLabel}
                </span>
                <span className="rounded-full border border-surface-200 bg-white px-2.5 py-1 text-surface-600">
                  运动：{selectedEpisodeVideoMotionLabel}
                </span>
                <span className="rounded-full border border-surface-200 bg-white px-2.5 py-1 text-surface-600">
                  模式：{selectedEpisodeVideoModeLabel}
                </span>
              </div>
              <p className="mt-2 text-[11px] text-surface-500">
                风格和运动模式会与本次选择的模型、尺寸、大小、清晰度一起写入生成提示词。
              </p>
            </div>
            <div className="grid gap-4 md:grid-cols-2">
              <div className="space-y-2">
                <Label>生成模型</Label>
                <Select value={selectedEpisodeVideoModel} onValueChange={(value) => setSelectedEpisodeVideoModel(value)}>                  <SelectTrigger>
                    <SelectValue placeholder="选择视频模型" />
                  </SelectTrigger>
                  <SelectContent>
                    {vtVideoModelOptions.map((item) => (
                      <SelectItem key={item.key} value={item.key}>
                        {item.icon} {item.label}{videoModelAvailability[item.key] === true ? ' ●' : videoModelAvailability[item.key] === false ? ' ○' : ''}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <p className="text-[11px] text-surface-500">{selectedEpisodeVideoModelMeta.desc}</p>
                <p className="text-[11px] text-violet-600">{VIDEO_MODEL_SELECTION_HINTS[selectedEpisodeVideoModel] ?? ''}</p>
              </div>
              <div className="space-y-2">
                <Label>画面风格</Label>
                <Select value={selectedEpisodeVideoStyle} onValueChange={(value) => setSelectedEpisodeVideoStyle(value)}>
                  <SelectTrigger>
                    <SelectValue placeholder="选择画面风格" />
                  </SelectTrigger>
                  <SelectContent>
                    {VIDEO_STYLE_COMPACT_OPTIONS.map((item) => (
                      <SelectItem key={item.key} value={item.key}>
                        {item.label} · {item.tone}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <p className="text-[11px] text-surface-500">{selectedEpisodeVideoStyleMeta.hint}</p>
              </div>
              <div className="space-y-2">
                <Label>尺寸</Label>
                <Select value={selectedEpisodeVideoFrameSize} onValueChange={(value) => setSelectedEpisodeVideoFrameSize(value as (typeof VIDEO_FRAME_SIZE_OPTIONS)[number]['key'])}>
                  <SelectTrigger>
                    <SelectValue placeholder="选择尺寸" />
                  </SelectTrigger>
                  <SelectContent>
                    {VIDEO_FRAME_SIZE_OPTIONS.map((item) => (
                      <SelectItem key={item.key} value={item.key}>{item.label}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <p className="text-[11px] text-surface-500">{selectedEpisodeVideoFrameSizeMeta.desc}</p>
              </div>
              <div className="space-y-2">
                <Label>大小</Label>
                <Select value={selectedEpisodeVideoSubjectSize} onValueChange={(value) => setSelectedEpisodeVideoSubjectSize(value as (typeof VIDEO_SUBJECT_SIZE_OPTIONS)[number]['key'])}>
                  <SelectTrigger>
                    <SelectValue placeholder="选择主体大小" />
                  </SelectTrigger>
                  <SelectContent>
                    {VIDEO_SUBJECT_SIZE_OPTIONS.map((item) => (
                      <SelectItem key={item.key} value={item.key}>{item.label}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <p className="text-[11px] text-surface-500">{selectedEpisodeVideoSubjectSizeMeta.desc}</p>
              </div>
              <div className="space-y-2">
                <Label>清晰度</Label>
                <Select value={selectedEpisodeVideoClarity} onValueChange={(value) => setSelectedEpisodeVideoClarity(value as (typeof VIDEO_CLARITY_OPTIONS)[number]['key'])}>
                  <SelectTrigger>
                    <SelectValue placeholder="选择清晰度" />
                  </SelectTrigger>
                  <SelectContent>
                    {VIDEO_CLARITY_OPTIONS.map((item) => (
                      <SelectItem key={item.key} value={item.key}>{item.label}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <p className="text-[11px] text-surface-500">{selectedEpisodeVideoClarityMeta.desc}</p>
              </div>
            </div>
            {/* Model-specific param selectors (aspect_ratio, resolution, duration, etc.) */}
            {(videoModelParams[selectedEpisodeVideoModel] ?? []).length > 0 ? (
              <div className="rounded-lg border border-blue-100 bg-blue-50/60 px-3 py-3">
                <p className="text-xs font-semibold text-blue-800">📐 模型生成参数</p>
                <p className="mt-0.5 text-[11px] text-blue-600">以下参数直接传给视频模型，影响生成画幅与分辨率。</p>
                <div className="mt-2 flex flex-wrap gap-3">
                  {(videoModelParams[selectedEpisodeVideoModel] ?? [])
                    .map((param) => (
                      <div key={param.key} className="flex flex-col gap-1">
                        <label className="text-[11px] font-medium text-blue-700">{param.label}</label>
                        <Select
                          value={getModelParam(selectedEpisodeVideoModel, param.key) || param.default}
                          onValueChange={(val) => setModelParam(selectedEpisodeVideoModel, param.key, val)}
                        >
                          <SelectTrigger className="h-8 w-36 border-blue-200 bg-white text-xs">
                            <SelectValue placeholder={`选择${param.label}`} />
                          </SelectTrigger>
                          <SelectContent>
                            {param.values.map((v) => (
                              <SelectItem key={v.value} value={v.value}>{v.label}</SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                      </div>
                    ))}
                </div>
              </div>
            ) : null}
            {/* Transition effect selector */}
            <div className="rounded-lg border border-purple-100 bg-purple-50/60 px-3 py-3">
              <p className="text-xs font-semibold text-purple-800">🎬 转场效果</p>
              <p className="mt-0.5 text-[11px] text-purple-600">控制片段间的过渡动画，dissolve 叠化最为流畅自然。</p>
              <div className="mt-2 flex flex-wrap items-end gap-3">
                <div className="flex flex-col gap-1">
                  <label className="text-[11px] font-medium text-purple-700">转场类型</label>
                  <Select value={selectedEpisodeTransition} onValueChange={setSelectedEpisodeTransition}>
                    <SelectTrigger className="h-8 w-36 border-purple-200 bg-white text-xs">
                      <SelectValue placeholder="选择转场" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="dissolve">叠化 (dissolve)</SelectItem>
                      <SelectItem value="fade">淡入淡出 (fade)</SelectItem>
                      <SelectItem value="wipeleft">向左划入 (wipeleft)</SelectItem>
                      <SelectItem value="wiperight">向右划入 (wiperight)</SelectItem>
                      <SelectItem value="circleclose">圆形收缩 (circleclose)</SelectItem>
                      <SelectItem value="none">无转场 (直切)</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
                {selectedEpisodeTransition !== 'none' && (
                  <div className="flex flex-col gap-1">
                    <label className="text-[11px] font-medium text-purple-700">时长 (秒)</label>
                    <Select value={selectedEpisodeTransitionDuration} onValueChange={setSelectedEpisodeTransitionDuration}>
                      <SelectTrigger className="h-8 w-28 border-purple-200 bg-white text-xs">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="0.3">0.3s</SelectItem>
                        <SelectItem value="0.5">0.5s</SelectItem>
                        <SelectItem value="0.8">0.8s</SelectItem>
                        <SelectItem value="1.0">1.0s</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                )}
              </div>
            </div>
            <div className="rounded-lg border border-sky-200 bg-sky-50 px-3 py-3 text-xs leading-5 text-sky-800">
              当前选择会作为提示词补充给视频模型：
              <span className="ml-1 font-medium">
                {selectedEpisodeVideoFrameSizeMeta.label} / {selectedEpisodeVideoSubjectSizeMeta.label} / {selectedEpisodeVideoClarityMeta.label}
              </span>
            </div>
          </div>
          <div className="shrink-0 flex justify-end gap-2 border-t border-surface-100 px-6 py-4">
            <Button variant="outline" onClick={() => setVideoDialogEpisodeId(null)} disabled={videoDialogEpisodeId !== null && generatingVideoEps.has(videoDialogEpisodeId)}>
              取消
            </Button>
            <Button onClick={handleConfirmEpisodeVideoGeneration} disabled={videoDialogEpisodeId === null || generatingVideoEps.has(videoDialogEpisodeId)}>
              {videoDialogEpisodeId !== null && generatingVideoEps.has(videoDialogEpisodeId) ? (
                <Loader2 className="mr-1.5 h-4 w-4 animate-spin" />
              ) : (
                <Video className="mr-1.5 h-4 w-4" />
              )}
              开始生成
            </Button>
          </div>
        </DialogContent>
      </Dialog>

      {/* Storyboard grid — grouped by episode */}
      {storyboards.length === 0 && episodes.length > 0 ? (
        <div className="py-12 text-center">
          <LayoutGrid className="mx-auto mb-3 h-10 w-10 text-surface-300" />
          <p className="mb-4 text-sm text-surface-500">当前项目有 {episodes.length} 集，但尚未创建{storyboardItemLabel}</p>
          <Button onClick={handleCreateFromEpisodes} title={`根据已有集数自动创建${storyboardItemLabel}`}>
            <Sparkles className="mr-1.5 h-4 w-4" />
            {`从集数创建${storyboardItemLabel}`}
          </Button>
        </div>
      ) : storyboards.length === 0 ? (
        <p className="py-12 text-center text-sm text-surface-400">{`暂无${storyboardItemLabel}`}</p>
      ) : (() => {
        // Group storyboards by episode
        const epMap = new Map(episodes.map((ep) => [ep.id, ep]))
        const grouped = new Map<number, Storyboard[]>()
        for (const sb of storyboards) {
          const epId = sb.episode_id ?? 0
          if (!grouped.has(epId)) grouped.set(epId, [])
          grouped.get(epId)!.push(sb)
        }
        // Sort groups: by episode_number, "unassigned" last
        const sortedGroups = Array.from(grouped.entries()).sort(([a], [b]) => {
          if (a === 0) return 1
          if (b === 0) return -1
          const epA = epMap.get(a)
          const epB = epMap.get(b)
          return (epA?.episode_number ?? a) - (epB?.episode_number ?? b)
        })

        const renderSbCard = (sb: Storyboard) => (
          <Card key={sb.id} className={`group overflow-hidden transition-shadow hover:shadow-md ${sb.status === 'failed' ? 'ring-2 ring-red-300' : ''}`}>
            <div
              className="relative aspect-video cursor-pointer overflow-hidden bg-surface-100"
              onClick={() => { setSelectedSb(sb); setVersionIdx(0) }}
            >
              {sb.image_url ? (
                <img src={sb.image_url} alt={`#${sb.sequence_number}`} className="h-full w-full object-cover transition-transform group-hover:scale-105" />
              ) : (
                <div className="flex h-full items-center justify-center text-surface-300">
                  <LayoutGrid className="h-8 w-8" />
                </div>
              )}
              {sb.image_url && (
                <div className="absolute right-2 bottom-2 opacity-0 transition-opacity group-hover:opacity-100">
                  <ZoomBadge src={sb.image_url} alt={`#${sb.sequence_number}`} />
                </div>
              )}
            </div>
            <CardContent className="p-3">
              <div className="mb-1 flex items-center justify-between">
                <span className="text-sm font-semibold">#{sb.sequence_number}</span>
                <StatusBadge status={sb.status} />
              </div>
              <p className="mb-1 line-clamp-2 text-xs text-surface-600">
                {sbDescLang === 'en' && sb.prompt_used ? sb.prompt_used : sb.scene_description}
              </p>
              {sb.status === 'failed' && (
                <p className="mb-1 line-clamp-1 text-[11px] text-red-500" title={sb.error_msg || ''}>
                  💡 {formatSbErrorMsg(sb.error_msg || '')}
                </p>
              )}
              {sb.characters && sb.characters.length > 0 && (
                <div className="mb-2 flex flex-wrap gap-1">
                  {sb.characters.map((c) => (
                    <span key={c} className="rounded bg-surface-100 px-1.5 py-0.5 text-[10px] text-surface-600">{c}</span>
                  ))}
                </div>
              )}
              {sb.versions && sb.versions.length > 1 && (
                <div className="mb-2">
                  <Select defaultValue={String(sb.current_version)} onValueChange={(v) => handleSwitchVersion(sb.id, Number(v))}>
                    <SelectTrigger className="h-7 text-xs">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {sb.versions.slice(0, 4).map((ver) => (
                        <SelectItem key={ver.id} value={String(ver.id)}>V{ver.version_number}{ver.is_current ? ' (当前)' : ''}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
              )}
              <div className="flex gap-1 opacity-0 transition-opacity group-hover:opacity-100">
                {(sb.status === 'failed' || sb.status === 'pending' || sb.status === 'paused') ? (
                  <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                      <Button size="sm" variant="ghost" className={`h-7 px-2 text-xs ${sb.status === 'failed' ? 'text-red-500 hover:text-red-700' : sb.status === 'paused' ? 'text-yellow-700 hover:text-yellow-800' : ''}`} title="选择模型生成">
                        {sb.status === 'failed' ? <RefreshCw className="mr-1 h-3 w-3" /> : sb.status === 'paused' ? <Play className="mr-1 h-3 w-3" /> : <Sparkles className="mr-1 h-3 w-3" />}
                        {sb.status === 'failed' ? '重试' : sb.status === 'paused' ? '继续' : '生成'}
                      </Button>
                    </DropdownMenuTrigger>
                    <ImageModelDropdownContent
                      options={SB_MODEL_OPTIONS}
                      availability={imageModelAvailability}
                      onSelect={(key) => handleRetry(sb.id, key)}
                      align="start"
                      showTags
                      stopPropagation
                    />
                  </DropdownMenu>
                ) : (
                  <Button size="sm" variant="ghost" className="h-7 w-7 p-0" onClick={() => handleGenerate(sb.id)} title="生成">
                    <Sparkles className="h-3.5 w-3.5" />
                  </Button>
                )}
                <Button size="sm" variant="ghost" className="h-7 w-7 p-0" onClick={() => { setSelectedSb(sb); setVersionIdx(0) }} title="查看">
                  <Eye className="h-3.5 w-3.5" />
                </Button>
                <Button size="sm" variant="ghost" className="h-7 w-7 p-0" onClick={() => handleVoid(sb.id)} title="作废">
                  <Ban className="h-3.5 w-3.5" />
                </Button>
                <Button size="sm" variant="ghost" className="h-7 w-7 p-0 text-red-500 hover:text-red-700" onClick={() => handleDelete(sb.id)} title="删除">
                  <Trash2 className="h-3.5 w-3.5" />
                </Button>
                <Button size="sm" variant="ghost" className="h-7 w-7 p-0" onClick={() => { setSelectedSb(sb); setVersionIdx(0) }} title="AI对话">
                  <MessageSquare className="h-3.5 w-3.5" />
                </Button>
              </div>
            </CardContent>
          </Card>
        )

        return (
          <div className="space-y-6">
            {sortedGroups.map(([epId, sbs]) => {
              const ep = epMap.get(epId)
              const sorted = [...sbs].sort((a, b) => a.sequence_number - b.sequence_number)
              const completedCount = sorted.filter(s => s.status === 'completed').length
              return (
                <div key={epId}>
                  <div className="mb-3 flex items-center justify-between border-b pb-2">
                    <div className="flex items-center gap-3">
                      <h3 className="text-sm font-semibold text-surface-800">
                        {ep ? `第 ${ep.episode_number} 集 · ${ep.title}` : '未分配集数'}
                      </h3>
                      <span className="text-xs text-surface-400">
                        {completedCount}/{sorted.length} 已完成
                      </span>
                    </div>
                    {epId > 0 && (
                      <div className="flex items-center gap-1">
                        <Button
                          size="sm"
                          variant="outline"
                          className="h-7 text-xs"
                          disabled={generatingVideoEps.has(epId) || (episodeCompletedMap.get(epId) ?? 0) === 0}
                          onClick={() => openEpisodeVideoDialog(epId)}
                          title={(episodeCompletedMap.get(epId) ?? 0) === 0 ? (sorted.length > 0 ? (isSerial ? `该集有 ${sorted.length} 条${storyboardItemLabel}（${completedCount} 条已完成），尚无可用首帧图片` : `该集有 ${sorted.length} 个${storyboardItemLabel}（${completedCount} 个已完成），尚无已生成图片的${storyboardItemLabel}`) : `当前集暂无已完成${storyboardItemLabel}，无法生成${storyboardVideoLabel}`) : `为当前集选择${storyboardVideoLabel}生成参数`}
                        >
                          {generatingVideoEps.has(epId) ? (
                            <Loader2 className="mr-1 h-3 w-3 animate-spin" />
                          ) : (
                            <Video className="mr-1 h-3.5 w-3.5" />
                          )}
                          {`当前集生成${storyboardVideoLabel}`}
                        </Button>
                      </div>
                    )}
                  </div>
                  <div className="grid grid-cols-2 gap-4 md:grid-cols-3 lg:grid-cols-4">
                    {sorted.map(renderSbCard)}
                  </div>
                </div>
              )
            })}
          </div>
        )
      })()}

      {/* Pagination */}
      {sbTotalPages > 1 && (
        <div className="mt-4 flex items-center justify-center gap-3">
          <Button size="sm" variant="outline" disabled={sbPage <= 1} onClick={() => setSbPage(sbPage - 1)} title="上一页分镜">
            <ChevronLeft className="mr-1 h-4 w-4" /> 上一页
          </Button>
          <span className="text-sm text-surface-600">
            第 {sbPage} / {sbTotalPages} 页（共 {sbTotal} 条）
          </span>
          <Button size="sm" variant="outline" disabled={sbPage >= sbTotalPages} onClick={() => setSbPage(sbPage + 1)} title="下一页分镜">
            下一页 <ChevronRight className="ml-1 h-4 w-4" />
          </Button>
        </div>
      )}

      {/* Storyboard detail panel */}
      {selectedSb && (
        <div className="fixed inset-0 z-40 flex justify-end">
          <div className="absolute inset-0 bg-black/30" onClick={() => setSelectedSb(null)} />
          <div className="relative z-50 flex h-full w-full max-w-6xl flex-col bg-white shadow-xl">
            <div className="flex items-center justify-between border-b px-4 py-3">
              <div>
                <h3 className="text-lg font-semibold">{`${storyboardItemLabel} #${selectedSb.sequence_number}`}</h3>
                <p className="text-xs text-surface-400">左侧查看版本与画面，右侧继续对话修改场景。</p>
              </div>
              <Button size="sm" variant="ghost" onClick={() => setSelectedSb(null)}>
                <X className="h-4 w-4" />
              </Button>
            </div>
            <div className="flex min-h-0 flex-1 flex-col lg:flex-row">
              <div className="flex min-h-0 w-full flex-col border-b bg-surface-50/60 lg:w-[380px] lg:border-b-0 lg:border-r">
                <div className="min-h-0 flex-1 overflow-y-auto p-4">
                  <div className="overflow-hidden rounded-2xl border border-surface-200 bg-white shadow-sm">
                    <div className="relative aspect-video overflow-hidden bg-surface-100">
                      {selectedStoryboardPreviewUrl ? (
                        <img
                          src={selectedStoryboardPreviewUrl}
                          alt={selectedStoryboardVersion ? `V${selectedStoryboardVersion.version_number}` : `${storyboardItemLabel} #${selectedSb.sequence_number}`}
                          className="h-full w-full object-cover"
                        />
                      ) : (
                        <div className="flex h-full items-center justify-center text-surface-300">
                          <LayoutGrid className="h-10 w-10" />
                        </div>
                      )}
                      {selectedSb.status === 'generating' && (
                        <div className="absolute inset-0 flex items-center justify-center bg-surface-950/35">
                          <div className="rounded-full bg-white/90 px-3 py-1 text-[11px] font-medium text-surface-700 shadow-sm">
                            新版本生成中
                          </div>
                        </div>
                      )}
                    </div>
                    <div className="border-t border-surface-100 px-4 py-3">
                      <div className="flex items-center justify-between gap-3">
                        <div>
                          <p className="text-sm font-medium text-surface-800">
                            {selectedStoryboardVersion ? `版本 V${selectedStoryboardVersion.version_number}` : `当前${storyboardItemLabel}预览`}
                          </p>
                          <p className="text-[11px] text-surface-400">
                            {selectedSb.status === 'generating'
                              ? '生成完成后会自动刷新到这里。'
                              : selectedSb.versions && selectedSb.versions.length > 1
                                ? `共 ${selectedSb.versions.length} 个版本`
                                : '目前仅展示当前版本'}
                          </p>
                        </div>
                        <StatusBadge status={selectedSb.status} />
                      </div>
                      {selectedSb.versions && selectedSb.versions.length > 1 && (
                        <div className="mt-3 flex items-center justify-between gap-2">
                          <Button size="sm" variant="ghost" className="h-8 w-8 p-0" disabled={versionIdx === 0} onClick={() => setVersionIdx(versionIdx - 1)}>
                            <ChevronLeft className="h-4 w-4" />
                          </Button>
                          <span className="text-xs text-surface-500">
                            V{selectedSb.versions[versionIdx]?.version_number} ({versionIdx + 1}/{selectedSb.versions.length})
                          </span>
                          <Button size="sm" variant="ghost" className="h-8 w-8 p-0" disabled={versionIdx >= selectedSb.versions.length - 1} onClick={() => setVersionIdx(versionIdx + 1)}>
                            <ChevronRight className="h-4 w-4" />
                          </Button>
                        </div>
                      )}
                    </div>
                  </div>

                  <div className="mt-4 grid grid-cols-3 gap-3 rounded-xl border border-surface-200 bg-white p-3 shadow-sm">
                    <div className="text-center">
                      <p className="text-[10px] text-surface-400">时长</p>
                      <p className="text-sm font-medium">{formatDuration(selectedSb.duration)}</p>
                    </div>
                    <div className="text-center">
                      <p className="text-[10px] text-surface-400 mb-1">运镜</p>
                      <Select
                        value={selectedSb.camera_movement || 'static'}
                        onValueChange={async (val) => {
                          const prev = selectedSb
                          setSelectedSb({ ...selectedSb, camera_movement: val })
                          try {
                            await storyboardAPI.update(projectId, selectedSb.id, { camera_movement: val } as Partial<Storyboard>)
                            mutateSb()
                          } catch {
                            setSelectedSb(prev)
                          }
                        }}
                      >
                        <SelectTrigger className="h-7 px-2 text-xs font-medium text-center border-surface-200">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          {([
                            { value: 'static', label: '静止镜头' },
                            { value: 'push-in', label: '缓慢推进' },
                            { value: 'pull-out', label: '缓慢拉远' },
                            { value: 'pan-left', label: '向左摇镜' },
                            { value: 'pan-right', label: '向右摇镜' },
                            { value: 'tracking', label: '跟随运镜' },
                            { value: 'handheld', label: '手持纪实' },
                          ] as const).map((m) => (
                            <SelectItem key={m.value} value={m.value} className="text-xs">{m.label}</SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </div>
                    <div className="text-center">
                      <p className="text-[10px] text-surface-400">比例</p>
                      <p className="text-sm font-medium">{selectedSb.aspect_ratio || '16:9'}</p>
                    </div>
                  </div>

                  <div className="mt-4 space-y-3 rounded-xl border border-surface-200 bg-white p-4 shadow-sm">
                    <div>
                      <div className="flex items-center justify-between">
                        <p className="text-xs font-medium text-surface-500">
                          {sbDescLang === 'zh' ? '场景描述' : '生成模型描述（英文）'}
                        </p>
                        <div className="flex items-center rounded-md border border-surface-200 bg-surface-50 p-0.5 text-[10px] font-semibold">
                          <button
                            className={`rounded px-1.5 py-0.5 transition-colors ${sbDescLang === 'zh' ? 'bg-white shadow-sm text-primary-700' : 'text-surface-400 hover:text-surface-600'}`}
                            onClick={() => setSbDescLang('zh')}
                          >中</button>
                          <button
                            className={`rounded px-1.5 py-0.5 transition-colors ${sbDescLang === 'en' ? 'bg-white shadow-sm text-primary-700' : 'text-surface-400 hover:text-surface-600'}`}
                            onClick={() => setSbDescLang('en')}
                          >EN</button>
                        </div>
                      </div>
                      {sbDescLang === 'zh' ? (
                        <p className="mt-1 whitespace-pre-wrap text-sm leading-6 text-surface-700">{selectedSb.scene_description || '暂无描述'}</p>
                      ) : (
                        <p className="mt-1 whitespace-pre-wrap break-all text-xs leading-5 text-surface-600">
                          {selectedSb.prompt_used || selectedSb.scene_description || '暂无描述'}
                        </p>
                      )}
                    </div>
                    {/* 关联资源信息 */}
                    {(selectedSb.characters?.length > 0 || selectedSb.location || (selectedSb.asset_ids?.length > 0)) && (
                      <div className="border-t border-surface-100 pt-3 space-y-2">
                        {selectedSb.characters?.length > 0 && (
                          <div className="flex items-start gap-2">
                            <span className="shrink-0 text-[10px] font-medium text-surface-400 w-10 mt-0.5">角色</span>
                            <div className="flex flex-wrap gap-1">
                              {selectedSb.characters.map((c) => (
                                <span key={c} className="rounded-full bg-primary-50 border border-primary-100 px-2 py-0.5 text-[11px] text-primary-700">👤 {c}</span>
                              ))}
                            </div>
                          </div>
                        )}
                        {selectedSb.location && (
                          <div className="flex items-center gap-2">
                            <span className="shrink-0 text-[10px] font-medium text-surface-400 w-10">场景</span>
                            <span className="rounded-full bg-surface-100 px-2 py-0.5 text-[11px] text-surface-700">📍 {selectedSb.location}</span>
                          </div>
                        )}
                        {selectedSb.asset_ids?.length > 0 && (() => {
                          const linkedAssets = storyboardAssets.filter((a) => selectedSb.asset_ids.includes(a.id))
                          if (linkedAssets.length === 0) return null
                          return (
                            <div className="flex items-start gap-2">
                              <span className="shrink-0 text-[10px] font-medium text-surface-400 w-10 mt-1">资源</span>
                              <div className="flex flex-wrap gap-1.5">
                                {linkedAssets.map((a) => (
                                  <div key={a.id} className="flex items-center gap-1 rounded-lg border border-surface-200 bg-surface-50 px-1.5 py-1" title={a.name}>
                                    {a.image_url ? (
                                      <img src={a.image_url} alt={a.name} className="h-7 w-7 rounded object-cover flex-shrink-0" />
                                    ) : (
                                      <div className="h-7 w-7 rounded bg-surface-200 flex items-center justify-center text-[10px] text-surface-400">?</div>
                                    )}
                                    <div className="max-w-[80px]">
                                      <p className="truncate text-[10px] font-medium text-surface-700">{a.name}</p>
                                      <p className="text-[9px] text-surface-400">{a.type === 'character' ? '人物' : a.type === 'scene' ? '场景' : '物品'}</p>
                                    </div>
                                  </div>
                                ))}
                              </div>
                            </div>
                          )
                        })()}
                      </div>
                    )}
                    {selectedSb.dialogue && (
                      <div className="rounded-xl bg-primary-50 p-3 text-sm text-primary-900">
                        <p className="mb-1 text-xs font-medium text-primary-500">台词</p>
                        <p className="whitespace-pre-wrap leading-6">{selectedSb.dialogue}</p>
                      </div>
                    )}

                    {/* ── 分镜语音生成 ── */}
                    {(project.enable_dubbing || project.enable_subtitle) && selectedSb.episode_id && selectedSb.dialogue && (
                      <div className="mt-3 rounded-xl border border-violet-200 bg-violet-50 p-3">
                        <div className="mb-2 flex items-center justify-between">
                          <div className="flex items-center gap-2">
                            <Mic className="h-3.5 w-3.5 text-violet-500" />
                            <span className="text-xs font-medium text-violet-700">语音生成</span>
                          </div>
                          <div className="flex items-center rounded-md border border-violet-200 bg-white p-0.5 text-[10px] font-medium">
                            <button
                              type="button"
                              className={`rounded px-2 py-0.5 transition-colors ${sbVoiceScope === 'single' ? 'bg-violet-500 text-white' : 'text-violet-600 hover:bg-violet-100'}`}
                              onClick={() => setSbVoiceScope('single')}
                            >单帧</button>
                            <button
                              type="button"
                              className={`rounded px-2 py-0.5 transition-colors ${sbVoiceScope === 'episode' ? 'bg-violet-500 text-white' : 'text-violet-600 hover:bg-violet-100'}`}
                              onClick={() => setSbVoiceScope('episode')}
                            >全集</button>
                          </div>
                        </div>
                        <p className="mb-2 text-[11px] text-violet-600">
                          {sbVoiceScope === 'single'
                            ? '仅使用本帧台词生成语音片段'
                            : '聚合本集所有分镜台词生成整集语音'}
                        </p>
                        <div className="mb-2 grid grid-cols-2 gap-1.5 sm:grid-cols-4">
                          <select
                            value={sbVoiceModel}
                            onChange={(e) => setSbVoiceModel(e.target.value)}
                            className="h-7 rounded-md border border-violet-200 bg-white px-2 text-[11px]"
                            title="音色"
                          >
                            {SB_VOICE_OPTIONS.map(v => (
                              <option key={v.value} value={v.value}>{v.label}</option>
                            ))}
                          </select>
                          <select
                            value={sbVoiceRate}
                            onChange={(e) => setSbVoiceRate(e.target.value)}
                            className="h-7 rounded-md border border-violet-200 bg-white px-2 text-[11px]"
                            title="语速"
                          >
                            {[{value:'-30%',label:'慢 -30%'},{value:'-15%',label:'慢 -15%'},{value:'+0%',label:'正常'},{value:'+15%',label:'快 +15%'},{value:'+30%',label:'快 +30%'}].map(v => (
                              <option key={v.value} value={v.value}>{v.label}</option>
                            ))}
                          </select>
                          <select
                            value={sbVoicePitch}
                            onChange={(e) => setSbVoicePitch(e.target.value)}
                            className="h-7 rounded-md border border-violet-200 bg-white px-2 text-[11px]"
                            title="音调"
                          >
                            {[{value:'-10Hz',label:'低 -10Hz'},{value:'-5Hz',label:'低 -5Hz'},{value:'+0Hz',label:'正常'},{value:'+5Hz',label:'高 +5Hz'},{value:'+10Hz',label:'高 +10Hz'}].map(v => (
                              <option key={v.value} value={v.value}>{v.label}</option>
                            ))}
                          </select>
                          <select
                            value={sbVoiceVolume}
                            onChange={(e) => setSbVoiceVolume(e.target.value)}
                            className="h-7 rounded-md border border-violet-200 bg-white px-2 text-[11px]"
                            title="音量"
                          >
                            {[{value:'-20%',label:'低 -20%'},{value:'-10%',label:'低 -10%'},{value:'+0%',label:'正常'},{value:'+10%',label:'高 +10%'},{value:'+20%',label:'高 +20%'}].map(v => (
                              <option key={v.value} value={v.value}>{v.label}</option>
                            ))}
                          </select>
                        </div>
                        <Button
                          size="sm"
                          className="w-full border-violet-300 bg-violet-600 text-white hover:bg-violet-700"
                          disabled={generatingSbVoice || (sbVoiceScope === 'single' && (storyboardTaskMap.get(selectedSb.id)?.status === 'pending' || storyboardTaskMap.get(selectedSb.id)?.status === 'processing'))}
                          onClick={handleSbGenerateVoice}
                          title={sbVoiceScope === 'single' ? '使用本帧台词生成语音' : '聚合本集全部分镜台词生成语音'}
                        >
                          {generatingSbVoice ? (
                            <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
                          ) : (
                            <Mic className="mr-1.5 h-3.5 w-3.5" />
                          )}
                          {sbVoiceScope === 'single' ? '生成本帧语音' : '生成全集语音'}
                        </Button>
                        {/* Per-storyboard task status & audio player */}
                        {sbVoiceScope === 'single' && (() => {
                          const sbTask = storyboardTaskMap.get(selectedSb.id)
                          if (!sbTask) return null
                          return (
                            <div className="mt-2">
                              {(sbTask.status === 'pending' || sbTask.status === 'processing') && (
                                <div className="flex items-center gap-1.5 text-[11px] text-amber-600">
                                  <Loader2 className="h-3 w-3 animate-spin" />
                                  {sbTask.status === 'pending' ? '排队等待中...' : `生成中 ${sbTask.chunks_total > 0 ? `(${sbTask.chunks_done}/${sbTask.chunks_total})` : ''}`}
                                </div>
                              )}
                              {sbTask.status === 'failed' && (
                                <p className="text-[11px] text-red-500">生成失败，可重新提交</p>
                              )}
                              {sbTask.status === 'succeeded' && sbTask.audio_url && (
                                <div className="space-y-1">
                                  <p className="text-[10px] text-violet-500">语音已生成</p>
                                  <audio controls className="h-8 w-full" src={sbTask.audio_url} />
                                  <a href={sbTask.audio_url} target="_blank" rel="noopener noreferrer" className="inline-flex items-center gap-1 text-[10px] text-violet-500 hover:underline">
                                    <Download className="h-3 w-3" /> 下载音频
                                  </a>
                                </div>
                              )}
                            </div>
                          )
                        })()}
                      </div>
                    )}
                  </div>

                  {(selectedSb.status === 'failed' || selectedSb.status === 'pending' || selectedSb.status === 'paused') && (
                    <div className={`mt-4 rounded-xl border p-4 shadow-sm ${selectedSb.status === 'failed' ? 'border-red-200 bg-red-50' : selectedSb.status === 'paused' ? 'border-yellow-200 bg-yellow-50' : 'border-amber-200 bg-amber-50'}`}>
                      <p className={`mb-1 text-sm font-medium ${selectedSb.status === 'failed' ? 'text-red-700' : selectedSb.status === 'paused' ? 'text-yellow-800' : 'text-amber-700'}`}>
                        {selectedSb.status === 'failed' ? `${storyboardGenerateLabel}失败，可直接换模型重试` : selectedSb.status === 'paused' ? `${storyboardGenerateLabel}已暂停，可直接继续` : `${storyboardImageLabel}待生成，可直接选择模型启动`}
                      </p>
                      <p className={`mb-3 text-[11px] ${selectedSb.status === 'failed' ? 'text-red-600' : selectedSb.status === 'paused' ? 'text-yellow-800/80' : 'text-amber-700/80'}`}>
                        {selectedSb.status === 'failed'
                          ? `原因：${formatSbErrorMsg(selectedSb.error_msg || '')}`
                          : selectedSb.status === 'paused'
                            ? `继续后，会从当前${storyboardItemLabel}队列重新拉起${storyboardImageLabel}生成。`
                            : `发送对话修改后，可在这里继续选择模型生成对应${storyboardImageLabel}。`}
                      </p>
                      <div className="max-h-[45vh] overflow-y-auto space-y-3 pr-0.5">
                        {(([
                          { label: '🌐 多模态推荐', filter: (m: typeof SB_MODEL_OPTIONS[0]) => m.tags.includes('多模态') },
                          { label: '🎨 高质量文生图', filter: (m: typeof SB_MODEL_OPTIONS[0]) => m.tags.includes('高质量') && !m.tags.includes('多模态') },
                          { label: '⚡ 高速 / 低成本', filter: (m: typeof SB_MODEL_OPTIONS[0]) => !m.tags.includes('多模态') && !m.tags.includes('高质量') && !m.tags.includes('本地') },
                          { label: '🖥️ 本地部署', filter: (m: typeof SB_MODEL_OPTIONS[0]) => m.tags.includes('本地') },
                        ] as Array<{ label: string; filter: (m: typeof SB_MODEL_OPTIONS[0]) => boolean }>).map(({ label: sectionLabel, filter }) => {
                          const sectionModels = SB_MODEL_OPTIONS.filter(filter)
                          if (sectionModels.length === 0) return null
                          return (
                            <div key={sectionLabel}>
                              <p className="mb-1 text-[10px] font-semibold uppercase tracking-wide text-surface-400">{sectionLabel}</p>
                              <div className="grid grid-cols-2 gap-2">
                                {sectionModels.map((m) => {
                                  const avail = imageModelAvailability[m.key]
                                  const broken = !!m.failureReason
                                  const globalIdx = SB_MODEL_OPTIONS.findIndex(item => item.key === m.key)
                                  return (
                                  <button
                                    key={m.key}
                                    title={broken ? `已停用：${m.failureReason}` : undefined}
                                    disabled={broken}
                                    className={`flex items-start gap-2 rounded-lg border p-2.5 text-left transition-colors ${broken ? 'cursor-not-allowed border-red-200 bg-red-50 opacity-60' : avail === false ? 'border-surface-200 bg-surface-50 opacity-50 cursor-not-allowed' : 'border-surface-200 bg-white hover:border-primary-300 hover:bg-primary-50'}`}
                                    onClick={() => broken ? undefined : handleRetry(selectedSb.id, m.key)}
                                  >
                                    <div className="mt-0.5 flex flex-col items-center gap-0.5">
                                      <span className="text-base leading-none">{m.icon}</span>
                                      <span className="rounded-full bg-surface-200 px-1 text-[8px] text-surface-500 font-bold">#{globalIdx + 1}</span>
                                    </div>
                                    <div className="flex-1 min-w-0">
                                      <div className="flex flex-wrap items-center gap-1">
                                        <span className="text-xs font-semibold text-surface-800">{m.label}</span>
                                        {!broken && m.speed === 'fast' && <span className="rounded bg-green-100 px-1 text-[9px] text-green-700">⚡快</span>}
                                        {!broken && m.quality === 'high' && <span className="rounded bg-blue-100 px-1 text-[9px] text-blue-700">★高质</span>}
                                        {!broken && avail === true && <span className="rounded bg-emerald-100 px-1 text-[9px] text-emerald-700">● 可用</span>}
                                        {!broken && avail === false && <span className="rounded bg-red-100 px-1 text-[9px] text-red-600">● 未配置</span>}
                                        {broken && <span className="rounded bg-red-100 px-1 text-[9px] text-red-600">⚠ 已停用</span>}
                                      </div>
                                      <p className="text-[10px] leading-none text-surface-400 mt-0.5">{getProviderLabel(m.provider)}</p>
                                      {broken ? (
                                        <p className="mt-0.5 text-[9px] leading-tight text-red-400">{m.failureReason}</p>
                                      ) : (
                                        <>
                                          <p className="mt-0.5 text-[10px] leading-tight text-surface-500">{m.desc}</p>
                                          <div className="mt-1 flex flex-wrap gap-0.5">
                                            {m.tags.map(t => (
                                              <span key={t} className="rounded-full bg-surface-100 px-1.5 text-[9px] text-surface-500">{t}</span>
                                            ))}
                                          </div>
                                        </>
                                      )}
                                    </div>
                                  </button>
                                  )
                                })}
                              </div>
                            </div>
                          )
                        }))}
                      </div>
                    </div>
                  )}
                </div>
              </div>

              <div className="flex min-h-0 flex-1 flex-col">
                <div className="flex min-h-0 flex-1 flex-col p-5">
                  <div className="mb-3 flex items-center justify-between">
                    <h4 className="text-sm font-semibold text-surface-700">AI 对话记录</h4>
                    <span className="text-[11px] text-surface-400">{selectedStoryboardMessageCount} 条消息</span>
                  </div>
                  <div
                    ref={chatListRef}
                    onScroll={handleChatListScroll}
                    className="min-h-0 flex-1 space-y-3 overflow-y-auto rounded-xl border border-surface-200 bg-surface-50/70 p-3 pr-2"
                  >
                    {selectedSb.status === 'generating' && (
                      <div className="rounded-2xl border border-primary-200 bg-primary-50/80 px-3 py-3 text-xs text-primary-900 shadow-sm">
                        <div className="flex items-center gap-2 font-medium">
                          <Loader2 className="h-3.5 w-3.5 animate-spin" />
                          <span>{`${storyboardItemLabel}正在生成新版本`}</span>
                        </div>
                        <p className="mt-2 text-[11px] text-primary-700/80">
                          当前画面会在生成完成后自动刷新到左侧版本预览，并保留在版本列表中。
                        </p>
                      </div>
                    )}
                    {selectedSb.status === 'generating' && (
                      <div className="flex justify-start">
                        <div className="max-w-[88%] rounded-2xl border border-surface-200 bg-white px-3 py-2.5 text-xs text-surface-700 shadow-sm">
                          <div className="mb-1 flex items-center gap-2 text-[10px] text-surface-400">
                            <Loader2 className="h-3 w-3 animate-spin" />
                            <span>AI 正在生成新的场景画面</span>
                          </div>
                          <div className="overflow-hidden rounded-xl border border-dashed border-primary-200 bg-primary-50/60">
                            {selectedStoryboardPreviewUrl ? (
                              <div className="relative">
                                <ZoomableImage src={selectedStoryboardPreviewUrl} alt="" className="h-36 w-full object-cover opacity-75" />
                                <div className="absolute inset-0 flex items-center justify-center bg-surface-950/35">
                                  <div className="rounded-full bg-white/90 px-3 py-1 text-[11px] font-medium text-surface-700 shadow-sm">
                                    {`新${storyboardImageLabel}生成中`}
                                  </div>
                                </div>
                              </div>
                            ) : (
                              <div className="flex h-36 w-full flex-col items-center justify-center gap-2">
                                <Image className="h-8 w-8 text-primary-300" />
                                <span className="text-[11px] text-surface-500">{`待返回的新${storyboardImageLabel}将在这里展示`}</span>
                              </div>
                            )}
                          </div>
                          <div className="mt-2 text-[11px] leading-5 text-surface-500">
                            {`你可以继续补充镜头语言、角色动作和环境氛围；本轮完成后左侧会显示新的${storyboardImageLabel}版本。`}
                          </div>
                        </div>
                      </div>
                    )}
                    {selectedStoryboardMessageCount === 0 ? (
                      <p className="py-4 text-center text-xs text-surface-400">暂无对话记录</p>
                    ) : (
                      selectedSb.agent_history.map((rawMsg, i) => {
                        const msg = rawMsg as LegacyChatMessage
                        const role = getChatRole(msg)
                        const content = getChatContent(msg)
                        const imageUrl = getChatImageUrl(msg)

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
                              <div className="whitespace-pre-wrap leading-5">{content}</div>
                              {imageUrl && <img src={imageUrl} alt="" className="mt-2 max-w-full rounded-lg border border-black/5" />}
                            </div>
                          </div>
                        )
                      })
                    )}
                    <div ref={chatBottomRef} />
                  </div>
                </div>

                <div className="border-t bg-white/95 p-4 backdrop-blur-sm">
                  <div className="rounded-xl border border-surface-200 bg-surface-50/70 p-3">
                    <div className="mb-2 flex items-center justify-between">
                      <div>
                        <p className="text-sm font-medium text-surface-800">对话修改</p>
                        <p className="text-[11px] text-surface-400">告诉 AI 你想怎么改场景画面、角色动作、镜头语言或对白氛围。</p>
                      </div>
                    </div>
                    <Textarea
                      value={chatInput}
                      onChange={(e) => setChatInput(e.target.value)}
                      placeholder="例如：保持夜景基调不变，把镜头拉近到人物半身，增加风吹衣摆和火把反光。"
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
    </div>
  )
}
