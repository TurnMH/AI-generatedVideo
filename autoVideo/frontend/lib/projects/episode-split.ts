import type { Project } from '@/types'

export const AUTO_EPISODE_SPLIT_HINT =
  '上传剧本后，系统会优先识别章节标题（第X章/回/节/集等）分集；若无章节标记，再按剧情结构或字数估算。'

type EpisodeSplitProject = Pick<Project, 'target_episodes' | 'project_type'> | null | undefined

/** 视频 / 连续剧项目默认走剧本自动分集，不强制用户填目标集数。 */
export function prefersAutoEpisodeSplit(project?: { project_type?: string } | null): boolean {
  const type = project?.project_type
  return type === 'video' || type === 'video_serial'
}

export function episodeSplitPendingHint(project: EpisodeSplitProject): string {
  if (!project) return '等待拆分'
  if (prefersAutoEpisodeSplit(project)) return '按剧本自动分集'
  if ((project.target_episodes ?? 0) > 0) return `目标 ${project.target_episodes} 集`
  return '等待拆分'
}

export function episodeSplitOverviewBadge(project: EpisodeSplitProject): string {
  if (!project) return '加载中'
  if (prefersAutoEpisodeSplit(project)) return '按剧本自动分集'
  if ((project.target_episodes ?? 0) > 0) return `目标 ${project.target_episodes} 集`
  return '未设置集数'
}
