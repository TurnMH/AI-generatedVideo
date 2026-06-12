'use client'

import { Check } from 'lucide-react'
import { GENRE_PICKER_INTRO, type QuickGenreOption } from '@/lib/projects/new/video-create-ui'

type VideoGenrePickerProps = {
  options: QuickGenreOption[]
  selectedKey: string
  onSelect: (presetKey: string) => void
}

export function VideoGenrePicker({ options, selectedKey, onSelect }: VideoGenrePickerProps) {
  const selected = options.find((option) => option.presetKey === selectedKey)

  return (
    <div className="space-y-3 rounded-2xl border border-surface-200 bg-white p-4">
      <div>
        <p className="text-sm font-medium text-surface-900">{GENRE_PICKER_INTRO.title}</p>
        <p className="mt-1 text-xs leading-5 text-surface-500">{GENRE_PICKER_INTRO.desc}</p>
      </div>

      <div className="grid gap-2 sm:grid-cols-2">
        {options.map((option) => {
          const active = selectedKey === option.presetKey
          return (
            <button
              key={option.presetKey}
              type="button"
              onClick={() => onSelect(option.presetKey)}
              className={`rounded-2xl border px-4 py-3 text-left transition-colors ${
                active
                  ? 'border-primary-300 bg-primary-50'
                  : 'border-surface-200 bg-surface-50/50 hover:border-surface-300 hover:bg-white'
              }`}
            >
              <div className="flex items-center justify-between gap-2">
                <span className="text-sm font-medium text-surface-900">{option.label}</span>
                {active ? <Check className="h-4 w-4 shrink-0 text-primary-500" /> : null}
              </div>
              <p className="mt-1 text-xs text-surface-500">{option.hint}</p>
            </button>
          )
        })}
      </div>

      {selected ? (
        <div className="rounded-xl border border-primary-200 bg-primary-50/60 p-3">
          <p className="text-xs font-medium text-primary-800">{GENRE_PICKER_INTRO.selectedTitle}</p>
          <p className="mt-1 text-xs leading-5 text-primary-700">{selected.effect}</p>
          <ul className="mt-2 grid gap-1 sm:grid-cols-2">
            {GENRE_PICKER_INTRO.effects.map((item) => (
              <li key={item} className="text-[11px] text-primary-600 before:mr-1 before:content-['·']">
                {item}
              </li>
            ))}
          </ul>
        </div>
      ) : (
        <p className="text-xs text-surface-400">不选也可以，系统会用通用默认画风。</p>
      )}
    </div>
  )
}
