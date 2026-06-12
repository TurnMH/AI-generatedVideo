import type { DubbingTask } from '@/lib/api'
import type { Storyboard } from '@/types'

/** Breathing room after narration so clip does not cut off the last syllable. */
export const CLIP_DURATION_NARRATION_PAD_SEC = 0.35

const MODEL_SNAP: Record<string, number[]> = {
  kling: [5, 10],
  wan: [5],
  vidu: [4, 8],
  veo: [4, 6, 8],
  doubao: [5, 8, 10],
  suanneng: [5, 8, 10],
}

export function normalizeVideoModelFamily(modelKey: string): string {
  const key = (modelKey || '').trim().toLowerCase()
  if (!key) return 'default'
  if (key === 'aiping' || key.includes('kling') || key.includes('tencent-vclm') || key.includes('hubagi-tc')) {
    return 'kling'
  }
  if (key.startsWith('vidu')) return 'vidu'
  if (key.includes('doubao') || key.includes('seedance') || key === 'suanneng') return 'doubao'
  if (key.includes('veo') || key.includes('voe') || key === 'sora2') return 'veo'
  if (key === 'wan') return 'wan'
  return 'default'
}

export function snapClipDurationToModel(seconds: number, modelKey: string): number {
  const clamped = Math.min(20, Math.max(3, seconds))
  const family = normalizeVideoModelFamily(modelKey)
  const allowed = MODEL_SNAP[family]
  if (!allowed || allowed.length === 0) return clamped

  let best = allowed[0]
  for (const candidate of allowed) {
    if (Math.abs(candidate - clamped) < Math.abs(best - clamped)) {
      best = candidate
    }
  }
  return best
}

export function resolveStoryboardClipDurationSec(
  storyboard: Pick<Storyboard, 'id' | 'duration'>,
  options: {
    modelKey: string
    defaultDuration: number
    dubbingTask?: Pick<DubbingTask, 'duration_sec' | 'status' | 'task_type'>
  },
): number {
  const fallback = storyboard.duration > 0 ? storyboard.duration : options.defaultDuration
  const dub = options.dubbingTask
  const fromDubbing =
    dub?.task_type === 'dubbing'
    && dub.status === 'succeeded'
    && dub.duration_sec > 0
      ? dub.duration_sec + CLIP_DURATION_NARRATION_PAD_SEC
      : 0

  const raw = fromDubbing > 0 ? fromDubbing : fallback
  return snapClipDurationToModel(raw, options.modelKey)
}

export function resolveStoryboardClipDurations(
  storyboards: Array<Pick<Storyboard, 'id' | 'duration'>>,
  options: {
    modelKey: string
    defaultDuration: number
    dubbingTasksByStoryboardId?: Map<number, DubbingTask>
  },
): number[] {
  return storyboards.map((sb) =>
    resolveStoryboardClipDurationSec(sb, {
      modelKey: options.modelKey,
      defaultDuration: options.defaultDuration,
      dubbingTask: options.dubbingTasksByStoryboardId?.get(sb.id),
    }),
  )
}

export function countDubbingAlignedClipDurations(
  storyboards: Array<Pick<Storyboard, 'id'>>,
  dubbingTasksByStoryboardId?: Map<number, DubbingTask>,
): number {
  if (!dubbingTasksByStoryboardId) return 0
  return storyboards.filter((sb) => {
    const task = dubbingTasksByStoryboardId.get(sb.id)
    return task?.task_type === 'dubbing' && task.status === 'succeeded' && task.duration_sec > 0
  }).length
}
