'use client'

import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'

type ScriptPreviewDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  scriptFileName: string
  scriptText: string
}

export function ScriptPreviewDialog({
  open,
  onOpenChange,
  scriptFileName,
  scriptText,
}: ScriptPreviewDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-5xl">
        <DialogHeader>
          <DialogTitle>剧本全文</DialogTitle>
        </DialogHeader>
        <div className="space-y-3">
          <div className="flex flex-wrap items-center justify-between gap-2 text-xs text-surface-500">
            <span>{scriptFileName}</span>
            <span>{scriptText.length} 字</span>
          </div>
          <div className="max-h-[70vh] overflow-auto rounded-lg border bg-surface-50 p-4">
            <pre className="whitespace-pre-wrap break-words text-sm leading-6 text-surface-700">
              {scriptText}
            </pre>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  )
}
