'use client'

import { useEffect, useMemo, useState } from 'react'
import type React from 'react'
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
import { modelAPI, projectAPI, storageAPI, utilsAPI, videoAPI } from '@/lib/api'
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
type VideoModelStatus = {
  key: string
  label?: string
  provider?: string
  provider_model?: string
  available: boolean
  native_audio?: boolean
  params?: ModelParamOption[]
}
type ManualMenuKey = 'text' | 'image' | 'reference' | 'start-end' | 'face-swap'
type ManualMenuDef = { key: ManualMenuKey; label: string; description: string; icon: React.ComponentType<{ className?: string }> }

type ManualFormState = {
  prompt: string
  dialogueText: string
  sourceImageUrl: string
  tailImageUrl: string
  referenceImages: string
  faceSourceUrl: string
  faceTargetUrl: string
  modelName: string
  aspectRatio: string
  resolution: string
  duration: string
  stylePreset: 'anime' | 'realistic'
  optimizeTextModel: string
  generateAudio: boolean
  sourceImageFile: File | null
  referenceImageFiles: File[]
}

type SubmitSummary = {
  projectId: number
  taskId: number
  mode: ManualMenuKey
  modelName: string
  modelLabel: string
  generateMode: string
  sourceCount: number
  referenceCount: number
  hasStartImage: boolean
  hasTailImage: boolean
  generateAudio: boolean
  routeNote: string
  createdAt: string
}

type PromptOptimizePreview = {
  original: string
  optimized: string
  visualPrompt: string
  dialogue?: string
  warning?: string
  modelLabel: string
}

function splitVisualPromptAndDialogue(raw: string) {
  const lines = raw
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean)
  if (lines.length <= 1) {
    return { visualPrompt: raw.trim(), dialogue: '' }
  }

  const inlineDialoguePrefix = /^(台词|对白|旁白|配音|人物说|角色说|dialogue|voiceover|voice over|vo)[:：]\s*/i
  const dialogueHeader = /^(台词如下|台词如下所示|台词如下内容|以下是台词|以下为台词|旁白如下|以下是旁白|以下为旁白|口播如下|以下是口播|以下为口播|配音如下|以下是配音|以下为配音|配音文案|解说词|以下是解说词|以下为解说词)([:：]?)$/i
  const visualLines: string[] = []
  const dialogueLines: string[] = []
  let inDialogueBlock = false

  for (let i = 0; i < lines.length; i += 1) {
    const line = lines[i]
    if (!line) continue

    if (inlineDialoguePrefix.test(line)) {
      const value = line.replace(inlineDialoguePrefix, '').trim()
      if (value) dialogueLines.push(value)
      inDialogueBlock = true
      continue
    }

    if (dialogueHeader.test(line)) {
      inDialogueBlock = true
      continue
    }

    if (inDialogueBlock) {
      const looksLikeVisualInstruction = /^(镜头|画面|场景|环境|人物|主体|构图|运镜|光线|氛围|风格|动作|特写|远景|中景|近景|航拍|俯拍|仰拍|转场|镜头语言)[:：]/.test(line)
      if (looksLikeVisualInstruction) {
        inDialogueBlock = false
        visualLines.push(line)
        continue
      }
      dialogueLines.push(line)
      continue
    }

    visualLines.push(line)
  }

  return {
    visualPrompt: (visualLines.join('\n') || raw).trim(),
    dialogue: dialogueLines.join('\n').trim(),
  }
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
  dialogueText: '',
  sourceImageUrl: '',
  tailImageUrl: '',
  referenceImages: '',
  faceSourceUrl: '',
  faceTargetUrl: '',
  modelName: '',
  aspectRatio: '',
  resolution: '',
  duration: '',
  stylePreset: 'anime',
  optimizeTextModel: 'default',
  generateAudio: false,
  sourceImageFile: null,
  referenceImageFiles: [],
}

