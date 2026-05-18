'use client'

import Link from "next/link"

import { useEffect, useMemo, useRef, useState } from 'react'
import { useRouter } from 'next/navigation'
import useSWR from 'swr'
import {
  ArrowRight,
  Download,
  Image as ImageIcon,
  Loader2,
  Megaphone,
  Repeat,
  RefreshCw,
  Sparkles,
  Upload,
  Wand2,
} from 'lucide-react'
import { assetAPI, modelAPI, projectAPI, storageAPI, taskAPI, videoAPI } from '@/lib/api'
import { savePendingProjectDraft } from '@/lib/project-draft'
import { ensureProjectMediaTag } from '@/lib/project-media'
import type { Asset } from '@/types'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { useToast } from '@/components/ui/toast'
import { VIDEO_MOTION_OPTIONS, VIDEO_STYLE_PRESETS } from '@/lib/video-style-config'

function parseLines(raw: string): string[] {
  return raw
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean)
}

function isHttpUrl(url: string): boolean {
  return /^https?:\/\//i.test(url)
}

function isSupportedVideoFile(file: File): boolean {
  const mime = String(file.type || '').toLowerCase()
  if (mime.startsWith('video/')) return true
  const name = String(file.name || '').toLowerCase()
  return /\.(mp4|mov|m4v|webm|mkv|avi)$/i.test(name)
}

const DEFAULT_AD_TAGS = ['广告', '品牌宣传', '短视频营销']

const AD_TEMPLATES = [
  {
    key: 'ecommerce-sale',
    label: '电商促销',
    hint: '强调优惠与转化，适合活动节点投放',
    promptSeed: '产品主打卖点清晰、限时促销、结尾强 CTA，节奏快，镜头以产品特写+真人使用场景为主。',
    style: 'live-action-short',
    motion: 'dynamic',
    duration: 4,
  },
  {
    key: 'brand-story',
    label: '品牌故事',
    hint: '强化品牌感与情绪价值，适合品牌曝光',
    promptSeed: '突出品牌理念与情绪共鸣，通过人物故事线带出产品价值，结尾口号有记忆点。',
    style: 'live-action-film',
    motion: 'cinematic',
    duration: 5,
  },
  {
    key: 'app-growth',
    label: '应用拉新',
    hint: '问题-解决方案-下载引导结构，适合信息流',
    promptSeed: '展示用户痛点与使用前后对比，强调功能亮点与一键下载，引导立即行动。',
    style: 'live-action-short',
    motion: 'gentle',
    duration: 3,
  },
] as const

type OptimizedAdResult = {
  title: string
  content: string
  outline: string[]
  tags: string[]
}

type VideoTaskSnapshot = {
  id: number
  status: string
  model_name?: string
  result_url?: string
  hls_url?: string
  error_msg?: string
  created_at?: string
  updated_at?: string
  clips?: Array<{ status?: string }>
  image_urls?: string[]
}

type GenerationContext = {
  projectId: number
  projectTitle: string
  prompt: string
  imageUrls: string[]
  sceneDescriptions: string[]
  modelName: string
  stylePreset: string
  motionMode: (typeof VIDEO_MOTION_OPTIONS)[number]['key']
  videoMode: 'frame_animation' | 'api_generation'
  clipDurationSec: number
  startedAt: string
}

type RetryRecord = {
  timestamp: string
  fromModel: string
  toModel: string
  reason: string
  status: 'submitted' | 'failed'
}

function normalizeFailureReason(raw?: string): string {
  const text = String(raw || '').trim()
  if (!text) return '未知失败'
  if (/timeout|timed out|超时/i.test(text)) return '超时'
  if (/quota|limit|额度|频率|429/i.test(text)) return '额度/频率限制'
  if (/auth|token|unauthorized|forbidden|401|403/i.test(text)) return '鉴权失败'
  if (/network|connect|dns|socket|网关|502|503|504/i.test(text)) return '网络/网关异常'
  if (/invalid|参数|bad request|400/i.test(text)) return '参数无效'
  return text.slice(0, 40)
}

function percentile(sortedValues: number[], p: number): number {
  if (sortedValues.length === 0) return 0
  const rank = Math.ceil((p / 100) * sortedValues.length) - 1
  const index = Math.min(sortedValues.length - 1, Math.max(0, rank))
  return sortedValues[index]
}

function estimateCostFactor(modelName: string): number {
  const key = modelName.toLowerCase()
  if (key.includes('sora') || key.includes('seedance') || key.includes('kling')) return 1.45
  if (key.includes('vidu') || key.includes('doubao') || key.includes('wan')) return 1.2
  if (key.includes('comfyui') || key.includes('local')) return 0.5
  return 1.0
}

function normalizeImageUrlFromAsset(asset: Partial<Asset> | null | undefined): string {
  if (!asset) return ''
  const direct = String(asset.image_url ?? '').trim()
  if (direct) return direct

  const selected = String((asset.metadata as Record<string, unknown> | undefined)?.selected_generated_image_url ?? '').trim()
  if (selected) return selected

  const generated = (asset.metadata as Record<string, unknown> | undefined)?.generated_images
  if (Array.isArray(generated)) {
    for (const item of generated) {
      if (item && typeof item === 'object') {
        const url = String((item as Record<string, unknown>).url ?? '').trim()
        if (url) return url
      }
    }
  }
  return ''
}

async function compressImage(file: File, maxSide: number, quality: number): Promise<File> {
  if (!file.type.startsWith('image/')) return file

  const image = await new Promise<HTMLImageElement>((resolve, reject) => {
    const img = new Image()
    img.onload = () => resolve(img)
    img.onerror = () => reject(new Error('图片读取失败'))
    img.src = URL.createObjectURL(file)
  })

  const ratio = image.naturalWidth / image.naturalHeight
  let width = image.naturalWidth
  let height = image.naturalHeight
  if (Math.max(width, height) > maxSide) {
    if (width >= height) {
      width = maxSide
      height = Math.round(maxSide / ratio)
    } else {
      height = maxSide
      width = Math.round(maxSide * ratio)
    }
  }

  const canvas = document.createElement('canvas')
  canvas.width = width
  canvas.height = height
  const ctx = canvas.getContext('2d')
  if (!ctx) throw new Error('图片处理失败')
  ctx.drawImage(image, 0, 0, width, height)

  const blob = await new Promise<Blob>((resolve, reject) => {
    canvas.toBlob((value) => {
      if (!value) {
        reject(new Error('图片压缩失败'))
        return
      }
      resolve(value)
    }, 'image/jpeg', quality)
  })

  const nextName = file.name.replace(/\.[^.]+$/, '') + '.jpg'
  return new File([blob], nextName, { type: 'image/jpeg' })
}

