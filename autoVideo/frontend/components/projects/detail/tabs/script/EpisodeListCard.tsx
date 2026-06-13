'use client'

import { Loader2, Plus, RefreshCw } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import type { Episode, Model, Project, Storyboard } from '@/types'
import type { EpisodeCountRecommendation } from '@/lib/projects/comic'
import type { VideoPipelineSnapshot } from '@/lib/projects/pipeline-status'
import { SplitAdvancedSettingsPanel } from './SplitAdvancedSettingsPanel'
import { ScriptPipelineBanners, EpisodeSplitDoneBanner, BatchFormattingBanner, AssetExtractionBanner } from './ScriptPipelineBanners'
import { KeywordLibraryPanel } from './KeywordLibraryPanel'
import { EpisodeGrid, SceneSplittingEpisodeGrid } from './EpisodeGrid'

type ModelAvailability = { label: string; color: string }

export type EpisodeListCardProps = {
  project: Project
  episodes: Episode[]
  pipeline: VideoPipelineSnapshot
  splitInProgress: boolean
  splitConfigReady: boolean
  effectiveSplitModel: Model | null
  splitModels: Model[]
  hasScriptText: boolean
  projectProgressMessage?: string
  textModelsLoading: boolean
  usesAutoEpisodeSplit: boolean
  splitSettingsDirty: boolean
  selectedSplitModelAvailability: ModelAvailability | null
  selectedSplitModelProvider: string | null
  hasValidTargetEpisodes: boolean
  parsedTargetEpisodes: number
  recommendedEpisodeCount: EpisodeCountRecommendation | null
  draftTargetEpisodes: string
  showSplitAdvancedSettings: boolean
  savingSplitModel: boolean
  isProcessing: boolean
  shouldShowSplitSearch: boolean
  splitModelSearch: string
  draftSplitModelId: string
  filteredSplitModels: Model[]
  splitModelCapabilities: string[]
  selectedSplitModelRemark: string
  textModelHealthMap: Record<string, 'healthy' | 'unhealthy' | 'unknown'>
  splitProgressSummary: string
  splitProgressPercent: number
  sceneReadyCount: number
  sceneProcessingSummary: string
  sceneProcessingProgress: number
  scriptTabStoryboards: Storyboard[]
  extractionInProgress: boolean
  assetGenerating: boolean
  extractionDone: boolean
  extractTotal: number
  episodeStoryboardDispatching: number | null
  deletingEpisodeId: number | null
  extractingEpisodeAssets: number | null
  generatingEpisodeAssets: number | null
  autoOptimizingEpisode: number | null
  optimizingEpisode: number | null
  reviewingEpisode: number | null
  applyingOptimized: number | null
  onOpenCreateEpisode: () => void
  onOpenRegenerate: () => void
  onDraftTargetEpisodesChange: (value: string) => void
  onMarkSplitSettingsDirty: () => void
  onToggleAdvancedSettings: () => void
  onSplitModelSearchChange: (value: string) => void
  onSplitModelChange: (value: string) => void
  onRetryStalledScript: () => void
  onStartAssetExtraction: () => void
  onSelectEpisode: (episode: Episode) => void
  onStartEpisodeStoryboard: (episodeId: number) => void
  onDeleteEpisode: (episode: Episode) => void
  onAutoStartEpisodeAssets: (episodeId: number, episodeNum: number) => void
  onExtractEpisodeAssets: (episodeId: number, episodeNum: number) => void
  onGenerateEpisodeAssetsFromScript: (episodeId: number, episodeNum: number) => void
  onAutoOptimizeReview: (episode: Episode) => void
  onOptimizeEpisode: (episode: Episode) => void
  onApplyOptimizedText: (episode: Episode) => void
  onReviewEpisode: (episode: Episode) => void
}

