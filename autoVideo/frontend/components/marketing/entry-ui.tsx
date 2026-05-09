import type { ReactNode } from 'react'
import Link from 'next/link'
import { Sparkles, Video, type LucideIcon } from 'lucide-react'

export interface MarketingItem {
  title: string
  description: string
  icon: LucideIcon
  accent: string
}

interface EntryPageLayoutProps {
  children: ReactNode
  className?: string
  animatedBackground?: boolean
}

interface EntryHeaderProps {
  actions?: ReactNode
}

interface MarketingShowcaseProps {
  badge?: string
  title?: ReactNode
  description?: string
  items: MarketingItem[]
  footer?: ReactNode
}

interface EntryCardProps {
  title: string
  subtitle?: string
  tags?: string[]
  children: ReactNode
}

interface EntryFeatureStripProps {
  items: string[]
  className?: string
}

const ENTRY_STAR_PARTICLES = [
  { left: '6%', top: '14%', delay: '0s', size: 'h-1 w-1', opacity: 'bg-white/60' },
  { left: '13%', top: '27%', delay: '1.4s', size: 'h-1.5 w-1.5', opacity: 'bg-white/80' },
  { left: '21%', top: '9%', delay: '0.7s', size: 'h-1 w-1', opacity: 'bg-cyan-100/70' },
  { left: '28%', top: '19%', delay: '2.1s', size: 'h-2 w-2', opacity: 'bg-white/80' },
  { left: '36%', top: '7%', delay: '1.1s', size: 'h-1 w-1', opacity: 'bg-violet-100/70' },
  { left: '44%', top: '24%', delay: '2.8s', size: 'h-1.5 w-1.5', opacity: 'bg-white/75' },
  { left: '53%', top: '11%', delay: '0.5s', size: 'h-1 w-1', opacity: 'bg-white/60' },
  { left: '62%', top: '21%', delay: '1.9s', size: 'h-2 w-2', opacity: 'bg-sky-100/75' },
  { left: '71%', top: '8%', delay: '3.1s', size: 'h-1 w-1', opacity: 'bg-white/70' },
  { left: '79%', top: '17%', delay: '1.6s', size: 'h-1.5 w-1.5', opacity: 'bg-pink-100/70' },
  { left: '88%', top: '12%', delay: '2.3s', size: 'h-1 w-1', opacity: 'bg-white/65' },
  { left: '10%', top: '42%', delay: '0.9s', size: 'h-1.5 w-1.5', opacity: 'bg-white/75' },
  { left: '18%', top: '56%', delay: '2.6s', size: 'h-2 w-2', opacity: 'bg-cyan-100/75' },
  { left: '26%', top: '47%', delay: '1.2s', size: 'h-1 w-1', opacity: 'bg-white/60' },
  { left: '33%', top: '63%', delay: '3.3s', size: 'h-1.5 w-1.5', opacity: 'bg-white/80' },
  { left: '41%', top: '52%', delay: '0.3s', size: 'h-1 w-1', opacity: 'bg-violet-100/70' },
  { left: '49%', top: '68%', delay: '2.4s', size: 'h-2 w-2', opacity: 'bg-white/75' },
  { left: '58%', top: '58%', delay: '1.7s', size: 'h-1 w-1', opacity: 'bg-sky-100/70' },
  { left: '67%', top: '66%', delay: '2.9s', size: 'h-1.5 w-1.5', opacity: 'bg-white/80' },
  { left: '76%', top: '53%', delay: '0.6s', size: 'h-1 w-1', opacity: 'bg-white/60' },
  { left: '84%', top: '61%', delay: '1.5s', size: 'h-2 w-2', opacity: 'bg-pink-100/75' },
  { left: '91%', top: '46%', delay: '2.2s', size: 'h-1 w-1', opacity: 'bg-white/65' },
  { left: '12%', top: '78%', delay: '3.4s', size: 'h-1.5 w-1.5', opacity: 'bg-white/80' },
  { left: '24%', top: '88%', delay: '0.8s', size: 'h-1 w-1', opacity: 'bg-cyan-100/70' },
  { left: '37%', top: '81%', delay: '1.8s', size: 'h-2 w-2', opacity: 'bg-white/75' },
  { left: '46%', top: '92%', delay: '2.7s', size: 'h-1 w-1', opacity: 'bg-white/60' },
  { left: '59%', top: '84%', delay: '1.3s', size: 'h-1.5 w-1.5', opacity: 'bg-violet-100/75' },
  { left: '68%', top: '91%', delay: '3.2s', size: 'h-1 w-1', opacity: 'bg-white/70' },
  { left: '82%', top: '83%', delay: '0.4s', size: 'h-2 w-2', opacity: 'bg-white/80' },
  { left: '89%', top: '74%', delay: '2.5s', size: 'h-1 w-1', opacity: 'bg-sky-100/70' },
]

