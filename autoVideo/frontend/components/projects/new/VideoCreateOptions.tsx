'use client'

import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import {
  FRIENDLY_ASPECT_OPTIONS,
  FRIENDLY_CONSISTENCY_OPTIONS,
  FRIENDLY_DURATION_OPTIONS,
} from '@/lib/projects/new/video-create-ui'

type VideoCreateOptionsProps = {
  aspectRatio: string
  duration: number
  consistencyStrength: number
  enableDubbing: boolean
  enableSubtitle: boolean
  onAspectRatioChange: (value: string) => void
  onDurationChange: (value: number) => void
  onConsistencyChange: (value: number) => void
  onDubbingChange: (value: boolean) => void
  onSubtitleChange: (value: boolean) => void
}

export function VideoCreateOptions({
  aspectRatio,
  duration,
  consistencyStrength,
  enableDubbing,
  enableSubtitle,
  onAspectRatioChange,
  onDurationChange,
  onConsistencyChange,
  onDubbingChange,
  onSubtitleChange,
}: VideoCreateOptionsProps) {
  return (
    <div className="space-y-5">
      <div className="space-y-2">
        <Label className="text-sm">视频形状</Label>
        <div className="flex flex-wrap gap-2">
          {FRIENDLY_ASPECT_OPTIONS.map((option) => {
            const active = aspectRatio === option.value
            return (
              <button
                key={option.value}
                type="button"
                onClick={() => onAspectRatioChange(option.value)}
                className={`rounded-2xl border px-4 py-3 text-left ${
                  active ? 'border-primary-300 bg-primary-50' : 'border-surface-200 bg-white'
                }`}
              >
                <p className="text-sm font-medium text-surface-900">{option.label}</p>
                <p className="mt-0.5 text-xs text-surface-500">{option.hint}</p>
              </button>
            )
          })}
        </div>
      </div>

      <div className="space-y-2">
        <Label className="text-sm">每段大约多长</Label>
        <div className="flex flex-wrap gap-2">
          {FRIENDLY_DURATION_OPTIONS.map((seconds) => {
            const active = duration === seconds
            return (
              <button
                key={seconds}
                type="button"
                onClick={() => onDurationChange(seconds)}
                className={`rounded-full border px-4 py-2 text-sm ${
                  active ? 'border-primary-300 bg-primary-50 text-primary-700' : 'border-surface-200 text-surface-700'
                }`}
              >
                {seconds} 秒
              </button>
            )
          })}
        </div>
      </div>

      <div className="space-y-2">
        <Label className="text-sm">角色长相稳定度</Label>
        <div className="flex flex-wrap gap-2">
          {FRIENDLY_CONSISTENCY_OPTIONS.map((option) => {
            const active = consistencyStrength === option.value
            return (
              <button
                key={option.value}
                type="button"
                onClick={() => onConsistencyChange(option.value)}
                className={`rounded-full border px-4 py-2 text-sm ${
                  active ? 'border-primary-300 bg-primary-50 text-primary-700' : 'border-surface-200 text-surface-700'
                }`}
              >
                {option.label}
              </button>
            )
          })}
        </div>
      </div>

      <div className="flex flex-col gap-3 rounded-2xl border border-surface-200 bg-surface-50/80 p-4 sm:flex-row sm:gap-8">
        <div className="flex items-center gap-3">
          <Switch checked={enableDubbing} onCheckedChange={onDubbingChange} />
          <div>
            <p className="text-sm font-medium text-surface-900">自动配音</p>
            <p className="text-xs text-surface-500">给台词和旁白生成声音</p>
          </div>
        </div>
        <div className="flex items-center gap-3">
          <Switch checked={enableSubtitle} onCheckedChange={onSubtitleChange} />
          <div>
            <p className="text-sm font-medium text-surface-900">显示字幕</p>
            <p className="text-xs text-surface-500">成片里带上文字</p>
          </div>
        </div>
      </div>
    </div>
  )
}
