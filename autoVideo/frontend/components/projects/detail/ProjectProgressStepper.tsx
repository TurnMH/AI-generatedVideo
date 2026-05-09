'use client'

import React from 'react'
import { AlertCircle, CheckCircle2, Clock, Loader2 } from 'lucide-react'
import type { Project } from '@/types'
import {
  buildWorkflowSteps,
  type StepAssetStats,
  type StepStoryboardStats,
  type StepDubbingStats,
  type StepVideoStats,
  type WorkflowStepKey,
} from '@/lib/projects/workflow'

type TabKey = WorkflowStepKey

export function ProjectProgressStepper({ project, episodeCount, assetStats, storyboardStats, dubbingStats, videoStats, activeTab, onStepChange, stepLabelOverrides }: {
  project: Project
  episodeCount?: number
  assetStats?: StepAssetStats
  storyboardStats?: StepStoryboardStats
  dubbingStats?: StepDubbingStats
  videoStats?: StepVideoStats
  activeTab: TabKey
  onStepChange: (tab: TabKey) => void
  stepLabelOverrides?: Partial<Record<WorkflowStepKey, string>>
}) {
  const steps = buildWorkflowSteps({
    project,
    episodeCount,
    assetStats,
    storyboardStats,
    dubbingStats,
    videoStats,
  }).map((step) => ({
    ...step,
    label: stepLabelOverrides?.[step.key] ?? step.label,
  }))
  const allDone = steps.every((step) => step.status === 'done' || step.status === 'skipped')

  return (
    <div className="rounded-xl border bg-white p-4 shadow-sm">
      <div className="mb-3 flex items-center justify-between gap-2">
        <div>
          <p className="text-sm font-semibold text-surface-900">项目流程</p>
        </div>
        {allDone ? <span className="rounded-full bg-green-100 px-2 py-1 text-[11px] text-green-700">全部完成</span> : null}
      </div>

      <div className="space-y-2">
        {steps.map((step, i) => {
          const isSelected = activeTab === step.key
          const isDone = step.status === 'done'
          const isCurrent = step.status === 'current'
          const isFailed = step.status === 'failed'
          const isPaused = step.status === 'paused'
          const isSkipped = step.status === 'skipped'

          const circleClass = isDone
            ? 'border-green-500 bg-green-500 text-white'
            : isCurrent && step.processing
              ? 'border-blue-500 bg-primary-500 text-white'
              : isCurrent
                ? 'border-blue-400 bg-primary-100 text-primary-600'
                : isFailed
                  ? 'border-red-400 bg-red-100 text-red-500'
                  : isPaused
                    ? 'border-yellow-400 bg-yellow-100 text-yellow-600'
                    : isSkipped
                      ? 'border-surface-300 bg-surface-50 text-surface-400'
                      : 'border-surface-300 bg-surface-100 text-surface-400'

          const labelClass = isDone
            ? 'font-medium text-green-600'
            : isCurrent
              ? 'font-semibold text-primary-600'
              : isFailed
                ? 'font-medium text-red-500'
                : isPaused
                  ? 'font-medium text-yellow-700'
                  : isSkipped
                      ? 'text-surface-400'
                      : 'text-surface-400'

          const lineClass = isDone || isSkipped ? 'bg-green-400' : 'bg-surface-200'
          const selectedClass = isSelected
            ? 'border-blue-200 bg-blue-50 shadow-sm'
            : 'border-surface-200 bg-white hover:border-surface-300 hover:bg-surface-50'

          return (
            <React.Fragment key={step.key}>
              <button
                type="button"
                onClick={() => onStepChange(step.key)}
                className={`w-full rounded-xl border px-3 py-3 text-left transition ${selectedClass}`}
                title={`切换到${step.label}`}
              >
                <div className="flex items-start gap-3">
                  <div className={`mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-full border-2 transition ${circleClass}`}>
                    {isDone ? (
                      <CheckCircle2 className="h-5 w-5" />
                    ) : isCurrent && step.processing ? (
                      <Loader2 className="h-4 w-4 animate-spin" />
                    ) : isFailed ? (
                      <AlertCircle className="h-4 w-4" />
                    ) : isPaused ? (
                      <Clock className="h-4 w-4" />
                    ) : (
                      <step.Icon className="h-4 w-4" />
                    )}
                  </div>

                  <div className="min-w-0 flex-1">
                    <div className="flex items-center justify-between gap-2">
                      <span className={`text-sm ${labelClass}`}>{step.label}</span>
                      {isCurrent && (
                        <span className="rounded-full bg-blue-100 px-1.5 py-0.5 text-[10px] text-blue-700">
                          {step.processing ? '进行中' : '当前'}
                        </span>
                      )}
                    </div>

                    {step.subLabel && (
                      <p className="mt-1 text-[11px] text-surface-500">{step.subLabel}</p>
                    )}

                    {step.progress !== null && (
                      <div className="mt-2 h-1.5 overflow-hidden rounded-full bg-surface-200">
                        <div
                          className={`h-full rounded-full transition-all ${isDone || isSkipped ? 'bg-green-400' : isFailed ? 'bg-red-400' : isPaused ? 'bg-yellow-400' : 'bg-blue-400'}`}
                          style={{ width: `${step.progress}%` }}
                        />
                      </div>
                    )}

                    <div className="mt-2 flex flex-wrap gap-1">
                      {isFailed && (
                        <span className="rounded bg-red-100 px-1.5 py-0.5 text-[10px] text-red-500">失败</span>
                      )}
                      {isPaused && (
                        <span className="rounded bg-yellow-100 px-1.5 py-0.5 text-[10px] text-yellow-700">已暂停</span>
                      )}
                      {isSkipped && (
                        <span className="rounded bg-surface-100 px-1.5 py-0.5 text-[10px] text-surface-500">可选</span>
                      )}
                    </div>
                  </div>
                </div>
              </button>
              {i < steps.length - 1 && (
                <div className="flex justify-center py-1">
                  <div className={`h-4 w-0.5 rounded-full ${lineClass}`} />
                </div>
              )}
            </React.Fragment>
          )
        })}
      </div>

      {allDone && <p className="mt-3 text-center text-xs text-green-600">🎉 所有阶段已完成</p>}
    </div>
  )
}
