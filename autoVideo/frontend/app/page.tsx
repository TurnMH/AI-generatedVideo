import Link from 'next/link'
import { ArrowRight, BookOpen, Clapperboard, ImageIcon, Music2, Video } from 'lucide-react'
import {
  EntryFeatureStrip,
  EntryHeader,
  EntryPageLayout,
  MarketingShowcase,
  type MarketingItem,
} from '@/components/marketing/entry-ui'

const creationPillars: MarketingItem[] = [
  {
    title: '图片引擎',
    description: '封面、海报、角色图、优化修图与动图处理统一完成。',
    icon: ImageIcon,
    accent: 'from-cyan-400/35 via-sky-500/20 to-transparent',
  },
  {
    title: '视频工作流',
    description: '从脚本、分镜到成片输出，把视频生产链路串起来。',
    icon: Video,
    accent: 'from-violet-400/35 via-fuchsia-500/20 to-transparent',
  },
  {
    title: '小说到内容',
    description: '把故事设定、世界观和剧情拆解成可生产的创作资产。',
    icon: BookOpen,
    accent: 'from-amber-300/35 via-orange-500/20 to-transparent',
  },
  {
    title: '漫画表达',
    description: '强化镜头感、画面节奏和角色表现，形成更强视觉风格。',
    icon: Clapperboard,
    accent: 'from-pink-400/35 via-rose-500/20 to-transparent',
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
              className="rounded-full border border-white/10 bg-white/5 px-4 py-2 text-sm text-surface-200 backdrop-blur-xl transition hover:border-white/20 hover:bg-white/10"
            >
              登录
            </Link>
            <Link
              href="/register"
              className="rounded-full border border-white/10 bg-[linear-gradient(135deg,rgba(56,189,248,0.95),rgba(139,92,246,0.95),rgba(236,72,153,0.92))] px-4 py-2 text-sm font-medium text-white shadow-[0_12px_34px_rgba(168,85,247,0.22)] transition hover:-translate-y-0.5 hover:shadow-[0_18px_44px_rgba(236,72,153,0.24)]"
            >
              注册
            </Link>
          </>
        }
      />

      <section className="flex flex-1 items-center py-10 lg:py-16">
        <div className="grid w-full items-center gap-10 lg:grid-cols-[1.05fr_0.95fr]">
          <div className="max-w-2xl">
            <div className="mb-6 inline-flex items-center gap-2 rounded-full border border-white/10 bg-white/5 px-4 py-2 text-sm text-surface-200 backdrop-blur-xl">
              时尚感更强的一体化内容入口
            </div>

            <h1 className="text-4xl font-semibold leading-tight sm:text-5xl lg:text-6xl">
              把
              <span className="bg-gradient-to-r from-cyan-300 via-violet-300 to-pink-300 bg-clip-text text-transparent">
                图片、视频、小说、漫画、音乐
              </span>
              创作汇聚到同一个工作台
            </h1>

            <p className="mt-6 max-w-xl text-base leading-7 text-surface-300 sm:text-lg">
              从灵感收集、剧本生成、视觉设计到成片输出，统一在 AutoVideo 中完成，让内容生产更流畅，也更有风格感。
            </p>

            <div className="mt-8 flex flex-wrap gap-3">
              <Link
                href="/video"
                className="inline-flex items-center gap-2 rounded-full border border-white/10 bg-[linear-gradient(135deg,rgba(255,255,255,0.96),rgba(224,231,255,0.92))] px-5 py-3 text-sm font-medium text-slate-900 shadow-[0_16px_42px_rgba(148,163,184,0.22)] transition hover:-translate-y-0.5 hover:shadow-[0_22px_48px_rgba(255,255,255,0.2)]"
              >
                进入工作台
                <ArrowRight className="h-4 w-4" />
              </Link>
              <Link
                href="/login"
                className="inline-flex items-center gap-2 rounded-full border border-white/10 bg-white/5 px-5 py-3 text-sm text-surface-200 backdrop-blur-xl transition hover:border-white/20 hover:bg-white/10"
              >
                去登录
              </Link>
            </div>

            <EntryFeatureStrip
              className="mt-10"
              items={['图片优化', '视频生成', '小说拆解', '漫画分镜', 'AI 音乐']}
            />
          </div>

          <div className="grid gap-4 sm:grid-cols-2">
            <MarketingShowcase
              badge="多模态创作模块"
              title={null}
              description=""
              items={creationPillars}
              footer={
                <div className="relative overflow-hidden rounded-3xl border border-white/10 bg-white/[0.05] p-5 backdrop-blur-2xl">
                  <div className="absolute inset-0 bg-gradient-to-r from-emerald-400/20 via-transparent to-cyan-400/10" />
                  <div className="relative flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
                    <div>
                      <p className="text-lg font-medium text-white">多模态创作流</p>
                      <p className="mt-2 max-w-lg text-sm leading-6 text-surface-300">
                        让图像、剧情、分镜、配乐与视频输出形成一条顺滑链路，适合做更完整的内容产品。
                      </p>
                    </div>
                    <Link
                      href="/register"
                      className="inline-flex items-center gap-2 self-start rounded-full border border-white/10 bg-white/10 px-4 py-2 text-sm text-white transition hover:bg-white/15"
                    >
                      立即开始
                      <ArrowRight className="h-4 w-4" />
                    </Link>
                  </div>
                </div>
              }
            />
          </div>
        </div>
      </section>
    </EntryPageLayout>
  )
}
