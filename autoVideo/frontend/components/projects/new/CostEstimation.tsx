'use client'

import { Sparkles } from 'lucide-react'
import type { Model } from '@/types'
import { Card, CardContent } from '@/components/ui/card'
import type { FormData } from '@/lib/projects/new/form-types'

export function CostEstimation({
  form,
  textModels,
  imageModels,
  videoModels,
  ttsModels,
}: {
  form: FormData
  textModels: Model[]
  imageModels: Model[]
  videoModels: Model[]
  ttsModels: Model[]
}) {
  const findModel = (models: Model[], id?: number) => models.find((m) => m.id === id)
  const textModel = findModel(textModels, form.text_model_id)
  const imageModel = findModel(imageModels, form.image_model_id)
  const videoModel = findModel(videoModels, form.video_model_id)
  const ttsModel = findModel(ttsModels, form.tts_model_id)

  const episodeCount = form.target_episodes || 1
  const estimatedScenes = episodeCount * 15

  const costs = [
    { label: '文本生成', cost: textModel ? textModel.cost_per_unit * episodeCount * 2 : 0 },
    { label: '图片生成', cost: imageModel ? imageModel.cost_per_unit * estimatedScenes : 0 },
    { label: '视频生成', cost: videoModel ? videoModel.cost_per_unit * estimatedScenes : 0 },
    { label: '语音合成', cost: form.enable_dubbing && ttsModel ? ttsModel.cost_per_unit * episodeCount * 5 : 0 },
  ]

  const totalCost = costs.reduce((sum, c) => sum + c.cost, 0)

  return (
    <Card className="bg-surface-50">
      <CardContent className="space-y-3 pt-4">
        <h4 className="flex items-center gap-2 text-sm font-medium text-surface-700">
          <Sparkles className="h-4 w-4 text-amber-500" />
          预估费用（仅供参考）
        </h4>
        <div className="space-y-1">
          {costs.map((c) => (
            <div key={c.label} className="flex justify-between text-xs text-surface-500">
              <span>{c.label}</span>
              <span>{c.cost > 0 ? `¥${c.cost.toFixed(2)}` : '-'}</span>
            </div>
          ))}
        </div>
        <div className="border-t pt-2">
          <div className="flex justify-between text-sm font-medium">
            <span>预估总计</span>
            <span className="text-primary-600">¥{totalCost.toFixed(2)}</span>
          </div>
          <p className="mt-1 text-[10px] text-surface-400">
            基于 {episodeCount} 集 × 约{estimatedScenes}个分镜估算
          </p>
        </div>
      </CardContent>
    </Card>
  )
}
