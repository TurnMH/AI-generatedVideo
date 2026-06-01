'use client'

import { useEffect, useMemo, useRef, useState } from 'react'
import { useRouter } from 'next/navigation'
import useSWR from 'swr'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Label } from '@/components/ui/label'
import { useToast } from '@/components/ui/toast'
import { modelAPI, projectAPI, videoAPI, type Model, type Project } from '@/lib/api'

type ModelParamValue = { value: string; label: string }
type ModelParamOption = { key: string; label: string; default: string; values?: ModelParamValue[] }
type VideoModelStatus = {
  key: string
  label?: string
  provider?: string
  provider_model?: string
  available: boolean
  native_audio?: boolean
  params?: ModelParamOption[]
}

function getFallbackVideoModelLabel(key: string) {
  const map: Record<string, string> = {
    wan: '通义-Wan-图生视频',
    'wan-t2v': '通义-Wan-文生视频',
    vidu: '生数-Vidu-标准版',
    'vidu-mix': '生数-Vidu-Mix',
    'vidu-offpeak': '生数-Vidu-离峰版',
    'vidu-mix-offpeak': '生数-Vidu-Mix离峰版',
    kling: '可灵-Kling-标准版',
    aiping: '爱评-Kling-K3',
    'tencent-vclm': '腾讯-VCLM-Kling',
    doubao: '豆包-视频生成-标准版',
    'doubao-seedance': '豆包-Seedance-2.0',
    suanneng: '算能-视频生成-标准版',
    'hubagi-voe3.1': 'Google-Veo-3.1',
    'hubagi-TC-GV': 'Google-TC-GV-标准版',
    sora2: 'OpenAI-Sora-2',
    'comfyui-video': 'ComfyUI-Video-本地版',
    runninghub: 'RunningHub-Video-标准版',
    cogvideo: 'CogVideo-Video-标准版',
    'baidu-bce': '百度-BCE-视频生成',
    gaga: 'Gaga-Video-标准版',
    minmax: 'MiniMax-Hailuo-标准版',
  }
  return map[key] || key
}

function formatVideoModelLabel(status?: Pick<VideoModelStatus, 'key' | 'label' | 'provider' | 'provider_model'> | null, keyFallback?: string) {
  const key = status?.key || keyFallback || ''
  return status?.label?.trim() || getFallbackVideoModelLabel(key)
}

