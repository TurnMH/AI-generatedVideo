import Link from 'next/link'
import {
  ArrowRight,
  BookOpen,
  Clapperboard,
  ImageIcon,
  Mic2,
  Sparkles,
  Video,
} from 'lucide-react'
import {
  EntryHeader,
  EntryPageLayout,
} from '@/components/marketing/entry-ui'

const creationPillars = [
  {
    title: '图片引擎',
    description: '封面、角色图与视觉修图集中处理。',
    icon: ImageIcon,
    accent: 'from-cyan-400/18 via-sky-500/12 to-transparent',
  },
  {
    title: '视频工作流',
    description: '从脚本到成片，压成一条连续生产线。',
    icon: Video,
    accent: 'from-emerald-300/18 via-cyan-500/12 to-transparent',
  },
  {
    title: '小说到内容',
    description: '把设定、世界观和剧情拆成可生产资产。',
    icon: BookOpen,
    accent: 'from-amber-300/18 via-orange-500/12 to-transparent',
  },
  {
    title: '漫画表达',
    description: '强化镜头感与角色表现，形成鲜明风格。',
    icon: Clapperboard,
    accent: 'from-rose-400/18 via-red-500/12 to-transparent',
  },
  {
    title: 'AI 音乐',
    description: '配乐、节奏与情绪控制纳入同一调度链路。',
    icon: Mic2,
    accent: 'from-sky-400/18 via-cyan-500/12 to-transparent',
  },
]

const pipelineSteps = [
  {
    title: '灵感收集',
    description: '把主题、角色、参考画面和情绪关键词先聚拢成项目母体。',
  },
  {
    title: '剧本生成',
    description: '把故事设定与节奏拆成能进入镜头生产的文本骨架。',
  },
  {
    title: '视觉设计',
    description: '将角色、封面、分镜和场景统一到同一视觉世界。',
  },
  {
    title: '成片输出',
    description: '汇总画面、旁白、配乐与剪辑结果，形成最终可发布内容。',
  },
]

