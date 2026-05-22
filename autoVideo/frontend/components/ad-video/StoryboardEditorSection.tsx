import { AlertCircle, CheckCircle2, Loader2, Sparkles } from 'lucide-react'
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

const storyboardStatusClass: Record<StoryboardEditorShot['storyboardStatusTone'], string> = {
  slate: 'border-surface-200 bg-white text-surface-600',
  amber: 'border-amber-200 bg-amber-50 text-amber-800',
  blue: 'border-cyan-200 bg-cyan-50 text-cyan-800',
  emerald: 'border-emerald-200 bg-emerald-50 text-emerald-800',
}

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
          <p className="text-xs text-surface-500">现在按“一条分镜占据一行”展示；图片描述词与视频描述词分开维护，避免混在一个字段里。</p>
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

      <div className="space-y-3">
        {storyboardPreview.map((shot) => (
          <div key={shot.index} className="rounded-2xl border border-surface-200 bg-surface-50 p-4">
            <div className="flex flex-wrap items-start justify-between gap-3">
              <div>
                <p className="text-xs uppercase tracking-[0.2em] text-surface-500">镜头 {shot.index + 1}</p>
                <p className="mt-1 text-sm font-medium text-surface-800">{shot.imageSource}</p>
              </div>
              <div className="flex flex-wrap items-center gap-2">
                <span className={[
                  'rounded-full border px-2 py-1 text-[11px] font-medium',
                  storyboardStatusClass[shot.storyboardStatusTone],
                ].join(' ')}>
                  {shot.storyboardStatusLabel}
                </span>
                <span className="rounded-full border border-surface-200 bg-white px-2 py-1 text-[11px] font-medium text-surface-600">
                  {shot.hasDialogue ? '台词已填' : '待补台词'}
                </span>
                <span className="rounded-full border border-surface-200 bg-white px-2 py-1 text-[11px] font-medium text-surface-600">
                  {shot.hasReferenceHint ? '图片词已填' : '待补图片词'}
                </span>
                <span className="rounded-full border border-violet-200 bg-white px-2 py-1 text-[11px] font-medium text-violet-700">{shot.imageStatusLabel}</span>
              </div>
            </div>

            <div className={[
              'mt-3 rounded-xl border px-3 py-2',
              shot.storyboardStatusTone === 'amber'
                ? 'border-amber-200 bg-amber-50/70'
                : shot.storyboardStatusTone === 'blue'
                  ? 'border-cyan-200 bg-cyan-50/70'
                  : shot.storyboardStatusTone === 'emerald'
                    ? 'border-emerald-200 bg-emerald-50/70'
                    : 'border-surface-200 bg-white',
            ].join(' ')}>
              <div className="flex items-start gap-2">
                {shot.storyboardStatusTone === 'amber' ? (
                  <AlertCircle className="mt-0.5 h-4 w-4 text-amber-600" />
                ) : shot.storyboardStatusTone === 'emerald' ? (
                  <CheckCircle2 className="mt-0.5 h-4 w-4 text-emerald-600" />
                ) : shot.storyboardStatusTone === 'blue' ? (
                  <Loader2 className="mt-0.5 h-4 w-4 animate-spin text-cyan-600" />
                ) : (
                  <Sparkles className="mt-0.5 h-4 w-4 text-surface-500" />
                )}
                <div className="min-w-0 flex-1">
                  <p className="text-[11px] font-medium text-surface-800">分镜状态</p>
                  <p className="mt-1 text-[11px] leading-5 text-surface-600">{shot.storyboardStatusDetail}</p>
                  {shot.realStoryboardId ? (
                    <div className="mt-2 flex flex-wrap gap-2 text-[10px] text-surface-500">
                      <span className="rounded-full border border-surface-200 bg-white px-2 py-0.5">
                        分镜ID #{shot.realStoryboardId}
                      </span>
                      {shot.realStoryboardSequence ? (
                        <span className="rounded-full border border-surface-200 bg-white px-2 py-0.5">
                          序号 {shot.realStoryboardSequence}
                        </span>
                      ) : null}
                      {shot.realStoryboardStatus ? (
                        <span className="rounded-full border border-surface-200 bg-white px-2 py-0.5">
                          后端状态 {shot.realStoryboardStatus}
                        </span>
                      ) : null}
                      {shot.realStoryboardUpdatedAt ? (
                        <span className="rounded-full border border-surface-200 bg-white px-2 py-0.5">
                          更新于 {new Date(shot.realStoryboardUpdatedAt).toLocaleString('zh-CN')}
                        </span>
                      ) : null}
                    </div>
                  ) : null}
                  {shot.realStoryboardError ? (
                    <p className="mt-2 text-[10px] leading-4 text-amber-700">失败原因：{shot.realStoryboardError}</p>
                  ) : null}
                  {shot.storyboardBlockingItems.length > 0 ? (
                    <div className="mt-2 flex flex-wrap gap-2">
                      {shot.storyboardBlockingItems.map((item) => (
                        <span key={`${shot.index}-${item}`} className="rounded-full border border-amber-200 bg-white px-2 py-0.5 text-[10px] text-amber-700">
                          {item}
                        </span>
                      ))}
                    </div>
                  ) : null}
                </div>
              </div>
            </div>

            <div className="mt-4 space-y-4">
              <div className="overflow-hidden rounded-xl border border-violet-100 bg-white">
                {shot.imagePreviewUrl ? (
                  <img src={shot.imagePreviewUrl} alt={`镜头 ${shot.index + 1} 参考图`} className="h-36 w-full object-cover" />
                ) : (
                  <div className="flex h-24 items-center justify-center px-4 text-center text-xs leading-5 text-violet-600">
                    暂无真实图片；后续将优先按图片描述词生成分镜图。
                  </div>
                )}
              </div>

              <div className="space-y-2 rounded-xl border border-cyan-200 bg-cyan-50/50 p-3">
                <div className="flex items-center justify-between gap-2">
                  <Label className="text-xs font-medium text-cyan-900">分镜图片描述词</Label>
                  <Button
                    type="button"
                    size="sm"
                    variant="ghost"
                    onClick={() => onFillReferenceHintAtIndex(shot)}
                    disabled={referenceHintGeneratingAll || referenceHintGeneratingIndex === shot.index}
                    className="h-7 gap-1 px-2 text-[11px] text-cyan-700 hover:bg-cyan-100 hover:text-cyan-900"
                  >
                    {referenceHintGeneratingIndex === shot.index ? (
                      <Loader2 className="h-3 w-3 animate-spin" />
                    ) : (
                      <Sparkles className="h-3 w-3" />
                    )}
                    AI 补全图片词
                  </Button>
                </div>
                <Textarea
                  rows={5}
                  value={shot.referenceHint}
                  placeholder={shot.referenceHintPlaceholder || '例如：人物半身，暖色逆光，办公桌前，科技感金融界面，真实摄影风格'}
                  onChange={(event) => onReferenceHintChange(shot.index, event.target.value)}
                />
                <div className="rounded-lg border border-cyan-100 bg-white px-3 py-2">
                  <p className="text-[11px] font-medium text-cyan-800">当前有效图片描述词</p>
                  <p className="mt-1 max-h-24 overflow-y-auto text-[11px] leading-5 text-cyan-700 whitespace-pre-wrap">{shot.referenceHintResolved}</p>
                </div>
              </div>

              <div className="space-y-2 rounded-xl border border-violet-200 bg-violet-50/50 p-3">
                <Label className="text-xs font-medium text-violet-900">分镜视频描述词</Label>
                <Textarea
                  rows={5}
                  value={shot.scene}
                  placeholder={shot.scenePlaceholder || '例如：镜头缓慢推进到人物表情，字幕从左下淡入，强调正在观看学习内容'}
                  onChange={(event) => onSceneChange(shot.index, event.target.value)}
                />
                <div className="rounded-lg border border-violet-100 bg-white px-3 py-2">
                  <p className="text-[11px] font-medium text-violet-800">当前有效视频描述词</p>
                  <p className="mt-1 max-h-24 overflow-y-auto text-[11px] leading-5 text-violet-700 whitespace-pre-wrap">{shot.sceneResolved}</p>
                </div>
              </div>

              <div className="space-y-2 rounded-xl border border-surface-200 bg-white p-3">
                <Label className="text-xs font-medium text-surface-800">字幕 / 口播</Label>
                <Textarea
                  rows={5}
                  value={shot.dialogue}
                  placeholder={shot.dialoguePlaceholder || '每行一句，支持逐镜头微调'}
                  onChange={(event) => onDialogueChange(shot.index, event.target.value)}
                />
                <div className="rounded-lg border border-surface-200 bg-surface-50 px-3 py-2">
                  <p className="text-[11px] font-medium text-surface-700">当前有效台词</p>
                  <p className="mt-1 text-[11px] leading-5 text-surface-600">{shot.dialogueResolved}</p>
                </div>
              </div>
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}
