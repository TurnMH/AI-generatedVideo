import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'

type TemplateOption = {
  key: string
  label: string
}

type VideoModelOption = {
  id: number
  name: string
  model_key: string
}

type StyleOption = {
  key: string
  label: string
}

type MotionOption = {
  key: string
  label: string
}

type AdAdvancedSettingsSectionProps = {
  templates: readonly TemplateOption[]
  selectedTemplate: string
  onSelectTemplate: (key: string) => void
  availableVideoModels: readonly VideoModelOption[]
  selectedVideoModel: string
  onSelectVideoModel: (value: string) => void
  styleOptions: readonly StyleOption[]
  selectedStylePreset: string
  onSelectStylePreset: (value: string) => void
  motionOptions: readonly MotionOption[]
  selectedMotionMode: string
  onSelectMotionMode: (value: string) => void
  clipDurationSec: number
  onClipDurationSecChange: (value: number) => void
  selectedVideoMode: 'frame_animation' | 'api_generation'
  onSelectVideoMode: (value: 'frame_animation' | 'api_generation') => void
  autoAvoidLowHourEnabled: boolean
  onAutoAvoidLowHourEnabledChange: (checked: boolean) => void
  lowHourThreshold: number
  onLowHourThresholdChange: (value: number) => void
  autoRetryEnabled: boolean
  onAutoRetryEnabledChange: (checked: boolean) => void
}

export function AdAdvancedSettingsSection({
  templates,
  selectedTemplate,
  onSelectTemplate,
  availableVideoModels,
  selectedVideoModel,
  onSelectVideoModel,
  styleOptions,
  selectedStylePreset,
  onSelectStylePreset,
  motionOptions,
  selectedMotionMode,
  onSelectMotionMode,
  clipDurationSec,
  onClipDurationSecChange,
  selectedVideoMode,
  onSelectVideoMode,
  autoAvoidLowHourEnabled,
  onAutoAvoidLowHourEnabledChange,
  lowHourThreshold,
  onLowHourThresholdChange,
  autoRetryEnabled,
  onAutoRetryEnabledChange,
}: AdAdvancedSettingsSectionProps) {
  return (
    <div className="space-y-3 rounded-xl border border-surface-200 bg-white p-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div>
          <p className="text-sm font-medium text-surface-800">广告模板与高级参数</p>
          <p className="text-xs text-surface-500">模板可快速套用投放场景，参数会直接影响最终视频生成质量与风格。</p>
        </div>
      </div>

      <div className="flex flex-wrap gap-2">
        {templates.map((template) => {
          const active = selectedTemplate === template.key
          return (
            <button
              key={template.key}
              type="button"
              onClick={() => onSelectTemplate(template.key)}
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

      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-5">
        <div className="space-y-1 xl:col-span-2">
          <Label className="text-xs text-surface-700">视频模型</Label>
          <Select value={selectedVideoModel} onValueChange={onSelectVideoModel}>
            <SelectTrigger>
              <SelectValue placeholder="选择视频模型" />
            </SelectTrigger>
            <SelectContent>
              {availableVideoModels.length > 0 ? (
                availableVideoModels.map((model) => (
                  <SelectItem key={model.id} value={model.model_key}>
                    {model.name}
                  </SelectItem>
                ))
              ) : (
                <SelectItem value="__no_model__" disabled>暂无可用模型</SelectItem>
              )}
            </SelectContent>
          </Select>
        </div>

        <div className="space-y-1">
          <Label className="text-xs text-surface-700">风格</Label>
          <Select value={selectedStylePreset} onValueChange={onSelectStylePreset}>
            <SelectTrigger>
              <SelectValue placeholder="选择风格" />
            </SelectTrigger>
            <SelectContent>
              {styleOptions.map((style) => (
                <SelectItem key={style.key} value={style.key}>
                  {style.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        <div className="space-y-1">
          <Label className="text-xs text-surface-700">运镜</Label>
          <Select value={selectedMotionMode} onValueChange={onSelectMotionMode}>
            <SelectTrigger>
              <SelectValue placeholder="选择运镜" />
            </SelectTrigger>
            <SelectContent>
              {motionOptions.map((motion) => (
                <SelectItem key={motion.key} value={motion.key}>
                  {motion.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        <div className="space-y-1">
          <Label className="text-xs text-surface-700">片段时长（秒）</Label>
          <Input
            type="number"
            min={2}
            max={10}
            value={clipDurationSec}
            onChange={(event) => onClipDurationSecChange(Number(event.target.value || 5))}
          />
        </div>
      </div>

      <div className="grid gap-4 md:grid-cols-2">
        <div className="space-y-1">
          <Label className="text-xs text-surface-700">生成模式</Label>
          <Select value={selectedVideoMode} onValueChange={(value) => onSelectVideoMode(value as 'frame_animation' | 'api_generation')}>
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="frame_animation">frame_animation</SelectItem>
              <SelectItem value="api_generation">api_generation</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div className="rounded-lg border border-surface-200 bg-surface-50 p-3 text-xs text-surface-600">
          当前参数会用于：项目默认 storyboard_config + 本次 videoAPI.generate 请求。
        </div>
      </div>

      <div className="grid gap-4 md:grid-cols-2">
        <div className="flex items-center gap-2 text-xs text-surface-600">
          <Switch checked={autoAvoidLowHourEnabled} onCheckedChange={onAutoAvoidLowHourEnabledChange} />
          低成功率时段自动切换推荐/备选模型
        </div>
        <div className="space-y-1">
          <Label className="text-xs text-surface-700">低成功率阈值（%）</Label>
          <Input
            type="number"
            min={20}
            max={95}
            value={lowHourThreshold}
            onChange={(event) => onLowHourThresholdChange(Number(event.target.value || 65))}
          />
        </div>
      </div>

      <div className="flex items-center gap-2 text-xs text-surface-600">
        <Switch checked={autoRetryEnabled} onCheckedChange={onAutoRetryEnabledChange} />
        失败时自动切换备用模型重试（最多 1 次）
      </div>
    </div>
  )
}
