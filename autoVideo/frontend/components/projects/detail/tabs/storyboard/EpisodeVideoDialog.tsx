'use client'

import { Loader2, Video } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Label } from '@/components/ui/label'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { VIDEO_STYLE_COMPACT_OPTIONS, VIDEO_GENERATION_PRESETS, VIDEO_MODEL_SELECTION_HINTS } from '@/lib/video-style-config'
import type { VideoModelCapability } from '@/lib/video-style-config'
import type { Episode } from '@/types'
import {
  VIDEO_STYLE_LABELS,
  VIDEO_MOTION_LABELS,
  VIDEO_FRAME_SIZE_OPTIONS,
  VIDEO_SUBJECT_SIZE_OPTIONS,
  VIDEO_CLARITY_OPTIONS,
  type VideoFrameSizeKey,
  type VideoSubjectSizeKey,
  type VideoClarityKey,
  type VideoMotionKey,
} from './episode-video-constants'

type VideoModelParam = {
  key: string
  label: string
  default: string
  values: { value: string; label: string }[]
}

export function EpisodeVideoDialog({
  open,
  episode,
  generating,
  videoModelOptions,
  videoModelAvailability,
  selectedModel,
  onSelectedModelChange,
  selectedStyle,
  onSelectedStyleChange,
  selectedMotionMode,
  onSelectedMotionModeChange,
  selectedFrameSize,
  onSelectedFrameSizeChange,
  selectedSubjectSize,
  onSelectedSubjectSizeChange,
  selectedClarity,
  onSelectedClarityChange,
  videoModeLabel,
  videoModelParams,
  getModelParam,
  setModelParam,
  selectedTransition,
  onSelectedTransitionChange,
  selectedTransitionDuration,
  onSelectedTransitionDurationChange,
  onApplyPreset,
  onConfirm,
  onClose,
}: {
  open: boolean
  episode: Episode | null
  generating: boolean
  videoModelOptions: VideoModelCapability[]
  videoModelAvailability: Record<string, boolean>
  selectedModel: string
  onSelectedModelChange: (value: string) => void
  selectedStyle: string
  onSelectedStyleChange: (value: string) => void
  selectedMotionMode: VideoMotionKey
  onSelectedMotionModeChange: (value: VideoMotionKey) => void
  selectedFrameSize: VideoFrameSizeKey
  onSelectedFrameSizeChange: (value: VideoFrameSizeKey) => void
  selectedSubjectSize: VideoSubjectSizeKey
  onSelectedSubjectSizeChange: (value: VideoSubjectSizeKey) => void
  selectedClarity: VideoClarityKey
  onSelectedClarityChange: (value: VideoClarityKey) => void
  videoModeLabel: string
  videoModelParams: Record<string, VideoModelParam[]>
  getModelParam: (modelKey: string, paramKey: string) => string
  setModelParam: (modelKey: string, paramKey: string, value: string) => void
  selectedTransition: string
  onSelectedTransitionChange: (value: string) => void
  selectedTransitionDuration: string
  onSelectedTransitionDurationChange: (value: string) => void
  onApplyPreset: (presetKey: string) => void
  onConfirm: () => void
  onClose: () => void
}) {
  const selectedModelMeta = videoModelOptions.find((item) => item.key === selectedModel) ?? videoModelOptions[0] ?? {
    key: '', label: '', desc: '', icon: '', provider: '', audioSupport: 'none' as const,
    aspectRatio: 'unsupported' as const, resolution: 'unsupported' as const, multiVariant: 'unsupported' as const,
    clipDuration: '', note: '', tags: [], speed: 'medium' as const, quality: 'standard' as const, bestFor: [],
  }
  const selectedStyleMeta = VIDEO_STYLE_COMPACT_OPTIONS.find((item) => item.key === selectedStyle) ?? VIDEO_STYLE_COMPACT_OPTIONS[0]
  const selectedStyleLabel = VIDEO_STYLE_LABELS[selectedStyle] ?? selectedStyle
  const selectedMotionLabel = VIDEO_MOTION_LABELS[selectedMotionMode] ?? selectedMotionMode
  const selectedFrameSizeMeta = VIDEO_FRAME_SIZE_OPTIONS.find((item) => item.key === selectedFrameSize) ?? VIDEO_FRAME_SIZE_OPTIONS[0]
  const selectedSubjectSizeMeta = VIDEO_SUBJECT_SIZE_OPTIONS.find((item) => item.key === selectedSubjectSize) ?? VIDEO_SUBJECT_SIZE_OPTIONS[0]
  const selectedClarityMeta = VIDEO_CLARITY_OPTIONS.find((item) => item.key === selectedClarity) ?? VIDEO_CLARITY_OPTIONS[0]

  return (
    <Dialog open={open} onOpenChange={(next) => { if (!next) onClose() }}>
      <DialogContent className="flex flex-col gap-0 overflow-hidden p-0 sm:max-w-2xl">
        <DialogHeader className="shrink-0 border-b border-surface-100 px-6 py-4">
          <DialogTitle>
            {episode
              ? `第 ${episode.episode_number} 集 · 当前集生成视频`
              : '当前集生成视频'}
          </DialogTitle>
        </DialogHeader>
        <div className="min-h-0 flex-1 space-y-4 overflow-y-auto px-6 py-4">
          <div className="rounded-lg border border-violet-200 bg-violet-50 px-3 py-3">
            <div className="flex items-center justify-between gap-2">
              <div>
                <p className="text-xs font-semibold text-violet-800">快速质感预设</p>
                <p className="text-[11px] text-violet-600">一键切换模型 + 风格 + 运动模式。写实大片优先“真人电影”，对白戏和人物关系优先“真人短剧”。</p>
              </div>
              <Badge variant="outline" className="border-violet-200 bg-white text-[10px] text-violet-700">
                当前：{selectedModelMeta.label} / {selectedStyleMeta.label}
              </Badge>
            </div>
            <div className="mt-3 flex flex-wrap gap-2">
              {VIDEO_GENERATION_PRESETS.map((preset) => {
                const active =
                  selectedModel === preset.model &&
                  selectedStyle === preset.style &&
                  selectedMotionMode === preset.motion
                return (
                  <button
                    key={preset.key}
                    type="button"
                    onClick={() => onApplyPreset(preset.key)}
                    className={`rounded-lg border px-3 py-2 text-left transition-colors ${
                      active
                        ? 'border-violet-300 bg-white text-violet-700 shadow-sm'
                        : 'border-violet-200/70 bg-white/70 text-surface-600 hover:border-violet-300 hover:bg-white'
                    }`}
                  >
                    <div className="flex items-center gap-2">
                      <span className="text-xs font-semibold">{preset.label}</span>
                      <span className="rounded-full bg-violet-100 px-2 py-0.5 text-[10px] text-violet-700">{preset.tone}</span>
                    </div>
                    <p className="mt-1 text-[11px] leading-5">{preset.hint}</p>
                  </button>
                )
              })}
            </div>
          </div>
          <div className="rounded-lg border border-surface-200 bg-surface-50 px-3 py-3">
            <p className="text-xs font-semibold text-surface-700">当前已选视频配置</p>
            <div className="mt-2 flex flex-wrap gap-2 text-[11px]">
              <span className="rounded-full border border-surface-200 bg-white px-2.5 py-1 text-surface-600">
                风格：{selectedStyleLabel}
              </span>
              <span className="rounded-full border border-surface-200 bg-white px-2.5 py-1 text-surface-600">
                运动：{selectedMotionLabel}
              </span>
              <span className="rounded-full border border-surface-200 bg-white px-2.5 py-1 text-surface-600">
                模式：{videoModeLabel}
              </span>
            </div>
            <p className="mt-2 text-[11px] text-surface-500">
              风格和运动模式会与本次选择的模型、尺寸、大小、清晰度一起写入生成提示词。
            </p>
          </div>
          <div className="grid gap-4 md:grid-cols-2">
            <div className="space-y-2">
              <Label>生成模型</Label>
              <Select value={selectedModel} onValueChange={onSelectedModelChange}>
                <SelectTrigger>
                  <SelectValue placeholder="选择视频模型" />
                </SelectTrigger>
                <SelectContent>
                  {videoModelOptions.map((item) => (
                    <SelectItem key={item.key} value={item.key}>
                      {item.icon} {item.label}{videoModelAvailability[item.key] === true ? ' ●' : videoModelAvailability[item.key] === false ? ' ○' : ''}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <p className="text-[11px] text-surface-500">{selectedModelMeta.desc}</p>
              <p className="text-[11px] text-violet-600">{VIDEO_MODEL_SELECTION_HINTS[selectedModel] ?? ''}</p>
            </div>
            <div className="space-y-2">
              <Label>画面风格</Label>
              <Select value={selectedStyle} onValueChange={onSelectedStyleChange}>
                <SelectTrigger>
                  <SelectValue placeholder="选择画面风格" />
                </SelectTrigger>
                <SelectContent>
                  {VIDEO_STYLE_COMPACT_OPTIONS.map((item) => (
                    <SelectItem key={item.key} value={item.key}>
                      {item.label} · {item.tone}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <p className="text-[11px] text-surface-500">{selectedStyleMeta.hint}</p>
            </div>
            <div className="space-y-2">
              <Label>尺寸</Label>
              <Select value={selectedFrameSize} onValueChange={(v) => onSelectedFrameSizeChange(v as VideoFrameSizeKey)}>
                <SelectTrigger>
                  <SelectValue placeholder="选择尺寸" />
                </SelectTrigger>
                <SelectContent>
                  {VIDEO_FRAME_SIZE_OPTIONS.map((item) => (
                    <SelectItem key={item.key} value={item.key}>{item.label}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <p className="text-[11px] text-surface-500">{selectedFrameSizeMeta.desc}</p>
            </div>
            <div className="space-y-2">
              <Label>大小</Label>
              <Select value={selectedSubjectSize} onValueChange={(v) => onSelectedSubjectSizeChange(v as VideoSubjectSizeKey)}>
                <SelectTrigger>
                  <SelectValue placeholder="选择主体大小" />
                </SelectTrigger>
                <SelectContent>
                  {VIDEO_SUBJECT_SIZE_OPTIONS.map((item) => (
                    <SelectItem key={item.key} value={item.key}>{item.label}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <p className="text-[11px] text-surface-500">{selectedSubjectSizeMeta.desc}</p>
            </div>
            <div className="space-y-2">
              <Label>清晰度</Label>
              <Select value={selectedClarity} onValueChange={(v) => onSelectedClarityChange(v as VideoClarityKey)}>
                <SelectTrigger>
                  <SelectValue placeholder="选择清晰度" />
                </SelectTrigger>
                <SelectContent>
                  {VIDEO_CLARITY_OPTIONS.map((item) => (
                    <SelectItem key={item.key} value={item.key}>{item.label}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <p className="text-[11px] text-surface-500">{selectedClarityMeta.desc}</p>
            </div>
          </div>
          {(videoModelParams[selectedModel] ?? []).length > 0 ? (
            <div className="rounded-lg border border-blue-100 bg-blue-50/60 px-3 py-3">
              <p className="text-xs font-semibold text-blue-800">📐 模型生成参数</p>
              <p className="mt-0.5 text-[11px] text-blue-600">以下参数直接传给视频模型，影响生成画幅与分辨率。</p>
              <div className="mt-2 flex flex-wrap gap-3">
                {(videoModelParams[selectedModel] ?? []).map((param) => (
                  <div key={param.key} className="flex flex-col gap-1">
                    <label className="text-[11px] font-medium text-blue-700">{param.label}</label>
                    <Select
                      value={getModelParam(selectedModel, param.key) || param.default}
                      onValueChange={(val) => setModelParam(selectedModel, param.key, val)}
                    >
                      <SelectTrigger className="h-8 w-36 border-blue-200 bg-white text-xs">
                        <SelectValue placeholder={`选择${param.label}`} />
                      </SelectTrigger>
                      <SelectContent>
                        {param.values.map((v) => (
                          <SelectItem key={v.value} value={v.value}>{v.label}</SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                ))}
              </div>
            </div>
          ) : null}
          <div className="rounded-lg border border-purple-100 bg-purple-50/60 px-3 py-3">
            <p className="text-xs font-semibold text-purple-800">🎬 转场效果</p>
            <p className="mt-0.5 text-[11px] text-purple-600">控制片段间的过渡动画，dissolve 叠化最为流畅自然。</p>
            <div className="mt-2 flex flex-wrap items-end gap-3">
              <div className="flex flex-col gap-1">
                <label className="text-[11px] font-medium text-purple-700">转场类型</label>
                <Select value={selectedTransition} onValueChange={onSelectedTransitionChange}>
                  <SelectTrigger className="h-8 w-36 border-purple-200 bg-white text-xs">
                    <SelectValue placeholder="选择转场" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="dissolve">叠化 (dissolve)</SelectItem>
                    <SelectItem value="fade">淡入淡出 (fade)</SelectItem>
                    <SelectItem value="wipeleft">向左划入 (wipeleft)</SelectItem>
                    <SelectItem value="wiperight">向右划入 (wiperight)</SelectItem>
                    <SelectItem value="circleclose">圆形收缩 (circleclose)</SelectItem>
                    <SelectItem value="none">无转场 (直切)</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              {selectedTransition !== 'none' && (
                <div className="flex flex-col gap-1">
                  <label className="text-[11px] font-medium text-purple-700">时长 (秒)</label>
                  <Select value={selectedTransitionDuration} onValueChange={onSelectedTransitionDurationChange}>
                    <SelectTrigger className="h-8 w-28 border-purple-200 bg-white text-xs">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="0.3">0.3s</SelectItem>
                      <SelectItem value="0.5">0.5s</SelectItem>
                      <SelectItem value="0.8">0.8s</SelectItem>
                      <SelectItem value="1.0">1.0s</SelectItem>
                    </SelectContent>
                  </Select>
                </div>
              )}
            </div>
          </div>
          <div className="rounded-lg border border-sky-200 bg-sky-50 px-3 py-3 text-xs leading-5 text-sky-800">
            当前选择会作为提示词补充给视频模型：
            <span className="ml-1 font-medium">
              {selectedFrameSizeMeta.label} / {selectedSubjectSizeMeta.label} / {selectedClarityMeta.label}
            </span>
          </div>
        </div>
        <div className="shrink-0 flex justify-end gap-2 border-t border-surface-100 px-6 py-4">
          <Button variant="outline" onClick={onClose} disabled={generating}>
            取消
          </Button>
          <Button onClick={onConfirm} disabled={generating}>
            {generating ? (
              <Loader2 className="mr-1.5 h-4 w-4 animate-spin" />
            ) : (
              <Video className="mr-1.5 h-4 w-4" />
            )}
            开始生成
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  )
}
