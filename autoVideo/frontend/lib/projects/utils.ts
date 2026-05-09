import { format } from 'date-fns'

export function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${(bytes / Math.pow(k, i)).toFixed(1)} ${sizes[i]}`
}

export function formatDuration(seconds: number): string {
  const m = Math.floor(seconds / 60)
  const s = Math.round(seconds % 60)
  return `${m}:${s.toString().padStart(2, '0')}`
}

export function parseTimestampMs(value?: string): number | null {
  if (!value) return null
  const ms = new Date(value).getTime()
  return Number.isNaN(ms) ? null : ms
}

export function formatRuntimeDuration(ms: number): string {
  const totalSeconds = Math.max(0, Math.round(ms / 1000))
  if (totalSeconds < 60) return `${totalSeconds} 秒`
  if (totalSeconds < 3600) {
    const minutes = Math.floor(totalSeconds / 60)
    const seconds = totalSeconds % 60
    return `${minutes} 分 ${seconds.toString().padStart(2, '0')} 秒`
  }
  const hours = Math.floor(totalSeconds / 3600)
  const minutes = Math.floor((totalSeconds % 3600) / 60)
  return `${hours} 小时 ${minutes} 分`
}

export function getElapsedTimeLabel(startedAt: string | undefined, nowMs: number): string | null {
  const startedAtMs = parseTimestampMs(startedAt)
  if (startedAtMs === null) return null
  return `已用时 ${formatRuntimeDuration(Math.max(0, nowMs - startedAtMs))}`
}

export function getEstimatedRemainingLabel(startedAt: string | undefined, ratio: number, nowMs: number): string | null {
  const startedAtMs = parseTimestampMs(startedAt)
  if (startedAtMs === null || ratio >= 1) return null

  const elapsedMs = Math.max(0, nowMs - startedAtMs)
  if (elapsedMs < 4000 || ratio < 0.12) return '预计还需片刻'

  const totalMs = elapsedMs / ratio
  const remainingMs = Math.max(0, totalMs - elapsedMs)
  if (!Number.isFinite(remainingMs) || remainingMs <= 0) return null
  return `预计还需 ${formatRuntimeDuration(remainingMs)}`
}

export function getTimingSummary(startedAt: string | undefined, ratio: number, nowMs: number): string | null {
  const parts = [
    getElapsedTimeLabel(startedAt, nowMs),
    getEstimatedRemainingLabel(startedAt, ratio, nowMs),
  ].filter(Boolean)

  if (parts.length === 0) return null
  return parts.join(' · ')
}

export function getEarliestTimestamp(values: Array<string | undefined>): string | undefined {
  const timestamps = values
    .map((value) => ({ value, ms: parseTimestampMs(value) }))
    .filter((item): item is { value: string; ms: number } => Boolean(item.value) && item.ms !== null)
    .sort((a, b) => a.ms - b.ms)

  return timestamps[0]?.value
}

export const SCRIPT_PROGRESS_STALL_MS = 2 * 60 * 1000
export const TASK_PROGRESS_STALL_MS = 2 * 60 * 1000

export function getProgressStallMeta(updatedAt?: string, thresholdMs = TASK_PROGRESS_STALL_MS) {
  if (!updatedAt) return null
  const updatedAtMs = new Date(updatedAt).getTime()
  if (Number.isNaN(updatedAtMs)) return null
  const elapsedMs = Date.now() - updatedAtMs
  if (elapsedMs < thresholdMs) return null
  const elapsedMinutes = Math.max(1, Math.floor(elapsedMs / 60000))
  return {
    elapsedMs,
    label: `已超过 ${elapsedMinutes} 分钟无进展更新`,
  }
}

export function getPendingQueueMeta(updatedAt?: string, thresholdMs = TASK_PROGRESS_STALL_MS) {
  if (!updatedAt) return null
  const updatedAtMs = new Date(updatedAt).getTime()
  if (Number.isNaN(updatedAtMs)) return null
  const elapsedMs = Date.now() - updatedAtMs
  if (elapsedMs < thresholdMs) return null
  const elapsedMinutes = Math.max(1, Math.floor(elapsedMs / 60000))
  return {
    elapsedMs,
    label: `已排队 ${elapsedMinutes} 分钟`,
  }
}

export function formatChatTimestamp(value?: string) {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  return format(date, 'MM-dd HH:mm')
}
