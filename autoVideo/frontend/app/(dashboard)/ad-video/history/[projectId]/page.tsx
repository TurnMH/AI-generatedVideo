'use client'

import Link from 'next/link'
import { useMemo } from 'react'
import { useParams } from 'next/navigation'
import useSWR from 'swr'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { projectAPI, storyboardAPI, videoAPI, type Episode, type Project } from '@/lib/api'
import type { Storyboard } from '@/types'

type VideoTask = {
  id: number
  project_id: number
  status?: string
  result_url?: string
  error_msg?: string
  created_at?: string
  updated_at?: string
  render_config?: Record<string, unknown>
}

function unwrap<T>(payload: unknown): T | null {
  if (!payload || typeof payload !== 'object') return null
  const maybe = payload as { data?: T }
  return maybe.data ?? (payload as T)
}

function taskResultUrl(task?: VideoTask | null) {
  if (!task) return ''
  const rc = task.render_config || {}
  return String(task.result_url || rc.subtitled_result_url || rc.original_result_url || '').trim()
}

export default function AdVideoHistoryDetailPage() {
  const params = useParams<{ projectId: string }>()
  const projectId = Number(params?.projectId || 0)

  const { data: projectData, mutate: mutateProject, isLoading } = useSWR(projectId ? ['ad-video-history-project', projectId] : null, async () => {
    const res = await projectAPI.get(projectId)
    return unwrap<Project>((res as { data?: unknown }).data)
  }, { revalidateOnFocus: true })

  const { data: episodesData, mutate: mutateEpisodes } = useSWR(projectId ? ['ad-video-history-episodes', projectId] : null, async () => {
    const res = await projectAPI.listEpisodes(projectId)
    return unwrap<Episode[]>((res as { data?: unknown }).data) || []
  }, { refreshInterval: 5000, revalidateOnFocus: true })

  const { data: storyboardsData, mutate: mutateStoryboards } = useSWR(projectId ? ['ad-video-history-storyboards', projectId] : null, async () => {
    const res = await storyboardAPI.listAll(projectId)
    const payload = (res as { data?: Storyboard[] }).data
    return Array.isArray(payload) ? payload : []
  }, { refreshInterval: 5000, revalidateOnFocus: true })

  const { data: taskData, mutate: mutateTasks } = useSWR(projectId ? ['ad-video-history-tasks', projectId] : null, async () => {
    const res = await videoAPI.listAllTasks({ project_id: projectId, page: 1, page_size: 200 })
    const payload = res as { data?: { items?: VideoTask[] } }
    return payload?.data?.items || []
  }, {
    refreshInterval: (latest) => Array.isArray(latest) && latest.some((task) => task.status === 'pending' || task.status === 'processing') ? 5000 : 0,
    revalidateOnFocus: true,
  })

  const project = projectData || null
  const episodes = (episodesData || []).slice().sort((a, b) => a.episode_number - b.episode_number)
  const storyboards = (storyboardsData || []).slice().sort((a, b) => a.sequence_number - b.sequence_number)
  const tasks = taskData || []
  const latestTask = useMemo(() => tasks.slice().sort((a, b) => Number(b.id) - Number(a.id))[0] || null, [tasks])
  const autoSplit = project?.progress?.auto_split || null
  const resultUrl = taskResultUrl(latestTask)

  return (
    <div className="space-y-6">
      <div className="flex items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold text-white">广告创建详情 / 进度</h1>
          <p className="mt-2 text-sm text-slate-300">从“文案优化”开始，把广告创建过程中的关键内容完整展开，并单独区分场景描述与台词。</p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" onClick={() => { void mutateProject(); void mutateEpisodes(); void mutateStoryboards(); void mutateTasks() }}>立即刷新</Button>
          <Button variant="outline" asChild><Link href="/ad-video/history">返回广告历史</Link></Button>
          <Button variant="outline" asChild><Link href="/ad-video">返回工作台</Link></Button>
        </div>
      </div>

      {isLoading || !project ? (
        <Card className="border-white/10 bg-slate-900/60 text-slate-100"><CardContent className="p-6 text-sm text-slate-300">正在加载广告创建详情…</CardContent></Card>
      ) : (
        <>
          <Card className="border-white/10 bg-slate-900/60 text-slate-100">
            <CardHeader>
              <CardTitle>项目总览</CardTitle>
              <CardDescription className="text-slate-400">project #{project.id} · {project.title}</CardDescription>
            </CardHeader>
            <CardContent className="space-y-3 text-sm text-slate-200">
              <div>状态：{project.status || '-'} · 当前阶段：{project.progress?.phase_label || project.progress?.stage || project.progress?.message || '暂无'}</div>
              <div>style_tags：{Array.isArray(project.style_tags) && project.style_tags.length > 0 ? project.style_tags.join(' / ') : '-'}</div>
              <div>文案长度：{autoSplit?.script_length || (project.script_text || '').length || '-'} · 预估分集：{autoSplit?.estimated_episodes || episodes.length || '-'}</div>
              {resultUrl && <div className="break-all">最新完整视频：<a className="text-cyan-300 underline" href={resultUrl} target="_blank" rel="noreferrer">{resultUrl}</a></div>}
              {latestTask?.error_msg && <div className="rounded-lg border border-rose-500/20 bg-rose-500/10 p-3 text-rose-200">最新视频任务错误：{latestTask.error_msg}</div>}
            </CardContent>
          </Card>

          <Card className="border-white/10 bg-slate-900/60 text-slate-100">
            <CardHeader>
              <CardTitle>文案优化与一致性前提</CardTitle>
              <CardDescription className="text-slate-400">这里展示从广告文案优化开始的真实运行态内容。</CardDescription>
            </CardHeader>
            <CardContent className="space-y-4 text-sm text-slate-200">
              <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                <div className="mb-2 text-xs font-medium text-slate-400">优化前全文</div>
                <div className="whitespace-pre-wrap break-words text-slate-100">{autoSplit?.original_script || project.script_text || '暂无'}</div>
              </div>
              <div className="rounded-lg border border-emerald-500/20 bg-emerald-500/10 p-3">
                <div className="mb-2 text-xs font-medium text-emerald-200">优化后全文</div>
                <div className="whitespace-pre-wrap break-words text-slate-100">{autoSplit?.optimized_script || '优化尚未完成'}</div>
              </div>
              <div className="rounded-lg border border-amber-500/20 bg-amber-500/10 p-3">
                <div className="mb-2 text-xs font-medium text-amber-200">一致性前提</div>
                <div className="whitespace-pre-wrap break-words text-slate-100">{autoSplit?.consistency_premise || '当前运行态尚未返回一致性前提。后续分镜/视频生成应继续补齐这一段。'}</div>
              </div>
            </CardContent>
          </Card>

          <Card className="border-white/10 bg-slate-900/60 text-slate-100">
            <CardHeader>
              <CardTitle>分集进度</CardTitle>
              <CardDescription className="text-slate-400">展示自动切分后的片段载体。</CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              {episodes.length === 0 ? (
                <div className="rounded-lg border border-white/10 bg-black/20 p-4 text-sm text-slate-300">当前还没有分集记录。</div>
              ) : episodes.map((episode) => (
                <div key={episode.id} className="rounded-lg border border-white/10 bg-black/20 p-4 text-sm text-slate-200">
                  <div>episode #{episode.episode_number} · {episode.title || '未命名片段'} · {episode.status}</div>
                  <div className="mt-2 whitespace-pre-wrap break-words text-slate-400">{episode.summary || episode.script_excerpt || '暂无摘要'}</div>
                </div>
              ))}
            </CardContent>
          </Card>

          <Card className="border-white/10 bg-slate-900/60 text-slate-100">
            <CardHeader>
              <CardTitle>场景描述 / 台词</CardTitle>
              <CardDescription className="text-slate-400">明确分开展示，避免后续生成链把视觉描述和口播混在一起。</CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              {storyboards.length === 0 ? (
                <div className="rounded-lg border border-white/10 bg-black/20 p-4 text-sm text-slate-300">当前还没有分镜记录。</div>
              ) : storyboards.map((storyboard) => (
                <div key={storyboard.id} className="rounded-xl border border-white/10 bg-black/20 p-4 space-y-3">
                  <div className="text-sm text-slate-100">分镜 #{storyboard.sequence_number} · {storyboard.status || '-'} · episode {storyboard.episode_id || '-'}</div>
                  <div className="grid gap-3 md:grid-cols-2">
                    <div className="rounded-lg border border-cyan-500/20 bg-cyan-500/10 p-3">
                      <div className="mb-2 text-xs font-medium text-cyan-200">场景描述</div>
                      <div className="whitespace-pre-wrap break-words text-sm text-slate-100">{storyboard.scene_description || '暂无场景描述'}</div>
                    </div>
                    <div className="rounded-lg border border-violet-500/20 bg-violet-500/10 p-3">
                      <div className="mb-2 text-xs font-medium text-violet-200">台词</div>
                      <div className="whitespace-pre-wrap break-words text-sm text-slate-100">{storyboard.dialogue || '暂无台词'}</div>
                    </div>
                  </div>
                  <div className="grid gap-2 text-xs text-slate-400 md:grid-cols-3">
                    <div>location：{storyboard.location || '-'}</div>
                    <div>camera_movement：{storyboard.camera_movement || '-'}</div>
                    <div>duration：{storyboard.duration || '-'} 秒</div>
                  </div>
                </div>
              ))}
            </CardContent>
          </Card>

          <Card className="border-white/10 bg-slate-900/60 text-slate-100">
            <CardHeader>
              <CardTitle>视频任务 / 完整视频</CardTitle>
              <CardDescription className="text-slate-400">广告历史详情内直接查看当前视频生成进度与完整视频结果。</CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              {tasks.length === 0 ? (
                <div className="rounded-lg border border-white/10 bg-black/20 p-4 text-sm text-slate-300">当前还没有视频任务记录。</div>
              ) : tasks.slice().sort((a, b) => Number(b.id) - Number(a.id)).map((task) => (
                <div key={task.id} className="rounded-lg border border-white/10 bg-black/20 p-4 text-sm text-slate-200">
                  <div>task #{task.id} · {task.status || '-'} · {task.created_at || '-'}</div>
                  {task.error_msg && <div className="mt-2 text-rose-300">错误：{task.error_msg}</div>}
                  {taskResultUrl(task) && <div className="mt-2 break-all"><a className="text-cyan-300 underline" href={taskResultUrl(task)} target="_blank" rel="noreferrer">打开结果视频</a></div>}
                </div>
              ))}
              {resultUrl && (
                <div className="overflow-hidden rounded-xl border border-white/10 bg-black/30 p-3">
                  <div className="mb-2 text-xs text-slate-400">最新完整视频预览</div>
                  <video className="max-h-[420px] w-full rounded-lg bg-black" controls preload="metadata" src={resultUrl} />
                </div>
              )}
            </CardContent>
          </Card>
        </>
      )}
    </div>
  )
}
