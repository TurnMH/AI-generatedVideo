'use client'

import { CheckCircle2, Eye, Image, LayoutGrid, Loader2, Sparkles, Trash2 } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import type { Episode, Storyboard } from '@/types'
import { formatDuration } from '@/lib/projects/utils'
import { StatusBadge } from '@/components/projects/detail/StatusBadge'

type SceneSplittingEpisodeGridProps = {
  episodes: Episode[]
  scriptTabStoryboards: Storyboard[]
  episodeStoryboardDispatching: number | null
  deletingEpisodeId: number | null
  onSelectEpisode: (episode: Episode) => void
  onStartEpisodeStoryboard: (episodeId: number) => void
  onDeleteEpisode: (episode: Episode) => void
}

export function SceneSplittingEpisodeGrid({
  episodes,
  scriptTabStoryboards,
  episodeStoryboardDispatching,
  deletingEpisodeId,
  onSelectEpisode,
  onStartEpisodeStoryboard,
  onDeleteEpisode,
}: SceneSplittingEpisodeGridProps) {
  return (
    <div className="grid grid-cols-1 gap-3 md:grid-cols-2 lg:grid-cols-3">
      {episodes.map((ep) => {
        const epSbCount = scriptTabStoryboards.filter((sb) => sb.episode_id === ep.id).length
        const statusBadge = ep.status === 'script_prepping' ? (
          <Badge variant="default" className="gap-1 bg-violet-500 hover:bg-violet-500"><Loader2 className="h-3 w-3 animate-spin" />优化中</Badge>
        ) : ep.status === 'scene_splitting' ? (
          <Badge variant="default" className="gap-1"><Loader2 className="h-3 w-3 animate-spin" />拆分中</Badge>
        ) : ep.status === 'scene_ready' || ep.status === 'done' ? (
          <Badge variant="success">{epSbCount} 个分镜</Badge>
        ) : ep.status === 'failed' ? (
          <Badge variant="destructive">失败</Badge>
        ) : ep.status === 'pending' ? (
          <Badge variant="secondary">等待中</Badge>
        ) : epSbCount > 0 ? (
          <Badge variant="success">{epSbCount} 个分镜</Badge>
        ) : (
          <Badge variant="secondary">等待分镜</Badge>
        )

        return (
          <div
            key={ep.id}
            className="relative cursor-pointer rounded-lg border p-4 transition-colors hover:bg-surface-50"
            onClick={() => onSelectEpisode(ep)}
          >
            {(ep.status === 'scene_splitting' || ep.status === 'script_prepping') && (
              <div className="absolute right-3 top-3">
                <Loader2 className="h-3.5 w-3.5 animate-spin text-amber-400" />
              </div>
            )}
            <div className="mb-2 flex items-center justify-between">
              <span className="text-sm font-semibold">第 {ep.episode_number} 集</span>
              <div className="flex items-center gap-2">
                {statusBadge}
              </div>
            </div>
            <p className="mb-1 text-sm font-medium text-surface-800">{ep.title}</p>
            <p className="mb-2 line-clamp-2 text-xs text-surface-500">{ep.summary || ep.script_excerpt?.slice(0, 80)}</p>
            <div className="flex items-center justify-between">
              <div className="flex gap-3 text-xs text-surface-400">
                <span>{ep.word_count ? `${ep.word_count} 字` : null}</span>
                <span>{ep.estimated_duration ? `~${formatDuration(ep.estimated_duration)}` : null}</span>
              </div>
              <div className="flex items-center gap-1">
                <Button
                  size="sm"
                  variant="ghost"
                  className="h-6 px-2 text-xs text-primary-600 hover:bg-primary-50"
                  onClick={(e) => { e.stopPropagation(); void onStartEpisodeStoryboard(ep.id) }}
                  disabled={episodeStoryboardDispatching === ep.id}
                  title="为本集生成分镜"
                >
                  {episodeStoryboardDispatching === ep.id ? (
                    <Loader2 className="mr-1 h-3 w-3 animate-spin" />
                  ) : (
                    <LayoutGrid className="mr-1 h-3 w-3" />
                  )}
                  生成分镜
                </Button>
                <Button
                  size="sm"
                  variant="ghost"
                  className="h-6 w-6 p-0 text-red-400 hover:bg-red-50 hover:text-red-600"
                  onClick={(e) => { e.stopPropagation(); onDeleteEpisode(ep) }}
                  disabled={deletingEpisodeId === ep.id}
                  title="删除本集"
                >
                  {deletingEpisodeId === ep.id ? (
                    <Loader2 className="h-3 w-3 animate-spin" />
                  ) : (
                    <Trash2 className="h-3 w-3" />
                  )}
                </Button>
              </div>
            </div>
          </div>
        )
      })}
    </div>
  )
}

