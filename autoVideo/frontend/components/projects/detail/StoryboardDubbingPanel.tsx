'use client'

import { useMemo, useState } from 'react'
import useSWR from 'swr'
import { Download, Eye, FileText, Loader2, Mic, RefreshCw, Sparkles } from 'lucide-react'
import { dubbingAPI, storyboardAPI, type DubbingTask } from '@/lib/api'
import type { Project, Storyboard } from '@/types'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { useToast } from '@/components/ui/toast'
import { formatDuration, getPendingQueueMeta, getProgressStallMeta } from '@/lib/projects/utils'
import {
  detectStoryboardVoiceRole,
  formatStoryboardDubbingText,
  getStoryboardVoiceRoleLabel,
  hasSpeakableStoryboardText,
  looksLikeStoryboardVisualDescription,
  resolveStoryboardSpeechLimit,
} from '@/lib/projects/storyboard-dubbing'

type VoiceOptions = {
  voice_model: string
  voice_rate: string
  voice_pitch: string
  voice_volume: string
}

type StoryboardDubbingPanelProps = {
  projectId: number
  project: Pick<Project, 'enable_dubbing' | 'enable_subtitle' | 'storyboard_config'>
  episodeId: number
  episodeNumber: number
  isCommentaryProject: boolean
  voiceOptions: VoiceOptions
  dubbingTaskMap: Map<number, DubbingTask>
  subtitleTaskMap: Map<number, DubbingTask>
  onTasksMutate: () => void
  batchBusy?: boolean
}

