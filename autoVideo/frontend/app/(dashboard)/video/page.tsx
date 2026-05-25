'use client'

import { useEffect, useMemo, useState } from 'react'
import useSWR from 'swr'
import Link from 'next/link'
import { useRouter, useSearchParams } from 'next/navigation'
import {
  Film,
  ImagePlus,
  Layers3,
  ArrowRightLeft,
  ScanFace,
  CheckCircle2,
  XCircle,
  Loader2,
  Info,
  Sparkles,
  Volume2,
  AlertTriangle,
} from 'lucide-react'
import { projectAPI, storageAPI, videoAPI } from '@/lib/api'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { cn } from '@/lib/utils'
import { useToast } from '@/components/ui/toast'

type ModelParamValue = { value: string; label: string }
type ModelParamOption = { key: string; label: string; default: string; values?: ModelParamValue[] }
type VideoModelStatus = { key: string; available: boolean; native_audio?: boolean; params?: ModelParamOption[] }
type ManualMenuKey = 'text' | 'image' | 'reference' | 'start-end' | 'face-swap'
type ManualMenuDef = { key: ManualMenuKey; label: string; description: string; icon: React.ComponentType<{ className?: string }> }

type ManualFormState = {
  prompt: string
  sourceImageUrl: string
  tailImageUrl: string
  referenceImages: string
  faceSourceUrl: string
  faceTargetUrl: string
  modelName: string
  aspectRatio: string
  resolution: string
  duration: string
  sourceImageFile: File | null
  referenceImageFiles: File[]
}

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

const MANUAL_VIDEO_HISTORY_KEY = 'manual-video-history-v1'

const MANUAL_MENU_ITEMS: ManualMenuDef[] = [
  { key: 'text', label: '文生视频', description: '仅输入提示词生成视频，优先展示支持纯文本生成的模型。', icon: Film },
  { key: 'image', label: '图生视频', description: '上传单张首帧图片驱动视频生成。', icon: ImagePlus },
  { key: 'reference', label: '融合生视频', description: '基于参考图/角色图做主体一致性生成。', icon: Layers3 },
  { key: 'start-end', label: '首尾针视频', description: '同时指定首帧与尾帧，生成过渡视频。', icon: ArrowRightLeft },
  { key: 'face-swap', label: '人物一致性参考', description: '复用 reference2video / 角色参考图能力，作为人物一致性增强入口，不冒充独立换脸后端。', icon: ScanFace }
]

const EMPTY_FORM: ManualFormState = {
  prompt: '',
  sourceImageUrl: '',
  tailImageUrl: '',
  referenceImages: '',
  faceSourceUrl: '',
  faceTargetUrl: '',
  modelName: '',
  aspectRatio: '',
  resolution: '',
  duration: '',
  sourceImageFile: null,
  referenceImageFiles: [],
}

const TEXT_MODELS = new Set(['wan', 'wan-t2v', 'vidu', 'vidu-offpeak'])
const START_END_MODELS = new Set(['doubao', 'doubao-seedance', 'suanneng', 'vidu', 'vidu-offpeak', 'kling'])
const REFERENCE_MODELS = new Set(['doubao', 'doubao-seedance', 'suanneng', 'vidu', 'vidu-offpeak', 'kling', 'wan'])
const FACE_SWAP_MODELS = new Set(['doubao', 'doubao-seedance', 'suanneng', 'vidu', 'vidu-offpeak', 'kling', 'wan', 'comfyui-video'])

function hasGenerateMode(model: VideoModelStatus, mode: string) {
  return (model.params || []).some((param) => param.key === 'generate_mode' && (param.values || []).some((item) => item.value === mode))
}

function getModelDisplayName(key: string) {
  const map: Record<string, string> = {
    'wan': 'Wan 图生视频',
    'wan-t2v': 'Wan 文生视频',
    'vidu': 'Vidu',
    'vidu-offpeak': 'Vidu（离峰）',
    'kling': 'Kling',
    'tencent-vclm': 'Tencent VCLM / Kling 路由',
    'doubao': 'Doubao',
    'doubao-seedance': 'Doubao Seedance',
    'suanneng': '算能',
    'hubagi-voe3.1': 'Veo 3.1（Hubagi）',
    'hubagi-TC-GV': 'TC-GV（Hubagi）',
    'sora2': 'Sora 2',
    'comfyui-video': 'ComfyUI Video',
    'runninghub': 'RunningHub',
    'cogvideo': 'CogVideo',
    'baidu-bce': 'Baidu BCE',
    'gaga': 'Gaga',
    'aiping': '爱评',
  }
  return map[key] || key
}

