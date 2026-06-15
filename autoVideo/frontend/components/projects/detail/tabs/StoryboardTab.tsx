'use client'

import React, { useMemo, useRef, useState } from 'react'
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
  onSbGenerateTriggerConsumed,
  onSbRegenerateTriggerConsumed,
  onSbPauseTriggerConsumed,
  onSbResumeTriggerConsumed,
  onSbAuditTriggerConsumed,
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
  onSbGenerateTriggerConsumed?: () => void
  onSbRegenerateTriggerConsumed?: () => void
  onSbPauseTriggerConsumed?: () => void
  onSbResumeTriggerConsumed?: () => void
  onSbAuditTriggerConsumed?: () => void
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
    onSbGenerateTriggerConsumed,
    onSbRegenerateTriggerConsumed,
    onSbPauseTriggerConsumed,
    onSbResumeTriggerConsumed,
    onSbAuditTriggerConsumed,
    toast,
  })

  const [selectedSb, setSelectedSb] = useState<Storyboard | null>(null)
  const [versionIdx, setVersionIdx] = useState(0)
  const [chatInput, setChatInput] = useState('')
  const [chatLoading, setChatLoading] = useState(false)
  const [sbDescLang, setSbDescLang] = useState<'zh' | 'en'>('zh')
  const chatListRef = useRef<HTMLDivElement>(null)
  const chatBottomRef = useRef<HTMLDivElement>(null)
  const shouldStickChatToBottomRef = useRef(true)

  const selectedStoryboardVersion = selectedSb?.versions?.[versionIdx]
  const selectedStoryboardPreviewUrl = selectedStoryboardVersion?.image_url || selectedSb?.image_url || ''
  const selectedStoryboardMessageCount = selectedSb?.agent_history?.length ?? 0

  React.useEffect(() => {
    if (!selectedSb) return
    const updated = storyboards.find((sb) => sb.id === selectedSb.id)
    if (updated) setSelectedSb(updated)
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [storyboards])

  React.useEffect(() => {
    shouldStickChatToBottomRef.current = true
  }, [selectedSb?.id])

  React.useEffect(() => {
    if (!selectedSb || !shouldStickChatToBottomRef.current) return
    chatBottomRef.current?.scrollIntoView({ block: 'end' })
  }, [
    selectedSb?.id,
    selectedSb?.status,
    selectedSb?.agent_history?.length,
    selectedStoryboardPreviewUrl,
    chatLoading,
  ])

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

  const handleChatListScroll = () => {
    const el = chatListRef.current
    if (!el) return
    const distanceToBottom = el.scrollHeight - el.scrollTop - el.clientHeight
    shouldStickChatToBottomRef.current = distanceToBottom <= 96
  }

  const handleChat = async () => {
    if (!selectedSb || !chatInput.trim()) return
    const message = chatInput.trim()
    const previousStoryboard = selectedSb
    const optimisticMessage = {
      role: 'user' as const,
      content: message,
      timestamp: new Date().toISOString(),
    }
    setSelectedSb({
      ...selectedSb,
      agent_history: [...(selectedSb.agent_history ?? []), optimisticMessage],
    })
    setChatInput('')
    setChatLoading(true)
    try {
      const res = await storyboardAPI.chat(projectId, selectedSb.id, message) as unknown as { data: Storyboard }
      if (res.data) setSelectedSb(res.data)
      mutateSb()
    } catch {
      setSelectedSb(previousStoryboard)
      setChatInput(message)
      toast({ title: '发送失败', variant: 'destructive' })
    } finally {
      setChatLoading(false)
    }
  }

  const handleCameraMovementChange = async (val: string) => {
    if (!selectedSb) return
    const prev = selectedSb
    setSelectedSb({ ...selectedSb, camera_movement: val })
    try {
      await storyboardAPI.update(projectId, selectedSb.id, { camera_movement: val } as Partial<Storyboard>)
      mutateSb()
    } catch {
      setSelectedSb(prev)
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
          selectedStoryboardMessageCount={selectedStoryboardMessageCount}
          versionIdx={versionIdx}
          onVersionIdxChange={setVersionIdx}
          sbDescLang={sbDescLang}
          onSbDescLangChange={setSbDescLang}
          storyboardAssets={storyboardAssets}
          onCameraMovementChange={handleCameraMovementChange}
          sbVoiceScope={actions.sbVoiceScope}
          onSbVoiceScopeChange={actions.setSbVoiceScope}
          sbVoiceModel={actions.sbVoiceModel}
          onSbVoiceModelChange={actions.setSbVoiceModel}
          sbVoiceRate={actions.sbVoiceRate}
          onSbVoiceRateChange={actions.setSbVoiceRate}
          sbVoicePitch={actions.sbVoicePitch}
          onSbVoicePitchChange={actions.setSbVoicePitch}
          sbVoiceVolume={actions.sbVoiceVolume}
          onSbVoiceVolumeChange={actions.setSbVoiceVolume}
          sbVoiceOptions={actions.SB_VOICE_OPTIONS}
          generatingSbVoice={actions.generatingSbVoice}
          onGenerateVoice={() => actions.handleSbGenerateVoice(selectedSb)}
          storyboardTaskMap={storyboardTaskMap}
          modelOptions={SB_MODEL_OPTIONS}
          imageModelAvailability={actions.imageModelAvailability}
          onGenerateOne={actions.handleGenerateOne}
          chatListRef={chatListRef}
          onChatListScroll={handleChatListScroll}
          chatBottomRef={chatBottomRef}
          chatInput={chatInput}
          onChatInputChange={setChatInput}
          chatLoading={chatLoading}
          onChat={handleChat}
        />
      )}
    </div>
  )
}
