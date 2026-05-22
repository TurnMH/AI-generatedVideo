import { CheckCircle2, Clapperboard, ImageIcon, Loader2, Wand2 } from 'lucide-react'
import { Button } from '@/components/ui/button'

type AdPrimaryActionsSectionProps = {
  creatingByText: boolean
  preparingProject: boolean
  generatingStoryboard: boolean
  submittingVideo: boolean
  composingVideo: boolean
  reviewReady: boolean
  projectReady: boolean
  storyboardReady: boolean
  videoReady: boolean
  onCreateFromText: () => void
  onPrepareProject: () => void
  onGenerateStoryboard: () => void
  onGenerateVideo: () => void
  onComposeVideo: () => void
}

export function AdPrimaryActionsSection({
  creatingByText,
  preparingProject,
  generatingStoryboard,
  submittingVideo,
  composingVideo,
  reviewReady,
  projectReady,
  storyboardReady,
  videoReady,
  onCreateFromText,
  onPrepareProject,
  onGenerateStoryboard,
  onGenerateVideo,
  onComposeVideo,
}: AdPrimaryActionsSectionProps) {
  const busy = creatingByText || preparingProject || generatingStoryboard || submittingVideo || composingVideo

  return (
    <div className="space-y-3">
      <div className="grid gap-3 lg:grid-cols-4">
        <Button type="button" onClick={onCreateFromText} disabled={busy} className="h-11 gap-2">
          {creatingByText ? <Loader2 className="h-4 w-4 animate-spin" /> : <Wand2 className="h-4 w-4" />}
          保存草稿
        </Button>

        <Button type="button" variant="outline" onClick={onPrepareProject} disabled={busy} className="h-11 gap-2">
          {preparingProject ? <Loader2 className="h-4 w-4 animate-spin" /> : <CheckCircle2 className="h-4 w-4" />}
          1. 准备项目
        </Button>

        <Button type="button" variant="outline" onClick={onGenerateStoryboard} disabled={busy} className="h-11 gap-2 border-cyan-200 bg-cyan-50 text-cyan-800 hover:bg-cyan-100">
          {generatingStoryboard ? <Loader2 className="h-4 w-4 animate-spin" /> : <ImageIcon className="h-4 w-4" />}
          2. 手动生成图片
        </Button>

        <Button type="button" variant="outline" onClick={onGenerateVideo} disabled={busy} className="h-11 gap-2 border-violet-200 bg-violet-50 text-violet-800 hover:bg-violet-100">
          {submittingVideo ? <Loader2 className="h-4 w-4 animate-spin" /> : <Clapperboard className="h-4 w-4" />}
          3. 手动生成视频
        </Button>
      </div>

      <div className="grid gap-3 lg:grid-cols-[minmax(0,1fr)_220px]">
        <div className="rounded-xl border border-surface-200 bg-surface-50 px-4 py-3 text-xs text-surface-600">
          <p className="font-medium text-surface-800">当前改为手动三段式流程</p>
          <p className="mt-1 leading-5">
            先优化/确认分镜文案，再手动点“准备项目”→“生成图片”→“生成视频”，最后再单独点“合成”。
          </p>
          <p className="mt-2 text-[11px] text-surface-500">
            状态：审核 {reviewReady ? '已通过' : '未完成'} · 项目 {projectReady ? '已准备' : '未准备'} · 分镜图 {storyboardReady ? '已触发/已就绪' : '未生成'} · 视频任务 {videoReady ? '已创建' : '未创建'}
          </p>
        </div>

        <Button type="button" onClick={onComposeVideo} disabled={busy} className="h-11 gap-2 bg-emerald-600 text-white hover:bg-emerald-700">
          {composingVideo ? <Loader2 className="h-4 w-4 animate-spin" /> : <CheckCircle2 className="h-4 w-4" />}
          4. 手动点合成
        </Button>
      </div>
    </div>
  )
}
