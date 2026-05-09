import type { ReactNode } from 'react'

type DashboardHeroStat = {
  icon: ReactNode
  label: ReactNode
  value: ReactNode
  description?: ReactNode
}

type DashboardHeroProps = {
  badge: ReactNode
  badgeIcon?: ReactNode
  title: ReactNode
  description: ReactNode
  actions?: ReactNode
  stats?: DashboardHeroStat[]
  gradientClassName?: string
  contentClassName?: string
}

export function DashboardHero({
  badge,
  badgeIcon,
  title,
  description,
  actions,
  stats = [],
  gradientClassName = 'from-slate-950 via-violet-950 to-slate-900',
  contentClassName = 'max-w-2xl',
}: DashboardHeroProps) {
  return (
    <div className={`overflow-hidden rounded-[28px] border border-surface-200/70 bg-gradient-to-br ${gradientClassName} p-6 text-white shadow-sm`}>
      <div className="flex flex-col gap-6 lg:flex-row lg:items-start lg:justify-between">
        <div className={contentClassName}>
          <div className="mb-4 inline-flex items-center gap-2 rounded-full border border-white/10 bg-white/10 px-3 py-1.5 text-xs font-medium text-surface-100 backdrop-blur">
            {badgeIcon}
            {badge}
          </div>
          <h1 className="text-2xl font-semibold tracking-tight text-white">{title}</h1>
          <p className="mt-2 text-sm leading-6 text-surface-300">{description}</p>
        </div>
        {actions ? <div className="flex flex-wrap gap-2">{actions}</div> : null}
      </div>

      {stats.length > 0 ? (
        <div className={`mt-6 grid gap-3 ${stats.length >= 4 ? 'sm:grid-cols-4' : 'sm:grid-cols-3'}`}>
          {stats.map((stat, index) => (
            <div key={index} className="rounded-2xl border border-white/10 bg-white/10 p-4 backdrop-blur">
              <div className="flex items-center gap-2 text-surface-300">
                {stat.icon}
                <span className="text-xs uppercase tracking-[0.2em]">{stat.label}</span>
              </div>
              <p className="mt-3 text-2xl font-semibold text-white">{stat.value}</p>
              {stat.description ? <p className="mt-1 text-xs text-surface-400">{stat.description}</p> : null}
            </div>
          ))}
        </div>
      ) : null}
    </div>
  )
}
