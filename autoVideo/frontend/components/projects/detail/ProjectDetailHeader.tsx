'use client'

import { ArrowLeft, ChevronRight, ListVideo } from 'lucide-react'
import type { ReactNode } from 'react'
import { Button } from '@/components/ui/button'
import { StatusBadge } from '@/components/projects/detail/StatusBadge'
import type { Episode, Project } from '@/types'

type ProjectDetailHeaderProps = {
  mode: 'project' | 'episode'
  project: Project
  episode?: Episode
  badgeLabel: string
  badgeIcon: ReactNode
  accentClass?: string
  stats: ReactNode
  description: string
  extraActions?: ReactNode
  onBack: () => void
}

export function ProjectDetailHeader({
  mode,
  project,
  episode,
  badgeLabel,
  badgeIcon,
  accentClass = 'text-primary-300',
  stats,
  description,
  extraActions,
  onBack,
}: ProjectDetailHeaderProps) {
  if (mode === 'episode') {
    return (
      <div className="rounded-2xl border border-surface-200 bg-white px-5 py-4 shadow-sm">
        <div className="flex flex-wrap items-start gap-4">
          <Button variant="outline" size="icon" className="shrink-0 rounded-xl" onClick={onBack}>
            <ArrowLeft className="h-4 w-4" />
          </Button>
          <div className="min-w-0 flex-1">
            <div className="mb-2 inline-flex items-center gap-2 rounded-full border border-surface-200 bg-surface-50 px-3 py-1 text-xs font-medium text-surface-600">
              <ListVideo className="h-3.5 w-3.5" />
              单集工作台
            </div>
            <div className="flex flex-wrap items-center gap-2">
              <h2 className="text-xl font-semibold tracking-tight text-surface-900">
                第 {episode?.episode_number ?? '—'} 集 · {episode?.title || '未命名分集'}
              </h2>
              {episode?.status ? <StatusBadge status={episode.status} /> : null}
            </div>
            <p className="mt-2 max-w-3xl text-sm leading-6 text-surface-600">{description}</p>
            <div className="mt-3 flex flex-wrap gap-2 text-xs text-surface-500">{stats}</div>
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="overflow-hidden rounded-[28px] border border-surface-200/70 bg-gradient-to-br from-slate-950 via-violet-950 to-slate-900 p-5 text-white shadow-sm">
      <div className="flex flex-col gap-4">
        <div className="min-w-0">
          <div className="mb-3 inline-flex items-center gap-2 rounded-full border border-white/10 bg-white/10 px-3 py-1.5 text-xs font-medium text-surface-100">
            <span className={accentClass}>{badgeIcon}</span>
            {badgeLabel}
          </div>
          <div className="flex items-start gap-3">
            <Button
              variant="ghost"
              size="icon"
              className="mt-0.5 shrink-0 rounded-2xl border border-white/10 bg-white/10"
              onClick={onBack}
            >
              <ArrowLeft className="h-4 w-4" />
            </Button>
            <div className="min-w-0">
              <h2 className="text-2xl font-semibold tracking-tight text-white">{project.title}</h2>
              <div className="mt-2 flex flex-wrap items-center gap-2">
                <StatusBadge status={project.status} />
                {stats}
              </div>
              <p className="mt-3 max-w-3xl text-sm leading-6 text-surface-200">{description}</p>
            </div>
          </div>
        </div>
        {extraActions ? <div>{extraActions}</div> : null}
      </div>
    </div>
  )
}

export function HeaderStatPill({ children, className = '' }: { children: ReactNode; className?: string }) {
  return (
    <span className={`rounded-full border px-3 py-1 text-xs ${className}`}>
      {children}
    </span>
  )
}

export function EpisodeTabLink({
  label,
  onClick,
}: {
  label: string
  onClick: () => void
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="inline-flex items-center gap-1 rounded-full border border-surface-200 bg-surface-50 px-3 py-1 text-xs text-surface-600 transition hover:border-primary-200 hover:bg-primary-50 hover:text-primary-700"
    >
      {label}
      <ChevronRight className="h-3 w-3" />
    </button>
  )
}
