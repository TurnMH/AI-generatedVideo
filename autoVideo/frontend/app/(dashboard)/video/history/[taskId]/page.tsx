'use client'

import { useState } from 'react'
import Link from 'next/link'
import useSWR from 'swr'
import { useParams } from 'next/navigation'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { videoAPI, type VideoTaskDetailResponse } from '@/lib/api'
import { useToast } from '@/components/ui/toast'

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
  subtitle_text?: string
  scene_description?: string
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
  const { toast } = useToast()
  const [busyAction, setBusyAction] = useState('')
  const [previewVariant, setPreviewVariant] = useState<'current' | 'original' | 'subtitled'>('current')
  const taskId = Number(params?.taskId || 0)
  const { data, isLoading, mutate } = useSWR(taskId ? `manual-video-task-${taskId}` : null, () => videoAPI.getTask<TaskShape>(taskId), {
    refreshInterval: (latest) => {
      const task = (latest as VideoTaskDetailResponse<TaskShape> | undefined)?.data?.task
      return task && (task.status === 'pending' || task.status === 'processing') ? 5000 : 0
    },
    revalidateOnFocus: true,
  })
  const payload = data as VideoTaskDetailResponse<TaskShape> | undefined
  const task = payload?.data?.task
  const taskDebug = payload?.data?.task_debug_summary
  const clipsDebug = payload?.data?.clips_debug || []
  const submissionPreview = (payload?.data as {
    submission_preview?: {
      generate_audio?: boolean
      strategy?: string
      note?: string
      items?: Array<{
        clip_order?: number
        visual_prompt?: string
        voice_text?: string
        actual_submission_text?: string
        generate_audio?: boolean
        native_audio_model_hint?: string
      }>
    }
  } | undefined)?.submission_preview
  const dialoguesText = Array.isArray(task?.render_config?.dialogues)
    ? (task?.render_config?.dialogues as unknown[]).map((item) => String(item || '')).filter(Boolean).join('\n\n')
    : ''
  const visualPromptText = Array.isArray(task?.render_config?.scene_descriptions)
    ? (task?.render_config?.scene_descriptions as unknown[]).map((item) => String(item || '')).join('\n\n')
    : String(task?.render_config?.scene_description || task?.['scene_description'] || '')
  const subtitleText = String(task?.subtitle_text || task?.render_config?.subtitle_text || '')
  const nativeAudioEnabled = Boolean(task?.render_config?.generate_audio)
  const audioSummary = dialoguesText || subtitleText
  const originalResultUrl = String(task?.render_config?.original_result_url || '').trim()
  const subtitledResultUrl = String(task?.render_config?.subtitled_result_url || '').trim()
  const activeResultVariant = String(task?.render_config?.active_result_variant || '').trim()
  const subtitleComposeStatus = String(task?.render_config?.subtitle_compose_status || '').trim()
  const subtitleComposeError = String(task?.render_config?.subtitle_compose_error || '').trim()
  const previewUrl = previewVariant === 'original'
    ? (originalResultUrl || task?.result_url || subtitledResultUrl)
    : previewVariant === 'subtitled'
      ? (subtitledResultUrl || task?.result_url || originalResultUrl)
      : (task?.result_url || subtitledResultUrl || originalResultUrl)

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
          <h1 className="text-2xl font-semibold text-white">手动视频任务详情</h1>
          <p className="mt-2 text-sm text-slate-300">直接读取后端任务详情接口，不再复用 projects 详情页；pending/processing 自动刷新。</p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" onClick={() => mutate()}>立即刷新</Button>
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
            <CardContent className="space-y-4">
              <div className="grid gap-3 text-sm text-slate-200 md:grid-cols-2">
                <div>model_name：{task.model_name || '-'}</div>
                <div>effective_model：{task.effective_model || '-'}</div>
                <div>requested_model：{task.requested_model || '-'}</div>
                <div>routed_generator：{task.routed_generator || '-'}</div>
                <div>runtime_provider：{task.runtime_provider || '-'}</div>
                <div>created_at：{task.created_at || '-'}</div>
                <div>updated_at：{task.updated_at || '-'}</div>
                <div className="md:col-span-2 break-all">result_url：{task.result_url || '-'}</div>
                <div className="md:col-span-2 break-all">original_result_url：{originalResultUrl || '-'}</div>
                <div className="md:col-span-2 break-all">subtitled_result_url：{subtitledResultUrl || '-'}</div>
                <div>subtitle_compose_status：{subtitleComposeStatus || '-'}</div>
                <div className="md:col-span-2 break-all">subtitle_compose_error：{subtitleComposeError || '-'}</div>
                <div className="md:col-span-2">error_msg：{task.error_msg || '-'}</div>
              </div>
              <div className="flex flex-wrap gap-2">
                {task.project_id > 0 && (
                  <Button variant="outline" disabled={busyAction !== ''} onClick={() => runAction('retry', () => videoAPI.retryVideoTask(task.project_id, task.id, task.requested_model || task.model_name), '已触发重试')}>
                    {busyAction === 'retry' ? '重试中…' : '重试任务'}
                  </Button>
                )}
                <Button variant="outline" disabled={busyAction !== ''} onClick={() => runAction('cancel', () => videoAPI.cancelVideoTask(task.id), '已取消/删除任务')}>
                  {busyAction === 'cancel' ? '处理中…' : '取消任务'}
                </Button>
                <Button variant="outline" disabled={busyAction !== ''} onClick={() => runAction('compose', () => videoAPI.compose(task.id), '已触发重新合成')}>
                  {busyAction === 'compose' ? '处理中…' : '重新合成'}
                </Button>
                {previewUrl && audioSummary && (
                  <Button
                    variant="outline"
                    disabled={busyAction !== ''}
                    onClick={() => runAction('subtitle', () => videoAPI.compose(task.id), '已触发重新合成（将尝试烧录字幕）')}
                  >
                    {busyAction === 'subtitle' ? '处理中…' : '添加字幕'}
                  </Button>
                )}
                {task.project_id > 0 && (
                  <Button variant="outline" disabled={busyAction !== ''} onClick={() => runAction('export', () => videoAPI.export(task.project_id, task.id), '已请求导出接口')}>
                    {busyAction === 'export' ? '处理中…' : '请求导出'}
                  </Button>
                )}
                {previewUrl && (
                  <Button variant="outline" asChild>
                    <a href={previewUrl} target="_blank" rel="noreferrer">打开当前结果</a>
                  </Button>
                )}
                {originalResultUrl && (
                  <Button variant="outline" asChild>
                    <a href={originalResultUrl} target="_blank" rel="noreferrer">打开原视频</a>
                  </Button>
                )}
                {subtitledResultUrl && (
                  <Button variant="outline" asChild>
                    <a href={subtitledResultUrl} target="_blank" rel="noreferrer">打开字幕版</a>
                  </Button>
                )}
              </div>
              {previewUrl && (
                <div className="overflow-hidden rounded-xl border border-white/10 bg-black/30 p-3">
                  <div className="mb-2 flex flex-wrap items-center gap-2 text-xs text-slate-400">
                    {subtitleComposeStatus === 'failed' && (
                      <span className="rounded-full bg-rose-500/15 px-2 py-0.5 text-[10px] text-rose-300">字幕烧录失败</span>
                    )}
                    {subtitleComposeStatus === 'applied' && (
                      <span className="rounded-full bg-emerald-500/15 px-2 py-0.5 text-[10px] text-emerald-300">字幕烧录成功</span>
                    )}
                    <span>任务结果在线查看</span>
                    <span className="rounded-full bg-slate-800 px-2 py-0.5 text-[10px] text-slate-200">
                      {activeResultVariant === 'subtitled' ? '当前默认：字幕版' : '当前默认：原视频'}
                    </span>
                    {originalResultUrl && subtitledResultUrl && (
                      <span className="rounded-full bg-amber-500/15 px-2 py-0.5 text-[10px] text-amber-300">已保留原视频 + 字幕版</span>
                    )}
                  </div>
                  {(originalResultUrl || subtitledResultUrl) && (
                    <div className="mb-3 flex flex-wrap gap-2">
                      <Button size="sm" variant={previewVariant === 'current' ? 'default' : 'outline'} onClick={() => setPreviewVariant('current')}>
                        当前结果预览
                      </Button>
                      {originalResultUrl && (
                        <Button size="sm" variant={previewVariant === 'original' ? 'default' : 'outline'} onClick={() => setPreviewVariant('original')}>
                          原视频预览
                        </Button>
                      )}
                      {subtitledResultUrl && (
                        <Button size="sm" variant={previewVariant === 'subtitled' ? 'default' : 'outline'} onClick={() => setPreviewVariant('subtitled')}>
                          字幕版预览
                        </Button>
                      )}
                    </div>
                  )}
                  <video className="max-h-[420px] w-full rounded-lg bg-black" controls preload="metadata" src={previewUrl} />
                </div>
              )}
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
              <CardTitle>真实提交文本 / 音频字段</CardTitle>
              <CardDescription className="text-slate-400">直接读取 task.render_config / subtitle_text / submission_preview；用于核对“展示台词”和“实际送模型文本”是否一致</CardDescription>
            </CardHeader>
            <CardContent className="space-y-4 text-sm text-slate-200">
              <div className="grid gap-3 md:grid-cols-2">
                <div>generate_audio：{String(nativeAudioEnabled)}</div>
                <div>subtitle_text：{subtitleText || '-'}</div>
              </div>
              <div className="grid gap-3 lg:grid-cols-2">
                <div className="space-y-2">
                  <div className="text-xs font-medium text-emerald-200">scene_descriptions（视觉文本）</div>
                  <div className="max-h-64 overflow-auto rounded-lg border border-emerald-400/20 bg-slate-950/60 p-3 whitespace-pre-wrap break-words">
                    {visualPromptText || '-'}
                  </div>
                </div>
                <div className="space-y-2">
                  <div className="text-xs font-medium text-violet-200">dialogues（展示中的旁白/对白文本）</div>
                  <div className="max-h-64 overflow-auto rounded-lg border border-violet-400/20 bg-slate-950/60 p-3 whitespace-pre-wrap break-words">
                    {dialoguesText || '（空）'}
                  </div>
                </div>
              </div>
              <div className="rounded-lg border border-dashed border-white/10 bg-slate-950/30 px-3 py-2 text-xs leading-5 text-slate-400">
                当前声音来源判定：{nativeAudioEnabled ? <span className="font-mono text-emerald-300">native audio（dialogues / subtitle_text 只是意图文本，不等于逐字朗读保证）</span> : <span className="font-mono text-slate-200">非 native audio / 需结合 dubbing 链路判断</span>}。视觉区里的 <span className="font-mono text-slate-200">scene_descriptions</span> 只描述画面，不应被当成旁白真值。
              </div>
              {submissionPreview?.items?.length ? (
                <div className="space-y-3">
                  <div className="text-xs font-medium text-amber-200">submission_preview（当前代码推导的实际送模型文本）</div>
                  <div className="rounded-lg border border-amber-400/20 bg-amber-950/10 px-3 py-2 text-xs leading-5 text-slate-300">
                    <div>strategy：{submissionPreview.strategy || '-'}</div>
                    <div className="mt-1">note：{submissionPreview.note || '-'}</div>
                  </div>
                  {submissionPreview.items.map((item, idx) => (
                    <div key={`submission-preview-${item.clip_order ?? idx}`} className="grid gap-3 rounded-xl border border-white/10 bg-slate-950/40 p-3 lg:grid-cols-3">
                      <div className="space-y-2">
                        <div className="text-xs font-medium text-emerald-200">clip #{item.clip_order ?? idx} 视觉 prompt</div>
                        <div className="max-h-56 overflow-auto rounded-lg border border-emerald-400/20 bg-slate-950/60 p-3 whitespace-pre-wrap break-words">
                          {item.visual_prompt || '-'}
                        </div>
                      </div>
                      <div className="space-y-2">
                        <div className="text-xs font-medium text-violet-200">clip #{item.clip_order ?? idx} 旁白/对白意图</div>
                        <div className="max-h-56 overflow-auto rounded-lg border border-violet-400/20 bg-slate-950/60 p-3 whitespace-pre-wrap break-words">
                          {item.voice_text || '（空）'}
                        </div>
                      </div>
                      <div className="space-y-2">
                        <div className="text-xs font-medium text-amber-200">clip #{item.clip_order ?? idx} 实际送模型文本</div>
                        <div className="max-h-56 overflow-auto rounded-lg border border-amber-400/20 bg-slate-950/60 p-3 whitespace-pre-wrap break-words">
                          {item.actual_submission_text || '-'}
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              ) : null}
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
                      <div className="break-all">clip_url：{clipTask?.clip_url || '-'}</div>
                      <div className="md:col-span-2">error_msg：{clipTask?.error_msg || '-'}</div>
                    </div>
                    {clipTask?.clip_url && (
                      <div className="mt-3 space-y-2">
                        <div className="flex flex-wrap gap-2">
                          <Button variant="outline" asChild>
                            <a href={clipTask.clip_url} target="_blank" rel="noreferrer">在线查看 clip</a>
                          </Button>
                        </div>
                        <video className="max-h-[320px] w-full rounded-lg bg-black" controls preload="metadata" src={clipTask.clip_url} />
                      </div>
                    )}
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
