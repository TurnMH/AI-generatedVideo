'use client'

import { useState } from 'react'
import { assetAPI } from '@/lib/api'
import type { Asset } from '@/types'
import { ZoomableImage } from '@/components/ui/image-lightbox'
import { CheckCircle2 } from 'lucide-react'

const CHARACTER_PANEL_META: Array<{ key: 'closeup' | 'front' | 'side' | 'back'; label: string }> = [
  { key: 'front', label: '正面' },
  { key: 'closeup', label: '特写' },
  { key: 'side', label: '侧面' },
  { key: 'back', label: '背面' },
]

export function CharacterPanelStrip({
  projectId,
  asset,
  onRegenQueued,
  onSetPrimary,
}: {
  projectId: number
  asset: Asset
  onRegenQueued?: () => void
  onSetPrimary?: (url: string) => void
}) {
  const panelUrls = (asset.panel_images ?? []) as string[]
  const [busyPanel, setBusyPanel] = useState<string | null>(null)
  const primaryUrl = (asset.metadata?.selected_generated_image_url as string | undefined)?.trim() || asset.image_url?.trim() || ''

  const handleRegen = async (panel: 'closeup' | 'front' | 'side' | 'back') => {
    if (busyPanel) return
    setBusyPanel(panel)
    try {
      await assetAPI.regenPanel(projectId, asset.id, panel)
      onRegenQueued?.()
    } catch (err) {
      console.error('regen panel failed', err)
    } finally {
      setBusyPanel(null)
    }
  }

  return (
    <div className="mt-3 rounded-xl border border-surface-100 bg-surface-50/70 p-3">
      <div className="mb-2 flex items-center justify-between text-[11px] text-surface-500">
        <span>四视图 · 点击图片可设为主图</span>
        <div className="flex items-center gap-2">
          {asset.panel_validation && (
            <span
              className={`rounded px-1.5 py-0.5 text-[10px] font-semibold ${asset.panel_validation.overall_pass ? 'bg-green-100 text-green-700' : 'bg-red-100 text-red-700'}`}
              title={asset.panel_validation.summary}
            >
              AI质检 {asset.panel_validation.overall_pass ? '✓ 通过' : '✗ 未通过'}
            </span>
          )}
          {typeof asset.seed === 'number' && asset.seed > 0 ? (
            <span className="font-mono text-[10px] text-surface-400">seed {asset.seed}</span>
          ) : null}
        </div>
      </div>
      <div className="grid grid-cols-2 gap-2">
        {CHARACTER_PANEL_META.map((meta, idx) => {
          const url = panelUrls[idx]?.trim() || ''
          const isBusy = busyPanel === meta.key
          const isPrimary = !!url && url === primaryUrl
          const vqa = asset.panel_validation?.panels.find(p => p.panel === meta.key)
          return (
            <div key={meta.key} className="group relative overflow-hidden rounded-lg border border-surface-200 bg-white">
              <div className="aspect-[3/4] w-full bg-surface-100">
                {url ? (
                  <button
                    type="button"
                    className="h-full w-full cursor-pointer focus:outline-none"
                    title={`点击将「${meta.label}」设为主图`}
                    onClick={() => onSetPrimary?.(url)}
                  >
                    <ZoomableImage src={url} alt={`${asset.name} · ${meta.label}`} className="h-full w-full object-cover" />
                  </button>
                ) : (
                  <div className="flex h-full flex-col items-center justify-center gap-1 text-[10px] text-surface-400">
                    <span>（生成中）</span>
                  </div>
                )}
              </div>
              {/* 视角标签 */}
              <div className="absolute inset-x-0 top-0 flex items-center justify-between bg-gradient-to-b from-black/55 to-transparent px-2 py-1">
                <span className="text-[10px] font-medium text-white">{meta.label}</span>
                {isPrimary && (
                  <span className="flex items-center gap-0.5 rounded bg-primary-500/90 px-1.5 py-0.5 text-[9px] font-semibold text-white">
                    <CheckCircle2 className="h-2.5 w-2.5" />
                    主图
                  </span>
                )}
              </div>
              {/* AI 质检徽章 */}
              {vqa && (
                <div
                  className={`absolute inset-x-0 bottom-7 mx-1 rounded px-1.5 py-0.5 text-[9px] font-semibold leading-tight text-white opacity-0 transition-opacity group-hover:opacity-100 ${vqa.pass ? 'bg-green-600/85' : 'bg-red-600/85'}`}
                  title={vqa.issues?.length ? vqa.issues.join(' | ') : (vqa.pass ? '质检通过' : '质检未通过')}
                >
                  {vqa.pass ? `✓ 质检通过 · ${vqa.score}/10` : `✗ 未通过 · ${vqa.score}/10`}
                </div>
              )}
              <button
                type="button"
                onClick={() => handleRegen(meta.key)}
                disabled={isBusy}
                className="absolute inset-x-1 bottom-1 rounded-md bg-black/75 px-2 py-1 text-[10px] font-medium text-white opacity-0 transition-opacity group-hover:opacity-100 disabled:opacity-50"
                title={`用相同 seed 重绘 ${meta.label} 视角`}
              >
                {isBusy ? '提交中…' : '重绘本栏'}
              </button>
            </div>
          )
        })}
      </div>
    </div>
  )
}
