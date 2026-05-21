import Link from 'next/link'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'

type GuidanceOption = {
  key: string
  label: string
}

type AdMarketGuidanceSectionProps = {
  targetMarketOptions: readonly GuidanceOption[]
  targetMarket: string
  onTargetMarketChange: (value: string) => void
  subtitleLanguageOptions: readonly GuidanceOption[]
  subtitleLanguage: string
  onSubtitleLanguageChange: (value: string) => void
  creativeModeOptions: readonly GuidanceOption[]
  creativeMode: string
  onCreativeModeChange: (value: string) => void
  subtitleText: string
  onSubtitleTextChange: (value: string) => void
  subtitleLineCount: number
  directorNote: string
  onDirectorNoteChange: (value: string) => void
}

export function AdMarketGuidanceSection({
  targetMarketOptions,
  targetMarket,
  onTargetMarketChange,
  subtitleLanguageOptions,
  subtitleLanguage,
  onSubtitleLanguageChange,
  creativeModeOptions,
  creativeMode,
  onCreativeModeChange,
  subtitleText,
  onSubtitleTextChange,
  subtitleLineCount,
  directorNote,
  onDirectorNoteChange,
}: AdMarketGuidanceSectionProps) {
  return (
    <>
      <div className="space-y-3 rounded-xl border border-surface-200 bg-white p-4">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div>
            <p className="text-sm font-medium text-surface-800">市场 / 台词 / 导演备注</p>
            <p className="text-xs text-surface-500">这些设置会进入文案优化、字幕烧录和视频生成提示词，直接影响市场匹配和口播一致性。</p>
          </div>
          <span className="rounded-full border border-cyan-200 bg-cyan-50 px-2.5 py-1 text-[11px] font-medium text-cyan-700">
            指导式生成
          </span>
        </div>

        <div className="grid gap-4 md:grid-cols-3">
          <div className="space-y-1">
            <Label className="text-xs text-surface-700">目标市场</Label>
            <Select value={targetMarket} onValueChange={onTargetMarketChange}>
              <SelectTrigger>
                <SelectValue placeholder="选择市场" />
              </SelectTrigger>
              <SelectContent>
                {targetMarketOptions.map((option) => (
                  <SelectItem key={option.key} value={option.key}>
                    {option.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-1">
            <Label className="text-xs text-surface-700">字幕语言</Label>
            <Select value={subtitleLanguage} onValueChange={onSubtitleLanguageChange}>
              <SelectTrigger>
                <SelectValue placeholder="选择字幕语言" />
              </SelectTrigger>
              <SelectContent>
                {subtitleLanguageOptions.map((option) => (
                  <SelectItem key={option.key} value={option.key}>
                    {option.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-1">
            <Label className="text-xs text-surface-700">创意模式</Label>
            <Select value={creativeMode} onValueChange={onCreativeModeChange}>
              <SelectTrigger>
                <SelectValue placeholder="选择创意模式" />
              </SelectTrigger>
              <SelectContent>
                {creativeModeOptions.map((option) => (
                  <SelectItem key={option.key} value={option.key}>
                    {option.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </div>

        <div className="grid gap-4 md:grid-cols-2">
          <div className="space-y-2">
            <Label htmlFor="subtitle-text">字幕 / 口播台词</Label>
            <Textarea
              id="subtitle-text"
              rows={7}
              placeholder="每行一句，生成时会自动分配到各镜头；支持中文、英文或中英双语。"
              value={subtitleText}
              onChange={(event) => onSubtitleTextChange(event.target.value)}
            />
            <p className="text-xs text-surface-500">
              当前已识别 {subtitleLineCount} 条台词；生成时会同步进入字幕和支持原生音频的模型输入。
            </p>
          </div>

          <div className="space-y-2">
            <Label htmlFor="director-note">导演备注 / 禁止项</Label>
            <Textarea
              id="director-note"
              rows={7}
              placeholder="例如：不要把品牌卖点改掉；前 5 秒必须交代市场利益点；字幕必须跟口播逐句对应。"
              value={directorNote}
              onChange={(event) => onDirectorNoteChange(event.target.value)}
            />
            <div className="rounded-lg border border-surface-200 bg-surface-50 px-3 py-2 text-xs text-surface-600">
              当前市场约束会自动进入文案优化与视频生成提示词，避免导演式自动改写把市场带偏。
            </div>
          </div>
        </div>
      </div>

      <div className="flex items-center justify-between rounded-xl border border-cyan-100 bg-cyan-50/40 p-4">
        <div>
          <p className="text-sm font-medium text-cyan-900">需要从已有视频提取文案？</p>
          <p className="mt-1 text-xs text-cyan-700">可使用视频工具区，自动转写本地或在线视频的画面和解说，再复制回来使用。</p>
        </div>
        <Button asChild variant="outline" size="sm" className="h-8 border-cyan-200 text-cyan-800 hover:bg-cyan-100 hover:text-cyan-900">
          <Link href="/tools/video">去工具区提取 &raquo;</Link>
        </Button>
      </div>
    </>
  )
}
