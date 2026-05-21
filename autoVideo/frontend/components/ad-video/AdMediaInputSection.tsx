import { Upload } from 'lucide-react'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'

type AdMediaInputSectionProps = {
  imageUrlsText: string
  onImageUrlsTextChange: (value: string) => void
  imageUrlCount: number
  sceneDescriptionsText: string
  onSceneDescriptionsTextChange: (value: string) => void
  localFiles: readonly File[]
  onLocalFilesChange: (files: FileList | null) => void
  enableLocalCompression: boolean
  onEnableLocalCompressionChange: (checked: boolean) => void
  maxImageSide: number
  onMaxImageSideChange: (value: number) => void
  jpegQuality: number
  onJpegQualityChange: (value: number) => void
  onRemoveLocalFile: (index: number) => void
}

export function AdMediaInputSection({
  imageUrlsText,
  onImageUrlsTextChange,
  imageUrlCount,
  sceneDescriptionsText,
  onSceneDescriptionsTextChange,
  localFiles,
  onLocalFilesChange,
  enableLocalCompression,
  onEnableLocalCompressionChange,
  maxImageSide,
  onMaxImageSideChange,
  jpegQuality,
  onJpegQualityChange,
  onRemoveLocalFile,
}: AdMediaInputSectionProps) {
  return (
    <>
      <div className="grid gap-6 lg:grid-cols-2">
        <div className="space-y-2">
          <Label htmlFor="image-urls" className="flex items-center gap-2">
            指定图片 URL（每行一个）
          </Label>
          <Textarea
            id="image-urls"
            rows={8}
            placeholder={'https://cdn.example.com/ad-shot-1.jpg\nhttps://cdn.example.com/ad-shot-2.jpg'}
            value={imageUrlsText}
            onChange={(event) => onImageUrlsTextChange(event.target.value)}
          />
          <p className="text-xs text-surface-500">已识别 {imageUrlCount} 张图片 URL</p>
        </div>

        <div className="space-y-2">
          <Label htmlFor="scene-lines">分镜描述（可选，每行对应一张图）</Label>
          <Textarea
            id="scene-lines"
            rows={8}
            placeholder={'开场特写：冰块与饮料碰撞，突出清凉感\n中景：年轻人聚会举杯，传达社交氛围'}
            value={sceneDescriptionsText}
            onChange={(event) => onSceneDescriptionsTextChange(event.target.value)}
          />
          <p className="text-xs text-surface-500">未填写时会默认使用广告文案作为场景描述。</p>
        </div>
      </div>

      <div className="space-y-3 rounded-xl border border-surface-200 bg-white p-4">
        <div className="flex items-center justify-between gap-3">
          <div>
            <p className="text-sm font-medium text-surface-800">本地图片上传与处理</p>
            <p className="text-xs text-surface-500">支持上传本地图后自动压缩并写入项目，再参与广告片段生成。</p>
          </div>
          <label className="inline-flex cursor-pointer items-center gap-2 rounded-lg border border-surface-200 bg-surface-50 px-3 py-2 text-xs font-medium text-surface-700 hover:bg-surface-100">
            <Upload className="h-3.5 w-3.5" />
            上传图片
            <input
              type="file"
              accept="image/*"
              multiple
              className="hidden"
              onChange={(event) => onLocalFilesChange(event.target.files)}
            />
          </label>
        </div>

        <div className="grid gap-4 md:grid-cols-3">
          <div className="flex items-center gap-2 text-xs text-surface-600">
            <Switch checked={enableLocalCompression} onCheckedChange={onEnableLocalCompressionChange} />
            上传前压缩处理
          </div>
          <div className="space-y-1">
            <Label htmlFor="max-side" className="text-xs text-surface-500">最长边（px）</Label>
            <Input
              id="max-side"
              type="number"
              min={640}
              max={4096}
              value={maxImageSide}
              onChange={(event) => onMaxImageSideChange(Number(event.target.value || 1920))}
            />
          </div>
          <div className="space-y-1">
            <Label htmlFor="jpeg-quality" className="text-xs text-surface-500">JPEG 质量（1-100）</Label>
            <Input
              id="jpeg-quality"
              type="number"
              min={1}
              max={100}
              value={jpegQuality}
              onChange={(event) => onJpegQualityChange(Number(event.target.value || 88))}
            />
          </div>
        </div>

        {localFiles.length > 0 ? (
          <div className="space-y-2">
            <p className="text-xs text-surface-500">已添加 {localFiles.length} 张本地图片</p>
            <div className="max-h-40 space-y-1 overflow-y-auto rounded-lg border border-surface-200 bg-surface-50 p-2">
              {localFiles.map((file, idx) => (
                <div key={`${file.name}-${idx}`} className="flex items-center justify-between gap-2 rounded-md bg-white px-2 py-1 text-xs">
                  <span className="truncate text-surface-700">{file.name}</span>
                  <button
                    type="button"
                    className="text-rose-500 hover:text-rose-600"
                    onClick={() => onRemoveLocalFile(idx)}
                  >
                    删除
                  </button>
                </div>
              ))}
            </div>
          </div>
        ) : null}
      </div>
    </>
  )
}
