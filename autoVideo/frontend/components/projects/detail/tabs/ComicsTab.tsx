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


export function ComicsTab({ projectId, project }: { projectId: number; project: Project }) {
  const { toast } = useToast()
  const comicsSharedEpisode = useProjectEpisodeFilter()
  const [episodeFilter, setEpisodeFilter] = useState<string>(() => comicsSharedEpisode.value)
  useEffect(() => {
    comicsSharedEpisode.setValue(episodeFilter)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [episodeFilter])
  const [readingMode, setReadingMode] = useState<'grid' | 'strip'>('grid')
  const [comicStyle, setComicStyle] = useState<ComicStylePresetKey>('story-manga')
  const [panelDensity, setPanelDensity] = useState<string>('4')
  const [creatingPanels, setCreatingPanels] = useState(false)
  const [generatingPanels, setGeneratingPanels] = useState(false)
  const [selectedComic, setSelectedComic] = useState<Storyboard | null>(null)

  const { data: episodesData } = useSWR(
    ['comic-episodes', projectId],
    () => projectAPI.listEpisodes(projectId) as unknown as Promise<{ data: Episode[] }>
  )
  const episodes = ((episodesData as { data?: Episode[] })?.data ?? [])
    .slice()
    .sort((a, b) => a.episode_number - b.episode_number)

  type ComicStatsData = { total: number; pending: number; generating: number; completed: number; failed: number; voided: number }
  const { data: comicStatsRaw, mutate: mutateComicStats } = useSWR(
    ['comic-stats', projectId],
    () => storyboardAPI.stats(projectId) as unknown as Promise<{ data: ComicStatsData }>,
    { refreshInterval: (data) => {
        const s = (data as { data?: ComicStatsData } | undefined)?.data
        if (!s) return 5000
        return (s.generating > 0 || s.pending > 0) ? 5000 : 30000
      } }
  )
  const comicStats: ComicStatsData = (comicStatsRaw as { data?: ComicStatsData })?.data ?? { total: 0, pending: 0, generating: 0, completed: 0, failed: 0, voided: 0 }

  const { data: storyboardsRaw, isLoading, mutate: mutateComics } = useSWR(
    ['comic-storyboards', projectId],
    () => storyboardAPI.listAll(projectId) as Promise<{ data: Storyboard[] }>,
    { refreshInterval: comicStats.generating > 0 || project.status === 'storyboard_generating' ? 4000 : 0 }
  )
  const allPanels = (((storyboardsRaw as { data?: Storyboard[] })?.data ?? []) as Storyboard[])
    .filter((panel) => !panel.is_voided)
    .sort((a, b) => {
      if ((a.episode_id ?? 0) !== (b.episode_id ?? 0)) return (a.episode_id ?? 0) - (b.episode_id ?? 0)
      return a.sequence_number - b.sequence_number
    })

  useEffect(() => {
    if (typeof window === 'undefined') return
    const savedStyle = window.localStorage.getItem('comic-style-preset') as ComicStylePresetKey | null
    const savedDensity = window.localStorage.getItem('comic-panel-density')
    const savedMode = window.localStorage.getItem('comic-reading-mode') as 'grid' | 'strip' | null
    if (savedStyle && COMIC_STYLE_PRESETS.some((preset) => preset.key === savedStyle)) setComicStyle(savedStyle)
    if (savedDensity && ['3', '4', '6'].includes(savedDensity)) setPanelDensity(savedDensity)
    if (savedMode === 'grid' || savedMode === 'strip') setReadingMode(savedMode)
  }, [])

  useEffect(() => {
    if (typeof window === 'undefined') return
    window.localStorage.setItem('comic-style-preset', comicStyle)
  }, [comicStyle])

  useEffect(() => {
    if (typeof window === 'undefined') return
    window.localStorage.setItem('comic-panel-density', panelDensity)
  }, [panelDensity])

  useEffect(() => {
    if (typeof window === 'undefined') return
    window.localStorage.setItem('comic-reading-mode', readingMode)
  }, [readingMode])

  const filteredEpisodes = useMemo(() => {
    if (episodeFilter === 'all') return episodes
    return episodes.filter((episode) => String(episode.id) === episodeFilter)
  }, [episodes, episodeFilter])

  const panelsByEpisode = useMemo(() => {
    const grouped = new Map<number, Storyboard[]>()
    for (const panel of allPanels) {
      const episodeId = panel.episode_id ?? 0
      if (!grouped.has(episodeId)) grouped.set(episodeId, [])
      grouped.get(episodeId)!.push(panel)
    }
    return grouped
  }, [allPanels])

  const filteredPanels = useMemo(() => {
    if (episodeFilter === 'all') return allPanels
    return allPanels.filter((panel) => String(panel.episode_id ?? '') === episodeFilter)
  }, [allPanels, episodeFilter])

  const completedPanels = filteredPanels.filter((panel) => panel.status === 'completed' && panel.image_url)
  const activeStyle = COMIC_STYLE_PRESETS.find((preset) => preset.key === comicStyle) ?? COMIC_STYLE_PRESETS[0]
  const densityValue = Number(panelDensity) || 4

  const createPanelsForEpisodes = async (targets: Episode[]) => {
    if (targets.length === 0) return 0
    let created = 0
    for (const episode of targets) {
      const existingPanels = panelsByEpisode.get(episode.id) ?? []
      if (existingPanels.length > 0) continue
      const panelDrafts = splitEpisodeIntoComicPanels(episode, densityValue, comicStyle)
      for (const [index, panelDraft] of panelDrafts.entries()) {
        await storyboardAPI.create(projectId, {
          episode_id: episode.id,
          sequence_number: index + 1,
          scene_description: panelDraft.scene_description,
          dialogue: panelDraft.dialogue,
          duration: Math.max(4, episode.estimated_duration || 4),
          aspect_ratio: '3:4',
          resolution: project.storyboard_config?.resolution || '1024x1536',
          camera_movement: 'static',
          video_mode: 'image',
        })
        created += 1
      }
    }
    return created
  }

  const handleCreateMissingPanels = async () => {
    if (episodes.length === 0) {
      toast({ title: '请先在剧本栏完成分集', variant: 'destructive' })
      return
    }
    setCreatingPanels(true)
    try {
      const count = await createPanelsForEpisodes(filteredEpisodes)
      toast({ title: count > 0 ? `已创建 ${count} 个漫画分镜` : '当前筛选集数已存在漫画分镜', variant: count > 0 ? 'success' : 'default' })
      mutateComics()
      mutateComicStats()
    } catch {
      toast({ title: '创建漫画分镜失败', variant: 'destructive' })
    } finally {
      setCreatingPanels(false)
    }
  }

  const handleGenerateComics = async () => {
    if (episodes.length === 0) {
      toast({ title: '请先生成集数后再一键生成漫画', variant: 'destructive' })
      return
    }
    setGeneratingPanels(true)
    try {
      await createPanelsForEpisodes(filteredEpisodes)
      const targetEpisodeIds = new Set(filteredEpisodes.map((episode) => episode.id))
      const latestPanels = (((await storyboardAPI.listAll(projectId)) as { data?: Storyboard[] }).data ?? [])
        .filter((panel) => !panel.is_voided && targetEpisodeIds.has(panel.episode_id ?? -1))
      const pendingPanels = latestPanels.filter((panel) => panel.status === 'pending' || panel.status === 'failed')
      if (pendingPanels.length === 0) {
        toast({ title: '当前筛选集数没有可生成的漫画画格', variant: 'default' })
        return
      }
      const results = await Promise.allSettled(
        pendingPanels.map((panel) => panel.status === 'failed' ? storyboardAPI.retry(projectId, panel.id) : storyboardAPI.generate(projectId, panel.id))
      )
      const triggered = results.filter((result) => result.status === 'fulfilled').length
      toast({ title: triggered > 0 ? `已提交 ${triggered} 个漫画画格生成任务` : '没有可生成的漫画画格', variant: triggered > 0 ? 'success' : 'default' })
      mutateComics()
      mutateComicStats()
    } catch {
      toast({ title: '漫画生成失败', variant: 'destructive' })
    } finally {
      setGeneratingPanels(false)
    }
  }

  const handleGenerateEpisode = async (episode: Episode) => {
    const panels = panelsByEpisode.get(episode.id) ?? []
    try {
      if (panels.length === 0) {
        await createPanelsForEpisodes([episode])
      }
      const nextPanels = panels.length > 0 ? panels : (((await storyboardAPI.listAll(projectId, { episode_id: episode.id })) as { data?: Storyboard[] }).data ?? [])
      const pendingPanels = nextPanels.filter((panel) => panel.status === 'pending' || panel.status === 'failed')
      if (pendingPanels.length === 0) {
        toast({ title: `第 ${episode.episode_number} 集暂无可生成的漫画画格`, variant: 'default' })
        return
      }
      const results = await Promise.allSettled(
        pendingPanels.map((panel) => panel.status === 'failed' ? storyboardAPI.retry(projectId, panel.id) : storyboardAPI.generate(projectId, panel.id))
      )
      const submitted = results.filter((result) => result.status === 'fulfilled').length
      toast({ title: submitted > 0 ? `第 ${episode.episode_number} 集已提交 ${submitted} 个漫画画格` : `第 ${episode.episode_number} 集提交失败`, variant: submitted > 0 ? 'success' : 'destructive' })
      mutateComics()
      mutateComicStats()
    } catch {
      toast({ title: `第 ${episode.episode_number} 集漫画生成失败`, variant: 'destructive' })
    }
  }

  const triggerPanelTask = async (panel: Storyboard) => {
    try {
      if (panel.status === 'failed') {
        await storyboardAPI.retry(projectId, panel.id)
        toast({ title: '漫画画格重试已启动', variant: 'success' })
      } else {
        await storyboardAPI.generate(projectId, panel.id)
        toast({ title: '漫画画格生成已启动', variant: 'success' })
      }
      mutateComics()
      mutateComicStats()
    } catch {
      toast({ title: panel.status === 'failed' ? '重试失败' : '生成失败', variant: 'destructive' })
    }
  }

  const gridClass = readingMode === 'grid'
    ? 'grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3'
    : 'grid grid-cols-1 gap-4'

  if (isLoading) return <TabSkeleton />

  return (
    <div className="space-y-6">
      <div className="rounded-xl border border-violet-200 bg-gradient-to-r from-violet-50 via-fuchsia-50 to-white p-4">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <div className="flex items-center gap-2 text-sm font-medium text-violet-800">
              <BookOpen className="h-4 w-4" />
              漫画栏目
            </div>
            <p className="mt-1 text-sm text-surface-600">
              基于当前剧本分集与片段，自动拆成多格漫画，并复用现有分镜图片生成能力输出漫画画格。
            </p>
            <p className="mt-2 text-xs text-violet-700">当前风格：{activeStyle.label} · {activeStyle.desc}</p>
          </div>
          <div className="flex flex-wrap gap-2">
            <Button size="sm" variant="outline" onClick={handleCreateMissingPanels} disabled={creatingPanels} title="按当前集数补齐漫画分镜">
              {creatingPanels ? <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" /> : <Plus className="mr-1.5 h-3.5 w-3.5" />}
              补齐漫画分镜
            </Button>
            <Button
              size="sm"
              onClick={handleGenerateComics}
              disabled={generatingPanels || filteredEpisodes.length === 0}
              title={filteredEpisodes.length === 0 ? '请先在剧本栏完成分集后再生成漫画' : '基于剧本分集生成漫画画格'}
            >
              {generatingPanels ? <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" /> : <Sparkles className="mr-1.5 h-3.5 w-3.5" />}
              一键生成漫画
            </Button>
          </div>
        </div>

        <div className="mt-4 grid gap-3 md:grid-cols-4">
          <div className="rounded-lg border border-white/70 bg-white/80 p-3">
            <p className="text-xs text-surface-400">漫画画格</p>
            <p className="mt-1 text-lg font-semibold text-surface-800">{comicStats.total}</p>
          </div>
          <div className="rounded-lg border border-white/70 bg-white/80 p-3">
            <p className="text-xs text-surface-400">已生成</p>
            <p className="mt-1 text-lg font-semibold text-emerald-700">{comicStats.completed}</p>
          </div>
          <div className="rounded-lg border border-white/70 bg-white/80 p-3">
            <p className="text-xs text-surface-400">生成中</p>
            <p className="mt-1 text-lg font-semibold text-blue-700">{comicStats.generating}</p>
          </div>
          <div className="rounded-lg border border-white/70 bg-white/80 p-3">
            <p className="text-xs text-surface-400">可阅读页</p>
            <p className="mt-1 text-lg font-semibold text-violet-700">{Math.ceil(completedPanels.length / 4) || 0}</p>
          </div>
        </div>
      </div>

      <div className="flex flex-wrap items-center gap-3">
        <Select value={episodeFilter} onValueChange={setEpisodeFilter}>
          <SelectTrigger className="w-40">
            <SelectValue placeholder="选择集数" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">全部集数</SelectItem>
            {episodes.map((episode) => (
              <SelectItem key={episode.id} value={String(episode.id)}>第 {episode.episode_number} 集</SelectItem>
            ))}
          </SelectContent>
        </Select>

        <Select value={comicStyle} onValueChange={(value) => setComicStyle(value as ComicStylePresetKey)}>
          <SelectTrigger className="w-44">
            <SelectValue placeholder="漫画风格" />
          </SelectTrigger>
          <SelectContent>
            {COMIC_STYLE_PRESETS.map((preset) => (
              <SelectItem key={preset.key} value={preset.key}>{preset.label}</SelectItem>
            ))}
          </SelectContent>
        </Select>

        <Select value={panelDensity} onValueChange={setPanelDensity}>
          <SelectTrigger className="w-36">
            <SelectValue placeholder="每集格数" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="3">每集 3 格</SelectItem>
            <SelectItem value="4">每集 4 格</SelectItem>
            <SelectItem value="6">每集 6 格</SelectItem>
          </SelectContent>
        </Select>

        <Select value={readingMode} onValueChange={(value) => setReadingMode(value as 'grid' | 'strip')}>
          <SelectTrigger className="w-36">
            <SelectValue placeholder="阅读模式" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="grid">网格阅读</SelectItem>
            <SelectItem value="strip">条漫阅读</SelectItem>
          </SelectContent>
        </Select>
      </div>

      {filteredEpisodes.length === 0 ? (
        <div className="rounded-lg border border-dashed border-surface-200 bg-surface-50 py-14 text-center">
          <BookOpen className="mx-auto mb-3 h-10 w-10 text-surface-300" />
          <p className="text-sm text-surface-500">请先在剧本栏完成分集，才能按剧本生成漫画。</p>
        </div>
      ) : (
        <div className="space-y-6">
          {filteredEpisodes.map((episode) => {
            const panels = (panelsByEpisode.get(episode.id) ?? []).slice().sort((a, b) => a.sequence_number - b.sequence_number)
            const doneCount = panels.filter((panel) => panel.status === 'completed').length
            const failedCount = panels.filter((panel) => panel.status === 'failed').length
            return (
              <section key={episode.id} className="rounded-xl border border-surface-200 bg-white p-4 shadow-sm">
                <div className="mb-4 flex flex-wrap items-start justify-between gap-3 border-b border-surface-100 pb-3">
                  <div>
                    <h3 className="text-base font-semibold text-surface-800">第 {episode.episode_number} 集 · {episode.title}</h3>
                    <p className="mt-1 max-w-3xl text-xs leading-5 text-surface-500">
                      {episode.summary || episode.script_excerpt || '当前剧本暂无摘要，生成时会使用标题补齐描述。'}
                    </p>
                  </div>
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="rounded bg-violet-100 px-2 py-1 text-[11px] text-violet-700">{doneCount}/{panels.length || densityValue} 已完成</span>
                    {failedCount > 0 && <span className="rounded bg-red-100 px-2 py-1 text-[11px] text-red-700">失败 {failedCount}</span>}
                    <Button size="sm" variant="outline" onClick={() => handleGenerateEpisode(episode)} title="生成本集漫画画格">
                      <Sparkles className="mr-1.5 h-3.5 w-3.5" />
                      生成本集漫画
                    </Button>
                  </div>
                </div>

                {panels.length === 0 ? (
                  <div className="rounded-lg border border-dashed border-surface-200 bg-surface-50 px-4 py-8 text-center">
                    <p className="text-sm text-surface-500">本集还没有漫画画格，点击上方按钮即可按剧本片段自动拆分并生成。</p>
                  </div>
                ) : (
                  <div className={gridClass}>
                    {panels.map((panel) => (
                      <Card key={panel.id} className={`overflow-hidden border-surface-200 ${panel.status === 'failed' ? 'ring-2 ring-red-200' : ''}`}>
                        <button
                          type="button"
                          className="block w-full text-left"
                          onClick={() => setSelectedComic(panel)}
                        >
                          <div className="aspect-[3/4] overflow-hidden bg-surface-100">
                            {panel.image_url ? (
                              <img src={panel.image_url} alt={`第${episode.episode_number}集第${panel.sequence_number}格`} className="h-full w-full object-cover" />
                            ) : (
                              <div className="flex h-full items-center justify-center text-surface-300">
                                <BookOpen className="h-10 w-10" />
                              </div>
                            )}
                          </div>
                        </button>
                        <CardContent className="space-y-3 p-3">
                          <div className="flex items-center justify-between">
                            <div className="flex items-center gap-2">
                              <span className="text-sm font-semibold text-surface-800">第 {panel.sequence_number} 格</span>
                              <StatusBadge status={panel.status} />
                            </div>
                            {(panel.status === 'pending' || panel.status === 'failed') ? (
                              <Button size="sm" variant="ghost" className="h-7 px-2 text-xs" onClick={() => triggerPanelTask(panel)}>
                                {panel.status === 'failed' ? <RefreshCw className="mr-1 h-3 w-3" /> : <Sparkles className="mr-1 h-3 w-3" />}
                                {panel.status === 'failed' ? '重试' : '生成'}
                              </Button>
                            ) : null}
                          </div>
                          <p className="line-clamp-2 text-xs leading-5 text-surface-600">{panel.scene_description}</p>
                          {panel.dialogue && (
                            <div className="rounded-2xl border border-violet-100 bg-violet-50 px-3 py-2 text-xs leading-5 text-violet-800">
                              “{panel.dialogue}”
                            </div>
                          )}
                          {panel.characters?.length > 0 && (
                            <div className="flex flex-wrap gap-1">
                              {panel.characters.map((character) => (
                                <Badge key={character} variant="outline" className="text-[10px] font-normal">
                                  {character}
                                </Badge>
                              ))}
                            </div>
                          )}
                        </CardContent>
                      </Card>
                    ))}
                  </div>
                )}
              </section>
            )
          })}
        </div>
      )}

      <Dialog open={Boolean(selectedComic)} onOpenChange={(open) => !open && setSelectedComic(null)}>
        <DialogContent className="max-w-5xl">
          {selectedComic && (
            <>
              <DialogHeader>
                <DialogTitle>漫画画格 · 第 {episodes.find((episode) => episode.id === selectedComic.episode_id)?.episode_number ?? '?'} 集 / 第 {selectedComic.sequence_number} 格</DialogTitle>
              </DialogHeader>
              <div className="grid gap-4 lg:grid-cols-[minmax(0,1.15fr)_minmax(320px,0.85fr)]">
                <div className="overflow-hidden rounded-xl border border-surface-200 bg-surface-50">
                  {selectedComic.image_url ? (
                    <ZoomableImage src={selectedComic.image_url} alt="漫画画格预览" className="h-full w-full object-contain" />
                  ) : (
                    <div className="flex min-h-[420px] items-center justify-center text-surface-300">
                      <BookOpen className="h-12 w-12" />
                    </div>
                  )}
                </div>
                <div className="space-y-4">
                  <div className="rounded-lg border border-surface-200 bg-surface-50 p-4">
                    <p className="text-xs text-surface-400">画面描述</p>
                    <p className="mt-2 text-sm leading-6 text-surface-700">{selectedComic.scene_description}</p>
                  </div>
                  <div className="rounded-lg border border-violet-100 bg-violet-50 p-4">
                    <p className="text-xs text-violet-500">气泡文案</p>
                    <p className="mt-2 text-sm leading-6 text-violet-900">{selectedComic.dialogue || '当前画格未填写对白'}</p>
                  </div>
                  <div className="grid gap-3 sm:grid-cols-2">
                    <div className="rounded-lg border border-surface-200 p-3">
                      <p className="text-xs text-surface-400">状态</p>
                      <div className="mt-2"><StatusBadge status={selectedComic.status} /></div>
                    </div>
                    <div className="rounded-lg border border-surface-200 p-3">
                      <p className="text-xs text-surface-400">时长参考</p>
                      <p className="mt-2 text-sm font-medium text-surface-800">{selectedComic.duration || 4} 秒</p>
                    </div>
                  </div>
                  {selectedComic.image_url && (
                    <a href={selectedComic.image_url} target="_blank" rel="noopener noreferrer" className="inline-flex items-center gap-1 text-sm text-violet-600 hover:underline">
                      <Download className="h-4 w-4" /> 下载当前漫画画格
                    </a>
                  )}
                </div>
              </div>
            </>
          )}
        </DialogContent>
      </Dialog>
    </div>
  )
}
