'use client'

import { Progress } from '@/components/ui/progress'

type WorkflowStepStatus = 'done' | 'active' | 'todo' | 'failed'

export type WorkflowSidebarStep = {
  key: string
  label: string
  detail: string
  status: WorkflowStepStatus
}

interface WorkflowSidebarProps {
  steps: WorkflowSidebarStep[]
  progressValue: number
  progressDetail: string
  taskProgressValue: number | null
  taskProgressDetail: string | null
  resourceSummary: string
}

const statusLabel: Record<WorkflowStepStatus, string> = {
  done: '已完成',
  active: '进行中',
  todo: '待办',
  failed: '失败',
}

const statusTone: Record<WorkflowStepStatus, string> = {
  done: 'border-emerald-200 bg-emerald-50 text-emerald-700',
  active: 'border-cyan-200 bg-cyan-50 text-cyan-800',
  todo: 'border-surface-200 bg-surface-50 text-surface-500',
  failed: 'border-rose-200 bg-rose-50 text-rose-700',
}

const dotTone: Record<WorkflowStepStatus, string> = {
  done: 'border-emerald-300 bg-emerald-500 text-white',
  active: 'border-cyan-300 bg-cyan-500 text-white',
  todo: 'border-surface-300 bg-white text-surface-500',
  failed: 'border-rose-300 bg-rose-500 text-white',
}

export function WorkflowSidebar({
  steps,
  progressValue,
  progressDetail,
  taskProgressValue,
  taskProgressDetail,
  resourceSummary,
}: WorkflowSidebarProps) {
  const currentStep = steps.find((step) => step.status === 'active' || step.status === 'failed')
    ?? steps.find((step) => step.status === 'todo')
    ?? steps[steps.length - 1]

  return (
    <section className="rounded-[28px] border border-cyan-200 bg-gradient-to-b from-cyan-50 via-white to-slate-50 p-4 shadow-sm lg:max-h-[calc(100vh-3rem)] lg:overflow-y-auto">
      <div className="rounded-[22px] border border-white/80 bg-white/90 p-4 shadow-sm">
        <p className="text-xs font-medium uppercase tracking-[0.24em] text-cyan-500">当前流程</p>
        <div className="mt-2 flex items-end justify-between gap-3">
          <div className="min-w-0">
            <p className="text-sm font-medium text-surface-900">{currentStep?.label ?? '等待开始'}</p>
            <p className="mt-1 text-xs leading-5 text-surface-600">{currentStep?.detail ?? '先填写广告文案，然后补齐镜头资源。'}</p>
          </div>
          <span className="shrink-0 rounded-full border border-cyan-200 bg-cyan-50 px-2.5 py-1 text-[11px] font-medium text-cyan-700">
            {Math.round(progressValue)}%
          </span>
        </div>
        <Progress value={progressValue} className="mt-3 h-2" />
        <p className="mt-2 text-[11px] leading-5 text-surface-500">{progressDetail}</p>
      </div>

      <div className="mt-4 rounded-[22px] border border-surface-200 bg-white/90 p-4 shadow-sm">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div>
            <p className="text-sm font-medium text-surface-900">步骤条</p>
            <p className="text-xs text-surface-500">按文案、资源、审核、提交和输出依次推进。</p>
          </div>
          <span className="rounded-full border border-surface-200 bg-surface-50 px-2 py-0.5 text-[11px] font-medium text-surface-600">
            {steps.length} 步
          </span>
        </div>

        <div className="mt-4 space-y-4">
          {steps.map((step, index) => (
            <div key={step.key} className="flex gap-3">
              <div className="flex flex-col items-center">
                <span className={[
                  'flex h-7 w-7 items-center justify-center rounded-full border text-[11px] font-semibold',
                  dotTone[step.status],
                ].join(' ')}>
                  {index + 1}
                </span>
                {index < steps.length - 1 ? <span className="mt-1 h-full w-px bg-surface-200" /> : null}
              </div>
              <div className="min-w-0 flex-1 pb-1">
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <p className="text-xs font-medium text-surface-800">{step.label}</p>
                  <span className={[
                    'rounded-full border px-2 py-0.5 text-[11px] font-medium',
                    statusTone[step.status],
                  ].join(' ')}>
                    {statusLabel[step.status]}
                  </span>
                </div>
                <p className="mt-1 text-[11px] leading-5 text-surface-500">{step.detail}</p>
              </div>
            </div>
          ))}
        </div>
      </div>

      <div className="mt-4 space-y-3">
        <div className="rounded-[22px] border border-surface-200 bg-white/90 p-4 shadow-sm">
          <p className="text-xs font-medium uppercase tracking-[0.2em] text-surface-500">镜头资源概览</p>
          <p className="mt-2 text-sm font-medium text-surface-900">{resourceSummary}</p>
          <p className="mt-1 text-xs leading-5 text-surface-500">本地文件和图片 URL 都会进入镜头预览与后续生成流程。</p>
        </div>

        {taskProgressValue !== null ? (
          <div className="rounded-[22px] border border-emerald-200 bg-emerald-50/80 p-4 shadow-sm">
            <p className="text-xs font-medium uppercase tracking-[0.2em] text-emerald-600">当前任务进度</p>
            <p className="mt-2 text-sm font-medium text-emerald-900">{taskProgressDetail ?? '任务已提交，后台正在处理'}</p>
            <Progress value={taskProgressValue} className="mt-3 h-2" />
            <p className="mt-2 text-[11px] text-emerald-700">{Math.round(taskProgressValue)}%</p>
          </div>
        ) : null}
      </div>
    </section>
  )
}
