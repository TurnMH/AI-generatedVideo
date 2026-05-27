'use client'

import { useEffect, useMemo, useState } from 'react'
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

type ModelParamValue = { value: string; label: string }
type ModelParamOption = { key: string; label: string; default: string; values?: ModelParamValue[] }
type VideoModelStatus = { key: string; available: boolean; native_audio?: boolean; params?: ModelParamOption[] }

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

const uniqueStrings = (values: string[]) => Array.from(new Set(values.map((item) => item.trim()).filter(Boolean)))

const extractStringArray = (config: Record<string, unknown> | undefined, keys: string[]) => {
  if (!config) return []
  for (const key of keys) {
    const value = config[key]
    if (Array.isArray(value)) {
      const items = uniqueStrings(value.map((item) => String(item)))
      if (items.length > 0) return items
    }
    if (typeof value === 'string') {
      const items = uniqueStrings(value.split(/[\s,|/]+/))
      if (items.length > 0) return items
    }
  }
  return []
}

const extractParamOptionValues = (status: VideoModelStatus | null | undefined, key: string) => {
  const values = status?.params?.find((item) => item.key === key)?.values || []
  return uniqueStrings(values.map((item) => String(item.value || '').trim()).filter(Boolean))
}

export default function AdVideoWorkbenchPage() {
  const { toast } = useToast()

  const [workflowForm, setWorkflowForm] = useState({
    title: '口播广告工作台项目',
    description: '在广告工作台内独立创建、生成、后处理，不再依赖单个项目页。',
    stylePreset: 'live-action-short',
    aspectRatio: '',
    resolution: '',
    duration: '',
    generateAudio: true,
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
    const [textRes, imageRes, videoRes, videoStatusRes] = await Promise.all([
      modelAPI.list({ type: 'llm', enabled: 'true', sort_by: 'priority' }),
      modelAPI.list({ type: 'image', enabled: 'true', sort_by: 'priority' }),
      modelAPI.list({ type: 'video', enabled: 'true', sort_by: 'priority' }),
      videoAPI.modelStatus(),
    ])
    const normalize = (payload: unknown): Model[] => {
      if (!payload || typeof payload !== 'object') return []
      const root = payload as { data?: unknown; items?: Model[] }
      if (Array.isArray(root.data)) return root.data as Model[]
      if (root.data && typeof root.data === 'object' && Array.isArray((root.data as { items?: Model[] }).items)) {
        return (root.data as { items?: Model[] }).items || []
      }
      if (Array.isArray(root.items)) return root.items || []
      return []
    }
    const statusPayload = (videoStatusRes as { data?: { models?: VideoModelStatus[] } })?.data
    return {
      text: normalize(textRes),
      image: normalize(imageRes),
      video: normalize(videoRes),
      videoStatus: Array.isArray(statusPayload?.models) ? statusPayload.models : [],
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
  const videoModelStatuses = workflowModelData?.videoStatus || []
  const selectedVideoModel = useMemo(() => {
    const selectedId = Number(workflowForm.videoModelId)
    if (!Number.isFinite(selectedId) || selectedId <= 0) return null
    return videoModels.find((item) => item.id === selectedId) || null
  }, [videoModels, workflowForm.videoModelId])
  const selectedVideoStatus = useMemo(() => {
    const modelKey = String(selectedVideoModel?.model_key || '').trim()
    if (!modelKey) return null
    return videoModelStatuses.find((item) => item.key === modelKey) || null
  }, [selectedVideoModel, videoModelStatuses])
  const selectedVideoParamEntries = useMemo(() => {
    const cfg = selectedVideoModel?.config
    if (!cfg || typeof cfg !== 'object') return [] as Array<{ key: string; value: string }>
    const entries = Object.entries(cfg)
      .filter(([, value]) => value !== null && value !== undefined && value !== '')
      .map(([key, value]) => ({
        key,
        value: Array.isArray(value) ? value.join(' / ') : typeof value === 'object' ? JSON.stringify(value) : String(value),
      }))
    return entries.slice(0, 12)
  }, [selectedVideoModel])
  const selectedVideoConfig = (selectedVideoModel?.config && typeof selectedVideoModel.config === 'object'
    ? selectedVideoModel.config
    : undefined) as Record<string, unknown> | undefined
  const aspectRatioOptions = useMemo(() => extractParamOptionValues(selectedVideoStatus, 'aspect_ratio'), [selectedVideoStatus])
  const resolutionOptions = useMemo(() => extractParamOptionValues(selectedVideoStatus, 'resolution'), [selectedVideoStatus])
  const durationOptions = useMemo(() => extractParamOptionValues(selectedVideoStatus, 'duration'), [selectedVideoStatus])
  const nativeAudioSupported = Boolean(selectedVideoStatus?.native_audio)
  const autoSplitInfo = workflowProgress?.auto_split || null

  useEffect(() => {
    if (!selectedVideoModel) return
    if (!aspectRatioOptions.includes(workflowForm.aspectRatio)) {
      setWorkflowForm((prev) => ({ ...prev, aspectRatio: aspectRatioOptions[0] || '' }))
    }
  }, [selectedVideoModel, aspectRatioOptions, workflowForm.aspectRatio])

  useEffect(() => {
    if (!selectedVideoModel) return
    if (!resolutionOptions.includes(workflowForm.resolution)) {
      setWorkflowForm((prev) => ({ ...prev, resolution: resolutionOptions[0] || '' }))
    }
  }, [selectedVideoModel, resolutionOptions, workflowForm.resolution])

  useEffect(() => {
    if (!selectedVideoModel) return
    if (!durationOptions.includes(workflowForm.duration)) {
      setWorkflowForm((prev) => ({ ...prev, duration: durationOptions[0] || '' }))
    }
  }, [selectedVideoModel, durationOptions, workflowForm.duration])

  useEffect(() => {
    if (!nativeAudioSupported && workflowForm.generateAudio) {
      setWorkflowForm((prev) => ({ ...prev, generateAudio: false }))
    }
  }, [nativeAudioSupported, workflowForm.generateAudio])

  const createWorkflowProject = async () => {
    if (!workflowForm.title.trim()) {
      toast({ title: '请先填写广告工作项目标题', variant: 'destructive' })
      return
    }
    if (!selectedVideoModel) {
      toast({ title: '请先选择视频模型', variant: 'destructive' })
      return
    }
    if (!workflowForm.aspectRatio || !aspectRatioOptions.includes(workflowForm.aspectRatio)) {
      toast({ title: '当前模型没有可用的画幅比例参数，请先切换模型', variant: 'destructive' })
      return
    }
    if (!workflowForm.resolution || !resolutionOptions.includes(workflowForm.resolution)) {
      toast({ title: '当前模型没有可用的分辨率参数，请先切换模型', variant: 'destructive' })
      return
    }
    if (!workflowForm.duration || !durationOptions.includes(workflowForm.duration)) {
      toast({ title: '当前模型没有可用的时长参数，请先切换模型', variant: 'destructive' })
      return
    }
    setCreatingProject(true)
    try {
      const parseModelId = (value: string) => {
        const num = Number(value)
        return Number.isFinite(num) && num > 0 ? num : undefined
      }
      const res = await projectAPI.create({
        title: workflowForm.title.trim(),
        description: workflowForm.description.trim(),
        project_type: 'video',
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
          duration: Number(workflowForm.duration),
          video_mode: 'api_generation',
          style_preset: workflowForm.stylePreset,
          motion_mode: 'gentle',
          generate_audio: nativeAudioSupported ? workflowForm.generateAudio : false,
          auto_split_after_optimization: true,
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
                <option value="live-action-short">真实环境 / 写实短视频风格</option>
                <option value="live-action-film">真实环境 / 电影感风格</option>
                <option value="anime-2d">动漫风格（2D）</option>
                <option value="anime-3d">动漫风格（3D）</option>
              </select>
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
            <div className="space-y-3 rounded-xl border border-white/10 bg-black/20 p-4 md:col-span-2">
              <div className="text-sm font-medium text-white">视频模型参数</div>
              {selectedVideoModel ? (
                <div className="space-y-4 text-xs text-slate-300">
                  <div className="rounded-lg border border-white/10 bg-slate-950/40 px-3 py-2">
                    <div className="text-[11px] text-slate-500">当前模型</div>
                    <div className="mt-1 break-all text-white">{selectedVideoModel.name} · {selectedVideoModel.model_key}</div>
                  </div>

                  <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
                    <div className="space-y-2">
                      <Label className="text-slate-200">画幅比例</Label>
                      <select
                        value={workflowForm.aspectRatio}
                        onChange={(e) => setWorkflowForm((prev) => ({ ...prev, aspectRatio: e.target.value }))}
                        disabled={aspectRatioOptions.length === 0}
                        className="flex h-10 w-full rounded-xl border border-surface-200 bg-white px-3 py-2 text-sm text-surface-900 disabled:cursor-not-allowed disabled:bg-slate-100"
                      >
                        {aspectRatioOptions.length > 0 ? (
                          aspectRatioOptions.map((ratio) => (
                            <option key={ratio} value={ratio}>{ratio}</option>
                          ))
                        ) : (
                          <option value="">当前模型未声明画幅比例</option>
                        )}
                      </select>
                      <div className="text-[11px] text-slate-500">只展示当前模型在 model-status 中真实声明的画幅比例；未声明就不允许乱填。</div>
                    </div>

                    <div className="space-y-2">
                      <Label className="text-slate-200">分辨率</Label>
                      <select
                        value={workflowForm.resolution}
                        onChange={(e) => setWorkflowForm((prev) => ({ ...prev, resolution: e.target.value }))}
                        disabled={resolutionOptions.length === 0}
                        className="flex h-10 w-full rounded-xl border border-surface-200 bg-white px-3 py-2 text-sm text-surface-900 disabled:cursor-not-allowed disabled:bg-slate-100"
                      >
                        {resolutionOptions.length > 0 ? (
                          resolutionOptions.map((resolution) => (
                            <option key={resolution} value={resolution}>{resolution}</option>
                          ))
                        ) : (
                          <option value="">当前模型未声明分辨率</option>
                        )}
                      </select>
                      <div className="text-[11px] text-slate-500">只展示当前模型在 model-status 中真实声明的分辨率；未声明就不允许乱填。</div>
                    </div>

                    <div className="space-y-2">
                      <Label className="text-slate-200">默认片段时长（秒）</Label>
                      <select
                        value={workflowForm.duration}
                        onChange={(e) => setWorkflowForm((prev) => ({ ...prev, duration: e.target.value }))}
                        disabled={durationOptions.length === 0}
                        className="flex h-10 w-full rounded-xl border border-surface-200 bg-white px-3 py-2 text-sm text-surface-900 disabled:cursor-not-allowed disabled:bg-slate-100"
                      >
                        {durationOptions.length > 0 ? (
                          durationOptions.map((duration) => (
                            <option key={duration} value={duration}>{duration} 秒</option>
                          ))
                        ) : (
                          <option value="">当前模型未声明时长</option>
                        )}
                      </select>
                      <div className="text-[11px] text-slate-500">只展示当前模型在 model-status 中真实声明的时长；未声明就不允许乱填，并且不会提交。</div>
                    </div>

                    <div className="space-y-2">
                      <Label className="text-slate-200">原生音频</Label>
                      {nativeAudioSupported ? (
                        <select
                          value={workflowForm.generateAudio ? 'on' : 'off'}
                          onChange={(e) => setWorkflowForm((prev) => ({ ...prev, generateAudio: e.target.value === 'on' }))}
                          className="flex h-10 w-full rounded-xl border border-surface-200 bg-white px-3 py-2 text-sm text-surface-900"
                        >
                          <option value="on">开启</option>
                          <option value="off">关闭</option>
                        </select>
                      ) : (
                        <Input value="当前模型不支持原生音频" disabled />
                      )}
                      <div className="text-[11px] text-slate-500">
                        {nativeAudioSupported
                          ? '按当前模型真实 native_audio 能力展示；创建项目时会真实写入 storyboard_config.generate_audio。'
                          : '当前模型未声明 native_audio，创建项目时会按关闭处理。'}
                      </div>
                    </div>
                  </div>

                  <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
                    <div className="rounded-lg border border-white/10 bg-slate-950/40 px-3 py-2">
                      <div className="text-[11px] text-slate-500">视频模式</div>
                      <div className="mt-1 text-white">{selectedVideoModel.video_mode || '未声明'}</div>
                    </div>
                    <div className="rounded-lg border border-white/10 bg-slate-950/40 px-3 py-2">
                      <div className="text-[11px] text-slate-500">最高分辨率</div>
                      <div className="mt-1 text-white">{selectedVideoModel.max_resolution || '未声明'}</div>
                    </div>
                    <div className="rounded-lg border border-white/10 bg-slate-950/40 px-3 py-2">
                      <div className="text-[11px] text-slate-500">原生音频能力</div>
                      <div className="mt-1 text-white">{nativeAudioSupported ? '支持' : '不支持 / 未声明'}</div>
                    </div>
                    <div className="rounded-lg border border-white/10 bg-slate-950/40 px-3 py-2 md:col-span-2 xl:col-span-2">
                      <div className="text-[11px] text-slate-500">支持时长</div>
                      <div className="mt-1 text-white">{durationOptions.length > 0 ? durationOptions.map((item) => `${item} 秒`).join(' / ') : '未声明，不可手填'}</div>
                    </div>
                    <div className="rounded-lg border border-white/10 bg-slate-950/40 px-3 py-2 md:col-span-2 xl:col-span-3">
                      <div className="text-[11px] text-slate-500">能力标签</div>
                      <div className="mt-2 flex flex-wrap gap-2">
                        {(selectedVideoModel.capability_tags?.length ? selectedVideoModel.capability_tags : ['暂无标签']).map((tag) => (
                          <span key={tag} className="rounded-md border border-cyan-400/20 bg-cyan-400/10 px-2 py-1 text-[11px] text-cyan-200">{tag}</span>
                        ))}
                      </div>
                    </div>
                    <div className="rounded-lg border border-white/10 bg-slate-950/40 px-3 py-2 md:col-span-2 xl:col-span-3">
                      <div className="text-[11px] text-slate-500">模型说明</div>
                      <div className="mt-1 leading-5 text-slate-300">{selectedVideoModel.description || '暂无模型说明'}</div>
                    </div>
                    <div className="rounded-lg border border-white/10 bg-slate-950/40 px-3 py-2 md:col-span-2 xl:col-span-3">
                      <div className="text-[11px] text-slate-500">模型原始附加参数</div>
                      {selectedVideoParamEntries.length > 0 ? (
                        <div className="mt-2 grid gap-2 md:grid-cols-2 xl:grid-cols-3">
                          {selectedVideoParamEntries.map((entry) => (
                            <div key={entry.key} className="rounded-md border border-white/10 px-2 py-2">
                              <div className="text-[11px] text-slate-500">{entry.key}</div>
                              <div className="mt-1 break-all text-slate-100">{entry.value}</div>
                            </div>
                          ))}
                        </div>
                      ) : (
                        <div className="mt-1 text-slate-300">当前模型没有返回额外参数配置。</div>
                      )}
                    </div>
                  </div>
                </div>
              ) : (
                <div className="text-xs text-slate-400">先选视频模型；选中后这里会直接出现可调整的模型参数。</div>
              )}
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
              <li>在这里直接启动“文案优化 → 自动分集 → 自动分镜”基础流程。</li>
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
              <div>状态：{workflowProject.status || '-'} · 当前已生成分集数：{workflowEpisodes.length || '-'}</div>
              <div>
                当前进度：
                {workflowProgress?.stage || workflowProgress?.phase_label || workflowProgress?.message || '暂无'}
              </div>
              {autoSplitInfo?.enabled && (
                <div className="space-y-3 rounded-lg border border-cyan-500/20 bg-cyan-500/10 p-3 text-xs text-cyan-100">
                  <div>
                    <div className="font-medium text-cyan-50">本次自动切分依据</div>
                    <div className="mt-1 break-all">
                      模型：{autoSplitInfo.video_model || selectedVideoModel?.model_key || '未记录'}
                      {' · '}时长：{autoSplitInfo.duration || workflowForm.duration || '-'} 秒
                      {' · '}风格：{autoSplitInfo.style_preset || workflowForm.stylePreset || '-'}
                    </div>
                    <div className="mt-1 break-all text-cyan-200/90">
                      文案长度：{autoSplitInfo.script_length || '-'} 字
                      {' · '}目标每集承载：{autoSplitInfo.target_chars_per_episode || '-'} 字
                      {' · '}最终分集数：{autoSplitInfo.estimated_episodes || workflowEpisodes.length || '-'}
                    </div>
                  </div>
                  {autoSplitInfo.original_script && (
                    <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                      <div className="font-medium text-cyan-50">优化前全文</div>
                      <div className="mt-2 max-h-40 overflow-y-auto whitespace-pre-wrap text-slate-200">
                        {autoSplitInfo.original_script}
                      </div>
                    </div>
                  )}
                  {autoSplitInfo.optimized_script && (
                    <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                      <div className="font-medium text-cyan-50">优化后全文</div>
                      <div className="mt-2 max-h-48 overflow-y-auto whitespace-pre-wrap text-slate-100">
                        {autoSplitInfo.optimized_script}
                      </div>
                    </div>
                  )}
                </div>
              )}
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
                <div className="text-xs text-slate-400">当前还没有分集记录；系统会在文案优化完成后，按所选视频模型时长自动切分并在这里出现内容。</div>
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
