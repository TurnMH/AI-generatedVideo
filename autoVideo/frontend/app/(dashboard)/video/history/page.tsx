'use client'

import { useEffect, useMemo, useState } from 'react'
import Link from 'next/link'
import useSWR from 'swr'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { videoAPI } from '@/lib/api'

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
  render_config?: Record<string, unknown>
  image_urls?: string[]
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
  const [localItems, setLocalItems] = useState<SubmitSummary[]>([])
  const { data, isLoading } = useSWR('manual-video-history-backend', () => videoAPI.listAllTasks({ page: 1, page_size: 50 }))

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

  return (
    <div className="space-y-6">
      <div className="flex items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold text-white">手动视频历史记录</h1>
          <p className="mt-2 text-sm text-slate-300">独立于 projects 页的手动视频任务中心，优先展示后端任务真相。</p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" asChild><Link href="/video">返回手动创建页</Link></Button>
        </div>
      </div>

      <Card className="border-white/10 bg-slate-900/60 text-slate-100">
        <CardHeader>
          <CardTitle>后端任务历史</CardTitle>
          <CardDescription className="text-slate-400">/api/v1/videos/tasks + 本地提交摘要补充</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          {isLoading ? (
            <div className="rounded-xl border border-white/10 bg-white/[0.03] p-4 text-sm text-slate-300">正在加载任务历史…</div>
          ) : backendItems.length === 0 ? (
            <div className="rounded-xl border border-white/10 bg-white/[0.03] p-4 text-sm text-slate-300">后端暂无任务记录。</div>
          ) : backendItems.map((task) => {
            const local = localMap.get(task.id)
            return (
              <div key={task.id} className="rounded-xl border border-white/10 bg-white/[0.03] p-4">
                <div className="flex flex-wrap items-center justify-between gap-3">
                  <div>
                    <div className="font-medium text-white">{local ? inferModeFromSummary(local) : inferModeFromTask(task)} · task #{task.id}</div>
                    <div className="mt-1 text-xs text-slate-400">
                      {task.created_at || local?.createdAt || '-'} · status={task.status || '-'} · model={task.effective_model || task.model_name || local?.modelName || '-'}
                    </div>
                  </div>
                  <Button variant="outline" asChild>
                    <Link href={`/video/history/${task.id}`}>查看详情</Link>
                  </Button>
                </div>
              </div>
            )
          })}
        </CardContent>
      </Card>
    </div>
  )
}
