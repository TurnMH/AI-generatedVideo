'use client'

import { useEffect, useMemo, useState } from 'react'
import {
  AlertCircle,
  ChevronDown,
  ChevronUp,
  LayoutGrid,
  Loader2,
  Sparkles,
  X,
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Badge } from '@/components/ui/badge'
import { StatusBadge } from '@/components/projects/detail/StatusBadge'
import { getProviderLabel } from '@/lib/model-feasibility'
import { canTriggerStoryboardImage } from '@/lib/projects/storyboard-image'
import { formatStoryboardErrorMessage } from '@/lib/projects/storyboard-status'
import type { ImageModelOption } from '@/lib/model-display'
import axios from 'axios'
import { storyboardAPI } from '@/lib/api'
import type { Asset, Project, Storyboard, StoryboardImageGenerationParams, StoryboardVersion } from '@/types'
import { buildStoryboardPromptSummaryZh, pickEditableChinesePrompt } from '@/lib/projects/prompt-display-zh'
import { inferLocationViewHint, LOCATION_VIEW_OPTIONS, locationViewLabel } from '@/lib/projects/location-zone'
import { ZoomableImage, ZoomBadge } from '@/components/ui/image-lightbox'

function formatPreviewError(err: unknown): string {
  if (axios.isAxiosError(err)) {
    const status = err.response?.status
    const message = typeof err.response?.data === 'object' && err.response?.data !== null && 'message' in err.response.data
      ? String((err.response.data as { message?: string }).message || '')
      : ''
    if (status === 404) {
      return '预览接口 404：当前 project-service 尚未包含 /image-generation-preview 路由。请重启 project 服务（goreman restart project 或重新运行 scripts/dev.sh）后刷新。'
    }
    if (status === 500 && message) return message
  }
  return err instanceof Error ? err.message : '加载传参预览失败'
}

function buildFallbackImageParams(
  sb: Storyboard,
  modelName: string,
): StoryboardImageGenerationParams {
  return {
    prompt: sb.prompt_used || sb.scene_description || '',
    prompt_display_zh: buildStoryboardPromptSummaryZh(sb, []),
    negative_prompt: '（预览接口不可用，负面词未加载）',
    model_name: modelName || 'dalle',
    style_preset: '（未知）',
    aspect_ratio: sb.aspect_ratio || '16:9',
    width: 0,
    height: 0,
    reference_image_urls: [],
    is_character_sheet: false,
    raw_prompt_mode: !!sb.prompt_locked,
    task_type: 'storyboard',
  }
}

function unwrapApiData<T>(res: unknown): T | null {
  if (!res || typeof res !== 'object') return null
  const root = res as Record<string, unknown>
  if (root.data && typeof root.data === 'object') return root.data as T
  return res as T
}

