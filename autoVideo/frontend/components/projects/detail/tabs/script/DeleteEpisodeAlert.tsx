'use client'

import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import type { Episode } from '@/types'

type DeleteEpisodeAlertProps = {
  episodeDeleteTarget: Episode | null
  deletingEpisodeId: number | null
  onOpenChange: (open: boolean) => void
  onConfirm: () => void
}

export function DeleteEpisodeAlert({
  episodeDeleteTarget,
  deletingEpisodeId,
  onOpenChange,
  onConfirm,
}: DeleteEpisodeAlertProps) {
  return (
    <AlertDialog open={!!episodeDeleteTarget} onOpenChange={onOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>确认删除分集</AlertDialogTitle>
          <AlertDialogDescription>
            确定要删除第 {episodeDeleteTarget?.episode_number} 集「{episodeDeleteTarget?.title}」吗？此操作不可恢复，该集的分镜等关联数据也将被删除。
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel disabled={!!deletingEpisodeId}>取消</AlertDialogCancel>
          <AlertDialogAction
            className="bg-red-500 hover:bg-red-600"
            disabled={!!deletingEpisodeId}
            onClick={onConfirm}
          >
            {deletingEpisodeId ? '删除中...' : '确认删除'}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}