function inferModelCategories(model: VideoModelStatus): ManualMenuKey[] {
  const categories = new Set<ManualMenuKey>()
  const key = model.key
  const supportsText = TEXT_MODELS.has(key) || hasGenerateMode(model, 'text2video')
  const supportsReference = REFERENCE_MODELS.has(key) || hasGenerateMode(model, 'reference2video')
  const supportsStartEnd = START_END_MODELS.has(key) || hasGenerateMode(model, 'startEnd2video')
  const supportsImage = !supportsText || key === 'wan' || supportsReference || supportsStartEnd
  if (supportsText) categories.add('text')
  if (supportsImage) categories.add('image')
  if (supportsReference) categories.add('reference')
  if (supportsStartEnd) categories.add('start-end')
  if (supportsReference && (FACE_SWAP_MODELS.has(key) || hasGenerateMode(model, 'reference2video'))) categories.add('face-swap')
  return Array.from(categories)
}

function capabilityHints(model: VideoModelStatus, categories: ManualMenuKey[]) {
  const hints: string[] = []
  if (categories.includes('text')) hints.push('支持文生视频')
  if (categories.includes('image')) hints.push('支持图生视频')
  if (categories.includes('reference')) hints.push('支持参考图/融合生成')
  if (categories.includes('start-end')) hints.push('支持首尾帧过渡')
  if (categories.includes('face-swap')) hints.push('可走人物一致性参考链')
  if (model.native_audio) hints.push('支持原生音频')
  if ((model.params || []).some((p) => p.key === 'aspect_ratio')) hints.push('可选画幅比例')
  if ((model.params || []).some((p) => p.key === 'resolution')) hints.push('可选分辨率')
  return hints
}

function buildHelperText(tab: ManualMenuKey) {
  switch (tab) {
    case 'text':
      return '文生视频会按真实模型能力走 text2video；不支持纯文本生成的模型不会出现在这里。'
    case 'image':
      return '图生视频要求首帧图片；本页只展示具备图生视频基础能力的模型。'
    case 'reference':
      return '融合生视频底层依赖 reference2video / 角色参考图能力。'
    case 'start-end':
      return '首尾针视频底层依赖 startEnd2video 能力，至少需要首帧和尾帧。'
    case 'face-swap':
      return '当前入口会走 reference2video / 人物一致性链，不宣称存在独立 face-swap 后端。'
  }
}

