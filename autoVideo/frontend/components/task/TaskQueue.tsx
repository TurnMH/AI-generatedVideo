'use client'
import useSWR from 'swr'
import { format } from 'date-fns'
import { useProjectProgress } from '@/lib/websocket'
import { useTaskStore } from '@/lib/store/task'
import { taskAPI } from '@/lib/api'
import type { Task } from '@/types'
import { Badge } from '@/components/ui/badge'
import { Progress } from '@/components/ui/progress'
import { Button } from '@/components/ui/button'
import { LoadingSpinner } from '@/components/common/LoadingSpinner'
import { useState } from 'react'

interface TaskQueueProps {
  projectId: number
}

const statusVariant: Record<
  Task['status'],
  'default' | 'secondary' | 'success' | 'destructive' | 'warning' | 'outline'
> = {
  pending: 'secondary',
  running: 'default',
  succeeded: 'success',
  failed: 'destructive',
  cancelled: 'outline',
}

const statusLabel: Record<Task['status'], string> = {
  pending: '待处理',
  running: '进行中',
  succeeded: '完成',
  failed: '失败',
  cancelled: '已取消',
}

export function TaskQueue({ projectId }: TaskQueueProps) {
  useProjectProgress(projectId)

  const { progressMap } = useTaskStore()
  const [expandedId, setExpandedId] = useState<number | null>(null)
  const [cancelling, setCancelling] = useState<number | null>(null)

  const { data, isLoading, mutate } = useSWR(
    ['tasks', projectId],
    () =>
      taskAPI.list({ page: 1, page_size: 50 }) as unknown as Promise<{
        data: { items: Task[] }
      }>,
    { refreshInterval: (data) => {
        const items = (data as { data?: { items?: Task[] } } | undefined)?.data?.items ?? []
        const active = items.some((t) => t.status === 'running')
        return active ? 5000 : 30000
      } }
  )

  const tasks: Task[] = (data as { data?: { items?: Task[] } })?.data?.items ?? []

  const handleCancel = async (taskId: number) => {
    setCancelling(taskId)
    try {
      await taskAPI.cancel(taskId)
      await mutate()
    } catch {
      // ignore
    } finally {
      setCancelling(null)
    }
  }

  const counts = tasks.reduce(
    (acc, t) => {
      acc[t.status] = (acc[t.status] ?? 0) + 1
      return acc
    },
    {} as Record<string, number>
  )

  if (isLoading) {
    return (
      <div className="flex h-40 items-center justify-center">
        <LoadingSpinner />
      </div>
    )
  }

  return (
    <div className="space-y-4">
      {/* Summary badges */}
      <div className="flex flex-wrap gap-3">
        {[
          { label: '待处理', key: 'pending', cls: 'bg-surface-100 text-surface-700' },
          { label: '进行中', key: 'running', cls: 'bg-primary-100 text-primary-700' },
          { label: '完成', key: 'succeeded', cls: 'bg-green-100 text-green-700' },
          { label: '失败', key: 'failed', cls: 'bg-red-100 text-red-700' },
        ].map(({ label, key, cls }) => (
          <div
            key={key}
            className={`flex items-center gap-2 rounded-lg px-4 py-2 text-sm font-medium ${cls}`}
          >
            <span>{label}</span>
            <span className="text-lg font-bold">{counts[key] ?? 0}</span>
          </div>
        ))}
      </div>

      {/* Task table */}
      <div className="overflow-hidden rounded-lg border border-surface-200">
        <table className="w-full text-sm">
          <thead className="bg-surface-50">
            <tr>
              {['ID', '类型', '状态', '进度', '创建时间', '操作'].map((h) => (
                <th
                  key={h}
                  className="px-4 py-3 text-left text-xs font-semibold uppercase tracking-wide text-surface-500"
                >
                  {h}
                </th>
              ))}
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-100">
            {tasks.length === 0 && (
              <tr>
                <td colSpan={6} className="py-10 text-center text-surface-400">
                  暂无任务
                </td>
              </tr>
            )}
            {tasks.map((task) => {
              const liveProgress = progressMap[task.id]
              const progress =
                liveProgress?.progress ?? task.progress ?? 0
              const isExpanded = expandedId === task.id

              return (
                <>
                  <tr
                    key={task.id}
                    className={`cursor-pointer transition-colors hover:bg-surface-50 ${
                      task.status === 'failed' ? 'cursor-pointer' : ''
                    }`}
                    onClick={() =>
                      task.status === 'failed' &&
                      setExpandedId(isExpanded ? null : task.id)
                    }
                  >
                    <td className="px-4 py-3 font-mono text-surface-500">#{task.id}</td>
                    <td className="px-4 py-3 text-surface-700">{task.task_type}</td>
                    <td className="px-4 py-3">
                      <Badge variant={statusVariant[task.status]}>
                        {statusLabel[task.status]}
                      </Badge>
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-2">
                        <Progress value={progress} className="w-24" />
                        <span className="w-8 text-right text-xs text-surface-500">
                          {progress}%
                        </span>
                      </div>
                    </td>
                    <td className="px-4 py-3 text-surface-500">
                      {format(new Date(task.created_at), 'MM-dd HH:mm')}
                    </td>
                    <td className="px-4 py-3">
                      {(task.status === 'pending' || task.status === 'running') && (
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={(e) => {
                            e.stopPropagation()
                            handleCancel(task.id)
                          }}
                          disabled={cancelling === task.id}
                          title="取消此任务"
                        >
                          {cancelling === task.id ? (
                            <LoadingSpinner size="sm" />
                          ) : (
                            '取消'
                          )}
                        </Button>
                      )}
                    </td>
                  </tr>
                  {isExpanded && task.error_msg && (
                    <tr key={`${task.id}-error`}>
                      <td
                        colSpan={6}
                        className="bg-red-50 px-4 py-3 text-xs text-red-700"
                      >
                        <strong>错误信息：</strong> {task.error_msg}
                      </td>
                    </tr>
                  )}
                </>
              )
            })}
          </tbody>
        </table>
      </div>
    </div>
  )
}
