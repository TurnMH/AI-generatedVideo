'use client'

import { Button } from '@/components/ui/button'

type GenerationTask = {
  id: string
  createdAt: string
  label: string
  status: 'queued' | 'optimizing' | 'uploading' | 'submitting' | 'running' | 'succeeded' | 'failed'
  step: string
  projectId?: number
  outputUrl?: string
  error?: string
  title: string
  marketLabel: string
  brandVoiceLabel: string
  storyboardLabel: string
  subtitleCount: number
  imageCount: number
}

interface GenerationQueuePanelProps {
  generationTasks: GenerationTask[]
  activeGenerationTaskId: string
  onOpenProject: (projectId: number) => void
  onOpenOutput: (outputUrl: string) => void
}

const statusClass: Record<GenerationTask['status'], string> = {
  queued: 'border-violet-200 bg-violet-50 text-violet-700',
  optimizing: 'border-violet-200 bg-violet-50 text-violet-700',
  uploading: 'border-violet-200 bg-violet-50 text-violet-700',
  submitting: 'border-violet-200 bg-violet-50 text-violet-700',
  running: 'border-violet-200 bg-violet-50 text-violet-700',
  succeeded: 'border-emerald-200 bg-emerald-50 text-emerald-700',
  failed: 'border-rose-200 bg-rose-50 text-rose-700',
}

const statusLabel: Record<GenerationTask['status'], string> = {
  queued: '排队中',
  optimizing: '文案优化中',
  uploading: '上传素材中',
  submitting: '提交中',
  running: '后台生成中',
  succeeded: '已完成',
  failed: '失败',
}

export function GenerationQueuePanel({
  generationTasks,
  activeGenerationTaskId,
  onOpenProject,
  onOpenOutput,
}: GenerationQueuePanelProps) {
  return (
    <div className="space-y-4 rounded-xl border border-violet-100 bg-violet-50/40 p-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div>
          <p className="text-sm font-medium text-violet-900">异步生成列表</p>
          <p className="text-xs text-violet-700">点击后会先加入列表，再在后台依次完成文案优化、素材上传和视频提交。</p>
        </div>
        <span className="rounded-full border border-violet-200 bg-white px-2.5 py-1 text-[11px] font-medium text-violet-700">
          当前任务 {generationTasks.length}
        </span>
      </div>

      {generationTasks.length > 0 ? (
        <div className="space-y-2">
          {generationTasks.map((task) => {
            const isActive = task.id === activeGenerationTaskId
            return (
              <div
                key={task.id}
                className={[
                  'rounded-lg border px-3 py-3 text-xs',
                  isActive ? 'border-violet-300 bg-white' : 'border-surface-200 bg-white/80',
                ].join(' ')}
              >
                <div className="flex flex-wrap items-start justify-between gap-2">
                  <div>
                    <p className="text-sm font-medium text-surface-800">{task.label}</p>
                    <p className="mt-1 text-[11px] text-surface-500">
                      {new Date(task.createdAt).toLocaleString('zh-CN')}
                    </p>
                  </div>
                  <span className={[
                    'rounded-full border px-2.5 py-1 text-[11px] font-medium',
                    statusClass[task.status],
                  ].join(' ')}>
                    {statusLabel[task.status]}
                  </span>
                </div>
                <p className="mt-2 text-surface-600">{task.step}</p>
                <div className="mt-2 flex flex-wrap gap-2 text-[11px] text-surface-500">
                  <span className="rounded-full border border-surface-200 bg-surface-50 px-2 py-0.5">市场 {task.marketLabel}</span>
                  <span className="rounded-full border border-surface-200 bg-surface-50 px-2 py-0.5">语气 {task.brandVoiceLabel}</span>
                  <span className="rounded-full border border-surface-200 bg-surface-50 px-2 py-0.5">分镜 {task.storyboardLabel}</span>
                  <span className="rounded-full border border-surface-200 bg-surface-50 px-2 py-0.5">图片 {task.imageCount}</span>
                  <span className="rounded-full border border-surface-200 bg-surface-50 px-2 py-0.5">台词 {task.subtitleCount}</span>
                  {task.projectId ? <span className="rounded-full border border-surface-200 bg-surface-50 px-2 py-0.5">项目 {task.projectId}</span> : null}
                </div>
                {task.error ? <p className="mt-2 text-[11px] text-rose-600">{task.error}</p> : null}
                {(task.projectId || task.outputUrl) ? (
                  <div className="mt-3 flex flex-wrap gap-2">
                    {task.projectId ? (
                      <Button size="sm" variant="outline" className="h-7 px-2.5 text-[11px]" onClick={() => onOpenProject(task.projectId as number)}>
                        打开项目
                      </Button>
                    ) : null}
                    {task.outputUrl ? (
                      <Button size="sm" variant="outline" className="h-7 px-2.5 text-[11px]" onClick={() => onOpenOutput(task.outputUrl as string)}>
                        预览/下载
                      </Button>
                    ) : null}
                  </div>
                ) : null}
              </div>
            )
          })}
        </div>
      ) : (
        <p className="text-xs text-violet-600">还没有提交任何异步生成任务。</p>
      )}
    </div>
  )
}
