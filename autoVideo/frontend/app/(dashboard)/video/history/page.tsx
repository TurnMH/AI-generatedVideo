'use client'

import { useEffect, useMemo, useState } from 'react'
import Link from 'next/link'
import useSWR from 'swr'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { videoAPI } from '@/lib/api'
import { useToast } from '@/components/ui/toast'

type ManualMenuKey = 'text' | 'image' | 'reference' | 'start-end' | 'face-swap'

type SubmitSummary = {
  projectId: number
  taskId: number
  mode: ManualMenuKey
  modelName: string
  generateMode: string
  sourceCount: number
  referenceCount: number
  hasStartImage: boolean
  hasTailImage: boolean
  routeNote: string
  createdAt: string
}

type BackendTask = {
  id: number
  project_id: number
  model_name?: string
  status?: string
  created_at?: string
  requested_model?: string
  routed_generator?: string
  runtime_provider?: string
  effective_model?: string
  result_url?: string
  render_config?: Record<string, unknown>
}

const HISTORY_KEY = 'manual-video-history-v1'

function modeLabel(mode: ManualMenuKey) {
  switch (mode) {
    case 'text': return '文生视频'
    case 'image': return '图生视频'
    case 'reference': return '融合生视频'
    case 'start-end': return '首尾针视频'
    case 'face-swap': return '人物一致性参考'
  }
}

function inferModeFromSummary(item: SubmitSummary | null | undefined): string {
  if (!item) return '未知'
  return modeLabel(item.mode)
}

function inferModeFromTask(task: BackendTask): string {
  const cfg = task.render_config || {}
  const generateMode = String(cfg.generate_mode || '')
  if (cfg.reference_mode === 'identity-consistency') return '人物一致性参考'
  if (generateMode === 'text2video') return '文生视频'
  if (generateMode === 'reference2video') return '融合生视频'
  if (generateMode === 'startEnd2video') return '首尾针视频'
  return '图生视频'
}