export function EpisodeListCard(props: EpisodeListCardProps) {
  const {
    project,
    episodes,
    pipeline,
    splitInProgress,
    splitConfigReady,
    effectiveSplitModel,
    splitModels,
    hasScriptText,
    onOpenCreateEpisode,
    onOpenRegenerate,
  } = props

  const showSceneSplittingGrid = pipeline.phase === 'scene_splitting'

  return (
    <Card>
      <CardHeader className="pb-3">
        <div className="flex items-center justify-between">
          <CardTitle className="text-base">分集列表</CardTitle>
          <div className="flex items-center gap-2">
            <Button
              size="sm"
              variant="outline"
              onClick={onOpenCreateEpisode}
              title="手动补充一集，可直接用于资源提取、分镜和视频流程"
            >
              <Plus className="mr-1.5 h-3.5 w-3.5" />
              手动创建分集
            </Button>
            <Button
              size="sm"
              variant={episodes.length > 0 ? 'outline' : 'default'}
              onClick={onOpenRegenerate}
              disabled={splitInProgress || !splitConfigReady || (!project.script_file_url && !hasScriptText)}
              title={splitInProgress
                ? (project.progress?.message || '当前已有分集任务进行中，请等待当前任务完成')
                : !splitConfigReady
                  ? (!effectiveSplitModel ? '请先在分集高级设置中选择分集模型' : '分集配置未就绪')
                  : (episodes.length > 0 ? '按当前配置重新分集：会替换旧分集，并清空旧分镜后重建' : '按当前手动配置开始自动分集')}
            >
              {splitInProgress ? (
                <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
              ) : (
                <RefreshCw className="mr-1.5 h-3.5 w-3.5" />
              )}
              {splitInProgress ? '分集进行中' : (episodes.length > 0 ? 'AI 重新分集' : 'AI 开始分集')}
            </Button>
          </div>
        </div>
      </CardHeader>
      <CardContent>
        <SplitAdvancedSettingsPanel
          textModelsLoading={props.textModelsLoading}
          splitModels={splitModels}
          usesAutoEpisodeSplit={props.usesAutoEpisodeSplit}
          splitSettingsDirty={props.splitSettingsDirty}
          selectedSplitModelAvailability={props.selectedSplitModelAvailability}
          effectiveSplitModel={effectiveSplitModel}
          selectedSplitModelProvider={props.selectedSplitModelProvider}
          hasValidTargetEpisodes={props.hasValidTargetEpisodes}
          parsedTargetEpisodes={props.parsedTargetEpisodes}
          recommendedEpisodeCount={props.recommendedEpisodeCount}
          draftTargetEpisodes={props.draftTargetEpisodes}
          onDraftTargetEpisodesChange={props.onDraftTargetEpisodesChange}
          onMarkSplitSettingsDirty={props.onMarkSplitSettingsDirty}
          showSplitAdvancedSettings={props.showSplitAdvancedSettings}
          onToggleAdvancedSettings={props.onToggleAdvancedSettings}
          savingSplitModel={props.savingSplitModel}
          isProcessing={props.isProcessing}
          shouldShowSplitSearch={props.shouldShowSplitSearch}
          splitModelSearch={props.splitModelSearch}
          onSplitModelSearchChange={props.onSplitModelSearchChange}
          draftSplitModelId={props.draftSplitModelId}
          onSplitModelChange={props.onSplitModelChange}
          filteredSplitModels={props.filteredSplitModels}
          splitModelCapabilities={props.splitModelCapabilities}
          selectedSplitModelRemark={props.selectedSplitModelRemark}
          textModelHealthMap={props.textModelHealthMap}
        />

        {['episode_splitting', 'script_prepping'].includes(pipeline.phase) ? (
          <ScriptPipelineBanners
            pipeline={pipeline}
            project={project}
            episodes={episodes}
            splitProgressSummary={props.splitProgressSummary}
            splitProgressPercent={props.splitProgressPercent}
            sceneReadyCount={props.sceneReadyCount}
            sceneProcessingSummary={props.sceneProcessingSummary}
            sceneProcessingProgress={props.sceneProcessingProgress}
            onRetryStalledScript={props.onRetryStalledScript}
          />
        ) : showSceneSplittingGrid ? (
          <div className="space-y-4">
            <ScriptPipelineBanners
              pipeline={pipeline}
              project={project}
              episodes={episodes}
              splitProgressSummary={props.splitProgressSummary}
              splitProgressPercent={props.splitProgressPercent}
              sceneReadyCount={props.sceneReadyCount}
              sceneProcessingSummary={props.sceneProcessingSummary}
              sceneProcessingProgress={props.sceneProcessingProgress}
              onRetryStalledScript={props.onRetryStalledScript}
            />
            <SceneSplittingEpisodeGrid
              episodes={episodes}
              scriptTabStoryboards={props.scriptTabStoryboards}
              episodeStoryboardDispatching={props.episodeStoryboardDispatching}
              deletingEpisodeId={props.deletingEpisodeId}
              onSelectEpisode={props.onSelectEpisode}
              onStartEpisodeStoryboard={props.onStartEpisodeStoryboard}
              onDeleteEpisode={props.onDeleteEpisode}
            />
          </div>
        ) : episodes.length === 0 ? (
          <div className="py-6 text-center text-sm text-surface-400">
            暂无分集，点击右上角「手动创建分集」添加，或上传剧本后 AI 自动分集。
          </div>
        ) : (
          <>
            {pipeline.episodeSplitDone && !pipeline.isActive ? (
              <EpisodeSplitDoneBanner episodesCount={episodes.length} nextStepHint={pipeline.nextStepHint} />
            ) : null}

            <BatchFormattingBanner episodes={episodes} />

            <AssetExtractionBanner
              projectStatus={project.status}
              extractionInProgress={props.extractionInProgress}
              assetGenerating={props.assetGenerating}
              extractionDone={props.extractionDone}
              extractTotal={props.extractTotal}
              onStartAssetExtraction={props.onStartAssetExtraction}
            />

            {project.keyword_library ? (
              <KeywordLibraryPanel keywordLibrary={project.keyword_library} />
            ) : null}

            <EpisodeGrid
              episodes={episodes}
              extractingEpisodeAssets={props.extractingEpisodeAssets}
              generatingEpisodeAssets={props.generatingEpisodeAssets}
              autoOptimizingEpisode={props.autoOptimizingEpisode}
              optimizingEpisode={props.optimizingEpisode}
              reviewingEpisode={props.reviewingEpisode}
              applyingOptimized={props.applyingOptimized}
              onSelectEpisode={props.onSelectEpisode}
              onAutoStartEpisodeAssets={props.onAutoStartEpisodeAssets}
              onExtractEpisodeAssets={props.onExtractEpisodeAssets}
              onGenerateEpisodeAssetsFromScript={props.onGenerateEpisodeAssetsFromScript}
              onAutoOptimizeReview={props.onAutoOptimizeReview}
              onOptimizeEpisode={props.onOptimizeEpisode}
              onApplyOptimizedText={props.onApplyOptimizedText}
              onReviewEpisode={props.onReviewEpisode}
            />
          </>
        )}
      </CardContent>
    </Card>
  )
}
