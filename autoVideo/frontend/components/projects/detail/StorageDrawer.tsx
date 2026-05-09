'use client'

import useSWR, { mutate as globalMutate } from 'swr'
import { Trash2, X } from 'lucide-react'
import { storageAPI } from '@/lib/api'
import type { StorageDetails } from '@/types'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { useToast } from '@/components/ui/toast'
import { formatBytes } from '@/lib/projects/utils'
import { TabSkeleton } from './TabSkeleton'

export function StorageDrawer({ projectId, open, onClose }: { projectId: number; open: boolean; onClose: () => void }) {
  const { toast } = useToast()

  const { data: storageData, isLoading, mutate: mutateStorage } = useSWR(
    open ? ['storage', projectId] : null,
    () => storageAPI.getDetails(projectId) as unknown as Promise<{ data: StorageDetails }>
  )
  const storage = (storageData as { data?: StorageDetails })?.data

  const handleDeleteFile = async (fileId: number) => {
    try {
      await storageAPI.deleteFiles(projectId, [fileId])
      toast({ title: '文件已删除', variant: 'success' })
      mutateStorage()
      globalMutate(['project', projectId])
    } catch {
      toast({ title: '删除失败', variant: 'destructive' })
    }
  }

  if (!open) return null

  const categories = storage?.categories ?? {}
  const totalBytes = storage?.total_bytes ?? 0
  const files = storage?.files ?? []
  const catEntries = Object.entries(categories)
  const CAT_COLORS: Record<string, string> = {
    script: 'bg-primary-500',
    asset: 'bg-purple-500',
    storyboard: 'bg-green-500',
    video: 'bg-orange-500',
    audio: 'bg-pink-500',
    other: 'bg-surface-400',
  }
  const CAT_LABELS: Record<string, string> = {
    script: '剧本',
    asset: '资源',
    storyboard: '分镜',
    video: '视频',
    audio: '音频',
    other: '其他',
  }

  return (
    <div className="fixed inset-0 z-50 flex justify-end">
      <div className="absolute inset-0 bg-black/30" onClick={onClose} />
      <div className="relative z-50 flex h-full w-full max-w-md flex-col bg-white shadow-xl">
        <div className="flex items-center justify-between border-b px-4 py-3">
          <h3 className="text-lg font-semibold">存储详情</h3>
          <Button size="sm" variant="ghost" onClick={onClose}>
            <X className="h-4 w-4" />
          </Button>
        </div>
        <div className="flex-1 overflow-y-auto p-4">
          {isLoading ? (
            <TabSkeleton />
          ) : (
            <>
              {/* Total usage */}
              <div className="mb-6 text-center">
                <p className="text-3xl font-bold text-surface-900">{formatBytes(totalBytes)}</p>
                <p className="text-sm text-surface-500">总使用量</p>
              </div>

              {/* Category breakdown bar */}
              <div className="mb-4">
                <div className="mb-2 flex h-4 w-full overflow-hidden rounded-full bg-surface-100">
                  {catEntries.map(([cat, summary]) => {
                    const pct = totalBytes > 0 ? (summary.bytes / totalBytes) * 100 : 0
                    if (pct < 0.5) return null
                    return (
                      <div
                        key={cat}
                        className={`${CAT_COLORS[cat] || 'bg-surface-400'} transition-all`}
                        style={{ width: `${pct}%` }}
                        title={`${CAT_LABELS[cat] || cat}: ${formatBytes(summary.bytes)}`}
                      />
                    )
                  })}
                </div>
                <div className="flex flex-wrap gap-3 text-xs text-surface-600">
                  {catEntries.map(([cat, summary]) => (
                    <span key={cat} className="flex items-center gap-1">
                      <span className={`inline-block h-2 w-2 rounded-full ${CAT_COLORS[cat] || 'bg-surface-400'}`} />
                      {CAT_LABELS[cat] || cat}: {formatBytes(summary.bytes)} ({summary.count}个)
                    </span>
                  ))}
                </div>
              </div>

              {/* File list */}
              <h4 className="mb-2 text-sm font-semibold text-surface-700">文件列表</h4>
              {files.length === 0 ? (
                <p className="py-4 text-center text-xs text-surface-400">暂无文件</p>
              ) : (
                <div className="space-y-2">
                  {files.map((f) => (
                    <div key={f.id} className="flex items-center justify-between rounded-lg border px-3 py-2">
                      <div className="min-w-0 flex-1">
                        <p className="truncate text-sm">{f.label}</p>
                        <div className="flex items-center gap-2 text-xs text-surface-400">
                          <span>{CAT_LABELS[f.category] || f.category}</span>
                          <span>{formatBytes(f.size_bytes)}</span>
                          {f.is_current && <Badge variant="success" className="text-[10px]">当前</Badge>}
                        </div>
                      </div>
                      {f.deletable && !f.is_current && (
                        <Button size="sm" variant="ghost" className="h-7 w-7 flex-shrink-0 p-0 text-red-500 hover:text-red-700" onClick={() => handleDeleteFile(f.id)} title="删除此文件">
                          <Trash2 className="h-3.5 w-3.5" />
                        </Button>
                      )}
                    </div>
                  ))}
                </div>
              )}
            </>
          )}
        </div>
      </div>
    </div>
  )
}
