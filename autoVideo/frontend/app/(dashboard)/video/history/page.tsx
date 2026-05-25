'use client'

import { useEffect, useMemo, useState } from 'react'
import Link from 'next/link'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'

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

export default function ManualVideoHistoryPage() {
  const [items, setItems] = useState<SubmitSummary[]>([])

  useEffect(() => {
    try {
      const raw = window.localStorage.getItem(HISTORY_KEY)
      const parsed = raw ? JSON.parse(raw) : []
      setItems(Array.isArray(parsed) ? parsed : [])
    } catch {
      setItems([])
    }
  }, [])

  const sorted = useMemo(() => [...items].sort((a, b) => String(b.createdAt).localeCompare(String(a.createdAt))), [items])

  return (
    <div className="space-y-6">
      <div className="flex items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold text-white">手动视频历史记录</h1>
          <p className="mt-2 text-sm text-slate-300">独立于 projects 页的手动视频创建历史。</p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" asChild><Link href="/video">返回手动创建页</Link></Button>
        </div>
      </div>

      <Card className="border-white/10 bg-slate-900/60 text-slate-100">
        <CardHeader>
          <CardTitle>历史任务</CardTitle>
          <CardDescription className="text-slate-400">最近 50 条本机记录</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          {sorted.length === 0 ? (
            <div className="rounded-xl border border-white/10 bg-white/[0.03] p-4 text-sm text-slate-300">暂无历史记录。</div>
          ) : sorted.map((item) => (
            <div key={`${item.taskId}-${item.createdAt}`} className="rounded-xl border border-white/10 bg-white/[0.03] p-4">
              <div className="flex flex-wrap items-center justify-between gap-3">
                <div>
                  <div className="font-medium text-white">{modeLabel(item.mode)} · task #{item.taskId}</div>
                  <div className="mt-1 text-xs text-slate-400">{item.createdAt} · model={item.modelName} · mode={item.generateMode}</div>
                </div>
                <Button variant="outline" asChild>
                  <Link href={`/video/history/${item.taskId}`}>查看详情</Link>
                </Button>
              </div>
            </div>
          ))}
        </CardContent>
      </Card>
    </div>
  )
}
