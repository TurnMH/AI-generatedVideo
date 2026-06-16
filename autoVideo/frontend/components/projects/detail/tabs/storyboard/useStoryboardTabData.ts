'use client'

import { useEffect, useMemo, useState } from 'react'
import useSWR from 'swr'
import { projectAPI, assetAPI, storyboardAPI, dubbingAPI, modelAPI, type DubbingTask } from '@/lib/api'
import { buildImageModelOption, buildVideoModelCapability, dedupeModels } from '@/lib/model-display'
import { useProjectEpisodeFilter } from '@/lib/projects/episode-filter'
import { getStoryboardAssetReadiness } from '@/lib/projects/storyboard-assets-readiness'
import type { Asset, Episode, Model, Project, Storyboard } from '@/types'

type StatsData = { total: number; pending: number; generating: number; paused: number; completed: number; failed: number; voided: number }

export function useStoryboardTabData(projectId: number, project: Project, episodeId?: number) {
  const sbSharedEpisode = useProjectEpisodeFilter()
  const [episodeFilter, setEpisodeFilter] = useState<string>(() => episodeId ? String(episodeId) : sbSharedEpisode.value)
  useEffect(() => {
    sbSharedEpisode.setValue(episodeFilter)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [episodeFilter])

  const [statusFilter, setStatusFilter] = useState<string>('all')
  const [keyword, setKeyword] = useState('')
  const [sbPage, setSbPage] = useState(1)
  const sbPageSize = 50

  useEffect(() => { setSbPage(1) }, [episodeFilter, statusFilter, keyword])

  const { data: sbImageModelsData } = useSWR(
    ['storyboard-image-models', projectId],
    () => modelAPI.list({ type: 'image', sort_by: 'priority' }) as unknown as Promise<{ data: Model[] }>
  )
  const sbImageModels: Model[] = (sbImageModelsData as { data?: Model[] })?.data ?? []
  const sbProjectImageModelKey = sbImageModels.find(m => m.id === project.image_model_id)?.model_key ?? ''

  const { data: sbVideoModelsData } = useSWR(
    ['storyboard-video-models', projectId],
    () => modelAPI.list({ type: 'video', sort_by: 'priority' }) as unknown as Promise<{ data: Model[] }>
  )
  const sbVideoModels: Model[] = (sbVideoModelsData as { data?: Model[] })?.data ?? []
  const vtVideoModelOptions = useMemo(
    () => dedupeModels(sbVideoModels.filter((m) => m.is_active)).map(buildVideoModelCapability),
    [sbVideoModels]
  )

  const { data: statsRaw, mutate: mutateStats } = useSWR(
    ['storyboard-stats', projectId],
    () => storyboardAPI.stats(projectId) as unknown as Promise<{ data: StatsData }>,
    {
      refreshInterval: (data) => {
        const s = (data as { data?: StatsData } | undefined)?.data
        if (!s) return 5000
        return (s.generating > 0 || s.pending > 0) ? 5000 : 30000
      },
    }
  )
  const stats: StatsData = (statsRaw as { data?: StatsData })?.data ?? { total: 0, pending: 0, generating: 0, paused: 0, completed: 0, failed: 0, voided: 0 }
  const isActive = stats.generating > 0

  const { data: storyboardAssetsRaw } = useSWR(
    ['storyboard-assets', projectId],
    () => assetAPI.list(projectId) as unknown as Promise<{ data: Asset[] }>,
    { refreshInterval: isActive ? 5000 : 30000 }
  )
  const storyboardAssets = ((storyboardAssetsRaw as { data?: Asset[] })?.data ?? []).filter((asset) => asset.name !== '__extracting__')
  const scopedStoryboardAssets = episodeFilter === 'all'
    ? storyboardAssets
    : storyboardAssets.filter((asset) => (asset.episode_ids ?? []).includes(Number(episodeFilter)))
  const assetReadiness = getStoryboardAssetReadiness(scopedStoryboardAssets)

  const { data: episodesData } = useSWR(
    ['episodes', projectId],
    () => projectAPI.listEpisodes(projectId) as unknown as Promise<{ data: Episode[] }>
  )
  const episodes = (episodesData as { data?: Episode[] })?.data ?? []

  type SbListResp = { data: Storyboard[] | { items: Storyboard[] }; page_info?: { page: number; page_size: number; total: number } }
  const { data: sbData, isLoading, mutate: mutateSb } = useSWR(
    ['storyboards', projectId, episodeFilter, statusFilter, keyword, sbPage],
    () => {
      const params: { episode_id?: number; status?: string; keyword?: string; page?: number; page_size?: number } = { page: sbPage, page_size: sbPageSize }
      if (episodeFilter !== 'all') params.episode_id = Number(episodeFilter)
      if (statusFilter !== 'all') params.status = statusFilter
      if (keyword.trim()) params.keyword = keyword.trim()
      return storyboardAPI.list(projectId, { ...params, include_versions: true }) as unknown as Promise<SbListResp>
    },
    {
      refreshInterval: () => {
        if (project.status === 'script_processing') return 3000
        if (isActive) return 5000
        const raw = (sbData as SbListResp)?.data
        const sbs: Storyboard[] = Array.isArray(raw) ? raw : (raw as { items?: Storyboard[] })?.items ?? []
        return sbs.some((s) => s.status === 'generating') ? 3000 : 0
      },
    }
  )

  const { data: epStatsRaw } = useSWR(
    stats.total > 0 ? ['storyboard-episode-stats', projectId] : null,
    () => storyboardAPI.episodeStats(projectId) as unknown as Promise<{ data: Record<string, number> }>,
    { refreshInterval: isActive ? 5000 : 0 }
  )
  const episodeCompletedMap = useMemo(() => {
    const raw = (epStatsRaw as { data?: Record<string, number> })?.data ?? {}
    const m = new Map<number, number>()
    for (const [k, v] of Object.entries(raw)) m.set(Number(k), v)
    return m
  }, [epStatsRaw])

  const rawSb = (sbData as SbListResp)?.data
  const storyboards: Storyboard[] = Array.isArray(rawSb) ? rawSb : (rawSb as { items?: Storyboard[] })?.items ?? []
  const sbTotal = (sbData as SbListResp)?.page_info?.total ?? storyboards.length
  const sbTotalPages = Math.max(1, Math.ceil(sbTotal / sbPageSize))

  const SB_MODEL_OPTIONS = useMemo(() => {
    const unsorted = dedupeModels([
      ...sbImageModels.filter((m) => m.is_active && m.model_key),
      ...sbImageModels.filter((m) => !m.is_active && m.failure_reason && m.model_key),
    ]).map(buildImageModelOption)

    return [...unsorted].sort((a, b) => {
      const aKey = (a.key || '').toLowerCase()
      const bKey = (b.key || '').toLowerCase()
      const aLabel = (a.label || '').toLowerCase()
      const bLabel = (b.label || '').toLowerCase()

      const isAGpt = aKey.includes('gpt') || aLabel.includes('gpt')
      const isBGpt = bKey.includes('gpt') || bLabel.includes('gpt')

      if (isAGpt && !isBGpt) return -1
      if (!isAGpt && isBGpt) return 1

      if (isAGpt && isBGpt) {
        // Both are GPT models, prioritize gpt-image-2 / gpt-img-2
        const isAGpt2 = aKey === 'gpt-image-2' || aKey === 'gpt-img-2'
        const isBGpt2 = bKey === 'gpt-image-2' || bKey === 'gpt-img-2'
        if (isAGpt2 && !isBGpt2) return -1
        if (!isAGpt2 && isBGpt2) return 1

        // Otherwise sort GPT models alphabetically
        return (a.label || '').localeCompare(b.label || '', 'zh-CN')
      }

      const providerCompare = (a.provider || '').localeCompare(b.provider || '', 'zh-CN')
      if (providerCompare !== 0) {
        return providerCompare
      }

      return (a.label || '').localeCompare(b.label || '', 'zh-CN')
    })
  }, [sbImageModels])
  const storyboardDefaultImageModelLabel = SB_MODEL_OPTIONS.find((model) => model.key === sbProjectImageModelKey)?.label || '项目默认模型'

  const { data: sbTasksData, mutate: mutateStoryboardTasks } = useSWR(
    project.enable_dubbing ? ['storyboard-dubbing-tasks', projectId] : null,
    () => dubbingAPI.listStoryboardTasks(projectId),
    {
      refreshInterval: (data) => {
        const tasks = Array.isArray(data) ? data as DubbingTask[] : []
        return tasks.some(t => t.status === 'processing' || t.status === 'pending') ? 3000 : 15000
      },
    }
  )
  const storyboardTaskMap = useMemo(() => {
    const map = new Map<number, DubbingTask>()
    for (const t of (sbTasksData as DubbingTask[] | undefined) ?? []) {
      if (t.storyboard_id != null) map.set(t.storyboard_id, t)
    }
    return map
  }, [sbTasksData])

  return {
    episodeFilter,
    setEpisodeFilter,
    statusFilter,
    setStatusFilter,
    keyword,
    setKeyword,
    sbPage,
    setSbPage,
    sbPageSize,
    sbImageModels,
    sbProjectImageModelKey,
    sbVideoModels,
    vtVideoModelOptions,
    stats,
    mutateStats,
    isActive,
    storyboardAssets,
    assetReadiness,
    episodes,
    storyboards,
    sbTotal,
    sbTotalPages,
    isLoading,
    mutateSb,
    episodeCompletedMap,
    SB_MODEL_OPTIONS,
    storyboardDefaultImageModelLabel,
    storyboardTaskMap,
    mutateStoryboardTasks,
  }
}
