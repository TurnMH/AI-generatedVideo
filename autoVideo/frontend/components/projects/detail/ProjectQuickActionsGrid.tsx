'use client'

import type { ReactNode } from 'react'
import { RefreshCw } from 'lucide-react'
import { Button } from '@/components/ui/button'

export type ProjectQuickAction = {
  icon: ReactNode
  title: string
  desc: string
  label: string
  onClick: () => void
  loading?: boolean
  disabled?: boolean
}

export function ProjectQuickActionsGrid({ actions }: { actions: ProjectQuickAction[] }) {
  return (
    <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
      {actions.map((card) => (
        <div key={card.title} className="rounded-xl border border-white/15 bg-white/5 p-3">
          <div className="mb-1.5 flex items-center gap-2">
            {card.icon}
            <span className="text-sm font-semibold text-white">{card.title}</span>
          </div>
          <p className="mb-2 text-[11px] leading-relaxed text-surface-300">{card.desc}</p>
          <Button
            variant="outline"
            size="sm"
            onClick={card.onClick}
            disabled={card.disabled}
            className="w-full border-white/20 bg-white/10 text-xs text-white hover:bg-white/20"
          >
            {card.loading ? <RefreshCw className="mr-1.5 h-3 w-3 animate-spin" /> : null}
            {card.label}
          </Button>
        </div>
      ))}
    </div>
  )
}
