'use client'

import { useEffect, useMemo, useState } from 'react'
import Link from 'next/link'
import useSWR from 'swr'
import { Music2, Plus, Sparkles, Loader2, Clock3, AudioLines, Mic2 } from 'lucide-react'
import { modelAPI, projectAPI, taskAPI } from '@/lib/api'
import { matchesProjectMedia, PROJECT_MEDIA_META } from '@/lib/project-media'
import type { Model, Project, Task } from '@/types'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent } from '@/components/ui/card'
import { Progress } from '@/components/ui/progress'
import { useToast } from '@/components/ui/toast'
import { DashboardHero } from '@/components/layout/dashboard-hero'
import { DashboardEmptyState } from '@/components/layout/dashboard-empty-state'

type MusicTask = Task & {
  payload?: {
    project_id?: number
    project_title?: string
    model_id?: number
    model_name?: string
    title?: string
    prompt?: string
    mood?: string
    instruments?: string
    duration_sec?: number
    has_vocals?: boolean
    lyrics?: string
    negative_prompt?: string
  }
}

type MusicFormState = {
  title: string
  prompt: string
  mood: string
  instruments: string
  duration_sec: string
  has_vocals: string
  lyrics: string
  negative_prompt: string
}

const DEFAULT_FORM: MusicFormState = {
  title: '',
  prompt: '',
  mood: '史诗感 / 情绪推进',
  instruments: '弦乐、钢琴、鼓点',
  duration_sec: '45',
  has_vocals: 'no',
  lyrics: '',
  negative_prompt: '',
}

const MUSIC_PRESETS: { label: string; form: Partial<MusicFormState> }[] = [
  { label: '热血战斗 BGM', form: { mood: '热血、紧张、推进感强', instruments: '鼓组、铜管、电吉他、合成器', prompt: '为高能战斗与追逐场景生成热血配乐，节奏强、推进快、高潮明确。', duration_sec: '60' } },
  { label: '国风仙侠配乐', form: { mood: '东方幻想、飘逸、空灵', instruments: '古筝、笛子、鼓、弦乐铺底', prompt: '为国风仙侠、神话旅程生成东方气质的背景音乐，兼具空灵与史诗感。', duration_sec: '75' } },
  { label: '悬疑反转氛围', form: { mood: '压迫、悬念、心理博弈', instruments: '低频鼓点、钢琴断奏、电子氛围音', prompt: '为悬疑推理和剧情反转片段生成压迫感强的氛围音乐，适合铺垫和揭晓。', duration_sec: '45' } },
  { label: '片尾主题歌', form: { mood: '抒情、收束、余韵', instruments: '钢琴、弦乐、人声铺垫', prompt: '生成具有情绪收束能力的片尾主题音乐，旋律清晰，适合字幕和情绪回落。', duration_sec: '90', has_vocals: 'yes' } },
]

const statusVariant: Record<Task['status'], 'default' | 'secondary' | 'success' | 'destructive' | 'outline'> = {
  pending: 'secondary',
  running: 'default',
  succeeded: 'success',
  failed: 'destructive',
  cancelled: 'outline',
}

const statusLabel: Record<Task['status'], string> = {
  pending: '待处理',
  running: '进行中',
  succeeded: '已完成',
  failed: '失败',
  cancelled: '已取消',
}