export default function VideoManualPage() {
  const { toast } = useToast()
  const router = useRouter()
  const searchParams = useSearchParams()
  const tabParam = searchParams.get('tab')
  const [activeMenu, setActiveMenu] = useState<ManualMenuKey>('text')
  const [form, setForm] = useState<ManualFormState>(EMPTY_FORM)
  const [submitting, setSubmitting] = useState(false)
  const [submitResult, setSubmitResult] = useState<SubmitSummary | null>(null)
  const [uploadProgress, setUploadProgress] = useState(0)
  const { data, isLoading } = useSWR('video-model-status', () => videoAPI.modelStatus())

  const models: VideoModelStatus[] = useMemo(() => ((data as { models?: VideoModelStatus[] } | undefined)?.models || []), [data])

  const grouped = useMemo(() => {
    const next: Record<ManualMenuKey, VideoModelStatus[]> = { text: [], image: [], reference: [], 'start-end': [], 'face-swap': [] }
    for (const model of models) {
      for (const category of inferModelCategories(model)) next[category].push(model)
    }
    return next
  }, [models])

  const activeModels = useMemo(() => grouped[activeMenu] || [], [grouped, activeMenu])
  const activeMeta = MANUAL_MENU_ITEMS.find((item) => item.key === activeMenu) || MANUAL_MENU_ITEMS[0]
  const fallbackModelKey = activeModels[0]?.key || ''
  const effectiveModelKey = activeModels.some((item) => item.key === form.modelName) ? form.modelName : fallbackModelKey
  const selectedModel = activeModels.find((item) => item.key === effectiveModelKey) || activeModels[0]

  useEffect(() => {
    const allowed = new Set<ManualMenuKey>(['text', 'image', 'reference', 'start-end', 'face-swap'])
    if (tabParam && allowed.has(tabParam as ManualMenuKey)) setActiveMenu(tabParam as ManualMenuKey)
  }, [tabParam])

  useEffect(() => {
    if (activeModels.length === 0) {
      const fallback = MANUAL_MENU_ITEMS.find((item) => (grouped[item.key] || []).length > 0)
      if (fallback && fallback.key !== activeMenu) setActiveMenu(fallback.key)
    }
  }, [activeMenu, activeModels.length, grouped])

  useEffect(() => {
    if (activeModels.length === 0) return
    setForm((prev) => {
      if (prev.modelName && activeModels.some((item) => item.key === prev.modelName)) {
        return prev
      }
      const nextKey = activeModels[0]?.key || prev.modelName
      if (nextKey === prev.modelName) return prev
      return { ...prev, modelName: nextKey }
    })
  }, [activeMenu, activeModels])

  const selectedAspectValues = (selectedModel?.params || []).find((p) => p.key === 'aspect_ratio')?.values || []
  const selectedResolutionValues = (selectedModel?.params || []).find((p) => p.key === 'resolution')?.values || []
  const selectedDurationValues = (selectedModel?.params || []).find((p) => p.key === 'duration')?.values || []

  const createManualVideoTask = async (mode: 'text' | 'image' | 'reference' | 'start-end' | 'face-swap') => {
    if (!form.prompt.trim()) {
      toast({ title: '请先填写提示词', variant: 'destructive' })
      return
    }
    if (!form.modelName) {
      toast({ title: '请先选择视频模型', variant: 'destructive' })
      return
    }

    const referenceUrlLines = form.referenceImages
      .split(/\r?\n/)
      .map((item) => item.trim())
      .filter(Boolean)

    if ((mode === 'image' || mode === 'start-end') && !form.sourceImageUrl.trim() && !form.sourceImageFile) {
      toast({ title: '请先填写首帧图片 URL 或上传本地图片', variant: 'destructive' })
      return
    }
    if (mode === 'reference' && !form.sourceImageUrl.trim() && !form.sourceImageFile && referenceUrlLines.length === 0 && form.referenceImageFiles.length === 0) {
      toast({ title: '请至少提供首帧图或参考图', variant: 'destructive' })
      return
    }
    if (mode === 'face-swap' && !form.faceTargetUrl.trim()) {
      toast({ title: '请先填写目标首帧 URL', variant: 'destructive' })
      return
    }
    if (mode === 'face-swap' && !form.faceSourceUrl.trim() && referenceUrlLines.length === 0) {
      toast({ title: '请至少提供一张人物参考图或主参考脸 URL', variant: 'destructive' })
      return
    }
    if (mode === 'start-end' && !form.tailImageUrl.trim()) {
      toast({ title: '请先填写尾帧图片 URL', variant: 'destructive' })
      return
    }

    setSubmitting(true)
    setUploadProgress(0)
    setSubmitResult(null)
    try {
      const modeLabel = mode === 'image' ? '图生' : mode === 'reference' ? '融合' : mode === 'start-end' ? '首尾针' : mode === 'face-swap' ? '人物一致性参考' : '文生'
      const projectRes = await projectAPI.create({
        title: `手动${modeLabel}视频-${new Date().toISOString().slice(0, 16).replace('T', ' ')}`,
        description: `手动创建视频-${modeLabel}视频临时项目`,
        project_type: 'video',
      } as never) as { data?: { id?: number } }
      const projectId = Number(projectRes?.data?.id || 0)
      if (!projectId) throw new Error('创建临时项目失败')

      let sourceImageUrl = form.sourceImageUrl.trim()
      let tailImageUrl = form.tailImageUrl.trim()
      if (!sourceImageUrl && form.sourceImageFile) {
        const uploadRes = await storageAPI.upload(projectId, form.sourceImageFile, {
          bucket: 'images',
          category: 'manual-video-source',
          onProgress: (percent) => setUploadProgress(percent),
        }) as { data?: { cdn_url?: string } }
        sourceImageUrl = String(uploadRes?.data?.cdn_url || '').trim()
      }

      const referenceImageUrls = [
        ...(mode === 'face-swap' && form.faceSourceUrl.trim() ? [form.faceSourceUrl.trim()] : []),
        ...referenceUrlLines,
      ]
      if (form.referenceImageFiles.length > 0) {
        for (const file of form.referenceImageFiles) {
          const uploadRes = await storageAPI.upload(projectId, file, {
            bucket: 'images',
            category: 'manual-video-reference',
            onProgress: (percent) => setUploadProgress(percent),
          }) as { data?: { cdn_url?: string } }
          const uploaded = String(uploadRes?.data?.cdn_url || '').trim()
          if (uploaded) referenceImageUrls.push(uploaded)
        }
      }

      if (mode === 'start-end' && !tailImageUrl) {
        throw new Error('首尾针视频必须提供尾帧图片 URL')
      }
      if (mode === 'image' && !sourceImageUrl) throw new Error('首帧图片上传成功，但未获取到可用链接')
      if (mode === 'reference' && !sourceImageUrl && referenceImageUrls.length === 0) throw new Error('融合生视频至少需要首帧图或参考图')
      if (mode === 'start-end' && !sourceImageUrl) throw new Error('首尾针视频必须提供首帧图片 URL')

      const renderConfig: Record<string, unknown> = {}
      if (form.aspectRatio) renderConfig.aspect_ratio = form.aspectRatio
      if (form.resolution) renderConfig.resolution = form.resolution
      if (mode === 'text' && form.modelName.includes('vidu')) renderConfig.generate_mode = 'text2video'
      if (mode === 'reference' || mode === 'face-swap') renderConfig.generate_mode = 'reference2video'
      if (mode === 'face-swap') {
        renderConfig.reference_mode = 'identity-consistency'
        renderConfig.face_swap_note = 'No dedicated face-swap backend detected; routed through reference2video / character consistency path.'
      }
      if (mode === 'start-end') {
        renderConfig.generate_mode = 'startEnd2video'
        renderConfig.tail_image_url = tailImageUrl
      }
      const duration = Number(form.duration)

      const imageUrls = mode === 'text'
        ? []
        : mode === 'face-swap'
          ? [faceTargetUrl || sourceImageUrl || referenceImageUrls[0]]
          : sourceImageUrl
            ? [sourceImageUrl]
            : [referenceImageUrls[0]]
      if ((mode === 'reference' || mode === 'face-swap') && referenceImageUrls.length > 0) {
        renderConfig.character_image_urls = referenceImageUrls
      }

      const generateRes = await videoAPI.generate(projectId, {
        image_urls: imageUrls,
        scene_descriptions: [form.prompt.trim()],
        scene_description: form.prompt.trim(),
        model_name: form.modelName,
        render_config: renderConfig,
        clip_duration_sec: Number.isFinite(duration) && duration > 0 ? duration : undefined,
      }) as { data?: { task_id?: number } }

      const taskId = Number(generateRes?.data?.task_id || 0)
      if (!taskId) throw new Error('视频任务创建成功，但未返回 task_id')

      const generateMode = String(renderConfig.generate_mode || (mode === 'image' ? 'img2video' : mode === 'text' ? 'text2video' : 'img2video'))
      const routeNote = mode === 'face-swap'
        ? '通过 reference2video / 人物一致性参考链提交，并非独立换脸后端。'
        : mode === 'start-end'
          ? '通过 startEnd2video 提交。'
          : mode === 'reference'
            ? '通过 reference2video 提交。'
            : mode === 'text'
              ? '通过 text2video 提交。'
              : '通过 img2video 提交。'
      const summary: SubmitSummary = {
        projectId,
        taskId,
        mode,
        modelName: form.modelName,
        generateMode,
        sourceCount: imageUrls.filter(Boolean).length,
        referenceCount: referenceImageUrls.filter(Boolean).length,
        hasStartImage: Boolean(imageUrls[0]),
        hasTailImage: Boolean(renderConfig.tail_image_url),
        routeNote,
        createdAt: new Date().toISOString(),
      }
      setSubmitResult(summary)
      if (typeof window !== 'undefined') {
        try {
          const raw = window.localStorage.getItem(MANUAL_VIDEO_HISTORY_KEY)
          const prev = raw ? JSON.parse(raw) : []
          const next = [summary, ...(Array.isArray(prev) ? prev : [])].slice(0, 50)
          window.localStorage.setItem(MANUAL_VIDEO_HISTORY_KEY, JSON.stringify(next))
        } catch {}
      }
      toast({ title: `${mode === 'image' ? '图生' : mode === 'reference' ? '融合' : mode === 'start-end' ? '首尾针' : mode === 'face-swap' ? '人物一致性参考' : '文生'}视频任务已创建`, description: `项目 ${projectId} / 任务 ${taskId}`, variant: 'success' })
    } catch (error) {
      const message = error instanceof Error ? error.message : `${mode === 'image' ? '图生' : mode === 'reference' ? '融合' : mode === 'start-end' ? '首尾针' : mode === 'face-swap' ? '人物一致性参考' : '文生'}视频创建失败`
      toast({ title: message, variant: 'destructive' })
    } finally {
      setSubmitting(false)
      setUploadProgress(0)
    }
  }

  const handleImageGenerate = async () => {
    await createManualVideoTask('image')
  }

  const renderForm = () => {
    const noModelsAvailable = activeModels.length === 0
    const disabledReason = noModelsAvailable
      ? `当前「${activeMeta.label}」没有可用视频模型，请切换分类或检查 model-status。`
      : ''

    return (
      <Card className="border-white/10 bg-slate-900/60 text-slate-100">
        <CardHeader>
          <CardTitle>{activeMeta.label}表单</CardTitle>
          <CardDescription className="text-slate-400">{buildHelperText(activeMenu)}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid gap-4 md:grid-cols-2">
            <div className="space-y-2 md:col-span-2">
              <Label>提示词</Label>
              <Textarea value={form.prompt} onChange={(e) => setForm((prev) => ({ ...prev, prompt: e.target.value }))} placeholder="输入视频提示词 / 场景描述 / 镜头要求" className="min-h-[120px]" />
            </div>

            {(activeMenu === 'image' || activeMenu === 'reference' || activeMenu === 'start-end') && (
              <div className="space-y-2">
                <Label>首帧图片 URL</Label>
                <Input value={form.sourceImageUrl} onChange={(e) => setForm((prev) => ({ ...prev, sourceImageUrl: e.target.value }))} placeholder="https://..." />
                {activeMenu === 'image' && (
                  <>
                    <div className="text-xs text-slate-500">或上传本地图片（会自动上传到 storage-service）</div>
                    <Input type="file" accept="image/*" onChange={(e) => setForm((prev) => ({ ...prev, sourceImageFile: e.target.files?.[0] || null }))} />
                    {form.sourceImageFile && <div className="text-xs text-slate-400">已选择：{form.sourceImageFile.name}</div>}
                    {submitting && uploadProgress > 0 && uploadProgress < 100 && <div className="text-xs text-cyan-300">上传进度：{uploadProgress}%</div>}
                  </>
                )}
              </div>
            )}

            {activeMenu === 'start-end' && (
              <div className="space-y-2">
                <Label>尾帧图片 URL</Label>
                <Input value={form.tailImageUrl} onChange={(e) => setForm((prev) => ({ ...prev, tailImageUrl: e.target.value }))} placeholder="https://..." />
              </div>
            )}

            {(activeMenu === 'reference' || activeMenu === 'face-swap') && (
              <div className="space-y-2 md:col-span-2">
                <Label>{activeMenu === 'face-swap' ? '人物参考图 / 脸部参考 URL（每行一张）' : '参考图 URL（每行一张）'}</Label>
                <Textarea value={form.referenceImages} onChange={(e) => setForm((prev) => ({ ...prev, referenceImages: e.target.value }))} placeholder={'https://ref-1\nhttps://ref-2'} className="min-h-[100px]" />
                {activeMenu === 'reference' && (
                  <>
                    <div className="text-xs text-slate-500">也可上传多张本地参考图</div>
                    <Input type="file" accept="image/*" multiple onChange={(e) => setForm((prev) => ({ ...prev, referenceImageFiles: Array.from(e.target.files || []) }))} />
                    {form.referenceImageFiles.length > 0 && <div className="text-xs text-slate-400">已选择 {form.referenceImageFiles.length} 张参考图</div>}
                  </>
                )}
              </div>
            )}

            {activeMenu === 'face-swap' && (
              <>
                <div className="space-y-2">
                  <Label>目标首帧 URL</Label>
                  <Input value={form.faceTargetUrl} onChange={(e) => setForm((prev) => ({ ...prev, faceTargetUrl: e.target.value }))} placeholder="https://..." />
                </div>
                <div className="space-y-2">
                  <Label>主参考脸 URL</Label>
                  <Input value={form.faceSourceUrl} onChange={(e) => setForm((prev) => ({ ...prev, faceSourceUrl: e.target.value }))} placeholder="https://..." />
                </div>
              </>
            )}

            <div className="space-y-2">
              <Label>视频模型</Label>
              <Select value={effectiveModelKey || selectedModel?.key || ''} onValueChange={(value) => setForm((prev) => ({ ...prev, modelName: value }))} disabled={noModelsAvailable}>
                <SelectTrigger><SelectValue placeholder={noModelsAvailable ? '当前分类无可用模型' : '选择模型'} /></SelectTrigger>
                <SelectContent>
                  {activeModels.map((model) => (
                    <SelectItem key={model.key} value={model.key}>{getModelDisplayName(model.key)}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
              {noModelsAvailable && <div className="text-xs text-amber-300">当前分类暂无可用模型，提交已禁用。</div>}
            </div>

            <div className="space-y-2">
              <Label>画幅比例</Label>
              <Select value={form.aspectRatio || selectedAspectValues[0]?.value || '__empty__'} onValueChange={(value) => setForm((prev) => ({ ...prev, aspectRatio: value === '__empty__' ? '' : value }))}>
                <SelectTrigger><SelectValue placeholder="自动" /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="__empty__">自动</SelectItem>
                  {selectedAspectValues.map((value) => (
                    <SelectItem key={value.value} value={value.value}>{value.label}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-2">
              <Label>分辨率</Label>
              <Select value={form.resolution || selectedResolutionValues[0]?.value || '__empty__'} onValueChange={(value) => setForm((prev) => ({ ...prev, resolution: value === '__empty__' ? '' : value }))}>
                <SelectTrigger><SelectValue placeholder="自动" /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="__empty__">自动</SelectItem>
                  {selectedResolutionValues.map((value) => (
                    <SelectItem key={value.value} value={value.value}>{value.label}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="space-y-2">
              <Label>时长</Label>
              <Select value={form.duration || selectedDurationValues[0]?.value || '__empty__'} onValueChange={(value) => setForm((prev) => ({ ...prev, duration: value === '__empty__' ? '' : value }))}>
                <SelectTrigger><SelectValue placeholder="默认" /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="__empty__">默认</SelectItem>
                  {selectedDurationValues.map((value) => (
                    <SelectItem key={value.value} value={value.value}>{value.label}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>

          {submitResult && (
            <div className="rounded-xl border border-emerald-400/20 bg-emerald-400/5 p-4 text-sm text-emerald-100">
              <div className="font-medium">任务已创建：{submitResult.mode === 'text' ? '文生视频' : submitResult.mode === 'image' ? '图生视频' : submitResult.mode === 'reference' ? '融合生视频' : submitResult.mode === 'start-end' ? '首尾针视频' : '人物一致性参考'}</div>
              <div className="mt-1">项目 ID：{submitResult.projectId}，任务 ID：{submitResult.taskId}</div>
              <div className="mt-3 grid gap-2 text-xs text-emerald-50/90 md:grid-cols-2">
                <div>模型：{getModelDisplayName(submitResult.modelName)}</div>
                <div>实际模式：{submitResult.generateMode}</div>
                <div>首帧输入：{submitResult.hasStartImage ? '有' : '无'}</div>
                <div>尾帧输入：{submitResult.hasTailImage ? '有' : '无'}</div>
                <div>首帧数量：{submitResult.sourceCount}</div>
                <div>参考图数量：{submitResult.referenceCount}</div>
              </div>
              <div className="mt-3 rounded-lg border border-emerald-300/15 bg-black/10 px-3 py-2 text-xs text-emerald-50/85">
                {submitResult.routeNote}
              </div>
              <div className="mt-3 flex flex-wrap gap-3">
                <Button variant="outline" asChild>
                  <Link href={`/video/history/${submitResult.taskId}`}>打开任务详情</Link>
                </Button>
                <Button variant="outline" asChild>
                  <Link href="/video/history">查看历史记录</Link>
                </Button>
              </div>
            </div>
          )}

          {disabledReason ? (
            <div className="rounded-xl border border-amber-400/20 bg-amber-400/5 p-4 text-sm text-amber-100">
              <div className="flex items-start gap-2">
                <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0" />
                <div>
                  <div className="font-medium">当前分类暂无可用模型</div>
                  <div className="mt-1 text-amber-100/80">{disabledReason}</div>
                </div>
              </div>
            </div>
          ) : (
            <div className="rounded-xl border border-cyan-400/20 bg-cyan-400/5 p-4 text-sm text-cyan-100">
              当前分类已经具备做最小提交流程的前置条件。下一步可继续接“临时项目 + 上传资源 + 调用 generate”闭环。
            </div>
          )}

          <div className="flex flex-wrap gap-3">
            {activeMenu === 'text' ? (
              <Button onClick={() => createManualVideoTask('text')} disabled={submitting || noModelsAvailable}>
                {submitting ? '正在创建文生视频任务…' : '创建文生视频任务'}
              </Button>
            ) : activeMenu === 'image' ? (
              <Button onClick={handleImageGenerate} disabled={submitting || noModelsAvailable}>
                {submitting ? '正在创建图生视频任务…' : '创建图生视频任务'}
              </Button>
            ) : activeMenu === 'reference' ? (
              <Button onClick={() => createManualVideoTask('reference')} disabled={submitting || noModelsAvailable}>
                {submitting ? '正在创建融合生视频任务…' : '创建融合生视频任务'}
              </Button>
            ) : activeMenu === 'start-end' ? (
              <Button onClick={() => createManualVideoTask('start-end')} disabled={submitting || noModelsAvailable}>
                {submitting ? '正在创建首尾针视频任务…' : '创建首尾针视频任务'}
              </Button>
            ) : activeMenu === 'face-swap' ? (
              <Button onClick={() => createManualVideoTask('face-swap')} disabled={submitting || noModelsAvailable}>
                {submitting ? '正在创建人物一致性任务…' : '创建人物一致性任务'}
              </Button>
            ) : (
              <Button disabled>{disabledReason ? '等待后端链路补齐' : '下一步接真实提交'}</Button>
            )}
            <Button variant="outline" asChild>
              <Link href="/projects">去现有项目视频链</Link>
            </Button>
          </div>
        </CardContent>
      </Card>
    )
  }

  return (
    <div className="space-y-6">
      <div className="flex items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold text-white">手动创建视频</h1>
          <p className="mt-2 text-sm text-slate-300">按现有 video-service 运行态模型能力分组展示，并补齐最小可操作表单骨架；未坐实的独立能力会按真实链路收口命名。</p>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button variant="outline" onClick={() => router.push('/video/history')}>查看独立历史页</Button>
          <Link href="/projects" className="inline-flex items-center gap-2 rounded-lg border border-white/10 bg-white/5 px-3 py-2 text-sm text-slate-200 transition hover:bg-white/10">去项目生成链</Link>
        </div>
      </div>

      <div className="grid gap-4 lg:grid-cols-[300px,minmax(0,1fr)]">
        <Card className="border-white/10 bg-slate-900/60 text-slate-100">
          <CardHeader>
            <CardTitle>手动创建视频</CardTitle>
            <CardDescription className="text-slate-400">二级菜单</CardDescription>
          </CardHeader>
          <CardContent className="space-y-2">
            {MANUAL_MENU_ITEMS.map((item) => {
              const Icon = item.icon
              const count = grouped[item.key]?.length || 0
              const active = item.key === activeMenu
              return (
                <button key={item.key} type="button" onClick={() => setActiveMenu(item.key)} className={cn('w-full rounded-xl border px-4 py-3 text-left transition', active ? 'border-cyan-400/50 bg-cyan-400/10 shadow-[0_0_0_1px_rgba(34,211,238,0.15)]' : 'border-white/8 bg-white/[0.03] hover:bg-white/[0.06]')}>
                  <div className="flex items-center justify-between gap-3">
                    <div className="flex items-center gap-3">
                      <span className="flex h-9 w-9 items-center justify-center rounded-lg bg-white/5"><Icon className="h-4 w-4" /></span>
                      <div>
                        <div className="font-medium text-slate-100">{item.label}</div>
                        <div className="mt-1 text-xs text-slate-400">{item.description}</div>
                      </div>
                    </div>
                    <span className="rounded-full bg-white/8 px-2 py-1 text-xs text-slate-300">{count}</span>
                  </div>
                </button>
              )
            })}
          </CardContent>
        </Card>

        <div className="space-y-4">
          <Card className="border-white/10 bg-slate-900/60 text-slate-100">
            <CardHeader>
              <div className="flex items-center gap-3">
                <span className="flex h-10 w-10 items-center justify-center rounded-xl bg-cyan-400/10 text-cyan-300"><activeMeta.icon className="h-5 w-5" /></span>
                <div>
                  <CardTitle>{activeMeta.label}</CardTitle>
                  <CardDescription className="text-slate-400">{activeMeta.description}</CardDescription>
                </div>
              </div>
            </CardHeader>
            <CardContent>
              {isLoading ? (
                <div className="flex items-center gap-2 text-sm text-slate-300"><Loader2 className="h-4 w-4 animate-spin" /> 正在加载视频模型能力…</div>
              ) : activeModels.length === 0 ? (
                <div className="rounded-xl border border-amber-400/20 bg-amber-400/5 p-4 text-sm text-amber-100">当前分类下没有可识别模型。</div>
              ) : (
                <div className="grid gap-4 xl:grid-cols-2">
                  {activeModels.map((model) => {
                    const categories = inferModelCategories(model)
                    const hints = capabilityHints(model, categories)
                    return (
                      <div key={`${activeMenu}-${model.key}`} className="rounded-2xl border border-white/10 bg-white/[0.03] p-4">
                        <div className="flex items-start justify-between gap-3">
                          <div>
                            <h3 className="text-base font-semibold text-white">{getModelDisplayName(model.key)}</h3>
                            <p className="mt-1 text-xs text-slate-400">模型键：{model.key}</p>
                          </div>
                          <span className={cn('inline-flex items-center gap-1 rounded-full px-2.5 py-1 text-xs', model.available ? 'bg-emerald-400/10 text-emerald-300' : 'bg-rose-400/10 text-rose-300')}>
                            {model.available ? <CheckCircle2 className="h-3.5 w-3.5" /> : <XCircle className="h-3.5 w-3.5" />} {model.available ? '可用' : '不可用'}
                          </span>
                        </div>
                        <div className="mt-3 flex flex-wrap gap-2">
                          {hints.map((hint) => <span key={hint} className="rounded-full bg-slate-800 px-2.5 py-1 text-[11px] text-slate-300">{hint}</span>)}
                        </div>
                        {!!model.params?.length && (
                          <div className="mt-4 space-y-2">
                            <div className="text-xs font-medium uppercase tracking-wider text-slate-500">可配置参数</div>
                            <div className="space-y-2">
                              {model.params.map((param) => (
                                <div key={`${model.key}-${param.key}`} className="rounded-xl bg-slate-950/50 p-3">
                                  <div className="flex items-center justify-between gap-2 text-sm">
                                    <span className="text-slate-200">{param.label}</span>
                                    <span className="text-xs text-slate-500">默认：{param.default || '-'}</span>
                                  </div>
                                  {!!param.values?.length && <div className="mt-2 flex flex-wrap gap-2">{param.values.map((value) => <span key={`${param.key}-${value.value}`} className="rounded-md border border-white/10 px-2 py-1 text-[11px] text-slate-300">{value.label}</span>)}</div>}
                                </div>
                              ))}
                            </div>
                          </div>
                        )}
                      </div>
                    )
                  })}
                </div>
              )}
            </CardContent>
          </Card>

          {renderForm()}

          <Card className="border-white/10 bg-slate-900/60 text-slate-100">
            <CardHeader>
              <CardTitle>真实能力说明</CardTitle>
              <CardDescription className="text-slate-400">避免把未完成的后端说成已完成</CardDescription>
            </CardHeader>
            <CardContent className="space-y-3 text-sm text-slate-300">
              <p className="flex items-start gap-2"><Info className="mt-0.5 h-4 w-4 text-cyan-300" /> 文生视频当前明确可坐实的是 Wan T2V 与 Vidu text2video，但项目级 `POST /api/v1/projects/:pid/videos/generate` 仍要求 `image_urls`。</p>
              <p className="flex items-start gap-2"><Sparkles className="mt-0.5 h-4 w-4 text-cyan-300" /> 融合生视频/首尾针视频的判定来自各 generator `ParamOptions()` 里的 `generate_mode` 值与实际分支实现。</p>
              <p className="flex items-start gap-2"><Volume2 className="mt-0.5 h-4 w-4 text-cyan-300" /> `native_audio` 已直接透传到前端，便于后续按模型能力展示原生音频支持。</p>
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  )
}
