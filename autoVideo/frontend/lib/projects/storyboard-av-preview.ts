import type { DubbingTask } from '@/lib/api'

export type StoryboardVideoClip = {
  clipUrl: string
  durationSec: number
  taskId: number
  clipOrder: number
}

export type StoryboardAvPreviewStatus =
  | 'none'
  | 'video_only'
  | 'audio_only'
  | 'ready'
  | 'dubbing_stale'

export type StoryboardAvPreviewSources = {
  status: StoryboardAvPreviewStatus
  video?: StoryboardVideoClip
  audioUrl?: string
  audioDurationSec?: number
}

export type VideoTaskForAvPreview = {
  id: number
  episode_id?: number | null
  status?: string
  render_config?: Record<string, unknown>
  clips?: Array<{
    clip_order: number
    status: string
    clip_url: string
    duration_sec: number
  }>
  updated_at?: string
  created_at?: string
}

function parseTimestampMs(value?: string): number {
  if (!value) return 0
  const ms = Date.parse(value)
  return Number.isFinite(ms) ? ms : 0
}

export function extractStoryboardIdsFromRenderConfig(
  renderConfig: Record<string, unknown> | undefined,
): number[] {
  const raw = renderConfig?.storyboard_ids
  if (!Array.isArray(raw)) return []
  return raw.map((id) => Number(id)).filter((id) => Number.isFinite(id) && id > 0)
}

export function resolveStoryboardIdForClip(
  renderConfig: Record<string, unknown> | undefined,
  clipOrder: number,
): number | undefined {
  const ids = extractStoryboardIdsFromRenderConfig(renderConfig)
  if (clipOrder < 0 || clipOrder >= ids.length) return undefined
  return ids[clipOrder]
}

export function findStoryboardVideoClip(
  storyboardId: number,
  episodeId: number | null | undefined,
  tasks: VideoTaskForAvPreview[],
): StoryboardVideoClip | undefined {
  const candidates = [...tasks]
    .filter((task) => {
      if (episodeId != null && episodeId > 0 && task.episode_id != null && task.episode_id !== episodeId) {
        return false
      }
      return true
    })
    .sort((a, b) => parseTimestampMs(b.updated_at ?? b.created_at) - parseTimestampMs(a.updated_at ?? a.created_at))

  for (const task of candidates) {
    const storyboardIds = extractStoryboardIdsFromRenderConfig(task.render_config)
    if (storyboardIds.length === 0) continue

    for (const clip of task.clips ?? []) {
      if (clip.status !== 'succeeded' || !clip.clip_url?.trim()) continue
      const idx = clip.clip_order
      if (idx < 0 || idx >= storyboardIds.length) continue
      if (storyboardIds[idx] !== storyboardId) continue
      return {
        clipUrl: clip.clip_url,
        durationSec: clip.duration_sec > 0 ? clip.duration_sec : 0,
        taskId: task.id,
        clipOrder: idx,
      }
    }
  }

  return undefined
}

export function isDubbingStaleForStoryboard(
  dubbingTask: DubbingTask | undefined,
  storyboardUpdatedAt?: string,
): boolean {
  if (!dubbingTask || dubbingTask.status !== 'succeeded') return false
  const dubbingMs = parseTimestampMs(dubbingTask.updated_at)
  const storyboardMs = parseTimestampMs(storyboardUpdatedAt)
  return storyboardMs > dubbingMs
}

export function resolveStoryboardAvPreview({
  storyboardId,
  episodeId,
  dubbingTask,
  storyboardUpdatedAt,
  videoTasks,
}: {
  storyboardId: number
  episodeId?: number | null
  dubbingTask?: DubbingTask
  storyboardUpdatedAt?: string
  videoTasks: VideoTaskForAvPreview[]
}): StoryboardAvPreviewSources {
  const video = findStoryboardVideoClip(storyboardId, episodeId, videoTasks)
  const audioUrl =
    dubbingTask?.status === 'succeeded' && dubbingTask.audio_url?.trim()
      ? dubbingTask.audio_url
      : undefined
  const audioDurationSec =
    audioUrl && dubbingTask && dubbingTask.duration_sec > 0
      ? dubbingTask.duration_sec
      : undefined

  if (video && audioUrl) {
    return {
      status: isDubbingStaleForStoryboard(dubbingTask, storyboardUpdatedAt) ? 'dubbing_stale' : 'ready',
      video,
      audioUrl,
      audioDurationSec,
    }
  }
  if (video) {
    return { status: 'video_only', video }
  }
  if (audioUrl) {
    return { status: 'audio_only', audioUrl, audioDurationSec }
  }
  return { status: 'none' }
}

export function formatAvDurationDriftHint(
  videoDurationSec?: number,
  audioDurationSec?: number,
): string | null {
  if (!videoDurationSec || !audioDurationSec) return null
  const diff = videoDurationSec - audioDurationSec
  if (diff > 0.5) {
    return `配音较短，合成时画面可能延长约 ${diff.toFixed(1)} 秒`
  }
  if (diff < -0.5) {
    return `配音较长，合成时可能加速或裁切约 ${Math.abs(diff).toFixed(1)} 秒`
  }
  return null
}

export type ClipPreviewPayload = {
  videoUrl: string
  audioUrl?: string
  videoDurationSec?: number
  audioDurationSec?: number
  title?: string
  staleHint?: string | null
}

export function buildClipPreviewFromTask({
  clipUrl,
  clipOrder,
  clipDurationSec,
  renderConfig,
  dubbingTask,
  storyboardUpdatedAt,
  title,
}: {
  clipUrl: string
  clipOrder: number
  clipDurationSec?: number
  renderConfig?: Record<string, unknown>
  dubbingTask?: DubbingTask
  storyboardUpdatedAt?: string
  title?: string
}): ClipPreviewPayload {
  const storyboardId = resolveStoryboardIdForClip(renderConfig, clipOrder)
  const audioUrl =
    dubbingTask?.status === 'succeeded' && dubbingTask.audio_url?.trim()
      ? dubbingTask.audio_url
      : undefined
  const audioDurationSec =
    audioUrl && dubbingTask && dubbingTask.duration_sec > 0
      ? dubbingTask.duration_sec
      : undefined
  const staleHint =
    audioUrl && isDubbingStaleForStoryboard(dubbingTask, storyboardUpdatedAt)
      ? '分镜台词已更新，当前配音可能不是最新版本'
      : null

  return {
    videoUrl: clipUrl,
    audioUrl,
    videoDurationSec: clipDurationSec && clipDurationSec > 0 ? clipDurationSec : undefined,
    audioDurationSec,
    title: title ?? (storyboardId ? `分镜 #${storyboardId}` : undefined),
    staleHint,
  }
}
