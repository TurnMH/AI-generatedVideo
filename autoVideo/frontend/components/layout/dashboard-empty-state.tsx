import type { ReactNode } from 'react'

type DashboardEmptyStateProps = {
  icon: ReactNode
  title: ReactNode
  description?: ReactNode
  action?: ReactNode
  className?: string
  innerClassName?: string
}

export function DashboardEmptyState({
  icon,
  title,
  description,
  action,
  className = 'rounded-[28px] border border-surface-200 bg-gradient-to-br from-white to-surface-50 p-3 shadow-sm',
  innerClassName = 'flex flex-col items-center justify-center rounded-[24px] border border-dashed border-surface-200 bg-[radial-gradient(circle_at_top_left,_rgba(99,102,241,0.08),_transparent_30%),radial-gradient(circle_at_bottom_right,_rgba(236,72,153,0.08),_transparent_28%)] px-6 py-16 text-center',
}: DashboardEmptyStateProps) {
  return (
    <div className={className}>
      <div className={innerClassName}>
        {icon}
        <p className="text-base font-medium text-surface-600">{title}</p>
        {description ? <p className="mt-2 text-sm text-surface-400">{description}</p> : null}
        {action ? <div className="mt-4">{action}</div> : null}
      </div>
    </div>
  )
}