type ProjectPayload = {
  code?: number
  data?: Project
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
  const router = useRouter()
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
  const creatingProjectRef = useRef(false)
  const [startingFlow, setStartingFlow] = useState(false)

  const { data: workflowProjectData, mutate: mutateProject } = useSWR(
    workflowProjectId ? ['ad-video-project', workflowProjectId] : null,
    async () => {
      const res = await projectAPI.get(workflowProjectId as number)
      return unwrap<Project>((res as { data?: ProjectPayload }).data)
    },
    { revalidateOnFocus: true },
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

  const workflowProject = workflowProjectData || null
  const textModels = useMemo(() => workflowModelData?.text || [], [workflowModelData])
  const imageModels = useMemo(() => workflowModelData?.image || [], [workflowModelData])
  const videoModels = useMemo(() => workflowModelData?.video || [], [workflowModelData])
  const videoModelStatuses = useMemo(() => workflowModelData?.videoStatus || [], [workflowModelData])
  const availableVideoModelKeys = useMemo(
    () => new Set(videoModelStatuses.filter((item) => item.available).map((item) => item.key)),
    [videoModelStatuses],
  )
  const creatableVideoModels = useMemo(
    () => videoModels.filter((item) => availableVideoModelKeys.has(String(item.model_key || '').trim())),
    [availableVideoModelKeys, videoModels],
  )
  const selectedVideoModel = useMemo(() => {
    const selectedId = Number(workflowForm.videoModelId)
    if (!Number.isFinite(selectedId) || selectedId <= 0) return null
    return creatableVideoModels.find((item) => item.id === selectedId) || null
  }, [creatableVideoModels, workflowForm.videoModelId])
  const selectedVideoStatus = useMemo(() => {
    const modelKey = String(selectedVideoModel?.model_key || '').trim()
    if (!modelKey) return null
    return videoModelStatuses.find((item) => item.key === modelKey) || null
  }, [selectedVideoModel, videoModelStatuses])
  const selectedVideoDisplayLabel = useMemo(
    () => formatVideoModelLabel(selectedVideoStatus, String(selectedVideoModel?.model_key || '').trim()),
    [selectedVideoModel?.model_key, selectedVideoStatus],
  )
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

  useEffect(() => {
    if (!textModels.length) return
    const currentId = Number(workflowForm.textModelId)
    if (Number.isFinite(currentId) && currentId > 0 && textModels.some((item) => item.id === currentId)) return
    setWorkflowForm((prev) => ({ ...prev, textModelId: String(textModels[0]?.id || 'default') }))
  }, [textModels, workflowForm.textModelId])

  useEffect(() => {
    if (!imageModels.length) return
    const currentId = Number(workflowForm.imageModelId)
    if (Number.isFinite(currentId) && currentId > 0 && imageModels.some((item) => item.id === currentId)) return
    setWorkflowForm((prev) => ({ ...prev, imageModelId: String(imageModels[0]?.id || 'default') }))
  }, [imageModels, workflowForm.imageModelId])

  useEffect(() => {
    if (!creatableVideoModels.length) return
    const currentId = Number(workflowForm.videoModelId)
    if (Number.isFinite(currentId) && currentId > 0 && creatableVideoModels.some((item) => item.id === currentId)) return
    setWorkflowForm((prev) => ({ ...prev, videoModelId: String(creatableVideoModels[0]?.id || 'default') }))
  }, [creatableVideoModels, workflowForm.videoModelId])

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
    if (creatingProjectRef.current) return
    if (!workflowForm.title.trim()) {
      toast({ title: '请先填写广告工作项目标题', variant: 'destructive' })
      return
    }
    if (!workflowForm.scriptText.trim()) {
      toast({ title: '请先填写广告文案，再开始创建与优化', variant: 'destructive' })
      return
    }
    const textModelIdNum = Number(workflowForm.textModelId)
    if (!Number.isFinite(textModelIdNum) || textModelIdNum <= 0) {
      toast({ title: '请先选择文本模型', variant: 'destructive' })
      return
    }
    const imageModelIdNum = Number(workflowForm.imageModelId)
    if (!Number.isFinite(imageModelIdNum) || imageModelIdNum <= 0) {
      toast({ title: '请先选择图片模型', variant: 'destructive' })
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
    creatingProjectRef.current = true
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
        throw new Error('创建广告项目失败：未拿到 project id')
      }
      const filenameBase = workflowForm.title.trim() || `ad-project-${project.id}`
      const file = new File([workflowForm.scriptText.trim()], `${filenameBase}.txt`, { type: 'text/plain' })
      await projectAPI.uploadScript(project.id, file)
      await projectAPI.generateEpisodes(project.id, undefined, { autoStoryboard: true })
      setWorkflowProjectId(project.id)
      toast({ title: `广告项目 #${project.id} 已创建，并已开始文案优化与自动分镜流程`, variant: 'success' })
      await mutateProject()
      router.push(`/ad-video/history/${project.id}`)
    } catch (error) {
      toast({ title: error instanceof Error ? error.message : '创建广告项目并启动优化失败', variant: 'destructive' })
    } finally {
      creatingProjectRef.current = false
      setCreatingProject(false)
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold text-white">口播广告工作台</h1>
          <p className="mt-2 text-sm text-slate-300">
            从这里新建广告项目。后续生成与处理统一在项目详情页完成。
          </p>
        </div>
        <Button variant="outline" asChild>
          <a href="/ad-video/history">历史记录</a>
        </Button>
      </div>

      <Card className="border-white/10 bg-slate-900/60 text-slate-100">
        <CardHeader>
          <CardTitle className="text-white">一、广告创建主入口</CardTitle>
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
                    {model.name}
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
                    {model.name}
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
                {creatableVideoModels.map((model) => {
                  const status = videoModelStatuses.find((item) => item.key === String(model.model_key || '').trim())
                  return (
                    <option key={model.id} value={String(model.id)}>
                      {formatVideoModelLabel(status, String(model.model_key || '').trim())}
                    </option>
                  )
                })}
              </select>
            </div>
            <div className="space-y-3 rounded-xl border border-white/10 bg-black/20 p-4 md:col-span-2">
              <div className="text-sm font-medium text-white">视频模型参数</div>
              {selectedVideoModel ? (
                <div className="space-y-4 text-xs text-slate-300">
                  <div className="rounded-lg border border-white/10 bg-slate-950/40 px-3 py-2">
                    <div className="text-[11px] text-slate-500">当前模型</div>
                    <div className="mt-1 break-all text-white">{selectedVideoDisplayLabel}</div>
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
                placeholder="把广告文案贴在这里。创建后会自动开始：文案优化 → 自动分集 → 自动分镜。"
              />
            </div>
          </div>

          <div className="flex justify-center">
            <Button
              size="lg"
              className="min-w-[220px] px-10 py-6 text-base font-medium"
              disabled={creatingProject}
              onClick={createWorkflowProject}
            >
              {creatingProject ? '创建中…' : '新建项目'}
            </Button>
          </div>

          <div className="rounded-xl border border-white/10 bg-black/20 p-4 text-sm text-slate-300">
            新建后会自动执行：文案优化、自动分集、自动分镜。
          </div>
        </CardContent>
      </Card>

    </div>
  )
}