export default function ManualVideoHistoryPage() {
  const { toast } = useToast()
  const [localItems, setLocalItems] = useState<SubmitSummary[]>([])
  const [query, setQuery] = useState('')
  const [statusFilter, setStatusFilter] = useState('all')
  const [busyAction, setBusyAction] = useState('')
  const { data, isLoading, mutate } = useSWR('manual-video-history-backend', () => videoAPI.listAllTasks({ page: 1, page_size: 100 }), {
    refreshInterval: (latest) => {
      const items = ((latest as { data?: { items?: BackendTask[] } } | undefined)?.data?.items || []) as BackendTask[]
      return items.some((task) => task.status === 'pending' || task.status === 'processing') ? 5000 : 0
    },
    revalidateOnFocus: true,
  })

  useEffect(() => {
    try {
      const raw = window.localStorage.getItem(HISTORY_KEY)
      const parsed = raw ? JSON.parse(raw) : []
      setLocalItems(Array.isArray(parsed) ? parsed : [])
    } catch {
      setLocalItems([])
    }
  }, [])

  const localMap = useMemo(() => new Map(localItems.map((item) => [item.taskId, item])), [localItems])
  const backendItems: BackendTask[] = useMemo(() => {
    const payload = data as { data?: { items?: BackendTask[] } } | undefined
    return payload?.data?.items || []
  }, [data])

  const filteredItems = useMemo(() => {
    const q = query.trim().toLowerCase()
    return backendItems.filter((task) => {
      const local = localMap.get(task.id)
      const mode = (local ? inferModeFromSummary(local) : inferModeFromTask(task)).toLowerCase()
      const statusOk = statusFilter === 'all' || (task.status || '') === statusFilter
      const haystack = [
        String(task.id),
        String(task.project_id),
        task.status || '',
        task.effective_model || task.model_name || local?.modelName || '',
        mode,
      ].join(' ').toLowerCase()
      return statusOk && (!q || haystack.includes(q))
    })
  }, [backendItems, localMap, query, statusFilter])

  const runAction = async (name: string, fn: () => Promise<unknown>, successText: string) => {
    try {
      setBusyAction(name)
      await fn()
      toast({ title: successText, variant: 'success' })
      await mutate()
    } catch (error) {
      toast({ title: error instanceof Error ? error.message : `${name} 失败`, variant: 'destructive' })
    } finally {
      setBusyAction('')
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold text-white">手动视频历史记录</h1>
          <p className="mt-2 text-sm text-slate-300">独立于 projects 页的手动视频任务中心，优先展示后端任务真相。</p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" onClick={() => mutate()}>立即刷新</Button>
          <Button variant="outline" asChild><Link href="/video">返回手动创建页</Link></Button>
        </div>
      </div>

      <Card className="border-white/10 bg-slate-900/60 text-slate-100">
        <CardHeader>
          <CardTitle>后端任务历史</CardTitle>
          <CardDescription className="text-slate-400">支持 task_id / project_id / model / mode 搜索，以及状态筛选；pending/processing 自动刷新</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="grid gap-3 md:grid-cols-[minmax(0,1fr),220px]">
            <Input value={query} onChange={(e) => setQuery(e.target.value)} placeholder="搜 task_id / project_id / model / mode" />
            <select value={statusFilter} onChange={(e) => setStatusFilter(e.target.value)} className="h-10 rounded-md border border-white/10 bg-slate-950 px-3 text-sm text-slate-100">
              <option value="all">全部状态</option>
              <option value="pending">pending</option>
              <option value="processing">processing</option>
              <option value="succeeded">succeeded</option>
              <option value="failed">failed</option>
              <option value="paused">paused</option>
              <option value="cancelled">cancelled</option>
            </select>
          </div>

          {isLoading ? (
            <div className="rounded-xl border border-white/10 bg-white/[0.03] p-4 text-sm text-slate-300">正在加载任务历史…</div>
          ) : filteredItems.length === 0 ? (
            <div className="rounded-xl border border-white/10 bg-white/[0.03] p-4 text-sm text-slate-300">没有匹配的任务。</div>
          ) : filteredItems.map((task) => {
            const local = localMap.get(task.id)
            return (
              <div key={task.id} className="rounded-xl border border-white/10 bg-white/[0.03] p-4">
                <div className="flex flex-wrap items-center justify-between gap-3">
                  <div>
                    <div className="font-medium text-white">{local ? inferModeFromSummary(local) : inferModeFromTask(task)} · task #{task.id}</div>
                    <div className="mt-1 text-xs text-slate-400">
                      {task.created_at || local?.createdAt || '-'} · status={task.status || '-'} · project={task.project_id} · model={task.effective_model || task.model_name || local?.modelName || '-'}
                    </div>
                  </div>
                  <div className="flex flex-wrap gap-2">
                    <Button variant="outline" asChild>
                      <Link href={`/video/history/${task.id}`}>详情</Link>
                    </Button>
                    {task.project_id > 0 && (
                      <Button variant="outline" disabled={busyAction !== ''} onClick={() => runAction(`retry-${task.id}`, () => videoAPI.retryVideoTask(task.project_id, task.id, task.requested_model || task.model_name), '已触发重试')}>
                        {busyAction === `retry-${task.id}` ? '处理中…' : '重试'}
                      </Button>
                    )}
                    <Button variant="outline" disabled={busyAction !== ''} onClick={() => runAction(`cancel-${task.id}`, () => videoAPI.cancelVideoTask(task.id), '已取消/删除任务')}>
                      {busyAction === `cancel-${task.id}` ? '处理中…' : '取消'}
                    </Button>
                    {task.result_url && (
                      <Button variant="outline" asChild>
                        <a href={task.result_url} target="_blank" rel="noreferrer">结果</a>
                      </Button>
                    )}
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
