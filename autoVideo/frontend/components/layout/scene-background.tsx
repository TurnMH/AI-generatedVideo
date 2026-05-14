type SceneBackgroundProps = {
  variant?: 'dashboard' | 'auth'
}

const STAR_TRACES = [
  { left: '8%', top: '12%', delay: '0s', width: 'w-24' },
  { left: '24%', top: '18%', delay: '1.8s', width: 'w-32' },
  { left: '52%', top: '10%', delay: '3.2s', width: 'w-28' },
  { left: '74%', top: '20%', delay: '2.4s', width: 'w-36' },
  { left: '84%', top: '34%', delay: '4.4s', width: 'w-24' },
]

const SKY_POINTS = [
  { left: '6%', top: '16%', size: 'h-1 w-1', delay: '0.4s' },
  { left: '14%', top: '24%', size: 'h-1.5 w-1.5', delay: '2.1s' },
  { left: '21%', top: '10%', size: 'h-1 w-1', delay: '1.1s' },
  { left: '36%', top: '18%', size: 'h-2 w-2', delay: '2.8s' },
  { left: '49%', top: '13%', size: 'h-1 w-1', delay: '0.7s' },
  { left: '61%', top: '22%', size: 'h-1.5 w-1.5', delay: '3.3s' },
  { left: '76%', top: '14%', size: 'h-1 w-1', delay: '1.5s' },
  { left: '88%', top: '28%', size: 'h-2 w-2', delay: '2.5s' },
]

