'use client'

import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { ProductionSkillsPanel } from '@/components/skills/ProductionSkillsPanel'
import type { Project } from '@/types'
import { TabSkeleton } from '@/components/projects/detail/TabSkeleton'
import { useScriptTab } from './script/use-script-tab'
import { ScriptFileCard } from './script/ScriptFileCard'
import { EpisodeListCard } from './script/EpisodeListCard'
import { RegenerateDialog } from './script/RegenerateDialog'
import { CreateEpisodeDialog } from './script/CreateEpisodeDialog'
import { ScriptPreviewDialog } from './script/ScriptPreviewDialog'
import { EpisodeDetailDialog } from './script/EpisodeDetailDialog'
import { DeleteEpisodeAlert } from './script/DeleteEpisodeAlert'

export function ScriptTab({
  projectId,
  project,
  mutateProject,
  onAutoStoryboardQueued,
}: {
  projectId: number
  project: Project
  mutateProject: () => void
  onAutoStoryboardQueued?: () => void
}) {
  const tab = useScriptTab({ projectId, project, mutateProject, onAutoStoryboardQueued })

  if (tab.episodesLoading) return <TabSkeleton />

  const scriptFileName = project.script_file_url?.split('/').pop() || '剧本文件'

  return (
    <div className="space-y-6">
      <ScriptFileCard
        project={project}
        fileRef={tab.fileRef}
        hasScriptText={tab.hasScriptText}
        onUpload={tab.handleUpload}
        onShowPreview={() => tab.setShowScriptPreviewDialog(true)}
        onTriggerUpload={() => tab.fileRef.current?.click()}
      />

      <EpisodeListCard
        project={project}
        episodes={tab.episodes}
        pipeline={tab.pipeline}
        splitInProgress={tab.splitInProgress}
        splitConfigReady={tab.splitConfigReady}
        effectiveSplitModel={tab.effectiveSplitModel}
        splitModels={tab.splitModels}
        hasScriptText={tab.hasScriptText}
        textModelsLoading={tab.textModelsLoading}
        usesAutoEpisodeSplit={tab.usesAutoEpisodeSplit}
        splitSettingsDirty={tab.splitSettingsDirty}
        selectedSplitModelAvailability={tab.selectedSplitModelAvailability}
        selectedSplitModelProvider={tab.selectedSplitModelProvider}
        hasValidTargetEpisodes={tab.hasValidTargetEpisodes}
        parsedTargetEpisodes={tab.parsedTargetEpisodes}
        recommendedEpisodeCount={tab.recommendedEpisodeCount}
        draftTargetEpisodes={tab.draftTargetEpisodes}
        showSplitAdvancedSettings={tab.showSplitAdvancedSettings}
        savingSplitModel={tab.savingSplitModel}
        isProcessing={tab.isProcessing}
        shouldShowSplitSearch={tab.shouldShowSplitSearch}
        splitModelSearch={tab.splitModelSearch}
        draftSplitModelId={tab.draftSplitModelId}
        filteredSplitModels={tab.filteredSplitModels}
        splitModelCapabilities={tab.splitModelCapabilities}
        selectedSplitModelRemark={tab.selectedSplitModelRemark}
        textModelHealthMap={tab.textModelHealthMap}
        splitProgressSummary={tab.splitProgressSummary}
        splitProgressPercent={tab.splitProgressPercent}
        sceneReadyCount={tab.sceneReadyCount}
        sceneProcessingSummary={tab.sceneProcessingSummary}
        sceneProcessingProgress={tab.sceneProcessingProgress}
        scriptTabStoryboards={tab.scriptTabStoryboards}
        extractionInProgress={tab.extractionInProgress}
        assetGenerating={tab.assetGenerating}
        extractionDone={tab.extractionDone}
        extractTotal={tab.extractTotal}
        episodeStoryboardDispatching={tab.episodeStoryboardDispatching}
        deletingEpisodeId={tab.deletingEpisodeId}
        extractingEpisodeAssets={tab.extractingEpisodeAssets}
        generatingEpisodeAssets={tab.generatingEpisodeAssets}
        autoOptimizingEpisode={tab.autoOptimizingEpisode}
        optimizingEpisode={tab.optimizingEpisode}
        reviewingEpisode={tab.reviewingEpisode}
        applyingOptimized={tab.applyingOptimized}
        onOpenCreateEpisode={tab.handleOpenCreateEpisode}
        onOpenRegenerate={tab.handleOpenRegenerate}
        onDraftTargetEpisodesChange={tab.setDraftTargetEpisodes}
        onMarkSplitSettingsDirty={() => tab.setSplitSettingsDirty(true)}
        onToggleAdvancedSettings={() => tab.setShowSplitAdvancedSettings((value) => !value)}
        onSplitModelSearchChange={tab.setSplitModelSearch}
        onSplitModelChange={tab.handleSplitModelChange}
        onRetryStalledScript={tab.handleRetryStalledScript}
        onStartAssetExtraction={tab.handleStartAssetExtraction}
        onSelectEpisode={tab.setSelectedEpisode}
        onStartEpisodeStoryboard={tab.handleStartEpisodeStoryboard}
        onDeleteEpisode={tab.setEpisodeDeleteTarget}
        onAutoStartEpisodeAssets={tab.handleAutoStartEpisodeAssets}
        onExtractEpisodeAssets={tab.handleExtractEpisodeAssets}
        onGenerateEpisodeAssetsFromScript={tab.handleGenerateEpisodeAssetsFromScript}
        onAutoOptimizeReview={tab.handleAutoOptimizeReview}
        onOptimizeEpisode={tab.handleOptimizeEpisode}
        onApplyOptimizedText={tab.handleApplyOptimizedText}
        onReviewEpisode={tab.handleReviewEpisode}
      />

      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-base flex items-center gap-2">
            <span>🎬</span> 影视部门标注技能
          </CardTitle>
        </CardHeader>
        <CardContent>
          <ProductionSkillsPanel projectId={projectId} />
        </CardContent>
      </Card>

      <RegenerateDialog
        open={tab.showRegenerateDialog}
        onOpenChange={tab.setShowRegenerateDialog}
        episodes={tab.episodes}
        usesAutoEpisodeSplit={tab.usesAutoEpisodeSplit}
        effectiveSplitModel={tab.effectiveSplitModel}
        selectedSplitModelAvailability={tab.selectedSplitModelAvailability}
        splitModelCapabilities={tab.splitModelCapabilities}
        selectedSplitModelRemark={tab.selectedSplitModelRemark}
        recommendedEpisodeCount={tab.recommendedEpisodeCount}
        hasValidTargetEpisodes={tab.hasValidTargetEpisodes}
        parsedTargetEpisodes={tab.parsedTargetEpisodes}
        kwSplitKeywords={tab.kwSplitKeywords}
        onKwSplitKeywordsChange={tab.setKwSplitKeywords}
        kwCharacters={tab.kwCharacters}
        onKwCharactersChange={tab.setKwCharacters}
        kwLocations={tab.kwLocations}
        onKwLocationsChange={tab.setKwLocations}
        kwEvents={tab.kwEvents}
        onKwEventsChange={tab.setKwEvents}
        kwProps={tab.kwProps}
        onKwPropsChange={tab.setKwProps}
        autoStoryboardAfterSplit={tab.autoStoryboardAfterSplit}
        onAutoStoryboardAfterSplitChange={tab.setAutoStoryboardAfterSplit}
        onConfirm={tab.handleGenerateEpisodes}
      />

      <CreateEpisodeDialog
        open={tab.showCreateEpisodeDialog}
        onOpenChange={tab.setShowCreateEpisodeDialog}
        nextManualEpisodeNumber={tab.nextManualEpisodeNumber}
        manualEpisodeNumber={tab.manualEpisodeNumber}
        onManualEpisodeNumberChange={tab.setManualEpisodeNumber}
        manualEpisodeNumberTaken={tab.manualEpisodeNumberTaken}
        parsedManualEpisodeNumber={tab.parsedManualEpisodeNumber}
        manualEpisodeTitle={tab.manualEpisodeTitle}
        onManualEpisodeTitleChange={tab.setManualEpisodeTitle}
        manualEpisodeSummary={tab.manualEpisodeSummary}
        onManualEpisodeSummaryChange={tab.setManualEpisodeSummary}
        manualEpisodeContent={tab.manualEpisodeContent}
        onManualEpisodeContentChange={tab.setManualEpisodeContent}
        creatingEpisode={tab.creatingEpisode}
        onCreate={tab.handleCreateEpisode}
      />

      <ScriptPreviewDialog
        open={tab.showScriptPreviewDialog}
        onOpenChange={tab.setShowScriptPreviewDialog}
        scriptFileName={scriptFileName}
        scriptText={tab.scriptText}
      />

      <EpisodeDetailDialog
        projectId={projectId}
        selectedEpisode={tab.selectedEpisode}
        editingEpisode={tab.editingEpisode}
        editEpisodeTitle={tab.editEpisodeTitle}
        onEditEpisodeTitleChange={tab.setEditEpisodeTitle}
        editEpisodeSummary={tab.editEpisodeSummary}
        onEditEpisodeSummaryChange={tab.setEditEpisodeSummary}
        editEpisodeContent={tab.editEpisodeContent}
        onEditEpisodeContentChange={tab.setEditEpisodeContent}
        polishingEpisode={tab.polishingEpisode}
        savingEpisodeEdit={tab.savingEpisodeEdit}
        autoOptimizingEpisode={tab.autoOptimizingEpisode}
        optimizingEpisode={tab.optimizingEpisode}
        reviewingEpisode={tab.reviewingEpisode}
        applyingOptimized={tab.applyingOptimized}
        onOpenChange={(open) => {
          if (!open) {
            tab.setSelectedEpisode(null)
            tab.setEditingEpisode(false)
          }
        }}
        onPolishEpisode={tab.handlePolishEpisode}
        onOpenEditEpisode={tab.handleOpenEditEpisode}
        onCancelEdit={() => tab.setEditingEpisode(false)}
        onSaveEpisodeEdit={tab.handleSaveEpisodeEdit}
        onAutoOptimizeReview={tab.handleAutoOptimizeReview}
        onOptimizeEpisode={tab.handleOptimizeEpisode}
        onApplyOptimizedText={tab.handleApplyOptimizedText}
        onReviewEpisode={tab.handleReviewEpisode}
      />

      <DeleteEpisodeAlert
        episodeDeleteTarget={tab.episodeDeleteTarget}
        deletingEpisodeId={tab.deletingEpisodeId}
        onOpenChange={(open) => { if (!open) tab.setEpisodeDeleteTarget(null) }}
        onConfirm={tab.handleDeleteEpisode}
      />
    </div>
  )
}
