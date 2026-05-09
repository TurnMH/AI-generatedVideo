
'use client'

import { useState } from 'react'
import useSWR from 'swr'
import { LayoutGrid, RefreshCw, Loader2, Image as ImageIcon, Sparkles, AlertCircle, Play } from 'lucide-react'
import { storyboardAPI } from '@/lib/api'
import type { Storyboard } from '@/types'
import { ZoomableImage } from '@/components/ui/image-lightbox'
import { formatDuration } from '@/lib/projects/utils'
import { StatusBadge } from './StatusBadge'
import { Button } from '@/components/ui/button'
import { useToast } from '@/components/ui/toast'

export function EpisodeStoryboardList({ projectId, episodeId }: { projectId: number; episodeId: number }) {
  const { toast } = useToast()
  const [retryingId, setRetryingId] = useState<number | null>(null)

  const { data: sbData, mutate } = useSWR(
    episodeId ? ['storyboards-episode', projectId, episodeId] : null,
    () => storyboardAPI.list(projectId, { episode_id: episodeId }) as unknown as Promise<{ data: Storyboard[] | { items: Storyboard[] } }>,
    { refreshInterval: (data) => {
      const items = Array.isArray(data?.data) ? data?.data : (data?.data as any)?.items || [];
      return items.some((sb: Storyboard) => sb.status === 'pending' || sb.status === 'generating') ? 3000 : 0;
    } }
  )
  
  const rawSb = (sbData as { data?: Storyboard[] | { items?: Storyboard[] } })?.data
  const storyboards: Storyboard[] = Array.isArray(rawSb) ? rawSb : (rawSb as { items?: Storyboard[] })?.items ?? []

  if (!episodeId) return <p className="text-xs text-surface-400">请选择集数</p>

  if (storyboards.length === 0) {
    return <p className="py-6 text-center text-xs text-surface-400">该集暂无分镜数据，请先生成或上传</p>
  }

  const handleRetry = async (sbId: number) => {
    setRetryingId(sbId)
    try {
      await storyboardAPI.retry(projectId, sbId)
      toast({ title: '已重新加入生成队列', variant: 'success' })
      mutate()
    } catch (e: any) {
      toast({ title: '重试失败', description: e?.message || '网络错误', variant: 'destructive' })
    } finally {
      setRetryingId(null)
    }
  }

  return (
    <div className="max-h-[36rem] space-y-3 overflow-y-auto pr-1">
      {storyboards
        .sort((a, b) => a.sequence_number - b.sequence_number)
        .map((sb) => {
          // 智能推断细粒度状态
          const isFailed = sb.status === 'failed'
          const isGenerating = sb.status === 'generating'
          const isPending = sb.status === 'pending'
          const isCompleted = sb.status === 'completed'
          
          // 如果正在 generating，有 prompt 视为正在生图，没有则认为还在优化提示词或排队
          const isOptimizingPrompt = isGenerating && !sb.prompt_used
          const isGeneratingImage = isGenerating && !!sb.prompt_used

          return (
            <div key={sb.id} className={`flex flex-col gap-2 rounded-xl border p-3 shadow-sm transition-all ${
              isFailed ? 'border-red-200 bg-red-50/30' : isCompleted ? 'border-surface-200 bg-white' : 'border-blue-100 bg-blue-50/20'
            }`}>
              <div className="flex items-start gap-4">
                <div className="relative h-20 w-32 flex-shrink-0 overflow-hidden rounded-md border border-surface-100 bg-surface-100 shadow-sm">
                  {sb.image_url ? (
                    <ZoomableImage src={sb.image_url} alt={`#${sb.sequence_number}`} className="h-full w-full object-cover" />
                  ) : (
                    <div className="flex h-full flex-col items-center justify-center text-surface-400 bg-surface-50">
                      {isGenerating ? <Loader2 className="h-5 w-5 animate-spin text-blue-500 mb-1" /> : <ImageIcon className="h-5 w-5 mb-1 opacity-50" />}
                      <span className="text-[10px]">{isGenerating ? '处理中...' : '待生成'}</span>
                    </div>
                  )}
                  {sb.duration > 0 && (
                    <div className="absolute bottom-1 right-1 rounded bg-black/60 px-1.5 py-0.5 text-[9px] font-medium text-white">
                      {formatDuration(sb.duration)}
                    </div>
                  )}
                </div>
                
                <div className="min-w-0 flex-1">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2">
                      <span className="text-sm font-bold text-surface-800">#{sb.sequence_number}</span>
                      <StatusBadge status={sb.status} />
                    </div>
                    
                    {isFailed && (
                      <Button 
                        size="sm" 
                        variant="outline" 
                        className="h-6 px-2 text-[11px] border-red-200 text-red-600 hover:bg-red-50"
                        onClick={() => handleRetry(sb.id)}
                        disabled={retryingId === sb.id}
                      >
                        {retryingId === sb.id ? <Loader2 className="mr-1 h-3 w-3 animate-spin" /> : <RefreshCw className="mr-1 h-3 w-3" />}
                        失败重试
                      </Button>
                    )}
                  </div>
                  <p className="mt-1 line-clamp-2 text-xs leading-5 text-surface-600" title={sb.scene_description}>
                    {sb.scene_description}
                  </p>
                  
                  {/* 对话与角色信息 (如果有) */}
                  {sb.dialogue && (
                     <p className="mt-1 text-[11px] text-purple-600 line-clamp-1">💬 {sb.dialogue}</p>
                  )}
                  
                  {/* 细粒度任务流水线展示 */}
                  <div className="mt-2.5 flex items-center gap-2 text-[10px] font-medium">
                    {/* 1. 提示词优化阶段 */}
                    <span className={`flex items-center gap-1 ${(isCompleted || isGeneratingImage) ? 'text-green-600' : isOptimizingPrompt ? 'text-blue-600' : 'text-surface-400'}`}>
                      {isOptimizingPrompt ? <Loader2 className="h-3 w-3 animate-spin" /> : <Sparkles className="h-3 w-3" />}
                      {isCompleted || isGeneratingImage ? '提示词已优化' : isOptimizingPrompt ? '提示词优化中...' : '提示词待优化'}
                    </span>
                    <span className="text-surface-300">→</span>
                    
                    {/* 2. 生图阶段 */}
                    <span className={`flex items-center gap-1 ${isCompleted ? 'text-green-600' : isGeneratingImage ? 'text-blue-600' : isFailed ? 'text-red-500' : 'text-surface-400'}`}>
                      {isGeneratingImage ? <Loader2 className="h-3 w-3 animate-spin" /> : isFailed ? <AlertCircle className="h-3 w-3" /> : <ImageIcon className="h-3 w-3" />}
                      {isCompleted ? '分镜图生成成功' : isGeneratingImage ? '分镜图生成中...' : isFailed ? '生图失败' : '排队中'}
                    </span>
                    <span className="text-surface-300">→</span>
                    
                    {/* 3. 视频阶段预留 */}
                    <span className="flex items-center gap-1 text-surface-400">
                      <Play className="h-3 w-3" />
                      视频待合成
                    </span>
                  </div>
                  
                  {/* 错误信息展示 */}
                  {isFailed && sb.error_msg && (
                    <div className="mt-1.5 rounded border border-red-100 bg-red-50 px-2 py-1 text-[10px] text-red-600">
                      失败原因：{sb.error_msg}
                    </div>
                  )}
                </div>
              </div>
            </div>
          )
        })}
    </div>
  )
}
