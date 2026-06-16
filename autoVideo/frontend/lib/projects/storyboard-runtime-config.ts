import { normalizeVideoStylePreset } from '@/lib/video-style-config'
import { commentaryProductionModeValue } from '@/lib/projects/commentary-project'
import type { Project } from '@/types'

export type SpeechPaceOption =
  | 'normal'
  | 'slightly_fast'
  | 'with_pauses'
  | 'very_fast'
  | 'medium_fast'
  | 'medium_steady'

export function defaultSpeechPaceForStyle(stylePreset?: string): SpeechPaceOption {
  switch (normalizeVideoStylePreset(stylePreset)) {
    case 'live-action-film':
      return 'medium_steady'
    case 'live-action-short':
      return 'with_pauses'
    case 'anime-3d':
      return 'slightly_fast'
    default:
      return 'normal'
  }
}

export function resolveProjectSpeechPace(project?: Pick<Project, 'storyboard_config'>): SpeechPaceOption {
  const configured = String(project?.storyboard_config?.speech_pace || '').trim() as SpeechPaceOption
  if (configured) return configured
  return defaultSpeechPaceForStyle(project?.storyboard_config?.style_preset)
}

export function buildProjectVideoRenderConfig(
  project: Pick<Project, 'storyboard_config' | 'style_tags'>,
  extras?: Record<string, unknown>,
): Record<string, unknown> {
  const cfg = project.storyboard_config ?? {}
  return {
    style_preset: normalizeVideoStylePreset(cfg.style_preset),
    speech_pace: resolveProjectSpeechPace(project),
    clip_duration_sec: cfg.duration,
    production_mode: commentaryProductionModeValue(project),
    ...extras,
  }
}

export function storyboardConfigFromProjectCreate(input: {
  aspect_ratio: string
  resolution: string
  duration?: number
  video_mode?: string
  style_preset: string
  motion_mode?: string
  production_mode?: string
  region?: string
  era?: string
  ethnicity?: string
}) {
  const stylePreset = normalizeVideoStylePreset(input.style_preset)
  return {
    aspect_ratio: input.aspect_ratio,
    resolution: input.resolution,
    ...(input.duration ? { duration: input.duration } : {}),
    ...(input.video_mode ? { video_mode: input.video_mode } : {}),
    style_preset: stylePreset,
    ...(input.motion_mode ? { motion_mode: input.motion_mode } : {}),
    speech_pace: defaultSpeechPaceForStyle(stylePreset),
    ...(input.production_mode ? { production_mode: input.production_mode } : {}),
    ...(input.region ? { region: input.region } : {}),
    ...(input.era ? { era: input.era } : {}),
    ...(input.ethnicity ? { ethnicity: input.ethnicity } : {}),
  }
}
