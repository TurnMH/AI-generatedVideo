'use client'

import Link from 'next/link'
import { type ChangeEvent, useEffect, useMemo, useState } from 'react'
import { useParams } from 'next/navigation'
import useSWR from 'swr'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { useToast } from '@/components/ui/toast'
import { assetAPI, projectAPI, storyboardAPI, videoAPI, type Episode, type Project } from '@/lib/api'
import type { Asset, Storyboard } from '@/types'

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
  const [selectedEpisodeId, setSelectedEpisodeId] = useState<string>('all')
  const [generationAction, setGenerationAction] = useState<string | null>(null)
  const [rerunAction, setRerunAction] = useState<string | null>(null)
  const [uploadingAssetId, setUploadingAssetId] = useState<number | null>(null)
  const [selectedVideoModel, setSelectedVideoModel] = useState('')
  const [selectedAspectRatio, setSelectedAspectRatio] = useState('')
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

  const completedStoryboardImages = useMemo(
    () => scopeStoryboards.filter((item) => String(item.image_url || '').trim()).length,
    [scopeStoryboards],
  )

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

  useEffect(() => {
    const next = typeof autoSplit?.optimized_script === 'string' ? autoSplit.optimized_script : ''
    setEditableOptimizedScript((prev) => {
      if (!prev.trim()) return next
      if (prev.trim() === next.trim()) return prev
      if (!next.trim()) return prev
      return prev
    })
  }, [autoSplit?.optimized_script])

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
    await Promise.all([mutateProject(), mutateEpisodes(), mutateStoryboards(), mutateTasks(), mutateAssets()])
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

  const rerunStoryboardPipeline = async () => {
    const scriptText = editableOptimizedScript.trim()
    if (!scriptText) {
      toast({ title: '请先保留一版可用的优化文案，再开始按视频配置重拆分', variant: 'destructive' })
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
              <CardTitle>项目总览</CardTitle>
              <CardDescription className="text-slate-400">project #{project.id} · {project.title}</CardDescription>
            </CardHeader>
            <CardContent className="space-y-3 text-sm text-slate-200">
              <div>状态：{project.status || '-'} · 当前阶段：{humanStage(project)}</div>
              <div>style_tags：{Array.isArray(project.style_tags) && project.style_tags.length > 0 ? project.style_tags.join(' / ') : '-'}</div>
              <div>文案长度：{autoSplit?.script_length || (project.script_text || '').length || '-'} · 预估分集：{autoSplit?.estimated_episodes || episodes.length || '-'}</div>
              <div>当前处理范围：{scopeLabel}</div>
              {resultUrl && <div className="break-all">最新完整视频：<a className="text-cyan-300 underline" href={resultUrl} target="_blank" rel="noreferrer">{resultUrl}</a></div>}
              {latestTask?.error_msg && <div className="rounded-lg border border-rose-500/20 bg-rose-500/10 p-3 text-rose-200">最新视频任务错误：{latestTask.error_msg}</div>}
            </CardContent>
          </Card>

          <Card className="border-white/10 bg-slate-900/60 text-slate-100">
            <CardHeader>
              <CardTitle>文案优化与一致性前提</CardTitle>
              <CardDescription className="text-slate-400">顶部先留住当前可编辑文案；真正的执行入口全部收进下面的流水线。</CardDescription>
            </CardHeader>
            <CardContent className="space-y-4 text-sm text-slate-200">
              <div className="rounded-lg border border-white/10 bg-black/20 p-3">
                <div className="mb-2 text-xs font-medium text-slate-400">优化前全文</div>
                <div className="whitespace-pre-wrap break-words text-slate-100">{autoSplit?.original_script || project.script_text || '暂无'}</div>
              </div>
              <div className="rounded-lg border border-emerald-500/20 bg-emerald-500/10 p-3 space-y-3">
                <div className="flex flex-wrap items-center justify-between gap-3">
                  <div>
                    <div className="mb-2 text-xs font-medium text-emerald-200">当前用于拆分的广告文案</div>
                    <div className="text-[11px] text-emerald-200/75">你可以先在这里修正文案，再按下方步骤 1 的视频配置重新拆分。</div>
                  </div>
                  <div className="text-[11px] text-emerald-200/75">{editableOptimizedScript.trim().length} 字</div>
                </div>
                <Textarea
                  value={editableOptimizedScript}
                  onChange={(e) => setEditableOptimizedScript(e.target.value)}
                  className="min-h-[220px] border-emerald-500/20 bg-black/20 text-slate-100"
                  placeholder="这里保留当前要进入流水线的广告文案。"
                />
              </div>
              <div className="rounded-lg border border-amber-500/20 bg-amber-500/10 p-3">
                <div className="mb-2 text-xs font-medium text-amber-200">一致性前提</div>
                <div className="whitespace-pre-wrap break-words text-slate-100">{autoSplit?.consistency_premise || '当前运行态尚未返回一致性前提。'}</div>
              </div>
            </CardContent>
          </Card>

          <Card className="border-white/10 bg-slate-900/60 text-slate-100">
            <CardHeader>
              <CardTitle>广告流水线</CardTitle>
              <CardDescription className="text-slate-400">顺序固定：1）文本拆分 → 2）人物图 / 分镜图准备 → 3）视频生成。</CardDescription>
            </CardHeader>
            <CardContent className="space-y-5">
              <div className="grid gap-3 md:grid-cols-3">
                <div className="rounded-xl border border-cyan-500/20 bg-cyan-500/10 p-4">
                  <div className="text-xs font-medium text-cyan-200">步骤 1</div>
                  <div className="mt-1 text-base font-semibold text-white">按视频配置重拆分文本</div>
                  <div className="mt-2 text-xs text-cyan-100/80">先确定视频模型、比例、分辨率、单分镜时长，再把当前文案重跑为分镜文本。</div>
                </div>
                <div className="rounded-xl border border-violet-500/20 bg-violet-500/10 p-4">
                  <div className="text-xs font-medium text-violet-200">步骤 2</div>
                  <div className="mt-1 text-base font-semibold text-white">上传人物图 / 刷新分镜图</div>
                  <div className="mt-2 text-xs text-violet-100/80">先准备人物 / 素材槽位，再为当前范围逐个上传真实参考图，最后刷新分镜图。</div>
                </div>
                <div className="rounded-xl border border-emerald-500/20 bg-emerald-500/10 p-4">
                  <div className="text-xs font-medium text-emerald-200">步骤 3</div>
                  <div className="mt-1 text-base font-semibold text-white">开始生成视频</div>
                  <div className="mt-2 text-xs text-emerald-100/80">只有分镜图准备完成后才启动视频生成，避免直接拿空图或错图提交。</div>
                </div>
              </div>

              <div className="rounded-xl border border-white/10 bg-black/20 p-4 space-y-4">
                <div className="grid gap-4 lg:grid-cols-[minmax(0,280px)_1fr]">
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
                    <div className="text-[11px] text-slate-400">步骤 2 和步骤 3 将按这个范围执行；步骤 1 始终基于当前全文重拆分。</div>
                  </div>

                  <div className="grid gap-3 md:grid-cols-4">
                    <div className="rounded-lg border border-white/10 bg-slate-950/50 p-3">
                      <div className="text-xs text-slate-400">已拆出的分镜</div>
                      <div className="mt-2 text-2xl font-semibold text-white">{scopeStoryboards.length}</div>
                    </div>
                    <div className="rounded-lg border border-white/10 bg-slate-950/50 p-3">
                      <div className="text-xs text-slate-400">已上传人物/素材图</div>
                      <div className="mt-2 text-2xl font-semibold text-white">{uploadedScopeAssets}/{scopeAssets.length}</div>
                    </div>
                    <div className="rounded-lg border border-white/10 bg-slate-950/50 p-3">
                      <div className="text-xs text-slate-400">已就绪分镜图</div>
                      <div className="mt-2 text-2xl font-semibold text-white">{completedStoryboardImages}/{scopeStoryboards.length}</div>
                    </div>
                    <div className="rounded-lg border border-white/10 bg-slate-950/50 p-3">
                      <div className="text-xs text-slate-400">进行中视频任务</div>
                      <div className="mt-2 text-2xl font-semibold text-white">{processingVideoTaskCount}</div>
                    </div>
                  </div>
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
                    disabled={rerunAction !== null || generationAction !== null || !editableOptimizedScript.trim() || !splitConfigReady}
                    onClick={() => void rerunStoryboardPipeline()}
                  >
                    {rerunAction === 'pipeline' ? '正在按当前配置重拆分…' : '开始步骤 1：按当前视频配置重拆分'}
                  </Button>
                </div>

                <div className="rounded-xl border border-violet-500/20 bg-violet-500/10 p-4 space-y-4">
                  <div>
                    <div className="text-sm font-medium text-violet-100">步骤 2：准备人物图 / 刷新分镜图</div>
                    <div className="mt-1 text-xs text-violet-100/80">真实链路是：先生成可上传的人物 / 素材槽位，再上传参考图，最后刷新当前范围的分镜图。</div>
                  </div>

                  <div className="flex flex-wrap gap-2">
                    <Button
                      size="sm"
                      variant="outline"
                      disabled={generationAction !== null}
                      onClick={() => void triggerAssetExtraction()}
                    >
                      {generationAction === 'asset-all' || generationAction === `asset-episode-${selectedEpisodeNumber}` ? '准备中…' : '先准备人物槽位'}
                    </Button>
                    <Button
                      size="sm"
                      disabled={generationAction !== null || scopeAssets.length === 0}
                      onClick={() => void triggerStoryboardImageGeneration()}
                    >
                      {generationAction === 'storyboard-image-all' || generationAction === `storyboard-image-episode-${selectedEpisodeNumber}` ? '刷新中…' : '上传完成后刷新分镜图'}
                    </Button>
                  </div>

                  <div className="max-h-[420px] space-y-3 overflow-auto pr-1">
                    {scopeAssets.length === 0 ? (
                      <div className="rounded-lg border border-white/10 bg-black/20 p-4 text-xs text-violet-100/80">
                        当前范围还没有人物 / 素材槽位。先点“准备人物槽位”，再逐个上传参考图。
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
                            {String(asset.image_url || '').trim() ? '已上传参考图' : '待上传'}
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
                            {uploadingAssetId === asset.id ? '上传中…' : String(asset.image_url || '').trim() ? '重新上传人物图' : '上传人物图'}
                            <input
                              type="file"
                              accept="image/*"
                              className="hidden"
                              disabled={uploadingAssetId !== null}
                              onChange={(event) => { void handleAssetUpload(asset.id, event) }}
                            />
                          </label>
                          <div className="text-[11px] text-slate-400">建议按当前角色 / 物件的最终定稿图上传，后续再刷新分镜图。</div>
                        </div>
                      </div>
                    ))}
                  </div>
                </div>

                <div className="rounded-xl border border-emerald-500/20 bg-emerald-500/10 p-4 space-y-4">
                  <div>
                    <div className="text-sm font-medium text-emerald-100">步骤 3：用当前分镜图启动视频生成</div>
                    <div className="mt-1 text-xs text-emerald-100/80">这里不会绕过前两步。当前范围必须至少有一批可用分镜图，才允许提交视频任务。</div>
                  </div>

                  <div className="rounded-lg border border-white/10 bg-black/20 p-3 text-xs text-emerald-100/85 space-y-2">
                    <div>当前模型：{selectedVideoModel || '未选择'}</div>
                    <div>比例 / 分辨率 / 单分镜时长：{selectedAspectRatio || '-'} / {selectedResolution || '-'} / {selectedDuration || '-'} 秒</div>
                    <div>当前范围：{scopeLabel}</div>
                    <div>可用分镜图：{completedStoryboardImages} / {scopeStoryboards.length}</div>
                  </div>

                  <Button
                    disabled={generationAction !== null || !splitConfigReady || completedStoryboardImages === 0}
                    onClick={() => void startScopedVideoGeneration()}
                  >
                    {generationAction === 'video-start' ? '正在提交视频任务…' : selectedEpisodeNumber ? '开始生成当前分集视频' : '开始生成当前范围视频'}
                  </Button>

                  <div className="text-[11px] text-emerald-100/75">
                    视频生成真实读取的是分镜图 `image_url`、分镜文案、台词、镜头运动、人物 / 素材引用等字段；不是直接拿人物图就开跑。
                  </div>
                </div>
              </div>
            </CardContent>
          </Card>

          <Card className="border-white/10 bg-slate-900/60 text-slate-100">
            <CardHeader>
              <CardTitle>分集进度</CardTitle>
              <CardDescription className="text-slate-400">展示自动切分后的片段载体。</CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              {episodes.length === 0 ? (
                <div className="rounded-lg border border-white/10 bg-black/20 p-4 text-sm text-slate-300">当前还没有分集记录。</div>
              ) : episodes.map((episode) => (
                <div key={episode.id} className="rounded-lg border border-white/10 bg-black/20 p-4 text-sm text-slate-200">
                  <div>episode #{episode.episode_number} · {episode.title || '未命名片段'} · {episode.status}</div>
                  <div className="mt-2 whitespace-pre-wrap break-words text-slate-400">{episode.summary || episode.script_excerpt || '暂无摘要'}</div>
                </div>
              ))}
            </CardContent>
          </Card>

          <Card className="border-white/10 bg-slate-900/60 text-slate-100">
            <CardHeader>
              <CardTitle>当前范围分镜</CardTitle>
              <CardDescription className="text-slate-400">这里直接看当前范围的 scene_description / dialogue / 分镜图是否齐了。</CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              {scopeStoryboards.length === 0 ? (
                <div className="rounded-lg border border-white/10 bg-black/20 p-4 text-sm text-slate-300">当前范围还没有分镜记录。</div>
              ) : scopeStoryboards.map((storyboard) => (
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
            </CardContent>
          </Card>

          <Card className="border-white/10 bg-slate-900/60 text-slate-100">
            <CardHeader>
              <CardTitle>视频任务 / 完整视频</CardTitle>
              <CardDescription className="text-slate-400">广告历史详情内直接查看当前视频生成进度与完整视频结果。</CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              {tasks.length === 0 ? (
                <div className="rounded-lg border border-white/10 bg-black/20 p-4 text-sm text-slate-300">当前还没有视频任务记录。</div>
              ) : tasks.slice().sort((a, b) => Number(b.id) - Number(a.id)).map((task) => (
                <div key={task.id} className="rounded-lg border border-white/10 bg-black/20 p-4 text-sm text-slate-200">
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
            </CardContent>
          </Card>
        </>
      )}
    </div>
  )
}