export function StoryboardDubbingPanel({
  projectId,
  project,
  episodeId,
  episodeNumber,
  isCommentaryProject,
  voiceOptions,
  dubbingTaskMap,
  subtitleTaskMap,
  onTasksMutate,
  batchBusy = false,
}: StoryboardDubbingPanelProps) {
  const { toast } = useToast()
  const [drafts, setDrafts] = useState<Record<number, string>>({})
  const [expandedStoryboardId, setExpandedStoryboardId] = useState<number | null>(null)
  const [generatingIds, setGeneratingIds] = useState<Set<number>>(new Set())
  const [retryingTaskIds, setRetryingTaskIds] = useState<number[]>([])
  const [subtitleTexts, setSubtitleTexts] = useState<Record<number, string>>({})
  const [loadingSubtitleId, setLoadingSubtitleId] = useState<number | null>(null)

  const { data: sbData, isLoading } = useSWR(
    ['storyboards-dubbing', projectId, episodeId],
    () => storyboardAPI.listAll(projectId, { episode_id: episodeId }) as Promise<{ data?: Storyboard[] }>,
    { revalidateOnFocus: false },
  )

  const storyboards = useMemo(
    () => [...(sbData?.data ?? [])].sort((a, b) => a.sequence_number - b.sequence_number),
    [sbData],
  )

  const getFormattedText = (sb: Storyboard) =>
    formatStoryboardDubbingText(sb, {
      isCommentary: isCommentaryProject,
      maxRunes: resolveStoryboardSpeechLimit(sb, project),
    })

  const getSubmitText = (sb: Storyboard) => {
    const draft = drafts[sb.id]
    if (draft != null) return draft.trim()
    return getFormattedText(sb)
  }

  const runGenerate = async (
    sb: Storyboard,
    taskType: 'dubbing' | 'subtitle',
    options?: { silent?: boolean },
  ) => {
    const text = getSubmitText(sb)
    if (!text) {
      if (!options?.silent) {
        toast({ title: `分镜 #${sb.sequence_number} 暂无可合成台词`, variant: 'destructive' })
      }
      return 'skipped' as const
    }
    if (!sb.episode_id) {
      if (!options?.silent) {
        toast({ title: '分镜缺少集数信息', variant: 'destructive' })
      }
      return 'failed' as const
    }

    setGeneratingIds((prev) => new Set(prev).add(sb.id))
    try {
      if (taskType === 'dubbing') {
        await dubbingAPI.generateForStoryboard(
          projectId,
          sb.id,
          sb.episode_id,
          text,
          voiceOptions.voice_model,
          voiceOptions,
        )
        if (!options?.silent) {
          toast({ title: `第 ${episodeNumber} 集 · 分镜 #${sb.sequence_number} 配音已提交`, variant: 'success' })
        }
      } else {
        await dubbingAPI.generateSubtitleForStoryboard(
          projectId,
          sb.id,
          sb.episode_id,
          text,
          voiceOptions,
        )
        if (!options?.silent) {
          toast({ title: `第 ${episodeNumber} 集 · 分镜 #${sb.sequence_number} 字幕已提交`, variant: 'success' })
        }
      }
      onTasksMutate()
      return 'submitted' as const
    } catch (err) {
      const status = (err as { response?: { status?: number } })?.response?.status
      if (!options?.silent) {
        toast({
          title: status === 409 ? '该分镜已有进行中的任务' : `${taskType === 'dubbing' ? '配音' : '字幕'}提交失败`,
          variant: 'destructive',
        })
      }
      return status === 409 ? ('conflict' as const) : ('failed' as const)
    } finally {
      setGeneratingIds((prev) => {
        const next = new Set(prev)
        next.delete(sb.id)
        return next
      })
    }
  }

  const handleRetry = async (task: DubbingTask, sb: Storyboard, label: string) => {
    setRetryingTaskIds((prev) => [...prev, task.id])
    try {
      await dubbingAPI.retryTask(projectId, task.id, getSubmitText(sb))
      toast({ title: `分镜 #${sb.sequence_number} ${label}任务已重新拉起`, variant: 'success' })
      onTasksMutate()
    } catch {
      toast({ title: `${label}重试失败`, variant: 'destructive' })
    } finally {
      setRetryingTaskIds((prev) => prev.filter((id) => id !== task.id))
    }
  }

  const handleGenerateEpisode = async (taskType: 'dubbing' | 'subtitle') => {
    const eligible = storyboards.filter((sb) => {
      if (!hasSpeakableStoryboardText(sb, { isCommentary: isCommentaryProject }) && drafts[sb.id] == null) {
        return false
      }
      const text = getSubmitText(sb)
      if (!text) return false
      const existing = taskType === 'dubbing' ? dubbingTaskMap.get(sb.id) : subtitleTaskMap.get(sb.id)
      return !(existing && (existing.status === 'succeeded' || existing.status === 'processing' || existing.status === 'pending'))
    })
    const skippedVisual = storyboards.filter(
      (sb) => !hasSpeakableStoryboardText(sb, { isCommentary: isCommentaryProject }) && drafts[sb.id] == null,
    ).length

    if (eligible.length === 0) {
      toast({
        title: skippedVisual > 0 ? `没有可提交的分镜（${skippedVisual} 个分镜仅有画面描述）` : '没有可提交的分镜',
        variant: 'default',
      })
      return
    }

    let submitted = 0
    let conflicts = 0
    let failed = 0
    for (const sb of eligible) {
      const result = await runGenerate(sb, taskType, { silent: true })
      if (result === 'submitted') submitted++
      else if (result === 'conflict') conflicts++
      else if (result === 'failed') failed++
    }

    const label = taskType === 'dubbing' ? '配音' : '字幕'
    toast({
      title: failed > 0
        ? `已提交 ${submitted} 个分镜${label}，${conflicts} 个跳过，${failed} 个失败${skippedVisual > 0 ? `，${skippedVisual} 个无可合成台词` : ''}`
        : conflicts > 0 || skippedVisual > 0
          ? `已提交 ${submitted} 个分镜${label}${conflicts > 0 ? `，${conflicts} 个跳过` : ''}${skippedVisual > 0 ? `，${skippedVisual} 个无可合成台词` : ''}`
          : `已提交 ${submitted} 个分镜${label}任务`,
      variant: failed > 0 ? 'default' : 'success',
    })
  }

  if (isLoading) {
    return (
      <div className="flex items-center gap-2 py-6 text-xs text-surface-400">
        <Loader2 className="h-4 w-4 animate-spin" /> 加载分镜列表…
      </div>
    )
  }

  if (storyboards.length === 0) {
    return <p className="py-4 text-xs text-surface-400">该集暂无分镜，请先在分镜页完成拆分</p>
  }

  const dubbingDone = storyboards.filter((sb) => dubbingTaskMap.get(sb.id)?.status === 'succeeded').length
  const subtitleDone = storyboards.filter((sb) => subtitleTaskMap.get(sb.id)?.status === 'succeeded').length

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center justify-between gap-2 rounded-md border border-dashed border-surface-200 bg-surface-50 px-3 py-2">
        <div className="text-[11px] text-surface-500">
          按分镜逐条合成，与视频片段一一对齐，减少整集配音带来的时长偏差。
          {project.enable_dubbing && (
            <span className="ml-2 rounded bg-blue-100 px-1.5 py-0.5 text-blue-700">
              配音 {dubbingDone}/{storyboards.length}
            </span>
          )}
          {project.enable_subtitle && (
            <span className="ml-1 rounded bg-green-100 px-1.5 py-0.5 text-green-700">
              字幕 {subtitleDone}/{storyboards.length}
            </span>
          )}
        </div>
        <div className="flex flex-wrap gap-2">
          {project.enable_dubbing && (
            <Button
              size="sm"
              variant="outline"
              className="h-7 text-[11px]"
              disabled={batchBusy || generatingIds.size > 0}
              onClick={() => handleGenerateEpisode('dubbing')}
            >
              <Mic className="mr-1 h-3 w-3" /> 本集全部配音
            </Button>
          )}
          {project.enable_subtitle && (
            <Button
              size="sm"
              variant="outline"
              className="h-7 text-[11px]"
              disabled={batchBusy || generatingIds.size > 0}
              onClick={() => handleGenerateEpisode('subtitle')}
            >
              <FileText className="mr-1 h-3 w-3" /> 本集全部字幕
            </Button>
          )}
        </div>
      </div>

      {storyboards.map((sb) => {
        const dubTask = dubbingTaskMap.get(sb.id)
        const subTask = subtitleTaskMap.get(sb.id)
        const role = detectStoryboardVoiceRole(sb)
        const roleLabel = getStoryboardVoiceRoleLabel(role)
        const visualOnly = !hasSpeakableStoryboardText(sb, { isCommentary: isCommentaryProject }) && drafts[sb.id] == null
        const formattedText = getFormattedText(sb)
        const roleClass =
          role === 'narrator'
            ? 'bg-violet-100 text-violet-700'
            : role === 'character'
              ? 'bg-amber-100 text-amber-700'
              : 'bg-surface-200 text-surface-600'
        const isGenerating = generatingIds.has(sb.id)

        return (
          <div key={sb.id} className="rounded-lg border border-surface-200 bg-white p-3">
            <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
              <div className="flex items-center gap-2">
                <span className="text-xs font-semibold text-surface-800">分镜 #{sb.sequence_number}</span>
                <span className={`rounded px-1.5 py-0.5 text-[10px] ${roleClass}`}>{roleLabel}</span>
                {visualOnly && (
                  <span className="rounded bg-surface-200 px-1.5 py-0.5 text-[10px] text-surface-500" title="该分镜台词仅为画面/构图描述，已自动跳过">
                    无台词
                  </span>
                )}
                {sb.duration > 0 && (
                  <span className="text-[10px] text-surface-400">{formatDuration(sb.duration)}</span>
                )}
              </div>
              <div className="flex flex-wrap items-center gap-1">
                {project.enable_dubbing && (
                  <Button
                    size="sm"
                    variant="ghost"
                    className="h-7 px-2 text-[11px]"
                    disabled={isGenerating || batchBusy || !getSubmitText(sb) || visualOnly || dubTask?.status === 'processing' || dubTask?.status === 'pending'}
                    onClick={() => runGenerate(sb, 'dubbing')}
                  >
                    {isGenerating || dubTask?.status === 'processing' || dubTask?.status === 'pending'
                      ? <Loader2 className="mr-1 h-3 w-3 animate-spin" />
                      : <Mic className="mr-1 h-3 w-3" />}
                    配音
                  </Button>
                )}
                {project.enable_subtitle && (
                  <Button
                    size="sm"
                    variant="ghost"
                    className="h-7 px-2 text-[11px]"
                    disabled={isGenerating || batchBusy || !getSubmitText(sb) || visualOnly || subTask?.status === 'processing' || subTask?.status === 'pending'}
                    onClick={() => runGenerate(sb, 'subtitle')}
                  >
                    {subTask?.status === 'processing' || subTask?.status === 'pending'
                      ? <Loader2 className="mr-1 h-3 w-3 animate-spin" />
                      : <FileText className="mr-1 h-3 w-3" />}
                    字幕
                  </Button>
                )}
              </div>
            </div>

            <Textarea
              value={drafts[sb.id] ?? formattedText}
              onChange={(e) => setDrafts((prev) => ({ ...prev, [sb.id]: e.target.value }))}
              rows={2}
              className="min-h-[56px] border-surface-200 bg-surface-50 text-xs"
              placeholder={
                visualOnly || looksLikeStoryboardVisualDescription(sb.dialogue || '')
                  ? '该分镜暂无解说台词（原内容为画面/构图描述，不会提交配音）'
                  : '分镜台词（建议使用「旁白：…」或「角色名：…」区分声线）'
              }
            />

            <div className="mt-2 grid gap-2 md:grid-cols-2">
              {project.enable_dubbing && (
                <StoryboardTaskStatus
                  label="配音"
                  color="blue"
                  task={dubTask}
                  expanded={expandedStoryboardId === sb.id}
                  onToggleExpand={() => setExpandedStoryboardId((prev) => (prev === sb.id ? null : sb.id))}
                  onRetry={dubTask ? () => handleRetry(dubTask, sb, '配音') : undefined}
                  retrying={dubTask ? retryingTaskIds.includes(dubTask.id) : false}
                />
              )}
              {project.enable_subtitle && (
                <StoryboardTaskStatus
                  label="字幕"
                  color="green"
                  task={subTask}
                  expanded={expandedStoryboardId === sb.id}
                  onToggleExpand={async () => {
                    if (expandedStoryboardId === sb.id) {
                      setExpandedStoryboardId(null)
                      return
                    }
                    setExpandedStoryboardId(sb.id)
                    if (!subTask?.subtitle_url || subtitleTexts[sb.id]) return
                    setLoadingSubtitleId(sb.id)
                    try {
                      const resp = await fetch(subTask.subtitle_url)
                      const text = await resp.text()
                      setSubtitleTexts((prev) => ({ ...prev, [sb.id]: text }))
                    } catch {
                      setSubtitleTexts((prev) => ({ ...prev, [sb.id]: '字幕加载失败' }))
                    } finally {
                      setLoadingSubtitleId(null)
                    }
                  }}
                  onRetry={subTask ? () => handleRetry(subTask, sb, '字幕') : undefined}
                  retrying={subTask ? retryingTaskIds.includes(subTask.id) : false}
                  subtitlePreview={subtitleTexts[sb.id]}
                  loadingSubtitle={loadingSubtitleId === sb.id}
                />
              )}
            </div>
          </div>
        )
      })}
    </div>
  )
}

