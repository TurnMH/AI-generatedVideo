
'use client'

import React, { useState, useMemo, useEffect, useRef } from 'react'
import { useParams, useRouter, useSearchParams } from 'next/navigation'
import useSWR from 'swr'
import {
  ArrowLeft,
  BookOpen,
  Sparkles,
  ListVideo,
  CheckCircle2,
  Loader2,
  LayoutGrid,
  RefreshCw,
  AlertTriangle,
  ArrowRight,
  Clock3,
  Image as ImageIcon,
  Mic,
  Video,
  FileText,
  Volume2,
  ChevronRight,
} from 'lucide-react'
import { projectAPI, assetAPI, storyboardAPI } from '@/lib/api'
import type { Project, Episode } from '@/types'
import { Button } from '@/components/ui/button'
import { StatusBadge } from '@/components/projects/detail/StatusBadge'
import { LoadingSpinner } from '@/components/common/LoadingSpinner'
import { useToast } from '@/components/ui/toast'
import { formatBytes } from '@/lib/projects/utils'
import { ProjectEpisodeFilterContext } from '@/lib/projects/episode-filter'
import { buildProjectOverview } from '@/lib/projects/workflow'
import { StorageDrawer } from '@/components/projects/detail/StorageDrawer'
import { ScriptTab } from '@/components/projects/detail/tabs/ScriptTab'
import { EpisodeWorkspace } from '@/components/projects/detail/EpisodeWorkspace'

