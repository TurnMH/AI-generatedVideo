'use client'

import Link from 'next/link'
import useSWR from 'swr'
import { useParams } from 'next/navigation'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { videoAPI, type VideoTaskDetailResponse } from '@/lib/api'

type TaskShape = {
  id: number
  project_id: number
  status?: string
  model_name?: string
  requested_model?: string
  routed_generator?: string
  runtime_provider?: string
  effective_model?: string
  result_url?: string
  error_msg?: string
  created_at?: string
  updated_at?: string
  render_config?: Record<string, unknown>
  image_urls?: string[]
  clips?: Array<{
    id: number
    clip_order: number
    status?: string
    clip_url?: string
    source_image_url?: string
    model_used?: string
    requested_model?: string
    routed_generator?: string
    runtime_provider?: string
    effective_model?: string
    error_msg?: string
  }>
}

export default function ManualVideoHistoryDetailPage() {
  const params = useParams<{ taskId: string }>()
  const taskId = Number(params?.taskId || 0)
  const { data, isLoading } = useSWR(taskId ? `manual-video-task-${taskId}` : null, () => videoAPI.getTask<TaskShape>(taskId))
  const payload = data as VideoTaskDetailResponse<TaskShape> | undefined
  const task = payload?.data?.task
  const taskDebug = payload?.data?.task_debug_summary
  const clipsDebug = payload?.data?.clips_debug || []

  return (
    <div className="space-y-6">
      <div className="flex items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold text-white">手动视频任务详情</h1>
          <p className="mt-2 text-sm text-slate-300">直接读取后端任务详情接口，不再复用 projects 详情页。</p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" asChild><Link href="/video/history">返回历史记录</Link></Button>
          <Button variant="outline" asChild><Link href="/video">返回创建页</Link></Button>
        </div>
      </div>

      {isLoading ? (
        <Card className="border-white/10 bg-slate-900/60 text-slate-100"><CardContent className="p-6 text-sm text-slate-300">正在加载任务详情…</CardContent></Card>
      ) : !task ? (
        <Card className="border-white/10 bg-slate-900/60 text-slate-100"><CardContent className="p-6 text-sm text-slate-300">未找到 task #{taskId}。</CardContent></Card>
      ) : (
        <>
          <Card className="border-white/10 bg-slate-900/60 text-slate-100">
            <CardHeader>
              <CardTitle>task #{task.id}</CardTitle>
              <CardDescription className="text-slate-400">project_id={task.project_id} · status={task.status || '-'}</CardDescription>
            </CardHeader>
            <CardContent className="grid gap-3 text-sm text-slate-200 md:grid-cols-2">
              <div>model_name：{task.model_name || '-'}</div>
              <div>effective_model：{task.effective_model || '-'}</div>
              <div>requested_model：{task.requested_model || '-'}</div>
              <div>routed_generator：{task.routed_generator || '-'}</div>
              <div>runtime_provider：{task.runtime_provider || '-'}</div>
              <div>created_at：{task.created_at || '-'}</div>
              <div>updated_at：{task.updated_at || '-'}</div>
              <div>result_url：{task.result_url || '-'}</div>
              <div className="md:col-span-2">error_msg：{task.error_msg || '-'}</div>
            </CardContent>
          </Card>

          <Card className="border-white/10 bg-slate-900/60 text-slate-100">
            <CardHeader>
              <CardTitle>任务调试摘要</CardTitle>
              <CardDescription className="text-slate-400">后端 task_debug_summary</CardDescription>
            </CardHeader>
            <CardContent className="grid gap-3 text-sm text-slate-200 md:grid-cols-2">
              <div>requested_model：{taskDebug?.requested_model || '-'}</div>
              <div>effective_model：{taskDebug?.effective_model || '-'}</div>
              <div>routed_generator：{taskDebug?.routed_generator || '-'}</div>
              <div>runtime_provider：{taskDebug?.runtime_provider || '-'}</div>
              <div>route_reason：{taskDebug?.route_reason || '-'}</div>
              <div>clip_count：{taskDebug?.clip_count ?? '-'}</div>
            </CardContent>
          </Card>

          <Card className="border-white/10 bg-slate-900/60 text-slate-100">
            <CardHeader>
              <CardTitle>分镜 / Clip 调试视图</CardTitle>
              <CardDescription className="text-slate-400">后端 clips_debug + task.clips</CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              {clipsDebug.length === 0 ? (
                <div className="text-sm text-slate-400">暂无 clip 调试信息。</div>
              ) : clipsDebug.map((clip, idx) => {
                const clipTask = task.clips?.find((item) => item.clip_order === clip.clip_order)
                return (
                  <div key={`${clip.clip_order}-${idx}`} className="rounded-xl border border-white/10 bg-white/[0.03] p-4 text-sm text-slate-200">
                    <div className="font-medium text-white">clip #{clip.clip_order}</div>
                    <div className="mt-2 grid gap-2 md:grid-cols-2">
                      <div>status：{clipTask?.status || '-'}</div>
                      <div>effective_model：{clip.effective_model || clipTask?.effective_model || '-'}</div>
                      <div>requested_model：{clip.requested_model || clipTask?.requested_model || '-'}</div>
                      <div>routed_generator：{clip.routed_generator || clipTask?.routed_generator || '-'}</div>
                      <div>runtime_provider：{clip.runtime_provider || clipTask?.runtime_provider || '-'}</div>
                      <div>clip_url：{clipTask?.clip_url || '-'}</div>
                      <div className="md:col-span-2">error_msg：{clipTask?.error_msg || '-'}</div>
                    </div>
                  </div>
                )
              })}
            </CardContent>
          </Card>
        </>
      )}
    </div>
  )
}
