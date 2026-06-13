import type { EpisodeCountRecommendation } from '@/lib/projects/comic'

export function resolveDraftTargetEpisodes(
  targetEpisodes: number,
  recommendation: EpisodeCountRecommendation | null | undefined,
  hasGeneratedEpisodes: boolean,
): string {
  if (
    recommendation
    && !hasGeneratedEpisodes
    && (targetEpisodes <= 0 || (targetEpisodes === 1 && recommendation.count !== 1))
  ) {
    return String(recommendation.count)
  }
  return targetEpisodes > 0 ? String(targetEpisodes) : ''
}
