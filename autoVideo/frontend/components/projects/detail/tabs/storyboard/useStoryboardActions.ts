'use client'

import { useEffect, useState } from 'react'
import { useOneShotTriggerEffect } from '@/lib/projects/use-one-shot-trigger'
import { storyboardAPI, videoAPI, dubbingAPI, assetAPI, type DubbingTask } from '@/lib/api'
import { normalizeVideoStylePreset, VIDEO_STYLE_COMPACT_OPTIONS, VIDEO_GENERATION_PRESETS } from '@/lib/video-style-config'
import { FALLBACK_VOICE_OPTIONS } from '@/lib/projects/constants'
import { formatStoryboardSpeechForVideo, formatStoryboardDubbingText, resolveStoryboardSpeechLimit, joinStoryboardDialogue, countStoryboardDialogueRunes } from '@/lib/projects/storyboard-dubbing'
import { canTriggerStoryboardImage, triggerStoryboardImageGeneration } from '@/lib/projects/storyboard-image'
import { resolveStoryboardClipDurationSec, resolveStoryboardClipDurations } from '@/lib/projects/video-clip-duration'
import { commentaryProductionModeValue } from '@/lib/projects/commentary-project'
import { getApiErrorMessage } from '@/lib/projects/get-api-error-message'
import { buildVideoSceneDescription } from '@/lib/projects/storyboard-video-prompt'
import { filterReadyVideoStoryboards, isStoryboardSerialCandidate } from '@/lib/projects/storyboard-video-filter'
import {
  persistStoryboardRuntimeConfig,
  storyboardMotionModePatch,
  storyboardStylePresetPatch,
} from '@/lib/projects/persist-storyboard-runtime-config'
import { buildProjectVideoRenderConfig } from '@/lib/projects/storyboard-runtime-config'
import type { ImageModelOption } from '@/lib/model-display'
import type { VideoModelCapability } from '@/lib/video-style-config'
import type { Episode, Project, Storyboard } from '@/types'
import useSWR from 'swr'
import {
  VIDEO_STYLE_LABELS,
  VIDEO_MOTION_LABELS,
  VIDEO_FRAME_SIZE_OPTIONS,
  VIDEO_SUBJECT_SIZE_OPTIONS,
  VIDEO_CLARITY_OPTIONS,
  type VideoFrameSizeKey,
  type VideoSubjectSizeKey,
  type VideoClarityKey,
  type VideoMotionKey,
} from './episode-video-constants'

type ToastFn = (opts: { title: string; description?: string; variant?: 'default' | 'destructive' | 'success' }) => void

