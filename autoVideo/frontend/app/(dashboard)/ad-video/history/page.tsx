'use client'

import Link from 'next/link'
import { useMemo, useState } from 'react'
import useSWR from 'swr'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { projectAPI, videoAPI, type Project } from '@/lib/api'

type ProjectListResponse = {
  data?: { items?: Project[] } | Project[]
}

type VideoTask = {
  id: number
  project_id: number
  status?: string
  result_url?: string
  created_at?: string
  updated_at?: string
  error_msg?: string
  render_config?: Record<string, unknown>
}

function unwrapProjects(payload: unknown): Project[] {
  if (!payload || typeof payload !== 'object') return []
  const root = payload as ProjectListResponse
  if (Array.isArray(root.data)) return root.data
  if (root.data && typeof root.data === 'object' && Array.isArray((root.data as { items?: Project[] }).items)) {
    return (root.data as { items?: Project[] }).items || []
  }
  return []
}

function taskResultUrl(task?: VideoTask | null) {
  if (!task) return ''
  const rc = task.render_config || {}
  return String(task.result_url || rc.subtitled_result_url || rc.original_result_url || '').trim()
}

export default function AdVideoHistoryPage() {
  const [query, setQuery] = useState('')
  const { data: projectData, mutate: mutateProjects, isLoading } = useSWR('ad-video-history-projects', async () => {
    const res = await projectAPI.list({ project_type: 'video', page: 1, page_size: 100 })
    const projects = unwrapProjects(res)
    return projects.filter((project) => Array.isArray(project.style_tags) && project.style_tags.includes('ad-workbench'))
  }, { revalidateOnFocus: true })

  const { data: taskData, mutate: mutateTasks } = useSWR('ad-video-history-tasks', async () => {
    const res = await videoAPI.listAllTasks({ page: 1, page_size: 200 })
    const payload = res as { data?: { items?: VideoTask[] } }
    return payload?.data?.items || []
  }, {
    refreshInterval: (latest) => Array.isArray(latest) && latest.some((task) => task.status === 'pending' || task.status === 'processing') ? 5000 : 0,
    revalidateOnFocus: true,
  })

  const projects = useMemo(() => projectData || [], [projectData])
  const tasks = useMemo(() => taskData || [], [taskData])
  const latestTaskMap = useMemo(() => {
    const map = new Map<number, VideoTask>()
    for (const task of tasks) {
      const prev = map.get(task.project_id)
      if (!prev || Number(task.id) > Number(prev.id)) {
        map.set(task.project_id, task)
      }
    }
    return map
  }, [tasks])

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase()
    return projects.filter((project) => {
      const task = latestTaskMap.get(project.id)
      const haystack = [
        String(project.id),
        project.title || '',
        project.description || '',
        project.status || '',
        task?.status || '',
        task?.error_msg || '',
        project.progress?.message || '',
      ].join(' ').toLowerCase()
      return !q || haystack.includes(q)
    })
  }, [projects, latestTaskMap, query])

  return (
    <div className="space-y-6">
      <div className="flex items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold text-white">广告创建历史</h1>
          <p className="mt-2 text-sm text-slate-300">这里只展示广告工作台创建的项目与成片历史，不和其他项目混合。</p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" onClick={() => { void mutateProjects(); void mutateTasks() }}>立即刷新</Button>
          <Button variant="outline" asChild><Link href="/ad-video">返回广告工作台</Link></Button>
        </div>
      </div>

      <Card className="border-white/10 bg-slate-900/60 text-slate-100">
        <CardHeader>
          <CardTitle>广告项目历史列表</CardTitle>
          <CardDescription className="text-slate-400">按 style_tags=ad-workbench 过滤；已完成项目可直接看到完整视频入口。</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <Input value={query} onChange={(e) => setQuery(e.target.value)} placeholder="搜项目标题 / project id / 状态 / 错误" />
          {isLoading ? (
            <div className="rounded-xl border border-white/10 bg-white/[0.03] p-4 text-sm text-slate-300">正在加载广告创建历史…</div>
          ) : filtered.length === 0 ? (
            <div className="rounded-xl border border-white/10 bg-white/[0.03] p-4 text-sm text-slate-300">暂无广告创建历史。</div>
          ) : filtered.map((project) => {
            const task = latestTaskMap.get(project.id)
            const resultUrl = taskResultUrl(task)
            const progress = project.progress || {}
            return (
              <div key={project.id} className="rounded-xl border border-white/10 bg-white/[0.03] p-4">
                <div className="flex flex-wrap items-start justify-between gap-3">
                  <div className="min-w-0 flex-1 space-y-2">
                    <div className="text-base font-medium text-white">#{project.id} · {project.title}</div>
                    <div className="text-xs text-slate-400">项目状态：{project.status || '-'} · 当前阶段：{progress.phase_label || progress.stage || progress.message || '暂无'}</div>
                    <div className="text-xs text-slate-400 line-clamp-2">{project.description || '暂无说明'}</div>
                    {progress.auto_split?.optimized_script && (
                      <div className="rounded-lg border border-emerald-500/20 bg-emerald-500/10 px-3 py-2 text-xs text-emerald-100">
                        已完成文案优化，历史详情页可查看优化全文 / 场景描述 / 台词 / 一致性前提。
                      </div>
                    )}
                    {task?.error_msg && (
                      <div className="rounded-lg border border-rose-500/20 bg-rose-500/10 px-3 py-2 text-xs text-rose-200">
                        最新视频任务错误：{task.error_msg}
                      </div>
                    )}
                  </div>
                  <div className="flex flex-wrap gap-2">
                    <Button variant="outline" asChild><Link href={`/ad-video/history/${project.id}`}>查看详情 / 进度</Link></Button>
                    {resultUrl && <Button variant="outline" asChild><a href={resultUrl} target="_blank" rel="noreferrer">查看完整视频</a></Button>}
                  </div>
                </div>
              </div>
            )
          })}
        </CardContent>
      </Card>
    </div>
  )
}
