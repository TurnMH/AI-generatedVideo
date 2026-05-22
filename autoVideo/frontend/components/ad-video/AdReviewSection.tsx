import type { ReactNode } from 'react'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'

type ReviewChecklistItem = {
  key: string
  label: string
  passed: boolean
  detail: string
  blocking: boolean
}

type StoryboardTemplateOption = {
  key: string
  label: string
}

type StoryboardTemplateMeta = {
  label: string
  hint: string
  sceneLines: readonly string[]
  dialogueLines: readonly string[]
  referenceLines: readonly string[]
}

type BrandVoiceTemplateOption = {
  key: string
  label: string
}

type BrandVoiceTemplateMeta = {
  label: string
  hint: string
  contrast: string
}

type AdReviewSectionProps = {
  reviewReady: boolean
  reviewConfirmed: boolean
  adReviewChecklist: readonly ReviewChecklistItem[]
  blockingReviewItems: readonly ReviewChecklistItem[]
  advisoryReviewItems: readonly ReviewChecklistItem[]
  onReviewConfirmedChange: (checked: boolean) => void
  storyboardTemplates: readonly StoryboardTemplateOption[]
  selectedStoryboardTemplate: string
  selectedStoryboardTemplateMeta: StoryboardTemplateMeta
  onSelectStoryboardTemplate: (key: string) => void
  brandVoiceTemplates: readonly BrandVoiceTemplateOption[]
  selectedBrandVoiceTemplate: string
  selectedBrandVoiceLabel: string
  onSelectBrandVoiceTemplate: (key: string) => void
  optimizedScript: string
  adPrompt: string
  brandVoiceBrief: string
  brandVoiceNotesText: string
  onBrandVoiceNotesTextChange: (value: string) => void
  selectedBrandVoiceTemplateMeta: BrandVoiceTemplateMeta
  storyboardEditor: ReactNode
}

