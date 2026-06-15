import type { Storyboard } from '@/types'

const SPEAKER_LINE =
  /^(?:[【\[(（]\s*)?([^:：\]）)】]{1,24})(?:\s*[】\])）])?\s*[:：]\s*(.+)$/u

const NARRATOR_SPEAKER =
  /^(旁白|主持人|主播|解说|画外音|OS|VO|narrator)/iu

const SUBTITLE_TAG = /\[字幕[:：]\s*([^\]]+?)\s*\]/gu
const QUOTED_SPEECH = /[“「『"]([^”」』"]+)[”」』"]/gu

const SCENE_SLUG =
  /^(?:【\s*)?(?:内景|外景|内外景|INT\.?|EXT\.?)(?:\s*[·．.、，,/\|｜\-—–]\s*|\s+).+(?:\s*】)?$/u

const LOCATION_LEAD =
  /^[\u4e00-\u9fff]{2,}(?:楼|堂|馆|店|铺|院|房|室|厨|厅|街|巷|路|园|场|殿|宫|城|村|镇|山|河|湖|海|门|间|内|外|里|中|屋|居|庄|司|厂)[，,]/u

const SCENE_SETTING =
  /^(?:[\u3400-\u4dbf\u4e00-\u9fffA-Za-z·0-9]{2,30}[，,]\s*)?(?:清晨|早晨|早上|上午|中午|午后|傍晚|黄昏|夜里|夜晚|夜间|深夜|凌晨|日间|日出|日落)[，,]/u

const SCREENPLAY_ACTION =
  /^[\u4e00-\u9fffA-Za-z0-9·\s]{1,16}[（(][^)）]{0,30}[）)]\s*[\u4e00-\u9fffA-Za-z]/u

const VISUAL_KEYWORDS =
  /画面|构图|近景|中景|远景|景别|空镜|机位|运镜|特写|环境光线|背景简洁|神情|面露|身穿|穿着|身形对比|视觉/gu

