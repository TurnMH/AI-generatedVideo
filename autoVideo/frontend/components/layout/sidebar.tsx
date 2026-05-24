'use client'
import Link from 'next/link'
import { usePathname, useRouter } from 'next/navigation'
import {
  FolderOpen,
  Settings,
  Cpu,
  LogOut,
  Video,
  ChevronRight,
  HardDrive,
  KeyRound,
  Sparkles,
  BookOpen,
  Music2,
  Image as ImageIcon,
  Zap,
  Code2,
  Wand2,
  MessageSquare,
  Wrench,
  FileText,
  Film,
  Eye,
} from 'lucide-react'
import { cn } from '@/lib/utils'
import { useAuthStore } from '@/lib/store/auth'
import { authAPI } from '@/lib/api'

const navItems = [
  {
    label: '视频',
    href: '/projects',
    icon: FolderOpen,
  },
  {
    label: '视频（串行）',
    href: '/video-serial',
    icon: Film,
  },
  {
    label: '快速生成',
    href: '/quick',
    icon: Wand2,
    badge: 'NEW',
  },
  {
    label: '剧本库',
    href: '/scripts',
    icon: BookOpen,
  },
  {
    label: '漫画',
    href: '/comics',
    icon: Sparkles,
  },
  {
    label: '音乐',
    href: '/music',
    icon: Music2,
  },
  {
    label: '图片',
    href: '/images',
    icon: ImageIcon,
  },
  {
    label: 'AI 对话',
    href: '/chat',
    icon: MessageSquare,
  },
  {
    label: '工具箱',
    href: '/tools',
    icon: Wrench,
    children: [
      { label: '图片工具', href: '/tools/image', icon: ImageIcon },
      { label: '文字工具', href: '/tools/text', icon: FileText },
      { label: '视频工具', href: '/tools/video', icon: Film },
      { label: '文档预览', href: '/tools/docs', icon: Eye },
    ],
  },
  {
    label: '设置',
    href: '/settings',
    icon: Settings,
    children: [
      { label: '模型配置', href: '/settings/models', icon: Cpu },
      { label: '存储管理', href: '/settings/storage', icon: HardDrive },
      { label: 'API 密钥', href: '/settings/api-keys', icon: KeyRound },
      { label: 'Skill 管理', href: '/settings/skills', icon: Zap },
      { label: '提示词模板', href: '/settings/prompts', icon: Code2 },
    ],
  },
]