export default function Home() {
  return (
    <EntryPageLayout animatedBackground>
      <EntryHeader
        actions={
          <>
            <Link
              href="/login"
              className="rounded-full border border-cyan-200/20 bg-[linear-gradient(135deg,rgba(34,211,238,0.22),rgba(45,212,191,0.16),rgba(255,255,255,0.08))] px-5 py-2 text-sm font-medium text-white shadow-[0_10px_28px_rgba(8,145,178,0.24)] backdrop-blur-xl transition hover:-translate-y-0.5 hover:border-cyan-200/30 hover:bg-[linear-gradient(135deg,rgba(34,211,238,0.28),rgba(45,212,191,0.2),rgba(255,255,255,0.1))]"
            >
              登录入口
            </Link>
            <Link
              href="/register"
              className="rounded-full border border-white/10 bg-white/6 px-4 py-2 text-sm font-medium text-slate-100 transition hover:-translate-y-0.5 hover:bg-white/10"
            >
              注册
            </Link>
          </>
        }
      />

      <section className="flex flex-1 items-center py-8 lg:py-12">
        <div className="w-full space-y-6 xl:space-y-8">
          <section className="relative overflow-hidden rounded-[44px] border border-white/10 bg-slate-950/30 p-6 backdrop-blur-2xl sm:p-8 xl:p-10">
            <div className="absolute inset-0 bg-[radial-gradient(circle_at_top_right,rgba(34,211,238,0.12),transparent_24%),radial-gradient(circle_at_bottom_left,rgba(245,158,11,0.12),transparent_22%)]" />
            <div className="relative grid gap-8 xl:grid-cols-[1.18fr_0.82fr] xl:items-stretch">
              <div className="flex h-full flex-col justify-between gap-10">
                <div>
                  <div className="inline-flex items-center gap-2 rounded-full border border-white/10 bg-white/5 px-4 py-2 text-sm text-surface-200 backdrop-blur-xl">
                    <Sparkles className="h-4 w-4 text-cyan-300" />
                    山河场景驱动的流媒体创作入口
                  </div>

                  <h1 className="mt-8 max-w-5xl text-4xl font-semibold leading-[0.98] sm:text-5xl lg:text-[5rem]">
                    用
                    <span className="bg-gradient-to-r from-cyan-300 via-teal-200 to-amber-300 bg-clip-text text-transparent">
                      AI Stream Media
                    </span>
                    串起图片、视频、剧情与流媒体内容工厂
                  </h1>

                  <div className="mt-8 max-w-2xl">
                    <p className="text-base leading-7 text-slate-300 sm:text-lg">
                      从灵感收集、剧本生成、视觉设计到成片输出，统一在 AI Stream Media 中完成，让内容生产更流畅，也更具场景感与调度能力。
                    </p>
                  </div>
                </div>

                <div className="rounded-[30px] border border-white/10 bg-[linear-gradient(135deg,rgba(34,211,238,0.08),rgba(15,23,42,0.6),rgba(245,158,11,0.08))] p-5 sm:p-6">
                  <div className="flex flex-wrap gap-2 text-xs text-slate-200">
                    {['图片优化', '视频生成', '小说拆解', '漫画分镜', 'AI 音乐'].map((item, index) => (
                      <span
                        key={item}
                        className={[
                          'inline-flex items-center gap-2 rounded-full border px-3 py-1.5 backdrop-blur-xl',
                          index === 0 && 'border-cyan-400/20 bg-cyan-400/10',
                          index === 1 && 'border-emerald-400/20 bg-emerald-400/10',
                          index === 2 && 'border-amber-400/20 bg-amber-400/10',
                          index === 3 && 'border-rose-400/20 bg-rose-400/10',
                          index === 4 && 'border-sky-400/20 bg-sky-400/10',
                        ]
                          .filter(Boolean)
                          .join(' ')}
                      >
                        <span className="h-1.5 w-1.5 rounded-full bg-current opacity-80" />
                        {item}
                      </span>
                    ))}
                  </div>
                </div>
              </div>

              <div className="grid gap-4 xl:grid-rows-[auto_auto_1fr]">
                <div className="rounded-[32px] border border-cyan-200/15 bg-[linear-gradient(135deg,rgba(34,211,238,0.18),rgba(15,23,42,0.5),rgba(255,255,255,0.06))] p-6 shadow-[0_16px_44px_rgba(8,145,178,0.18)]">
                  <p className="text-[11px] uppercase tracking-[0.22em] text-cyan-100/70">Quick Access</p>
                  <p className="mt-3 text-xl font-semibold text-white">直接进入登录或注册，马上继续项目。</p>
                  <p className="mt-3 text-sm leading-6 text-slate-200">
                    把入口收敛到第一屏右侧，不再让用户从长页面里找登录位置。
                  </p>
                </div>

                <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-1">
                  <Link
                    href="/login"
                    className="inline-flex items-center justify-center gap-2 rounded-full border border-cyan-200/20 bg-[linear-gradient(135deg,rgba(34,211,238,0.96),rgba(45,212,191,0.92),rgba(255,255,255,0.92))] px-5 py-3 text-sm font-semibold text-slate-950 shadow-[0_18px_42px_rgba(8,145,178,0.24)] transition hover:-translate-y-0.5 hover:shadow-[0_24px_48px_rgba(8,145,178,0.28)]"
                  >
                    去登录
                    <ArrowRight className="h-4 w-4" />
                  </Link>
                  <Link
                    href="/register"
                    className="inline-flex items-center justify-center gap-2 rounded-full border border-white/10 bg-[linear-gradient(135deg,rgba(255,255,255,0.96),rgba(224,231,255,0.92))] px-5 py-3 text-sm font-medium text-slate-900 shadow-[0_16px_42px_rgba(148,163,184,0.22)] transition hover:-translate-y-0.5 hover:shadow-[0_22px_48px_rgba(255,255,255,0.2)]"
                  >
                    创建账号
                    <ArrowRight className="h-4 w-4" />
                  </Link>
                </div>

                <div className="rounded-[30px] border border-white/10 bg-white/[0.04] p-5">
                  <p className="text-xs uppercase tracking-[0.24em] text-slate-400">Start Fast</p>
                  <div className="mt-4 space-y-3">
                    {[
                      '登录后回到项目与创作工作流',
                      '注册后即可进入内容生产界面',
                      '图片、视频、剧情与配乐集中调度',
                    ].map((item, index) => (
                      <div key={item} className="flex items-start gap-3 rounded-2xl border border-white/8 bg-black/10 px-4 py-3">
                        <span className="mt-0.5 flex h-6 w-6 shrink-0 items-center justify-center rounded-full border border-white/10 bg-white/[0.06] text-xs font-medium text-white">
                          {index + 1}
                        </span>
                        <p className="text-sm leading-6 text-slate-200">{item}</p>
                      </div>
                    ))}
                  </div>
                </div>
              </div>
            </div>
          </section>

          <div className="grid gap-6 xl:grid-cols-[0.8fr_1.2fr] xl:items-start">
            <section className="rounded-[44px] border border-white/10 bg-slate-950/30 p-6 backdrop-blur-2xl sm:p-8">
              <div>
                <p className="text-xs uppercase tracking-[0.26em] text-slate-400">Production Route</p>
                <h2 className="mt-3 text-3xl font-semibold leading-tight text-white">把创作流程做成一条纵向航道，而不是一堆并列说明</h2>
              </div>

              <div className="mt-8 space-y-5">
                {pipelineSteps.map((step, index) => (
                  <div key={step.title} className="flex gap-4 rounded-[28px] border border-white/10 bg-white/[0.03] p-5">
                    <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full border border-white/10 bg-white/[0.06] text-sm font-medium text-white">
                      {index + 1}
                    </div>
                    <div>
                      <h3 className="text-lg font-medium text-white">{step.title}</h3>
                      <p className="mt-2 text-sm leading-6 text-slate-300">{step.description}</p>
                    </div>
                  </div>
                ))}
              </div>
            </section>

            <section className="grid gap-4 md:grid-cols-2 xl:auto-rows-fr">
              {creationPillars.map((item, index) => {
                const Icon = item.icon
                return (
                  <div
                    key={item.title}
                    className={[
                      'group relative overflow-hidden rounded-[32px] border border-white/10 bg-slate-950/34 p-6 backdrop-blur-2xl transition duration-300 hover:-translate-y-1 hover:border-white/20',
                      index === 1 && 'md:row-span-2',
                      index === 4 && 'md:col-span-2',
                    ]
                      .filter(Boolean)
                      .join(' ')}
                  >
                    <div className={`absolute inset-0 bg-gradient-to-br ${item.accent} opacity-95`} />
                    <div className="relative flex h-full flex-col justify-between gap-8">
                      <div>
                        <div className="mb-5 flex h-12 w-12 items-center justify-center rounded-2xl border border-white/10 bg-black/20 shadow-[0_12px_28px_rgba(2,6,23,0.22)]">
                          <Icon className="h-5 w-5 text-white" />
                        </div>
                        <h3 className="text-xl font-medium text-white">{item.title}</h3>
                        <p className="mt-3 max-w-md text-sm leading-7 text-slate-300">{item.description}</p>
                      </div>

                      {index === 4 ? (
                        <div className="flex flex-wrap gap-3">
                          <Link
                            href="/register"
                            className="inline-flex items-center gap-2 rounded-full border border-white/10 bg-white/10 px-5 py-3 text-sm text-white transition hover:bg-white/15"
                          >
                            立即开始
                            <ArrowRight className="h-4 w-4" />
                          </Link>
                          <Link
                            href="/login"
                            className="inline-flex items-center gap-2 rounded-full border border-white/10 bg-transparent px-5 py-3 text-sm text-slate-200 transition hover:bg-white/8"
                          >
                            去登录
                          </Link>
                        </div>
                      ) : null}
                    </div>
                  </div>
                )
              })}
            </section>
          </div>
        </div>
      </section>
    </EntryPageLayout>
  )
}
