import type { Project } from '@/types'

/** True when the project should follow commentary-comic (解说漫) narration rules. */
export function isCommentaryProject(
  project: Pick<Project, 'storyboard_config' | 'style_tags'>,
): boolean {
  const configured = project.storyboard_config?.production_mode
  if (configured === 'commentary_comic') return true
  return (project.style_tags ?? []).includes('解说漫')
}

export function commentaryProductionModeValue(
  project: Pick<Project, 'storyboard_config' | 'style_tags'>,
): 'commentary_comic' | undefined {
  return isCommentaryProject(project) ? 'commentary_comic' : undefined
}