function StoryboardTaskStatus({
  label,
  color,
  task,
  expanded,
  onToggleExpand,
  onRetry,
  retrying,
  subtitlePreview,
  loadingSubtitle,
}: {
  label: string
  color: 'blue' | 'green'
  task?: DubbingTask
  expanded: boolean
  onToggleExpand: () => void
  onRetry?: () => void
  retrying: boolean
  subtitlePreview?: string
  loadingSubtitle?: boolean
}) {
  const chipClass = color === 'blue' ? 'bg-blue-100 text-blue-700' : 'bg-green-100 text-green-700'
  const barClass = color === 'blue' ? 'bg-blue-400' : 'bg-green-400'
  const trackClass = color === 'blue' ? 'bg-blue-100' : 'bg-green-100'

  return (
    <div className={`rounded-md px-2.5 py-2 ${color === 'blue' ? 'bg-blue-50' : 'bg-green-50'}`}>
      <div className="flex items-center justify-between gap-2">
        <div className="flex items-center gap-1.5 text-[11px]">
          <span className={`font-medium ${color === 'blue' ? 'text-blue-700' : 'text-green-700'}`}>{label}</span>
          {!task ? (
            <span className="text-surface-400">待生成</span>
          ) : task.status === 'succeeded' ? (
            <span className={`rounded px-1.5 py-0.5 text-[10px] ${chipClass}`}>已完成</span>
          ) : task.status === 'processing' || task.status === 'pending' ? (
            <span className="flex items-center gap-1 text-amber-600">
              <Loader2 className="h-2.5 w-2.5 animate-spin" />
              {task.chunks_total > 0 ? `${task.chunks_done}/${task.chunks_total}` : '排队中'}
            </span>
          ) : (
            <span className="rounded bg-red-100 px-1.5 py-0.5 text-[10px] text-red-600" title={task.error_msg}>失败</span>
          )}
        </div>
        <div className="flex items-center gap-1">
          {(task?.audio_url || task?.subtitle_url) && (
            <Button size="sm" variant="ghost" className="h-6 w-6 p-0" onClick={onToggleExpand} title="预览">
              <Eye className="h-3 w-3" />
            </Button>
          )}
          {task?.status === 'failed' || (task?.status === 'processing' && getProgressStallMeta(task.updated_at)) ? (
            <Button
              size="sm"
              variant="ghost"
              className="h-6 px-1.5 text-[10px]"
              disabled={retrying}
              onClick={onRetry}
            >
              {retrying ? <Loader2 className="h-3 w-3 animate-spin" /> : <RefreshCw className="h-3 w-3" />}
            </Button>
          ) : null}
        </div>
      </div>

      {task && (task.status === 'processing' || task.status === 'pending') && (
        <div className={`mt-1.5 h-1 overflow-hidden rounded-full ${trackClass}`}>
          <div
            className={`h-full rounded-full transition-all ${barClass}`}
            style={{
              width: `${task.chunks_total > 0
                ? Math.max((task.chunks_done / task.chunks_total) * 100, task.status === 'pending' ? 8 : 0)
                : 8}%`,
            }}
          />
        </div>
      )}

      {task?.status === 'pending' && getPendingQueueMeta(task.updated_at) && (
        <p className="mt-1 text-[10px] text-amber-600">任务排队中，请稍候</p>
      )}

      {expanded && task?.audio_url && (
        <div className="mt-2">
          <audio controls className="h-8 w-full" src={task.audio_url} />
          <a href={task.audio_url} target="_blank" rel="noopener noreferrer" className="mt-1 inline-flex items-center gap-1 text-[10px] text-blue-600 hover:underline">
            <Download className="h-3 w-3" /> 下载
          </a>
        </div>
      )}

      {expanded && task?.subtitle_url && (
        <div className="mt-2">
          {loadingSubtitle ? (
            <div className="flex items-center gap-1 text-[10px] text-surface-400">
              <Loader2 className="h-3 w-3 animate-spin" /> 加载中…
            </div>
          ) : subtitlePreview ? (
            <pre className="max-h-32 overflow-y-auto rounded bg-white/80 p-2 text-[10px] whitespace-pre-wrap text-surface-600">{subtitlePreview}</pre>
          ) : null}
          <a href={task.subtitle_url} target="_blank" rel="noopener noreferrer" className="mt-1 inline-flex items-center gap-1 text-[10px] text-green-600 hover:underline">
            <Download className="h-3 w-3" /> 下载 VTT
          </a>
        </div>
      )}
    </div>
  )
}
