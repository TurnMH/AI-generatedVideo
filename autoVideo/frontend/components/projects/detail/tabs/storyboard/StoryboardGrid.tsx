'use client'

import {
  ArrowUpToLine,
  Ban,
  Eye,
  LayoutGrid,
  Loader2,
  MessageSquare,
  Play,
  RefreshCw,
  Sparkles,
  Trash2,
  Video,
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import {
  DropdownMenu,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { ZoomBadge } from '@/components/ui/image-lightbox'
import { StatusBadge } from '@/components/projects/detail/StatusBadge'
import { canTriggerStoryboardImage } from '@/lib/projects/storyboard-image'
import { formatStoryboardErrorMessage } from '@/lib/projects/storyboard-status'
import type { ImageModelOption } from '@/lib/model-display'
import type { Episode, Storyboard } from '@/types'
import { ImageModelDropdownContent } from './ImageModelDropdownContent'

export function StoryboardGrid({
  storyboards,
  episodes,
  storyboardItemLabel,
  storyboardImageLabel,
  storyboardVideoLabel,
  isSerial,
  sbDescLang,
  modelOptions,
  imageModelAvailability,
  episodeCompletedMap,
  generatingVideoEps,
  onSelectStoryboard,
  onGenerateOne,
  onSwitchVersion,
  onVoid,
  onDelete,
  onMergeWithPrevious,
  onOpenEpisodeVideoDialog,
  onCreateFromEpisodes,
}: {
  storyboards: Storyboard[]
  episodes: Episode[]
  storyboardItemLabel: string
  storyboardImageLabel: string
  storyboardVideoLabel: string
  isSerial: boolean
  sbDescLang: 'zh' | 'en'
  modelOptions: ImageModelOption[]
  imageModelAvailability: Record<string, boolean>
  episodeCompletedMap: Map<number, number>
  generatingVideoEps: Set<number>
  onSelectStoryboard: (sb: Storyboard) => void
  onGenerateOne: (sb: Storyboard, modelName?: string) => void
  onSwitchVersion: (sbId: number, versionId: number) => void
  onVoid: (id: number) => void
  onDelete: (id: number) => void
  onMergeWithPrevious: (current: Storyboard, previous: Storyboard) => void
  onOpenEpisodeVideoDialog: (episodeId: number) => void
  onCreateFromEpisodes: () => void
}) {
  if (storyboards.length === 0 && episodes.length > 0) {
    return (
      <div className="py-12 text-center">
        <LayoutGrid className="mx-auto mb-3 h-10 w-10 text-surface-300" />
        <p className="mb-4 text-sm text-surface-500">当前项目有 {episodes.length} 集，但尚未创建{storyboardItemLabel}</p>
        <Button onClick={onCreateFromEpisodes} title={`根据已有集数自动创建${storyboardItemLabel}`}>
          <Sparkles className="mr-1.5 h-4 w-4" />
          {`从集数创建${storyboardItemLabel}`}
        </Button>
      </div>
    )
  }

  if (storyboards.length === 0) {
    return <p className="py-12 text-center text-sm text-surface-400">{`暂无${storyboardItemLabel}`}</p>
  }

  const epMap = new Map(episodes.map((ep) => [ep.id, ep]))
  const grouped = new Map<number, Storyboard[]>()
  for (const sb of storyboards) {
    const epId = sb.episode_id ?? 0
    if (!grouped.has(epId)) grouped.set(epId, [])
    grouped.get(epId)!.push(sb)
  }
  const sortedGroups = Array.from(grouped.entries()).sort(([a], [b]) => {
    if (a === 0) return 1
    if (b === 0) return -1
    const epA = epMap.get(a)
    const epB = epMap.get(b)
    return (epA?.episode_number ?? a) - (epB?.episode_number ?? b)
  })

  const renderSbCard = (sb: Storyboard, previousSb: Storyboard | null) => (
    <Card key={sb.id} className={`group overflow-hidden transition-shadow hover:shadow-md ${sb.status === 'failed' ? 'ring-2 ring-red-300' : ''}`}>
      <div
        className="relative aspect-video cursor-pointer overflow-hidden bg-surface-100"
        onClick={() => onSelectStoryboard(sb)}
      >
        {sb.image_url ? (
          <img src={sb.image_url} alt={`#${sb.sequence_number}`} className="h-full w-full object-cover transition-transform group-hover:scale-105" />
        ) : (
          <div className="flex h-full items-center justify-center text-surface-300">
            <LayoutGrid className="h-8 w-8" />
          </div>
        )}
        {sb.image_url && (
          <div className="absolute right-2 bottom-2 opacity-0 transition-opacity group-hover:opacity-100">
            <ZoomBadge src={sb.image_url} alt={`#${sb.sequence_number}`} />
          </div>
        )}
      </div>
      <CardContent className="p-3">
        <div className="mb-1 flex items-center justify-between">
          <span className="text-sm font-semibold">#{sb.sequence_number}</span>
          <StatusBadge status={sb.status} />
        </div>
        <p className="mb-1 line-clamp-2 text-xs text-surface-600">
          {sbDescLang === 'en' && sb.prompt_used ? sb.prompt_used : sb.scene_description}
        </p>
        {sb.status === 'failed' && (
          <p className="mb-1 line-clamp-1 text-[11px] text-red-500" title={sb.error_msg || ''}>
            💡 {formatStoryboardErrorMessage(sb.error_msg || '')}
          </p>
        )}
        {sb.characters && sb.characters.length > 0 && (
          <div className="mb-2 flex flex-wrap gap-1">
            {sb.characters.map((c) => (
              <span key={c} className="rounded bg-surface-100 px-1.5 py-0.5 text-[10px] text-surface-600">{c}</span>
            ))}
          </div>
        )}
        {sb.versions && sb.versions.length > 1 && (
          <div className="mb-2">
            <Select
              value={String(sb.versions.find((ver) => ver.is_current)?.id ?? sb.versions.find((ver) => ver.version_number === sb.current_version)?.id ?? sb.versions[0]?.id ?? '')}
              onValueChange={(v) => onSwitchVersion(sb.id, Number(v))}
            >
              <SelectTrigger className="h-7 text-xs">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {sb.versions.map((ver) => (
                  <SelectItem key={ver.id} value={String(ver.id)}>V{ver.version_number}{ver.is_current ? ' (当前)' : ''}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        )}
        <div className="flex gap-1 opacity-0 transition-opacity group-hover:opacity-100">
          {canTriggerStoryboardImage(sb) ? (
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button
                  size="sm"
                  variant="ghost"
                  className={`h-7 px-2 text-xs ${sb.status === 'failed' ? 'text-red-500 hover:text-red-700' : sb.status === 'paused' ? 'text-yellow-700 hover:text-yellow-800' : ''}`}
                  title="选择模型生成"
                >
                  {sb.status === 'failed' ? <RefreshCw className="mr-1 h-3 w-3" /> : sb.status === 'paused' ? <Play className="mr-1 h-3 w-3" /> : <Sparkles className="mr-1 h-3 w-3" />}
                  {sb.status === 'failed' ? '重试' : sb.status === 'paused' ? '继续' : sb.status === 'completed' ? '重生成' : '生成'}
                </Button>
              </DropdownMenuTrigger>
              <ImageModelDropdownContent
                options={modelOptions}
                availability={imageModelAvailability}
                onSelect={(key) => onGenerateOne(sb, key)}
                align="start"
                showTags
                stopPropagation
              />
            </DropdownMenu>
          ) : null}
          <Button size="sm" variant="ghost" className="h-7 w-7 p-0" onClick={() => onSelectStoryboard(sb)} title="查看">
            <Eye className="h-3.5 w-3.5" />
          </Button>
          {previousSb ? (
            <Button
              size="sm"
              variant="ghost"
              className="h-7 w-7 p-0"
              onClick={() => onMergeWithPrevious(sb, previousSb)}
              title="与上一镜合并"
            >
              <ArrowUpToLine className="h-3.5 w-3.5" />
            </Button>
          ) : null}
          <Button size="sm" variant="ghost" className="h-7 w-7 p-0" onClick={() => onVoid(sb.id)} title="作废">
            <Ban className="h-3.5 w-3.5" />
          </Button>
          <Button size="sm" variant="ghost" className="h-7 w-7 p-0 text-red-500 hover:text-red-700" onClick={() => onDelete(sb.id)} title="删除">
            <Trash2 className="h-3.5 w-3.5" />
          </Button>
          <Button size="sm" variant="ghost" className="h-7 w-7 p-0" onClick={() => onSelectStoryboard(sb)} title="AI对话">
            <MessageSquare className="h-3.5 w-3.5" />
          </Button>
        </div>
      </CardContent>
    </Card>
  )

  return (
    <div className="space-y-6">
      {sortedGroups.map(([epId, sbs]) => {
        const ep = epMap.get(epId)
        const sorted = [...sbs].sort((a, b) => a.sequence_number - b.sequence_number)
        const completedCount = sorted.filter(s => s.status === 'completed').length
        return (
          <div key={epId}>
            <div className="mb-3 flex items-center justify-between border-b pb-2">
              <div className="flex items-center gap-3">
                <h3 className="text-sm font-semibold text-surface-800">
                  {ep ? `第 ${ep.episode_number} 集 · ${ep.title}` : '未分配集数'}
                </h3>
                <span className="text-xs text-surface-400">
                  {completedCount}/{sorted.length} 已完成
                </span>
              </div>
              {epId > 0 && (
                <div className="flex items-center gap-1">
                  <Button
                    size="sm"
                    variant="outline"
                    className="h-7 text-xs"
                    disabled={generatingVideoEps.has(epId) || (episodeCompletedMap.get(epId) ?? 0) === 0}
                    onClick={() => onOpenEpisodeVideoDialog(epId)}
                    title={(episodeCompletedMap.get(epId) ?? 0) === 0 ? (sorted.length > 0 ? (isSerial ? `该集有 ${sorted.length} 条${storyboardItemLabel}（${completedCount} 条已完成），尚无可用首帧图片` : `该集有 ${sorted.length} 个${storyboardItemLabel}（${completedCount} 个已完成），尚无已生成图片的${storyboardItemLabel}`) : `当前集暂无已完成${storyboardItemLabel}，无法生成${storyboardVideoLabel}`) : `为当前集选择${storyboardVideoLabel}生成参数`}
                  >
                    {generatingVideoEps.has(epId) ? (
                      <Loader2 className="mr-1 h-3 w-3 animate-spin" />
                    ) : (
                      <Video className="mr-1.5 h-3.5 w-3.5" />
                    )}
                    {`当前集生成${storyboardVideoLabel}`}
                  </Button>
                </div>
              )}
            </div>
            <div className="grid grid-cols-2 gap-4 md:grid-cols-3 lg:grid-cols-4">
              {sorted.map((sb, index) => renderSbCard(sb, index > 0 ? sorted[index - 1] : null))}
            </div>
          </div>
        )
      })}
    </div>
  )
}
