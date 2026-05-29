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
import { assetAPI, modelAPI, projectAPI, storyboardAPI, videoAPI, type Episode, type Project } from '@/lib/api'
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

function formatModelParamValues(param?: VideoModelParam) {
  if (!param?.values?.length) return '未声明可选值'
  return param.values.map((item) => item.label || item.value).join(' / ')
}

const DEFAULT_AD_COPY_OPTIMIZATION_PROMPT = `你是广告短视频编剧、导演统筹和连续性审校。你的任务不是直接分集，也不是直接写成逐镜头分镜稿，而是先把整篇广告文案优化成更适合后续“按台词 / 口播为主自动切分成多个视频片段”的中间稿，并补出后续生成时必须遵守的一致性前提。

必须遵守：
- 保留原始产品卖点、人物设定、核心承诺与事实信息，不得胡编功效。
- 按当前目标风格重写语言与镜头感，使文案更适合后续广告视频生成，但绝不能提前把它写成 storyboard / shotlist / 分镜脚本。
- 必须主动补全并澄清以下 14 个维度：1）世界观/故事发生的视觉宇宙；2）空间（在哪里）；3）时间（几点/昼夜/时序）；4）人物（谁）；5）服装（穿什么）；6）动作（做什么）；7）核心物件/镜头重点；8）光线（怎么打光）；9）色彩（什么色调）；10）材质（表面质感）；11）镜头运动（怎么拍）；12）情绪（传达什么感觉）；13）转场（怎么切）；14）字幕/屏幕文字、配音/口播内容、以及最终给 AI 的生成 Prompt 描述。
- optimized_script 必须是“可继续拆分的广告中间稿”，核心是口播 / 台词 / 信息块顺序清楚，而不是已经拆好的镜头列表。
- consistency_premise 必须单独总结以上 14 个维度里“后续不得漂移”的硬约束，写成清晰条目。
- 把长段落整理成更自然的台词 / 口播句群，让后续系统更容易按单分镜时长进行台词拆分；优先保证一句口播能在一个完整镜头里说完。
- 每个段落优先围绕“一个卖点 / 一个信息推进 / 一个情绪动作”来写，不要为了增加画面感把一句话拆成多个视觉段。
- 可以补充必要的视觉约束，但只能轻量嵌入同一段中；不要给每段都单独展开“画面 / 字幕 / 配音 / Prompt”四件套。
- 严禁使用类似“【画面1】/【镜头1】/【字幕】/【口播】/【Prompt】”的逐段标签式输出；不要显式编号，不要写成 shot-by-shot 结构。
- 除收尾 CTA 外，不要主动新增无台词视觉段；不要为了渲染镜头感平白增加多个空镜、转场镜头、补充动作镜头。
- 优化后的正文总长度应尽量克制，通常控制在原文的 1.2x~1.6x 内；若明显超过，优先压缩视觉描述，而不是继续扩写。
- 如果是写实风格，优先真实场景、生活化表达、自然口语；如果是动漫风格，允许更鲜明的视觉感，但不要失去广告转化目标。
- 不要输出分集编号，不要显式写“第一段/第二段”，只输出优化后的完整文案和 consistency_premise。`

const DEFAULT_STORYBOARD_SPLIT_BUILTIN_PROMPT = `你是一位专业的广告分镜师和摄影指导。当前步骤 1 的分镜拆分必须遵守以下内置规则：

1. 最高优先级：当规则发生冲突时，一律以“时长优先、台词 / 口播承载量优先”为最高准则。
2. 本项目目标单分镜时长以当前用户所选值为准；如当前未显式指定，则按模型默认允许时长执行。
3. 核心原则：优先判断一段台词 / 口播是否能在当前目标单分镜时长内完整表达，并同时追求视觉单位连贯性、空间方位一致性与整体表达稳定性，而不是追求最小视觉单位。
4. 如果同一段卖点说明、同一段口播、同一段连续动作在当前目标时长内可以完整表达，应优先合并为一个主分镜或少量连续分镜；即使进入新卖点，也只有在当前时长已经承载不下时才拆镜。
5. 判断是否拆镜的唯一依据是：观众是否会在该镜头内获得新的信息或新的情绪锚点；若没有，则不拆。
6. 口播内容必须在一个完整分镜内说完，不得为动作细节拆散口播。无 dialogue 分镜只能作为极短辅助镜头（建议不超过总分镜数的 20%），不可连续出现，不可单独承担卖点传达；除最后一个分镜外，若当前分镜没有台词，或台词长度明显不足以支撑当前目标时长，就必须继续合并、重写或调整拆分。
7. 只有最后一个分镜允许在确有必要时作为收束镜头例外，但即便如此也应尽量带有一句完整收尾口播、CTA 或字幕，不要轻易留空。
8. dialogue 只能放真的会被念出来或打上字幕的文字；如果某段只有动作或镜头说明、没有可念文本，优先继续调整拆分，让它并回前后有台词的分镜，而不是直接保留。
9. description 必须使用结构化格式：[景别] + [人物/主体位置与动作] + [环境与光线] + [关键道具或视觉锚点]；每条尽量不超过 60 字。`

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

const SPEECH_PACE_OPTIONS = [
  { value: 'normal', label: '正常', hint: '10 秒内按常规商业口播节奏拆分。' },
  { value: 'slightly_fast', label: '稍快', hint: '10 秒内可承载更多字数，适合信息密度略高的广告。' },
  { value: 'with_pauses', label: '有停顿', hint: '要给停顿和强调留空间，会更积极拆镜。' },
  { value: 'very_fast', label: '很快', hint: '10 秒内承载量最高，但仍以完整句群为主。' },
  { value: 'medium_fast', label: '中速偏快', hint: '介于正常和稍快之间，适合信息流广告。' },
  { value: 'medium_steady', label: '中速稳重', hint: '节奏稳，句间更讲究停连，避免单镜过满。' },
] as const

type SpeechPaceOption = typeof SPEECH_PACE_OPTIONS[number]['value']

function pickAllowedValue(options: VideoModelParamOption[], preferred?: string | number | null, fallbackToFirst = true) {
  if (!options.length) return ''
  const normalizedPreferred = String(preferred ?? '').trim()
  if (normalizedPreferred && options.some((item) => item.value === normalizedPreferred)) {
    return normalizedPreferred
  }
  return fallbackToFirst ? (options[0]?.value || '') : normalizedPreferred
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
    .slice()
    .sort((a, b) => a.sequence_number - b.sequence_number)

  const firstImageIndex = sorted.findIndex((item) => String(item.image_url || '').trim())
  const serialStoryboards = firstImageIndex > 0 ? sorted.slice(firstImageIndex) : sorted
  const sceneDescriptions = serialStoryboards.map((item) => item.prompt_used || item.scene_description || '')
  const dialogues = serialStoryboards.map((item) => item.dialogue || '')
  const durations = serialStoryboards.map((item) => item.duration || 0)
  const cameraMovements = serialStoryboards.map((item) => item.camera_movement || '')
  const moods = serialStoryboards.map((item) => item.mood || '')
  const spatialAnchors = serialStoryboards.map((item) => item.spatial_anchor || '')
  const subjectPositions = serialStoryboards.map((item) => item.subject_positions || '')
  const transitionNotes = serialStoryboards.map((item) => item.transition_note || '')
  const sceneCharacters = serialStoryboards.map((item) => item.characters || [])
  const sceneAssetIds = serialStoryboards.map((item) => item.asset_ids || [])

  return {
    episode_id: episodeId,
    image_urls: serialStoryboards.map((item, index) => index === 0 ? item.image_url : ''),
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
    scene_group_keys: serialStoryboards.map(() => `ad-episode-${episodeId || 'single'}`),
  }
}

