'use client'
import { useParams, useRouter } from 'next/navigation'
import useSWR from 'swr'
import { useState } from 'react'
import { projectAPI, sceneAPI } from '@/lib/api'
import type { Episode, Project, Scene } from '@/types'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { StoryboardBoard } from '@/components/storyboard/StoryboardBoard'
import { LoadingSpinner } from '@/components/common/LoadingSpinner'
import { EmptyState } from '@/components/common/EmptyState'
import { ArrowLeft, Clapperboard, FileText, Image, Sparkles } from 'lucide-react'
import { Button } from '@/components/ui/button'

export default function StoryboardPage() {
  const router = useRouter()
  const params = useParams()
  const projectId = Number(params.id)
  const [scenes, setScenes] = useState<Record<number, Scene[]>>({})

  const { data: projectRaw } = useSWR(
    ['project', projectId],
    () => projectAPI.get(projectId) as unknown as Promise<{ data: Project }>
  )
  const project = (projectRaw as { data?: Project })?.data

  const { data: episodesData, isLoading: epLoading } = useSWR(
    ['episodes', projectId],
    () =>
      projectAPI.listEpisodes(projectId) as unknown as Promise<{
        data: Episode[]
      }>
  )

  const episodes: Episode[] =
    (episodesData as { data?: Episode[] })?.data ?? []

  const loadScenes = async (episodeId: number) => {
    if (scenes[episodeId]) return
    const res = (await sceneAPI.list(episodeId)) as unknown as {
      data: Scene[]
    }
    setScenes((prev) => ({ ...prev, [episodeId]: res.data ?? [] }))
  }

  if (epLoading) {
    return (
      <div className="flex h-64 items-center justify-center">
        <LoadingSpinner size="lg" />
      </div>
    )
  }

  if (episodes.length === 0) {
    return (
      <div className="space-y-6">
        <div className="overflow-hidden rounded-[28px] border border-surface-200/70 bg-gradient-to-br from-slate-950 via-violet-950 to-slate-900 p-6 text-white shadow-sm">
          <div className="flex flex-col gap-6 lg:flex-row lg:items-start lg:justify-between">
            <div className="max-w-2xl">
              <div className="mb-4 inline-flex items-center gap-2 rounded-full border border-white/10 bg-white/10 px-3 py-1.5 text-xs font-medium text-surface-100 backdrop-blur">
                <Sparkles className="h-3.5 w-3.5 text-primary-300" />
                分镜叙事工作区
              </div>
              <div className="flex items-start gap-4">
                <Button
                  variant="ghost"
                  size="icon"
                  className="mt-0.5 shrink-0 rounded-2xl border border-white/10 bg-white/10 text-white hover:bg-white/15 hover:text-white"
                  onClick={() => router.push(`/video/${projectId}`)}
                >
                  <ArrowLeft className="h-4 w-4" />
                </Button>
                <div>
                  <h2 className="text-2xl font-semibold tracking-tight text-white">分镜看板</h2>
                  <p className="mt-2 text-sm leading-6 text-surface-300">
                    {project?.title
                      ? `${project.title} · 先创建分集，再进入分镜生成与镜头编排。`
                      : '先创建分集，再进入分镜生成与镜头编排。'}
                  </p>
                </div>
              </div>
            </div>

            <div className="grid gap-3 sm:grid-cols-2">
              <div className="rounded-2xl border border-white/10 bg-white/10 p-4 backdrop-blur">
                <div className="flex items-center gap-2 text-surface-300">
                  <FileText className="h-4 w-4 text-cyan-300" />
                  <span className="text-xs uppercase tracking-[0.2em]">分集数量</span>
                </div>
                <p className="mt-3 text-2xl font-semibold text-white">0</p>
                <p className="mt-1 text-xs text-surface-400">还没有可供拆分的剧集</p>
              </div>
              <div className="rounded-2xl border border-white/10 bg-white/10 p-4 backdrop-blur">
                <div className="flex items-center gap-2 text-surface-300">
                  <Image className="h-4 w-4 text-violet-300" />
                  <span className="text-xs uppercase tracking-[0.2em]">分镜状态</span>
                </div>
                <p className="mt-3 text-2xl font-semibold text-white">待开始</p>
                <p className="mt-1 text-xs text-surface-400">生成后即可进入画面编辑与排序</p>
              </div>
            </div>
          </div>
        </div>

        <div className="rounded-[28px] border border-surface-200 bg-gradient-to-br from-white to-surface-50 p-3 shadow-sm">
          <div className="rounded-[24px] border border-dashed border-surface-200 bg-[radial-gradient(circle_at_top_left,_rgba(99,102,241,0.08),_transparent_30%),radial-gradient(circle_at_bottom_right,_rgba(236,72,153,0.08),_transparent_28%)] p-4">
            <EmptyState
              icon={Clapperboard}
              title="暂无分集"
              description="请先在项目中创建分集，然后运行分镜生成流程"
            />
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div className="overflow-hidden rounded-[28px] border border-surface-200/70 bg-gradient-to-br from-slate-950 via-violet-950 to-slate-900 p-6 text-white shadow-sm">
        <div className="flex flex-col gap-6 lg:flex-row lg:items-start lg:justify-between">
          <div className="max-w-2xl">
            <div className="mb-4 inline-flex items-center gap-2 rounded-full border border-white/10 bg-white/10 px-3 py-1.5 text-xs font-medium text-surface-100 backdrop-blur">
              <Sparkles className="h-3.5 w-3.5 text-primary-300" />
              分镜叙事工作区
            </div>
            <div className="flex items-start gap-4">
              <Button
                variant="ghost"
                size="icon"
                className="mt-0.5 shrink-0 rounded-2xl border border-white/10 bg-white/10 text-white hover:bg-white/15 hover:text-white"
                onClick={() => router.push(`/video/${projectId}`)}
              >
                <ArrowLeft className="h-4 w-4" />
              </Button>
              <div>
                <h2 className="text-2xl font-semibold tracking-tight text-white">分镜看板</h2>
                <p className="mt-2 text-sm leading-6 text-surface-300">
                  {project?.title
                    ? `${project.title} · 拖拽调整场景顺序，点击编辑 Prompt，持续优化镜头表达。`
                    : '拖拽调整场景顺序，点击编辑 Prompt。'}
                </p>
              </div>
            </div>
          </div>

          <div className="grid gap-3 sm:grid-cols-2">
            <div className="rounded-2xl border border-white/10 bg-white/10 p-4 backdrop-blur">
              <div className="flex items-center gap-2 text-surface-300">
                <FileText className="h-4 w-4 text-cyan-300" />
                <span className="text-xs uppercase tracking-[0.2em]">分集数量</span>
              </div>
              <p className="mt-3 text-2xl font-semibold text-white">{episodes.length}</p>
              <p className="mt-1 text-xs text-surface-400">可切换不同剧集查看分镜场景</p>
            </div>
            <div className="rounded-2xl border border-white/10 bg-white/10 p-4 backdrop-blur">
              <div className="flex items-center gap-2 text-surface-300">
                <Image className="h-4 w-4 text-violet-300" />
                <span className="text-xs uppercase tracking-[0.2em]">当前状态</span>
              </div>
              <p className="mt-3 text-2xl font-semibold text-white">可编辑</p>
              <p className="mt-1 text-xs text-surface-400">进入单集后即可继续调整场景与提示词</p>
            </div>
          </div>
        </div>
      </div>

      <div className="rounded-[24px] border border-surface-200 bg-white/80 p-4 shadow-sm backdrop-blur">
        <p className="text-sm font-medium text-surface-900">剧集切换</p>
        <p className="mt-1 text-xs text-surface-500">按集查看分镜内容，逐步完善镜头、顺序和画面描述。</p>
      </div>

      <Tabs
        defaultValue={String(episodes[0]?.id)}
        onValueChange={(v) => loadScenes(Number(v))}
      >
        <TabsList className="h-auto flex-wrap justify-start gap-2 bg-transparent p-0">
          {episodes.map((ep) => (
            <TabsTrigger
              key={ep.id}
              value={String(ep.id)}
              className="rounded-full border border-surface-200 bg-white px-4 py-2 data-[state=active]:border-primary-200 data-[state=active]:bg-primary-50"
            >
              第 {ep.episode_number} 集
            </TabsTrigger>
          ))}
        </TabsList>

        {episodes.map((ep) => (
          <TabsContent key={ep.id} value={String(ep.id)}>
            <EpisodeScenes
              episodeId={ep.id}
              initialScenes={scenes[ep.id]}
              onLoad={() => loadScenes(ep.id)}
            />
          </TabsContent>
        ))}
      </Tabs>
    </div>
  )
}

function EpisodeScenes({
  episodeId,
  initialScenes,
  onLoad,
}: {
  episodeId: number
  initialScenes?: Scene[]
  onLoad: () => void
}) {
  const [localScenes, setLocalScenes] = useState<Scene[] | undefined>(initialScenes)

  useSWR(
    ['scenes', episodeId],
    async () => {
      const res = (await sceneAPI.list(episodeId)) as unknown as {
        data: Scene[]
      }
      const items = res.data ?? []
      setLocalScenes(items)
      return items
    },
    { onSuccess: (data) => setLocalScenes(data) }
  )

  if (localScenes === undefined) {
    return (
      <div className="flex h-40 items-center justify-center">
        <LoadingSpinner />
      </div>
    )
  }

  if (localScenes.length === 0) {
    return (
      <EmptyState
        icon={Clapperboard}
        title="本集暂无场景"
        description="运行分镜生成后，场景将显示在这里"
      />
    )
  }

  return (
    <StoryboardBoard
      scenes={localScenes}
      episodeId={episodeId}
      onScenesChange={setLocalScenes}
    />
  )
}
