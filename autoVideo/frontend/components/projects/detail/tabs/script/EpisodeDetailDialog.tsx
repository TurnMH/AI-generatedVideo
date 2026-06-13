'use client'

import { Loader2, Pencil, Sparkles } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { EpisodeStoryboardList } from '@/components/projects/detail/EpisodeStoryboardList'
import type { Episode } from '@/types'
import { formatDuration } from '@/lib/projects/utils'

type EpisodeDetailDialogProps = {
  projectId: number
  selectedEpisode: Episode | null
  editingEpisode: boolean
  editEpisodeTitle: string
  onEditEpisodeTitleChange: (value: string) => void
  editEpisodeSummary: string
  onEditEpisodeSummaryChange: (value: string) => void
  editEpisodeContent: string
  onEditEpisodeContentChange: (value: string) => void
  polishingEpisode: boolean
  savingEpisodeEdit: boolean
  autoOptimizingEpisode: number | null
  optimizingEpisode: number | null
  reviewingEpisode: number | null
  applyingOptimized: number | null
  onOpenChange: (open: boolean) => void
  onPolishEpisode: () => void
  onOpenEditEpisode: (episode: Episode) => void
  onCancelEdit: () => void
  onSaveEpisodeEdit: () => void
  onAutoOptimizeReview: (episode: Episode) => void
  onOptimizeEpisode: (episode: Episode) => void
  onApplyOptimizedText: (episode: Episode) => void
  onReviewEpisode: (episode: Episode) => void
}