export default function AdVideoHistoryDetailPage() {
  const { toast } = useToast()
  const params = useParams<{ projectId: string }>()
  const projectId = Number(params?.projectId || 0)

  const [editableOptimizedScript, setEditableOptimizedScript] = useState('')
  const [editableOriginalScript, setEditableOriginalScript] = useState('')
  const [editableOptimizationPrompt, setEditableOptimizationPrompt] = useState('')
  const [editableStoryboardSplitPrompt, setEditableStoryboardSplitPrompt] = useState('')
  const [optimizingCopy, setOptimizingCopy] = useState(false)
  const [adCopyOptimizationPending, setAdCopyOptimizationPending] = useState(false)
  const [savingCopyDraft, setSavingCopyDraft] = useState(false)
  const [selectedEpisodeId, setSelectedEpisodeId] = useState<string>('all')
  const [generationAction, setGenerationAction] = useState<string | null>(null)
  const [rerunAction, setRerunAction] = useState<string | null>(null)
  const [uploadingAssetId, setUploadingAssetId] = useState<number | null>(null)
  const [selectedTextModelId, setSelectedTextModelId] = useState('default')
  const [selectedConstraintVideoModel, setSelectedConstraintVideoModel] = useState('')
  const [selectedGenerationVideoModel, setSelectedGenerationVideoModel] = useState('')
  const [selectedAspectRatio, setSelectedAspectRatio] = useState('')
  const [activePipelineStep, setActivePipelineStep] = useState<'step1' | 'step2' | 'step3'>('step1')
  const userSelectedPipelineStepRef = useRef(false)
  const previousStoryboardsRef = useRef<Storyboard[]>([])
  const actionLocksRef = useRef<Set<string>>(new Set())
  const [selectedResolution, setSelectedResolution] = useState('')
  const [selectedDuration, setSelectedDuration] = useState('')
  const [selectedStep3AspectRatio, setSelectedStep3AspectRatio] = useState('')
  const [selectedStep3Resolution, setSelectedStep3Resolution] = useState('')
  const [selectedStep3Duration, setSelectedStep3Duration] = useState('')
  const [selectedSpeechPace, setSelectedSpeechPace] = useState<SpeechPaceOption>('normal')
  const [selectedGenerateAudio, setSelectedGenerateAudio] = useState(false)
  const [selectedStep3GenerateAudio, setSelectedStep3GenerateAudio] = useState(false)
  const [videoModelMismatch, setVideoModelMismatch] = useState('')
  const [focusedAssetId, setFocusedAssetId] = useState<number | null>(null)
  const [focusedStoryboardId, setFocusedStoryboardId] = useState<number | null>(null)

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
  }, {
    revalidateOnFocus: true,
    refreshInterval: adCopyOptimizationPending ? 3000 : 0,
  })

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

  const { data: textModels } = useSWR(projectId ? ['ad-video-history-text-models'] : null, async () => {
    const res = await modelAPI.list({ type: 'llm', enabled: 'true', sort_by: 'priority' })
    const payload = res as {
      data?: Array<{ id: number; name: string; model_key: string; is_active?: boolean }> | { items?: Array<{ id: number; name: string; model_key: string; is_active?: boolean }> }
    }
    const rawItems = Array.isArray(payload?.data)
      ? payload.data
      : Array.isArray(payload?.data?.items)
        ? payload.data.items
        : []
    return rawItems.filter((item) => item.is_active !== false)
  }, { revalidateOnFocus: false })

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
  const textModelOptions = useMemo(() => {
    const base = (textModels || []).map((item) => ({
      id: item.id,
      label: `${item.name} · ${item.model_key}`,
    }))
    const persistedId = project?.text_model_id
    if (persistedId && !base.some((item) => item.id === persistedId)) {
      base.unshift({
        id: persistedId,
        label: `当前项目已选模型 #${persistedId}（未出现在当前可用列表中）`,
      })
    }
    return base
  }, [project?.text_model_id, textModels])
  const assets = (assetsData || []).slice().sort((a, b) => Number(a.id) - Number(b.id))
  const availableModels = (videoModelStatus || []).filter((item) => item.available)
  const latestTask = useMemo(() => tasks.slice().sort((a, b) => Number(b.id) - Number(a.id))[0] || null, [tasks])
  const autoSplit = project?.progress?.auto_split || null
  const realOptimizedScript = useMemo(
    () => String(adCopyState?.optimized_script || autoSplit?.optimized_script || '').trim(),
    [adCopyState?.optimized_script, autoSplit?.optimized_script],
  )
  const displayedOptimizedScript = useMemo(
    () => editableOptimizedScript.trim() || realOptimizedScript,
    [editableOptimizedScript, realOptimizedScript],
  )
  const realOriginalScript = useMemo(
    () => String(adCopyState?.original_script || autoSplit?.original_script || project?.script_text || '').trim(),
    [adCopyState?.original_script, autoSplit?.original_script, project?.script_text],
  )
  const realOptimizationPrompt = useMemo(
    () => String(adCopyState?.optimization_prompt || autoSplit?.optimization_prompt || project?.storyboard_config?.ad_copy_optimization_prompt || DEFAULT_AD_COPY_OPTIMIZATION_PROMPT).trim(),
    [adCopyState?.optimization_prompt, autoSplit?.optimization_prompt, project?.storyboard_config?.ad_copy_optimization_prompt],
  )
  const isAdCopyProgressAdvancing = useMemo(() => {
    if (!project?.progress) return false
    return project.progress.stage === 'script_processing'
      || project.progress.stage === 'episode_splitting'
      || project.progress.stage === 'scene_splitting'
      || project.progress.stage === 'script_prepping'
  }, [project?.progress])
  const storyboardSplitBuiltinPrompt = useMemo(
    () => String(adCopyState?.storyboard_split_prompt_builtin || DEFAULT_STORYBOARD_SPLIT_BUILTIN_PROMPT).trim(),
    [adCopyState?.storyboard_split_prompt_builtin],
  )
  const selectedSpeechPaceMeta = useMemo(
    () => SPEECH_PACE_OPTIONS.find((item) => item.value === selectedSpeechPace) || SPEECH_PACE_OPTIONS[0],
    [selectedSpeechPace],
  )
  const storyboardSplitPromptPreview = useMemo(() => {
    const custom = editableStoryboardSplitPrompt.trim()
    const paceBlock = `# 本次步骤 1 语速档位\n${selectedSpeechPaceMeta.label}：${selectedSpeechPaceMeta.hint}`
    return custom
      ? `${storyboardSplitBuiltinPrompt}

${paceBlock}

# 项目级补充规则
${custom}` : `${storyboardSplitBuiltinPrompt}

${paceBlock}`
  }, [editableStoryboardSplitPrompt, selectedSpeechPaceMeta, storyboardSplitBuiltinPrompt])
  const resultUrl = taskResultUrl(latestTask)

  const selectedEpisodeNumber = useMemo(() => {
    const value = Number(selectedEpisodeId)
    return Number.isFinite(value) && value > 0 ? value : null
  }, [selectedEpisodeId])

  const selectedEpisode = useMemo(
    () => episodes.find((episode) => episode.id === selectedEpisodeNumber) || null,
    [episodes, selectedEpisodeNumber],
  )

  useEffect(() => {
    if (selectedEpisodeId === 'all') return
    if (selectedEpisode) return
    setSelectedEpisodeId('all')
  }, [selectedEpisode, selectedEpisodeId])

  const persistedVideoModel = useMemo(
    () => String(project?.storyboard_config?.video_model || autoSplit?.video_model || '').trim(),
    [project?.storyboard_config?.video_model, autoSplit?.video_model],
  )

  const effectiveConstraintVideoModel = useMemo(
    () => selectedConstraintVideoModel || persistedVideoModel,
    [selectedConstraintVideoModel, persistedVideoModel],
  )

  const effectiveSelectedVideoModel = useMemo(
    () => selectedGenerationVideoModel || persistedVideoModel,
    [selectedGenerationVideoModel, persistedVideoModel],
  )

  const constraintModelMeta = useMemo(
    () => availableModels.find((item) => item.key === effectiveConstraintVideoModel) || null,
    [availableModels, effectiveConstraintVideoModel],
  )

  const selectedModelMeta = useMemo(
    () => availableModels.find((item) => item.key === effectiveSelectedVideoModel) || null,
    [availableModels, effectiveSelectedVideoModel],
  )
  const selectedModelParams = useMemo(() => selectedModelMeta?.params || [], [selectedModelMeta])

  const videoModelsForStep3 = useMemo(() => {
    const map = new Map<string, VideoModelMeta>()
    for (const item of availableModels) map.set(item.key, item)
    if (persistedVideoModel && !map.has(persistedVideoModel)) {
      map.set(persistedVideoModel, {
        key: persistedVideoModel,
        available: false,
        native_audio: false,
        params: [],
      })
    }
    return Array.from(map.values())
  }, [availableModels, persistedVideoModel])

  const aspectRatioOptions = useMemo(() => getParamOptions(constraintModelMeta, 'aspect_ratio'), [constraintModelMeta])
  const resolutionOptions = useMemo(() => getParamOptions(constraintModelMeta, 'resolution'), [constraintModelMeta])
  const durationOptions = useMemo(() => getParamOptions(constraintModelMeta, 'duration'), [constraintModelMeta])
  const step3AspectRatioOptions = useMemo(() => getParamOptions(selectedModelMeta, 'aspect_ratio'), [selectedModelMeta])
  const step3ResolutionOptions = useMemo(() => getParamOptions(selectedModelMeta, 'resolution'), [selectedModelMeta])
  const step3DurationOptions = useMemo(() => getParamOptions(selectedModelMeta, 'duration'), [selectedModelMeta])
  const persistedDuration = useMemo(
    () => String(project?.storyboard_config?.duration || autoSplit?.duration || '').trim(),
    [project?.storyboard_config?.duration, autoSplit?.duration],
  )
  const durationMismatch = useMemo(
    () => Boolean(persistedDuration && durationOptions.length > 0 && !durationOptions.some((item) => item.value === persistedDuration)),
    [durationOptions, persistedDuration],
  )

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
    effectiveConstraintVideoModel
      && selectedAspectRatio
      && selectedResolution
      && selectedDuration
      && aspectRatioOptions.length > 0
      && resolutionOptions.length > 0
      && durationOptions.length > 0,
  )

  const step3ConfigReady = Boolean(
    effectiveSelectedVideoModel
      && selectedStep3AspectRatio
      && selectedStep3Resolution
      && selectedStep3Duration
      && step3AspectRatioOptions.length > 0
      && step3ResolutionOptions.length > 0
      && step3DurationOptions.length > 0,
  )

  const pipelineBusy = Boolean(rerunAction !== null || generationAction !== null || uploadingAssetId !== null)
  const step1Running = rerunAction === 'pipeline'
    || project?.status === 'script_processing'
    || project?.progress?.stage === 'episode_splitting'
    || project?.progress?.stage === 'scene_splitting'
    || project?.progress?.stage === 'script_prepping'

  const displayStoryboards = useMemo(() => {
    if (scopeStoryboards.length > 0) return scopeStoryboards
    if (step1Running && previousStoryboardsRef.current.length > 0) return previousStoryboardsRef.current
    return []
  }, [scopeStoryboards, step1Running])

  useEffect(() => {
    if (scopeStoryboards.length > 0) return
    if (selectedEpisodeId !== 'all' && storyboards.length > 0 && !step1Running) {
      setSelectedEpisodeId('all')
    }
  }, [scopeStoryboards, selectedEpisodeId, step1Running, storyboards.length])

  const completedStoryboardImages = useMemo(
    () => displayStoryboards.filter((item) => String(item.image_url || '').trim()).length,
    [displayStoryboards],
  )

  const assetToStoryboardMap = useMemo(() => {
    const map = new Map<number, Storyboard[]>()
    for (const storyboard of displayStoryboards) {
      for (const assetId of storyboard.asset_ids || []) {
        const bucket = map.get(assetId) ?? []
        bucket.push(storyboard)
        map.set(assetId, bucket)
      }
    }
    return map
  }, [displayStoryboards])

  const storyboardAssetDetailMap = useMemo(() => {
    const map = new Map<number, Asset[]>()
    for (const storyboard of displayStoryboards) {
      const details = (storyboard.asset_ids || [])
        .map((assetId) => scopeAssets.find((asset) => asset.id === assetId) || assets.find((asset) => asset.id === assetId) || null)
        .filter((item): item is Asset => Boolean(item))
      map.set(storyboard.id, details)
    }
    return map
  }, [displayStoryboards, scopeAssets, assets])

  const focusedStoryboardIds = useMemo(() => {
    if (focusedAssetId == null) return new Set<number>()
    return new Set((assetToStoryboardMap.get(focusedAssetId) || []).map((storyboard) => storyboard.id))
  }, [assetToStoryboardMap, focusedAssetId])

  const focusedAssetIds = useMemo(() => {
    if (focusedStoryboardId == null) return new Set<number>()
    return new Set((storyboardAssetDetailMap.get(focusedStoryboardId) || []).map((asset) => asset.id))
  }, [storyboardAssetDetailMap, focusedStoryboardId])

  const storyboardScopeReady = displayStoryboards.length > 0
  const assetScopeReady = scopeAssets.length > 0
  const allScopeAssetsUploaded = assetScopeReady && uploadedScopeAssets === scopeAssets.length
  const storyboardImagesReady = storyboardScopeReady && completedStoryboardImages > 0
  const storyboardImagesComplete = storyboardScopeReady && completedStoryboardImages === displayStoryboards.length
  const serialVideoSeedReady = storyboardScopeReady && displayStoryboards.some((item) => String(item.image_url || '').trim())

  const step1Done = splitConfigReady && storyboardScopeReady && !step1Running
  const step2Running = generationAction?.startsWith('asset-') || generationAction?.startsWith('storyboard-image-') || uploadingAssetId !== null || project?.status === 'asset_generating' || project?.status === 'storyboard_generating'
  const step2Enabled = step1Done
  const step2Done = step1Done && assetScopeReady && allScopeAssetsUploaded && storyboardImagesComplete
  const step3Running = generationAction === 'video-start' || processingVideoTaskCount > 0 || project?.status === 'video_generating'
  const step3Enabled = step1Done && serialVideoSeedReady && step3ConfigReady
  const step3Done = Boolean(resultUrl)

  const step1Status: 'pending' | 'active' | 'done' | 'blocked' = step1Running ? 'active' : step1Done ? 'done' : 'pending'
  const step2Status: 'pending' | 'active' | 'done' | 'blocked' = !step2Enabled ? 'blocked' : step2Running ? 'active' : step2Done ? 'done' : 'pending'
  const step3Status: 'pending' | 'active' | 'done' | 'blocked' = !step3Enabled ? 'blocked' : step3Running ? 'active' : step3Done ? 'done' : 'pending'

  useEffect(() => {
    if (userSelectedPipelineStepRef.current && !step1Running && !step2Running && !step3Running) return
    if (step1Running) {
      setActivePipelineStep('step1')
      return
    }
    if (step2Running) {
      setActivePipelineStep('step2')
      return
    }
    if (step3Running) {
      setActivePipelineStep('step3')
      return
    }
  }, [step1Running, step2Running, step3Running])

  const step1Hint = step1Running
    ? '当前正在重跑文本拆分 / 自动分镜；后端会重建分集与分镜，若原分集范围失效，页面会自动回到“全部分集”。'
    : !splitConfigReady
      ? '先补齐视频模型、比例、分辨率、单分镜时长。'
      : !editableOriginalScript.trim()
        ? '当前原文为空，无法拆分。'
        : storyboardScopeReady
          ? `当前范围已经有可用分镜，可继续重跑覆盖；当前语速档位：${selectedSpeechPaceMeta.label}。`
          : `先执行这一步，按 ${selectedSpeechPaceMeta.label} 语速产出新的分集与分镜文本。若原分集范围失效，页面会自动回到“全部分集”。`

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
    ? '当前范围还没有可用的首张分镜图。先在步骤 2 至少生成第 1 张分镜图，后续视频会用上一段视频尾帧作为下一段首帧串行衔接。'
    : !step3ConfigReady
      ? '请先在步骤 3 选择一个可用视频模型，并补齐它支持的比例 / 分辨率 / 时长参数。'
      : step3Running
        ? '当前已经有视频任务在执行，先等这一轮结果。'
      : completedStoryboardImages === 0
        ? '当前范围还没有可用的首张分镜图，所以现在不能提交视频。'
        : !step2Done
          ? '当前范围已有首张分镜图，可以直接提交串行视频；后续 clip 将复用前一段视频检测到的尾帧作为下一段首帧，不再要求每条分镜图都先生成。'
          : step3Done
            ? '当前已经有成片结果；如果不满意，可以基于这一版分镜图继续重生。'
            : '当前范围已经有首张分镜图，可以开始提交串行视频任务。'

  useEffect(() => {
    if (!realOptimizedScript) return
    setEditableOptimizedScript((prev) => {
      if (!prev.trim()) return realOptimizedScript
      if (prev.trim() === realOptimizedScript) return prev
      if (step1Running || optimizingCopy) return prev
      return realOptimizedScript
    })
  }, [realOptimizedScript, step1Running, optimizingCopy])

  useEffect(() => {
    if (!adCopyOptimizationPending) return
    if (realOptimizedScript || isAdCopyProgressAdvancing) {
      setAdCopyOptimizationPending(false)
    }
  }, [adCopyOptimizationPending, realOptimizedScript, isAdCopyProgressAdvancing])

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
    const preferred = persistedVideoModel
    if (preferred && availableModels.some((item) => item.key === preferred)) {
      setVideoModelMismatch('')
      setSelectedConstraintVideoModel((prev) => (prev && availableModels.some((item) => item.key === prev) ? prev : preferred))
      setSelectedGenerationVideoModel((prev) => (prev && availableModels.some((item) => item.key === prev) ? prev : preferred))
      return
    }
    if (preferred) {
      setVideoModelMismatch(preferred)
    } else {
      setVideoModelMismatch('')
    }
    const fallback = availableModels[0]?.key || ''
    setSelectedConstraintVideoModel((prev) => (prev && availableModels.some((item) => item.key === prev) ? prev : fallback))
    setSelectedGenerationVideoModel((prev) => (prev && availableModels.some((item) => item.key === prev) ? prev : fallback))
  }, [availableModels, persistedVideoModel])

  useEffect(() => {
    const persistedTextModelId = project?.text_model_id ? String(project.text_model_id) : 'default'
    setSelectedTextModelId(persistedTextModelId)
  }, [project?.text_model_id])

  useEffect(() => {
    const nextAspect = pickAllowedValue(aspectRatioOptions, project?.storyboard_config?.aspect_ratio)
    setSelectedAspectRatio((prev) => (prev && aspectRatioOptions.some((item) => item.value === prev) ? prev : nextAspect))
  }, [aspectRatioOptions, project?.storyboard_config?.aspect_ratio])

  useEffect(() => {
    const nextAspect = pickAllowedValue(step3AspectRatioOptions, project?.storyboard_config?.aspect_ratio)
    setSelectedStep3AspectRatio((prev) => (prev && step3AspectRatioOptions.some((item) => item.value === prev) ? prev : nextAspect))
  }, [step3AspectRatioOptions, project?.storyboard_config?.aspect_ratio])

  useEffect(() => {
    const nextResolution = pickAllowedValue(resolutionOptions, project?.storyboard_config?.resolution)
    setSelectedResolution((prev) => (prev && resolutionOptions.some((item) => item.value === prev) ? prev : nextResolution))
  }, [resolutionOptions, project?.storyboard_config?.resolution])

  useEffect(() => {
    const nextResolution = pickAllowedValue(step3ResolutionOptions, project?.storyboard_config?.resolution)
    setSelectedStep3Resolution((prev) => (prev && step3ResolutionOptions.some((item) => item.value === prev) ? prev : nextResolution))
  }, [step3ResolutionOptions, project?.storyboard_config?.resolution])

  useEffect(() => {
    const nextDuration = pickAllowedValue(durationOptions, persistedDuration, false)
    setSelectedDuration((prev) => {
      if (prev && (durationOptions.some((item) => item.value === prev) || prev === persistedDuration)) return prev
      return nextDuration
    })
  }, [durationOptions, persistedDuration])

  useEffect(() => {
    const nextDuration = pickAllowedValue(step3DurationOptions, persistedDuration, false)
    setSelectedStep3Duration((prev) => {
      if (prev && (step3DurationOptions.some((item) => item.value === prev) || prev === persistedDuration)) return prev
      return nextDuration
    })
  }, [step3DurationOptions, persistedDuration])

  useEffect(() => {
    const persisted = String(project?.storyboard_config?.speech_pace || '').trim() as SpeechPaceOption | ''
    if (persisted && SPEECH_PACE_OPTIONS.some((item) => item.value === persisted)) {
      setSelectedSpeechPace(persisted)
      return
    }
    setSelectedSpeechPace('normal')
  }, [project?.storyboard_config?.speech_pace])

  useEffect(() => {
    setSelectedGenerateAudio(Boolean(project?.storyboard_config?.generate_audio))
  }, [project?.storyboard_config?.generate_audio])

  useEffect(() => {
    setSelectedStep3GenerateAudio(Boolean(project?.storyboard_config?.generate_audio))
  }, [project?.storyboard_config?.generate_audio])

  const refreshAll = async () => {
    await Promise.all([mutateProject(), mutateEpisodes(), mutateStoryboards(), mutateTasks(), mutateAssets(), mutateAdCopyState()])
  }

  const acquireActionLock = (action: string) => {
    if (actionLocksRef.current.has(action)) return false
    actionLocksRef.current.add(action)
    return true
  }

  const releaseActionLock = (action: string) => {
    actionLocksRef.current.delete(action)
  }

  const runScopedAction = async (action: string, runner: () => Promise<unknown>, successTitle: string) => {
    if (!acquireActionLock(action)) {
      toast({ title: '当前操作正在进行中，请勿重复点击', variant: 'destructive' })
      return
    }
    setGenerationAction(action)
    try {
      await runner()
      await refreshAll()
      toast({ title: successTitle, variant: 'success' })
    } catch (error) {
      toast({ title: error instanceof Error ? error.message : '操作失败', variant: 'destructive' })
    } finally {
      setGenerationAction(null)
      releaseActionLock(action)
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
      await projectAPI.optimizeAdCopy(projectId, {
        original_script: originalScript,
        optimization_prompt: optimizationPrompt,
        persist_original: true,
      })
      setAdCopyOptimizationPending(true)
      await refreshAll()
      toast({ title: displayedOptimizedScript ? '已启动重新优化，结果会自动回填' : '已启动文案优化，结果会自动回填', variant: 'success' })
    } catch (error) {
      toast({ title: error instanceof Error ? error.message : '文案优化失败', variant: 'destructive' })
    } finally {
      setOptimizingCopy(false)
    }
  }

  const rerunStoryboardPipeline = async () => {
    if (!acquireActionLock('pipeline')) {
      toast({ title: '步骤 1 已在进行中，请勿重复点击', variant: 'destructive' })
      return
    }

    const scriptText = editableOriginalScript.trim()
    if (!scriptText) {
      toast({ title: '请先保留一版可用的原文，再开始按文本模型重拆分', variant: 'destructive' })
      releaseActionLock('pipeline')
      return
    }
    if (!splitConfigReady) {
      toast({ title: '当前模型没有完整声明 aspect_ratio / resolution / duration，不能启动这条广告流水线', variant: 'destructive' })
      releaseActionLock('pipeline')
      return
    }
    if (project?.status === 'script_processing' || project?.progress?.stage === 'episode_splitting') {
      toast({ title: '当前项目仍在拆分中，请等本轮完成后再重跑，避免再次触发 409', variant: 'destructive' })
      releaseActionLock('pipeline')
      return
    }

    setRerunAction('pipeline')
    try {
      await projectAPI.update(projectId, {
        text_model_id: selectedTextModelId === 'default' ? undefined : Number(selectedTextModelId),
      })
      await storyboardAPI.updateConfig(projectId, {
        video_model: effectiveConstraintVideoModel,
        aspect_ratio: selectedAspectRatio,
        resolution: selectedResolution,
        duration: Number(selectedDuration),
        speech_pace: selectedSpeechPace,
        auto_split_after_optimization: true,
        generate_audio: Boolean(selectedModelMeta?.native_audio && selectedGenerateAudio),
      })

      const filenameBase = (project?.title || `ad-project-${projectId}`).trim() || `ad-project-${projectId}`
      const file = new File([scriptText], `${filenameBase}-pipeline.txt`, { type: 'text/plain' })
      await projectAPI.uploadScript(projectId, file)
      await projectAPI.generateEpisodes(projectId, undefined, { rebuild: true, autoStoryboard: true })
      await refreshAll()
      toast({
        title: `已按当前文本模型、${selectedSpeechPaceMeta.label}语速，并参考所选视频时长约束，重跑“文本拆分 → 分镜文本”`,
        variant: 'success',
      })
    } catch (error) {
      toast({ title: error instanceof Error ? error.message : '重跑拆分失败', variant: 'destructive' })
    } finally {
      setRerunAction(null)
      releaseActionLock('pipeline')
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

  const regenerateSingleStoryboardImage = async (storyboardId: number, sequenceNumber: number) => {
    const action = `storyboard-regenerate-${storyboardId}`
    await runScopedAction(
      action,
      () => storyboardAPI.generate(projectId, storyboardId),
      `已重新提交分镜 #${sequenceNumber} 的分镜图生成`,
    )
  }

  const regenerateSingleAssetImage = async (assetId: number, assetName: string) => {
    const action = `asset-regenerate-${assetId}`
    await runScopedAction(
      action,
      () => assetAPI.retry(projectId, assetId),
      `已重新提交参考图槽位「${assetName || `#${assetId}`}」的生成`,
    )
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
    if (!acquireActionLock('video-start')) {
      toast({ title: '步骤 3 已在进行中，请勿重复点击', variant: 'destructive' })
      return
    }

    if (!step3ConfigReady) {
      toast({ title: '步骤 3 当前模型没有完整声明 aspect_ratio / resolution / duration，不能直接提交视频生成', variant: 'destructive' })
      releaseActionLock('video-start')
      return
    }

    const storyboardPool = scopeStoryboards
      .slice()
      .sort((a, b) => a.sequence_number - b.sequence_number)

    if (storyboardPool.length === 0) {
      toast({ title: '当前范围还没有分镜文本，请先完成步骤 1', variant: 'destructive' })
      releaseActionLock('video-start')
      return
    }
    if (!storyboardPool.some((item) => String(item.image_url || '').trim())) {
      toast({ title: '当前范围还没有可用的首张分镜图，请先在步骤 2 至少生成第 1 张分镜图', variant: 'destructive' })
      releaseActionLock('video-start')
      return
    }

    const renderConfig: Record<string, unknown> = {
      aspect_ratio: selectedStep3AspectRatio,
      resolution: selectedStep3Resolution,
      generate_audio: selectedModelMeta?.native_audio ? selectedStep3GenerateAudio : undefined,
    }

    const stylePreset = project?.storyboard_config?.style_preset || autoSplit?.style_preset || undefined
    const motionMode = project?.storyboard_config?.motion_mode || undefined
    const clipDuration = Number(selectedStep3Duration || project?.storyboard_config?.duration || 0) || undefined

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
          model_name: effectiveSelectedVideoModel,
          style_preset: stylePreset,
          motion_mode: motionMode,
          video_mode: project?.video_mode,
          clip_duration_sec: clipDuration,
          render_config: renderConfig,
          serial_scene: true,
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
          .filter((item) => String(item.image_urls?.[0] || '').trim())

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
            model_name: effectiveSelectedVideoModel,
            style_preset: stylePreset,
            motion_mode: motionMode,
            video_mode: project?.video_mode,
            clip_duration_sec: clipDuration,
            render_config: renderConfig,
            serial_scene: true,
          })
        }

        const noEpisodeStoryboards = groups.get(0) || []
        if (noEpisodeStoryboards.length > 0) {
          const payload = buildEpisodeVideoPayload(noEpisodeStoryboards)
          if (String(payload.image_urls?.[0] || '').trim()) {
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
              model_name: effectiveSelectedVideoModel,
              style_preset: stylePreset,
              motion_mode: motionMode,
              video_mode: project?.video_mode,
              clip_duration_sec: clipDuration,
              render_config: renderConfig,
              serial_scene: true,
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
      releaseActionLock('video-start')
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
            </CardHeader>
            <CardContent>
              <Tabs defaultValue="copy" className="space-y-4">
                <TabsList className="h-auto w-full flex-wrap justify-start gap-2 rounded-xl bg-black/20 p-1 text-slate-300">
                  <TabsTrigger value="copy">文案</TabsTrigger>
                  <TabsTrigger value="storyboard">分镜</TabsTrigger>
                  <TabsTrigger value="video">视频</TabsTrigger>
                </TabsList>

                <TabsContent value="copy" className="space-y-4">
                  <div className="rounded-xl border border-cyan-500/20 bg-cyan-500/10 p-4 space-y-4">
                    <div className="flex flex-wrap items-start justify-between gap-3">
                      <div>
                        <div className="text-sm font-medium text-cyan-100">文案优化提示词</div>
                        <div className="mt-1 text-xs text-cyan-100/80">这里展示并编辑当前广告项目真实使用的优化提示词。前一步的目标不是泛化润色，而是把项目文案优化成更适合后续“按台词 / 口播为主进行拆分”的基准稿。</div>
                      </div>
                      <div className="flex flex-wrap gap-2">
                        <Button variant="outline" onClick={() => { void saveAdCopyDraft() }} disabled={savingCopyDraft || optimizingCopy || adCopyOptimizationPending || pipelineBusy}>
                          {savingCopyDraft ? '保存中…' : '保存原文 / 文案'}
                        </Button>
                        <Button onClick={() => { void optimizeAdCopy() }} disabled={optimizingCopy || adCopyOptimizationPending || savingCopyDraft || pipelineBusy}>
                          {optimizingCopy ? '提交中…' : adCopyOptimizationPending ? '优化进行中…' : displayedOptimizedScript ? '重新优化' : '开始优化'}
                        </Button>
                      </div>
                    </div>
                    <Textarea
                      value={editableOptimizationPrompt}
                      onChange={(e) => setEditableOptimizationPrompt(e.target.value)}
                      className="min-h-[180px] border-cyan-500/20 bg-black/20 text-slate-100 caret-cyan-300"
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
                        <div className="text-[11px] text-emerald-200/75">{displayedOptimizedScript.length} 字</div>
                      </div>
                      {displayedOptimizedScript ? (
                        <Textarea
                          value={editableOptimizedScript}
                          onChange={(e) => setEditableOptimizedScript(e.target.value)}
                          className="min-h-[520px] border-emerald-500/20 bg-black/20 text-slate-100 caret-emerald-300"
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
                          <div className="mt-1 text-[11px] text-slate-400">这里支持直接修改原文；点击上方“保存原文 / 文案”后会真实保存。下次点击“开始优化 / 重新优化”会以这里的当前文本为准。</div>
                        </div>
                        <div className="text-[11px] text-slate-500">{editableOriginalScript.trim().length || realOriginalScript.length} 字</div>
                      </div>
                      <Textarea
                        value={editableOriginalScript}
                        onChange={(e) => setEditableOriginalScript(e.target.value)}
                        className="min-h-[520px] border-white/10 bg-black/20 text-slate-100 caret-cyan-300"
                        placeholder="请输入或调整当前原文。"
                      />
                    </div>
                  </div>
                </TabsContent>

                <TabsContent value="storyboard" className="space-y-4">
                  <div className="rounded-xl border border-white/10 bg-black/20 p-4">
                    <div className="flex flex-wrap items-center justify-between gap-3">
                      <div className="text-sm font-medium text-white">当前范围分镜</div>
                      <div className="text-[11px] text-slate-400">当前范围：{scopeLabel}{step1Running && scopeStoryboards.length === 0 && previousStoryboardsRef.current.length > 0 ? ' · 正在重建，先显示上一版' : ''}</div>
                    </div>
                  </div>

                  {displayStoryboards.length === 0 ? (
                    <div className="rounded-lg border border-white/10 bg-black/20 p-4 text-sm text-slate-300">{step1Running ? '当前正在重跑步骤 1，后端会先删旧分镜再重建新分镜，请稍等这一轮回流。' : storyboards.length > 0 ? '当前分集范围没有命中新分镜，页面已自动回退到“全部分集”重新展示。' : '当前范围还没有分镜记录。'}</div>
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
                        <div className="rounded-lg border border-violet-500/20 bg-violet-500/10 p-3">
                          <div className="mb-2 flex items-center justify-between gap-2">
                            <div className="text-xs font-medium text-violet-200">台词 / 口播</div>
                            <div className="text-[11px] text-violet-200/75">目标时长：{storyboard.duration || '-'} 秒</div>
                          </div>
                          <div className="whitespace-pre-wrap break-words text-sm text-slate-100">{storyboard.dialogue || '暂无台词'}</div>
                        </div>
                        <div className="rounded-lg border border-cyan-500/20 bg-cyan-500/10 p-3">
                          <div className="mb-2 text-xs font-medium text-cyan-200">场景描述</div>
                          <div className="whitespace-pre-wrap break-words text-sm text-slate-100">{storyboard.scene_description || '暂无场景描述'}</div>
                        </div>
                      </div>
                      <div className="grid gap-2 text-xs text-slate-400 md:grid-cols-4">
                        <div>location：{storyboard.location || '-'}</div>
                        <div>camera：{storyboard.camera_movement || '-'}</div>
                        <div>asset_ids：{storyboard.asset_ids?.length ? storyboard.asset_ids.join(', ') : '-'}</div>
                        <div>状态：{storyboard.status || '-'}</div>
                      </div>
                    </div>
                  ))}
                </TabsContent>

                <TabsContent value="video" className="space-y-4">
                  <div className="rounded-xl border border-white/10 bg-black/20 p-4 space-y-3">
                    <div className="text-sm font-medium text-white">视频任务 / 完整视频</div>

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
                <button type="button" onClick={() => { userSelectedPipelineStepRef.current = true; setActivePipelineStep('step1') }} className={`rounded-xl border p-4 text-left transition ${stepTone(step1Status)} ${activePipelineStep === 'step1' ? 'ring-2 ring-white/30' : 'hover:bg-white/5'}`}>
                  <div className="flex items-center justify-between gap-2">
                    <div className="text-xs font-medium">步骤 1</div>
                    <div className="rounded-full border border-current/20 px-2 py-0.5 text-[10px]">{stepLabel(step1Status)}</div>
                  </div>
                  <div className="mt-1 text-base font-semibold text-white">按台词时长重拆分文本</div>
                  <div className="mt-2 text-xs text-current/80">先确定视频模型、单分镜时长与语速，再按台词 / 口播承载量重跑当前文案；比例和分辨率用于同步约束构图与画面复杂度。重拆分后若原分集编号失效，页面会自动回到全部分集展示新结果。</div>
                  <div className="mt-3 text-[11px] text-current/80">{activePipelineStep === 'step1' ? '当前已展开' : '点击查看这一步的详细操作'}</div>
                </button>
                <button type="button" onClick={() => { userSelectedPipelineStepRef.current = true; setActivePipelineStep('step2') }} className={`rounded-xl border p-4 text-left transition ${stepTone(step2Status)} ${activePipelineStep === 'step2' ? 'ring-2 ring-white/30' : 'hover:bg-white/5'}`}>
                  <div className="flex items-center justify-between gap-2">
                    <div className="text-xs font-medium">步骤 2</div>
                    <div className="rounded-full border border-current/20 px-2 py-0.5 text-[10px]">{stepLabel(step2Status)}</div>
                  </div>
                  <div className="mt-1 text-base font-semibold text-white">上传人物图 / 刷新分镜图</div>
                  <div className="mt-2 text-xs text-current/80">先准备人物 / 素材槽位，再为当前范围逐个上传真实参考图，最后刷新分镜图。</div>
                  <div className="mt-3 text-[11px] text-current/80">{activePipelineStep === 'step2' ? '当前已展开' : '点击查看这一步的详细操作'}</div>
                </button>
                <button type="button" onClick={() => { userSelectedPipelineStepRef.current = true; setActivePipelineStep('step3') }} className={`rounded-xl border p-4 text-left transition ${stepTone(step3Status)} ${activePipelineStep === 'step3' ? 'ring-2 ring-white/30' : 'hover:bg-white/5'}`}>
                  <div className="flex items-center justify-between gap-2">
                    <div className="text-xs font-medium">步骤 3</div>
                    <div className="rounded-full border border-current/20 px-2 py-0.5 text-[10px]">{stepLabel(step3Status)}</div>
                  </div>
                  <div className="mt-1 text-base font-semibold text-white">开始生成视频</div>
                  <div className="mt-2 text-xs text-current/80">只有分镜图准备完成后才启动视频生成，避免直接拿空图或错图提交。</div>
                  <div className="mt-3 text-[11px] text-current/80">{activePipelineStep === 'step3' ? '当前已展开' : '点击查看这一步的详细操作'}</div>
                </button>
              </div>

              <div className="grid gap-4">
                {activePipelineStep === 'step1' && (
                  <div className="rounded-xl border border-cyan-500/20 bg-cyan-500/10 p-4 space-y-4">
                  <div>
                    <div className="text-sm font-medium text-cyan-100">步骤 1：按文本模型重跑“文本 → 分镜文本”</div>
                    <div className="mt-1 text-xs text-cyan-100/80">这里用文本模型完成广告文案优化与自动分集；下方视频模型只用于提供单分镜时长和后续视频能力约束，不是文本拆分模型。</div>
                    {step1Running && (
                      <div className="mt-2 inline-flex rounded-full border border-cyan-400/30 bg-cyan-500/10 px-3 py-1 text-[11px] text-cyan-100">
                        当前进行中：正在用文本模型重跑文本拆分 / 自动分镜，请勿重复点击
                      </div>
                    )}
                  </div>

                  <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-5">
                    <div className="space-y-2 xl:col-span-2">
                      <Label className="text-slate-100">文本模型（步骤 1 实际使用）</Label>
                      <select
                        value={selectedTextModelId}
                        onChange={(e) => setSelectedTextModelId(e.target.value)}
                        className="flex h-10 w-full rounded-xl border border-surface-200 bg-white px-3 py-2 text-sm text-surface-900"
                      >
                        <option value="default">系统默认</option>
                        {textModelOptions.map((item) => (
                          <option key={item.id} value={String(item.id)}>{item.label}</option>
                        ))}
                      </select>
                      <div className="text-[11px] text-cyan-100/75">这里选择的是步骤 1 文案优化 / 台词拆分实际使用的文本模型。默认回填创建项目时选中的文本模型；重跑前会先写回项目。</div>
                      <div className="text-[11px] text-cyan-100/65">当前项目已保存的文本模型 ID：{project?.text_model_id ? String(project.text_model_id) : '未设置'}；当前下拉值：{selectedTextModelId || '空'}</div>
                    </div>

                    <div className="space-y-2 xl:col-span-2">
                      <Label className="text-slate-100">约束模型（仅用于步骤 1 时长/能力约束）</Label>
                      <select
                        value={effectiveConstraintVideoModel}
                        onChange={(e) => setSelectedConstraintVideoModel(e.target.value)}
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
                      <div className="text-[11px] text-cyan-100/75">画面比例不只是输出参数，也会参与步骤 1 的构图约束：它会影响主体排布、左右留白、景别选择和空间层次。</div>
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
                      <div className="text-[11px] text-cyan-100/75">分辨率会影响单镜细节密度。分辨率较低时，步骤 1 会倾向减少同镜头里的小字、复杂背景和过多主体，优先保证卖点清晰。</div>
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
                      <div className="text-[11px] text-cyan-100/75">单分镜时长会直接约束单镜可承载的台词 / 口播长度；步骤 1 会优先按台词承载量判断该合并还是继续拆分，再补足动作与画面。</div>
                    </div>

                    <div className="space-y-2">
                      <Label className="text-slate-100">10 秒语速档位</Label>
                      <select
                        value={selectedSpeechPace}
                        onChange={(e) => setSelectedSpeechPace(e.target.value as SpeechPaceOption)}
                        className="flex h-10 w-full rounded-xl border border-surface-200 bg-white px-3 py-2 text-sm text-surface-900"
                      >
                        {SPEECH_PACE_OPTIONS.map((item) => (
                          <option key={item.value} value={item.value}>{item.label}</option>
                        ))}
                      </select>
                      <div className="text-[11px] text-cyan-100/75">按 10 秒口播承载量控制步骤 1 的拆分密度，防止分镜拆分不够。当前说明：{selectedSpeechPaceMeta.hint}</div>
                    </div>
                  </div>

                    <div className="space-y-2">
                      <Label className="text-slate-100">语音输出</Label>
                      <select
                        value={selectedModelMeta?.native_audio ? (selectedGenerateAudio ? 'enabled' : 'disabled') : 'unsupported'}
                        onChange={(e) => setSelectedGenerateAudio(e.target.value === 'enabled')}
                        disabled={!selectedModelMeta?.native_audio}
                        className="flex h-10 w-full rounded-xl border border-surface-200 bg-white px-3 py-2 text-sm text-surface-900 disabled:cursor-not-allowed disabled:opacity-60"
                      >
                        {selectedModelMeta?.native_audio ? (
                          <>
                            <option value="disabled">不生成语音</option>
                            <option value="enabled">生成语音（原生）</option>
                          </>
                        ) : (
                          <option value="unsupported">当前模型不支持语音</option>
                        )}
                      </select>
                      <div className="text-[11px] text-cyan-100/75">
                        {selectedModelMeta?.native_audio ? '当前模型已声明 native_audio，可选择是否继续透传原生语音能力。' : '当前模型未声明 native_audio，因此这里不可开启语音。'}
                      </div>
                    </div>

                  {videoModelMismatch && (
                    <div className="rounded-lg border border-amber-500/20 bg-amber-500/10 p-3 text-xs text-amber-100">
                      当前项目创建时保存的视频模型是 `{videoModelMismatch}`，但它不在当前 `/api/v1/videos/model-status` 的可用列表里；页面已临时回退到 `{effectiveConstraintVideoModel || effectiveSelectedVideoModel || '未选择'}`。请确认运行态模型配置是否变更。
                    </div>
                  )}

                  {!splitConfigReady && (
                    <div className="rounded-lg border border-amber-500/20 bg-amber-500/10 p-3 text-xs text-amber-100">
                      当前模型必须同时声明 aspect_ratio / resolution / duration，才能进入这条广告流水线。
                    </div>
                  )}

                  {durationMismatch && (
                    <div className="rounded-lg border border-amber-500/20 bg-amber-500/10 p-3 text-xs text-amber-100">
                      当前项目创建时保存的单分镜时长是 `{persistedDuration}` 秒，但它不在当前模型声明的时长列表里；页面会优先保留项目持久化值，不再静默回退成默认的 5 秒。请确认运行态模型参数是否变更。
                    </div>
                  )}

                  <div className="space-y-3">
                    <div className="flex items-center justify-between gap-3">
                      <Label className="text-slate-100">分镜拆分提示词</Label>
                      <span className="text-[11px] text-cyan-100/75">就在开始步骤 1 之前修改；保存后会在下次重跑时生效</span>
                    </div>

                    <div className="space-y-2">
                      <div className="text-[11px] font-medium text-cyan-100/85">步骤 1 内置分镜拆分规则（只读）</div>
                      <div className="rounded-lg border border-cyan-500/20 bg-cyan-500/10 p-3 text-[11px] text-cyan-50 whitespace-pre-wrap break-words">{storyboardSplitBuiltinPrompt}</div>
                    </div>

                    <div className="space-y-2">
                      <div className="flex items-center justify-between gap-3">
                        <div className="text-[11px] font-medium text-slate-200">项目级补充规则（可编辑）</div>
                        <span className="text-[11px] text-slate-400">只保存你额外追加的规则，不覆盖系统底座</span>
                      </div>
                      <Textarea
                        value={editableStoryboardSplitPrompt}
                        onChange={(event) => setEditableStoryboardSplitPrompt(event.target.value)}
                        className="min-h-[180px] border-white/10 bg-black/20 text-slate-100"
                        placeholder="这里填写项目级台词拆分 / 分镜补充规则，例如：同一段口播尽量合并、以台词句群为主切分、无台词镜头比例要低、产品卖点优先由主讲镜头承载。"
                      />
                    </div>

                    <div className="space-y-2">
                      <div className="text-[11px] font-medium text-emerald-100/85">本次实际生效预览</div>
                      <div className="rounded-lg border border-emerald-500/20 bg-emerald-500/10 p-3 text-[11px] text-emerald-50 whitespace-pre-wrap break-words">{storyboardSplitPromptPreview}</div>
                    </div>

                    <div className="rounded-lg border border-white/10 bg-black/20 p-3 text-[11px] text-slate-300">
                      {adCopyState?.storyboard_split_prompt_hint || '上方已拆成两层：步骤 1 内置分镜拆分规则（只读） + 项目级补充规则（可编辑）。最下方绿色区域展示的是本次真正会生效的完整“步骤 1 分镜拆分提示词”预览。'}
                    </div>
                  </div>

                  <div className="flex flex-wrap items-center gap-3">
                    <Button
                      disabled={pipelineBusy || step1Running || !editableOriginalScript.trim() || !splitConfigReady}
                      onClick={() => void rerunStoryboardPipeline()}
                    >
                      {step1Running ? '步骤 1 进行中…' : rerunAction === 'pipeline' ? '正在按当前配置重拆分…' : '开始步骤 1：按文本模型重拆分（参考视频时长 + 语速约束）'}
                    </Button>
                    <div className="rounded-lg border border-white/10 bg-black/20 px-3 py-2 text-[11px] text-cyan-100/80">
                      {step1Hint}
                    </div>
                    <div className="rounded-lg border border-cyan-500/20 bg-cyan-500/10 px-3 py-2 text-[11px] text-cyan-50">
                      提醒：步骤 1 会重新生成分集，旧分集编号可能失效；如果你之前只看某一集，完成后页面会自动切回“全部分集”，避免误判成“没有分镜产出”。
                    </div>
                  </div>
                  </div>
                )}

                {activePipelineStep === 'step2' && (
                  <div className="rounded-xl border border-violet-500/20 bg-violet-500/10 p-4 space-y-4">
                    <div>
                      <div className="text-sm font-medium text-violet-100">步骤 2：准备参考图并刷新分镜图</div>
                      <div className="mt-1 text-xs text-violet-100/80">这一块只做三件事：先生成当前范围需要的素材槽位，再上传参考图，最后刷新这一范围的分镜图。</div>
                      {step2Running && (
                        <div className="mt-2 inline-flex rounded-full border border-violet-400/30 bg-violet-500/10 px-3 py-1 text-[11px] text-violet-100">
                          当前进行中：正在准备素材槽位 / 上传参考图 / 刷新分镜图，请勿重复点击
                        </div>
                      )}
                    </div>

                    <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
                      <div className="rounded-lg border border-white/10 bg-black/20 p-3 text-xs text-violet-100/85">
                        <div className="text-[11px] text-violet-200/70">当前范围</div>
                        <div className="mt-1 text-sm text-white">{scopeLabel}</div>
                      </div>
                      <div className="rounded-lg border border-white/10 bg-black/20 p-3 text-xs text-violet-100/85">
                        <div className="text-[11px] text-violet-200/70">素材槽位</div>
                        <div className="mt-1 text-sm text-white">{scopeAssets.length} 个</div>
                      </div>
                      <div className="rounded-lg border border-white/10 bg-black/20 p-3 text-xs text-violet-100/85">
                        <div className="text-[11px] text-violet-200/70">已上传参考图</div>
                        <div className="mt-1 text-sm text-white">{uploadedScopeAssets} / {scopeAssets.length}</div>
                      </div>
                      <div className="rounded-lg border border-white/10 bg-black/20 p-3 text-xs text-violet-100/85">
                        <div className="text-[11px] text-violet-200/70">可用分镜图</div>
                        <div className="mt-1 text-sm text-white">{completedStoryboardImages} / {displayStoryboards.length}</div>
                      </div>
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
                      <Button
                        size="sm"
                        variant="secondary"
                        disabled={!step3Enabled}
                        onClick={() => { userSelectedPipelineStepRef.current = true; setActivePipelineStep('step3') }}
                      >
                        去步骤 3 生成视频
                      </Button>
                    </div>

                    <div className="rounded-lg border border-white/10 bg-black/20 p-3 text-[11px] text-violet-100/80">
                      {step2Hint}
                      {step2Done && (
                        <div className="mt-2 text-emerald-200/90">
                          当前范围的参考图和分镜图已准备完成，可以直接点上方“去步骤 3 生成视频”。
                        </div>
                      )}
                      {!step2Done && step3Enabled && (
                        <div className="mt-2 text-emerald-200/90">
                        当前范围已经有首张分镜图了，虽然步骤 2 还没完全补齐，但已经可以先去步骤 3 提交串行视频；后续片段会用上一段尾帧接下一段首帧。
                        </div>
                      )}
                    </div>

                    <div className="grid gap-4 xl:grid-cols-[minmax(0,1.1fr)_minmax(0,0.9fr)]">
                      <div className="min-h-0 rounded-xl border border-white/10 bg-black/20 p-4">
                        <div className="mb-3 flex items-center justify-between gap-3">
                          <div className="text-sm font-medium text-white">参考图槽位</div>
                          <div className="text-[11px] text-violet-100/80">已上传 {uploadedScopeAssets} / {scopeAssets.length}</div>
                        </div>

                        <div className="max-h-[520px] space-y-3 overflow-auto pr-1">
                          {scopeAssets.length === 0 ? (
                            <div className="rounded-lg border border-white/10 bg-black/20 p-4 text-xs text-violet-100/80">
                              当前范围还没有可上传的素材槽位。先点上面的“准备人物 / 素材槽位”，系统才会生成这一轮需要上传的角色/物件入口。
                            </div>
                          ) : scopeAssets.map((asset) => (
                            <button
                              key={asset.id}
                              type="button"
                              onClick={() => {
                                setFocusedAssetId((prev) => (prev === asset.id ? null : asset.id))
                                setFocusedStoryboardId(null)
                              }}
                              className={`w-full rounded-lg border p-3 text-left space-y-3 transition ${focusedAssetId === asset.id || focusedAssetIds.has(asset.id) ? 'border-cyan-400/40 bg-cyan-500/10 ring-1 ring-cyan-400/30' : 'border-white/10 bg-slate-950/40 hover:bg-white/5'}`}
                            >
                              <div className="flex items-start justify-between gap-3">
                                <div className="min-w-0 flex-1">
                                  <div className="text-sm font-medium text-white">#{asset.id} · {asset.name || '未命名素材'}</div>
                                  <div className="mt-1 text-[11px] text-slate-400">{asset.type || '-'}{assetToStoryboardMap.get(asset.id)?.length ? ` · 对应 ${assetToStoryboardMap.get(asset.id)?.length || 0} 条分镜` : ' · 当前未映射分镜'}</div>
                                </div>
                                <div className={`rounded-full border px-2 py-0.5 text-[11px] ${String(asset.image_url || '').trim() ? 'border-emerald-400/30 bg-emerald-500/10 text-emerald-200' : 'border-violet-400/30 bg-violet-500/10 text-violet-100'}`}>
                                  {String(asset.image_url || '').trim() ? '已上传' : '待上传'}
                                </div>
                              </div>

                              <div className="grid gap-3 md:grid-cols-[112px_minmax(0,1fr)]">
                                {String(asset.image_url || '').trim() ? (
                                  <div className="overflow-hidden rounded-lg border border-white/10 bg-black/30">
                                    {/* eslint-disable-next-line @next/next/no-img-element */}
                                    <img src={asset.image_url} alt={asset.name || `asset-${asset.id}`} className="h-28 w-full object-cover" />
                                  </div>
                                ) : (
                                  <div className="flex h-28 items-center justify-center rounded-lg border border-dashed border-white/10 bg-black/10 text-[11px] text-slate-500">
                                    暂无参考图
                                  </div>
                                )}

                                <div className="space-y-3">
                                  <div className="flex flex-wrap gap-1">
                                    {(assetToStoryboardMap.get(asset.id) || []).length > 0 ? (assetToStoryboardMap.get(asset.id) || []).map((storyboard) => (
                                      <span key={`asset-${asset.id}-storyboard-${storyboard.id}`} className={`rounded-full border px-2 py-0.5 text-[10px] ${focusedStoryboardId === storyboard.id || focusedStoryboardIds.has(storyboard.id) ? 'border-cyan-300/40 bg-cyan-400/20 text-cyan-50' : 'border-cyan-400/20 bg-cyan-500/10 text-cyan-100'}`}>
                                        分镜 #{storyboard.sequence_number}
                                      </span>
                                    )) : (
                                      <span className="rounded-full border border-white/10 bg-white/5 px-2 py-0.5 text-[10px] text-slate-400">未映射</span>
                                    )}
                                  </div>

                                  <div className="flex flex-wrap items-center gap-2">
                                    <label className="inline-flex cursor-pointer items-center rounded-lg border border-violet-400/30 bg-violet-500/10 px-3 py-2 text-xs text-violet-100 transition hover:bg-violet-500/20">
                                      {uploadingAssetId === asset.id ? '上传中…' : String(asset.image_url || '').trim() ? '重新上传参考图' : '上传参考图'}
                                      <input
                                        type="file"
                                        accept="image/*"
                                        className="hidden"
                                        disabled={uploadingAssetId !== null || !step2Enabled || pipelineBusy}
                                        onChange={(event) => { event.stopPropagation(); void handleAssetUpload(asset.id, event) }}
                                      />
                                    </label>
                                    <Button
                                      type="button"
                                      size="sm"
                                      variant="outline"
                                      className="h-9 rounded-lg border-cyan-400/30 bg-cyan-500/10 px-3 text-xs text-cyan-100 hover:bg-cyan-500/20"
                                      disabled={pipelineBusy || !step2Enabled || uploadingAssetId !== null}
                                      onClick={(event) => {
                                        event.stopPropagation()
                                        void regenerateSingleAssetImage(asset.id, asset.name || `#${asset.id}`)
                                      }}
                                    >
                                      重新生成参考图
                                    </Button>
                                  </div>
                                </div>
                              </div>
                            </button>
                          ))}
                        </div>
                      </div>

                      <div className="min-h-0 rounded-xl border border-white/10 bg-black/20 p-4">
                        <div className="mb-3 flex items-center justify-between gap-3">
                          <div className="text-sm font-medium text-white">当前范围分镜</div>
                          <div className="text-[11px] text-violet-100/80">分镜图 {completedStoryboardImages} / {displayStoryboards.length}</div>
                        </div>

                        <div className="max-h-[520px] space-y-3 overflow-auto pr-1">
                          {displayStoryboards.length === 0 ? (
                            <div className="rounded-lg border border-white/10 bg-black/20 p-4 text-xs text-violet-100/80">
                              {step1Running ? '当前正在重跑步骤 1，后端会先删旧分镜再重建新分镜，请稍等这一轮回流。' : '当前范围还没有分镜记录。'}
                            </div>
                          ) : displayStoryboards.map((storyboard) => (
                            <button
                              key={storyboard.id}
                              type="button"
                              onClick={() => {
                                setFocusedStoryboardId((prev) => (prev === storyboard.id ? null : storyboard.id))
                                setFocusedAssetId(null)
                              }}
                              className={`w-full rounded-lg border p-3 text-left space-y-3 transition ${focusedStoryboardId === storyboard.id || focusedStoryboardIds.has(storyboard.id) ? 'border-violet-400/40 bg-violet-500/10 ring-1 ring-violet-400/30' : 'border-white/10 bg-slate-950/40 hover:bg-white/5'}`}
                            >
                              <div className="flex items-start justify-between gap-3">
                                <div className="min-w-0 flex-1">
                                  <div className="text-sm text-slate-100">分镜 #{storyboard.sequence_number}</div>
                                  <div className="mt-1 text-[11px] text-slate-400">episode {storyboard.episode_id || '-'} · {(storyboardAssetDetailMap.get(storyboard.id) || []).length} 个参考图槽位</div>
                                </div>
                                <div className="flex flex-wrap items-center justify-end gap-2">
                                  <div className={`rounded-full border px-2 py-0.5 text-[11px] ${String(storyboard.image_url || '').trim() ? 'border-emerald-400/30 bg-emerald-500/10 text-emerald-200' : 'border-amber-400/30 bg-amber-500/10 text-amber-100'}`}>
                                    {String(storyboard.image_url || '').trim() ? '分镜图已就绪' : '待生成'}
                                  </div>
                                  <Button
                                    type="button"
                                    size="sm"
                                    variant="outline"
                                    className="h-7 rounded-lg border-violet-400/30 bg-violet-500/10 px-2 text-[11px] text-violet-100 hover:bg-violet-500/20"
                                    disabled={pipelineBusy || !step2Enabled || scopeAssets.length === 0 || step2Running}
                                    onClick={(event) => {
                                      event.stopPropagation()
                                      void regenerateSingleStoryboardImage(storyboard.id, storyboard.sequence_number)
                                    }}
                                  >
                                    重新生成
                                  </Button>
                                </div>
                              </div>

                              <div className="grid gap-3 md:grid-cols-[112px_minmax(0,1fr)]">
                                {String(storyboard.image_url || '').trim() ? (
                                  <div className="overflow-hidden rounded-lg border border-white/10 bg-black/30">
                                    {/* eslint-disable-next-line @next/next/no-img-element */}
                                    <img src={storyboard.image_url} alt={`storyboard-${storyboard.id}`} className="h-28 w-full object-cover" />
                                  </div>
                                ) : (
                                  <div className="flex h-28 items-center justify-center rounded-lg border border-dashed border-white/10 bg-black/10 text-[11px] text-slate-500">
                                    暂无分镜图
                                  </div>
                                )}

                                <div className="space-y-3">
                                  <div className="flex flex-wrap gap-1">
                                    {(storyboardAssetDetailMap.get(storyboard.id) || []).length > 0 ? (storyboardAssetDetailMap.get(storyboard.id) || []).map((asset) => (
                                      <span key={`storyboard-${storyboard.id}-asset-${asset.id}`} className={`rounded-full border px-2 py-0.5 text-[10px] ${focusedAssetId === asset.id || focusedAssetIds.has(asset.id) ? 'border-violet-300/40 bg-violet-400/20 text-violet-50' : 'border-violet-400/20 bg-violet-500/10 text-violet-100'}`}>
                                        #{asset.id} {asset.name || '未命名素材'}
                                      </span>
                                    )) : (
                                      <span className="rounded-full border border-white/10 bg-white/5 px-2 py-0.5 text-[10px] text-slate-400">无参考图槽位</span>
                                    )}
                                  </div>

                                  <div className="rounded-lg border border-white/10 bg-white/5 p-3 text-[11px] text-slate-300 space-y-2">
                                    <div>
                                      <div className="mb-1 text-[10px] uppercase tracking-wide text-slate-500">场景</div>
                                      <div className="line-clamp-3 whitespace-pre-wrap break-words text-slate-100">{storyboard.scene_description || '暂无场景描述'}</div>
                                    </div>
                                    <div>
                                      <div className="mb-1 text-[10px] uppercase tracking-wide text-slate-500">台词</div>
                                      <div className="line-clamp-2 whitespace-pre-wrap break-words text-slate-100">{storyboard.dialogue || '暂无台词'}</div>
                                    </div>
                                  </div>
                                </div>
                              </div>
                            </button>
                          ))}
                        </div>
                      </div>
                    </div>
                  </div>
                )}

                {activePipelineStep === 'step3' && (
                  <div className="rounded-xl border border-emerald-500/20 bg-emerald-500/10 p-4 space-y-4">
                    <div>
                      <div className="text-sm font-medium text-emerald-100">步骤 3：基于当前范围分镜图提交视频</div>
                      <div className="mt-1 text-xs text-emerald-100/80">这里只要求当前范围至少有首张分镜图；提交后会串行生成，后续 clip 使用上一段视频检测到的尾帧作为下一段首帧。</div>
                      {step3Running && (
                        <div className="mt-2 inline-flex rounded-full border border-emerald-400/30 bg-emerald-500/10 px-3 py-1 text-[11px] text-emerald-100">
                          当前进行中：正在提交视频任务，请勿重复点击
                        </div>
                      )}
                    </div>

                    <div className="grid gap-3 md:grid-cols-2">
                      <div className="space-y-2">
                        <Label className="text-emerald-100">视频生成模型（步骤 3 实际使用）</Label>
                        <select
                          value={effectiveSelectedVideoModel}
                          onChange={(e) => setSelectedGenerationVideoModel(e.target.value)}
                          disabled={step3Running}
                          className="flex h-10 w-full rounded-xl border border-emerald-200/30 bg-white px-3 py-2 text-sm text-surface-900 disabled:cursor-not-allowed disabled:opacity-60"
                        >
                          <option value="">请选择模型</option>
                          {videoModelsForStep3.map((item) => {
                            const params = item.params || []
                            const keys = new Set(params.map((param) => param.key))
                            const missing = ['aspect_ratio', 'resolution', 'duration'].filter((key) => !keys.has(key))
                            const suffix = item.available
                              ? missing.length > 0 ? `（缺少 ${missing.join(' / ')}，参数需手动补齐）` : ''
                              : '（当前运行态不可用）'
                            return <option key={item.key} value={item.key} disabled={!item.available}>{item.key}{suffix}</option>
                          })}
                        </select>
                        <div className="text-[11px] text-emerald-100/75">这里选择的是步骤 3 真正提交给 video-service 的 `model_name`；如果当前模型容易被拒，可以先切到别的模型，再按这个模型支持的参数重选后提交。</div>
                      </div>

                      <div className="space-y-2 rounded-lg border border-white/10 bg-black/20 p-3 text-xs text-emerald-100/85">
                        <div>当前范围：{scopeLabel}</div>
                        <div>目标模型：{effectiveSelectedVideoModel || '未选择'}</div>
                        <div>视频配置：{selectedStep3AspectRatio || '-'} / {selectedStep3Resolution || '-'} / {selectedStep3Duration || '-'} 秒</div>
                        <div>当前可提交分镜图：{completedStoryboardImages} / {displayStoryboards.length}</div>
                      </div>
                    </div>

                    <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
                      <div className="space-y-2">
                        <Label className="text-emerald-100">步骤 3 画面比例</Label>
                        <select
                          value={selectedStep3AspectRatio}
                          onChange={(e) => setSelectedStep3AspectRatio(e.target.value)}
                          disabled={step3AspectRatioOptions.length === 0 || step3Running}
                          className="flex h-10 w-full rounded-xl border border-emerald-200/30 bg-white px-3 py-2 text-sm text-surface-900 disabled:cursor-not-allowed disabled:opacity-60"
                        >
                          <option value="">请选择比例</option>
                          {step3AspectRatioOptions.map((item) => (
                            <option key={`step3-aspect-${item.value}`} value={item.value}>{item.label || item.value}</option>
                          ))}
                        </select>
                      </div>

                      <div className="space-y-2">
                        <Label className="text-emerald-100">步骤 3 分辨率</Label>
                        <select
                          value={selectedStep3Resolution}
                          onChange={(e) => setSelectedStep3Resolution(e.target.value)}
                          disabled={step3ResolutionOptions.length === 0 || step3Running}
                          className="flex h-10 w-full rounded-xl border border-emerald-200/30 bg-white px-3 py-2 text-sm text-surface-900 disabled:cursor-not-allowed disabled:opacity-60"
                        >
                          <option value="">请选择分辨率</option>
                          {step3ResolutionOptions.map((item) => (
                            <option key={`step3-resolution-${item.value}`} value={item.value}>{item.label || item.value}</option>
                          ))}
                        </select>
                      </div>

                      <div className="space-y-2">
                        <Label className="text-emerald-100">步骤 3 单分镜时长</Label>
                        <select
                          value={selectedStep3Duration}
                          onChange={(e) => setSelectedStep3Duration(e.target.value)}
                          disabled={step3DurationOptions.length === 0 || step3Running}
                          className="flex h-10 w-full rounded-xl border border-emerald-200/30 bg-white px-3 py-2 text-sm text-surface-900 disabled:cursor-not-allowed disabled:opacity-60"
                        >
                          <option value="">请选择时长</option>
                          {step3DurationOptions.map((item) => (
                            <option key={`step3-duration-${item.value}`} value={item.value}>{item.label || item.value}</option>
                          ))}
                        </select>
                      </div>

                      <div className="space-y-2">
                        <Label className="text-emerald-100">步骤 3 语音输出</Label>
                        <select
                          value={selectedModelMeta?.native_audio ? (selectedStep3GenerateAudio ? 'enabled' : 'disabled') : 'unsupported'}
                          onChange={(e) => setSelectedStep3GenerateAudio(e.target.value === 'enabled')}
                          disabled={!selectedModelMeta?.native_audio || step3Running}
                          className="flex h-10 w-full rounded-xl border border-emerald-200/30 bg-white px-3 py-2 text-sm text-surface-900 disabled:cursor-not-allowed disabled:opacity-60"
                        >
                          {selectedModelMeta?.native_audio ? (
                            <>
                              <option value="disabled">不生成语音</option>
                              <option value="enabled">生成语音（原生）</option>
                            </>
                          ) : (
                            <option value="unsupported">当前模型不支持语音</option>
                          )}
                        </select>
                      </div>
                    </div>

                    <div className="rounded-lg border border-emerald-500/20 bg-emerald-500/10 p-3 space-y-2 text-xs text-emerald-50">
                      <div className="font-medium text-emerald-100">步骤 3 当前生成模型能力声明：{effectiveSelectedVideoModel || '未选择'}</div>
                      {selectedModelMeta ? (
                        <>
                          <div>available：{selectedModelMeta.available ? 'true' : 'false'}；native_audio：{selectedModelMeta.native_audio ? 'true' : 'false'}</div>
                          {selectedModelParams.length > 0 ? (
                            <div className="space-y-2">
                              {selectedModelParams.map((param) => (
                                <div key={`step3-${param.key}`} className="rounded border border-emerald-400/15 bg-black/10 p-2">
                                  <div><span className="text-emerald-100">{param.label || param.key}</span> <span className="text-emerald-200/70">({param.key})</span></div>
                                  <div className="text-emerald-100/80">默认值：{param.default || '未声明'}</div>
                                  <div className="text-emerald-100/75">可选值：{formatModelParamValues(param)}</div>
                                </div>
                              ))}
                            </div>
                          ) : (
                            <div className="text-emerald-100/75">当前模型没有返回 params 声明。</div>
                          )}
                        </>
                      ) : (
                        <div className="text-emerald-100/75">当前未匹配到该模型的运行态能力声明；建议先切到一个能正常返回能力声明的模型，再提交生成，避免 provider 拒绝时难以判断原因。</div>
                      )}
                    </div>

                    <Button
                      disabled={pipelineBusy || !step3Enabled || !splitConfigReady || !serialVideoSeedReady}
                      onClick={() => void startScopedVideoGeneration()}
                    >
                      {generationAction === 'video-start' ? '正在提交视频任务…' : selectedEpisodeNumber ? '开始生成当前分集视频' : '开始生成当前范围视频'}
                    </Button>

                    <div className="rounded-lg border border-white/10 bg-black/20 p-3 text-[11px] text-emerald-100/80">
                      {step3Hint}
                    </div>

                    <div className="text-[11px] text-emerald-100/75">
                      真正提交给视频服务的是当前范围内从首张可用 `image_url` 开始的完整分镜序列；只有第一段带首图，后续片段依赖上一段视频尾帧串行衔接，同时会带上这里选中的视频模型、分镜文案、台词、镜头运动、角色/素材引用等字段。
                    </div>
                  </div>
                )}
              </div>
            </CardContent>
          </Card>

        </>
      )}
    </div>
  )
}
