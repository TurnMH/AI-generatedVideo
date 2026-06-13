'use client'

import { AlertCircle, Loader2, Sparkles } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { getProviderLabel } from '@/lib/model-feasibility'
import type { ImageModelOption } from '@/lib/model-display'
import type { Episode } from '@/types'

type BatchActionKind = 'generate' | 'force' | 'retryFailed'

export function BatchStoryboardDialog({
  open,
  onOpenChange,
  batchActionKind,
  selectedEpisode,
  storyboardItemLabel,
  storyboardImageLabel,
  modelOptions,
  imageModelAvailability,
  selectedModels,
  onSelectedModelsChange,
  defaultImageModelLabel,
  running,
  onConfirm,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  batchActionKind: BatchActionKind
  selectedEpisode: Episode | null
  storyboardItemLabel: string
  storyboardImageLabel: string
  modelOptions: ImageModelOption[]
  imageModelAvailability: Record<string, boolean>
  selectedModels: string[]
  onSelectedModelsChange: (models: string[]) => void
  defaultImageModelLabel: string
  running: boolean
  onConfirm: () => void
}) {
  const title =
    batchActionKind === 'retryFailed'
      ? (selectedEpisode ? `第 ${selectedEpisode.episode_number} 集 · 重试失败${storyboardItemLabel}` : `批量重试失败${storyboardItemLabel}`)
      : batchActionKind === 'force'
        ? (selectedEpisode ? `第 ${selectedEpisode.episode_number} 集 · 重生成${storyboardImageLabel}` : `重生成${storyboardImageLabel}`)
        : (selectedEpisode ? `第 ${selectedEpisode.episode_number} 集 · 生成${storyboardImageLabel}` : `批量生成${storyboardImageLabel}`)

  const confirmLabel =
    batchActionKind === 'retryFailed'
      ? '开始重试'
      : batchActionKind === 'force'
        ? '确认重生成图'
        : '开始生成'

  const modelSections = [
    { label: '🌐 多模态推荐', filter: (m: ImageModelOption) => m.tags.includes('多模态') },
    { label: '🎨 高质量文生图', filter: (m: ImageModelOption) => m.tags.includes('高质量') && !m.tags.includes('多模态') },
    { label: '⚡ 高速 / 低成本', filter: (m: ImageModelOption) => !m.tags.includes('多模态') && !m.tags.includes('高质量') && !m.tags.includes('本地') },
    { label: '🖥️ 本地部署', filter: (m: ImageModelOption) => m.tags.includes('本地') },
  ]

  return (
    <Dialog open={open} onOpenChange={(next) => { if (!running) onOpenChange(next) }}>
      <DialogContent className="flex max-h-[85vh] flex-col gap-0 overflow-hidden p-0 sm:max-w-3xl">
        <DialogHeader className="shrink-0 border-b border-surface-100 px-6 py-4">
          <DialogTitle>{title}</DialogTitle>
        </DialogHeader>
        <div className="min-h-0 flex-1 space-y-4 overflow-y-auto px-6 py-4">
          {batchActionKind === 'force' && (
            <div className="flex items-start gap-2 rounded-lg border border-amber-200 bg-amber-50 px-3 py-3">
              <AlertCircle className="mt-0.5 h-4 w-4 flex-shrink-0 text-amber-600" />
              <p className="text-[12px] leading-5 text-amber-800">
                这里是“重生成图”，不是“重新拆分镜头”。单模型会按原流程覆盖刷新当前{storyboardImageLabel}；如果同时选择多个模型，系统会为同一条{storyboardItemLabel}追加多版候选图，保留当前结果供你稍后挑选。
              </p>
            </div>
          )}
          <div className="rounded-lg border border-sky-200 bg-sky-50 px-3 py-3 text-[12px] leading-5 text-sky-800">
            这里支持同时选择多个模型。选择多个模型时，同一条{storyboardItemLabel}会为每个模型各生成一版候选图，完成后你可以在列表里切换版本并保留最满意的一张。
          </div>
          <div className="space-y-3">
            <Label className="text-xs font-medium">生成模型（可多选）</Label>
            {modelSections.map(({ label: sectionLabel, filter }) => {
              const models = modelOptions.filter(filter)
              if (models.length === 0) return null
              return (
                <div key={sectionLabel}>
                  <p className="mb-1.5 text-[10px] font-semibold uppercase tracking-wide text-surface-400">{sectionLabel}</p>
                  <div className="grid gap-2 sm:grid-cols-2">
                    {models.map((m) => {
                      const avail = imageModelAvailability[m.key]
                      const selected = selectedModels.includes(m.key)
                      const broken = !!m.failureReason
                      return (
                        <button
                          key={m.key}
                          type="button"
                          title={broken ? `已停用：${m.failureReason}` : undefined}
                          onClick={() => {
                            if (avail === false || broken) return
                            onSelectedModelsChange(
                              selected
                                ? selectedModels.filter((item) => item !== m.key)
                                : [...selectedModels, m.key]
                            )
                          }}
                          className={`flex items-start gap-2 rounded-lg border p-2.5 text-left transition-colors ${
                            broken
                              ? 'cursor-not-allowed border-red-200 bg-red-50 opacity-60'
                              : selected
                                ? 'border-primary-400 bg-primary-50 ring-1 ring-primary-400'
                                : avail === false
                                  ? 'border-surface-200 bg-surface-50 opacity-50'
                                  : 'border-surface-200 bg-white hover:border-surface-300'
                          }`}
                        >
                          <span className="mt-0.5 text-base">{m.icon}</span>
                          <div className="min-w-0 flex-1">
                            <div className="flex flex-wrap items-center gap-1">
                              <span className="text-xs font-semibold">{m.label}</span>
                              {selected && <span className="rounded bg-primary-100 px-1 text-[9px] text-primary-700">已选</span>}
                              {!broken && m.speed === 'fast' && <span className="rounded bg-green-100 px-1 text-[9px] text-green-700">⚡ 快</span>}
                              {!broken && m.quality === 'high' && <span className="rounded bg-blue-100 px-1 text-[9px] text-blue-700">★ 高质</span>}
                              {!broken && avail === true && <span className="rounded bg-emerald-100 px-1 text-[9px] text-emerald-700">● 可用</span>}
                              {!broken && avail === false && <span className="rounded bg-red-100 px-1 text-[9px] text-red-600">● 未配置</span>}
                              {broken && <span className="rounded bg-red-100 px-1 text-[9px] text-red-600">⚠ 已停用</span>}
                            </div>
                            <p className="text-[10px] text-surface-400">{getProviderLabel(m.provider)}</p>
                            {broken ? (
                              <p className="mt-0.5 text-[9px] leading-snug text-red-400">{m.failureReason}</p>
                            ) : (
                              <>
                                <p className="mt-0.5 text-[9px] leading-snug text-surface-500">{m.desc}</p>
                                <div className="mt-1 flex flex-wrap gap-1">
                                  {m.tags.map((tag) => (
                                    <span key={tag} className="rounded-full bg-surface-100 px-1.5 py-0 text-[9px] text-surface-500">{tag}</span>
                                  ))}
                                </div>
                              </>
                            )}
                          </div>
                        </button>
                      )
                    })}
                  </div>
                </div>
              )
            })}
            {selectedModels.length === 0 ? (
              <p className="text-[11px] text-surface-400">未选择时将使用项目默认图片模型：{defaultImageModelLabel}</p>
            ) : selectedModels.length === 1 ? (
              <p className="text-[11px] text-surface-400">已选 1 个模型；当前批次将统一使用该模型。</p>
            ) : (
              <p className="text-[11px] text-surface-400">已选 {selectedModels.length} 个模型；同一条{storyboardItemLabel}会为每个模型各生成一版候选图，后续可在列表里切换版本。</p>
            )}
          </div>
        </div>
        <div className="shrink-0 flex justify-end gap-2 border-t border-surface-100 px-6 py-4">
          <Button variant="outline" onClick={() => onOpenChange(false)} disabled={running}>
            取消
          </Button>
          <Button onClick={onConfirm} disabled={running}>
            {running ? (
              <Loader2 className="mr-1.5 h-4 w-4 animate-spin" />
            ) : (
              <Sparkles className="mr-1.5 h-4 w-4" />
            )}
            {confirmLabel}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  )
}