const SPEAKER_BEFORE_QUOTE =
  /([\u4e00-\u9fffA-Za-z·]{2,8})(?:[（(][^)）]{0,16}[）)])?(?:[^"「『"]{0,32})?(?:说|喊|叫|问|答|回应|道|唤|开口|低声|轻声|沉声|冷声|怒|笑)[^"「『"]{0,12}[""「『"]/u

export type StoryboardVoiceRole = 'narrator' | 'character' | 'mixed'

const SPEECH_PACE_CHARS_PER_10_SEC: Record<string, number> = {
  normal: 48,
  slightly_fast: 56,
  with_pauses: 38,
  very_fast: 66,
  medium_fast: 52,
  medium_steady: 42,
}

/** Estimate speakable character budget from clip duration and speech pace. */
export function resolveSpeechMaxRunesForClip(durationSec: number, speechPace?: string): number {
  const duration = Math.max(3, Math.min(20, durationSec > 0 ? durationSec : 5))
  const pace = speechPace?.trim() || 'normal'
  const charsPer10Sec = SPEECH_PACE_CHARS_PER_10_SEC[pace] ?? 48
  return Math.max(16, Math.min(120, Math.round(charsPer10Sec * duration / 10)))
}

export function resolveStoryboardSpeechLimit(
  storyboard: Pick<Storyboard, 'duration'>,
  project?: { storyboard_config?: { duration?: number; speech_pace?: string } },
): number {
  const duration = storyboard.duration > 0
    ? storyboard.duration
    : (project?.storyboard_config?.duration || 5)
  return resolveSpeechMaxRunesForClip(duration, project?.storyboard_config?.speech_pace)
}

/** Join two storyboard dialogue fields for manual merge-up. */
export function joinStoryboardDialogue(a: string, b: string): string {
  const left = a.trim()
  const right = b.trim().replace(/^[，,]+/, '')
  if (!left) return right
  if (!right) return left
  if (/[。！？]$/.test(left)) return left + right
  if (/[，,]$/.test(left)) return left + right
  return `${left}，${right}`
}

export function countStoryboardDialogueRunes(text: string): number {
  return [...text.trim()].length
}

export type CharacterQuote = {
  speaker: string
  quote: string
}

export type StoryboardSpeechInput = Pick<Storyboard, 'dialogue' | 'characters' | 'scene_description' | 'duration'>

export function hasExplicitSpeakerLabel(text: string): boolean {
  const trimmed = text.trim()
  if (!trimmed) return false
  return trimmed.split(/\r?\n/).some((line) => {
    const candidate = line.trim()
    if (!candidate) return false
    const match = candidate.match(SPEAKER_LINE)
    if (!match?.[1] || !match?.[2]) return false
    const label = match[1].replace(/\s+/g, '')
    if (label.length > 16) return false
    return !/^[，。！？,!?;；/、]/.test(label)
  })
}

function looksLikeCompleteUtterance(text: string): boolean {
  const trimmed = text.trim()
  if (!trimmed) return false
  if (QUOTED_SPEECH.test(trimmed)) return true
  return /[。！？!?；;]/.test(trimmed) && [...trimmed].length >= 6
}

export function looksLikeSceneDescription(text: string): boolean {
  const trimmed = text.trim()
  if (!trimmed) return false
  if (SCENE_SLUG.test(trimmed)) return true
  if (SCENE_SETTING.test(trimmed) || LOCATION_LEAD.test(trimmed)) return true
  if (SCREENPLAY_ACTION.test(trimmed) && !SPEAKER_LINE.test(trimmed)) {
    return [...trimmed].length >= 12
  }
  return false
}

function looksLikeSpeakerVisualStaging(text: string): boolean {
  const trimmed = text.trim()
  if (!trimmed) return true

  const keywords = trimmed.match(VISUAL_KEYWORDS) ?? []
  if (keywords.length >= 2) return true
  if (trimmed.includes('内部，') && (trimmed.includes('神情') || trimmed.includes('表情') || trimmed.includes('光线'))) {
    return true
  }
  if (looksLikeCompleteUtterance(trimmed) && keywords.length === 0) return false
  if (SCREENPLAY_ACTION.test(trimmed) && !looksLikeCompleteUtterance(trimmed)) return true
  if (keywords.length === 1) return true
  return false
}

export function looksLikeStoryboardVisualDescription(text: string): boolean {
  const trimmed = text.trim()
  if (!trimmed) return false
  if (looksLikeSceneDescription(trimmed)) return true
  return looksLikeSpeakerVisualStaging(trimmed)
}

function extractSubtitleTagNarration(text: string): string {
  const parts = [...text.matchAll(SUBTITLE_TAG)]
    .map((match) => match[1]?.trim())
    .filter(Boolean) as string[]
  return parts.join('\n').trim()
}

function extractQuotedSpeechLines(text: string): string {
  const parts = [...text.matchAll(QUOTED_SPEECH)]
    .map((match) => match[1]?.trim())
    .filter(Boolean) as string[]
  return parts.join('\n').trim()
}

function normalizeCharacterName(name: string): string {
  return name.trim().replace(/^[\[(（【]+|[\])）】]+$/g, '').trim()
}

function pickFallbackSpeaker(characters: string[], quote: string): string {
  for (const name of characters) {
    const normalized = normalizeCharacterName(name)
    if (normalized && normalized !== quote) return normalized
  }
  return ''
}

function inferSpeakerBeforeQuote(before: string, characters: string[], quote: string): string {
  const trimmedBefore = before.trim()
  if (!trimmedBefore) return pickFallbackSpeaker(characters, quote)

  const speakerMatch = trimmedBefore.match(SPEAKER_BEFORE_QUOTE)
  if (speakerMatch?.[1]) {
    const speaker = normalizeCharacterName(speakerMatch[1])
    if (speaker && speaker !== quote) return speaker
  }

  let lastIdx = -1
  let lastName = ''
  for (const name of characters) {
    const normalized = normalizeCharacterName(name)
    if (!normalized || normalized === quote) continue
    const idx = trimmedBefore.lastIndexOf(normalized)
    if (idx > lastIdx) {
      lastIdx = idx
      lastName = normalized
    }
  }
  if (lastName) return lastName
  return pickFallbackSpeaker(characters, quote)
}

/** Extract quoted character lines from scene/action descriptions. */
export function extractCharacterQuotesFromScene(
  sceneText: string,
  characters: string[] = [],
): CharacterQuote[] {
  return extractCharacterQuotesFromText(sceneText, characters)
}

/** Extract quoted character lines from dialogue or scene text. */
export function extractCharacterQuotesFromText(
  text: string,
  characters: string[] = [],
): CharacterQuote[] {
  if (!text) return []

  const results: CharacterQuote[] = []
  const seen = new Set<string>()
  for (const match of text.matchAll(QUOTED_SPEECH)) {
    const quote = match[1]?.trim()
    if (!quote || quote.length > 120 || [...quote].length < 4) continue
    if (looksLikeStoryboardVisualDescription(quote) || looksLikeSceneDescription(quote)) continue

    const start = match.index ?? 0
    const before = text.slice(Math.max(0, start - 48), start)
    const speaker = inferSpeakerBeforeQuote(before, characters, quote)
    if (!speaker) continue

    const key = `${speaker}\0${quote}`
    if (seen.has(key)) continue
    seen.add(key)
    results.push({ speaker, quote })
  }
  return results
}

function escapeRegex(text: string): string {
  return text.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

function removeQuotedSpeechFromText(text: string, quotes: CharacterQuote[]): string {
  let out = text
  for (const { quote } of quotes) {
    const trimmed = quote.trim()
    if (!trimmed) continue
    const patterns = [
      `[“「『"]${escapeRegex(trimmed)}[”」』"]`,
      `"${escapeRegex(trimmed)}"`,
    ]
    for (const pattern of patterns) {
      out = out.replace(new RegExp(pattern, 'g'), ' ')
    }
  }
  return stripTrailingSpeechLead(out.replace(/\s{2,}/g, ' ').trim())
}

function stripTrailingSpeechLead(text: string): string {
  return text.replace(/(?:，|,)?(?:说|问|答|道|喊|叫)[：:]\s*$/u, '').trim()
}

function removeQuoteMentionsFromText(text: string, quotes: CharacterQuote[]): string {
  let out = text
  for (const { quote } of quotes) {
    const trimmed = quote.trim().replace(/[。！？!?]+$/u, '')
    if (!trimmed || [...trimmed].length < 4) continue
    out = out.replace(new RegExp(escapeRegex(trimmed) + `[。！？!?]?`, 'gu'), ' ')
  }
  return out.replace(/\s{2,}/g, ' ').trim()
}

const FIRST_PERSON_NARRATOR =
  /^(?:我|咱|没抬头|我正在|我坐直|我把|我看|我认出来|我老了|我给您|我没等)/

function inferDirectAddressSpeech(
  text: string,
  characters: string[],
  isCommentary?: boolean,
): CharacterQuote | null {
  if (!isCommentary || characters.length < 1) return null
  const trimmed = text.replace(/\r\n?/g, ' ').trim()
  if (!trimmed) return null
  const match = trimmed.match(/^([\u4e00-\u9fffA-Za-z·]{2,8})[，,]\s*(.+)$/u)
  if (!match?.[1] || !match?.[2]) return null
  if (FIRST_PERSON_NARRATOR.test(trimmed)) return null
  const addressee = normalizeCharacterName(match[1])
  const quote = match[2].trim()
  if (!quote || [...quote].length < 4) return null
  const normalizedChars = characters.map(normalizeCharacterName).filter(Boolean)
  if (normalizedChars.length === 1) {
    const sole = normalizedChars[0]
    if (sole && addressee && sole !== addressee && !sole.includes(addressee) && !addressee.includes(sole)) {
      return { speaker: sole, quote }
    }
    return null
  }
  if (!normalizedChars.some((name) => name === addressee || name.includes(addressee))) return null
  const speaker = normalizedChars.find((name) => name !== addressee && !name.includes(addressee) && !addressee.includes(name))
  if (!speaker) return null
  return { speaker, quote }
}

function dedupeSpeechLines(lines: string[]): string[] {
  const out: string[] = []
  const seen = new Set<string>()
  for (const line of lines) {
    const trimmed = line.trim()
    if (!trimmed || seen.has(trimmed)) continue
    seen.add(trimmed)
    out.push(trimmed)
  }
  return out
}

function normalizeSpeechKey(text: string): string {
  return text.replace(/[\s　“”「」『』"'']/gu, '')
}

function dedupeSpeechUnits(text: string): string {
  const units = text
    .replace(/\r\n?/g, '\n')
    .split(/[。！？!?；;\n]+/)
    .map((part) => part.trim())
    .filter(Boolean)
  const out: string[] = []
  const seen = new Set<string>()
  for (const unit of units) {
    const key = normalizeSpeechKey(unit)
    if (!key || seen.has(key)) continue
    seen.add(key)
    out.push(unit)
  }
  if (out.length === 0) return text.trim()
  if (out.length === 1) return out[0]
  return `${out.join('。')}。`
}

function firstSpeakableSentence(text: string): string {
  const trimmed = text.trim()
  if (!trimmed) return ''
  const match = trimmed.match(/^[^。！？!?；;\n]+[。！？!?]?/)
  return match?.[0]?.trim() || trimmed
}

function compactSingleSpeechBody(text: string, maxRunes: number): string {
  const trimmed = text.trim()
  if (!trimmed) return ''

  const subtitleParts = [...trimmed.matchAll(SUBTITLE_TAG)]
    .map((match) => match[1]?.trim())
    .filter(Boolean) as string[]
  let working = subtitleParts.length > 0 ? subtitleParts[0] : trimmed

  if ([...working].length > maxRunes) {
    const quoted = extractQuotedSpeechLines(working)
    if (quoted) working = quoted.split('\n')[0]?.trim() || working
  }

  working = dedupeSpeechUnits(working)
  if ([...working].length > maxRunes) {
    const slice = [...working].slice(0, maxRunes).join('')
    const cut = slice.lastIndexOf('，') >= maxRunes / 3 ? slice.slice(0, slice.lastIndexOf('，')) : slice
    working = `${cut.trim()}。`
  }
  return working.trim()
}

export function compactClipDialogue(text: string, maxRunes = 180): string {
  const trimmed = text.trim()
  if (!trimmed) return ''

  const lines = trimmed.replace(/\r\n?/g, '\n').split('\n').map((line) => line.trim()).filter(Boolean)
  if (lines.length > 1 || lines.some((line) => SPEAKER_LINE.test(line))) {
    const perLine = Math.max(12, Math.floor(maxRunes / Math.max(lines.length, 1)))
    return lines
      .map((line) => compactSingleSpeechBody(line, perLine))
      .filter(Boolean)
      .join('\n')
  }
  return compactSingleSpeechBody(trimmed, maxRunes)
}

function extractNarrationFromDialogue(raw: string, isCommentary?: boolean): string[] {
  const text = raw.trim()
  if (!text) return []

  const subtitleNarration = extractSubtitleTagNarration(text)
  if (subtitleNarration) {
    const lines = dedupeSpeechLines(subtitleNarration.split('\n').map((line) => line.trim()).filter(Boolean))
    if (isCommentary && lines.length > 1) return lines.slice(0, 1)
    return lines
  }

  if (isCommentary) {
    const narrationLines = text
      .replace(/\r\n?/g, '\n')
      .split('\n')
      .map((line) => line.trim())
      .filter((line) => line && !/^(?:【\s*)?(?:内景|外景|内外景)/u.test(line))
      .map((line) => line.replace(/^[\u3400-\u4dbf\u4e00-\u9fffA-Za-z·]{1,8}[（(][^)）]{0,24}[）)]\s*/, '').trim())
      .filter((line) => {
        if (!line) return false
        if (SPEAKER_LINE.test(line) && !NARRATOR_SPEAKER.test(line.match(SPEAKER_LINE)?.[1] ?? '')) return false
        return line.length >= 6 && !looksLikeStoryboardVisualDescription(line)
      })
    if (narrationLines.length > 0) {
      return dedupeSpeechLines(narrationLines).slice(0, 1)
    }
  }

  const parts: string[] = []
  for (const rawLine of text.replace(/\r\n?/g, '\n').split('\n')) {
    const line = rawLine.trim()
    if (!line) continue

    const speakerMatch = line.match(SPEAKER_LINE)
    if (speakerMatch?.[1] && speakerMatch?.[2]) {
      const speaker = speakerMatch[1].trim()
      const content = speakerMatch[2].trim()
      if (NARRATOR_SPEAKER.test(speaker.replace(/\s+/g, ''))) {
        if (content && !looksLikeSpeakerVisualStaging(content)) parts.push(content)
      }
      continue
    }

    if (looksLikeStoryboardVisualDescription(line)) continue
    if ([...line].length >= 6 && !looksLikeSceneDescription(line) && !extractQuotedSpeechLines(line)) {
      parts.push(line)
    }
  }
  return dedupeSpeechLines(parts)
}

function extractCharacterLinesFromDialogue(raw: string): string[] {
  const lines: string[] = []
  for (const rawLine of raw.replace(/\r\n?/g, '\n').split('\n')) {
    const line = rawLine.trim()
    if (!line) continue
    const speakerMatch = line.match(SPEAKER_LINE)
    if (!speakerMatch?.[1] || !speakerMatch?.[2]) continue
    const speaker = speakerMatch[1].trim()
    const content = speakerMatch[2].trim()
    if (!content || NARRATOR_SPEAKER.test(speaker.replace(/\s+/g, ''))) continue
    if (looksLikeSpeakerVisualStaging(content)) continue
    lines.push(`${speaker}：${content}`)
  }
  return dedupeSpeechLines(lines)
}

/** Pull speakable narration from mixed storyboard dialogue (strip visual/staging text). */
export function extractStoryboardSpeechText(
  storyboard: StoryboardSpeechInput,
  options?: { isCommentary?: boolean },
): string {
  return formatStoryboardDubbingText(storyboard, options)
}

export function hasSpeakableStoryboardText(
  storyboard: StoryboardSpeechInput,
  options?: { isCommentary?: boolean },
): boolean {
  return extractStoryboardSpeechText(storyboard, options).length > 0
}

export function detectStoryboardVoiceRole(
  storyboard: StoryboardSpeechInput,
): StoryboardVoiceRole {
  const text = extractStoryboardSpeechText(storyboard)
  if (!text) return 'narrator'

  const speakers = new Set<string>()
  for (const line of text.split(/\r?\n/)) {
    const match = line.trim().match(SPEAKER_LINE)
    if (!match?.[1]) continue
    speakers.add(match[1].replace(/\s+/g, ''))
  }
  if (speakers.size > 1) return 'mixed'
  if (speakers.size === 1) {
    const only = [...speakers][0]
    return NARRATOR_SPEAKER.test(only) ? 'narrator' : 'character'
  }

  const chars = (storyboard.characters || []).map((name) => name.trim()).filter(Boolean)
  if (chars.length === 1) return 'character'
  return 'narrator'
}

function shouldRelabelAsNarrator(speaker: string, content: string): boolean {
  const normalizedSpeaker = speaker.replace(/\s+/g, '')
  const trimmedContent = content.trim()
  if (!normalizedSpeaker || !trimmedContent) return false
  if (NARRATOR_SPEAKER.test(normalizedSpeaker)) return false
  if (/[你我咱]/.test(trimmedContent) && !trimmedContent.startsWith(normalizedSpeaker) &&
    !looksLikeStoryboardVisualDescription(trimmedContent) && !looksLikeSceneDescription(trimmedContent)) {
    return false
  }
  if (QUOTED_SPEECH.test(trimmedContent)) return false
  if (trimmedContent.startsWith(normalizedSpeaker)) return true
  if (looksLikeStoryboardVisualDescription(trimmedContent) || looksLikeSceneDescription(trimmedContent)) return true
  if (/[（(]/.test(normalizedSpeaker)) return true
  return false
}

function normalizeMislabeledNarrationSpeakers(text: string): string {
  const trimmed = text.trim()
  if (!trimmed) return ''
  return trimmed
    .replace(/\r\n?/g, '\n')
    .split('\n')
    .map((rawLine) => {
      const line = rawLine.trim()
      if (!line) return ''
      const match = line.match(SPEAKER_LINE)
      if (!match?.[1] || !match?.[2]) return line
      const speaker = match[1].trim()
      const content = match[2].trim()
      return shouldRelabelAsNarrator(speaker, content) ? `旁白：${content}` : line
    })
    .filter(Boolean)
    .join('\n')
}

/** Format storyboard dialogue for TTS: narration as 旁白, quoted character lines with speaker labels. */
export function formatStoryboardDubbingText(
  storyboard: StoryboardSpeechInput,
  options?: { isCommentary?: boolean; maxRunes?: number; project?: { storyboard_config?: { duration?: number; speech_pace?: string } } },
): string {
  const maxRunes = options?.maxRunes ?? resolveStoryboardSpeechLimit(storyboard, options?.project)
  const chars = (storyboard.characters || []).map((name) => name.trim()).filter(Boolean)
  const rawDialogue = (storyboard.dialogue || '').trim()
  const directSpeech = inferDirectAddressSpeech(rawDialogue, chars, options?.isCommentary)

  const quoteMap = new Map<string, CharacterQuote>()
  for (const quote of [
    ...extractCharacterQuotesFromText(rawDialogue, chars),
    ...extractCharacterQuotesFromScene(storyboard.scene_description || '', chars),
  ]) {
    quoteMap.set(`${quote.speaker}\0${quote.quote}`, quote)
  }
  const embeddedQuotes = [...quoteMap.values()]

  const narrationSource = directSpeech
    ? ''
    : removeQuoteMentionsFromText(
        removeQuotedSpeechFromText(rawDialogue, embeddedQuotes),
        embeddedQuotes,
      )
  const parts: string[] = []

  const narrationLines = extractNarrationFromDialogue(narrationSource, options?.isCommentary)
  for (const line of narrationLines) {
    parts.push(`旁白：${line}`)
  }

  const dialogueCharacterLines = extractCharacterLinesFromDialogue(rawDialogue)
  const sceneQuotes = embeddedQuotes

  const characterLines = [...dialogueCharacterLines]
  if (directSpeech) {
    characterLines.push(`${directSpeech.speaker}：${directSpeech.quote}`)
  }
  for (const { speaker, quote } of sceneQuotes) {
    const labeled = `${speaker}：${quote}`
    if (characterLines.includes(labeled)) continue
    characterLines.push(labeled)
  }

  parts.push(...characterLines)

  let result = ''
  if (parts.length === 0) {
    const quoted = extractQuotedSpeechLines(rawDialogue)
    if (quoted) {
      if (!options?.isCommentary && chars.length === 1) result = `${chars[0]}：${quoted}`
      else result = `旁白：${quoted}`
    }
  } else {
    result = normalizeMislabeledNarrationSpeakers(parts.join('\n'))
  }
  if (options?.isCommentary && maxRunes <= 120 && !result.includes('\n') && result.startsWith('旁白：')) {
    const narratorOnly = result.replace(/^旁白：/, '')
    const first = firstSpeakableSentence(narratorOnly)
    if (first) result = `旁白：${first}`
  }
  return compactClipDialogue(result, maxRunes)
}

/** Remove routing labels like "旁白：" / "角色名：" before video native audio or subtitles. */
export function stripSpeakerLabelsForSpeech(text: string): string {
  const trimmed = text.trim()
  if (!trimmed) return ''

  const parts: string[] = []
  for (const rawLine of trimmed.replace(/\r\n?/g, '\n').split('\n')) {
    const line = rawLine.trim()
    if (!line) continue
    const match = line.match(SPEAKER_LINE)
    if (match?.[1] && match?.[2]) {
      const speaker = match[1].trim().replace(/\s+/g, '')
      const content = match[2].trim()
      if (content && (NARRATOR_SPEAKER.test(speaker) || speaker.length <= 16)) {
        parts.push(content)
        continue
      }
    }
    parts.push(line)
  }
  return parts.join('\n').trim()
}

/** Compact speakable text for video generation without routing labels. */
export function formatStoryboardSpeechForVideo(
  storyboard: StoryboardSpeechInput & Pick<Storyboard, 'duration'>,
  options?: {
    isCommentary?: boolean
    maxRunes?: number
    project?: { storyboard_config?: { duration?: number; speech_pace?: string } }
  },
): string {
  const maxRunes = options?.maxRunes ?? resolveStoryboardSpeechLimit(storyboard, options?.project)
  return stripSpeakerLabelsForSpeech(formatStoryboardDubbingText(storyboard, { isCommentary: options?.isCommentary, maxRunes }))
}

export function getStoryboardVoiceRoleLabel(role: StoryboardVoiceRole): string {
  if (role === 'character') return '角色'
  if (role === 'mixed') return '多角色'
  return '旁白'
}

export function normalizeStoryboardTaskList(
  payload: unknown,
): Array<{ task: import('@/lib/api').DubbingTask; voice_debug_available?: boolean }> {
  if (!Array.isArray(payload)) return []
  return payload
    .map((item) => {
      if (!item || typeof item !== 'object') return null
      const record = item as { task?: import('@/lib/api').DubbingTask }
      if (record.task?.id) return record as { task: import('@/lib/api').DubbingTask }
      const asTask = item as import('@/lib/api').DubbingTask
      if (asTask.id && asTask.storyboard_id != null) {
        return { task: asTask }
      }
      return null
    })
    .filter(Boolean) as Array<{ task: import('@/lib/api').DubbingTask }>
}