const ENTRY_CROSS_STARS = [
  { left: '9%', top: '22%', delay: '0.2s', size: 'h-3 w-3', opacity: 'text-white/65' },
  { left: '31%', top: '13%', delay: '1.1s', size: 'h-5 w-5', opacity: 'text-cyan-100/70' },
  { left: '47%', top: '29%', delay: '2.4s', size: 'h-4 w-4', opacity: 'text-white/70' },
  { left: '63%', top: '16%', delay: '0.9s', size: 'h-3.5 w-3.5', opacity: 'text-violet-100/70' },
  { left: '81%', top: '27%', delay: '1.8s', size: 'h-4.5 w-4.5', opacity: 'text-white/65' },
  { left: '17%', top: '64%', delay: '2.9s', size: 'h-3 w-3', opacity: 'text-pink-100/70' },
  { left: '39%', top: '73%', delay: '0.6s', size: 'h-5 w-5', opacity: 'text-white/70' },
  { left: '57%', top: '87%', delay: '1.5s', size: 'h-3.5 w-3.5', opacity: 'text-cyan-100/65' },
  { left: '78%', top: '69%', delay: '2.2s', size: 'h-4 w-4', opacity: 'text-white/70' },
]

const ENTRY_METEORS = [
  { left: '12%', top: '16%', delay: '0.8s', width: 'w-28' },
  { left: '58%', top: '12%', delay: '3.6s', width: 'w-36' },
  { left: '74%', top: '34%', delay: '6.2s', width: 'w-24' },
]

export function EntryPageLayout({
  children,
  className = '',
  animatedBackground = false,
}: EntryPageLayoutProps) {
  return (
    <main className={`relative min-h-screen overflow-hidden bg-[#060816] text-white ${className}`}>
      <div className="pointer-events-none absolute inset-0">
        <div className="absolute inset-0 bg-[radial-gradient(circle_at_top_left,_rgba(56,189,248,0.18),_transparent_28%),radial-gradient(circle_at_top_right,_rgba(168,85,247,0.16),_transparent_26%),radial-gradient(circle_at_bottom,_rgba(244,114,182,0.14),_transparent_30%)]" />
        <div className="absolute inset-0 bg-[radial-gradient(circle_at_center,_transparent_34%,_rgba(3,7,18,0.3)_72%,_rgba(2,6,23,0.7)_100%)]" />
        <div className="absolute left-[8%] top-12 h-44 w-44 rounded-full bg-cyan-400/20 blur-[110px]" />
        <div className="absolute right-[12%] top-[14%] h-64 w-64 rounded-full bg-violet-500/20 blur-[130px]" />
        <div className="absolute bottom-6 left-1/2 h-64 w-64 -translate-x-1/2 rounded-full bg-pink-500/15 blur-[140px]" />
        {animatedBackground ? (
          <>
            <div className="absolute inset-[-20%] animate-pulse opacity-60 bg-[radial-gradient(circle_at_20%_20%,_rgba(34,211,238,0.18),_transparent_22%),radial-gradient(circle_at_80%_24%,_rgba(168,85,247,0.18),_transparent_24%),radial-gradient(circle_at_50%_72%,_rgba(244,114,182,0.14),_transparent_26%)] blur-[30px]" />
            <div className="absolute inset-0 opacity-[0.14] [background-image:linear-gradient(rgba(255,255,255,0.08)_1px,transparent_1px),linear-gradient(90deg,rgba(255,255,255,0.08)_1px,transparent_1px)] [background-size:42px_42px] [mask-image:radial-gradient(circle_at_center,black,transparent_75%)]" />
            <div className="absolute inset-0 opacity-40 [background-image:radial-gradient(rgba(255,255,255,0.9)_0.8px,transparent_0.8px)] [background-size:26px_26px] [mask-image:radial-gradient(circle_at_center,black,transparent_82%)]" />
            <div className="absolute -left-[10%] top-[12%] h-40 w-[32rem] animate-pulse rounded-full bg-cyan-300/10 blur-3xl [transform:rotate(-18deg)]" />
            <div className="absolute right-[-10%] top-[44%] h-44 w-[28rem] animate-pulse rounded-full bg-fuchsia-400/10 blur-3xl [transform:rotate(20deg)]" style={{ animationDelay: '1.2s' }} />
            <div className="absolute left-[6%] top-[16%] h-24 w-24 animate-pulse rounded-full bg-cyan-300/20 blur-2xl" />
            <div className="absolute right-[10%] top-[22%] h-20 w-20 animate-pulse rounded-full bg-violet-300/20 blur-2xl" style={{ animationDelay: '0.8s' }} />
            <div className="absolute bottom-[18%] left-[16%] h-16 w-16 animate-pulse rounded-full bg-pink-300/20 blur-2xl" style={{ animationDelay: '1.6s' }} />
            <div className="absolute bottom-[10%] right-[20%] h-28 w-28 animate-pulse rounded-full bg-sky-300/15 blur-3xl" style={{ animationDelay: '2.4s' }} />
            {ENTRY_STAR_PARTICLES.map((particle, index) => (
              <span
                key={`${particle.left}-${particle.top}-${index}`}
                className={`absolute animate-pulse rounded-full shadow-[0_0_18px_rgba(255,255,255,0.35)] ${particle.size} ${particle.opacity}`}
                style={{ left: particle.left, top: particle.top, animationDelay: particle.delay }}
              />
            ))}
            {ENTRY_CROSS_STARS.map((star, index) => (
              <span
                key={`${star.left}-${star.top}-${index}`}
                className={`absolute flex animate-pulse items-center justify-center ${star.size} ${star.opacity}`}
                style={{ left: star.left, top: star.top, animationDelay: star.delay }}
              >
                <span className="absolute h-full w-px bg-current" />
                <span className="absolute h-px w-full bg-current" />
              </span>
            ))}
            {ENTRY_METEORS.map((meteor, index) => (
              <span
                key={`${meteor.left}-${meteor.top}-${index}`}
                className={`absolute ${meteor.width} h-px rotate-[-28deg] animate-pulse bg-gradient-to-r from-white/0 via-white/80 to-cyan-200/0 opacity-60 blur-[0.5px]`}
                style={{ left: meteor.left, top: meteor.top, animationDelay: meteor.delay, animationDuration: '8s' }}
              >
                <span className="absolute right-6 top-1/2 h-1.5 w-1.5 -translate-y-1/2 rounded-full bg-white/80 shadow-[0_0_14px_rgba(255,255,255,0.6)]" />
              </span>
            ))}
          </>
        ) : null}
      </div>

      <div className="relative z-10 mx-auto flex min-h-screen max-w-7xl flex-col px-4 py-8 sm:px-6 lg:px-8">
        {children}
      </div>
    </main>
  )
}

