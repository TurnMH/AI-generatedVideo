'use client'

import { BookOpen, ChevronRight, Loader2 } from 'lucide-react'
import { EpisodeListItem } from '@/components/projects/detail/EpisodeListItem'
import type { EpisodeWorkspaceMeta } from '@/lib/projects/episode-list-status'
import type { EpisodeAutoPipelineAction } from '@/lib/projects/episode-pipeline'
import type { Episode } from '@/types'

type ProjectEpisodeSidebarProps = {
  episodes: Episode[]
  selectedEpisodeId: number | null
  episodeWorkspaceMeta: Map<number, EpisodeWorkspaceMeta>
  projectSharedAssetCount: number
  accent?: 'primary' | 'indigo'
  projectSplitInProgress?: boolean
  projectSplitTotal?: number
  projectSplitCompleted?: number
  getEpisodeAutoAction: (episode: Episode) => EpisodeAutoPipelineAction
  onSelectOverview: () => void
  onSelectEpisode: (episodeId: number) => void
  onEpisodeAutoAction: (episodeId: number, event: React.MouseEvent) => void
  storyboardLabels?: { splitting: string; generating: string }
}

export function ProjectEpisodeSidebar({
  episodes,
  selectedEpisodeId,
  episodeWorkspaceMeta,
  projectSharedAssetCount,
  accent = 'primary',
  projectSplitInProgress = false,
  projectSplitTotal = 0,
  projectSplitCompleted = 0,
  getEpisodeAutoAction,
  onSelectOverview,
  onSelectEpisode,
  onEpisodeAutoAction,
  storyboardLabels,
}: ProjectEpisodeSidebarProps) {
  const overviewSelected = selectedEpisodeId === null
  const overviewClass = accent === 'indigo'
    ? overviewSelected ? 'border-indigo-200 bg-indigo-50 text-indigo-700 shadow-sm' : 'border-surface-200 bg-surface-50 text-surface-700 hover:border-indigo-200 hover:bg-indigo-50/60 hover:text-indigo-700'
    : overviewSelected ? 'border-primary-200 bg-primary-50 text-primary-700 shadow-sm' : 'border-surface-200 bg-surface-50 text-surface-700 hover:border-primary-200 hover:bg-primary-50/60 hover:text-primary-700'

  return (
    <aside className="lg:sticky lg:top-4 lg:self-start">
      <div className="rounded-2xl border border-surface-200 bg-white p-3 shadow-sm">
        <div className="px-2 pb-2 pt-1 text-[11px] font-semibold uppercase tracking-widest text-surface-400">
          项目总览
        </div>
        <button
          type="button"
          onClick={onSelectOverview}
          className={`group flex w-full cursor-pointer items-center gap-3 rounded-xl border px-3 py-3 text-sm font-medium transition-all ${overviewClass}`}
        >
          <span className={`flex h-7 w-7 shrink-0 items-center justify-center rounded-lg ${
            overviewSelected
              ? accent === 'indigo' ? 'bg-indigo-100 text-indigo-600' : 'bg-primary-100 text-primary-600'
              : 'bg-surface-100 text-surface-500 group-hover:bg-primary-100 group-hover:text-primary-600'
          }`}>
            <BookOpen className="h-3.5 w-3.5" />
          </span>
          <span className="flex-1 text-left">剧本大纲与分集</span>
          <ChevronRight className={`h-4 w-4 shrink-0 transition-transform ${overviewSelected ? 'text-primary-400' : 'text-surface-300 group-hover:translate-x-0.5 group-hover:text-primary-400'}`} />
        </button>

        <div className="mb-2 mt-4 px-2 pb-1 text-[11px] font-semibold uppercase tracking-widest text-surface-400">
          单集工作区
        </div>
        <div className="max-h-[60vh] space-y-1.5 overflow-y-auto pr-0.5">
          {episodes.map((episode) => (
            <EpisodeListItem
              key={episode.id}
              episode={episode}
              selected={selectedEpisodeId === episode.id}
              meta={episodeWorkspaceMeta.get(episode.id)}
              projectSharedAssetCount={projectSharedAssetCount}
              autoAction={getEpisodeAutoAction(episode)}
              accent={accent}
              storyboardLabels={storyboardLabels}
              onSelect={() => onSelectEpisode(episode.id)}
              onAutoAction={(event) => onEpisodeAutoAction(episode.id, event)}
            />
          ))}
          {episodes.length === 0 ? (
            projectSplitInProgress ? (
              <div className="rounded-xl border border-primary-200 bg-primary-50 px-3 py-5 text-center">
                <div className="flex flex-col items-center gap-2 text-primary-700">
                  <div className="flex h-9 w-9 items-center justify-center rounded-full bg-primary-100">
                    <Loader2 className="h-4 w-4 animate-spin" />
                  </div>
                  <p className="text-sm font-semibold">AI 自动处理中</p>
                  <p className="max-w-[220px] text-xs leading-5 text-primary-500">
                    {projectSplitTotal > 0
                      ? `剧本解析中（${projectSplitCompleted}/${projectSplitTotal}），分集完成后自动出现在这里`
                      : '正在解析剧本与自动分集，完成后分集会自动出现在这里'}
                  </p>
                </div>
              </div>
            ) : (
              <div className="rounded-xl border border-dashed border-surface-200 px-3 py-5 text-center text-xs text-surface-400">
                暂无分集数据，请先生成大纲
              </div>
            )
          ) : null}
        </div>
      </div>
    </aside>
  )
}
