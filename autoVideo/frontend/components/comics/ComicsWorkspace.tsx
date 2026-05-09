'use client'

import { useEffect, useMemo, useState } from 'react'
import { useRouter } from 'next/navigation'
import useSWR from 'swr'
import { BookOpen, Download, Loader2, Plus, RefreshCw, Sparkles } from 'lucide-react'
import { projectAPI, storyboardAPI } from '@/lib/api'
import type { Episode, Project, Storyboard } from '@/types'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent } from '@/components/ui/card'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { useToast } from '@/components/ui/toast'

const COMIC_STYLE_PRESETS = [
  { key: 'story-manga', label: '剧情漫画', desc: '保留人物表演与镜头叙事，适合连续剧情。', suffix: '剧情漫画分镜，黑白网点，清晰线稿，电影感构图，角色表情明确' },
  { key: 'action-comic', label: '热血战斗', desc: '强调速度线、冲击力与夸张动作。', suffix: '热血战斗漫画分镜，强透视，速度线，动作张力，夸张漫画构图' },
  { key: 'guofeng-comic', label: '国风条漫', desc: '适合神话、仙侠、古风题材。', suffix: '国风漫画分镜，东方服饰，长卷式构图，层次云雾，细腻线稿' },
] as const

type ComicStylePresetKey = (typeof COMIC_STYLE_PRESETS)[number]['key']

type ComicStatsData = { total: number; pending: number; generating: number; completed: number; failed: number; voided: number }

function StatusBadge({ status }: { status: string }) {
  const map: Record<string, string> = {
    pending: '待生成',
    generating: '生成中',
    completed: '已完成',
    failed: '失败',
    voided: '已作废',
  }
  const color = status === 'completed'
    ? 'success'
    : status === 'failed'
      ? 'destructive'
      : status === 'generating'
        ? 'default'
        : 'secondary'
  return <Badge variant={color as 'default' | 'secondary' | 'success' | 'destructive'}>{map[status] || status}</Badge>
}

function splitEpisodeIntoComicPanels(episode: Episode, panelCount: number, styleKey: ComicStylePresetKey) {
  const source = `${episode.summary || ''}\n${episode.script_excerpt || ''}`.trim() || episode.title
  const normalized = source.replace(/\r/g, '').replace(/\n{2,}/g, '\n').trim()
  const rawSegments = normalized.split(/[\n。！？!?；;]+/).map((segment) => segment.trim()).filter(Boolean)
  const style = COMIC_STYLE_PRESETS.find((preset) => preset.key === styleKey) ?? COMIC_STYLE_PRESETS[0]

  if (rawSegments.length === 0) {
    return [{ scene_description: `${episode.title}。${style.suffix}`, dialogue: episode.summary || episode.title }]
  }

  const limitedSegments = rawSegments.slice(0, Math.max(panelCount * 2, panelCount))
  const actualPanels = Math.min(panelCount, Math.max(1, limitedSegments.length))
  const chunkSize = Math.max(1, Math.ceil(limitedSegments.length / actualPanels))

  return Array.from({ length: actualPanels }, (_, index) => {
    const chunk = limitedSegments.slice(index * chunkSize, (index + 1) * chunkSize)
    const combined = chunk.join('，').trim()
    const dialogue = combined.length > 80 ? `${combined.slice(0, 80)}...` : combined
    const panelLead = index === 0 ? `第${episode.episode_number}集开场` : `第${episode.episode_number}集第${index + 1}格`
    return {
      scene_description: `${panelLead}：${combined || episode.title}。${style.suffix}`,
      dialogue,
    }
  })
}

