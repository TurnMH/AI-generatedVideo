'use client'

import { CheckCircle2, Loader2, RefreshCw, Sparkles } from 'lucide-react'
import { Button } from '@/components/ui/button'
import type { EpisodeAutoPipelineAction } from '@/lib/projects/episode-pipeline'

type EpisodeAutoPipelineFooterProps = {
  action: EpisodeAutoPipelineAction
  tone?: 'primary' | 'indigo'
  disabled?: boolean
  onAction: (event: React.MouseEvent) => void
}

export function EpisodeAutoPipelineFooter({
  action,
  tone = 'primary',
  disabled = false,
  onAction,
}: EpisodeAutoPipelineFooterProps) {
  if (action.type === 'hidden') return null

  const runningClass = tone === 'indigo' ? 'text-indigo-600' : 'text-primary-600'
  const successClass = tone === 'indigo' ? 'text-emerald-700' : 'text-emerald-700'
  const retryButtonClass = tone === 'indigo'
    ? 'border-amber-200 text-amber-800 hover:bg-amber-50'
    : 'border-amber-200 text-amber-800 hover:bg-amber-50'
  const startButtonClass = tone === 'indigo'
    ? 'border-indigo-200 text-indigo-700 hover:bg-indigo-50'
    : 'border-primary-200 text-primary-700 hover:bg-primary-50'

  return (
    <div className="border-t border-surface-100 px-3 py-2">
      {action.type === 'running' ? (
        <div className={`flex items-center gap-2 text-[11px] ${runningClass}`}>
          <Loader2 className="h-3.5 w-3.5 animate-spin" />
          {action.label}
        </div>
      ) : null}

      {action.type === 'success' ? (
        <div className={`flex items-center gap-2 text-[11px] ${successClass}`}>
          <CheckCircle2 className="h-3.5 w-3.5 shrink-0" />
          <span>{action.label}</span>
        </div>
      ) : null}

      {action.type === 'start' ? (
        <Button
          type="button"
          size="sm"
          variant="outline"
          disabled={disabled}
          className={`h-7 w-full text-xs ${startButtonClass}`}
          onClick={onAction}
        >
          <Sparkles className="mr-1.5 h-3.5 w-3.5" />
          自动处理本集
        </Button>
      ) : null}

      {action.type === 'retry' ? (
        <div className="space-y-1.5">
          {action.reason ? (
            <p className="text-[10px] leading-4 text-amber-700">{action.reason}</p>
          ) : null}
          <Button
            type="button"
            size="sm"
            variant="outline"
            disabled={disabled}
            className={`h-7 w-full text-xs ${retryButtonClass}`}
            onClick={onAction}
          >
            <RefreshCw className="mr-1.5 h-3.5 w-3.5" />
            {action.label}
          </Button>
        </div>
      ) : null}
    </div>
  )
}
