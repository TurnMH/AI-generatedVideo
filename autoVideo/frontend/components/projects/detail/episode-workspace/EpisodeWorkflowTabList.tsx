'use client'

import { Image as ImageIcon, LayoutGrid, Mic, Video, CheckCircle2, AlertCircle } from 'lucide-react'
import { TabsList, TabsTrigger } from '@/components/ui/tabs'
import type { WorkflowStep } from '@/lib/projects/episode-workspace-workflow-steps'
import { workflowStatusClass, workflowStatusLabel } from '@/lib/projects/episode-workspace-workflow-steps'
import { workflowStepTheme } from '@/components/projects/detail/episode-workspace/themes'

export function EpisodeWorkflowTabList({ steps }: { steps: WorkflowStep[] }) {
  return (
    <TabsList className="grid h-auto w-full grid-cols-4 gap-1.5 rounded-2xl bg-surface-100 p-1.5">
      {steps.map((step) => (
        <TabsTrigger
          key={step.key}
          value={step.key}
          className={`group relative flex cursor-pointer flex-col gap-1.5 overflow-hidden rounded-xl border border-surface-200/80 bg-white px-3 py-2.5 text-left transition-all duration-200 hover:-translate-y-px hover:shadow-sm data-[state=active]:shadow-md ${workflowStepTheme[step.key].tab}`}
        >
          <span className={`absolute inset-x-0 top-0 h-[3px] rounded-t-xl transition-colors duration-300 ${
            step.status === 'done' ? 'bg-emerald-400'
              : step.status === 'current' ? workflowStepTheme[step.key].currentStrip
                : step.status === 'failed' ? 'bg-red-400'
                  : step.status === 'skipped' ? 'bg-surface-150'
                    : workflowStepTheme[step.key].pendingStrip
          }`} />

          <div className="flex w-full items-center justify-between gap-2">
            <div className="flex min-w-0 items-center gap-2">
              <div className="relative flex h-6 w-6 shrink-0 items-center justify-center">
                {step.status === 'current' && (
                  <span className={`absolute inset-0 animate-ping rounded-full opacity-30 ${workflowStepTheme[step.key].pulse}`} />
                )}
                <span className={`relative flex h-6 w-6 items-center justify-center rounded-full transition-colors duration-200 ${
                  step.status === 'done' ? 'bg-emerald-100 text-emerald-600'
                    : step.status === 'current' ? workflowStepTheme[step.key].icon
                      : step.status === 'failed' ? 'bg-red-100 text-red-600'
                        : step.status === 'skipped' ? 'bg-surface-100 text-surface-300'
                          : workflowStepTheme[step.key].icon
                }`}>
                  {step.key === 'assets' && <ImageIcon className="h-3 w-3" />}
                  {step.key === 'storyboard' && <LayoutGrid className="h-3 w-3" />}
                  {step.key === 'dubbing' && <Mic className="h-3 w-3" />}
                  {step.key === 'video' && <Video className="h-3 w-3" />}
                </span>
              </div>
              <span className={`truncate text-xs font-semibold text-surface-700 transition-colors duration-200 ${workflowStepTheme[step.key].title}`}>
                {step.label}
              </span>
            </div>

            <span className={`inline-flex shrink-0 items-center gap-1 rounded-full border px-2 py-0.5 text-[10px] font-semibold transition-colors duration-200 ${step.status === 'current' ? workflowStepTheme[step.key].currentBadge : workflowStatusClass[step.status]}`}>
              {step.status === 'current' && <span className={`h-1.5 w-1.5 animate-pulse rounded-full ${workflowStepTheme[step.key].currentDot}`} />}
              {step.status === 'done' && <CheckCircle2 className="h-3 w-3" />}
              {step.status === 'failed' && <AlertCircle className="h-3 w-3" />}
              {step.statusLabel || workflowStatusLabel[step.status]}
            </span>
          </div>

          <div className="flex w-full items-center justify-between gap-2">
            <span className={`min-w-0 flex-1 truncate text-[11px] text-surface-500 transition-colors duration-200 ${workflowStepTheme[step.key].hint}`}>
              {step.hint}
            </span>
            <span className={`shrink-0 text-[10px] font-medium group-data-[state=active]:hidden ${workflowStepTheme[step.key].click}`}>点击进入</span>
            <span className="hidden shrink-0 text-[10px] font-medium text-surface-400 group-data-[state=active]:inline">当前</span>
          </div>
        </TabsTrigger>
      ))}
    </TabsList>
  )
}
