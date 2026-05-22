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
  imageModels: Array<{ id: number; name: string; model_key: string }>
  selectedImageModel: string
  onSelectedImageModelChange: (value: string) => void
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
  imageModels,
  selectedImageModel,
  onSelectedImageModelChange,
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

      <div className="rounded-xl border border-violet-200 bg-violet-50/60 p-3">
        <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
          <div className="space-y-1">
            <p className="text-xs font-medium text-violet-900">图片生成逻辑</p>
            <p className="text-[11px] leading-5 text-violet-700">若你没有提供真实图片，系统会优先依据“镜头参考图提示词”配合所选图片模型生成镜头图，再交给视频模型合成；如果你已上传真实图片，则优先使用你的图片素材。</p>
          </div>
          <div className="flex flex-wrap gap-2">
            <div className="flex items-center gap-2 rounded-lg border border-violet-200 bg-white px-2 py-1">
              <Label className="text-[11px] text-violet-800">图片模型</Label>
              <Select value={selectedImageModel} onValueChange={onSelectedImageModelChange}>
                <SelectTrigger className="h-7 w-[210px] border-0 bg-transparent px-1 text-xs shadow-none focus:ring-0">
                  <SelectValue placeholder="选择图片模型" />
                </SelectTrigger>
                <SelectContent>
                  {imageModels.length > 0 ? imageModels.map((model) => (
                    <SelectItem key={`image-model-${model.id}`} value={model.model_key}>{model.name}</SelectItem>
                  )) : <SelectItem value="__none" disabled>暂无可用图片模型</SelectItem>}
                </SelectContent>
              </Select>
            </div>
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
          </div>
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
              <div className="grid gap-3 rounded-xl border border-dashed border-violet-200 bg-violet-50/40 p-3 md:grid-cols-[180px,1fr]">
                <div className="overflow-hidden rounded-lg border border-violet-100 bg-white">
                  {shot.imagePreviewUrl ? (
                    <img src={shot.imagePreviewUrl} alt={`镜头 ${shot.index + 1} 参考图`} className="h-40 w-full object-cover" />
                  ) : (
                    <div className="flex h-40 items-center justify-center px-4 text-center text-xs leading-5 text-violet-600">
                      暂无真实图片；确认提示词后会优先按所选图片模型生成分镜图。
                    </div>
                  )}
                </div>
                <div className="space-y-2">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="rounded-full border border-violet-200 bg-white px-2 py-1 text-[11px] font-medium text-violet-700">{shot.imageStatusLabel}</span>
                    <span className="rounded-full border border-violet-200 bg-white px-2 py-1 text-[11px] font-medium text-violet-700">当前有效提示词</span>
                  </div>
                  <p className="rounded-lg border border-violet-100 bg-white px-3 py-2 text-xs leading-5 text-violet-800">{shot.referenceHintResolved}</p>
                  <p className="text-[11px] leading-5 text-violet-600">当前分镜区会同时展示“提示词”和“图片来源”。如果没有真实图，后续应按这里的提示词先生成分镜图，再推进视频生成。</p>
                </div>
              </div>
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