export function EntryHeader({ actions }: EntryHeaderProps) {
  return (
    <header className="flex items-center justify-between py-4">
      <BrandLockup />
      {actions ? <div className="flex items-center gap-3">{actions}</div> : null}
    </header>
  )
}

export function BrandLockup() {
  return (
    <div className="flex items-center gap-3">
      <div className="relative flex h-11 w-11 items-center justify-center rounded-2xl bg-gradient-to-br from-primary-500 via-violet-500 to-pink-500 shadow-glow">
        <span className="pointer-events-none absolute inset-[-5px] rounded-[20px] bg-gradient-to-br from-cyan-400/30 via-violet-400/25 to-pink-400/30 blur-md" />
        <Video className="h-5 w-5 text-white" />
        <Sparkles className="absolute -right-1 -top-1 h-3 w-3 text-amber-300" />
      </div>
      <div>
        <p className="text-lg font-semibold tracking-wide">AutoVideo</p>
        <p className="text-xs text-surface-400">AI 创作工作台</p>
      </div>
    </div>
  )
}

export function MarketingShowcase({
  badge,
  title,
  description,
  items,
  footer,
}: MarketingShowcaseProps) {
  return (
    <section className="hidden lg:block">
      <div className="max-w-2xl">
        {badge ? (
          <div className="mb-6 inline-flex items-center gap-2 rounded-full border border-white/10 bg-white/5 px-4 py-2 text-sm text-surface-200 backdrop-blur-xl">
            <Sparkles className="h-4 w-4 text-primary-300" />
            {badge}
          </div>
        ) : null}

        {title ? <h1 className="max-w-xl text-5xl font-semibold leading-tight">{title}</h1> : null}

        {description ? (
          <p className="mt-5 max-w-xl text-base leading-7 text-surface-300">{description}</p>
        ) : null}

        <div className="mt-10 grid gap-4 sm:grid-cols-2">
          {items.map((item) => {
            const Icon = item.icon
            return (
              <div
                key={item.title}
                className="group relative overflow-hidden rounded-3xl border border-white/10 bg-white/[0.06] p-5 backdrop-blur-2xl transition duration-300 hover:-translate-y-1 hover:border-white/20 hover:bg-white/[0.08]"
              >
                <div className={`absolute inset-0 bg-gradient-to-br ${item.accent} opacity-80`} />
                <div className="pointer-events-none absolute -left-1/3 top-0 h-full w-1/2 -skew-x-12 bg-gradient-to-r from-transparent via-white/10 to-transparent opacity-0 transition duration-700 group-hover:translate-x-[220%] group-hover:opacity-100" />
                <div className="relative">
                  <div className="mb-4 flex h-12 w-12 items-center justify-center rounded-2xl border border-white/10 bg-black/20 shadow-[0_12px_32px_rgba(15,23,42,0.22)]">
                    <Icon className="h-6 w-6 text-white" />
                  </div>
                  <h2 className="text-lg font-medium text-white">{item.title}</h2>
                  <p className="mt-2 text-sm leading-6 text-surface-300">{item.description}</p>
                </div>
              </div>
            )
          })}
        </div>

        {footer ? <div className="mt-8">{footer}</div> : null}
      </div>
    </section>
  )
}

