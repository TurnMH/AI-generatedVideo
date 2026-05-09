'use client'
import { cn } from '@/lib/utils'
import type { PipelineStage } from '@/types'
import { Check, X, Loader2 } from 'lucide-react'

interface PipelineProgressProps {
  currentStage: PipelineStage
  startTime?: Date
}

const stages: { key: PipelineStage; label: string; description: string }[] = [
  { key: 'INIT', label: '初始化', description: '准备流水线环境' },
  { key: 'SCRIPT_ANALYZING', label: '脚本分析', description: '解析剧本结构与角色' },
  { key: 'ASSETS_EXTRACTING', label: '素材提取', description: '提取场景与角色资产' },
  { key: 'STORYBOARD_GENERATING', label: '分镜生成', description: '生成分镜脚本与提示词' },
  { key: 'IMAGES_GENERATING', label: '图像生成', description: 'AI 生成场景图像' },
  { key: 'VIDEOS_GENERATING', label: '视频生成', description: 'AI 生成视频片段' },
  { key: 'COMPOSING', label: '合成剪辑', description: '拼接视频与音频' },
  { key: 'AUTO_FIXING', label: '自动修复', description: '检测并修复质量问题' },
]

const stageOrder: PipelineStage[] = [
  'INIT',
  'SCRIPT_ANALYZING',
  'ASSETS_EXTRACTING',
  'STORYBOARD_GENERATING',
  'IMAGES_GENERATING',
  'VIDEOS_GENERATING',
  'COMPOSING',
  'AUTO_FIXING',
  'DONE',
]

function getStageIndex(stage: PipelineStage): number {
  return stageOrder.indexOf(stage)
}

type StageStatus = 'done' | 'current' | 'pending' | 'failed'

function getStageStatus(
  stageKey: PipelineStage,
  currentStage: PipelineStage
): StageStatus {
  if (currentStage === 'FAILED') {
    const stageIdx = getStageIndex(stageKey)
    const currentIdx = getStageIndex(currentStage)
    if (stageIdx < currentIdx) return 'done'
    if (stageIdx === currentIdx) return 'failed'
    return 'pending'
  }
  const stageIdx = getStageIndex(stageKey)
  const currentIdx = getStageIndex(currentStage)
  if (currentStage === 'DONE') return 'done'
  if (stageIdx < currentIdx) return 'done'
  if (stageIdx === currentIdx) return 'current'
  return 'pending'
}

export function PipelineProgress({ currentStage, startTime }: PipelineProgressProps) {
  const isDone = currentStage === 'DONE'
  const isFailed = currentStage === 'FAILED'

  return (
    <div className="rounded-lg border border-surface-200 bg-white p-6">
      <div className="mb-4 flex items-center justify-between">
        <h3 className="text-sm font-semibold text-surface-900">流水线进度</h3>
        {startTime && (
          <span className="text-xs text-surface-400">
            开始于 {startTime.toLocaleTimeString()}
          </span>
        )}
        {isDone && (
          <span className="text-xs font-medium text-green-600">✓ 已完成</span>
        )}
        {isFailed && (
          <span className="text-xs font-medium text-red-600">✗ 失败</span>
        )}
      </div>

      <div className="relative">
        {stages.map((stage, index) => {
          const status = getStageStatus(stage.key, currentStage)
          const isLast = index === stages.length - 1

          return (
            <div key={stage.key} className="flex gap-4">
              {/* Timeline indicator */}
              <div className="flex flex-col items-center">
                <div
                  className={cn(
                    'flex h-8 w-8 items-center justify-center rounded-full border-2 transition-colors',
                    status === 'done' &&
                      'border-green-500 bg-green-500 text-white',
                    status === 'current' &&
                      'border-blue-500 bg-primary-50 text-primary-600',
                    status === 'pending' &&
                      'border-surface-200 bg-white text-surface-300',
                    status === 'failed' && 'border-red-500 bg-red-500 text-white'
                  )}
                >
                  {status === 'done' && <Check className="h-4 w-4" />}
                  {status === 'current' && (
                    <Loader2 className="h-4 w-4 animate-spin" />
                  )}
                  {status === 'pending' && (
                    <div className="h-2 w-2 rounded-full bg-surface-300" />
                  )}
                  {status === 'failed' && <X className="h-4 w-4" />}
                </div>
                {!isLast && (
                  <div
                    className={cn(
                      'mt-1 w-0.5 flex-1',
                      status === 'done' ? 'bg-green-300' : 'bg-surface-200'
                    )}
                    style={{ minHeight: '2rem' }}
                  />
                )}
              </div>

              {/* Stage content */}
              <div className={cn('pb-6', isLast && 'pb-0')}>
                <p
                  className={cn(
                    'text-sm font-medium leading-8',
                    status === 'done' && 'text-green-700',
                    status === 'current' && 'text-primary-700',
                    status === 'pending' && 'text-surface-400',
                    status === 'failed' && 'text-red-700'
                  )}
                >
                  {stage.label}
                </p>
                <p className="text-xs text-surface-400">{stage.description}</p>
              </div>
            </div>
          )
        })}
      </div>
    </div>
  )
}
