'use client'

import { useEffect, useMemo, useState, useRef } from 'react'
import { useRouter } from 'next/navigation'
import useSWR from 'swr'
import {
  ArrowLeft,
  ArrowRight,
  Upload,
  X,
  Check,
  FileText,
  Image,
  Video,
  Mic,
  Type,
  Sparkles,
  Loader2,
} from 'lucide-react'
import { projectAPI, modelAPI } from '@/lib/api'
import { ensureProjectMediaTag, normalizeProjectMediaKind, PROJECT_MEDIA_META, stripProjectMediaTags, type ProjectMediaKind } from '@/lib/project-media'
import { consumePendingProjectDraft } from '@/lib/project-draft'
import { pickPreferredModel } from '@/lib/model-selection'
import {
  normalizeVideoStylePreset,
  isLiveActionStyle,
  VIDEO_STYLE_PRESETS,
  VIDEO_MOTION_OPTIONS,
} from '@/lib/video-style-config'
import type { Model, Project } from '@/types'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { Slider } from '@/components/ui/slider'
import { Badge } from '@/components/ui/badge'
import { useToast } from '@/components/ui/toast'

// === Extracted create-page modules ===
import { STYLE_OPTIONS, ASPECT_RATIOS, RESOLUTIONS, WATERMARK_POSITIONS, VIDEO_STYLE_CREATION_SPOTLIGHT_KEYS, VIDEO_STYLE_PRESET_LABELS, VIDEO_MOTION_MODE_LABELS, VIDEO_RUNTIME_KEY_LABELS, CREATE_PAGE_DISPLAY, type CreatePageDisplayConfig } from '@/lib/projects/new/create-config'
import { initialFormData, type FormData } from '@/lib/projects/new/form-types'
import { PROJECT_PRESET_TEMPLATES, type ProjectPresetTemplate, summarizePresetTemplate, buildDraftScriptFileName, recommendPresetTemplate } from '@/lib/projects/new/preset-templates'
import { MEDIA_STARTER_TEMPLATES, MEDIA_STYLE_OPTIONS, type MediaStarterTemplate } from '@/lib/projects/new/media-templates'
import { mapVideoModelToRuntimeKey, findPreferredVideoModelId } from '@/lib/projects/models'
import { StepIndicator } from '@/components/projects/new/StepIndicator'
import { ModelSelector } from '@/components/projects/new/ModelSelector'
import { CostEstimation } from '@/components/projects/new/CostEstimation'












// ─── Page ────────────────────────────────────────────────────

