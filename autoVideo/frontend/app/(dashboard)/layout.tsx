'use client'

import { useEffect } from 'react'
import { useRouter, usePathname } from 'next/navigation'
import { Loader2 } from 'lucide-react'
import { Sidebar } from '@/components/layout/sidebar'
import { Header } from '@/components/layout/header'
import { SceneBackground } from '@/components/layout/scene-background'
import { useAuthStore } from '@/lib/store/auth'

export default function DashboardLayout({ children }: { children: React.ReactNode }) {
  const router = useRouter()
  const pathname = usePathname()
  const { isAuthenticated, isHydrated } = useAuthStore()

  useEffect(() => {
    if (isHydrated && !isAuthenticated) {
      router.replace(`/login?redirect=${encodeURIComponent(pathname)}`)
    }
  }, [isHydrated, isAuthenticated, router, pathname])

  // Waiting for zustand to rehydrate from localStorage
  if (!isHydrated) {
    return (
      <div className="relative flex h-screen items-center justify-center overflow-hidden bg-slate-950">
        <SceneBackground />
        <div className="relative z-10 flex flex-col items-center gap-3 rounded-[28px] border border-white/10 bg-slate-950/45 px-8 py-7 backdrop-blur-2xl">
          <Loader2 className="h-8 w-8 animate-spin text-cyan-300" />
          <p className="text-sm text-slate-200">正在初始化…</p>
        </div>
      </div>
    )
  }

  // Hydrated but not authenticated — redirect is in-flight, render nothing
  if (!isAuthenticated) {
    return null
  }

  return (
    <div className="relative flex h-screen overflow-hidden bg-slate-950 text-slate-100">
      <SceneBackground />
      <div className="absolute inset-0 bg-[linear-gradient(180deg,rgba(2,6,23,0.2),rgba(2,6,23,0.48))]" />
      <div className="relative z-10 flex h-full w-full overflow-hidden">
        <Sidebar />
        <div className="flex flex-1 flex-col overflow-hidden">
          <Header />
          <main className="flex-1 overflow-y-auto p-6">{children}</main>
        </div>
      </div>
    </div>
  )
}
