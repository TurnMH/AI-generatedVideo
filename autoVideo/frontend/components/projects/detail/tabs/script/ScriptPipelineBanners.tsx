'use client'

import { CheckCircle2, Loader2, RefreshCw, Sparkles } from 'lucide-react'
import { Button } from '@/components/ui/button'
import type { Episode, Project } from '@/types'
import type { VideoPipelineSnapshot } from '@/lib/projects/pipeline-status'
import { isCommentaryProject } from '@/lib/projects/commentary-project'
import { getProgressStallMeta, SCRIPT_PROGRESS_STALL_MS } from '@/lib/projects/utils'

type ScriptPipelineBannersProps = {
  pipeline: VideoPipelineSnapshot
  project: Project
  episodes: Episode[]
  splitProgressSummary: string
  splitProgressPercent: number
  sceneReadyCount: number
  sceneProcessingSummary: string
  sceneProcessingProgress: number
  onRetryStalledScript: () => void
}

export function ScriptPipelineBanners({
  pipeline,
  project,
  episodes,
  splitProgressSummary,
  splitProgressPercent,
  sceneReadyCount,
  sceneProcessingSummary,
  sceneProcessingProgress,
  onRetryStalledScript,
}: ScriptPipelineBannersProps) {
  const scriptProgressStalled = getProgressStallMeta(project.progress?.updated_at, SCRIPT_PROGRESS_STALL_MS)
  const commentaryProject = isCommentaryProject(project)

  if (pipeline.phase === 'episode_splitting') {
    return (
      <div className="rounded-xl border border-primary-100 bg-gradient-to-b from-primary-50/80 to-white px-5 py-5">
        <div className="flex items-start gap-3">
          <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-primary-100">
            <Loader2 className="h-4 w-4 animate-spin text-primary-600" />
          </div>
          <div className="min-w-0 flex-1">
            <div className="flex flex-wrap items-center gap-2">
              <p className="text-sm font-semibold text-primary-900">剧本分集中</p>
              {project.progress?.episode_split?.total ? (
                <span className="rounded-full bg-primary-100 px-2 py-0.5 text-[10px] font-medium text-primary-700">
                  {project.progress.episode_split.completed ?? 0}/{project.progress.episode_split.total} 集
                </span>
              ) : null}
            </div>
            <p className="mt-1 text-sm leading-6 text-primary-800">{pipeline.activeDetail || splitProgressSummary}</p>
            <p className="mt-1 text-xs leading-5 text-primary-600">
              分集完成后会自动出现在下方列表，详细进度见上方「项目总控」。
            </p>
            {splitProgressPercent > 0 ? (
              <div className="mt-3 h-1.5 w-full overflow-hidden rounded-full bg-primary-100">
                <div className="h-full rounded-full bg-primary-500 transition-all duration-700" style={{ width: `${splitProgressPercent}%` }} />
              </div>
            ) : null}
          </div>
        </div>

        {scriptProgressStalled && (
          <div className="mt-4 rounded-lg border border-amber-200 bg-amber-50 px-4 py-2.5 text-xs text-amber-700">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <span>剧本解析超过 2 分钟无进展，可能已卡住。</span>
              <Button size="sm" variant="outline" className="h-7 border-amber-300 bg-white text-amber-700 hover:bg-amber-100" onClick={onRetryStalledScript}>
                <RefreshCw className="mr-1 h-3 w-3" />
                重新拉起
              </Button>
            </div>
          </div>
        )}
      </div>
    )
  }

  if (pipeline.phase === 'script_prepping') {
    return (
      <div className="rounded-xl border border-blue-100 bg-gradient-to-r from-blue-50 to-sky-50 px-4 py-3.5">
        <div className="flex items-start gap-3">
          <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-blue-100">
            <Loader2 className="h-4 w-4 animate-spin text-blue-600" />
          </div>
          <div className="min-w-0 flex-1">
            <p className="text-sm font-semibold text-blue-900">
              {commentaryProject ? '原文自动处理中' : '剧本润色与自动准备中'}
            </p>
            <p className="mt-1 text-sm leading-6 text-blue-800">{pipeline.activeDetail}</p>
            <p className="mt-1 text-xs text-blue-600">
              {commentaryProject
                ? `分集已完成（${episodes.length} 集）。系统将直接使用上传原文提取资源并拆分分镜，不会进行 AI 润色优化。`
                : `分集已完成（${episodes.length} 集）。系统正在润色剧本并串联后续流程，这不是分镜拆分。`}
            </p>
          </div>
        </div>
      </div>
    )
  }

  if (pipeline.phase === 'scene_splitting') {
    return (
      <div className="rounded-xl border border-violet-200 bg-gradient-to-r from-violet-50 to-purple-50 px-4 py-3.5">
        <div className="flex items-start gap-3">
          <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-violet-100">
            <Loader2 className="h-4 w-4 animate-spin text-violet-600" />
          </div>
          <div className="min-w-0 flex-1">
            <div className="flex flex-wrap items-center gap-2">
              <p className="text-sm font-semibold text-violet-900">分镜拆分中</p>
              <span className="inline-flex items-center gap-1 rounded-full bg-violet-100 px-2 py-0.5 text-[10px] font-medium text-violet-600">
                {sceneReadyCount}/{Math.max(episodes.length, 1)} 集已拆分
              </span>
            </div>
            <p className="mt-1 text-sm leading-6 text-violet-800">{pipeline.activeDetail}</p>
            <p className="mt-1 text-xs text-violet-600">{sceneProcessingSummary}</p>
            <div className="mt-2 h-1.5 w-full overflow-hidden rounded-full bg-violet-200">
              <div className="h-full rounded-full bg-violet-500 transition-all duration-700" style={{ width: `${sceneProcessingProgress}%` }} />
            </div>
            {scriptProgressStalled && (
              <div className="mt-2 rounded-lg border border-amber-200 bg-amber-50/80 px-3 py-2 text-xs text-amber-800">
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <span>分镜拆分进度长时间未更新，可能已卡住。</span>
                  <Button size="sm" variant="outline" className="h-7 border-amber-300 bg-white text-amber-700 hover:bg-amber-100" onClick={onRetryStalledScript}>
                    <RefreshCw className="mr-1 h-3 w-3" />
                    重新拉起
                  </Button>
                </div>
              </div>
            )}
          </div>
        </div>
      </div>
    )
  }

  return null
}

