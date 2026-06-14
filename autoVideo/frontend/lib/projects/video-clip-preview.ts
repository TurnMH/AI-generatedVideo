import type { Storyboard } from '@/types'
import { stripSpeakerLabelsForSpeech } from '@/lib/projects/storyboard-dubbing'
import { buildVideoSceneDescription } from '@/lib/projects/storyboard-video-prompt'

type RenderConfig = Record<string, unknown> | undefined

function renderConfigStrings(config: RenderConfig, key: string): string[] {
  const raw = config?.[key]
  if (!Array.isArray(raw)) return []
  return raw.map((item) => String(item ?? '').trim())
}

export function resolveVideoClipPreview({
  taskSceneDescription,
  renderConfig,
  clipOrder,
  storyboard,
  voiceText,
}: {
  taskSceneDescription?: string
  renderConfig?: RenderConfig
  clipOrder: number
  storyboard?: Pick<Storyboard, 'scene_description' | 'dialogue' | 'prompt_used'>
  voiceText?: string
}): { videoPrompt: string; voiceText: string } {
  const storedVideoPrompt = renderConfigStrings(renderConfig, 'scene_descriptions')[clipOrder] ?? ''
  const storedVoiceText = renderConfigStrings(renderConfig, 'dialogues')[clipOrder] ?? ''

  const derivedVideoPrompt = storyboard ? buildVideoSceneDescription(storyboard) : ''
  const videoPrompt = storedVideoPrompt
    || derivedVideoPrompt
    || (clipOrder === 0 ? String(taskSceneDescription ?? '').trim() : '')

  const derivedVoiceText = voiceText?.trim() ?? ''
  const resolvedVoiceText = stripSpeakerLabelsForSpeech(storedVoiceText || derivedVoiceText)

  return { videoPrompt, voiceText: resolvedVoiceText }
}
