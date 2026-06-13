'use client'

import { CheckCircle2, ChevronRight, Loader2 } from 'lucide-react'
import { EpisodeAutoPipelineFooter } from '@/components/projects/detail/EpisodeAutoPipelineFooter'
import {
  deriveEpisodeAssetBadge,
  deriveEpisodePhase,
  deriveEpisodeStoryboardBadge,
  episodeBadgeClassName,
  type EpisodeWorkspaceMeta,
} from '@/lib/projects/episode-list-status'
import type { EpisodeAutoPipelineAction } from '@/lib/projects/episode-pipeline'
import type { Episode } from '@/types'

type EpisodeListItemProps = {
  episode: Episode
  selected: boolean
  meta?: EpisodeWorkspaceMeta
  projectSharedAssetCount: number
  autoAction: EpisodeAutoPipelineAction
  accent?: 'primary' | 'indigo'
  storyboardLabels?: { splitting: string; generating: string }
  onSelect: () => void
  onAutoAction: (event: React.MouseEvent) => void
}

export function EpisodeListItem({
  episode,
  selected,
  meta,
  projectSharedAssetCount,
  autoAction,
  accent = 'primary',
  storyboardLabels,
  onSelect,
  onAutoAction,
}: EpisodeListItemProps) {
  const assetTotal = meta?.assetTotal ?? 0
  const assetCompleted = meta?.assetCompleted ?? 0
  const storyboardTotal = meta?.storyboardTotal ?? 0
  const storyboardCompleted = meta?.storyboardCompleted ?? 0
  const resourceSummary = assetTotal > 0 ? `${assetCompleted}/${assetTotal}` : '0/-'
  const storyboardSummary = storyboardTotal > 0 ? `${storyboardCompleted}/${storyboardTotal}` : '0/-'

  const phase = deriveEpisodePhase(episode, meta)
  const assetBadge = deriveEpisodeAssetBadge(episode, meta, projectSharedAssetCount)
  const storyboardBadge = deriveEpisodeStoryboardBadge(episode, meta, storyboardLabels)

  const selectedClass = accent === 'indigo'
    ? 'bg-indigo-50 border-indigo-200 text-indigo-800'
    : 'bg-primary-50 border-primary-200 text-primary-800'
  const numberClass = accent === 'indigo'
    ? selected ? 'bg-indigo-200 text-indigo-700' : 'bg-surface-100 text-surface-500 group-hover:bg-indigo-100 group-hover:text-indigo-600'
    : selected ? 'bg-primary-200 text-primary-700' : 'bg-surface-100 text-surface-500 group-hover:bg-primary-100 group-hover:text-primary-600'

  return (
    <div className={`group flex w-full flex-col rounded-xl border text-sm transition-all ${
      selected ? `${selectedClass} shadow-sm` : 'border-surface-200 bg-white text-surface-700 hover:border-primary-200 hover:bg-primary-50/60 hover:shadow-sm'
    }`}>
      <button type="button" onClick={onSelect} className="flex w-full cursor-pointer flex-col px-3 py-3 text-left">
        <div className="flex items-center justify-between gap-2 font-medium">
          <span className="flex min-w-0 items-center gap-2 truncate">
            <span className={`flex h-6 w-6 shrink-0 items-center justify-center rounded-md text-[11px] font-bold ${numberClass}`}>
              {episode.episode_number}
            </span>
            <span className="truncate">{episode.title || '未命名分集'}</span>
          </span>
          <ChevronRight className={`h-3.5 w-3.5 shrink-0 transition-transform ${selected ? 'text-primary-400' : 'text-surface-300 group-hover:translate-x-0.5 group-hover:text-primary-400'}`} />
        </div>
        <p className={`mt-1.5 text-[11px] font-medium ${phase.className}`}>当前阶段：{phase.label}</p>
        <p className="mt-0.5 text-[11px] text-surface-400">资源 {resourceSummary} · 分镜 {storyboardSummary}</p>
        <div className="mt-2 flex flex-wrap gap-1">
          <span className={`inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-[10px] font-medium ${episodeBadgeClassName(assetBadge.tone)}`}>
            {assetBadge.spinning ? <Loader2 className="h-3 w-3 animate-spin" /> : assetBadge.tone === 'green' ? <CheckCircle2 className="h-3 w-3" /> : null}
            {assetBadge.label}
          </span>
          <span className={`inline-flex items-center gap-1 rounded px-1.5 py-0.5 text-[10px] font-medium ${episodeBadgeClassName(storyboardBadge.tone)}`}>
            {storyboardBadge.spinning ? <Loader2 className="h-3 w-3 animate-spin" /> : null}
            {storyboardBadge.label}
          </span>
        </div>
      </button>
      <EpisodeAutoPipelineFooter
        action={autoAction}
        tone={accent === 'indigo' ? 'indigo' : 'primary'}
        onAction={onAutoAction}
      />
    </div>
  )
}