export function ComicsWorkspace({ projectId, project }: { projectId: number; project: Project }) {
  const { toast } = useToast()
  const router = useRouter()
  const [episodeFilter, setEpisodeFilter] = useState<string>('all')
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
  const episodes = ((episodesData as { data?: Episode[] })?.data ?? []).slice().sort((a, b) => a.episode_number - b.episode_number)
  const selectedEpisodeId = episodeFilter !== 'all' ? Number(episodeFilter) : undefined
  const comicRefreshInterval = project.status === 'storyboard_generating' ? 4000 : 0

  const { data: comicStatsRaw, mutate: mutateComicStats } = useSWR(
    ['comic-stats', projectId],
    () => storyboardAPI.stats(projectId) as unknown as Promise<{ data: ComicStatsData }>,
    { refreshInterval: comicRefreshInterval }
  )
  const comicStats: ComicStatsData = (comicStatsRaw as { data?: ComicStatsData })?.data ?? { total: 0, pending: 0, generating: 0, completed: 0, failed: 0, voided: 0 }

  const { data: storyboardsRaw, isLoading, mutate: mutateComics } = useSWR(
    ['comic-storyboards', projectId, selectedEpisodeId ?? 'all'],
    () => storyboardAPI.listAll(projectId, selectedEpisodeId ? { episode_id: selectedEpisodeId } : undefined) as Promise<{ data: Storyboard[] }>,
    { refreshInterval: comicStats.generating > 0 || comicStats.pending > 0 || comicRefreshInterval > 0 ? 4000 : 0 }
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

  const filteredEpisodes = useMemo(() => episodeFilter === 'all' ? episodes : episodes.filter((episode) => String(episode.id) === episodeFilter), [episodes, episodeFilter])

  const panelsByEpisode = useMemo(() => {
    const grouped = new Map<number, Storyboard[]>()
    for (const panel of allPanels) {
      const episodeId = panel.episode_id ?? 0
      if (!grouped.has(episodeId)) grouped.set(episodeId, [])
      grouped.get(episodeId)!.push(panel)
    }
    return grouped
  }, [allPanels])

  const filteredPanels = useMemo(() => episodeFilter === 'all' ? allPanels : allPanels.filter((panel) => String(panel.episode_id ?? '') === episodeFilter), [allPanels, episodeFilter])
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
      toast({ title: '请先在项目内完成剧本分集', variant: 'destructive' })
      return
    }
    setCreatingPanels(true)
    try {
      const count = await createPanelsForEpisodes(filteredEpisodes)
      toast({ title: count > 0 ? `已创建 ${count} 个漫画分镜` : '当前筛选项目已存在漫画分镜', variant: count > 0 ? 'success' : 'default' })
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
      toast({ title: '请先在项目内完成剧本分集，再生成漫画分镜', variant: 'destructive' })
      return
    }
    setGeneratingPanels(true)
    try {
      await createPanelsForEpisodes(filteredEpisodes)
      const res = await storyboardAPI.generateAll(projectId, selectedEpisodeId) as unknown as { data?: { triggered?: number } }
      const triggered = res?.data?.triggered ?? 0
      toast({
        title: triggered > 0
          ? (selectedEpisodeId ? `当前分集已提交 ${triggered} 个漫画画格生成任务` : `已提交 ${triggered} 个漫画画格生成任务`)
          : (selectedEpisodeId ? '当前分集没有可生成的漫画画格' : '没有可生成的漫画画格'),
        variant: triggered > 0 ? 'success' : 'default',
      })
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
      if (panels.length === 0) await createPanelsForEpisodes([episode])
      const res = await storyboardAPI.generateAll(projectId, episode.id) as unknown as { data?: { triggered?: number } }
      const submitted = res?.data?.triggered ?? 0
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

  const gridClass = readingMode === 'grid' ? 'grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3' : 'grid grid-cols-1 gap-4'
  const openEpisodeGenerator = () => router.push(`/video/${projectId}`)

  if (isLoading) {
    return <div className="space-y-4 py-6">{[1, 2, 3].map((i) => <div key={i} className="h-20 animate-pulse rounded-lg bg-surface-100" />)}</div>
  }

  return (
    <div className="space-y-6">
      <div className="rounded-xl border border-violet-200 bg-gradient-to-r from-violet-50 via-fuchsia-50 to-white p-4">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <div className="flex items-center gap-2 text-sm font-medium text-violet-800">
              <BookOpen className="h-4 w-4" />
              漫画栏目
            </div>
            <p className="mt-1 text-sm text-surface-600">漫画工作台复用项目内的剧本分集来生成漫画分镜，不再把“视频分集”和“漫画分镜”混用成同一概念。</p>
            <p className="mt-2 text-xs text-violet-700">当前风格：{activeStyle.label} · {activeStyle.desc}</p>
          </div>
          <div className="flex flex-wrap gap-2">
            <Button size="sm" variant="outline" onClick={handleCreateMissingPanels} disabled={creatingPanels}>
              {creatingPanels ? <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" /> : <Plus className="mr-1.5 h-3.5 w-3.5" />}
              补齐漫画分镜
            </Button>
            {episodes.length === 0 && (
              <Button size="sm" variant="secondary" onClick={openEpisodeGenerator}>
                去生成剧本分集
              </Button>
            )}
            <Button size="sm" onClick={handleGenerateComics} disabled={generatingPanels || episodes.length === 0}>
              {generatingPanels ? <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" /> : <Sparkles className="mr-1.5 h-3.5 w-3.5" />}
              一键生成漫画
            </Button>
            {comicStats.completed > 0 && (
              <a
                href={storyboardAPI.exportURL(projectId)}
                download={`storyboards_${projectId}.zip`}
                className="inline-flex items-center gap-1.5 rounded-md border border-surface-200 bg-white px-3 py-1.5 text-sm font-medium text-surface-700 shadow-sm hover:bg-surface-50"
              >
                <Download className="h-3.5 w-3.5" />
                导出全部图片
              </a>
            )}
          </div>
        </div>

        <div className="mt-4 grid gap-3 md:grid-cols-4">
          <div className="rounded-lg border border-white/70 bg-white/80 p-3"><p className="text-xs text-surface-400">漫画画格</p><p className="mt-1 text-lg font-semibold text-surface-800">{comicStats.total}</p></div>
          <div className="rounded-lg border border-white/70 bg-white/80 p-3"><p className="text-xs text-surface-400">已生成</p><p className="mt-1 text-lg font-semibold text-emerald-700">{comicStats.completed}</p></div>
          <div className="rounded-lg border border-white/70 bg-white/80 p-3"><p className="text-xs text-surface-400">生成中</p><p className="mt-1 text-lg font-semibold text-blue-700">{comicStats.generating}</p></div>
          <div className="rounded-lg border border-white/70 bg-white/80 p-3"><p className="text-xs text-surface-400">可阅读页</p><p className="mt-1 text-lg font-semibold text-violet-700">{Math.ceil(completedPanels.length / 4) || 0}</p></div>
        </div>
      </div>

      <div className="flex flex-wrap items-center gap-3">
        <Select value={episodeFilter} onValueChange={setEpisodeFilter}>
          <SelectTrigger className="w-40"><SelectValue placeholder="选择剧本分集" /></SelectTrigger>
          <SelectContent>
            <SelectItem value="all">全部剧本分集</SelectItem>
            {episodes.map((episode) => <SelectItem key={episode.id} value={String(episode.id)}>剧本第 {episode.episode_number} 集</SelectItem>)}
          </SelectContent>
        </Select>

        <Select value={comicStyle} onValueChange={(value) => setComicStyle(value as ComicStylePresetKey)}>
          <SelectTrigger className="w-44"><SelectValue placeholder="漫画风格" /></SelectTrigger>
          <SelectContent>{COMIC_STYLE_PRESETS.map((preset) => <SelectItem key={preset.key} value={preset.key}>{preset.label}</SelectItem>)}</SelectContent>
        </Select>

        <Select value={panelDensity} onValueChange={setPanelDensity}>
          <SelectTrigger className="w-36"><SelectValue placeholder="每集格数" /></SelectTrigger>
          <SelectContent>
            <SelectItem value="3">每集 3 格</SelectItem>
            <SelectItem value="4">每集 4 格</SelectItem>
            <SelectItem value="6">每集 6 格</SelectItem>
          </SelectContent>
        </Select>

        <Select value={readingMode} onValueChange={(value) => setReadingMode(value as 'grid' | 'strip')}>
          <SelectTrigger className="w-36"><SelectValue placeholder="阅读模式" /></SelectTrigger>
          <SelectContent>
            <SelectItem value="grid">网格阅读</SelectItem>
            <SelectItem value="strip">条漫阅读</SelectItem>
          </SelectContent>
        </Select>
      </div>

      {filteredEpisodes.length === 0 ? (
        <div className="rounded-lg border border-dashed border-surface-200 bg-surface-50 py-14 text-center">
          <BookOpen className="mx-auto mb-3 h-10 w-10 text-surface-300" />
          <p className="text-sm text-surface-500">请先在项目内完成剧本分集，再按这些分集生成漫画分镜。</p>
          <div className="mt-4">
            <Button onClick={openEpisodeGenerator}>去生成剧本分集</Button>
          </div>
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
                    <p className="mt-1 max-w-3xl text-xs leading-5 text-surface-500">{episode.summary || episode.script_excerpt || '当前剧本暂无摘要，生成时会使用标题补齐描述。'}</p>
                  </div>
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="rounded bg-violet-100 px-2 py-1 text-[11px] text-violet-700">{doneCount}/{panels.length || densityValue} 已完成</span>
                    {failedCount > 0 && <span className="rounded bg-red-100 px-2 py-1 text-[11px] text-red-700">失败 {failedCount}</span>}
                    <Button size="sm" variant="outline" onClick={() => handleGenerateEpisode(episode)} disabled={panels.some((panel) => panel.status === 'generating')}>
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
                        <button type="button" className="block w-full text-left" onClick={() => setSelectedComic(panel)}>
                          <div className="aspect-[3/4] overflow-hidden bg-surface-100">
                            {panel.image_url ? <img src={panel.image_url} alt={`第${episode.episode_number}集第${panel.sequence_number}格`} className="h-full w-full object-cover" /> : <div className="flex h-full items-center justify-center text-surface-300"><BookOpen className="h-10 w-10" /></div>}
                          </div>
                        </button>
                        <CardContent className="space-y-3 p-3">
                          <div className="flex items-center justify-between">
                            <div className="flex items-center gap-2"><span className="text-sm font-semibold text-surface-800">第 {panel.sequence_number} 格</span><StatusBadge status={panel.status} /></div>
                            {(panel.status === 'pending' || panel.status === 'failed') ? <Button size="sm" variant="ghost" className="h-7 px-2 text-xs" onClick={() => triggerPanelTask(panel)}>{panel.status === 'failed' ? <RefreshCw className="mr-1 h-3 w-3" /> : <Sparkles className="mr-1 h-3 w-3" />}{panel.status === 'failed' ? '重试' : '生成'}</Button> : null}
                          </div>
                          <p className="line-clamp-2 text-xs leading-5 text-surface-600">{panel.scene_description}</p>
                          {panel.dialogue ? <div className="rounded-2xl border border-violet-100 bg-violet-50 px-3 py-2 text-xs leading-5 text-violet-800">“{panel.dialogue}”</div> : null}
                          {panel.characters?.length > 0 ? <div className="flex flex-wrap gap-1">{panel.characters.map((character) => <Badge key={character} variant="outline" className="text-[10px] font-normal">{character}</Badge>)}</div> : null}
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
          {selectedComic ? (
            <>
              <DialogHeader><DialogTitle>漫画画格 · 第 {episodes.find((episode) => episode.id === selectedComic.episode_id)?.episode_number ?? '?'} 集 / 第 {selectedComic.sequence_number} 格</DialogTitle></DialogHeader>
              <div className="grid gap-4 lg:grid-cols-[minmax(0,1.15fr)_minmax(320px,0.85fr)]">
                <div className="overflow-hidden rounded-xl border border-surface-200 bg-surface-50">{selectedComic.image_url ? <img src={selectedComic.image_url} alt="漫画画格预览" className="h-full w-full object-contain" /> : <div className="flex min-h-[420px] items-center justify-center text-surface-300"><BookOpen className="h-12 w-12" /></div>}</div>
                <div className="space-y-4">
                  <div className="rounded-lg border border-surface-200 bg-surface-50 p-4"><p className="text-xs text-surface-400">画面描述</p><p className="mt-2 text-sm leading-6 text-surface-700">{selectedComic.scene_description}</p></div>
                  <div className="rounded-lg border border-violet-100 bg-violet-50 p-4"><p className="text-xs text-violet-500">气泡文案</p><p className="mt-2 text-sm leading-6 text-violet-900">{selectedComic.dialogue || '当前画格未填写对白'}</p></div>
                  <div className="grid gap-3 sm:grid-cols-2">
                    <div className="rounded-lg border border-surface-200 p-3"><p className="text-xs text-surface-400">状态</p><div className="mt-2"><StatusBadge status={selectedComic.status} /></div></div>
                    <div className="rounded-lg border border-surface-200 p-3"><p className="text-xs text-surface-400">时长参考</p><p className="mt-2 text-sm font-medium text-surface-800">{selectedComic.duration || 4} 秒</p></div>
                  </div>
                  {selectedComic.image_url ? <a href={selectedComic.image_url} target="_blank" rel="noopener noreferrer" className="inline-flex items-center gap-1 text-sm text-violet-600 hover:underline"><Download className="h-4 w-4" /> 下载当前漫画画格</a> : null}
                </div>
              </div>
            </>
          ) : null}
        </DialogContent>
      </Dialog>
    </div>
  )
}