export default function MusicPage() {
  const { toast } = useToast()
  const [selectedProjectId, setSelectedProjectId] = useState<string>('')
  const [preferredProjectId, setPreferredProjectId] = useState<string | null>(null)
  const [selectedModelId, setSelectedModelId] = useState<string>('')
  const [submitting, setSubmitting] = useState(false)
  const [form, setForm] = useState<MusicFormState>(DEFAULT_FORM)

  useEffect(() => {
    if (typeof window === 'undefined') return
    setPreferredProjectId(new URLSearchParams(window.location.search).get('project'))
  }, [])

  const { data: projectsRaw, isLoading: projectsLoading } = useSWR(
    ['music-projects'],
    () => projectAPI.list({ project_type: 'music', page: 1, page_size: 100 }) as unknown as Promise<{ data?: { items?: Project[] } | Project[] }>
  )
  const projectsData = (projectsRaw as { data?: { items?: Project[] } | Project[] })?.data
  const projects = useMemo(
    () => (Array.isArray(projectsData) ? projectsData : (projectsData?.items ?? [])).filter((project) => matchesProjectMedia(project, 'music')),
    [projectsData]
  )

  useEffect(() => {
    if (projects.length === 0) return
    if (preferredProjectId && projects.some((project) => String(project.id) === preferredProjectId)) {
      if (selectedProjectId !== preferredProjectId) {
        setSelectedProjectId(preferredProjectId)
      }
      return
    }
    if (!selectedProjectId || !projects.some((project) => String(project.id) === selectedProjectId)) {
      setSelectedProjectId(String(projects[0].id))
    }
  }, [preferredProjectId, projects, selectedProjectId])

  const numericProjectId = Number(selectedProjectId)
  const { data: projectRaw } = useSWR(
    Number.isFinite(numericProjectId) && numericProjectId > 0 ? ['music-project', numericProjectId] : null,
    () => projectAPI.get(numericProjectId) as unknown as Promise<{ data: Project }>
  )
  const project = (projectRaw as { data?: Project })?.data

  const { data: modelsRaw, isLoading: modelsLoading } = useSWR(
    ['audio-models'],
    () => modelAPI.list({ type: 'audio', enabled: 'true', sort_by: 'priority' }) as unknown as Promise<{ data?: Model[] }>
  )
  const audioModels = ((modelsRaw as { data?: Model[] })?.data ?? []).filter((model) => model.type === 'audio')

  useEffect(() => {
    if (!selectedModelId && audioModels.length > 0) {
      const preferred = audioModels.find((model) => model.is_default) ?? audioModels[0]
      setSelectedModelId(String(preferred.id))
    }
  }, [audioModels, selectedModelId])

  useEffect(() => {
    if (typeof window === 'undefined') return
    const raw = window.localStorage.getItem('music-workspace-form')
    if (!raw) return
    try {
      const saved = JSON.parse(raw) as Partial<MusicFormState>
      setForm((prev) => ({ ...prev, ...saved }))
    } catch {
      // ignore invalid cache
    }
  }, [])

  useEffect(() => {
    if (typeof window === 'undefined') return
    window.localStorage.setItem('music-workspace-form', JSON.stringify(form))
  }, [form])

  const { data: tasksRaw, mutate: mutateTasks } = useSWR(
    Number.isFinite(numericProjectId) && numericProjectId > 0 ? ['music-tasks', numericProjectId] : null,
    () => taskAPI.list({ type: 'music_generate', project_id: numericProjectId, page: 1, page_size: 100 }) as unknown as Promise<{ data?: { items?: MusicTask[] } }>,
    { refreshInterval: (data) => {
        const items = (data as { data?: { items?: MusicTask[] } } | undefined)?.data?.items ?? []
        const active = items.some((t) => t.status === 'running')
        return active ? 5000 : 30000
      } }
  )
  const tasks = ((tasksRaw as { data?: { items?: MusicTask[] } })?.data?.items ?? [])
  const visibleTasks = tasks
  const selectedModel = audioModels.find((model) => String(model.id) === selectedModelId)

  const counts = visibleTasks.reduce((acc, task) => {
    acc[task.status] = (acc[task.status] ?? 0) + 1
    return acc
  }, {} as Record<string, number>)

  const applyPreset = (preset: Partial<MusicFormState>) => {
    setForm((prev) => ({ ...prev, ...preset }))
  }

  const handleSubmit = async () => {
    if (!project) {
      toast({ title: '请先选择项目', variant: 'destructive' })
      return
    }
    if (!selectedModel) {
      toast({ title: '请先选择音频模型', variant: 'destructive' })
      return
    }
    if (!form.prompt.trim()) {
      toast({ title: '请输入音乐创作提示词', variant: 'destructive' })
      return
    }

    setSubmitting(true)
    try {
      await taskAPI.create({
        task_type: 'music_generate',
        priority: 5,
        payload: {
          project_id: project.id,
          project_title: project.title,
          model_id: selectedModel.id,
          model_name: selectedModel.name,
          title: form.title.trim() || `${project.title} 配乐`,
          prompt: form.prompt.trim(),
          mood: form.mood.trim(),
          instruments: form.instruments.trim(),
          duration_sec: Number(form.duration_sec) || 45,
          has_vocals: form.has_vocals === 'yes',
          lyrics: form.has_vocals === 'yes' ? form.lyrics.trim() : '',
          negative_prompt: form.negative_prompt.trim(),
        },
      })
      toast({ title: 'AI 音乐任务已提交', variant: 'success' })
      mutateTasks()
    } catch {
      toast({ title: '音乐任务提交失败', variant: 'destructive' })
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="space-y-6">
      <DashboardHero
        badge="音乐生成工作台"
        badgeIcon={<Music2 className="h-3.5 w-3.5 text-emerald-300" />}
        title="使用 AI 生成音乐"
        description="这里是独立的音乐项目列表与工作台，只展示音乐项目，不再和视频项目混用展示。"
        gradientClassName="from-slate-950 via-emerald-950 to-slate-900"
        actions={
          <Button asChild className="bg-white text-slate-900 hover:bg-slate-100"><Link href={PROJECT_MEDIA_META.music.createHref}><Plus className="mr-1.5 h-4 w-4" />{PROJECT_MEDIA_META.music.createLabel}</Link></Button>
        }
        stats={[
          {
            icon: <Music2 className="h-4 w-4 text-emerald-300" />,
            label: '音乐项目',
            value: projects.length,
            description: '已进入独立音乐工作流的项目数',
          },
          {
            icon: <AudioLines className="h-4 w-4 text-cyan-300" />,
            label: '任务总数',
            value: visibleTasks.length,
            description: '当前筛选范围内的音乐生成任务',
          },
          {
            icon: <Sparkles className="h-4 w-4 text-violet-300" />,
            label: '运行中',
            value: counts.running ?? 0,
            description: '正在生成中的音乐任务',
          },
          {
            icon: <Mic2 className="h-4 w-4 text-amber-300" />,
            label: '已完成',
            value: counts.succeeded ?? 0,
            description: '可继续筛选、复用与追踪结果',
          },
        ]}
      />

      <div className="rounded-[24px] border border-surface-200 bg-white/85 p-4 shadow-sm backdrop-blur">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h2 className="text-base font-semibold text-surface-900">{PROJECT_MEDIA_META.music.listTitle}</h2>
            <p className="text-sm text-surface-500">共 {projects.length} {PROJECT_MEDIA_META.music.countLabel}，按项目维度管理音乐创作。</p>
          </div>
          {project ? (
            <div className="rounded-lg bg-emerald-50 px-3 py-2 text-xs text-emerald-700">
              当前音乐项目：<span className="font-medium">{project.title}</span>
            </div>
          ) : null}
        </div>

        {!projectsLoading && projects.length > 0 ? (
          <div className="mt-4 grid gap-3 md:grid-cols-2 xl:grid-cols-3">
            {projects.map((item) => {
              const active = String(item.id) === selectedProjectId
              return (
                <button
                  key={item.id}
                  type="button"
                  onClick={() => setSelectedProjectId(String(item.id))}
                  className={`rounded-xl border p-4 text-left transition-colors ${
                    active
                      ? 'border-emerald-300 bg-emerald-50'
                      : 'border-surface-200 bg-surface-50 hover:border-surface-300 hover:bg-white'
                  }`}
                >
                  <div className="flex items-center justify-between gap-2">
                    <p className="line-clamp-1 text-sm font-medium text-surface-800">{item.title}</p>
                    {active ? <span className="rounded-full bg-emerald-100 px-2 py-0.5 text-[10px] text-emerald-700">当前</span> : null}
                  </div>
                  <p className="mt-2 line-clamp-2 text-xs leading-5 text-surface-500">
                    {item.description || '用于 AI 配乐、BGM、主题曲等音乐生成。'}
                  </p>
                </button>
              )
            })}
          </div>
        ) : null}
      </div>

      {!projectsLoading && projects.length === 0 ? (
        <DashboardEmptyState
          icon={<Music2 className="mx-auto mb-3 h-10 w-10 text-surface-300" />}
          title={PROJECT_MEDIA_META.music.emptyTitle}
          description={PROJECT_MEDIA_META.music.emptyDescription}
          action={
            <Button asChild>
              <Link href={PROJECT_MEDIA_META.music.createHref}>
                <Sparkles className="mr-1.5 h-4 w-4" />
                去创建音乐项目
              </Link>
            </Button>
          }
          innerClassName="flex flex-col items-center justify-center rounded-[24px] border border-dashed border-surface-200 bg-[radial-gradient(circle_at_top_left,_rgba(16,185,129,0.08),_transparent_30%),radial-gradient(circle_at_bottom_right,_rgba(6,182,212,0.08),_transparent_28%)] px-6 py-16 text-center"
        />
      ) : (
      <div className="grid gap-6 xl:grid-cols-[minmax(0,1.1fr)_380px]">
        <div className="space-y-6">
          <div className="rounded-xl border border-emerald-200 bg-gradient-to-r from-emerald-50 via-white to-cyan-50 p-4">
            <div className="flex flex-wrap items-center gap-3">
              <div className="min-w-[220px] flex-1">
                <p className="mb-2 text-xs font-medium text-surface-500">快速切换音乐项目</p>
                <Select value={selectedProjectId} onValueChange={setSelectedProjectId}>
                  <SelectTrigger><SelectValue placeholder={projectsLoading ? '项目加载中...' : PROJECT_MEDIA_META.music.selectPlaceholder} /></SelectTrigger>
                  <SelectContent>{projects.map((item) => <SelectItem key={item.id} value={String(item.id)}>{item.title}</SelectItem>)}</SelectContent>
                </Select>
              </div>
              <div className="min-w-[220px] flex-1">
                <p className="mb-2 text-xs font-medium text-surface-500">音频模型</p>
                <Select value={selectedModelId} onValueChange={setSelectedModelId}>
                  <SelectTrigger><SelectValue placeholder={modelsLoading ? '模型加载中...' : '请选择音频模型'} /></SelectTrigger>
                  <SelectContent>{audioModels.map((model) => <SelectItem key={model.id} value={String(model.id)}>{model.name}</SelectItem>)}</SelectContent>
                </Select>
              </div>
            </div>

            <div className="mt-4 flex flex-wrap gap-2">{MUSIC_PRESETS.map((preset) => <Button key={preset.label} size="sm" variant="outline" onClick={() => applyPreset(preset.form)}>{preset.label}</Button>)}</div>
            {selectedModel ? <div className="mt-4 rounded-lg border border-emerald-100 bg-white/80 px-3 py-2 text-xs text-emerald-700">当前模型：<span className="font-medium">{selectedModel.name}</span>{selectedModel.description ? <span className="ml-2 text-emerald-600">{selectedModel.description}</span> : null}</div> : null}
          </div>

          <div className="rounded-xl border border-surface-200 bg-white p-4 shadow-sm">
            <div className="grid gap-4 md:grid-cols-2">
              <div><p className="mb-2 text-xs font-medium text-surface-500">音乐标题</p><Input value={form.title} onChange={(e) => setForm((prev) => ({ ...prev, title: e.target.value }))} placeholder="例如：西游主题战斗配乐" /></div>
              <div><p className="mb-2 text-xs font-medium text-surface-500">目标时长（秒）</p><Input value={form.duration_sec} onChange={(e) => setForm((prev) => ({ ...prev, duration_sec: e.target.value }))} placeholder="45" /></div>
              <div><p className="mb-2 text-xs font-medium text-surface-500">情绪 / Mood</p><Input value={form.mood} onChange={(e) => setForm((prev) => ({ ...prev, mood: e.target.value }))} placeholder="热血、史诗、悬疑、空灵..." /></div>
              <div><p className="mb-2 text-xs font-medium text-surface-500">乐器编制</p><Input value={form.instruments} onChange={(e) => setForm((prev) => ({ ...prev, instruments: e.target.value }))} placeholder="弦乐、钢琴、电子鼓、笛子..." /></div>
            </div>

            <div className="mt-4"><p className="mb-2 text-xs font-medium text-surface-500">音乐提示词</p><Textarea value={form.prompt} onChange={(e) => setForm((prev) => ({ ...prev, prompt: e.target.value }))} rows={5} placeholder="描述这段音乐要服务的剧情、节奏、高潮、参考风格与场景用途。" /></div>
            <div className="mt-4 grid gap-4 md:grid-cols-2">
              <div>
                <p className="mb-2 text-xs font-medium text-surface-500">是否带人声</p>
                <Select value={form.has_vocals} onValueChange={(value) => setForm((prev) => ({ ...prev, has_vocals: value }))}>
                  <SelectTrigger><SelectValue placeholder="选择是否带人声" /></SelectTrigger>
                  <SelectContent><SelectItem value="no">纯音乐 / BGM</SelectItem><SelectItem value="yes">带歌词 / 主题曲</SelectItem></SelectContent>
                </Select>
              </div>
              <div><p className="mb-2 text-xs font-medium text-surface-500">反向提示词</p><Input value={form.negative_prompt} onChange={(e) => setForm((prev) => ({ ...prev, negative_prompt: e.target.value }))} placeholder="例如：不要过于欢快、不要儿童感" /></div>
            </div>
            {form.has_vocals === 'yes' ? <div className="mt-4"><p className="mb-2 text-xs font-medium text-surface-500">歌词 / Hook</p><Textarea value={form.lyrics} onChange={(e) => setForm((prev) => ({ ...prev, lyrics: e.target.value }))} rows={4} placeholder="输入主歌、副歌或希望 AI 围绕其创作的歌词方向。" /></div> : null}

            <div className="mt-5 flex flex-wrap items-center justify-between gap-3">
              <div className="rounded-lg bg-surface-50 px-3 py-2 text-xs text-surface-500">{project ? <span>当前音乐项目：<span className="font-medium text-surface-700">{project.title}</span></span> : '请先选择音乐项目'}</div>
              <Button onClick={handleSubmit} disabled={submitting || !project || !selectedModel}>{submitting ? <Loader2 className="mr-1.5 h-4 w-4 animate-spin" /> : <Sparkles className="mr-1.5 h-4 w-4" />}提交 AI 音乐任务</Button>
            </div>
          </div>
        </div>

        <div className="space-y-4">
          <div className="rounded-xl border border-surface-200 bg-white p-4 shadow-sm">
            <div className="flex items-center gap-2 text-sm font-semibold text-surface-800"><AudioLines className="h-4 w-4 text-emerald-600" />音乐任务概览</div>
            <div className="mt-4 grid gap-3 sm:grid-cols-2 xl:grid-cols-1">
              {[{ label: '待处理', key: 'pending', cls: 'bg-surface-100 text-surface-700' }, { label: '运行中', key: 'running', cls: 'bg-blue-100 text-blue-700' }, { label: '已完成', key: 'succeeded', cls: 'bg-green-100 text-green-700' }, { label: '失败', key: 'failed', cls: 'bg-red-100 text-red-700' }].map((item) => <div key={item.key} className={`rounded-lg px-3 py-2 text-sm ${item.cls}`}><div className="flex items-center justify-between"><span>{item.label}</span><span className="text-lg font-bold">{counts[item.key] ?? 0}</span></div></div>)}
            </div>
          </div>

          <div className="rounded-xl border border-surface-200 bg-white p-4 shadow-sm">
            <div className="flex items-center gap-2 text-sm font-semibold text-surface-800"><Mic2 className="h-4 w-4 text-violet-600" />任务队列</div>
            <div className="mt-4 space-y-3">
              {visibleTasks.length === 0 ? <div className="rounded-lg border border-dashed border-surface-200 bg-surface-50 px-4 py-8 text-center text-sm text-surface-400">当前音乐项目暂无任务</div> : visibleTasks.map((task) => <Card key={task.id} className="border-surface-200"><CardContent className="space-y-3 p-4"><div className="flex items-start justify-between gap-3"><div className="min-w-0"><div className="flex items-center gap-2"><span className="text-sm font-semibold text-surface-800">{task.payload?.title || `音乐任务 #${task.id}`}</span><Badge variant={statusVariant[task.status]}>{statusLabel[task.status]}</Badge></div><p className="mt-1 text-xs text-surface-500">{task.payload?.project_title || '未标记音乐项目'} · {task.payload?.model_name || '未标记模型'}</p></div><span className="text-[11px] text-surface-400">{new Date(task.created_at).toLocaleString('zh-CN', { month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit' })}</span></div>{task.payload?.prompt ? <p className="line-clamp-3 text-xs leading-5 text-surface-600">{task.payload.prompt}</p> : null}<div className="flex flex-wrap gap-2 text-[11px] text-surface-500">{task.payload?.mood ? <span className="rounded bg-surface-100 px-2 py-1">情绪：{task.payload.mood}</span> : null}{task.payload?.instruments ? <span className="rounded bg-surface-100 px-2 py-1">乐器：{task.payload.instruments}</span> : null}{task.payload?.duration_sec ? <span className="rounded bg-surface-100 px-2 py-1">时长：{task.payload.duration_sec}s</span> : null}{task.payload?.has_vocals ? <span className="rounded bg-violet-100 px-2 py-1 text-violet-700">带人声</span> : <span className="rounded bg-emerald-100 px-2 py-1 text-emerald-700">纯音乐</span>}</div><div className="flex items-center gap-2"><Progress value={task.progress ?? (task.status === 'succeeded' ? 100 : task.status === 'running' ? 45 : 5)} className="h-2 flex-1" /><span className="text-[11px] text-surface-400">{task.progress ?? (task.status === 'succeeded' ? 100 : task.status === 'running' ? 45 : 5)}%</span></div>{task.error_msg ? <p className="text-xs text-red-500">{task.error_msg}</p> : null}</CardContent></Card>)}
            </div>
          </div>

          <div className="rounded-xl border border-amber-200 bg-amber-50 p-4 text-xs leading-5 text-amber-800"><div className="flex items-center gap-2 font-medium"><Clock3 className="h-4 w-4" />当前接入方式</div><p className="mt-2">本页已接入音频模型选择与 <code>music_generate</code> 任务提交流程，适合作为 AI 音乐统一入口。音乐任务会进入系统任务队列，后续接入专门的音乐 worker 后即可自动产出音频结果。</p></div>
        </div>
      </div>
      )}
    </div>
  )
}