const TEXT_MODELS = new Set(['wan', 'wan-t2v', 'vidu', 'vidu-offpeak', 'cogvideo'])
const START_END_MODELS = new Set(['doubao', 'doubao-seedance', 'suanneng', 'vidu', 'vidu-offpeak', 'kling', 'tencent-vclm', 'runninghub'])
const REFERENCE_MODELS = new Set(['doubao', 'doubao-seedance', 'suanneng', 'vidu', 'vidu-offpeak', 'kling', 'tencent-vclm', 'wan', 'runninghub', 'comfyui-video'])
const FACE_SWAP_MODELS = new Set(['doubao', 'doubao-seedance', 'suanneng', 'vidu', 'vidu-offpeak', 'kling', 'tencent-vclm', 'wan', 'comfyui-video', 'runninghub'])
const IMAGE_ONLY_VIDEO_MODELS = new Set(['hubagi-voe3.1', 'hubagi-TC-GV', 'sora2', 'baidu-bce', 'gaga', 'aiping'])

function hasGenerateMode(model: VideoModelStatus, mode: string) {
  return (model.params || []).some((param) => param.key === 'generate_mode' && (param.values || []).some((item) => item.value === mode))
}

function isModelKey(model: VideoModelStatus, ...prefixes: string[]) {
  return prefixes.some((prefix) => model.key === prefix || model.key.startsWith(`${prefix}-`))
}

function getFallbackModelDisplayName(key: string) {
  const map: Record<string, string> = {
    'wan': '通义-Wan-图生视频',
    'wan-t2v': '通义-Wan-文生视频',
    'vidu': '生数-Vidu-标准版',
    'vidu-mix': '生数-Vidu-Mix',
    'vidu-offpeak': '生数-Vidu-离峰版',
    'vidu-mix-offpeak': '生数-Vidu-Mix离峰版',
    'kling': '可灵-Kling-标准版',
    'tencent-vclm': '腾讯-VCLM-Kling',
    'doubao': '豆包-视频生成-标准版',
    'doubao-seedance': '豆包-Seedance-2.0',
    'suanneng': '算能-视频生成-标准版',
    'hubagi-voe3.1': 'Google-Veo-3.1',
    'hubagi-TC-GV': 'Google-TC-GV-标准版',
    'sora2': 'OpenAI-Sora-2',
    'comfyui-video': 'ComfyUI-Video-本地版',
    'runninghub': 'RunningHub-Video-标准版',
    'cogvideo': 'CogVideo-Video-标准版',
    'baidu-bce': '百度-BCE-视频生成',
    'gaga': 'Gaga-Video-标准版',
    'aiping': '爱评-Kling-K3',
    'minmax': 'MiniMax-Hailuo-标准版',
  }
  return map[key] || key
}

function getModelDisplayName(model?: Pick<VideoModelStatus, 'key' | 'label' | 'provider' | 'provider_model'> | null, keyFallback?: string) {
  const key = model?.key || keyFallback || ''
  const label = model?.label?.trim()
  return label || getFallbackModelDisplayName(key)
}

