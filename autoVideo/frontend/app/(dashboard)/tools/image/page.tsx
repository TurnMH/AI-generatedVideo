'use client'

import { useEffect, useMemo, useRef, useState, type PointerEvent as ReactPointerEvent } from 'react'
import { Upload, Download, ImageIcon, Info, Cpu, RefreshCw, RotateCw, Crop as CropIcon, Move } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs'
import { Slider } from '@/components/ui/slider'
import { Badge } from '@/components/ui/badge'

// ─── Types ────────────────────────────────────────────────────────────────────

type OutputFormat = 'image/jpeg' | 'image/png' | 'image/webp'

type ImageInfo = {
  name: string
  size: number
  type: string
  width: number
  height: number
  dataUrl: string
}

type CropRect = {
  x: number
  y: number
  width: number
  height: number
}

type CropHandle = 'move' | 'nw' | 'ne' | 'sw' | 'se'

type CropInteraction = {
  handle: CropHandle
  startX: number
  startY: number
  startCrop: CropRect
}

const MIN_CROP_SIZE = 24

function clamp(value: number, min: number, max: number) {
  if (max < min) return min
  return Math.min(Math.max(value, min), max)
}

// ─── Format compatibility matrix ──────────────────────────────────────────────

function detectFormatSupport() {
  if (typeof document === 'undefined') return {}
  const canvas = document.createElement('canvas')
  canvas.width = 1
  canvas.height = 1
  return {
    webp: canvas.toDataURL('image/webp').startsWith('data:image/webp'),
    avif: canvas.toDataURL('image/avif').startsWith('data:image/avif'),
    jpeg: true,
    png: true,
  }
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / 1024 / 1024).toFixed(2)} MB`
}

function loadImageInfo(file: File): Promise<ImageInfo> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = (e) => {
      const dataUrl = e.target?.result as string
      const img = new Image()
      img.onload = () =>
        resolve({
          name: file.name,
          size: file.size,
          type: file.type,
          width: img.naturalWidth,
          height: img.naturalHeight,
          dataUrl,
        })
      img.onerror = () => reject(new Error('图片加载失败'))
      img.src = dataUrl
    }
    reader.onerror = () => reject(new Error('文件读取失败'))
    reader.readAsDataURL(file)
  })
}

function loadHtmlImage(dataUrl: string): Promise<HTMLImageElement> {
  return new Promise((resolve, reject) => {
    const img = new Image()
    img.onload = () => resolve(img)
    img.onerror = () => reject(new Error('图片加载失败'))
    img.src = dataUrl
  })
}

function convertImage(
  info: ImageInfo,
  format: OutputFormat,
  quality: number,
  maxWidth?: number,
  maxHeight?: number,
): Promise<{ blob: Blob; url: string }> {
  return new Promise((resolve, reject) => {
    const img = new Image()
    img.onload = () => {
      let w = img.naturalWidth
      let h = img.naturalHeight
      // Resize if needed
      if (maxWidth && w > maxWidth) { h = Math.round((h * maxWidth) / w); w = maxWidth }
      if (maxHeight && h > maxHeight) { w = Math.round((w * maxHeight) / h); h = maxHeight }
      const canvas = document.createElement('canvas')
      canvas.width = w
      canvas.height = h
      const ctx = canvas.getContext('2d')!
      if (format === 'image/jpeg') {
        ctx.fillStyle = '#ffffff'
        ctx.fillRect(0, 0, w, h)
      }
      ctx.drawImage(img, 0, 0, w, h)
      canvas.toBlob(
        (blob) => {
          if (!blob) { reject(new Error('转换失败')); return }
          resolve({ blob, url: URL.createObjectURL(blob) })
        },
        format,
        quality / 100,
      )
    }
    img.onerror = () => reject(new Error('图片加载失败'))
    img.src = info.dataUrl
  })
}

function extOf(format: OutputFormat): string {
  return { 'image/jpeg': 'jpg', 'image/png': 'png', 'image/webp': 'webp' }[format]
}

function createDefaultCrop(info: ImageInfo): CropRect {
  const safeMin = Math.max(1, Math.min(MIN_CROP_SIZE, info.width, info.height))
  const width = Math.max(safeMin, Math.round(info.width * 0.8))
  const height = Math.max(safeMin, Math.round(info.height * 0.8))
  return {
    x: Math.max(0, Math.round((info.width - width) / 2)),
    y: Math.max(0, Math.round((info.height - height) / 2)),
    width: Math.min(width, info.width),
    height: Math.min(height, info.height),
  }
}

function normalizeCropRect(rect: CropRect, info: ImageInfo): CropRect {
  const safeMin = Math.max(1, Math.min(MIN_CROP_SIZE, info.width, info.height))
  const width = clamp(Math.round(rect.width), safeMin, info.width)
  const height = clamp(Math.round(rect.height), safeMin, info.height)
  const x = clamp(Math.round(rect.x), 0, Math.max(0, info.width - width))
  const y = clamp(Math.round(rect.y), 0, Math.max(0, info.height - height))
  return { x, y, width, height }
}

function moveCropRect(startCrop: CropRect, deltaX: number, deltaY: number, info: ImageInfo): CropRect {
  return normalizeCropRect(
    {
      ...startCrop,
      x: startCrop.x + deltaX,
      y: startCrop.y + deltaY,
    },
    info,
  )
}

function resizeCropRect(startCrop: CropRect, handle: Exclude<CropHandle, 'move'>, deltaX: number, deltaY: number, info: ImageInfo): CropRect {
  const left = startCrop.x
  const top = startCrop.y
  const right = startCrop.x + startCrop.width
  const bottom = startCrop.y + startCrop.height
  const safeMin = Math.max(1, Math.min(MIN_CROP_SIZE, info.width, info.height))

  let nextLeft = left
  let nextTop = top
  let nextRight = right
  let nextBottom = bottom

  if (handle === 'nw') {
    nextLeft = clamp(left + deltaX, 0, right - safeMin)
    nextTop = clamp(top + deltaY, 0, bottom - safeMin)
  }
  if (handle === 'ne') {
    nextRight = clamp(right + deltaX, left + safeMin, info.width)
    nextTop = clamp(top + deltaY, 0, bottom - safeMin)
  }
  if (handle === 'sw') {
    nextLeft = clamp(left + deltaX, 0, right - safeMin)
    nextBottom = clamp(bottom + deltaY, top + safeMin, info.height)
  }
  if (handle === 'se') {
    nextRight = clamp(right + deltaX, left + safeMin, info.width)
    nextBottom = clamp(bottom + deltaY, top + safeMin, info.height)
  }

  return normalizeCropRect(
    {
      x: nextLeft,
      y: nextTop,
      width: nextRight - nextLeft,
      height: nextBottom - nextTop,
    },
    info,
  )
}

function canvasToBlob(canvas: HTMLCanvasElement, format: OutputFormat, quality: number): Promise<Blob> {
  return new Promise((resolve, reject) => {
    canvas.toBlob(
      (blob) => {
        if (!blob) {
          reject(new Error('裁剪失败'))
          return
        }
        resolve(blob)
      },
      format,
      format === 'image/png' ? undefined : quality / 100,
    )
  })
}

function calculateContainBox(stageWidth: number, stageHeight: number, imageWidth: number, imageHeight: number) {
  if (stageWidth <= 0 || stageHeight <= 0 || imageWidth <= 0 || imageHeight <= 0) {
    return null
  }
  const scale = Math.min(stageWidth / imageWidth, stageHeight / imageHeight)
  const width = imageWidth * scale
  const height = imageHeight * scale
  return {
    width,
    height,
    left: (stageWidth - width) / 2,
    top: (stageHeight - height) / 2,
    scale,
  }
}

// ─── Drop zone ────────────────────────────────────────────────────────────────

function DropZone({ onFile }: { onFile: (f: File) => void }) {
  const inputRef = useRef<HTMLInputElement>(null)
  const [dragging, setDragging] = useState(false)

  return (
    <div
      onDragOver={(e) => { e.preventDefault(); setDragging(true) }}
      onDragLeave={() => setDragging(false)}
      onDrop={(e) => {
        e.preventDefault(); setDragging(false)
        const file = e.dataTransfer.files[0]
        if (file && file.type.startsWith('image/')) onFile(file)
      }}
      onClick={() => inputRef.current?.click()}
      className={`flex h-32 cursor-pointer flex-col items-center justify-center gap-2 rounded-xl border-2 border-dashed transition-colors
        ${dragging ? 'border-indigo-400 bg-indigo-50' : 'border-surface-200 bg-surface-50 hover:border-indigo-300 hover:bg-indigo-50/40'}`}
    >
      <Upload className="h-6 w-6 text-surface-400" />
      <p className="text-sm text-surface-500">拖入或点击上传图片</p>
      <p className="text-xs text-surface-400">支持 JPG · PNG · WebP · GIF · BMP · AVIF</p>
      <input
        ref={inputRef}
        type="file"
        accept="image/*"
        className="hidden"
        onChange={(e) => { const f = e.target.files?.[0]; if (f) onFile(f) }}
      />
    </div>
  )
}

// ─── Adjust Tool ────────────────────────────────────────────────────────────────

type FilterState = {
  brightness: number; contrast: number; saturation: number
  hue: number; blur: number; grayscale: boolean; sepia: boolean; invert: boolean; opacity: number
}

const DEFAULT_FILTERS: FilterState = {
  brightness: 100, contrast: 100, saturation: 100, hue: 0,
  blur: 0, grayscale: false, sepia: false, invert: false, opacity: 100,
}

function buildFilter(f: FilterState) {
  return [
    `brightness(${f.brightness}%)`, `contrast(${f.contrast}%)`, `saturate(${f.saturation}%)`,
    `hue-rotate(${f.hue}deg)`,
    f.blur > 0 ? `blur(${f.blur}px)` : '',
    f.grayscale ? 'grayscale(1)' : '', f.sepia ? 'sepia(1)' : '', f.invert ? 'invert(1)' : '',
    `opacity(${f.opacity}%)`,
  ].filter(Boolean).join(' ')
}

function AdjustTool({ info }: { info: ImageInfo }) {
  const [filters, setFilters] = useState<FilterState>(DEFAULT_FILTERS)
  const [saving, setSaving] = useState(false)

  function set<K extends keyof FilterState>(k: K, v: FilterState[K]) {
    setFilters(p => ({ ...p, [k]: v }))
  }

  async function download() {
    setSaving(true)
    try {
      const img = new Image()
      await new Promise<void>((res, rej) => { img.onload = () => res(); img.onerror = rej; img.src = info.dataUrl })
      const canvas = document.createElement('canvas')
      canvas.width = img.naturalWidth; canvas.height = img.naturalHeight
      const ctx = canvas.getContext('2d')!
      ctx.filter = buildFilter(filters)
      ctx.drawImage(img, 0, 0)
      const blob: Blob = await new Promise((res, rej) => canvas.toBlob(b => b ? res(b) : rej(), 'image/jpeg', 0.92))
      const a = document.createElement('a')
      a.href = URL.createObjectURL(blob)
      a.download = info.name.replace(/\.[^.]+$/, '') + '_adjusted.jpg'
      a.click()
    } finally { setSaving(false) }
  }

  const filterStr = buildFilter(filters)
  const isDefault = JSON.stringify(filters) === JSON.stringify(DEFAULT_FILTERS)

  return (
    <div className="grid grid-cols-2 gap-4">
      <div className="space-y-3">
        {([
          { key: 'brightness' as const, label: '亮度', min: 0, max: 200, step: 1, unit: '%' },
          { key: 'contrast' as const, label: '对比度', min: 0, max: 200, step: 1, unit: '%' },
          { key: 'saturation' as const, label: '饱和度', min: 0, max: 200, step: 1, unit: '%' },
          { key: 'hue' as const, label: '色相偏移', min: -180, max: 180, step: 1, unit: '°' },
          { key: 'blur' as const, label: '模糊', min: 0, max: 20, step: 0.5, unit: 'px' },
          { key: 'opacity' as const, label: '不透明度', min: 0, max: 100, step: 1, unit: '%' },
        ] as const).map(({ key, label, min, max, step, unit }) => (
          <div key={key} className="space-y-1">
            <div className="flex items-center justify-between text-sm">
              <span className="text-surface-600">{label}</span>
              <span className="font-mono text-indigo-600">{filters[key] as number}{unit}</span>
            </div>
            <Slider min={min} max={max} step={step} value={[filters[key] as number]} onValueChange={([v]) => set(key, v as any)} />
          </div>
        ))}
        <div className="flex gap-2 flex-wrap pt-1">
          {([['grayscale', '灰度'], ['sepia', '怀旧色'], ['invert', '反色']] as const).map(([k, label]) => (
            <button key={k} onClick={() => set(k, !filters[k])}
              className={`rounded-full border px-3 py-1 text-xs font-medium transition-colors
                ${filters[k] ? 'border-indigo-400 bg-indigo-50 text-indigo-700' : 'border-surface-200 text-surface-600 hover:border-indigo-300'}`}>
              {label}
            </button>
          ))}
        </div>
        <div className="flex gap-2 pt-1">
          <Button size="sm" onClick={download} disabled={saving} className="gap-1.5">
            <Download className="h-3.5 w-3.5" /> {saving ? '生成中…' : '下载结果 (JPG)'}
          </Button>
          {!isDefault && (
            <Button size="sm" variant="outline" onClick={() => setFilters(DEFAULT_FILTERS)} className="gap-1.5">
              <RefreshCw className="h-3.5 w-3.5" /> 重置
            </Button>
          )}
        </div>
      </div>
      <div className="flex items-center justify-center rounded-xl border border-surface-200 bg-surface-50 p-3 min-h-[200px] overflow-hidden">
        <img src={info.dataUrl} alt="preview" style={{ filter: filterStr }} className="max-w-full max-h-64 object-contain rounded-lg" />
      </div>
    </div>
  )
}

// ─── Transform Tool ───────────────────────────────────────────────────────────

function TransformTool({ info }: { info: ImageInfo }) {
  const [targetW, setTargetW] = useState(info.width)
  const [targetH, setTargetH] = useState(info.height)
  const [lockAspect, setLockAspect] = useState(true)
  const [rotation, setRotation] = useState(0)
  const [flipH, setFlipH] = useState(false)
  const [flipV, setFlipV] = useState(false)
  const [processing, setProcessing] = useState(false)
  const [previewUrl, setPreviewUrl] = useState('')
  const [resultSize, setResultSize] = useState(0)
  const ratio = info.width / info.height

  function updateW(v: number) {
    const n = Math.max(1, v)
    setTargetW(n)
    if (lockAspect) setTargetH(Math.round(n / ratio))
  }
  function updateH(v: number) {
    const n = Math.max(1, v)
    setTargetH(n)
    if (lockAspect) setTargetW(Math.round(n * ratio))
  }

  async function apply() {
    setProcessing(true)
    try {
      const img = new Image()
      await new Promise<void>((res, rej) => { img.onload = () => res(); img.onerror = rej; img.src = info.dataUrl })
      const isVertical = rotation === 90 || rotation === 270
      const cw = isVertical ? targetH : targetW
      const ch = isVertical ? targetW : targetH
      const canvas = document.createElement('canvas')
      canvas.width = cw; canvas.height = ch
      const ctx = canvas.getContext('2d')!
      ctx.translate(cw / 2, ch / 2)
      ctx.rotate((rotation * Math.PI) / 180)
      ctx.scale(flipH ? -1 : 1, flipV ? -1 : 1)
      ctx.drawImage(img, -targetW / 2, -targetH / 2, targetW, targetH)
      const blob: Blob = await new Promise((res, rej) => canvas.toBlob(b => b ? res(b) : rej(), 'image/jpeg', 0.92))
      setPreviewUrl(URL.createObjectURL(blob))
      setResultSize(blob.size)
    } finally { setProcessing(false) }
  }

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-2 gap-4">
        <div className="rounded-xl border border-surface-200 bg-white p-4 space-y-3 shadow-sm">
          <p className="text-sm font-medium text-surface-700">尺寸调整</p>
          <div className="flex items-center gap-2">
            <div className="flex-1 space-y-1">
              <label className="text-xs text-surface-500">宽度 (px)</label>
              <input type="number" min={1} max={9999} value={targetW} onChange={(e) => updateW(+e.target.value)}
                className="w-full rounded-lg border border-surface-200 px-3 py-1.5 text-sm font-mono focus:outline-none focus:border-indigo-400" />
            </div>
            <button onClick={() => setLockAspect(l => !l)}
              className={`mt-5 rounded-lg border px-2 py-1.5 text-sm transition-colors ${lockAspect ? 'border-indigo-400 bg-indigo-50 text-indigo-600' : 'border-surface-200 text-surface-400'}`}
              title="锁定宽高比">
              {lockAspect ? '🔒' : '🔓'}
            </button>
            <div className="flex-1 space-y-1">
              <label className="text-xs text-surface-500">高度 (px)</label>
              <input type="number" min={1} max={9999} value={targetH} onChange={(e) => updateH(+e.target.value)}
                className="w-full rounded-lg border border-surface-200 px-3 py-1.5 text-sm font-mono focus:outline-none focus:border-indigo-400" />
            </div>
          </div>
          <p className="text-xs text-surface-400">原始：{info.width} × {info.height} px</p>
          <div className="flex flex-wrap gap-1.5">
            {[[25, '25%'], [50, '50%'], [75, '75%'], [100, '原图'], [200, '200%']].map(([pct, label]) => (
              <button key={pct} onClick={() => updateW(Math.round(info.width * +pct / 100))}
                className="rounded border border-surface-200 px-2 py-0.5 text-xs text-surface-600 hover:border-indigo-300 hover:text-indigo-600 transition-colors">
                {label}
              </button>
            ))}
          </div>
        </div>
        <div className="rounded-xl border border-surface-200 bg-white p-4 space-y-3 shadow-sm">
          <p className="text-sm font-medium text-surface-700">旋转 / 翻转</p>
          <div className="space-y-2">
            <p className="text-xs text-surface-500">旋转角度</p>
            <div className="flex gap-1.5">
              {[0, 90, 180, 270].map((r) => (
                <button key={r} onClick={() => setRotation(r)}
                  className={`flex-1 rounded-lg border py-1.5 text-xs font-medium transition-colors ${rotation === r ? 'border-indigo-500 bg-indigo-50 text-indigo-700' : 'border-surface-200 text-surface-600 hover:border-indigo-300'}`}>
                  {r}°
                </button>
              ))}
            </div>
            <p className="text-xs text-surface-500 pt-1">翻转</p>
            <div className="flex gap-2">
              <button onClick={() => setFlipH(h => !h)}
                className={`flex-1 rounded-lg border py-1.5 text-xs font-medium transition-colors ${flipH ? 'border-indigo-500 bg-indigo-50 text-indigo-700' : 'border-surface-200 text-surface-600 hover:border-surface-300'}`}>
                ↔ 水平翻转
              </button>
              <button onClick={() => setFlipV(v => !v)}
                className={`flex-1 rounded-lg border py-1.5 text-xs font-medium transition-colors ${flipV ? 'border-indigo-500 bg-indigo-50 text-indigo-700' : 'border-surface-200 text-surface-600 hover:border-surface-300'}`}>
                ↕ 垂直翻转
              </button>
            </div>
          </div>
        </div>
      </div>
      <div className="flex gap-2">
        <Button onClick={apply} disabled={processing} className="gap-2">
          {processing ? <RefreshCw className="h-4 w-4 animate-spin" /> : <RotateCw className="h-4 w-4" />}
          {processing ? '处理中…' : '应用变换'}
        </Button>
        {previewUrl && (
          <Button variant="outline" onClick={() => { const a = document.createElement('a'); a.href = previewUrl; a.download = info.name.replace(/\.[^.]+$/, '') + '_transformed.jpg'; a.click() }} className="gap-2">
            <Download className="h-4 w-4" /> 下载 ({formatBytes(resultSize)})
          </Button>
        )}
      </div>
      {previewUrl && (
        <div className="rounded-xl border border-surface-200 bg-surface-50 p-3">
          <img src={previewUrl} alt="transformed" className="max-w-full max-h-64 object-contain rounded-lg mx-auto block" />
        </div>
      )}
    </div>
  )
}

// ─── Crop Tool ────────────────────────────────────────────────────────────────

function CropTool({ info, onFile }: { info: ImageInfo | null; onFile: (file: File) => void }) {
  const stageRef = useRef<HTMLDivElement>(null)
  const previewUrlRef = useRef('')
  const [crop, setCrop] = useState<CropRect | null>(info ? createDefaultCrop(info) : null)
  const [stageSize, setStageSize] = useState({ width: 0, height: 0 })
  const [interaction, setInteraction] = useState<CropInteraction | null>(null)
  const [cropFormat, setCropFormat] = useState<OutputFormat>('image/png')
  const [cropQuality, setCropQuality] = useState(92)
  const [exporting, setExporting] = useState(false)
  const [previewUrl, setPreviewUrl] = useState('')
  const [resultSize, setResultSize] = useState(0)
  const [cropError, setCropError] = useState('')

  useEffect(() => {
    if (previewUrlRef.current) {
      URL.revokeObjectURL(previewUrlRef.current)
      previewUrlRef.current = ''
    }
    setPreviewUrl('')
    setResultSize(0)
    setCropError('')
    setInteraction(null)
    setCrop(info ? createDefaultCrop(info) : null)
  }, [info?.name, info?.size])

  useEffect(() => () => {
    if (previewUrlRef.current) {
      URL.revokeObjectURL(previewUrlRef.current)
      previewUrlRef.current = ''
    }
  }, [])

  useEffect(() => {
    if (!info) return
    const element = stageRef.current
    if (!element) return

    const updateSize = () => {
      setStageSize({ width: element.clientWidth, height: element.clientHeight })
    }

    updateSize()

    if (typeof ResizeObserver === 'undefined') {
      const handleResize = () => updateSize()
      window.addEventListener('resize', handleResize)
      return () => window.removeEventListener('resize', handleResize)
    }

    const observer = new ResizeObserver(updateSize)
    observer.observe(element)
    return () => observer.disconnect()
  }, [info])

  const imageBox = useMemo(() => {
    if (!info) return null
    return calculateContainBox(stageSize.width, stageSize.height, info.width, info.height)
  }, [info, stageSize.width, stageSize.height])

  useEffect(() => {
    if (!interaction || !info || !imageBox) return

    const handleMove = (event: PointerEvent) => {
      const deltaX = (event.clientX - interaction.startX) / imageBox.scale
      const deltaY = (event.clientY - interaction.startY) / imageBox.scale
      setCrop(interaction.handle === 'move'
        ? moveCropRect(interaction.startCrop, deltaX, deltaY, info)
        : resizeCropRect(interaction.startCrop, interaction.handle, deltaX, deltaY, info))
    }

    const handleEnd = () => setInteraction(null)

    window.addEventListener('pointermove', handleMove)
    window.addEventListener('pointerup', handleEnd)
    window.addEventListener('pointercancel', handleEnd)
    return () => {
      window.removeEventListener('pointermove', handleMove)
      window.removeEventListener('pointerup', handleEnd)
      window.removeEventListener('pointercancel', handleEnd)
    }
  }, [interaction, imageBox, info])

  const cropStyle = useMemo(() => {
    if (!crop || !imageBox) return null
    return {
      left: imageBox.left + crop.x * imageBox.scale,
      top: imageBox.top + crop.y * imageBox.scale,
      width: crop.width * imageBox.scale,
      height: crop.height * imageBox.scale,
    }
  }, [crop, imageBox])

  function startInteraction(handle: CropHandle, event: ReactPointerEvent<HTMLDivElement>) {
    if (!info || !crop || !imageBox) return
    event.preventDefault()
    event.stopPropagation()
    setCropError('')
    setInteraction({
      handle,
      startX: event.clientX,
      startY: event.clientY,
      startCrop: crop,
    })
  }

  async function exportCrop() {
    if (!info || !crop) return
    setExporting(true)
    setCropError('')
    try {
      const img = await loadHtmlImage(info.dataUrl)
      const canvas = document.createElement('canvas')
      canvas.width = Math.max(1, Math.round(crop.width))
      canvas.height = Math.max(1, Math.round(crop.height))
      const ctx = canvas.getContext('2d')
      if (!ctx) throw new Error('画布初始化失败')
      if (cropFormat === 'image/jpeg') {
        ctx.fillStyle = '#ffffff'
        ctx.fillRect(0, 0, canvas.width, canvas.height)
      }
      ctx.drawImage(img, crop.x, crop.y, crop.width, crop.height, 0, 0, canvas.width, canvas.height)
      const blob = await canvasToBlob(canvas, cropFormat, cropQuality)
      const nextUrl = URL.createObjectURL(blob)
      if (previewUrlRef.current) URL.revokeObjectURL(previewUrlRef.current)
      previewUrlRef.current = nextUrl
      setPreviewUrl(nextUrl)
      setResultSize(blob.size)

      const downloadName = info.name.replace(/\.[^.]+$/, '') + `_cropped.${extOf(cropFormat)}`
      const anchor = document.createElement('a')
      anchor.href = nextUrl
      anchor.download = downloadName
      anchor.click()
    } catch (error: any) {
      setCropError(error?.message || '裁剪导出失败')
    } finally {
      setExporting(false)
    }
  }

  function resetCrop() {
    if (!info) return
    setCrop(createDefaultCrop(info))
    setCropError('')
  }

  if (!info) {
    return (
      <div className="space-y-4">
        <DropZone onFile={onFile} />
        <div className="rounded-xl border border-dashed border-surface-200 bg-white p-5 text-sm text-surface-500 shadow-sm">
          先上传一张图片，然后就可以在画布里拖动裁剪框，或者拖拽四个角点调整范围。
        </div>
      </div>
    )
  }

  return (
    <div className="space-y-4">
      <div className="grid gap-4 lg:grid-cols-[minmax(0,1.5fr)_320px]">
        <div className="space-y-3">
          <div className="flex items-center justify-between rounded-xl border border-surface-200 bg-white px-4 py-3 shadow-sm">
            <div>
              <p className="text-sm font-semibold text-surface-800">在线裁剪画布</p>
              <p className="text-xs text-surface-500">拖动裁剪框移动，拖动角点缩放</p>
            </div>
            <div className="rounded-full bg-slate-100 px-3 py-1 text-xs font-mono text-slate-600">
              {crop ? `${crop.width} × ${crop.height}px` : '未初始化'}
            </div>
          </div>

          <div
            ref={stageRef}
            className="relative h-[460px] overflow-hidden rounded-2xl border border-surface-200 bg-slate-950 shadow-sm"
          >
            {imageBox && crop && cropStyle && (
              <>
                <img
                  src={info.dataUrl}
                  alt="crop preview"
                  draggable={false}
                  className="absolute select-none object-fill"
                  style={{
                    width: imageBox.width,
                    height: imageBox.height,
                    left: imageBox.left,
                    top: imageBox.top,
                  }}
                />

                <div
                  className="absolute rounded-lg border-2 border-cyan-300 bg-cyan-400/10 shadow-[0_0_0_9999px_rgba(2,6,23,0.58)]"
                  style={cropStyle}
                  onPointerDown={(event) => startInteraction('move', event)}
                >
                  <div className="absolute inset-0 pointer-events-none border border-white/30">
                    <div className="absolute left-1/3 top-0 h-full w-px bg-white/20" />
                    <div className="absolute left-2/3 top-0 h-full w-px bg-white/20" />
                    <div className="absolute top-1/3 left-0 w-full h-px bg-white/20" />
                    <div className="absolute top-2/3 left-0 w-full h-px bg-white/20" />
                  </div>

                  <div className="pointer-events-none absolute left-2 top-2 inline-flex items-center gap-1 rounded-full bg-slate-950/70 px-2 py-1 text-[10px] font-medium text-white backdrop-blur">
                    <CropIcon className="h-3 w-3" />
                    拖动编辑
                  </div>

                  {([
                    ['nw', '-left-2 -top-2 cursor-nwse-resize'],
                    ['ne', '-right-2 -top-2 cursor-nesw-resize'],
                    ['sw', '-left-2 -bottom-2 cursor-nesw-resize'],
                    ['se', '-right-2 -bottom-2 cursor-nwse-resize'],
                  ] as const).map(([handle, position]) => (
                    <button
                      key={handle}
                      type="button"
                      className={`absolute ${position} h-4 w-4 rounded-full border border-white bg-cyan-400 shadow-sm transition-transform hover:scale-110`}
                      onPointerDown={(event) => startInteraction(handle, event)}
                    />
                  ))}
                </div>
              </>
            )}

            {!imageBox && (
              <div className="flex h-full items-center justify-center text-sm text-slate-300">
                图片加载中...
              </div>
            )}
          </div>
        </div>

        <div className="space-y-3 rounded-2xl border border-surface-200 bg-white p-4 shadow-sm">
          <div className="flex items-center gap-2">
            <Move className="h-4 w-4 text-cyan-600" />
            <p className="text-sm font-semibold text-surface-800">裁剪设置</p>
          </div>

          <div className="grid grid-cols-2 gap-2 text-xs text-surface-500">
            <div className="rounded-lg bg-surface-50 px-3 py-2">
              <div>起点</div>
              <div className="font-mono text-surface-700">{crop ? `${crop.x}, ${crop.y}` : '--'}</div>
            </div>
            <div className="rounded-lg bg-surface-50 px-3 py-2">
              <div>尺寸</div>
              <div className="font-mono text-surface-700">{crop ? `${crop.width} × ${crop.height}` : '--'}</div>
            </div>
          </div>

          <div className="space-y-2">
            <p className="text-sm font-medium text-surface-700">输出格式</p>
            <Select value={cropFormat} onValueChange={(value) => setCropFormat(value as OutputFormat)}>
              <SelectTrigger>
                <SelectValue placeholder="选择格式" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="image/png">PNG</SelectItem>
                <SelectItem value="image/jpeg">JPG</SelectItem>
                <SelectItem value="image/webp">WebP</SelectItem>
              </SelectContent>
            </Select>
            {cropFormat !== 'image/png' && (
              <div className="space-y-2">
                <div className="flex items-center justify-between text-xs text-surface-500">
                  <span>压缩质量</span>
                  <span className="font-mono text-cyan-700">{cropQuality}%</span>
                </div>
                <Slider min={20} max={100} step={1} value={[cropQuality]} onValueChange={([value]) => setCropQuality(value)} />
              </div>
            )}
          </div>

          <div className="flex flex-wrap gap-2">
            <Button type="button" size="sm" className="gap-1.5" onClick={exportCrop} disabled={exporting || !crop}>
              {exporting ? <RefreshCw className="h-3.5 w-3.5 animate-spin" /> : <CropIcon className="h-3.5 w-3.5" />}
              {exporting ? '导出中…' : '导出并下载'}
            </Button>
            <Button type="button" size="sm" variant="outline" className="gap-1.5" onClick={resetCrop}>
              <RefreshCw className="h-3.5 w-3.5" />
              重置裁剪框
            </Button>
          </div>

          {cropError && (
            <div className="rounded-lg bg-red-50 px-3 py-2 text-xs text-red-700">
              {cropError}
            </div>
          )}

          {previewUrl && (
            <div className="space-y-2 rounded-xl bg-surface-50 p-3">
              <div className="flex items-center justify-between text-xs text-surface-500">
                <span>导出结果</span>
                <span className="font-mono">{formatBytes(resultSize)}</span>
              </div>
              <img src={previewUrl} alt="cropped preview" className="max-h-48 w-full rounded-lg object-contain bg-white" />
            </div>
          )}
        </div>
      </div>

      <div className="rounded-xl border border-dashed border-surface-200 bg-white px-4 py-3 text-xs text-surface-500 shadow-sm">
        提示：先按住裁剪框中间拖动，再拖四角调整大小。导出会直接下载当前裁剪结果。
      </div>
    </div>
  )
}

// ─── Page ─────────────────────────────────────────────────────────────────────

export default function ImageToolsPage() {
  const [info, setInfo] = useState<ImageInfo | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  // Convert
  const [outFormat, setOutFormat] = useState<OutputFormat>('image/jpeg')
  const [quality, setQuality] = useState(85)
  const [result, setResult] = useState<{ url: string; size: number; blob: Blob } | null>(null)
  const [converting, setConverting] = useState(false)

  // Support
  const support = detectFormatSupport()

  async function handleFile(file: File) {
    setLoading(true); setError(''); setResult(null)
    try {
      const imgInfo = await loadImageInfo(file)
      setInfo(imgInfo)
    } catch (e: any) {
      setError(e.message)
    } finally {
      setLoading(false)
    }
  }

  async function handleConvert() {
    if (!info) return
    setConverting(true); setError('')
    try {
      const res = await convertImage(info, outFormat, quality)
      setResult({ url: res.url, size: res.blob.size, blob: res.blob })
    } catch (e: any) {
      setError(e.message)
    } finally {
      setConverting(false)
    }
  }

  function handleDownload() {
    if (!result || !info) return
    const a = document.createElement('a')
    a.href = result.url
    a.download = info.name.replace(/\.[^.]+$/, '') + '.' + extOf(outFormat)
    a.click()
  }

  return (
    <div className="mx-auto max-w-3xl space-y-6">
      <div className="flex items-center gap-3">
        <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-gradient-to-br from-pink-500 to-rose-500 shadow-sm">
          <ImageIcon className="h-5 w-5 text-white" />
        </div>
        <div>
          <h1 className="text-lg font-bold text-surface-900">图片工具</h1>
          <p className="text-sm text-surface-500">格式转换 · 滤镜调整 · 裁剪编辑 · 图片信息 · 兼容性</p>
        </div>
      </div>

      <Tabs defaultValue="convert">
        <div className="overflow-x-auto">
          <TabsList className="flex w-max min-w-full">
            <TabsTrigger value="convert">格式转换</TabsTrigger>
            <TabsTrigger value="adjust">滤镜调整</TabsTrigger>
            <TabsTrigger value="crop">裁剪编辑</TabsTrigger>
            <TabsTrigger value="transform">裁剪变换</TabsTrigger>
            <TabsTrigger value="info">图片信息</TabsTrigger>
            <TabsTrigger value="compat">格式兼容性</TabsTrigger>
          </TabsList>
        </div>

        {/* ── Convert ── */}
        <TabsContent value="convert" className="space-y-4 pt-2">
          <DropZone onFile={handleFile} />
          {loading && <p className="text-sm text-surface-500">读取中…</p>}
          {error && <p className="text-sm text-red-500">{error}</p>}

          {info && (
            <div className="rounded-xl border border-surface-200 bg-white p-4 shadow-sm">
              <div className="mb-4 flex items-center gap-4">
                <img src={info.dataUrl} alt="preview" className="h-20 w-20 rounded-lg object-contain border border-surface-100 bg-surface-50" />
                <div className="text-sm space-y-1 text-surface-600">
                  <p className="font-medium text-surface-800 truncate max-w-[200px]">{info.name}</p>
                  <p>{info.width} × {info.height} px</p>
                  <p>原始大小：{formatBytes(info.size)}</p>
                  <p>格式：<span className="font-mono text-xs">{info.type || '未知'}</span></p>
                </div>
              </div>

              {/* Output format */}
              <div className="mb-4 space-y-2">
                <p className="text-sm font-medium text-surface-700">输出格式</p>
                <div className="flex gap-2">
                  {(['image/jpeg', 'image/png', 'image/webp'] as OutputFormat[]).map((f) => (
                    <button
                      key={f}
                      onClick={() => setOutFormat(f)}
                      className={`rounded-lg border px-3 py-1.5 text-sm font-medium transition-colors
                        ${outFormat === f
                          ? 'border-indigo-500 bg-indigo-50 text-indigo-700'
                          : 'border-surface-200 bg-white text-surface-600 hover:border-indigo-300'}`}
                    >
                      {extOf(f).toUpperCase()}
                    </button>
                  ))}
                </div>
                <p className="text-xs text-surface-400">
                  {outFormat === 'image/webp' && '⚡ WebP：高压缩率，现代浏览器均支持，老版 iOS/Safari 不支持'}
                  {outFormat === 'image/jpeg' && '✅ JPG：兼容性最好，适合发送给任何软件/设备，不支持透明度'}
                  {outFormat === 'image/png' && '✅ PNG：无损压缩，支持透明度，文件较大'}
                </p>
              </div>

              {/* Quality */}
              {outFormat !== 'image/png' && (
                <div className="mb-4 space-y-2">
                  <div className="flex items-center justify-between">
                    <p className="text-sm font-medium text-surface-700">压缩质量</p>
                    <span className="text-sm font-mono text-indigo-600">{quality}%</span>
                  </div>
                  <Slider
                    min={10} max={100} step={1} value={[quality]}
                    onValueChange={([v]) => setQuality(v)}
                  />
                  <div className="flex justify-between text-xs text-surface-400">
                    <span>最大压缩（10%）</span>
                    <span>最高质量（100%）</span>
                  </div>
                </div>
              )}

              <div className="flex gap-2">
                <Button onClick={handleConvert} disabled={converting} className="gap-2">
                  {converting ? <RefreshCw className="h-4 w-4 animate-spin" /> : <Cpu className="h-4 w-4" />}
                  {converting ? '转换中…' : '开始转换'}
                </Button>
                {result && (
                  <Button variant="outline" onClick={handleDownload} className="gap-2">
                    <Download className="h-4 w-4" />
                    下载（{formatBytes(result.size)}）
                  </Button>
                )}
              </div>

              {result && (
                <div className="mt-3 rounded-lg bg-green-50 px-4 py-3 text-sm text-green-700">
                  转换完成 · 原始 {formatBytes(info.size)} → {formatBytes(result.size)}
                  （{((1 - result.size / info.size) * 100).toFixed(1)}% 压缩率）
                </div>
              )}
            </div>
          )}
        </TabsContent>

        {/* ── Info ── */}
        <TabsContent value="info" className="space-y-4 pt-2">
          <DropZone onFile={handleFile} />
          {info && (
            <div className="rounded-xl border border-surface-200 bg-white p-5 shadow-sm space-y-4">
              <div className="flex items-start gap-4">
                <img src={info.dataUrl} alt="preview" className="h-32 w-32 rounded-xl object-contain border border-surface-100 bg-surface-50" />
                <div className="flex-1 space-y-2">
                  <h3 className="font-semibold text-surface-900">{info.name}</h3>
                  <table className="w-full text-sm">
                    <tbody className="divide-y divide-surface-100">
                      {[
                        ['MIME 类型', info.type || '未知'],
                        ['宽度', `${info.width} px`],
                        ['高度', `${info.height} px`],
                        ['分辨率', `${info.width} × ${info.height}`],
                        ['宽高比', (() => {
                          const g = (a: number, b: number): number => b === 0 ? a : g(b, a % b)
                          const d = g(info.width, info.height)
                          return `${info.width / d} : ${info.height / d}`
                        })()],
                        ['文件大小', formatBytes(info.size)],
                      ].map(([k, v]) => (
                        <tr key={k}>
                          <td className="py-1.5 pr-4 text-surface-500 w-28">{k}</td>
                          <td className="py-1.5 font-mono text-surface-800 text-xs">{v}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </div>
            </div>
          )}
        </TabsContent>

        {/* ── Compatibility ── */}
        <TabsContent value="compat" className="pt-2">
          <div className="rounded-xl border border-surface-200 bg-white p-5 shadow-sm space-y-4">
            <div className="flex items-center gap-2 mb-1">
              <Info className="h-4 w-4 text-indigo-500" />
              <h3 className="font-medium text-surface-800">当前浏览器图片格式支持</h3>
            </div>
            <div className="grid grid-cols-2 gap-3">
              {[
                { key: 'jpeg', label: 'JPEG / JPG', desc: '兼容性最好，全平台支持' },
                { key: 'png', label: 'PNG', desc: '无损，支持透明，全平台支持' },
                { key: 'webp', label: 'WebP', desc: 'Google 格式，Chrome/Firefox/Safari 14+ 支持' },
                { key: 'avif', label: 'AVIF', desc: '新一代格式，Chrome 85+/Safari 16+ 支持' },
              ].map(({ key, label, desc }) => (
                <div key={key} className="flex items-start gap-3 rounded-lg border border-surface-200 p-3">
                  <div className={`mt-0.5 h-5 w-5 rounded-full flex items-center justify-center text-xs font-bold
                    ${(support as any)[key] ? 'bg-green-100 text-green-700' : 'bg-red-100 text-red-600'}`}>
                    {(support as any)[key] ? '✓' : '✗'}
                  </div>
                  <div>
                    <p className="text-sm font-medium text-surface-800">{label}</p>
                    <p className="text-xs text-surface-500 mt-0.5">{desc}</p>
                    <Badge
                      variant={(support as any)[key] ? 'default' : 'outline'}
                      className={`mt-1 text-[10px] px-1.5 ${(support as any)[key] ? 'bg-green-100 text-green-700 border-green-200 hover:bg-green-100' : 'text-red-500 border-red-200'}`}
                    >
                      {(support as any)[key] ? '支持' : '不支持'}
                    </Badge>
                  </div>
                </div>
              ))}
            </div>
            <div className="rounded-lg bg-amber-50 px-4 py-3 text-sm text-amber-700 space-y-1">
              <p className="font-medium">兼容性建议</p>
              <p>• 需要最广泛兼容（微信/企业微信/老 App）→ 使用 <strong>JPG</strong></p>
              <p>• 需要透明背景（Logo/贴纸）→ 使用 <strong>PNG</strong></p>
              <p>• Web 页面优化（减少流量）→ 使用 <strong>WebP</strong>，提供 JPG 降级</p>
              <p>• 超高压缩率场景 → 使用 <strong>AVIF</strong>，注意老设备不支持</p>
            </div>
          </div>
        </TabsContent>

        {/* ── Adjust ── */}
        <TabsContent value="adjust" className="pt-3">
          {info
            ? <AdjustTool info={info} />
            : <p className="mt-4 py-10 text-center text-sm text-surface-400">请先在「格式转换」或「图片信息」标签上传图片</p>}
        </TabsContent>

        {/* ── Transform ── */}
        <TabsContent value="transform" className="pt-3">
          {info
            ? <TransformTool info={info} key={info.name + info.size} />
            : <p className="mt-4 py-10 text-center text-sm text-surface-400">请先在「格式转换」或「图片信息」标签上传图片</p>}
        </TabsContent>

        {/* ── Crop ── */}
        <TabsContent value="crop" className="pt-3">
          <CropTool info={info} onFile={handleFile} key={info ? info.name + info.size : 'crop-empty'} />
        </TabsContent>
      </Tabs>
    </div>
  )
}
