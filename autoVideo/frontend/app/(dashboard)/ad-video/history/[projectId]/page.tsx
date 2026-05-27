'use client'

import Link from 'next/link'
import { type ChangeEvent, useEffect, useMemo, useRef, useState } from 'react'
import { useParams } from 'next/navigation'
import useSWR from 'swr'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Label } from '@/components/ui/label'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Textarea } from '@/components/ui/textarea'
import { useToast } from '@/components/ui/toast'
import { assetAPI, projectAPI, storyboardAPI, videoAPI, type Episode, type Project } from '@/lib/api'
import type { AdCopyOptimizationState, Asset, Storyboard } from '@/types'

type VideoTask = {
  id: number
  project_id: number
  episode_id?: number | null
  status?: string
  model_name?: string
  result_url?: string
  error_msg?: string
  created_at?: string
  updated_at?: string
  render_config?: Record<string, unknown>
}

type VideoModelParamOption = {
  value: string
  label: string
}

type VideoModelParam = {
  key: string
  label: string
  default?: string
  values: VideoModelParamOption[]
}

type VideoModelMeta = {
  key: string
  available: boolean
  native_audio?: boolean
  params?: VideoModelParam[]
}

const DEFAULT_AD_COPY_OPTIMIZATION_PROMPT = `你是广告短视频编剧、导演统筹和连续性审校。你的任务不是直接分集，而是先把整篇广告文案优化成更适合后续“自动切分成多个视频片段”的中间稿，并补出后续生成时必须遵守的一致性前提。

必须遵守：
- 保留原始产品卖点、人物设定、核心承诺与事实信息，不得胡编功效。
- 按当前目标风格重写语言与镜头感，使文案更适合后续广告视频生成。
- 必须主动补全并澄清以下 14 个维度：1）世界观/故事发生的视觉宇宙；2）空间（在哪里）；3）时间（几点/昼夜/时序）；4）人物（谁）；5）服装（穿什么）；6）动作（做什么）；7）核心物件/镜头重点；8）光线（怎么打光）；9）色彩（什么色调）；10）材质（表面质感）；11）镜头运动（怎么拍）；12）情绪（传达什么感觉）；13）转场（怎么切）；14）字幕/屏幕文字、配音/口播内容、以及最终给 AI 的生成 Prompt 描述。
- optimized_script 必须是可直接用于后续自动分集的广告正文；但文中要自然包含这些维度所需的信息，不要只给抽象概念。
- consistency_premise 必须单独总结以上 14 个维度里“后续不得漂移”的硬约束，写成清晰条目。
- 把长段落整理成更自然的口播 / 画面节奏单元，让后续系统更容易按时长自动切分。
- 段落之间要有清楚转场，避免一句话承载过多镜头。
- 如果是写实风格，优先真实场景、生活化表达、自然口语；如果是动漫风格，允许更鲜明的视觉感，但不要失去广告转化目标。
- 不要输出分集编号，不要显式写“第一段/第二段”，只输出优化后的完整文案。
- 要明确区分：哪些是画面信息、哪些是台词/配音、哪些是屏幕字幕、哪些是最终喂给模型的视觉 Prompt 重点。`

function unwrap<T>(payload: unknown): T | null {
  if (!payload || typeof payload !== 'object') return null
  const maybe = payload as { data?: T }
  return maybe.data ?? (payload as T)
}

function unwrapArray<T>(payload: unknown): T[] {
  if (Array.isArray(payload)) return payload
  if (payload && typeof payload === 'object') {
    const maybe = payload as { items?: T[]; data?: T[] }
    if (Array.isArray(maybe.items)) return maybe.items
    if (Array.isArray(maybe.data)) return maybe.data
  }
  return []
}

function taskResultUrl(task?: VideoTask | null) {
  if (!task) return ''
  const rc = task.render_config || {}
  return String(task.result_url || rc.subtitled_result_url || rc.original_result_url || '').trim()
}

function getParamOptions(model: VideoModelMeta | null, key: string): VideoModelParamOption[] {
  if (!model?.params?.length) return []
  const param = model.params.find((item) => item.key === key)
  return Array.isArray(param?.values) ? param!.values : []
}

function pickAllowedValue(options: VideoModelParamOption[], preferred?: string | number | null) {
  if (!options.length) return ''
  const normalizedPreferred = String(preferred ?? '').trim()
  if (normalizedPreferred && options.some((item) => item.value === normalizedPreferred)) {
    return normalizedPreferred
  }
  return options[0]?.value || ''
}

function humanStage(project: Project | null) {
  if (!project) return '暂无'
  return project.progress?.phase_label || project.progress?.stage || project.progress?.message || project.status || '暂无'
}

function stepTone(status: 'pending' | 'active' | 'done' | 'blocked') {
  switch (status) {
    case 'done':
      return 'border-emerald-500/30 bg-emerald-500/15 text-emerald-100'
    case 'active':
      return 'border-cyan-500/30 bg-cyan-500/15 text-cyan-100'
    case 'blocked':
      return 'border-rose-500/30 bg-rose-500/15 text-rose-100'
    default:
      return 'border-white/10 bg-black/20 text-slate-200'
  }
}

function stepLabel(status: 'pending' | 'active' | 'done' | 'blocked') {
  switch (status) {
    case 'done':
      return '已完成'
    case 'active':
      return '当前执行'
    case 'blocked':
      return '前置未完成'
    default:
      return '待执行'
  }
}

function buildEpisodeVideoPayload(storyboards: Storyboard[], episodeId?: number) {
  const sorted = storyboards
    .filter((item) => String(item.image_url || '').trim())
    .slice()
    .sort((a, b) => a.sequence_number - b.sequence_number)

  const sceneDescriptions = sorted.map((item) => item.prompt_used || item.scene_description || '')
  const dialogues = sorted.map((item) => item.dialogue || '')
  const durations = sorted.map((item) => item.duration || 0)
  const cameraMovements = sorted.map((item) => item.camera_movement || '')
  const moods = sorted.map((item) => item.mood || '')
  const spatialAnchors = sorted.map((item) => item.spatial_anchor || '')
  const subjectPositions = sorted.map((item) => item.subject_positions || '')
  const transitionNotes = sorted.map((item) => item.transition_note || '')
  const sceneCharacters = sorted.map((item) => item.characters || [])
  const sceneAssetIds = sorted.map((item) => item.asset_ids || [])
  const sceneGroupKeys = sorted.map((item) => item.scene_group_key || '')

  return {
    episode_id: episodeId,
    image_urls: sorted.map((item) => item.image_url),
    scene_descriptions: sceneDescriptions,
    dialogues: dialogues.some(Boolean) ? dialogues : undefined,
    durations: durations.some(Boolean) ? durations : undefined,
    camera_movements: cameraMovements.some(Boolean) ? cameraMovements : undefined,
    moods: moods.some(Boolean) ? moods : undefined,
    spatial_anchors: spatialAnchors.some(Boolean) ? spatialAnchors : undefined,
    subject_positions: subjectPositions.some(Boolean) ? subjectPositions : undefined,
    transition_notes: transitionNotes.some(Boolean) ? transitionNotes : undefined,
    scene_characters: sceneCharacters.some((arr) => arr.length > 0) ? sceneCharacters : undefined,
    scene_asset_ids: sceneAssetIds.some((arr) => arr.length > 0) ? sceneAssetIds : undefined,
    scene_description: sceneDescriptions.filter(Boolean).join(' ') || undefined,
    scene_group_keys: sceneGroupKeys.some(Boolean) ? sceneGroupKeys : undefined,
  }
}

