'use client'

import {
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
} from '@/components/ui/dropdown-menu'
import { getProviderLabel } from '@/lib/model-feasibility'
import type { ImageModelOption } from '@/lib/model-display'

export function ImageModelDropdownContent({
  options,
  availability,
  onSelect,
  label = '选择生成模型',
  align = 'end',
  showTags = false,
  stopPropagation = false,
}: {
  options: ImageModelOption[]
  availability: Record<string, boolean>
  onSelect: (key: string) => void
  label?: string
  align?: 'end' | 'start'
  showTags?: boolean
  stopPropagation?: boolean
}) {
  return (
    <DropdownMenuContent
      align={align}
      className="w-72 max-h-[70vh] overflow-y-auto"
      onClick={stopPropagation ? (e) => e.stopPropagation() : undefined}
    >
      <DropdownMenuLabel className="text-[10px] text-surface-400">{label}</DropdownMenuLabel>
      <DropdownMenuSeparator />
      {options.map((m, idx) => {
        const avail = availability[m.key]
        const broken = !!m.failureReason
        return (
          <DropdownMenuItem
            key={m.key}
            disabled={broken}
            title={broken ? `已停用：${m.failureReason}` : undefined}
            className={`cursor-pointer px-3 py-2 ${avail === false ? 'opacity-50' : ''} ${broken ? 'cursor-not-allowed opacity-40' : ''}`}
            onClick={() => broken ? undefined : onSelect(m.key)}
          >
            <div className="flex w-full items-start gap-2">
              <div className="mt-0.5 flex flex-col items-center gap-0.5">
                <span className="text-sm">{m.icon}</span>
                <span className="rounded-full bg-surface-200 px-1 text-[8px] text-surface-500 font-bold">#{idx + 1}</span>
              </div>
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-1.5 flex-wrap">
                  <span className="text-xs font-semibold">{m.label}</span>
                  {!broken && m.speed === 'fast' && <span className="rounded bg-green-100 px-1 py-0 text-[9px] text-green-700">⚡ 快</span>}
                  {!broken && m.quality === 'high' && <span className="rounded bg-blue-100 px-1 py-0 text-[9px] text-blue-700">★ 高质</span>}
                  {!broken && avail === true && <span className="rounded bg-emerald-100 px-1 py-0 text-[9px] text-emerald-700">● 可用</span>}
                  {!broken && avail === false && <span className="rounded bg-red-100 px-1 py-0 text-[9px] text-red-600">● 未配置</span>}
                  {broken && <span className="rounded bg-red-100 px-1 py-0 text-[9px] text-red-600">⚠ 已停用</span>}
                </div>
                <p className="text-[10px] text-surface-400 leading-none mt-0.5">{getProviderLabel(m.provider)}</p>
                {broken ? (
                  <p className="mt-0.5 text-[9px] text-red-400 leading-tight">{m.failureReason}</p>
                ) : (
                  <>
                    <p className="mt-0.5 text-[10px] text-surface-500 leading-tight">{m.desc}</p>
                    {showTags && (
                      <div className="mt-1 flex flex-wrap gap-1">
                        {m.tags.map(t => (
                          <span key={t} className="rounded-full bg-surface-100 px-1.5 py-0 text-[9px] text-surface-500">{t}</span>
                        ))}
                      </div>
                    )}
                  </>
                )}
              </div>
            </div>
          </DropdownMenuItem>
        )
      })}
    </DropdownMenuContent>
  )
}