type EpisodeSplitDoneBannerProps = {
  episodesCount: number
  nextStepHint: string
  commentaryProject?: boolean
}

export function EpisodeSplitDoneBanner({ episodesCount, nextStepHint, commentaryProject = false }: EpisodeSplitDoneBannerProps) {
  return (
    <div className="mb-4 rounded-xl border border-emerald-200 bg-emerald-50/70 px-4 py-3">
      <div className="flex items-start gap-3">
        <CheckCircle2 className="mt-0.5 h-5 w-5 shrink-0 text-emerald-600" />
        <div className="min-w-0 flex-1">
          <p className="text-sm font-semibold text-emerald-900">分集已完成（{episodesCount} 集）</p>
          <p className="mt-1 text-xs leading-5 text-emerald-700">
            {episodesCount > 1
              ? (commentaryProject
                ? '系统已基于原文自动处理第 1 集示范流程（资源 → 分镜），其余分集请在左侧单集列表点击「自动处理」。'
                : '系统已自动润色优化第 1 集示范剧本（仅文本），资源与分镜请在左侧单集列表点击「自动处理」。')
              : nextStepHint}
          </p>
        </div>
      </div>
    </div>
  )
}

type BatchFormattingBannerProps = {
  episodes: Episode[]
  commentaryProject?: boolean
}

