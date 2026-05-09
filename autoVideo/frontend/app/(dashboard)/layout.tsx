'use client'

import { useEffect } from 'react'
import { useRouter, usePathname } from 'next/navigation'
import { Loader2 } from 'lucide-react'
import { Sidebar } from '@/components/layout/sidebar'
import { Header } from '@/components/layout/header'
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
      <div className="flex h-screen items-center justify-center bg-gradient-to-br from-surface-50 via-primary-50/30 to-accent-50/20">
        <div className="flex flex-col items-center gap-3">
          <Loader2 className="h-8 w-8 animate-spin text-primary-400" />
          <p className="text-sm text-surface-400">正在初始化…</p>
        </div>
      </div>
    )
  }

  // Hydrated but not authenticated — redirect is in-flight, render nothing
  if (!isAuthenticated) {
    return null
  }

  return (
    <div className="flex h-screen overflow-hidden bg-gradient-to-br from-surface-50 via-primary-50/30 to-accent-50/20">
      <Sidebar />
      <div className="flex flex-1 flex-col overflow-hidden">
        <Header />
        <main className="flex-1 overflow-y-auto p-6">{children}</main>
      </div>
    </div>
  )
}
