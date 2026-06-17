
'use client'

import { useState } from 'react'
import useSWR from 'swr'
import { RefreshCw, Loader2, Image as ImageIcon, Sparkles, AlertCircle } from 'lucide-react'
import type { Storyboard } from '@/types'
import { storyboardAPI } from '@/lib/api'
import { canTriggerStoryboardImage, triggerStoryboardImageGeneration } from '@/lib/projects/storyboard-image'
import { ZoomableImage, ZoomBadge } from '@/components/ui/image-lightbox'
import { formatDuration } from '@/lib/projects/utils'
import { StatusBadge } from './StatusBadge'
import { Button } from '@/components/ui/button'
import { useToast } from '@/components/ui/toast'

export function EpisodeStoryboardList({ projectId, episodeId }: { projectId: number; episodeId: number }) {
  const { toast } = useToast()
  const [runningId, setRunningId] = useState<number | null>(null)

  const { data: sbData, mutate } = useSWR(
    episodeId ? ['storyboards-episode', projectId, episodeId] : null,
    () => storyboardAPI.list(projectId, { episode_id: episodeId }) as unknown as Promise<{ data: Storyboard[] | { items: Storyboard[] } }>,
    { refreshInterval: (data) => {
      const items = Array.isArray(data?.data) ? data?.data : (data?.data as { items?: Storyboard[] })?.items || [];
      return items.some((sb: Storyboard) => sb.status === 'pending' || sb.status === 'generating') ? 3000 : 0;
    } }
  )
  
  const rawSb = (sbData as { data?: Storyboard[] | { items?: Storyboard[] } })?.data
  const storyboards: Storyboard[] = Array.isArray(rawSb) ? rawSb : (rawSb as { items?: Storyboard[] })?.items ?? []

  if (!episodeId) return <p className="text-xs text-surface-400">请选择集数</p>

  if (storyboards.length === 0) {
    return <p className="py-6 text-center text-xs text-surface-400">该集暂无分镜数据，请先生成或上传</p>
  }

  const handleGenerateOne = async (sb: Storyboard) => {
    setRunningId(sb.id)
    try {
      await triggerStoryboardImageGeneration(projectId, sb)
      toast({ title: sb.status === 'failed' ? '已重新加入生成队列' : '分镜图片生成已启动', variant: 'success' })
      mutate()
    } catch (e: unknown) {
      const message = e instanceof Error ? e.message : '网络错误'
      toast({ title: '生成失败', description: message, variant: 'destructive' })
    } finally {
      setRunningId(null)
    }
  }

  return (
    <div className="max-h-[36rem] space-y-3 overflow-y-auto pr-1">
      {storyboards
        .sort((a, b) => a.sequence_number - b.sequence_number)
        .map((sb) => {
          const isFailed = sb.status === 'failed'
          const isGenerating = sb.status === 'generating'
          const isPending = sb.status === 'pending'
          const isPaused = sb.status === 'paused'
          const isCompleted = sb.status === 'completed'
          const showGenerateAction = canTriggerStoryboardImage(sb) && !isGenerating
          
          const isOptimizingPrompt = isGenerating && !sb.prompt_used
          const isGeneratingImage = isGenerating && !!sb.prompt_used

          return (
            <div key={sb.id} className={`flex flex-col gap-2 rounded-xl border p-3 shadow-sm transition-all ${
              isFailed ? 'border-red-200 bg-red-50/30' : isCompleted ? 'border-surface-200 bg-white' : 'border-blue-100 bg-blue-50/20'
            }`}>
              <div className="flex items-start gap-4">
                <div className="relative h-20 w-32 flex-shrink-0 overflow-hidden rounded-md border border-surface-100 bg-surface-100 shadow-sm group">
                  {sb.image_url ? (
                    <>
                      <ZoomableImage src={sb.image_url} alt={`#${sb.sequence_number}`} className="h-full w-full object-cover" />
                      <div className="absolute right-1 top-1 z-10">
                        <ZoomBadge src={sb.image_url} alt={`#${sb.sequence_number}`} />
                      </div>
                    </>
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
                  <div className="flex items-center justify-between gap-2">
                    <div className="flex items-center gap-2">
                      <span className="text-sm font-bold text-surface-800">#{sb.sequence_number}</span>
                      <StatusBadge status={sb.status} />
                    </div>
                    
                    {showGenerateAction && (
                      <Button 
                        size="sm" 
                        variant="outline" 
                        className={`h-6 px-2 text-[11px] ${isFailed ? 'border-red-200 text-red-600 hover:bg-red-50' : 'border-primary-200 text-primary-700 hover:bg-primary-50'}`}
                        onClick={() => handleGenerateOne(sb)}
                        disabled={runningId === sb.id}
                      >
                        {runningId === sb.id ? (
                          <Loader2 className="mr-1 h-3 w-3 animate-spin" />
                        ) : isFailed ? (
                          <RefreshCw className="mr-1 h-3 w-3" />
                        ) : (
                          <Sparkles className="mr-1 h-3 w-3" />
                        )}
                        {isFailed ? '失败重试' : isCompleted && sb.image_url ? '重新生成' : '生成图片'}
                      </Button>
                    )}
                  </div>
                  <p className="mt-1 line-clamp-2 text-xs leading-5 text-surface-600" title={sb.scene_description}>
                    {sb.scene_description}
                  </p>
                  
                  {sb.dialogue && (
                     <p className="mt-1 text-[11px] text-purple-600 line-clamp-1">💬 {sb.dialogue}</p>
                  )}
                  
                  <div className="mt-2.5 flex items-center gap-2 text-[10px] font-medium">
                    <span className={`flex items-center gap-1 ${(isCompleted || isGeneratingImage) ? 'text-green-600' : isOptimizingPrompt ? 'text-blue-600' : 'text-surface-400'}`}>
                      {isOptimizingPrompt ? <Loader2 className="h-3 w-3 animate-spin" /> : <Sparkles className="h-3 w-3" />}
                      {isCompleted || isGeneratingImage ? '提示词已优化' : isOptimizingPrompt ? '提示词优化中...' : '提示词待优化'}
                    </span>
                    <span className="text-surface-300">→</span>
                    
                    <span className={`flex items-center gap-1 ${isCompleted && sb.image_url ? 'text-green-600' : isGeneratingImage ? 'text-blue-600' : isFailed ? 'text-red-500' : isPending || isPaused ? 'text-amber-600' : 'text-surface-400'}`}>
                      {isGeneratingImage ? <Loader2 className="h-3 w-3 animate-spin" /> : isFailed ? <AlertCircle className="h-3 w-3" /> : <ImageIcon className="h-3 w-3" />}
                      {isCompleted && sb.image_url ? '分镜图生成成功' : isGeneratingImage ? '分镜图生成中...' : isFailed ? '生图失败' : isPending || isPaused ? '待生成' : '排队中'}
                    </span>
                  </div>
                  
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