export default function NewProjectPage() {
  const router = useRouter()
  const { toast } = useToast()
  const [mediaKind, setMediaKind] = useState(() => normalizeProjectMediaKind(null))
  const mediaMeta = PROJECT_MEDIA_META[mediaKind]
  const display = CREATE_PAGE_DISPLAY[mediaKind]
  const [step, setStep] = useState(1)
  const [form, setForm] = useState<FormData>(initialFormData)
  const [selectedPresetTemplate, setSelectedPresetTemplate] = useState<string>('custom')
  const [submitting, setSubmitting] = useState(false)
  const [errors, setErrors] = useState<Record<string, string>>({})
  const logoInputRef = useRef<HTMLInputElement>(null)
  const scriptInputRef = useRef<HTMLInputElement>(null)
  const presetDrivenKeys: (keyof FormData)[] = [
    'style_tags',
    'video_model_id',
    'video_style_preset',
    'video_mode',
    'enable_dubbing',
    'enable_subtitle',
    'storyboard_aspect_ratio',
    'storyboard_resolution',
    'storyboard_duration',
    'consistency_strength',
    'video_motion_mode',
  ]

  const update = <K extends keyof FormData>(key: K, value: FormData[K]) => {
    const nextValue =
      key === 'video_style_preset'
        ? (normalizeVideoStylePreset(String(value)) as FormData[K])
        : value

    setForm((prev) => {
      const nextForm = { ...prev, [key]: nextValue } as any
      
      // [智能联动] 根据选择的视频模型，展示/调整默认参数
      if (key === 'video_model_id' && value) {
        const selectedModel = videoModels.find((m) => m.id === Number(value))
        if (selectedModel) {
          const name = selectedModel.name.toLowerCase()
          if (name.includes('kling') || name.includes('runway') || name.includes('sora')) {
             nextForm.storyboard_duration = 5
          } else {
             nextForm.storyboard_duration = 4
          }
        }
      }
      // [智能联动] 根据图片模型调整默认参数
      if (key === 'image_model_id' && value) {
        const selectedModel = imageModels.find((m) => m.id === Number(value))
        if (selectedModel) {
          const name = selectedModel.name.toLowerCase()
          if (name.includes('mj') || name.includes('midjourney')) {
             nextForm.storyboard_resolution = '1080p'
          } else {
             nextForm.storyboard_resolution = '1080p'
          }
        }
      }
      return nextForm
    })

    if (selectedPresetTemplate !== 'custom' && presetDrivenKeys.includes(key)) {
      setSelectedPresetTemplate('custom')
    }
    setErrors((prev) => {
      const next = { ...prev }
      delete next[key]
      return next
    })
  }

  // Fetch models
  const { data: textModelsData, isLoading: loadingText } = useSWR(
    'models-text',
    () => modelAPI.list({ type: 'llm', sort_by: 'priority' }) as unknown as Promise<{ data: Model[] }>
  )
  const { data: imageModelsData, isLoading: loadingImage } = useSWR(
    'models-image',
    () => modelAPI.list({ type: 'image' }) as unknown as Promise<{ data: Model[] }>
  )
  const { data: videoModelsData, isLoading: loadingVideo } = useSWR(
    'models-video',
    () => modelAPI.list({ type: 'video' }) as unknown as Promise<{ data: Model[] }>
  )
  const { data: ttsModelsData, isLoading: loadingTTS } = useSWR(
    'models-tts',
    () => modelAPI.list({ type: 'audio' }) as unknown as Promise<{ data: Model[] }>
  )

  const textModels: Model[] = ((textModelsData as { data?: Model[] })?.data ?? []).filter((m) => m.is_active)
  const imageModels: Model[] = (imageModelsData as { data?: Model[] })?.data ?? []
  const videoModels: Model[] = (videoModelsData as { data?: Model[] })?.data ?? []
  const ttsModels: Model[] = (ttsModelsData as { data?: Model[] })?.data ?? []
  const modelSelectorDefs = {
    text: {
      label: '文本模型',
      icon: <Type className="h-4 w-4 text-primary-500" />,
      models: textModels,
      isLoading: loadingText,
      value: form.text_model_id,
      onChange: (v: number | undefined) => update('text_model_id', v),
    },
    image: {
      label: '图片模型',
      icon: <Image className="h-4 w-4 text-purple-500" />,
      models: imageModels,
      isLoading: loadingImage,
      value: form.image_model_id,
      onChange: (v: number | undefined) => update('image_model_id', v),
    },
    video: {
      label: '视频模型',
      icon: <Video className="h-4 w-4 text-green-500" />,
      models: videoModels,
      isLoading: loadingVideo,
      value: form.video_model_id,
      onChange: (v: number | undefined) => update('video_model_id', v),
    },
    tts: {
      label: '语音合成模型',
      icon: <Mic className="h-4 w-4 text-pink-500" />,
      models: ttsModels,
      isLoading: loadingTTS,
      value: form.tts_model_id,
      onChange: (v: number | undefined) => update('tts_model_id', v),
    },
  } satisfies Record<CreatePageDisplayConfig['modelSelectorKeys'][number], {
    label: string
    icon: React.ReactNode
    models: Model[]
    isLoading: boolean
    value: number | undefined
    onChange: (v: number | undefined) => void
  }>
  const visibleModelSelectors = display.modelSelectorKeys.map((key) => ({ key, ...modelSelectorDefs[key] }))
  const mediaStarterTemplates = MEDIA_STARTER_TEMPLATES[mediaKind]
  const visibleStyleTags = stripProjectMediaTags(form.style_tags)
  const mediaStyleOptions = useMemo(() => {
    const base = MEDIA_STYLE_OPTIONS[mediaKind]
    const extras = visibleStyleTags.filter((tag) => !base.includes(tag))
    return [...base, ...extras]
  }, [mediaKind, visibleStyleTags])
  const storyboardFieldCount = 2 + (display.showStoryboardDuration ? 1 : 0)
  const storyboardGridClassName =
    storyboardFieldCount >= 4
      ? 'grid gap-4 sm:grid-cols-2 xl:grid-cols-4'
      : storyboardFieldCount === 3
      ? 'grid gap-4 sm:grid-cols-2 xl:grid-cols-3'
      : 'grid gap-4 sm:grid-cols-2'
  const recommendedPreset = useMemo(
    () => recommendPresetTemplate(form.title, form.description, form.scriptPreview),
    [form.description, form.scriptPreview, form.title]
  )

  // File handlers
  const handleLogoSelect = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    update('logoFile', file)
    const reader = new FileReader()
    reader.onload = () => update('logoPreview', reader.result as string)
    reader.readAsDataURL(file)
  }

  const handleScriptSelect = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    update('scriptFile', file)
    if (file.name.endsWith('.txt') || file.name.endsWith('.md')) {
      const reader = new FileReader()
      reader.onload = () => {
        const text = reader.result as string
        update('scriptPreview', text.slice(0, 2000))
      }
      reader.readAsText(file)
    } else {
      update('scriptPreview', `已选择文件: ${file.name} (${(file.size / 1024).toFixed(1)} KB)`)
    }
  }

  const toggleStyleTag = (tag: string) => {
    setForm((prev) => ({
      ...prev,
      style_tags: prev.style_tags.includes(tag)
        ? prev.style_tags.filter((t) => t !== tag)
        : [...prev.style_tags, tag],
    }))
    if (selectedPresetTemplate !== 'custom') {
      setSelectedPresetTemplate('custom')
    }
  }

  const applyMediaStarterTemplate = (template: MediaStarterTemplate) => {
    setForm((prev) => {
      const mergedTags = Array.from(new Set([...template.styleTags, ...stripProjectMediaTags(prev.style_tags)]))
      return {
        ...prev,
        title: prev.title.trim() ? prev.title : template.title,
        description: prev.description.trim() ? prev.description : template.description,
        style_tags: ensureProjectMediaTag(mergedTags, mediaKind),
        target_episodes: template.targetEpisodes && display.showTargetEpisodes && prev.target_episodes === initialFormData.target_episodes
          ? template.targetEpisodes
          : prev.target_episodes,
        storyboard_aspect_ratio: template.storyboardAspectRatio ?? prev.storyboard_aspect_ratio,
        storyboard_resolution: template.storyboardResolution ?? prev.storyboard_resolution,
        consistency_strength: template.consistencyStrength ?? prev.consistency_strength,
      }
    })
    toast({ title: `已应用${template.label}创建建议`, description: template.desc, variant: 'success' })
  }

  const applyPresetTemplate = (presetKey: string) => {
    const preset = PROJECT_PRESET_TEMPLATES.find((item) => item.key === presetKey)
    if (!preset) return
    const preferredVideoModelId = findPreferredVideoModelId(videoModels, preset.preferredVideoRuntimeKeys)
    setForm((prev) => ({
      ...prev,
      style_tags: Array.from(preset.styleTags),
      video_model_id: preferredVideoModelId ?? prev.video_model_id,
      video_style_preset: normalizeVideoStylePreset(preset.videoStylePreset),
      video_motion_mode: preset.videoMotionMode,
      storyboard_aspect_ratio: preset.storyboardAspectRatio,
      storyboard_resolution: preset.storyboardResolution,
      storyboard_duration: preset.storyboardDuration,
      consistency_strength: preset.consistencyStrength,
      enable_dubbing: preset.enableDubbing,
      enable_subtitle: preset.enableSubtitle,
      video_mode: preset.videoMode,
    }))
    setSelectedPresetTemplate(preset.key)
    toast({ title: `已应用模板：${preset.label}`, description: preset.desc, variant: 'success' })
  }

  useEffect(() => {
    if (typeof window === 'undefined') return
    const searchParams = new URLSearchParams(window.location.search)
    const mediaFromQuery = searchParams.get('media')
    const draftId = searchParams.get('draft')
    let nextMediaKind = normalizeProjectMediaKind(mediaFromQuery)

    if (draftId) {
      const draft = consumePendingProjectDraft(draftId)
      if (draft) {
        nextMediaKind = normalizeProjectMediaKind(mediaFromQuery ?? draft.media ?? null)
        setForm((prev) => {
          const next: FormData = {
            ...prev,
            title: draft.title?.trim() || prev.title,
            description: draft.description?.trim() || prev.description,
            target_episodes: draft.targetEpisodes && draft.targetEpisodes > 0 ? draft.targetEpisodes : prev.target_episodes,
            style_tags: draft.styleTags?.length ? draft.styleTags : prev.style_tags,
          }

          if (draft.scriptContent?.trim()) {
            const fileName = buildDraftScriptFileName(draft.title || prev.title, draft.scriptFileName)
            next.scriptFile = new File(
              [draft.scriptContent],
              fileName,
              { type: draft.scriptMimeType || 'text/plain;charset=utf-8' }
            )
            next.scriptPreview = draft.scriptContent.slice(0, 2000)
          }

          return next
        })

        toast({
          title: '已自动带入小说/剧本',
          description: '你可以继续调整模型、风格和分镜配置',
          variant: 'success',
        })
      }

      searchParams.delete('draft')
      const nextQuery = searchParams.toString()
      window.history.replaceState({}, '', `${window.location.pathname}${nextQuery ? `?${nextQuery}` : ''}`)
    }

    setMediaKind(nextMediaKind)
  }, [toast])

  useEffect(() => {
    setForm((prev) => {
      const nextTextModelId = prev.text_model_id ?? pickPreferredModel(textModels)?.id
      const nextImageModelId = prev.image_model_id ?? pickPreferredModel(imageModels)?.id
      const nextVideoModelId = prev.video_model_id ?? pickPreferredModel(videoModels)?.id
      const nextTtsModelId = prev.tts_model_id ?? pickPreferredModel(ttsModels)?.id

      if (
        nextTextModelId === prev.text_model_id &&
        nextImageModelId === prev.image_model_id &&
        nextVideoModelId === prev.video_model_id &&
        nextTtsModelId === prev.tts_model_id
      ) {
        return prev
      }

      return {
        ...prev,
        text_model_id: nextTextModelId,
        image_model_id: nextImageModelId,
        video_model_id: nextVideoModelId,
        tts_model_id: nextTtsModelId,
      }
    })
  }, [imageModels, textModels, ttsModels, videoModels])

  useEffect(() => {
    setForm((prev) => {
      const nextTags = ensureProjectMediaTag(prev.style_tags, mediaKind)
      const next: FormData = {
        ...prev,
        style_tags: nextTags,
      }

      if (mediaKind === 'comics' && prev.storyboard_aspect_ratio === initialFormData.storyboard_aspect_ratio) {
        next.storyboard_aspect_ratio = '3:4'
      }

      if (mediaKind === 'music' || mediaKind === 'image') {
        if (prev.enable_dubbing === initialFormData.enable_dubbing) next.enable_dubbing = false
        if (prev.enable_subtitle === initialFormData.enable_subtitle) next.enable_subtitle = false
      }

      if (mediaKind === 'image' && prev.storyboard_aspect_ratio === initialFormData.storyboard_aspect_ratio) {
        next.storyboard_aspect_ratio = '1:1'
      }

      const unchanged =
        next.style_tags.length === prev.style_tags.length &&
        next.style_tags.every((tag, index) => tag === prev.style_tags[index]) &&
        next.storyboard_aspect_ratio === prev.storyboard_aspect_ratio &&
        next.enable_dubbing === prev.enable_dubbing &&
        next.enable_subtitle === prev.enable_subtitle

      return unchanged ? prev : next
    })
  }, [mediaKind])

  // Validation
  const validateStep = (s: number): boolean => {
    const newErrors: Record<string, string> = {}
    if (s === 1) {
      if (!form.title.trim()) newErrors.title = '请输入项目名称'
      if (display.showTargetEpisodes && form.target_episodes < 1) newErrors.target_episodes = '至少为 1'
    }
    setErrors(newErrors)
    return Object.keys(newErrors).length === 0
  }

  const goNext = () => {
    if (validateStep(step)) setStep((s) => Math.min(s + 1, 3))
  }
  const goBack = () => setStep((s) => Math.max(s - 1, 1))

  // Submit
  const handleSubmit = async () => {
    if (!validateStep(step)) return
    setSubmitting(true)
    try {
      const payload: Record<string, unknown> = {
        title: form.title.trim(),
        description: form.description.trim(),
        project_type: mediaKind,
        style_tags: ensureProjectMediaTag(stripProjectMediaTags(form.style_tags), mediaKind),
      }

      if (display.showTargetEpisodes) {
        payload.target_episodes = form.target_episodes
      }
      if (display.modelSelectorKeys.includes('text')) {
        payload.text_model_id = form.text_model_id
      }
      if (display.modelSelectorKeys.includes('image')) {
        payload.image_model_id = form.image_model_id
      }
      if (display.modelSelectorKeys.includes('video')) {
        payload.video_model_id = form.video_model_id
      }
      if (display.modelSelectorKeys.includes('tts')) {
        payload.tts_model_id = form.tts_model_id
      }
      if (display.showVideoMode) {
        payload.video_mode = form.video_mode
      }
      if (display.showDubSubtitle) {
        payload.enable_dubbing = form.enable_dubbing
        payload.enable_subtitle = form.enable_subtitle
      }
      if (display.showWatermark) {
        payload.watermark_config = {
          enabled: form.watermark_enabled,
          type: form.watermark_type,
          text: form.watermark_text,
          position: form.watermark_position,
          opacity: form.watermark_opacity / 100,
          size: 'medium',
          apply_to: ['video'],
        }
      }
      if (display.showStoryboardConfig) {
        payload.storyboard_config = {
          aspect_ratio: form.storyboard_aspect_ratio,
          resolution: form.storyboard_resolution,
          ...(display.showStoryboardDuration ? { duration: form.storyboard_duration } : {}),
          ...(display.showVideoMode ? { video_mode: form.video_mode } : {}),
          ...(display.showVideoStyle ? { style_preset: form.video_style_preset } : {}),
          ...(display.showVideoMotion ? { motion_mode: form.video_motion_mode } : {}),
          ...(display.showVideoStyle && isLiveActionStyle(form.video_style_preset) && form.liveaction_region ? { region: form.liveaction_region.trim() } : {}),
          ...(display.showVideoStyle && isLiveActionStyle(form.video_style_preset) && form.liveaction_era ? { era: form.liveaction_era.trim() } : {}),
          ...(display.showVideoStyle && isLiveActionStyle(form.video_style_preset) && form.liveaction_ethnicity ? { ethnicity: form.liveaction_ethnicity.trim() } : {}),
        }
      }
      if (display.showConsistency) {
        payload.consistency_strength = form.consistency_strength / 100
      }

      const res = (await projectAPI.create(payload as never)) as { data: { id: number } }
      const projectId = res.data.id
      const videoRuntimeKey = mapVideoModelToRuntimeKey(videoModels.find((m) => m.id === form.video_model_id))

      // Persist the video model key into storyboard_config so the backend scene-split
      // can apply model-specific duration constraints (Kling: 5/10s, Vidu: 4/8s, etc.).
      if (videoRuntimeKey && display.showStoryboardConfig) {
        await projectAPI.update(projectId, {
          storyboard_config: { ...(payload.storyboard_config ?? {}), video_model: videoRuntimeKey },
        } as Partial<Project>)
      }

      if (form.scriptFile) {
        await projectAPI.uploadScript(projectId, form.scriptFile)
        // 上传了剧本文件时，自动触发分集+资源提取+分镜提取全链路（fire-and-forget）
        void projectAPI.generateEpisodes(projectId, undefined, { autoStoryboard: true })
      }

      if (mediaKind === 'video') {
        window.localStorage.setItem(`project-video-style-selection:${projectId}`, form.video_style_preset)
        window.localStorage.setItem(`project-video-motion-selection:${projectId}`, form.video_motion_mode)
        if (videoRuntimeKey) {
          window.localStorage.setItem(`project-video-model-selection:${projectId}`, videoRuntimeKey)
        }
      }

      const autoStart = form.scriptFile ? '?autoStart=1' : ''
      toast({ title: `${mediaMeta.label}项目已创建`, description: `「${form.title}」创建成功`, variant: 'success' })
      router.push(mediaKind === 'video' ? `/projects/${projectId}${autoStart}` : `${mediaMeta.listHref}?project=${projectId}`)
    } catch {
      toast({ title: '创建失败', description: '请检查输入后重试', variant: 'destructive' })
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="mx-auto max-w-5xl space-y-6 pb-12">
      {/* Header */}
      <div className="overflow-hidden rounded-[28px] border border-surface-200/70 bg-gradient-to-br from-slate-950 via-violet-950 to-slate-900 p-6 text-white shadow-sm">
        <div className="flex flex-col gap-5 lg:flex-row lg:items-start lg:justify-between">
          <div className="max-w-2xl">
            <div className="mb-4 inline-flex items-center gap-2 rounded-full border border-white/10 bg-white/10 px-3 py-1.5 text-xs font-medium text-surface-100 backdrop-blur">
              <Sparkles className="h-3.5 w-3.5 text-primary-300" />
              创建 {mediaMeta.label} 项目
            </div>
            <div className="flex items-start gap-4">
              <Button
                variant="ghost"
                size="icon"
                className="mt-0.5 shrink-0 rounded-2xl border border-white/10 bg-white/10 text-white hover:bg-white/15 hover:text-white"
                onClick={() => router.push(mediaMeta.listHref)}
                title={`返回${mediaMeta.listTitle}`}
              >
                <ArrowLeft className="h-5 w-5" />
              </Button>
              <div>
                <h2 className="text-2xl font-semibold tracking-tight">{mediaMeta.createTitle}</h2>
                <p className="mt-2 text-sm leading-6 text-surface-300">{mediaMeta.createDescription}</p>
              </div>
            </div>
          </div>

          <div className="grid gap-3 sm:grid-cols-3">
            <div className="rounded-2xl border border-white/10 bg-white/10 p-4 backdrop-blur">
              <p className="text-xs uppercase tracking-[0.2em] text-surface-300">流程</p>
              <p className="mt-2 text-lg font-semibold text-white">3 步完成</p>
              <p className="mt-1 text-xs text-surface-400">{display.flowHint}</p>
            </div>
            <div className="rounded-2xl border border-white/10 bg-white/10 p-4 backdrop-blur">
              <p className="text-xs uppercase tracking-[0.2em] text-surface-300">当前介质</p>
              <p className="mt-2 text-lg font-semibold text-white">{mediaMeta.label}</p>
              <p className="mt-1 text-xs text-surface-400">会自动写入对应项目类型</p>
            </div>
            <div className="rounded-2xl border border-white/10 bg-white/10 p-4 backdrop-blur">
              <p className="text-xs uppercase tracking-[0.2em] text-surface-300">创建后</p>
              <p className="mt-2 text-lg font-semibold text-white">{display.afterCreateTitle}</p>
              <p className="mt-1 text-xs text-surface-400">{display.afterCreateHint}</p>
            </div>
          </div>
        </div>
      </div>

      {/* Step indicator */}
      <div className="rounded-[24px] border border-surface-200 bg-white/80 p-4 shadow-sm backdrop-blur">
        <div className="mb-3 flex items-center justify-between gap-3">
          <div>
            <p className="text-sm font-medium text-surface-900">创建向导</p>
            <p className="text-xs text-surface-500">
              第 {step} 步 / 3：{display.stepDescriptions[step - 1]}
            </p>
          </div>
          <Badge variant="outline" className="rounded-full px-3 py-1 text-xs">
            {mediaMeta.label} 项目
          </Badge>
        </div>
        <StepIndicator current={step} steps={display.stepLabels} />
      </div>

      {/* Step Content */}
      <Card className="overflow-hidden rounded-[28px] border-surface-200 shadow-sm">
        <CardContent className="space-y-6 bg-gradient-to-b from-white to-surface-50/60 pt-6">
          {/* ─── Step 1: Basic Info ────────────────────────── */}
          {step === 1 && (
            <>
              <div className="space-y-2">
                <Label htmlFor="title">
                  项目名称 <span className="text-red-500">*</span>
                </Label>
                <Input
                  id="title"
                  placeholder={display.titlePlaceholder}
                  value={form.title}
                  onChange={(e) => update('title', e.target.value)}
                  className={errors.title ? 'border-red-500' : ''}
                />
                {errors.title && <p className="text-xs text-red-500">{errors.title}</p>}
              </div>

              <div className="space-y-2">
                <Label htmlFor="description">项目描述</Label>
                <Textarea
                  id="description"
                  placeholder={display.descriptionPlaceholder}
                  value={form.description}
                  onChange={(e) => update('description', e.target.value)}
                  rows={3}
                />
              </div>

              {display.showPresetTemplates && recommendedPreset ? (
                <div className="rounded-xl border border-primary-200 bg-primary-50 p-4">
                  <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
                    <div>
                      <p className="flex items-center gap-2 text-sm font-medium text-primary-700">
                        <Sparkles className="h-4 w-4" />
                        已为当前项目推荐模板：{recommendedPreset.preset.label}
                      </p>
                      <p className="mt-1 text-xs text-primary-600">{recommendedPreset.reason}</p>
                      <p className="mt-1 text-xs text-surface-500">{recommendedPreset.preset.desc}</p>
                      <div className="mt-2 flex flex-wrap gap-1.5">
                        {summarizePresetTemplate(recommendedPreset.preset).slice(0, 4).map((item) => (
                          <span
                            key={`${recommendedPreset.preset.key}-${item.label}`}
                            className="rounded-full border border-primary-200 bg-white px-2 py-0.5 text-[10px] text-primary-700"
                          >
                            {item.label}·{item.value}
                          </span>
                        ))}
                      </div>
                    </div>
                    <Button
                      type="button"
                      size="sm"
                      onClick={() => applyPresetTemplate(recommendedPreset.preset.key)}
                    >
                      一键套用推荐模板
                    </Button>
                  </div>
                </div>
              ) : null}

              {display.showPresetTemplates ? (
                <div className="space-y-3 rounded-xl border border-surface-200 bg-surface-50/70 p-4">
                  <div className="flex flex-col gap-2 md:flex-row md:items-center md:justify-between">
                    <div>
                      <Label className="flex items-center gap-2 text-sm font-medium">
                        <Sparkles className="h-4 w-4 text-amber-500" />
                        题材预设模板
                      </Label>
                      <p className="mt-1 text-xs text-surface-500">
                        一键带出整套项目默认参数，包括视觉基调、推荐模型、画幅、运镜和默认时长，后续步骤仍可继续微调。
                      </p>
                    </div>
                    <Badge variant="outline" className="w-fit">
                      当前：{selectedPresetTemplate === 'custom'
                        ? '自定义'
                        : PROJECT_PRESET_TEMPLATES.find((item) => item.key === selectedPresetTemplate)?.label ?? '自定义'}
                    </Badge>
                  </div>
                  <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
                    {PROJECT_PRESET_TEMPLATES.map((preset) => {
                      const active = preset.key === selectedPresetTemplate
                      return (
                        <button
                          key={preset.key}
                          type="button"
                          onClick={() => applyPresetTemplate(preset.key)}
                          className={`rounded-xl border p-4 text-left transition-colors ${
                            active
                              ? 'border-primary-300 bg-primary-50'
                              : 'border-surface-200 bg-white hover:border-surface-300 hover:bg-surface-50'
                          }`}
                        >
                          <div className="flex items-start justify-between gap-2">
                            <div>
                              <p className="text-sm font-medium text-surface-800">{preset.label}</p>
                              <p className="mt-1 text-xs leading-5 text-surface-500">{preset.desc}</p>
                            </div>
                            {active ? <Check className="h-4 w-4 text-primary-500" /> : null}
                          </div>
                          <div className="mt-3 flex flex-wrap gap-1.5">
                            {summarizePresetTemplate(preset).map((item) => (
                              <span
                                key={`${preset.key}-${item.label}`}
                                className={`rounded-full px-2 py-0.5 text-[10px] ${
                                  active
                                    ? 'bg-white text-primary-700 ring-1 ring-primary-200'
                                    : 'bg-primary-50 text-primary-700'
                                }`}
                              >
                                {item.label}·{item.value}
                              </span>
                            ))}
                          </div>
                          <div className="mt-2 flex flex-wrap gap-1.5">
                            {preset.tags.map((tag) => (
                              <span key={tag} className="rounded-full bg-surface-100 px-2 py-0.5 text-[10px] text-surface-500">
                                {tag}
                              </span>
                            ))}
                          </div>
                        </button>
                      )
                    })}
                  </div>
                </div>
              ) : (
                <div className="rounded-xl border border-surface-200 bg-surface-50/70 p-4">
                  <p className="text-sm font-medium text-surface-800">已切换为 {mediaMeta.label} 创建模式</p>
                  <p className="mt-1 text-xs leading-5 text-surface-500">
                    当前页面已自动隐藏视频专属模板和无关配置，只保留对 {mediaMeta.label} 项目真正有用的参数。
                  </p>
                </div>
              )}

              {mediaStarterTemplates.length > 0 ? (
                <div className="space-y-3 rounded-xl border border-surface-200 bg-white p-4">
                  <div className="flex flex-col gap-1">
                    <Label className="text-sm font-medium">快速创建建议</Label>
                    <p className="text-xs text-surface-500">
                      针对 {mediaMeta.label} 项目提供更贴近当前工作台的起步模板，自动补齐标题、描述和推荐标签。
                    </p>
                  </div>
                  <div className="grid gap-3 md:grid-cols-2">
                    {mediaStarterTemplates.map((template) => (
                      <button
                        key={template.key}
                        type="button"
                        onClick={() => applyMediaStarterTemplate(template)}
                        className="rounded-xl border border-surface-200 bg-surface-50 p-4 text-left transition-colors hover:border-surface-300 hover:bg-white"
                      >
                        <div className="flex items-start justify-between gap-2">
                          <div>
                            <p className="text-sm font-medium text-surface-800">{template.label}</p>
                            <p className="mt-1 text-xs leading-5 text-surface-500">{template.desc}</p>
                          </div>
                          <Sparkles className="h-4 w-4 text-primary-500" />
                        </div>
                        <div className="mt-3 flex flex-wrap gap-1.5">
                          {template.styleTags.map((tag) => (
                            <span key={tag} className="rounded-full bg-white px-2 py-0.5 text-[10px] text-surface-500">
                              {tag}
                            </span>
                          ))}
                        </div>
                      </button>
                    ))}
                  </div>
                </div>
              ) : null}

              <div className="space-y-2">
                <Label>项目 Logo（可选）</Label>
                <input
                  ref={logoInputRef}
                  type="file"
                  accept="image/*"
                  className="hidden"
                  onChange={handleLogoSelect}
                />
                {form.logoPreview ? (
                  <div className="relative inline-block">
                    <img
                      src={form.logoPreview}
                      alt="Logo preview"
                      className="h-24 w-24 rounded-lg border object-cover"
                    />
                    <button
                      type="button"
                      className="absolute -right-2 -top-2 flex h-5 w-5 items-center justify-center rounded-full bg-red-500 text-white hover:bg-red-600"
                      onClick={() => {
                        update('logoFile', null)
                        update('logoPreview', '')
                      }}
                      title="移除已上传的 Logo"
                    >
                      <X className="h-3 w-3" />
                    </button>
                  </div>
                ) : (
                  <button
                    type="button"
                    onClick={() => logoInputRef.current?.click()}
                    className="flex h-24 w-24 flex-col items-center justify-center gap-1 rounded-lg border-2 border-dashed border-surface-300 text-surface-400 hover:border-blue-400 hover:text-primary-500"
                    title="点击上传项目 Logo"
                  >
                    <Upload className="h-5 w-5" />
                    <span className="text-[10px]">上传</span>
                  </button>
                )}
              </div>

              <div className="space-y-2">
                <Label>风格标签</Label>
                <p className="text-xs text-surface-500">{display.styleTagHint}</p>
                <div className="flex flex-wrap gap-2">
                  {mediaStyleOptions.map((tag) => {
                    const selected = visibleStyleTags.includes(tag)
                    return (
                      <button
                        key={tag}
                        type="button"
                        onClick={() => toggleStyleTag(tag)}
                        className={`rounded-full px-3 py-1 text-sm transition-colors ${
                          selected
                            ? 'bg-primary-100 text-primary-700 ring-1 ring-blue-300'
                            : 'bg-surface-100 text-surface-600 hover:bg-surface-200'
                        }`}
                      >
                        {tag}
                      </button>
                    )
                  })}
                </div>
              </div>

              {display.showTargetEpisodes && (
                <div className="space-y-2">
                  <Label htmlFor="episodes">{display.targetEpisodesLabel}</Label>
                  <Input
                    id="episodes"
                    type="number"
                    min={1}
                    max={999}
                    value={form.target_episodes}
                    onChange={(e) => update('target_episodes', Math.max(1, Number(e.target.value)))}
                    className={`w-32 ${errors.target_episodes ? 'border-red-500' : ''}`}
                  />
                  {display.targetEpisodesHint ? (
                    <p className="text-xs text-surface-500">{display.targetEpisodesHint}</p>
                  ) : null}
                  {errors.target_episodes && (
                    <p className="text-xs text-red-500">{errors.target_episodes}</p>
                  )}
                </div>
              )}
            </>
          )}

          {/* ─── Step 2: Model & Feature Selection ─────────── */}
          {step === 2 && (
            <>
              <div className="rounded-xl border border-surface-200 bg-surface-50/70 p-4">
                <p className="text-sm font-medium text-surface-800">当前为 {mediaMeta.label} 专属模型布局</p>
                <p className="mt-1 text-xs leading-5 text-surface-500">
                  只展示当前项目会用到的模型与能力，减少无关视频参数对创建流程的干扰。
                </p>
              </div>

              <div className="grid gap-6 sm:grid-cols-2">
                {visibleModelSelectors.map((selector) => (
                  <ModelSelector
                    key={selector.key}
                    label={selector.label}
                    icon={selector.icon}
                    models={selector.models}
                    isLoading={selector.isLoading}
                    value={selector.value}
                    onChange={selector.onChange}
                  />
                ))}
              </div>

              {display.showVideoMode && (
                <div className="space-y-2">
                  <Label>视频模式</Label>
                  <RadioGroup
                    value={form.video_mode}
                    onValueChange={(v) => update('video_mode', v as FormData['video_mode'])}
                    className="flex gap-6"
                  >
                    <label className="flex cursor-pointer items-center gap-2">
                      <RadioGroupItem value="frame_animation" />
                      <span className="text-sm">逐帧动画</span>
                    </label>
                    <label className="flex cursor-pointer items-center gap-2">
                      <RadioGroupItem value="api_generation" />
                      <span className="text-sm">API生成</span>
                    </label>
                  </RadioGroup>
                </div>
              )}

              {display.showVideoStyle && (
                <div className="space-y-3 rounded-lg border p-4">
                  <div>
                    <Label className="flex items-center gap-2">
                      <Sparkles className="h-4 w-4 text-amber-500" />
                      项目视觉基调
                    </Label>
                    <p className="mt-1 text-xs text-surface-500">
                      创建项目时先选好，后续资源图、分镜图和视频生成都会默认沿用这个方向。
                    </p>
                  </div>
                  <Select
                    value={form.video_style_preset}
                    onValueChange={(v) => update('video_style_preset', v)}
                  >
                    <SelectTrigger>
                      <SelectValue placeholder="选择默认视频风格" />
                    </SelectTrigger>
                    <SelectContent>
                      {VIDEO_STYLE_PRESETS.map((style) => (
                        <SelectItem key={style.key} value={style.key}>
                          {style.label}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <div className="grid gap-2 sm:grid-cols-2">
                    {VIDEO_STYLE_PRESETS.filter((style) => VIDEO_STYLE_CREATION_SPOTLIGHT_KEYS.has(style.key)).map((style) => {
                      const active = style.key === form.video_style_preset
                      return (
                        <button
                          key={style.key}
                          type="button"
                          onClick={() => update('video_style_preset', style.key)}
                          className={`rounded-lg border px-3 py-2 text-left transition-colors ${
                            active
                              ? 'border-primary-300 bg-primary-50'
                              : 'border-surface-200 bg-surface-50 hover:border-surface-300 hover:bg-white'
                          }`}
                        >
                          <div className="flex items-center justify-between gap-2">
                            <span className="text-sm font-medium text-surface-800">{style.label}</span>
                            {active ? <Check className="h-4 w-4 text-primary-500" /> : null}
                          </div>
                          <p className="mt-1 text-xs text-surface-500">{style.desc}</p>
                        </button>
                      )
                    })}
                  </div>
                </div>
              )}

              {display.showVideoStyle && isLiveActionStyle(form.video_style_preset) && (
                <div className="space-y-3 rounded-lg border border-amber-200 bg-amber-50/60 p-4">
                  <div>
                    <Label className="flex items-center gap-2">
                      <span className="text-base">🎬</span>
                      真人参考背景（可选）
                    </Label>
                    <p className="mt-1 text-xs text-surface-500">
                      选填后将用于完善资源图和分镜图的提示词，使生成效果更贴合故事背景。
                    </p>
                  </div>
                  <div className="grid gap-3 sm:grid-cols-3">
                    <div className="space-y-1">
                      <Label className="text-xs text-surface-600">国家 / 地区</Label>
                      <Input
                        placeholder="如：中国大陆、日本、美国"
                        value={form.liveaction_region}
                        onChange={(e) => update('liveaction_region', e.target.value)}
                        className="h-8 text-sm"
                      />
                    </div>
                    <div className="space-y-1">
                      <Label className="text-xs text-surface-600">年代 / 时代</Label>
                      <Input
                        placeholder="如：当代、1990年代、明朝末年"
                        value={form.liveaction_era}
                        onChange={(e) => update('liveaction_era', e.target.value)}
                        className="h-8 text-sm"
                      />
                    </div>
                    <div className="space-y-1">
                      <Label className="text-xs text-surface-600">种族 / 民族</Label>
                      <Input
                        placeholder="如：汉族、东亚人、混血欧亚裔"
                        value={form.liveaction_ethnicity}
                        onChange={(e) => update('liveaction_ethnicity', e.target.value)}
                        className="h-8 text-sm"
                      />
                    </div>
                  </div>
                </div>
              )}

              {display.showVideoMotion && (
                <div className="space-y-3 rounded-lg border p-4">
                  <div>
                    <Label className="flex items-center gap-2">
                      <Video className="h-4 w-4 text-green-500" />
                      视频默认运动模式
                    </Label>
                    <p className="mt-1 text-xs text-surface-500">
                      用于控制生成视频的整体镜头节奏，创建项目后会自动带入视频页默认值。
                    </p>
                  </div>
                  <div className="grid gap-3 sm:grid-cols-3">
                    {VIDEO_MOTION_OPTIONS.map((mode) => {
                      const active = mode.key === form.video_motion_mode
                      return (
                        <button
                          key={mode.key}
                          type="button"
                          onClick={() => update('video_motion_mode', mode.key)}
                          className={`rounded-lg border px-3 py-3 text-left transition-colors ${
                            active
                              ? 'border-primary-300 bg-primary-50'
                              : 'border-surface-200 bg-surface-50 hover:border-surface-300 hover:bg-white'
                          }`}
                        >
                          <div className="flex items-center justify-between gap-2">
                            <span className="text-sm font-medium text-surface-800">{mode.label}</span>
                            {active ? <Check className="h-4 w-4 text-primary-500" /> : null}
                          </div>
                          <p className="mt-1 text-xs leading-5 text-surface-500">{mode.desc}</p>
                        </button>
                      )
                    })}
                  </div>
                </div>
              )}

              {display.showDubSubtitle && (
                <div className="flex flex-col gap-4 sm:flex-row sm:gap-8">
                  <div className="flex items-center gap-3">
                    <Switch
                      checked={form.enable_dubbing}
                      onCheckedChange={(v) => update('enable_dubbing', v)}
                    />
                    <Label className="cursor-pointer">启用配音</Label>
                  </div>
                  <div className="flex items-center gap-3">
                    <Switch
                      checked={form.enable_subtitle}
                      onCheckedChange={(v) => update('enable_subtitle', v)}
                    />
                    <Label className="cursor-pointer">启用字幕</Label>
                  </div>
                </div>
              )}

              {display.showWatermark && (
                <div className="space-y-4 rounded-lg border p-4">
                  <div className="flex items-center justify-between">
                    <Label className="text-sm font-medium">水印配置</Label>
                    <Switch
                      checked={form.watermark_enabled}
                      onCheckedChange={(v) => update('watermark_enabled', v)}
                    />
                  </div>

                  {form.watermark_enabled && (
                    <div className="space-y-4 pt-2">
                      <div className="grid gap-4 sm:grid-cols-2">
                        <div className="space-y-2">
                          <Label className="text-xs">类型</Label>
                          <Select
                            value={form.watermark_type}
                            onValueChange={(v) => update('watermark_type', v as 'text' | 'image')}
                          >
                            <SelectTrigger>
                              <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                              <SelectItem value="text">文字水印</SelectItem>
                              <SelectItem value="image">图片水印</SelectItem>
                            </SelectContent>
                          </Select>
                        </div>

                        <div className="space-y-2">
                          <Label className="text-xs">位置</Label>
                          <Select
                            value={form.watermark_position}
                            onValueChange={(v) =>
                              update('watermark_position', v as FormData['watermark_position'])
                            }
                          >
                            <SelectTrigger>
                              <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                              {WATERMARK_POSITIONS.map((p) => (
                                <SelectItem key={p.value} value={p.value}>
                                  {p.label}
                                </SelectItem>
                              ))}
                            </SelectContent>
                          </Select>
                        </div>
                      </div>

                      {form.watermark_type === 'text' && (
                        <div className="space-y-2">
                          <Label className="text-xs">水印文本</Label>
                          <Input
                            placeholder="输入水印文字"
                            value={form.watermark_text}
                            onChange={(e) => update('watermark_text', e.target.value)}
                          />
                        </div>
                      )}

                      <div className="space-y-2">
                        <div className="flex items-center justify-between">
                          <Label className="text-xs">透明度</Label>
                          <span className="text-xs text-surface-500">{form.watermark_opacity}%</span>
                        </div>
                        <Slider
                          value={[form.watermark_opacity]}
                          onValueChange={(v) => update('watermark_opacity', v[0])}
                          min={10}
                          max={100}
                          step={5}
                        />
                      </div>
                    </div>
                  )}
                </div>
              )}
            
              {display.showStoryboardConfig && (
                <div className="space-y-4">
                  <h4 className="text-sm font-medium text-surface-700">{display.storyboardSectionTitle}</h4>
                  <div className={storyboardGridClassName}>
                    <div className="space-y-2">
                      <Label className="text-xs">画面比例</Label>
                      <Select
                        value={form.storyboard_aspect_ratio}
                        onValueChange={(v) => update('storyboard_aspect_ratio', v)}
                      >
                        <SelectTrigger>
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          {ASPECT_RATIOS.map((r) => (
                            <SelectItem key={r} value={r}>
                              {r}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </div>

                    <div className="space-y-2">
                      <Label className="text-xs">分辨率</Label>
                      <Select
                        value={form.storyboard_resolution}
                        onValueChange={(v) => update('storyboard_resolution', v)}
                      >
                        <SelectTrigger>
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          {RESOLUTIONS.map((r) => (
                            <SelectItem key={r} value={r}>
                              {r}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </div>

                    {display.showStoryboardDuration && (
                      <div className="space-y-2">
                        <Label className="text-xs">默认时长（秒）</Label>
                        <Input
                          type="number"
                          min={1}
                          max={60}
                          value={form.storyboard_duration}
                          onChange={(e) =>
                            update('storyboard_duration', Math.max(1, Number(e.target.value)))
                          }
                        />
                      </div>
                    )}
                  </div>

                  {display.showConsistency && (
                    <div className="rounded-lg border p-4">
                      <div className="flex items-center justify-between">
                        <div>
                          <Label className="text-sm font-medium">{display.consistencyLabel}</Label>
                          <p className="mt-1 text-xs text-surface-500">
                            {display.consistencyHint}
                          </p>
                        </div>
                        <span className="text-sm font-medium text-primary-600">{form.consistency_strength}%</span>
                      </div>
                      <div className="mt-4">
                        <Slider
                          value={[form.consistency_strength]}
                          onValueChange={(v) => update('consistency_strength', v[0])}
                          min={0}
                          max={100}
                          step={5}
                        />
                      </div>
                    </div>
                  )}
                </div>
              )}
            </>
          )}

          {/* ─── Step 3: Script & Config ───────────────────── */}
          {step === 3 && (
            <>
              <div className="space-y-2">
                <Label>{display.scriptLabel}</Label>
                <input
                  ref={scriptInputRef}
                  type="file"
                  accept=".txt,.md,.docx"
                  className="hidden"
                  onChange={handleScriptSelect}
                />
                {form.scriptFile ? (
                  <div className="rounded-lg border p-4">
                    <div className="flex items-center justify-between">
                      <div className="flex items-center gap-2">
                        <FileText className="h-5 w-5 text-primary-500" />
                        <div>
                          <p className="text-sm font-medium">{form.scriptFile.name}</p>
                          <p className="text-xs text-surface-400">
                            {(form.scriptFile.size / 1024).toFixed(1)} KB
                          </p>
                        </div>
                      </div>
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-7 w-7 text-surface-400 hover:text-red-500"
                        onClick={() => {
                          update('scriptFile', null)
                          update('scriptPreview', '')
                        }}
                      >
                        <X className="h-4 w-4" />
                      </Button>
                    </div>
                    {form.scriptPreview && (
                      <pre className="mt-3 max-h-48 overflow-auto rounded bg-surface-50 p-3 text-xs text-surface-600">
                        {form.scriptPreview}
                      </pre>
                    )}
                  </div>
                ) : (
                  <button
                    type="button"
                    onClick={() => scriptInputRef.current?.click()}
                    className="flex w-full flex-col items-center gap-2 rounded-lg border-2 border-dashed border-surface-300 py-8 text-surface-400 hover:border-blue-400 hover:text-primary-500"
                    title={display.scriptUploadHint}
                  >
                    <Upload className="h-8 w-8" />
                    <span className="text-sm">{display.scriptUploadTitle}</span>
                    <span className="text-xs">{display.scriptUploadHint}</span>
                  </button>
                )}
              </div>

              

              {display.showCostEstimation && (
                <CostEstimation
                  form={form}
                  textModels={textModels}
                  imageModels={imageModels}
                  videoModels={videoModels}
                  ttsModels={ttsModels}
                />
              )}
            </>
          )}
        </CardContent>
      </Card>

      {/* Navigation */}
      <div className="sticky bottom-4 z-10 rounded-[24px] border border-surface-200 bg-white/90 p-4 shadow-lg backdrop-blur">
        <div className="flex items-center justify-between">
        <Button
          variant="outline"
          onClick={step === 1 ? () => router.push(mediaMeta.listHref) : goBack}
          title={step === 1 ? '取消创建，返回项目列表' : '返回上一步'}
        >
          <ArrowLeft className="mr-2 h-4 w-4" />
          {step === 1 ? '取消' : '上一步'}
        </Button>

        {step < 3 ? (
          <Button onClick={goNext} title="进入下一步">
            下一步
            <ArrowRight className="ml-2 h-4 w-4" />
          </Button>
        ) : (
          <Button onClick={handleSubmit} disabled={submitting} title="确认创建项目">
            {submitting ? (
              <>
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                创建中...
              </>
            ) : (
              <>
                <Check className="mr-2 h-4 w-4" />
                创建项目
              </>
            )}
          </Button>
        )}
        </div>
      </div>
    </div>
  )
}