export function useStoryboardActions({
  projectId,
  project,
  isSerial,
  isCommentaryProject,
  labels,
  episodeFilter,
  episodes,
  SB_MODEL_OPTIONS,
  sbProjectImageModelKey,
  storyboardDefaultImageModelLabel,
  storyboardAssetsReady,
  storyboardAssetsBlockingReason,
  vtVideoModelOptions,
  storyboardTaskMap,
  mutateSb,
  mutateStats,
  mutateStoryboardTasks,
  sbGenerateTrigger,
  sbRegenerateTrigger,
  sbPauseTrigger,
  sbResumeTrigger,
  sbAuditTrigger,
  sbRepairMetadataTrigger,
  onSbGenerateTriggerConsumed,
  onSbRegenerateTriggerConsumed,
  onSbPauseTriggerConsumed,
  onSbResumeTriggerConsumed,
  onSbAuditTriggerConsumed,
  onSbRepairMetadataTriggerConsumed,
  toast,
}: {
  projectId: number
  project: Project
  isSerial: boolean
  isCommentaryProject: boolean
  labels: {
    storyboardItemLabel: string
    extractStoryboardLabel: string
    storyboardGenerateLabel: string
    storyboardImageLabel: string
    storyboardVideoLabel: string
  }
  episodeFilter: string
  episodes: Episode[]
  SB_MODEL_OPTIONS: ImageModelOption[]
  sbProjectImageModelKey: string
  storyboardDefaultImageModelLabel: string
  storyboardAssetsReady: boolean
  storyboardAssetsBlockingReason: string
  vtVideoModelOptions: VideoModelCapability[]
  storyboardTaskMap: Map<number, DubbingTask>
  mutateSb: () => void
  mutateStats: () => void
  mutateStoryboardTasks: () => void
  sbGenerateTrigger?: number
  sbRegenerateTrigger?: number
  sbPauseTrigger?: number
  sbResumeTrigger?: number
  sbAuditTrigger?: number
  sbRepairMetadataTrigger?: number
  onSbGenerateTriggerConsumed?: () => void
  onSbRegenerateTriggerConsumed?: () => void
  onSbPauseTriggerConsumed?: () => void
  onSbResumeTriggerConsumed?: () => void
  onSbAuditTriggerConsumed?: () => void
  onSbRepairMetadataTriggerConsumed?: () => void
  toast: ToastFn
}) {
  const {
    storyboardItemLabel,
    extractStoryboardLabel,
    storyboardGenerateLabel,
    storyboardImageLabel,
    storyboardVideoLabel,
  } = labels

  const storyboardGenerateBlockedText = storyboardAssetsBlockingReason || `请先完成资源图生成后再开始${storyboardGenerateLabel}`
  const storyboardResumeBlockedText = storyboardAssetsBlockingReason || `请先完成资源图生成后再继续${storyboardGenerateLabel}`

  const speakableDialogue = (sb: Storyboard) =>
    formatStoryboardSpeechForVideo(sb, { isCommentary: isCommentaryProject, project })

  const [pausingGeneration, setPausingGeneration] = useState(false)
  const [resumingGeneration, setResumingGeneration] = useState(false)
  const [isAuditingContinuity, setIsAuditingContinuity] = useState(false)
  const [isRepairingMetadata, setIsRepairingMetadata] = useState(false)
  const [showBatchStoryboardDialog, setShowBatchStoryboardDialog] = useState(false)
  const [batchStoryboardAction, setBatchStoryboardAction] = useState<{ kind: 'generate' | 'force' | 'retryFailed'; episodeId?: number }>({ kind: 'generate' })
  const [batchStoryboardModels, setBatchStoryboardModels] = useState<string[]>([])
  const [batchStoryboardRunning, setBatchStoryboardRunning] = useState(false)

  const [imageModelAvailability, setImageModelAvailability] = useState<Record<string, boolean>>({})
  const [videoModelAvailability, setVideoModelAvailability] = useState<Record<string, boolean>>({})
  const [videoModelParams, setVideoModelParams] = useState<Record<string, { key: string; label: string; default: string; values: { value: string; label: string }[] }[]>>({})
  const [videoParamSelections, setVideoParamSelections] = useState<Record<string, Record<string, string>>>({})

  const getModelParam = (modelKey: string, paramKey: string): string => {
    const sel = videoParamSelections[modelKey] ?? {}
    if (sel[paramKey]) return sel[paramKey]
    const param = (videoModelParams[modelKey] ?? []).find((p) => p.key === paramKey)
    return param?.default ?? ''
  }
  const setModelParam = (modelKey: string, paramKey: string, value: string) => {
    setVideoParamSelections((prev) => ({
      ...prev,
      [modelKey]: { ...(prev[modelKey] ?? {}), [paramKey]: value },
    }))
  }

  useEffect(() => {
    assetAPI.modelStatus().then((res) => {
      const map: Record<string, boolean> = {}
      const models: { key: string; available: boolean }[] = (res as { models?: { key: string; available: boolean }[]; data?: { models?: { key: string; available: boolean }[] } })?.models ?? (res as { data?: { models?: { key: string; available: boolean }[] } })?.data?.models ?? []
      models.forEach((m) => { map[m.key] = m.available })
      setImageModelAvailability(map)
    }).catch((e) => { console.warn('[assetAPI.modelStatus]', e) })
    videoAPI.modelStatus().then((res) => {
      const avail: Record<string, boolean> = {}
      const params: Record<string, { key: string; label: string; default: string; values: { value: string; label: string }[] }[]> = {}
      const vmodels = (res as { models?: { key: string; available: boolean; params?: { key: string; label: string; default: string; values: { value: string; label: string }[] }[] }[]; data?: { models?: { key: string; available: boolean; params?: { key: string; label: string; default: string; values: { value: string; label: string }[] }[] }[] } })?.models ?? (res as { data?: { models?: { key: string; available: boolean; params?: { key: string; label: string; default: string; values: { value: string; label: string }[] }[] }[] } })?.data?.models ?? []
      vmodels.forEach((m) => {
        avail[m.key] = m.available
        if (m.params && m.params.length > 0) params[m.key] = m.params
      })
      setVideoModelAvailability(avail)
      setVideoModelParams(params)
    }).catch((e) => { console.warn('[videoAPI.modelStatus]', e) })
  }, [])

  const [generatingVideoEps, setGeneratingVideoEps] = useState<Set<number>>(new Set())
  const [videoDialogEpisodeId, setVideoDialogEpisodeId] = useState<number | null>(null)
  const [generatingAllVideos, setGeneratingAllVideos] = useState(false)
  const [selectedEpisodeTransition, setSelectedEpisodeTransition] = useState('dissolve')
  const [selectedEpisodeTransitionDuration, setSelectedEpisodeTransitionDuration] = useState('0.5')
  const [selectedEpisodeVideoModel, setSelectedEpisodeVideoModel] = useState<string>('wan')
  const [selectedEpisodeVideoStyle, setSelectedEpisodeVideoStyle] = useState('anime-2d')
  const [selectedEpisodeVideoMotionMode, setSelectedEpisodeVideoMotionMode] = useState<VideoMotionKey>('gentle')
  const [selectedEpisodeVideoFrameSize, setSelectedEpisodeVideoFrameSize] = useState<VideoFrameSizeKey>('landscape-16-9')
  const [selectedEpisodeVideoSubjectSize, setSelectedEpisodeVideoSubjectSize] = useState<VideoSubjectSizeKey>('medium-shot')
  const [selectedEpisodeVideoClarity, setSelectedEpisodeVideoClarity] = useState<VideoClarityKey>('high')

  const selectedEpisodeVideoStyleLabel = VIDEO_STYLE_LABELS[selectedEpisodeVideoStyle] ?? selectedEpisodeVideoStyle
  const selectedEpisodeVideoMotionLabel = VIDEO_MOTION_LABELS[selectedEpisodeVideoMotionMode] ?? selectedEpisodeVideoMotionMode
  const selectedEpisodeVideoModeLabel = project.video_mode === 'api_generation' ? 'API生成' : '逐帧动画'
  const selectedVideoDialogEpisode = videoDialogEpisodeId ? episodes.find((episode) => episode.id === videoDialogEpisodeId) ?? null : null
  const selectedStoryboardBatchEpisode = batchStoryboardAction.episodeId
    ? episodes.find((episode) => episode.id === batchStoryboardAction.episodeId) ?? null
    : null

  const episodeVideoModelStorageKey = `project-video-model-selection:${projectId}`
  const episodeVideoStyleStorageKey = `project-video-style-selection:${projectId}`
  const episodeVideoMotionStorageKey = `project-video-motion-selection:${projectId}`
  const projectConfiguredVideoStyle = normalizeVideoStylePreset(
    typeof project.storyboard_config?.style_preset === 'string' ? project.storyboard_config.style_preset : ''
  )
  const projectConfiguredVideoMotion =
    typeof project.storyboard_config?.motion_mode === 'string' ? project.storyboard_config.motion_mode : ''

  const syncEpisodeVideoSelections = () => {
    if (typeof window === 'undefined') return
    try {
      const savedModel = window.localStorage.getItem(episodeVideoModelStorageKey)
      if (savedModel && vtVideoModelOptions.some((item) => item.key === savedModel)) {
        setSelectedEpisodeVideoModel(savedModel)
      }
      const savedStyle = normalizeVideoStylePreset(window.localStorage.getItem(episodeVideoStyleStorageKey) ?? '')
      if (VIDEO_STYLE_COMPACT_OPTIONS.some((style) => style.key === projectConfiguredVideoStyle)) {
        setSelectedEpisodeVideoStyle(projectConfiguredVideoStyle)
      } else if (VIDEO_STYLE_COMPACT_OPTIONS.some((style) => style.key === savedStyle)) {
        setSelectedEpisodeVideoStyle(savedStyle)
      }
      const savedMotion = window.localStorage.getItem(episodeVideoMotionStorageKey)
      if (projectConfiguredVideoMotion && projectConfiguredVideoMotion in VIDEO_MOTION_LABELS) {
        setSelectedEpisodeVideoMotionMode(projectConfiguredVideoMotion as VideoMotionKey)
      } else if (savedMotion && savedMotion in VIDEO_MOTION_LABELS) {
        setSelectedEpisodeVideoMotionMode(savedMotion as VideoMotionKey)
      }
    } catch {}
  }

  useEffect(() => {
    syncEpisodeVideoSelections()
  }, [projectConfiguredVideoMotion, projectConfiguredVideoStyle, projectId])

  useEffect(() => {
    if (typeof window === 'undefined') return
    try {
      window.localStorage.setItem(episodeVideoModelStorageKey, selectedEpisodeVideoModel)
      window.localStorage.setItem(episodeVideoStyleStorageKey, selectedEpisodeVideoStyle)
      window.localStorage.setItem(episodeVideoMotionStorageKey, selectedEpisodeVideoMotionMode)
    } catch {}
    if (selectedEpisodeVideoModel && project.storyboard_config?.video_model !== selectedEpisodeVideoModel) {
      persistStoryboardRuntimeConfig(projectId, project.storyboard_config, {
        video_model: selectedEpisodeVideoModel,
      }).catch(() => {})
    }
    const stylePatch = storyboardStylePresetPatch(project.storyboard_config, selectedEpisodeVideoStyle)
    if (stylePatch) {
      persistStoryboardRuntimeConfig(projectId, project.storyboard_config, stylePatch).catch(() => {})
    }
    const motionPatch = storyboardMotionModePatch(project.storyboard_config, selectedEpisodeVideoMotionMode)
    if (motionPatch) {
      persistStoryboardRuntimeConfig(projectId, project.storyboard_config, motionPatch).catch(() => {})
    }
  }, [
    episodeVideoModelStorageKey,
    episodeVideoMotionStorageKey,
    episodeVideoStyleStorageKey,
    project.storyboard_config,
    projectId,
    selectedEpisodeVideoModel,
    selectedEpisodeVideoMotionMode,
    selectedEpisodeVideoStyle,
  ])

  const applyEpisodeVideoPreset = (presetKey: string) => {
    const preset = VIDEO_GENERATION_PRESETS.find((item) => item.key === presetKey)
    if (!preset) return
    if (vtVideoModelOptions.some((item) => item.key === preset.model)) {
      setSelectedEpisodeVideoModel(preset.model)
    }
    setSelectedEpisodeVideoStyle(preset.style)
    setSelectedEpisodeVideoMotionMode(preset.motion)
  }

  const handleGenerateVideoByEpisode = async (
    episodeId: number,
    options?: {
      modelName?: string
      frameSize?: VideoFrameSizeKey
      subjectSize?: VideoSubjectSizeKey
      clarity?: VideoClarityKey
    }
  ) => {
    const completedSbs = ((await storyboardAPI.listAll(projectId, { episode_id: episodeId, status: 'completed' })) as { data?: Storyboard[] }).data ?? []
    const sortedSbs = filterReadyVideoStoryboards(completedSbs)
    const imageUrls = sortedSbs.map((sb) => sb.image_url)
    const sceneDescriptions = sortedSbs.map((sb) => buildVideoSceneDescription(sb))
    const modelName = options?.modelName || 'wan'
    const defaultClipDuration = (() => {
      const durSel = videoParamSelections[modelName]?.duration
      if (durSel) return parseFloat(durSel)
      return project.storyboard_config?.duration || 5
    })()
    const dialogues = sortedSbs.map((sb) => speakableDialogue(sb))
    const durations = resolveStoryboardClipDurations(sortedSbs, {
      modelKey: modelName,
      defaultDuration: defaultClipDuration,
      dubbingTasksByStoryboardId: storyboardTaskMap,
    })
    const cameraMovements = sortedSbs.map((sb) => sb.camera_movement || '')
    const moods = sortedSbs.map((sb) => sb.mood || '')
    const sceneCharacters = sortedSbs.map((sb) => sb.characters || [])
    const sceneAssetIds = sortedSbs.map((sb) => sb.asset_ids || [])
    const sceneDescription = sceneDescriptions.filter(Boolean).join(' ')
    const sceneGroupKeys = sortedSbs.map((sb) => sb.scene_group_key || '')
    const isSerialScene = sceneGroupKeys.some(Boolean) || project.project_type === 'video_serial'

    if (!imageUrls.some(Boolean)) {
      toast({ title: isSerialScene ? '此集暂无可用首帧图片' : '此集暂无已完成的分镜图片', variant: 'destructive' })
      return
    }

    setGeneratingVideoEps(prev => new Set(prev).add(episodeId))
    try {
      await videoAPI.generate(projectId, {
        episode_id: episodeId,
        image_urls: imageUrls,
        scene_descriptions: sceneDescriptions,
        dialogues: dialogues.some(Boolean) ? dialogues : undefined,
        durations: durations.some(Boolean) ? durations : undefined,
        camera_movements: cameraMovements.some(Boolean) ? cameraMovements : undefined,
        moods: moods.some(Boolean) ? moods : undefined,
        scene_characters: sceneCharacters.some((arr) => arr.length > 0) ? sceneCharacters : undefined,
        scene_asset_ids: sceneAssetIds.some((arr) => arr.length > 0) ? sceneAssetIds : undefined,
        model_name: modelName,
        style_preset: selectedEpisodeVideoStyle,
        motion_mode: selectedEpisodeVideoMotionMode,
        video_mode: project.video_mode,
        scene_description: sceneDescription || undefined,
        clip_duration_sec: defaultClipDuration,
        serial_scene: isSerialScene || undefined,
        scene_group_keys: isSerialScene && sceneGroupKeys.some(Boolean) ? sceneGroupKeys : undefined,
        render_config: buildProjectVideoRenderConfig(project, {
          frame_size: options?.frameSize || selectedEpisodeVideoFrameSize,
          subject_size: options?.subjectSize || selectedEpisodeVideoSubjectSize,
          clarity: options?.clarity || selectedEpisodeVideoClarity,
          transition: selectedEpisodeTransition === 'none' ? undefined : selectedEpisodeTransition,
          transition_duration: selectedEpisodeTransition !== 'none' ? parseFloat(selectedEpisodeTransitionDuration) : undefined,
          ...(videoParamSelections[modelName] ?? {}),
        }),
      })
      const ep = episodes.find(e => e.id === episodeId)
      const label = vtVideoModelOptions.find(m => m.key === modelName)?.label || modelName
      const frameLabel = VIDEO_FRAME_SIZE_OPTIONS.find((item) => item.key === (options?.frameSize || selectedEpisodeVideoFrameSize))?.label ?? (options?.frameSize || selectedEpisodeVideoFrameSize)
      const sizeLabel = VIDEO_SUBJECT_SIZE_OPTIONS.find((item) => item.key === (options?.subjectSize || selectedEpisodeVideoSubjectSize))?.label ?? (options?.subjectSize || selectedEpisodeVideoSubjectSize)
      const clarityLabel = VIDEO_CLARITY_OPTIONS.find((item) => item.key === (options?.clarity || selectedEpisodeVideoClarity))?.label ?? (options?.clarity || selectedEpisodeVideoClarity)
      toast({
        title: `第 ${ep?.episode_number ?? '?'} 集${storyboardVideoLabel}生成已启动（${label}）`,
        description: `${selectedEpisodeVideoStyleLabel} / ${selectedEpisodeVideoMotionLabel} / ${frameLabel} / ${sizeLabel} / ${clarityLabel} · ${isSerialScene ? `${sceneGroupKeys.filter(Boolean).length} 个场景组` : `${imageUrls.filter(Boolean).length} 张图`}`,
        variant: 'success',
      })
      return true
    } catch {
      toast({ title: '视频生成失败', variant: 'destructive' })
      return false
    } finally {
      setGeneratingVideoEps(prev => { const s = new Set(prev); s.delete(episodeId); return s })
    }
  }

  const openEpisodeVideoDialog = (epId: number) => {
    syncEpisodeVideoSelections()
    setVideoDialogEpisodeId(epId)
  }

  const handleConfirmEpisodeVideoGeneration = async () => {
    if (!videoDialogEpisodeId) return
    const ok = await handleGenerateVideoByEpisode(videoDialogEpisodeId, {
      modelName: selectedEpisodeVideoModel,
      frameSize: selectedEpisodeVideoFrameSize,
      subjectSize: selectedEpisodeVideoSubjectSize,
      clarity: selectedEpisodeVideoClarity,
    })
    if (ok) setVideoDialogEpisodeId(null)
  }

  const handleGenerateAllEpisodeVideos = async (modelName: string) => {
    const completedSbs = ((await storyboardAPI.listAll(projectId, { status: 'completed' })) as { data?: Storyboard[] }).data ?? []
    const eligibleSbs = filterReadyVideoStoryboards(completedSbs)
    const hasSerialCandidate = eligibleSbs.some((sb) => isStoryboardSerialCandidate(sb)) || project.project_type === 'video_serial'
    if (eligibleSbs.length === 0 || !eligibleSbs.some((sb) => sb.image_url)) {
      toast({ title: hasSerialCandidate ? '暂无可用场景首帧，请先完成首帧准备' : '暂无已完成的分镜图片，请先生成分镜图片', variant: 'destructive' })
      return
    }
    const byEpisode = new Map<number, string[]>()
    const byEpisodeDesc = new Map<number, string[]>()
    const byEpisodeDialogue = new Map<number, string[]>()
    const byEpisodeDuration = new Map<number, number[]>()
    const byEpisodeCamera = new Map<number, string[]>()
    const byEpisodeMood = new Map<number, string[]>()
    const byEpisodeChars = new Map<number, string[][]>()
    const byEpisodeAssetIds = new Map<number, number[][]>()
    const byEpisodeSceneGroupKeys = new Map<number, string[]>()
    for (const sb of eligibleSbs) {
      const epId = sb.episode_id ?? 0
      if (epId === 0) continue
      if (!byEpisode.has(epId)) {
        byEpisode.set(epId, []); byEpisodeDesc.set(epId, []); byEpisodeDialogue.set(epId, [])
        byEpisodeDuration.set(epId, []); byEpisodeCamera.set(epId, []); byEpisodeMood.set(epId, [])
        byEpisodeChars.set(epId, [])
        byEpisodeAssetIds.set(epId, [])
        byEpisodeSceneGroupKeys.set(epId, [])
      }
      byEpisode.get(epId)!.push(sb.image_url)
      byEpisodeDesc.get(epId)!.push(buildVideoSceneDescription(sb))
      byEpisodeDialogue.get(epId)!.push(speakableDialogue(sb))
      byEpisodeDuration.get(epId)!.push(
        resolveStoryboardClipDurationSec(sb, {
          modelKey: selectedEpisodeVideoModel || 'wan',
          defaultDuration: project.storyboard_config?.duration || 5,
          dubbingTask: storyboardTaskMap.get(sb.id),
        }),
      )
      byEpisodeCamera.get(epId)!.push(sb.camera_movement || '')
      byEpisodeMood.get(epId)!.push(sb.mood || '')
      byEpisodeChars.get(epId)!.push(sb.characters || [])
      byEpisodeAssetIds.get(epId)!.push(sb.asset_ids || [])
      byEpisodeSceneGroupKeys.get(epId)!.push(sb.scene_group_key || '')
    }
    if (byEpisode.size === 0) {
      toast({ title: `没有分配到集数的已完成${storyboardItemLabel}`, variant: 'destructive' })
      return
    }
    setGeneratingAllVideos(true)
    try {
      const isSerialScene = eligibleSbs.some((sb) => sb.scene_group_key) || project.project_type === 'video_serial'
      const episodeBatch = Array.from(byEpisode.entries()).map(([epId, urls]) => {
        const dlgs = byEpisodeDialogue.get(epId) ?? []
        const durs = byEpisodeDuration.get(epId) ?? []
        const cams = byEpisodeCamera.get(epId) ?? []
        const mds = byEpisodeMood.get(epId) ?? []
        const chars = byEpisodeChars.get(epId) ?? []
        const assetIds = byEpisodeAssetIds.get(epId) ?? []
        const descs = byEpisodeDesc.get(epId) ?? []
        const sceneGroupKeys = byEpisodeSceneGroupKeys.get(epId) ?? []
        return {
          episode_id: epId,
          image_urls: urls,
          scene_descriptions: descs,
          dialogues: dlgs.some(Boolean) ? dlgs : undefined,
          durations: durs.some(Boolean) ? durs : undefined,
          camera_movements: cams.some(Boolean) ? cams : undefined,
          moods: mds.some(Boolean) ? mds : undefined,
          scene_characters: chars.some((arr) => arr.length > 0) ? chars : undefined,
          scene_asset_ids: assetIds.some((arr) => arr.length > 0) ? assetIds : undefined,
          scene_description: descs.filter(Boolean).join(' ') || undefined,
          scene_group_keys: isSerialScene && sceneGroupKeys.some(Boolean) ? sceneGroupKeys : undefined,
        }
      })
      await videoAPI.generateBatch(projectId, { episodes: episodeBatch, model_name: modelName, serial_scene: isSerialScene || undefined })
      toast({ title: `已启动 ${episodeBatch.length} 集${storyboardVideoLabel}生成（共 ${isSerialScene ? `${eligibleSbs.filter((sb) => sb.scene_group_key).length} 个场景链` : `${eligibleSbs.filter((sb) => sb.image_url).length} 张图`})`, variant: 'success' })
    } catch {
      toast({ title: `批量${storyboardVideoLabel}生成失败`, variant: 'destructive' })
    } finally {
      setGeneratingAllVideos(false)
    }
  }

  const handleGenerateOne = async (sb: Storyboard, modelName?: string) => {
    if (!storyboardAssetsReady) {
      toast({ title: storyboardGenerateBlockedText, variant: 'destructive' })
      return
    }
    if (!canTriggerStoryboardImage(sb)) {
      toast({ title: '当前分镜正在生成中', variant: 'destructive' })
      return
    }
    const effectiveModel = modelName || sbProjectImageModelKey || undefined
    try {
      await triggerStoryboardImageGeneration(projectId, sb, effectiveModel)
      const label = effectiveModel
        ? SB_MODEL_OPTIONS.find((m) => m.key === effectiveModel)?.label || effectiveModel
        : (SB_MODEL_OPTIONS.find((m) => m.key === sbProjectImageModelKey)?.label || '项目默认模型')
      toast({
        title: sb.status === 'failed' ? `使用 ${label} 重试已启动` : `${storyboardGenerateLabel}已启动`,
        description: label ? `模型：${label}` : undefined,
        variant: 'success',
      })
      mutateSb()
      mutateStats()
    } catch (e: unknown) {
      const message = getApiErrorMessage(e)
      toast({ title: '生成失败', description: message || undefined, variant: 'destructive' })
    }
  }

  // handleGenerateModels —— 单条分镜「确认生成」：支持多选模型并行出多版本。
  // 选 0/1 个模型时走原有 retry/generate 路径；选多个时调用单条 generate 的 model_names 扇出。
  const handleGenerateModels = async (sb: Storyboard, modelKeys: string[]) => {
    if (!storyboardAssetsReady) {
      toast({ title: storyboardGenerateBlockedText, variant: 'destructive' })
      return
    }
    if (!canTriggerStoryboardImage(sb)) {
      toast({ title: '当前分镜正在生成中', variant: 'destructive' })
      return
    }
    const keys = (modelKeys || []).filter(Boolean)
    try {
      if (keys.length <= 1) {
        await triggerStoryboardImageGeneration(projectId, sb, keys[0] || sbProjectImageModelKey || undefined)
      } else {
        await storyboardAPI.generate(projectId, sb.id, undefined, { modelNames: keys })
      }
      const labels = keys.map((k) => SB_MODEL_OPTIONS.find((m) => m.key === k)?.label || k)
      toast({
        title: sb.status === 'failed' ? '重新生成已启动' : `${storyboardGenerateLabel}已启动`,
        description: sb.prompt_locked
          ? (keys.length > 1
            ? `高级模式 · 并行 ${keys.length} 个模型：${labels.join('、')}`
            : `高级模式 · ${labels[0] ? `模型：${labels[0]}` : `使用项目默认模型：${storyboardDefaultImageModelLabel}`}`)
          : (keys.length > 1
            ? `并行 ${keys.length} 个模型：${labels.join('、')}`
            : (labels[0] ? `模型：${labels[0]}` : `使用项目默认模型：${storyboardDefaultImageModelLabel}`)),
        variant: 'success',
      })
      mutateSb()
      mutateStats()
    } catch (e: unknown) {
      const message = getApiErrorMessage(e)
      toast({ title: '生成失败', description: message || undefined, variant: 'destructive' })
    }
  }

  const runStoryboardBatchJobs = async (jobs: Array<() => Promise<unknown>>, concurrency = 4) => {
    let cursor = 0
    let success = 0
    let failed = 0
    const worker = async () => {
      while (cursor < jobs.length) {
        const current = jobs[cursor]
        cursor += 1
        try {
          await current()
          success += 1
        } catch {
          failed += 1
        }
      }
    }
    await Promise.all(Array.from({ length: Math.min(concurrency, jobs.length) }, () => worker()))
    return { success, failed }
  }

  const getMultiCandidateStoryboardTargets = async (
    kind: 'generate' | 'force' | 'retryFailed',
    epId?: number,
  ) => {
    const allStoryboards = ((await storyboardAPI.listAll(projectId, {
      ...(epId !== undefined ? { episode_id: epId } : {}),
    })) as { data?: Storyboard[] }).data ?? []
    return allStoryboards.filter((sb) => {
      if (isSerial && sb.scene_group_key && !sb.is_scene_first_clip) return false
      if (kind === 'force') return true
      if (kind === 'retryFailed') return sb.status === 'failed'
      return sb.status === 'pending' || sb.status === 'failed'
    })
  }

  const resolveStoryboardModelSelection = (modelKeys?: string[]) => {
    const selectedModels = SB_MODEL_OPTIONS.filter((model) => modelKeys?.includes(model.key))
    const selectedModelKeys = selectedModels.map((model) => model.key)
    const effectiveModelName = selectedModels[0]?.key || sbProjectImageModelKey || undefined
    const description = selectedModels.length === 0
      ? (sbProjectImageModelKey ? `使用项目默认模型：${storyboardDefaultImageModelLabel}` : undefined)
      : selectedModels.length === 1
        ? `使用模型：${selectedModels[0].label}`
        : `使用模型：${selectedModels.map((model) => model.label).join('、')}；每条${storyboardItemLabel}会生成多版候选供你选择`
    return { selectedModelKeys, effectiveModelName, description }
  }

  const handleGenerateSameStoryboardWithAllModels = async (
    kind: 'generate' | 'force' | 'retryFailed',
    modelKeys: string[],
  ) => {
    const selectedEpisodeId = episodeFilter !== 'all' ? Number(episodeFilter) : undefined
    if (kind === 'force' && !selectedEpisodeId) return false
    const selectedModels = SB_MODEL_OPTIONS.filter((model) => modelKeys.includes(model.key))
    const targets = await getMultiCandidateStoryboardTargets(kind, selectedEpisodeId)
    if (targets.length === 0) {
      toast({
        title: kind === 'retryFailed'
          ? (selectedEpisodeId ? `当前集没有失败${storyboardItemLabel}` : `当前没有失败${storyboardItemLabel}`)
          : kind === 'force'
            ? `没有可重新生成的${storyboardImageLabel}`
            : (selectedEpisodeId ? `当前集没有可生成的${storyboardImageLabel}` : `没有可生成的${storyboardImageLabel}`),
        variant: 'default',
      })
      return false
    }
    const jobs = targets.flatMap((sb) => selectedModels.map((model) => () => storyboardAPI.generate(projectId, sb.id, model.key)))
    const { success, failed } = await runStoryboardBatchJobs(jobs)
    const modelHint = `模型：${selectedModels.map((model) => model.label).join('、')}`
    const versionHint = `每条${storyboardItemLabel}都会为所选模型各生成一版，完成后可在列表中切换版本。`
    toast({
      title: success > 0 ? `已提交 ${success} 个${storyboardImageLabel}候选图任务` : (kind === 'retryFailed' ? '批量重试失败' : '批量生成失败'),
      description: success > 0
        ? `${modelHint}。${versionHint}${failed > 0 ? ` 另有 ${failed} 个任务提交失败。` : ''}`
        : undefined,
      variant: success > 0 ? (failed > 0 ? 'default' : 'success') : 'destructive',
    })
    mutateSb()
    mutateStats()
    return success > 0
  }

  const handleForceGenerateEpisode = async (modelKey?: string, modelKeys?: string[]) => {
    if (!storyboardAssetsReady) {
      toast({ title: storyboardGenerateBlockedText, variant: 'destructive' })
      return false
    }
    const selectedEpisodeId = episodeFilter !== 'all' ? Number(episodeFilter) : undefined
    if (!selectedEpisodeId) return false
    if (modelKeys === undefined && !window.confirm(`这只会重置本集已有的${storyboardImageLabel}生成结果并重新出图，不会重新拆分${storyboardItemLabel}结构。若要按新的文案/镜头重新拆分，请使用“${extractStoryboardLabel}”。确认继续？`)) return false
    try {
      const { effectiveModelName, selectedModelKeys, description } = resolveStoryboardModelSelection(modelKeys ?? (modelKey ? [modelKey] : undefined))
      if (selectedModelKeys.length > 1) {
        return await handleGenerateSameStoryboardWithAllModels('force', selectedModelKeys)
      }
      const res = await storyboardAPI.generateAll(
        projectId,
        selectedEpisodeId,
        effectiveModelName,
        true,
        selectedModelKeys.length > 1 ? { modelNames: selectedModelKeys } : undefined,
      ) as unknown as { data: { triggered: number } }
      const n = res?.data?.triggered ?? 0
      toast({
        title: n > 0 ? `当前集重新${storyboardGenerateLabel}已启动，共 ${n} 个` : `没有可重新生成的${storyboardImageLabel}`,
        description: n > 0 ? description : undefined,
        variant: n > 0 ? 'success' : 'default',
      })
      mutateStats()
      mutateSb()
      return true
    } catch {
      toast({ title: '重新生成失败', variant: 'destructive' })
      return false
    }
  }

  const handleGenerateAll = async (modelKey?: string, modelKeys?: string[]) => {
    if (!storyboardAssetsReady) {
      toast({ title: storyboardGenerateBlockedText, variant: 'destructive' })
      return false
    }
    try {
      const selectedEpisodeId = episodeFilter !== 'all' ? Number(episodeFilter) : undefined
      const { effectiveModelName, selectedModelKeys, description } = resolveStoryboardModelSelection(modelKeys ?? (modelKey ? [modelKey] : undefined))
      if (selectedModelKeys.length > 1) {
        return await handleGenerateSameStoryboardWithAllModels('generate', selectedModelKeys)
      }
      const res = await storyboardAPI.generateAll(
        projectId,
        selectedEpisodeId,
        effectiveModelName,
        false,
        selectedModelKeys.length > 1 ? { modelNames: selectedModelKeys } : undefined,
      ) as unknown as { data: { triggered: number } }
      const n = res?.data?.triggered ?? 0
      toast({
        title: n > 0
          ? (selectedEpisodeId ? `当前集${storyboardGenerateLabel}已启动，共 ${n} 个` : `批量${storyboardGenerateLabel}已启动，共 ${n} 个`)
          : (selectedEpisodeId ? `当前集没有可生成的${storyboardImageLabel}` : `没有可生成的${storyboardImageLabel}`),
        description: n > 0 ? description : undefined,
        variant: n > 0 ? 'success' : 'default',
      })
      mutateStats()
      mutateSb()
      return true
    } catch {
      toast({ title: '批量生成失败', variant: 'destructive' })
      return false
    }
  }

  const openBatchStoryboardDialog = (kind: 'generate' | 'force' | 'retryFailed') => {
    const defaultModels = sbProjectImageModelKey && SB_MODEL_OPTIONS.some((model) => model.key === sbProjectImageModelKey)
      ? [sbProjectImageModelKey]
      : []
    setBatchStoryboardAction({
      kind,
      episodeId: episodeFilter !== 'all' ? Number(episodeFilter) : undefined,
    })
    setBatchStoryboardModels(defaultModels)
    setShowBatchStoryboardDialog(true)
  }

  const handleRetryAllFailed = async (modelName?: string, modelNames?: string[]) => {
    if (!storyboardAssetsReady) {
      toast({ title: storyboardGenerateBlockedText, variant: 'destructive' })
      return false
    }
    try {
      const selectedEpisodeId = episodeFilter !== 'all' ? Number(episodeFilter) : undefined
      const { effectiveModelName, selectedModelKeys, description } = resolveStoryboardModelSelection(modelNames ?? (modelName ? [modelName] : undefined))
      if (selectedModelKeys.length > 1) {
        return await handleGenerateSameStoryboardWithAllModels('retryFailed', selectedModelKeys)
      }
      const res = await storyboardAPI.retryFailed(
        projectId,
        effectiveModelName,
        selectedEpisodeId,
        selectedModelKeys.length > 1 ? { modelNames: selectedModelKeys } : undefined,
      ) as unknown as { data: { retried: number } }
      const n = res?.data?.retried ?? 0
      toast({
        title: n > 0
          ? (selectedEpisodeId ? `当前集已启动 ${n} 个失败${storyboardItemLabel}重试` : `批量重试 ${n} 个失败${storyboardItemLabel}`)
          : (selectedEpisodeId ? `当前集没有失败${storyboardItemLabel}` : `当前没有失败${storyboardItemLabel}`),
        description: n > 0 ? description : undefined,
        variant: n > 0 ? 'success' : 'default',
      })
      mutateSb()
      mutateStats()
      return true
    } catch {
      toast({ title: '批量重试失败', variant: 'destructive' })
      return false
    }
  }

  const executeBatchStoryboardAction = async () => {
    setBatchStoryboardRunning(true)
    try {
      const selectedModelKeys = SB_MODEL_OPTIONS
        .filter((model) => batchStoryboardModels.includes(model.key))
        .map((model) => model.key)
      const ok = batchStoryboardAction.kind === 'force'
        ? await handleForceGenerateEpisode(selectedModelKeys[0], selectedModelKeys)
        : batchStoryboardAction.kind === 'retryFailed'
          ? await handleRetryAllFailed(selectedModelKeys[0], selectedModelKeys)
          : await handleGenerateAll(selectedModelKeys[0], selectedModelKeys)
      if (ok) setShowBatchStoryboardDialog(false)
    } finally {
      setBatchStoryboardRunning(false)
    }
  }

  const handlePauseGeneration = async () => {
    setPausingGeneration(true)
    try {
      const selectedEpisodeId = episodeFilter !== 'all' ? Number(episodeFilter) : undefined
      const res = await storyboardAPI.pauseGeneration(projectId, selectedEpisodeId) as unknown as { data?: { paused?: number } }
      const paused = res?.data?.paused ?? 0
      toast({
        title: selectedEpisodeId ? `已暂停当前集${storyboardGenerateLabel}（${paused} 项）` : `已暂停${storyboardGenerateLabel}（${paused} 项）`,
        variant: 'success',
      })
      mutateStats()
      mutateSb()
    } catch {
      toast({ title: `暂停${storyboardGenerateLabel}失败`, variant: 'destructive' })
    } finally {
      setPausingGeneration(false)
    }
  }

  const handleResumeGeneration = async () => {
    if (!storyboardAssetsReady) {
      toast({ title: storyboardResumeBlockedText, variant: 'destructive' })
      return
    }
    setResumingGeneration(true)
    try {
      const selectedEpisodeId = episodeFilter !== 'all' ? Number(episodeFilter) : undefined
      const res = await storyboardAPI.resumeGeneration(projectId, selectedEpisodeId) as unknown as { data?: { triggered?: number } }
      const triggered = res?.data?.triggered ?? 0
      toast({
        title: triggered > 0
          ? (selectedEpisodeId ? `已继续当前集${storyboardGenerateLabel}（${triggered} 项）` : `已继续${storyboardGenerateLabel}（${triggered} 项）`)
          : (selectedEpisodeId ? `当前集没有已暂停的${storyboardItemLabel}` : `当前没有已暂停的${storyboardItemLabel}`),
        variant: triggered > 0 ? 'success' : 'default',
      })
      mutateStats()
      mutateSb()
    } catch {
      toast({ title: `继续${storyboardGenerateLabel}失败`, variant: 'destructive' })
    } finally {
      setResumingGeneration(false)
    }
  }

  const handleRepairMetadata = async () => {
    const selectedEpisodeId = episodeFilter !== 'all' ? Number(episodeFilter) : undefined
    if (!selectedEpisodeId) {
      toast({ title: '请先选择具体集数', variant: 'destructive' })
      return
    }
    setIsRepairingMetadata(true)
    try {
      const res = await storyboardAPI.repairMetadata(projectId, selectedEpisodeId) as unknown as {
        data?: { repaired?: number; characters_filled?: number; prompts_cleared?: number }
      }
      const repaired = res?.data?.repaired ?? 0
      toast({
        title: repaired > 0 ? `已修复 ${repaired} 条${storyboardItemLabel}元数据` : `本集${storyboardItemLabel}元数据无需修复`,
        description: repaired > 0
          ? `人物 ${res?.data?.characters_filled ?? 0} · 清理 prompt ${res?.data?.prompts_cleared ?? 0}`
          : undefined,
        variant: repaired > 0 ? 'success' : 'default',
      })
      mutateSb()
      mutateStats()
    } catch {
      toast({ title: '元数据修复失败', variant: 'destructive' })
    } finally {
      setIsRepairingMetadata(false)
    }
  }

  const handleAuditContinuity = async () => {
    setIsAuditingContinuity(true)
    try {
      const selectedEpisodeId = episodeFilter !== 'all' ? Number(episodeFilter) : undefined
      const res = await storyboardAPI.auditContinuity(projectId, selectedEpisodeId) as unknown as { data?: { total_patched?: number } }
      const n = res?.data?.total_patched ?? 0
      toast({
        title: n > 0 ? `AI 已补全 ${n} 条${storyboardItemLabel}缺失信息` : `检查完成，未发现需要补全的${storyboardItemLabel}信息`,
        variant: n > 0 ? 'success' : 'default',
      })
      mutateSb()
    } catch {
      toast({ title: 'AI 补全失败', variant: 'destructive' })
    } finally {
      setIsAuditingContinuity(false)
    }
  }

  // eslint-disable-next-line react-hooks/exhaustive-deps
  useOneShotTriggerEffect(sbGenerateTrigger, () => { void handleGenerateAll(undefined) }, onSbGenerateTriggerConsumed)
  // eslint-disable-next-line react-hooks/exhaustive-deps
  useOneShotTriggerEffect(sbRegenerateTrigger, () => { void handleForceGenerateEpisode(undefined) }, onSbRegenerateTriggerConsumed)
  // eslint-disable-next-line react-hooks/exhaustive-deps
  useOneShotTriggerEffect(sbPauseTrigger, () => handlePauseGeneration(), onSbPauseTriggerConsumed)
  // eslint-disable-next-line react-hooks/exhaustive-deps
  useOneShotTriggerEffect(sbResumeTrigger, () => handleResumeGeneration(), onSbResumeTriggerConsumed)
  // eslint-disable-next-line react-hooks/exhaustive-deps
  useOneShotTriggerEffect(sbAuditTrigger, () => handleAuditContinuity(), onSbAuditTriggerConsumed)
  // eslint-disable-next-line react-hooks/exhaustive-deps
  useOneShotTriggerEffect(sbRepairMetadataTrigger, () => { void handleRepairMetadata() }, onSbRepairMetadataTriggerConsumed)

  const handleVoid = async (id: number, selectedSb: Storyboard | null, setSelectedSb: (sb: Storyboard | null) => void) => {
    try {
      await storyboardAPI.void(projectId, id)
      toast({ title: '已作废', variant: 'success' })
      mutateSb()
      if (selectedSb?.id === id) setSelectedSb(null)
    } catch {
      toast({ title: '操作失败', variant: 'destructive' })
    }
  }

  const handleDelete = async (id: number, selectedSb: Storyboard | null, setSelectedSb: (sb: Storyboard | null) => void) => {
    if (!window.confirm('确认永久删除该分镜？此操作不可撤销。')) return
    try {
      await storyboardAPI.delete(projectId, id)
      toast({ title: '已删除', variant: 'success' })
      mutateSb()
      mutateStats()
      if (selectedSb?.id === id) setSelectedSb(null)
    } catch {
      toast({ title: '删除失败', variant: 'destructive' })
    }
  }

  const handleMergeWithPrevious = async (
    current: Storyboard,
    previous: Storyboard,
    selectedSb: Storyboard | null,
    setSelectedSb: (sb: Storyboard | null) => void,
  ) => {
    if (current.episode_id !== previous.episode_id) {
      toast({ title: '只能合并同集分镜', variant: 'destructive' })
      return
    }
    const mergedDialogue = joinStoryboardDialogue(previous.dialogue ?? '', current.dialogue ?? '')
    const limit = resolveStoryboardSpeechLimit(previous, project)
    const mergedRunes = countStoryboardDialogueRunes(mergedDialogue)
    const confirmMessage = mergedRunes > limit
      ? `合并后台词约 ${mergedRunes} 字，超过建议上限 ${limit} 字，配音可能偏长。仍要将第 ${current.sequence_number} 镜并入上一镜并删除本镜吗？`
      : `将第 ${current.sequence_number} 镜台词并入第 ${previous.sequence_number} 镜并删除本镜？`
    if (!window.confirm(confirmMessage)) return
    try {
      await storyboardAPI.update(projectId, previous.id, { dialogue: mergedDialogue })
      await storyboardAPI.delete(projectId, current.id)
      toast({ title: '已合并到上一镜', variant: 'success' })
      mutateSb()
      mutateStats()
      if (selectedSb?.id === current.id) {
        setSelectedSb(null)
      } else if (selectedSb?.id === previous.id) {
        setSelectedSb({ ...previous, dialogue: mergedDialogue })
      }
    } catch {
      toast({ title: '合并失败', variant: 'destructive' })
    }
  }

  const handleSwitchVersion = async (sbId: number, versionId: number) => {
    try {
      await storyboardAPI.switchVersion(projectId, sbId, versionId)
      toast({ title: '版本已切换', variant: 'success' })
      mutateSb()
    } catch {
      toast({ title: '切换失败', variant: 'destructive' })
    }
  }

  const handleCreateFromEpisodes = async () => {
    try {
      for (const ep of episodes) {
        await storyboardAPI.create(projectId, {
          episode_id: ep.id,
          sequence_number: ep.episode_number,
          scene_description: ep.summary || ep.title,
          duration: ep.estimated_duration || 4,
        })
      }
      toast({ title: `已创建 ${episodes.length} 条${storyboardItemLabel}`, variant: 'success' })
      mutateSb()
    } catch {
      toast({ title: '创建失败', variant: 'destructive' })
    }
  }

  const { data: voicesDataSb } = useSWR(
    'voices',
    () => dubbingAPI.listVoices().then((r) => (r as { data?: { voices?: { key: string; label: string }[] } }).data?.voices ?? null),
    { revalidateOnFocus: false, revalidateOnReconnect: false }
  )
  const SB_VOICE_OPTIONS = [
    { value: 'default', label: '默认音色' },
    { value: 'auto', label: '自动按人物分配' },
    ...(voicesDataSb ?? FALLBACK_VOICE_OPTIONS).map((v) => {
      const key = (v as { key?: string }).key ?? (v as { value?: string }).value ?? ''
      return { value: key, label: v.label }
    }),
  ]

  const [sbVoiceScope, setSbVoiceScope] = useState<'single' | 'episode'>('single')
  const [sbVoiceModel, setSbVoiceModel] = useState('default')
  const [sbVoiceRate, setSbVoiceRate] = useState('+0%')
  const [sbVoicePitch, setSbVoicePitch] = useState('+0Hz')
  const [sbVoiceVolume, setSbVoiceVolume] = useState('+0%')
  const [generatingSbVoice, setGeneratingSbVoice] = useState(false)

  const handleSbGenerateVoice = async (selectedSb: Storyboard) => {
    if (!selectedSb.episode_id) return
    setGeneratingSbVoice(true)
    try {
      if (sbVoiceScope === 'single') {
        const text = formatStoryboardDubbingText(selectedSb, {
          isCommentary: isCommentaryProject,
          maxRunes: resolveStoryboardSpeechLimit(selectedSb, project),
        })
        if (!text) {
          toast({ title: '该分镜暂无台词，无法生成语音', variant: 'destructive' })
          return
        }
        await dubbingAPI.generateForStoryboard(projectId, selectedSb.id, selectedSb.episode_id, text, sbVoiceModel, {
          voice_rate: sbVoiceRate,
          voice_pitch: sbVoicePitch,
          voice_volume: sbVoiceVolume,
        })
        mutateStoryboardTasks()
        toast({ title: '单帧语音任务已提交', variant: 'success' })
      } else {
        const allSbsRes = await storyboardAPI.listAll(projectId, { episode_id: selectedSb.episode_id }) as { data?: Storyboard[] }
        const eligible = (allSbsRes?.data ?? [])
          .sort((a, b) => a.sequence_number - b.sequence_number)
          .filter((sb) => formatStoryboardDubbingText(sb, {
            isCommentary: isCommentaryProject,
            maxRunes: resolveStoryboardSpeechLimit(sb, project),
          }))
        if (eligible.length === 0) {
          toast({ title: '当前集暂无台词，无法生成语音', variant: 'destructive' })
          return
        }
        let submitted = 0
        for (const sb of eligible) {
          const text = formatStoryboardDubbingText(sb, {
            isCommentary: isCommentaryProject,
            maxRunes: resolveStoryboardSpeechLimit(sb, project),
          })
          if (!sb.episode_id || !text) continue
          try {
            await dubbingAPI.generateForStoryboard(projectId, sb.id, sb.episode_id, text, sbVoiceModel, {
              voice_rate: sbVoiceRate,
              voice_pitch: sbVoicePitch,
              voice_volume: sbVoiceVolume,
            })
            submitted++
          } catch {
            // skip per storyboard
          }
        }
        mutateStoryboardTasks()
        toast({ title: `已提交 ${submitted} 个分镜语音任务`, variant: submitted > 0 ? 'success' : 'destructive' })
      }
    } catch (err) {
      const status = (err as { response?: { status?: number } })?.response?.status
      toast({
        title: status === 409 ? '当前分镜/集已有进行中的语音任务' : '语音生成提交失败',
        variant: 'destructive',
      })
    } finally {
      setGeneratingSbVoice(false)
    }
  }

  return {
    imageModelAvailability,
    videoModelAvailability,
    videoModelParams,
    getModelParam,
    setModelParam,
    pausingGeneration,
    resumingGeneration,
    isAuditingContinuity,
    isRepairingMetadata,
    showBatchStoryboardDialog,
    setShowBatchStoryboardDialog,
    batchStoryboardAction,
    batchStoryboardModels,
    setBatchStoryboardModels,
    batchStoryboardRunning,
    generatingVideoEps,
    videoDialogEpisodeId,
    setVideoDialogEpisodeId,
    generatingAllVideos,
    selectedEpisodeTransition,
    setSelectedEpisodeTransition,
    selectedEpisodeTransitionDuration,
    setSelectedEpisodeTransitionDuration,
    selectedEpisodeVideoModel,
    setSelectedEpisodeVideoModel,
    selectedEpisodeVideoStyle,
    setSelectedEpisodeVideoStyle,
    selectedEpisodeVideoMotionMode,
    setSelectedEpisodeVideoMotionMode,
    selectedEpisodeVideoFrameSize,
    setSelectedEpisodeVideoFrameSize,
    selectedEpisodeVideoSubjectSize,
    setSelectedEpisodeVideoSubjectSize,
    selectedEpisodeVideoClarity,
    setSelectedEpisodeVideoClarity,
    selectedEpisodeVideoModeLabel,
    selectedVideoDialogEpisode,
    selectedStoryboardBatchEpisode,
    applyEpisodeVideoPreset,
    openEpisodeVideoDialog,
    handleConfirmEpisodeVideoGeneration,
    handleGenerateAllEpisodeVideos,
    handleGenerateOne,
    handleGenerateModels,
    openBatchStoryboardDialog,
    executeBatchStoryboardAction,
    handlePauseGeneration,
    handleResumeGeneration,
    handleAuditContinuity,
    handleRepairMetadata,
    handleVoid,
    handleDelete,
    handleMergeWithPrevious,
    handleSwitchVersion,
    handleCreateFromEpisodes,
    SB_VOICE_OPTIONS,
    sbVoiceScope,
    setSbVoiceScope,
    sbVoiceModel,
    setSbVoiceModel,
    sbVoiceRate,
    setSbVoiceRate,
    sbVoicePitch,
    setSbVoicePitch,
    sbVoiceVolume,
    setSbVoiceVolume,
    generatingSbVoice,
    handleSbGenerateVoice,
  }
}
