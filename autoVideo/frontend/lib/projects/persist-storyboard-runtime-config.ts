import { projectAPI } from '@/lib/api'
import { normalizeVideoStylePreset } from '@/lib/video-style-config'
import { defaultSpeechPaceForStyle } from '@/lib/projects/storyboard-runtime-config'
import type { Project, StoryboardConfig } from '@/types'

export async function persistStoryboardRuntimeConfig(
  projectId: number,
  current: StoryboardConfig | undefined,
  patch: Partial<StoryboardConfig>,
): Promise<void> {
  await projectAPI.update(projectId, {
    storyboard_config: { ...(current ?? {}), ...patch },
  } as Partial<Project>)
}

export function storyboardStylePresetPatch(
  current: StoryboardConfig | undefined,
  selectedStyle: string,
): Partial<StoryboardConfig> | null {
  const normalized = normalizeVideoStylePreset(selectedStyle)
  if (current?.style_preset === normalized) return null
  return {
    style_preset: normalized,
    speech_pace: defaultSpeechPaceForStyle(normalized),
  }
}

export function storyboardMotionModePatch(
  current: StoryboardConfig | undefined,
  selectedMotion: string,
): Partial<StoryboardConfig> | null {
  const trimmed = selectedMotion.trim()
  if (!trimmed || current?.motion_mode === trimmed) return null
  return { motion_mode: trimmed }
}
