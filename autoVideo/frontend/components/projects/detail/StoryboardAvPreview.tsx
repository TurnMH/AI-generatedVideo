'use client'

import { useMemo } from 'react'
import useSWR from 'swr'
import { Loader2, Video } from 'lucide-react'
import { videoAPI, type DubbingTask } from '@/lib/api'
import {
  resolveStoryboardAvPreview,
  type VideoTaskForAvPreview,
} from '@/lib/projects/storyboard-av-preview'
import { SyncedClipPlayer } from '@/components/projects/detail/SyncedClipPlayer'

type StoryboardAvPreviewProps = {
  projectId: number
  storyboardId: number
  episodeId?: number | null
  dubbingTask?: DubbingTask
  storyboardUpdatedAt?: string
  compact?: boolean
  className?: string
  videoTasks?: VideoTaskForAvPreview[]
}

function extractVideoTasksFromListResponse(payload: unknown): VideoTaskForAvPreview[] {
  if (!payload || typeof payload !== 'object') return []
  const items = (payload as { items?: unknown[] }).items
  if (!Array.isArray(items)) return []
  return items.map((item) => {
    if (item && typeof item === 'object' && 'task' in item) {
      const wrapped = item as { task?: VideoTaskForAvPreview }
      return wrapped.task ?? (item as unknown as VideoTaskForAvPreview)
    }
    return item as VideoTaskForAvPreview
  })
}

export function StoryboardAvPreview({
  projectId,
  storyboardId,
  episodeId,
  dubbingTask,
  storyboardUpdatedAt,
  compact = false,
  className,
  videoTasks: videoTasksProp,
}: StoryboardAvPreviewProps) {
  const shouldFetch = videoTasksProp == null
  const { data: tasksRaw, isLoading } = useSWR(
    shouldFetch ? ['storyboard-av-video-tasks', projectId] : null,
    () => videoAPI.listTasks(projectId, { page: 1, page_size: 200 }).then((r) => r.data),
    { revalidateOnFocus: false },
  )

  const videoTasks = useMemo(
    () => videoTasksProp ?? extractVideoTasksFromListResponse(tasksRaw),
    [videoTasksProp, tasksRaw],
  )

  const sources = useMemo(
    () => resolveStoryboardAvPreview({
      storyboardId,
      episodeId,
      dubbingTask,
      storyboardUpdatedAt,
      videoTasks,
    }),
    [storyboardId, episodeId, dubbingTask, storyboardUpdatedAt, videoTasks],
  )

  if (shouldFetch && isLoading && sources.status === 'none') {
    return (
      <div className={`flex items-center gap-1.5 text-[11px] text-surface-400 ${className ?? ''}`}>
        <Loader2 className="h-3 w-3 animate-spin" /> 查找视频片段…
      </div>
    )
  }

  if (sources.status === 'ready' || sources.status === 'dubbing_stale') {
    if (!sources.video?.clipUrl || !sources.audioUrl) return null
    return (
      <div className={className}>
        <p className="mb-1.5 flex items-center gap-1 text-[10px] font-medium text-surface-500">
          <Video className="h-3 w-3" /> 音画同步预览
        </p>
        <SyncedClipPlayer
          videoUrl={sources.video.clipUrl}
          audioUrl={sources.audioUrl}
          videoDurationSec={sources.video.durationSec}
          audioDurationSec={sources.audioDurationSec}
          compact={compact}
          staleHint={
            sources.status === 'dubbing_stale'
              ? '分镜台词已更新，当前配音可能不是最新版本'
              : null
          }
        />
      </div>
    )
  }

  if (sources.status === 'video_only') {
    return (
      <p className={`text-[10px] text-surface-400 ${className ?? ''}`}>
        视频片段已就绪，生成配音后可音画同步预览
      </p>
    )
  }

  if (sources.status === 'audio_only') {
    return (
      <p className={`text-[10px] text-surface-400 ${className ?? ''}`}>
        配音已就绪，生成对应视频片段后可音画同步预览
      </p>
    )
  }

  return null
}
