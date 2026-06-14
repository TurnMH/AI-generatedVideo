import type { Storyboard } from '@/lib/api'

type StoryboardVideoPromptInput = Pick<Storyboard, 'scene_description' | 'dialogue' | 'prompt_used'>

const ACTION_TAG = /\[动作[:：]\s*([^\]]+?)\s*\]/gu
const APPEARANCE_NOISE =
  /(?:身穿|身着|穿着|发型|黑发|花白|脸型|圆润|商人气息|环境光线|背景简洁|神情|面露|身形对比|气氛紧张|近景突出|远景)/u

const ACTION_VERBS =
  /(?:揉|站|走|拿|放|推|拉|看|转|抬|低|开|关|切|递|接|坐|蹲|跑|握|拍|敲|揉面|开门|探头)/u

function extractActionTags(text: string): string[] {
  const out: string[] = []
  const seen = new Set<string>()
  for (const match of text.matchAll(ACTION_TAG)) {
    const action = match[1]?.trim()
    if (!action || seen.has(action)) continue
    seen.add(action)
    out.push(action)
  }
  return out
}

function clauseHasAction(clause: string): boolean {
  return ACTION_VERBS.test(clause)
}

function pruneAppearanceCatalog(desc: string): string {
  const clauses = desc.replace(/；/g, '，').split('，').map((part) => part.trim()).filter(Boolean)
  const kept = clauses.filter((clause) => !(APPEARANCE_NOISE.test(clause) && !clauseHasAction(clause)))
  if (kept.length === 0) return desc.trim()
  const out = kept.join('，')
  return out.endsWith('。') ? out : `${out}。`
}

/** Build the per-clip visual prompt for video generation. Dialogue is sent separately. */
export function buildVideoSceneDescription(storyboard: StoryboardVideoPromptInput): string {
  const scene = String(storyboard.scene_description || '').trim()
  if (!scene) return ''

  const actions = extractActionTags(scene)
  if (actions.length > 0) {
    return actions.join('，') + (actions.join('，').endsWith('。') ? '' : '。')
  }
  return pruneAppearanceCatalog(scene)
}