export default function ProjectDetailPage() {
  const params = useParams()
  const router = useRouter()
  const searchParams = useSearchParams()
  const projectId = Number(params.id)

  const [selectedEpisodeId, setSelectedEpisodeId] = useState<number | null>(null)
  const [storageOpen, setStorageOpen] = useState(false)
  const [sharedEpisodeFilter, setSharedEpisodeFilter] = useState<string>('all')
  const [isExtractingStoryboards, setIsExtractingStoryboards] = useState(false)
  const [isExtractingAssets, setIsExtractingAssets] = useState(false)
  const [isGeneratingProjectImages, setIsGeneratingProjectImages] = useState(false)
  const [episodeEntryTab, setEpisodeEntryTab] = useState<'assets' | 'storyboard' | 'dubbing' | 'video'>('assets')
  // 是否处于自动管线模式（新建项目或手动触发自动分集时置 true）
  const [autoStoryboardPending, setAutoStoryboardPending] = useState(() => searchParams.get('autoStart') === '1')
  const autoOpenedRef = useRef(false)
  const { toast } = useToast()

  const sharedEpisodeFilterValue = useMemo(
    () => ({ value: sharedEpisodeFilter, setValue: setSharedEpisodeFilter }),
    [sharedEpisodeFilter]
  )

  const { data, isLoading, mutate: mutateProject } = useSWR(
    ['project', projectId],
    () => projectAPI.get(projectId) as unknown as Promise<{ data: Project }>,
    { refreshInterval: 3000 }
  )
  const project = data?.data

  // 如果是串行项目，重定向到专属路由
  useEffect(() => {
    if (project?.project_type === 'video_serial') {
      router.replace(`/video-serial/${projectId}`)
    }
  }, [project, projectId, router])

  const { data: episodesData } = useSWR(
    project ? ['stepper-episodes', projectId] : null,
    () => projectAPI.listEpisodes(projectId) as unknown as Promise<{ data: Episode[] }>,
    { refreshInterval: 5000 }
  )
  const episodes = episodesData?.data || []

  // 自动管线：有 autoStart 或 ScriptTab 触发了分镜自动排队后，等分集出现就自动打开第一集
  useEffect(() => {
    if (!autoStoryboardPending) return
    if (autoOpenedRef.current) return
    if (episodes.length === 0) return
    autoOpenedRef.current = true
    setSelectedEpisodeId(episodes[0].id)
    setEpisodeEntryTab('assets')
    // 清理 URL 中的 autoStart 参数，保持地址干净
    const url = new URL(window.location.href)
    url.searchParams.delete('autoStart')
    router.replace(url.pathname + (url.search || ''))
  }, [autoStoryboardPending, episodes, router])

  const { data: assetsCountData, mutate: mutateAssetsCount } = useSWR(
    project ? ['stepper-assets', projectId] : null,
    () => assetAPI.list(projectId) as unknown as Promise<{ data: any[] }>,
    { refreshInterval: 5000 }
  )
  const stepperAssetsRaw = (assetsCountData as { data?: any[] })?.data ?? []

  const { data: storyboardsData, mutate: mutateStoryboardsData } = useSWR(
    project ? ['stepper-storyboards', projectId] : null,
    () => storyboardAPI.listAll(projectId) as Promise<{ data: any[] }>,
    { refreshInterval: 5000 }
  )
  const stepperStoryboardsRaw = (storyboardsData as { data?: any[] })?.data ?? []

  const episodeWorkspaceMeta = useMemo(() => {
    const meta = new Map<number, {
      assetTotal: number
      assetCompleted: number
      assetFailed: number
      assetExtracting: boolean
      assetGenerating: boolean
      storyboardTotal: number
      storyboardCompleted: number
      storyboardGenerating: boolean
      storyboardFailed: boolean
    }>()

    for (const ep of episodes) {
      meta.set(ep.id, {
        assetTotal: 0,
        assetCompleted: 0,
        assetFailed: 0,
        assetExtracting: false,
        assetGenerating: false,
        storyboardTotal: 0,
        storyboardCompleted: 0,
        storyboardGenerating: false,
        storyboardFailed: false,
      })
    }

    for (const asset of stepperAssetsRaw) {
      if (asset?.name === '__extracting__' || asset?.status === 'extracting') continue
      const episodeIds = Array.isArray(asset?.episode_ids)
        ? asset.episode_ids.map((id: unknown) => Number(id)).filter((id: number) => Number.isFinite(id))
        : []
      for (const eid of episodeIds) {
        const current = meta.get(eid)
        if (!current) continue
        current.assetTotal += 1
        if (asset?.status === 'completed') current.assetCompleted += 1
        if (asset?.status === 'failed' || asset?.status === 'qa_failed') current.assetFailed += 1
        if (asset?.status === 'pending' || asset?.status === 'generating' || asset?.status === 'paused') current.assetGenerating = true
      }
    }

    for (const sb of stepperStoryboardsRaw) {
      const eid = Number(sb?.episode_id)
      if (!Number.isFinite(eid) || eid <= 0) continue
      const current = meta.get(eid)
      if (!current) continue
      current.storyboardTotal += 1
      if (sb?.status === 'completed' && sb?.image_url) current.storyboardCompleted += 1
      if (sb?.status === 'pending' || sb?.status === 'generating' || sb?.status === 'paused') current.storyboardGenerating = true
      if (sb?.status === 'failed') current.storyboardFailed = true
    }

    return meta
  }, [episodes, stepperAssetsRaw, stepperStoryboardsRaw])

  const projectOverview = useMemo(() => {
    if (!project) {
      return {
        workflowSteps: [] as Array<{ key: string; label: string; hint: string; status: 'done' | 'current' | 'pending' | 'failed' | 'skipped' }>,
        notices: [] as Array<{ tone: 'blue' | 'red'; title: string; description: string }>,
        nextAction: {
          title: '请先加载项目数据',
          description: '项目数据加载完成后，可继续查看剧本大纲与分集工作台。',
          cta: '稍候',
          type: 'noop' as const,
          tab: 'assets' as const,
          disabled: true,
        },
        stats: {
          episodeReady: 0,
          episodeFailed: 0,
          assetTotal: 0,
          assetCompleted: 0,
          assetActive: 0,
          assetFailed: 0,
          storyboardTotal: 0,
          storyboardCompleted: 0,
          storyboardActive: 0,
          storyboardFailed: 0,
        },
      }
    }

    const episodeReady = episodes.filter((ep) => ep.status === 'scene_ready').length
    const episodeFailed = episodes.filter((ep) => ep.status === 'failed').length
    const episodeSplitting = episodes.filter((ep) => ep.status === 'scene_splitting').length
    const stepperAssetsVisible = stepperAssetsRaw.filter((a) => a?.name !== '__extracting__' && a?.status !== 'extracting')
    const assetTotal = stepperAssetsVisible.length
    const assetCompleted = stepperAssetsVisible.filter((asset) => asset?.status === 'completed').length
    const assetActive = stepperAssetsVisible.filter((asset) => ['pending', 'generating', 'paused'].includes(asset?.status)).length
    const assetFailed = stepperAssetsVisible.filter((asset) => asset?.status === 'failed' || asset?.status === 'qa_failed').length
    const storyboardTotal = stepperStoryboardsRaw.length
    const storyboardCompleted = stepperStoryboardsRaw.filter((sb) => sb?.status === 'completed' && sb?.image_url).length
    const storyboardActive = stepperStoryboardsRaw.filter((sb) => ['pending', 'generating', 'paused'].includes(sb?.status)).length
    const storyboardFailed = stepperStoryboardsRaw.filter((sb) => sb?.status === 'failed').length

    const firstAssetFailureEpisodeId = episodes.find((ep) => (episodeWorkspaceMeta.get(ep.id)?.assetFailed ?? 0) > 0)?.id ?? null
    const firstStoryboardFailureEpisodeId = episodes.find((ep) => (episodeWorkspaceMeta.get(ep.id)?.storyboardFailed ?? false))?.id ?? null
    const firstStoryboardActiveEpisodeId = episodes.find((ep) => (episodeWorkspaceMeta.get(ep.id)?.storyboardGenerating ?? false) || ep.status === 'scene_splitting')?.id ?? null
    const firstStoryboardReadyEpisodeId = episodes.find((ep) => (episodeWorkspaceMeta.get(ep.id)?.storyboardTotal ?? 0) > 0)?.id ?? null
    const firstStoryboardImageReadyEpisodeId = episodes.find((ep) => stepperStoryboardsRaw.some((sb) => Number(sb?.episode_id) === ep.id && sb?.status === 'completed' && sb?.image_url))?.id ?? null
    const firstAssetReadyEpisodeId = episodes.find((ep) => (episodeWorkspaceMeta.get(ep.id)?.assetTotal ?? 0) > 0)?.id ?? null

    const overview = buildProjectOverview({
      project,
      episodeCount: episodes.length,
      episodeSplitting,
      assetTotal,
      assetCompleted,
      assetActive,
      assetFailed,
      storyboardTotal,
      storyboardImageReady: storyboardCompleted,
      storyboardActive,
      storyboardFailed,
      isExtractingStoryboards,
      firstAssetFailureEpisodeId,
      firstStoryboardFailureEpisodeId,
      firstStoryboardActiveEpisodeId,
      firstStoryboardReadyEpisodeId,
      firstStoryboardImageReadyEpisodeId,
      firstAssetReadyEpisodeId,
    })

    return {
      ...overview,
      stats: {
        episodeReady,
        episodeFailed,
        assetTotal,
        assetCompleted,
        assetActive,
        assetFailed,
        storyboardTotal,
        storyboardCompleted,
        storyboardActive,
        storyboardFailed,
      },
    }
  }, [project, episodes, stepperAssetsRaw, stepperStoryboardsRaw, episodeWorkspaceMeta, isExtractingStoryboards])

  const projectProgress = project?.progress
  const projectTargetEpisodes = project?.target_episodes ?? 0
  const projectSplitInProgress = selectedEpisodeId === null && episodes.length === 0 && (
    project?.status === 'script_processing' || projectProgress?.stage === 'episode_splitting'
  )
  const projectSplitTotal = projectProgress?.episode_split?.total ?? 0
  const projectSplitCompleted = projectProgress?.episode_split?.completed ?? 0
  const projectSplitTitle = projectProgress?.message?.trim()
    || (projectSplitTotal > 0
      ? `正在生成分集结构（${projectSplitCompleted}/${projectSplitTotal}）`
      : projectTargetEpisodes > 0
        ? `正在按目标 ${projectTargetEpisodes} 集拆分剧本`
        : '正在拆分剧本并生成分集结构')
  const projectSplitDescription = projectSplitTotal > 0
    ? `系统正在分析剧本、提取关键词并生成分集。当前已识别 ${projectSplitCompleted}/${projectSplitTotal} 个分集草稿，完成后左侧分集列表会自动出现。`
    : '系统正在分析剧本内容并自动生成分集。生成完成后，左侧会自动出现分集列表，下面的剧本区也会刷新出最新结果。'
  const projectSplitFootnote = '无需重复点击开始分集，当前页面会自动轮询刷新。分集完成后再继续资源提取、镜头拆分和出图。'

  const selectedEpisode = selectedEpisodeId == null
    ? undefined
    : episodes.find((ep) => ep.id === selectedEpisodeId)
  const selectedEpisodeMeta = selectedEpisode ? episodeWorkspaceMeta.get(selectedEpisode.id) : undefined
  const selectedEpisodeAssetSummary = selectedEpisodeMeta
    ? `${selectedEpisodeMeta.assetCompleted}/${selectedEpisodeMeta.assetTotal || 0}`
    : '0/0'
  const selectedEpisodeStoryboardSummary = selectedEpisodeMeta
    ? `${selectedEpisodeMeta.storyboardCompleted}/${selectedEpisodeMeta.storyboardTotal || 0}`
    : '0/0'
  const selectedEpisodeUpdatedLabel = selectedEpisode?.updated_at
    ? new Date(selectedEpisode.updated_at).toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
    : '—'

  const openEpisodeWorkspace = (
    targetEpisodeId?: number | null,
    tab: 'assets' | 'storyboard' | 'dubbing' | 'video' = 'assets'
  ) => {
    if (!targetEpisodeId) {
      toast({
        title: '暂无可进入的分集',
        description: '请先生成分集后再继续后续制作流程。',
        variant: 'default',
      })
      return
    }
    setEpisodeEntryTab(tab)
    setSelectedEpisodeId(targetEpisodeId)
  }

  const openProjectStageFromOverview = (tab: 'assets' | 'storyboard' | 'dubbing' | 'video') => {
    const storyboardReadyEpisodeId = episodes.find((ep) => (episodeWorkspaceMeta.get(ep.id)?.storyboardTotal ?? 0) > 0)?.id
    const assetReadyEpisodeId = episodes.find((ep) => (episodeWorkspaceMeta.get(ep.id)?.assetTotal ?? 0) > 0)?.id
    const targetEpisodeId = tab === 'dubbing' || tab === 'video'
      ? storyboardReadyEpisodeId ?? assetReadyEpisodeId ?? episodes[0]?.id
      : tab === 'storyboard'
        ? storyboardReadyEpisodeId ?? assetReadyEpisodeId ?? episodes[0]?.id
        : assetReadyEpisodeId ?? episodes[0]?.id

    openEpisodeWorkspace(targetEpisodeId, tab)
  }

  const handleExtractProjectAssets = async () => {
    setIsExtractingAssets(true)
    try {
      await assetAPI.extract(projectId)
      await mutateAssetsCount()
      toast({
        title: '已提交至大模型处理队列',
        description: '系统正在调用大模型分析剧本，识别并提取全部角色、场景、道具，完成后自动衔接分镜提取。',
        variant: 'success',
      })
    } catch (error: any) {
      toast({
        title: '项目资源提取失败',
        description: error?.response?.data?.error || error?.response?.data?.message || '服务器发生错误',
        variant: 'destructive',
      })
    } finally {
      setIsExtractingAssets(false)
    }
  }

  const handleExtractProjectStoryboards = () => {
    setIsExtractingStoryboards(true)
    ;(projectAPI.extractStoryboards(projectId) as Promise<unknown>)
      .then(() => {
        void mutateStoryboardsData()
        setTimeout(() => setIsExtractingStoryboards(false), 5000)
      })
      .catch((error: any) => {
        setIsExtractingStoryboards(false)
        toast({
          title: '项目分镜提取失败',
          description: error?.response?.data?.error || error?.response?.data?.message || '服务器发生错误',
          variant: 'destructive',
        })
      })
    toast({
      title: '项目分镜提取已开始',
      description: '大模型正在为各集拆分分镜序列，完成后可在各集工作台查看进度。',
      variant: 'success',
    })
  }

  const handleGenerateProjectImages = async () => {
    setIsGeneratingProjectImages(true)
    try {
      await storyboardAPI.generateAll(projectId)
      await mutateStoryboardsData()
      toast({
        title: '已提交至图像生成模型队列',
        description: '系统正在调用图像生成模型为全部分镜批量渲染图片，根据提示词自动绘制每个镜头的画面。',
        variant: 'success',
      })
    } catch (error: any) {
      toast({
        title: '项目图片生成失败',
        description: error?.response?.data?.error || error?.response?.data?.message || '服务器发生错误',
        variant: 'destructive',
      })
    } finally {
      setIsGeneratingProjectImages(false)
    }
  }

  const handleProjectOverviewAction = async () => {
    if (projectOverview.nextAction.type === 'extract_assets') {
      await handleExtractProjectAssets()
      return
    }
    if (projectOverview.nextAction.type === 'extract_storyboards') {
      await handleExtractProjectStoryboards()
      return
    }
    if (projectOverview.nextAction.type === 'select_episode' && projectOverview.nextAction.episodeId) {
      openEpisodeWorkspace(projectOverview.nextAction.episodeId, projectOverview.nextAction.tab ?? 'assets')
    }
  }

  const projectQuickActions = [
    {
      icon: <Sparkles className="h-4 w-4 text-emerald-300" />,
      title: '项目资源提取',
      desc: '从剧本中识别并提取全部角色、场景、道具，为后续分镜制作提供素材库。',
      label: '开始提取',
      onClick: handleExtractProjectAssets,
      loading: isExtractingAssets,
      disabled: isExtractingAssets || episodes.length === 0,
    },
    {
      icon: <LayoutGrid className="h-4 w-4 text-violet-300" />,
      title: '镜头拆分',
      desc: '按项目维度统一拆分剧本为镜头条目，为后续分镜图片生成建立镜头序列。',
      label: '开始拆分',
      onClick: handleExtractProjectStoryboards,
      loading: isExtractingStoryboards,
      disabled: isExtractingStoryboards || episodes.length === 0,
    },
    {
      icon: <ImageIcon className="h-4 w-4 text-amber-300" />,
      title: '分镜图片生成',
      desc: '为项目内全部分镜批量生成图片，根据提示词自动渲染每个镜头的画面。',
      label: '开始生成',
      onClick: handleGenerateProjectImages,
      loading: isGeneratingProjectImages,
      disabled: isGeneratingProjectImages || projectOverview.stats.storyboardTotal === 0,
    },
    {
      icon: <Volume2 className="h-4 w-4 text-blue-300" />,
      title: '配音合成',
      desc: '根据角色台词自动生成语音，支持多角色配音与情感语调调节。',
      label: '进入配音',
      onClick: () => openProjectStageFromOverview('dubbing'),
      loading: false,
      disabled: episodes.length === 0,
    },
    {
      icon: <FileText className="h-4 w-4 text-cyan-300" />,
      title: '字幕生成',
      desc: '自动生成时间轴字幕，支持多语言翻译与字幕样式自定义。',
      label: '进入字幕',
      onClick: () => openProjectStageFromOverview('dubbing'),
      loading: false,
      disabled: episodes.length === 0,
    },
    {
      icon: <Video className="h-4 w-4 text-rose-300" />,
      title: '视频成片',
      desc: '将分镜图片、配音、字幕合成为最终视频，支持多种分辨率与格式导出。',
      label: '进入视频',
      onClick: () => openProjectStageFromOverview('video'),
      loading: false,
      disabled: episodes.length === 0 || projectOverview.stats.storyboardCompleted === 0,
    },
  ]

  if (isLoading) {
    return (
      <div className="flex h-64 items-center justify-center">
        <LoadingSpinner size="lg" />
      </div>
    )
  }

  if (!project) {
    return (
      <div className="flex min-h-[280px] flex-col items-center justify-center">
        <p>项目不存在</p>
        <Button onClick={() => router.push('/video')}>返回列表</Button>
      </div>
    )
  }

  return (
    <div className="space-y-4">
      {/* Header */}
      {selectedEpisodeId === null ? (
        <div className="overflow-hidden rounded-[28px] border border-surface-200/70 bg-gradient-to-br from-slate-950 via-violet-950 to-slate-900 p-6 text-white shadow-sm">
          <div className="flex flex-col gap-6 xl:flex-row xl:items-start xl:justify-between">
            <div className="max-w-3xl">
              <div className="mb-4 inline-flex items-center gap-2 rounded-full border border-white/10 bg-white/10 px-3 py-1.5 text-xs font-medium text-surface-100">
                <Sparkles className="h-3.5 w-3.5 text-primary-300" />
                项目总控台
              </div>
              <div className="flex items-start gap-4">
                <Button
                  variant="ghost"
                  size="icon"
                  className="mt-0.5 shrink-0 rounded-2xl border border-white/10 bg-white/10"
                  onClick={() => router.push('/video')}
                >
                  <ArrowLeft className="h-4 w-4" />
                </Button>
                <div>
                  <h2 className="text-2xl font-semibold tracking-tight text-white">{project.title}</h2>
                  <div className="mt-2 flex flex-wrap items-center gap-2">
                    <StatusBadge status={project.status} />
                    <span className="rounded-full border border-cyan-400/20 bg-cyan-400/10 px-3 py-1 text-xs text-cyan-100">
                      剧集 {episodes.length}
                    </span>
                    <span className="rounded-full border border-violet-400/20 bg-violet-400/10 px-3 py-1 text-xs text-violet-100">
                      风格标签 {project.style_tags?.length ?? 0}
                    </span>
                    <span className="rounded-full border border-emerald-400/20 bg-emerald-400/10 px-3 py-1 text-xs text-emerald-100">
                      资源 {projectOverview.stats.assetCompleted}/{projectOverview.stats.assetTotal}
                    </span>
                    <span className="rounded-full border border-amber-400/20 bg-amber-400/10 px-3 py-1 text-xs text-amber-100">
                      分镜 {projectOverview.stats.storyboardCompleted}/{projectOverview.stats.storyboardTotal}
                    </span>
                  </div>
                  <p className="mt-4 max-w-2xl text-sm leading-6 text-surface-200">
                    当前处于项目级总览模式。这里统一处理整体剧本大纲、项目级资源提取、分镜拆分、图片生成，以及进入语音与视频阶段的全局入口。
                  </p>
                  {projectSplitInProgress && (
                    <div className="mt-4 rounded-2xl border border-cyan-300/20 bg-cyan-400/10 px-4 py-3 text-left">
                      <div className="flex items-start gap-3">
                        <Loader2 className="mt-0.5 h-4 w-4 shrink-0 animate-spin text-cyan-200" />
                        <div>
                          <p className="text-sm font-semibold text-cyan-50">{projectSplitTitle}</p>
                          <p className="mt-1 text-sm leading-6 text-cyan-100/90">{projectSplitDescription}</p>
                          <p className="mt-2 text-xs text-cyan-100/75">{projectSplitFootnote}</p>
                        </div>
                      </div>
                    </div>
                  )}
                </div>
              </div>
            </div>

            <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3 xl:w-[720px]">
              {projectQuickActions.map((card) => (
                <div key={card.title} className="rounded-xl border border-white/15 bg-white/5 p-3">
                  <div className="flex items-center gap-2 mb-1.5">
                    {card.icon}
                    <span className="text-sm font-semibold text-white">{card.title}</span>
                  </div>
                  <p className="text-[11px] text-surface-300 leading-relaxed mb-2">{card.desc}</p>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={card.onClick}
                    disabled={card.disabled}
                    className="w-full border-white/20 bg-white/10 text-white hover:bg-white/20 text-xs"
                  >
                    {card.loading ? <RefreshCw className="mr-1.5 h-3 w-3 animate-spin" /> : null}
                    {card.label}
                  </Button>
                </div>
              ))}
            </div>
          </div>
        </div>
      ) : (
        <div className="overflow-hidden rounded-[28px] border border-surface-200 bg-white p-6 shadow-sm">
          <div className="flex flex-col gap-5 xl:flex-row xl:items-start xl:justify-between">
            <div className="max-w-3xl">
              <div className="mb-4 inline-flex items-center gap-2 rounded-full border border-primary-100 bg-primary-50 px-3 py-1.5 text-xs font-medium text-primary-700">
                <ListVideo className="h-3.5 w-3.5" />
                当前分集工作台
              </div>
              <div className="flex items-start gap-4">
                <Button
                  variant="outline"
                  size="icon"
                  className="mt-0.5 shrink-0 rounded-2xl"
                  onClick={() => setSelectedEpisodeId(null)}
                >
                  <ArrowLeft className="h-4 w-4" />
                </Button>
                <div>
                  <div className="flex flex-wrap items-center gap-2">
                    <h2 className="text-2xl font-semibold tracking-tight text-surface-900">
                      第 {selectedEpisode?.episode_number ?? selectedEpisodeId} 集 · {selectedEpisode?.title || '未命名分集'}
                    </h2>
                    {selectedEpisode?.status ? <StatusBadge status={selectedEpisode.status} /> : null}
                  </div>
                  <p className="mt-3 max-w-2xl text-sm leading-6 text-surface-600">
                    {selectedEpisode?.summary?.trim() || '当前处于单集工作模式，顶部仅展示本集相关入口，不再展示项目级全局操作。'}
                  </p>
                  <div className="mt-4 flex flex-wrap gap-2 text-xs text-surface-500">
                    <span className="rounded-full border border-surface-200 bg-surface-50 px-3 py-1">
                      资源 {selectedEpisodeAssetSummary}
                    </span>
                    <span className="rounded-full border border-surface-200 bg-surface-50 px-3 py-1">
                      分镜 {selectedEpisodeStoryboardSummary}
                    </span>
                    <span className="rounded-full border border-surface-200 bg-surface-50 px-3 py-1">
                      更新时间 {selectedEpisodeUpdatedLabel}
                    </span>
                  </div>
                </div>
              </div>
            </div>

            <div className="grid gap-3 sm:grid-cols-2 xl:w-[520px]">
              {/* 当前集资源 */}
              <div className="rounded-xl border border-violet-200 bg-violet-50/50 p-3">
                <div className="flex items-center gap-2 mb-1">
                  <ImageIcon className="h-4 w-4 text-violet-600" />
                  <span className="text-sm font-semibold text-surface-900">本集资源</span>
                </div>
                <p className="text-[11px] text-surface-500 leading-relaxed mb-2">
                  管理、生成当前分集的角色、场景、道具等素材资源。
                </p>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => openEpisodeWorkspace(selectedEpisode?.id, 'assets')}
                  className="w-full border-violet-200 bg-white text-violet-700 hover:bg-violet-100 text-xs"
                >
                  进入资源
                </Button>
              </div>

              {/* 当前集分镜 */}
              <div className="rounded-xl border border-primary-200 bg-primary-50/50 p-3">
                <div className="flex items-center gap-2 mb-1">
                  <LayoutGrid className="h-4 w-4 text-primary-600" />
                  <span className="text-sm font-semibold text-surface-900">本集分镜</span>
                </div>
                <p className="text-[11px] text-surface-500 leading-relaxed mb-2">
                  查看、编辑本集分镜序列，生成与调整镜头画面。
                </p>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => openEpisodeWorkspace(selectedEpisode?.id, 'storyboard')}
                  className="w-full border-primary-200 bg-white text-primary-700 hover:bg-primary-100 text-xs"
                >
                  进入分镜
                </Button>
              </div>

              {/* 当前集配音 */}
              <div className="rounded-xl border border-emerald-200 bg-emerald-50/50 p-3">
                <div className="flex items-center gap-2 mb-1">
                  <Volume2 className="h-4 w-4 text-emerald-600" />
                  <span className="text-sm font-semibold text-surface-900">本集配音</span>
                </div>
                <p className="text-[11px] text-surface-500 leading-relaxed mb-2">
                  为本集角色台词生成配音，调节语速、音调与情感。
                </p>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => openEpisodeWorkspace(selectedEpisode?.id, 'dubbing')}
                  className="w-full border-emerald-200 bg-white text-emerald-700 hover:bg-emerald-100 text-xs"
                >
                  进入配音
                </Button>
              </div>

              {/* 当前集字幕 */}
              <div className="rounded-xl border border-cyan-200 bg-cyan-50/50 p-3">
                <div className="flex items-center gap-2 mb-1">
                  <FileText className="h-4 w-4 text-cyan-600" />
                  <span className="text-sm font-semibold text-surface-900">本集字幕</span>
                </div>
                <p className="text-[11px] text-surface-500 leading-relaxed mb-2">
                  生成、编辑本集时间轴字幕，支持多语言翻译。
                </p>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => openEpisodeWorkspace(selectedEpisode?.id, 'dubbing')}
                  className="w-full border-cyan-200 bg-white text-cyan-700 hover:bg-cyan-100 text-xs"
                >
                  进入字幕
                </Button>
              </div>

              {/* 当前集视频 */}
              <div className="rounded-xl border border-amber-200 bg-amber-50/50 p-3 sm:col-span-2">
                <div className="flex items-center gap-2 mb-1">
                  <Video className="h-4 w-4 text-amber-600" />
                  <span className="text-sm font-semibold text-surface-900">本集视频成片</span>
                </div>
                <p className="text-[11px] text-surface-500 leading-relaxed mb-2">
                  将本集分镜、配音、字幕合成为最终视频文件，支持预览与导出。
                </p>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => openEpisodeWorkspace(selectedEpisode?.id, 'video')}
                  className="w-full border-amber-200 bg-white text-amber-700 hover:bg-amber-100 text-xs"
                >
                  进入视频
                </Button>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Main Layout */}
      <div className="grid gap-4 lg:grid-cols-[272px_minmax(0,1fr)]">
        <aside className="lg:sticky lg:top-4 lg:self-start">
          <div className="rounded-2xl border border-surface-200 bg-white p-3 shadow-sm">
            <div className="px-2 pb-2 pt-1 text-[11px] font-semibold uppercase tracking-widest text-surface-400">
              项目总览
            </div>
            <button
              onClick={() => setSelectedEpisodeId(null)}
              className={`group flex w-full cursor-pointer items-center gap-3 rounded-xl border px-3 py-3 text-sm font-medium transition-all ${
                selectedEpisodeId === null
                  ? 'border-primary-200 bg-primary-50 text-primary-700 shadow-sm'
                  : 'border-surface-200 bg-surface-50 text-surface-700 hover:border-primary-200 hover:bg-primary-50/60 hover:text-primary-700'
              }`}
            >
              <span className={`flex h-7 w-7 shrink-0 items-center justify-center rounded-lg ${
                selectedEpisodeId === null ? 'bg-primary-100 text-primary-600' : 'bg-surface-100 text-surface-500 group-hover:bg-primary-100 group-hover:text-primary-600'
              }`}>
                <BookOpen className="h-3.5 w-3.5" />
              </span>
              <span className="flex-1 text-left">剧本大纲与分集</span>
              <ChevronRight className={`h-4 w-4 shrink-0 transition-transform ${
                selectedEpisodeId === null ? 'text-primary-400' : 'text-surface-300 group-hover:translate-x-0.5 group-hover:text-primary-400'
              }`} />
            </button>
            
            <div className="mb-2 mt-4 px-2 pb-1 text-[11px] font-semibold uppercase tracking-widest text-surface-400">
              单集工作区
            </div>
            <div className="space-y-1.5 max-h-[60vh] overflow-y-auto pr-0.5">
              {episodes.map(ep => {
                const meta = episodeWorkspaceMeta.get(ep.id)
                const assetTotal = meta?.assetTotal ?? 0
                const assetCompleted = meta?.assetCompleted ?? 0
                const assetFailed = meta?.assetFailed ?? 0
                const storyboardTotal = meta?.storyboardTotal ?? 0
                const storyboardCompleted = meta?.storyboardCompleted ?? 0
                const isAssetExtracting = meta?.assetExtracting ?? false
                const isAssetGenerating = meta?.assetGenerating ?? false
                const isStoryboardGenerating = meta?.storyboardGenerating ?? false
                const hasStoryboardFailure = meta?.storyboardFailed ?? false
                const hasAssetFailure = assetFailed > 0
                const resourceSummary = assetTotal > 0 ? `${assetCompleted}/${assetTotal}` : '0/-'
                const storyboardSummary = storyboardTotal > 0 ? `${storyboardCompleted}/${storyboardTotal}` : '0/-'

                const episodePhase = (() => {
                  if (ep.status === 'failed' || (storyboardTotal > 0 && hasStoryboardFailure)) {
                    return {
                      label: '当前阶段：异常待处理',
                      className: 'text-red-600',
                    }
                  }
                  if (isStoryboardGenerating || ep.status === 'scene_splitting') {
                    return {
                      label: '当前阶段：分镜提取中',
                      className: 'text-blue-600',
                    }
                  }
                  if (storyboardTotal > 0) {
                    return {
                      label: '当前阶段：分镜已就绪',
                      className: 'text-violet-600',
                    }
                  }
                  if (isAssetExtracting) {
                    return {
                      label: '当前阶段：资源提取中',
                      className: 'text-amber-600',
                    }
                  }
                  if (isAssetGenerating) {
                    return {
                      label: '当前阶段：资源生成中',
                      className: 'text-blue-600',
                    }
                  }
                  if (assetTotal > 0) {
                    return {
                      label: '当前阶段：等待自动分镜',
                      className: 'text-primary-600',
                    }
                  }
                  return {
                    label: '当前阶段：等待资源提取',
                    className: 'text-surface-400',
                  }
                })()

                const storyboardBadge = (() => {
                  if (ep.status === 'failed' && storyboardTotal === 0) {
                    return (
                      <span className="inline-flex items-center gap-1 rounded bg-red-100 px-1.5 py-0.5 text-[10px] font-medium text-red-700">
                        分镜异常
                      </span>
                    )
                  }
                  if (isStoryboardGenerating || ep.status === 'scene_splitting') {
                    return (
                      <span className="inline-flex items-center gap-1 rounded bg-blue-100 px-1.5 py-0.5 text-[10px] font-medium text-blue-700">
                        <Loader2 className="h-3 w-3 animate-spin" /> 分镜提取中 {storyboardTotal > 0 ? `${storyboardCompleted}/${storyboardTotal}` : ''}
                      </span>
                    )
                  }
                  if (storyboardTotal > 0 && hasStoryboardFailure) {
                    return (
                      <span className="inline-flex items-center gap-1 rounded bg-red-100 px-1.5 py-0.5 text-[10px] font-medium text-red-700">
                        分镜异常 {storyboardCompleted}/{storyboardTotal}
                      </span>
                    )
                  }
                  if (storyboardTotal > 0) {
                    return (
                      <span className="inline-flex items-center gap-1 rounded bg-violet-100 px-1.5 py-0.5 text-[10px] font-medium text-violet-700">
                        分镜就绪 {storyboardCompleted}/{storyboardTotal}
                      </span>
                    )
                  }
                  if (isAssetExtracting || isAssetGenerating) {
                    return (
                      <span className="inline-flex items-center gap-1 rounded bg-surface-100 px-1.5 py-0.5 text-[10px] font-medium text-surface-500">
                        等待资源完成
                      </span>
                    )
                  }
                  if (assetTotal > 0) {
                    return (
                      <span className="inline-flex items-center gap-1 rounded bg-blue-100 px-1.5 py-0.5 text-[10px] font-medium text-blue-700">
                        待自动分镜
                      </span>
                    )
                  }
                  return (
                    <span className="inline-flex items-center gap-1 rounded bg-surface-100 px-1.5 py-0.5 text-[10px] font-medium text-surface-500">
                      分镜待提取
                    </span>
                  )
                })()

                const assetBadge = (() => {
                  if (isAssetExtracting) {
                    return (
                      <span className="inline-flex items-center gap-1 rounded bg-yellow-100 px-1.5 py-0.5 text-[10px] font-medium text-yellow-800">
                        <Loader2 className="h-3 w-3 animate-spin" /> 资源提取中
                      </span>
                    )
                  }
                  if (assetTotal === 0) {
                    return (
                      <span className="inline-flex items-center gap-1 rounded bg-surface-100 px-1.5 py-0.5 text-[10px] font-medium text-surface-500">
                        暂无资源
                      </span>
                    )
                  }
                  if (isAssetGenerating) {
                    return (
                      <span className="inline-flex items-center gap-1 rounded bg-blue-100 px-1.5 py-0.5 text-[10px] font-medium text-blue-700">
                        <Loader2 className="h-3 w-3 animate-spin" /> 资源生成中 {assetCompleted}/{assetTotal}
                      </span>
                    )
                  }
                  if (hasAssetFailure) {
                    return (
                      <span className="inline-flex items-center gap-1 rounded bg-red-100 px-1.5 py-0.5 text-[10px] font-medium text-red-700">
                        资源异常 {assetCompleted}/{assetTotal}
                      </span>
                    )
                  }
                  return (
                    <span className="inline-flex items-center gap-1 rounded bg-green-100 px-1.5 py-0.5 text-[10px] font-medium text-green-800">
                      <CheckCircle2 className="h-3 w-3" /> 资源就绪 {assetCompleted}/{assetTotal}
                    </span>
                  )
                })()

                return (
                  <button
                    key={ep.id}
                    onClick={() => openEpisodeWorkspace(ep.id, 'assets')}
                    className={`group flex flex-col w-full cursor-pointer text-left rounded-xl px-3 py-3 text-sm transition-all border ${
                      selectedEpisodeId === ep.id
                        ? 'bg-primary-50 border-primary-200 text-primary-800 shadow-sm'
                        : 'bg-white border-surface-200 text-surface-700 hover:border-primary-200 hover:bg-primary-50/60 hover:shadow-sm'
                    }`}
                  >
                    <div className="flex items-center justify-between font-medium gap-2">
                      <span className="flex items-center gap-2 truncate">
                        <span className={`flex h-6 w-6 shrink-0 items-center justify-center rounded-md text-[11px] font-bold ${
                          selectedEpisodeId === ep.id ? 'bg-primary-200 text-primary-700' : 'bg-surface-100 text-surface-500 group-hover:bg-primary-100 group-hover:text-primary-600'
                        }`}>
                          {ep.episode_number}
                        </span>
                        <span className="truncate">{ep.title || '未命名分集'}</span>
                      </span>
                      <ChevronRight className={`h-3.5 w-3.5 shrink-0 transition-transform ${
                        selectedEpisodeId === ep.id ? 'text-primary-400' : 'text-surface-300 group-hover:translate-x-0.5 group-hover:text-primary-400'
                      }`} />
                    </div>
                    <p className={`mt-1.5 text-[11px] font-medium ${episodePhase.className}`}>
                      {episodePhase.label}
                    </p>
                    <p className="mt-0.5 text-[11px] text-surface-400">
                      资源 {resourceSummary} · 分镜 {storyboardSummary}
                    </p>
                    <div className="mt-2 flex flex-wrap gap-1">
                      {assetBadge}
                      {storyboardBadge}
                    </div>
                  </button>
                )
              })}
              {episodes.length === 0 && (
                projectSplitInProgress ? (
                  <div className="rounded-xl border border-blue-200 bg-blue-50 px-3 py-5 text-center">
                    <div className="flex flex-col items-center gap-2 text-blue-700">
                      <Loader2 className="h-4 w-4 animate-spin" />
                      <p className="text-sm font-medium">正在自动生成分集</p>
                      <p className="max-w-[220px] text-xs leading-5 text-blue-600">
                        {projectSplitTotal > 0
                          ? `已识别 ${projectSplitCompleted}/${projectSplitTotal}，完成后分集会自动出现在这里。`
                          : '系统正在分析剧本，分集生成后会自动出现在这里。'}
                      </p>
                    </div>
                  </div>
                ) : (
                  <div className="rounded-xl border border-dashed border-surface-200 px-3 py-5 text-center text-xs text-surface-400">
                    暂无分集数据，请先生成大纲
                  </div>
                )
              )}
            </div>
          </div>

        </aside>

        {/* Content Area */}
        <div className="min-h-[400px] min-w-0">
          <ProjectEpisodeFilterContext.Provider value={sharedEpisodeFilterValue}>
            {selectedEpisodeId === null ? (
              <div className="space-y-6">
                <div className="rounded-3xl border border-surface-200 bg-white p-5 shadow-sm">
                  <div className="grid gap-5 xl:grid-cols-[minmax(0,1.5fr)_360px]">
                    <div className="space-y-5">
                      <div>
                        <div className="flex flex-wrap items-center gap-2 text-xs text-surface-500">
                          <span className="rounded-full border border-surface-200 bg-surface-50 px-2.5 py-1">
                            项目总览
                          </span>
                          <span className="rounded-full border border-surface-200 bg-surface-50 px-2.5 py-1">
                            剧集 {episodes.length}
                          </span>
                          <span className="rounded-full border border-surface-200 bg-surface-50 px-2.5 py-1">
                            风格标签 {project.style_tags?.length ?? 0}
                          </span>
                          <span className="inline-flex items-center gap-1 rounded-full border border-surface-200 bg-surface-50 px-2.5 py-1">
                            <Clock3 className="h-3.5 w-3.5" /> 目标 {project.target_episodes || 0} 集
                          </span>
                        </div>
                        <h3 className="mt-3 text-xl font-semibold text-surface-900">剧本大纲与项目总控</h3>
                        <p className="mt-2 text-sm leading-6 text-surface-600">
                          在这里统览整个项目的剧本拆分、资源提取、分镜制作与后续成片进度，再决定优先进入哪一集继续处理。
                        </p>
                      </div>

                    </div>

                  </div>
                </div>

                <ScriptTab
                  projectId={projectId}
                  project={project}
                  mutateProject={mutateProject}
                  onAutoStoryboardQueued={() => {
                    autoOpenedRef.current = false
                    setAutoStoryboardPending(true)
                  }}
                />
              </div>
            ) : (
              <EpisodeWorkspace 
                projectId={projectId} 
                episodeId={selectedEpisodeId} 
                episode={selectedEpisode}
                project={project} 
                initialTab={episodeEntryTab}
                initialAwaitingAutoStoryboard={autoStoryboardPending}
              />
            )}
          </ProjectEpisodeFilterContext.Provider>
        </div>
      </div>

      <StorageDrawer projectId={projectId} open={storageOpen} onClose={() => setStorageOpen(false)} />
    </div>
  )
}
