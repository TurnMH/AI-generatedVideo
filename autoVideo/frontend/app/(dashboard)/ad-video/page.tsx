'use client'

import { useMemo, useState } from 'react'
import Link from 'next/link'
import useSWR from 'swr'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { videoAPI } from '@/lib/api'

const DEFAULT_IDS = '150,151,152,153,159,160,161,162,163'

type Task = { id: number; status?: string; result_url?: string; subtitle_text?: string; render_config?: Record<string, unknown> }
const getResultUrl = (task?: Task) => {
  if (!task) return ''
  const rc = task.render_config || {}
  const active = String(rc.active_result_variant || '')
  const original = String(rc.original_result_url || '').trim()
  const subtitled = String(rc.subtitled_result_url || '').trim()
  if (active === 'subtitled' && subtitled) return subtitled
  if (active === 'original' && original) return original
  return task.result_url || subtitled || original || ''
}
export default function AdVideoWorkbenchPage() {
  const [idsText, setIdsText] = useState(DEFAULT_IDS)
  const [orderedIds, setOrderedIds] = useState<number[]>(DEFAULT_IDS.split(',').map((v) => Number(v.trim())).filter(Boolean))
  const [busy, setBusy] = useState(false)
  const [resultUrl, setResultUrl] = useState('')
  const idsKey = useMemo(() => orderedIds.join(','), [orderedIds])
  const { data, mutate, isLoading } = useSWR(idsKey ? ['ad-video-tasks', idsKey] : null, async () => Promise.all(orderedIds.map(async (id) => (await videoAPI.getTask<Task>(id)).data.task as Task)))
  const tasks = data || []
  const loadIds = () => { const next = idsText.split(/[\s,]+/).map((v) => Number(v)).filter((v) => Number.isFinite(v) && v > 0); setOrderedIds(next); setResultUrl('') }
  const move = (idx: number, delta: number) => { const next = [...orderedIds]; const target = idx + delta; if (target < 0 || target >= next.length) return; [next[idx], next[target]] = [next[target], next[idx]]; setOrderedIds(next); setIdsText(next.join(',')); setResultUrl('') }
  const compose = async () => { setBusy(true); try { const res = await videoAPI.composeAdVideo(orderedIds); setResultUrl(res.data.result_url); await mutate() } finally { setBusy(false) } }
  return <div className="space-y-6"><div className="flex items-center justify-between gap-4"><div><h1 className="text-2xl font-semibold text-white">口播广告工作台</h1><p className="mt-2 text-sm text-slate-300">把现有 video task 按顺序合成一个广告成片。</p></div><Button asChild variant="outline"><Link href="/video/history">返回历史</Link></Button></div><Card className="border-white/10 bg-slate-900/60 text-slate-100"><CardHeader><CardTitle>任务 ID</CardTitle></CardHeader><CardContent className="space-y-3"><Input value={idsText} onChange={(e) => setIdsText(e.target.value)} placeholder="150,151,152..." /><div className="flex gap-2"><Button onClick={loadIds}>加载</Button><Button variant="outline" disabled={busy || orderedIds.length === 0} onClick={compose}>{busy ? '合成中…' : '合成一个广告视频'}</Button><Button variant="outline" onClick={() => { setIdsText(DEFAULT_IDS); setOrderedIds(DEFAULT_IDS.split(',').map((v) => Number(v))); setResultUrl('') }}>恢复默认</Button></div></CardContent></Card><div className="space-y-3">{isLoading ? <div className="text-slate-300">加载中…</div> : tasks.map((task, idx) => { const url = getResultUrl(task); const subtitle = String(task.subtitle_text || task.render_config?.subtitle_text || ''); return <Card key={task.id} className="border-white/10 bg-slate-900/60 text-slate-100"><CardContent className="flex flex-wrap items-center justify-between gap-3 p-4"><div className="min-w-0 flex-1"><div className="font-medium">task #{task.id} · {task.status || '-'}</div><div className="break-all text-xs text-slate-400">{url || '无结果 URL'}</div>{subtitle && <div className="mt-2 line-clamp-2 text-xs text-violet-200">{subtitle}</div>}</div><div className="flex flex-wrap gap-2"><Button size="sm" variant="outline" onClick={() => move(idx, -1)}>上移</Button><Button size="sm" variant="outline" onClick={() => move(idx, 1)}>下移</Button><Button size="sm" variant="outline" asChild><Link href={`/video/history/${task.id}`}>详情</Link></Button><Button size="sm" variant="outline" asChild><a href={`/video/history/${task.id}?preview=1`} target="_blank" rel="noreferrer">看结果</a></Button><Button size="sm" variant="outline" asChild><a href={url || '#'} target="_blank" rel="noreferrer">打开结果</a></Button></div></CardContent></Card> })}</div>{resultUrl && <Card className="border-white/10 bg-slate-900/60 text-slate-100"><CardHeader><CardTitle>合成结果</CardTitle></CardHeader><CardContent className="space-y-3"><div className="break-all text-sm text-slate-300">{resultUrl}</div><video className="w-full rounded-lg bg-black" controls src={resultUrl} /></CardContent></Card>}</div>
}
