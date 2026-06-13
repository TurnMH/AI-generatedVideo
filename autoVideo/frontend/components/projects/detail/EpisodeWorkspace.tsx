'use client'

import { Tabs } from '@/components/ui/tabs'
import type { Episode, Project } from '@/types'
import { useEpisodeWorkspaceData, type EpisodeWorkspaceTab } from '@/lib/projects/use-episode-workspace-data'
import { contentShellTheme, sidebarTheme, workflowStepTheme } from '@/components/projects/detail/episode-workspace/themes'
import { EpisodeWorkflowTabList } from './episode-workspace/EpisodeWorkflowTabList'
import { EpisodeWorkspaceHeaderPanel } from './episode-workspace/EpisodeWorkspaceHeaderPanel'
import { EpisodeStageActionPanel } from './episode-workspace/EpisodeStageActionPanel'
import { EpisodeWorkspaceTabPanels } from './episode-workspace/EpisodeWorkspaceTabPanels'

interface EpisodeWorkspaceProps {
  projectId: number
  episodeId: number
  episode?: Episode
  project: Project
  initialTab?: EpisodeWorkspaceTab
  initialAwaitingAutoStoryboard?: boolean
  autoPipelineActive?: boolean
}

export function EpisodeWorkspace({
  projectId,
  episodeId,
  episode,
  project,
  initialTab = 'assets',
  initialAwaitingAutoStoryboard = false,
  autoPipelineActive = false,
}: EpisodeWorkspaceProps) {
  const ws = useEpisodeWorkspaceData({
    projectId,
    episodeId,
    episode,
    project,
    initialTab,
    initialAwaitingAutoStoryboard,
    autoPipelineActive,
  })

  const activeSidebarTheme = sidebarTheme[ws.activeTab]
  const activeContentShellTheme = contentShellTheme[ws.activeTab]
  const activeWorkflowTheme = workflowStepTheme[ws.activeTab]

  return (
    <div className="space-y-6">
      <Tabs value={ws.activeTab} onValueChange={(value) => ws.setActiveTab(value as EpisodeWorkspaceTab)} className="w-full">
        <EpisodeWorkflowTabList steps={ws.workflowSteps} />

        <div className={`relative mt-6 overflow-hidden rounded-3xl border p-5 shadow-sm ${activeContentShellTheme.frame}`}>
          <span className={`absolute inset-x-0 top-0 h-1 ${activeContentShellTheme.strip}`} />
          <div className="grid gap-5 xl:grid-cols-[minmax(0,1.5fr)_360px]">
            <EpisodeWorkspaceHeaderPanel
              episode={episode}
              episodeId={episodeId}
              episodeSummary={ws.episodeSummary}
              updatedAtLabel={ws.updatedAtLabel}
              pipelineStatus={ws.pipelineStatus}
              sidebarTheme={activeSidebarTheme}
              contentShellTheme={activeContentShellTheme}
            />
            <EpisodeStageActionPanel
              activeTab={ws.activeTab}
              workflowSteps={ws.workflowSteps}
              storyboardImageLabel={ws.labels.storyboardImageLabel}
              assetStats={ws.assetStats}
              storyboardStats={ws.storyboardStats}
              sidebarTheme={activeSidebarTheme}
              workflowTheme={activeWorkflowTheme}
              resourceButtonDisabled={ws.resourceButtonDisabled}
              storyboardButtonDisabled={ws.storyboardButtonDisabled}
              autoMatchingVoices={ws.autoMatchingVoices}
              pausingGeneration={ws.pausingGeneration}
              resumingGeneration={ws.resumingGeneration}
              onGenerateAll={() => ws.setGenerateTrigger((t) => t + 1)}
              onAutoMatchVoices={ws.handleAutoMatchVoices}
              onPauseGeneration={ws.handlePauseGeneration}
              onResumeGeneration={ws.handleResumeGeneration}
              onRegenerateAssets={() => ws.setRegenerateTrigger((t) => t + 1)}
              onGenerateStoryboards={() => ws.setSbGenerateTrigger((t) => t + 1)}
              onAuditStoryboards={() => ws.setSbAuditTrigger((t) => t + 1)}
              onPauseStoryboards={() => ws.setSbPauseTrigger((t) => t + 1)}
              onResumeStoryboards={() => ws.setSbResumeTrigger((t) => t + 1)}
              onRegenerateStoryboards={() => ws.setSbRegenerateTrigger((t) => t + 1)}
            />
          </div>
        </div>

        <EpisodeWorkspaceTabPanels
          projectId={projectId}
          project={project}
          episodeId={episodeId}
          episode={episode}
          resourceButtonDisabled={ws.resourceButtonDisabled}
          isExtractingStoryboards={ws.isExtractingStoryboards}
          awaitingAutoStoryboard={ws.awaitingAutoStoryboard}
          storyboardButtonDisabled={ws.storyboardButtonDisabled}
          generateTrigger={ws.generateTrigger}
          regenerateTrigger={ws.regenerateTrigger}
          sbGenerateTrigger={ws.sbGenerateTrigger}
          sbRegenerateTrigger={ws.sbRegenerateTrigger}
          sbPauseTrigger={ws.sbPauseTrigger}
          sbResumeTrigger={ws.sbResumeTrigger}
          sbAuditTrigger={ws.sbAuditTrigger}
          onExtractAssets={ws.handleExtractAssets}
          onExtractStoryboards={ws.handleExtractStoryboards}
        />
      </Tabs>
    </div>
  )
}
