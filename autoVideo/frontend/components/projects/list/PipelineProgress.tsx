'use client'

import type { Project, StageProgress } from '@/types'

export function PipelineProgress({ progress }: { progress: Project['progress'] }) {
  const stages = [
    { key: 'script',     label: '剧本', color: 'bg-primary-500' },
    { key: 'asset',      label: '资源', color: 'bg-accent-500' },
    { key: 'storyboard', label: '分镜', color: 'bg-amber-500' },
    { key: 'dubbing',    label: '配音', color: 'bg-pink-500' },
    { key: 'video',      label: '视频', color: 'bg-emerald-500' },
  ] as const

  if (!progress) return <div className="h-2 rounded-full bg-surface-100" />

  const getPct = (s?: StageProgress) => {
    if (!s) return 0
    if (s.status === 'done' || s.status === 'skipped') return 100
    if (s.total && s.total > 0) return Math.min(100, Math.round(((s.completed ?? 0) / s.total) * 100))
    if (s.status === 'running') return 50
    return 0
  }

  return (
    <div className="space-y-1">
      <div className="flex h-2 gap-0.5 overflow-hidden rounded-full">
        {stages.map((stage) => {
          const s = progress[stage.key as keyof typeof progress] as StageProgress | undefined
          const pct = getPct(s)
          const isActive = s?.status === 'running'
          return (
            <div key={stage.key} className="flex-1 overflow-hidden rounded-sm bg-surface-100">
              <div
                className={`h-full rounded-sm ${stage.color} transition-all duration-500 ${isActive ? 'animate-pulse' : ''}`}
                style={{ width: `${pct}%` }}
              />
            </div>
          )
        })}
      </div>
      <div className="flex text-[10px] text-surface-400">
        {stages.map((stage) => {
          const s = progress[stage.key as keyof typeof progress] as StageProgress | undefined
          const pct = getPct(s)
          return (
            <span key={stage.key} className="flex-1 text-center" title={`${stage.label} ${pct}%`}>
              {stage.label}
            </span>
          )
        })}
      </div>
    </div>
  )
}