export function Sidebar() {
  const pathname = usePathname()
  const router = useRouter()
  const { user, clearAuth } = useAuthStore()

  const handleLogout = async () => {
    try {
      await authAPI.logout()
    } catch {
      // ignore
    } finally {
      clearAuth()
      router.push('/login')
    }
  }

  return (
    <aside className="sidebar-panel flex h-screen w-64 flex-col border-r border-white/[0.06] shadow-[0_24px_60px_rgba(2,6,23,0.38)]">
      {/* Logo */}
      <div className="flex h-16 items-center gap-3 border-b border-white/[0.06] px-5">
        <div className="relative flex h-9 w-9 items-center justify-center rounded-xl bg-gradient-to-br from-cyan-400 via-teal-400 to-amber-400 shadow-glow-sm">
          <Video className="h-4.5 w-4.5 text-white" />
          <Sparkles className="absolute -right-1 -top-1 h-3 w-3 text-amber-400 animate-pulse" />
        </div>
        <span className="text-base font-bold tracking-wide text-white">AI Stream Media</span>
        <span className="ml-auto rounded-full bg-cyan-400/12 px-2 py-0.5 text-[10px] font-medium text-cyan-200">Studio</span>
      </div>

      {/* Navigation */}
      <nav className="flex-1 overflow-y-auto px-3 py-5">
        <ul className="space-y-1">
          {navItems.map((item) => {
            const isActive =
              pathname === item.href || pathname.startsWith(item.href + '/')
            const Icon = item.icon

            if (item.children) {
              return (
                <li key={item.href}>
                  <div className="mb-2 mt-4 flex items-center gap-2 px-3 py-1 text-[11px] font-semibold uppercase tracking-widest text-surface-400/60">
                    <Icon className="h-3.5 w-3.5" />
                    {item.label}
                  </div>
                  <ul className="space-y-0.5 pl-1">
                    {item.children.map((child) => {
                      const ChildIcon = child.icon
                      const childActive = pathname === child.href
                      return (
                        <li key={child.href}>
                          <Link
                            href={child.href}
                            prefetch={false}
                            className={cn(
                              'group flex items-center gap-2.5 rounded-lg px-3 py-2 text-sm transition-all duration-200',
                              childActive
                                ? 'bg-gradient-to-r from-cyan-400/18 to-amber-400/14 text-white font-medium shadow-inner-glow'
                                : 'text-surface-400 hover:bg-white/[0.06] hover:text-surface-200'
                            )}
                          >
                            <ChildIcon className={cn(
                              'h-4 w-4 transition-colors',
                              childActive ? 'text-cyan-300' : 'text-surface-500 group-hover:text-surface-300'
                            )} />
                            {child.label}
                            {childActive && (
                              <div className="ml-auto flex items-center gap-1">
                                <span className="h-1.5 w-1.5 rounded-full bg-cyan-300 animate-pulse" />
                                <ChevronRight className="h-3.5 w-3.5 text-cyan-300" />
                              </div>
                            )}
                          </Link>
                        </li>
                      )
                    })}
                  </ul>
                </li>
              )
            }

            return (
              <li key={item.href}>
                <Link
                  href={item.href}
                  prefetch={false}
                  className={cn(
                    'group flex items-center gap-2.5 rounded-lg px-3 py-2.5 text-sm transition-all duration-200',
                    isActive
                      ? 'bg-gradient-to-r from-cyan-400/18 to-amber-400/14 text-white font-medium shadow-inner-glow'
                      : 'text-surface-400 hover:bg-white/[0.06] hover:text-surface-200'
                  )}
                >
                  <Icon className={cn(
                    'h-4 w-4 transition-colors',
                    isActive ? 'text-cyan-300' : 'text-surface-500 group-hover:text-surface-300'
                  )} />
                  {item.label}
                  {'badge' in item && item.badge && !isActive && (
                    <span className="ml-1 rounded-full bg-amber-400/15 px-1.5 py-0.5 text-[10px] font-semibold text-amber-200">
                      {item.badge}
                    </span>
                  )}
                  {isActive && (
                    <div className="ml-auto flex items-center gap-1">
                      <span className="h-1.5 w-1.5 rounded-full bg-cyan-300 animate-pulse" />
                      <ChevronRight className="h-3.5 w-3.5 text-cyan-300" />
                    </div>
                  )}
                </Link>
              </li>
            )
          })}
        </ul>
      </nav>

      {/* User */}
      <div className="border-t border-white/[0.06] p-3">
        <div className="flex items-center gap-3 rounded-lg px-2.5 py-2.5 transition-colors hover:bg-white/[0.04]">
          <div className="flex h-9 w-9 items-center justify-center rounded-full bg-gradient-to-br from-cyan-400 to-amber-400 text-sm font-bold text-white ring-2 ring-cyan-400/20">
            {user?.username?.[0]?.toUpperCase() ?? 'U'}
          </div>
          <div className="min-w-0 flex-1">
            <p className="truncate text-sm font-medium text-white">
              {user?.username ?? '用户'}
            </p>
            <p className="truncate text-xs text-surface-500">{user?.email ?? ''}</p>
          </div>
          <button
            onClick={handleLogout}
            className="rounded-lg p-1.5 text-surface-500 transition-all hover:bg-red-500/10 hover:text-red-400"
            title="退出登录"
          >
            <LogOut className="h-4 w-4" />
          </button>
        </div>
      </div>
    </aside>
  )
}
