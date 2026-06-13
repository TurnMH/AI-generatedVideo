'use client'

import { CheckCircle2, Loader2, Sparkles } from 'lucide-react'
import type { Episode } from '@/types'
import type { EpisodePipelineStatus } from '@/lib/projects/episode-workspace-pipeline-status'
import { pipelineToneBadgeClass, pipelineToneDotClass, pipelineToneLabel } from '@/lib/projects/episode-workspace-pipeline-status'
import type { SidebarTheme, ContentShellTheme } from '@/components/projects/detail/episode-workspace/themes'

export function EpisodeWorkspaceHeaderPanel({
  episode,
  episodeId,
  episodeSummary,
  updatedAtLabel,
  pipelineStatus,
  sidebarTheme,
  contentShellTheme,
}: {
  episode?: Episode
  episodeId: number
  episodeSummary: string
  updatedAtLabel: string
  pipelineStatus: EpisodePipelineStatus
  sidebarTheme: SidebarTheme
  contentShellTheme: ContentShellTheme
}) {
  return (
    <div className="space-y-5">
      <div>
        <div className="flex flex-wrap items-center gap-2 text-xs text-surface-500">
          <span className={`rounded-full border px-2.5 py-1 ${contentShellTheme.metaPill}`}>
            第 {episode?.episode_number ?? episodeId} 集
          </span>
          <span className={`rounded-full border px-2.5 py-1 ${contentShellTheme.metaPill}`}>
            更新时间 {updatedAtLabel}
          </span>
          <span className={`rounded-full border px-2.5 py-1 ${contentShellTheme.metaPill}`}>
            关键词 <span className={`font-semibold ${contentShellTheme.metaCount}`}>{episode?.keywords?.length ?? 0}</span>
          </span>
        </div>
        <h3 className="mt-3 text-xl font-semibold text-surface-900">
          {episode?.title || `第 ${episode?.episode_number ?? episodeId} 集`}
        </h3>
        <p className="mt-2 text-sm leading-6 text-surface-600">{episodeSummary}</p>
      </div>

      <div className={`relative overflow-hidden rounded-2xl border px-4 py-3 ${sidebarTheme.card}`}>
        <span className={`absolute inset-x-0 top-0 h-1 ${sidebarTheme.cardStrip}`} />
        <div className="mb-3 flex items-center justify-between gap-3">
          <span className={`inline-flex items-center gap-2 rounded-full border px-2.5 py-1 text-[11px] font-semibold ${pipelineToneBadgeClass[pipelineStatus.tone]}`}>
            <span className={`h-1.5 w-1.5 rounded-full ${pipelineToneDotClass[pipelineStatus.tone]}`} />
            {pipelineToneLabel[pipelineStatus.tone]}
          </span>
          <span className={`text-[11px] font-medium ${sidebarTheme.subtle}`}>当前阶段</span>
        </div>
        <div className="flex items-start gap-3">
          <div className={`mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-full ${sidebarTheme.iconWrap}`}>
            {pipelineStatus.tone === 'green' ? (
              <CheckCircle2 className={`h-4 w-4 ${sidebarTheme.icon}`} />
            ) : pipelineStatus.tone === 'slate' ? (
              <Sparkles className={`h-4 w-4 ${sidebarTheme.icon}`} />
            ) : (
              <Loader2 className={`h-4 w-4 animate-spin ${sidebarTheme.icon}`} />
            )}
          </div>
          <div>
            <p className={`text-sm font-semibold ${sidebarTheme.title}`}>{pipelineStatus.title}</p>
            <p className={`mt-1 text-xs ${sidebarTheme.desc}`}>{pipelineStatus.description}</p>
          </div>
        </div>
      </div>
    </div>
  )
}
