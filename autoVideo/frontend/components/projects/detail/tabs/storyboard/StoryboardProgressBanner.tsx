'use client'

import { Ban, CheckCircle2, AlertCircle, Clock, Loader2, Pause } from 'lucide-react'
import { Progress } from '@/components/ui/progress'

type StatsData = {
  total: number
  pending: number
  generating: number
  paused: number
  completed: number
  failed: number
  voided: number
}

export function StoryboardProgressBanner({
  stats,
  isActive,
  storyboardGenerateLabel,
  onFailedClick,
}: {
  stats: StatsData
  isActive: boolean
  storyboardGenerateLabel: string
  onFailedClick: () => void
}) {
  if (stats.total <= 0) return null

  return (
    <div className={`mb-4 rounded-lg border px-4 py-3 ${isActive ? 'border-blue-200 bg-blue-50' : 'border-surface-200 bg-surface-50'}`}>
      <div className="mb-2 flex items-center justify-between">
        <div className="flex items-center gap-2">
          {isActive ? <Loader2 className="h-4 w-4 animate-spin text-blue-600" /> : stats.paused > 0 ? <Pause className="h-4 w-4 text-yellow-700" /> : null}
          <span className={`text-sm font-medium ${isActive ? 'text-blue-800' : 'text-surface-700'}`}>
            {isActive ? `${storyboardGenerateLabel}中...` : stats.paused > 0 ? `${storyboardGenerateLabel}已暂停` : `${storyboardGenerateLabel}进度`}
          </span>
        </div>
        <span className="text-sm font-semibold text-surface-700">
          {stats.completed}/{stats.total}
        </span>
      </div>
      <Progress value={stats.total > 0 ? (stats.completed / stats.total) * 100 : 0} className="mb-2 h-2" />
      <div className="flex flex-wrap gap-3 text-xs">
        <span className="flex items-center gap-1 text-green-600">
          <CheckCircle2 className="h-3 w-3" /> 已完成 {stats.completed}
        </span>
        {stats.generating > 0 && (
          <span className="flex items-center gap-1 text-blue-600">
            <Loader2 className="h-3 w-3 animate-spin" /> 生成中 {stats.generating}
          </span>
        )}
        {stats.paused > 0 && (
          <span className="flex items-center gap-1 text-yellow-700">
            <Pause className="h-3 w-3" /> 已暂停 {stats.paused}
          </span>
        )}
        {stats.pending > 0 && (
          <span className="flex items-center gap-1 text-surface-500">
            <Clock className="h-3 w-3" /> 待生成 {stats.pending}
          </span>
        )}
        {stats.failed > 0 && (
          <span
            className="flex cursor-pointer items-center gap-1 text-red-500 hover:underline"
            onClick={onFailedClick}
          >
            <AlertCircle className="h-3 w-3" /> 失败 {stats.failed}
          </span>
        )}
        {stats.voided > 0 && (
          <span className="flex items-center gap-1 text-surface-400">
            <Ban className="h-3 w-3" /> 已作废 {stats.voided}
          </span>
        )}
      </div>
    </div>
  )
}
