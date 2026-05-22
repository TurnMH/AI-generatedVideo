import type { AssetLike, AdVideoDraftSnapshot, AdVideoHistoryEntry } from '@/components/ad-video/types'
import {
  AD_VIDEO_DRAFT_STORAGE_KEY,
  AD_VIDEO_HISTORY_STORAGE_KEY,
  BRAND_VOICE_TEMPLATES,
  CREATIVE_MODE_OPTIONS,
  SUBTITLE_LANGUAGE_OPTIONS,
  TARGET_MARKET_OPTIONS,
} from '@/components/ad-video/constants'

export function parseLines(raw: string): string[] {
  return raw
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean)
}

export function isHttpUrl(url: string): boolean {
  return /^https?:\/\//i.test(url)
}

export function isSupportedVideoFile(file: File): boolean {
  const mime = String(file.type || '').toLowerCase()
  if (mime.startsWith('video/')) return true
  const name = String(file.name || '').toLowerCase()
  return /\.(mp4|mov|m4v|webm|mkv|avi)$/i.test(name)
}

export function splitSubtitleScript(raw: string): string[] {
  const normalized = String(raw || '').replace(/\r\n/g, '\n').trim()
  if (!normalized) return []

  const directLines = normalized
    .split('\n')
    .map((line) => line.replace(/^\s*(?:\d+[.)]|[-•*])\s*/, '').trim())
    .filter(Boolean)

  if (directLines.length > 0) return directLines

  return normalized
    .split(/[。！？!?；;]+/)
    .map((line) => line.replace(/^\s*(?:\d+[.)]|[-•*])\s*/, '').trim())
    .filter(Boolean)
}

export function splitEditableLines(raw: string): string[] {
  return String(raw || '').replace(/\r\n/g, '\n').split('\n')
}

export function updateLineAtIndex(raw: string, index: number, nextValue: string): string {
  const lines = splitEditableLines(raw)
  while (lines.length <= index) lines.push('')
  lines[index] = nextValue
  return lines.join('\n')
}

export function normalizeEditableLine(raw: string): string {
  return String(raw || '')
    .replace(/^\s*(?:\d+[.)]|[-•*])\s*/, '')
    .trim()
}

export function countEditableSlots(lines: readonly string[]): number {
  return lines.some((line) => normalizeEditableLine(line).length > 0) ? lines.length : 0
}

export function distributeDialogues(lines: readonly string[], clipCount: number): string[] {
  if (clipCount <= 0) return []

  const normalized = lines.map((line) => line.trim()).filter(Boolean)
  if (normalized.length === 0) return Array.from({ length: clipCount }, () => '')
  if (normalized.length === 1) return Array.from({ length: clipCount }, () => normalized[0])

  const result: string[] = []
  for (let index = 0; index < clipCount; index += 1) {
    const start = Math.floor((index * normalized.length) / clipCount)
    const end = Math.max(start + 1, Math.floor(((index + 1) * normalized.length) / clipCount))
    const chunk = normalized.slice(start, end)
    result.push(chunk.join(' ').trim() || normalized[Math.min(start, normalized.length - 1)] || '')
  }
  return result
}

export function buildMarketDirective(
  marketKey: string,
  subtitleLanguageKey: string,
  creativeModeKey: string,
  directorNote: string,
  brandVoiceKey: string,
  brandVoiceNotes: string,
): string {
  const marketOption = TARGET_MARKET_OPTIONS.find((item) => item.key === marketKey) ?? TARGET_MARKET_OPTIONS[0]
  const subtitleLanguageOption = SUBTITLE_LANGUAGE_OPTIONS.find((item) => item.key === subtitleLanguageKey) ?? SUBTITLE_LANGUAGE_OPTIONS[0]
  const creativeModeOption = CREATIVE_MODE_OPTIONS.find((item) => item.key === creativeModeKey) ?? CREATIVE_MODE_OPTIONS[0]
  const brandVoiceOption = BRAND_VOICE_TEMPLATES.find((item) => item.key === brandVoiceKey) ?? BRAND_VOICE_TEMPLATES[0]
  const note = directorNote.trim()
  const voiceNote = brandVoiceNotes.trim()

  return [
    `目标市场：${marketOption.label}`,
    marketOption.prompt,
    `字幕语言：${subtitleLanguageOption.label}`,
    subtitleLanguageOption.prompt,
    `创意模式：${creativeModeOption.label}`,
    creativeModeOption.prompt,
    `品牌语气：${brandVoiceOption.label}`,
    brandVoiceOption.directive,
    brandVoiceOption.contrast,
    voiceNote ? `品牌语气补充：${voiceNote}` : '',
    note ? `导演备注：${note}` : '',
    '要求：字幕、口播和镜头说明要一一对应，保持品牌卖点，不要把本地市场脚本自动改写成泛化风格。',
  ].filter(Boolean).join('\n')
}

