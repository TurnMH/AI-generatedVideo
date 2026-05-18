'use client'
import { Bell, Sparkles } from 'lucide-react'
import { usePathname } from 'next/navigation'
import { Tip } from '@/components/ui/tooltip'

const pageTitles: Record<string, string> = {
  '/video': '项目列表',
  '/settings/models': '模型配置',
}

function getTitle(pathname: string): string {
  if (pageTitles[pathname]) return pageTitles[pathname]
  if (pathname.includes('/ad-video')) return '视频广告生成'
  if (pathname.includes('/storyboard')) return '分镜看板'
  if (pathname.includes('/characters')) return '角色管理'
  if (pathname.includes('/generate')) return '生成中心'
  if (pathname.includes('/video/')) return '项目详情'
  return 'AI Stream Media'
}

function getSubtitle(pathname: string): string {
  if (pathname.includes('/ad-video')) return '通过文案与指定图片，快速产出广告短视频'
  if (pathname === '/video') return '集中管理你的 AI 视频项目、进度与产出'
  if (pathname.includes('/storyboard')) return '按镜头推进分镜与画面表达'
  if (pathname.includes('/characters')) return '统一管理角色设定与视觉资产'
  if (pathname.includes('/generate')) return '在同一工作流里推进生成任务'
  if (pathname.includes('/settings')) return '维护模型、存储与服务连接配置'
  return '统一编排内容、资产与流媒体生产链路'
}

function getEmoji(pathname: string): string {
  if (pathname.includes('/ad-video')) return '📣'
  if (pathname.includes('/storyboard')) return '🎬'
  if (pathname.includes('/characters')) return '👤'
  if (pathname.includes('/generate')) return '⚡'
  if (pathname.includes('/models')) return '🧠'
  if (pathname.includes('/storage')) return '💾'
  if (pathname.includes('/api-keys')) return '🔑'
  if (pathname.includes('/video')) return '📁'
  return '✨'
}

export function Header() {
  const pathname = usePathname()
  const title = getTitle(pathname)
  const emoji = getEmoji(pathname)
  const subtitle = getSubtitle(pathname)

  return (
    <header className="flex min-h-16 items-center justify-between border-b border-white/10 bg-slate-950/35 px-6 py-3 backdrop-blur-xl">
      <div className="flex items-center gap-3">
        <div className="flex h-10 w-10 items-center justify-center rounded-2xl bg-gradient-to-br from-cyan-400/18 via-emerald-300/16 to-amber-300/18 text-lg shadow-[0_12px_28px_rgba(8,145,178,0.18)] ring-1 ring-white/10">
          <span>{emoji}</span>
        </div>
        <div>
          <h1 className="text-lg font-semibold text-white">{title}</h1>
          <p className="hidden text-xs text-slate-300 sm:block">{subtitle}</p>
        </div>
      </div>
      <div className="flex items-center gap-3">
        <div className="hidden items-center gap-2 rounded-full border border-white/10 bg-white/5 px-3 py-1.5 sm:flex">
          <Sparkles className="h-3.5 w-3.5 text-cyan-300" />
          <span className="text-xs font-medium text-slate-100">Stream Online</span>
        </div>
        <Tip content="查看系统通知">
          <button className="relative rounded-xl p-2 text-slate-300 transition-all hover:bg-white/8 hover:text-white hover:shadow-sm">
            <Bell className="h-5 w-5" />
            <span className="absolute right-1.5 top-1.5 flex h-2.5 w-2.5">
              <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-cyan-300 opacity-75" />
              <span className="relative inline-flex h-2.5 w-2.5 rounded-full bg-gradient-to-r from-cyan-400 to-amber-300" />
            </span>
          </button>
        </Tip>
      </div>
    </header>
  )
}
