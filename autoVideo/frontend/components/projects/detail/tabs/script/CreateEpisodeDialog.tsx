'use client'

import { Loader2, Plus } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'

type CreateEpisodeDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  nextManualEpisodeNumber: number
  manualEpisodeNumber: string
  onManualEpisodeNumberChange: (value: string) => void
  manualEpisodeNumberTaken: boolean
  parsedManualEpisodeNumber: number
  manualEpisodeTitle: string
  onManualEpisodeTitleChange: (value: string) => void
  manualEpisodeSummary: string
  onManualEpisodeSummaryChange: (value: string) => void
  manualEpisodeContent: string
  onManualEpisodeContentChange: (value: string) => void
  creatingEpisode: boolean
  onCreate: () => void
}

export function CreateEpisodeDialog({
  open,
  onOpenChange,
  nextManualEpisodeNumber,
  manualEpisodeNumber,
  onManualEpisodeNumberChange,
  manualEpisodeNumberTaken,
  parsedManualEpisodeNumber,
  manualEpisodeTitle,
  onManualEpisodeTitleChange,
  manualEpisodeSummary,
  onManualEpisodeSummaryChange,
  manualEpisodeContent,
  onManualEpisodeContentChange,
  creatingEpisode,
  onCreate,
}: CreateEpisodeDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>手动创建分集</DialogTitle>
        </DialogHeader>
        <div className="space-y-4 pt-2">
          <div className="rounded-md border border-primary-100 bg-primary-50/60 p-3">
            <p className="text-sm font-medium text-surface-800">适合补充插叙、番外、加更，或在没有剧本拆分结果时先手动搭建分集结构。</p>
            <p className="mt-1 text-xs text-surface-500">
              建议序号：第 {nextManualEpisodeNumber} 集。创建后可直接用于资源提取、从分集创建分镜，以及后续按集生成视频。
            </p>
          </div>
          <div className="space-y-2">
            <Label>分集序号</Label>
            <Input
              type="number"
              min={1}
              step={1}
              value={manualEpisodeNumber}
              onChange={(event) => onManualEpisodeNumberChange(event.target.value.replace(/[^\d]/g, ''))}
              placeholder="例如 1"
            />
            {manualEpisodeNumberTaken ? (
              <p className="text-xs text-red-500">第 {parsedManualEpisodeNumber} 集已存在，请换一个未占用的分集序号。</p>
            ) : null}
          </div>
          <div className="space-y-2">
            <Label>分集标题</Label>
            <Input
              value={manualEpisodeTitle}
              onChange={(event) => onManualEpisodeTitleChange(event.target.value)}
              placeholder="例如：初入花果山"
            />
          </div>
          <div className="space-y-2">
            <Label>分集摘要</Label>
            <Textarea
              value={manualEpisodeSummary}
              onChange={(event) => onManualEpisodeSummaryChange(event.target.value)}
              rows={3}
              placeholder="简要描述这一集的主要剧情、人物和场景，方便后续提取资源与生成分镜。"
            />
          </div>
          <div className="space-y-2">
            <Label>
              分集正文内容
              <span className="ml-1 text-xs font-normal text-surface-400">（可选）分镜生成时会优先使用此内容</span>
            </Label>
            <Textarea
              value={manualEpisodeContent}
              onChange={(event) => onManualEpisodeContentChange(event.target.value)}
              rows={5}
              placeholder="粘贴或输入本集的具体剧本/小说原文，AI 将基于此内容拆分分镜。不填则使用摘要。"
            />
          </div>
          <div className="flex justify-end gap-2 pt-2">
            <Button variant="outline" onClick={() => onOpenChange(false)} disabled={creatingEpisode}>
              取消
            </Button>
            <Button onClick={onCreate} disabled={creatingEpisode}>
              {creatingEpisode ? <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" /> : <Plus className="mr-1.5 h-3.5 w-3.5" />}
              创建分集
            </Button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  )
}
