'use client'

import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'

export type VoicePickerOption = {
  value: string
  label: string
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
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
        </DialogHeader>
        <div className="space-y-3">
          <Input
            value={search}
            onChange={(e) => onSearchChange(e.target.value)}
            placeholder="搜索音色名称 / key / 风格 / 分类"
            className="h-9"
          />
          <div className="max-h-[55vh] overflow-y-auto rounded-md border border-surface-200">
            <div className="divide-y divide-surface-100">
              {options.map((option) => (
                <button
                  key={option.value}
                  type="button"
                  className={`flex w-full items-center justify-between gap-3 px-3 py-2 text-left hover:bg-surface-50 ${selectedValue === option.value ? 'bg-primary-50' : 'bg-white'}`}
                  onClick={() => onSelect(option.value)}
                >
                  <div className="min-w-0 flex-1">
                    <div className="truncate text-sm text-surface-800">{option.label}</div>
                    <div className="truncate text-[11px] text-surface-400">{option.value || '未绑定音色'}</div>
                  </div>
                  {onPreview && option.value && option.value !== 'auto' ? (
                    <Button
                      type="button"
                      size="sm"
                      variant="outline"
                      className="h-7 px-2 text-[10px]"
                      onClick={async (e) => {
                        e.stopPropagation()
                        await onPreview(option.value)
                      }}
                    >
                      试听
                    </Button>
                  ) : null}
                </button>
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
