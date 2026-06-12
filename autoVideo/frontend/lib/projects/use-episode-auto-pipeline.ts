import { useCallback, useEffect, useMemo, useState } from 'react'
import { mutate as globalMutate } from 'swr'
import { projectAPI } from '@/lib/api'
import type { Episode } from '@/types'
import {
  getFirstEpisodeNumber,
  isEpisodePipelineComplete,
  isEpisodePipelineFailed,
  isEpisodePipelineRunning,
  resolveEpisodeAutoPipelineAction,
  type EpisodeAutoPipelineAction,
} from '@/lib/projects/episode-pipeline'

type EpisodeWorkspaceMeta = {
  storyboardTotal?: number
  assetExtracting?: boolean
  assetGenerating?: boolean
  storyboardGenerating?: boolean
  storyboardFailed?: boolean
  assetFailed?: number
}

type UseEpisodeAutoPipelineOptions = {
  projectId: number
  episodes: Episode[]
  episodeWorkspaceMeta: Map<number, EpisodeWorkspaceMeta>
  firstEpisodeAutoActive: boolean
  mutateProject: () => void
  toast: (input: { title: string; description?: string; variant?: 'default' | 'success' | 'destructive' }) => void
}

export function useEpisodeAutoPipeline({
  projectId,
  episodes,
  episodeWorkspaceMeta,
  firstEpisodeAutoActive,
  mutateProject,
  toast,
}: UseEpisodeAutoPipelineOptions) {
  const [autoPreparingEpisodeId, setAutoPreparingEpisodeId] = useState<number | null>(null)
  const [attemptedEpisodeIds, setAttemptedEpisodeIds] = useState<Record<number, true>>({})

  const firstEpisodeNumber = useMemo(() => getFirstEpisodeNumber(episodes), [episodes])

  useEffect(() => {
    if (!firstEpisodeNumber) return
    const firstEpisode = episodes.find((episode) => episode.episode_number === firstEpisodeNumber)
    if (!firstEpisode) return
    const meta = episodeWorkspaceMeta.get(firstEpisode.id)
    const storyboardTotal = meta?.storyboardTotal ?? 0
    const shouldMarkAttempted = firstEpisodeAutoActive || isEpisodePipelineComplete(firstEpisode, storyboardTotal)
    if (!shouldMarkAttempted) return
    setAttemptedEpisodeIds((prev) => (prev[firstEpisode.id] ? prev : { ...prev, [firstEpisode.id]: true }))
  }, [episodes, episodeWorkspaceMeta, firstEpisodeAutoActive, firstEpisodeNumber])

  useEffect(() => {
    if (!autoPreparingEpisodeId) return
    const target = episodes.find((episode) => episode.id === autoPreparingEpisodeId)
    if (!target) return

    const meta = episodeWorkspaceMeta.get(target.id)
    const storyboardTotal = meta?.storyboardTotal ?? 0
    const running = isEpisodePipelineRunning({
      episode: target,
      storyboardTotal,
      firstEpisodeNumber,
      autoPreparingEpisodeId,
      firstEpisodeAutoActive,
      assetExtracting: meta?.assetExtracting,
      assetGenerating: meta?.assetGenerating,
      storyboardGenerating: meta?.storyboardGenerating,
    })
    const complete = isEpisodePipelineComplete(target, storyboardTotal)
    const failed = isEpisodePipelineFailed({
      episode: target,
      storyboardTotal,
      hasStoryboardFailure: meta?.storyboardFailed,
      hasAssetFailure: (meta?.assetFailed ?? 0) > 0,
    })

    if (!running && (complete || failed)) {
      setAutoPreparingEpisodeId(null)
    }
  }, [
    autoPreparingEpisodeId,
    episodes,
    episodeWorkspaceMeta,
    firstEpisodeAutoActive,
    firstEpisodeNumber,
  ])

  const markAttempted = useCallback((episodeId: number) => {
    setAttemptedEpisodeIds((prev) => ({ ...prev, [episodeId]: true }))
  }, [])

  const handleEpisodeAutoPipeline = useCallback(async (episodeId: number, event?: React.MouseEvent) => {
    event?.stopPropagation()
    markAttempted(episodeId)
    setAutoPreparingEpisodeId(episodeId)
    try {
      await projectAPI.autoPrepareEpisode(projectId, episodeId)
      toast({
        title: '已启动单集自动处理',
        description: '将依次完成剧本润色、资源提取与分镜拆分，状态会在列表中实时更新。',
        variant: 'success',
      })
      mutateProject()
      globalMutate(['episodes', projectId])
    } catch (error: unknown) {
      const message = (error as { response?: { data?: { message?: string } } })?.response?.data?.message
      toast({
        title: '自动处理启动失败',
        description: message || '请稍后重试',
        variant: 'destructive',
      })
      setAutoPreparingEpisodeId(null)
    }
  }, [markAttempted, mutateProject, projectId, toast])

  const getEpisodeAutoAction = useCallback((episode: Episode): EpisodeAutoPipelineAction => {
    const meta = episodeWorkspaceMeta.get(episode.id)
    const storyboardTotal = meta?.storyboardTotal ?? 0
    return resolveEpisodeAutoPipelineAction({
      episode,
      storyboardTotal,
      firstEpisodeNumber,
      autoPreparingEpisodeId,
      firstEpisodeAutoActive,
      assetExtracting: meta?.assetExtracting,
      assetGenerating: meta?.assetGenerating,
      storyboardGenerating: meta?.storyboardGenerating,
      hasStoryboardFailure: meta?.storyboardFailed,
      hasAssetFailure: (meta?.assetFailed ?? 0) > 0,
      wasAutoAttempted: Boolean(attemptedEpisodeIds[episode.id]),
    })
  }, [
    attemptedEpisodeIds,
    autoPreparingEpisodeId,
    episodeWorkspaceMeta,
    firstEpisodeAutoActive,
    firstEpisodeNumber,
  ])

  return {
    autoPreparingEpisodeId,
    getEpisodeAutoAction,
    handleEpisodeAutoPipeline,
  }
}
