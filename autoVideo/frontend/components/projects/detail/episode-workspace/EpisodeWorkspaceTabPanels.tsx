'use client'

import { TabsContent } from '@/components/ui/tabs'
import type { Episode, Project } from '@/types'
import { AssetsTab } from '@/components/projects/detail/tabs/AssetsTab'
import { StoryboardTab } from '@/components/projects/detail/tabs/StoryboardTab'
import { DubbingTab } from '@/components/projects/detail/tabs/DubbingTab'
import { VideoTab } from '@/components/projects/detail/tabs/VideoTab'
import { contentShellTheme } from '@/components/projects/detail/episode-workspace/themes'

export function EpisodeWorkspaceTabPanels({
  projectId,
  project,
  episodeId,
  episode,
  resourceButtonDisabled,
  isExtractingStoryboards,
  awaitingAutoStoryboard,
  storyboardButtonDisabled,
  generateTrigger,
  regenerateTrigger,
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
  onGenerateTriggerConsumed,
  onRegenerateTriggerConsumed,
  onExtractAssets,
  onExtractStoryboards,
}: {
  projectId: number
  project: Project
  episodeId: number
  episode?: Episode
  resourceButtonDisabled: boolean
  isExtractingStoryboards: boolean
  awaitingAutoStoryboard: boolean
  storyboardButtonDisabled: boolean
  generateTrigger: number
  regenerateTrigger: number
  sbGenerateTrigger: number
  sbRegenerateTrigger: number
  sbPauseTrigger: number
  sbResumeTrigger: number
  sbAuditTrigger: number
  sbRepairMetadataTrigger: number
  onSbGenerateTriggerConsumed: () => void
  onSbRegenerateTriggerConsumed: () => void
  onSbPauseTriggerConsumed: () => void
  onSbResumeTriggerConsumed: () => void
  onSbAuditTriggerConsumed: () => void
  onSbRepairMetadataTriggerConsumed: () => void
  onGenerateTriggerConsumed: () => void
  onRegenerateTriggerConsumed: () => void
  onExtractAssets: () => void
  onExtractStoryboards: () => void
}) {
  return (
    <div className="mt-6">
      <TabsContent value="assets" className="mt-0">
        <div className={`rounded-3xl border p-3 ${contentShellTheme.assets.contentWrap}`}>
          <AssetsTab
            projectId={projectId}
            project={project}
            episodeId={episodeId}
            onExtractEpisodeAssets={onExtractAssets}
            isExtractingEpisodeAssets={resourceButtonDisabled}
            generateTrigger={generateTrigger}
            regenerateTrigger={regenerateTrigger}
            onGenerateTriggerConsumed={onGenerateTriggerConsumed}
            onRegenerateTriggerConsumed={onRegenerateTriggerConsumed}
            hideActionBar
          />
        </div>
      </TabsContent>
      <TabsContent value="storyboard" className="mt-0">
        <div className={`rounded-3xl border p-3 ${contentShellTheme.storyboard.contentWrap}`}>
          <StoryboardTab
            projectId={projectId}
            project={project}
            episodeId={episodeId}
            onExtractStoryboards={onExtractStoryboards}
            isExtractingStoryboards={isExtractingStoryboards || episode?.status === 'scene_splitting'}
            awaitingAutoStoryboard={awaitingAutoStoryboard}
            storyboardButtonDisabled={storyboardButtonDisabled}
            hideActionBar
            sbGenerateTrigger={sbGenerateTrigger}
            sbRegenerateTrigger={sbRegenerateTrigger}
            sbPauseTrigger={sbPauseTrigger}
            sbResumeTrigger={sbResumeTrigger}
            sbAuditTrigger={sbAuditTrigger}
            sbRepairMetadataTrigger={sbRepairMetadataTrigger}
            onSbGenerateTriggerConsumed={onSbGenerateTriggerConsumed}
            onSbRegenerateTriggerConsumed={onSbRegenerateTriggerConsumed}
            onSbPauseTriggerConsumed={onSbPauseTriggerConsumed}
            onSbResumeTriggerConsumed={onSbResumeTriggerConsumed}
            onSbAuditTriggerConsumed={onSbAuditTriggerConsumed}
            onSbRepairMetadataTriggerConsumed={onSbRepairMetadataTriggerConsumed}
          />
        </div>
      </TabsContent>
      <TabsContent value="dubbing" className="mt-0">
        <div className={`rounded-3xl border p-3 ${contentShellTheme.dubbing.contentWrap}`}>
          <DubbingTab projectId={projectId} project={project} mutateProject={() => {}} episodeId={episodeId} />
        </div>
      </TabsContent>
      <TabsContent value="video" className="mt-0">
        <div className={`rounded-3xl border p-3 ${contentShellTheme.video.contentWrap}`}>
          <VideoTab projectId={projectId} project={project} episodeId={episodeId} />
        </div>
      </TabsContent>
    </div>
  )
}
