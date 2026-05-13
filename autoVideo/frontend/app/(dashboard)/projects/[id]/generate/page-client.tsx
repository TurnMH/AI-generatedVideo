'use client'
import { useParams, usePathname, useRouter } from 'next/navigation'
import { useEffect, useState } from 'react'
import useSWR from 'swr'
import {
  ArrowLeft, CheckCircle2, ChevronRight, Circle,
  Film, ImageIcon, LayoutGrid, Layers, Pause, Play,
  RefreshCw, Sparkles, Video, Zap,
} from 'lucide-react'
import { pipelineAPI, projectAPI, scriptAPI, storyboardAPI, videoAPI } from '@/lib/api'
import type { Project, Storyboard } from '@/types'
import { TaskQueue } from '@/components/task/TaskQueue'
import { Button } from '@/components/ui/button'
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
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '@/components/ui/dialog'
import { Label } from '@/components/ui/label'
import { LoadingSpinner } from '@/components/common/LoadingSpinner'
import { useToast } from '@/components/ui/toast'
import { fetchModelIdentity } from '@/lib/model-selection'
import { normalizeVideoStylePreset } from '@/lib/video-style-config'
import { SerialSceneGroups } from '@/components/projects/serial/SerialSceneGroups'
import { cn } from '@/lib/utils'
import { resolveProjectIdParam } from '@/lib/project-route'

/* ─── style presets ───────────────────────────────────────────── */
const STYLE_PRESETS = [
  { value: 'anime-2d',          label: '2D 动漫',    emoji: '🎨', desc: '日系二维动画风格' },
  { value: 'anime-3d',          label: '3D 动漫',    emoji: '✨', desc: '三维渲染动画风格' },
  { value: 'live-action-film',  label: '真人电影',   emoji: '🎬', desc: '写实电影级画质' },
  { value: 'live-action-short', label: '真人短剧',   emoji: '📱', desc: '竖屏短视频风格' },
]

const QUALITY_OPTIONS = [
  { value: 'draft',    label: '草稿',  badge: '快',  color: 'text-amber-600',  bg: 'bg-amber-50 border-amber-200',  desc: '优先速度，适合预览' },
  { value: 'standard', label: '标准',  badge: '均衡', color: 'text-indigo-600', bg: 'bg-indigo-50 border-indigo-200', desc: '速度与质量平衡' },
  { value: 'high',     label: '精品',  badge: '慢',  color: 'text-emerald-600',bg: 'bg-emerald-50 border-emerald-200',desc: '最高质量，耗时较长' },
]

type VideoLaunchPlan = {
  episodes: Array<{
    episode_id: number
    image_urls: string[]
    scene_descriptions?: string[]
    dialogues?: string[]
    durations?: number[]
    camera_movements?: string[]
    moods?: string[]
    scene_characters?: string[][]
    scene_asset_ids?: number[][]
    scene_description?: string
  }>
  fallbackRequest?: {
    image_urls: string[]
    scene_descriptions?: string[]
    dialogues?: string[]
    durations?: number[]
    camera_movements?: string[]
    moods?: string[]
    scene_characters?: string[][]
    scene_asset_ids?: number[][]
    scene_description?: string
  }
  totalClips: number
}