export function EntryCard({ title, subtitle, tags, children }: EntryCardProps) {
  return (
    <div className="group relative overflow-hidden rounded-[32px] border border-white/10 bg-white/[0.08] p-6 shadow-2xl backdrop-blur-2xl sm:p-8">
      <div className="pointer-events-none absolute inset-0 bg-[linear-gradient(135deg,rgba(255,255,255,0.14),transparent_22%,transparent_68%,rgba(255,255,255,0.08))]" />
      <div className="pointer-events-none absolute inset-x-10 top-0 h-px bg-gradient-to-r from-transparent via-white/70 to-transparent opacity-70" />
      <div className="pointer-events-none absolute -left-1/3 top-[-18%] h-40 w-[72%] rotate-[18deg] bg-gradient-to-r from-transparent via-white/22 to-transparent blur-2xl transition-transform duration-1000 group-hover:translate-x-[42%]" />
      <div className="pointer-events-none absolute right-6 top-6 h-20 w-20 rounded-full bg-cyan-300/10 blur-2xl" />
      <div className="pointer-events-none absolute bottom-8 left-8 h-16 w-16 rounded-full bg-violet-300/10 blur-2xl" />

      <div className="relative mb-8 flex items-center gap-4">
        <div className="flex h-14 w-14 items-center justify-center rounded-2xl bg-gradient-to-br from-primary-500 via-violet-500 to-pink-500 shadow-glow">
          <Video className="h-7 w-7 text-white" />
        </div>
        <div>
          <p className="text-sm uppercase tracking-[0.2em] text-surface-400">AutoVideo</p>
          <h2 className="text-2xl font-semibold text-white">{title}</h2>
          {subtitle ? <p className="mt-1 text-sm text-surface-400">{subtitle}</p> : null}
        </div>
      </div>

      {tags?.length ? (
        <div className="relative mb-6 flex flex-wrap gap-2 text-xs text-surface-300">
          {tags.map((tag, index) => (
            <span
              key={tag}
              className={[
                'rounded-full px-3 py-1',
                index === 0 && 'border border-cyan-400/20 bg-cyan-400/10',
                index === 1 && 'border border-violet-400/20 bg-violet-400/10',
                index === 2 && 'border border-amber-400/20 bg-amber-400/10',
                index === 3 && 'border border-pink-400/20 bg-pink-400/10',
                index > 3 && 'border border-emerald-400/20 bg-emerald-400/10',
              ]
                .filter(Boolean)
                .join(' ')}
            >
              {tag}
            </span>
          ))}
        </div>
      ) : null}

      <div className="relative">{children}</div>
    </div>
  )
}

export function EntryFeatureStrip({ items, className = '' }: EntryFeatureStripProps) {
  return (
    <div className={`flex flex-wrap gap-2 text-xs text-surface-200 ${className}`}>
      {items.map((item, index) => (
        <span
          key={item}
          className={[
            'inline-flex items-center gap-2 rounded-full border px-3 py-1.5 backdrop-blur-xl',
            index % 4 === 0 && 'border-cyan-400/20 bg-cyan-400/10',
            index % 4 === 1 && 'border-violet-400/20 bg-violet-400/10',
            index % 4 === 2 && 'border-pink-400/20 bg-pink-400/10',
            index % 4 === 3 && 'border-emerald-400/20 bg-emerald-400/10',
          ]
            .filter(Boolean)
            .join(' ')}
        >
          <span className="h-1.5 w-1.5 rounded-full bg-current opacity-80" />
          {item}
        </span>
      ))}
    </div>
  )
}

export function EntryFooterLink({
  prefix,
  href,
  label,
}: {
  prefix: string
  href: string
  label: string
}) {
  return (
    <p className="mt-5 text-center text-sm text-surface-400">
      {prefix}{' '}
      <Link href={href} className="font-medium text-primary-300 transition-colors hover:text-primary-200">
        {label}
      </Link>
    </p>
  )
}

export function EntryCopyright() {
  return (
    <p className="mt-4 text-center text-xs text-surface-600">
      AutoVideo &copy; {new Date().getFullYear()}
    </p>
  )
}
