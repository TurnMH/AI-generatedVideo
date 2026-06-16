'use client'

import { Download, X } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { SyncedClipPlayer } from '@/components/projects/detail/SyncedClipPlayer'

export type ClipPreviewState = {
  videoUrl: string
  audioUrl?: string
  videoDurationSec?: number
  audioDurationSec?: number
  title?: string
  staleHint?: string | null
}

type ClipPreviewModalProps = {
  preview: ClipPreviewState
  onClose: () => void
}

export function ClipPreviewModal({ preview, onClose }: ClipPreviewModalProps) {
  const hasSyncedAudio = Boolean(preview.audioUrl)

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/80" onClick={onClose}>
      <div className="relative mx-4 w-full max-w-4xl" onClick={(e) => e.stopPropagation()}>
        <div className="mb-3 flex items-center justify-between">
          <span className="text-sm text-white/70">
            {hasSyncedAudio ? '音画同步预览' : '视频预览'}
            {preview.title ? ` · ${preview.title}` : ''}
          </span>
          <div className="flex items-center gap-2">
            {!hasSyncedAudio && (
              <a
                href={preview.videoUrl}
                download
                target="_blank"
                rel="noopener noreferrer"
                className="flex items-center gap-1.5 rounded-md bg-white/10 px-3 py-1.5 text-xs text-white transition-colors hover:bg-white/20"
              >
                <Download className="h-3.5 w-3.5" /> 下载视频
              </a>
            )}
            <Button size="sm" variant="ghost" className="text-white hover:bg-white/10" onClick={onClose} title="关闭预览">
              <X className="h-4 w-4" />
            </Button>
          </div>
        </div>
        <div className="overflow-hidden rounded-lg bg-black shadow-2xl">
          {hasSyncedAudio && preview.audioUrl ? (
            <div className="p-4">
              <SyncedClipPlayer
                videoUrl={preview.videoUrl}
                audioUrl={preview.audioUrl}
                videoDurationSec={preview.videoDurationSec}
                audioDurationSec={preview.audioDurationSec}
                staleHint={preview.staleHint}
                label="播放"
              />
            </div>
          ) : (
            <video className="max-h-[80vh] w-full" controls autoPlay key={preview.videoUrl}>
              <source src={preview.videoUrl} type="video/mp4" />
              您的浏览器不支持视频播放
            </video>
          )}
        </div>
      </div>
    </div>
  )
}
