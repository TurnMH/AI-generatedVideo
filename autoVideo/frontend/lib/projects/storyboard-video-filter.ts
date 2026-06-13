import type { Storyboard } from '@/types'

export function isStoryboardSerialCandidate(storyboard?: Pick<Storyboard, 'scene_group_key'> | null) {
  return Boolean(storyboard?.scene_group_key)
}

export function includeStoryboardForVideoTask(storyboard?: Pick<Storyboard, 'image_url' | 'scene_group_key'> | null) {
  return Boolean(storyboard?.image_url || isStoryboardSerialCandidate(storyboard))
}

export function filterReadyVideoStoryboards(items: Storyboard[]) {
  const ordered = [...items].sort((a, b) => a.sequence_number - b.sequence_number)
  const serialGroups = new Map<string, Storyboard[]>()
  const eligible: Storyboard[] = []

  for (const storyboard of ordered) {
    if (!includeStoryboardForVideoTask(storyboard)) continue
    if (!isStoryboardSerialCandidate(storyboard)) {
      eligible.push(storyboard)
      continue
    }
    const key = `${storyboard.episode_id ?? 0}:${storyboard.scene_group_key}`
    const current = serialGroups.get(key) ?? []
    current.push(storyboard)
    serialGroups.set(key, current)
  }

  for (const group of serialGroups.values()) {
    const firstClip = group.find((storyboard) => storyboard.is_scene_first_clip) ?? group[0]
    if (!firstClip?.image_url) continue
    eligible.push(...group)
  }

  return eligible.sort((a, b) => a.sequence_number - b.sequence_number)
}
