'use client'

import React, { useMemo, useState } from 'react'
import { storyboardAPI } from '@/lib/api'
import type { Project, Storyboard } from '@/types'
import { useToast } from '@/components/ui/toast'
import { isCommentaryProject as detectCommentaryProject } from '@/lib/projects/commentary-project'
import { getStoryboardTabLabels } from '@/lib/projects/storyboard-labels'
import { TabSkeleton } from '@/components/projects/detail/TabSkeleton'
import { SerialSceneGroups } from '@/components/projects/serial/SerialSceneGroups'
import { StoryboardProgressBanner } from './storyboard/StoryboardProgressBanner'
import { StoryboardToolbar } from './storyboard/StoryboardToolbar'
import { BatchStoryboardDialog } from './storyboard/BatchStoryboardDialog'
import { EpisodeVideoDialog } from './storyboard/EpisodeVideoDialog'
import { StoryboardGrid } from './storyboard/StoryboardGrid'
import { StoryboardPagination } from './storyboard/StoryboardPagination'
import { StoryboardDetailPanel } from './storyboard/StoryboardDetailPanel'
import { useStoryboardTabData } from './storyboard/useStoryboardTabData'
import { useStoryboardActions } from './storyboard/useStoryboardActions'

export function StoryboardTab({
  projectId,
  project,
  episodeId,
  onExtractStoryboards,
  isExtractingStoryboards,
  awaitingAutoStoryboard,
  storyboardButtonDisabled,
  hideActionBar,
  sbGenerateTrigger,
  sbRegenerateTrigger,
  sbPauseTrigger,
  sbResumeTrigger,
  sbAuditTrigger,
  sbRepairMetadataTrigger,
  onSbGenerateTriggerConsumed,
  onSbRegenerateTriggerConsumed,
  onSbPauseTriggerConsumed,
  onSbResumeTriggerConsumed,
  onSbAuditTriggerConsumed,
  onSbRepairMetadataTriggerConsumed,
}: {
  projectId: number
  project: Project
  episodeId?: number
  onExtractStoryboards?: () => void
  isExtractingStoryboards?: boolean
  awaitingAutoStoryboard?: boolean
  storyboardButtonDisabled?: boolean
  hideActionBar?: boolean
  sbGenerateTrigger?: number
  sbRegenerateTrigger?: number
  sbPauseTrigger?: number
  sbResumeTrigger?: number
  sbAuditTrigger?: number
  sbRepairMetadataTrigger?: number
  onSbGenerateTriggerConsumed?: () => void
  onSbRegenerateTriggerConsumed?: () => void
  onSbPauseTriggerConsumed?: () => void
  onSbResumeTriggerConsumed?: () => void
  onSbAuditTriggerConsumed?: () => void
  onSbRepairMetadataTriggerConsumed?: () => void
}) {
  const { toast } = useToast()
  const isSerial = project.project_type === 'video_serial'
  const labels = getStoryboardTabLabels(isSerial)
  const isCommentaryProject = useMemo(() => detectCommentaryProject(project), [project])

  const data = useStoryboardTabData(projectId, project, episodeId)
  const {
    episodeFilter,
    statusFilter,
    setStatusFilter,
    keyword,
    setKeyword,
    sbPage,
    setSbPage,
    vtVideoModelOptions,
    stats,
    mutateStats,
    isActive,
    storyboardAssets,
    assetReadiness,
    episodes,
    storyboards,
    sbTotal,
    sbTotalPages,
    isLoading,
    mutateSb,
    episodeCompletedMap,
    SB_MODEL_OPTIONS,
    storyboardDefaultImageModelLabel,
    storyboardTaskMap,
    mutateStoryboardTasks,
    sbProjectImageModelKey,
  } = data

  const actions = useStoryboardActions({
    projectId,
    project,
    isSerial,
    isCommentaryProject,
    labels,
    episodeFilter,
    episodes,
    SB_MODEL_OPTIONS,
    sbProjectImageModelKey,
    storyboardDefaultImageModelLabel,
    storyboardAssetsReady: assetReadiness.ready,
    storyboardAssetsBlockingReason: assetReadiness.blockingReason,
    vtVideoModelOptions,
    storyboardTaskMap,
    mutateSb,
    mutateStats,
    mutateStoryboardTasks,
    sbGenerateTrigger,
    sbRegenerateTrigger,
    sbPauseTrigger,
    sbResumeTrigger,
    sbAuditTrigger,
    sbRepairMetadataTrigger,
    onSbGenerateTriggerConsumed,
    onSbRegenerateTriggerConsumed,
    onSbPauseTriggerConsumed,
    onSbResumeTriggerConsumed,
    onSbAuditTriggerConsumed,
    onSbRepairMetadataTriggerConsumed,
    toast,
  })

  const [selectedSb, setSelectedSb] = useState<Storyboard | null>(null)
  const [isEditingPrompt, setIsEditingPrompt] = useState(false)
  const [versionIdx, setVersionIdx] = useState(0)
  const [sbDescLang, setSbDescLang] = useState<'zh' | 'en'>('zh')

  const selectedStoryboardVersion = selectedSb?.versions?.[versionIdx]
  const selectedStoryboardPreviewUrl = selectedStoryboardVersion?.image_url || selectedSb?.image_url || ''

  React.useEffect(() => {
    if (!selectedSb) return
    // 用户正在编辑提示词时，不要用轮询拉取的新对象替换 selectedSb，
    // 否则受控输入框（尤其中文输入法）会被周期性重渲染打断，表现为"编辑不了"。
    if (isEditingPrompt) return
    const updated = storyboards.find((sb) => sb.id === selectedSb.id)
    if (updated) setSelectedSb(updated)
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [storyboards, isEditingPrompt])

  React.useEffect(() => {
    if (!selectedSb) return
    const versions = selectedSb.versions ?? []
    if (versions.length === 0) {
      if (versionIdx !== 0) setVersionIdx(0)
      return
    }
    if (versionIdx > versions.length - 1) {
      setVersionIdx(versions.length - 1)
    }
  }, [selectedSb, versionIdx])

  const handleSavePrompt = async (
    updates: Partial<Pick<Storyboard, 'scene_description' | 'prompt_used' | 'prompt_locked' | 'location_zone' | 'spatial_anchor' | 'subject_positions' | 'transition_note'>>,
    options?: { silent?: boolean },
  ) => {
    if (!selectedSb) return
    const prev = selectedSb
    setSelectedSb({ ...selectedSb, ...updates })
    try {
      await storyboardAPI.update(projectId, selectedSb.id, updates as Partial<Storyboard>)
      mutateSb()
      if (!options?.silent) {
        toast({
          title: updates.prompt_locked ? '最终提示词已保存' : '提示词已保存',
          description: '在下方选择模型即可按新提示词重新生成。',
          variant: 'success',
        })
      }
    } catch {
      setSelectedSb(prev)
      if (!options?.silent) {
        toast({ title: '保存失败', variant: 'destructive' })
      }
      throw new Error('save prompt failed')
    }
  }

  if (isLoading) return <TabSkeleton />

  return (
    <div className="relative">
      <StoryboardProgressBanner
        stats={stats}
        isActive={isActive}
        storyboardGenerateLabel={labels.storyboardGenerateLabel}
        onFailedClick={() => setStatusFilter('failed')}
      />

      {project.project_type === 'video_serial' && (
        <div className="mb-4 rounded-xl border border-indigo-200 bg-indigo-50/30 p-4">
          <SerialSceneGroups projectId={projectId} episodeId={episodeId} />
        </div>
      )}

      <StoryboardToolbar
        keyword={keyword}
        onKeywordChange={setKeyword}
        statusFilter={statusFilter}
        onStatusFilterChange={setStatusFilter}
        episodeId={episodeId}
        onExtractStoryboards={onExtractStoryboards}
        isExtractingStoryboards={isExtractingStoryboards}
        awaitingAutoStoryboard={awaitingAutoStoryboard}
        storyboardButtonDisabled={storyboardButtonDisabled}
        extractStoryboardLabel={labels.extractStoryboardLabel}
        storyboardItemLabel={labels.storyboardItemLabel}
        storyboardImageLabel={labels.storyboardImageLabel}
        storyboardVideoLabel={labels.storyboardVideoLabel}
        storyboardGenerateLabel={labels.storyboardGenerateLabel}
        isSerial={isSerial}
        hideActionBar={hideActionBar}
        isAuditingContinuity={actions.isAuditingContinuity}
        onAuditContinuity={actions.handleAuditContinuity}
        pausingGeneration={actions.pausingGeneration}
        onPauseGeneration={actions.handlePauseGeneration}
        resumingGeneration={actions.resumingGeneration}
        onResumeGeneration={actions.handleResumeGeneration}
        stats={stats}
        isActive={isActive}
        storyboardAssetsReady={assetReadiness.ready}
        storyboardAssetsBlockingReason={assetReadiness.blockingReason}
        episodeFilter={episodeFilter}
        onOpenBatchDialog={actions.openBatchStoryboardDialog}
        generatingAllVideos={actions.generatingAllVideos}
        onGenerateAllEpisodeVideos={actions.handleGenerateAllEpisodeVideos}
        vtVideoModelOptions={vtVideoModelOptions}
        videoModelAvailability={actions.videoModelAvailability}
      />

      {!assetReadiness.ready && (
        <div className="mb-4 rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-xs text-amber-800">
          {assetReadiness.blockingReason}
        </div>
      )}

      <BatchStoryboardDialog
        open={actions.showBatchStoryboardDialog}
        onOpenChange={actions.setShowBatchStoryboardDialog}
        batchActionKind={actions.batchStoryboardAction.kind}
        selectedEpisode={actions.selectedStoryboardBatchEpisode}
        storyboardItemLabel={labels.storyboardItemLabel}
        storyboardImageLabel={labels.storyboardImageLabel}
        modelOptions={SB_MODEL_OPTIONS}
        imageModelAvailability={actions.imageModelAvailability}
        selectedModels={actions.batchStoryboardModels}
        onSelectedModelsChange={actions.setBatchStoryboardModels}
        defaultImageModelLabel={storyboardDefaultImageModelLabel}
        running={actions.batchStoryboardRunning}
        onConfirm={actions.executeBatchStoryboardAction}
      />

      <EpisodeVideoDialog
        open={actions.videoDialogEpisodeId !== null}
        episode={actions.selectedVideoDialogEpisode}
        generating={actions.videoDialogEpisodeId !== null && actions.generatingVideoEps.has(actions.videoDialogEpisodeId)}
        videoModelOptions={vtVideoModelOptions}
        videoModelAvailability={actions.videoModelAvailability}
        selectedModel={actions.selectedEpisodeVideoModel}
        onSelectedModelChange={actions.setSelectedEpisodeVideoModel}
        selectedStyle={actions.selectedEpisodeVideoStyle}
        onSelectedStyleChange={actions.setSelectedEpisodeVideoStyle}
        selectedMotionMode={actions.selectedEpisodeVideoMotionMode}
        onSelectedMotionModeChange={actions.setSelectedEpisodeVideoMotionMode}
        selectedFrameSize={actions.selectedEpisodeVideoFrameSize}
        onSelectedFrameSizeChange={actions.setSelectedEpisodeVideoFrameSize}
        selectedSubjectSize={actions.selectedEpisodeVideoSubjectSize}
        onSelectedSubjectSizeChange={actions.setSelectedEpisodeVideoSubjectSize}
        selectedClarity={actions.selectedEpisodeVideoClarity}
        onSelectedClarityChange={actions.setSelectedEpisodeVideoClarity}
        videoModeLabel={actions.selectedEpisodeVideoModeLabel}
        videoModelParams={actions.videoModelParams}
        getModelParam={actions.getModelParam}
        setModelParam={actions.setModelParam}
        selectedTransition={actions.selectedEpisodeTransition}
        onSelectedTransitionChange={actions.setSelectedEpisodeTransition}
        selectedTransitionDuration={actions.selectedEpisodeTransitionDuration}
        onSelectedTransitionDurationChange={actions.setSelectedEpisodeTransitionDuration}
        onApplyPreset={actions.applyEpisodeVideoPreset}
        onConfirm={actions.handleConfirmEpisodeVideoGeneration}
        onClose={() => actions.setVideoDialogEpisodeId(null)}
      />

      <StoryboardGrid
        storyboards={storyboards}
        episodes={episodes}
        storyboardItemLabel={labels.storyboardItemLabel}
        storyboardImageLabel={labels.storyboardImageLabel}
        storyboardVideoLabel={labels.storyboardVideoLabel}
        isSerial={isSerial}
        sbDescLang={sbDescLang}
        modelOptions={SB_MODEL_OPTIONS}
        imageModelAvailability={actions.imageModelAvailability}
        episodeCompletedMap={episodeCompletedMap}
        generatingVideoEps={actions.generatingVideoEps}
        onSelectStoryboard={(sb) => { setSelectedSb(sb); setVersionIdx(0) }}
        onGenerateOne={actions.handleGenerateOne}
        onSwitchVersion={actions.handleSwitchVersion}
        onVoid={(id) => actions.handleVoid(id, selectedSb, setSelectedSb)}
        onDelete={(id) => actions.handleDelete(id, selectedSb, setSelectedSb)}
        onMergeWithPrevious={(current, previous) => actions.handleMergeWithPrevious(current, previous, selectedSb, setSelectedSb)}
        onOpenEpisodeVideoDialog={actions.openEpisodeVideoDialog}
        onCreateFromEpisodes={actions.handleCreateFromEpisodes}
      />

      <StoryboardPagination
        sbPage={sbPage}
        sbTotalPages={sbTotalPages}
        sbTotal={sbTotal}
        onPageChange={setSbPage}
      />

      {selectedSb && (
        <StoryboardDetailPanel
          project={project}
          selectedSb={selectedSb}
          onClose={() => setSelectedSb(null)}
          storyboardItemLabel={labels.storyboardItemLabel}
          storyboardImageLabel={labels.storyboardImageLabel}
          storyboardGenerateLabel={labels.storyboardGenerateLabel}
          selectedStoryboardVersion={selectedStoryboardVersion}
          selectedStoryboardPreviewUrl={selectedStoryboardPreviewUrl}
          versionIdx={versionIdx}
          onVersionIdxChange={setVersionIdx}
          sbDescLang={sbDescLang}
          onSbDescLangChange={setSbDescLang}
          storyboardAssets={storyboardAssets}
          onSavePrompt={handleSavePrompt}
          onEditingPromptChange={setIsEditingPrompt}
          modelOptions={SB_MODEL_OPTIONS}
          imageModelAvailability={actions.imageModelAvailability}
          defaultImageModelLabel={storyboardDefaultImageModelLabel}
          onGenerate={actions.handleGenerateModels}
        />
      )}
    </div>
  )
}
