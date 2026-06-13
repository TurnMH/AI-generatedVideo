'use client'

import React, { useEffect, useMemo, useRef, useState } from 'react'
import { useParams, usePathname, useRouter, useSearchParams } from 'next/navigation'
import {
  FileText,
  Image as ImageIcon,
  LayoutGrid,
  Sparkles,
  Video,
  Volume2,
} from 'lucide-react'
import { projectAPI, assetAPI, storyboardAPI } from '@/lib/api'
import { Button } from '@/components/ui/button'
import { LoadingSpinner } from '@/components/common/LoadingSpinner'
import { useToast } from '@/components/ui/toast'
import { ProjectEpisodeFilterContext } from '@/lib/projects/episode-filter'
import { useEpisodeAutoPipeline } from '@/lib/projects/use-episode-auto-pipeline'
import { useProjectDetailData } from '@/lib/projects/use-project-detail-data'
import { fetchModelIdentity } from '@/lib/model-selection'
import { resolveProjectIdParam } from '@/lib/project-route'
import { EpisodeWorkspace } from '@/components/projects/detail/EpisodeWorkspace'
import { EpisodeTabLink, HeaderStatPill, ProjectDetailHeader } from '@/components/projects/detail/ProjectDetailHeader'
import { ProjectEpisodeSidebar } from '@/components/projects/detail/ProjectEpisodeSidebar'
import { ProjectOverviewPanel } from '@/components/projects/detail/ProjectOverviewPanel'
import { ProjectPipelineBanners } from '@/components/projects/detail/ProjectPipelineBanners'
import { ProjectQuickActionsGrid } from '@/components/projects/detail/ProjectQuickActionsGrid'
import { StorageDrawer } from '@/components/projects/detail/StorageDrawer'
import { ScriptTab } from '@/components/projects/detail/tabs/ScriptTab'