export function EpisodeDetailDialog({
  projectId,
  selectedEpisode,
  editingEpisode,
  editEpisodeTitle,
  onEditEpisodeTitleChange,
  editEpisodeSummary,
  onEditEpisodeSummaryChange,
  editEpisodeContent,
  onEditEpisodeContentChange,
  polishingEpisode,
  savingEpisodeEdit,
  autoOptimizingEpisode,
  optimizingEpisode,
  reviewingEpisode,
  applyingOptimized,
  onOpenChange,
  onPolishEpisode,
  onOpenEditEpisode,
  onCancelEdit,
  onSaveEpisodeEdit,
  onAutoOptimizeReview,
  onOptimizeEpisode,
  onApplyOptimizedText,
  onReviewEpisode,
}: EpisodeDetailDialogProps) {
  return (
    <Dialog open={!!selectedEpisode} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-4xl">
        <DialogHeader>
          <div className="flex items-center justify-between gap-3 pr-8">
            <DialogTitle>
              第 {selectedEpisode?.episode_number} 集 · {editingEpisode ? (editEpisodeTitle || selectedEpisode?.title) : selectedEpisode?.title}
            </DialogTitle>
            {!editingEpisode ? (
              <div className="flex gap-2 shrink-0">
                <Button size="sm" variant="outline" onClick={onPolishEpisode} disabled={polishingEpisode}>
                  {polishingEpisode ? <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" /> : <Sparkles className="mr-1.5 h-3.5 w-3.5" />}
                  AI 润色
                </Button>
                <Button size="sm" variant="outline" className="shrink-0" onClick={() => selectedEpisode && onOpenEditEpisode(selectedEpisode)}>
                  <Pencil className="mr-1.5 h-3.5 w-3.5" />
                  编辑
                </Button>
              </div>
            ) : (
              <div className="flex gap-2 shrink-0">
                <Button type="button" size="sm" variant="outline" onClick={onCancelEdit} disabled={savingEpisodeEdit}>取消</Button>
                <Button type="button" size="sm" onClick={onSaveEpisodeEdit} disabled={savingEpisodeEdit}>
                  {savingEpisodeEdit ? <Loader2 className="mr-1 h-3 w-3 animate-spin" /> : null}保存
                </Button>
              </div>
            )}
          </div>
        </DialogHeader>
        <div className="grid grid-cols-1 gap-6 md:grid-cols-2">
          <div className="space-y-3">
            {editingEpisode ? (
              <>
                <div className="space-y-1.5">
                  <Label className="text-xs">标题</Label>
                  <Input value={editEpisodeTitle} onChange={(e) => onEditEpisodeTitleChange(e.target.value)} placeholder="分集标题" />
                </div>
                <div className="space-y-1.5">
                  <Label className="text-xs">剧情摘要</Label>
                  <Textarea value={editEpisodeSummary} onChange={(e) => onEditEpisodeSummaryChange(e.target.value)} rows={3} placeholder="简要描述这一集的主要剧情" />
                </div>
                <div className="space-y-1.5">
                  <Label className="text-xs">
                    正文内容
                    <span className="ml-1 font-normal text-surface-400">（分镜生成时优先使用）</span>
                  </Label>
                  <Textarea value={editEpisodeContent} onChange={(e) => onEditEpisodeContentChange(e.target.value)} rows={8} placeholder="粘贴或输入本集的具体剧本/小说原文，AI 将基于此内容拆分分镜。不填则使用摘要。" className="font-mono text-xs" />
                </div>
              </>
            ) : (
              <>
                {selectedEpisode?.summary && (
                  <div className="rounded-md bg-surface-50 px-4 py-3">
                    <p className="mb-1 text-xs font-semibold uppercase tracking-wide text-surface-400">剧情摘要</p>
                    <p className="text-sm text-surface-700">{selectedEpisode.summary}</p>
                  </div>
                )}
                {selectedEpisode?.script_excerpt ? (
                  <div className="rounded-md bg-surface-50 px-4 py-3">
                    <p className="mb-1 text-xs font-semibold uppercase tracking-wide text-surface-400">本集原文内容</p>
                    <div className="max-h-96 overflow-y-auto">
                      <p className="whitespace-pre-wrap text-sm leading-relaxed text-surface-700">
                        {selectedEpisode.script_excerpt}
                      </p>
                    </div>
                  </div>
                ) : (
                  <div className="rounded-md border border-dashed border-surface-200 px-4 py-3 text-center">
                    <p className="text-xs text-surface-400">暂无正文内容，点击「编辑」可添加剧本原文，提升分镜生成质量。</p>
                  </div>
                )}
                <div className="flex gap-4 text-xs text-surface-400">
                  <span>{selectedEpisode?.word_count} 字</span>
                  <span>~{formatDuration(selectedEpisode?.estimated_duration ?? 0)}</span>
                </div>

                <div className="rounded-md border border-amber-200 bg-amber-50 px-3 py-3">
                  <div className="mb-2 flex items-center justify-between">
                    <p className="text-xs font-semibold text-amber-800">剧本格式优化</p>
                    <div className="flex items-center gap-2">
                      {selectedEpisode?.optimize_status === 'done' && (
                        <span className="rounded bg-amber-200 px-1.5 py-0.5 text-[10px] text-amber-800">已优化</span>
                      )}
                      {selectedEpisode?.optimize_status === 'optimizing' && (
                        <span className="rounded bg-amber-100 px-1.5 py-0.5 text-[10px] text-amber-600">优化中...</span>
                      )}
                      <Button
                        size="sm"
                        variant="outline"
                        className="h-6 border-purple-300 px-2 text-xs text-purple-700 hover:bg-purple-100"
                        onClick={() => selectedEpisode && onAutoOptimizeReview(selectedEpisode)}
                        disabled={autoOptimizingEpisode === selectedEpisode?.id || optimizingEpisode === selectedEpisode?.id}
                        title="AI 自动完成：转剧本格式 → 审查 → 弥补不足"
                      >
                        {autoOptimizingEpisode === selectedEpisode?.id ? <Loader2 className="mr-1 h-3 w-3 animate-spin" /> : <Sparkles className="mr-1 h-3 w-3" />}
                        AI 一键优化
                      </Button>
                      <Button
                        size="sm"
                        variant="outline"
                        className="h-6 border-amber-300 px-2 text-xs text-amber-700 hover:bg-amber-100"
                        onClick={() => selectedEpisode && onOptimizeEpisode(selectedEpisode)}
                        disabled={optimizingEpisode === selectedEpisode?.id || autoOptimizingEpisode === selectedEpisode?.id}
                      >
                        {optimizingEpisode === selectedEpisode?.id ? <Loader2 className="mr-1 h-3 w-3 animate-spin" /> : null}
                        转剧本格式
                      </Button>
                    </div>
                  </div>
                  {selectedEpisode?.optimize_status === 'done' && selectedEpisode.optimized_text ? (
                    <>
                      <div className="grid grid-cols-2 gap-2">
                        <div>
                          <p className="mb-1 text-[10px] font-medium text-surface-500">原文</p>
                          <div className="max-h-40 overflow-y-auto rounded bg-white px-2 py-2">
                            <p className="whitespace-pre-wrap font-mono text-[11px] leading-relaxed text-surface-600">
                              {selectedEpisode.original_excerpt || selectedEpisode.script_excerpt}
                            </p>
                          </div>
                        </div>
                        <div>
                          <p className="mb-1 text-[10px] font-medium text-amber-700">优化后</p>
                          <div className="max-h-40 overflow-y-auto rounded bg-amber-50/60 px-2 py-2 border border-amber-200">
                            <p className="whitespace-pre-wrap font-mono text-[11px] leading-relaxed text-amber-900">
                              {selectedEpisode.optimized_text}
                            </p>
                          </div>
                        </div>
                      </div>
                      <div className="mt-2 flex justify-end">
                        <Button
                          size="sm"
                          className="h-7 bg-amber-500 px-3 text-xs text-white hover:bg-amber-600"
                          onClick={() => selectedEpisode && onApplyOptimizedText(selectedEpisode)}
                          disabled={applyingOptimized === selectedEpisode?.id}
                        >
                          {applyingOptimized === selectedEpisode?.id ? <Loader2 className="mr-1 h-3 w-3 animate-spin" /> : null}
                          确认应用优化内容
                        </Button>
                      </div>
                    </>
                  ) : (
                    <p className="text-[11px] text-amber-600">点击&quot;转剧本格式&quot;将小说原文转化为标准剧本格式（场景标题、动作描述、台词等）。</p>
                  )}
                </div>

                <div className="rounded-md border border-green-200 bg-green-50 px-3 py-3">
                  <div className="mb-2 flex items-center justify-between">
                    <p className="text-xs font-semibold text-green-800">AI 质量审查</p>
                    <div className="flex items-center gap-2">
                      {selectedEpisode?.review_status === 'done' && (
                        <span className="rounded bg-green-200 px-1.5 py-0.5 text-[10px] text-green-800">已审查</span>
                      )}
                      {selectedEpisode?.review_status === 'reviewing' && (
                        <span className="rounded bg-green-100 px-1.5 py-0.5 text-[10px] text-green-600">审查中...</span>
                      )}
                      <Button
                        size="sm"
                        variant="outline"
                        className="h-6 border-green-300 px-2 text-xs text-green-700 hover:bg-green-100"
                        onClick={() => selectedEpisode && onReviewEpisode(selectedEpisode)}
                        disabled={reviewingEpisode === selectedEpisode?.id}
                      >
                        {reviewingEpisode === selectedEpisode?.id ? <Loader2 className="mr-1 h-3 w-3 animate-spin" /> : null}
                        AI 审查
                      </Button>
                    </div>
                  </div>
                  {selectedEpisode?.review_status === 'done' && selectedEpisode.review_result ? (
                    <div className="space-y-2">
                      {(() => {
                        const s = selectedEpisode.review_result.score
                        const dims = [
                          { label: '完整度', val: s.completeness },
                          { label: '连贯性', val: s.integrity },
                          { label: '一致性', val: s.consistency },
                          { label: '衔接性', val: s.transitions },
                          { label: '台词质量', val: s.dialog_quality },
                        ]
                        return (
                          <div className="space-y-1">
                            {dims.map((d) => (
                              <div key={d.label} className="flex items-center gap-2">
                                <span className="w-14 shrink-0 text-[10px] text-surface-600">{d.label}</span>
                                <div className="h-1.5 flex-1 rounded-full bg-surface-200">
                                  <div
                                    className={`h-1.5 rounded-full ${d.val >= 80 ? 'bg-green-400' : d.val >= 60 ? 'bg-yellow-400' : 'bg-red-400'}`}
                                    style={{ width: `${d.val}%` }}
                                  />
                                </div>
                                <span className="w-7 shrink-0 text-right text-[10px] font-medium text-surface-700">{d.val}</span>
                              </div>
                            ))}
                          </div>
                        )
                      })()}
                      {selectedEpisode.review_result.overall && (
                        <p className="text-[11px] text-surface-700">{selectedEpisode.review_result.overall}</p>
                      )}
                      {selectedEpisode.review_result.issues?.length > 0 && (
                        <div className="space-y-1">
                          {selectedEpisode.review_result.issues.map((issue, i) => (
                            <div key={i} className={`rounded px-2 py-1.5 text-[11px] ${issue.severity === 'critical' ? 'bg-red-50 text-red-800' : issue.severity === 'warning' ? 'bg-yellow-50 text-yellow-800' : 'bg-blue-50 text-blue-800'}`}>
                              <span className="font-medium">[{issue.type}]</span> {issue.description}
                              {issue.suggestion && <p className="mt-0.5 opacity-80">→ {issue.suggestion}</p>}
                            </div>
                          ))}
                        </div>
                      )}
                    </div>
                  ) : (
                    <p className="text-[11px] text-green-600">点击&quot;AI 审查&quot;分析本集剧本的完整度、一致性、台词质量及情节衔接。</p>
                  )}
                </div>
              </>
            )}
          </div>
          <div>
            <p className="mb-2 text-xs font-semibold uppercase tracking-wide text-surface-400">分镜列表</p>
            <EpisodeStoryboardList projectId={projectId} episodeId={selectedEpisode?.id ?? 0} />
          </div>
        </div>
      </DialogContent>
    </Dialog>
  )
}