export function BatchFormattingBanner({ episodes, commentaryProject = false }: BatchFormattingBannerProps) {
  if (commentaryProject) return null
  const formattingCount = episodes.filter((ep) => ep.optimize_status === 'optimizing').length
  const formattedCount = episodes.filter((ep) => ep.optimize_status === 'done').length
  const pendingCount = episodes.filter((ep) => (ep.optimize_status ?? '') === '').length
  const isAutoFormatting = formattingCount > 0 || (formattedCount > 0 && formattedCount < episodes.length && pendingCount > 0)

  if (!isAutoFormatting && formattingCount === 0) return null

  const progressPct = episodes.length > 0 ? Math.round((formattedCount / episodes.length) * 100) : 0

  return (
    <div className="mb-4 rounded-xl border border-violet-200 bg-gradient-to-r from-violet-50 to-purple-50/60 px-4 py-3">
      <div className="flex items-center gap-3">
        <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-violet-100">
          <Loader2 className="h-4 w-4 animate-spin text-violet-600" />
        </div>
        <div className="flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <p className="text-sm font-semibold text-violet-900">AI 自动处理中 · 剧本润色</p>
            <span className="rounded-full bg-violet-100 px-2 py-0.5 text-[10px] font-medium text-violet-600">
              {formattedCount}/{episodes.length} 集
            </span>
          </div>
          <p className="mt-0.5 text-xs text-violet-600">
            {formattingCount > 0 ? `当前 ${formattingCount} 集正在润色` : `剩余 ${pendingCount} 集待处理`}
            ，仅处理剧本文本，不会自动提取资源或出图；完成后请进入各集工作台手动操作，或点击「自动处理本集」
          </p>
          <div className="mt-1.5 h-1.5 w-full overflow-hidden rounded-full bg-violet-200">
            <div
              className="h-full rounded-full bg-violet-500 transition-all duration-700"
              style={{ width: `${progressPct}%` }}
            />
          </div>
        </div>
      </div>
    </div>
  )
}

type AssetExtractionBannerProps = {
  projectStatus: Project['status']
  extractionInProgress: boolean
  assetGenerating: boolean
  extractionDone: boolean
  extractTotal: number
  onStartAssetExtraction: () => void
}

export function AssetExtractionBanner({
  projectStatus,
  extractionInProgress,
  assetGenerating,
  extractionDone,
  extractTotal,
  onStartAssetExtraction,
}: AssetExtractionBannerProps) {
  if (!['draft', 'script_processing', 'script_ready', 'asset_generating'].includes(projectStatus)) {
    return null
  }

  return (
    <div className={`mb-4 rounded-lg border px-4 py-3 ${
      extractionInProgress || assetGenerating
        ? 'border-yellow-200 bg-yellow-50'
        : extractionDone
          ? 'border-green-200 bg-green-50'
          : 'border-blue-100 bg-primary-50'
    }`}>
      <div className="flex flex-col gap-4">
        <div className="flex items-center justify-between">
          <div className="flex-1">
            {extractionInProgress || assetGenerating ? (
              <>
                <div className="flex items-center gap-2">
                  <Loader2 className="h-4 w-4 animate-spin text-yellow-600" />
                  <p className="text-sm font-semibold text-yellow-800">AI 自动处理中 · 资源提取</p>
                  <span className="rounded-full bg-yellow-100 px-2 py-0.5 text-[10px] font-medium text-yellow-700">步骤 2/3</span>
                </div>
                <p className="mt-1 text-xs text-yellow-700">
                  正在从剧本文本中识别并提取角色、场景、道具资源，完成后可在各集工作台继续出图
                </p>
              </>
            ) : extractionDone ? (
              <>
                <div className="flex items-center gap-2">
                  <CheckCircle2 className="h-4 w-4 text-green-600" />
                  <p className="text-sm font-medium text-green-800">资源提取完成</p>
                </div>
                <p className="mt-1 text-xs text-green-600">
                  共提取 {extractTotal} 项资源，可前往「资源」标签页查看与生成图像
                </p>
              </>
            ) : (
              <>
                <p className="text-sm font-medium text-primary-800">分集完成，下一步：提取资源</p>
                <p className="text-xs text-primary-600">手动提取前会先清除旧资源，再重新识别角色、场景、道具等内容</p>
              </>
            )}
          </div>
          <Button
            size="sm"
            onClick={onStartAssetExtraction}
            disabled={assetGenerating}
            className="ml-4 shrink-0"
            title={extractionDone ? '清除旧资源后重新提取角色、场景、道具等资源' : extractionInProgress ? '重置提取状态并重新开始' : '手动提取角色、场景、道具等资源'}
          >
            {assetGenerating ? (
              <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
            ) : (
              <Sparkles className="mr-1.5 h-3.5 w-3.5" />
            )}
            {extractionDone ? '清除后重新提取' : extractionInProgress ? '重置并重新提取' : '手动开始提取'}
          </Button>
        </div>
      </div>
    </div>
  )
}
