'use client'

import { useCallback, useEffect, useRef, useState } from 'react'
import { Download, Pause, Play } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { formatAvDurationDriftHint } from '@/lib/projects/storyboard-av-preview'

type SyncedClipPlayerProps = {
  videoUrl: string
  audioUrl: string
  videoDurationSec?: number
  audioDurationSec?: number
  className?: string
  compact?: boolean
  showDownload?: boolean
  label?: string
  staleHint?: string | null
}

const DRIFT_THRESHOLD_SEC = 0.15

export function SyncedClipPlayer({
  videoUrl,
  audioUrl,
  videoDurationSec,
  audioDurationSec,
  className,
  compact = false,
  showDownload = true,
  label = '预览本镜',
  staleHint,
}: SyncedClipPlayerProps) {
  const videoRef = useRef<HTMLVideoElement>(null)
  const audioRef = useRef<HTMLAudioElement>(null)
  const [playing, setPlaying] = useState(false)
  const driftHint = formatAvDurationDriftHint(videoDurationSec, audioDurationSec)

  const syncAudioToVideo = useCallback((video: HTMLVideoElement, audio: HTMLAudioElement) => {
    if (Math.abs(video.currentTime - audio.currentTime) > DRIFT_THRESHOLD_SEC) {
      audio.currentTime = video.currentTime
    }
  }, [])

  const handlePlay = useCallback(async () => {
    const video = videoRef.current
    const audio = audioRef.current
    if (!video || !audio) return

    if (playing) {
      video.pause()
      audio.pause()
      setPlaying(false)
      return
    }

    audio.currentTime = video.currentTime
    try {
      await Promise.all([video.play(), audio.play()])
      setPlaying(true)
    } catch {
      setPlaying(false)
    }
  }, [playing])

  useEffect(() => {
    const video = videoRef.current
    const audio = audioRef.current
    if (!video || !audio) return

    const onVideoPlay = () => {
      audio.currentTime = video.currentTime
      void audio.play().catch(() => {})
      setPlaying(true)
    }
    const onVideoPause = () => {
      audio.pause()
      setPlaying(false)
    }
    const onVideoSeeked = () => {
      audio.currentTime = video.currentTime
    }
    const onTimeUpdate = () => {
      if (!video.paused) syncAudioToVideo(video, audio)
    }
    const onEnded = () => {
      video.pause()
      audio.pause()
      setPlaying(false)
    }

    video.addEventListener('play', onVideoPlay)
    video.addEventListener('pause', onVideoPause)
    video.addEventListener('seeked', onVideoSeeked)
    video.addEventListener('timeupdate', onTimeUpdate)
    video.addEventListener('ended', onEnded)
    audio.addEventListener('ended', onEnded)

    return () => {
      video.removeEventListener('play', onVideoPlay)
      video.removeEventListener('pause', onVideoPause)
      video.removeEventListener('seeked', onVideoSeeked)
      video.removeEventListener('timeupdate', onTimeUpdate)
      video.removeEventListener('ended', onEnded)
      audio.removeEventListener('ended', onEnded)
    }
  }, [syncAudioToVideo])

  useEffect(() => {
    setPlaying(false)
    if (videoRef.current) videoRef.current.pause()
    if (audioRef.current) audioRef.current.pause()
  }, [videoUrl, audioUrl])

  return (
    <div className={className}>
      <div className={`relative overflow-hidden rounded-lg bg-black ${compact ? '' : 'shadow-sm'}`}>
        <video
          ref={videoRef}
          src={videoUrl}
          className={`w-full bg-black ${compact ? 'max-h-40' : 'max-h-[50vh]'}`}
          playsInline
          preload="metadata"
          controls={!compact}
        />
        <audio ref={audioRef} src={audioUrl} preload="metadata" className="hidden" />
        {compact && (
          <div className="absolute inset-0 flex items-center justify-center bg-black/20">
            <Button
              type="button"
              size="sm"
              variant="secondary"
              className="h-8 bg-white/90 text-surface-800 hover:bg-white"
              onClick={() => { void handlePlay() }}
            >
              {playing ? <Pause className="mr-1 h-3.5 w-3.5" /> : <Play className="mr-1 h-3.5 w-3.5" />}
              {playing ? '暂停' : label}
            </Button>
          </div>
        )}
      </div>

      {!compact && (
        <div className="mt-2 flex flex-wrap items-center gap-2">
          <Button type="button" size="sm" variant="outline" onClick={() => { void handlePlay() }}>
            {playing ? <Pause className="mr-1 h-3.5 w-3.5" /> : <Play className="mr-1 h-3.5 w-3.5" />}
            {playing ? '暂停' : label}
          </Button>
          {showDownload && (
            <>
              <a
                href={videoUrl}
                download
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex items-center gap-1 rounded-md border border-surface-200 px-2.5 py-1 text-[11px] text-surface-600 hover:bg-surface-50"
              >
                <Download className="h-3 w-3" /> 视频
              </a>
              <a
                href={audioUrl}
                download
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex items-center gap-1 rounded-md border border-surface-200 px-2.5 py-1 text-[11px] text-surface-600 hover:bg-surface-50"
              >
                <Download className="h-3 w-3" /> 配音
              </a>
            </>
          )}
        </div>
      )}

      {staleHint && (
        <p className="mt-1.5 text-[11px] text-amber-600">{staleHint}</p>
      )}
      {driftHint && (
        <p className="mt-1 text-[11px] text-surface-500">{driftHint}</p>
      )}
    </div>
  )
}