export default function AdVideoHistoryDetailPage() {
  const { toast } = useToast()
  const params = useParams<{ projectId: string }>()
  const projectId = Number(params?.projectId || 0)

  const [editableOptimizedScript, setEditableOptimizedScript] = useState('')
  const [editableOriginalScript, setEditableOriginalScript] = useState('')
  const [editableOptimizationPrompt, setEditableOptimizationPrompt] = useState('')
  const [optimizingCopy, setOptimizingCopy] = useState(false)
  const [savingCopyDraft, setSavingCopyDraft] = useState(false)
  const [selectedEpisodeId, setSelectedEpisodeId] = useState<string>('all')
  const [generationAction, setGenerationAction] = useState<string | null>(null)
  const [rerunAction, setRerunAction] = useState<string | null>(null)
  const [uploadingAssetId, setUploadingAssetId] = useState<number | null>(null)
  const [selectedVideoModel, setSelectedVideoModel] = useState('')
  const [selectedAspectRatio, setSelectedAspectRatio] = useState('')
  const previousStoryboardsRef = useRef<Storyboard[]>([])
  const [selectedResolution, setSelectedResolution] = useState('')
  const [selectedDuration, setSelectedDuration] = useState('')
  const [selectedGenerateAudio, setSelectedGenerateAudio] = useState(false)

  const { data: projectData, mutate: mutateProject, isLoading } = useSWR(projectId ? ['ad-video-history-project', projectId] : null, async () => {
    const res = await projectAPI.get(projectId)
    return unwrap<Project>((res as { data?: unknown }).data)
  }, { revalidateOnFocus: true })

  const { data: episodesData, mutate: mutateEpisodes } = useSWR(projectId ? ['ad-video-history-episodes', projectId] : null, async () => {
    const res = await projectAPI.listEpisodes(projectId)
    return unwrap<Episode[]>((res as { data?: unknown }).data) || []
  }, { refreshInterval: 5000, revalidateOnFocus: true })

  const { data: adCopyState, mutate: mutateAdCopyState } = useSWR(projectId ? ['ad-video-history-ad-copy', projectId] : null, async () => {
    const res = await projectAPI.getAdCopyOptimizationState(projectId)
    return unwrap<AdCopyOptimizationState>((res as { data?: unknown }).data)
  }, { revalidateOnFocus: true })

  const { data: storyboardsData, mutate: mutateStoryboards } = useSWR(projectId ? ['ad-video-history-storyboards', projectId] : null, async () => {
    const res = await storyboardAPI.listAll(projectId)
    const payload = (res as { data?: Storyboard[] }).data
    return Array.isArray(payload) ? payload : []
  }, { refreshInterval: 5000, revalidateOnFocus: true })

  const { data: taskData, mutate: mutateTasks } = useSWR(projectId ? ['ad-video-history-tasks', projectId] : null, async () => {
    const res = await videoAPI.listAllTasks({ project_id: projectId, page: 1, page_size: 200 })
    const payload = res as { data?: { items?: VideoTask[] } }
    return payload?.data?.items || []
  }, {
    refreshInterval: (latest) => Array.isArray(latest) && latest.some((task) => task.status === 'pending' || task.status === 'processing') ? 5000 : 0,
    revalidateOnFocus: true,
  })

  const { data: assetsData, mutate: mutateAssets } = useSWR(projectId ? ['ad-video-history-assets', projectId] : null, async () => {
    const res = await assetAPI.list(projectId)
    return unwrapArray<Asset>((res as { data?: unknown }).data)
  }, { refreshInterval: 5000, revalidateOnFocus: true })

  const { data: videoModelStatus } = useSWR(projectId ? ['ad-video-history-video-model-status'] : null, async () => {
    const res = await videoAPI.modelStatus()
    const payload = res as { data?: { models?: VideoModelMeta[] } | VideoModelMeta[]; models?: VideoModelMeta[] }
    if (Array.isArray(payload.models)) return payload.models
    if (Array.isArray(payload.data)) return payload.data
    if (payload.data && typeof payload.data === 'object' && Array.isArray((payload.data as { models?: VideoModelMeta[] }).models)) {
      return (payload.data as { models?: VideoModelMeta[] }).models || []
    }
    return []
  }, { revalidateOnFocus: false })

  const project = projectData || null
  const episodes = (episodesData || []).slice().sort((a, b) => a.episode_number - b.episode_number)
  const storyboards = (storyboardsData || []).slice().sort((a, b) => a.sequence_number - b.sequence_number)
  const tasks = useMemo(() => taskData || [], [taskData])
  const assets = (assetsData || []).slice().sort((a, b) => Number(a.id) - Number(b.id))
  const availableModels = (videoModelStatus || []).filter((item) => item.available)
  const latestTask = useMemo(() => tasks.slice().sort((a, b) => Number(b.id) - Number(a.id))[0] || null, [tasks])
  const autoSplit = project?.progress?.auto_split || null
  const realOptimizedScript = useMemo(
    () => String(adCopyState?.optimized_script || autoSplit?.optimized_script || '').trim(),
    [adCopyState?.optimized_script, autoSplit?.optimized_script],
  )
  const realOriginalScript = useMemo(
    () => String(adCopyState?.original_script || autoSplit?.original_script || project?.script_text || '').trim(),
    [adCopyState?.original_script, autoSplit?.original_script, project?.script_text],
  )
  const realOptimizationPrompt = useMemo(
    () => String(adCopyState?.optimization_prompt || autoSplit?.optimization_prompt || project?.storyboard_config?.ad_copy_optimization_prompt || DEFAULT_AD_COPY_OPTIMIZATION_PROMPT).trim(),
    [adCopyState?.optimization_prompt, autoSplit?.optimization_prompt, project?.storyboard_config?.ad_copy_optimization_prompt],
  )
  const resultUrl = taskResultUrl(latestTask)

  const selectedEpisodeNumber = useMemo(() => {
    const value = Number(selectedEpisodeId)
    return Number.isFinite(value) && value > 0 ? value : null
  }, [selectedEpisodeId])

  const selectedEpisode = useMemo(
    () => episodes.find((episode) => episode.id === selectedEpisodeNumber) || null,
    [episodes, selectedEpisodeNumber],
  )

  const selectedModelMeta = useMemo(
    () => availableModels.find((item) => item.key === selectedVideoModel) || null,
    [availableModels, selectedVideoModel],
  )

  const aspectRatioOptions = useMemo(() => getParamOptions(selectedModelMeta, 'aspect_ratio'), [selectedModelMeta])
  const resolutionOptions = useMemo(() => getParamOptions(selectedModelMeta, 'resolution'), [selectedModelMeta])
  const durationOptions = useMemo(() => getParamOptions(selectedModelMeta, 'duration'), [selectedModelMeta])

  const scopeStoryboards = useMemo(() => {
    if (!selectedEpisodeNumber) return storyboards
    return storyboards.filter((item) => Number(item.episode_id) === selectedEpisodeNumber)
  }, [selectedEpisodeNumber, storyboards])

  useEffect(() => {
    if (scopeStoryboards.length > 0) {
      previousStoryboardsRef.current = scopeStoryboards
    }
  }, [scopeStoryboards])

  const scopeStoryboardAssetIds = useMemo(() => {
    const ids = new Set<number>()
    for (const storyboard of scopeStoryboards) {
      for (const assetId of storyboard.asset_ids || []) ids.add(assetId)
    }
    return ids
  }, [scopeStoryboards])

  const scopeAssets = useMemo(() => {
    const base = selectedEpisodeNumber
      ? assets.filter((item) => !item.episode_ids?.length || item.episode_ids.includes(selectedEpisodeNumber))
      : assets

    const referenced = base.filter((item) => scopeStoryboardAssetIds.has(item.id))
    return referenced.length > 0 ? referenced : base
  }, [assets, scopeStoryboardAssetIds, selectedEpisodeNumber])

  const uploadedScopeAssets = useMemo(
    () => scopeAssets.filter((item) => String(item.image_url || '').trim()).length,
    [scopeAssets],
  )

  const processingVideoTaskCount = useMemo(
    () => tasks.filter((item) => item.status === 'pending' || item.status === 'processing').length,
    [tasks],
  )

  const splitConfigReady = Boolean(
    selectedVideoModel
      && selectedAspectRatio
      && selectedResolution
      && selectedDuration
      && aspectRatioOptions.length > 0
      && resolutionOptions.length > 0
      && durationOptions.length > 0,
  )

  const pipelineBusy = Boolean(rerunAction !== null || generationAction !== null || uploadingAssetId !== null)
  const step1Running = rerunAction === 'pipeline' || project?.status === 'script_processing' || project?.progress?.stage === 'episode_splitting'

  const displayStoryboards = useMemo(() => {
    if (scopeStoryboards.length > 0) return scopeStoryboards
    if (step1Running && previousStoryboardsRef.current.length > 0) return previousStoryboardsRef.current
    return []
  }, [scopeStoryboards, step1Running])

  const completedStoryboardImages = useMemo(
    () => displayStoryboards.filter((item) => String(item.image_url || '').trim()).length,
    [displayStoryboards],
  )

  const storyboardScopeReady = displayStoryboards.length > 0
  const assetScopeReady = scopeAssets.length > 0
  const allScopeAssetsUploaded = assetScopeReady && uploadedScopeAssets === scopeAssets.length
  const storyboardImagesReady = storyboardScopeReady && completedStoryboardImages > 0
  const storyboardImagesComplete = storyboardScopeReady && completedStoryboardImages === displayStoryboards.length

  const step1Done = splitConfigReady && storyboardScopeReady && !step1Running
  const step2Running = generationAction?.startsWith('asset-') || generationAction?.startsWith('storyboard-image-') || uploadingAssetId !== null || project?.status === 'asset_generating' || project?.status === 'storyboard_generating'
  const step2Enabled = step1Done
  const step2Done = step1Done && assetScopeReady && allScopeAssetsUploaded && storyboardImagesComplete
  const step3Running = generationAction === 'video-start' || processingVideoTaskCount > 0 || project?.status === 'video_generating'
  const step3Enabled = step2Done
  const step3Done = Boolean(resultUrl)

  const step1Status: 'pending' | 'active' | 'done' | 'blocked' = step1Running ? 'active' : step1Done ? 'done' : 'pending'
  const step2Status: 'pending' | 'active' | 'done' | 'blocked' = !step2Enabled ? 'blocked' : step2Running ? 'active' : step2Done ? 'done' : 'pending'
  const step3Status: 'pending' | 'active' | 'done' | 'blocked' = !step3Enabled ? 'blocked' : step3Running ? 'active' : step3Done ? 'done' : 'pending'

  const step1Hint = step1Running
    ? '当前正在重跑文本拆分 / 自动分镜，请先等这一轮结束。'
    : !splitConfigReady
      ? '先补齐视频模型、比例、分辨率、单分镜时长。'
      : !editableOriginalScript.trim()
        ? '当前原文为空，无法拆分。'
        : storyboardScopeReady
          ? '当前范围已经有可用分镜，可继续重跑覆盖。'
          : '先执行这一步，产出新的分集与分镜文本。'

  const step2Hint = !step2Enabled
    ? '先完成步骤 1，先让这一轮视频配置真正产出新的分集和分镜文案。'
    : step2Running
      ? '当前正在执行步骤 2：准备素材槽位或刷新分镜图，请等这一轮回流。'
      : !assetScopeReady
        ? '当前范围还没有可上传的人物 / 素材槽位，先点“准备人物槽位”。'
        : !allScopeAssetsUploaded
          ? '先把当前范围需要的参考图补齐；没上传完之前，不建议刷新分镜图。'
          : !storyboardImagesReady
            ? '参考图已经齐了，下一步就是刷新当前范围的分镜图。'
            : storyboardImagesComplete
              ? '当前范围的分镜图已经齐了，步骤 2 可以视为完成。'
              : '当前范围已有部分分镜图，但还没补齐，建议继续刷新。'

  const step3Hint = !step3Enabled
    ? '先完成步骤 2：把当前范围需要的参考图补齐，并生成出可用分镜图。'
    : step3Running
      ? '当前已经有视频任务在执行，先等这一轮结果。'
      : completedStoryboardImages === 0
        ? '当前范围还没有可用分镜图，所以现在不能提交视频。'
        : step3Done
          ? '当前已经有成片结果；如果不满意，可以基于这一版分镜图继续重生。'
          : '当前范围已经有可用分镜图，可以开始提交视频任务。'

  useEffect(() => {
    if (!realOptimizedScript) return
    setEditableOptimizedScript((prev) => {
      if (!prev.trim()) return realOptimizedScript
      if (prev.trim() === realOptimizedScript) return prev
      if (step1Running) return prev
      return realOptimizedScript
    })
  }, [realOptimizedScript, step1Running])

  useEffect(() => {
    setEditableOriginalScript((prev) => {
      if (!prev.trim()) return realOriginalScript
      if (prev.trim() === realOriginalScript) return prev
      if (!realOriginalScript) return prev
      return realOriginalScript
    })
  }, [realOriginalScript])

  useEffect(() => {
    setEditableOptimizationPrompt((prev) => {
      if (!prev.trim()) return realOptimizationPrompt
      if (prev.trim() === realOptimizationPrompt) return prev
      if (!realOptimizationPrompt) return prev
      return realOptimizationPrompt
    })
  }, [realOptimizationPrompt])

  useEffect(() => {
    if (!availableModels.length) return
    setSelectedVideoModel((prev) => {
      if (prev && availableModels.some((item) => item.key === prev)) return prev
      const preferred = String(autoSplit?.video_model || project?.storyboard_config?.video_model || '').trim()
      if (preferred && availableModels.some((item) => item.key === preferred)) return preferred
      return availableModels[0]?.key || ''
    })
  }, [availableModels, autoSplit?.video_model, project?.storyboard_config?.video_model])

  useEffect(() => {
    const nextAspect = pickAllowedValue(aspectRatioOptions, project?.storyboard_config?.aspect_ratio)
    setSelectedAspectRatio((prev) => (prev && aspectRatioOptions.some((item) => item.value === prev) ? prev : nextAspect))
  }, [aspectRatioOptions, project?.storyboard_config?.aspect_ratio])

  useEffect(() => {
    const nextResolution = pickAllowedValue(resolutionOptions, project?.storyboard_config?.resolution)
    setSelectedResolution((prev) => (prev && resolutionOptions.some((item) => item.value === prev) ? prev : nextResolution))
  }, [resolutionOptions, project?.storyboard_config?.resolution])

  useEffect(() => {
    const preferredDuration = autoSplit?.duration || project?.storyboard_config?.duration || ''
    const nextDuration = pickAllowedValue(durationOptions, preferredDuration)
    setSelectedDuration((prev) => (prev && durationOptions.some((item) => item.value === prev) ? prev : nextDuration))
  }, [durationOptions, autoSplit?.duration, project?.storyboard_config?.duration])

  useEffect(() => {
    setSelectedGenerateAudio(Boolean(project?.storyboard_config?.generate_audio))
  }, [project?.storyboard_config?.generate_audio])

  const refreshAll = async () => {
    await Promise.all([mutateProject(), mutateEpisodes(), mutateStoryboards(), mutateTasks(), mutateAssets(), mutateAdCopyState()])
  }

  const runScopedAction = async (action: string, runner: () => Promise<unknown>, successTitle: string) => {
    setGenerationAction(action)
    try {
      await runner()
      await refreshAll()
      toast({ title: successTitle, variant: 'success' })
    } catch (error) {
      toast({ title: error instanceof Error ? error.message : '操作失败', variant: 'destructive' })
    } finally {
      setGenerationAction(null)
    }
  }

  const saveAdCopyDraft = async () => {
    setSavingCopyDraft(true)
    try {
      const res = await projectAPI.saveAdCopyDraft(projectId, {
        original_script: editableOriginalScript.trim(),
        optimization_prompt: editableOptimizationPrompt.trim(),
        optimized_script: editableOptimizedScript.trim(),
        persist_original: true,
      })
      const payload = unwrap<AdCopyOptimizationState>((res as { data?: unknown }).data)
      if (payload) {
        setEditableOriginalScript(payload.original_script || '')
        setEditableOptimizationPrompt(payload.optimization_prompt || '')
        setEditableOptimizedScript(payload.optimized_script || '')
      }
      await refreshAll()
      toast({ title: '已保存当前文案工作区', variant: 'success' })
    } catch (error) {
      toast({ title: error instanceof Error ? error.message : '保存文案失败', variant: 'destructive' })
    } finally {
      setSavingCopyDraft(false)
    }
  }

  const optimizeAdCopy = async () => {
    const originalScript = editableOriginalScript.trim()
    const optimizationPrompt = editableOptimizationPrompt.trim()
    if (!originalScript) {
      toast({ title: '请先填写原文，再开始优化', variant: 'destructive' })
      return
    }
    if (!optimizationPrompt) {
      toast({ title: '请先填写文案优化提示词', variant: 'destructive' })
      return
    }
    setOptimizingCopy(true)
    try {
      await storyboardAPI.updateConfig(projectId, {
        ad_copy_optimization_prompt: optimizationPrompt,
      })
      const res = await projectAPI.optimizeAdCopy(projectId, {
        original_script: originalScript,
        optimization_prompt: optimizationPrompt,
        persist_original: true,
      })
      const payload = unwrap<AdCopyOptimizationState>((res as { data?: unknown }).data)
      if (payload) {
        setEditableOriginalScript(payload.original_script || originalScript)
        setEditableOptimizationPrompt(payload.optimization_prompt || optimizationPrompt)
        setEditableOptimizedScript(payload.optimized_script || '')
      }
      await refreshAll()
      toast({ title: realOptimizedScript ? '已按当前提示词重新优化文案' : '已完成首次文案优化', variant: 'success' })
    } catch (error) {
      toast({ title: error instanceof Error ? error.message : '文案优化失败', variant: 'destructive' })
    } finally {
      setOptimizingCopy(false)
    }
  }

  const rerunStoryboardPipeline = async () => {
    const scriptText = editableOriginalScript.trim()
    if (!scriptText) {
      toast({ title: '请先保留一版可用的原文，再开始按当前视频配置重拆分', variant: 'destructive' })
      return
    }
    if (!splitConfigReady) {
      toast({ title: '当前模型没有完整声明 aspect_ratio / resolution / duration，不能启动这条广告流水线', variant: 'destructive' })
      return
    }
    if (project?.status === 'script_processing' || project?.progress?.stage === 'episode_splitting') {
      toast({ title: '当前项目仍在拆分中，请等本轮完成后再重跑，避免再次触发 409', variant: 'destructive' })
      return
    }

    setRerunAction('pipeline')
    try {
      await storyboardAPI.updateConfig(projectId, {
        video_model: selectedVideoModel,
        aspect_ratio: selectedAspectRatio,
        resolution: selectedResolution,
        duration: Number(selectedDuration),
        auto_split_after_optimization: true,
        generate_audio: Boolean(selectedModelMeta?.native_audio && selectedGenerateAudio),
      })

      const filenameBase = (project?.title || `ad-project-${projectId}`).trim() || `ad-project-${projectId}`
      const file = new File([scriptText], `${filenameBase}-pipeline.txt`, { type: 'text/plain' })
      await projectAPI.uploadScript(projectId, file)
      await projectAPI.generateEpisodes(projectId, undefined, { autoStoryboard: true })
      await refreshAll()
      toast({
        title: '已按当前视频模型与单分镜时长重跑“文本拆分 → 分镜文本”',
        variant: 'success',
      })
    } catch (error) {
      toast({ title: error instanceof Error ? error.message : '重跑拆分失败', variant: 'destructive' })
    } finally {
      setRerunAction(null)
    }
  }

  const triggerAssetExtraction = async () => {
    if (selectedEpisodeNumber) {
      await runScopedAction(
        `asset-episode-${selectedEpisodeNumber}`,
        () => assetAPI.extractEpisode(projectId, selectedEpisodeNumber),
        `已为 episode ${selectedEpisode?.episode_number || selectedEpisodeNumber} 生成可上传的人物 / 素材槽位`,
      )
      return
    }
    await runScopedAction('asset-all', () => assetAPI.extract(projectId), '已为整项目生成可上传的人物 / 素材槽位')
  }

  const triggerStoryboardImageGeneration = async () => {
    if (scopeAssets.length === 0) {
      toast({ title: '当前范围还没有人物 / 素材槽位，请先点击“准备人物槽位”', variant: 'destructive' })
      return
    }
    if (selectedEpisodeNumber) {
      await runScopedAction(
        `storyboard-image-episode-${selectedEpisodeNumber}`,
        () => storyboardAPI.generateAll(projectId, selectedEpisodeNumber),
        `已按当前分集的人物图继续生成分镜图`,
      )
      return
    }
    await runScopedAction('storyboard-image-all', () => storyboardAPI.generateAll(projectId), '已按整项目人物图继续生成分镜图')
  }

  const handleAssetUpload = async (assetId: number, event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0]
    event.target.value = ''
    if (!file) return

    setUploadingAssetId(assetId)
    try {
      await assetAPI.upload(projectId, assetId, file)
      await refreshAll()
      toast({ title: `素材 #${assetId} 上传完成`, variant: 'success' })
    } catch (error) {
      toast({ title: error instanceof Error ? error.message : '上传人物图失败', variant: 'destructive' })
    } finally {
      setUploadingAssetId(null)
    }
  }

  const startScopedVideoGeneration = async () => {
    if (!splitConfigReady) {
      toast({ title: '当前模型没有完整声明 aspect_ratio / resolution / duration，不能直接提交视频生成', variant: 'destructive' })
      return
    }

    const storyboardPool = scopeStoryboards
      .filter((item) => String(item.image_url || '').trim())
      .slice()
      .sort((a, b) => a.sequence_number - b.sequence_number)

    if (storyboardPool.length === 0) {
      toast({ title: '当前范围还没有可用的分镜图，请先上传人物图并刷新分镜图', variant: 'destructive' })
      return
    }

    const renderConfig: Record<string, unknown> = {
      aspect_ratio: selectedAspectRatio,
      resolution: selectedResolution,
      generate_audio: selectedModelMeta?.native_audio ? selectedGenerateAudio : undefined,
    }

    const stylePreset = project?.storyboard_config?.style_preset || autoSplit?.style_preset || undefined
    const motionMode = project?.storyboard_config?.motion_mode || undefined
    const clipDuration = Number(selectedDuration || project?.storyboard_config?.duration || 0) || undefined

    setGenerationAction('video-start')
    try {
      if (selectedEpisodeNumber) {
        const payload = buildEpisodeVideoPayload(storyboardPool, selectedEpisodeNumber)
        await videoAPI.generate(projectId, {
          episode_id: selectedEpisodeNumber,
          image_urls: payload.image_urls,
          scene_descriptions: payload.scene_descriptions,
          dialogues: payload.dialogues,
          durations: payload.durations,
          camera_movements: payload.camera_movements,
          moods: payload.moods,
          spatial_anchors: payload.spatial_anchors,
          subject_positions: payload.subject_positions,
          transition_notes: payload.transition_notes,
          scene_characters: payload.scene_characters,
          scene_asset_ids: payload.scene_asset_ids,
          scene_description: payload.scene_description,
          scene_group_keys: payload.scene_group_keys,
          model_name: selectedVideoModel,
          style_preset: stylePreset,
          motion_mode: motionMode,
          video_mode: project?.video_mode,
          clip_duration_sec: clipDuration,
          render_config: renderConfig,
        })
      } else {
        const groups = new Map<number, Storyboard[]>()
        for (const storyboard of storyboardPool) {
          const eid = Number(storyboard.episode_id || 0)
          const bucket = groups.get(eid) ?? []
          bucket.push(storyboard)
          groups.set(eid, bucket)
        }

        const batchEpisodes = Array.from(groups.entries())
          .filter(([episodeId]) => episodeId > 0)
          .map(([episodeId, items]) => buildEpisodeVideoPayload(items, episodeId))
          .filter((item) => item.image_urls.length > 0)

        if (batchEpisodes.length > 0) {
          await videoAPI.generateBatch(projectId, {
            episodes: batchEpisodes.map((item) => ({
              episode_id: item.episode_id || 0,
              image_urls: item.image_urls,
              scene_descriptions: item.scene_descriptions,
              dialogues: item.dialogues,
              durations: item.durations,
              camera_movements: item.camera_movements,
              moods: item.moods,
              spatial_anchors: item.spatial_anchors,
              subject_positions: item.subject_positions,
              transition_notes: item.transition_notes,
              scene_characters: item.scene_characters,
              scene_asset_ids: item.scene_asset_ids,
              scene_description: item.scene_description,
              scene_group_keys: item.scene_group_keys,
            })),
            model_name: selectedVideoModel,
            style_preset: stylePreset,
            motion_mode: motionMode,
            video_mode: project?.video_mode,
            clip_duration_sec: clipDuration,
            render_config: renderConfig,
          })
        }

        const noEpisodeStoryboards = groups.get(0) || []
        if (noEpisodeStoryboards.length > 0) {
          const payload = buildEpisodeVideoPayload(noEpisodeStoryboards)
          if (payload.image_urls.length > 0) {
            await videoAPI.generate(projectId, {
              image_urls: payload.image_urls,
              scene_descriptions: payload.scene_descriptions,
              dialogues: payload.dialogues,
              durations: payload.durations,
              camera_movements: payload.camera_movements,
              moods: payload.moods,
              spatial_anchors: payload.spatial_anchors,
              subject_positions: payload.subject_positions,
              transition_notes: payload.transition_notes,
              scene_characters: payload.scene_characters,
              scene_asset_ids: payload.scene_asset_ids,
              scene_description: payload.scene_description,
              scene_group_keys: payload.scene_group_keys,
              model_name: selectedVideoModel,
              style_preset: stylePreset,
              motion_mode: motionMode,
              video_mode: project?.video_mode,
              clip_duration_sec: clipDuration,
              render_config: renderConfig,
            })
          }
        }
      }

      await refreshAll()
      toast({
        title: selectedEpisodeNumber
          ? `已启动 episode ${selectedEpisode?.episode_number || selectedEpisodeNumber} 的视频生成`
          : '已启动当前项目范围的视频生成',
        variant: 'success',
      })
    } catch (error) {
      toast({ title: error instanceof Error ? error.message : '视频生成失败', variant: 'destructive' })
    } finally {
      setGenerationAction(null)
    }
  }

  const scopeLabel = selectedEpisode
    ? `episode #${selectedEpisode.episode_number} · ${selectedEpisode.title || '未命名片段'}`
    : '整项目（全部分集）'

  return (
    <div className="space-y-6">
      <div className="flex items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold text-white">广告项目详情 / 流水式作业</h1>
          <p className="mt-2 text-sm text-slate-300">
            这里不再堆散按钮。主流程固定为：先按目标视频配置拆分文本，再上传人物图/素材图，最后基于已完成分镜图启动视频生成。
          </p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" onClick={() => { void refreshAll() }}>立即刷新</Button>
          <Button variant="outline" asChild><Link href="/ad-video/history">返回广告历史</Link></Button>
          <Button variant="outline" asChild><Link href="/ad-video">返回工作台</Link></Button>
        </div>
      </div>

      {isLoading || !project ? (
        <Card className="border-white/10 bg-slate-900/60 text-slate-100">
          <CardContent className="p-6 text-sm text-slate-300">正在加载广告详情…</CardContent>
        </Card>
      ) : (
        <>
          <Card className="border-white/10 bg-slate-900/60 text-slate-100">
            <CardHeader>
              <CardTitle>广告详情内容</CardTitle>
              <CardDescription className="text-slate-400">把文案、分镜、视频分开查看；优化后的文案固定放在“文案”页签中。</CardDescription>
            </CardHeader>
            <CardContent>
              <Tabs defaultValue="copy" className="space-y-4">
                <TabsList className="h-auto w-full flex-wrap justify-start gap-2 rounded-xl bg-black/20 p-1 text-slate-300">
                  <TabsTrigger value="copy">文案 · {realOptimizedScript.length} 字 / {episodes.length} 集</TabsTrigger>
                  <TabsTrigger value="storyboard">分镜 · {displayStoryboards.length} 条 / {completedStoryboardImages} 张图</TabsTrigger>
                  <TabsTrigger value="video">视频 · {tasks.length} 个任务{resultUrl ? ' / 有成片' : ''}</TabsTrigger>
                </TabsList>

                <TabsContent value="copy" className="space-y-4">
                  <div className="rounded-xl border border-cyan-500/20 bg-cyan-500/10 p-4 space-y-4">
                    <div className="flex flex-wrap items-start justify-between gap-3">
                      <div>
                        <div className="text-sm font-medium text-cyan-100">文案优化提示词</div>
                        <div className="mt-1 text-xs text-cyan-100/80">这里展示并编辑当前广告项目真实使用的优化提示词。点击按钮后支持首次优化，也支持基于当前原文和当前提示词多次重新优化。</div>
                      </div>
                      <div className="flex flex-wrap gap-2">
                        <Button variant="outline" onClick={() => { void saveAdCopyDraft() }} disabled={savingCopyDraft || optimizingCopy || pipelineBusy}>
                          {savingCopyDraft ? '保存中…' : '保存文案'}
                        </Button>
                        <Button onClick={() => { void optimizeAdCopy() }} disabled={optimizingCopy || savingCopyDraft || pipelineBusy}>
                          {optimizingCopy ? '优化中…' : realOptimizedScript ? '重新优化' : '开始优化'}
                        </Button>
                      </div>
                    </div>
                    <Textarea
                      value={editableOptimizationPrompt}
                      onChange={(e) => setEditableOptimizationPrompt(e.target.value)}
                      className="min-h-[180px] border-cyan-500/20 bg-black/20 text-slate-100"
                      placeholder="请输入文案优化提示词。"
                    />
                  </div>

                  <div className="grid gap-4 xl:grid-cols-2">
                    <div className="rounded-xl border border-emerald-500/20 bg-emerald-500/10 p-4">
                      <div className="mb-3 flex items-center justify-between gap-2">
                        <div>
                          <div className="text-sm font-medium text-white">优化后的文案</div>
                          <div className="mt-1 text-[11px] text-emerald-200/75">这里固定放真正的优化稿，真实来源仍是 `project.progress.auto_split.optimized_script`。</div>
                        </div>
                        <div className="text-[11px] text-emerald-200/75">{editableOptimizedScript.trim().length || realOptimizedScript.length} 字</div>
                      </div>
                      {realOptimizedScript ? (
                        <Textarea
                          value={editableOptimizedScript}
                          onChange={(e) => setEditableOptimizedScript(e.target.value)}
                          className="min-h-[520px] border-emerald-500/20 bg-black/20 text-slate-100"
                          placeholder="这里保留当前要进入流水线的广告文案。"
                        />
                      ) : (
                        <div className="rounded-lg border border-amber-500/20 bg-amber-500/10 p-4 text-sm text-amber-100">
                          当前项目还没有真正优化后的文案。请先填写左侧原文和上方提示词，再点击“开始优化”。
                        </div>
                      )}
                    </div>

                    <div className="rounded-xl border border-white/10 bg-black/20 p-4">
                      <div className="mb-3 flex items-center justify-between gap-2">
                        <div>
                          <div className="text-sm font-medium text-white">原文</div>
                          <div className="mt-1 text-[11px] text-slate-500">右侧原文支持直接调整；下次点击“开始优化 / 重新优化”会以这里的当前文本为准。</div>
                        </div>
                        <div className="text-[11px] text-slate-500">{editableOriginalScript.trim().length || realOriginalScript.length} 字</div>
                      </div>
                      <Textarea
                        value={editableOriginalScript}
                        onChange={(e) => setEditableOriginalScript(e.target.value)}
                        className="min-h-[520px] border-white/10 bg-black/20 text-slate-100"
                        placeholder="请输入或调整当前原文。"
                      />
                    </div>
                  </div>
                </TabsContent>

                <TabsContent value="storyboard" className="space-y-4">
                  <div className="rounded-xl border border-violet-500/20 bg-violet-500/10 p-4 text-sm text-violet-100">
                    <div className="font-medium">分镜工作区</div>
                    <div className="mt-1 text-xs text-violet-100/80">这里集中看当前范围分镜、场景描述、台词和分镜图补齐情况；步骤 2 的人物图上传与分镜图刷新仍在上方流水线执行。</div>
                  </div>
                  <div className="rounded-xl border border-white/10 bg-black/20 p-4">
                    <div className="flex flex-wrap items-center justify-between gap-3">
                      <div>
                        <div className="text-sm font-medium text-white">当前范围分镜</div>
                        <div className="mt-1 text-xs text-slate-400">这里直接看当前范围的 scene_description / dialogue / 分镜图是否齐了。</div>
                      </div>
                      <div className="text-[11px] text-slate-400">当前范围：{scopeLabel} · 分镜 {displayStoryboards.length} 条{step1Running && scopeStoryboards.length === 0 && previousStoryboardsRef.current.length > 0 ? ' · 正在重建，先显示上一版' : ''}</div>
                    </div>
                  </div>

                  {displayStoryboards.length === 0 ? (
                    <div className="rounded-lg border border-white/10 bg-black/20 p-4 text-sm text-slate-300">{step1Running ? '当前正在重跑步骤 1，后端会先删旧分镜再重建新分镜，请稍等这一轮回流。' : '当前范围还没有分镜记录。'}</div>
                  ) : displayStoryboards.map((storyboard) => (
                    <div key={storyboard.id} className="rounded-xl border border-white/10 bg-black/20 p-4 space-y-3">
                      <div className="flex items-start justify-between gap-3">
                        <div className="text-sm text-slate-100">分镜 #{storyboard.sequence_number} · {storyboard.status || '-'} · episode {storyboard.episode_id || '-'}</div>
                        <div className="text-[11px] text-slate-400">引用素材：{storyboard.asset_ids?.length || 0}</div>
                      </div>
                      {String(storyboard.image_url || '').trim() && (
                        <div className="overflow-hidden rounded-lg border border-white/10 bg-black/30">
                          {/* eslint-disable-next-line @next/next/no-img-element */}
                          <img src={storyboard.image_url} alt={`storyboard-${storyboard.id}`} className="max-h-64 w-full object-cover" />
                        </div>
                      )}
                      <div className="grid gap-3 md:grid-cols-2">
                        <div className="rounded-lg border border-cyan-500/20 bg-cyan-500/10 p-3">
                          <div className="mb-2 text-xs font-medium text-cyan-200">场景描述</div>
                          <div className="whitespace-pre-wrap break-words text-sm text-slate-100">{storyboard.scene_description || '暂无场景描述'}</div>
                        </div>
                        <div className="rounded-lg border border-violet-500/20 bg-violet-500/10 p-3">
                          <div className="mb-2 text-xs font-medium text-violet-200">台词</div>
                          <div className="whitespace-pre-wrap break-words text-sm text-slate-100">{storyboard.dialogue || '暂无台词'}</div>
                        </div>
                      </div>
                      <div className="grid gap-2 text-xs text-slate-400 md:grid-cols-4">
                        <div>location：{storyboard.location || '-'}</div>
                        <div>camera：{storyboard.camera_movement || '-'}</div>
                        <div>duration：{storyboard.duration || '-'} 秒</div>
                        <div>asset_ids：{storyboard.asset_ids?.length ? storyboard.asset_ids.join(', ') : '-'}</div>
                      </div>
                    </div>
                  ))}
                </TabsContent>

                <TabsContent value="video" className="space-y-4">
                  <div className="rounded-xl border border-cyan-500/20 bg-cyan-500/10 p-4 text-sm text-cyan-100">
                    <div className="font-medium">视频工作区</div>
                    <div className="mt-1 text-xs text-cyan-100/80">这里集中看当前视频任务、报错、结果链接与成片预览；步骤 3 的提交动作仍在上方流水线执行。</div>
                  </div>
                  <div className="rounded-xl border border-white/10 bg-black/20 p-4 space-y-3">
                    <div className="flex flex-wrap items-center justify-between gap-3">
                      <div>
                        <div className="text-sm font-medium text-white">视频任务 / 完整视频</div>
                        <div className="mt-1 text-xs text-slate-400">广告历史详情内直接查看当前视频生成进度与完整视频结果。</div>
                      </div>
                      <div className="text-[11px] text-slate-400">当前任务数：{tasks.length}</div>
                    </div>

                    {tasks.length === 0 ? (
                      <div className="rounded-lg border border-white/10 bg-slate-950/40 p-4 text-sm text-slate-300">当前还没有视频任务记录。</div>
                    ) : tasks.slice().sort((a, b) => Number(b.id) - Number(a.id)).map((task) => (
                      <div key={task.id} className="rounded-lg border border-white/10 bg-slate-950/40 p-4 text-sm text-slate-200">
                        <div>task #{task.id} · {task.status || '-'} · model {task.model_name || '-'} · {task.created_at || '-'}</div>
                        {task.error_msg && <div className="mt-2 text-rose-300">错误：{task.error_msg}</div>}
                        {taskResultUrl(task) && <div className="mt-2 break-all"><a className="text-cyan-300 underline" href={taskResultUrl(task)} target="_blank" rel="noreferrer">打开结果视频</a></div>}
                      </div>
                    ))}

                    {resultUrl && (
                      <div className="overflow-hidden rounded-xl border border-white/10 bg-black/30 p-3">
                        <div className="mb-2 text-xs text-slate-400">最新完整视频预览</div>
                        <video className="max-h-[420px] w-full rounded-lg bg-black" controls preload="metadata" src={resultUrl} />
                      </div>
                    )}
                  </div>
                </TabsContent>
              </Tabs>
            </CardContent>
          </Card>

          <Card className="border-white/10 bg-slate-900/60 text-slate-100">
            <CardHeader>
              <CardTitle>广告流水线</CardTitle>
              <CardDescription className="text-slate-400">顺序固定：1）文本拆分 → 2）人物图 / 分镜图准备 → 3）视频生成。</CardDescription>
            </CardHeader>
            <CardContent className="space-y-5">
              <div className="grid gap-3 md:grid-cols-3">
                <div className={`rounded-xl border p-4 ${stepTone(step1Status)}`}>
                  <div className="flex items-center justify-between gap-2">
                    <div className="text-xs font-medium">步骤 1</div>
                    <div className="rounded-full border border-current/20 px-2 py-0.5 text-[10px]">{stepLabel(step1Status)}</div>
                  </div>
                  <div className="mt-1 text-base font-semibold text-white">按视频配置重拆分文本</div>
                  <div className="mt-2 text-xs text-current/80">先确定视频模型、比例、分辨率、单分镜时长，再把当前文案重跑为分镜文本。</div>
                  <div className="mt-3 text-[11px] text-current/80">{step1Hint}</div>
                </div>
                <div className={`rounded-xl border p-4 ${stepTone(step2Status)}`}>
                  <div className="flex items-center justify-between gap-2">
                    <div className="text-xs font-medium">步骤 2</div>
                    <div className="rounded-full border border-current/20 px-2 py-0.5 text-[10px]">{stepLabel(step2Status)}</div>
                  </div>
                  <div className="mt-1 text-base font-semibold text-white">上传人物图 / 刷新分镜图</div>
                  <div className="mt-2 text-xs text-current/80">先准备人物 / 素材槽位，再为当前范围逐个上传真实参考图，最后刷新分镜图。</div>
                  <div className="mt-3 text-[11px] text-current/80">{step2Hint}</div>
                </div>
                <div className={`rounded-xl border p-4 ${stepTone(step3Status)}`}>
                  <div className="flex items-center justify-between gap-2">
                    <div className="text-xs font-medium">步骤 3</div>
                    <div className="rounded-full border border-current/20 px-2 py-0.5 text-[10px]">{stepLabel(step3Status)}</div>
                  </div>
                  <div className="mt-1 text-base font-semibold text-white">开始生成视频</div>
                  <div className="mt-2 text-xs text-current/80">只有分镜图准备完成后才启动视频生成，避免直接拿空图或错图提交。</div>
                  <div className="mt-3 text-[11px] text-current/80">{step3Hint}</div>
                </div>
              </div>

              <div className="rounded-xl border border-white/10 bg-black/20 p-4 space-y-3">
                <div className="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-white/10 bg-slate-950/40 p-3 text-xs text-slate-300">
                  <div>流水线状态：{pipelineBusy ? '执行中' : '空闲'}</div>
                  <div>步骤 1：{stepLabel(step1Status)} / 步骤 2：{stepLabel(step2Status)} / 步骤 3：{stepLabel(step3Status)}</div>
                </div>
                <div className="space-y-2">
                  <Label className="text-slate-200">当前处理范围</Label>
                  <select
                    value={selectedEpisodeId}
                    onChange={(e) => setSelectedEpisodeId(e.target.value)}
                    className="flex h-10 w-full rounded-xl border border-surface-200 bg-white px-3 py-2 text-sm text-surface-900"
                  >
                    <option value="all">整项目（全部分集）</option>
                    {episodes.map((episode) => (
                      <option key={episode.id} value={String(episode.id)}>
                        episode #{episode.episode_number} · {episode.title || '未命名片段'}
                      </option>
                    ))}
                  </select>
                  <div className="text-[11px] text-slate-400">这个范围只影响步骤 2 和步骤 3；步骤 1 始终基于当前全文重新拆分。</div>
                </div>
              </div>

              <div className="grid gap-4 xl:grid-cols-3">
                <div className="rounded-xl border border-cyan-500/20 bg-cyan-500/10 p-4 space-y-4">
                  <div>
                    <div className="text-sm font-medium text-cyan-100">步骤 1：按目标视频配置重跑“文本 → 分镜文本”</div>
                    <div className="mt-1 text-xs text-cyan-100/80">这里决定后续的分镜粒度。单分镜时长直接来自所选视频模型的真实声明。</div>
                  </div>

                  <div className="space-y-2">
                    <Label className="text-slate-100">生成视频模型</Label>
                    <select
                      value={selectedVideoModel}
                      onChange={(e) => setSelectedVideoModel(e.target.value)}
                      className="flex h-10 w-full rounded-xl border border-surface-200 bg-white px-3 py-2 text-sm text-surface-900"
                    >
                      <option value="">请选择模型</option>
                      {availableModels.map((item) => (
                        <option key={item.key} value={item.key}>{item.key}</option>
                      ))}
                    </select>
                  </div>

                  <div className="space-y-2">
                    <Label className="text-slate-100">画面比例</Label>
                    <select
                      value={selectedAspectRatio}
                      onChange={(e) => setSelectedAspectRatio(e.target.value)}
                      disabled={aspectRatioOptions.length === 0}
                      className="flex h-10 w-full rounded-xl border border-surface-200 bg-white px-3 py-2 text-sm text-surface-900 disabled:cursor-not-allowed disabled:opacity-60"
                    >
                      <option value="">请选择比例</option>
                      {aspectRatioOptions.map((item) => (
                        <option key={item.value} value={item.value}>{item.label || item.value}</option>
                      ))}
                    </select>
                  </div>

                  <div className="space-y-2">
                    <Label className="text-slate-100">分辨率</Label>
                    <select
                      value={selectedResolution}
                      onChange={(e) => setSelectedResolution(e.target.value)}
                      disabled={resolutionOptions.length === 0}
                      className="flex h-10 w-full rounded-xl border border-surface-200 bg-white px-3 py-2 text-sm text-surface-900 disabled:cursor-not-allowed disabled:opacity-60"
                    >
                      <option value="">请选择分辨率</option>
                      {resolutionOptions.map((item) => (
                        <option key={item.value} value={item.value}>{item.label || item.value}</option>
                      ))}
                    </select>
                  </div>

                  <div className="space-y-2">
                    <Label className="text-slate-100">单分镜时长</Label>
                    <select
                      value={selectedDuration}
                      onChange={(e) => setSelectedDuration(e.target.value)}
                      disabled={durationOptions.length === 0}
                      className="flex h-10 w-full rounded-xl border border-surface-200 bg-white px-3 py-2 text-sm text-surface-900 disabled:cursor-not-allowed disabled:opacity-60"
                    >
                      <option value="">请选择时长</option>
                      {durationOptions.map((item) => (
                        <option key={item.value} value={item.value}>{item.label || item.value}</option>
                      ))}
                    </select>
                  </div>

                  {selectedModelMeta?.native_audio ? (
                    <label className="flex items-center gap-2 text-xs text-cyan-100/85">
                      <input
                        type="checkbox"
                        checked={selectedGenerateAudio}
                        onChange={(e) => setSelectedGenerateAudio(e.target.checked)}
                      />
                      同步记住该模型的原生音频能力（若后端支持，将继续透传）
                    </label>
                  ) : (
                    <div className="text-[11px] text-cyan-100/75">当前模型未声明 native_audio，音频开关不会透传。</div>
                  )}

                  {!splitConfigReady && (
                    <div className="rounded-lg border border-amber-500/20 bg-amber-500/10 p-3 text-xs text-amber-100">
                      当前模型必须同时声明 aspect_ratio / resolution / duration，才能进入这条广告流水线。
                    </div>
                  )}

                  <Button
                    disabled={pipelineBusy || !editableOriginalScript.trim() || !splitConfigReady}
                    onClick={() => void rerunStoryboardPipeline()}
                  >
                    {rerunAction === 'pipeline' ? '正在按当前配置重拆分…' : '开始步骤 1：按当前视频配置重拆分'}
                  </Button>

                  <div className="rounded-lg border border-white/10 bg-black/20 p-3 text-[11px] text-cyan-100/80">
                    {step1Hint}
                  </div>
                </div>

                <div className="rounded-xl border border-violet-500/20 bg-violet-500/10 p-4 space-y-4">
                  <div>
                    <div className="text-sm font-medium text-violet-100">步骤 2：准备参考图并刷新分镜图</div>
                    <div className="mt-1 text-xs text-violet-100/80">这一块只做三件事：先生成当前范围需要的素材槽位，再上传参考图，最后刷新这一范围的分镜图。</div>
                  </div>

                  <div className="rounded-lg border border-white/10 bg-black/20 p-3 text-xs text-violet-100/85 space-y-1">
                    <div>当前范围：{scopeLabel}</div>
                    <div>素材槽位：{scopeAssets.length} 个</div>
                    <div>已上传参考图：{uploadedScopeAssets} / {scopeAssets.length}</div>
                    <div>可用分镜图：{completedStoryboardImages} / {displayStoryboards.length}</div>
                  </div>

                  <div className="flex flex-wrap gap-2">
                    <Button
                      size="sm"
                      variant="outline"
                      disabled={pipelineBusy || !step2Enabled}
                      onClick={() => void triggerAssetExtraction()}
                    >
                      {generationAction === 'asset-all' || generationAction === `asset-episode-${selectedEpisodeNumber}` ? '正在准备槽位…' : '1）先准备人物 / 素材槽位'}
                    </Button>
                    <Button
                      size="sm"
                      disabled={pipelineBusy || !step2Enabled || scopeAssets.length === 0}
                      onClick={() => void triggerStoryboardImageGeneration()}
                    >
                      {generationAction === 'storyboard-image-all' || generationAction === `storyboard-image-episode-${selectedEpisodeNumber}` ? '正在刷新分镜图…' : '3）刷新当前范围分镜图'}
                    </Button>
                  </div>

                  <div className="rounded-lg border border-white/10 bg-black/20 p-3 text-[11px] text-violet-100/80">
                    {step2Hint}
                  </div>

                  <div className="max-h-[420px] space-y-3 overflow-auto pr-1">
                    {scopeAssets.length === 0 ? (
                      <div className="rounded-lg border border-white/10 bg-black/20 p-4 text-xs text-violet-100/80">
                        当前范围还没有可上传的素材槽位。先点上面的“准备人物 / 素材槽位”，系统才会生成这一轮需要上传的角色/物件入口。
                      </div>
                    ) : scopeAssets.map((asset) => (
                      <div key={asset.id} className="rounded-lg border border-white/10 bg-black/20 p-3 space-y-3">
                        <div className="flex items-start justify-between gap-3">
                          <div>
                            <div className="text-sm font-medium text-white">#{asset.id} · {asset.name || '未命名素材'}</div>
                            <div className="mt-1 text-[11px] text-slate-400">类型：{asset.type || '-'} · 状态：{asset.status || '-'}</div>
                            {!!asset.episode_ids?.length && (
                              <div className="mt-1 text-[11px] text-slate-500">关联分集：{asset.episode_ids.join(' / ')}</div>
                            )}
                          </div>
                          <div className="text-[11px] text-violet-100/80">
                            {String(asset.image_url || '').trim() ? '第 2 步已上传' : '第 2 步待上传'}
                          </div>
                        </div>

                        {String(asset.image_url || '').trim() && (
                          <div className="overflow-hidden rounded-lg border border-white/10 bg-black/30">
                            {/* eslint-disable-next-line @next/next/no-img-element */}
                            <img src={asset.image_url} alt={asset.name || `asset-${asset.id}`} className="h-32 w-full object-cover" />
                          </div>
                        )}

                        <div className="flex flex-wrap items-center gap-2">
                          <label className="inline-flex cursor-pointer items-center rounded-lg border border-violet-400/30 bg-violet-500/10 px-3 py-2 text-xs text-violet-100 transition hover:bg-violet-500/20">
                            {uploadingAssetId === asset.id ? '上传中…' : String(asset.image_url || '').trim() ? '2）重新上传参考图' : '2）上传参考图'}
                            <input
                              type="file"
                              accept="image/*"
                              className="hidden"
                              disabled={uploadingAssetId !== null || !step2Enabled || pipelineBusy}
                              onChange={(event) => { void handleAssetUpload(asset.id, event) }}
                            />
                          </label>
                          <div className="text-[11px] text-slate-400">这里上传的是这轮分镜要参考的最终定稿图；上传完再点上面的“刷新当前范围分镜图”。</div>
                        </div>
                      </div>
                    ))}
                  </div>
                </div>

                <div className="rounded-xl border border-emerald-500/20 bg-emerald-500/10 p-4 space-y-4">
                  <div>
                    <div className="text-sm font-medium text-emerald-100">步骤 3：基于当前范围分镜图提交视频</div>
                    <div className="mt-1 text-xs text-emerald-100/80">这里只有在当前范围已经有可用分镜图时才允许提交；人物图本身不会直接拿去生成视频。</div>
                  </div>

                  <div className="rounded-lg border border-white/10 bg-black/20 p-3 text-xs text-emerald-100/85 space-y-1">
                    <div>当前范围：{scopeLabel}</div>
                    <div>目标模型：{selectedVideoModel || '未选择'}</div>
                    <div>视频配置：{selectedAspectRatio || '-'} / {selectedResolution || '-'} / {selectedDuration || '-'} 秒</div>
                    <div>当前可提交分镜图：{completedStoryboardImages} / {displayStoryboards.length}</div>
                  </div>

                  <Button
                    disabled={pipelineBusy || !step3Enabled || !splitConfigReady || completedStoryboardImages === 0}
                    onClick={() => void startScopedVideoGeneration()}
                  >
                    {generationAction === 'video-start' ? '正在提交视频任务…' : selectedEpisodeNumber ? '开始生成当前分集视频' : '开始生成当前范围视频'}
                  </Button>

                  <div className="rounded-lg border border-white/10 bg-black/20 p-3 text-[11px] text-emerald-100/80">
                    {step3Hint}
                  </div>

                  <div className="text-[11px] text-emerald-100/75">
                    真正提交给视频服务的是当前范围内那些已经有 `image_url` 的分镜图，以及对应的分镜文案、台词、镜头运动、角色/素材引用等字段。
                  </div>
                </div>
              </div>
            </CardContent>
          </Card>

        </>
      )}
    </div>
  )
}