export function getTargetMarketLabelSafe(marketKey: string): string {
  return TARGET_MARKET_OPTIONS.find((item) => item.key === marketKey)?.label ?? TARGET_MARKET_OPTIONS[0].label
}

export function readAdVideoDraft(): { savedAt: string | null; state: Partial<AdVideoDraftSnapshot> | null } {
  if (typeof window === 'undefined') return { savedAt: null, state: null }
  try {
    const raw = window.localStorage.getItem(AD_VIDEO_DRAFT_STORAGE_KEY)
    if (!raw) return { savedAt: null, state: null }
    const parsed = JSON.parse(raw) as { savedAt?: string; state?: Partial<AdVideoDraftSnapshot> }
    return {
      savedAt: typeof parsed.savedAt === 'string' ? parsed.savedAt : null,
      state: parsed && typeof parsed === 'object' ? (parsed.state ?? null) : null,
    }
  } catch {
    return { savedAt: null, state: null }
  }
}

export function writeAdVideoDraft(state: AdVideoDraftSnapshot): string {
  const savedAt = new Date().toISOString()
  if (typeof window !== 'undefined') {
    window.localStorage.setItem(AD_VIDEO_DRAFT_STORAGE_KEY, JSON.stringify({ savedAt, state }))
  }
  return savedAt
}

export function readAdVideoHistory(): AdVideoHistoryEntry[] {
  if (typeof window === 'undefined') return []
  try {
    const raw = window.localStorage.getItem(AD_VIDEO_HISTORY_STORAGE_KEY)
    if (!raw) return []
    const parsed = JSON.parse(raw)
    return Array.isArray(parsed) ? parsed : []
  } catch {
    return []
  }
}

export function writeAdVideoHistory(entries: AdVideoHistoryEntry[]) {
  if (typeof window !== 'undefined') {
    window.localStorage.setItem(AD_VIDEO_HISTORY_STORAGE_KEY, JSON.stringify(entries))
  }
}

export function clearAdVideoDraft() {
  if (typeof window !== 'undefined') {
    window.localStorage.removeItem(AD_VIDEO_DRAFT_STORAGE_KEY)
  }
}

export function normalizeFailureReason(raw?: string): string {
  const message = String(raw || '').trim()
  if (!message) return '生成失败'
  return message
    .replace(/^error:\s*/i, '')
    .replace(/_/g, ' ')
    .trim()
}

export function percentile(sortedValues: number[], p: number): number {
  if (sortedValues.length === 0) return 0
  const idx = Math.min(sortedValues.length - 1, Math.max(0, Math.floor((sortedValues.length - 1) * p)))
  return sortedValues[idx] ?? 0
}

export function estimateCostFactor(modelName: string): number {
  const name = modelName.toLowerCase()
  if (name.includes('kling') || name.includes('veo')) return 1.6
  if (name.includes('wan') || name.includes('vidu')) return 1.25
  return 1
}

export function normalizeImageUrlFromAsset(asset: AssetLike): string {
  if (!asset) return ''

  const metadata = asset.metadata && typeof asset.metadata === 'object'
    ? asset.metadata as Record<string, unknown>
    : null

  return String(
    asset.image_url
      || asset.composite_image_url
      || asset.url
      || asset.file_url
      || asset.thumbnail_url
      || metadata?.image_url
      || metadata?.file_url
      || metadata?.thumbnail_url
      || '',
  ).trim()
}
