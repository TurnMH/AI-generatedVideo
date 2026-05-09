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
  Info,
  Link2Off,
  Link2,
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


export function VideoTab({ projectId, project, episodeId }: { projectId: number; project: Project ; episodeId?: number }) {
  const { toast } = useToast()
  const isSerialProject = project.project_type === 'video_serial'
  const videoSharedEpisode = useProjectEpisodeFilter()
  const [videoEpisodeFilter, setVideoEpisodeFilter] = useState<string>(() => episodeId ? String(episodeId) : videoSharedEpisode.value)
  useEffect(() => {
    videoSharedEpisode.setValue(videoEpisodeFilter)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [videoEpisodeFilter])
  const [generating, setGenerating] = useState(false)
  const [previewUrl, setPreviewUrl] = useState<string | null>(null)
  // AI 串行失败诊断弹窗
  const [aiAnalysisClip, setAiAnalysisClip] = useState<VClip | null>(null)
  // clip 详情弹窗（串行/并行均可查看）
  const [clipDetailInfo, setClipDetailInfo] = useState<{ clip: VClip; task: VTask } | null>(null)
  const parsedAnalysis = (() => {
    if (!aiAnalysisClip?.chain_failure_analysis) return null
    try { return JSON.parse(aiAnalysisClip.chain_failure_analysis) as { reason: string; suggestions: string[] } } catch { return null }
  })()
  const videoTaskPageSize = 100

  // Poll video tasks with clip progress
  const { data: tasksRaw, isLoading, mutate: mutateTasks } = useSWR(
    ['video-tasks', projectId],
    async () => {
      let page = 1
      let total = 0
      const items: VTask[] = []

      for (;;) {
        const response = await videoAPI.listTasks(projectId, { page, page_size: videoTaskPageSize })
        const payload = response.data ?? {}
        const batch = (payload.items ?? []) as VTask[]
        total = payload.total ?? batch.length
        items.push(...batch)

        if (batch.length < videoTaskPageSize || items.length >= total) {
          break
        }
        page += 1
      }

      return { items, total }
    },
    { refreshInterval: (data) => {
        const items = (data as { items?: VTask[] } | undefined)?.items ?? []
        const active = items.some((t) => t.status === 'processing' || t.status === 'pending' || Boolean(t.compose_stage && t.compose_stage !== 'done'))
        return active ? 5000 : 30000
      }, revalidateOnFocus: true }
  )

  interface VClip { id: number; clip_order: number; status: string; clip_url: string; duration_sec: number; error_msg: string; chain_failure_analysis?: string; scene_group_key?: string; scene_seq?: number; end_frame_image_url?: string }
  interface VTask {
    id: number; project_id: number; episode_id: number | null; status: string; model_name: string;
    result_url: string; hls_url: string; duration_sec: number; error_msg: string;
    clips: VClip[]; created_at: string; updated_at: string; image_urls: string[];
    style_preset?: string;
    compose_stage?: string;
    serial_scene?: boolean;
    scene_group_keys?: string[];
  }
  interface VTaskListResponse {
    items: VTask[]
    total: number
  }

  const composeStageLabel: Record<string, string> = {
    concatenating: '拼接片段中…',
    adding_audio: '添加音频中…',
    adding_subtitle: '烧录字幕中…',
    uploading: '上传视频中…',
    done: '合成完成',
  }
  const composeStageOrder = ['concatenating', 'adding_audio', 'adding_subtitle', 'uploading', 'done']
  const composeActiveStages = composeStageOrder.filter((stage) => stage !== 'done')
  const isComposeRunning = (task: Pick<VTask, 'compose_stage'>) =>
    Boolean(task.compose_stage && task.compose_stage !== '' && task.compose_stage !== 'done')

  const rawItems = (tasksRaw as { items?: VTask[] })?.items ?? (Array.isArray(tasksRaw) ? tasksRaw as VTask[] : [])
  const tasks: VTask[] = rawItems.filter(t => t.status !== 'cancelled')
  const composingCount = tasks.filter((task) => isComposeRunning(task)).length

  const markTaskComposing = (taskId: number) => {
    void mutateTasks((current?: VTaskListResponse) => {
      if (!current || !Array.isArray(current.items)) return current
      return {
        ...current,
        items: current.items.map((task) =>
          task.id === taskId
            ? {
                ...task,
                status: 'processing',
                error_msg: '',
                compose_stage: task.compose_stage && task.compose_stage !== 'done' ? task.compose_stage : 'concatenating',
              }
            : task
        ),
      }
    }, false)
  }

  // Enable polling when any task is processing/pending. SWR's refreshInterval
  // (configured above) handles the actual polling; we just sync the ref so the
  // next SWR tick picks up the active-vs-idle cadence.
  const anyActive = tasks.some((t) => t.status === 'processing' || t.status === 'pending' || isComposeRunning(t))

  // Fetch storyboards for generation
  const { data: sbData } = useSWR(
    ['storyboards-for-video', projectId],
    () => storyboardAPI.listAll(projectId) as unknown as Promise<{ data: Storyboard[] }>
  )
  const rawSbV = (sbData as { data?: Storyboard[] })?.data
  const storyboards: Storyboard[] = Array.isArray(rawSbV) ? rawSbV : []

  // Fetch episodes for display
  const { data: episodesData } = useSWR(
    ['episodes', projectId],
    () => projectAPI.listEpisodes(projectId) as unknown as Promise<{ data: Episode[] }>
  )
  const episodes = (episodesData as { data?: Episode[] })?.data ?? []
  const episodeMap = new Map(episodes.map((ep) => [ep.id, ep]))
  const videoTaskGroups = React.useMemo(() => {
    const groups = new Map<string, {
      key: string
      label: string
      episodeNumber: number | null
      fallbackEpisodeID: number | null
      tasks: VTask[]
    }>()

    for (const task of tasks) {
      const episodeID = task.episode_id ?? null
      const episode = episodeID ? episodeMap.get(episodeID) : null
      const groupKey = episodeID ? `episode-${episodeID}` : 'unassigned'
      if (!groups.has(groupKey)) {
        groups.set(groupKey, {
          key: groupKey,
          label: episode
            ? `第 ${episode.episode_number} 集 · ${episode.title}`
            : episodeID
              ? `第 ${episodeID} 集`
              : '未分集',
          episodeNumber: episode?.episode_number ?? null,
          fallbackEpisodeID: episodeID,
          tasks: [],
        })
      }
      groups.get(groupKey)!.tasks.push(task)
    }

    return Array.from(groups.values())
      .map((group) => ({
        ...group,
        tasks: [...group.tasks].sort((left, right) => new Date(right.created_at).getTime() - new Date(left.created_at).getTime()),
      }))
      .filter((group) => {
        if (videoEpisodeFilter === 'all') return true
        const selected = Number(videoEpisodeFilter)
        if (Number.isNaN(selected)) return true
        return group.fallbackEpisodeID === selected
      })
      .sort((left, right) => {
        if (left.episodeNumber == null && right.episodeNumber == null) {
          return (left.fallbackEpisodeID ?? Number.MAX_SAFE_INTEGER) - (right.fallbackEpisodeID ?? Number.MAX_SAFE_INTEGER)
        }
        if (left.episodeNumber == null) return 1
        if (right.episodeNumber == null) return -1
        if (left.episodeNumber !== right.episodeNumber) return left.episodeNumber - right.episodeNumber
        return (left.fallbackEpisodeID ?? 0) - (right.fallbackEpisodeID ?? 0)
      })
  }, [episodeMap, tasks, videoEpisodeFilter])

  const VIDEO_PARAMETER_GROUPS: {
    title: string
    toneClassName: string
    items: { key: string; desc: string }[]
  }[] = [
    {
      title: '当前直达底层模型',
      toneClassName: 'border-sky-200 bg-sky-50/80',
      items: [
        { key: 'model_name', desc: '选择底层视频模型。' },
        { key: 'image_urls', desc: '每张图生成一个片段，按顺序拼接。' },
        { key: 'style_preset', desc: '转换成风格提示词，影响画面质感。' },
        { key: 'motion_mode', desc: '转换成镜头运动提示词，影响动感节奏。' },
        { key: 'duration', desc: '服务端当前固定按约 5 秒/段提交，页面暂未开放自定义。' },
      ],
    },
    {
      title: '合成阶段生效',
      toneClassName: 'border-emerald-200 bg-emerald-50/80',
      items: [
        { key: 'audio_url', desc: '有值时在 FFmpeg 合成阶段加音轨，不是模型原生出声。' },
        { key: 'subtitle_text', desc: '后端支持烧录字幕，属于合成阶段参数。' },
      ],
    },
    {
      title: '任务层记录 / 当前未直达模型',
      toneClassName: 'border-surface-200 bg-surface-50',
      items: [
        { key: 'episode_id', desc: '用于分集任务归属和查询。' },
        { key: 'video_mode', desc: '当前主要用于任务模式标记，未直接传给视频生成器。' },
        { key: 'export_format', desc: '当前主要用于导出格式记录，未改变底层模型请求。' },
      ],
    },
  ]

  const [selectedVideoModel, setSelectedVideoModel] = useState('wan')
  const [selectedVideoMotionMode, setSelectedVideoMotionMode] = useState<(typeof VIDEO_MOTION_OPTIONS)[number]['key']>('gentle')
  const [videoModelAvailability, setVideoModelAvailability] = useState<Record<string, boolean>>({})
  const [videoModelNativeAudio, setVideoModelNativeAudio] = useState<Record<string, boolean>>({})
  const [videoModelParams, setVideoModelParams] = useState<Record<string, { key: string; label: string; default: string; values: { value: string; label: string }[] }[]>>({})
  const [videoParamSelections, setVideoParamSelections] = useState<Record<string, Record<string, string>>>({})
  // Transition effect for video tab generation
  const [vtTransition, setVtTransition] = useState('dissolve')
  const [vtTransitionDuration, setVtTransitionDuration] = useState('0.5')
  // Generation mode: img2video | startEnd2video | reference2video
  const [vtGenerateMode, setVtGenerateMode] = useState('img2video')
  // Native audio flag (only applies to models with audioSupport === 'native')
  const [vtGenerateAudio, setVtGenerateAudio] = useState(false)
  // When the video model embeds native audio, let users opt-in to additionally
  // mix TTS dubbing on top. Default false preserves prior auto-skip behavior.
  const [vtAttachDubbing, setVtAttachDubbing] = useState(false)
  // End frame image URL for startEnd2video mode
  const [vtEndImageURL, setVtEndImageURL] = useState('')
  // Start frame image URL (optional; empty = use scene's own image)
  const [vtStartImageURL, setVtStartImageURL] = useState('')
  // Video param confirmation dialog: null=closed, episodeId>0=single episode, 0=all episodes
  const [vtDialogEpisodeId, setVtDialogEpisodeId] = useState<number | null>(null)
  const openVtDialog = (episodeId: number | 0) => { setVtDialogEpisodeId(episodeId) }

  // 自动审片 dialog
  const [clipDialogEpisodeId, setClipDialogEpisodeId] = useState<number | null>(null)
  const [clipScriptText, setClipScriptText] = useState('')
  const [clipTriggerLoading, setClipTriggerLoading] = useState(false)
  const [clipJobId, setClipJobId] = useState<string | null>(null)
  const [clipTriggerError, setClipTriggerError] = useState<string | null>(null)

  const getVideoParam = (paramKey: string): string => {
    const sel = videoParamSelections[selectedVideoModel] ?? {}
    if (sel[paramKey]) return sel[paramKey]
    const param = (videoModelParams[selectedVideoModel] ?? []).find((p) => p.key === paramKey)
    return param?.default ?? ''
  }
  const setVideoParam = (paramKey: string, value: string) => {
    setVideoParamSelections((prev) => ({
      ...prev,
      [selectedVideoModel]: { ...(prev[selectedVideoModel] ?? {}), [paramKey]: value },
    }))
  }

  React.useEffect(() => {
    videoAPI.modelStatus().then((res) => {
      const avail: Record<string, boolean> = {}
      const audio: Record<string, boolean> = {}
      const params: Record<string, { key: string; label: string; default: string; values: { value: string; label: string }[] }[]> = {}
      res.data.models.forEach((m) => {
        avail[m.key] = m.available
        if (m.native_audio) audio[m.key] = true
        if (m.params && m.params.length > 0) params[m.key] = m.params
      })
      setVideoModelAvailability(avail)
      setVideoModelNativeAudio(audio)
      setVideoModelParams(params)
    }).catch((e) => { console.warn('[videoAPI.modelStatus]', e) })
  }, [])

  const { data: vtModelsData } = useSWR(
    ['video-tab-models', projectId],
    () => modelAPI.list({ type: 'video', sort_by: 'priority' }) as unknown as Promise<{ data: Model[] }>
  )
  const vtVideoModels: Model[] = (vtModelsData as { data?: Model[] })?.data ?? []
  const vtVideoModelOptions = useMemo(
    () => dedupeModels(vtVideoModels.filter((m) => m.is_active)).map(buildVideoModelCapability),
    [vtVideoModels]
  )
  // Filter: only show models that support voice/audio (native or runtime-detected).
  const [vtFilterAudioOnly, setVtFilterAudioOnly] = useState(false)
  const vtVideoModelOptionsVisible = useMemo(() => {
    if (!vtFilterAudioOnly) return vtVideoModelOptions
    return vtVideoModelOptions.filter(
      (m) => m.audioSupport === 'native' || videoModelNativeAudio[m.key] === true
    )
  }, [vtVideoModelOptions, vtFilterAudioOnly, videoModelNativeAudio])
  const VIDEO_STYLE_FAVORITES_STORAGE_KEY = 'project-video-style-favorites'
  const videoModelSelectionStorageKey = `project-video-model-selection:${projectId}`
  const videoMotionSelectionStorageKey = `project-video-motion-selection:${projectId}`
  const videoStyleSelectionStorageKey = `project-video-style-selection:${projectId}`
  const projectConfiguredVideoStyle = normalizeVideoStylePreset(
    typeof project.storyboard_config?.style_preset === 'string' ? project.storyboard_config.style_preset : ''
  )
  const projectConfiguredVideoMotion =
    typeof project.storyboard_config?.motion_mode === 'string' ? project.storyboard_config.motion_mode : ''
  const [selectedVideoStyle, setSelectedVideoStyle] = useState('anime-2d')
  const [selectedVideoStyleFilter, setSelectedVideoStyleFilter] = useState<(typeof VIDEO_STYLE_FILTERS)[number]>('全部')
  const [videoStyleSearch, setVideoStyleSearch] = useState('')
  const [favoriteOnly, setFavoriteOnly] = useState(false)
  const [favoriteVideoStyles, setFavoriteVideoStyles] = useState<string[]>([])

  const favoriteVideoStyleSet = useMemo(() => new Set(favoriteVideoStyles), [favoriteVideoStyles])
  const selectedVideoModelMeta: VideoModelCapability = vtVideoModelOptions.find((m) => m.key === selectedVideoModel)
    ?? vtVideoModelOptions[0]
    ?? { key: selectedVideoModel, label: selectedVideoModel, icon: '🎬', desc: '', provider: '', audioSupport: 'none', aspectRatio: 'unsupported', resolution: 'unsupported', multiVariant: 'unsupported', clipDuration: '', note: '', tags: [], speed: 'medium', quality: 'standard', bestFor: [], supportsStartEnd: false, supportsReference: false }
  const selectedVideoStyleMeta = VIDEO_STYLE_PRESETS.find((style) => style.key === selectedVideoStyle) ?? VIDEO_STYLE_PRESETS[0]
  const selectedVideoStyleCompactMeta = VIDEO_STYLE_COMPACT_OPTIONS.find((style) => style.key === selectedVideoStyle)
  const projectVideoStyleMeta = VIDEO_STYLE_PRESETS.find((style) => style.key === projectConfiguredVideoStyle) ?? selectedVideoStyleMeta
  const projectVideoStyleCompactMeta = VIDEO_STYLE_COMPACT_OPTIONS.find((style) => style.key === projectConfiguredVideoStyle) ?? selectedVideoStyleCompactMeta
  const selectedVideoMotionMeta = VIDEO_MOTION_OPTIONS.find((item) => item.key === selectedVideoMotionMode) ?? VIDEO_MOTION_OPTIONS[0]
  const showVideoCapabilityMatrix = false
  const showVideoStyleLibrary = false
  const videoEpisodeOptions = useMemo(() => {
    const storyboardCounts = new Map<number, { total: number; completed: number; pending: number; firstClipTotal: number; firstClipReady: number }>()
    const taskCounts = new Map<number, { total: number; active: number; failed: number; succeeded: number }>()

    for (const sb of storyboards) {
      if (!sb.episode_id) continue
      const current = storyboardCounts.get(sb.episode_id) ?? { total: 0, completed: 0, pending: 0, firstClipTotal: 0, firstClipReady: 0 }
      current.total += 1
      if (sb.status === 'completed' && sb.image_url) current.completed += 1
      if (sb.status === 'pending') current.pending += 1
      if (isSerialProject && sb.scene_group_key && sb.is_scene_first_clip) {
        current.firstClipTotal += 1
        if (sb.status === 'completed' && sb.image_url) current.firstClipReady += 1
      }
      storyboardCounts.set(sb.episode_id, current)
    }

    for (const task of tasks) {
      if (!task.episode_id) continue
      const current = taskCounts.get(task.episode_id) ?? { total: 0, active: 0, failed: 0, succeeded: 0 }
      current.total += 1
      if (task.status === 'pending' || task.status === 'processing') current.active += 1
      if (task.status === 'failed') current.failed += 1
      if (task.status === 'succeeded') current.succeeded += 1
      taskCounts.set(task.episode_id, current)
    }

    return episodes
      .map((episode) => {
        const storyboard = storyboardCounts.get(episode.id) ?? { total: 0, completed: 0, pending: 0, firstClipTotal: 0, firstClipReady: 0 }
        const task = taskCounts.get(episode.id) ?? { total: 0, active: 0, failed: 0, succeeded: 0 }
        return {
          episodeId: episode.id,
          episodeNumber: episode.episode_number,
          title: episode.title,
          totalStoryboardCount: storyboard.total,
          completedStoryboardCount: storyboard.completed,
          pendingStoryboardCount: storyboard.pending,
          firstClipTotal: storyboard.firstClipTotal,
          firstClipReady: storyboard.firstClipReady,
          canGenerateVideo: isSerialProject ? storyboard.firstClipReady > 0 : storyboard.completed > 0,
          taskCount: task.total,
          activeTaskCount: task.active,
          failedTaskCount: task.failed,
          succeededTaskCount: task.succeeded,
        }
      })
      .filter((item) => item.totalStoryboardCount > 0 || item.taskCount > 0)
      .sort((left, right) => left.episodeNumber - right.episodeNumber)
  }, [episodes, storyboards, tasks, isSerialProject])
  const videoEpisodeOptionMap = useMemo(
    () => new Map(videoEpisodeOptions.map((item) => [item.episodeId, item])),
    [videoEpisodeOptions]
  )

  const projectContextText = useMemo(() => {
    const chunks = [
      project.title,
      project.description,
      project.script_text,
      ...episodes.flatMap((episode) => [episode.title, episode.summary, episode.script_excerpt]),
      ...storyboards.flatMap((storyboard) => [
        storyboard.scene_description,
        storyboard.location,
        storyboard.camera_movement,
        ...(storyboard.characters ?? []),
      ]),
    ]
    return chunks
      .filter(Boolean)
      .join(' ')
      .toLowerCase()
  }, [episodes, project.description, project.script_text, project.title, storyboards])

  const filteredVideoStyles = useMemo(() => {
    const keyword = videoStyleSearch.trim().toLowerCase()
    return VIDEO_STYLE_PRESETS
      .filter((style) =>
        selectedVideoStyleFilter === '全部'
          ? true
          : style.category === selectedVideoStyleFilter || style.families.includes(selectedVideoStyleFilter) || style.modeTags.includes(selectedVideoStyleFilter as VideoStyleMode)
      )
      .filter((style) => (favoriteOnly ? favoriteVideoStyleSet.has(style.key) : true))
      .filter((style) => {
        if (!keyword) return true
        const haystack = [style.label, style.desc, style.category, style.bestFor, style.texture, ...style.modeTags, ...style.tags, ...style.families]
          .join(' ')
          .toLowerCase()
        return haystack.includes(keyword)
      })
      .sort((a, b) => {
        const favoriteDiff = Number(favoriteVideoStyleSet.has(b.key)) - Number(favoriteVideoStyleSet.has(a.key))
        if (favoriteDiff !== 0) return favoriteDiff
        return 0
      })
  }, [favoriteOnly, favoriteVideoStyleSet, selectedVideoStyleFilter, videoStyleSearch])

  const videoStyleRecommendations = useMemo(() => {
    const keywordGroups: Record<string, string[]> = {
      'anime-2d': ['动漫', '二次元', '番剧', '校园', '热血', '少女', '少年', '漫画'],
      'anime-3d': ['3d', '三维', 'cg', '建模', '游戏感', '奇幻', '梦境', '机甲'],
      'live-action-film': ['史诗', '宏大', '战争', '电影', '大片', '悬疑', '赛博', '复古'],
      'live-action-short': ['短剧', '都市', '情感', '对白', '人物', '家庭', '现实', '纪实'],
    }

    return VIDEO_STYLE_PRESETS.map((style) => {
      let score = favoriteVideoStyleSet.has(style.key) ? 1 : 0
      const reasons: string[] = []
      const matchedKeywords = (keywordGroups[style.key] ?? []).filter((term) => projectContextText.includes(term))

      if (style.recommendedModels.includes(selectedVideoModel)) {
        score += 3
        reasons.push(`适配当前模型 ${selectedVideoModelMeta.label}`)
      }
      if (matchedKeywords.length > 0) {
        score += Math.min(4, matchedKeywords.length * 2)
        reasons.push(`匹配项目题材：${matchedKeywords.slice(0, 3).join(' / ')}`)
      }
      if (projectContextText.includes(style.category)) {
        score += 1
      }
      if (style.families.some((family) => projectContextText.includes(family))) {
        score += 1
      }
      if (favoriteVideoStyleSet.has(style.key)) {
        reasons.push('已加入常用风格')
      }
      if (reasons.length === 0) {
        reasons.push(`适合 ${style.bestFor}`)
      }

      return {
        ...style,
        score,
        reasons,
      }
    }).sort((a, b) => b.score - a.score)
  }, [favoriteVideoStyleSet, projectContextText, selectedVideoModel, selectedVideoModelMeta?.label])

  const recommendedVideoStyles = videoStyleRecommendations.slice(0, 3)
  const recommendedVideoStyleKeys = new Set(recommendedVideoStyles.map((style) => style.key))

  useEffect(() => {
    try {
      const raw = window.localStorage.getItem(VIDEO_STYLE_FAVORITES_STORAGE_KEY)
      if (!raw) return
      const parsed = JSON.parse(raw)
      if (!Array.isArray(parsed)) return
      const valid = Array.from(
        new Set(
          parsed
            .filter((key): key is string => typeof key === 'string')
            .map((key) => normalizeVideoStylePreset(key))
            .filter((key) => VIDEO_STYLE_PRESETS.some((style) => style.key === key))
        )
      )
      setFavoriteVideoStyles(valid)
    } catch {
      setFavoriteVideoStyles([])
    }
  }, [])

  useEffect(() => {
    try {
      const saved = window.localStorage.getItem(videoModelSelectionStorageKey)
      if (saved && vtVideoModelOptions.some((model) => model.key === saved)) {
        setSelectedVideoModel(saved)
        return
      }
      // Secondary: check storyboard_config.video_model (persisted across devices)
      const cfgModel = project.storyboard_config?.video_model
      if (cfgModel && vtVideoModelOptions.some((model) => model.key === cfgModel)) {
        setSelectedVideoModel(cfgModel)
        return
      }
      // Fallback: derive runtime key from project.video_model_id
      if (project.video_model_id && vtVideoModels.length > 0) {
        const projectModel = vtVideoModels.find(m => m.id === project.video_model_id)
        const runtimeKey = mapVideoModelToRuntimeKey(projectModel ?? undefined)
        if (runtimeKey && vtVideoModelOptions.some((model) => model.key === runtimeKey)) {
          setSelectedVideoModel(runtimeKey)
        }
      }
    } catch {}
  }, [videoModelSelectionStorageKey, vtVideoModels, project.video_model_id, project.storyboard_config?.video_model])

  useEffect(() => {
    try {
      const saved = window.localStorage.getItem(videoMotionSelectionStorageKey)
      if (saved && VIDEO_MOTION_OPTIONS.some((option) => option.key === saved)) {
        setSelectedVideoMotionMode(saved as (typeof VIDEO_MOTION_OPTIONS)[number]['key'])
        return
      }
    } catch {}
    if (VIDEO_MOTION_OPTIONS.some((option) => option.key === projectConfiguredVideoMotion)) {
      setSelectedVideoMotionMode(projectConfiguredVideoMotion as (typeof VIDEO_MOTION_OPTIONS)[number]['key'])
    }
  }, [projectConfiguredVideoMotion, videoMotionSelectionStorageKey])

  useEffect(() => {
    // Project configured style takes priority over saved preference
    if (VIDEO_STYLE_PRESETS.some((style) => style.key === projectConfiguredVideoStyle)) {
      setSelectedVideoStyle(projectConfiguredVideoStyle)
      return
    }
    try {
      const saved = normalizeVideoStylePreset(window.localStorage.getItem(videoStyleSelectionStorageKey) ?? '')
      if (VIDEO_STYLE_PRESETS.some((style) => style.key === saved)) {
        setSelectedVideoStyle(saved)
      }
    } catch {}
  }, [projectConfiguredVideoStyle, videoStyleSelectionStorageKey])

  useEffect(() => {
    window.localStorage.setItem(VIDEO_STYLE_FAVORITES_STORAGE_KEY, JSON.stringify(favoriteVideoStyles))
  }, [favoriteVideoStyles])

  useEffect(() => {
    window.localStorage.setItem(videoModelSelectionStorageKey, selectedVideoModel)
    // Also persist to storyboard_config so the backend scene-split knows the target model.
    if (selectedVideoModel && project.storyboard_config?.video_model !== selectedVideoModel) {
      projectAPI.update(projectId, {
        storyboard_config: { ...(project.storyboard_config ?? {}), video_model: selectedVideoModel },
      } as Partial<Project>).catch(() => {/* non-critical, ignore errors */})
    }
  }, [selectedVideoModel, videoModelSelectionStorageKey])

  useEffect(() => {
    window.localStorage.setItem(videoMotionSelectionStorageKey, selectedVideoMotionMode)
  }, [selectedVideoMotionMode, videoMotionSelectionStorageKey])

  useEffect(() => {
    window.localStorage.setItem(videoStyleSelectionStorageKey, selectedVideoStyle)
  }, [selectedVideoStyle, videoStyleSelectionStorageKey])

  useEffect(() => {
    if (filteredVideoStyles.length === 0) return
    if (!filteredVideoStyles.some((style) => style.key === selectedVideoStyle)) {
      setSelectedVideoStyle(filteredVideoStyles[0].key)
    }
  }, [filteredVideoStyles, selectedVideoStyle])

  const toggleFavoriteVideoStyle = (styleKey: string) => {
    setFavoriteVideoStyles((current) =>
      current.includes(styleKey) ? current.filter((key) => key !== styleKey) : [styleKey, ...current]
    )
  }

  const handleApplyRecommendedStyle = () => {
    const topStyle = recommendedVideoStyles[0]
    if (!topStyle) return
    setFavoriteOnly(false)
    setSelectedVideoStyleFilter('全部')
    setVideoStyleSearch('')
    setSelectedVideoStyle(topStyle.key)
    toast({ title: `已应用推荐风格：${topStyle.label}`, description: topStyle.reasons[0], variant: 'success' })
  }

  const applyVideoGenerationPreset = (presetKey: string) => {
    const preset = VIDEO_GENERATION_PRESETS.find((item) => item.key === presetKey)
    if (!preset) return
    const modelLabel = vtVideoModelOptions.find((item) => item.key === preset.model)?.label ?? preset.model
    const styleLabel = VIDEO_STYLE_COMPACT_OPTIONS.find((item) => item.key === preset.style)?.label ?? preset.style
    if (vtVideoModelOptions.some((item) => item.key === preset.model)) {
      setSelectedVideoModel(preset.model)
    }
    setSelectedVideoStyle(preset.style)
    setSelectedVideoMotionMode(preset.motion)
    setFavoriteOnly(false)
    setSelectedVideoStyleFilter('全部')
    setVideoStyleSearch('')
    toast({
      title: `已切换为${preset.label}`,
      description: `${preset.hint} · ${modelLabel} / ${styleLabel}`,
      variant: 'success',
    })
  }

  const capabilityBadge = (
    status: 'supported' | 'unsupported' | 'dubbed' | 'native' | 'none'
  ) => {
    if (status === 'supported') return <Badge variant="success" className="text-[10px]">已支持</Badge>
    if (status === 'native') return <Badge variant="success" className="text-[10px]">模型直出</Badge>
    if (status === 'dubbed') return <Badge variant="outline" className="text-[10px] border-green-200 text-green-700">静音片段 / 后配音</Badge>
    if (status === 'none') return <Badge variant="secondary" className="text-[10px]">不支持</Badge>
    return <Badge variant="secondary" className="text-[10px]">未接入</Badge>
  }

  const { data: dubbingRaw } = useSWR(
    ['dubbing-tasks', projectId],
    () => dubbingAPI.listTasks(projectId).then(r => r.data ?? [])
  )
  const videoDubbingTasks: DubbingTask[] = Array.isArray(dubbingRaw) ? dubbingRaw : []

  // Build episode→audio map from succeeded dubbing tasks
  const dubbingAudioMap = new Map<number, string>()
  for (const dt of videoDubbingTasks) {
    if (dt.task_type === 'dubbing' && dt.status === 'succeeded' && dt.audio_url) {
      dubbingAudioMap.set(dt.episode_id, dt.audio_url)
    }
  }

  const [generatingVideoEpisodeIds, setGeneratingVideoEpisodeIds] = useState<Set<number>>(new Set())

  const handleGenerateEpisode = async (episodeId: number) => {
    const episode = episodeMap.get(episodeId)
    const completedSbs = ((await storyboardAPI.listAll(projectId, { episode_id: episodeId, status: 'completed' })) as { data?: Storyboard[] }).data ?? []
    // 串行模式：非首帧分镜无 image_url（由视频服务用前一片段末帧填充），但仍需包含在 clips 列表里以触发串行链。
    // 非串行模式：只包含有 image_url 的分镜。
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
    const isSerial = project.project_type === 'video_serial' || sceneGroupKeys.some(Boolean)

    // 需要至少一张有效的首帧图片才能生成视频
    if (!imageUrls.some(Boolean)) {
      toast({ title: isSerialProject ? '当前集暂无可用首帧图片' : '当前集暂无已完成的分镜图片', variant: 'destructive' })
      return
    }

    setGeneratingVideoEpisodeIds((prev) => new Set(prev).add(episodeId))
    try {
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
        model_name: selectedVideoModel,
        style_preset: selectedVideoStyle,
        motion_mode: selectedVideoMotionMode,
        video_mode: project.video_mode,
        audio_url: dubbingAudioMap.get(episodeId),
        scene_description: sceneDescription || undefined,
        clip_duration_sec: (() => {
          const durSel = videoParamSelections[selectedVideoModel]?.duration
          if (durSel) return parseFloat(durSel)
          return project.storyboard_config?.duration || 5
        })(),
        serial_scene: isSerial || undefined,
        scene_group_keys: isSerial && sceneGroupKeys.some(Boolean) ? sceneGroupKeys : undefined,
        render_config: {
            ...(Object.keys(videoParamSelections[selectedVideoModel] ?? {}).length > 0 ? videoParamSelections[selectedVideoModel] : {}),
            transition: vtTransition !== 'none' ? vtTransition : undefined,
            transition_duration: vtTransition !== 'none' ? parseFloat(vtTransitionDuration) : undefined,
            generate_mode: vtGenerateMode !== 'img2video' ? vtGenerateMode : undefined,
            generate_audio: vtGenerateAudio || undefined, attach_dubbing: vtAttachDubbing || undefined,
            end_image_url: vtGenerateMode === 'startEnd2video' && vtEndImageURL ? vtEndImageURL : undefined,
            start_image_url: vtStartImageURL ? vtStartImageURL : undefined,
          },
      })
      toast({
        title: `第 ${episode?.episode_number ?? episodeId} 集视频生成已启动`,
        description: `${selectedVideoModelMeta.label} / ${selectedVideoStyleMeta.label} / ${selectedVideoMotionMeta.label}`,
        variant: 'success',
      })
      mutateTasks()
    } catch {
      toast({ title: '当前集视频生成失败', variant: 'destructive' })
    } finally {
      setGeneratingVideoEpisodeIds((prev) => {
        const next = new Set(prev)
        next.delete(episodeId)
        return next
      })
    }
  }

  // Called when user clicks "开始生成" in the param confirmation dialog
  const applyVtPreset = (presetKey: string) => {
    const preset = VIDEO_GENERATION_PRESETS.find((item) => item.key === presetKey)
    if (!preset) return
    if (vtVideoModelOptions.some((item) => item.key === preset.model)) setSelectedVideoModel(preset.model)
    setSelectedVideoStyle(preset.style)
    setSelectedVideoMotionMode(preset.motion as (typeof VIDEO_MOTION_OPTIONS)[number]['key'])
  }

  // Called when user clicks "开始生成" in the param confirmation dialog
  const handleVtDialogConfirm = async () => {
    if (vtDialogEpisodeId === null) return
    if (vtDialogEpisodeId === 0) {
      setVtDialogEpisodeId(null)
      await handleGenerateAll()
    } else {
      setVtDialogEpisodeId(null)
      await handleGenerateEpisode(vtDialogEpisodeId)
    }
  }

  const handleRetryTaskWithModel = async (taskId: number, modelName: string) => {
    const modelLabel = vtVideoModelOptions.find((item) => item.key === modelName)?.label ?? modelName
    try {
      await videoAPI.retry(projectId, taskId, modelName)
      toast({ title: `已使用 ${modelLabel} 启动重试`, variant: 'success' })
      mutateTasks()
    } catch {
      toast({ title: '重试失败', variant: 'destructive' })
    }
  }

  const handleRetryClipWithModel = async (taskId: number, clipId: number, clipOrder: number, modelName: string) => {
    const modelLabel = vtVideoModelOptions.find((item) => item.key === modelName)?.label ?? modelName
    try {
      await videoAPI.retryClip(projectId, taskId, clipId, modelName)
      toast({ title: `已使用 ${modelLabel} 重试分镜 ${clipOrder + 1}`, variant: 'success' })
      mutateTasks()
    } catch {
      toast({ title: '分镜重试失败', variant: 'destructive' })
    }
  }

  const handleGenerateAll = async () => {
    const completedSb = ((await storyboardAPI.listAll(projectId, { status: 'completed' })) as { data?: Storyboard[] }).data ?? []
    // 串行模式：非首帧分镜（image_url 为空但有 scene_group_key）也需要包含，
    // 视频服务会用前一片段末帧作为其首帧。
    const eligibleSbs = completedSb
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
      if (!byEpisode.has(epId)) {
        byEpisode.set(epId, []); byEpisodeDesc.set(epId, []); byEpisodeDialogue.set(epId, [])
        byEpisodeDuration.set(epId, []); byEpisodeCamera.set(epId, []); byEpisodeMood.set(epId, [])
        byEpisodeChars.set(epId, [])
        byEpisodeAssetIds.set(epId, [])
        byEpisodeSceneGroupKeys.set(epId, [])
      }
      byEpisode.get(epId)!.push(sb.image_url)
      // Prefer LLM-refined prompt_used; fall back to raw scene_description.
      byEpisodeDesc.get(epId)!.push(sb.prompt_used || sb.scene_description || '')
      byEpisodeDialogue.get(epId)!.push(sb.dialogue || '')
      byEpisodeDuration.get(epId)!.push(sb.duration || 0)
      byEpisodeCamera.get(epId)!.push(sb.camera_movement || '')
      byEpisodeMood.get(epId)!.push(sb.mood || '')
      byEpisodeChars.get(epId)!.push(sb.characters || [])
      byEpisodeAssetIds.get(epId)!.push(sb.asset_ids || [])
      byEpisodeSceneGroupKeys.get(epId)!.push(sb.scene_group_key || '')
    }
    const isSerial = project.project_type === 'video_serial' ||
      eligibleSbs.some((sb) => sb.scene_group_key)
    setGenerating(true)
    try {
        if (byEpisode.size > 1 || (byEpisode.size === 1 && !byEpisode.has(0))) {
          const episodeBatch = Array.from(byEpisode.entries())
            .filter(([epId]) => epId > 0)
            .map(([epId, urls]) => {
              const dlgs = byEpisodeDialogue.get(epId) ?? []
              const durs = byEpisodeDuration.get(epId) ?? []
              const cams = byEpisodeCamera.get(epId) ?? []
              const moods = byEpisodeMood.get(epId) ?? []
              const chars = byEpisodeChars.get(epId) ?? []
              const assetIds = byEpisodeAssetIds.get(epId) ?? []
              const sgKeys = byEpisodeSceneGroupKeys.get(epId) ?? []
              return {
                episode_id: epId,
                image_urls: urls,
                scene_descriptions: byEpisodeDesc.get(epId),
                dialogues: dlgs.some(Boolean) ? dlgs : undefined,
                durations: durs.some(Boolean) ? durs : undefined,
                camera_movements: cams.some(Boolean) ? cams : undefined,
                moods: moods.some(Boolean) ? moods : undefined,
                scene_characters: chars.some((arr) => arr.length > 0) ? chars : undefined,
                scene_asset_ids: assetIds.some((arr) => arr.length > 0) ? assetIds : undefined,
                audio_url: dubbingAudioMap.get(epId),
                scene_description: byEpisodeDesc.get(epId)?.filter(Boolean).join(' ') || undefined,
                scene_group_keys: isSerial && sgKeys.some(Boolean) ? sgKeys : undefined,
              }
            })
          if (episodeBatch.length > 0) {
            await videoAPI.generateBatch(projectId, {
              episodes: episodeBatch,
              model_name: selectedVideoModel,
              style_preset: selectedVideoStyle,
              motion_mode: selectedVideoMotionMode,
              video_mode: project.video_mode,
              clip_duration_sec: (() => { const d = videoParamSelections[selectedVideoModel]?.duration; return d ? parseFloat(d) : (project.storyboard_config?.duration || 5) })(),
              serial_scene: isSerial || undefined,
              render_config: { ...(videoParamSelections[selectedVideoModel] ?? {}), transition: vtTransition !== 'none' ? vtTransition : undefined, transition_duration: vtTransition !== 'none' ? parseFloat(vtTransitionDuration) : undefined, generate_mode: vtGenerateMode !== 'img2video' ? vtGenerateMode : undefined, generate_audio: vtGenerateAudio || undefined, attach_dubbing: vtAttachDubbing || undefined, end_image_url: vtGenerateMode === 'startEnd2video' && vtEndImageURL ? vtEndImageURL : undefined, start_image_url: vtStartImageURL ? vtStartImageURL : undefined },
            })
          }
          const noEpUrls = byEpisode.get(0)
          if (noEpUrls && noEpUrls.length > 0) {
            const noEpDescs = byEpisodeDesc.get(0) ?? []
            const noEpDlgs = byEpisodeDialogue.get(0) ?? []
            const noEpDurs = byEpisodeDuration.get(0) ?? []
            const noEpCams = byEpisodeCamera.get(0) ?? []
            const noEpMoods = byEpisodeMood.get(0) ?? []
            const noEpChars = byEpisodeChars.get(0) ?? []
            const noEpAssetIds = byEpisodeAssetIds.get(0) ?? []
            const noEpSceneGroupKeys = byEpisodeSceneGroupKeys.get(0) ?? []
            await videoAPI.generate(projectId, {
              image_urls: noEpUrls,
              scene_descriptions: noEpDescs,
              dialogues: noEpDlgs.some(Boolean) ? noEpDlgs : undefined,
              durations: noEpDurs.some(Boolean) ? noEpDurs : undefined,
              camera_movements: noEpCams.some(Boolean) ? noEpCams : undefined,
              moods: noEpMoods.some(Boolean) ? noEpMoods : undefined,
              scene_characters: noEpChars.some((arr) => arr.length > 0) ? noEpChars : undefined,
              scene_asset_ids: noEpAssetIds.some((arr) => arr.length > 0) ? noEpAssetIds : undefined,
              model_name: selectedVideoModel,
              style_preset: selectedVideoStyle,
              motion_mode: selectedVideoMotionMode,
              video_mode: project.video_mode,
              scene_description: noEpDescs.filter(Boolean).join(' ') || undefined,
              clip_duration_sec: (() => { const d = videoParamSelections[selectedVideoModel]?.duration; return d ? parseFloat(d) : (project.storyboard_config?.duration || 5) })(),
              serial_scene: isSerial || undefined,
              scene_group_keys: isSerial && noEpSceneGroupKeys.some(Boolean) ? noEpSceneGroupKeys : undefined,
              render_config: { ...(videoParamSelections[selectedVideoModel] ?? {}), transition: vtTransition !== 'none' ? vtTransition : undefined, transition_duration: vtTransition !== 'none' ? parseFloat(vtTransitionDuration) : undefined, generate_mode: vtGenerateMode !== 'img2video' ? vtGenerateMode : undefined, generate_audio: vtGenerateAudio || undefined, attach_dubbing: vtAttachDubbing || undefined, end_image_url: vtGenerateMode === 'startEnd2video' && vtEndImageURL ? vtEndImageURL : undefined, start_image_url: vtStartImageURL ? vtStartImageURL : undefined },
            })
          }
        } else {
          const allUrls = eligibleSbs.map((sb) => sb.image_url)
          // Prefer LLM-refined prompt_used; fall back to raw scene_description.
          const allDescs = eligibleSbs.map((sb) => sb.prompt_used || sb.scene_description || '')
          const allDlgs = eligibleSbs.map((sb) => sb.dialogue || '')
          const allDurs = eligibleSbs.map((sb) => sb.duration || 0)
          const allCams = eligibleSbs.map((sb) => sb.camera_movement || '')
          const allMoods = eligibleSbs.map((sb) => sb.mood || '')
          const allChars = eligibleSbs.map((sb) => sb.characters || [])
          const allAssetIds = eligibleSbs.map((sb) => sb.asset_ids || [])
          const allSceneGroupKeys = eligibleSbs.map((sb) => sb.scene_group_key || '')
          const firstAudio = dubbingAudioMap.values().next().value as string | undefined
          await videoAPI.generate(projectId, {
            image_urls: allUrls,
            scene_descriptions: allDescs,
            dialogues: allDlgs.some(Boolean) ? allDlgs : undefined,
            durations: allDurs.some(Boolean) ? allDurs : undefined,
            camera_movements: allCams.some(Boolean) ? allCams : undefined,
            moods: allMoods.some(Boolean) ? allMoods : undefined,
            scene_characters: allChars.some((arr) => arr.length > 0) ? allChars : undefined,
            scene_asset_ids: allAssetIds.some((arr) => arr.length > 0) ? allAssetIds : undefined,
            model_name: selectedVideoModel,
            style_preset: selectedVideoStyle,
            motion_mode: selectedVideoMotionMode,
            video_mode: project.video_mode,
            audio_url: firstAudio,
            scene_description: allDescs.filter(Boolean).join(' ') || undefined,
            clip_duration_sec: (() => { const d = videoParamSelections[selectedVideoModel]?.duration; return d ? parseFloat(d) : (project.storyboard_config?.duration || 5) })(),
            serial_scene: isSerial || undefined,
            scene_group_keys: isSerial && allSceneGroupKeys.some(Boolean) ? allSceneGroupKeys : undefined,
            render_config: { ...(videoParamSelections[selectedVideoModel] ?? {}), transition: vtTransition !== 'none' ? vtTransition : undefined, transition_duration: vtTransition !== 'none' ? parseFloat(vtTransitionDuration) : undefined, generate_mode: vtGenerateMode !== 'img2video' ? vtGenerateMode : undefined, generate_audio: vtGenerateAudio || undefined, attach_dubbing: vtAttachDubbing || undefined, end_image_url: vtGenerateMode === 'startEnd2video' && vtEndImageURL ? vtEndImageURL : undefined, start_image_url: vtStartImageURL ? vtStartImageURL : undefined },
          })
        }
       toast({ title: `已按“${selectedVideoStyleMeta.label} / ${VIDEO_MOTION_OPTIONS.find((item) => item.key === selectedVideoMotionMode)?.label ?? selectedVideoMotionMode}”启动生成`, variant: 'success' })
       mutateTasks()
     } catch {
       toast({ title: '生成失败', variant: 'destructive' })
    } finally {
      setGenerating(false)
    }
  }

  if (isLoading) return <TabSkeleton />

  // Compute summary
  const totalClips = tasks.reduce((n, t) => n + (t.clips?.length ?? t.image_urls?.length ?? 0), 0)
  const doneClips = tasks.reduce((n, t) => n + (t.clips?.filter(c => c.status === 'succeeded').length ?? 0), 0)
  const processingCount = tasks.filter((t) => t.status === 'processing' || t.status === 'pending' || isComposeRunning(t)).length

  return (
    <div className="space-y-6">
      {/* ── 自动审片 dialog ──────────────────────────────────────────── */}
      <Dialog open={clipDialogEpisodeId !== null} onOpenChange={(open) => {
        if (!open) { setClipDialogEpisodeId(null); setClipJobId(null); setClipScriptText(''); setClipTriggerError(null) }
      }}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>
              {clipDialogEpisodeId !== null
                ? `自动审片 — 第 ${episodeMap.get(clipDialogEpisodeId)?.episode_number ?? clipDialogEpisodeId} 集`
                : '自动审片'}
            </DialogTitle>
          </DialogHeader>
          <div className="space-y-4 pb-2">
            {clipJobId ? (
              <div className="rounded-lg border border-green-200 bg-green-50 px-4 py-3 text-sm text-green-800">
                <div className="flex items-center gap-2">
                  <CheckCircle2 className="h-4 w-4 text-green-600 shrink-0" />
                  <span>审片任务已提交，Job ID：<span className="font-mono font-semibold">{clipJobId}</span></span>
                </div>
                <p className="mt-2 text-xs text-green-700">后台正在自动下载视频片段、分析内容、生成 BGM 建议，完成后结果保存在 clip-service jobs 目录。</p>
              </div>
            ) : (
              <>
                <div className="rounded-lg border border-amber-100 bg-amber-50 px-3 py-2 text-xs text-amber-800">
                  系统将自动获取本集已生成的视频片段，结合剧本内容进行 AI 审片分析，生成剪辑方案与 BGM 建议。
                </div>
                {clipTriggerError && (
                  <div className="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-xs text-red-700">
                    <AlertCircle className="mr-1 inline h-3.5 w-3.5" />{clipTriggerError}
                  </div>
                )}
                <div className="space-y-2">
                  <Label htmlFor="clip-script-text">剧本文本（可选，有助于提升分析质量）</Label>
                  <Textarea
                    id="clip-script-text"
                    placeholder="粘贴本集剧本或简介内容…"
                    rows={5}
                    value={clipScriptText}
                    onChange={(e) => setClipScriptText(e.target.value)}
                    className="resize-none text-sm"
                  />
                </div>
                <div className="flex justify-end gap-2">
                  <Button variant="outline" onClick={() => { setClipDialogEpisodeId(null); setClipScriptText(''); setClipTriggerError(null) }}>取消</Button>
                  <Button
                    disabled={clipTriggerLoading}
                    onClick={async () => {
                      if (clipDialogEpisodeId === null) return
                      setClipTriggerLoading(true)
                      setClipTriggerError(null)
                      try {
                        const res = await videoAPI.triggerClipPipeline(projectId, clipDialogEpisodeId, clipScriptText) as { data?: { job_id?: string }; job_id?: string }
                        const jobId = res?.data?.job_id ?? res?.job_id ?? '(已提交)'
                        setClipJobId(jobId)
                      } catch (e: unknown) {
                        const msg = e instanceof Error ? e.message : '请求失败，请检查 clip-service 是否运行'
                        setClipTriggerError(msg)
                      } finally {
                        setClipTriggerLoading(false)
                      }
                    }}
                  >
                    {clipTriggerLoading ? <Loader2 className="mr-1.5 h-4 w-4 animate-spin" /> : <Film className="mr-1.5 h-4 w-4" />}
                    开始审片
                  </Button>
                </div>
              </>
            )}
          </div>
        </DialogContent>
      </Dialog>

      {/* ── Video parameter confirmation dialog ─────────────────────── */}
      <Dialog open={vtDialogEpisodeId !== null} onOpenChange={(open) => { if (!open) setVtDialogEpisodeId(null) }}>
        <DialogContent className="sm:max-w-xl">
          <DialogHeader>
            <DialogTitle>
              {vtDialogEpisodeId === 0
                ? '全部生成 — 视频参数确认'
                : vtDialogEpisodeId !== null
                  ? `第 ${episodeMap.get(vtDialogEpisodeId)?.episode_number ?? vtDialogEpisodeId} 集 · 视频参数确认`
                  : '视频参数确认'}
            </DialogTitle>
          </DialogHeader>
          <div className="space-y-4 pb-2">
            {/* Presets */}
            <div className="rounded-lg border border-violet-200 bg-violet-50 px-3 py-3">
              <div className="flex items-center justify-between gap-2">
                <div>
                  <p className="text-xs font-semibold text-violet-800">快速质感预设</p>
                  <p className="text-[11px] text-violet-600">一键切换模型 + 风格 + 运动模式。</p>
                </div>
                <Badge variant="outline" className="border-violet-200 bg-white text-[10px] text-violet-700">
                  {selectedVideoModelMeta.icon} {selectedVideoModelMeta.label} / {VIDEO_STYLE_COMPACT_OPTIONS.find(s => s.key === selectedVideoStyle)?.label ?? selectedVideoStyle}
                </Badge>
              </div>
              <div className="mt-3 flex flex-wrap gap-2">
                {VIDEO_GENERATION_PRESETS.map((preset) => {
                  const active = selectedVideoModel === preset.model && selectedVideoStyle === preset.style && selectedVideoMotionMode === preset.motion
                  return (
                    <button key={preset.key} type="button" onClick={() => applyVtPreset(preset.key)}
                      className={`rounded-lg border px-3 py-2 text-left transition-colors ${active ? 'border-violet-300 bg-white text-violet-700 shadow-sm' : 'border-violet-200/70 bg-white/70 text-surface-600 hover:border-violet-300 hover:bg-white'}`}>
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
            {/* Model + Style */}
            <div className="grid gap-4 md:grid-cols-2">
              <div className="space-y-2">
                <Label>生成模型</Label>
                <Select value={selectedVideoModel} onValueChange={setSelectedVideoModel}>
                  <SelectTrigger><SelectValue placeholder="选择视频模型" /></SelectTrigger>
                  <SelectContent className="max-h-[60vh] overflow-y-auto">
                    {vtVideoModelOptions.map((item) => (
                      <SelectItem key={item.key} value={item.key}>
                        {item.icon} {item.label}
                        {item.speed === 'fast' ? ' ⚡' : item.quality === 'high' ? ' ★' : ''}
                        {videoModelAvailability[item.key] === true ? ' ●' : videoModelAvailability[item.key] === false ? ' ○' : ''}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <p className="text-[11px] text-surface-500">{selectedVideoModelMeta.desc}</p>
              </div>
              <div className="space-y-2">
                <Label>画面风格</Label>
                <Select value={selectedVideoStyle} onValueChange={setSelectedVideoStyle}>
                  <SelectTrigger><SelectValue placeholder="选择画面风格" /></SelectTrigger>
                  <SelectContent>
                    {VIDEO_STYLE_COMPACT_OPTIONS.map((item) => (
                      <SelectItem key={item.key} value={item.key}>{item.label} · {item.tone}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-2">
                <Label>运动模式</Label>
                <Select value={selectedVideoMotionMode} onValueChange={(v) => setSelectedVideoMotionMode(v as (typeof VIDEO_MOTION_OPTIONS)[number]['key'])}>
                  <SelectTrigger><SelectValue placeholder="选择运动模式" /></SelectTrigger>
                  <SelectContent>
                    {VIDEO_MOTION_OPTIONS.map((item) => (
                      <SelectItem key={item.key} value={item.key}>{item.label} — {item.desc}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            </div>
            {/* Model-specific params (duration, aspect_ratio, etc.) */}
            {(videoModelParams[selectedVideoModel] ?? []).length > 0 && (
              <div className="rounded-lg border border-blue-100 bg-blue-50/60 px-3 py-3">
                <p className="text-xs font-semibold text-blue-800">📐 模型生成参数</p>
                <p className="mt-0.5 text-[11px] text-blue-600">以下参数直接传给视频模型，影响时长与画幅。</p>
                <div className="mt-2 flex flex-wrap gap-3">
                  {(videoModelParams[selectedVideoModel] ?? []).map((param) => (
                    <div key={param.key} className="flex flex-col gap-1">
                      <label className="text-[11px] font-medium text-blue-700">{param.label}</label>
                      <Select value={getVideoParam(param.key) || param.default} onValueChange={(val) => setVideoParam(param.key, val)}>
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
            )}
            {/* Transition */}
            <div className="rounded-lg border border-purple-100 bg-purple-50/60 px-3 py-3">
              <p className="text-xs font-semibold text-purple-800">🎬 转场效果</p>
              <div className="mt-2 flex flex-wrap items-end gap-3">
                <div className="flex flex-col gap-1">
                  <label className="text-[11px] font-medium text-purple-700">转场类型</label>
                  <Select value={vtTransition} onValueChange={setVtTransition}>
                    <SelectTrigger className="h-8 w-40 border-purple-200 bg-white text-xs"><SelectValue /></SelectTrigger>
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
                {vtTransition !== 'none' && (
                  <div className="flex flex-col gap-1">
                    <label className="text-[11px] font-medium text-purple-700">时长 (秒)</label>
                    <Select value={vtTransitionDuration} onValueChange={setVtTransitionDuration}>
                      <SelectTrigger className="h-8 w-28 border-purple-200 bg-white text-xs"><SelectValue /></SelectTrigger>
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
            {/* Summary */}
            <div className="rounded-lg border border-sky-200 bg-sky-50 px-3 py-2 text-xs text-sky-800">
              将使用：<span className="font-medium">{selectedVideoModelMeta.label}</span> ·
              <span className="ml-1 font-medium">{VIDEO_STYLE_COMPACT_OPTIONS.find(s => s.key === selectedVideoStyle)?.label ?? selectedVideoStyle}</span> ·
              <span className="ml-1 font-medium">{VIDEO_MOTION_OPTIONS.find(m => m.key === selectedVideoMotionMode)?.label ?? selectedVideoMotionMode}</span>
              {vtTransition !== 'none' && <span className="ml-1">· 转场 {vtTransition} {vtTransitionDuration}s</span>}
            </div>
            <div className="flex justify-end gap-2">
              <Button variant="outline" onClick={() => setVtDialogEpisodeId(null)}>取消</Button>
              <Button onClick={handleVtDialogConfirm}
                disabled={vtDialogEpisodeId !== null && vtDialogEpisodeId > 0 && generatingVideoEpisodeIds.has(vtDialogEpisodeId)}>
                {vtDialogEpisodeId !== null && vtDialogEpisodeId > 0 && generatingVideoEpisodeIds.has(vtDialogEpisodeId)
                  ? <Loader2 className="mr-1.5 h-4 w-4 animate-spin" />
                  : <Video className="mr-1.5 h-4 w-4" />}
                开始生成
              </Button>
            </div>
          </div>
        </DialogContent>
      </Dialog>

      {showVideoCapabilityMatrix ? (
        <div className="rounded-xl border border-surface-200 bg-surface-50/70 p-4">
          <div className="flex flex-col gap-2 md:flex-row md:items-center md:justify-between">
            <div>
              <p className="text-sm font-semibold text-surface-800">视频模型能力矩阵</p>
              <p className="text-[11px] text-surface-500">
                以下展示的是当前项目已接入的可配置能力，不是厂商全部官方能力。
              </p>
            </div>
            <Badge variant="outline" className="w-fit">
              当前选中：{selectedVideoModelMeta.icon} {selectedVideoModelMeta.label}
            </Badge>
          </div>
          <div className="mt-4 overflow-x-auto">
            <div className="mb-2 flex items-center justify-between gap-3">
              <label className="flex cursor-pointer items-center gap-1.5 text-[11px] text-surface-600 select-none">
                <input
                  type="checkbox"
                  checked={vtFilterAudioOnly}
                  onChange={(e) => setVtFilterAudioOnly(e.target.checked)}
                  className="h-3.5 w-3.5 rounded border-surface-300 accent-violet-600"
                />
                🔊 仅展示支持原生语音的模型
                <span className="rounded bg-surface-100 px-1 py-0 text-[10px] text-surface-500">
                  {vtVideoModelOptionsVisible.length}/{vtVideoModelOptions.length}
                </span>
              </label>
            </div>
            <table className="min-w-full text-left text-xs">
              <thead>
                <tr className="border-b text-surface-500">
                  <th className="px-2 py-2 font-medium">模型</th>
                  <th className="px-2 py-2 font-medium">速度 / 质量</th>
                  <th className="px-2 py-2 font-medium">标签</th>
                  <th className="px-2 py-2 font-medium">音频方式</th>
                  <th className="px-2 py-2 font-medium">宽高比</th>
                  <th className="px-2 py-2 font-medium">分辨率</th>
                  <th className="px-2 py-2 font-medium">同片段多结果</th>
                  <th className="px-2 py-2 font-medium">首尾帧</th>
                  <th className="px-2 py-2 font-medium">融合生成</th>
                  <th className="px-2 py-2 font-medium">当前片段时长</th>
                </tr>
              </thead>
              <tbody>
                {vtVideoModelOptionsVisible.map((model) => (
                  <tr key={model.key} className={`border-b last:border-b-0 ${selectedVideoModel === model.key ? 'bg-white' : ''}`}>
                    <td className="px-2 py-3">
                      <button
                        type="button"
                        className="text-left"
                        onClick={() => setSelectedVideoModel(model.key)}
                      >
                        <div className="flex items-center gap-2">
                          <span>{model.icon}</span>
                          <div>
                            <div className="flex items-center gap-1.5 flex-wrap">
                              <p className="font-medium text-surface-800">{model.label}</p>
                              {videoModelAvailability[model.key] === true && <span className="rounded bg-emerald-100 px-1 py-0 text-[9px] text-emerald-700">● 可用</span>}
                              {videoModelAvailability[model.key] === false && <span className="rounded bg-red-100 px-1 py-0 text-[9px] text-red-600">● 未配置</span>}
                            </div>
                            <p className="text-[10px] text-surface-400">{model.provider}</p>
                            <p className="text-[10px] text-surface-500 mt-0.5 max-w-[160px]">{model.desc}</p>
                          </div>
                        </div>
                      </button>
                    </td>
                    <td className="px-2 py-3">
                      <div className="flex flex-col gap-1">
                        <span className={`text-[10px] px-1.5 py-0.5 rounded font-medium w-fit ${model.speed === 'fast' ? 'bg-green-100 text-green-700' : model.speed === 'slow' ? 'bg-orange-100 text-orange-700' : 'bg-blue-100 text-blue-700'}`}>
                          {model.speed === 'fast' ? '⚡ 快速' : model.speed === 'slow' ? '🐢 慢速' : '⏱ 均衡'}
                        </span>
                        <span className={`text-[10px] px-1.5 py-0.5 rounded font-medium w-fit ${model.quality === 'high' ? 'bg-purple-100 text-purple-700' : 'bg-surface-100 text-surface-500'}`}>
                          {model.quality === 'high' ? '★ 高质量' : '◎ 标准'}
                        </span>
                      </div>
                    </td>
                    <td className="px-2 py-3">
                      <div className="flex flex-wrap gap-1">
                        {model.tags.map((tag) => (
                          <span key={tag} className="text-[10px] bg-surface-100 text-surface-500 rounded px-1.5 py-0.5">{tag}</span>
                        ))}
                      </div>
                    </td>
                    <td className="px-2 py-3">{capabilityBadge(videoModelNativeAudio[model.key] ? 'native' : model.audioSupport)}</td>
                    <td className="px-2 py-3">{capabilityBadge(model.aspectRatio)}</td>
                    <td className="px-2 py-3">{capabilityBadge(model.resolution)}</td>
                    <td className="px-2 py-3">{capabilityBadge(model.multiVariant)}</td>
                    <td className="px-2 py-3">{capabilityBadge(model.supportsStartEnd ? 'supported' : 'unsupported')}</td>
                    <td className="px-2 py-3">{capabilityBadge(model.supportsReference ? 'supported' : 'unsupported')}</td>
                    <td className="px-2 py-3 text-surface-600">{model.clipDuration}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <div className="mt-4 grid gap-3 rounded-xl border border-surface-200 bg-white p-4 md:grid-cols-2">
            <div className="space-y-2">
              <div className="flex items-center gap-2">
                <span className="text-base">{selectedVideoModelMeta.icon}</span>
                <div>
                  <p className="text-sm font-medium text-surface-800">{selectedVideoModelMeta.label}</p>
                  <p className="text-[11px] text-surface-400">{selectedVideoModelMeta.provider}</p>
                </div>
              </div>
              <p className="text-xs leading-5 text-surface-600">{selectedVideoModelMeta.note}</p>
              {(videoModelNativeAudio[selectedVideoModelMeta.key] ? 'native' : selectedVideoModelMeta.audioSupport) === 'dubbed' ? (
                <div className="rounded-lg border border-amber-200 bg-amber-50 px-3 py-2 text-xs leading-5 text-amber-800">
                  当前接入下，<span className="font-medium">{selectedVideoModelMeta.label}</span> 会先生成静音视频片段；只有任务显式带上
                  <code className="mx-1 rounded bg-white/80 px-1 py-0.5 text-[11px]">audio_url</code>
                  或项目已有配音时，系统才会在合成阶段追加声音。
                </div>
              ) : null}
            </div>
            <div className="grid grid-cols-2 gap-2 text-xs">
              <div className="rounded-lg border border-surface-200 p-3">
                <p className="text-[10px] text-surface-400">音频方式</p>
                <div className="mt-1">{capabilityBadge(videoModelNativeAudio[selectedVideoModelMeta.key] ? 'native' : selectedVideoModelMeta.audioSupport)}</div>
              </div>
              <div className="rounded-lg border border-surface-200 p-3">
                <p className="text-[10px] text-surface-400">宽高比参数</p>
                <div className="mt-1">{capabilityBadge(selectedVideoModelMeta.aspectRatio)}</div>
              </div>
              <div className="rounded-lg border border-surface-200 p-3">
                <p className="text-[10px] text-surface-400">分辨率参数</p>
                <div className="mt-1">{capabilityBadge(selectedVideoModelMeta.resolution)}</div>
              </div>
              <div className="rounded-lg border border-surface-200 p-3">
                <p className="text-[10px] text-surface-400">同片段多结果</p>
                <div className="mt-1">{capabilityBadge(selectedVideoModelMeta.multiVariant)}</div>
              </div>
              <div className="rounded-lg border border-surface-200 p-3">
                <p className="text-[10px] text-surface-400">首尾帧生视频</p>
                <div className="mt-1">{capabilityBadge(selectedVideoModelMeta.supportsStartEnd ? 'supported' : 'unsupported')}</div>
              </div>
              <div className="rounded-lg border border-surface-200 p-3">
                <p className="text-[10px] text-surface-400">融合生视频</p>
                <div className="mt-1">{capabilityBadge(selectedVideoModelMeta.supportsReference ? 'supported' : 'unsupported')}</div>
              </div>
            </div>
          </div>
          {/* Model param selectors: shown when current model has configurable params */}
          {(videoModelParams[selectedVideoModel] ?? []).length > 0 ? (
            <div className="mt-3 rounded-xl border border-blue-100 bg-blue-50/60 p-4">
              <p className="text-xs font-semibold text-blue-800">
                📐 生成参数 — {selectedVideoModelMeta.label}
              </p>
              <p className="mt-0.5 text-[11px] text-blue-600">
                以下参数会在生成视频时一并传给模型，选择后对本次所有分集生效。
              </p>
              <div className="mt-3 flex flex-wrap gap-3">
                {(videoModelParams[selectedVideoModel] ?? [])
                  .map((param) => (
                    <div key={param.key} className="flex flex-col gap-1">
                      <label className="text-[11px] font-medium text-blue-700">{param.label}</label>
                      <Select
                        value={getVideoParam(param.key) || param.default}
                        onValueChange={(val) => setVideoParam(param.key, val)}
                      >
                        <SelectTrigger className="h-8 w-40 border-blue-200 bg-white text-xs">
                          <SelectValue placeholder={`选择${param.label}`} />
                        </SelectTrigger>
                        <SelectContent>
                          {param.values.map((v) => (
                            <SelectItem key={v.value} value={v.value}>
                              {v.label}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </div>
                  ))}
              </div>
            </div>
          ) : null}
          {/* Transition effect selector */}
          <div className="mt-3 rounded-xl border border-purple-100 bg-purple-50/60 p-4">
            <p className="text-xs font-semibold text-purple-800">🎬 转场效果</p>
            <p className="mt-0.5 text-[11px] text-purple-600">控制片段间的过渡动画，dissolve 叠化最为流畅自然，建议保持默认。</p>
            <div className="mt-3 flex flex-wrap items-end gap-3">
              <div className="flex flex-col gap-1">
                <label className="text-[11px] font-medium text-purple-700">转场类型</label>
                <Select value={vtTransition} onValueChange={setVtTransition}>
                  <SelectTrigger className="h-8 w-44 border-purple-200 bg-white text-xs">
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
              {vtTransition !== 'none' && (
                <div className="flex flex-col gap-1">
                  <label className="text-[11px] font-medium text-purple-700">时长 (秒)</label>
                  <Select value={vtTransitionDuration} onValueChange={setVtTransitionDuration}>
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
          {/* Generate mode, audio, and end-frame controls */}
          {(selectedVideoModelMeta.supportsStartEnd || selectedVideoModelMeta.supportsReference || selectedVideoModelMeta.audioSupport === 'native') && (
            <div className="mt-3 rounded-xl border border-blue-100 bg-blue-50/60 p-4">
              <p className="text-xs font-semibold text-blue-800">🎞️ 生成模式</p>
              <p className="mt-0.5 text-[11px] text-blue-600">当前模型支持多种生成方式，可选首尾帧或融合参考生成。</p>
              <div className="mt-3 flex flex-wrap items-end gap-3">
                <div className="flex flex-col gap-1">
                  <label className="text-[11px] font-medium text-blue-700">生成方式</label>
                  <Select value={vtGenerateMode} onValueChange={setVtGenerateMode}>
                    <SelectTrigger className="h-8 w-44 border-blue-200 bg-white text-xs">
                      <SelectValue placeholder="选择生成方式" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="img2video">图生视频（默认）</SelectItem>
                      {selectedVideoModelMeta.supportsStartEnd && (
                        <SelectItem value="startEnd2video">首尾帧生视频</SelectItem>
                      )}
                      {selectedVideoModelMeta.supportsReference && (
                        <SelectItem value="reference2video">融合生视频</SelectItem>
                      )}
                    </SelectContent>
                  </Select>
                </div>
                {selectedVideoModelMeta.audioSupport === 'native' && (
                  <div className="flex flex-col gap-1">
                    <label className="text-[11px] font-medium text-blue-700">原生音频</label>
                    <Select value={vtGenerateAudio ? 'true' : 'false'} onValueChange={(v) => setVtGenerateAudio(v === 'true')}>
                      <SelectTrigger className="h-8 w-32 border-blue-200 bg-white text-xs">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="false">无音频</SelectItem>
                        <SelectItem value="true">生成音频</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                )}
                {selectedVideoModelMeta.audioSupport === 'native' && (
                  <div className="flex flex-col gap-1">
                    <label className="text-[11px] font-medium text-blue-700" title="模型已生成原生音频时，是否再叠加 TTS 配音">叠加配音</label>
                    <Select value={vtAttachDubbing ? 'true' : 'false'} onValueChange={(v) => setVtAttachDubbing(v === 'true')}>
                      <SelectTrigger className="h-8 w-32 border-blue-200 bg-white text-xs">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="false">跳过（默认）</SelectItem>
                        <SelectItem value="true">叠加 TTS 配音</SelectItem>
                      </SelectContent>
                    </Select>
                  </div>
                )}
              </div>
              {vtGenerateMode === 'startEnd2video' && (
                <div className="mt-3 grid gap-3 sm:grid-cols-2">
                  <div>
                    <label className="text-[11px] font-medium text-blue-700">起始帧图片 URL（可选）</label>
                    <input
                      type="text"
                      className="mt-1 w-full rounded-md border border-blue-200 bg-white px-3 py-1.5 text-xs text-surface-800 placeholder:text-surface-400 focus:outline-none focus:ring-1 focus:ring-blue-300"
                      placeholder="留空则使用分镜自身图像作为起始帧"
                      value={vtStartImageURL}
                      onChange={(e) => setVtStartImageURL(e.target.value)}
                    />
                    {vtStartImageURL && (
                      <div className="mt-2 inline-block rounded-md border border-blue-100 bg-white p-1">
                        <ZoomableImage src={vtStartImageURL} alt="起始帧预览" className="h-20 w-32 rounded object-cover" />
                      </div>
                    )}
                  </div>
                  <div>
                    <label className="text-[11px] font-medium text-blue-700">结尾帧图片 URL <span className="text-red-500">*</span></label>
                    <input
                      type="text"
                      className="mt-1 w-full rounded-md border border-blue-200 bg-white px-3 py-1.5 text-xs text-surface-800 placeholder:text-surface-400 focus:outline-none focus:ring-1 focus:ring-blue-300"
                      placeholder="输入结尾帧图片 URL（首尾帧生视频必填）"
                      value={vtEndImageURL}
                      onChange={(e) => setVtEndImageURL(e.target.value)}
                    />
                    {vtEndImageURL && (
                      <div className="mt-2 inline-block rounded-md border border-blue-100 bg-white p-1">
                        <ZoomableImage src={vtEndImageURL} alt="结尾帧预览" className="h-20 w-32 rounded object-cover" />
                      </div>
                    )}
                  </div>
                  <p className="text-[11px] text-blue-500 sm:col-span-2">📌 首尾帧视频：起始帧默认为当前分镜图像，可用 URL 覆盖；结尾帧必填，建议使用下一分镜图像以获得自然转场。</p>
                </div>
              )}
              {vtGenerateMode === 'reference2video' && (
                <p className="mt-2 text-[11px] text-blue-500">📌 融合生视频将自动使用项目人物角色图作为参考图，无需额外设置。</p>
              )}
            </div>
          )}
          <div className="mt-3 grid gap-3 xl:grid-cols-3">
            {VIDEO_PARAMETER_GROUPS.map((group) => (
              <div key={group.title} className={`rounded-xl border p-4 ${group.toneClassName}`}>
                <p className="text-xs font-semibold text-surface-800">{group.title}</p>
                <div className="mt-3 space-y-2">
                  {group.items.map((item) => (
                    <div key={item.key} className="rounded-lg border border-white/70 bg-white/80 px-3 py-2">
                      <code className="rounded bg-surface-100 px-1.5 py-0.5 text-[11px] text-surface-700">{item.key}</code>
                      <p className="mt-1 text-[11px] leading-5 text-surface-600">{item.desc}</p>
                    </div>
                  ))}
                </div>
              </div>
            ))}
          </div>
        </div>
      ) : null}

      {showVideoStyleLibrary ? (
      <div className="rounded-xl border border-surface-200 bg-white p-4">
        <div className="flex flex-col gap-2 md:flex-row md:items-center md:justify-between">
          <div>
            <p className="text-sm font-semibold text-surface-800">视频风格库</p>
            <p className="text-[11px] text-surface-500">
              风格会直接写入当前视频生成提示词，用于统一画面气质、镜头氛围和材质表现。
            </p>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <Badge variant="outline" className="w-fit">
              当前风格：{selectedVideoStyleMeta.label} · {selectedVideoStyleMeta.modeTags.join(' / ')}
            </Badge>
            <Badge variant="outline" className="w-fit border-surface-200 bg-surface-50 text-surface-600">
              分类：{selectedVideoStyleMeta.category}
            </Badge>
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={handleApplyRecommendedStyle}
              disabled={recommendedVideoStyles.length === 0}
              className="h-8"
            >
              <Sparkles className="mr-1.5 h-3.5 w-3.5" />
              一键应用推荐
            </Button>
          </div>
        </div>
        <div className="mt-4 rounded-xl border border-surface-200 bg-surface-50/70 p-4">
          <div className="flex flex-col gap-2 md:flex-row md:items-center md:justify-between">
            <div>
              <p className="text-sm font-medium text-surface-800">视频生成参数</p>
              <p className="text-[11px] text-surface-500">
                创建项目时预设的默认值会自动带到这里，生成前也可以随时调整。
              </p>
            </div>
            <Badge variant="outline" className="w-fit">
              当前模式：{project.video_mode === 'api_generation' ? 'API生成' : '逐帧动画'}
            </Badge>
          </div>
          <div className="mt-4 grid gap-3 md:grid-cols-3">
            {VIDEO_MOTION_OPTIONS.map((option) => {
              const active = option.key === selectedVideoMotionMode
              return (
                <button
                  key={option.key}
                  type="button"
                  onClick={() => setSelectedVideoMotionMode(option.key)}
                  className={`rounded-lg border px-3 py-3 text-left transition-colors ${
                    active
                      ? 'border-primary-300 bg-primary-50'
                      : 'border-surface-200 bg-white hover:border-surface-300'
                  }`}
                >
                  <div className="flex items-center justify-between gap-2">
                    <span className="text-sm font-medium text-surface-800">{option.label}</span>
                    {active ? <CheckCircle2 className="h-4 w-4 text-primary-500" /> : null}
                  </div>
                  <p className="mt-1 text-[11px] leading-5 text-surface-500">{option.desc}</p>
                </button>
              )
            })}
          </div>
        </div>
        <div className="mt-4 flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
          <div className="relative w-full lg:max-w-sm">
            <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-surface-400" />
            <Input
              value={videoStyleSearch}
              onChange={(e) => setVideoStyleSearch(e.target.value)}
              placeholder="搜索风格、题材、质感、标签"
              className="pl-9"
            />
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <button
              type="button"
              onClick={() => setFavoriteOnly((current) => !current)}
              className={`inline-flex items-center gap-1 rounded-full border px-3 py-1.5 text-xs transition-colors ${
                favoriteOnly
                  ? 'border-amber-300 bg-amber-50 text-amber-700'
                  : 'border-surface-200 bg-surface-50 text-surface-500 hover:border-surface-300 hover:bg-white'
              }`}
            >
              <Star className={`h-3.5 w-3.5 ${favoriteOnly ? 'fill-current' : ''}`} />
              只看常用
            </button>
            <span className="text-[11px] text-surface-400">已收藏 {favoriteVideoStyles.length} 个</span>
          </div>
        </div>
        <div className="mt-4 grid gap-2 md:grid-cols-2 xl:grid-cols-4">
          {(['2维动漫', '3维动漫', '真人电影', '真人短剧'] as VideoStyleMode[]).map((mode) => (
            <div
              key={mode}
              className={`rounded-xl border px-3 py-2 ${VIDEO_STYLE_MODE_META[mode].className}`}
            >
              <div className="text-xs font-semibold">{VIDEO_STYLE_MODE_META[mode].label}模式</div>
              <div className="mt-1 text-[11px] opacity-90">{VIDEO_STYLE_MODE_META[mode].hint}</div>
            </div>
          ))}
        </div>
        <div className="mt-4 grid gap-3 lg:grid-cols-3">
          {recommendedVideoStyles.map((style, index) => (
            <button
              key={style.key}
              type="button"
              onClick={() => setSelectedVideoStyle(style.key)}
              className={`rounded-xl border p-3 text-left transition-colors ${
                selectedVideoStyle === style.key
                  ? 'border-primary-300 bg-primary-50'
                  : 'border-surface-200 bg-surface-50 hover:border-surface-300 hover:bg-white'
              }`}
            >
              <div className="flex items-center justify-between gap-2">
                <div className="flex items-center gap-2">
                  <span className="inline-flex h-5 min-w-5 items-center justify-center rounded-full bg-primary-100 px-1.5 text-[10px] font-semibold text-primary-700">
                    TOP {index + 1}
                  </span>
                  <p className="text-sm font-medium text-surface-800">{style.label}</p>
                </div>
                <Sparkles className="h-4 w-4 text-primary-500" />
              </div>
              <p className="mt-2 line-clamp-2 text-[11px] leading-5 text-surface-500">{style.reasons[0]}</p>
              <div className="mt-2 flex flex-wrap gap-1.5">
                {style.reasons.slice(0, 2).map((reason) => (
                  <span key={reason} className="rounded-full bg-white px-2 py-0.5 text-[10px] text-surface-500">
                    {reason}
                  </span>
                ))}
              </div>
              {index === 0 ? (
                <div className="mt-3 text-[10px] font-medium text-primary-600">
                  点击右上角“一键应用推荐”可设为当前项目默认风格
                </div>
              ) : null}
            </button>
          ))}
        </div>
        <div className="mt-4 flex flex-wrap gap-2">
          {VIDEO_STYLE_FILTERS.map((filter) => {
            const active = filter === selectedVideoStyleFilter
            return (
              <button
                key={filter}
                type="button"
                onClick={() => setSelectedVideoStyleFilter(filter)}
                className={`rounded-full border px-3 py-1 text-xs transition-colors ${
                  active
                    ? 'border-primary-300 bg-primary-50 text-primary-700'
                    : 'border-surface-200 bg-surface-50 text-surface-500 hover:border-surface-300 hover:bg-white'
                }`}
              >
                {filter}
              </button>
            )
          })}
        </div>
        <div className="mt-3 flex items-center justify-between text-[11px] text-surface-400">
          <span>当前筛选：{selectedVideoStyleFilter}{videoStyleSearch ? ` · 搜索“${videoStyleSearch}”` : ''}</span>
          <span>{filteredVideoStyles.length} 个风格{favoriteOnly ? ' · 常用模式' : ''}</span>
        </div>
        <div className="mt-4 grid gap-3 md:grid-cols-2 xl:grid-cols-3">
          {filteredVideoStyles.map((style) => {
            const active = style.key === selectedVideoStyle
            const isFavorite = favoriteVideoStyleSet.has(style.key)
            const isRecommended = recommendedVideoStyleKeys.has(style.key)
            const recommendation = videoStyleRecommendations.find((item) => item.key === style.key)
            return (
              <div
                key={style.key}
                role="button"
                tabIndex={0}
                onClick={() => setSelectedVideoStyle(style.key)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' || e.key === ' ') {
                    e.preventDefault()
                    setSelectedVideoStyle(style.key)
                  }
                }}
                className={`rounded-xl border p-4 text-left transition-all ${
                  active
                    ? 'border-primary-300 bg-primary-50 shadow-sm'
                    : 'border-surface-200 bg-surface-50/50 hover:border-surface-300 hover:bg-white'
                }`}
              >
                <div className="flex items-start justify-between gap-3">
                  <div>
                    <div className="flex flex-wrap items-center gap-2">
                      <p className="text-sm font-medium text-surface-800">{style.label}</p>
                      {style.modeTags.map((mode) => (
                        <span
                          key={mode}
                          className={`rounded-full border px-2.5 py-0.5 text-[10px] font-semibold ${VIDEO_STYLE_MODE_META[mode].className}`}
                        >
                          {mode}
                        </span>
                      ))}
                      <span className="rounded-full border border-surface-200 bg-surface-100 px-2 py-0.5 text-[10px] text-surface-600">
                        {style.category}
                      </span>
                      {isRecommended ? (
                        <span className="rounded-full bg-primary-100 px-2 py-0.5 text-[10px] text-primary-700">
                          推荐
                        </span>
                      ) : null}
                    </div>
                    <p className="mt-1 text-[11px] leading-5 text-surface-500">{style.desc}</p>
                    {recommendation?.reasons?.[0] ? (
                      <p className="mt-1 text-[10px] text-primary-600">{recommendation.reasons[0]}</p>
                    ) : null}
                  </div>
                  <div className="flex items-center gap-2">
                    <button
                      type="button"
                      aria-label={isFavorite ? `取消收藏${style.label}` : `收藏${style.label}`}
                      onClick={(e) => {
                        e.stopPropagation()
                        toggleFavoriteVideoStyle(style.key)
                      }}
                      className={`rounded-full border p-1 transition-colors ${
                        isFavorite
                          ? 'border-amber-300 bg-amber-50 text-amber-600'
                          : 'border-surface-200 bg-white text-surface-400 hover:border-amber-200 hover:text-amber-500'
                      }`}
                    >
                      <Star className={`h-3.5 w-3.5 ${isFavorite ? 'fill-current' : ''}`} />
                    </button>
                    {active ? <CheckCircle2 className="h-4 w-4 shrink-0 text-primary-500" /> : null}
                  </div>
                </div>
                <div className="mt-3 grid gap-2 text-[11px] text-surface-500">
                  <div className="rounded-lg bg-white/70 px-2.5 py-2">
                    <p className="text-[10px] text-surface-400">适合内容</p>
                    <p className="mt-1 leading-5">{style.bestFor}</p>
                  </div>
                  <div className="rounded-lg bg-white/70 px-2.5 py-2">
                    <p className="text-[10px] text-surface-400">画面质感</p>
                    <p className="mt-1 leading-5">{style.texture}</p>
                  </div>
                </div>
                <div className="mt-3 flex flex-wrap gap-1.5">
                  {style.tags.map((tag) => (
                    <span key={tag} className={`rounded-full px-2 py-0.5 text-[10px] ${
                      active ? 'bg-primary-100 text-primary-700' : 'bg-white text-surface-500'
                    }`}>
                      {tag}
                    </span>
                  ))}
                  {style.families.map((family) => (
                    <span key={family} className={`rounded-full border px-2 py-0.5 text-[10px] ${
                      active
                        ? 'border-primary-200 text-primary-700'
                        : 'border-surface-200 text-surface-400'
                    }`}>
                      {family}
                    </span>
                  ))}
                </div>
              </div>
            )
          })}
        </div>
        {filteredVideoStyles.length === 0 ? (
          <div className="mt-4 rounded-xl border border-dashed border-surface-200 bg-surface-50 px-4 py-6 text-center text-sm text-surface-500">
            没有找到匹配的风格，试试切换分类、取消“只看常用”或换个关键词。
          </div>
        ) : null}
      </div>
      ) : null}

      {/* Header */}
      <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-2">
        <div className="flex flex-wrap items-center gap-2">
          <span className="text-sm text-surface-500">共 {tasks.length} 个任务 · 按 {videoTaskGroups.length} 组展示</span>
          {processingCount > 0 && (
            <span className="flex items-center gap-1 text-xs text-blue-600">
              <Loader2 className="h-3 w-3 animate-spin" />
              {processingCount} 个进行中 · 片段 {doneClips}/{totalClips}
            </span>
          )}
          {composingCount > 0 && (
            <span className="flex items-center gap-1 text-xs text-purple-600">
              <Film className="h-3 w-3" />
              合成中 {composingCount}
            </span>
          )}
          {dubbingAudioMap.size > 0 ? (
            <span className="flex items-center gap-1 text-xs text-green-600">
              <Mic className="h-3 w-3" /> 配音就绪 ({dubbingAudioMap.size} 集，生成后自动合成)
            </span>
          ) : (
            <span className="flex items-center gap-1 text-xs text-surface-400">
              <Mic className="h-3 w-3" /> 无配音，生成后为静音
            </span>
          )}
          <span className="rounded bg-purple-100 px-1.5 py-0.5 text-[11px] text-purple-700">
            {projectVideoStyleMeta.label} / {projectVideoStyleMeta.category}{projectVideoStyleCompactMeta ? ` · ${projectVideoStyleCompactMeta.hint}` : ''}
          </span>
        </div>
        <div className="flex items-center gap-2">
          <select
            className="rounded-md border border-surface-200 bg-white px-2 py-1.5 text-xs"
            value={selectedVideoModel}
            onChange={(e) => setSelectedVideoModel(e.target.value)}
          >
            {vtVideoModelOptions.map((m) => (
              <option key={m.key} value={m.key}>{m.icon} {m.label}{videoModelAvailability[m.key] === true ? ' ●' : videoModelAvailability[m.key] === false ? ' ○' : ''} ({getProviderLabel(m.provider)})</option>
            ))}
          </select>
          <select
            className="min-w-[200px] rounded-md border border-surface-200 bg-white px-2 py-1.5 text-xs"
            value={selectedVideoStyle}
            onChange={(e) => setSelectedVideoStyle(e.target.value)}
          >
            {VIDEO_STYLE_COMPACT_OPTIONS.map((style) => (
              <option key={style.key} value={style.key}>
                {style.label} · {style.tone}
              </option>
            ))}
          </select>
        </div>
      </div>

      {/* Task list */}
      {tasks.length === 0 ? (
        <div className="flex flex-col items-center justify-center py-16">
          <Video className="mb-3 h-12 w-12 text-surface-300" />
          <p className="text-sm text-surface-400">暂无视频任务，请先生成</p>
        </div>
      ) : (
        <div className="space-y-3">
          {videoTaskGroups.map((group) => {
            const groupActiveCount = group.tasks.filter((task) => task.status === 'processing' || task.status === 'pending').length
            const groupSucceededCount = group.tasks.filter((task) => task.status === 'succeeded').length
            const groupEpisodeId = group.fallbackEpisodeID ?? null
            const groupEpisodeOption = groupEpisodeId ? videoEpisodeOptionMap.get(groupEpisodeId) : undefined
            return (
              <div key={group.key} className="space-y-3 rounded-xl border border-surface-200 bg-surface-50/60 p-3">
                <div className="flex flex-wrap items-center justify-between gap-2 rounded-lg bg-white/90 px-3 py-2">
                  <div className="flex items-center gap-2">
                    <Badge variant="outline" className="border-surface-200 bg-surface-50 text-surface-700">
                      {group.label}
                    </Badge>
                    <span className="text-xs text-surface-500">{group.tasks.length} 个任务</span>
                    {groupSucceededCount > 0 ? <span className="text-xs text-green-600">已完成 {groupSucceededCount}</span> : null}
                    {groupActiveCount > 0 ? <span className="text-xs text-blue-600">进行中 {groupActiveCount}</span> : null}
                    {groupEpisodeOption ? <span className="text-xs text-surface-400" title={groupEpisodeOption.pendingStoryboardCount > 0 ? `待完成镜头: ${groupEpisodeOption.pendingStoryboardCount}` : undefined}>{isSerialProject ? `首帧就绪 ${groupEpisodeOption.firstClipReady}/${groupEpisodeOption.firstClipTotal || 0}` : `可生成分镜 ${groupEpisodeOption.completedStoryboardCount}/${groupEpisodeOption.totalStoryboardCount}`}{!groupEpisodeOption.canGenerateVideo && groupEpisodeOption.pendingStoryboardCount > 0 ? (isSerialProject ? '（需先准备首帧）' : '（需先生成分镜）') : ''}</span> : null}
                  </div>
                  <div className="flex items-center gap-2">
                    {groupEpisodeId ? (
                      <>
                        <Button
                          size="sm"
                          variant="outline"
                          onClick={() => openVtDialog(groupEpisodeId)}
                          disabled={
                            generatingVideoEpisodeIds.has(groupEpisodeId) ||
                            !groupEpisodeOption ||
                            !groupEpisodeOption.canGenerateVideo
                          }
                          title={!groupEpisodeOption || !groupEpisodeOption.canGenerateVideo ? (!groupEpisodeOption ? (isSerialProject ? '当前集暂无镜头数据' : '当前集暂无分镜数据') : isSerialProject ? groupEpisodeOption.pendingStoryboardCount > 0 ? `该集有 ${groupEpisodeOption.pendingStoryboardCount} 个镜头待完成，请先在镜头标签页准备场景首帧` : groupEpisodeOption.firstClipTotal > 0 ? `该集已有 ${groupEpisodeOption.firstClipTotal} 个场景首帧位，但尚无可用首帧图片` : '该集尚未完成场景分组或首帧准备' : groupEpisodeOption.pendingStoryboardCount > 0 ? `该集有 ${groupEpisodeOption.pendingStoryboardCount} 个分镜待生成，请先在分镜标签页生成分镜图片` : `该集共 ${groupEpisodeOption.totalStoryboardCount} 个分镜，尚无已完成且有图片的分镜`) : (isSerialProject ? '确认视频参数后生成本集串行视频' : '确认视频参数后生成本集视频')}
                        >
                          {generatingVideoEpisodeIds.has(groupEpisodeId) ? (
                            <Loader2 className="mr-1 h-3.5 w-3.5 animate-spin" />
                          ) : (
                            <Video className="mr-1 h-3.5 w-3.5" />
                          )}
                          生成本集
                        </Button>
                        <Button
                          size="sm"
                          variant="outline"
                          onClick={() => { setClipDialogEpisodeId(groupEpisodeId); setClipJobId(null); setClipScriptText(project.script_text ?? '') }}
                          title="对本集已生成的视频片段进行 AI 自动审片"
                          disabled={groupSucceededCount === 0}
                        >
                          <Film className="mr-1 h-3.5 w-3.5" />
                          自动审片
                        </Button>
                      </>
                    ) : null}
                  </div>
                </div>
                {group.tasks.map((t) => {
                  const ep = t.episode_id ? episodeMap.get(t.episode_id) : null
                  const clips = t.clips ?? []
                  const clipsDone = clips.filter(c => c.status === 'succeeded').length
                  const clipsFailed = clips.filter(c => c.status === 'failed').length
                  const clipsTotal = clips.length || t.image_urls?.length || 0
                  const progress = clipsTotal > 0 ? Math.round((clipsDone / clipsTotal) * 100) : 0
                  const isActive = t.status === 'processing' || t.status === 'pending' || isComposeRunning(t)

                  return (
                    <Card key={t.id} className={`transition-shadow hover:shadow-sm ${isActive ? 'border-blue-200 bg-blue-50/30' : ''}`}>
                      <CardContent className="p-4">
                        <div className="flex items-center justify-between">
                          <div className="flex items-center gap-4">
                            <div className="flex h-14 w-24 items-center justify-center overflow-hidden rounded-md bg-surface-900 text-surface-400">
                              {t.status === 'succeeded' && (t.result_url || t.hls_url) ? (
                                <button onClick={() => setPreviewUrl(t.result_url || t.hls_url)} className="flex h-full w-full items-center justify-center hover:text-white transition-colors">
                                  <Play className="h-6 w-6 text-white" />
                                </button>
                              ) : isActive ? (
                                <Loader2 className="h-5 w-5 animate-spin text-blue-400" />
                              ) : (
                                <Video className="h-5 w-5" />
                              )}
                            </div>
                            <div className="space-y-1">
                              <p className="text-sm font-medium">
                                {ep ? `第 ${ep.episode_number} 集 · ${ep.title}` : t.episode_id ? `第 ${t.episode_id} 集` : `任务 #${t.id}`}
                              </p>
                              <div className="flex items-center gap-3 text-xs text-surface-400">
                                <VideoTaskStatusBadge status={t.status} />
                                <span>{t.model_name}</span>
                                {t.serial_scene && (
                                  <span className="rounded-full bg-indigo-100 px-2 py-0.5 text-[10px] font-medium text-indigo-600">
                                    串行模式{t.scene_group_keys?.filter(Boolean).length ? ` · ${t.scene_group_keys!.filter(Boolean).length} 场景组` : ''}
                                  </span>
                                )}
                                {t.style_preset ? <span>风格 {VIDEO_STYLE_PRESETS.find((style) => style.key === normalizeVideoStylePreset(t.style_preset))?.label || normalizeVideoStylePreset(t.style_preset)}</span> : null}
                                {t.duration_sec > 0 && <span>{formatDuration(t.duration_sec)}</span>}
                                <span className="text-surface-300">{new Date(t.created_at).toLocaleString('zh-CN', { month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit' })}</span>
                              </div>
                              {t.error_msg && <p className="text-xs text-red-500 truncate max-w-md">{t.error_msg}</p>}
                            </div>
                          </div>
                          <div className="flex items-center gap-3">
                            {clipsTotal > 0 && (
                              <div className="text-right">
                                <div className="flex items-center gap-2">
                                  <Progress value={progress} className="h-1.5 w-28" />
                                  <span className="text-xs text-surface-500 whitespace-nowrap">{clipsDone}/{clipsTotal}</span>
                                </div>
                                {isActive && clipsDone < clipsTotal && (
                                  <span className="flex items-center justify-end text-xs text-blue-500 whitespace-nowrap">
                                    <Clock className="mr-0.5 h-3 w-3" />
                                    约{(() => { const s = (clipsTotal - clipsDone) * 30; return s >= 60 ? `${Math.round(s / 60)}分钟` : `${s}秒` })()}
                                  </span>
                                )}
                                {clipsFailed > 0 && <span className="text-xs text-red-500">{clipsFailed} 失败</span>}
                              </div>
                            )}
                            {t.status === 'failed' && (
                              <DropdownMenu>
                                <DropdownMenuTrigger asChild>
                                  <Button size="sm" variant="outline">
                                    <RefreshCw className="mr-1 h-3.5 w-3.5" /> 选择模型重试
                                  </Button>
                                </DropdownMenuTrigger>
                                <DropdownMenuContent align="end" className="w-80">
                                  <DropdownMenuLabel className="text-[10px] text-surface-400">选择重试模型</DropdownMenuLabel>
                                  <DropdownMenuSeparator />
                                  {vtVideoModelOptions.map((item) => {
                                    const avail = videoModelAvailability[item.key]
                                    return (
                                    <DropdownMenuItem key={item.key} className={`cursor-pointer px-3 py-2 ${avail === false ? 'opacity-50' : ''}`} onClick={() => handleRetryTaskWithModel(t.id, item.key)}>
                                      <div className="flex items-start gap-2 w-full">
                                        <span className="text-base mt-0.5">{item.icon}</span>
                                        <div className="flex flex-col gap-0.5 min-w-0 flex-1">
                                          <div className="flex items-center gap-1.5 flex-wrap">
                                            <span className="text-xs font-semibold">{item.label}</span>
                                            <span className={`text-[10px] px-1 py-0 rounded font-medium ${item.speed === 'fast' ? 'bg-green-100 text-green-700' : item.speed === 'slow' ? 'bg-orange-100 text-orange-700' : 'bg-blue-100 text-blue-700'}`}>
                                              {item.speed === 'fast' ? '快速' : item.speed === 'slow' ? '慢速' : '均衡'}
                                            </span>
                                            <span className={`text-[10px] px-1 py-0 rounded font-medium ${item.quality === 'high' ? 'bg-purple-100 text-purple-700' : 'bg-surface-100 text-surface-500'}`}>
                                              {item.quality === 'high' ? '高质量' : '标准'}
                                            </span>
                                            {avail === true && <span className="rounded bg-emerald-100 px-1 py-0 text-[9px] text-emerald-700">● 可用</span>}
                                            {avail === false && <span className="rounded bg-red-100 px-1 py-0 text-[9px] text-red-600">● 未配置</span>}
                                          </div>
                                          <span className="text-[10px] text-surface-400 leading-none">{getProviderLabel(item.provider)}</span>
                                          <span className="text-[11px] text-surface-500 leading-snug">{item.desc}</span>
                                          <div className="flex flex-wrap gap-1 mt-0.5">
                                            {item.tags.map((tag) => (
                                              <span key={tag} className="text-[10px] bg-surface-100 text-surface-500 rounded px-1">{tag}</span>
                                            ))}
                                          </div>
                                        </div>
                                      </div>
                                    </DropdownMenuItem>
                                    )
                                  })}
                                </DropdownMenuContent>
                              </DropdownMenu>
                            )}
                            {clipsDone > 0 && clipsDone === clipsTotal && !isActive && (
                              <Button size="sm" variant="outline" title="将所有片段合成为完整视频" onClick={async () => {
                                try {
                                  await videoAPI.compose(t.id)
                                  markTaskComposing(t.id)
                                  toast({ title: '合成已启动', variant: 'success' })
                                  mutateTasks()
                                } catch { toast({ title: '合成失败', variant: 'destructive' }) }
                              }}>
                                <Film className="mr-1 h-3.5 w-3.5" /> 合成
                              </Button>
                            )}
                            {t.status === 'succeeded' && (t.result_url || t.hls_url) && (
                              <>
                                <Button size="sm" variant="ghost" onClick={() => setPreviewUrl(t.result_url || t.hls_url)} title="预览视频">
                                  <Play className="h-4 w-4" />
                                </Button>
                                <a href={t.result_url || t.hls_url} download target="_blank" rel="noopener noreferrer">
                                  <Button size="sm" variant="ghost" title="下载视频">
                                    <Download className="h-4 w-4" />
                                  </Button>
                                </a>
                              </>
                            )}
                            {(t.status === 'pending' || t.status === 'processing') && (
                              <Button
                                size="sm"
                                variant="ghost"
                                className="text-orange-500 hover:text-orange-700 hover:bg-orange-50"
                                title="取消此任务"
                                onClick={async () => {
                                  if (!confirm('确定要取消此视频任务吗？')) return
                                  try {
                                    await videoAPI.cancelVideoTask(t.id)
                                    toast({ title: '任务已取消', variant: 'success' })
                                    mutateTasks()
                                  } catch { toast({ title: '取消失败', variant: 'destructive' }) }
                                }}
                              >
                                <X className="mr-1 h-3.5 w-3.5" /> 取消
                              </Button>
                            )}
                            {!isActive && (
                              <Button size="sm" variant="ghost" className="text-red-500 hover:text-red-700 hover:bg-red-50" title="删除此视频任务" onClick={async () => {
                                if (!confirm('确定要删除此视频任务吗？')) return
                                try {
                                  await videoAPI.deleteTask(t.id)
                                  toast({ title: '已删除', variant: 'success' })
                                  mutateTasks()
                                } catch { toast({ title: '删除失败', variant: 'destructive' }) }
                              }}>
                                <Trash2 className="h-4 w-4" />
                              </Button>
                            )}
                          </div>
                        </div>

                        {t.compose_stage && t.compose_stage !== '' && t.compose_stage !== 'done' && (
                          <div className="mt-3 rounded-lg bg-purple-50 border border-purple-200 p-3">
                            {(() => {
                              const currentIdx = composeStageOrder.indexOf(t.compose_stage!)
                              const activeIdx = composeActiveStages.indexOf(t.compose_stage!)
                              const completedStages = activeIdx < 0 ? 0 : activeIdx
                              const progressValue = activeIdx < 0 ? 0 : Math.round(((activeIdx + 1) / composeActiveStages.length) * 100)
                              return (
                                <>
                            <div className="flex items-center justify-between mb-2">
                              <div className="flex items-center gap-2">
                                <Loader2 className="h-4 w-4 text-purple-500 animate-spin" />
                                <span className="text-sm font-medium text-purple-700">合成中</span>
                              </div>
                              <div className="text-right">
                                <div className="text-xs font-medium text-purple-600">{composeStageLabel[t.compose_stage] || '处理中…'}</div>
                                <div className="text-[10px] text-purple-500">{completedStages}/{composeActiveStages.length} 已完成 · {progressValue}%</div>
                              </div>
                            </div>
                            <Progress value={progressValue} className="mb-3 h-2 bg-purple-100" />
                            <div className="flex gap-1">
                              {composeActiveStages.map((stage) => {
                                const stageIdx = composeStageOrder.indexOf(stage)
                                return (
                                  <div key={stage} className="flex-1 flex flex-col gap-1">
                                    <div
                                      className={`h-2 rounded-full transition-all ${
                                        stageIdx < currentIdx ? 'bg-purple-500' :
                                        stageIdx === currentIdx ? 'bg-purple-500 animate-pulse' :
                                        'bg-purple-200'
                                      }`}
                                    />
                                    <span className={`text-[10px] text-center leading-tight ${
                                      stageIdx <= currentIdx ? 'text-purple-600' : 'text-purple-300'
                                    }`}>
                                      {stage === 'concatenating' ? '拼接' : stage === 'adding_audio' ? '音频' : stage === 'adding_subtitle' ? '字幕' : '上传'}
                                    </span>
                                  </div>
                                )
                              })}
                            </div>
                                </>
                              )
                            })()}
                          </div>
                        )}

                        {t.status === 'succeeded' && (t.result_url || t.hls_url) && (
                          <div className="mt-3 flex items-center gap-2 rounded-lg bg-green-50 border border-green-200 px-3 py-2">
                            <CheckCircle2 className="h-4 w-4 text-green-500 shrink-0" />
                            <span className="text-xs text-green-700 font-medium shrink-0">视频已就绪</span>
                            <a
                              href={t.result_url || t.hls_url}
                              target="_blank"
                              rel="noopener noreferrer"
                              className="text-xs text-green-600 hover:text-green-800 underline truncate flex-1 min-w-0"
                              title={t.result_url || t.hls_url}
                            >
                              {t.result_url || t.hls_url}
                            </a>
                            <button
                              className="text-xs text-green-600 hover:text-green-800 shrink-0 underline"
                              onClick={() => { navigator.clipboard.writeText(t.result_url || t.hls_url); toast({ title: '链接已复制' }) }}
                            >
                              复制
                            </button>
                          </div>
                        )}

                        {clips.length > 0 && (() => {
                          const sortedClips = [...clips].sort((a, b) => a.clip_order - b.clip_order)
                          const renderClip = (clip: VClip) => {
                            const isSerialClip = !!(t.serial_scene || clip.scene_group_key)
                            const isSerialNonFirst = isSerialClip && (clip.scene_seq ?? 0) > 0
                            const hasEndFrame = !!(clip.end_frame_image_url)
                            return (
                            <div
                              className={`group relative rounded-md overflow-hidden border cursor-pointer transition-all ${
                                clip.status === 'succeeded' ? 'border-green-300 hover:border-green-500 hover:shadow-md' :
                                clip.status === 'failed' ? 'border-red-300' :
                                clip.status === 'processing' ? 'border-blue-300' :
                                'border-surface-200'
                              } ${isSerialClip ? 'w-24 h-14' : 'aspect-video'}`}
                              title={`片段 ${clip.clip_order + 1}${isSerialClip ? ` (组内第${(clip.scene_seq ?? 0) + 1}帧)` : ''}: ${clip.status}${clip.error_msg ? ' — ' + clip.error_msg : ''}`}
                              onClick={() => {
                                if (clip.status === 'succeeded' && clip.clip_url && !isSerialClip) {
                                  setPreviewUrl(clip.clip_url)
                                } else {
                                  setClipDetailInfo({ clip, task: t })
                                }
                              }}
                            >
                              {(() => {
                                // 优先用末帧图（生成成功后由 frame-extractor 提取），其次用首帧图（分镜图）。
                                // 对串行非首帧 clip，分镜图为空，但末帧图在生成后可用。
                                const thumbSrc = clip.end_frame_image_url || t.image_urls?.[clip.clip_order]
                                if (thumbSrc) {
                                  return <img src={thumbSrc} alt="" className="h-full w-full object-cover" />
                                }
                                // 串行非首帧 clip 尚未生成时，显示"链式"图标作为占位符
                                return (
                                  <div className="flex h-full w-full items-center justify-center bg-surface-100">
                                    {isSerialNonFirst
                                      ? <Link2 className="h-3 w-3 text-surface-400" />
                                      : <Video className="h-3 w-3 text-surface-300" />}
                                  </div>
                                )
                              })()}
                              {/* 末帧已提取标识 */}
                              {hasEndFrame && isSerialClip && (
                                <div className="absolute left-1 top-1 flex items-center gap-0.5 rounded bg-teal-500/80 px-1 py-0 text-[9px] text-white leading-4" title="末帧已提取，可用于下一片段串行锚点">
                                  末帧
                                </div>
                              )}
                              {clip.status === 'succeeded' && clip.clip_url ? (
                                <div className="absolute inset-0 flex items-center justify-center bg-black/20 group-hover:bg-black/40 transition-colors">
                                  <div className="flex h-7 w-7 items-center justify-center rounded-full bg-green-500/90 shadow-lg group-hover:scale-110 transition-transform">
                                    <Play className="h-3.5 w-3.5 text-white ml-0.5" />
                                  </div>
                                </div>
                              ) : clip.status === 'processing' ? (
                                <div className="absolute inset-0 flex items-center justify-center bg-blue-500/30">
                                  <div className="flex h-7 w-7 items-center justify-center rounded-full bg-blue-500/90 shadow-lg animate-pulse">
                                    <Loader2 className="h-4 w-4 text-white animate-spin" />
                                  </div>
                                </div>
                              ) : clip.status === 'failed' ? (
                                <div className="absolute inset-0 bg-red-500/20">
                                  <div className="absolute inset-0 flex items-center justify-center">
                                    {clip.error_msg?.startsWith('serial chain broken') ? (
                                      <div className="flex h-7 w-7 items-center justify-center rounded-full bg-orange-500/90 shadow-lg" title="上游串行片段失败，本片段被级联跳过">
                                        <Link2Off className="h-4 w-4 text-white" />
                                      </div>
                                    ) : (
                                      <div className="flex h-7 w-7 items-center justify-center rounded-full bg-red-500/90 shadow-lg">
                                        <AlertCircle className="h-4 w-4 text-white" />
                                      </div>
                                    )}
                                  </div>
                                  {/* AI 诊断按钮（仅根因 clip 展示）*/}
                                  {clip.chain_failure_analysis && (
                                    <button
                                      type="button"
                                      className="absolute left-1 top-1 flex items-center gap-0.5 rounded bg-amber-500/90 px-1.5 py-0.5 text-[10px] text-white hover:bg-amber-600/90"
                                      onClick={(e) => { e.stopPropagation(); setAiAnalysisClip(clip) }}
                                      title="查看 AI 失败诊断"
                                    >
                                      <Info className="h-2.5 w-2.5" /> AI诊断
                                    </button>
                                  )}
                                  <DropdownMenu>
                                    <DropdownMenuTrigger asChild>
                                      <button
                                        type="button"
                                        className="absolute right-1 top-1 rounded bg-black/70 px-1.5 py-0.5 text-[10px] text-white hover:bg-black/80"
                                        onClick={(event) => event.stopPropagation()}
                                      >
                                        重试
                                      </button>
                                    </DropdownMenuTrigger>
                                    <DropdownMenuContent align="end" className="w-80" onClick={(event) => event.stopPropagation()}>
                                      <DropdownMenuLabel className="text-[10px] text-surface-400">选择模型重试该分镜</DropdownMenuLabel>
                                      <DropdownMenuSeparator />
                                      {vtVideoModelOptions.map((item) => {
                                        const avail = videoModelAvailability[item.key]
                                        return (
                                        <DropdownMenuItem
                                          key={item.key}
                                          className={`cursor-pointer px-3 py-2 ${avail === false ? 'opacity-50' : ''}`}
                                          onClick={() => handleRetryClipWithModel(t.id, clip.id, clip.clip_order, item.key)}
                                        >
                                          <div className="flex items-start gap-2 w-full">
                                            <span className="text-base mt-0.5">{item.icon}</span>
                                            <div className="flex flex-col gap-0.5 min-w-0 flex-1">
                                              <div className="flex items-center gap-1.5 flex-wrap">
                                                <span className="text-xs font-semibold">{item.label}</span>
                                                <span className={`text-[10px] px-1 py-0 rounded font-medium ${item.speed === 'fast' ? 'bg-green-100 text-green-700' : item.speed === 'slow' ? 'bg-orange-100 text-orange-700' : 'bg-blue-100 text-blue-700'}`}>
                                                  {item.speed === 'fast' ? '快速' : item.speed === 'slow' ? '慢速' : '均衡'}
                                                </span>
                                                <span className={`text-[10px] px-1 py-0 rounded font-medium ${item.quality === 'high' ? 'bg-purple-100 text-purple-700' : 'bg-surface-100 text-surface-500'}`}>
                                                  {item.quality === 'high' ? '高质量' : '标准'}
                                                </span>
                                                {avail === true && <span className="rounded bg-emerald-100 px-1 py-0 text-[9px] text-emerald-700">● 可用</span>}
                                                {avail === false && <span className="rounded bg-red-100 px-1 py-0 text-[9px] text-red-600">● 未配置</span>}
                                              </div>
                                              <span className="text-[10px] text-surface-400 leading-none">{getProviderLabel(item.provider)}</span>
                                              <span className="text-[11px] text-surface-500 leading-snug">{item.desc}</span>
                                              <div className="flex flex-wrap gap-1 mt-0.5">
                                                {item.tags.map((tag) => (
                                                  <span key={tag} className="text-[10px] bg-surface-100 text-surface-500 rounded px-1">{tag}</span>
                                                ))}
                                              </div>
                                            </div>
                                          </div>
                                        </DropdownMenuItem>
                                        )
                                      })}
                                    </DropdownMenuContent>
                                  </DropdownMenu>
                                </div>
                              ) : (
                                <div className="absolute inset-0 flex items-center justify-center bg-black/40">
                                  <Clock className="h-4 w-4 text-white/60" />
                                </div>
                              )}
                              <span className="absolute bottom-0 right-0 bg-black/60 px-1 text-[10px] text-white leading-4">
                                {isSerialClip ? `${(clip.scene_seq ?? 0) + 1}/${t.clips?.filter(c => c.scene_group_key === clip.scene_group_key).length ?? 1}` : clip.clip_order + 1}
                              </span>
                            </div>
                          )}

                          if (t.serial_scene || project.project_type === 'video_serial') {
                            const groupMap = new Map<string, VClip[]>()
                            for (const c of sortedClips) {
                              const key = c.scene_group_key || '_default'
                              if (!groupMap.has(key)) groupMap.set(key, [])
                              groupMap.get(key)!.push(c)
                            }
                            const groups = Array.from(groupMap.entries()).map(([key, items]) => ({ key, clips: items }))
                            return (
                              <div className="mt-3 space-y-2">
                                {groups.map(({ key, clips: groupClips }) => {
                                  const groupDone = groupClips.filter(c => c.status === 'succeeded').length
                                  const groupFailed = groupClips.filter(c => c.status === 'failed' && !c.error_msg?.startsWith('serial chain broken')).length
                                  const groupBroken = groupClips.filter(c => c.error_msg?.startsWith('serial chain broken')).length
                                  const chainBroken = groupFailed > 0
                                  return (
                                  <div key={key} className={`rounded-lg border p-2 ${chainBroken ? 'border-red-200 bg-red-50/30' : 'border-indigo-100 bg-indigo-50/40'}`}>
                                    <div className="mb-2 flex items-center gap-2 flex-wrap">
                                      <Film className={`h-3 w-3 shrink-0 ${chainBroken ? 'text-red-400' : 'text-indigo-400'}`} />
                                      <span className={`text-[11px] font-medium truncate max-w-[200px] ${chainBroken ? 'text-red-600' : 'text-indigo-600'}`}>{key === '_default' ? '默认分组' : key}</span>
                                      {/* 链健康状态 */}
                                      {chainBroken ? (
                                        <span className="flex items-center gap-0.5 rounded bg-red-100 px-1.5 text-[10px] text-red-600">
                                          <Link2Off className="h-2.5 w-2.5" /> 链断裂 · {groupFailed} 根因
                                          {groupBroken > 0 && ` · ${groupBroken} 级联`}
                                        </span>
                                      ) : (
                                        <span className="rounded bg-indigo-100 px-1.5 text-[10px] text-indigo-500">
                                          {groupDone}/{groupClips.length} 完成
                                        </span>
                                      )}
                                      {/* 末帧提取进度 */}
                                      {!chainBroken && groupDone > 0 && (
                                        <span className="rounded bg-teal-50 px-1.5 text-[10px] text-teal-600">
                                          {groupClips.filter(c => c.end_frame_image_url).length} 末帧已提取
                                        </span>
                                      )}
                                    </div>
                                    <div className="flex items-center gap-1 flex-wrap">
                                      {groupClips.map((clip, idx) => (
                                        <React.Fragment key={clip.id}>
                                          {idx > 0 && (
                                            <ChevronRight className={`h-3 w-3 shrink-0 ${
                                              clip.error_msg?.startsWith('serial chain broken') ? 'text-red-300' :
                                              groupClips[idx - 1]?.status === 'succeeded' ? 'text-teal-400' :
                                              'text-indigo-300'
                                            }`} />
                                          )}
                                          {renderClip(clip)}
                                        </React.Fragment>
                                      ))}
                                    </div>
                                  </div>
                                  )
                                })}
                              </div>
                            )
                          }

                          return (
                            <div className="mt-3 grid grid-cols-5 sm:grid-cols-8 md:grid-cols-10 gap-2">
                              {sortedClips.map((clip) => (
                                <React.Fragment key={clip.id}>
                                  {renderClip(clip)}
                                </React.Fragment>
                              ))}
                            </div>
                          )
                        })()}
                      </CardContent>
                    </Card>
                  )
                })}
              </div>
            )
          })}
        </div>
      )}

      {/* Video preview — fullscreen modal */}
      {/* AI 串行失败诊断 Dialog */}
      <Dialog open={!!aiAnalysisClip} onOpenChange={(open) => { if (!open) setAiAnalysisClip(null) }}>
        <DialogContent className="max-w-lg">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2 text-amber-700">
              <Info className="h-4 w-4" />
              AI 失败诊断 — 片段 {aiAnalysisClip ? aiAnalysisClip.clip_order + 1 : ''}
            </DialogTitle>
          </DialogHeader>
          {parsedAnalysis ? (
            <div className="space-y-4">
              <div className="rounded-lg bg-red-50 border border-red-200 p-3">
                <p className="text-xs font-semibold text-red-700 mb-1">失败原因</p>
                <p className="text-sm text-red-800 leading-relaxed">{parsedAnalysis.reason}</p>
              </div>
              {parsedAnalysis.suggestions?.length > 0 && (
                <div className="rounded-lg bg-amber-50 border border-amber-200 p-3">
                  <p className="text-xs font-semibold text-amber-700 mb-2">优化建议</p>
                  <ol className="space-y-1.5 list-decimal list-inside">
                    {parsedAnalysis.suggestions.map((s: string, i: number) => (
                      <li key={i} className="text-sm text-amber-800 leading-relaxed">{s}</li>
                    ))}
                  </ol>
                </div>
              )}
              {aiAnalysisClip?.error_msg && (
                <div className="rounded-lg bg-surface-50 border border-surface-200 p-3">
                  <p className="text-xs font-semibold text-surface-500 mb-1">原始错误</p>
                  <p className="text-xs text-surface-600 font-mono break-all">{aiAnalysisClip.error_msg}</p>
                </div>
              )}
            </div>
          ) : (
            <p className="text-sm text-surface-500">暂无诊断信息</p>
          )}
        </DialogContent>
      </Dialog>

      {/* Clip 详情 Dialog（串行/并行均可查看）*/}
      <Dialog open={!!clipDetailInfo} onOpenChange={(open) => { if (!open) setClipDetailInfo(null) }}>
        <DialogContent className="max-w-xl">
          {clipDetailInfo && (() => {
            const { clip, task } = clipDetailInfo
            const isSerial = !!(task.serial_scene || clip.scene_group_key)
            const statusColor = clip.status === 'succeeded' ? 'text-green-700' : clip.status === 'failed' ? 'text-red-700' : clip.status === 'processing' ? 'text-blue-700' : 'text-surface-600'
            const statusLabel: Record<string, string> = { succeeded: '生成成功', failed: '生成失败', processing: '生成中', pending: '等待中' }
            const chainBroken = clip.error_msg?.startsWith('serial chain broken')
            const parsedChainAnalysis = (() => {
              if (!clip.chain_failure_analysis) return null
              try { return JSON.parse(clip.chain_failure_analysis) as { reason: string; suggestions: string[] } } catch { return null }
            })()
            return (
              <>
                <DialogHeader>
                  <DialogTitle className="flex items-center gap-2">
                    {isSerial ? <Link2 className="h-4 w-4 text-indigo-500" /> : <Video className="h-4 w-4 text-surface-500" />}
                    <span>片段 {clip.clip_order + 1} 详情</span>
                    <span className={`text-sm font-normal ${statusColor}`}>— {statusLabel[clip.status] ?? clip.status}</span>
                  </DialogTitle>
                </DialogHeader>
                <div className="space-y-3 text-sm">
                  {/* 基础信息 */}
                  <div className="grid grid-cols-2 gap-2 rounded-lg bg-surface-50 border border-surface-100 p-3 text-xs">
                    <div>
                      <span className="text-surface-400">片段序号</span>
                      <p className="font-medium text-surface-700">#{clip.clip_order + 1}</p>
                    </div>
                    {isSerial && (
                      <>
                        <div>
                          <span className="text-surface-400">场景组</span>
                          <p className="font-medium text-indigo-700 truncate" title={clip.scene_group_key}>{clip.scene_group_key || '—'}</p>
                        </div>
                        <div>
                          <span className="text-surface-400">组内位置</span>
                          <p className="font-medium text-indigo-700">第 {(clip.scene_seq ?? 0) + 1} 帧</p>
                        </div>
                      </>
                    )}
                    {clip.duration_sec > 0 && (
                      <div>
                        <span className="text-surface-400">时长</span>
                        <p className="font-medium text-surface-700">{clip.duration_sec}s</p>
                      </div>
                    )}
                  </div>

                  {/* 缩略图对比：首帧 vs 末帧 */}
                  {(task.image_urls?.[clip.clip_order] || clip.end_frame_image_url) && (
                    <div className="grid grid-cols-2 gap-3">
                      {task.image_urls?.[clip.clip_order] && (
                        <div>
                          <p className="text-[11px] text-surface-400 mb-1">首帧（分镜图）</p>
                          <img src={task.image_urls[clip.clip_order]} alt="首帧" className="w-full rounded border border-surface-200 object-cover aspect-video" />
                        </div>
                      )}
                      {clip.end_frame_image_url && (
                        <div>
                          <p className="text-[11px] text-teal-600 mb-1 font-medium">末帧（串行锚点）</p>
                          <img src={clip.end_frame_image_url} alt="末帧" className="w-full rounded border border-teal-200 object-cover aspect-video" />
                        </div>
                      )}
                    </div>
                  )}

                  {/* 视频播放（成功时） */}
                  {clip.status === 'succeeded' && clip.clip_url && (
                    <div className="rounded-lg bg-green-50 border border-green-200 p-3">
                      <p className="text-xs font-semibold text-green-700 mb-2">视频片段</p>
                      <video className="w-full rounded" controls>
                        <source src={clip.clip_url} type="video/mp4" />
                      </video>
                      <a href={clip.clip_url} download target="_blank" rel="noopener noreferrer" className="mt-2 flex items-center gap-1 text-xs text-green-600 hover:text-green-800 underline">
                        <Download className="h-3 w-3" /> 下载片段
                      </a>
                    </div>
                  )}

                  {/* 级联断链提示 */}
                  {chainBroken && (
                    <div className="rounded-lg bg-orange-50 border border-orange-200 p-3">
                      <p className="text-xs font-semibold text-orange-700 mb-1 flex items-center gap-1">
                        <Link2Off className="h-3 w-3" /> 级联跳过
                      </p>
                      <p className="text-xs text-orange-600">上游串行片段失败，本片段被自动跳过以避免错误传播。请先修复根因失败片段后重试。</p>
                    </div>
                  )}

                  {/* AI 链式失败诊断 */}
                  {parsedChainAnalysis && (
                    <div className="space-y-2">
                      <div className="rounded-lg bg-red-50 border border-red-200 p-3">
                        <p className="text-xs font-semibold text-red-700 mb-1">AI 失败诊断 — 原因</p>
                        <p className="text-xs text-red-800 leading-relaxed">{parsedChainAnalysis.reason}</p>
                      </div>
                      {parsedChainAnalysis.suggestions?.length > 0 && (
                        <div className="rounded-lg bg-amber-50 border border-amber-200 p-3">
                          <p className="text-xs font-semibold text-amber-700 mb-2">优化建议</p>
                          <ol className="space-y-1 list-decimal list-inside">
                            {parsedChainAnalysis.suggestions.map((s: string, i: number) => (
                              <li key={i} className="text-xs text-amber-800 leading-relaxed">{s}</li>
                            ))}
                          </ol>
                        </div>
                      )}
                    </div>
                  )}

                  {/* 原始错误信息 */}
                  {clip.error_msg && !chainBroken && (
                    <div className="rounded-lg bg-surface-50 border border-surface-200 p-3">
                      <p className="text-xs font-semibold text-surface-500 mb-1">错误信息</p>
                      <p className="text-xs text-surface-600 font-mono break-all leading-relaxed">{clip.error_msg}</p>
                    </div>
                  )}
                </div>
              </>
            )
          })()}
        </DialogContent>
      </Dialog>

      {/* Video preview — fullscreen modal */}
      {previewUrl && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/80" onClick={() => setPreviewUrl(null)}>
          <div className="relative w-full max-w-4xl mx-4" onClick={(e) => e.stopPropagation()}>
            <div className="flex items-center justify-between mb-3">
              <span className="text-sm text-white/70">视频预览</span>
              <div className="flex items-center gap-2">
                <a
                  href={previewUrl}
                  download
                  target="_blank"
                  rel="noopener noreferrer"
                  className="flex items-center gap-1.5 rounded-md bg-white/10 px-3 py-1.5 text-xs text-white hover:bg-white/20 transition-colors"
                >
                  <Download className="h-3.5 w-3.5" /> 下载视频
                </a>
                <Button size="sm" variant="ghost" className="text-white hover:bg-white/10" onClick={() => setPreviewUrl(null)} title="关闭预览">
                  <X className="h-4 w-4" />
                </Button>
              </div>
            </div>
            <div className="overflow-hidden rounded-lg bg-black shadow-2xl">
              <video className="w-full max-h-[80vh]" controls autoPlay key={previewUrl}>
                <source src={previewUrl} type="video/mp4" />
                您的浏览器不支持视频播放
              </video>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
