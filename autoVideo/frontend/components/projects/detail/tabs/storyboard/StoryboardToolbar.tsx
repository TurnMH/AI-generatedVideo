'use client'

import {
  Bot,
  LayoutGrid,
  Loader2,
  Pause,
  Play,
  RefreshCw,
  RotateCcw,
  Search,
  Sparkles,
  Video,
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
} from '@/components/ui/dropdown-menu'
import { getProviderLabel } from '@/lib/model-feasibility'
import type { VideoModelCapability } from '@/lib/video-style-config'

type StatsData = {
  total: number
  pending: number
  generating: number
  paused: number
  completed: number
  failed: number
  voided: number
}

export function StoryboardToolbar({
  keyword,
  onKeywordChange,
  statusFilter,
  onStatusFilterChange,
  episodeId,
  onExtractStoryboards,
  isExtractingStoryboards,
  awaitingAutoStoryboard,
  storyboardButtonDisabled,
  extractStoryboardLabel,
  storyboardItemLabel,
  storyboardImageLabel,
  storyboardVideoLabel,
  storyboardGenerateLabel,
  isSerial,
  hideActionBar,
  isAuditingContinuity,
  onAuditContinuity,
  pausingGeneration,
  onPauseGeneration,
  resumingGeneration,
  onResumeGeneration,
  stats,
  isActive,
  storyboardAssetsReady,
  storyboardAssetsBlockingReason,
  episodeFilter,
  onOpenBatchDialog,
  generatingAllVideos,
  onGenerateAllEpisodeVideos,
  vtVideoModelOptions,
  videoModelAvailability,
}: {
  keyword: string
  onKeywordChange: (value: string) => void
  statusFilter: string
  onStatusFilterChange: (value: string) => void
  episodeId?: number
  onExtractStoryboards?: () => void
  isExtractingStoryboards?: boolean
  awaitingAutoStoryboard?: boolean
  storyboardButtonDisabled?: boolean
  extractStoryboardLabel: string
  storyboardItemLabel: string
  storyboardImageLabel: string
  storyboardVideoLabel: string
  storyboardGenerateLabel: string
  isSerial: boolean
  hideActionBar?: boolean
  isAuditingContinuity: boolean
  onAuditContinuity: () => void
  pausingGeneration: boolean
  onPauseGeneration: () => void
  resumingGeneration: boolean
  onResumeGeneration: () => void
  stats: StatsData
  isActive: boolean
  storyboardAssetsReady: boolean
  storyboardAssetsBlockingReason: string
  episodeFilter: string
  onOpenBatchDialog: (kind: 'generate' | 'force' | 'retryFailed') => void
  generatingAllVideos: boolean
  onGenerateAllEpisodeVideos: (modelKey: string) => void
  vtVideoModelOptions: VideoModelCapability[]
  videoModelAvailability: Record<string, boolean>
}) {
  return (
    <div className="mb-4 flex flex-wrap items-center gap-3">
      <div className="relative min-w-[220px] flex-1 sm:max-w-xs">
        <Search className="pointer-events-none absolute left-3 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-surface-400" />
        <Input
          value={keyword}
          onChange={(e) => onKeywordChange(e.target.value)}
          placeholder="搜索场景 / 地点 / 台词 / 角色"
          className="pl-8"
        />
      </div>
      <Select value={statusFilter} onValueChange={onStatusFilterChange}>
        <SelectTrigger className="w-32">
          <SelectValue placeholder="状态" />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="all">全部状态</SelectItem>
          <SelectItem value="pending">待生成</SelectItem>
          <SelectItem value="generating">生成中</SelectItem>
          <SelectItem value="paused">已暂停</SelectItem>
          <SelectItem value="completed">已完成</SelectItem>
          <SelectItem value="failed">失败</SelectItem>
          <SelectItem value="voided">已作废</SelectItem>
        </SelectContent>
      </Select>
      <div className="ml-auto flex flex-wrap items-center gap-2">
        {episodeId !== undefined && onExtractStoryboards && (
          <Button
            onClick={onExtractStoryboards}
            disabled={storyboardButtonDisabled}
            variant="outline"
            size="sm"
            className="border-primary-200 bg-primary-50 text-primary-700 hover:bg-primary-100 hover:text-primary-800 disabled:opacity-60"
            title={awaitingAutoStoryboard ? '「自动处理本集」启动后，系统会自动发起镜头拆分' : `${extractStoryboardLabel}（会删除当前集旧${storyboardItemLabel}并重新拆分结构）`}
          >
            {isExtractingStoryboards ? <RefreshCw className="mr-1.5 h-3.5 w-3.5 animate-spin" /> : <LayoutGrid className="mr-1.5 h-3.5 w-3.5" />}
            {awaitingAutoStoryboard ? '等待自动拆分镜头' : isExtractingStoryboards ? '镜头拆分中…' : extractStoryboardLabel}
          </Button>
        )}
        {!hideActionBar && (
          <>
            <span className="text-[10px] font-medium text-surface-400">{storyboardImageLabel}</span>
            <Button
              size="sm"
              variant="outline"
              onClick={onAuditContinuity}
              disabled={isAuditingContinuity}
              title="AI 检查并补全缺失的角色、地点和描述信息"
            >
              {isAuditingContinuity ? (
                <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
              ) : (
                <Bot className="mr-1.5 h-3.5 w-3.5" />
              )}
              AI 补全缺失信息
            </Button>
            <Button
              size="sm"
              variant="outline"
              onClick={onPauseGeneration}
              disabled={pausingGeneration || (stats.generating + stats.pending === 0)}
              title={`暂停整个项目的${storyboardImageLabel}生成；已在执行中的个别任务可能自然完成`}
            >
              {pausingGeneration ? (
                <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
              ) : (
                <Pause className="mr-1.5 h-3.5 w-3.5" />
              )}
              暂停生成
            </Button>
            <Button
              size="sm"
              variant="outline"
              onClick={onResumeGeneration}
              disabled={resumingGeneration || stats.paused === 0}
              title={`继续项目下所有已暂停的${storyboardImageLabel}生成`}
            >
              {resumingGeneration ? (
                <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
              ) : (
                <Play className="mr-1.5 h-3.5 w-3.5" />
              )}
              继续生成
            </Button>
          </>
        )}
        {stats.failed > 0 && (
          <Button
            size="sm"
            variant="outline"
            className="border-red-200 text-red-600 hover:bg-red-50"
            title={`选择一个或多个模型，为同一条${storyboardItemLabel}生成多版候选后再挑选`}
            onClick={() => onOpenBatchDialog('retryFailed')}
          >
            <RefreshCw className="mr-1.5 h-3.5 w-3.5" />
            重试失败 ({stats.failed})
          </Button>
        )}
        {!hideActionBar && (
          <>
            {episodeFilter !== 'all' && (storyboardAssetsReady || isActive || stats.paused > 0) && (
              <>
                {isActive ? (
                  <Button size="sm" variant="outline" disabled>
                    <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
                    生成中...
                  </Button>
                ) : (
                  <Button
                    size="sm"
                    variant="outline"
                    disabled={!storyboardAssetsReady}
                    title={!storyboardAssetsReady ? storyboardAssetsBlockingReason || '请先完成资源图生成' : `选择一个或多个模型，为本集同一条${storyboardItemLabel}生成多版候选后再挑选`}
                    onClick={() => onOpenBatchDialog('generate')}
                  >
                    <Sparkles className="mr-1.5 h-3.5 w-3.5" />
                    {`生成本集${storyboardImageLabel}`}
                  </Button>
                )}
                <Button
                  size="sm"
                  variant="outline"
                  disabled={isActive}
                  title={isActive ? `${storyboardGenerateLabel}进行中，请等待或先暂停` : `仅重置本集已有${storyboardImageLabel}并重新出图，不会重新拆分${storyboardItemLabel}结构；多选模型时会为同一条${storyboardItemLabel}追加多版候选`}
                  onClick={() => onOpenBatchDialog('force')}
                >
                  <RotateCcw className="mr-1.5 h-3.5 w-3.5" />
                  重生成图（本集）
                </Button>
              </>
            )}
            {episodeFilter === 'all' && (
              isActive ? (
                <Button size="sm" disabled>
                  <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
                  生成中...
                </Button>
              ) : (
                <Button
                  size="sm"
                  disabled={!storyboardAssetsReady}
                  title={!storyboardAssetsReady ? storyboardAssetsBlockingReason || '请先完成资源图生成' : `选择一个或多个模型，为当前范围内同一条${storyboardItemLabel}生成多版候选后再挑选`}
                  onClick={() => onOpenBatchDialog('generate')}
                >
                  <Sparkles className="mr-1.5 h-3.5 w-3.5" />
                  {`一键生成${storyboardImageLabel}`}
                </Button>
              )
            )}
          </>
        )}
        {stats.completed > 0 && (
          <>
            <span className="h-5 w-px bg-surface-200" />
            <span className="text-[10px] font-medium text-green-700">视频</span>
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button size="sm" variant="outline" className="border-green-200 text-green-700 hover:bg-green-50" disabled={generatingAllVideos} title={`选择模型为所有已完成${isSerial ? '场景首帧' : '分镜图片'}批量生成${storyboardVideoLabel}`}>
                  {generatingAllVideos ? (
                    <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
                  ) : (
                    <Video className="mr-1.5 h-3.5 w-3.5" />
                  )}
                  {`一键生成${storyboardVideoLabel}`}
                </Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end" className="w-80">
                <DropdownMenuLabel className="text-[10px] text-surface-400">选择视频生成模型</DropdownMenuLabel>
                <DropdownMenuSeparator />
                {vtVideoModelOptions.map((m, idx) => {
                  const avail = videoModelAvailability[m.key]
                  return (
                    <DropdownMenuItem key={m.key} className={`cursor-pointer px-3 py-2 ${avail === false ? 'opacity-50' : ''}`} onClick={() => onGenerateAllEpisodeVideos(m.key)}>
                      <div className="flex w-full items-start gap-2">
                        <div className="mt-0.5 flex flex-col items-center gap-0.5">
                          <span className="text-sm">{m.icon}</span>
                          <span className="rounded-full bg-surface-200 px-1 text-[8px] text-surface-500 font-bold">#{idx + 1}</span>
                        </div>
                        <div className="flex-1 min-w-0">
                          <div className="flex items-center gap-1.5 flex-wrap">
                            <span className="text-xs font-semibold">{m.label}</span>
                            {m.speed === 'fast' && <span className="rounded bg-green-100 px-1 py-0 text-[9px] text-green-700">⚡ 快</span>}
                            {m.quality === 'high' && <span className="rounded bg-purple-100 px-1 py-0 text-[9px] text-purple-700">★ 高质</span>}
                            {avail === true && <span className="rounded bg-emerald-100 px-1 py-0 text-[9px] text-emerald-700">● 可用</span>}
                            {avail === false && <span className="rounded bg-red-100 px-1 py-0 text-[9px] text-red-600">● 未配置</span>}
                          </div>
                          <p className="text-[10px] text-surface-400 leading-none mt-0.5">{getProviderLabel(m.provider)}</p>
                          <p className="mt-0.5 text-[10px] text-surface-500 leading-tight">{m.desc}</p>
                          <div className="mt-1 flex flex-wrap gap-1">
                            {m.tags.map(t => (
                              <span key={t} className="rounded-full bg-surface-100 px-1.5 py-0 text-[9px] text-surface-500">{t}</span>
                            ))}
                          </div>
                        </div>
                      </div>
                    </DropdownMenuItem>
                  )
                })}
              </DropdownMenuContent>
            </DropdownMenu>
          </>
        )}
      </div>
    </div>
  )
}
