'use client'

import { Bot, Loader2, Mic, Pause, Play, RotateCcw, Sparkles, RefreshCw } from 'lucide-react'
import { Button } from '@/components/ui/button'
import type { EpisodeAssetStats, EpisodeStoryboardStats } from '@/lib/projects/episode-workspace-stats'
import type { WorkflowStep, WorkflowStepKey } from '@/lib/projects/episode-workspace-workflow-steps'
import type { SidebarTheme, WorkflowStepTheme } from '@/components/projects/detail/episode-workspace/themes'

export function EpisodeStageActionPanel({
  activeTab,
  workflowSteps,
  storyboardImageLabel,
  assetStats,
  storyboardStats,
  sidebarTheme,
  workflowTheme,
  resourceButtonDisabled,
  storyboardButtonDisabled,
  autoMatchingVoices,
  pausingGeneration,
  resumingGeneration,
  onGenerateAll,
  onAutoMatchVoices,
  onPauseGeneration,
  onResumeGeneration,
  onRegenerateAssets,
  onGenerateStoryboards,
  onAuditStoryboards,
  onRepairStoryboardMetadata,
  onPauseStoryboards,
  onResumeStoryboards,
  onRegenerateStoryboards,
  onExtractStoryboards,
  isExtractingStoryboards,
}: {
  activeTab: WorkflowStepKey
  workflowSteps: WorkflowStep[]
  storyboardImageLabel: string
  assetStats: EpisodeAssetStats
  storyboardStats: EpisodeStoryboardStats
  sidebarTheme: SidebarTheme
  workflowTheme: WorkflowStepTheme
  resourceButtonDisabled: boolean
  storyboardButtonDisabled: boolean
  autoMatchingVoices: boolean
  pausingGeneration: boolean
  resumingGeneration: boolean
  onGenerateAll: () => void
  onAutoMatchVoices: () => void
  onPauseGeneration: () => void
  onResumeGeneration: () => void
  onRegenerateAssets: () => void
  onGenerateStoryboards: () => void
  onAuditStoryboards: () => void
  onRepairStoryboardMetadata: () => void
  onPauseStoryboards: () => void
  onResumeStoryboards: () => void
  onRegenerateStoryboards: () => void
  onExtractStoryboards?: () => void
  isExtractingStoryboards?: boolean
}) {
  const stageHint = activeTab === 'assets'
    ? '继续补齐本集角色、场景和道具资源。'
    : activeTab === 'storyboard'
      ? `继续推进镜头拆分与${storyboardImageLabel}生成。`
      : activeTab === 'dubbing'
        ? '当前阶段请在下方工作台处理中配音与字幕。'
        : '当前阶段请在下方工作台继续合成视频成片。'

  return (
    <div className={`rounded-2xl border p-4 ${sidebarTheme.panel}`}>
      <div className="mb-4 flex items-start justify-between gap-3">
        <div>
          <p className={`text-sm font-semibold ${sidebarTheme.panelTitle}`}>当前阶段操作</p>
          <p className={`mt-1 text-xs ${sidebarTheme.panelDesc}`}>{stageHint}</p>
        </div>
        <span className={`rounded-full px-2 py-1 text-[10px] font-semibold ${workflowTheme.currentBadge}`}>
          {workflowSteps.find((step) => step.key === activeTab)?.label}
        </span>
      </div>

      {activeTab === 'assets' && (
        <div className="flex flex-col gap-2">
          <Button size="sm" onClick={onGenerateAll} disabled={resourceButtonDisabled} className={`w-full ${sidebarTheme.primaryButton}`}>
            {resourceButtonDisabled ? <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" /> : <Sparkles className="mr-1.5 h-3.5 w-3.5" />}
            一键生成全部
          </Button>
          <Button size="sm" variant="outline" onClick={onAutoMatchVoices} disabled={autoMatchingVoices} className={`w-full ${sidebarTheme.secondaryButton}`}>
            {autoMatchingVoices ? <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" /> : <Mic className="mr-1.5 h-3.5 w-3.5" />}
            自动匹配音色
          </Button>
          {assetStats.paused > 0 ? (
            <Button size="sm" variant="outline" onClick={onResumeGeneration} disabled={resumingGeneration} className={`w-full ${sidebarTheme.secondaryButton}`}>
              {resumingGeneration ? <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" /> : <Play className="mr-1.5 h-3.5 w-3.5" />}
              继续生成 ({assetStats.paused})
            </Button>
          ) : assetStats.generating > 0 ? (
            <Button size="sm" variant="outline" onClick={onPauseGeneration} disabled={pausingGeneration} className={`w-full ${sidebarTheme.secondaryButton}`}>
              {pausingGeneration ? <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" /> : <Pause className="mr-1.5 h-3.5 w-3.5" />}
              暂停生成
            </Button>
          ) : null}
          <Button size="sm" variant="outline" onClick={onRegenerateAssets} className={`w-full ${sidebarTheme.secondaryButton}`}>
            <RotateCcw className="mr-1.5 h-3.5 w-3.5" />
            当前集重新生成
          </Button>
        </div>
      )}

      {activeTab === 'storyboard' && (
        <div className="flex flex-col gap-2">
          <Button size="sm" onClick={onGenerateStoryboards} disabled={storyboardButtonDisabled} className={`w-full ${sidebarTheme.primaryButton}`}>
            {storyboardButtonDisabled ? <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" /> : <Sparkles className="mr-1.5 h-3.5 w-3.5" />}
            生成本集{storyboardImageLabel}
          </Button>
          <Button size="sm" variant="outline" onClick={onAuditStoryboards} className={`w-full ${sidebarTheme.secondaryButton}`}>
            <Bot className="mr-1.5 h-3.5 w-3.5" />
            AI 补全缺失信息
          </Button>
          <Button size="sm" variant="outline" onClick={onRepairStoryboardMetadata} className={`w-full ${sidebarTheme.secondaryButton}`}>
            <RotateCcw className="mr-1.5 h-3.5 w-3.5" />
            修复本集元数据
          </Button>
          {storyboardStats.paused > 0 ? (
            <Button size="sm" variant="outline" onClick={onResumeStoryboards} className={`w-full ${sidebarTheme.secondaryButton}`}>
              <Play className="mr-1.5 h-3.5 w-3.5" />
              继续生成 ({storyboardStats.paused})
            </Button>
          ) : storyboardStats.generating > 0 ? (
            <Button size="sm" variant="outline" onClick={onPauseStoryboards} className={`w-full ${sidebarTheme.secondaryButton}`}>
              <Pause className="mr-1.5 h-3.5 w-3.5" />
              暂停生成
            </Button>
          ) : null}
          <Button size="sm" variant="outline" onClick={onRegenerateStoryboards} className={`w-full ${sidebarTheme.secondaryButton}`}>
            <RotateCcw className="mr-1.5 h-3.5 w-3.5" />
            重新生成本集
          </Button>
          {onExtractStoryboards && (
            <Button
              size="sm"
              variant="outline"
              onClick={() => {
                if (window.confirm('重新拆分镜会删除当前集已有的所有分镜和生成图，并重新拆分结构。确认重新拆分吗？')) {
                  onExtractStoryboards()
                }
              }}
              disabled={isExtractingStoryboards}
              className={`w-full ${sidebarTheme.secondaryButton}`}
            >
              {isExtractingStoryboards ? (
                <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
              ) : (
                <RefreshCw className="mr-1.5 h-3.5 w-3.5" />
              )}
              重新拆分镜
            </Button>
          )}
        </div>
      )}

      {(activeTab === 'dubbing' || activeTab === 'video') && (
        <div className={`rounded-xl border border-dashed px-3 py-4 text-sm ${sidebarTheme.secondaryButton}`}>
          当前阶段的详细操作已放在下方工作台中，进入对应内容区即可继续处理。
        </div>
      )}
    </div>
  )
}
