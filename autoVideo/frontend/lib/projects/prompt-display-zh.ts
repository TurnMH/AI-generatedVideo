import { utilsAPI } from '@/lib/api'
import type { Asset, Storyboard } from '@/types'

const zhDisplayCache = new Map<string, string>()

export function hasEnoughChinese(text: string): boolean {
  const han = (text.match(/[\u4e00-\u9fff]/g) || []).length
  if (han >= 12) return true
  const compact = text.replace(/\s/g, '')
  return compact.length > 0 && han / compact.length >= 0.12
}

/** 文本主体是否为中文（排除「英文 prompt + 少量中文人名」的混排） */
export function isPrimarilyChinese(text: string): boolean {
  const trimmed = text.trim()
  if (!trimmed) return false
  const han = (trimmed.match(/[\u4e00-\u9fff]/g) || []).length
  const latin = (trimmed.match(/[a-zA-Z]/g) || []).length
  if (han === 0) return false
  if (latin > han) return false
  const compact = trimmed.replace(/\s/g, '')
  return compact.length > 0 && han / compact.length >= 0.18
}

function extractTranslated(res: unknown): { text: string; failed: boolean } {
  if (!res || typeof res !== 'object') return { text: '', failed: true }
  const root = res as Record<string, unknown>
  const warning = typeof root.warning === 'string' ? root.warning : ''
  if (root.data && typeof root.data === 'object') {
    const nested = root.data as Record<string, unknown>
    const nestedWarning = typeof nested.warning === 'string' ? nested.warning : warning
    if (typeof nested.translated === 'string') {
      return { text: nested.translated.trim(), failed: Boolean(nestedWarning) }
    }
  }
  if (typeof root.translated === 'string') {
    return { text: root.translated.trim(), failed: Boolean(warning) }
  }
  return { text: '', failed: true }
}

function translationLooksValid(source: string, translated: string, failed: boolean): boolean {
  if (failed || !translated) return false
  if (translated === source.trim()) return isPrimarilyChinese(translated)
  return isPrimarilyChinese(translated)
}


export function pickEditableChinesePrompt(
  sceneDescription?: string,
  promptUsed?: string,
): string {
  const scene = (sceneDescription || '').trim()
  const used = (promptUsed || '').trim()
  if (used && isPrimarilyChinese(used)) return used
  return scene
}

type StoryboardSummaryInput = Pick<
  Storyboard,
  | 'scene_description'
  | 'characters'
  | 'location'
  | 'location_zone'
  | 'mood'
  | 'dialogue'
  | 'spatial_anchor'
  | 'subject_positions'
  | 'transition_note'
  | 'aspect_ratio'
>

type AssetSummaryInput = Pick<Asset, 'name' | 'type' | 'description'>

const ASSET_TYPE_LABELS: Record<string, string> = {
  character: '人物',
  scene: '场景',
  prop: '物品',
}

/** 分镜成图提示词的中文摘要（翻译 API 不可用时的兜底展示） */
export function buildStoryboardPromptSummaryZh(
  sb: StoryboardSummaryInput,
  linkedAssets?: AssetSummaryInput[],
): string {
  const lines: string[] = []
  if (sb.scene_description?.trim()) lines.push(`【主要画面】${sb.scene_description.trim()}`)
  if (sb.characters?.length) lines.push(`【出场人物】${sb.characters.join('、')}`)
  if (linkedAssets?.length) {
    for (const asset of linkedAssets) {
      const desc = asset.description?.trim()
      if (!desc) continue
      const typeLabel = ASSET_TYPE_LABELS[asset.type] ?? '资源'
      const name = asset.name?.trim() || typeLabel
      lines.push(`【${typeLabel}锁定·${name}】${desc}`)
    }
  }
  if (sb.location?.trim()) lines.push(`【场景地点】${sb.location.trim()}`)
  if (sb.location_zone?.trim()) lines.push(`【空间视角】${sb.location_zone.trim()}`)
  if (sb.spatial_anchor?.trim()) lines.push(`【空间锚点】${sb.spatial_anchor.trim()}`)
  if (sb.subject_positions?.trim()) lines.push(`【主体站位】${sb.subject_positions.trim()}`)
  if (sb.mood?.trim()) lines.push(`【情绪氛围】${sb.mood.trim()}`)
  if (sb.dialogue?.trim()) lines.push(`【台词参考】${sb.dialogue.trim()}`)
  if (sb.transition_note?.trim()) lines.push(`【转场说明】${sb.transition_note.trim()}`)
  return lines.join('\n')
}

/** 资源成图提示词的中文摘要（翻译 API 不可用时的兜底展示） */
export function buildAssetPromptSummaryZh(asset: Pick<Asset, 'description' | 'name' | 'type'>): string {
  const typeLabel = asset.type === 'character' ? '人物' : asset.type === 'scene' ? '场景' : asset.type === 'prop' ? '物品' : '资源'
  const lines: string[] = []
  if (asset.name?.trim()) lines.push(`【${typeLabel}名称】${asset.name.trim()}`)
  if (asset.description?.trim()) lines.push(`【视觉描述】${asset.description.trim()}`)
  return lines.join('\n')
}

export type PromptChineseDisplayResult = {
  text: string
  /** true 表示来自 LLM 翻译；false 表示本地摘要或原文 */
  fromTranslation: boolean
}

/** 展示用中文：优先摘要，翻译结果须通过 isPrimarilyChinese 校验。 */
export async function fetchPromptChineseDisplay(
  text: string,
  fallback?: () => string,
): Promise<PromptChineseDisplayResult> {
  const key = text.trim()
  const fb = fallback?.() ?? ''
  if (!key) return { text: fb, fromTranslation: false }
  if (isPrimarilyChinese(key)) return { text: key, fromTranslation: false }

  const cacheKey = `zh:v4:${key}`
  if (zhDisplayCache.has(cacheKey)) {
    const cached = zhDisplayCache.get(cacheKey)!
    if (isPrimarilyChinese(cached)) {
      return { text: cached, fromTranslation: Boolean(fb && cached !== fb) }
    }
    return { text: fb || cached, fromTranslation: false }
  }

  try {
    const res = await utilsAPI.translatePromptDisplay(key, 'zh')
    const { text: translated, failed } = extractTranslated(res)
    if (translationLooksValid(key, translated, failed)) {
      zhDisplayCache.set(cacheKey, translated)
      return { text: translated, fromTranslation: true }
    }
  } catch {
    // fall through to fallback
  }

  if (fb) {
    zhDisplayCache.set(cacheKey, fb)
    return { text: fb, fromTranslation: false }
  }
  return { text: key, fromTranslation: false }
}
