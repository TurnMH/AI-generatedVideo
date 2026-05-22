'use client'

import { Button } from '@/components/ui/button'
import { Download, Loader2, Repeat, RefreshCw } from 'lucide-react'

type TaskLogEntry = {
  at: string
  level: 'info' | 'progress' | 'success' | 'warning' | 'error'
  message: string
}

interface CurrentTaskPanelProps {
  activeProjectId: number | null
  activeTaskId: number | null
  taskStatus: 'idle' | 'pending' | 'processing' | 'succeeded' | 'failed'
  taskError: string
  taskOutputUrl: string
  taskClipProgress: { done: number; total: number }
  autoRetryAttempts: number
  adTaskLogs: TaskLogEntry[]
  activeOptimizeTaskId: number | null
  manualRerunLoading: boolean
  exportingPackage: boolean
  nextActionLabel?: string
  nextActionHint?: string
  onOpenProject: (projectId: number) => void
  onOpenOutput: () => void
  onRerunAnotherVersion: () => void
  onExportPackage: () => void
}

export function CurrentTaskPanel({
  activeProjectId,
  activeTaskId,
  taskStatus,
  taskError,
  taskOutputUrl,
  taskClipProgress,
  autoRetryAttempts,
  adTaskLogs,
  activeOptimizeTaskId,
  manualRerunLoading,
  exportingPackage,
  nextActionLabel,
  nextActionHint,
  onOpenProject,
  onOpenOutput,
  onRerunAnotherVersion,
  onExportPackage,
}: CurrentTaskPanelProps) {
  if (!activeProjectId) return null

  return (
    <div className="rounded-xl border border-cyan-200 bg-cyan-50/60 p-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <p className="text-sm font-medium text-cyan-900">广告视频输出状态</p>
          <p className="mt-1 text-xs text-cyan-700">
            项目 ID: {activeProjectId}
            {activeTaskId ? ` · 任务 ID: ${activeTaskId}` : ''}
            {taskClipProgress.total > 0 ? ` · 片段 ${taskClipProgress.done}/${taskClipProgress.total}` : ''}
          </p>
        </div>
        <div className="flex items-center gap-2">
          <span className="rounded-full border border-cyan-300 bg-white px-3 py-1 text-xs font-medium text-cyan-800">
            {taskStatus === 'idle' && '等待开始'}
            {taskStatus === 'pending' && '排队中'}
            {taskStatus === 'processing' && '生成中'}
            {taskStatus === 'succeeded' && '已完成'}
            {taskStatus === 'failed' && '失败'}
          </span>
          <Button size="sm" variant="outline" onClick={() => onOpenProject(activeProjectId)}>
            打开项目
          </Button>
        </div>
      </div>

      <div className="mt-3 rounded-lg border border-cyan-200 bg-white/80 p-3">
        <p className="text-xs font-medium text-cyan-900">下一步建议</p>
        <p className="mt-1 text-sm text-cyan-800">{nextActionLabel || '继续查看当前任务状态'}</p>
        {nextActionHint ? <p className="mt-1 text-[11px] leading-5 text-cyan-700">{nextActionHint}</p> : null}
        {taskStatus === 'failed' && taskError ? (
          <p className="mt-2 text-[11px] text-rose-600">失败原因：{taskError}</p>
        ) : null}
      </div>

      {autoRetryAttempts > 0 ? (
        <p className="mt-2 text-xs text-cyan-700">已执行自动重试次数：{autoRetryAttempts}</p>
      ) : null}

      <div className="mt-3 rounded-lg border border-cyan-200 bg-white/80 p-3">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div>
            <p className="text-xs font-medium text-cyan-900">当前广告任务日志</p>
            <p className="text-[11px] text-cyan-700">
              {activeOptimizeTaskId ? `文案任务 #${activeOptimizeTaskId}` : '本地任务流'}
            </p>
          </div>
          <span className="rounded-full border border-cyan-200 bg-cyan-50 px-2 py-0.5 text-[11px] font-medium text-cyan-700">
            {adTaskLogs.length} 条
          </span>
        </div>
        <div className="mt-3 max-h-56 space-y-2 overflow-auto pr-1">
          {adTaskLogs.length > 0 ? (
            adTaskLogs.map((entry, index) => {
              const colorClass = {
                info: 'text-surface-600',
                progress: 'text-cyan-700',
                success: 'text-emerald-700',
                warning: 'text-amber-700',
                error: 'text-rose-700',
              }[entry.level]
              return (
                <div key={`${entry.at}-${index}`} className="flex gap-2 text-[11px] leading-5">
                  <span className="shrink-0 font-mono text-surface-400">
                    {new Date(entry.at).toLocaleTimeString('zh-CN', { hour12: false })}
                  </span>
                  <span className={`min-w-0 ${colorClass}`}>{entry.message}</span>
                </div>
              )
            })
          ) : (
            <p className="text-[11px] text-surface-500">当前还没有任务日志，开始生成后会自动显示。</p>
          )}
        </div>
      </div>

      {taskStatus === 'succeeded' && taskOutputUrl ? (
        <div className="mt-3 flex flex-wrap gap-2">
          <Button size="sm" className="gap-1.5" onClick={onOpenOutput}>
            <Download className="h-3.5 w-3.5" />
            预览/下载成片
          </Button>
          <Button
            size="sm"
            variant="outline"
            className="gap-1.5"
            onClick={onRerunAnotherVersion}
            disabled={manualRerunLoading}
          >
            {manualRerunLoading ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Repeat className="h-3.5 w-3.5" />}
            再生成一个版本
          </Button>
          <Button
            size="sm"
            variant="outline"
            className="gap-1.5"
            onClick={onExportPackage}
            disabled={exportingPackage}
          >
            {exportingPackage ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Download className="h-3.5 w-3.5" />}
            导出投放包
          </Button>
          <a
            href={taskOutputUrl}
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center rounded-md border border-cyan-300 bg-white px-3 py-1.5 text-xs font-medium text-cyan-800 hover:bg-cyan-50"
          >
            新标签打开输出链接
          </a>
        </div>
      ) : null}

      {(taskStatus === 'pending' || taskStatus === 'processing') ? (
        <p className="mt-2 flex items-center gap-1.5 text-xs text-cyan-700">
          <RefreshCw className="h-3.5 w-3.5 animate-spin" />
          正在自动轮询任务结果，生成完成后会直接显示下载入口。
        </p>
      ) : null}
    </div>
  )
}
