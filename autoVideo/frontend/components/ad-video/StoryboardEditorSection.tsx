import { Loader2, Sparkles } from 'lucide-react'
import type { StoryboardPreviewItem } from '@/components/ad-video/types'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'

type StoryboardEditorShot = StoryboardPreviewItem

type StoryboardEditorSectionProps = {
  storyboardPreview: StoryboardEditorShot[]
  referenceHintGeneratingAll: boolean
  referenceHintGeneratingIndex: number | null
  referenceHintModels: Array<{ id: number; name: string; model_key: string }>
  selectedReferenceHintModel: string
  onSelectedReferenceHintModelChange: (value: string) => void
  onFillAllReferenceHints: () => void
  onFillReferenceHintAtIndex: (shot: StoryboardEditorShot) => void
  onSceneChange: (index: number, value: string) => void
  onReferenceHintChange: (index: number, value: string) => void
  onDialogueChange: (index: number, value: string) => void
}

export function StoryboardEditorSection({
  storyboardPreview,
  referenceHintGeneratingAll,
  referenceHintGeneratingIndex,
  referenceHintModels,
  selectedReferenceHintModel,
  onSelectedReferenceHintModelChange,
  onFillAllReferenceHints,
  onFillReferenceHintAtIndex,
  onSceneChange,
  onReferenceHintChange,
  onDialogueChange,
}: StoryboardEditorSectionProps) {
  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div>
          <p className="text-sm font-medium text-surface-800">分镜可视化编辑</p>
          <p className="text-xs text-surface-500">每张卡片都对应一段镜头，修改会同步回上方分镜描述和字幕文本。</p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <span className="rounded-full border border-cyan-200 bg-cyan-50 px-2.5 py-1 text-[11px] font-medium text-cyan-700">
            {storyboardPreview.length} 个镜头
          </span>
          <div className="flex items-center gap-2 rounded-lg border border-cyan-200 bg-white px-2 py-1">
            <Label className="text-[11px] text-cyan-800">提示词模型</Label>
            <Select value={selectedReferenceHintModel} onValueChange={onSelectedReferenceHintModelChange}>
              <SelectTrigger className="h-7 w-[210px] border-0 bg-transparent px-1 text-xs shadow-none focus:ring-0">
                <SelectValue placeholder="选择提示词模型" />
              </SelectTrigger>
              <SelectContent>
                {referenceHintModels.length > 0 ? referenceHintModels.map((model) => (
                  <SelectItem key={`ref-hint-${model.id}`} value={model.model_key}>{model.name}</SelectItem>
                )) : <SelectItem value="__none" disabled>暂无可用文本模型</SelectItem>}
              </SelectContent>
            </Select>
          </div>
          <Button
            type="button"
            size="sm"
            variant="outline"
            onClick={onFillAllReferenceHints}
            disabled={referenceHintGeneratingAll || referenceHintGeneratingIndex !== null || storyboardPreview.length === 0}
            className="h-8 gap-2 border-cyan-200 bg-cyan-50 text-cyan-800 hover:bg-cyan-100"
          >
            {referenceHintGeneratingAll ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Sparkles className="h-3.5 w-3.5" />}
            AI 补全全部参考图提示词
          </Button>
        </div>
      </div>

      <div className="grid gap-4 xl:grid-cols-2">
        {storyboardPreview.map((shot) => (
          <div key={shot.index} className="rounded-2xl border border-surface-200 bg-surface-50 p-4">
            <div className="flex items-start justify-between gap-3">
              <div>
                <p className="text-xs uppercase tracking-[0.2em] text-surface-500">镜头 {shot.index + 1}</p>
                <p className="mt-1 text-sm font-medium text-surface-800">{shot.imageSource}</p>
              </div>
              <div className="flex flex-col items-end gap-1">
                <span className="rounded-full border border-surface-200 bg-white px-2 py-1 text-[11px] font-medium text-surface-600">
                  {shot.hasDialogue ? '台词已填' : '待补台词'}
                </span>
                <span className="rounded-full border border-surface-200 bg-white px-2 py-1 text-[11px] font-medium text-surface-600">
                  {shot.hasReferenceHint ? '参考图已填' : '待补参考图'}
                </span>
              </div>
            </div>
            <div className="mt-3 space-y-3">
              <div className="space-y-1">
                <Label className="text-xs text-surface-700">分镜描述</Label>
                <Textarea
                  rows={3}
                  value={shot.scene}
                  placeholder={shot.scenePlaceholder || '未填写时会使用广告文案兜底'}
                  onChange={(event) => onSceneChange(shot.index, event.target.value)}
                />
              </div>
              <div className="space-y-1">
                <div className="flex items-center justify-between gap-2">
                  <Label className="text-xs text-surface-700">镜头参考图</Label>
                  <Button
                    type="button"
                    size="sm"
                    variant="ghost"
                    onClick={() => onFillReferenceHintAtIndex(shot)}
                    disabled={referenceHintGeneratingAll || referenceHintGeneratingIndex === shot.index}
                    className="h-7 gap-1 px-2 text-[11px] text-cyan-700 hover:bg-cyan-50 hover:text-cyan-900"
                  >
                    {referenceHintGeneratingIndex === shot.index ? (
                      <Loader2 className="h-3 w-3 animate-spin" />
                    ) : (
                      <Sparkles className="h-3 w-3" />
                    )}
                    AI 补全提示词
                  </Button>
                </div>
                <Input
                  value={shot.referenceHint}
                  placeholder={shot.referenceHintPlaceholder || '例如：白底产品特写 / 手持使用场景'}
                  onChange={(event) => onReferenceHintChange(shot.index, event.target.value)}
                />
                <p className="text-[11px] leading-5 text-surface-500">这里填写参考图风格、构图、主体或检索关键词，供后续找图、选图或生成提示词使用，不会自动上传真实图片。上方可单独选择用于生成提示词的文本模型。</p>
              </div>
              <div className="space-y-1">
                <Label className="text-xs text-surface-700">字幕 / 口播</Label>
                <Textarea
                  rows={3}
                  value={shot.dialogue}
                  placeholder={shot.dialoguePlaceholder || '每行一句，支持逐镜头微调'}
                  onChange={(event) => onDialogueChange(shot.index, event.target.value)}
                />
              </div>
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}
