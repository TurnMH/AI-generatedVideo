'use client'

import { Check } from 'lucide-react'

export function StepIndicator({ current, steps }: { current: number; steps: string[] }) {
  return (
    <div className="flex flex-wrap items-center justify-center gap-2 sm:gap-3">
      {steps.map((label, i) => {
        const step = i + 1
        const isActive = step === current
        const isDone = step < current
        return (
          <div key={step} className="flex items-center gap-2">
            {i > 0 && (
              <div className={`h-px w-8 sm:w-12 ${isDone ? 'bg-primary-500' : 'bg-surface-300/80'}`} />
            )}
            <div className="flex items-center gap-2">
              <div
                className={`flex h-9 w-9 items-center justify-center rounded-full text-sm font-medium shadow-sm transition-colors ${
                  isActive
                    ? 'bg-gradient-to-br from-primary-600 to-violet-500 text-white'
                    : isDone
                    ? 'bg-primary-100 text-primary-700 ring-1 ring-primary-200'
                    : 'bg-white text-surface-400 ring-1 ring-surface-200'
                }`}
              >
                {isDone ? <Check className="h-4 w-4" /> : step}
              </div>
              <span
                className={`hidden text-sm sm:inline ${
                  isActive ? 'font-medium text-surface-900' : isDone ? 'text-surface-700' : 'text-surface-400'
                }`}
              >
                {label}
              </span>
            </div>
          </div>
        )
      })}
    </div>
  )
}
