'use client'

import { useEffect, useMemo, useState } from 'react'
import Link from 'next/link'
import { useParams } from 'next/navigation'
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

export default function ManualVideoHistoryDetailPage() {
  const params = useParams<{ taskId: string }>()
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

  const taskId = Number(params?.taskId || 0)
  const item = useMemo(() => items.find((entry) => entry.taskId === taskId) || null, [items, taskId])

  return (
    <div className="space-y-6">
      <div className="flex items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold text-white">手动视频任务详情</h1>
          <p className="mt-2 text-sm text-slate-300">独立于 projects 页的任务详情入口。</p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" asChild><Link href="/video/history">返回历史记录</Link></Button>
          <Button variant="outline" asChild><Link href="/video">返回创建页</Link></Button>
        </div>
      </div>

      {!item ? (
        <Card className="border-white/10 bg-slate-900/60 text-slate-100">
          <CardContent className="p-6 text-sm text-slate-300">未在本机历史中找到 task #{taskId}。</CardContent>
        </Card>
      ) : (
        <Card className="border-white/10 bg-slate-900/60 text-slate-100">
          <CardHeader>
            <CardTitle>{modeLabel(item.mode)} · task #{item.taskId}</CardTitle>
            <CardDescription className="text-slate-400">{item.createdAt}</CardDescription>
          </CardHeader>
          <CardContent className="space-y-3 text-sm text-slate-200">
            <div>project_id：{item.projectId}</div>
            <div>model：{item.modelName}</div>
            <div>generate_mode：{item.generateMode}</div>
            <div>首帧输入：{item.hasStartImage ? '有' : '无'}</div>
            <div>尾帧输入：{item.hasTailImage ? '有' : '无'}</div>
            <div>首帧数量：{item.sourceCount}</div>
            <div>参考图数量：{item.referenceCount}</div>
            <div className="rounded-lg border border-white/10 bg-white/[0.03] p-3 text-slate-300">{item.routeNote}</div>
          </CardContent>
        </Card>
      )}
    </div>
  )
}
