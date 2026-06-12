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

const NAME_ONLY = /^[\u4e00-\u9fffA-Za-z·]{1,8}[。！?？]?$/u

const ACTION_ONLY =
  /^[\u4e00-\u9fff]{1,8}(?:缓缓|慢慢|轻轻|忽然|猛然|转身|抬头|低头|走|跑|拿|放|推|拉|看|望|站|坐|蹲|靠|握|举|切|揉|炒|煮|递|接|挥|指|叹|笑|哭|愣|震|顿|沉默|专注).+[。！]?$/u

const TIME_TRANSITION =
  /^(?:\d+年[前后]?|\d+个?月[前后]?|三天后|翌日|次日|同时|此时|那一刻|三个月前|一年前|数日后|片刻后)[。！]?$/u

const VISUAL_KEYWORDS =
  /画面|构图|近景|中景|远景|景别|空镜|机位|运镜|特写|环境光线|背景简洁|神情|面露|身穿|穿着|身形对比|视觉/gu

export type StoryboardVoiceRole = 'narrator' | 'character' | 'mixed'

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
  if (TIME_TRANSITION.test(trimmed)) return true
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

/** Pull speakable narration from mixed storyboard dialogue (strip visual/staging text). */
export function extractStoryboardSpeechText(
  storyboard: Pick<Storyboard, 'dialogue' | 'characters'>,
  options?: { isCommentary?: boolean },
): string {
  const raw = (storyboard.dialogue || '').trim()
  if (!raw) return ''

  const subtitleNarration = extractSubtitleTagNarration(raw)
  if (subtitleNarration) return subtitleNarration

  const quoted = extractQuotedSpeechLines(raw)
  if (quoted) return quoted

  if (options?.isCommentary) {
    const narrationLines = raw
      .replace(/\r\n?/g, '\n')
      .split('\n')
      .map((line) => line.trim())
      .filter((line) => line && !/^(?:【\s*)?(?:内景|外景|内外景)/u.test(line))
      .map((line) => line.replace(/^[\u3400-\u4dbf\u4e00-\u9fffA-Za-z·]{1,8}[（(][^)）]{0,24}[）)]\s*/, '').trim())
      .filter((line) => line.length >= 10 && !looksLikeStoryboardVisualDescription(line))
    if (narrationLines.length > 0) {
      return narrationLines.join('\n').trim()
    }
  }

  const parts: string[] = []
  for (const rawLine of raw.replace(/\r\n?/g, '\n').split('\n')) {
    const line = rawLine.trim()
    if (!line) continue

    const speakerMatch = line.match(SPEAKER_LINE)
    if (speakerMatch?.[1] && speakerMatch?.[2]) {
      const speaker = speakerMatch[1].trim()
      const content = speakerMatch[2].trim()
      if (!content || looksLikeSpeakerVisualStaging(content)) continue
      parts.push(`${speaker}：${content}`)
      continue
    }

    if (looksLikeStoryboardVisualDescription(line)) continue
    if ([...line].length >= 6 && !looksLikeSceneDescription(line)) {
      parts.push(line)
    }
  }

  return parts.join('\n').trim()
}

export function hasSpeakableStoryboardText(
  storyboard: Pick<Storyboard, 'dialogue' | 'characters'>,
  options?: { isCommentary?: boolean },
): boolean {
  return extractStoryboardSpeechText(storyboard, options).length > 0
}

export function detectStoryboardVoiceRole(
  storyboard: Pick<Storyboard, 'dialogue' | 'characters'>,
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

/** Format storyboard dialogue for TTS with explicit speaker labels. */
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

export function formatStoryboardDubbingText(
  storyboard: Pick<Storyboard, 'dialogue' | 'characters'>,
  options?: { isCommentary?: boolean },
): string {
  const extracted = extractStoryboardSpeechText(storyboard, options)
  if (!extracted) return ''
  const normalized = normalizeMislabeledNarrationSpeakers(extracted)
  if (hasExplicitSpeakerLabel(normalized)) return normalized

  const chars = (storyboard.characters || []).map((name) => name.trim()).filter(Boolean)
  if (!options?.isCommentary && chars.length === 1) {
    return `${chars[0]}：${normalized}`
  }
  return `旁白：${normalized}`
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
