'use client'

import { RefreshCw } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import type { Episode, Model } from '@/types'
import type { EpisodeCountRecommendation } from '@/lib/projects/comic'

type ModelAvailability = { label: string; color: string }

type RegenerateDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  episodes: Episode[]
  usesAutoEpisodeSplit: boolean
  effectiveSplitModel: Model | null
  selectedSplitModelAvailability: ModelAvailability | null
  splitModelCapabilities: string[]
  selectedSplitModelRemark: string
  recommendedEpisodeCount: EpisodeCountRecommendation | null
  hasValidTargetEpisodes: boolean
  parsedTargetEpisodes: number
  kwSplitKeywords: string
  onKwSplitKeywordsChange: (value: string) => void
  kwCharacters: string
  onKwCharactersChange: (value: string) => void
  kwLocations: string
  onKwLocationsChange: (value: string) => void
  kwEvents: string
  onKwEventsChange: (value: string) => void
  kwProps: string
  onKwPropsChange: (value: string) => void
  autoStoryboardAfterSplit: boolean
  onAutoStoryboardAfterSplitChange: (value: boolean) => void
  onConfirm: () => void
}

export function RegenerateDialog({
  open,
  onOpenChange,
  episodes,
  usesAutoEpisodeSplit,
  effectiveSplitModel,
  selectedSplitModelAvailability,
  splitModelCapabilities,
  selectedSplitModelRemark,
  recommendedEpisodeCount,
  hasValidTargetEpisodes,
  parsedTargetEpisodes,
  kwSplitKeywords,
  onKwSplitKeywordsChange,
  kwCharacters,
  onKwCharactersChange,
  kwLocations,
  onKwLocationsChange,
  kwEvents,
  onKwEventsChange,
  kwProps,
  onKwPropsChange,
  autoStoryboardAfterSplit,
  onAutoStoryboardAfterSplitChange,
  onConfirm,
}: RegenerateDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>重新自动分集</DialogTitle>
        </DialogHeader>
        <div className="space-y-3 pt-2">
          <div className="rounded-md bg-amber-50 border border-amber-200 p-3">
            <p className="text-xs text-amber-800 font-medium">⚠️ 重新生成将清除以下已有数据：</p>
            <ul className="text-xs text-amber-700 mt-1 list-disc list-inside space-y-0.5">
              <li>所有剧本分集</li>
              <li>所有分镜片段</li>
              <li>所有已提取的资源（包括提取中的）</li>
              <li>所有已生成的视频</li>
            </ul>
            <p className="text-xs text-amber-600 mt-1">锁定的资源不受影响。</p>
          </div>
          <p className="text-xs text-surface-500">
            输入关键词可帮助 AI 更精准地拆分和理解内容。留空则由 AI 自动提取。
          </p>
          <div className="rounded-md border border-primary-100 bg-primary-50/60 p-3">
            <div className="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
              <div>
                <p className="text-xs font-medium text-surface-800">本次分集使用模型</p>
                <p className="mt-1 text-xs text-surface-500">
                  {usesAutoEpisodeSplit
                    ? '这里展示你手动选择的分集模型。确认后将按剧本内容自动拆分，无需填写目标集数。'
                    : '这里展示你手动选择的分集模型与目标分集数，确认后才会开始本次自动分集。'}
                </p>
              </div>
              {effectiveSplitModel ? (
                <div className="flex flex-wrap items-center gap-2">
                  <Badge variant="secondary">{effectiveSplitModel.name}</Badge>
                  {selectedSplitModelAvailability ? (
                    <Badge className={selectedSplitModelAvailability.color}>{selectedSplitModelAvailability.label}</Badge>
                  ) : null}
                </div>
              ) : (
                <Badge variant="outline">尚未选择分集模型</Badge>
              )}
            </div>
            <div className="mt-2 flex flex-wrap items-center gap-2">
              <Badge variant="outline">
                {usesAutoEpisodeSplit
                  ? `分集方式：按剧本自动拆分${recommendedEpisodeCount ? `（约 ${recommendedEpisodeCount.count} 集）` : ''}`
                  : `目标分集数：${hasValidTargetEpisodes ? parsedTargetEpisodes : '未填写'}`}
              </Badge>
            </div>
            {effectiveSplitModel ? (
              <>
                <div className="mt-2 flex flex-wrap gap-1">
                  {splitModelCapabilities.map((label) => (
                    <Badge key={label} variant="outline" className="text-[11px] font-normal">
                      {label}
                    </Badge>
                  ))}
                </div>
                <p className="mt-2 text-xs leading-5 text-surface-500">
                  {selectedSplitModelRemark}
                </p>
              </>
            ) : null}
          </div>
          <div className="space-y-2">
            <Label className="text-xs font-medium text-red-700">✂️ 分集关键字（每行一个，仅在未识别到章节标题时作为分集边界）</Label>
            <Textarea
              placeholder={'第一回 灵根育孕源流出 心性修持大道生\n第二回 悟彻菩提真妙理 断魔归本合元神\n第三回 四海千山皆拱伏 九幽十类尽除名'}
              value={kwSplitKeywords}
              onChange={(e: React.ChangeEvent<HTMLTextAreaElement>) => onKwSplitKeywordsChange(e.target.value)}
              rows={3}
              className="text-xs"
            />
          </div>
          <div className="border-t pt-3">
            <p className="text-xs text-surface-400 mb-2">以下关键词用于辅助 AI 理解内容（可选）：</p>
          </div>
          <div className="space-y-2">
            <Label className="text-xs font-medium text-purple-700">👤 人物</Label>
            <Input
              placeholder="孙悟空、唐僧、猪八戒、沙悟净"
              value={kwCharacters}
              onChange={(e: React.ChangeEvent<HTMLInputElement>) => onKwCharactersChange(e.target.value)}
            />
          </div>
          <div className="space-y-2">
            <Label className="text-xs font-medium text-primary-700">📍 地点</Label>
            <Input
              placeholder="花果山、东海龙宫、五行山"
              value={kwLocations}
              onChange={(e: React.ChangeEvent<HTMLInputElement>) => onKwLocationsChange(e.target.value)}
            />
          </div>
          <div className="space-y-2">
            <Label className="text-xs font-medium text-orange-700">⚡ 事件</Label>
            <Input
              placeholder="大闹天宫、三打白骨精、西天取经"
              value={kwEvents}
              onChange={(e: React.ChangeEvent<HTMLInputElement>) => onKwEventsChange(e.target.value)}
            />
          </div>
          <div className="space-y-2">
            <Label className="text-xs font-medium text-green-700">🔧 道具</Label>
            <Input
              placeholder="金箍棒、紧箍咒、芭蕉扇"
              value={kwProps}
              onChange={(e: React.ChangeEvent<HTMLInputElement>) => onKwPropsChange(e.target.value)}
            />
          </div>
          <div className="rounded-md border border-surface-200 bg-surface-50 p-3">
            <div className="flex items-center justify-between gap-3">
              <div>
                <p className="text-sm font-medium text-surface-800">分集完成后自动润色优化第 1 集示范剧本</p>
                <p className="mt-1 text-xs text-surface-500">
                  仅执行文本模型的润色、优化与审查，不会自动提取资源或拆分分镜。资源与分镜请在单集列表点击「自动处理」手动启动。
                </p>
              </div>
              <Switch checked={autoStoryboardAfterSplit} onCheckedChange={onAutoStoryboardAfterSplitChange} />
            </div>
          </div>
          <div className="flex justify-end gap-2 pt-3">
            <Button variant="outline" onClick={() => onOpenChange(false)}>取消</Button>
            <Button onClick={onConfirm}>
              <RefreshCw className="mr-1.5 h-3.5 w-3.5" />
              {episodes.length > 0 ? '确认重新分集' : '开始自动分集'}
            </Button>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  )
}