export function AdReviewSection({
  reviewReady,
  reviewConfirmed,
  adReviewChecklist,
  blockingReviewItems,
  advisoryReviewItems,
  onReviewConfirmedChange,
  storyboardTemplates,
  selectedStoryboardTemplate,
  selectedStoryboardTemplateMeta,
  onSelectStoryboardTemplate,
  brandVoiceTemplates,
  selectedBrandVoiceTemplate,
  selectedBrandVoiceLabel,
  onSelectBrandVoiceTemplate,
  optimizedScript,
  adPrompt,
  brandVoiceBrief,
  brandVoiceNotesText,
  onBrandVoiceNotesTextChange,
  selectedBrandVoiceTemplateMeta,
  storyboardEditor,
}: AdReviewSectionProps) {
  return (
    <div className="space-y-4 rounded-xl border border-surface-200 bg-white p-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div>
          <p className="text-sm font-medium text-surface-800">本地审核与分镜预览</p>
          <p className="text-xs text-surface-500">先检查广告文案、素材、台词和分镜，再勾选确认后生成。编辑卡片会同步回上方文本区。</p>
        </div>
        <span className={[
          'rounded-full border px-2.5 py-1 text-[11px] font-medium',
          reviewReady
            ? 'border-emerald-200 bg-emerald-50 text-emerald-700'
            : reviewConfirmed
              ? 'border-amber-200 bg-amber-50 text-amber-700'
              : 'border-surface-200 bg-surface-50 text-surface-600',
        ].join(' ')}>
          {reviewReady ? '已确认，可生成' : reviewConfirmed ? '等待补全项' : '等待审核确认'}
        </span>
      </div>

      <div className="space-y-3 rounded-xl border border-cyan-100 bg-cyan-50/40 p-4">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div>
            <p className="text-sm font-medium text-cyan-900">分镜模板库</p>
            <p className="text-xs text-cyan-700">一键套用镜头结构、台词节奏和参考图提示，再在卡片里做局部微调。</p>
          </div>
          <span className="rounded-full border border-cyan-200 bg-white px-2.5 py-1 text-[11px] font-medium text-cyan-700">
            当前：{selectedStoryboardTemplateMeta.label}
          </span>
        </div>

        <div className="flex flex-wrap gap-2">
          {storyboardTemplates.map((template) => {
            const active = selectedStoryboardTemplate === template.key
            return (
              <button
                key={template.key}
                type="button"
                onClick={() => onSelectStoryboardTemplate(template.key)}
                className={[
                  'rounded-full border px-3 py-1.5 text-xs font-medium transition-colors',
                  active
                    ? 'border-cyan-300 bg-cyan-50 text-cyan-800'
                    : 'border-surface-200 bg-white text-surface-600 hover:border-cyan-200 hover:bg-cyan-50/40',
                ].join(' ')}
              >
                {template.label}
              </button>
            )
          })}
        </div>

        <div className="rounded-lg border border-cyan-100 bg-white px-3 py-3 text-xs text-cyan-700">
          <p className="font-medium text-cyan-900">{selectedStoryboardTemplateMeta.label}</p>
          <p className="mt-1 leading-5">{selectedStoryboardTemplateMeta.hint}</p>
          <p className="mt-2 text-[11px] leading-5 text-cyan-600">
            场景建议 {selectedStoryboardTemplateMeta.sceneLines.length} 条 · 台词建议 {selectedStoryboardTemplateMeta.dialogueLines.length} 条 · 参考图建议 {selectedStoryboardTemplateMeta.referenceLines.length} 条
          </p>
        </div>
      </div>

      {storyboardEditor}

      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
        {adReviewChecklist.map((item) => (
          <div
            key={item.key}
            className={[
              'rounded-xl border p-3',
              item.passed
                ? 'border-emerald-200 bg-emerald-50/60'
                : item.blocking
                  ? 'border-rose-200 bg-rose-50/60'
                  : 'border-amber-200 bg-amber-50/60',
            ].join(' ')}
          >
            <div className="flex items-center justify-between gap-2">
              <p className="text-xs font-medium text-surface-800">{item.label}</p>
              <span className={[
                'rounded-full border px-2 py-0.5 text-[11px] font-medium',
                item.passed
                  ? 'border-emerald-200 bg-white/80 text-emerald-700'
                  : item.blocking
                    ? 'border-rose-200 bg-white/80 text-rose-700'
                    : 'border-amber-200 bg-white/80 text-amber-700',
              ].join(' ')}>
                {item.passed ? '通过' : item.blocking ? '缺失' : '待优化'}
              </span>
            </div>
            <p className="mt-2 text-xs leading-5 text-surface-600">{item.detail}</p>
          </div>
        ))}
      </div>

      {blockingReviewItems.length > 0 ? (
        <p className="text-xs text-rose-600">先补全：{blockingReviewItems.map((item) => item.label).join('、')}</p>
      ) : advisoryReviewItems.length > 0 ? (
        <p className="text-xs text-amber-600">建议先优化：{advisoryReviewItems.map((item) => item.label).join('、')}</p>
      ) : (
        <p className="text-xs text-emerald-600">当前审核项已满足，可以进入生成。</p>
      )}

      <div className="flex items-start gap-3 rounded-xl border border-cyan-200 bg-cyan-50/60 p-3">
        <Switch checked={reviewConfirmed} onCheckedChange={onReviewConfirmedChange} />
        <div className="space-y-1">
          <p className="text-sm font-medium text-cyan-900">我已确认市场、台词和分镜无误</p>
          <p className="text-xs text-cyan-700">勾选后才能提交生成；修改任意输入后会自动取消确认。</p>
        </div>
      </div>

      <div className="space-y-3 rounded-xl border border-violet-100 bg-violet-50/40 p-4">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div>
            <p className="text-sm font-medium text-violet-900">品牌语气模板</p>
            <p className="text-xs text-violet-700">模板会进入文案优化和视频生成提示词，并影响当前文案的表达方向。</p>
          </div>
          <span className="rounded-full border border-violet-200 bg-white px-2.5 py-1 text-[11px] font-medium text-violet-700">
            当前：{selectedBrandVoiceLabel}
          </span>
        </div>

        <div className="flex flex-wrap gap-2">
          {brandVoiceTemplates.map((template) => {
            const active = selectedBrandVoiceTemplate === template.key
            return (
              <button
                key={template.key}
                type="button"
                onClick={() => onSelectBrandVoiceTemplate(template.key)}
                className={[
                  'rounded-full border px-3 py-1.5 text-xs font-medium transition-colors',
                  active
                    ? 'border-violet-300 bg-violet-50 text-violet-800'
                    : 'border-surface-200 bg-white text-surface-600 hover:border-violet-200 hover:bg-violet-50/40',
                ].join(' ')}
              >
                {template.label}
              </button>
            )
          })}
        </div>

        <div className="grid gap-3 md:grid-cols-2">
          <div className="rounded-lg border border-surface-200 bg-white p-3">
            <p className="text-xs font-medium text-surface-700">切换前预览</p>
            <div className="mt-2 max-h-36 overflow-y-auto whitespace-pre-wrap text-xs leading-5 text-surface-600">
              {optimizedScript.trim() || adPrompt.trim() || '请先填写广告文案，以便对比品牌语气模板。'}
            </div>
          </div>
          <div className="rounded-lg border border-violet-200 bg-white p-3">
            <p className="text-xs font-medium text-violet-800">切换后预览</p>
            <div className="mt-2 max-h-36 overflow-y-auto whitespace-pre-wrap text-xs leading-5 text-violet-700">
              {brandVoiceBrief}
            </div>
          </div>
        </div>

        <div className="space-y-1">
          <Label className="text-xs text-violet-800">品牌语气补充说明</Label>
          <Textarea
            rows={3}
            value={brandVoiceNotesText}
            onChange={(event) => onBrandVoiceNotesTextChange(event.target.value)}
            placeholder="例如：更克制、更高级，不要太热闹；品牌口径要统一；避免过度促销腔。"
          />
          <p className="text-[11px] leading-5 text-violet-600">这里写更细的口吻要求，模板会保留，你可以只改局部表达。</p>
        </div>

        <div className="rounded-lg border border-violet-200 bg-violet-50/60 px-3 py-2 text-xs text-violet-700">
          <p className="font-medium text-violet-900">{selectedBrandVoiceTemplateMeta.label}</p>
          <p className="mt-1 leading-5">{selectedBrandVoiceTemplateMeta.hint}</p>
          <p className="mt-2 text-[11px] leading-5 text-violet-600">{selectedBrandVoiceTemplateMeta.contrast}</p>
        </div>
      </div>
    </div>
  )
}
