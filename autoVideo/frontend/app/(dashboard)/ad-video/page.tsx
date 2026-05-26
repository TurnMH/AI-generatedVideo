'use client'

import { useMemo, useState } from 'react'
import Link from 'next/link'
import useSWR from 'swr'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { useToast } from '@/components/ui/toast'
import { videoAPI, type VideoTaskDetailResponse } from '@/lib/api'

const DEFAULT_IDS = '150,151,152,153,159,160,161,162,163'

type Task = {
  id: number
  project_id: number
  status?: string
  result_url?: string
  subtitle_text?: string
  error_msg?: string
  render_config?: Record<string, unknown>
}

type ComposeAdResponse = {
  code?: number
  data?: {
    task_ids?: number[]
    task_id?: number
    result_url?: string
    task?: Task
    meta?: Record<string, unknown>
  }
}

const parseIds = (raw: string) =>
  raw
    .split(/[\s,]+/)
    .map((v) => Number(v.trim()))
    .filter((v) => Number.isFinite(v) && v > 0)

const getResultUrl = (task?: Task) => {
  if (!task) return ''
  const rc = task.render_config || {}
  const active = String(rc.active_result_variant || '').trim()
  const original = String(rc.original_result_url || '').trim()
  const subtitled = String(rc.subtitled_result_url || '').trim()
  if (active === 'subtitled' && subtitled) return subtitled
  if (active === 'original' && original) return original
  return String(task.result_url || subtitled || original || '').trim()
}