function inferModelCategories(model: VideoModelStatus): ManualMenuKey[] {
  const categories = new Set<ManualMenuKey>()
  const key = model.key
  const supportsText = TEXT_MODELS.has(key) || hasGenerateMode(model, 'text2video') || isModelKey(model, 'wan-t2v', 'cogvideo')
  const supportsReference = REFERENCE_MODELS.has(key) || hasGenerateMode(model, 'reference2video') || isModelKey(model, 'kling', 'tencent-vclm', 'runninghub', 'comfyui-video')
  const supportsStartEnd = START_END_MODELS.has(key) || hasGenerateMode(model, 'startEnd2video') || isModelKey(model, 'doubao', 'vidu', 'suanneng', 'tencent-vclm', 'runninghub')
  const supportsImage =
    hasGenerateMode(model, 'image2video') ||
    hasGenerateMode(model, 'img2video') ||
    key === 'wan' ||
    supportsReference ||
    supportsStartEnd ||
    IMAGE_ONLY_VIDEO_MODELS.has(key) ||
    (!supportsText && model.available)

  if (supportsText) categories.add('text')
  if (supportsImage) categories.add('image')
  if (supportsReference) categories.add('reference')
  if (supportsStartEnd) categories.add('start-end')
  if (supportsReference && (FACE_SWAP_MODELS.has(key) || hasGenerateMode(model, 'reference2video') || isModelKey(model, 'comfyui-video', 'runninghub', 'tencent-vclm'))) {
    categories.add('face-swap')
  }
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

function getModelGenerateModes(model: VideoModelStatus) {
  return ((model.params || []).find((param) => param.key === 'generate_mode')?.values || []).map((item) => item.value)
}

function getCategoryLabel(category: ManualMenuKey) {
  return MANUAL_MENU_ITEMS.find((item) => item.key === category)?.label || category
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
  const [optimizingPrompt, setOptimizingPrompt] = useState(false)
  const [promptPreview, setPromptPreview] = useState<PromptOptimizePreview | null>(null)
  const [submitResult, setSubmitResult] = useState<SubmitSummary | null>(null)
  const [uploadProgress, setUploadProgress] = useState(0)
  const { data, isLoading } = useSWR('video-model-status', () => videoAPI.modelStatus())
  const { data: llmModelResp } = useSWR('llm-model-list', () => modelAPI.list({ type: 'llm', enabled: 'true', sort_by: 'priority' }))

  const models: VideoModelStatus[] = useMemo(() => {
    const payload = data as { models?: VideoModelStatus[]; data?: { models?: VideoModelStatus[] } } | undefined
    return payload?.data?.models || payload?.models || []
  }, [data])

  const grouped = useMemo(() => {
    const next: Record<ManualMenuKey, VideoModelStatus[]> = { text: [], image: [], reference: [], 'start-end': [], 'face-swap': [] }
    for (const model of models) {
      for (const category of inferModelCategories(model)) next[category].push(model)
    }
    return next
  }, [models])

  const optimizeTextModels = useMemo(() => {
    const payload = llmModelResp as { data?: { items?: Array<{ id: number; name: string; model_key: string; is_active: boolean }> } | Array<{ id: number; name: string; model_key: string; is_active: boolean }> } | undefined
    const raw = Array.isArray(payload?.data) ? payload?.data : payload?.data?.items || []
    const seen = new Set<string>()
    const active = raw
      .filter((item) => item?.is_active !== false && item?.model_key)
      .map((item) => ({ value: item.model_key, label: item.name || item.model_key }))
      .filter((item) => {
        if (seen.has(item.value)) return false
        seen.add(item.value)
        return true
      })
    if (active.length > 0) return active
    return [{ value: 'default', label: '系统默认文本模型' }]
  }, [llmModelResp])

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

  useEffect(() => {
    if (!optimizeTextModels.some((item) => item.value === form.optimizeTextModel)) {
      setForm((prev) => ({ ...prev, optimizeTextModel: optimizeTextModels[0]?.value || 'default' }))
    }
  }, [optimizeTextModels, form.optimizeTextModel])

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

    const requestedReferenceCount = referenceUrlLines.length + (form.faceSourceUrl.trim() ? 1 : 0) + form.referenceImageFiles.length
    if ((mode === 'reference' || mode === 'face-swap') && /kling|aiping|tencent-vclm/i.test(form.modelName) && requestedReferenceCount > 3) {
      toast({ title: '当前 Kling 系模型最多支持 3 张参考图', description: `你现在提供了 ${requestedReferenceCount} 张，请删减后再提交。`, variant: 'destructive' })
      return
    }

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

      const faceTargetUrl = form.faceTargetUrl.trim()
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
      renderConfig.style_preset = form.stylePreset
      renderConfig.generate_audio = Boolean(selectedModel?.native_audio && form.generateAudio)
      if (mode === 'text') renderConfig.generate_mode = 'text2video'
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

      const { visualPrompt, dialogue } = splitVisualPromptAndDialogue(form.prompt)
      const finalDialogue = form.dialogueText.trim() || dialogue
      const requestBody = {
        project_id: projectId,
        image_urls: imageUrls,
        scene_descriptions: [visualPrompt],
        scene_description: visualPrompt,
        dialogues: finalDialogue ? [finalDialogue] : undefined,
        model_name: form.modelName,
        style_preset: form.stylePreset,
        render_config: renderConfig,
        clip_duration_sec: Number.isFinite(duration) && duration > 0 ? duration : undefined,
      }

      const generateRes = await (mode === 'text'
        ? videoAPI.generateManual(requestBody)
        : videoAPI.generate(projectId, requestBody)) as { data?: { task_id?: number } }

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
        modelLabel: getModelDisplayName(selectedModel, form.modelName),
        generateMode,
        sourceCount: imageUrls.filter(Boolean).length,
        referenceCount: referenceImageUrls.filter(Boolean).length,
        hasStartImage: Boolean(imageUrls[0]),
        hasTailImage: Boolean(renderConfig.tail_image_url),
        generateAudio: Boolean(renderConfig.generate_audio),
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

  const optimizePromptForCurrentMode = async () => {
    const rawPrompt = form.prompt.trim()
    if (!rawPrompt) {
      toast({ title: '请先填写提示词', variant: 'destructive' })
      return
    }
    setOptimizingPrompt(true)
    try {
      const modeLabel = activeMeta.label
      const modelLabel = selectedModel ? getModelDisplayName(selectedModel) : form.modelName || '当前模型'
      const payload = await utilsAPI.optimizeVideoPrompt({
        prompt: rawPrompt,
        target_model: modelLabel,
        text_model: form.optimizeTextModel === 'default' ? '' : form.optimizeTextModel,
        mode: modeLabel,
        style_preset: form.stylePreset,
        aspect_ratio: form.aspectRatio,
        duration: form.duration,
        generate_audio: Boolean(selectedModel?.native_audio && form.generateAudio),
      }) as { optimized?: string; warning?: string }
      const optimized = payload?.optimized?.trim()
      if (!optimized) throw new Error(payload?.warning || '优化结果为空')
      const optimizeModelLabel = optimizeTextModels.find((item) => item.value === form.optimizeTextModel)?.label || '系统默认文本模型'
      const { visualPrompt, dialogue } = splitVisualPromptAndDialogue(optimized)
      setPromptPreview({
        original: rawPrompt,
        optimized,
        visualPrompt,
        dialogue: form.dialogueText.trim() || dialogue,
        warning: payload?.warning,
        modelLabel: optimizeModelLabel,
      })
      if (optimized === rawPrompt) {
        toast({ title: '优化已返回，但内容无变化', description: payload?.warning || `文本模型：${optimizeModelLabel}；当前返回结果与原提示词一致。`, variant: 'default' })
        return
      }
      toast({ title: '已生成优化结果，请确认是否应用', description: `文本模型：${optimizeModelLabel}`, variant: 'success' })
    } catch (error: any) {
      toast({ title: '提示词优化失败', description: error?.message || '请稍后重试', variant: 'destructive' })
    } finally {
      setOptimizingPrompt(false)
    }
  }

  const renderForm = () => {
    const noModelsAvailable = activeModels.length === 0
    const disabledReason = noModelsAvailable
      ? `当前「${activeMeta.label}」没有可用视频模型，请切换分类或检查 model-status。`
      : ''
    const selectedGenerateModes = selectedModel ? getModelGenerateModes(selectedModel) : []
    const selectedHints = selectedModel ? capabilityHints(selectedModel, inferModelCategories(selectedModel)) : []
    const styleLabel = form.stylePreset === 'realistic' ? '真实环境 / 写实风格' : '动漫风格'
    const nativeAudioSupported = Boolean(selectedModel?.native_audio)
    const isKlingFamily = Boolean(selectedModel?.key && /kling|aiping|tencent-vclm/i.test(selectedModel.key))
    const maxReferenceImages = isKlingFamily ? 3 : null
    const imageSupportSummary = activeMenu === 'text'
      ? '文生：0 张图片，直接用提示词提交。'
      : activeMenu === 'image'
        ? '图生：1 张首帧图。多传不会变成“多图融合”，而是可能被当成多 clip。'
        : activeMenu === 'reference'
          ? `融合：1 张首帧图可选 + 多张参考图。${maxReferenceImages ? `当前模型建议最多 ${maxReferenceImages} 张参考图。` : '当前模型代码层未额外限死参考图张数。'}`
          : activeMenu === 'start-end'
            ? '首尾针：固定 2 张，首帧 + 尾帧。'
            : `人物一致性参考：1 张目标首帧 + 多张人物参考图。${maxReferenceImages ? `当前模型建议最多 ${maxReferenceImages} 张参考图。` : '当前模型代码层未额外限死参考图张数。'}`

    return (
      <Card className="border-white/10 bg-slate-900/60 text-slate-100">
        <CardHeader>
          <CardTitle>{activeMeta.label}表单</CardTitle>
          <CardDescription className="text-slate-400">{buildHelperText(activeMenu)}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid gap-4 md:grid-cols-2">
            <div className="space-y-3 md:col-span-2 rounded-xl border border-white/10 bg-slate-950/30 p-4">
              <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
                <div className="space-y-1">
                  <Label className="text-sm text-slate-100">提示词</Label>
                  <div className="text-xs text-slate-500">输入视频提示词、场景描述或镜头要求；可先写粗稿，再用下方文本模型做优化。</div>
                </div>
                <Button type="button" variant="outline" size="sm" className="self-start" onClick={optimizePromptForCurrentMode} disabled={optimizingPrompt || !form.prompt.trim()}>
                  {optimizingPrompt ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : <Sparkles className="mr-2 h-4 w-4" />}
                  优化提示词
                </Button>
              </div>

              <Textarea value={form.prompt} onChange={(e) => setForm((prev) => ({ ...prev, prompt: e.target.value }))} placeholder="输入视频提示词 / 场景描述 / 镜头要求" className="min-h-[160px]" />

              <div className="space-y-2">
                <Label className="text-xs text-slate-300">对白 / 旁白（可手动编辑，优先于自动提取）</Label>
                <Textarea
                  value={form.dialogueText}
                  onChange={(e) => setForm((prev) => ({ ...prev, dialogueText: e.target.value }))}
                  placeholder="可选：手动填写旁白、口播、对白。若填写，这里会优先作为 dialogues 提交。"
                  className="min-h-[120px]"
                />
                <div className="text-xs text-slate-500">如果优化结果里的“台词如下 / 旁白如下”提取不准，可以直接在这里手动改；创建视频时这里的内容优先提交到 dialogues。</div>
              </div>

              <div className="grid gap-3 lg:grid-cols-[minmax(0,320px)_1fr] lg:items-end">
                <div className="space-y-2">
                  <Label className="text-xs text-slate-300">优化用文本大模型</Label>
                  <Select value={form.optimizeTextModel} onValueChange={(value) => setForm((prev) => ({ ...prev, optimizeTextModel: value }))}>
                    <SelectTrigger><SelectValue placeholder="选择文本模型" /></SelectTrigger>
                    <SelectContent>
                      {optimizeTextModels.map((item) => (
                        <SelectItem key={item.value} value={item.value}>{item.label}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div className="rounded-lg border border-dashed border-white/10 bg-slate-900/40 px-3 py-2 text-xs leading-5 text-slate-500">
                  文本模型列表优先读取系统当前已配置的 LLM 模型；如果暂时取不到，会回退为系统默认文本模型。优化时会把当前视频模型、模式、风格、音频、画幅和时长一起传给后端。
                </div>
              </div>

              {promptPreview && (
                <div className="space-y-3 rounded-xl border border-cyan-400/20 bg-cyan-500/5 p-3">
                  <div className="grid gap-3 lg:grid-cols-2">
                    <div className="space-y-2">
                      <div className="text-xs font-medium text-slate-300">优化前</div>
                      <div className="max-h-56 overflow-auto rounded-lg border border-white/10 bg-slate-950/50 p-3 text-sm leading-6 text-slate-300 whitespace-pre-wrap">{promptPreview.original}</div>
                    </div>
                    <div className="space-y-2">
                      <div className="flex items-center justify-between gap-2">
                        <div className="text-xs font-medium text-cyan-200">优化结果（完整文本）</div>
                        <div className="text-[11px] text-slate-400">{promptPreview.modelLabel}</div>
                      </div>
                      <div className="max-h-56 overflow-auto rounded-lg border border-cyan-400/20 bg-slate-950/60 p-3 text-sm leading-6 text-slate-100 whitespace-pre-wrap">{promptPreview.optimized}</div>
                    </div>
                  </div>

                  <div className="grid gap-3 lg:grid-cols-2">
                    <div className="space-y-2">
                      <div className="text-xs font-medium text-emerald-200">最终视觉提示词（实际送视频生成器）</div>
                      <div className="max-h-56 overflow-auto rounded-lg border border-emerald-400/20 bg-slate-950/60 p-3 text-sm leading-6 text-slate-100 whitespace-pre-wrap">{promptPreview.visualPrompt || '（空）'}</div>
                    </div>
                    <div className="space-y-2">
                      <div className="text-xs font-medium text-violet-200">对白 / 旁白（实际送 dialogues）</div>
                      <div className="max-h-56 overflow-auto rounded-lg border border-violet-400/20 bg-slate-950/60 p-3 text-sm leading-6 text-slate-100 whitespace-pre-wrap">{promptPreview.dialogue || '（未提取到对白/旁白，且当前未手动填写，将不会单独提交 dialogues）'}</div>
                    </div>
                  </div>

                  <div className="rounded-lg border border-dashed border-cyan-400/20 bg-slate-950/30 px-3 py-2 text-xs leading-5 text-slate-400">
                    创建视频时会按上面两块自动拆分提交：视觉提示词进入 <span className="font-mono text-slate-200">scene_description / scene_descriptions</span>，对白/旁白进入 <span className="font-mono text-slate-200">dialogues</span>，因此“优化结果完整文本”和“最终送视频生成器的文本”可能不完全相同。
                  </div>

                  {promptPreview.warning && <div className="text-xs text-amber-300">提示：{promptPreview.warning}</div>}
                  <div className="flex flex-wrap gap-2">
                    <Button type="button" size="sm" onClick={() => {
                      setForm((prev) => ({ ...prev, prompt: promptPreview.optimized, dialogueText: prev.dialogueText || promptPreview.dialogue || '' }))
                      toast({ title: '已应用优化后的提示词', description: '创建视频时会按视觉提示词 / 对白自动拆分提交，手动填写的对白优先。', variant: 'success' })
                    }}>应用优化结果</Button>
                    <Button type="button" size="sm" variant="outline" onClick={() => setPromptPreview(null)}>关闭预览</Button>
                  </div>
                </div>
              )}
            </div>

            {(activeMenu === 'image' || activeMenu === 'reference' || activeMenu === 'start-end') && (
              <div className="space-y-2">
                <Label>首帧图片 URL</Label>
                <Input value={form.sourceImageUrl} onChange={(e) => setForm((prev) => ({ ...prev, sourceImageUrl: e.target.value }))} placeholder="https://..." />
                {activeMenu === 'image' && (
                  <>
                    <div className="text-xs text-slate-500">图生当前按真实链路只支持 1 张首帧图；也可上传本地图片（会自动上传到 storage-service）</div>
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
                    <div className="text-xs text-slate-500">也可上传多张本地参考图{maxReferenceImages ? `（当前模型建议最多 ${maxReferenceImages} 张）` : ''}</div>
                    <Input type="file" accept="image/*" multiple onChange={(e) => setForm((prev) => ({ ...prev, referenceImageFiles: Array.from(e.target.files || []) }))} />
                    {form.referenceImageFiles.length > 0 && <div className="text-xs text-slate-400">已选择 {form.referenceImageFiles.length} 张参考图{maxReferenceImages ? ` / 建议上限 ${maxReferenceImages} 张` : ''}</div>}
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
                    <SelectItem key={model.key} value={model.key}>{getModelDisplayName(model)}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
              {noModelsAvailable && <div className="text-xs text-amber-300">当前分类暂无可用模型，提交已禁用。</div>}
            </div>

            <div className="space-y-2">
              <Label>画面风格</Label>
              <Select value={form.stylePreset} onValueChange={(value: 'anime' | 'realistic') => setForm((prev) => ({ ...prev, stylePreset: value }))}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  <SelectItem value="anime">动漫风格</SelectItem>
                  <SelectItem value="realistic">真实环境 / 写实风格</SelectItem>
                </SelectContent>
              </Select>
              <div className="text-xs text-slate-500">这个风格会真实写入本次视频任务的 `style_preset`，用于提示模型偏向动漫感还是写实真人感。</div>
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

            {nativeAudioSupported && (
              <div className="space-y-2 md:col-span-2">
                <Label>原生音频</Label>
                <Select value={form.generateAudio ? 'true' : 'false'} onValueChange={(value) => setForm((prev) => ({ ...prev, generateAudio: value === 'true' }))}>
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>
                    <SelectItem value="false">关闭</SelectItem>
                    <SelectItem value="true">开启</SelectItem>
                  </SelectContent>
                </Select>
                <div className="text-xs text-slate-500">当前模型支持原生音频，会真实写入 `render_config.generate_audio`。关闭时只生成视频画面，不请求模型内置音频。</div>
              </div>
            )}
          </div>

          <div className="rounded-xl border border-white/10 bg-white/[0.03] p-4">
            <div className="text-sm font-medium text-white">当前参数摘要</div>
            <div className="mt-3 grid gap-3 text-xs text-slate-300 md:grid-cols-2 xl:grid-cols-3">
              <div className="rounded-lg border border-white/10 bg-slate-950/40 px-3 py-2">
                <div className="text-[11px] text-slate-500">当前分类</div>
                <div className="mt-1 text-white">{activeMeta.label}</div>
              </div>
              <div className="rounded-lg border border-white/10 bg-slate-950/40 px-3 py-2">
                <div className="text-[11px] text-slate-500">当前模型</div>
                <div className="mt-1 break-all text-white">{selectedModel ? getModelDisplayName(selectedModel) : '未选择'}</div>
              </div>
              <div className="rounded-lg border border-white/10 bg-slate-950/40 px-3 py-2">
                <div className="text-[11px] text-slate-500">画面风格</div>
                <div className="mt-1 text-white">{styleLabel}</div>
              </div>
              <div className="rounded-lg border border-white/10 bg-slate-950/40 px-3 py-2">
                <div className="text-[11px] text-slate-500">画幅比例</div>
                <div className="mt-1 text-white">{form.aspectRatio || selectedAspectValues[0]?.label || '自动'}</div>
              </div>
              <div className="rounded-lg border border-white/10 bg-slate-950/40 px-3 py-2">
                <div className="text-[11px] text-slate-500">分辨率</div>
                <div className="mt-1 text-white">{form.resolution || selectedResolutionValues[0]?.label || '自动'}</div>
              </div>
              <div className="rounded-lg border border-white/10 bg-slate-950/40 px-3 py-2">
                <div className="text-[11px] text-slate-500">时长</div>
                <div className="mt-1 text-white">{form.duration || selectedDurationValues[0]?.label || '默认'}</div>
              </div>
              <div className="rounded-lg border border-white/10 bg-slate-950/40 px-3 py-2 md:col-span-2 xl:col-span-3">
                <div className="text-[11px] text-slate-500">模式参数</div>
                <div className="mt-2 flex flex-wrap gap-2">
                  {(selectedGenerateModes.length > 0 ? selectedGenerateModes : ['未返回 generate_mode']).map((mode) => (
                    <span key={mode} className="rounded-md border border-white/10 px-2 py-1 text-[11px] text-slate-200">{mode}</span>
                  ))}
                </div>
              </div>
              <div className="rounded-lg border border-white/10 bg-slate-950/40 px-3 py-2 md:col-span-2 xl:col-span-3">
                <div className="text-[11px] text-slate-500">能力提示</div>
                <div className="mt-2 flex flex-wrap gap-2">
                  {(selectedHints.length > 0 ? selectedHints : ['暂无能力提示']).map((hint) => (
                    <span key={hint} className="rounded-md border border-cyan-400/20 bg-cyan-400/10 px-2 py-1 text-[11px] text-cyan-200">{hint}</span>
                  ))}
                </div>
              </div>
              <div className="rounded-lg border border-white/10 bg-slate-950/40 px-3 py-2">
                <div className="text-[11px] text-slate-500">原生音频能力</div>
                <div className="mt-1 text-white">{nativeAudioSupported ? '支持' : '不支持'}</div>
              </div>
              <div className="rounded-lg border border-white/10 bg-slate-950/40 px-3 py-2">
                <div className="text-[11px] text-slate-500">当前音频配置</div>
                <div className="mt-1 text-white">{nativeAudioSupported ? (form.generateAudio ? '已开启原生音频' : '未开启原生音频') : '当前模型无此配置'}</div>
              </div>
              <div className="rounded-lg border border-white/10 bg-slate-950/40 px-3 py-2 md:col-span-2 xl:col-span-3">
                <div className="text-[11px] text-slate-500">当前输入要求</div>
                <div className="mt-1 leading-5 text-slate-300">
                  {activeMenu === 'text'
                    ? '只需要提示词；当前风格选择会影响文生视频画面偏向。'
                    : activeMenu === 'image'
                      ? '需要首帧图，可填 URL 或上传本地图片。'
                      : activeMenu === 'reference'
                        ? '至少需要首帧图或参考图；参考图可多张。'
                        : activeMenu === 'start-end'
                          ? '必须同时提供首帧图与尾帧图。'
                          : '需要目标首帧 + 主参考脸/人物参考图，用人物一致性链提交。'}
                </div>
              </div>
              <div className="rounded-lg border border-white/10 bg-slate-950/40 px-3 py-2 md:col-span-2 xl:col-span-3">
                <div className="text-[11px] text-slate-500">图片支持真相</div>
                <div className="mt-1 leading-5 text-slate-300">{imageSupportSummary}</div>
              </div>
            </div>
          </div>

          {submitResult && (
            <div className="rounded-xl border border-emerald-400/20 bg-emerald-400/5 p-4 text-sm text-emerald-100">
              <div className="font-medium">任务已创建：{submitResult.mode === 'text' ? '文生视频' : submitResult.mode === 'image' ? '图生视频' : submitResult.mode === 'reference' ? '融合生视频' : submitResult.mode === 'start-end' ? '首尾针视频' : '人物一致性参考'}</div>
              <div className="mt-1">项目 ID：{submitResult.projectId}，任务 ID：{submitResult.taskId}</div>
              <div className="mt-3 grid gap-2 text-xs text-emerald-50/90 md:grid-cols-2">
                <div>模型：{submitResult.modelLabel || getFallbackModelDisplayName(submitResult.modelName)}</div>
                <div>实际模式：{submitResult.generateMode}</div>
                <div>首帧输入：{submitResult.hasStartImage ? '有' : '无'}</div>
                <div>尾帧输入：{submitResult.hasTailImage ? '有' : '无'}</div>
                <div>首帧数量：{submitResult.sourceCount}</div>
                <div>参考图数量：{submitResult.referenceCount}</div>
                <div>原生音频：{submitResult.generateAudio ? '开启' : '关闭/不支持'}</div>
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
            <CardDescription className="text-slate-400">二级菜单（显示当前前端归类命中的模型数量）</CardDescription>
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
                    <div className="flex flex-col items-end gap-1">
                      <span className="rounded-full bg-white/8 px-2 py-1 text-xs text-slate-300">{count}</span>
                      <span className="text-[10px] text-slate-500">models</span>
                    </div>
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
                <div className="space-y-3 rounded-xl border border-amber-400/20 bg-amber-400/5 p-4 text-sm text-amber-100">
                  <div>当前分类下没有可识别模型。</div>
                  <div className="text-xs text-amber-100/80">可先检查 `/api/v1/videos/model-status` 是否返回了 `generate_mode`，或切换其它分类。</div>
                </div>
              ) : (
                <div className="rounded-xl border border-slate-700/60 bg-slate-950/40 p-4 text-sm text-slate-300 space-y-3">
                  <div>当前分类的模型展开诊断列表先统一隐藏，避免页面被大块模型卡片干扰。</div>
                  <div className="grid gap-3 md:grid-cols-3">
                    <div className="rounded-lg border border-white/10 bg-white/[0.03] px-3 py-2">
                      <div className="text-[11px] text-slate-500">当前分类</div>
                      <div className="mt-1 font-medium text-white">{activeMeta.label}</div>
                    </div>
                    <div className="rounded-lg border border-white/10 bg-white/[0.03] px-3 py-2">
                      <div className="text-[11px] text-slate-500">可识别模型数</div>
                      <div className="mt-1 font-medium text-white">{activeModels.length}</div>
                    </div>
                    <div className="rounded-lg border border-white/10 bg-white/[0.03] px-3 py-2">
                      <div className="text-[11px] text-slate-500">当前选中模型</div>
                      <div className="mt-1 font-medium text-white break-all">{selectedModel ? getModelDisplayName(selectedModel) : '未选择'}</div>
                    </div>
                  </div>
                  <div className="text-xs text-slate-400">
                    保留左侧分类入口、表单与真实提交链；如需再看详细 raw key / generate_mode / 参数展开，再单独做诊断开关。
                  </div>
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