export function SceneBackground({ variant = 'dashboard' }: SceneBackgroundProps) {
  const isAuth = variant === 'auth'

  return (
    <div className="pointer-events-none absolute inset-0 overflow-hidden">
      <div className="absolute inset-0 bg-[radial-gradient(circle_at_top,_rgba(18,61,84,0.82),transparent_42%),linear-gradient(180deg,#020712_0%,#07131d_32%,#0a1d22_58%,#0f1d1b_76%,#05070d_100%)]" />
      <div className="absolute inset-0 bg-[radial-gradient(circle_at_18%_18%,rgba(76,211,194,0.16),transparent_22%),radial-gradient(circle_at_82%_22%,rgba(251,191,36,0.12),transparent_18%),radial-gradient(circle_at_50%_10%,rgba(248,113,113,0.12),transparent_24%)]" />
      <div className="absolute inset-x-0 top-0 h-[44%] bg-[linear-gradient(180deg,rgba(120,216,255,0.08),rgba(120,216,255,0))]" />

      <div className="absolute left-[8%] top-[10%] h-40 w-40 rounded-full bg-cyan-300/10 blur-[110px]" />
      <div className="absolute right-[10%] top-[16%] h-56 w-56 rounded-full bg-amber-300/10 blur-[120px]" />
      <div className="absolute left-[32%] top-[8%] h-32 w-[28rem] rounded-full bg-emerald-300/10 blur-[90px]" />

      {SKY_POINTS.map((point, index) => (
        <span
          key={`${point.left}-${point.top}-${index}`}
          className={`absolute rounded-full bg-white/70 shadow-[0_0_18px_rgba(255,255,255,0.4)] animate-pulse ${point.size}`}
          style={{ left: point.left, top: point.top, animationDelay: point.delay }}
        />
      ))}

      {STAR_TRACES.map((trace, index) => (
        <span
          key={`${trace.left}-${trace.top}-${index}`}
          className={`absolute ${trace.width} h-px rotate-[-24deg] bg-gradient-to-r from-white/0 via-cyan-100/90 to-white/0 opacity-70`}
          style={{ left: trace.left, top: trace.top, animation: `meteor-drift 10s linear ${trace.delay} infinite` }}
        >
          <span className="absolute right-5 top-1/2 h-1.5 w-1.5 -translate-y-1/2 rounded-full bg-white shadow-[0_0_16px_rgba(255,255,255,0.65)]" />
        </span>
      ))}

      <div className="absolute left-[-6%] top-[18%] h-44 w-[28rem] rotate-[-12deg] rounded-full border border-cyan-300/20 bg-cyan-300/5 blur-[1px]"
        style={{ animation: 'dragon-orbit 18s ease-in-out infinite' }}
      />
      <div className="absolute right-[-8%] top-[14%] h-48 w-[30rem] rotate-[16deg] rounded-full border border-amber-300/20 bg-amber-200/5 blur-[1px]"
        style={{ animation: 'dragon-orbit 22s ease-in-out 1.4s infinite reverse' }}
      />
      <div className="absolute left-[14%] top-[20%] h-28 w-[18rem] rotate-[-10deg] rounded-full bg-gradient-to-r from-transparent via-cyan-200/18 to-transparent blur-2xl"
        style={{ animation: 'aurora-sway 15s ease-in-out infinite' }}
      />
      <div className="absolute right-[10%] top-[26%] h-28 w-[16rem] rotate-[8deg] rounded-full bg-gradient-to-r from-transparent via-amber-200/14 to-transparent blur-2xl"
        style={{ animation: 'aurora-sway 17s ease-in-out 1.2s infinite reverse' }}
      />

      <div className="absolute inset-x-0 bottom-[18%] h-[24%] bg-[radial-gradient(circle_at_50%_0%,rgba(147,197,253,0.22),transparent_48%)] opacity-80" />
      <div className="absolute bottom-[10%] left-1/2 h-24 w-[72%] -translate-x-1/2 rounded-[100%] bg-[linear-gradient(90deg,rgba(56,189,248,0),rgba(125,211,252,0.3),rgba(255,255,255,0.16),rgba(125,211,252,0.3),rgba(56,189,248,0))] blur-2xl"
        style={{ animation: 'river-flow 14s linear infinite' }}
      />
      <div className="absolute bottom-[8%] left-1/2 h-16 w-[55%] -translate-x-1/2 rounded-[100%] bg-[linear-gradient(90deg,rgba(14,165,233,0),rgba(56,189,248,0.4),rgba(255,255,255,0.24),rgba(56,189,248,0.4),rgba(14,165,233,0))] blur-xl"
        style={{ animation: 'river-flow 11s linear infinite reverse' }}
      />

      <div className="absolute inset-x-0 bottom-0 h-[38%] bg-[linear-gradient(180deg,transparent,rgba(2,6,23,0.24)_18%,rgba(2,6,23,0.72)_100%)]" />
      <div className="absolute bottom-0 left-[-4%] h-[30%] w-[36%] bg-slate-950 [clip-path:polygon(0_100%,0_48%,18%_22%,34%_42%,48%_16%,66%_44%,84%_14%,100%_56%,100%_100%)] opacity-95" />
      <div className="absolute bottom-0 left-[18%] h-[34%] w-[38%] bg-slate-900 [clip-path:polygon(0_100%,0_58%,18%_36%,34%_18%,52%_42%,68%_14%,84%_40%,100%_52%,100%_100%)] opacity-95" />
      <div className="absolute bottom-0 right-[14%] h-[28%] w-[34%] bg-slate-950 [clip-path:polygon(0_100%,0_52%,20%_18%,38%_40%,56%_12%,74%_46%,88%_28%,100%_58%,100%_100%)] opacity-95" />
      <div className="absolute bottom-0 right-[-6%] h-[32%] w-[34%] bg-slate-900 [clip-path:polygon(0_100%,0_48%,16%_26%,34%_12%,52%_44%,68%_20%,86%_46%,100%_34%,100%_100%)] opacity-95" />

      <div className="absolute bottom-[17%] left-[12%] h-6 w-6 rounded-full bg-cyan-200/70 blur-md" />
      <div className="absolute bottom-[14%] right-[18%] h-5 w-5 rounded-full bg-amber-200/70 blur-md" />

      {isAuth ? (
        <div className="absolute inset-0 bg-[radial-gradient(circle_at_50%_38%,transparent_0,transparent_42%,rgba(255,255,255,0.04)_62%,transparent_74%)]" />
      ) : null}
    </div>
  )
}