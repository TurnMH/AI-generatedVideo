import { Loader2 } from 'lucide-react'

/**
 * Next.js App Router route-level loading UI.
 * Shown automatically during page-level Suspense (navigation + slow server components).
 */
export default function DashboardLoading() {
  return (
    <div className="flex flex-1 items-center justify-center">
      <div className="flex flex-col items-center gap-3">
        <Loader2 className="h-8 w-8 animate-spin text-primary-400" />
        <p className="text-sm text-surface-400">加载中…</p>
      </div>
    </div>
  )
}
