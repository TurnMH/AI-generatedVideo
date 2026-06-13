'use client'

import { AlertCircle, CheckCircle2, Clock3, Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { episodeSplitOverviewBadge } from '@/lib/projects/episode-split'
import type { VideoPipelineStageView } from '@/lib/projects/pipeline-status'
import type { ProjectOverviewAction, ProjectOverviewNotice, ProjectOverviewStepView } from '@/lib/projects/workflow'
import type { Project } from '@/types'
import type { VideoPipelineSnapshot } from '@/lib/projects/pipeline-status'

type ProjectOverviewPanelProps = {
  project: Project
  episodeCount: number
  pipelineSnapshot: VideoPipelineSnapshot
  projectControlStages: VideoPipelineStageView[]
  projectControlDoneCount: number
  projectControlOverallProgress: number
  projectControlCurrentStage?: VideoPipelineStageView
  workflowSteps: ProjectOverviewStepView[]
  notices: ProjectOverviewNotice[]
  nextAction: ProjectOverviewAction
  onNextAction: () => void
}

const stepStatusClass: Record<ProjectOverviewStepView['status'], string> = {
  done: 'border-emerald-200 bg-emerald-50 text-emerald-700',
  current: 'border-primary-200 bg-primary-50 text-primary-700',
  pending: 'border-surface-200 bg-surface-50 text-surface-500',
  failed: 'border-red-200 bg-red-50 text-red-700',
  skipped: 'border-surface-200 bg-surface-50 text-surface-400',
}

export function ProjectOverviewPanel({
  project,
  episodeCount,
  pipelineSnapshot,
  projectControlStages,
  projectControlDoneCount,
  projectControlOverallProgress,
  projectControlCurrentStage,
  workflowSteps,
  notices,
  nextAction,
  onNextAction,
}: ProjectOverviewPanelProps) {
  return (
    <div className="rounded-3xl border border-surface-200 bg-white p-5 shadow-sm">
      <div className="flex flex-wrap items-center gap-2 text-xs text-surface-500">
        <span className="rounded-full border border-surface-200 bg-surface-50 px-2.5 py-1">项目总览</span>
        <span className="rounded-full border border-surface-200 bg-surface-50 px-2.5 py-1">剧集 {episodeCount}</span>
        <span className="rounded-full border border-surface-200 bg-surface-50 px-2.5 py-1">
          风格标签 {project.style_tags?.length ?? 0}
        </span>
        <span className="inline-flex items-center gap-1 rounded-full border border-surface-200 bg-surface-50 px-2.5 py-1">
          <Clock3 className="h-3.5 w-3.5" /> {episodeSplitOverviewBadge(project)}
        </span>
      </div>

      <div className="mt-4 flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
        <div className="min-w-0 flex-1">
          <h3 className="text-xl font-semibold text-surface-900">剧本大纲与项目总控</h3>
          <p className="mt-2 text-sm leading-6 text-surface-600">
            在这里查看项目进度、获取下一步建议，并在下方剧本区继续分集与编辑。
          </p>
        </div>
        <div className="w-full shrink-0 rounded-2xl border border-primary-100 bg-primary-50/60 px-4 py-3 lg:max-w-sm">
          <p className="text-sm font-semibold text-surface-900">{nextAction.title}</p>
          <p className="mt-1 text-xs leading-5 text-surface-600">{nextAction.description}</p>
          {nextAction.type !== 'noop' ? (
            <Button
              size="sm"
              className="mt-3"
              disabled={nextAction.disabled}
              onClick={onNextAction}
            >
              {nextAction.cta}
            </Button>
          ) : null}
        </div>
      </div>

      {notices.length > 0 ? (
        <div className="mt-4 space-y-2">
          {notices.map((notice) => (
            <div
              key={`${notice.title}-${notice.description}`}
              className={`flex items-start gap-2 rounded-xl border px-3 py-2 text-xs ${
                notice.tone === 'red'
                  ? 'border-red-200 bg-red-50 text-red-800'
                  : 'border-blue-200 bg-blue-50 text-blue-800'
              }`}
            >
              <AlertCircle className="mt-0.5 h-3.5 w-3.5 shrink-0" />
              <div>
                <p className="font-semibold">{notice.title}</p>
                <p className="mt-0.5 leading-5 opacity-90">{notice.description}</p>
              </div>
            </div>
          ))}
        </div>
      ) : null}

      <div className="mt-4 rounded-2xl border border-surface-200 bg-surface-50/70 p-4">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <p className="text-sm font-medium text-surface-900">
              {pipelineSnapshot.isActive && projectControlCurrentStage
                ? `当前进行中 · ${projectControlCurrentStage.label}`
                : pipelineSnapshot.episodeSplitDone && !pipelineSnapshot.isActive
                  ? '分集已完成'
                  : '项目总控进度'}
            </p>
            <p className="mt-1 text-xs leading-5 text-surface-500">
              {pipelineSnapshot.isActive
                ? pipelineSnapshot.activeDetail
                : pipelineSnapshot.episodeSplitDone
                  ? pipelineSnapshot.nextStepHint
                  : '上传剧本后，分集、资源提取与分镜拆分会在这里显示进度。'}
            </p>
          </div>
          <div className="text-right">
            <p className="text-[11px] uppercase tracking-[0.18em] text-surface-400">Progress</p>
            <p className="text-2xl font-semibold text-surface-900">{projectControlOverallProgress}%</p>
            <p className="text-[11px] text-surface-500">已完成 {projectControlDoneCount}/3 阶段</p>
          </div>
        </div>
        <div className="mt-3 h-2 overflow-hidden rounded-full bg-white">
          <div
            className="h-full rounded-full bg-primary-500 transition-all duration-500"
            style={{ width: `${projectControlOverallProgress}%` }}
          />
        </div>
        <div className="mt-4 grid gap-3 md:grid-cols-3">
          {projectControlStages.map((stage, index) => {
            const isRunning = stage.status === 'running'
            const isDone = stage.status === 'done'
            return (
              <div
                key={stage.key}
                className={`rounded-xl border px-3 py-2.5 ${
                  isRunning ? 'border-primary-200 bg-white' : isDone ? 'border-emerald-200 bg-white' : 'border-surface-200 bg-white'
                }`}
              >
                <div className="flex items-center gap-2">
                  <div className={`flex h-6 w-6 items-center justify-center rounded-full text-[11px] font-semibold ${
                    isRunning ? 'bg-primary-500 text-white' : isDone ? 'bg-emerald-500 text-white' : 'bg-surface-100 text-surface-500'
                  }`}>
                    {isRunning ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : isDone ? <CheckCircle2 className="h-3.5 w-3.5" /> : index + 1}
                  </div>
                  <div>
                    <p className="text-sm font-semibold text-surface-900">{stage.label}</p>
                    <p className={`text-[11px] ${isRunning ? 'text-primary-600' : isDone ? 'text-emerald-600' : 'text-surface-400'}`}>
                      {isRunning ? '处理中' : isDone ? '已完成' : '待开始'}
                    </p>
                  </div>
                </div>
                <p className="mt-2 text-xs leading-5 text-surface-600">{stage.detail}</p>
              </div>
            )
          })}
        </div>
      </div>

      <div className="mt-4 grid gap-2 sm:grid-cols-2 xl:grid-cols-3">
        {workflowSteps.map((step) => (
          <div key={step.key} className={`rounded-xl border px-3 py-2.5 ${stepStatusClass[step.status]}`}>
            <p className="text-sm font-semibold">{step.label}</p>
            <p className="mt-1 text-xs leading-5 opacity-90">{step.hint}</p>
          </div>
        ))}
      </div>
    </div>
  )
}
