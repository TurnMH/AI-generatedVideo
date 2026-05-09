'use client'

import type { Model } from '@/types'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Badge } from '@/components/ui/badge'

export function ModelSelector({
  label,
  icon,
  models,
  isLoading,
  value,
  onChange,
}: {
  label: string
  icon: React.ReactNode
  models: Model[]
  isLoading: boolean
  value: number | undefined
  onChange: (v: number | undefined) => void
}) {
  return (
    <div className="space-y-2">
      <Label className="flex items-center gap-2">
        {icon}
        {label}
      </Label>
      {isLoading ? (
        <div className="h-10 animate-pulse rounded-md bg-surface-100" />
      ) : models.length === 0 ? (
        <p className="text-sm text-surface-400">暂无可用模型</p>
      ) : (
        <Select
          value={value?.toString() || ''}
          onValueChange={(v) => onChange(v ? Number(v) : undefined)}
        >
          <SelectTrigger>
            <SelectValue placeholder={`选择${label}`} />
          </SelectTrigger>
          <SelectContent>
            {models.map((m) => (
              <SelectItem key={m.id} value={m.id.toString()}>
                <div className="flex items-center gap-2">
                  <span>{m.name}</span>
                  <span className="text-xs text-surface-400">({m.provider})</span>
                  {m.is_default && (
                    <Badge variant="outline" className="px-1 py-0 text-[10px]">
                      默认
                    </Badge>
                  )}
                </div>
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      )}
    </div>
  )
}
