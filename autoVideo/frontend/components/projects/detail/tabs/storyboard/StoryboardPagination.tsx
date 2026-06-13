'use client'

import { ChevronLeft, ChevronRight } from 'lucide-react'
import { Button } from '@/components/ui/button'

export function StoryboardPagination({
  sbPage,
  sbTotalPages,
  sbTotal,
  onPageChange,
}: {
  sbPage: number
  sbTotalPages: number
  sbTotal: number
  onPageChange: (page: number) => void
}) {
  if (sbTotalPages <= 1) return null

  return (
    <div className="mt-4 flex items-center justify-center gap-3">
      <Button size="sm" variant="outline" disabled={sbPage <= 1} onClick={() => onPageChange(sbPage - 1)} title="上一页分镜">
        <ChevronLeft className="mr-1 h-4 w-4" /> 上一页
      </Button>
      <span className="text-sm text-surface-600">
        第 {sbPage} / {sbTotalPages} 页（共 {sbTotal} 条）
      </span>
      <Button size="sm" variant="outline" disabled={sbPage >= sbTotalPages} onClick={() => onPageChange(sbPage + 1)} title="下一页分镜">
        下一页 <ChevronRight className="ml-1 h-4 w-4" />
      </Button>
    </div>
  )
}
