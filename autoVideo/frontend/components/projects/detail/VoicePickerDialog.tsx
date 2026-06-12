'use client'

import * as React from 'react'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'

export type VoicePickerOption = {
  value: string
  label: string
  provider?: string
  voiceName?: string
  locale?: string
  gender?: string
  style?: string
  autoAssignable?: boolean
  category?: string
  recommended?: boolean
}

function translateVoiceMeta(value?: string) {
  const raw = (value || '').trim()
  if (!raw) return ''
  const map: Record<string, string> = {
    male: '男',
    female: '女',
    child: '儿童',
    narrator: '旁白',
    calm: '沉稳',
    deep: '低沉',
    warm: '温暖',
    bright: '明亮',
    multilingual: '多语',
    mainland: '大陆',
    mandarin: '普通话',
    regional: '方言',
    auto: '自动',
    assignable: '可自动分配',
  }
  return raw.split(/[-_\s/]+/).map(part => map[part.toLowerCase()] || part).join(' / ')
}

function getVoiceGroupLabel(option: VoicePickerOption) {
  if (option.recommended) return '推荐音色'
  if (option.category) return translateVoiceMeta(option.category)
  if (option.locale) return translateVoiceMeta(option.locale)
  return '其他音色'
}

function compareVoiceOption(a: VoicePickerOption, b: VoicePickerOption) {
  const score = (option: VoicePickerOption) => {
    let total = 0
    if (option.recommended) total += 100
    if (option.autoAssignable) total += 20
    if (option.category === 'mandarin') total += 8
    if (option.locale === 'mainland') total += 4
    if (!option.value || option.value === 'auto') total -= 50
    return total
  }
  const diff = score(b) - score(a)
  if (diff !== 0) return diff
  return a.label.localeCompare(b.label, 'zh-CN')
}

export function VoicePickerDialog({
  open,
  onOpenChange,
  title,
  search,
  onSearchChange,
  options,
  selectedValue,
  previewAudioUrl,
  onSelect,
  onPreview,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: string
  search: string
  onSearchChange: (value: string) => void
  options: VoicePickerOption[]
  selectedValue?: string
  previewAudioUrl?: string
  onSelect: (value: string) => void | Promise<void>
  onPreview?: (value: string) => void | Promise<void>
}) {
  const groupedOptions = React.useMemo(() => {
    const groups = new Map<string, VoicePickerOption[]>()
    const sorted = [...options].sort(compareVoiceOption)
    for (const option of sorted) {
      const key = getVoiceGroupLabel(option)
      const current = groups.get(key) ?? []
      current.push(option)
      groups.set(key, current)
    }
    return Array.from(groups.entries())
  }, [options])

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription className="sr-only">搜索并选择配音音色，可试听预览。</DialogDescription>
        </DialogHeader>
        <div className="space-y-3">
          <Input
            value={search}
            onChange={(e) => onSearchChange(e.target.value)}
            placeholder="搜索音色名称 / key / 风格 / 分类"
            className="h-9"
          />
          <div className="max-h-[60vh] overflow-y-auto rounded-md border border-surface-200 bg-surface-50/40 px-2 py-2">
            <div className="space-y-3">
              {groupedOptions.map(([group, groupOptions]) => (
                <div key={group} className="space-y-1.5">
                  <div className="sticky top-0 z-10 flex items-center gap-2 bg-surface-50/95 px-1 py-1 backdrop-blur">
                    <span className="text-xs font-medium text-surface-600">{group}</span>
                    <Badge variant="secondary" className="h-5 px-1.5 text-[10px]">
                      {groupOptions.length}
                    </Badge>
                  </div>
                  <div className="overflow-hidden rounded-md border border-surface-200 bg-white">
                    <div className="divide-y divide-surface-100">
                      {groupOptions.map((option) => (
                        <div
                          key={option.value || '__empty__'}
                          className={`flex w-full items-center justify-between gap-3 px-3 py-2 ${selectedValue === option.value ? 'bg-primary-50' : 'bg-white'}`}
                        >
                          <button
                            type="button"
                            className="min-w-0 flex-1 space-y-1 text-left hover:opacity-90"
                            onClick={() => onSelect(option.value)}
                          >
                            <div className="flex flex-wrap items-center gap-1.5">
                              <div className="truncate text-sm text-surface-800">{option.label}</div>
                              {option.recommended ? <Badge className="h-5 px-1.5 text-[10px]">推荐</Badge> : null}
                              {option.autoAssignable ? <Badge variant="outline" className="h-5 px-1.5 text-[10px]">可自动分配</Badge> : null}
                              {option.category ? <Badge variant="secondary" className="h-5 px-1.5 text-[10px]">{translateVoiceMeta(option.category)}</Badge> : null}
                              {option.gender ? <Badge variant="secondary" className="h-5 px-1.5 text-[10px]">{translateVoiceMeta(option.gender)}</Badge> : null}
                              {option.style ? <Badge variant="secondary" className="h-5 px-1.5 text-[10px]">{translateVoiceMeta(option.style)}</Badge> : null}
                            </div>
                            <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-[11px] text-surface-400">
                              <span>{option.value || '未绑定音色'}</span>
                              {option.locale ? <span>{translateVoiceMeta(option.locale)}</span> : null}
                              {option.provider ? <span>{option.provider}</span> : null}
                              {option.voiceName ? <span>{translateVoiceMeta(option.voiceName)}</span> : null}
                            </div>
                          </button>
                          {onPreview && option.value && option.value !== 'auto' ? (
                            <Button
                              type="button"
                              size="sm"
                              variant="outline"
                              className="h-7 shrink-0 px-2 text-[10px]"
                              onClick={async () => {
                                await onPreview(option.value)
                              }}
                            >
                              试听
                            </Button>
                          ) : null}
                        </div>
                      ))}
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </div>
          {previewAudioUrl ? (
            <audio controls className="h-8 w-full" src={previewAudioUrl} />
          ) : null}
        </div>
      </DialogContent>
    </Dialog>
  )
}
