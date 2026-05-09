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
  if (pathname.includes('/storyboard')) return '分镜看板'
  if (pathname.includes('/characters')) return '角色管理'
  if (pathname.includes('/generate')) return '生成中心'
  if (pathname.includes('/video/')) return '项目详情'
  return 'AutoVideo'
}

function getSubtitle(pathname: string): string {
  if (pathname === '/video') return '集中管理你的 AI 视频项目、进度与产出'
  if (pathname.includes('/storyboard')) return '按镜头推进分镜与画面表达'
  if (pathname.includes('/characters')) return '统一管理角色设定与视觉资产'
  if (pathname.includes('/generate')) return '在同一工作流里推进生成任务'
  if (pathname.includes('/settings')) return '维护模型、存储与服务连接配置'
  return '让内容生产更流畅，也更有风格感'
}

function getEmoji(pathname: string): string {
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
    <header className="flex min-h-16 items-center justify-between border-b border-surface-200/60 bg-white/80 px-6 py-3 backdrop-blur-md">
      <div className="flex items-center gap-3">
        <div className="flex h-10 w-10 items-center justify-center rounded-2xl bg-gradient-to-br from-primary-50 via-violet-50 to-pink-50 text-lg shadow-sm ring-1 ring-surface-200/70">
          <span>{emoji}</span>
        </div>
        <div>
          <h1 className="text-lg font-semibold text-surface-900">{title}</h1>
          <p className="hidden text-xs text-surface-500 sm:block">{subtitle}</p>
        </div>
      </div>
      <div className="flex items-center gap-3">
        <div className="hidden sm:flex items-center gap-2 rounded-full bg-gradient-to-r from-primary-50 to-accent-50 px-3 py-1.5">
          <Sparkles className="h-3.5 w-3.5 text-primary-500" />
          <span className="text-xs font-medium text-primary-600">AI 就绪</span>
        </div>
        <Tip content="查看系统通知">
          <button className="relative rounded-xl p-2 text-surface-400 transition-all hover:bg-surface-100 hover:text-surface-600 hover:shadow-sm">
            <Bell className="h-5 w-5" />
            <span className="absolute right-1.5 top-1.5 flex h-2.5 w-2.5">
              <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-primary-400 opacity-75" />
              <span className="relative inline-flex h-2.5 w-2.5 rounded-full bg-gradient-to-r from-primary-500 to-accent-500" />
            </span>
          </button>
        </Tip>
      </div>
    </header>
  )
}
