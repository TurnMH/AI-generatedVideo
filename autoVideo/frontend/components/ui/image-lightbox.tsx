'use client'

import { useEffect, useState, type ImgHTMLAttributes } from 'react'
import { X, ZoomIn } from 'lucide-react'

interface ImageLightboxProps {
  src: string
  alt?: string
  onClose: () => void
}

export function ImageLightbox({ src, alt, onClose }: ImageLightboxProps) {
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    document.addEventListener('keydown', onKey)
    const prev = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => {
      document.removeEventListener('keydown', onKey)
      document.body.style.overflow = prev
    }
  }, [onClose])

  return (
    <div
      className="fixed inset-0 z-[100] flex items-center justify-center bg-black/85 p-4"
      onClick={onClose}
      role="dialog"
      aria-modal="true"
    >
      <button
        type="button"
        aria-label="关闭"
        onClick={(e) => {
          e.stopPropagation()
          onClose()
        }}
        className="absolute right-4 top-4 rounded-full bg-white/10 p-2 text-white backdrop-blur transition hover:bg-white/20"
      >
        <X className="h-5 w-5" />
      </button>
      {/* eslint-disable-next-line @next/next/no-img-element */}
      <img
        src={src}
        alt={alt ?? ''}
        onClick={(e) => e.stopPropagation()}
        className="max-h-[92vh] max-w-[92vw] rounded-lg object-contain shadow-2xl"
      />
    </div>
  )
}

type ZoomableImageProps = ImgHTMLAttributes<HTMLImageElement> & {
  src: string
  alt?: string
  previewSrc?: string
}

/**
 * ZoomableImage renders an <img> that opens a full-screen ImageLightbox on click.
 * Use previewSrc to show a higher-resolution image in the lightbox.
 */
export function ZoomableImage({ src, alt, previewSrc, onClick, className, ...rest }: ZoomableImageProps) {
  const [open, setOpen] = useState(false)
  return (
    <>
      {/* eslint-disable-next-line @next/next/no-img-element */}
      <img
        {...rest}
        src={src}
        alt={alt ?? ''}
        className={`${className ?? ''} cursor-zoom-in`}
        onClick={(e) => {
          onClick?.(e)
          if (!e.defaultPrevented) {
            e.stopPropagation()
            setOpen(true)
          }
        }}
      />
      {open && <ImageLightbox src={previewSrc ?? src} alt={alt} onClose={() => setOpen(false)} />}
    </>
  )
}

interface ZoomBadgeProps {
  src: string
  alt?: string
  className?: string
  title?: string
}

/**
 * ZoomBadge is a small overlay button (e.g. top-right of a clickable card)
 * that opens a full-screen image lightbox without triggering the parent card's
 * onClick handler. Useful when the parent already uses click-to-select.
 */
export function ZoomBadge({ src, alt, className, title = '查看大图' }: ZoomBadgeProps) {
  const [open, setOpen] = useState(false)
  if (!src) return null
  return (
    <>
      <button
        type="button"
        title={title}
        aria-label={title}
        onClick={(e) => {
          e.stopPropagation()
          e.preventDefault()
          setOpen(true)
        }}
        className={`rounded-md bg-black/55 p-1 text-white shadow-sm backdrop-blur transition hover:bg-black/75 ${className ?? ''}`}
      >
        <ZoomIn className="h-3.5 w-3.5" />
      </button>
      {open && <ImageLightbox src={src} alt={alt} onClose={() => setOpen(false)} />}
    </>
  )
}
