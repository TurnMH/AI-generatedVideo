'use client'

import { Loader2 } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { STATUS_MAP } from '@/lib/projects/status'

export function StatusBadge({ status }: { status: string }) {
  const cfg = STATUS_MAP[status] || { label: status, variant: 'secondary' as const }
  return <Badge variant={cfg.variant}>{cfg.label}</Badge>
}

export function VideoTaskStatusBadge({ status }: { status: string }) {
  const cfg = STATUS_MAP[status] || { label: status, variant: 'secondary' as const }
  const icon: React.ReactNode =
    status === 'pending'    ? <span className="mr-1 text-[11px]">⏳</span> :
    status === 'processing' ? <Loader2 className="mr-1 h-3 w-3 animate-spin" /> :
    status === 'succeeded'  ? <span className="mr-1 text-[11px]">✅</span> :
    status === 'failed'     ? <span className="mr-1 text-[11px]">❌</span> :
    status === 'paused'     ? <span className="mr-1 text-[11px]">⏸</span> :
    null
  return (
    <Badge variant={cfg.variant} className="flex items-center">
      {icon}
      {cfg.label}
    </Badge>
  )
}