export default function AdVideoWorkbenchPage() {
  const { toast } = useToast()
  const [idsText, setIdsText] = useState(DEFAULT_IDS)
  const [orderedIds, setOrderedIds] = useState<number[]>(parseIds(DEFAULT_IDS))
  const [busy, setBusy] = useState(false)
  const [busyTaskId, setBusyTaskId] = useState<number | null>(null)
  const [resultUrl, setResultUrl] = useState('')
  const [resultTaskId, setResultTaskId] = useState<number | null>(null)
  const idsKey = useMemo(() => orderedIds.join(','), [orderedIds])

  const { data, mutate, isLoading } = useSWR(
    idsKey ? ['ad-video-tasks', idsKey] : null,
    async () => {
      const results = await Promise.all(
        orderedIds.map(async (id) => {
          const res = await videoAPI.getTask<Task>(id)
          const payload = res.data as VideoTaskDetailResponse<Task>
          return payload?.data?.task as Task
        }),
      )
      return results.filter(Boolean)
    },
    {
      refreshInterval: (latest) =>
        Array.isArray(latest) && latest.some((task) => task?.status === 'pending' || task?.status === 'processing') ? 5000 : 0,
      revalidateOnFocus: true,
    },
  )

  const tasks = data || []

  const loadIds = () => {
    const next = parseIds(idsText)
    setOrderedIds(next)
    setResultUrl('')
    setResultTaskId(null)
  }

  const move = (idx: number, delta: number) => {
    const next = [...orderedIds]
    const target = idx + delta
    if (target < 0 || target >= next.length) return
    ;[next[idx], next[target]] = [next[target], next[idx]]
    setOrderedIds(next)
    setIdsText(next.join(','))
    setResultUrl('')
    setResultTaskId(null)
  }

  const compose = async () => {
    if (orderedIds.length === 0) {
      toast({ title: '请先输入 task id', variant: 'destructive' })
      return
    }
    setBusy(true)
    try {
      const res = await videoAPI.composeAdVideo(orderedIds)
      const payload = res.data as ComposeAdResponse
      const url = String(payload?.data?.result_url || '').trim()
      const taskId = Number(payload?.data?.task_id || 0)
      if (!url) {
        throw new Error('广告合成接口已返回，但 result_url 为空')
      }
      setResultUrl(url)
      setResultTaskId(Number.isFinite(taskId) && taskId > 0 ? taskId : null)
      toast({ title: taskId > 0 ? `广告合成完成，已落为 task #${taskId}` : '广告合成完成', variant: 'success' })
      await mutate()
    } catch (error) {
      toast({ title: error instanceof Error ? error.message : '广告合成失败', variant: 'destructive' })
    } finally {
      setBusy(false)
    }
  }

  const composeSubtitle = async (taskId: number) => {
    setBusyTaskId(taskId)
    try {
      await videoAPI.compose(taskId)
      toast({ title: `已触发 task #${taskId} 添加字幕`, variant: 'success' })
      await mutate()
    } catch (error) {
      toast({ title: error instanceof Error ? error.message : `task #${taskId} 添加字幕失败`, variant: 'destructive' })
    } finally {
      setBusyTaskId(null)
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold text-white">口播广告工作台</h1>
          <p className="mt-2 text-sm text-slate-300">把现有 video task 按顺序合成一个广告成片，并支持单条继续处理。</p>
        </div>
        <div className="flex gap-2">
          <Button asChild variant="outline">
            <Link href="/video/history">返回历史</Link>
          </Button>
          <Button asChild variant="outline">
            <Link href="/video">返回手动创建</Link>
          </Button>
        </div>
      </div>

      <Card className="border-white/10 bg-slate-900/60 text-slate-100">
        <CardHeader>
          <CardTitle>任务 ID 与整片合成</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          <Input value={idsText} onChange={(e) => setIdsText(e.target.value)} placeholder="150,151,152..." />
          <div className="flex flex-wrap gap-2">
            <Button onClick={loadIds}>加载</Button>
            <Button variant="outline" disabled={busy || orderedIds.length === 0} onClick={compose}>
              {busy ? '合成中…' : '合成一个广告视频'}
            </Button>
            <Button
              variant="outline"
              onClick={() => {
                setIdsText(DEFAULT_IDS)
                setOrderedIds(parseIds(DEFAULT_IDS))
                setResultUrl('')
                setResultTaskId(null)
              }}
            >
              恢复默认
            </Button>
          </div>
          <div className="text-xs text-slate-400">当前顺序：{orderedIds.length > 0 ? orderedIds.join(' → ') : '未选择任务'}</div>
        </CardContent>
      </Card>

      <div className="space-y-3">
        {isLoading ? (
          <div className="text-slate-300">加载中…</div>
        ) : tasks.length === 0 ? (
          <div className="rounded-xl border border-white/10 bg-slate-900/60 p-4 text-sm text-slate-300">当前没有可展示的任务。</div>
        ) : (
          tasks.map((task, idx) => {
            const url = getResultUrl(task)
            const subtitle = String(task.subtitle_text || task.render_config?.subtitle_text || '').trim()
            const subtitleStatus = String(task.render_config?.subtitle_compose_status || '').trim()
            return (
              <Card key={task.id} className="border-white/10 bg-slate-900/60 text-slate-100">
                <CardContent className="flex flex-wrap items-center justify-between gap-3 p-4">
                  <div className="min-w-0 flex-1">
                    <div className="font-medium">task #{task.id} · {task.status || '-'}</div>
                    <div className="mt-1 text-xs text-slate-400">project={task.project_id} · subtitle={subtitleStatus || '-'}</div>
                    <div className="mt-1 break-all text-xs text-slate-400">{url || '无结果 URL'}</div>
                    {subtitle && <div className="mt-2 line-clamp-2 text-xs text-violet-200">{subtitle}</div>}
                    {task.error_msg && <div className="mt-2 text-xs text-rose-300">{task.error_msg}</div>}
                  </div>
                  <div className="flex flex-wrap gap-2">
                    <Button size="sm" variant="outline" onClick={() => move(idx, -1)}>
                      上移
                    </Button>
                    <Button size="sm" variant="outline" onClick={() => move(idx, 1)}>
                      下移
                    </Button>
                    <Button size="sm" variant="outline" disabled={busyTaskId !== null} onClick={() => composeSubtitle(task.id)}>
                      {busyTaskId === task.id ? '处理中…' : '添加字幕'}
                    </Button>
                    <Button size="sm" variant="outline" asChild>
                      <Link href={`/video/history/${task.id}`}>详情</Link>
                    </Button>
                    {url ? (
                      <Button size="sm" variant="outline" asChild>
                        <a href={url} target="_blank" rel="noreferrer">打开结果</a>
                      </Button>
                    ) : (
                      <Button size="sm" variant="outline" disabled>
                        打开结果
                      </Button>
                    )}
                  </div>
                </CardContent>
              </Card>
            )
          })
        )}
      </div>

      {resultUrl && (
        <Card className="border-white/10 bg-slate-900/60 text-slate-100">
          <CardHeader>
            <CardTitle>合成结果</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="break-all text-sm text-slate-300">{resultUrl}</div>
            <div className="flex flex-wrap gap-2">
              <Button variant="outline" asChild>
                <a href={resultUrl} target="_blank" rel="noreferrer">打开合成结果</a>
              </Button>
              {resultTaskId && (
                <Button variant="outline" asChild>
                  <Link href={`/video/history/${resultTaskId}`}>查看合成任务详情</Link>
                </Button>
              )}
            </div>
            <video className="w-full rounded-lg bg-black" controls src={resultUrl} />
          </CardContent>
        </Card>
      )}
    </div>
  )
}
