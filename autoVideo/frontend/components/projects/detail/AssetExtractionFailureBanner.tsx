'use client'

import { AlertTriangle, Loader2, RefreshCw } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import type { Model } from '@/types'

export function AssetExtractionFailureBanner({
  errorMessage,
  modelName,
  textModels,
  selectedModelKey,
  onSelectedModelKeyChange,
  onRetry,
  retrying,
}: {
  errorMessage: string
  modelName?: string
  textModels: Model[]
  selectedModelKey: string
  onSelectedModelKeyChange: (value: string) => void
  onRetry: () => void
  retrying?: boolean
}) {
  return (
    <div className="mb-4 rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-800">
      <div className="flex flex-wrap items-start gap-3">
        <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0 text-red-600" />
        <div className="min-w-0 flex-1 space-y-2">
          <div>
            <p className="font-medium">资源提取失败</p>
            <p className="mt-1 text-xs leading-5 text-red-700">{errorMessage}</p>
            {modelName ? (
              <p className="mt-1 text-[11px] text-red-600">上次使用模型：{modelName}</p>
            ) : null}
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <Select value={selectedModelKey} onValueChange={onSelectedModelKeyChange}>
              <SelectTrigger className="h-8 w-[220px] bg-white text-xs">
                <SelectValue placeholder="选择提取模型" />
              </SelectTrigger>
              <SelectContent>
                {textModels.map((model) => (
                  <SelectItem key={model.id} value={model.model_key || model.name}>
                    {model.name}{model.is_default ? '（默认）' : ''}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Button size="sm" variant="outline" onClick={onRetry} disabled={retrying || !selectedModelKey} className="h-8 border-red-200 bg-white text-red-700 hover:bg-red-100">
              {retrying ? <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" /> : <RefreshCw className="mr-1.5 h-3.5 w-3.5" />}
              切换模型并重试
            </Button>
          </div>
        </div>
      </div>
    </div>
  )
}

export function AssetExtractionProgressBanner() {
  return (
    <div className="mb-4 inline-flex items-center gap-2 rounded-full border border-primary-200 bg-primary-50 px-3 py-1.5 text-xs text-primary-700">
      <Loader2 className="h-3.5 w-3.5 animate-spin" />
      正在提取资源，请稍候…
    </div>
  )
}

export function AssetGenerationFailureBanner({
  failedCount,
  sampleErrors,
  onShowFailed,
  onRetryFailed,
  retrying,
}: {
  failedCount: number
  sampleErrors: string[]
  onShowFailed: () => void
  onRetryFailed: () => void
  retrying?: boolean
}) {
  if (failedCount <= 0) return null
  return (
    <div className="mb-4 rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-800">
      <div className="flex flex-wrap items-start gap-3">
        <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0 text-red-600" />
        <div className="min-w-0 flex-1 space-y-2">
          <div>
            <p className="font-medium">资源生成失败（{failedCount} 个）</p>
            {sampleErrors.length > 0 ? (
              <ul className="mt-1 space-y-0.5 text-xs leading-5 text-red-700">
                {sampleErrors.map((error) => (
                  <li key={error}>· {error}</li>
                ))}
              </ul>
            ) : (
              <p className="mt-1 text-xs leading-5 text-red-700">部分资源图片生成失败，请查看下方卡片或重试。</p>
            )}
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <Button size="sm" variant="outline" onClick={onShowFailed} className="h-8 border-red-200 bg-white text-red-700 hover:bg-red-100">
              查看失败资源
            </Button>
            <Button size="sm" variant="outline" onClick={onRetryFailed} disabled={retrying} className="h-8 border-red-200 bg-white text-red-700 hover:bg-red-100">
              {retrying ? <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" /> : <RefreshCw className="mr-1.5 h-3.5 w-3.5" />}
              重试失败 ({failedCount})
            </Button>
          </div>
        </div>
      </div>
    </div>
  )
}
