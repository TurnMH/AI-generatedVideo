'use client'

import { useMemo, useState } from 'react'
import Link from 'next/link'
import useSWR from 'swr'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Label } from '@/components/ui/label'
import { useToast } from '@/components/ui/toast'
import { modelAPI, projectAPI, videoAPI, type Episode, type Model, type Project, type VideoTaskDetailResponse } from '@/lib/api'

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

type ProjectPayload = {
  code?: number
  data?: Project
}

type EpisodesPayload = {
  code?: number
  data?: Episode[]
}

type ProgressPayload = {
  code?: number
  data?: Project['progress']
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

const unwrap = <T,>(payload: unknown): T | null => {
  if (!payload || typeof payload !== 'object') return null
  const maybe = payload as { data?: T }
  return maybe.data ?? (payload as T)
}

export default function AdVideoWorkbenchPage() {
  const { toast } = useToast()

  const [workflowForm, setWorkflowForm] = useState({
    title: '口播广告工作台项目',
    description: '在广告工作台内独立创建、生成、后处理，不再依赖单个项目页。',
    targetEpisodes: '9',
    stylePreset: 'realistic',
    aspectRatio: '16:9',
    resolution: '1080p',
    duration: '10',
    textModelId: 'default',
    imageModelId: 'default',
    videoModelId: 'default',
    scriptText: '',
  })
  const [workflowProjectId, setWorkflowProjectId] = useState<number | null>(null)
  const [creatingProject, setCreatingProject] = useState(false)
  const [uploadingScript, setUploadingScript] = useState(false)
  const [startingFlow, setStartingFlow] = useState(false)

  const [idsText, setIdsText] = useState(DEFAULT_IDS)
  const [orderedIds, setOrderedIds] = useState<number[]>(parseIds(DEFAULT_IDS))
  const [busy, setBusy] = useState(false)
  const [busyTaskId, setBusyTaskId] = useState<number | null>(null)
  const [resultUrl, setResultUrl] = useState('')
  const [resultTaskId, setResultTaskId] = useState<number | null>(null)
  const idsKey = useMemo(() => orderedIds.join(','), [orderedIds])

  const { data: workflowProjectData, mutate: mutateProject } = useSWR(
    workflowProjectId ? ['ad-video-project', workflowProjectId] : null,
    async () => {
      const res = await projectAPI.get(workflowProjectId as number)
      return unwrap<Project>((res as { data?: ProjectPayload }).data)
    },
    { revalidateOnFocus: true },
  )

  const { data: workflowEpisodesData, mutate: mutateEpisodes } = useSWR(
    workflowProjectId ? ['ad-video-episodes', workflowProjectId] : null,
    async () => {
      const res = await projectAPI.listEpisodes(workflowProjectId as number)
      const payload = (res as { data?: EpisodesPayload }).data
      return unwrap<Episode[]>(payload) ?? []
    },
    { refreshInterval: 5000, revalidateOnFocus: true },
  )

  const { data: workflowProgressData, mutate: mutateProgress } = useSWR(
    workflowProjectId ? ['ad-video-progress', workflowProjectId] : null,
    async () => {
      const res = await projectAPI.getProgress(workflowProjectId as number)
      const payload = (res as { data?: ProgressPayload }).data
      return unwrap<Project['progress']>(payload)
    },
    { refreshInterval: 5000, revalidateOnFocus: true },
  )

  const { data: workflowModelData } = useSWR(['ad-video-models'], async () => {
    const [textRes, imageRes, videoRes] = await Promise.all([
      modelAPI.list({ type: 'llm', enabled: 'true', sort_by: 'priority' }),
      modelAPI.list({ type: 'image', enabled: 'true', sort_by: 'priority' }),
      modelAPI.list({ type: 'video', enabled: 'true', sort_by: 'priority' }),
    ])
    const normalize = (payload: unknown): Model[] => {
      if (!payload || typeof payload !== 'object') return []
      const obj = payload as { data?: unknown }
      const data = obj.data
      if (Array.isArray(data)) return data as Model[]
      if (data && typeof data === 'object' && Array.isArray((data as { items?: Model[] }).items)) {
        return (data as { items?: Model[] }).items || []
      }
      if (Array.isArray((payload as { items?: Model[] }).items)) {
        return (payload as { items?: Model[] }).items || []
      }
      return []
    }
    return {
      text: normalize((textRes as { data?: unknown }).data),
      image: normalize((imageRes as { data?: unknown }).data),
      video: normalize((videoRes as { data?: unknown }).data),
    }
  }, { revalidateOnFocus: true })

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
  const workflowEpisodes = workflowEpisodesData || []
  const workflowProgress = workflowProgressData || null
  const workflowProject = workflowProjectData || null
  const textModels = workflowModelData?.text || []
  const imageModels = workflowModelData?.image || []
  const videoModels = workflowModelData?.video || []

  const createWorkflowProject = async () => {
    if (!workflowForm.title.trim()) {
      toast({ title: '请先填写广告工作项目标题', variant: 'destructive' })
      return
    }
    setCreatingProject(true)
    try {
      const parseModelId = (value: string) => {
        const num = Number(value)
        return Number.isFinite(num) && num > 0 ? num : undefined
      }
      const selectedVideoModel = videoModels.find((item) => item.id === parseModelId(workflowForm.videoModelId))
      const res = await projectAPI.create({
        title: workflowForm.title.trim(),
        description: workflowForm.description.trim(),
        project_type: 'video',
        target_episodes: Math.max(1, Number(workflowForm.targetEpisodes) || 1),
        text_model_id: parseModelId(workflowForm.textModelId),
        image_model_id: parseModelId(workflowForm.imageModelId),
        video_model_id: parseModelId(workflowForm.videoModelId),
        enable_subtitle: true,
        enable_dubbing: true,
        video_mode: 'api_generation',
        mode: 'script',
        style_tags: ['ad-workbench'],
        storyboard_config: {
          aspect_ratio: workflowForm.aspectRatio,
          resolution: workflowForm.resolution,
          duration: Math.max(1, Number(workflowForm.duration) || 10),
          video_mode: 'api_generation',
          style_preset: workflowForm.stylePreset,
          motion_mode: 'gentle',
          ...(selectedVideoModel?.model_key ? { video_model: selectedVideoModel.model_key } : {}),
        },
      })
      const project = unwrap<Project>((res as { data?: ProjectPayload }).data)
      if (!project?.id) {
        throw new Error('创建广告工作项目失败：未拿到 project id')
      }
      setWorkflowProjectId(project.id)
      toast({ title: `广告工作项目已创建 #${project.id}`, variant: 'success' })
      await mutateProject()
    } catch (error) {
      toast({ title: error instanceof Error ? error.message : '创建广告工作项目失败', variant: 'destructive' })
    } finally {
      setCreatingProject(false)
    }
  }

  const uploadWorkflowScript = async () => {
    if (!workflowProjectId) {
      toast({ title: '请先创建广告工作项目', variant: 'destructive' })
      return
    }
    const text = workflowForm.scriptText.trim()
    if (!text) {
      toast({ title: '请先粘贴广告脚本文案', variant: 'destructive' })
      return
    }
    setUploadingScript(true)
    try {
      const filenameBase = workflowForm.title.trim() || `ad-project-${workflowProjectId}`
      const file = new File([text], `${filenameBase}.txt`, { type: 'text/plain' })
      await projectAPI.uploadScript(workflowProjectId, file)
      toast({ title: '广告脚本已上传到当前工作项目', variant: 'success' })
      await mutateProject()
      await mutateProgress()
    } catch (error) {
      toast({ title: error instanceof Error ? error.message : '上传广告脚本失败', variant: 'destructive' })
    } finally {
      setUploadingScript(false)
    }
  }

  const startWorkflowGeneration = async () => {
    if (!workflowProjectId) {
      toast({ title: '请先创建广告工作项目', variant: 'destructive' })
      return
    }
    setStartingFlow(true)
    try {
      await projectAPI.generateEpisodes(workflowProjectId, undefined, { force: true, autoStoryboard: true })
      toast({ title: '已启动广告工作流基础生成（分集 + 自动分镜）', variant: 'success' })
      await mutateEpisodes()
      await mutateProgress()
      await mutateProject()
    } catch (error) {
      toast({ title: error instanceof Error ? error.message : '启动广告工作流失败', variant: 'destructive' })
    } finally {
      setStartingFlow(false)
    }
  }

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
          <p className="mt-2 text-sm text-slate-300">
            从这里独立完成广告创建、基础生成、后处理与整片合成。单个项目页后续只作为底层流程测试入口。
          </p>
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
          <CardTitle className="text-white">一、广告工作流主入口（第一阶段骨架）</CardTitle>
        </CardHeader>
        <CardContent className="space-y-5">
          <div className="grid gap-4 md:grid-cols-2">
            <div className="space-y-2">
              <Label className="text-slate-200">广告工作项目标题</Label>
              <Input
                value={workflowForm.title}
                onChange={(e) => setWorkflowForm((prev) => ({ ...prev, title: e.target.value }))}
                placeholder="例如：李恩泽口播广告 0527"
              />
            </div>
            <div className="space-y-2">
              <Label className="text-slate-200">目标片段数 / 分集数</Label>
              <Input
                type="number"
                min={1}
                value={workflowForm.targetEpisodes}
                onChange={(e) => setWorkflowForm((prev) => ({ ...prev, targetEpisodes: e.target.value }))}
              />
            </div>
            <div className="space-y-2 md:col-span-2">
              <Label className="text-slate-200">项目说明</Label>
              <Textarea
                value={workflowForm.description}
                onChange={(e) => setWorkflowForm((prev) => ({ ...prev, description: e.target.value }))}
                className="min-h-[88px]"
                placeholder="说明这次广告工作台的目标、风格、产品或人物设定。"
              />
            </div>
            <div className="space-y-2">
              <Label className="text-slate-200">画面风格</Label>
              <select
                value={workflowForm.stylePreset}
                onChange={(e) => setWorkflowForm((prev) => ({ ...prev, stylePreset: e.target.value }))}
                className="flex h-10 w-full rounded-xl border border-surface-200 bg-white px-3 py-2 text-sm text-surface-900"
              >
                <option value="realistic">真实环境 / 写实风格</option>
                <option value="anime">动漫风格</option>
              </select>
            </div>
            <div className="space-y-2">
              <Label className="text-slate-200">画幅比例</Label>
              <select
                value={workflowForm.aspectRatio}
                onChange={(e) => setWorkflowForm((prev) => ({ ...prev, aspectRatio: e.target.value }))}
                className="flex h-10 w-full rounded-xl border border-surface-200 bg-white px-3 py-2 text-sm text-surface-900"
              >
                <option value="16:9">16:9 横屏</option>
                <option value="9:16">9:16 竖屏</option>
                <option value="1:1">1:1 方图</option>
              </select>
            </div>
            <div className="space-y-2">
              <Label className="text-slate-200">分辨率</Label>
              <select
                value={workflowForm.resolution}
                onChange={(e) => setWorkflowForm((prev) => ({ ...prev, resolution: e.target.value }))}
                className="flex h-10 w-full rounded-xl border border-surface-200 bg-white px-3 py-2 text-sm text-surface-900"
              >
                <option value="1080p">1080p</option>
                <option value="720p">720p</option>
              </select>
            </div>
            <div className="space-y-2">
              <Label className="text-slate-200">默认片段时长（秒）</Label>
              <Input
                type="number"
                min={1}
                value={workflowForm.duration}
                onChange={(e) => setWorkflowForm((prev) => ({ ...prev, duration: e.target.value }))}
              />
            </div>
            <div className="space-y-2">
              <Label className="text-slate-200">文本模型</Label>
              <select
                value={workflowForm.textModelId}
                onChange={(e) => setWorkflowForm((prev) => ({ ...prev, textModelId: e.target.value }))}
                className="flex h-10 w-full rounded-xl border border-surface-200 bg-white px-3 py-2 text-sm text-surface-900"
              >
                <option value="default">系统默认</option>
                {textModels.map((model) => (
                  <option key={model.id} value={String(model.id)}>
                    {model.name} · {model.model_key}
                  </option>
                ))}
              </select>
            </div>
            <div className="space-y-2">
              <Label className="text-slate-200">图片模型</Label>
              <select
                value={workflowForm.imageModelId}
                onChange={(e) => setWorkflowForm((prev) => ({ ...prev, imageModelId: e.target.value }))}
                className="flex h-10 w-full rounded-xl border border-surface-200 bg-white px-3 py-2 text-sm text-surface-900"
              >
                <option value="default">系统默认</option>
                {imageModels.map((model) => (
                  <option key={model.id} value={String(model.id)}>
                    {model.name} · {model.model_key}
                  </option>
                ))}
              </select>
            </div>
            <div className="space-y-2 md:col-span-2">
              <Label className="text-slate-200">视频模型</Label>
              <select
                value={workflowForm.videoModelId}
                onChange={(e) => setWorkflowForm((prev) => ({ ...prev, videoModelId: e.target.value }))}
                className="flex h-10 w-full rounded-xl border border-surface-200 bg-white px-3 py-2 text-sm text-surface-900"
              >
                <option value="default">系统默认</option>
                {videoModels.map((model) => (
                  <option key={model.id} value={String(model.id)}>
                    {model.name} · {model.model_key}
                  </option>
                ))}
              </select>
            </div>
            <div className="space-y-2 md:col-span-2">
              <Label className="text-slate-200">广告脚本 / 口播文案</Label>
              <Textarea
                value={workflowForm.scriptText}
                onChange={(e) => setWorkflowForm((prev) => ({ ...prev, scriptText: e.target.value }))}
                className="min-h-[220px]"
                placeholder="把整套广告脚本直接贴在这里。第一阶段先复用项目脚本上传 + 分集生成链，让广告工作台不再依赖先去单个项目页手工起流程。"
              />
            </div>
          </div>

          <div className="flex flex-wrap gap-2">
            <Button disabled={creatingProject} onClick={createWorkflowProject}>
              {creatingProject ? '创建中…' : '1）创建广告工作项目'}
            </Button>
            <Button variant="outline" disabled={!workflowProjectId || uploadingScript} onClick={uploadWorkflowScript}>
              {uploadingScript ? '上传中…' : '2）上传当前脚本'}
            </Button>
            <Button variant="outline" disabled={!workflowProjectId || startingFlow} onClick={startWorkflowGeneration}>
              {startingFlow ? '启动中…' : '3）启动基础生成（分集 + 自动分镜）'}
            </Button>
            {workflowProjectId && (
              <Button variant="outline" asChild>
                <Link href={`/projects/${workflowProjectId}`}>打开工作项目</Link>
              </Button>
            )}
          </div>

          <div className="rounded-xl border border-white/10 bg-black/20 p-4 text-sm text-slate-300">
            <div>当前第一阶段先复用已有项目链做底座，但入口已经前置到广告工作台内部：</div>
            <ul className="mt-2 list-disc space-y-1 pl-5 text-xs text-slate-400">
              <li>在这里直接创建广告项目载体，而不是先去项目页手工新建。</li>
              <li>在这里直接粘贴脚本并上传到该广告项目。</li>
              <li>在这里直接启动分集 + 自动分镜基础流程。</li>
              <li>后续再继续把资产生成、视频生成、排序、合成彻底前移到本页。</li>
            </ul>
          </div>
        </CardContent>
      </Card>

      <Card className="border-white/10 bg-slate-900/60 text-slate-100">
        <CardHeader>
          <CardTitle className="text-white">当前广告工作项目状态</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3 text-sm text-slate-300">
          {workflowProject ? (
            <>
              <div>项目：#{workflowProject.id} · {workflowProject.title}</div>
              <div>状态：{workflowProject.status || '-'} · 目标片段数：{workflowProject.target_episodes ?? '-'}</div>
              <div>
                当前进度：
                {workflowProgress?.stage || workflowProgress?.phase_label || workflowProgress?.message || '暂无'}
              </div>
              {workflowEpisodes.length > 0 ? (
                <div className="space-y-2">
                  <div className="text-slate-200">已生成的分集 / 片段载体</div>
                  {workflowEpisodes.map((episode) => (
                    <div key={episode.id} className="rounded-lg border border-white/10 bg-black/20 p-3">
                      <div>episode #{episode.episode_number} · {episode.title || '未命名片段'} · {episode.status}</div>
                      <div className="mt-1 text-xs text-slate-400 line-clamp-2">{episode.summary || episode.script_excerpt || '暂无摘要'}</div>
                    </div>
                  ))}
                </div>
              ) : (
                <div className="text-xs text-slate-400">当前还没有分集记录；启动基础生成后这里会开始出现内容。</div>
              )}
            </>
          ) : (
            <div className="text-xs text-slate-400">还没创建广告工作项目。先在上面完成“创建项目 → 上传脚本 → 启动基础生成”。</div>
          )}
        </CardContent>
      </Card>

      <Card className="border-white/10 bg-slate-900/60 text-slate-100">
        <CardHeader>
          <CardTitle className="text-white">二、广告后处理与整片合成</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3">
          <Input value={idsText} onChange={(e) => setIdsText(e.target.value)} placeholder="150,151,152..." />
          <div className="flex flex-wrap gap-2">
            <Button onClick={loadIds}>加载任务</Button>
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
            <CardTitle className="text-white">合成结果</CardTitle>
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