export function StoryboardDetailPanel({
  project,
  selectedSb,
  onClose,
  storyboardItemLabel,
  storyboardImageLabel,
  storyboardGenerateLabel,
  selectedStoryboardVersion,
  selectedStoryboardPreviewUrl,
  versionIdx,
  onVersionIdxChange,
  sbDescLang: _sbDescLang,
  onSbDescLangChange: _onSbDescLangChange,
  storyboardAssets,
  onSavePrompt,
  onEditingPromptChange,
  modelOptions,
  imageModelAvailability,
  defaultImageModelLabel,
  onGenerate,
}: {
  project: Project
  selectedSb: Storyboard
  onClose: () => void
  storyboardItemLabel: string
  storyboardImageLabel: string
  storyboardGenerateLabel: string
  selectedStoryboardVersion: StoryboardVersion | undefined
  selectedStoryboardPreviewUrl: string
  versionIdx: number
  onVersionIdxChange: (idx: number) => void
  sbDescLang: 'zh' | 'en'
  onSbDescLangChange: (lang: 'zh' | 'en') => void
  storyboardAssets: Asset[]
  onSavePrompt: (updates: Partial<Pick<Storyboard, 'scene_description' | 'prompt_used' | 'prompt_locked' | 'location_zone' | 'spatial_anchor' | 'subject_positions' | 'transition_note'>>, options?: { silent?: boolean }) => Promise<void>
  onEditingPromptChange?: (editing: boolean) => void
  modelOptions: ImageModelOption[]
  imageModelAvailability: Record<string, boolean>
  defaultImageModelLabel: string
  onGenerate: (sb: Storyboard, modelKeys: string[]) => void | Promise<void>
}) {
  const [promptMode, setPromptMode] = useState<'auto' | 'manual'>('auto')
  const [promptDraft, setPromptDraft] = useState('')
  const [finalPromptDraft, setFinalPromptDraft] = useState('')
  const [selectedModelKeys, setSelectedModelKeys] = useState<string[]>([])
  const [generating, setGenerating] = useState(false)
  const [imageParams, setImageParams] = useState<StoryboardImageGenerationParams | null>(null)
  const [imageParamsLoading, setImageParamsLoading] = useState(false)
  const [imageParamsError, setImageParamsError] = useState('')
  const [previewRetryToken, setPreviewRetryToken] = useState(0)
  const [showEnglishPrompt, setShowEnglishPrompt] = useState(false)

  const linkedAssets = useMemo(
    () => storyboardAssets.filter((a) => selectedSb.asset_ids?.includes(a.id)),
    [storyboardAssets, selectedSb.asset_ids],
  )
  const previewModelName = selectedModelKeys[0] || modelOptions[0]?.key || ''
  const promptDisplayZh = useMemo(
    () => (imageParams?.prompt_display_zh?.trim() || buildStoryboardPromptSummaryZh(selectedSb, linkedAssets)),
    [imageParams?.prompt_display_zh, selectedSb, linkedAssets],
  )
  const promptAutoSupplementsZh = imageParams?.prompt_auto_supplements_zh?.trim() || ''
  const inferredLocationView = useMemo(
    () => inferLocationViewHint(`${selectedSb.scene_description || ''} ${selectedSb.location || ''}`),
    [selectedSb.scene_description, selectedSb.location],
  )
  const effectiveLocationView = selectedSb.location_zone?.trim() || inferredLocationView

  const handleLocationZoneChange = async (nextZone: string) => {
    const normalized = nextZone.trim()
    if (normalized === (selectedSb.location_zone || '').trim()) return
    await onSavePrompt({ location_zone: normalized }, { silent: true })
    setPreviewRetryToken((v) => v + 1)
  }

  useEffect(() => {
    setPromptMode(selectedSb.prompt_locked ? 'manual' : 'auto')
    setPromptDraft(selectedSb.scene_description || '')
    setFinalPromptDraft(pickEditableChinesePrompt(selectedSb.scene_description, selectedSb.prompt_used))
    setSelectedModelKeys([])
    setImageParams(null)
    setImageParamsLoading(false)
    setImageParamsError('')
    setShowEnglishPrompt(false)
    onEditingPromptChange?.(false)
    return () => onEditingPromptChange?.(false)
  }, [selectedSb.id, onEditingPromptChange])

  useEffect(() => {
    let cancelled = false
    setImageParamsLoading(true)
    setImageParamsError('')
    setShowEnglishPrompt(false)
    void storyboardAPI.getImageGenerationPreview(project.id, selectedSb.id, previewModelName || undefined)
      .then((res) => {
        if (!cancelled) {
          setImageParams(unwrapApiData<StoryboardImageGenerationParams>(res))
          setImageParamsLoading(false)
        }
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setImageParams(buildFallbackImageParams(selectedSb, previewModelName))
          setImageParamsError(formatPreviewError(err))
          setImageParamsLoading(false)
        }
      })
    return () => { cancelled = true }
  }, [
    project.id,
    selectedSb,
    previewModelName,
    previewRetryToken,
  ])

  const markPromptEditing = () => onEditingPromptChange?.(true)

  const toggleModel = (key: string) => {
    setSelectedModelKeys((prev) =>
      prev.includes(key) ? prev.filter((k) => k !== key) : [...prev, key],
    )
  }

  const handleConfirmGenerate = async () => {
    if (promptMode === 'manual' && !finalPromptDraft.trim()) return
    setGenerating(true)
    try {
      if (promptMode === 'manual') {
        await onSavePrompt({
          prompt_used: finalPromptDraft.trim(),
          prompt_locked: true,
        }, { silent: true })
      } else {
        const newDesc = promptDraft.trim()
        const descChanged = newDesc !== (selectedSb.scene_description ?? '').trim()
        const wasLocked = !!selectedSb.prompt_locked
        if (descChanged || wasLocked) {
          await onSavePrompt({
            ...(descChanged ? { scene_description: newDesc } : {}),
            prompt_locked: false,
            ...(wasLocked ? { prompt_used: '' } : {}),
          }, { silent: true })
        }
      }
      await onGenerate({
        ...selectedSb,
        scene_description: promptMode === 'auto' ? (promptDraft.trim() || selectedSb.scene_description) : selectedSb.scene_description,
        prompt_used: promptMode === 'manual' ? finalPromptDraft.trim() : selectedSb.prompt_used,
        prompt_locked: promptMode === 'manual',
      }, selectedModelKeys)
    } finally {
      setGenerating(false)
    }
  }

  const modelSections = [
    { label: '🌐 多模态推荐', filter: (m: ImageModelOption) => m.tags.includes('多模态') },
    { label: '🎨 高质量文生图', filter: (m: ImageModelOption) => m.tags.includes('高质量') && !m.tags.includes('多模态') },
    { label: '⚡ 高速 / 低成本', filter: (m: ImageModelOption) => !m.tags.includes('多模态') && !m.tags.includes('高质量') && !m.tags.includes('本地') },
    { label: '🖥️ 本地部署', filter: (m: ImageModelOption) => m.tags.includes('本地') },
  ]

  const canGenerate = canTriggerStoryboardImage(selectedSb)

  return (
    <div className="fixed inset-0 z-40 flex justify-end">
      <div className="absolute inset-0 bg-black/30" onClick={onClose} />
      <div className="relative z-50 flex h-full w-full max-w-6xl flex-col bg-white shadow-xl xl:max-w-7xl">
        <div className="flex items-center justify-between border-b bg-white/95 px-5 py-4 backdrop-blur-sm">
          <div className="min-w-0">
            <h3 className="truncate text-lg font-semibold">{`${storyboardItemLabel} #${selectedSb.sequence_number}`}</h3>
            <p className="mt-1 text-xs text-surface-400">分镜详情与图片生成</p>
          </div>
          <Button size="sm" variant="ghost" onClick={onClose}>
            <X className="h-4 w-4" />
          </Button>
        </div>

        <div className="grid min-h-0 flex-1 lg:grid-cols-[minmax(0,1.05fr)_minmax(460px,0.95fr)]">
          {/* 左侧：预览与分镜信息 */}
          <div className="flex min-h-0 flex-col border-b border-surface-200 bg-surface-50/60 p-5 lg:border-b-0 lg:border-r">
            <div className="mb-3 flex items-center justify-between">
              <h4 className="text-sm font-semibold text-surface-700">{storyboardImageLabel}预览</h4>
              <StatusBadge status={selectedSb.status} />
            </div>

            <div className="flex-1 overflow-y-auto pr-1">
              {selectedStoryboardPreviewUrl ? (
                <div className="rounded-2xl border border-surface-200 bg-white p-3 shadow-sm">
                  <div className="relative aspect-video overflow-hidden rounded-xl border border-surface-100 bg-surface-100">
                    <ZoomableImage
                      src={selectedStoryboardPreviewUrl}
                      alt={selectedStoryboardVersion ? `V${selectedStoryboardVersion.version_number}` : `${storyboardItemLabel} #${selectedSb.sequence_number}`}
                      className="h-full w-full object-cover"
                    />
                    <div className="absolute right-2 top-2 z-10">
                      <ZoomBadge src={selectedStoryboardPreviewUrl} alt={selectedStoryboardVersion ? `V${selectedStoryboardVersion.version_number}` : `${storyboardItemLabel} #${selectedSb.sequence_number}`} />
                    </div>
                    {selectedSb.status === 'generating' && (
                      <div className="absolute inset-x-3 bottom-3 rounded-xl bg-black/65 px-3 py-2 text-xs text-white shadow-lg backdrop-blur-sm">
                        <div className="flex items-center gap-2">
                          <Loader2 className="h-3.5 w-3.5 animate-spin" />
                          <span>新版本生成中，完成后会自动刷新</span>
                        </div>
                      </div>
                    )}
                  </div>

                  <div className="mt-3 flex flex-wrap items-center gap-2">
                    {selectedStoryboardVersion ? (
                      <Badge variant="outline">版本 V{selectedStoryboardVersion.version_number}</Badge>
                    ) : null}
                    {selectedSb.prompt_locked ? (
                      <Badge variant="outline" className="text-amber-700">提示词已锁定</Badge>
                    ) : null}
                  </div>
                </div>
              ) : selectedSb.status === 'generating' ? (
                <div className="flex min-h-[360px] flex-col items-center justify-center rounded-2xl border border-dashed border-primary-200 bg-white/90 p-6 text-center">
                  <Loader2 className="h-10 w-10 animate-spin text-primary-500" />
                  <p className="mt-4 text-sm font-medium text-surface-800">{storyboardImageLabel}生成中</p>
                  <p className="mt-2 max-w-sm text-xs leading-6 text-surface-500">
                    生成完成后会在左侧自动展示图片，右侧可继续编辑提示词并重新生成。
                  </p>
                </div>
              ) : (
                <div className="flex min-h-[360px] flex-col items-center justify-center rounded-2xl border border-dashed border-surface-200 bg-white/80 p-6 text-center">
                  <LayoutGrid className="h-10 w-10 text-surface-300" />
                  <p className="mt-4 text-sm font-medium text-surface-700">暂无{storyboardImageLabel}</p>
                  <p className="mt-2 max-w-sm text-xs leading-6 text-surface-400">
                    你可以在右侧编辑提示词并选择模型生成；生成结果会显示在这里。
                  </p>
                </div>
              )}

              <div className="mt-4 space-y-3 pb-3">
                {selectedSb.versions && selectedSb.versions.length > 0 && (
                  <div className="rounded-xl border border-surface-200 bg-white p-4 shadow-sm">
                    <div className="flex items-center justify-between gap-3">
                      <div>
                        <p className="text-xs font-medium text-surface-500">候选版本</p>
                      </div>
                      <Badge variant="outline">{selectedSb.versions.length} 版</Badge>
                    </div>
                    <div className="mt-3 grid grid-cols-3 gap-2">
                      {selectedSb.versions.map((version, idx) => {
                        const isSelected = idx === versionIdx
                        return (
                          <button
                            key={version.id}
                            type="button"
                            className={`relative overflow-hidden rounded-xl border bg-surface-50 transition w-full ${
                              isSelected ? 'border-primary-500 ring-2 ring-primary-200' : 'border-surface-200 hover:border-primary-300'
                            }`}
                            onClick={() => onVersionIdxChange(idx)}
                            title={version.prompt_used ? `V${version.version_number} · 点击查看` : `版本 V${version.version_number}`}
                          >
                            {version.image_url ? (
                              <img src={version.image_url} alt="" className="aspect-square w-full object-cover" />
                            ) : (
                              <div className="flex aspect-square w-full items-center justify-center text-surface-300">
                                <LayoutGrid className="h-6 w-6" />
                              </div>
                            )}
                            <div className="absolute inset-x-0 bottom-0 bg-gradient-to-t from-black/70 via-black/25 to-transparent px-2 py-1.5 text-left text-[10px] text-white">
                              {isSelected ? '当前展示' : `V${version.version_number}`}
                            </div>
                          </button>
                        )
                      })}
                    </div>
                  </div>
                )}

                <div className="rounded-xl border border-surface-200 bg-white p-4 shadow-sm">
                  <p className="text-xs font-medium text-surface-500">当前描述</p>
                  <p className="mt-1 text-sm leading-6 text-surface-700">{selectedSb.scene_description || '暂无描述'}</p>

                  {(selectedSb.characters?.length > 0 || selectedSb.location || linkedAssets.length > 0) && (
                    <div className="mt-3 space-y-2 border-t border-surface-100 pt-3">
                      {selectedSb.characters?.length > 0 && (
                        <div className="flex items-start gap-2">
                          <span className="shrink-0 text-[10px] font-medium text-surface-400 w-10 mt-0.5">角色</span>
                          <div className="flex flex-wrap gap-1">
                            {selectedSb.characters.map((c) => (
                              <span key={c} className="rounded-full bg-primary-50 border border-primary-100 px-2 py-0.5 text-[11px] text-primary-700">👤 {c}</span>
                            ))}
                          </div>
                        </div>
                      )}
                      {selectedSb.location && (
                        <div className="flex items-center gap-2">
                          <span className="shrink-0 text-[10px] font-medium text-surface-400 w-10">场景</span>
                          <span className="rounded-full bg-surface-100 px-2 py-0.5 text-[11px] text-surface-700">📍 {selectedSb.location}</span>
                        </div>
                      )}
                      <div className="flex items-center gap-2">
                        <span className="shrink-0 text-[10px] font-medium text-surface-400 w-10">视角</span>
                        <select
                          className="h-7 max-w-[140px] rounded-md border border-surface-200 bg-white px-2 text-[11px] text-surface-700"
                          value={selectedSb.location_zone || ''}
                          onChange={(e) => { void handleLocationZoneChange(e.target.value) }}
                        >
                          {LOCATION_VIEW_OPTIONS.map((opt) => (
                            <option key={opt.value || 'auto'} value={opt.value}>{opt.label}</option>
                          ))}
                        </select>
                        {!selectedSb.location_zone?.trim() && effectiveLocationView ? (
                          <span className="text-[10px] text-surface-400">推断：{locationViewLabel(effectiveLocationView)}</span>
                        ) : null}
                      </div>
                      {(selectedSb.spatial_anchor || selectedSb.subject_positions || selectedSb.transition_note) ? (
                        <div className="space-y-1.5 rounded-lg border border-indigo-100 bg-indigo-50/50 px-2.5 py-2">
                          {selectedSb.spatial_anchor ? (
                            <p className="text-[10px] leading-5 text-indigo-900"><span className="font-medium">空间锚点：</span>{selectedSb.spatial_anchor}</p>
                          ) : null}
                          {selectedSb.subject_positions ? (
                            <p className="text-[10px] leading-5 text-indigo-900"><span className="font-medium">主体站位：</span>{selectedSb.subject_positions}</p>
                          ) : null}
                          {selectedSb.transition_note ? (
                            <p className="text-[10px] leading-5 text-indigo-900"><span className="font-medium">转场：</span>{selectedSb.transition_note}</p>
                          ) : null}
                        </div>
                      ) : null}
                      {linkedAssets.length > 0 && (
                        <div className="flex items-start gap-2">
                          <span className="shrink-0 text-[10px] font-medium text-surface-400 w-10 mt-1">资源</span>
                          <div className="flex flex-wrap gap-1.5">
                            {linkedAssets.map((a) => (
                              <div key={a.id} className="flex items-center gap-1 rounded-lg border border-surface-200 bg-surface-50 px-1.5 py-1" title={a.name}>
                                {a.image_url ? (
                                  <img src={a.image_url} alt={a.name} className="h-7 w-7 rounded object-cover flex-shrink-0" />
                                ) : (
                                  <div className="h-7 w-7 rounded bg-surface-200 flex items-center justify-center text-[10px] text-surface-400">?</div>
                                )}
                                <div className="max-w-[80px]">
                                  <p className="truncate text-[10px] font-medium text-surface-700">{a.name}</p>
                                  <p className="text-[9px] text-surface-400">{a.type === 'character' ? '人物' : a.type === 'scene' ? '场景' : '物品'}</p>
                                </div>
                              </div>
                            ))}
                          </div>
                        </div>
                      )}
                    </div>
                  )}

                  {selectedSb.dialogue && (
                    <>
                      <p className="mt-3 text-xs font-medium text-surface-500">台词</p>
                      <p className="mt-1 whitespace-pre-wrap text-sm leading-6 text-surface-700">{selectedSb.dialogue}</p>
                    </>
                  )}
                </div>
              </div>
            </div>
          </div>

          {/* 右侧：提示词编辑与模型选择（与资源弹窗对齐：单区域滚动 + 底部固定生成） */}
          <div className="flex min-h-0 flex-col overflow-hidden bg-white">
            <div className="min-h-0 flex-1 overflow-y-auto p-5">
              <div className="space-y-4">
                {selectedSb.status === 'failed' && selectedSb.error_msg && (
                  <div className="rounded-md border border-red-200 bg-red-50 p-3">
                    <div className="mb-1 flex items-center gap-1.5">
                      <AlertCircle className="h-3.5 w-3.5 text-red-500" />
                      <span className="text-xs font-medium text-red-700">生成失败</span>
                    </div>
                    <p className="text-xs text-red-600">{formatStoryboardErrorMessage(selectedSb.error_msg)}</p>
                  </div>
                )}

                <div className="rounded-xl border border-surface-200 bg-surface-50/80 p-3">
                  <div className="mb-2 flex items-center justify-between gap-2">
                    <p className="text-xs font-medium text-surface-700">图片服务实际传参</p>
                    <div className="flex items-center gap-2">
                      {imageParamsLoading && (
                        <span className="flex items-center gap-1 text-[10px] text-surface-500">
                          <Loader2 className="h-3 w-3 animate-spin" />
                          计算中…
                        </span>
                      )}
                      <Button
                        type="button"
                        size="sm"
                        variant="ghost"
                        className="h-7 px-2 text-[10px]"
                        disabled={imageParamsLoading}
                        onClick={() => setPreviewRetryToken((v) => v + 1)}
                      >
                        重新加载
                      </Button>
                    </div>
                  </div>
                  {imageParamsError ? (
                    <p className="mb-3 rounded-lg border border-amber-200 bg-amber-50 px-2 py-2 text-xs text-amber-800">{imageParamsError}</p>
                  ) : null}
                  {imageParams ? (
                    <div className="space-y-3 text-xs">
                      <ParamRow label="模型" value={imageParams.model_name} />
                      <ParamRow label="风格 preset" value={imageParams.style_preset} />
                      <ParamRow label="画幅" value={imageParams.width > 0 ? `${imageParams.aspect_ratio} (${imageParams.width}×${imageParams.height})` : imageParams.aspect_ratio} />
                      <ParamRow label="任务类型" value={imageParams.task_type} />
                      <ParamRow label="高级锁定" value={imageParams.raw_prompt_mode ? '是（原样使用 prompt）' : '否（自动组装）'} />
                      <div>
                        <div className="mb-1 flex items-center justify-between gap-2">
                          <span className="font-medium text-surface-600">prompt</span>
                          <CopyButton text={promptDisplayZh} />
                        </div>
                        <pre className="max-h-56 overflow-auto whitespace-pre-wrap break-words rounded-lg border border-surface-200 bg-white p-2 text-[11px] leading-5 text-surface-800">{promptDisplayZh}</pre>
                        {promptAutoSupplementsZh ? (
                          <div className="mt-2 rounded-lg border border-amber-200 bg-amber-50/70 p-2">
                            <div className="mb-1 flex items-center justify-between gap-2">
                              <span className="text-[10px] font-medium text-amber-800">系统自动补充</span>
                              <CopyButton text={promptAutoSupplementsZh} />
                            </div>
                            <pre className="max-h-48 overflow-auto whitespace-pre-wrap break-words text-[11px] leading-5 text-amber-950">{promptAutoSupplementsZh}</pre>
                          </div>
                        ) : null}
                        {imageParams.prompt ? (
                          <div className="mt-2">
                            <button
                              type="button"
                              className="flex items-center gap-1 text-[10px] font-medium text-surface-500 hover:text-surface-700"
                              onClick={() => setShowEnglishPrompt((v) => !v)}
                            >
                              {showEnglishPrompt ? (
                                <ChevronUp className="h-3 w-3" />
                              ) : (
                                <ChevronDown className="h-3 w-3" />
                              )}
                              {showEnglishPrompt ? '收起英文' : '查看英文传参'}
                            </button>
                            {showEnglishPrompt ? (
                              <div className="mt-2 rounded-lg border border-surface-200 bg-surface-50 p-2">
                                <div className="mb-1 flex items-center justify-between gap-2">
                                  <span className="text-[10px] font-medium text-surface-500">prompt（英文）</span>
                                  <CopyButton text={imageParams.prompt} />
                                </div>
                                <pre className="max-h-48 overflow-auto whitespace-pre-wrap break-words text-[11px] leading-5 text-surface-700">{imageParams.prompt}</pre>
                              </div>
                            ) : null}
                          </div>
                        ) : null}
                      </div>
                      <div>
                        <div className="mb-1 flex items-center justify-between gap-2">
                          <span className="font-medium text-surface-600">negative_prompt</span>
                          <CopyButton text={imageParams.negative_prompt} />
                        </div>
                        <pre className="max-h-32 overflow-auto whitespace-pre-wrap break-words rounded-lg border border-surface-200 bg-white p-2 text-[11px] leading-5 text-surface-700">{imageParams.negative_prompt || '（空）'}</pre>
                      </div>
                      {imageParams.style_reference_url ? (
                        <div>
                          <div className="mb-1 flex items-center justify-between gap-2">
                            <span className="font-medium text-surface-600">style_reference_url</span>
                            <CopyButton text={imageParams.style_reference_url} />
                          </div>
                          <a href={imageParams.style_reference_url} target="_blank" rel="noreferrer" className="break-all text-[11px] text-primary-600 hover:underline">{imageParams.style_reference_url}</a>
                        </div>
                      ) : null}
                      {imageParams.reference_image_urls?.length > 0 ? (
                        <div>
                          <p className="mb-1 font-medium text-surface-600">reference_image_urls ({imageParams.reference_image_urls.length})</p>
                          <ul className="space-y-1">
                            {imageParams.reference_image_urls.map((url) => (
                              <li key={url} className="flex items-start justify-between gap-2">
                                <a href={url} target="_blank" rel="noreferrer" className="min-w-0 flex-1 break-all text-[11px] text-primary-600 hover:underline">{url}</a>
                                <CopyButton text={url} />
                              </li>
                            ))}
                          </ul>
                        </div>
                      ) : (
                        <ParamRow label="reference_image_urls" value="（无）" />
                      )}
                      <ParamRow label="is_character_sheet" value={imageParams.is_character_sheet ? 'true' : 'false'} />
                    </div>
                  ) : !imageParamsLoading ? (
                    <p className="text-xs text-surface-500">暂无预览数据</p>
                  ) : null}
                </div>

                <div>
                  <h4 className="text-sm font-semibold text-surface-700">编辑提示词并生成</h4>
                </div>

                {selectedSb.status === 'generating' && (
                  <div className="rounded-2xl border border-primary-200 bg-primary-50/80 px-3 py-3 text-xs text-primary-900 shadow-sm">
                    <div className="flex items-center justify-between gap-3">
                      <div className="flex items-center gap-2 font-medium">
                        <Loader2 className="h-3.5 w-3.5 animate-spin" />
                        <span>正在生成新版本</span>
                      </div>
                    </div>
                    <p className="mt-2 text-[11px] text-primary-700/80">生成中…</p>
                  </div>
                )}

                <div className="rounded-xl border border-surface-200 bg-surface-50/70 p-3">
                  <div className="mb-2 flex items-center justify-between gap-2">
                    <Label className="text-xs font-medium">提示词</Label>
                    <div className="flex items-center rounded-md border border-surface-200 bg-white p-0.5 text-[10px] font-medium">
                        <button
                          type="button"
                          className={`rounded px-2 py-0.5 transition-colors ${promptMode === 'auto' ? 'bg-primary-500 text-white' : 'text-surface-500 hover:bg-surface-100'}`}
                          onClick={() => setPromptMode('auto')}
                        >自动（编辑描述）</button>
                        <button
                          type="button"
                          className={`rounded px-2 py-0.5 transition-colors ${promptMode === 'manual' ? 'bg-primary-500 text-white' : 'text-surface-500 hover:bg-surface-100'}`}
                          onClick={() => {
                            if (!finalPromptDraft.trim()) {
                              setFinalPromptDraft(pickEditableChinesePrompt(promptDraft, selectedSb.prompt_used))
                            }
                            setPromptMode('manual')
                          }}
                        >高级（最终提示词）</button>
                    </div>
                  </div>
                  {promptMode === 'auto' ? (
                    <>
                      <Textarea
                        value={promptDraft}
                        onChange={(e) => {
                          markPromptEditing()
                          setPromptDraft(e.target.value)
                        }}
                        placeholder="描述本镜画面内容、人物动作与环境氛围，可直接作为图像生成提示词。"
                        className="min-h-[140px] resize-y bg-white text-xs leading-5"
                      />
                    </>
                  ) : (
                    <>
                      <Textarea
                        value={finalPromptDraft}
                        onChange={(e) => {
                          markPromptEditing()
                          setFinalPromptDraft(e.target.value)
                        }}
                        placeholder="直接编写最终提示词，确认生成后将原样使用（跳过自动组装）。"
                        className="min-h-[180px] resize-y bg-white text-xs leading-5"
                      />
                    </>
                  )}
                </div>

                {canGenerate && (
                  <div className="space-y-3">
                    <Label className="text-xs font-medium">生成模型（可多选）</Label>
                    {modelSections.map(({ label: sectionLabel, filter }) => {
                      const sectionModels = modelOptions.filter(filter)
                      if (sectionModels.length === 0) return null
                      return (
                        <div key={sectionLabel}>
                          <p className="mb-1.5 text-[10px] font-semibold uppercase tracking-wide text-surface-400">{sectionLabel}</p>
                          <div className="grid gap-2 sm:grid-cols-2">
                            {sectionModels.map((m) => {
                              const avail = imageModelAvailability[m.key]
                              const broken = !!m.failureReason
                              const selected = selectedModelKeys.includes(m.key)
                              return (
                                <button
                                  key={m.key}
                                  type="button"
                                  title={broken ? `已停用：${m.failureReason}` : undefined}
                                  disabled={broken}
                                  onClick={() => {
                                    if (broken || avail === false) return
                                    toggleModel(m.key)
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
                                      <span className="text-xs font-semibold text-surface-800">{m.label}</span>
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
                                      <p className="mt-0.5 text-[9px] leading-snug text-surface-500">{m.desc}</p>
                                    )}
                                  </div>
                                </button>
                              )
                            })}
                          </div>
                        </div>
                      )
                    })}
                    {selectedModelKeys.length === 0 ? (
                      <p className="text-[11px] text-surface-400">未选择将使用项目默认图片模型：{defaultImageModelLabel}</p>
                    ) : (
                      <p className="text-[11px] text-surface-400">已选 {selectedModelKeys.length} 个模型{selectedModelKeys.length > 1 ? '（每个模型各生成一版候选图）' : ''}。</p>
                    )}
                  </div>
                )}
              </div>
            </div>

            {canGenerate && (
              <div className="shrink-0 border-t border-surface-200 bg-white/95 p-4 backdrop-blur-sm">
                <Button
                  className="w-full"
                  onClick={() => void handleConfirmGenerate()}
                  disabled={generating || selectedSb.status === 'generating'}
                >
                  {generating ? (
                    <Loader2 className="mr-1.5 h-4 w-4 animate-spin" />
                  ) : (
                    <Sparkles className="mr-1.5 h-4 w-4" />
                  )}
                  {selectedSb.status === 'failed'
                    ? '重新生成'
                    : `${storyboardGenerateLabel}${selectedModelKeys.length > 1 ? `（${selectedModelKeys.length}）` : ''}`}
                </Button>
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}

function ParamRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex flex-wrap gap-x-2 gap-y-0.5">
      <span className="font-medium text-surface-600">{label}:</span>
      <span className="break-all text-surface-800">{value}</span>
    </div>
  )
}

function CopyButton({ text }: { text: string }) {
  return (
    <Button
      type="button"
      size="sm"
      variant="ghost"
      className="h-6 px-2 text-[10px]"
      onClick={() => { void navigator.clipboard.writeText(text) }}
    >
      复制
    </Button>
  )
}
