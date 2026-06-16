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
import { projectAPI, assetAPI, storyboardAPI, storageAPI, videoAPI, dubbingAPI, modelAPI, utilsAPI, type DubbingTask, type VoiceCatalogItem } from '@/lib/api'
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
import { StoryboardDubbingPanel } from '@/components/projects/detail/StoryboardDubbingPanel'
import { VoicePickerDialog } from '@/components/projects/detail/VoicePickerDialog'
import { isCommentaryProject as detectCommentaryProject } from '@/lib/projects/commentary-project'
import { formatStoryboardDubbingText, extractStoryboardSpeechText, hasSpeakableStoryboardText, resolveStoryboardSpeechLimit } from '@/lib/projects/storyboard-dubbing'

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
  const [voiceModel, setVoiceModel] = useState('auto')
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
  const [voiceSearch, setVoiceSearch] = useState('')
  const [voicePickerEpisodeId, setVoicePickerEpisodeId] = useState<number | null>(null)
  const [previewingEpisodeId, setPreviewingEpisodeId] = useState<number | null>(null)
  const [previewAudioUrlByEpisode, setPreviewAudioUrlByEpisode] = useState<Record<number, string>>({})
  const [retryingTaskIds, setRetryingTaskIds] = useState<number[]>([])
  const [retryingGroup, setRetryingGroup] = useState<'dubbing' | 'subtitle' | null>(null)
  const [aggregatingDialogues, setAggregatingDialogues] = useState<Record<number, boolean>>({})

  // Dynamically fetch project voice catalog; fall back to static list if API unavailable
  const { data: voicesDataDub } = useSWR(
    projectId ? ['project-voice-catalog', projectId] : null,
    () => dubbingAPI.listVoiceCatalog(projectId).then((r) => r.data?.items ?? null),
    { revalidateOnFocus: false, revalidateOnReconnect: false }
  )
  const translateVoiceMeta = (value?: string) => {
    const raw = (value || '').trim()
    if (!raw) return ''
    const map: Record<string, string> = {
      male: '男', female: '女', child: '儿童', narrator: '旁白', calm: '沉稳', deep: '低沉', warm: '温暖', bright: '明亮',
      multilingual: '多语', mainland: '大陆', mandarin: '普通话', regional: '方言', auto: '自动', assignable: '可自动分配',
    }
    return raw.split(/[-_\s/]+/).map(part => map[part.toLowerCase()] || part).join(' / ')
  }
  const formatVoiceOptionLabel = (voice: { key?: string; value?: string; label?: string; voice_name?: string; gender?: string; style?: string; category?: string; locale?: string }) => {
    if (voice.label) return voice.label
    const parts = [translateVoiceMeta(voice.voice_name) || voice.key || voice.value || '未命名音色']
    if (voice.gender) parts.push(translateVoiceMeta(voice.gender))
    if (voice.style) parts.push(translateVoiceMeta(voice.style))
    if (voice.category) parts.push(translateVoiceMeta(voice.category))
    if (voice.locale) parts.push(translateVoiceMeta(voice.locale))
    return parts.filter(Boolean).join(' · ')
  }
  const recommendedVoiceKeys = new Set(((voicesDataDub as { recommended?: Array<{ key?: string; value?: string }> } | null | undefined)?.recommended ?? []).map((v) => v.key ?? v.value ?? '').filter(Boolean))
  const VOICE_OPTIONS = [
    { value: 'auto', label: '自动按人物分配', category: 'auto', recommended: true },
    ...(voicesDataDub ?? FALLBACK_VOICE_OPTIONS).map((v: VoiceCatalogItem | { value: string; label: string; category?: string; recommended?: boolean }) => {
      const voice = v as { key?: string; value?: string; label?: string; voice_name?: string; gender?: string; style?: string; category?: string; locale?: string; provider?: string; auto_assignable?: boolean }
      const key = voice.key ?? voice.value ?? ''
      return {
        value: key,
        label: formatVoiceOptionLabel(voice),
        provider: voice.provider,
        voiceName: voice.voice_name,
        locale: voice.locale,
        gender: voice.gender,
        style: voice.style,
        autoAssignable: voice.auto_assignable,
        category: voice.category,
        recommended: recommendedVoiceKeys.has(key),
      }
    }),
  ]
  const FILTERED_VOICE_OPTIONS = VOICE_OPTIONS.filter((option) => {
    const q = voiceSearch.trim().toLowerCase()
    if (!q) return true
    return option.label.toLowerCase().includes(q) || option.value.toLowerCase().includes(q)
  })
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
  const activeVoicePickerEpisode = voicePickerEpisodeId
    ? displayedEpisodes.find((ep) => ep.id === voicePickerEpisodeId) ?? null
    : null

  // Poll per-storyboard dubbing/subtitle tasks
  const { data: storyboardTasksData, mutate: mutateStoryboardTasks } = useSWR(
    project.enable_dubbing || project.enable_subtitle ? ['storyboard-dubbing-tasks', projectId] : null,
    () => dubbingAPI.listStoryboardTasks(projectId),
    {
      refreshInterval: (data) => {
        const currentTasks = Array.isArray(data) ? data as DubbingTask[] : []
        if (currentTasks.some((task) => task.status === 'processing' || task.status === 'pending')) return 3000
        return 10000
      },
    },
  )
  const storyboardTaskList: DubbingTask[] = (storyboardTasksData as DubbingTask[] | undefined) ?? []

  const dubbingTaskByStoryboard = useMemo(() => {
    const map = new Map<number, DubbingTask>()
    for (const task of storyboardTaskList) {
      if (task.storyboard_id == null || task.task_type !== 'dubbing') continue
      map.set(task.storyboard_id, task)
    }
    return map
  }, [storyboardTaskList])

  const subtitleTaskByStoryboard = useMemo(() => {
    const map = new Map<number, DubbingTask>()
    for (const task of storyboardTaskList) {
      if (task.storyboard_id == null || task.task_type !== 'subtitle') continue
      map.set(task.storyboard_id, task)
    }
    return map
  }, [storyboardTaskList])

  const { data: allStoryboardsData } = useSWR(
    project.enable_dubbing || project.enable_subtitle ? ['storyboards-all-dubbing', projectId] : null,
    () => storyboardAPI.listAll(projectId) as Promise<{ data?: import('@/types').Storyboard[] }>,
    { revalidateOnFocus: false },
  )
  const allStoryboards = allStoryboardsData?.data ?? []

  const getTaskSubmitError = (err: unknown, label: '配音' | '字幕') => {
    const status = (err as { response?: { status?: number } })?.response?.status
    if (status === 409) {
      return `当前分镜已有进行中的${label}任务`
    }
    if (status === 503) {
      return `${label}服务暂时不可用，请稍后重试`
    }
    return `${label}提交失败`
  }

  const processingTasks = storyboardTaskList.filter((t) => t.status === 'processing' || t.status === 'pending')
  const hasProcessing = processingTasks.length > 0
  const dubbingProcessingTasks = processingTasks.filter((t) => t.task_type === 'dubbing')
  const subtitleProcessingTasks = processingTasks.filter((t) => t.task_type === 'subtitle')
  const dubbingDone = dubbingTaskByStoryboard.size > 0
    ? [...dubbingTaskByStoryboard.values()].filter((t) => t.status === 'succeeded').length
    : 0
  const subtitleDone = subtitleTaskByStoryboard.size > 0
    ? [...subtitleTaskByStoryboard.values()].filter((t) => t.status === 'succeeded').length
    : 0
  const totalStoryboardSlots = allStoryboards.filter((sb) => (sb.dialogue || '').trim()).length
  const dubbingDoneCount = [...dubbingTaskByStoryboard.values()].filter((t) => t.status === 'succeeded').length
  const subtitleDoneCount = [...subtitleTaskByStoryboard.values()].filter((t) => t.status === 'succeeded').length

  const isCommentaryProject = useMemo(() => detectCommentaryProject(project), [project])
  const storyboardSpeechOptions = (sb: Pick<Storyboard, 'duration'>) => ({
    isCommentary: isCommentaryProject,
    maxRunes: resolveStoryboardSpeechLimit(sb, project),
  })

  const pickEpisodeTextSource = (ep: Episode) => {
    const candidates = isCommentaryProject
      ? [ep.optimized_text, ep.script_excerpt, ep.original_excerpt, ep.summary, ep.title]
      : [ep.script_excerpt, ep.optimized_text, ep.summary, ep.title]
    return candidates.find((value) => value?.trim())?.trim() ?? ''
  }

  const stripSpeechPrefix = (line: string) => line
    .replace(/^[\s•·▪▫◦○●✔✅☑️□■▶▷►▸▹▻➤➜➟➠]+/, '')
    .replace(/^(旁白|主持人|主播|解说|老师|嘉宾|男声|女声|人物|角色|画外音|OS|VO)\s*(?:[：:｜|丨-].*?|（[^）]*）|\([^)]*\))?\s*/u, '')
    .trim()

  const isDirectionOnlyLine = (line: string) => {
    const trimmed = line.trim()
    if (!trimmed) return true
    if (/^[【\[].*[】\]]$/u.test(trimmed)) return true
    if (/^[（(].*[)）]$/u.test(trimmed)) return true
    return /^(镜头|画面|音效|音响|音乐|背景音乐|BGM|灯光|调色|转场|场景|内景|外景|景别|构图|运镜|摄影|机位|特写|远景|中景|近景|空镜|字幕|贴纸|包装|出字|旁白说明|动作|表情|情绪|氛围|布景|服装|道具|时间|地点)[：:：]/u.test(trimmed)
  }

  const looksLikeSpeakerCue = (line: string) => {
    const trimmed = line.trim()
    return /^(旁白|主持人|主播|解说|老师|嘉宾|男声|女声|人物|角色|画外音|OS|VO)(?:\s*(?:[：:｜|丨-].*?|（[^）]*）|\([^)]*\)))?$/u.test(trimmed)
  }

  const extractCommentaryNarration = (text: string) => {
    const subtitleMatches = [...text.matchAll(/\[字幕[:：]\s*([^\]]+?)\s*\]/gu)]
      .map((match) => match[1]?.trim())
      .filter(Boolean)
    if (subtitleMatches.length > 0) {
      return subtitleMatches.join('\n')
    }

    const quotedMatches = [...text.matchAll(/[“「『"]([^”」』"]+)[”」』"]/gu)]
      .map((match) => match[1]?.trim())
      .filter(Boolean)
    if (quotedMatches.length > 0) {
      return quotedMatches.join('\n')
    }

    const narrationLines = text
      .replace(/\r\n?/g, '\n')
      .split('\n')
      .map((line) => line.trim())
      .filter((line) => line && !/^(?:【\s*)?(?:内景|外景|内外景)/u.test(line))
      .map((line) => line.replace(/^[\u3400-\u4dbf\u4e00-\u9fffA-Za-z·]{1,8}[（(][^)）]{0,24}[）)]\s*/, '').trim())
      .filter((line) => line.length >= 10 && !isDirectionOnlyLine(line))
    if (narrationLines.length > 0) {
      return narrationLines.join('\n')
    }
    return ''
  }

  const extractSpokenText = (text: string) => {
    if (isCommentaryProject) {
      const commentary = extractCommentaryNarration(text)
      if (commentary) return commentary
    }

    const lines = text
      .replace(/\r\n?/g, '\n')
      .split('\n')
      .map((line) => line.trim())

    const spoken: string[] = []
    let pendingSpeaker = false

    for (const rawLine of lines) {
      const line = rawLine.trim()
      if (!line) {
        pendingSpeaker = false
        continue
      }
      if (isDirectionOnlyLine(line)) {
        continue
      }
      if (looksLikeSpeakerCue(line)) {
        pendingSpeaker = true
        continue
      }

      let candidate = stripSpeechPrefix(line)
      candidate = candidate
        .replace(/^[-—–:：]+\s*/, '')
        .replace(/\s{2,}/g, ' ')
        .trim()

      if (!candidate) continue
      if (isDirectionOnlyLine(candidate)) continue

      if (pendingSpeaker || candidate) {
        spoken.push(candidate)
      }
      pendingSpeaker = false
    }

    const cleaned = spoken
      .map((line) => line.replace(/^[：:、，,；;]+/, '').trim())
      .filter(Boolean)
      .join('\n')
      .trim()

    return cleaned || text.trim()
  }

  const getEpisodeBaseText = (ep: Episode) => extractSpokenText(pickEpisodeTextSource(ep))
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
      const res = await storyboardAPI.listAll(projectId, { episode_id: episodeId }) as {
        data?: Array<{
          id: number
          sequence_number: number
          dialogue?: string
          scene_description?: string
          characters?: string[]
          duration?: number
        }>
      }
      const storyboards = (res?.data ?? []).sort((a, b) => a.sequence_number - b.sequence_number)
      if (storyboards.length === 0) {
        toast({ title: '当前集暂无分镜台词，请先生成分镜', variant: 'destructive' })
        return
      }
      const aggregated = storyboards
        .map((sb) => {
          const duration = sb.duration ?? 4
          return formatStoryboardDubbingText(
            { dialogue: sb.dialogue || '', characters: sb.characters || [], scene_description: sb.scene_description || '', duration },
            storyboardSpeechOptions({ duration }),
          )
        })
        .filter(Boolean)
        .join('\n')
      if (!aggregated) {
        toast({ title: '分镜台词清洗后为空，请检查分镜对白字段', variant: 'destructive' })
        return
      }
      setDubbingDrafts((prev) => ({ ...prev, [episodeId]: aggregated }))
      setSubtitleDrafts((prev) => ({ ...prev, [episodeId]: aggregated }))
      toast({ title: `已从 ${storyboards.length} 个分镜提取台词`, variant: 'success' })
    } catch {
      toast({ title: '提取分镜台词失败', variant: 'destructive' })
    } finally {
      setAggregatingDialogues((prev) => ({ ...prev, [episodeId]: false }))
    }
  }
  const stalledDubbingTasks = dubbingProcessingTasks.filter((task) => task.status === 'processing' && getProgressStallMeta(task.updated_at))
  const stalledSubtitleTasks = subtitleProcessingTasks.filter((task) => task.status === 'processing' && getProgressStallMeta(task.updated_at))
  const handleRetryTask = async (task: DubbingTask, label: '配音' | '字幕') => {
    const storyboard = allStoryboards.find((sb) => sb.id === task.storyboard_id)
    const fallbackText = storyboard
      ? formatStoryboardDubbingText(storyboard, storyboardSpeechOptions(storyboard))
      : undefined
    setRetryingTaskIds((prev) => prev.includes(task.id) ? prev : [...prev, task.id])
    try {
      await dubbingAPI.retryTask(projectId, task.id, fallbackText)
      const episode = episodes.find((ep) => ep.id === task.episode_id)
      toast({ title: `第 ${episode?.episode_number ?? '?'} 集 · 分镜任务${label}已重新拉起`, variant: 'success' })
      mutateStoryboardTasks()
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
      list.findIndex((item) => item.storyboard_id === task.storyboard_id && item.task_type === task.task_type) === index
    )
    setRetryingTaskIds((prev) => Array.from(new Set([...prev, ...uniqueTasks.map((task) => task.id)])))
    let succeeded = 0
    let conflicts = 0
    let failed = 0
    try {
      const res = await dubbingAPI.retryTasksBatch(
        projectId,
        uniqueTasks.map((task) => {
          const storyboard = allStoryboards.find((sb) => sb.id === task.storyboard_id)
          const fallbackText = storyboard
            ? formatStoryboardDubbingText(storyboard, storyboardSpeechOptions(storyboard))
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
      mutateStoryboardTasks()
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
      const skippedVisual = allStoryboards.filter(
        (sb) => !hasSpeakableStoryboardText(sb, { isCommentary: isCommentaryProject }),
      ).length
      const eligible = allStoryboards.filter((sb) => {
        const text = formatStoryboardDubbingText(sb, storyboardSpeechOptions(sb))
        if (!text || !sb.episode_id) return false
        const existing = dubbingTaskByStoryboard.get(sb.id)
        return !(existing && (existing.status === 'succeeded' || existing.status === 'processing' || existing.status === 'pending'))
      })
      if (eligible.length === 0) {
        toast({
          title: skippedVisual > 0 ? `没有可提交的分镜配音（${skippedVisual} 个分镜仅有画面描述）` : '没有可提交的分镜配音',
          variant: 'default',
        })
        return
      }

      let submitted = 0
      let conflicts = 0
      let failed = 0
      for (const sb of eligible) {
        const episodeVoiceOptions = getEpisodeVoiceOptions(sb.episode_id!)
        const text = formatStoryboardDubbingText(sb, storyboardSpeechOptions(sb))
        try {
          await dubbingAPI.generateForStoryboard(
            projectId,
            sb.id,
            sb.episode_id!,
            text,
            episodeVoiceOptions.voice_model,
            episodeVoiceOptions,
          )
          submitted++
        } catch (err) {
          const status = (err as { response?: { status?: number } })?.response?.status
          if (status === 409) conflicts++
          else failed++
        }
      }

      toast({
        title: failed > 0
          ? `已提交 ${submitted} 个分镜配音，${conflicts} 个跳过，${failed} 个失败${skippedVisual > 0 ? `，${skippedVisual} 个无可合成台词` : ''}`
          : conflicts > 0 || skippedVisual > 0
            ? `已提交 ${submitted} 个分镜配音${conflicts > 0 ? `，${conflicts} 个跳过` : ''}${skippedVisual > 0 ? `，${skippedVisual} 个无可合成台词` : ''}`
            : `已提交 ${submitted} 个分镜配音任务`,
        variant: failed > 0 ? 'default' : 'success',
      })
      mutateStoryboardTasks()
    } catch (err) {
      toast({ title: getTaskSubmitError(err, '配音'), variant: 'destructive' })
    } finally {
      setBatchDubbing(false)
    }
  }

  const handleGenerateAllSubtitle = async () => {
    setBatchSubtitle(true)
    try {
      const skippedVisual = allStoryboards.filter(
        (sb) => !hasSpeakableStoryboardText(sb, { isCommentary: isCommentaryProject }),
      ).length
      const eligible = allStoryboards.filter((sb) => {
        const text = formatStoryboardDubbingText(sb, storyboardSpeechOptions(sb))
        if (!text || !sb.episode_id) return false
        const existing = subtitleTaskByStoryboard.get(sb.id)
        return !(existing && (existing.status === 'succeeded' || existing.status === 'processing' || existing.status === 'pending'))
      })
      if (eligible.length === 0) {
        toast({
          title: skippedVisual > 0 ? `没有可提交的分镜字幕（${skippedVisual} 个分镜仅有画面描述）` : '没有可提交的分镜字幕',
          variant: 'default',
        })
        return
      }

      let submitted = 0
      let conflicts = 0
      let failed = 0
      for (const sb of eligible) {
        const episodeVoiceOptions = getEpisodeVoiceOptions(sb.episode_id!)
        const text = formatStoryboardDubbingText(sb, storyboardSpeechOptions(sb))
        try {
          await dubbingAPI.generateSubtitleForStoryboard(
            projectId,
            sb.id,
            sb.episode_id!,
            text,
            episodeVoiceOptions,
          )
          submitted++
        } catch (err) {
          const status = (err as { response?: { status?: number } })?.response?.status
          if (status === 409) conflicts++
          else failed++
        }
      }

      toast({
        title: failed > 0
          ? `已提交 ${submitted} 个分镜字幕，${conflicts} 个跳过，${failed} 个失败${skippedVisual > 0 ? `，${skippedVisual} 个无可合成台词` : ''}`
          : conflicts > 0 || skippedVisual > 0
            ? `已提交 ${submitted} 个分镜字幕${conflicts > 0 ? `，${conflicts} 个跳过` : ''}${skippedVisual > 0 ? `，${skippedVisual} 个无可合成台词` : ''}`
            : `已提交 ${submitted} 个分镜字幕任务`,
        variant: failed > 0 ? 'default' : 'success',
      })
      mutateStoryboardTasks()
    } catch (err) {
      toast({ title: getTaskSubmitError(err, '字幕'), variant: 'destructive' })
    } finally {
      setBatchSubtitle(false)
    }
  }

  const autoVoiceEnabled = voiceModel === 'auto'

  return (
    <div className="space-y-6">
      {/* Action bar — hidden in single-episode mode since batch ops don't apply */}
      {!isSingleEpisodeMode && (
      <div className="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-surface-200 bg-surface-50 px-4 py-3">
        <div className="flex items-center gap-2 text-sm text-surface-600">
          <Mic className="h-4 w-4" />
          <span>共 {episodes.length} 集 · {totalStoryboardSlots || allStoryboards.length} 个分镜</span>
          {project.enable_dubbing && <span className="rounded bg-blue-100 px-1.5 py-0.5 text-[11px] text-blue-700">配音 {dubbingDoneCount}/{totalStoryboardSlots || allStoryboards.length}</span>}
          {project.enable_subtitle && <span className="rounded bg-green-100 px-1.5 py-0.5 text-[11px] text-green-700">字幕 {subtitleDoneCount}/{totalStoryboardSlots || allStoryboards.length}</span>}
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
            <Button size="sm" variant="outline" onClick={handleGenerateAllDubbing} disabled={batchDubbing} title="为所有分镜批量生成配音">
              {batchDubbing ? <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" /> : <Mic className="mr-1.5 h-3.5 w-3.5" />}
              一键生成配音
            </Button>
          )}
          {project.enable_subtitle && (
            <Button size="sm" variant="outline" onClick={handleGenerateAllSubtitle} disabled={batchSubtitle} title="为所有分镜批量生成字幕">
              {batchSubtitle ? <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" /> : <FileText className="mr-1.5 h-3.5 w-3.5" />}
              一键生成字幕
            </Button>
          )}
        </div>
      </div>
      )}

      {autoVoiceEnabled && (
        <div className="rounded-lg border border-violet-200 bg-violet-50 px-4 py-3 text-xs text-violet-700">
          已启用「自动按人物分配」：每个分镜单独合成，与视频片段对齐；文案中使用 <span className="font-medium">角色名：台词</span> 与 <span className="font-medium">旁白：内容</span> 可区分角色声线与旁白；未标注时默认按旁白处理。
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
                  {group.tasks.slice(0, 4).map((task) => {
                    const sb = allStoryboards.find((item) => item.id === task.storyboard_id)
                    return (
                      <span key={task.id}>
                        第{episodes.find((e) => e.id === task.episode_id)?.episode_number ?? '?'}集
                        {sb ? ` #${sb.sequence_number}` : ''}
                        {task.chunks_total > 0 ? ` (${task.chunks_done}/${task.chunks_total})` : ' (等待中)'}
                      </span>
                    )
                  })}
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

                <div className="mb-2 flex items-center justify-between gap-2">
                  <div className="text-[10px] text-surface-500">
                    当前音色：{VOICE_OPTIONS.find((v) => v.value === episodeVoiceOptions.voice_model)?.label || '未选择'}
                  </div>
                  <div className="flex items-center gap-2">
                    <Button
                      type="button"
                      size="sm"
                      variant="outline"
                      className="h-7 px-2 text-[10px]"
                      onClick={() => {
                        setVoicePickerEpisodeId(ep.id)
                        setVoiceSearch('')
                      }}
                    >
                      选择音色
                    </Button>
                    <Button
                      type="button"
                      size="sm"
                      variant="outline"
                      className="h-7 px-2 text-[10px]"
                      disabled={!episodeVoiceOptions.voice_model || episodeVoiceOptions.voice_model === 'auto' || previewingEpisodeId === ep.id}
                      onClick={async () => {
                        try {
                          setPreviewingEpisodeId(ep.id)
                          const res = await dubbingAPI.previewVoice(projectId, {
                            voice_model: episodeVoiceOptions.voice_model,
                            voice_rate: episodeVoiceOptions.voice_rate,
                            voice_pitch: episodeVoiceOptions.voice_pitch,
                            voice_volume: episodeVoiceOptions.voice_volume,
                          })
                          const audioUrl = res.data?.audio_url
                          if (!audioUrl) throw new Error('empty audio url')
                          setPreviewAudioUrlByEpisode((prev) => ({ ...prev, [ep.id]: audioUrl }))
                          const audio = new Audio(audioUrl)
                          await audio.play()
                          toast({ title: `第 ${ep.episode_number} 集音色试听已开始`, variant: 'success' })
                        } catch {
                          toast({ title: '音色试听失败', variant: 'destructive' })
                        } finally {
                          setPreviewingEpisodeId(null)
                        }
                      }}
                    >
                      {previewingEpisodeId === ep.id ? '试听中...' : '试听当前音色'}
                    </Button>
                  </div>
                </div>

                <div className="mb-3 grid grid-cols-1 gap-2 rounded-md border border-dashed border-surface-200 bg-surface-50 p-3 md:grid-cols-3">
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
                {previewAudioUrlByEpisode[ep.id] && (
                  <div className="mb-3 rounded-md border border-surface-200 bg-white p-2">
                    <audio controls className="h-8 w-full" src={previewAudioUrlByEpisode[ep.id]} />
                  </div>
                )}

                <StoryboardDubbingPanel
                  projectId={projectId}
                  project={project}
                  episodeId={ep.id}
                  episodeNumber={ep.episode_number}
                  isCommentaryProject={isCommentaryProject}
                  voiceOptions={episodeVoiceOptions}
                  dubbingTaskMap={dubbingTaskByStoryboard}
                  subtitleTaskMap={subtitleTaskByStoryboard}
                  onTasksMutate={() => { mutateStoryboardTasks(); mutateProject() }}
                  batchBusy={batchDubbing || batchSubtitle}
                />
              </div>
            )
          })}
        </div>
      )}

      <VoicePickerDialog
        open={!!voicePickerEpisodeId}
        onOpenChange={(open) => { if (!open) setVoicePickerEpisodeId(null) }}
        title={`选择本集音色${activeVoicePickerEpisode ? ` · 第 ${activeVoicePickerEpisode.episode_number} 集` : ''}`}
        search={voiceSearch}
        onSearchChange={setVoiceSearch}
        options={FILTERED_VOICE_OPTIONS}
        selectedValue={activeVoicePickerEpisode ? getEpisodeVoiceOptions(activeVoicePickerEpisode.id).voice_model : ''}
        previewAudioUrl={activeVoicePickerEpisode ? previewAudioUrlByEpisode[activeVoicePickerEpisode.id] : undefined}
        onSelect={(value) => {
          if (!activeVoicePickerEpisode) return
          updateEpisodeVoiceOverride(activeVoicePickerEpisode.id, 'voice_model', value)
          setVoicePickerEpisodeId(null)
        }}
        onPreview={async (value) => {
          if (!activeVoicePickerEpisode || !value || value === 'auto') return
          try {
            setPreviewingEpisodeId(activeVoicePickerEpisode.id)
            const current = getEpisodeVoiceOptions(activeVoicePickerEpisode.id)
            const res = await dubbingAPI.previewVoice(projectId, {
              voice_model: value,
              voice_rate: current.voice_rate,
              voice_pitch: current.voice_pitch,
              voice_volume: current.voice_volume,
            })
            const audioUrl = res.data?.audio_url
            if (!audioUrl) throw new Error('empty audio url')
            setPreviewAudioUrlByEpisode((prev) => ({ ...prev, [activeVoicePickerEpisode.id]: audioUrl }))
            const audio = new Audio(audioUrl)
            await audio.play()
          } catch {
            toast({ title: '音色试听失败', variant: 'destructive' })
          } finally {
            setPreviewingEpisodeId(null)
          }
        }}
      />
    </div>
  )
}
