'use client'

import Link from 'next/link'
import { Check, Clapperboard, Film, Mic } from 'lucide-react'
import {
  AD_VIDEO_CREATE_HREF,
  type VideoProductionMode,
  VIDEO_PRODUCTION_MODES,
} from '@/lib/projects/new/production-mode'

type ProductionModeSelectorProps = {
  value: VideoProductionMode
  onChange: (mode: VideoProductionMode) => void
}

export function ProductionModeSelector({ value, onChange }: ProductionModeSelectorProps) {
  return (
    <div className="space-y-3">
      <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
        <p className="text-sm font-medium text-surface-900">你想做哪种视频？</p>
        <Link
          href={AD_VIDEO_CREATE_HREF}
          className="text-xs text-amber-700 underline-offset-2 hover:underline"
        >
          要做口播广告？去广告工作台
        </Link>
      </div>

      <div className="grid gap-3 sm:grid-cols-2">
        {VIDEO_PRODUCTION_MODES.map((mode) => {
          const selected = mode.key === value
          const Icon = mode.key === 'commentary_comic' ? Mic : Film
          return (
            <button
              key={mode.key}
              type="button"
              onClick={() => onChange(mode.key)}
              className={`rounded-2xl border p-4 text-left transition-colors ${
                selected
                  ? 'border-primary-300 bg-primary-50 shadow-sm'
                  : 'border-surface-200 bg-white hover:border-surface-300'
              }`}
            >
              <div className="flex items-start justify-between gap-3">
                <div className="flex items-start gap-3">
                  <div
                    className={`flex h-10 w-10 items-center justify-center rounded-xl ${
                      selected ? 'bg-primary-100 text-primary-700' : 'bg-surface-100 text-surface-600'
                    }`}
                  >
                    <Icon className="h-5 w-5" />
                  </div>
                  <div>
                    <p className="text-sm font-semibold text-surface-900">{mode.shortLabel}</p>
                    <p className="mt-1 text-xs leading-5 text-surface-500">{mode.desc}</p>
                  </div>
                </div>
                {selected ? <Check className="h-4 w-4 shrink-0 text-primary-500" /> : null}
              </div>
            </button>
          )
        })}
      </div>
    </div>
  )
}