export default function GeneratePage() {
  const router = useRouter()
  const params = useParams()
  const pathname = usePathname()
  const projectId = resolveProjectIdParam(params.id, pathname, 'projects') ?? 0
  const hasValidProjectId = projectId > 0
  const { toast } = useToast()
  const [showStart, setShowStart] = useState(false)
  const [starting, setStarting] = useState(false)
  const [style, setStyle] = useState('anime-2d')
  const [quality, setQuality] = useState('standard')
  const [episodes, setEpisodes] = useState('1')
  const [storyboardLoading, setStoryboardLoading] = useState(false)
  const [videoLoading, setVideoLoading] = useState(false)
  const [videoConfirmOpen, setVideoConfirmOpen] = useState(false)
  const [videoLaunchPlan, setVideoLaunchPlan] = useState<VideoLaunchPlan | null>(null)

  useEffect(() => {
    if (!hasValidProjectId) {
      router.replace('/projects')
    }
  }, [hasValidProjectId, router])

  const { data: projectRaw, isLoading: projectLoading, mutate: mutateProject } = useSWR(
    hasValidProjectId ? ['project', projectId] : null,
    () => projectAPI.get(projectId) as unknown as Promise<{ data: Project }>
  )
  const project = (projectRaw as { data?: Project })?.data

  const { data: sbStatsRaw, mutate: mutateSbStats } = useSWR(
    hasValidProjectId ? ['sb-stats', projectId] : null,
    () => storyboardAPI.stats(projectId) as unknown as Promise<{ data?: { total?: number; completed?: number; pending?: number; failed?: number } }>
  )
  const sbStats = (sbStatsRaw as { data?: { total?: number; completed?: number; pending?: number; failed?: number } })?.data ?? {}

  const { data: vidStatsRaw, mutate: mutateVidStats } = useSWR(
    hasValidProjectId ? ['vid-stats', projectId] : null,
    () => videoAPI.stats(projectId) as unknown as Promise<{ data?: { total?: number; succeeded?: number; pending?: number; failed?: number } }>
  )
  const vidStats = (vidStatsRaw as { data?: { total?: number; succeeded?: number; pending?: number; failed?: number } })?.data ?? {}

  const { mutate: mutateStoryboards } = useSWR(
    hasValidProjectId ? ['storyboards-for-generate', projectId] : null,
    () => storyboardAPI.list(projectId, { page_size: 20 })
  )

  useEffect(() => {
    if (!project) return
    setStyle(normalizeVideoStylePreset(project.storyboard_config?.style_preset))
  }, [project])

  if (!hasValidProjectId) {
    return (
      <div className="flex h-64 items-center justify-center">
        <LoadingSpinner size="lg" />
      </div>
    )
  }

  const resolveQualityMode = () => {
    if (quality === 'draft') return 'speed'
    if (quality === 'high') return 'quality'
    return 'balanced'
  }

  const handleStart = async () => {
    setStarting(true)
    try {
      const scriptRes = await scriptAPI.listByProject(projectId, { page: 1, page_size: 1 })
      const latestScript = scriptRes.data.items[0]
      if (!latestScript) {
        toast({ title: '当前项目还没有可用剧本，请先上传或生成剧本', variant: 'destructive' })
        return
      }
      await pipelineAPI.start(projectId, latestScript.id, {
        episode_count: Number(episodes),
        style_preset: style,
        quality_mode: resolveQualityMode(),
        auto_fix: true,
        image_model: 'auto',
        video_model: 'auto',
        enable_audio: project?.enable_dubbing ?? false,
        enable_subtitle: project?.enable_subtitle ?? false,
      })
      setShowStart(false)
      toast({ title: '自动成片已启动', variant: 'success' })
      mutateProject(); mutateSbStats(); mutateVidStats()
    } catch (err: unknown) {
      const axiosErr = err as { response?: { data?: { message?: string } } }
      toast({ title: axiosErr?.response?.data?.message ?? '自动成片启动失败', variant: 'destructive' })
    } finally {
      setStarting(false)
    }
  }

  const handlePause = async () => {
    try {
      await projectAPI.pause(projectId)
      toast({ title: '项目已暂停', variant: 'success' })
      mutateProject()
    } catch { toast({ title: '暂停失败', variant: 'destructive' }) }
  }

  const handleResume = async () => {
    try {
      await projectAPI.resume(projectId)
      toast({ title: '项目已继续', variant: 'success' })
      mutateProject()
    } catch { toast({ title: '继续失败', variant: 'destructive' }) }
  }

  const handleStartStoryboard = async () => {
    setStoryboardLoading(true)
    try {
      const { modelKey, modelLabel } = await fetchModelIdentity(project?.image_model_id)
      const res = await storyboardAPI.generateAll(projectId, undefined, modelKey) as unknown as { data?: { triggered?: number } }
      const triggered = res?.data?.triggered ?? 0
      toast({
        title: triggered > 0 ? `分镜生成已启动，共 ${triggered} 个` : '没有可生成的分镜',
        description: triggered > 0 && modelLabel ? `当前使用模型：${modelLabel}` : undefined,
        variant: triggered > 0 ? 'success' : 'default',
      })
      mutateProject(); mutateSbStats(); mutateStoryboards()
    } catch { toast({ title: '分镜启动失败', variant: 'destructive' }) }
    finally { setStoryboardLoading(false) }
  }

  const buildVideoLaunchPlan = (storyboards: Storyboard[]): VideoLaunchPlan => {
    const eligibleSbs = [...storyboards]
      .filter((sb) => sb.image_url)
      .sort((a, b) => a.sequence_number - b.sequence_number)
    const byEpisode = new Map<number, {
      imageUrls: string[]
      descriptions: string[]
      dialogues: string[]
      durations: number[]
      cameras: string[]
      moods: string[]
      characters: string[][]
      assetIds: number[][]
    }>()

    for (const sb of eligibleSbs) {
      const episodeId = sb.episode_id ?? 0
      if (!byEpisode.has(episodeId)) {
        byEpisode.set(episodeId, {
          imageUrls: [],
          descriptions: [],
          dialogues: [],
          durations: [],
          cameras: [],
          moods: [],
          characters: [],
          assetIds: [],
        })
      }
      const bucket = byEpisode.get(episodeId)!
      bucket.imageUrls.push(sb.image_url)
      bucket.descriptions.push(sb.prompt_used || sb.scene_description || '')
      bucket.dialogues.push(sb.dialogue || '')
      bucket.durations.push(sb.duration || 0)
      bucket.cameras.push(sb.camera_movement || '')
      bucket.moods.push(sb.mood || '')
      bucket.characters.push(sb.characters || [])
      bucket.assetIds.push(sb.asset_ids || [])
    }

    const episodes = Array.from(byEpisode.entries())
      .filter(([episodeId, bucket]) => episodeId > 0 && bucket.imageUrls.length > 0)
      .map(([episodeId, bucket]) => ({
        episode_id: episodeId,
        image_urls: bucket.imageUrls,
        scene_descriptions: bucket.descriptions,
        dialogues: bucket.dialogues.some(Boolean) ? bucket.dialogues : undefined,
        durations: bucket.durations.some(Boolean) ? bucket.durations : undefined,
        camera_movements: bucket.cameras.some(Boolean) ? bucket.cameras : undefined,
        moods: bucket.moods.some(Boolean) ? bucket.moods : undefined,
        scene_characters: bucket.characters.some((items) => items.length > 0) ? bucket.characters : undefined,
        scene_asset_ids: bucket.assetIds.some((items) => items.length > 0) ? bucket.assetIds : undefined,
        scene_description: bucket.descriptions.filter(Boolean).join(' ') || undefined,
      }))

    const fallbackBucket = byEpisode.get(0)
    const fallbackRequest = fallbackBucket && fallbackBucket.imageUrls.length > 0
      ? {
          image_urls: fallbackBucket.imageUrls,
          scene_descriptions: fallbackBucket.descriptions,
          dialogues: fallbackBucket.dialogues.some(Boolean) ? fallbackBucket.dialogues : undefined,
          durations: fallbackBucket.durations.some(Boolean) ? fallbackBucket.durations : undefined,
          camera_movements: fallbackBucket.cameras.some(Boolean) ? fallbackBucket.cameras : undefined,
          moods: fallbackBucket.moods.some(Boolean) ? fallbackBucket.moods : undefined,
          scene_characters: fallbackBucket.characters.some((items) => items.length > 0) ? fallbackBucket.characters : undefined,
          scene_asset_ids: fallbackBucket.assetIds.some((items) => items.length > 0) ? fallbackBucket.assetIds : undefined,
          scene_description: fallbackBucket.descriptions.filter(Boolean).join(' ') || undefined,
        }
      : undefined

    return {
      episodes,
      fallbackRequest,
      totalClips: eligibleSbs.length,
    }
  }

  const handleStartVideo = async () => {
    try {
      const completedSb = ((await storyboardAPI.listAll(projectId, { status: 'completed' })) as { data?: Storyboard[] }).data ?? []
      const plan = buildVideoLaunchPlan(completedSb)
      if (plan.totalClips === 0) {
        toast({ title: '暂无已完成的分镜图片，请先生成分镜', variant: 'destructive' })
        return
      }
      setVideoLaunchPlan(plan)
      setVideoConfirmOpen(true)
    } catch {
      toast({ title: '读取分镜状态失败', variant: 'destructive' })
    }
  }

  const confirmStartVideo = async () => {
    if (!videoLaunchPlan) return
    setVideoLoading(true)
    try {
      if (videoLaunchPlan.episodes.length > 0) {
        await videoAPI.generateBatch(projectId, { episodes: videoLaunchPlan.episodes })
      }
      if (videoLaunchPlan.fallbackRequest) {
        await videoAPI.generate(projectId, videoLaunchPlan.fallbackRequest)
      }
      setVideoConfirmOpen(false)
      setVideoLaunchPlan(null)
      toast({ title: '视频生成已启动', variant: 'success' })
      mutateProject(); mutateVidStats()
    } catch {
      toast({ title: '视频启动失败', variant: 'destructive' })
    } finally {
      setVideoLoading(false)
    }
  }

  if (projectLoading) {
    return (
      <div className="flex h-64 items-center justify-center">
        <LoadingSpinner size="lg" />
      </div>
    )
  }

  const isPaused = project?.status === 'paused'
  const selectedStyle = STYLE_PRESETS.find((s) => s.value === style) ?? STYLE_PRESETS[0]
  const selectedQuality = QUALITY_OPTIONS.find((q) => q.value === quality) ?? QUALITY_OPTIONS[1]

  /* ── derived stats ── */
  const sbTotal     = sbStats.total ?? 0
  const sbCompleted = sbStats.completed ?? 0
  const sbPending   = sbStats.pending ?? 0
  const sbFailed    = sbStats.failed ?? 0
  const sbPct       = sbTotal > 0 ? Math.round((sbCompleted / sbTotal) * 100) : 0

  const vidTotal     = vidStats.total ?? 0
  const vidSucceeded = vidStats.succeeded ?? 0
  const vidPending   = vidStats.pending ?? 0
  const vidFailed    = vidStats.failed ?? 0
  const vidPct       = vidTotal > 0 ? Math.round((vidSucceeded / vidTotal) * 100) : 0

  return (
    <div className="space-y-5">

      {/* ── Top header bar ─────────────────────────────────────── */}
      <div className="flex items-center gap-3">
        <Button
          variant="ghost"
          size="icon"
          className="shrink-0 rounded-2xl border border-surface-200"
          onClick={() => router.push(`/projects/${projectId}`)}
        >
          <ArrowLeft className="h-4 w-4" />
        </Button>
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <h2 className="truncate text-xl font-semibold text-surface-900">生成中心</h2>
            {project?.project_type === 'video_serial' && (
              <span className="rounded-full bg-indigo-100 px-2 py-0.5 text-[11px] font-medium text-indigo-700">串行</span>
            )}
          </div>
          {project?.title && (
            <p className="mt-0.5 truncate text-sm text-surface-400">{project.title}</p>
          )}
        </div>
        <div className="flex items-center gap-2">
          {isPaused ? (
            <Button size="sm" onClick={handleResume} className="gap-1.5">
              <Play className="h-3.5 w-3.5" /> 继续
            </Button>
          ) : (
            <Button size="sm" variant="outline" onClick={handlePause} disabled={!project} className="gap-1.5">
              <Pause className="h-3.5 w-3.5" /> 暂停
            </Button>
          )}
          <Button size="sm" onClick={() => setShowStart(true)} className="gap-1.5 bg-gradient-to-r from-violet-600 to-indigo-600 text-white hover:from-violet-700 hover:to-indigo-700">
            <Zap className="h-3.5 w-3.5" />
            自动成片
          </Button>
        </div>
      </div>

      {/* ── Stats row ──────────────────────────────────────────── */}
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
        {/* Project status */}
        <div className="rounded-2xl border border-surface-200 bg-white p-4">
          <div className="flex items-center gap-2">
            <div className={cn('h-2 w-2 rounded-full', isPaused ? 'bg-amber-400' : 'bg-emerald-400 animate-pulse')} />
            <span className="text-xs text-surface-400">项目状态</span>
          </div>
          <p className="mt-2 text-lg font-semibold text-surface-900">{isPaused ? '已暂停' : '运行中'}</p>
          <p className="text-[11px] text-surface-400">{project?.project_type ?? '视频项目'}</p>
        </div>

        {/* Storyboard stats */}
        <div className="rounded-2xl border border-surface-200 bg-white p-4">
          <div className="flex items-center gap-2">
            <LayoutGrid className="h-3.5 w-3.5 text-violet-400" />
            <span className="text-xs text-surface-400">分镜</span>
          </div>
          <p className="mt-2 text-lg font-semibold text-surface-900">{sbCompleted}<span className="text-sm text-surface-400"> / {sbTotal}</span></p>
          <div className="mt-2 flex items-center gap-1.5">
            <div className="h-1 flex-1 overflow-hidden rounded-full bg-surface-100">
              <div className="h-full rounded-full bg-violet-400 transition-all" style={{ width: `${sbPct}%` }} />
            </div>
            <span className="text-[10px] text-surface-400">{sbPct}%</span>
          </div>
        </div>

        {/* Video stats */}
        <div className="rounded-2xl border border-surface-200 bg-white p-4">
          <div className="flex items-center gap-2">
            <Film className="h-3.5 w-3.5 text-cyan-400" />
            <span className="text-xs text-surface-400">视频</span>
          </div>
          <p className="mt-2 text-lg font-semibold text-surface-900">{vidSucceeded}<span className="text-sm text-surface-400"> / {vidTotal}</span></p>
          <div className="mt-2 flex items-center gap-1.5">
            <div className="h-1 flex-1 overflow-hidden rounded-full bg-surface-100">
              <div className="h-full rounded-full bg-cyan-400 transition-all" style={{ width: `${vidPct}%` }} />
            </div>
            <span className="text-[10px] text-surface-400">{vidPct}%</span>
          </div>
        </div>

        {/* Current config */}
        <div className="rounded-2xl border border-surface-200 bg-white p-4">
          <div className="flex items-center gap-2">
            <Sparkles className="h-3.5 w-3.5 text-amber-400" />
            <span className="text-xs text-surface-400">当前配置</span>
          </div>
          <p className="mt-2 text-sm font-medium text-surface-900">{selectedStyle.label}</p>
          <p className="text-[11px] text-surface-400">{selectedQuality.label} · {episodes} 集</p>
        </div>
      </div>

      {/* ── Quick actions ───────────────────────────────────────── */}
      <div className="grid gap-3 sm:grid-cols-3">

        {/* Storyboard generation */}
        <div className="group relative overflow-hidden rounded-2xl border border-surface-200 bg-white p-5 transition-shadow hover:shadow-md">
          <div className="flex items-start justify-between">
            <div className="rounded-xl bg-violet-50 p-2.5">
              <LayoutGrid className="h-5 w-5 text-violet-500" />
            </div>
            {sbPending > 0 && (
              <span className="rounded-full bg-amber-100 px-2 py-0.5 text-[10px] font-medium text-amber-700">
                {sbPending} 待处理
              </span>
            )}
            {sbFailed > 0 && (
              <span className="rounded-full bg-red-100 px-2 py-0.5 text-[10px] font-medium text-red-600">
                {sbFailed} 失败
              </span>
            )}
          </div>
          <h3 className="mt-3 font-semibold text-surface-900">分镜生成</h3>
          <p className="mt-1 text-xs leading-5 text-surface-400">
            为项目内所有集数的场景自动生成分镜图片，支持批量触发。
          </p>
          <div className="mt-4 flex items-center justify-between">
            <div className="flex items-center gap-3 text-[11px]">
              <span className="flex items-center gap-1 text-emerald-600">
                <CheckCircle2 className="h-3 w-3" />{sbCompleted} 已完成
              </span>
              {sbTotal > 0 && (
                <span className="flex items-center gap-1 text-surface-400">
                  <Circle className="h-3 w-3" />{sbTotal} 总计
                </span>
              )}
            </div>
            <Button
              size="sm"
              variant="outline"
              onClick={handleStartStoryboard}
              disabled={storyboardLoading}
              className="h-8 gap-1.5 border-violet-200 text-violet-700 hover:bg-violet-50"
            >
              {storyboardLoading ? <LoadingSpinner size="sm" /> : <><LayoutGrid className="h-3.5 w-3.5" />启动</>}
            </Button>
          </div>
        </div>

        {/* Video generation */}
        <div className="group relative overflow-hidden rounded-2xl border border-surface-200 bg-white p-5 transition-shadow hover:shadow-md">
          <div className="flex items-start justify-between">
            <div className="rounded-xl bg-cyan-50 p-2.5">
              <Film className="h-5 w-5 text-cyan-500" />
            </div>
            {vidPending > 0 && (
              <span className="rounded-full bg-amber-100 px-2 py-0.5 text-[10px] font-medium text-amber-700">
                {vidPending} 进行中
              </span>
            )}
            {vidFailed > 0 && (
              <span className="rounded-full bg-red-100 px-2 py-0.5 text-[10px] font-medium text-red-600">
                {vidFailed} 失败
              </span>
            )}
          </div>
          <h3 className="mt-3 font-semibold text-surface-900">视频合成</h3>
          <p className="mt-1 text-xs leading-5 text-surface-400">
            将已完成的分镜图片批量接入视频生成链路，按集数并行处理。
          </p>
          <div className="mt-4 flex items-center justify-between">
            <div className="flex items-center gap-3 text-[11px]">
              <span className="flex items-center gap-1 text-emerald-600">
                <CheckCircle2 className="h-3 w-3" />{vidSucceeded} 已完成
              </span>
              {vidTotal > 0 && (
                <span className="flex items-center gap-1 text-surface-400">
                  <Circle className="h-3 w-3" />{vidTotal} 总计
                </span>
              )}
            </div>
            <Button
              size="sm"
              variant="outline"
              onClick={handleStartVideo}
              disabled={videoLoading}
              className="h-8 gap-1.5 border-cyan-200 text-cyan-700 hover:bg-cyan-50"
            >
              {videoLoading ? <LoadingSpinner size="sm" /> : <><Film className="h-3.5 w-3.5" />启动</>}
            </Button>
          </div>
        </div>

        {/* Auto pipeline */}
        <div className="group relative overflow-hidden rounded-2xl border border-indigo-200 bg-gradient-to-br from-indigo-50 to-violet-50 p-5 transition-shadow hover:shadow-md">
          <div className="flex items-start justify-between">
            <div className="rounded-xl bg-indigo-100 p-2.5">
              <Zap className="h-5 w-5 text-indigo-600" />
            </div>
            <span className="rounded-full bg-indigo-100 px-2 py-0.5 text-[10px] font-medium text-indigo-600">推荐</span>
          </div>
          <h3 className="mt-3 font-semibold text-surface-900">自动成片</h3>
          <p className="mt-1 text-xs leading-5 text-surface-500">
            从剧本到视频一键全流程启动：分镜 → 配图 → 合成，自动依序执行。
          </p>
          <div className="mt-4 flex items-center justify-between">
            <div className="flex items-center gap-1 text-[11px] text-surface-400">
              <Layers className="h-3 w-3" />
              {selectedStyle.label} · {selectedQuality.label}
            </div>
            <Button
              size="sm"
              onClick={() => setShowStart(true)}
              className="h-8 gap-1.5 bg-indigo-600 text-white hover:bg-indigo-700"
            >
              <Zap className="h-3.5 w-3.5" />配置启动
            </Button>
          </div>
        </div>
      </div>

      {/* ── Flow steps hint ─────────────────────────────────────── */}
      <div className="flex items-center gap-2 overflow-x-auto rounded-2xl border border-surface-100 bg-surface-50/80 px-4 py-3">
        {[
          { icon: <ImageIcon className="h-3.5 w-3.5" />, label: '上传剧本', done: true },
          { icon: <LayoutGrid className="h-3.5 w-3.5" />, label: '生成分镜', done: sbCompleted > 0 },
          { icon: <Film className="h-3.5 w-3.5" />, label: '合成视频', done: vidSucceeded > 0 },
          { icon: <Video className="h-3.5 w-3.5" />, label: '导出成片', done: false },
        ].map((step, i, arr) => (
          <div key={i} className="flex shrink-0 items-center gap-2">
            <div className={cn(
              'flex items-center gap-1.5 rounded-full px-3 py-1 text-[11px] font-medium',
              step.done
                ? 'bg-emerald-100 text-emerald-700'
                : 'bg-white border border-surface-200 text-surface-400'
            )}>
              {step.icon}{step.label}
            </div>
            {i < arr.length - 1 && <ChevronRight className="h-3.5 w-3.5 shrink-0 text-surface-300" />}
          </div>
        ))}
        <div className="ml-auto flex shrink-0 items-center gap-1.5 text-[11px] text-surface-400">
          <RefreshCw className="h-3 w-3" />
          <button onClick={() => { mutateSbStats(); mutateVidStats(); mutateProject() }} className="hover:text-surface-600">刷新状态</button>
        </div>
      </div>

      {/* ── Task queue ─────────────────────────────────────────── */}
      <TaskQueue projectId={projectId} />

      {/* Serial scene groups */}
      {project?.project_type === 'video_serial' && (
        <div className="rounded-[20px] border border-surface-200 bg-white p-5">
          <SerialSceneGroups projectId={projectId} />
        </div>
      )}

      {/* ── Auto pipeline dialog ────────────────────────────────── */}
      <Dialog open={showStart} onOpenChange={setShowStart}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <Zap className="h-5 w-5 text-indigo-500" />
              配置自动成片
            </DialogTitle>
          </DialogHeader>

          <div className="space-y-5 py-1">
            {/* Style preset cards */}
            <div className="space-y-2">
              <Label className="text-xs font-medium uppercase tracking-wider text-surface-400">视频风格</Label>
              <div className="grid grid-cols-2 gap-2">
                {STYLE_PRESETS.map((s) => (
                  <button
                    key={s.value}
                    onClick={() => setStyle(s.value)}
                    className={cn(
                      'flex flex-col items-start rounded-xl border p-3 text-left transition-all',
                      style === s.value
                        ? 'border-indigo-400 bg-indigo-50 ring-1 ring-indigo-300'
                        : 'border-surface-200 bg-white hover:border-surface-300'
                    )}
                  >
                    <span className="text-base">{s.emoji}</span>
                    <span className="mt-1.5 text-xs font-semibold text-surface-800">{s.label}</span>
                    <span className="text-[10px] text-surface-400">{s.desc}</span>
                  </button>
                ))}
              </div>
            </div>

            {/* Quality options */}
            <div className="space-y-2">
              <Label className="text-xs font-medium uppercase tracking-wider text-surface-400">生成质量</Label>
              <div className="grid grid-cols-3 gap-2">
                {QUALITY_OPTIONS.map((q) => (
                  <button
                    key={q.value}
                    onClick={() => setQuality(q.value)}
                    className={cn(
                      'flex flex-col items-center rounded-xl border p-2.5 text-center transition-all',
                      quality === q.value
                        ? cn('ring-1', q.bg, q.color, 'border-transparent')
                        : 'border-surface-200 bg-white hover:border-surface-300'
                    )}
                  >
                    <span className={cn('rounded-full px-2 py-0.5 text-[10px] font-bold', quality === q.value ? q.bg : 'bg-surface-100 text-surface-500')}>{q.badge}</span>
                    <span className="mt-1 text-xs font-semibold text-surface-800">{q.label}</span>
                    <span className="text-[10px] text-surface-400">{q.desc}</span>
                  </button>
                ))}
              </div>
            </div>

            {/* Episodes */}
            <div className="space-y-2">
              <Label className="text-xs font-medium uppercase tracking-wider text-surface-400">生成集数</Label>
              <div className="flex gap-2">
                {[1, 2, 3, 5, 10].map((n) => (
                  <button
                    key={n}
                    onClick={() => setEpisodes(String(n))}
                    className={cn(
                      'flex-1 rounded-xl border py-2 text-sm font-medium transition-all',
                      episodes === String(n)
                        ? 'border-indigo-400 bg-indigo-50 text-indigo-700 ring-1 ring-indigo-300'
                        : 'border-surface-200 bg-white text-surface-600 hover:border-surface-300'
                    )}
                  >
                    {n}
                  </button>
                ))}
              </div>
            </div>

            {/* Summary */}
            <div className="rounded-xl border border-surface-200 bg-surface-50 px-4 py-3">
              <p className="text-[11px] text-surface-400">即将启动</p>
              <p className="mt-1 text-sm font-medium text-surface-800">
                {selectedStyle.emoji} {selectedStyle.label} · {selectedQuality.label}质量 · {episodes} 集
              </p>
            </div>
          </div>

          <DialogFooter className="gap-2">
            <Button variant="outline" onClick={() => setShowStart(false)} disabled={starting}>取消</Button>
            <Button
              onClick={handleStart}
              disabled={starting}
              className="gap-2 bg-indigo-600 text-white hover:bg-indigo-700"
            >
              {starting ? <><LoadingSpinner size="sm" />启动中…</> : <><Zap className="h-4 w-4" />开始生成</>}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <AlertDialog
        open={videoConfirmOpen}
        onOpenChange={(open) => {
          setVideoConfirmOpen(open)
          if (!open && !videoLoading) setVideoLaunchPlan(null)
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>确认启动批量视频生成</AlertDialogTitle>
            <AlertDialogDescription>
              {videoLaunchPlan
                ? `本次将按当前已完成分镜启动 ${videoLaunchPlan.totalClips} 个片段，覆盖 ${videoLaunchPlan.episodes.length} 个分集。未完成或失败分镜不会进入本次批量任务。`
                : '将按当前已完成分镜发起批量视频生成。'}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={videoLoading}>取消</AlertDialogCancel>
            <AlertDialogAction onClick={confirmStartVideo} disabled={videoLoading} className="bg-cyan-600 hover:bg-cyan-700 focus:ring-cyan-500">
              {videoLoading ? '启动中...' : '确认启动'}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