export default function AdVideoPage() {
  const router = useRouter()
  const { toast } = useToast()
  const taskPollRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const autoRetryingRef = useRef(false)

  const [title, setTitle] = useState('')
  const [adPrompt, setAdPrompt] = useState('')
  const [optimizedScript, setOptimizedScript] = useState('')
  const [imageUrlsText, setImageUrlsText] = useState('')
  const [sceneDescriptionsText, setSceneDescriptionsText] = useState('')
  const [localFiles, setLocalFiles] = useState<File[]>([])
  const [enableLocalCompression, setEnableLocalCompression] = useState(true)
  const [maxImageSide, setMaxImageSide] = useState(1920)
  const [jpegQuality, setJpegQuality] = useState(88)
  const [autoOptimizeCopy, setAutoOptimizeCopy] = useState(true)
  const [optimizingCopy, setOptimizingCopy] = useState(false)
  const [creatingByText, setCreatingByText] = useState(false)
  const [creatingByImages, setCreatingByImages] = useState(false)
  const [activeProjectId, setActiveProjectId] = useState<number | null>(null)
  const [activeTaskId, setActiveTaskId] = useState<number | null>(null)
  const [activeTaskStartedAt, setActiveTaskStartedAt] = useState<string | null>(null)
  const [taskStatus, setTaskStatus] = useState<'idle' | 'pending' | 'processing' | 'succeeded' | 'failed'>('idle')
  const [taskError, setTaskError] = useState('')
  const [taskOutputUrl, setTaskOutputUrl] = useState('')
  const [taskClipProgress, setTaskClipProgress] = useState({ done: 0, total: 0 })
  const [lastGenerationContext, setLastGenerationContext] = useState<GenerationContext | null>(null)
  const [autoRetryEnabled, setAutoRetryEnabled] = useState(true)
  const [autoRetryAttempts, setAutoRetryAttempts] = useState(0)
  const [manualRerunLoading, setManualRerunLoading] = useState(false)
  const [exportingPackage, setExportingPackage] = useState(false)
  const [triedModelKeys, setTriedModelKeys] = useState<string[]>([])
  const [retryHistory, setRetryHistory] = useState<RetryRecord[]>([])
  const [batchModelKeys, setBatchModelKeys] = useState<string[]>([])
  const [batchGenerating, setBatchGenerating] = useState(false)
  const [batchSubmittedCount, setBatchSubmittedCount] = useState(0)
  const [compareExporting, setCompareExporting] = useState(false)
  const [sessionAnchorAt, setSessionAnchorAt] = useState<string | null>(null)
  const [trendWindow, setTrendWindow] = useState<'10' | '20' | '50'>('10')
  const [autoAvoidLowHourEnabled, setAutoAvoidLowHourEnabled] = useState(true)
  const [lowHourThreshold, setLowHourThreshold] = useState(65)
  const [lockedModelKey, setLockedModelKey] = useState('')
  const [lockedModelRemaining, setLockedModelRemaining] = useState(0)
  const [lockRunsInput, setLockRunsInput] = useState(3)
  const [adviceExporting, setAdviceExporting] = useState(false)
  const [selectedTemplate, setSelectedTemplate] = useState<(typeof AD_TEMPLATES)[number]['key']>('ecommerce-sale')
  const [selectedVideoModel, setSelectedVideoModel] = useState('')
  const [selectedStylePreset, setSelectedStylePreset] = useState('live-action-short')
  const [selectedMotionMode, setSelectedMotionMode] = useState<(typeof VIDEO_MOTION_OPTIONS)[number]['key']>('dynamic')
  const [selectedVideoMode, setSelectedVideoMode] = useState<'frame_animation' | 'api_generation'>('frame_animation')
  const [clipDurationSec, setClipDurationSec] = useState(5)
  const [videoModelAvailability, setVideoModelAvailability] = useState<Record<string, boolean>>({})


  const imageUrls = useMemo(() => parseLines(imageUrlsText), [imageUrlsText])
  const sceneDescriptions = useMemo(() => parseLines(sceneDescriptionsText), [sceneDescriptionsText])

  const { data: videoModelsData } = useSWR(
    'ad-video-models',
    () => modelAPI.list({ type: 'video', sort_by: 'priority' }) as unknown as Promise<{ data: Array<{ id: number; name: string; model_key: string; is_active: boolean }> }>
  )
  const allVideoModels = useMemo(
    () => (((videoModelsData as { data?: Array<{ id: number; name: string; model_key: string; is_active: boolean }> })?.data ?? [])
      .filter((item) => item.is_active && item.model_key)),
    [videoModelsData]
  )

  const { data: projectTasksRaw } = useSWR(
    activeProjectId ? ['ad-video-project-tasks', activeProjectId] : null,
    () => videoAPI.listTasks(activeProjectId as number, { page: 1, page_size: 100 }) as unknown as Promise<{ data?: { items?: VideoTaskSnapshot[] } }>,
    {
      refreshInterval: taskStatus === 'pending' || taskStatus === 'processing' ? 5000 : 30000,
      revalidateOnFocus: true,
    }
  )

  const projectTasks = ((projectTasksRaw as { data?: { items?: VideoTaskSnapshot[] } })?.data?.items ?? []) as VideoTaskSnapshot[]

  const sessionTasks = useMemo(() => {
    if (!sessionAnchorAt) return projectTasks
    const anchor = new Date(sessionAnchorAt).getTime() - 10000
    return projectTasks.filter((task) => {
      const created = String(task.created_at ?? '').trim()
      if (!created) return true
      return new Date(created).getTime() >= anchor
    })
  }, [projectTasks, sessionAnchorAt])

  const modelCompareRows = useMemo(() => {
    const bucket = new Map<string, {
      modelName: string
      total: number
      succeeded: number
      failed: number
      processing: number
      durationsSec: number[]
      latestOutputUrl: string
      latestUpdatedAt: number
      latestError: string
      failureReasons: Record<string, number>
    }>()

    for (const task of sessionTasks) {
      const modelName = String(task.model_name || 'unknown').trim() || 'unknown'
      const row = bucket.get(modelName) ?? {
        modelName,
        total: 0,
        succeeded: 0,
        failed: 0,
        processing: 0,
        durationsSec: [],
        latestOutputUrl: '',
        latestUpdatedAt: 0,
        latestError: '',
        failureReasons: {},
      }
      row.total += 1
      if (task.status === 'succeeded') row.succeeded += 1
      else if (task.status === 'failed') row.failed += 1
      else if (task.status === 'pending' || task.status === 'processing') row.processing += 1

      const createdAtMs = new Date(String(task.created_at || '')).getTime()
      const updatedAtMs = new Date(String(task.updated_at || task.created_at || '')).getTime()
      if (!Number.isNaN(createdAtMs) && !Number.isNaN(updatedAtMs) && updatedAtMs >= createdAtMs) {
        const durationSec = Math.round((updatedAtMs - createdAtMs) / 1000)
        if (durationSec > 0) row.durationsSec.push(durationSec)
      }

      const updatedAt = new Date(String(task.updated_at || task.created_at || '')).getTime()
      if (!Number.isNaN(updatedAt) && updatedAt >= row.latestUpdatedAt) {
        row.latestUpdatedAt = updatedAt
        row.latestOutputUrl = String(task.result_url || task.hls_url || '').trim()
        row.latestError = task.status === 'failed' ? String(task.error_msg || '') : row.latestError
      }

      if (task.status === 'failed' && task.error_msg) {
        row.latestError = String(task.error_msg)
        const reason = normalizeFailureReason(task.error_msg)
        row.failureReasons[reason] = (row.failureReasons[reason] ?? 0) + 1
      }

      bucket.set(modelName, row)
    }

    return Array.from(bucket.values()).map((row) => {
      const sortedDurations = [...row.durationsSec].sort((a, b) => a - b)
      const avgDurationSec = sortedDurations.length > 0
        ? Math.round(sortedDurations.reduce((sum, value) => sum + value, 0) / sortedDurations.length)
        : 0
      const p95DurationSec = sortedDurations.length > 0 ? percentile(sortedDurations, 95) : 0
      const successRate = row.total > 0 ? Math.round((row.succeeded / row.total) * 100) : 0
      const speedScore = avgDurationSec > 0 ? Math.max(0, 100 - Math.min(100, Math.round(avgDurationSec / 3))) : 50
      const stabilityPenalty = Math.min(30, row.failed * 5)
      const score = Math.max(0, Math.round(successRate * 0.7 + speedScore * 0.3 - stabilityPenalty))
      const topFailureReasons = Object.entries(row.failureReasons)
        .sort((a, b) => b[1] - a[1])
        .slice(0, 3)
        .map(([reason, count]) => ({ reason, count }))

      return {
        ...row,
        successRate,
        avgDurationSec,
        p95DurationSec,
        score,
        estimatedCostPerClip: Math.round(estimateCostFactor(row.modelName) * Math.max(2, avgDurationSec || 5) * 10) / 10,
        topFailureReasons,
      }
    })
      .sort((a, b) => {
        if (b.score !== a.score) return b.score - a.score
        if (b.successRate !== a.successRate) return b.successRate - a.successRate
        return b.total - a.total
      })
  }, [sessionTasks])

  const recommendedModel = modelCompareRows[0] ?? null

  const hourlyStats = useMemo(() => {
    const bucket = new Map<string, {
      hour: string
      total: number
      succeeded: number
      failed: number
      processing: number
      durations: number[]
    }>()

    for (const task of sessionTasks) {
      const createdRaw = String(task.created_at || '')
      const createdAt = new Date(createdRaw)
      if (Number.isNaN(createdAt.getTime())) continue
      const hour = `${createdAt.getHours().toString().padStart(2, '0')}:00`
      const row = bucket.get(hour) ?? {
        hour,
        total: 0,
        succeeded: 0,
        failed: 0,
        processing: 0,
        durations: [],
      }
      row.total += 1
      if (task.status === 'succeeded') row.succeeded += 1
      else if (task.status === 'failed') row.failed += 1
      else if (task.status === 'pending' || task.status === 'processing') row.processing += 1

      const createdMs = new Date(String(task.created_at || '')).getTime()
      const updatedMs = new Date(String(task.updated_at || task.created_at || '')).getTime()
      if (!Number.isNaN(createdMs) && !Number.isNaN(updatedMs) && updatedMs >= createdMs) {
        const duration = Math.round((updatedMs - createdMs) / 1000)
        if (duration > 0) row.durations.push(duration)
      }
      bucket.set(hour, row)
    }

    return Array.from(bucket.values())
      .map((row) => {
        const avgDurationSec = row.durations.length > 0
          ? Math.round(row.durations.reduce((sum, value) => sum + value, 0) / row.durations.length)
          : 0
        const successRate = row.total > 0 ? Math.round((row.succeeded / row.total) * 100) : 0
        return {
          ...row,
          avgDurationSec,
          successRate,
        }
      })
      .sort((a, b) => a.hour.localeCompare(b.hour))
  }, [sessionTasks])

  const recentTrend = useMemo(() => {
    const sorted = [...sessionTasks].sort((a, b) => {
      const at = new Date(String(a.created_at || '')).getTime()
      const bt = new Date(String(b.created_at || '')).getTime()
      return bt - at
    })
    const windowSize = Number(trendWindow)
    const recent = sorted.slice(0, windowSize)
    const total = recent.length
    const succeeded = recent.filter((task) => task.status === 'succeeded').length
    const failed = recent.filter((task) => task.status === 'failed').length
    const processing = recent.filter((task) => task.status === 'pending' || task.status === 'processing').length

    const durations = recent
      .map((task) => {
        const createdMs = new Date(String(task.created_at || '')).getTime()
        const updatedMs = new Date(String(task.updated_at || task.created_at || '')).getTime()
        if (Number.isNaN(createdMs) || Number.isNaN(updatedMs) || updatedMs < createdMs) return 0
        return Math.round((updatedMs - createdMs) / 1000)
      })
      .filter((value) => value > 0)

    const avgDurationSec = durations.length > 0
      ? Math.round(durations.reduce((sum, value) => sum + value, 0) / durations.length)
      : 0
    const successRate = total > 0 ? Math.round((succeeded / total) * 100) : 0

    return {
      windowSize,
      total,
      succeeded,
      failed,
      processing,
      successRate,
      avgDurationSec,
    }
  }, [sessionTasks, trendWindow])

  const currentHourStat = useMemo(() => {
    const hourKey = `${new Date().getHours().toString().padStart(2, '0')}:00`
    return hourlyStats.find((item) => item.hour === hourKey) ?? null
  }, [hourlyStats])

  const bestHourSuggestion = useMemo(() => {
    const candidates = hourlyStats.filter((item) => item.total >= 2)
    if (candidates.length === 0) return null
    return [...candidates].sort((a, b) => {
      if (b.successRate !== a.successRate) return b.successRate - a.successRate
      return a.avgDurationSec - b.avgDurationSec
    })[0]
  }, [hourlyStats])

  const failureReasonClusters = useMemo(() => {
    const merged: Record<string, number> = {}
    for (const row of modelCompareRows) {
      for (const entry of row.topFailureReasons) {
        merged[entry.reason] = (merged[entry.reason] ?? 0) + entry.count
      }
    }
    return Object.entries(merged)
      .sort((a, b) => b[1] - a[1])
      .slice(0, 6)
      .map(([reason, count]) => ({ reason, count }))
  }, [modelCompareRows])

  const overallCompareStats = useMemo(() => {
    const total = sessionTasks.length
    const succeeded = sessionTasks.filter((task) => task.status === 'succeeded').length
    const failed = sessionTasks.filter((task) => task.status === 'failed').length
    const processing = sessionTasks.filter((task) => task.status === 'pending' || task.status === 'processing').length
    const successRate = total > 0 ? Math.round((succeeded / total) * 100) : 0
    return { total, succeeded, failed, processing, successRate }
  }, [sessionTasks])

  useEffect(() => {
    videoAPI.modelStatus()
      .then((res) => {
        const models = (res as { models?: Array<{ key: string; available: boolean }> }).models ?? []
        const map: Record<string, boolean> = {}
        for (const model of models) map[model.key] = model.available
        setVideoModelAvailability(map)
      })
      .catch(() => {
        setVideoModelAvailability({})
      })
  }, [])

  const availableVideoModels = useMemo(() => {
    if (Object.keys(videoModelAvailability).length === 0) return allVideoModels
    return allVideoModels.filter((item) => videoModelAvailability[item.model_key] === true)
  }, [allVideoModels, videoModelAvailability])

  useEffect(() => {
    if (!selectedVideoModel && availableVideoModels.length > 0) {
      setSelectedVideoModel(availableVideoModels[0].model_key)
    }
  }, [availableVideoModels, selectedVideoModel])

  useEffect(() => {
    if (availableVideoModels.length === 0) {
      setBatchModelKeys((prev) => (prev.length === 0 ? prev : []))
      return
    }

    setBatchModelKeys((prev) => {
      const availableKeys = new Set(availableVideoModels.map((item) => item.model_key))
      const filtered = prev.filter((key) => availableKeys.has(key))
      if (filtered.length > 0) {
        if (filtered.length === prev.length && filtered.every((value, idx) => value === prev[idx])) {
          return prev
        }
        return filtered
      }
      const defaults = availableVideoModels.slice(0, 2).map((item) => item.model_key)
      if (defaults.length === prev.length && defaults.every((value, idx) => value === prev[idx])) {
        return prev
      }
      return defaults
    })
  }, [availableVideoModels])

  const applyTemplate = (templateKey: (typeof AD_TEMPLATES)[number]['key']) => {
    const template = AD_TEMPLATES.find((item) => item.key === templateKey)
    if (!template) return
    setSelectedTemplate(template.key)
    setSelectedStylePreset(template.style)
    setSelectedMotionMode(template.motion)
    setClipDurationSec(template.duration)
    setAdPrompt((prev) => {
      const trimmed = prev.trim()
      return trimmed ? `${trimmed}\n${template.promptSeed}` : template.promptSeed
    })
    toast({ title: `已应用模板：${template.label}`, description: template.hint, variant: 'success' })
  }

  const buildProjectTitle = () => {
    const trimmed = title.trim()
    if (trimmed) return trimmed
    return `广告视频-${new Date().toISOString().slice(0, 10)}`
  }

  const handleLocalFiles = (files: FileList | null) => {
    if (!files) return
    const next = Array.from(files).filter((file) => file.type.startsWith('image/'))
    if (next.length === 0) return
    setLocalFiles((prev) => [...prev, ...next])
  }

  const removeLocalFile = (idx: number) => {
    setLocalFiles((prev) => prev.filter((_, index) => index !== idx))
  }

  const triggerVideoGeneration = async (ctx: Omit<GenerationContext, 'startedAt'>) => {
    const startedAt = new Date().toISOString()
    setActiveProjectId(ctx.projectId)
    setActiveTaskId(null)
    setActiveTaskStartedAt(startedAt)
    setTaskStatus('pending')
    setTaskError('')
    setTaskOutputUrl('')
    setTaskClipProgress({ done: 0, total: 0 })

    await videoAPI.generate(ctx.projectId, {
      image_urls: ctx.imageUrls,
      scene_descriptions: ctx.sceneDescriptions,
      scene_description: ctx.prompt,
      style_preset: ctx.stylePreset,
      motion_mode: ctx.motionMode,
      video_mode: ctx.videoMode,
      model_name: ctx.modelName || undefined,
      clip_duration_sec: ctx.clipDurationSec,
    })

    setLastGenerationContext({ ...ctx, startedAt })
    setTriedModelKeys((prev) => {
      const next = prev.filter((item) => item !== ctx.modelName)
      next.push(ctx.modelName)
      return next.slice(-6)
    })
    if (lockedModelKey && ctx.modelName === lockedModelKey && lockedModelRemaining > 0) {
      setLockedModelRemaining((prev) => Math.max(0, prev - 1))
    }
  }

  const pickBackupModel = (exclude: string[]): string | null => {
    const all = availableVideoModels.map((item) => item.model_key)
    const candidate = all.find((key) => !exclude.includes(key))
    return candidate ?? null
  }

  const chooseModelForSubmission = (preferredModel: string): string => {
    if (lockedModelKey && lockedModelRemaining > 0) {
      const stillAvailable = availableVideoModels.some((item) => item.model_key === lockedModelKey)
      if (stillAvailable) return lockedModelKey
    }
    if (!autoAvoidLowHourEnabled) return preferredModel
    if (!currentHourStat) return preferredModel
    if (currentHourStat.total < 3) return preferredModel
    if (currentHourStat.successRate >= lowHourThreshold) return preferredModel

    if (recommendedModel?.modelName && recommendedModel.modelName !== preferredModel) {
      return recommendedModel.modelName
    }
    const backup = pickBackupModel([preferredModel])
    return backup ?? preferredModel
  }

  const downloadTextFile = (content: string, filename: string, mime: string) => {
    const blob = new Blob([content], { type: mime })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = filename
    a.click()
    URL.revokeObjectURL(url)
  }

  const ensureProjectIdForExtraction = async (): Promise<number | null> => {
    const existingProjectId = activeProjectId ?? lastGenerationContext?.projectId ?? null
    if (existingProjectId) return existingProjectId

    try {
      const autoTitle = title.trim() || `视频文案提取-${new Date().toISOString().slice(0, 16).replace('T', ' ')}`
      const createRes = (await projectAPI.create({
        title: autoTitle,
        description: '用于视频文案提取的自动创建项目',
        project_type: 'video',
        style_tags: ensureProjectMediaTag(DEFAULT_AD_TAGS, 'video'),
        target_episodes: 1,
        video_mode: selectedVideoMode,
        storyboard_config: {
          style_preset: selectedStylePreset,
          motion_mode: selectedMotionMode,
          duration: clipDurationSec,
          aspect_ratio: '16:9',
          resolution: '1080p',
        },
      } as never)) as { data?: { id?: number } }

      const createdProjectId = Number(createRes?.data?.id ?? 0)
      if (!createdProjectId) {
        toast({ title: '自动创建项目失败，请稍后重试', variant: 'destructive' })
        return null
      }

      setActiveProjectId(createdProjectId)
      toast({ title: '已自动创建项目并继续执行', variant: 'success' })
      return createdProjectId
    } catch {
      toast({ title: '自动创建项目失败，请稍后重试', variant: 'destructive' })
      return null
    }
  }


  const runCopyOptimization = async (): Promise<OptimizedAdResult | null> => {
    const premise = adPrompt.trim()
    if (premise.length < 10) {
      toast({ title: '请先输入足够详细的广告文案', variant: 'destructive' })
      return null
    }

    setOptimizingCopy(true)
    try {
      const taskRes = await taskAPI.create({
        task_type: 'script_quick_generate',
        payload: {
          mode: 'script',
          premise,
          genre: '广告短片',
          platform: '短视频投放',
          delivery_format: '分镜脚本+口播文案+结尾CTA',
          episode_duration: '15-45秒',
          tone: '明确卖点、节奏紧凑、转化导向',
          requirements: '输出可直接用于广告视频生成，包含镜头建议、产品卖点、情绪转折和行动号召',
          target_words: 600,
          chapter_count: 4,
        },
      }) as unknown as { data?: { id?: number } }

      const optimizeTaskId = Number(taskRes?.data?.id ?? 0)
      if (!optimizeTaskId) throw new Error('文案优化任务创建失败')

      const result = await new Promise<OptimizedAdResult>((resolve, reject) => {
        let elapsed = 0
        const timer = setInterval(async () => {
          elapsed += 3
          if (elapsed > 180) {
            clearInterval(timer)
            reject(new Error('文案优化超时，请稍后重试'))
            return
          }

          try {
            const taskResp = await taskAPI.get(optimizeTaskId) as unknown as {
              data?: {
                status?: string
                error_msg?: string
                result?: {
                  title?: string
                  content?: string
                  outline?: string[]
                  tags?: string[]
                }
              }
            }
            const task = taskResp?.data
            if (!task) return

            if (task.status === 'succeeded') {
              clearInterval(timer)
              const taskResult = task.result
              if (!taskResult?.content?.trim()) {
                reject(new Error('文案优化完成但结果为空'))
                return
              }
              resolve({
                title: String(taskResult.title ?? '').trim(),
                content: String(taskResult.content ?? '').trim(),
                outline: Array.isArray(taskResult.outline) ? taskResult.outline : [],
                tags: Array.isArray(taskResult.tags) ? taskResult.tags : [],
              })
            } else if (task.status === 'failed') {
              clearInterval(timer)
              reject(new Error(task.error_msg || '文案优化失败'))
            }
          } catch {
            // ignore transient polling errors
          }
        }, 3000)
      })

      setOptimizedScript(result.content)
      if (result.title) setTitle((prev) => prev.trim() || result.title)
      if (result.outline.length > 0 && sceneDescriptions.length === 0) {
        setSceneDescriptionsText(result.outline.join('\n'))
      }
      toast({ title: '文案优化完成', description: '已自动回填优化结果与分镜建议', variant: 'success' })
      return result
    } catch (err: unknown) {
      toast({
        title: '文案优化失败',
        description: err instanceof Error ? err.message : '请稍后重试',
        variant: 'destructive',
      })
      return null
    } finally {
      setOptimizingCopy(false)
    }
  }

  const handleCreateFromText = () => {
    const trimmedPrompt = (optimizedScript || adPrompt).trim()
    if (trimmedPrompt.length < 10) {
      toast({ title: '请至少输入 10 个字的广告文案', variant: 'destructive' })
      return
    }

    setCreatingByText(true)
    try {
      const draftId = savePendingProjectDraft({
        title: buildProjectTitle(),
        description: '由视频广告生成器创建',
        scriptContent: trimmedPrompt,
        scriptFileName: 'ad-script.txt',
        scriptMimeType: 'text/plain;charset=utf-8',
        targetEpisodes: 1,
        styleTags: DEFAULT_AD_TAGS,
        media: 'video',
      })

      toast({
        title: '文案已带入创建向导',
        description: '你可以继续调整模型与风格后创建项目',
        variant: 'success',
      })
      router.push(`/projects/new?media=video&draft=${encodeURIComponent(draftId)}`)
    } catch {
      toast({ title: '创建草稿失败，请稍后重试', variant: 'destructive' })
      setCreatingByText(false)
    }
  }

  const handleGenerateByImages = async () => {
    const basePrompt = adPrompt.trim()
    if (basePrompt.length < 10) {
      toast({ title: '请先输入广告文案，用于场景描述和视频语义', variant: 'destructive' })
      return
    }

    const optimized = autoOptimizeCopy ? await runCopyOptimization() : null
    const trimmedPrompt = (optimized?.content || optimizedScript || basePrompt).trim()
    if (trimmedPrompt.length < 10) {
      toast({ title: '请先输入广告文案，用于场景描述和视频语义', variant: 'destructive' })
      return
    }

    if (imageUrls.length === 0 && localFiles.length === 0) {
      toast({ title: '请至少提供 1 张图片（URL 或本地上传）', variant: 'destructive' })
      return
    }

    const invalidUrl = imageUrls.find((url) => !isHttpUrl(url))
    if (invalidUrl) {
      toast({ title: '存在无效图片 URL，请使用 http/https 链接', description: invalidUrl, variant: 'destructive' })
      return
    }

    setCreatingByImages(true)

    try {
      const projectTitle = buildProjectTitle()
      const createRes = (await projectAPI.create({
        title: projectTitle,
        description: '由视频广告生成器创建',
        project_type: 'video',
        style_tags: ensureProjectMediaTag(DEFAULT_AD_TAGS, 'video'),
        target_episodes: 1,
        video_mode: selectedVideoMode,
        storyboard_config: {
          style_preset: selectedStylePreset,
          motion_mode: selectedMotionMode,
          duration: clipDurationSec,
          aspect_ratio: '16:9',
          resolution: '1080p',
        },
      } as never)) as { data: { id: number } }

      const projectId = createRes.data.id
      setAutoRetryAttempts(0)
      autoRetryingRef.current = false
      setSessionAnchorAt(new Date().toISOString())
      setRetryHistory([])
      setBatchSubmittedCount(0)

      const scriptFile = new File([trimmedPrompt], 'ad-script.txt', {
        type: 'text/plain;charset=utf-8',
      })
      await projectAPI.uploadScript(projectId, scriptFile)

      const dedupedUrlSet = new Set(imageUrls)
      const finalImageUrls: string[] = [...dedupedUrlSet]

      for (const sourceFile of localFiles) {
        const file = enableLocalCompression
          ? await compressImage(sourceFile, maxImageSide, Math.max(0.3, Math.min(0.98, jpegQuality / 100)))
          : sourceFile

        const createdAssetRes = await assetAPI.create(projectId, {
          type: 'image',
          name: `广告图-${file.name}`,
          description: trimmedPrompt,
          is_manual: true,
        }) as unknown as { data?: { id?: number } }

        const assetId = Number(createdAssetRes?.data?.id ?? 0)
        if (!assetId) continue

        await assetAPI.upload(projectId, assetId, file)
        const assetRes = await assetAPI.get(projectId, assetId) as unknown as { data?: Asset }
        const resolvedUrl = normalizeImageUrlFromAsset(assetRes?.data)
        if (resolvedUrl && !dedupedUrlSet.has(resolvedUrl)) {
          dedupedUrlSet.add(resolvedUrl)
          finalImageUrls.push(resolvedUrl)
        }
      }

      if (finalImageUrls.length === 0) {
        throw new Error('图片处理后未得到可用图片地址')
      }

      const resolvedDescriptions = finalImageUrls.map((_, index) => {
        const line = sceneDescriptions[index] ?? optimized?.outline?.[index] ?? sceneDescriptions[sceneDescriptions.length - 1] ?? trimmedPrompt
        return line.trim()
      })

      await triggerVideoGeneration({
        projectId,
        projectTitle,
        prompt: trimmedPrompt,
        imageUrls: finalImageUrls,
        sceneDescriptions: resolvedDescriptions,
        modelName: chooseModelForSubmission(selectedVideoModel || (availableVideoModels[0]?.model_key ?? '')),
        stylePreset: selectedStylePreset,
        motionMode: selectedMotionMode,
        videoMode: selectedVideoMode,
        clipDurationSec,
      })

      toast({
        title: '广告视频生成已启动',
        description: `项目「${projectTitle}」已提交，系统将自动轮询直至生成完成`,
        variant: 'success',
      })
    } catch {
      toast({ title: '启动生成失败，请稍后重试', variant: 'destructive' })
      setTaskStatus('failed')
      setTaskError('启动生成失败，请稍后重试')
    } finally {
      setCreatingByImages(false)
    }
  }

  const handleRerunAnotherVersion = async () => {
    if (!lastGenerationContext) return
    const backupModel = pickBackupModel([lastGenerationContext.modelName, ...triedModelKeys])
    const rerunModel = backupModel ?? lastGenerationContext.modelName

    setManualRerunLoading(true)
    try {
      await triggerVideoGeneration({
        ...lastGenerationContext,
        modelName: rerunModel,
      })
      toast({
        title: '已启动 A/B 复投版本',
        description: `当前复投模型：${rerunModel}`,
        variant: 'success',
      })
    } catch {
      toast({ title: '复投失败，请稍后重试', variant: 'destructive' })
    } finally {
      setManualRerunLoading(false)
    }
  }

  const handleApplyRecommendedAndRerun = async () => {
    if (!lastGenerationContext || !recommendedModel) return
    setManualRerunLoading(true)
    try {
      setSelectedVideoModel(recommendedModel.modelName)
      await triggerVideoGeneration({
        ...lastGenerationContext,
        modelName: recommendedModel.modelName,
      })
      toast({
        title: '已套用推荐模型并复投',
        description: `当前模型：${recommendedModel.modelName}`,
        variant: 'success',
      })
    } catch {
      toast({ title: '推荐模型复投失败，请稍后重试', variant: 'destructive' })
    } finally {
      setManualRerunLoading(false)
    }
  }

  const toggleBatchModel = (key: string) => {
    setBatchModelKeys((prev) => {
      if (prev.includes(key)) {
        if (prev.length === 1) return prev
        return prev.filter((item) => item !== key)
      }
      if (prev.length >= 4) return prev
      return [...prev, key]
    })
  }

  const handleBatchGenerateVersions = async () => {
    if (!lastGenerationContext) return
    if (batchModelKeys.length === 0) {
      toast({ title: '请先选择至少 1 个模型', variant: 'destructive' })
      return
    }

    const models = batchModelKeys.slice(0, 4)
    setBatchGenerating(true)
    try {
      let submitted = 0
      for (const modelKey of models) {
        await videoAPI.generate(lastGenerationContext.projectId, {
          image_urls: lastGenerationContext.imageUrls,
          scene_descriptions: lastGenerationContext.sceneDescriptions,
          scene_description: lastGenerationContext.prompt,
          style_preset: lastGenerationContext.stylePreset,
          motion_mode: lastGenerationContext.motionMode,
          video_mode: lastGenerationContext.videoMode,
          model_name: modelKey,
          clip_duration_sec: lastGenerationContext.clipDurationSec,
        })
        submitted += 1
      }
      setBatchSubmittedCount((prev) => prev + submitted)
      setTaskStatus('pending')
      setActiveTaskId(null)
      setActiveTaskStartedAt(new Date().toISOString())
      setTaskError('')
      setTaskOutputUrl('')
      setTaskClipProgress({ done: 0, total: 0 })
      setTriedModelKeys((prev) => {
        const set = new Set(prev)
        for (const modelKey of models) set.add(modelKey)
        return Array.from(set).slice(-12)
      })
      toast({ title: '批量版本生成已提交', description: `已提交 ${submitted} 个模型版本`, variant: 'success' })
    } catch {
      toast({ title: '批量提交失败', description: '请稍后重试', variant: 'destructive' })
    } finally {
      setBatchGenerating(false)
    }
  }

  const handleExportPackage = () => {
    if (!lastGenerationContext) return
    setExportingPackage(true)
    try {
      const now = new Date().toISOString().replace(/[:.]/g, '-')
      const baseName = `ad-package-${lastGenerationContext.projectId}-${now}`

      const jsonPayload = {
        project_id: lastGenerationContext.projectId,
        project_title: lastGenerationContext.projectTitle,
        generated_at: new Date().toISOString(),
        model_name: lastGenerationContext.modelName,
        style_preset: lastGenerationContext.stylePreset,
        motion_mode: lastGenerationContext.motionMode,
        video_mode: lastGenerationContext.videoMode,
        clip_duration_sec: lastGenerationContext.clipDurationSec,
        prompt: lastGenerationContext.prompt,
        scene_descriptions: lastGenerationContext.sceneDescriptions,
        image_urls: lastGenerationContext.imageUrls,
        output_url: taskOutputUrl,
        status: taskStatus,
        model_availability_snapshot: videoModelAvailability,
        tried_model_keys: triedModelKeys,
        retry_history: retryHistory,
        auto_retry_enabled: autoRetryEnabled,
        auto_retry_attempts: autoRetryAttempts,
        batch_submitted_count: batchSubmittedCount,
      }

      const markdown = [
        '# 广告投放导出包',
        '',
        `- 项目ID: ${lastGenerationContext.projectId}`,
        `- 项目名称: ${lastGenerationContext.projectTitle}`,
        `- 生成时间: ${new Date().toLocaleString('zh-CN')}`,
        `- 模型: ${lastGenerationContext.modelName}`,
        `- 风格: ${lastGenerationContext.stylePreset}`,
        `- 运镜: ${lastGenerationContext.motionMode}`,
        `- 模式: ${lastGenerationContext.videoMode}`,
        `- 片段时长: ${lastGenerationContext.clipDurationSec}s`,
        `- 输出链接: ${taskOutputUrl || '未完成'}`,
        `- 自动重试: ${autoRetryEnabled ? '开启' : '关闭'}`,
        `- 自动重试次数: ${autoRetryAttempts}`,
        `- 批量提交总数: ${batchSubmittedCount}`,
        '',
        '## 广告文案',
        '',
        lastGenerationContext.prompt,
        '',
        '## 分镜描述',
        '',
        ...lastGenerationContext.sceneDescriptions.map((item, idx) => `${idx + 1}. ${item}`),
        '',
        '## 素材URL',
        '',
        ...lastGenerationContext.imageUrls.map((item, idx) => `${idx + 1}. ${item}`),
        '',
        '## 模型可用性快照',
        '',
        ...Object.keys(videoModelAvailability).length > 0
          ? Object.entries(videoModelAvailability).map(([key, ok]) => `- ${key}: ${ok ? 'available' : 'unavailable'}`)
          : ['- 未获取到模型可用性信息'],
        '',
        '## 重试链路',
        '',
        ...retryHistory.length > 0
          ? retryHistory.map((item, idx) => `${idx + 1}. ${item.timestamp} | ${item.fromModel} -> ${item.toModel} | ${item.reason} | ${item.status}`)
          : ['- 本次无自动重试记录'],
      ].join('\n')

      downloadTextFile(JSON.stringify(jsonPayload, null, 2), `${baseName}.json`, 'application/json;charset=utf-8')
      downloadTextFile(markdown, `${baseName}.md`, 'text/markdown;charset=utf-8')
      toast({ title: '投放导出包已下载', description: '已导出 JSON 和 Markdown 两份文件', variant: 'success' })
    } finally {
      setExportingPackage(false)
    }
  }

  const handleExportCompareReport = () => {
    if (!lastGenerationContext) return
    setCompareExporting(true)
    try {
      const now = new Date().toISOString().replace(/[:.]/g, '-')
      const baseName = `ad-compare-report-${lastGenerationContext.projectId}-${now}`
      const payload = {
        project_id: lastGenerationContext.projectId,
        generated_at: new Date().toISOString(),
        anchor_at: sessionAnchorAt,
        overall: overallCompareStats,
        recent_trend: recentTrend,
        by_model: modelCompareRows,
        by_hour: hourlyStats,
        best_hour_suggestion: bestHourSuggestion,
        failure_reason_clusters: failureReasonClusters,
        recommended_model: recommendedModel,
        retry_history: retryHistory,
        model_availability_snapshot: videoModelAvailability,
      }
      const markdown = [
        '# 广告多版本对比报告',
        '',
        `- 项目ID: ${lastGenerationContext.projectId}`,
        `- 统计样本: ${overallCompareStats.total}`,
        `- 成功数: ${overallCompareStats.succeeded}`,
        `- 失败数: ${overallCompareStats.failed}`,
        `- 进行中: ${overallCompareStats.processing}`,
        `- 总体成功率: ${overallCompareStats.successRate}%`,
        `- 最近${recentTrend.windowSize}条成功率: ${recentTrend.successRate}%`,
        `- 最近${recentTrend.windowSize}条平均耗时: ${recentTrend.avgDurationSec || 0}s`,
        `- 推荐模型: ${recommendedModel?.modelName ?? '暂无'}`,
        recommendedModel ? `- 推荐评分: ${recommendedModel.score}` : '- 推荐评分: -',
        recommendedModel ? `- 推荐模型成本指数: ${recommendedModel.estimatedCostPerClip}` : '- 推荐模型成本指数: -',
        bestHourSuggestion ? `- 建议投放时段: ${bestHourSuggestion.hour}（成功率 ${bestHourSuggestion.successRate}%）` : '- 建议投放时段: 暂无',
        '',
        '## 模型对比',
        '',
        ...modelCompareRows.flatMap((row) => [
          `### ${row.modelName}`,
          `- 任务数: ${row.total}`,
          `- 成功: ${row.succeeded}`,
          `- 失败: ${row.failed}`,
          `- 进行中: ${row.processing}`,
          `- 成功率: ${row.successRate}%`,
          `- 平均耗时: ${row.avgDurationSec || 0}s`,
          `- P95耗时: ${row.p95DurationSec || 0}s`,
          `- 评分: ${row.score}`,
          `- 估算单片成本指数: ${row.estimatedCostPerClip}`,
          `- 最新输出: ${row.latestOutputUrl || '无'}`,
          `- 最近失败原因: ${row.latestError || '无'}`,
          `- 失败原因Top: ${row.topFailureReasons.length > 0 ? row.topFailureReasons.map((item) => `${item.reason}(${item.count})`).join('，') : '无'}`,
          '',
        ]),
        '## 全局失败原因聚类',
        '',
        ...(failureReasonClusters.length > 0
          ? failureReasonClusters.map((item, idx) => `${idx + 1}. ${item.reason} (${item.count})`)
          : ['暂无失败样本']),
        '',
        '## 分时段表现',
        '',
        ...(hourlyStats.length > 0
          ? hourlyStats.map((item) => `- ${item.hour} | 样本 ${item.total} | 成功率 ${item.successRate}% | 平均耗时 ${item.avgDurationSec || 0}s`)
          : ['暂无分时段数据']),
      ].join('\n')

      downloadTextFile(JSON.stringify(payload, null, 2), `${baseName}.json`, 'application/json;charset=utf-8')
      downloadTextFile(markdown, `${baseName}.md`, 'text/markdown;charset=utf-8')
      toast({ title: '对比报告已导出', description: '已下载 JSON 和 Markdown', variant: 'success' })
    } finally {
      setCompareExporting(false)
    }
  }

  const handleExportDailyAdvice = () => {
    if (!lastGenerationContext) return
    setAdviceExporting(true)
    try {
      const now = new Date().toISOString().replace(/[:.]/g, '-')
      const baseName = `ad-daily-advice-${lastGenerationContext.projectId}-${now}`
      const advice = {
        generated_at: new Date().toISOString(),
        project_id: lastGenerationContext.projectId,
        recommended_model: recommendedModel?.modelName ?? null,
        recommended_model_score: recommendedModel?.score ?? null,
        recommended_model_cost_per_clip: recommendedModel?.estimatedCostPerClip ?? null,
        best_hour: bestHourSuggestion,
        current_hour: currentHourStat,
        trend: recentTrend,
        low_hour_strategy: {
          enabled: autoAvoidLowHourEnabled,
          threshold: lowHourThreshold,
        },
        top_failure_clusters: failureReasonClusters,
      }
      const markdown = [
        '# 今日投放建议单',
        '',
        `- 项目ID: ${lastGenerationContext.projectId}`,
        `- 生成时间: ${new Date().toLocaleString('zh-CN')}`,
        `- 推荐模型: ${recommendedModel?.modelName ?? '暂无'}`,
        `- 推荐评分: ${recommendedModel?.score ?? '-'}`,
        `- 估算单片成本指数: ${recommendedModel?.estimatedCostPerClip ?? '-'}`,
        `- 建议投放时段: ${bestHourSuggestion ? `${bestHourSuggestion.hour}（成功率 ${bestHourSuggestion.successRate}%）` : '暂无'}`,
        `- 最近${recentTrend.windowSize}条趋势: 成功率 ${recentTrend.successRate}% / 平均耗时 ${recentTrend.avgDurationSec || 0}s`,
        '',
        '## 风险提示',
        '',
        ...(failureReasonClusters.length > 0
          ? failureReasonClusters.map((item, idx) => `${idx + 1}. ${item.reason}（${item.count}）`)
          : ['暂无明显失败聚类']),
      ].join('\n')

      downloadTextFile(JSON.stringify(advice, null, 2), `${baseName}.json`, 'application/json;charset=utf-8')
      downloadTextFile(markdown, `${baseName}.md`, 'text/markdown;charset=utf-8')
      toast({ title: '今日投放建议单已导出', description: '已下载 JSON 和 Markdown', variant: 'success' })
    } finally {
      setAdviceExporting(false)
    }
  }

  useEffect(() => {
    if (!activeProjectId || !activeTaskStartedAt) return
    if (taskStatus === 'succeeded' || taskStatus === 'failed') return

    if (taskPollRef.current) clearInterval(taskPollRef.current)

    taskPollRef.current = setInterval(async () => {
      try {
        const response = await videoAPI.listTasks(activeProjectId, { page: 1, page_size: 20 }) as unknown as {
          data?: { items?: VideoTaskSnapshot[] }
        }
        const items = response?.data?.items ?? []
        if (!Array.isArray(items) || items.length === 0) return

        let target = activeTaskId
          ? items.find((item) => item.id === activeTaskId)
          : undefined

        if (!target) {
          target = items
            .filter((item) => {
              const created = String(item.created_at ?? '').trim()
              if (!created) return true
              return new Date(created).getTime() >= new Date(activeTaskStartedAt).getTime() - 10000
            })
            .sort((a, b) => new Date(String(b.created_at ?? '')).getTime() - new Date(String(a.created_at ?? '')).getTime())[0]
        }

        if (!target) return
        if (!activeTaskId) setActiveTaskId(target.id)

        const clipsTotal = target.clips?.length ?? target.image_urls?.length ?? 0
        const clipsDone = target.clips?.filter((clip) => clip.status === 'succeeded').length ?? 0
        setTaskClipProgress({ done: clipsDone, total: clipsTotal })

        if (target.status === 'succeeded') {
          setTaskStatus('succeeded')
          const outputUrl = String(target.result_url || target.hls_url || '').trim()
          setTaskOutputUrl(outputUrl)
          if (taskPollRef.current) {
            clearInterval(taskPollRef.current)
            taskPollRef.current = null
          }
          toast({ title: '广告视频生成完成', description: '可以直接预览或下载输出视频', variant: 'success' })
          return
        }

        if (target.status === 'failed') {
          const errorMessage = String(target.error_msg || '生成失败')
          setTaskStatus('failed')
          setTaskError(errorMessage)

          if (
            autoRetryEnabled
            && !autoRetryingRef.current
            && lastGenerationContext
            && autoRetryAttempts < 1
          ) {
            autoRetryingRef.current = true
            const backupModel = pickBackupModel([lastGenerationContext.modelName, ...triedModelKeys])
            if (backupModel) {
              try {
                setAutoRetryAttempts((prev) => prev + 1)
                await triggerVideoGeneration({
                  ...lastGenerationContext,
                  modelName: backupModel,
                })
                setRetryHistory((prev) => [
                  ...prev,
                  {
                    timestamp: new Date().toISOString(),
                    fromModel: lastGenerationContext.modelName,
                    toModel: backupModel,
                    reason: errorMessage,
                    status: 'submitted',
                  },
                ])
                toast({
                  title: '主模型失败，已自动切换模型重试',
                  description: `重试模型：${backupModel}`,
                  variant: 'default',
                })
                autoRetryingRef.current = false
                return
              } catch {
                setRetryHistory((prev) => [
                  ...prev,
                  {
                    timestamp: new Date().toISOString(),
                    fromModel: lastGenerationContext.modelName,
                    toModel: backupModel,
                    reason: errorMessage,
                    status: 'failed',
                  },
                ])
                // Keep failed status if retry also fails to submit.
              }
            }
            autoRetryingRef.current = false
          }

          if (taskPollRef.current) {
            clearInterval(taskPollRef.current)
            taskPollRef.current = null
          }
          return
        }

        if (target.status === 'processing' || target.status === 'pending') {
          setTaskStatus(target.status)
        }
      } catch {
        // ignore transient polling errors
      }
    }, 5000)

    return () => {
      if (taskPollRef.current) {
        clearInterval(taskPollRef.current)
        taskPollRef.current = null
      }
    }
  }, [activeProjectId, activeTaskId, activeTaskStartedAt, autoRetryAttempts, autoRetryEnabled, lastGenerationContext, taskStatus, toast, triedModelKeys])

  const openOutput = () => {
    if (!taskOutputUrl) return
    window.open(taskOutputUrl, '_blank', 'noopener,noreferrer')
  }

  return (
    <div className="mx-auto max-w-5xl space-y-6 pb-10">
      <div className="overflow-hidden rounded-[28px] border border-surface-200/70 bg-gradient-to-br from-slate-950 via-cyan-950 to-slate-900 p-6 text-white shadow-sm">
        <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
          <div className="max-w-2xl">
            <div className="mb-3 inline-flex items-center gap-2 rounded-full border border-white/10 bg-white/10 px-3 py-1.5 text-xs font-medium text-surface-100 backdrop-blur">
              <Megaphone className="h-3.5 w-3.5 text-cyan-300" />
              广告视频工作台
            </div>
            <h2 className="text-2xl font-semibold tracking-tight">文案 + 指定图片，一步生成广告视频</h2>
            <p className="mt-2 text-sm leading-6 text-surface-300">
              先写广告文案，再选择生成方式：
              <span className="text-cyan-200">文案驱动创建项目</span>
              或
              <span className="text-amber-200">按图片 URL 直接启动视频生成</span>。
            </p>
          </div>

          <div className="grid gap-3 sm:grid-cols-2">
            <div className="rounded-2xl border border-white/10 bg-white/10 p-4 backdrop-blur">
              <p className="text-xs uppercase tracking-[0.2em] text-surface-300">方式 A</p>
              <p className="mt-2 text-base font-semibold text-white">文案生成项目</p>
              <p className="mt-1 text-xs text-surface-400">进入创建页继续调整参数</p>
            </div>
            <div className="rounded-2xl border border-white/10 bg-white/10 p-4 backdrop-blur">
              <p className="text-xs uppercase tracking-[0.2em] text-surface-300">方式 B</p>
              <p className="mt-2 text-base font-semibold text-white">指定图片直出视频</p>
              <p className="mt-1 text-xs text-surface-400">创建项目后直接触发视频任务</p>
            </div>
          </div>
        </div>
      </div>

      <Card className="overflow-hidden rounded-[24px] border-surface-200 shadow-sm">
        <CardContent className="space-y-6 bg-gradient-to-b from-white to-surface-50/60 pt-6 text-surface-900">
          <div className="grid gap-5 lg:grid-cols-2">
            <div className="space-y-2">
              <Label htmlFor="ad-title">项目名称（可选）</Label>
              <Input
                id="ad-title"
                placeholder="例如：618 夏季清凉饮料投放"
                value={title}
                onChange={(e) => setTitle(e.target.value)}
              />
            </div>
            <div className="rounded-xl border border-cyan-200 bg-cyan-50/70 p-4">
              <p className="flex items-center gap-2 text-sm font-medium text-cyan-800">
                <Sparkles className="h-4 w-4" />
                小提示
              </p>
              <p className="mt-1 text-xs leading-5 text-cyan-700">
                广告文案里建议包含：产品卖点、目标人群、品牌语气、行动号召（CTA）。
              </p>
            </div>
          </div>

          <div className="space-y-2">
            <Label htmlFor="ad-prompt">广告文案</Label>
            <Textarea
              id="ad-prompt"
              rows={7}
              placeholder="请输入广告文案，例如：主打“0糖0脂”的夏季气泡饮，受众为 18-30 岁白领，风格轻快明亮，结尾强调“限时第二件半价”。"
              value={adPrompt}
              onChange={(e) => setAdPrompt(e.target.value)}
            />
            <div className="flex flex-wrap items-center gap-3 pt-1">
              <div className="flex items-center gap-2 text-xs text-surface-600">
                <Switch checked={autoOptimizeCopy} onCheckedChange={setAutoOptimizeCopy} />
                生成前自动优化文案
              </div>
              <Button
                type="button"
                variant="outline"
                size="sm"
                className="h-8 gap-1.5"
                onClick={runCopyOptimization}
                disabled={optimizingCopy || creatingByImages}
              >
                {optimizingCopy ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Sparkles className="h-3.5 w-3.5" />}
                先优化文案
              </Button>
            </div>
            {optimizedScript ? (
              <div className="rounded-xl border border-emerald-200 bg-emerald-50/60 p-3">
                <p className="text-xs font-medium text-emerald-700">优化后文案（已用于生成）</p>
                <p className="mt-1 line-clamp-4 text-xs leading-5 text-emerald-800">{optimizedScript}</p>
              </div>
            ) : null}
          </div>

          <div className="flex items-center justify-between rounded-xl border border-cyan-100 bg-cyan-50/40 p-4">
            <div>
              <p className="text-sm font-medium text-cyan-900">需要从已有视频提取文案？</p>
              <p className="mt-1 text-xs text-cyan-700">可使用视频工具区，自动转写本地或在线视频的画面和解说，再复制回来使用。</p>
            </div>
            <Button asChild variant="outline" size="sm" className="h-8 border-cyan-200 text-cyan-800 hover:bg-cyan-100 hover:text-cyan-900">
              <Link href="/tools/video">去工具区提取 &raquo;</Link>
            </Button>
          </div>


          <div className="space-y-3 rounded-xl border border-surface-200 bg-white p-4">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <div>
                <p className="text-sm font-medium text-surface-800">广告模板与高级参数</p>
                <p className="text-xs text-surface-500">模板可快速套用投放场景，参数会直接影响最终视频生成质量与风格。</p>
              </div>
            </div>

            <div className="flex flex-wrap gap-2">
              {AD_TEMPLATES.map((template) => {
                const active = selectedTemplate === template.key
                return (
                  <button
                    key={template.key}
                    type="button"
                    onClick={() => applyTemplate(template.key)}
                    className={[
                      'rounded-full border px-3 py-1.5 text-xs font-medium transition-colors',
                      active
                        ? 'border-cyan-300 bg-cyan-50 text-cyan-800'
                        : 'border-surface-200 bg-white text-surface-600 hover:border-cyan-200 hover:bg-cyan-50/40',
                    ].join(' ')}
                  >
                    {template.label}
                  </button>
                )
              })}
            </div>

            <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-5">
              <div className="space-y-1 xl:col-span-2">
                <Label className="text-xs text-surface-700">视频模型</Label>
                <Select value={selectedVideoModel} onValueChange={setSelectedVideoModel}>
                  <SelectTrigger>
                    <SelectValue placeholder="选择视频模型" />
                  </SelectTrigger>
                  <SelectContent>
                    {availableVideoModels.length > 0 ? (
                      availableVideoModels.map((model) => (
                        <SelectItem key={model.id} value={model.model_key}>
                          {model.name}
                        </SelectItem>
                      ))
                    ) : (
                      <SelectItem value="__no_model__" disabled>暂无可用模型</SelectItem>
                    )}
                  </SelectContent>
                </Select>
              </div>

              <div className="space-y-1">
                <Label className="text-xs text-surface-700">风格</Label>
                <Select value={selectedStylePreset} onValueChange={setSelectedStylePreset}>
                  <SelectTrigger>
                    <SelectValue placeholder="选择风格" />
                  </SelectTrigger>
                  <SelectContent>
                    {VIDEO_STYLE_PRESETS.map((style) => (
                      <SelectItem key={style.key} value={style.key}>
                        {style.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              <div className="space-y-1">
                <Label className="text-xs text-surface-700">运镜</Label>
                <Select value={selectedMotionMode} onValueChange={(value) => setSelectedMotionMode(value as (typeof VIDEO_MOTION_OPTIONS)[number]['key'])}>
                  <SelectTrigger>
                    <SelectValue placeholder="选择运镜" />
                  </SelectTrigger>
                  <SelectContent>
                    {VIDEO_MOTION_OPTIONS.map((motion) => (
                      <SelectItem key={motion.key} value={motion.key}>
                        {motion.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              <div className="space-y-1">
                <Label className="text-xs text-surface-700">片段时长（秒）</Label>
                <Input
                  type="number"
                  min={2}
                  max={10}
                  value={clipDurationSec}
                  onChange={(e) => setClipDurationSec(Number(e.target.value || 5))}
                />
              </div>
            </div>

            <div className="grid gap-4 md:grid-cols-2">
              <div className="space-y-1">
                <Label className="text-xs text-surface-700">生成模式</Label>
                <Select value={selectedVideoMode} onValueChange={(value) => setSelectedVideoMode(value as 'frame_animation' | 'api_generation')}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="frame_animation">frame_animation</SelectItem>
                    <SelectItem value="api_generation">api_generation</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="rounded-lg border border-surface-200 bg-surface-50 p-3 text-xs text-surface-600">
                当前参数会用于：项目默认 storyboard_config + 本次 videoAPI.generate 请求。
              </div>
            </div>

            <div className="grid gap-4 md:grid-cols-2">
              <div className="flex items-center gap-2 text-xs text-surface-600">
                <Switch checked={autoAvoidLowHourEnabled} onCheckedChange={setAutoAvoidLowHourEnabled} />
                低成功率时段自动切换推荐/备选模型
              </div>
              <div className="space-y-1">
                <Label className="text-xs text-surface-700">低成功率阈值（%）</Label>
                <Input
                  type="number"
                  min={20}
                  max={95}
                  value={lowHourThreshold}
                  onChange={(e) => setLowHourThreshold(Number(e.target.value || 65))}
                />
              </div>
            </div>

            <div className="flex items-center gap-2 text-xs text-surface-600">
              <Switch checked={autoRetryEnabled} onCheckedChange={setAutoRetryEnabled} />
              失败时自动切换备用模型重试（最多 1 次）
            </div>
          </div>

          <div className="grid gap-6 lg:grid-cols-2">
            <div className="space-y-2">
              <Label htmlFor="image-urls" className="flex items-center gap-2">
                <ImageIcon className="h-4 w-4" />
                指定图片 URL（每行一个）
              </Label>
              <Textarea
                id="image-urls"
                rows={8}
                placeholder={'https://cdn.example.com/ad-shot-1.jpg\nhttps://cdn.example.com/ad-shot-2.jpg'}
                value={imageUrlsText}
                onChange={(e) => setImageUrlsText(e.target.value)}
              />
              <p className="text-xs text-surface-500">已识别 {imageUrls.length} 张图片 URL</p>
            </div>

            <div className="space-y-2">
              <Label htmlFor="scene-lines">分镜描述（可选，每行对应一张图）</Label>
              <Textarea
                id="scene-lines"
                rows={8}
                placeholder={'开场特写：冰块与饮料碰撞，突出清凉感\n中景：年轻人聚会举杯，传达社交氛围'}
                value={sceneDescriptionsText}
                onChange={(e) => setSceneDescriptionsText(e.target.value)}
              />
              <p className="text-xs text-surface-500">
                未填写时会默认使用广告文案作为场景描述。
              </p>
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
                  onChange={(event) => handleLocalFiles(event.target.files)}
                />
              </label>
            </div>

            <div className="grid gap-4 md:grid-cols-3">
              <div className="flex items-center gap-2 text-xs text-surface-600">
                <Switch checked={enableLocalCompression} onCheckedChange={setEnableLocalCompression} />
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
                  onChange={(e) => setMaxImageSide(Number(e.target.value || 1920))}
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
                  onChange={(e) => setJpegQuality(Number(e.target.value || 88))}
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
                        onClick={() => removeLocalFile(idx)}
                      >
                        删除
                      </button>
                    </div>
                  ))}
                </div>
              </div>
            ) : null}
          </div>

          <div className="grid gap-3 sm:grid-cols-2">
            <Button
              type="button"
              onClick={handleCreateFromText}
              disabled={creatingByText || creatingByImages}
              className="h-11 gap-2"
            >
              {creatingByText ? <Loader2 className="h-4 w-4 animate-spin" /> : <Wand2 className="h-4 w-4" />}
              文案生成项目
              <ArrowRight className="h-4 w-4" />
            </Button>
            <Button
              type="button"
              variant="outline"
              onClick={handleGenerateByImages}
              disabled={creatingByImages || creatingByText}
              className="h-11 gap-2 border-cyan-200 bg-cyan-50 text-cyan-800 hover:bg-cyan-100"
            >
              {creatingByImages ? <Loader2 className="h-4 w-4 animate-spin" /> : <ImageIcon className="h-4 w-4" />}
              指定图片直接生成
              <ArrowRight className="h-4 w-4" />
            </Button>
          </div>

          {activeProjectId ? (
            <div className="rounded-xl border border-cyan-200 bg-cyan-50/60 p-4">
              <div className="flex flex-wrap items-center justify-between gap-3">
                <div>
                  <p className="text-sm font-medium text-cyan-900">广告视频输出状态</p>
                  <p className="mt-1 text-xs text-cyan-700">
                    项目 ID: {activeProjectId}
                    {activeTaskId ? ` · 任务 ID: ${activeTaskId}` : ''}
                    {taskClipProgress.total > 0 ? ` · 片段 ${taskClipProgress.done}/${taskClipProgress.total}` : ''}
                  </p>
                </div>
                <div className="flex items-center gap-2">
                  <span className="rounded-full border border-cyan-300 bg-white px-3 py-1 text-xs font-medium text-cyan-800">
                    {taskStatus === 'idle' && '等待开始'}
                    {taskStatus === 'pending' && '排队中'}
                    {taskStatus === 'processing' && '生成中'}
                    {taskStatus === 'succeeded' && '已完成'}
                    {taskStatus === 'failed' && '失败'}
                  </span>
                  <Button size="sm" variant="outline" onClick={() => router.push(`/projects/${activeProjectId}`)}>
                    打开项目
                  </Button>
                </div>
              </div>

              {taskStatus === 'failed' && taskError ? (
                <p className="mt-2 text-xs text-rose-600">{taskError}</p>
              ) : null}

              {autoRetryAttempts > 0 ? (
                <p className="mt-2 text-xs text-cyan-700">已执行自动重试次数：{autoRetryAttempts}</p>
              ) : null}

              {taskStatus === 'succeeded' && taskOutputUrl ? (
                <div className="mt-3 flex flex-wrap gap-2">
                  <Button size="sm" className="gap-1.5" onClick={openOutput}>
                    <Download className="h-3.5 w-3.5" />
                    预览/下载成片
                  </Button>
                  <Button
                    size="sm"
                    variant="outline"
                    className="gap-1.5"
                    onClick={handleRerunAnotherVersion}
                    disabled={manualRerunLoading}
                  >
                    {manualRerunLoading ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Repeat className="h-3.5 w-3.5" />}
                    再生成一个版本
                  </Button>
                  <Button
                    size="sm"
                    variant="outline"
                    className="gap-1.5"
                    onClick={handleExportPackage}
                    disabled={exportingPackage}
                  >
                    {exportingPackage ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Download className="h-3.5 w-3.5" />}
                    导出投放包
                  </Button>
                  <a href={taskOutputUrl} target="_blank" rel="noopener noreferrer" className="inline-flex items-center rounded-md border border-cyan-300 bg-white px-3 py-1.5 text-xs font-medium text-cyan-800 hover:bg-cyan-50">
                    新标签打开输出链接
                  </a>
                </div>
              ) : null}

              {lastGenerationContext ? (
                <div className="mt-3 space-y-2 rounded-lg border border-cyan-200 bg-white/70 p-3">
                  <p className="text-xs font-medium text-cyan-900">批量多版本生成（2-4 模型）</p>
                  <div className="flex flex-wrap gap-2">
                    {availableVideoModels.map((model) => {
                      const checked = batchModelKeys.includes(model.model_key)
                      return (
                        <button
                          key={`batch-${model.id}`}
                          type="button"
                          onClick={() => toggleBatchModel(model.model_key)}
                          className={[
                            'rounded-full border px-2.5 py-1 text-xs',
                            checked
                              ? 'border-cyan-300 bg-cyan-50 text-cyan-800'
                              : 'border-surface-200 bg-white text-surface-600',
                          ].join(' ')}
                        >
                          {model.name}
                        </button>
                      )
                    })}
                  </div>
                  <div className="flex flex-wrap items-center gap-2">
                    <Button
                      size="sm"
                      variant="outline"
                      className="gap-1.5"
                      onClick={handleBatchGenerateVersions}
                      disabled={batchGenerating || batchModelKeys.length === 0}
                    >
                      {batchGenerating ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Repeat className="h-3.5 w-3.5" />}
                      批量多版本生成
                    </Button>
                    <span className="text-xs text-cyan-700">已选 {batchModelKeys.length} 个模型 · 累计批量提交 {batchSubmittedCount}</span>
                  </div>
                </div>
              ) : null}

              {modelCompareRows.length > 0 ? (
                <div className="mt-3 space-y-3 rounded-lg border border-cyan-200 bg-white/80 p-3">
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <p className="text-xs font-medium text-cyan-900">多版本结果看板</p>
                    <div className="flex items-center gap-2">
                      <span className="text-xs text-cyan-700">
                        样本 {overallCompareStats.total} · 成功率 {overallCompareStats.successRate}%
                      </span>
                      <Button
                        size="sm"
                        variant="outline"
                        className="gap-1.5"
                        onClick={handleExportCompareReport}
                        disabled={compareExporting}
                      >
                        {compareExporting ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Download className="h-3.5 w-3.5" />}
                        导出对比报告
                      </Button>
                      <Button
                        size="sm"
                        variant="outline"
                        className="gap-1.5"
                        onClick={handleExportDailyAdvice}
                        disabled={adviceExporting}
                      >
                        {adviceExporting ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Download className="h-3.5 w-3.5" />}
                        导出今日建议单
                      </Button>
                    </div>
                  </div>
                  {recommendedModel ? (
                    <div className="rounded-md border border-emerald-200 bg-emerald-50/60 px-3 py-2 text-xs text-emerald-800">
                      <div className="flex flex-wrap items-center justify-between gap-2">
                        <span>
                          推荐模型：{recommendedModel.modelName} · 评分 {recommendedModel.score} · 成功率 {recommendedModel.successRate}% · 平均耗时 {recommendedModel.avgDurationSec || 0}s · 成本指数 {recommendedModel.estimatedCostPerClip}
                        </span>
                        <div className="flex items-center gap-2">
                          <Button
                            size="sm"
                            variant="outline"
                            className="h-7 border-emerald-300 bg-white text-emerald-800 hover:bg-emerald-100"
                            onClick={() => {
                              setSelectedVideoModel(recommendedModel.modelName)
                              toast({ title: `已套用推荐模型：${recommendedModel.modelName}`, variant: 'success' })
                            }}
                          >
                            一键套用
                          </Button>
                          <Button
                            size="sm"
                            variant="outline"
                            className="h-7 border-emerald-300 bg-white text-emerald-800 hover:bg-emerald-100"
                            onClick={handleApplyRecommendedAndRerun}
                            disabled={manualRerunLoading}
                          >
                            {manualRerunLoading ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : '套用并复投'}
                          </Button>
                          <div className="flex items-center gap-1 rounded-md border border-emerald-300 bg-white px-1 py-0.5">
                            <Input
                              type="number"
                              min={1}
                              max={20}
                              value={lockRunsInput}
                              onChange={(e) => setLockRunsInput(Number(e.target.value || 3))}
                              className="h-6 w-14 border-0 px-1 text-xs"
                            />
                            <Button
                              size="sm"
                              variant="outline"
                              className="h-6 border-emerald-300 px-2 text-xs text-emerald-800 hover:bg-emerald-100"
                              onClick={() => {
                                const runs = Math.max(1, Math.min(20, lockRunsInput))
                                setLockedModelKey(recommendedModel.modelName)
                                setLockedModelRemaining(runs)
                                setSelectedVideoModel(recommendedModel.modelName)
                                toast({ title: `已锁定推荐模型 ${recommendedModel.modelName}`, description: `将优先用于接下来 ${runs} 次生成`, variant: 'success' })
                              }}
                            >
                              锁定N次
                            </Button>
                          </div>
                        </div>
                      </div>
                      {lockedModelKey ? (
                        <p className="mt-1 text-xs text-emerald-700">当前锁定：{lockedModelKey} · 剩余 {lockedModelRemaining} 次</p>
                      ) : null}
                    </div>
                  ) : null}
                  <div className="rounded-md border border-surface-200 bg-surface-50 px-3 py-2 text-xs text-surface-700">
                    <div className="flex flex-wrap items-center justify-between gap-2">
                      <span>最近{recentTrend.windowSize}条趋势：样本 {recentTrend.total} · 成功率 {recentTrend.successRate}% · 平均耗时 {recentTrend.avgDurationSec || 0}s · 失败 {recentTrend.failed}</span>
                      <Select value={trendWindow} onValueChange={(value) => setTrendWindow(value as '10' | '20' | '50')}>
                        <SelectTrigger className="h-7 w-24 bg-white text-xs">
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="10">最近10</SelectItem>
                          <SelectItem value="20">最近20</SelectItem>
                          <SelectItem value="50">最近50</SelectItem>
                        </SelectContent>
                      </Select>
                    </div>
                  </div>
                  {currentHourStat ? (
                    <div className="rounded-md border border-surface-200 bg-surface-50 px-3 py-2 text-xs text-surface-700">
                      当前时段 {currentHourStat.hour}：样本 {currentHourStat.total} · 成功率 {currentHourStat.successRate}% · 平均耗时 {currentHourStat.avgDurationSec || 0}s
                      {autoAvoidLowHourEnabled && currentHourStat.total >= 3 && currentHourStat.successRate < lowHourThreshold
                        ? ` · 已启用自动避坑阈值 ${lowHourThreshold}%`
                        : ''}
                    </div>
                  ) : null}
                  <div className="grid gap-2 md:grid-cols-2">
                    {modelCompareRows.map((row) => (
                      <div key={`compare-${row.modelName}`} className="rounded-md border border-surface-200 bg-white p-3 text-xs">
                        <p className="font-medium text-surface-800">{row.modelName}</p>
                        <p className="mt-1 text-surface-600">任务 {row.total} · 成功 {row.succeeded} · 失败 {row.failed} · 进行中 {row.processing}</p>
                        <p className="mt-1 text-cyan-700">成功率 {row.successRate}% · 评分 {row.score}</p>
                        <p className="mt-1 text-surface-600">平均耗时 {row.avgDurationSec || 0}s · P95 {row.p95DurationSec || 0}s · 成本指数 {row.estimatedCostPerClip}</p>
                        {row.latestOutputUrl ? (
                          <a href={row.latestOutputUrl} target="_blank" rel="noopener noreferrer" className="mt-1 inline-flex text-cyan-700 hover:text-cyan-800">
                            查看最新输出
                          </a>
                        ) : null}
                        {row.latestError ? (
                          <p className="mt-1 text-rose-600 line-clamp-2">最近失败: {row.latestError}</p>
                        ) : null}
                        {row.topFailureReasons.length > 0 ? (
                          <p className="mt-1 text-rose-700">失败聚类: {row.topFailureReasons.map((item) => `${item.reason}(${item.count})`).join('，')}</p>
                        ) : null}
                      </div>
                    ))}
                  </div>
                  {failureReasonClusters.length > 0 ? (
                    <div className="rounded-md border border-surface-200 bg-surface-50 px-3 py-2 text-xs text-surface-700">
                      全局失败原因: {failureReasonClusters.map((item) => `${item.reason}(${item.count})`).join('，')}
                    </div>
                  ) : null}
                  {hourlyStats.length > 0 ? (
                    <div className="rounded-md border border-surface-200 bg-surface-50 px-3 py-2 text-xs text-surface-700">
                      <p className="font-medium text-surface-800">分时段稳定性</p>
                      <div className="mt-1 flex flex-wrap gap-2">
                        {hourlyStats.map((item) => (
                          <span key={`hour-${item.hour}`} className="rounded-full border border-surface-300 bg-white px-2 py-0.5">
                            {item.hour} 成功率 {item.successRate}% · 均耗时 {item.avgDurationSec || 0}s
                          </span>
                        ))}
                      </div>
                    </div>
                  ) : null}
                </div>
              ) : null}

              {(taskStatus === 'pending' || taskStatus === 'processing') ? (
                <p className="mt-2 flex items-center gap-1.5 text-xs text-cyan-700">
                  <RefreshCw className="h-3.5 w-3.5 animate-spin" />
                  正在自动轮询任务结果，生成完成后会直接显示下载入口。
                </p>
              ) : null}
            </div>
          ) : null}
        </CardContent>
      </Card>
    </div>
  )
}
