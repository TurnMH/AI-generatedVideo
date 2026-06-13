'use client'

import { AlertTriangle, Loader2 } from 'lucide-react'
import type { ReactNode } from 'react'
import { Button } from '@/components/ui/button'
import type { Project } from '@/types'
import type { VideoPipelineSnapshot } from '@/lib/projects/pipeline-status'

type RecoveryAction = {
  label: string
  description: string
  onClick: () => void | Promise<void>
} | null

type ProjectPipelineBannersProps = {
  project: Project
  pipelineSnapshot: VideoPipelineSnapshot
  assetExtractionRunning: boolean
  assetActiveCount: number
  isExtractingAssets: boolean
  isExtractingStoryboards: boolean
  storyboardRunning: boolean
  structuredPhaseLabel?: string
  structuredNextStep?: string
  progressMessage?: string
  recoveryAction?: RecoveryAction
}

export function ProjectPipelineBanners({
  project,
  pipelineSnapshot,
  assetExtractionRunning,
  assetActiveCount,
  isExtractingAssets,
  isExtractingStoryboards,
  storyboardRunning,
  structuredPhaseLabel,
  structuredNextStep,
  progressMessage,
  recoveryAction,
}: ProjectPipelineBannersProps) {
  const isScriptProcessing = pipelineSnapshot.phase === 'episode_splitting'
    || pipelineSnapshot.phase === 'script_prepping'
  const isAssetExtracting = assetExtractionRunning || assetActiveCount > 0 || isExtractingAssets
  const isStoryboardRunning = isExtractingStoryboards || storyboardRunning || pipelineSnapshot.phase === 'scene_splitting'
  const showRecoveryBanner = project.status === 'failed' || project.status === 'paused'

  if (!showRecoveryBanner && !isScriptProcessing && !isAssetExtracting && !isStoryboardRunning) {
    return null
  }

  const banners: Array<{ icon: ReactNode; title: string; desc: string; step: string; color: string }> = []

  if (isScriptProcessing) {
    const postSplitRunning = pipelineSnapshot.phase === 'script_prepping'
    banners.push({
      icon: <Loader2 className="h-5 w-5 animate-spin text-blue-300" />,
      title: postSplitRunning ? '示范剧本润色优化中（仅文本）' : '剧本分集中',
      desc: pipelineSnapshot.activeDetail,
      step: postSplitRunning ? '自动准备' : '步骤 1/3',
      color: 'border-blue-400/30 bg-blue-500/10',
    })
  }

  if (isAssetExtracting) {
    banners.push({
      icon: <Loader2 className="h-5 w-5 animate-spin text-amber-300" />,
      title: assetExtractionRunning ? '资源条目提取中' : `资源图生成中（${assetActiveCount} 个处理队列）`,
      desc: assetExtractionRunning
        ? 'AI 正在识别剧本中的角色、场景、道具等资源条目。提取完成后可在各集点击「自动处理本集」，或在工作台手动生成资源图与拆分分镜。'
        : '资源图正在后台生成中，可在各集工作台查看进度。',
      step: '步骤 2/3',
      color: 'border-amber-400/30 bg-amber-500/10',
    })
  }

  if (isStoryboardRunning) {
    banners.push({
      icon: <Loader2 className="h-5 w-5 animate-spin text-violet-300" />,
      title: '分镜拆分进行中',
      desc: pipelineSnapshot.activeDetail || '系统正在为各分集拆分镜头序列，完成后可在各集工作台查看分镜并批量生成图片。',
      step: '步骤 3/3',
      color: 'border-violet-400/30 bg-violet-500/10',
    })
  }

  return (
    <div className="space-y-2">
      {showRecoveryBanner ? (
        <div className="flex items-start gap-4 rounded-2xl border border-red-300/40 bg-gradient-to-r from-red-950/85 to-slate-900/80 px-5 py-4 text-white backdrop-blur-sm">
          <div className="mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-full border border-white/15 bg-white/10">
            <AlertTriangle className="h-5 w-5 text-red-200" />
          </div>
          <div className="min-w-0 flex-1">
            <div className="flex flex-wrap items-center gap-2">
              <span className="text-sm font-semibold text-white">{project.status === 'paused' ? '流程已暂停' : '流程已中断'}</span>
              <span className="rounded-full border border-white/20 bg-white/10 px-2 py-0.5 text-[10px] font-medium text-white/70">恢复建议</span>
            </div>
            <p className="mt-1 text-xs leading-5 text-white/75">
              {structuredPhaseLabel || progressMessage || '当前自动流程没有完整结束，请从当前可用阶段继续。'}
            </p>
            <p className="mt-1 text-[11px] text-white/55">
              {recoveryAction?.description || structuredNextStep || '建议优先从当前阶段继续推进，避免重复消耗。'}
            </p>
            {recoveryAction ? (
              <Button
                variant="outline"
                size="sm"
                onClick={() => void recoveryAction.onClick()}
                className="mt-3 border-white/20 bg-white/10 text-white hover:bg-white/20"
              >
                {recoveryAction.label}
              </Button>
            ) : null}
          </div>
        </div>
      ) : null}
      {banners.map((banner) => (
        <div
          key={`${banner.step}-${banner.title}`}
          className={`flex items-start gap-4 rounded-2xl border px-5 py-4 text-white backdrop-blur-sm ${banner.color} bg-gradient-to-r from-slate-900/80 to-slate-800/60`}
        >
          <div className="mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-full border border-white/15 bg-white/10">
            {banner.icon}
          </div>
          <div className="min-w-0 flex-1">
            <div className="flex flex-wrap items-center gap-2">
              <span className="text-sm font-semibold text-white">{banner.title}</span>
              <span className="rounded-full border border-white/20 bg-white/10 px-2 py-0.5 text-[10px] font-medium text-white/70">
                {banner.step}
              </span>
              <span className="inline-flex items-center gap-1 rounded-full border border-white/15 bg-white/10 px-2 py-0.5 text-[10px] text-white/60">
                <span className="h-1.5 w-1.5 animate-pulse rounded-full bg-green-400" />
                进行中
              </span>
            </div>
            <p className="mt-1 text-xs leading-5 text-white/70">{banner.desc}</p>
          </div>
        </div>
      ))}
    </div>
  )
}
