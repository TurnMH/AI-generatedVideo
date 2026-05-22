import { ArrowRight, ImageIcon, Loader2, Wand2 } from 'lucide-react'
import { Button } from '@/components/ui/button'

type AdPrimaryActionsSectionProps = {
  creatingByText: boolean
  creatingByImages: boolean
  reviewReady: boolean
  onCreateFromText: () => void
  onGenerateByImages: () => void
}

export function AdPrimaryActionsSection({
  creatingByText,
  creatingByImages,
  reviewReady,
  onCreateFromText,
  onGenerateByImages,
}: AdPrimaryActionsSectionProps) {
  return (
    <div className="grid gap-3 sm:grid-cols-2">
      <Button
        type="button"
        onClick={onCreateFromText}
        disabled={creatingByText || creatingByImages}
        className="h-11 gap-2"
      >
        {creatingByText ? <Loader2 className="h-4 w-4 animate-spin" /> : <Wand2 className="h-4 w-4" />}
        保存草稿并继续编辑
      </Button>
      <div className="space-y-1">
        <Button
          type="button"
          variant="outline"
          onClick={onGenerateByImages}
          disabled={creatingByImages || creatingByText}
          className="h-11 w-full gap-2 border-cyan-200 bg-cyan-50 text-cyan-800 hover:bg-cyan-100"
        >
          {creatingByImages ? <Loader2 className="h-4 w-4 animate-spin" /> : <ImageIcon className="h-4 w-4" />}
          审核通过后异步生成
          <ArrowRight className="h-4 w-4" />
        </Button>
        {!reviewReady ? (
          <p className="text-xs text-amber-600">当前仍有未确认或未补全项；点击后会直接提示具体缺口，不再无反应。</p>
        ) : (
          <p className="text-xs text-emerald-600">审核条件已满足，可直接提交异步生成。</p>
        )}
      </div>
    </div>
  )
}