export default function ProjectDetailPage() {
  const params = useParams()
  const pathname = usePathname()
  const router = useRouter()
  const searchParams = useSearchParams()
  const routeBase = '/projects'
  const projectId = resolveProjectIdParam(params.id, pathname, 'projects') ?? 0
  const hasValidProjectId = projectId > 0

  const [selectedEpisodeId, setSelectedEpisodeId] = useState<number | null>(null)
  const [storageOpen, setStorageOpen] = useState(false)
  const [sharedEpisodeFilter, setSharedEpisodeFilter] = useState<string>('all')
  const [episodeEntryTab, setEpisodeEntryTab] = useState<'assets' | 'storyboard' | 'dubbing' | 'video'>('assets')
  const [autoStoryboardPending, setAutoStoryboardPending] = useState(() => searchParams.get('autoStart') === '1')
  const autoOpenedRef = useRef(false)
  const { toast } = useToast()

  const sharedEpisodeFilterValue = useMemo(
    () => ({ value: sharedEpisodeFilter, setValue: setSharedEpisodeFilter }),
    [sharedEpisodeFilter],
  )

  useEffect(() => {
    if (!hasValidProjectId) router.replace(routeBase)
  }, [hasValidProjectId, routeBase, router])

  const detail = useProjectDetailData(projectId, hasValidProjectId)
  const {
    project,
    episodes,
    isLoading,
    mutateProject,
    mutateAssetsCount,
    mutateStoryboardsData,
    stepperAssetsRaw,
    stepperStoryboardsRaw,
    episodeWorkspaceMeta,
    projectSharedAssetCount,
    hasUsableScriptContent,
    assetExtractionRunning,
    pipelineSnapshot,
    projectOverview,
    structuredPhaseLabel,
    structuredNextStep,
    projectSplitTotal,
    projectSplitCompleted,
    projectControlStages,
    projectControlDoneCount,
    projectControlOverallProgress,
    projectControlCurrentStage,
    firstEpisodeAutoActive,
    isExtractingStoryboards,
    setIsExtractingStoryboards,
    isExtractingAssets,
    setIsExtractingAssets,
    isGeneratingProjectImages,
    setIsGeneratingProjectImages,
  } = detail

  useEffect(() => {
    if (project?.project_type === 'video_serial') {
      router.replace(`/video-serial/${projectId}`)
    }
  }, [project, projectId, router])

  useEffect(() => {
    if (!autoStoryboardPending || autoOpenedRef.current || episodes.length === 0) return
    autoOpenedRef.current = true
    setSelectedEpisodeId(episodes[0].id)
    setEpisodeEntryTab('assets')
    const url = new URL(window.location.href)
    url.searchParams.delete('autoStart')
    router.replace(url.pathname + (url.search || ''))
  }, [autoStoryboardPending, episodes, router])

  const { autoPreparingEpisodeId, getEpisodeAutoAction, handleEpisodeAutoPipeline } = useEpisodeAutoPipeline({
    projectId,
    episodes,
    episodeWorkspaceMeta,
    firstEpisodeAutoActive,
    mutateProject,
    toast,
  })

  const projectSplitInProgress = selectedEpisodeId === null && pipelineSnapshot.phase === 'episode_splitting'
  const selectedEpisode = selectedEpisodeId == null ? undefined : episodes.find((ep) => ep.id === selectedEpisodeId)
  const selectedEpisodeMeta = selectedEpisode ? episodeWorkspaceMeta.get(selectedEpisode.id) : undefined

  const openEpisodeWorkspace = (
    targetEpisodeId?: number | null,
    tab: 'assets' | 'storyboard' | 'dubbing' | 'video' = 'assets',
  ) => {
    if (!targetEpisodeId) {
      toast({ title: '暂无可进入的分集', description: '请先生成分集后再继续后续制作流程。', variant: 'default' })
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
    if (!hasUsableScriptContent) {
      toast({
        title: '项目资源提取暂不可用',
        description: '当前项目还没有可用剧本内容，请先上传剧本，或先完成分集/优化后再提取资源。',
        variant: 'destructive',
      })
      return
    }
    setIsExtractingAssets(true)
    try {
      await assetAPI.extract(projectId)
      await mutateAssetsCount()
      toast({
        title: '已提交至大模型处理队列',
        description: '系统正在识别剧本中的角色、场景、道具等资源条目。提取完成后请手动生成资源图或点击各集「自动处理本集」。',
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
    if (!hasUsableScriptContent) {
      toast({
        title: '项目分镜提取暂不可用',
        description: '当前项目还没有可用剧本内容，请先完成剧本录入或优化后再提取分镜。',
        variant: 'destructive',
      })
      return
    }
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
      const { modelKey, modelLabel } = await fetchModelIdentity(project?.image_model_id)
      await storyboardAPI.generateAll(projectId, undefined, modelKey)
      await mutateStoryboardsData()
      toast({
        title: '已提交至图像生成模型队列',
        description: modelLabel
          ? `系统正在调用 ${modelLabel} 为全部分镜批量渲染图片。`
          : '系统正在调用图像生成模型为全部分镜批量渲染图片。',
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
      handleExtractProjectStoryboards()
      return
    }
    if (projectOverview.nextAction.type === 'select_episode' && projectOverview.nextAction.episodeId) {
      openEpisodeWorkspace(projectOverview.nextAction.episodeId, projectOverview.nextAction.tab ?? 'assets')
    }
  }

  const recoveryAction = (() => {
    if (!project || (project.status !== 'failed' && project.status !== 'paused')) return null
    if (assetExtractionRunning) return null
    const hasAssets = stepperAssetsRaw.some((asset: any) => asset?.name !== '__extracting__')
    const hasStoryboards = stepperStoryboardsRaw.length > 0
    if (episodes.length === 0) {
      return {
        label: '重新开始分集',
        description: '当前还没有可用分集，建议重新发起分集生成。',
        onClick: async () => {
          await projectAPI.generateEpisodes(projectId, undefined, { autoStoryboard: true })
          toast({ title: '已重新提交分集任务', description: '系统会重新拆分剧本并继续自动流程。', variant: 'success' })
        },
      }
    }
    if (!hasAssets) {
      return { label: '继续资源提取', description: '已保留现有分集，建议从资源提取继续。', onClick: handleExtractProjectAssets }
    }
    if (!hasStoryboards) {
      return { label: '继续分镜拆分', description: '资源已有结果，建议直接继续分镜拆分。', onClick: handleExtractProjectStoryboards }
    }
    return null
  })()

  const projectQuickActions = [
    {
      icon: <Sparkles className="h-4 w-4 text-emerald-300" />,
      title: '项目资源提取',
      desc: '从剧本中识别并提取全部角色、场景、道具。',
      label: '开始提取',
      onClick: handleExtractProjectAssets,
      loading: isExtractingAssets,
      disabled: isExtractingAssets || episodes.length === 0 || !hasUsableScriptContent,
    },
    {
      icon: <LayoutGrid className="h-4 w-4 text-violet-300" />,
      title: '镜头拆分',
      desc: '按项目维度统一拆分剧本为镜头条目。',
      label: '开始拆分',
      onClick: handleExtractProjectStoryboards,
      loading: isExtractingStoryboards,
      disabled: isExtractingStoryboards || episodes.length === 0 || !hasUsableScriptContent,
    },
    {
      icon: <ImageIcon className="h-4 w-4 text-amber-300" />,
      title: '分镜图片生成',
      desc: '为项目内全部分镜批量生成图片。',
      label: '开始生成',
      onClick: handleGenerateProjectImages,
      loading: isGeneratingProjectImages,
      disabled: isGeneratingProjectImages || projectOverview.stats.storyboardTotal === 0,
    },
    {
      icon: <Volume2 className="h-4 w-4 text-blue-300" />,
      title: '配音合成',
      desc: '根据角色台词自动生成语音。',
      label: '进入配音',
      onClick: () => openProjectStageFromOverview('dubbing'),
      disabled: episodes.length === 0,
    },
    {
      icon: <FileText className="h-4 w-4 text-cyan-300" />,
      title: '字幕生成',
      desc: '自动生成时间轴字幕。',
      label: '进入字幕',
      onClick: () => openProjectStageFromOverview('dubbing'),
      disabled: episodes.length === 0,
    },
    {
      icon: <Video className="h-4 w-4 text-rose-300" />,
      title: '视频成片',
      desc: '将分镜、配音、字幕合成为最终视频。',
      label: '进入视频',
      onClick: () => openProjectStageFromOverview('video'),
      disabled: episodes.length === 0 || projectOverview.stats.storyboardCompleted === 0,
    },
  ]

  const assetActiveCount = stepperAssetsRaw.filter(
    (asset: any) => asset?.name !== '__extracting__' && ['pending', 'generating', 'paused'].includes(asset?.status),
  ).length
  const storyboardRunning = stepperStoryboardsRaw.some((sb: any) => ['pending', 'generating', 'paused'].includes(sb?.status))

  if (!hasValidProjectId || isLoading) {
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
        <Button onClick={() => router.push(routeBase)}>返回列表</Button>
      </div>
    )
  }

  return (
    <div className="space-y-4">
      <ProjectPipelineBanners
        project={project}
        pipelineSnapshot={pipelineSnapshot}
        assetExtractionRunning={assetExtractionRunning}
        assetActiveCount={assetActiveCount}
        isExtractingAssets={isExtractingAssets}
        isExtractingStoryboards={isExtractingStoryboards}
        storyboardRunning={storyboardRunning}
        structuredPhaseLabel={structuredPhaseLabel}
        structuredNextStep={structuredNextStep}
        progressMessage={project.progress?.message}
        recoveryAction={recoveryAction}
      />

      <ProjectDetailHeader
        mode={selectedEpisodeId === null ? 'project' : 'episode'}
        project={project}
        episode={selectedEpisode}
        badgeLabel="项目总控台"
        badgeIcon={<Sparkles className="h-3.5 w-3.5" />}
        accentClass="text-primary-300"
        onBack={() => (selectedEpisodeId === null ? router.push(routeBase) : setSelectedEpisodeId(null))}
        description={
          selectedEpisodeId === null
            ? '管理剧本拆分、资源提取、分镜制作与成片入口。左侧选择单集进入工作台。'
            : selectedEpisode?.summary?.trim() || '在下方工作台处理本集资源、分镜、配音与视频。'
        }
        stats={
          selectedEpisodeId === null ? (
            <>
              <HeaderStatPill className="border-cyan-400/20 bg-cyan-400/10 text-cyan-100">剧集 {episodes.length}</HeaderStatPill>
              <HeaderStatPill className="border-emerald-400/20 bg-emerald-400/10 text-emerald-100">
                资源 {projectOverview.stats.assetCompleted}/{projectOverview.stats.assetTotal}
              </HeaderStatPill>
              <HeaderStatPill className="border-amber-400/20 bg-amber-400/10 text-amber-100">
                分镜 {projectOverview.stats.storyboardCompleted}/{projectOverview.stats.storyboardTotal}
              </HeaderStatPill>
            </>
          ) : (
            <>
              <HeaderStatPill className="border-surface-200 bg-surface-50 text-surface-600">
                资源 {selectedEpisodeMeta?.assetCompleted ?? 0}/{selectedEpisodeMeta?.assetTotal ?? 0}
              </HeaderStatPill>
              <HeaderStatPill className="border-surface-200 bg-surface-50 text-surface-600">
                分镜 {selectedEpisodeMeta?.storyboardCompleted ?? 0}/{selectedEpisodeMeta?.storyboardTotal ?? 0}
              </HeaderStatPill>
              {(['assets', 'storyboard', 'dubbing', 'video'] as const).map((tab) => (
                <EpisodeTabLink
                  key={tab}
                  label={tab === 'assets' ? '资源' : tab === 'storyboard' ? '分镜' : tab === 'dubbing' ? '配音' : '视频'}
                  onClick={() => openEpisodeWorkspace(selectedEpisode?.id, tab)}
                />
              ))}
            </>
          )
        }
        extraActions={selectedEpisodeId === null ? <ProjectQuickActionsGrid actions={projectQuickActions} /> : undefined}
      />

      <div className="grid gap-4 lg:grid-cols-[272px_minmax(0,1fr)]">
        <ProjectEpisodeSidebar
          episodes={episodes}
          selectedEpisodeId={selectedEpisodeId}
          episodeWorkspaceMeta={episodeWorkspaceMeta}
          projectSharedAssetCount={projectSharedAssetCount}
          projectSplitInProgress={projectSplitInProgress}
          projectSplitTotal={projectSplitTotal}
          projectSplitCompleted={projectSplitCompleted}
          getEpisodeAutoAction={getEpisodeAutoAction}
          onSelectOverview={() => setSelectedEpisodeId(null)}
          onSelectEpisode={(episodeId) => openEpisodeWorkspace(episodeId, 'assets')}
          onEpisodeAutoAction={(episodeId, event) => void handleEpisodeAutoPipeline(episodeId, event)}
        />

        <div className="min-h-[400px] min-w-0">
          <ProjectEpisodeFilterContext.Provider value={sharedEpisodeFilterValue}>
            {selectedEpisodeId === null ? (
              <div className="space-y-6">
                <ProjectOverviewPanel
                  project={project}
                  episodeCount={episodes.length}
                  pipelineSnapshot={pipelineSnapshot}
                  projectControlStages={projectControlStages}
                  projectControlDoneCount={projectControlDoneCount}
                  projectControlOverallProgress={projectControlOverallProgress}
                  projectControlCurrentStage={projectControlCurrentStage}
                  workflowSteps={projectOverview.workflowSteps}
                  notices={projectOverview.notices}
                  nextAction={projectOverview.nextAction}
                  onNextAction={() => void handleProjectOverviewAction()}
                />
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
                autoPipelineActive={autoPreparingEpisodeId === selectedEpisodeId}
              />
            )}
          </ProjectEpisodeFilterContext.Provider>
        </div>
      </div>

      <StorageDrawer projectId={projectId} open={storageOpen} onClose={() => setStorageOpen(false)} />
    </div>
  )
}