type EpisodeGridProps = {
  episodes: Episode[]
  extractingEpisodeAssets: number | null
  generatingEpisodeAssets: number | null
  autoOptimizingEpisode: number | null
  optimizingEpisode: number | null
  reviewingEpisode: number | null
  applyingOptimized: number | null
  onSelectEpisode: (episode: Episode) => void
  onAutoStartEpisodeAssets: (episodeId: number, episodeNum: number) => void
  onExtractEpisodeAssets: (episodeId: number, episodeNum: number) => void
  onGenerateEpisodeAssetsFromScript: (episodeId: number, episodeNum: number) => void
  onAutoOptimizeReview: (episode: Episode) => void
  onOptimizeEpisode: (episode: Episode) => void
  onApplyOptimizedText: (episode: Episode) => void
  onReviewEpisode: (episode: Episode) => void
}

export function EpisodeGrid({
  episodes,
  extractingEpisodeAssets,
  generatingEpisodeAssets,
  autoOptimizingEpisode,
  optimizingEpisode,
  reviewingEpisode,
  applyingOptimized,
  onSelectEpisode,
  onAutoStartEpisodeAssets,
  onExtractEpisodeAssets,
  onGenerateEpisodeAssetsFromScript,
  onAutoOptimizeReview,
  onOptimizeEpisode,
  onApplyOptimizedText,
  onReviewEpisode,
}: EpisodeGridProps) {
  return (
    <div className="grid grid-cols-1 gap-3 md:grid-cols-2 lg:grid-cols-3">
      {episodes.map((ep) => (
        <div key={ep.id} className="rounded-lg border p-4 transition-colors hover:bg-surface-50">
          <div className="mb-2 flex items-center justify-between">
            <span className="text-sm font-semibold">第 {ep.episode_number} 集</span>
            <div className="flex items-center gap-2">
              <StatusBadge status={ep.status} />
              {ep.optimize_status === 'optimizing' && (
                <span className="inline-flex items-center gap-1 rounded-full bg-amber-100 px-2 py-0.5 text-[10px] text-amber-700">
                  <Loader2 className="h-2.5 w-2.5 animate-spin" />格式化中
                </span>
              )}
              {ep.optimize_status === 'done' && (
                <span className="inline-flex items-center gap-1 rounded-full bg-green-100 px-2 py-0.5 text-[10px] text-green-700">
                  <CheckCircle2 className="h-2.5 w-2.5" />已格式化
                </span>
              )}
              {(ep.script_excerpt || ep.summary) && (
                <Button
                  size="sm"
                  variant="ghost"
                  className="h-6 px-2 text-xs text-primary-600 hover:text-primary-800"
                  onClick={() => onSelectEpisode(ep)}
                  title="查看本集详情"
                >
                  <Eye className="mr-1 h-3 w-3" />
                  查看详情
                </Button>
              )}
            </div>
          </div>
          <p className="mb-1 text-sm font-medium text-surface-800">{ep.title}</p>
          <p className="mb-2 line-clamp-2 text-xs text-surface-500">{ep.summary}</p>
          {ep.keywords?.length > 0 && (
            <div className="mb-2 flex flex-wrap gap-1">
              {ep.keywords.slice(0, 6).map((k) => (
                <span key={k} className="rounded bg-surface-100 px-1.5 py-0.5 text-[10px] text-surface-600">{k}</span>
              ))}
            </div>
          )}
          <div className="flex items-center justify-between text-xs text-surface-400">
            <div className="flex gap-3">
              <span>{ep.word_count} 字</span>
              <span>~{formatDuration(ep.estimated_duration)}</span>
            </div>
            <div className="flex gap-1">
              <Button
                size="sm"
                variant="ghost"
                className="h-6 px-2 text-xs text-violet-600 hover:text-violet-800"
                onClick={() => onAutoStartEpisodeAssets(ep.id, ep.episode_number)}
                disabled={extractingEpisodeAssets === ep.id || generatingEpisodeAssets === ep.id}
                title="自动执行：先提取本集资源，再开始生成本集资源图"
              >
                {extractingEpisodeAssets === ep.id || generatingEpisodeAssets === ep.id ? (
                  <Loader2 className="mr-1 h-3 w-3 animate-spin" />
                ) : (
                  <Sparkles className="mr-1 h-3 w-3" />
                )}
                自动开始提取生成
              </Button>
              <Button
                size="sm"
                variant="ghost"
                className="h-6 px-2 text-xs text-orange-500 hover:text-orange-700"
                onClick={() => onExtractEpisodeAssets(ep.id, ep.episode_number)}
                disabled={extractingEpisodeAssets === ep.id || generatingEpisodeAssets === ep.id}
                title="从本集剧本中提取角色、场景等资源"
              >
                {extractingEpisodeAssets === ep.id ? (
                  <Loader2 className="mr-1 h-3 w-3 animate-spin" />
                ) : (
                  <Sparkles className="mr-1 h-3 w-3" />
                )}
                提取资源
              </Button>
              <Button
                size="sm"
                variant="ghost"
                className="h-6 px-2 text-xs text-blue-500 hover:text-blue-700"
                onClick={() => onGenerateEpisodeAssetsFromScript(ep.id, ep.episode_number)}
                disabled={extractingEpisodeAssets === ep.id || generatingEpisodeAssets === ep.id}
                title="生成本集所有待处理资源的图片"
              >
                {generatingEpisodeAssets === ep.id ? (
                  <Loader2 className="mr-1 h-3 w-3 animate-spin" />
                ) : (
                  <Image className="mr-1 h-3 w-3" />
                )}
                按集生成
              </Button>
            </div>
          </div>
          <div className="mt-2 flex items-center gap-1 border-t border-surface-100 pt-2">
            <Button
              size="sm"
              variant="ghost"
              className="h-6 px-2 text-xs text-purple-600 hover:bg-purple-50 hover:text-purple-800"
              onClick={() => { onAutoOptimizeReview(ep); onSelectEpisode(ep) }}
              disabled={autoOptimizingEpisode === ep.id || optimizingEpisode === ep.id || reviewingEpisode === ep.id}
              title="AI 自动完成：转剧本格式 → 审查 → 弥补不足"
            >
              {autoOptimizingEpisode === ep.id ? <Loader2 className="mr-1 h-3 w-3 animate-spin" /> : <Sparkles className="mr-1 h-3 w-3" />}
              AI 一键优化
            </Button>
            <Button
              size="sm"
              variant="ghost"
              className="h-6 px-2 text-xs text-amber-600 hover:bg-amber-50 hover:text-amber-800"
              onClick={() => onOptimizeEpisode(ep)}
              disabled={optimizingEpisode === ep.id || autoOptimizingEpisode === ep.id}
              title="将本集小说原文转化为标准剧本格式"
            >
              {optimizingEpisode === ep.id ? <Loader2 className="mr-1 h-3 w-3 animate-spin" /> : null}
              转剧本格式
              {ep.optimize_status === 'done' && <span className="ml-1 rounded bg-amber-100 px-1 text-[9px] text-amber-700">✓</span>}
            </Button>
            {ep.optimize_status === 'done' && ep.optimized_text && (
              <Button
                size="sm"
                variant="ghost"
                className="h-6 px-2 text-xs text-amber-500 hover:bg-amber-50"
                onClick={() => onApplyOptimizedText(ep)}
                disabled={applyingOptimized === ep.id}
                title="将优化后的剧本格式应用为正式内容"
              >
                {applyingOptimized === ep.id ? <Loader2 className="mr-1 h-3 w-3 animate-spin" /> : null}
                确认应用
              </Button>
            )}
            <Button
              size="sm"
              variant="ghost"
              className="h-6 px-2 text-xs text-green-600 hover:bg-green-50 hover:text-green-800"
              onClick={() => { onReviewEpisode(ep); onSelectEpisode(ep) }}
              disabled={reviewingEpisode === ep.id || autoOptimizingEpisode === ep.id}
              title="AI 审查本集剧本质量与一致性"
            >
              {reviewingEpisode === ep.id ? <Loader2 className="mr-1 h-3 w-3 animate-spin" /> : null}
              AI 审查
              {ep.review_status === 'done' && <span className="ml-1 rounded bg-green-100 px-1 text-[9px] text-green-700">✓</span>}
            </Button>
          </div>
        </div>
      ))}
    </div>
  )
}
