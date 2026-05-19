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
import { assetAPI, chatAPI, modelAPI, projectAPI, storageAPI, taskAPI, videoAPI } from '@/lib/api'
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
import { CurrentTaskPanel } from '@/components/ad-video/CurrentTaskPanel'
import { GenerationQueuePanel } from '@/components/ad-video/GenerationQueuePanel'
import { LocalHistoryPanel } from '@/components/ad-video/LocalHistoryPanel'
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

const STORYBOARD_TEMPLATES = [
  {
    key: 'product-reveal',
    label: '产品开场',
    hint: '适合先展示产品本体，再用使用场景和收尾 CTA 完成转化。',
    sceneLines: [
      '开场产品特写：直接展示品牌主视觉与核心卖点。',
      '功能细节镜头：突出材质、界面或使用方式。',
      '真实使用场景：让目标用户看到自己在画面里的样子。',
      '收尾 CTA：强化优惠、购买或下载行动。',
    ],
    dialogueLines: [
      '先把最强卖点讲出来。',
      '再补一条能感知到的功能优势。',
      '把用户放进真实使用场景里。',
      '最后明确行动号召。',
    ],
    referenceLines: [
      '白底产品特写 / 主视觉海报',
      '功能细节近景 / 包装或界面截图',
      '人物手持使用 / 场景化照片',
      '品牌结尾海报 / 优惠 CTA 图',
    ],
  },
  {
    key: 'pain-solution',
    label: '痛点解决',
    hint: '适合先抛出痛点，再给出解决方案和结果对比。',
    sceneLines: [
      '痛点开场：展示用户当前遇到的困扰。',
      '方案登场：让产品作为解决方案出现。',
      '结果对比：突出使用前后变化。',
      '行动号召：引导立即体验或购买。',
    ],
    dialogueLines: [
      '这个问题是不是你也遇到过？',
      '我们用这个方案直接解决。',
      '前后变化一眼就能看懂。',
      '现在就去试试。',
    ],
    referenceLines: [
      '问题场景抓拍 / 用户痛点画面',
      '产品解决方案图 / 功能演示截图',
      '前后对比拼图 / 结果对照图',
      '下载页 / 购买按钮 / 优惠弹窗',
    ],
  },
  {
    key: 'social-proof',
    label: '口碑转化',
    hint: '适合用评价、测评和真实反馈增强信任。',
    sceneLines: [
      '用户口碑开场：先给出好评或评分。',
      '真实测评镜头：展示产品在手里的状态。',
      '结果反馈：补充用户使用后的感受。',
      '品牌收尾：统一品牌信息与 CTA。',
    ],
    dialogueLines: [
      '大家都在夸的点，先看这里。',
      '实测一下，效果很直接。',
      '用户反馈和结果都很清晰。',
      '想要同款，马上行动。',
    ],
    referenceLines: [
      '评分截图 / 评论区高赞图',
      '实拍测评 / 近景手持图',
      '用户反馈截图 / 对比图',
      '品牌收口海报 / CTA 图',
    ],
  },
] as const

const BRAND_VOICE_TEMPLATES = [
  {
    key: 'premium',
    label: '高端质感',
    hint: '适合强调质感、克制和品牌信任的广告。',
    directive: '品牌语气要克制、干净、稍有留白，突出高级感和可信度。',
    contrast: '更适合美妆、消费电子、高客单价品牌。',
  },
  {
    key: 'youthful',
    label: '年轻活力',
    hint: '适合轻快、社交感和即时反馈强的广告。',
    directive: '品牌语气要轻快、口语化、带一点社交感，结尾 CTA 要直接。',
    contrast: '更适合饮料、零食、APP 拉新和短视频投放。',
  },
  {
    key: 'expert',
    label: '专业可信',
    hint: '适合功能说明、工具类和知识型产品。',
    directive: '品牌语气要专业、清楚、避免夸张，用事实和功能点建立信任。',
    contrast: '更适合工具、科技、教育和 B2B 内容。',
  },
  {
    key: 'promo',
    label: '促销直给',
    hint: '适合活动投放、限时促销和转化导向广告。',
    directive: '品牌语气要直接、明确、转化导向强，少修辞，多利益点和行动号召。',
    contrast: '更适合活动节点、优惠券和强 CTA 场景。',
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

type TaskProgressRecord = {
  id?: number
  task_id: number
  progress: number
  message: string
  status: string
  timestamp: number
  created_at?: string
}

type GenerationContext = {
  projectId: number
  projectTitle: string
  prompt: string
  imageUrls: string[]
  sceneDescriptions: string[]
  storyboardTemplate: string
  referenceImageHints: string[]
  brandVoiceTemplate: string
  brandVoiceNotes: string
  modelName: string
  stylePreset: string
  motionMode: (typeof VIDEO_MOTION_OPTIONS)[number]['key']
  videoMode: 'frame_animation' | 'api_generation'
  clipDurationSec: number
  targetMarket: string
  subtitleLanguage: string
  creativeMode: string
  directorNote: string
  subtitleText: string
  dialogues: string[]
  startedAt: string
}

type RetryRecord = {
  timestamp: string
  fromModel: string
  toModel: string
  reason: string
  status: 'submitted' | 'failed'
}

type AdTaskLogEntry = {
  at: string
  level: 'info' | 'progress' | 'success' | 'warning' | 'error'
  message: string
}

const TARGET_MARKET_OPTIONS = [
  {
    key: 'cn-mainland',
    label: '中国大陆',
    prompt: '使用大陆短视频广告口吻，优先本地化消费场景、直接卖点和明确 CTA，避免泛国际化表达。',
  },
  {
    key: 'global-en',
    label: '海外英语',
    prompt: 'Use natural market-local English copy, avoid literal translation, and keep the CTA concise and persuasive.',
  },
  {
    key: 'sea',
    label: '东南亚',
    prompt: '使用容易理解的本地化营销文案，强调价格感、利益点和直接转化，不要过度文艺化。',
  },
] as const

const SUBTITLE_LANGUAGE_OPTIONS = [
  {
    key: 'zh-CN',
    label: '中文',
    prompt: '字幕、口播与镜头文案全部使用中文，句子短一点，便于烧录和 TTS 对齐。',
  },
  {
    key: 'en-US',
    label: '英文',
    prompt: 'Subtitle and spoken lines should be in natural English; avoid direct translation and keep the lines short.',
  },
  {
    key: 'bilingual',
    label: '中英双语',
    prompt: '字幕按中英双语输出，优先保证中文卖点不丢失，同时保留英文可投放版本。',
  },
] as const

const CREATIVE_MODE_OPTIONS = [
  {
    key: 'market-first',
    label: '市场优先',
    prompt: '优先匹配目标市场，不要把脚本自动改成过于泛化的广告腔。',
  },
  {
    key: 'script-preserved',
    label: '文案保真',
    prompt: '尽量保留用户原文的卖点和节奏，只做必要的整理，不要重写核心卖点。',
  },
  {
    key: 'director-led',
    label: '导演强化',
    prompt: '允许强化镜头感和节奏，但不要偏离品牌信息和目标市场。',
  },
] as const

const AD_VIDEO_DRAFT_STORAGE_KEY = 'autovideo:ad-video-draft:v1'
const AD_VIDEO_HISTORY_STORAGE_KEY = 'autovideo:ad-video-history:v1'

type AdVideoDraftSnapshot = {
  title: string
  adPrompt: string
  optimizedScript: string
  imageUrlsText: string
  sceneDescriptionsText: string
  referenceImageHintsText: string
  brandVoiceNotesText: string
  targetMarket: string
  subtitleLanguage: string
  creativeMode: string
  directorNote: string
  subtitleText: string
  selectedTemplate: string
  selectedStoryboardTemplate: string
  selectedBrandVoiceTemplate: string
  selectedVideoModel: string
  selectedStylePreset: string
  selectedMotionMode: (typeof VIDEO_MOTION_OPTIONS)[number]['key']
  selectedVideoMode: 'frame_animation' | 'api_generation'
  clipDurationSec: number
  autoOptimizeCopy: boolean
  enableLocalCompression: boolean
  maxImageSide: number
  jpegQuality: number
  autoAvoidLowHourEnabled: boolean
  lowHourThreshold: number
  autoRetryEnabled: boolean
}

type AdReviewChecklistItem = {
  key: string
  label: string
  passed: boolean
  detail: string
  blocking: boolean
}

type AdVideoHistoryEntry = {
  id: string
  savedAt: string
  label: string
  state: AdVideoDraftSnapshot
}

type AdGenerationTaskStatus = 'queued' | 'optimizing' | 'uploading' | 'submitting' | 'running' | 'succeeded' | 'failed'

type AdGenerationTaskEntry = {
  id: string
  createdAt: string
  updatedAt: string
  label: string
  status: AdGenerationTaskStatus
  step: string
  projectId?: number
  outputUrl?: string
  error?: string
  title: string
  marketLabel: string
  brandVoiceLabel: string
  storyboardLabel: string
  subtitleCount: number
  imageCount: number
}

type StoryboardPreviewItem = {
  index: number
  scene: string
  dialogue: string
  referenceHint: string
  imageSource: string
}

type BrandVoiceTemplateKey = (typeof BRAND_VOICE_TEMPLATES)[number]['key']

function splitSubtitleScript(raw: string): string[] {
  const normalized = String(raw || '').replace(/\r\n/g, '\n').trim()
  if (!normalized) return []

  const directLines = normalized
    .split('\n')
    .map((line) => line.replace(/^\s*(?:\d+[.)]|[-•*])\s*/, '').trim())
    .filter(Boolean)

  if (directLines.length > 0) {
    return directLines
  }

  return normalized
    .split(/[。！？!?；;]+/)
    .map((line) => line.replace(/^\s*(?:\d+[.)]|[-•*])\s*/, '').trim())
    .filter(Boolean)
}

function splitEditableLines(raw: string): string[] {
  return String(raw || '').replace(/\r\n/g, '\n').split('\n')
}

function updateLineAtIndex(raw: string, index: number, nextValue: string): string {
  const lines = splitEditableLines(raw)
  while (lines.length <= index) {
    lines.push('')
  }
  lines[index] = nextValue
  return lines.join('\n')
}

function distributeDialogues(lines: readonly string[], clipCount: number): string[] {
  if (clipCount <= 0) return []

  const normalized = lines.map((line) => line.trim()).filter(Boolean)
  if (normalized.length === 0) {
    return Array.from({ length: clipCount }, () => '')
  }

  if (normalized.length === 1) {
    return Array.from({ length: clipCount }, () => normalized[0])
  }

  const result: string[] = []
  for (let index = 0; index < clipCount; index += 1) {
    const start = Math.floor((index * normalized.length) / clipCount)
    const end = Math.max(start + 1, Math.floor(((index + 1) * normalized.length) / clipCount))
    const chunk = normalized.slice(start, end)
    result.push(chunk.join(' ').trim() || normalized[Math.min(start, normalized.length - 1)] || '')
  }
  return result
}

function buildMarketDirective(
  marketKey: string,
  subtitleLanguageKey: string,
  creativeModeKey: string,
  directorNote: string,
  brandVoiceKey: string,
  brandVoiceNotes: string,
): string {
  const marketOption = TARGET_MARKET_OPTIONS.find((item) => item.key === marketKey) ?? TARGET_MARKET_OPTIONS[0]
  const subtitleLanguageOption = SUBTITLE_LANGUAGE_OPTIONS.find((item) => item.key === subtitleLanguageKey) ?? SUBTITLE_LANGUAGE_OPTIONS[0]
  const creativeModeOption = CREATIVE_MODE_OPTIONS.find((item) => item.key === creativeModeKey) ?? CREATIVE_MODE_OPTIONS[0]
  const brandVoiceOption = BRAND_VOICE_TEMPLATES.find((item) => item.key === brandVoiceKey) ?? BRAND_VOICE_TEMPLATES[0]
  const note = directorNote.trim()
  const voiceNote = brandVoiceNotes.trim()

  return [
    `目标市场：${marketOption.label}`,
    marketOption.prompt,
    `字幕语言：${subtitleLanguageOption.label}`,
    subtitleLanguageOption.prompt,
    `创意模式：${creativeModeOption.label}`,
    creativeModeOption.prompt,
    `品牌语气：${brandVoiceOption.label}`,
    brandVoiceOption.directive,
    brandVoiceOption.contrast,
    voiceNote ? `品牌语气补充：${voiceNote}` : '',
    note ? `导演备注：${note}` : '',
    '要求：字幕、口播和镜头说明要一一对应，保持品牌卖点，不要把本地市场脚本自动改写成泛化风格。',
  ].filter(Boolean).join('\n')
}

function getTargetMarketLabelSafe(marketKey: string): string {
  return TARGET_MARKET_OPTIONS.find((item) => item.key === marketKey)?.label ?? TARGET_MARKET_OPTIONS[0].label
}

function readAdVideoDraft(): { savedAt: string | null; state: Partial<AdVideoDraftSnapshot> | null } {
  if (typeof window === 'undefined') {
    return { savedAt: null, state: null }
  }

  try {
    const raw = window.localStorage.getItem(AD_VIDEO_DRAFT_STORAGE_KEY)
    if (!raw) {
      return { savedAt: null, state: null }
    }

    const parsed = JSON.parse(raw) as { savedAt?: string; state?: Partial<AdVideoDraftSnapshot> }
    return {
      savedAt: typeof parsed.savedAt === 'string' ? parsed.savedAt : null,
      state: parsed && typeof parsed === 'object' ? (parsed.state ?? null) : null,
    }
  } catch {
    return { savedAt: null, state: null }
  }
}

function writeAdVideoDraft(state: AdVideoDraftSnapshot): string {
  if (typeof window === 'undefined') {
    return ''
  }

  const savedAt = new Date().toISOString()
  window.localStorage.setItem(AD_VIDEO_DRAFT_STORAGE_KEY, JSON.stringify({
    savedAt,
    state,
  }))
  return savedAt
}

function readAdVideoHistory(): AdVideoHistoryEntry[] {
  if (typeof window === 'undefined') {
    return []
  }

  try {
    const raw = window.localStorage.getItem(AD_VIDEO_HISTORY_STORAGE_KEY)
    if (!raw) return []
    const parsed = JSON.parse(raw)
    if (!Array.isArray(parsed)) return []
    return parsed.filter((entry) => entry && typeof entry === 'object') as AdVideoHistoryEntry[]
  } catch {
    return []
  }
}

function writeAdVideoHistory(entries: AdVideoHistoryEntry[]) {
  if (typeof window === 'undefined') {
    return
  }

  window.localStorage.setItem(AD_VIDEO_HISTORY_STORAGE_KEY, JSON.stringify(entries.slice(0, 8)))
}

function clearAdVideoDraft() {
  if (typeof window === 'undefined') {
    return
  }

  window.localStorage.removeItem(AD_VIDEO_DRAFT_STORAGE_KEY)
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
  const [targetMarket, setTargetMarket] = useState<string>(TARGET_MARKET_OPTIONS[0].key)
  const [subtitleLanguage, setSubtitleLanguage] = useState<string>(SUBTITLE_LANGUAGE_OPTIONS[0].key)
  const [creativeMode, setCreativeMode] = useState<string>(CREATIVE_MODE_OPTIONS[0].key)
  const [directorNote, setDirectorNote] = useState('')
  const [subtitleText, setSubtitleText] = useState('')
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
  const [selectedStoryboardTemplate, setSelectedStoryboardTemplate] = useState<string>(STORYBOARD_TEMPLATES[0].key)
  const [selectedBrandVoiceTemplate, setSelectedBrandVoiceTemplate] = useState<BrandVoiceTemplateKey>(BRAND_VOICE_TEMPLATES[0].key)
  const [selectedVideoModel, setSelectedVideoModel] = useState('')
  const [selectedStylePreset, setSelectedStylePreset] = useState('live-action-short')
  const [selectedMotionMode, setSelectedMotionMode] = useState<(typeof VIDEO_MOTION_OPTIONS)[number]['key']>('dynamic')
  const [selectedVideoMode, setSelectedVideoMode] = useState<'frame_animation' | 'api_generation'>('frame_animation')
  const [clipDurationSec, setClipDurationSec] = useState(5)
  const [videoModelAvailability, setVideoModelAvailability] = useState<Record<string, boolean>>({})
  const [draftHydrated, setDraftHydrated] = useState(false)
  const [draftSavedAt, setDraftSavedAt] = useState<string | null>(null)
  const [referenceImageHintsText, setReferenceImageHintsText] = useState('')
  const [brandVoiceNotesText, setBrandVoiceNotesText] = useState('')
  const [historyEntries, setHistoryEntries] = useState<AdVideoHistoryEntry[]>([])
  const [selectedHistoryEntryId, setSelectedHistoryEntryId] = useState('')
  const [generationTasks, setGenerationTasks] = useState<AdGenerationTaskEntry[]>([])
  const [activeGenerationTaskId, setActiveGenerationTaskId] = useState('')
  const [adTaskLogs, setAdTaskLogs] = useState<AdTaskLogEntry[]>([])
  const [activeOptimizeTaskId, setActiveOptimizeTaskId] = useState<number | null>(null)
  const [referenceHintGeneratingAll, setReferenceHintGeneratingAll] = useState(false)
  const [referenceHintGeneratingIndex, setReferenceHintGeneratingIndex] = useState<number | null>(null)
  const [reviewConfirmed, setReviewConfirmed] = useState(false)
  const [pendingDraftRestore, setPendingDraftRestore] = useState<Partial<AdVideoDraftSnapshot> | null>(null)
  const adTaskLogProgressRef = useRef<Set<string>>(new Set())
  const adTaskStatusRef = useRef<'idle' | 'pending' | 'processing' | 'succeeded' | 'failed'>('idle')


  const imageUrls = useMemo(() => parseLines(imageUrlsText), [imageUrlsText])
  const sceneDescriptions = useMemo(() => parseLines(sceneDescriptionsText), [sceneDescriptionsText])
  const subtitleLines = useMemo(() => splitSubtitleScript(subtitleText), [subtitleText])
  const referenceImageHints = useMemo(() => parseLines(referenceImageHintsText), [referenceImageHintsText])
  const selectedStoryboardTemplateMeta = useMemo(
    () => STORYBOARD_TEMPLATES.find((item) => item.key === selectedStoryboardTemplate) ?? STORYBOARD_TEMPLATES[0],
    [selectedStoryboardTemplate],
  )
  const selectedBrandVoiceTemplateMeta = useMemo(
    () => BRAND_VOICE_TEMPLATES.find((item) => item.key === selectedBrandVoiceTemplate) ?? BRAND_VOICE_TEMPLATES[0],
    [selectedBrandVoiceTemplate],
  )
  const selectedHistoryEntry = useMemo(
    () => historyEntries.find((entry) => entry.id === selectedHistoryEntryId) ?? null,
    [historyEntries, selectedHistoryEntryId],
  )
  const appendAdTaskLog = (message: string, level: AdTaskLogEntry['level'] = 'info') => {
    setAdTaskLogs((prev) => [
      ...prev,
      {
        at: new Date().toISOString(),
        level,
        message,
      },
    ].slice(-24))
  }
  const resetAdTaskLogs = (message: string) => {
    adTaskLogProgressRef.current = new Set()
    adTaskStatusRef.current = 'idle'
    setActiveOptimizeTaskId(null)
    setAdTaskLogs([{
      at: new Date().toISOString(),
      level: 'info',
      message,
    }])
  }
  const ingestBackendProgress = (records: TaskProgressRecord[]) => {
    if (records.length === 0) return
    setAdTaskLogs((prev) => {
      const next = [...prev]
      const seen = new Set(prev.map((item) => `${item.at}|${item.level}|${item.message}`))
      const sorted = [...records].sort((a, b) => a.timestamp - b.timestamp)
      for (const record of sorted) {
        const at = record.created_at || new Date(record.timestamp).toISOString()
        const key = `${at}|${record.progress}|${record.message}`
        if (adTaskLogProgressRef.current.has(key) || seen.has(`${at}|progress|${record.message}`)) {
          continue
        }
        adTaskLogProgressRef.current.add(key)
        next.push({
          at,
          level: 'progress',
          message: `${record.progress}% · ${record.message}`.trim(),
        })
      }
      return next.slice(-24)
    })
  }
  const brandVoiceBrief = useMemo(() => {
    const notes = brandVoiceNotesText.trim()
    return [
      `品牌语气：${selectedBrandVoiceTemplateMeta.label}`,
      selectedBrandVoiceTemplateMeta.directive,
      notes ? `补充要求：${notes}` : '',
    ].filter(Boolean).join('\n')
  }, [brandVoiceNotesText, selectedBrandVoiceTemplateMeta])
  const storyboardPreview = useMemo<StoryboardPreviewItem[]>(() => {
    const clipCount = Math.max(
      imageUrls.length,
      sceneDescriptions.length,
      subtitleLines.length,
      referenceImageHints.length,
      selectedStoryboardTemplateMeta.sceneLines.length,
      selectedStoryboardTemplateMeta.referenceLines.length,
      localFiles.length,
      1,
    )
    const fallbackSceneText = optimizedScript.trim() || adPrompt.trim()
    const fallbackDialogueText = subtitleText.trim()
    return Array.from({ length: clipCount }, (_, index) => {
      const scene = sceneDescriptions[index]
        ?? sceneDescriptions[sceneDescriptions.length - 1]
        ?? selectedStoryboardTemplateMeta.sceneLines[index]
        ?? selectedStoryboardTemplateMeta.sceneLines[selectedStoryboardTemplateMeta.sceneLines.length - 1]
        ?? fallbackSceneText
        ?? ''
      const dialogue = subtitleLines[index]
        ?? subtitleLines[subtitleLines.length - 1]
        ?? selectedStoryboardTemplateMeta.dialogueLines[index]
        ?? selectedStoryboardTemplateMeta.dialogueLines[selectedStoryboardTemplateMeta.dialogueLines.length - 1]
        ?? fallbackDialogueText
        ?? ''
      const referenceHint = referenceImageHints[index]
        ?? referenceImageHints[referenceImageHints.length - 1]
        ?? selectedStoryboardTemplateMeta.referenceLines[index]
        ?? selectedStoryboardTemplateMeta.referenceLines[selectedStoryboardTemplateMeta.referenceLines.length - 1]
        ?? ''
      const imageSource = imageUrls[index]
        ?? localFiles[index]?.name
        ?? imageUrls[imageUrls.length - 1]
        ?? localFiles[localFiles.length - 1]?.name
        ?? '待补图片'

      return {
        index,
        scene,
        dialogue,
        referenceHint,
        imageSource,
      }
    })
  }, [adPrompt, imageUrls, localFiles, optimizedScript, referenceImageHints, sceneDescriptions, selectedStoryboardTemplateMeta, subtitleLines, subtitleText])

  const readChatReply = (payload: unknown): string => {
    const data = payload as {
      data?: { reply?: string; parts?: Array<{ type?: string; text?: string }> }
      reply?: string
      parts?: Array<{ type?: string; text?: string }>
    }
    const reply = String(data?.data?.reply ?? data?.reply ?? '').trim()
    if (reply) return reply
    const parts = data?.data?.parts ?? data?.parts ?? []
    return parts
      .filter((part) => part?.type === 'text' && typeof part.text === 'string')
      .map((part) => String(part.text).trim())
      .filter(Boolean)
      .join('\n')
      .trim()
  }

  const normalizeReferenceHintLine = (text: string): string =>
    text
      .replace(/^\s*(?:\d+[.)]|[-•*])\s*/, '')
      .replace(/^["'“”]+|["'“”]+$/g, '')
      .trim()

  const buildReferenceHintPrompt = (shot: StoryboardPreviewItem, total: number) => [
    '你是广告分镜参考图提示词助手。',
    '请根据广告文案、分镜描述和口播，生成适合 AI 画图的“镜头参考图提示词”。',
    '要求：',
    '1. 只输出 1 行中文提示词，不要编号，不要解释。',
    '2. 侧重主体、构图、场景、光影、情绪，不要写成完整文案。',
    '3. 尽量控制在 12 到 24 个字。',
    `当前广告标题：${title.trim() || '未命名广告'}`,
    `当前广告文案：${adPrompt.trim() || optimizedScript.trim() || '暂无'}`,
    `当前目标市场：${getTargetMarketLabel()}`,
    `当前分镜模板：${selectedStoryboardTemplateMeta.label}`,
    `镜头 ${shot.index + 1}/${total}`,
    `分镜描述：${shot.scene || '暂无'}`,
    `字幕 / 口播：${shot.dialogue || '暂无'}`,
    `现有参考提示：${shot.referenceHint || '暂无'}`,
  ].join('\n')

  const fillReferenceHintAtIndex = async (shot: StoryboardPreviewItem) => {
    if (referenceHintGeneratingAll || referenceHintGeneratingIndex !== null) return
    setReferenceHintGeneratingIndex(shot.index)
    try {
      const res = await chatAPI.sendGemini([
        { role: 'system', content: '你是一个擅长把分镜转成画图提示词的助手。' },
        { role: 'user', content: buildReferenceHintPrompt(shot, storyboardPreview.length) },
      ]) as unknown as { data?: unknown }
      const reply = normalizeReferenceHintLine(readChatReply(res.data))
      if (!reply) throw new Error('AI 未返回有效参考图提示')
      setReferenceImageHintsText((prev) => updateLineAtIndex(prev, shot.index, reply))
      toast({ title: '已补全参考图提示', description: `镜头 ${shot.index + 1} 的参考图提示已生成`, variant: 'success' })
    } catch (err: unknown) {
      toast({
        title: '参考图提示生成失败',
        description: err instanceof Error ? err.message : '请稍后重试',
        variant: 'destructive',
      })
    } finally {
      setReferenceHintGeneratingIndex(null)
    }
  }

  const fillReferenceHintsForAll = async () => {
    if (storyboardPreview.length === 0 || referenceHintGeneratingAll || referenceHintGeneratingIndex !== null) return
    setReferenceHintGeneratingAll(true)
    try {
      const prompt = [
        '你是广告分镜参考图提示词助手。',
        '请一次性为下面每个镜头输出 1 行中文参考图提示词。',
        '要求：',
        '1. 每行对应一个镜头，禁止编号、禁止解释、禁止空行。',
        '2. 每行尽量 12 到 24 个字，侧重主体、构图、场景、光影、情绪。',
        '3. 只输出提示词本身，不要写完整文案。',
        `当前广告标题：${title.trim() || '未命名广告'}`,
        `当前广告文案：${adPrompt.trim() || optimizedScript.trim() || '暂无'}`,
        `当前目标市场：${getTargetMarketLabel()}`,
        `当前分镜模板：${selectedStoryboardTemplateMeta.label}`,
        '',
        ...storyboardPreview.map((shot) => [
          `镜头 ${shot.index + 1}/${storyboardPreview.length}`,
          `分镜描述：${shot.scene || '暂无'}`,
          `字幕 / 口播：${shot.dialogue || '暂无'}`,
          `现有参考提示：${shot.referenceHint || '暂无'}`,
          '',
        ].join('\n')),
      ].join('\n')

      const res = await chatAPI.sendGemini([
        { role: 'system', content: '你是一个擅长把分镜转成画图提示词的助手。' },
        { role: 'user', content: prompt },
      ]) as unknown as { data?: unknown }
      const replyLines = readChatReply(res.data)
        .split(/\r?\n/)
        .map(normalizeReferenceHintLine)
        .filter(Boolean)

      if (replyLines.length === 0) throw new Error('AI 未返回有效参考图提示')

      setReferenceImageHintsText((prev) => {
        let next = prev
        storyboardPreview.forEach((shot, idx) => {
          const fallback = replyLines[Math.min(idx, replyLines.length - 1)] ?? ''
          if (fallback) next = updateLineAtIndex(next, shot.index, fallback)
        })
        return next
      })
      toast({ title: '已补全全部参考图提示', description: '可继续手动微调每个镜头的提示词', variant: 'success' })
    } catch (err: unknown) {
      toast({
        title: '参考图提示生成失败',
        description: err instanceof Error ? err.message : '请稍后重试',
        variant: 'destructive',
      })
    } finally {
      setReferenceHintGeneratingAll(false)
    }
  }
  const adReviewChecklist = useMemo<AdReviewChecklistItem[]>(() => {
    const promptText = adPrompt.trim()
    const subtitleCount = subtitleLines.length
    const sceneCount = sceneDescriptions.length
    const mediaCount = imageUrls.length + localFiles.length
    const subtitleReady = subtitleCount > 0 || subtitleText.trim().length > 0
    const referenceReady = referenceImageHints.length > 0
    const brandVoiceReady = Boolean(selectedBrandVoiceTemplate)
    const storyboardAligned = subtitleCount === 0 || sceneCount === 0 || Math.abs(sceneCount - subtitleCount) <= 1
    const directorNoteReady = directorNote.trim().length > 0
    const marketLabel = TARGET_MARKET_OPTIONS.find((item) => item.key === targetMarket)?.label ?? TARGET_MARKET_OPTIONS[0].label
    const creativeLabel = CREATIVE_MODE_OPTIONS.find((item) => item.key === creativeMode)?.label ?? CREATIVE_MODE_OPTIONS[0].label

    return [
      {
        key: 'prompt',
        label: '广告文案',
        passed: promptText.length >= 10,
        detail: promptText.length >= 10 ? `已填写 ${promptText.length} 字` : '至少输入 10 个字',
        blocking: true,
      },
      {
        key: 'assets',
        label: '图片素材',
        passed: mediaCount > 0,
        detail: mediaCount > 0 ? `已准备 ${mediaCount} 份图片素材` : '需要至少 1 张图片 URL 或本地图片',
        blocking: true,
      },
      {
        key: 'market',
        label: '目标市场',
        passed: Boolean(targetMarket),
        detail: marketLabel,
        blocking: false,
      },
      {
        key: 'subtitle',
        label: '字幕 / 台词',
        passed: subtitleReady,
        detail: subtitleReady ? `已准备 ${subtitleCount} 条台词` : '还未填写台词，生成时会用广告文案兜底',
        blocking: false,
      },
      {
        key: 'storyboard',
        label: '分镜对齐',
        passed: storyboardAligned,
        detail: storyboardAligned ? '分镜与台词数量基本一致' : `分镜 ${sceneCount} 行 / 台词 ${subtitleCount} 行`,
        blocking: false,
      },
      {
        key: 'directive',
        label: '导演备注',
        passed: directorNoteReady,
        detail: directorNoteReady ? '已填写' : '建议补充市场禁忌与口播要求',
        blocking: false,
      },
      {
        key: 'creative',
        label: '创意模式',
        passed: Boolean(creativeMode),
        detail: creativeLabel,
        blocking: false,
      },
      {
        key: 'reference',
        label: '镜头参考图',
        passed: referenceReady,
        detail: referenceReady ? `已填写 ${referenceImageHints.length} 条参考提示` : `可用「${selectedStoryboardTemplateMeta.label}」模板快速补齐`,
        blocking: false,
      },
      {
        key: 'brand-voice',
        label: '品牌语气',
        passed: brandVoiceReady,
        detail: `${selectedBrandVoiceTemplateMeta.label} · ${selectedBrandVoiceTemplateMeta.hint}`,
        blocking: false,
      },
    ]
  }, [adPrompt, brandVoiceBrief, creativeMode, directorNote, imageUrls.length, localFiles.length, referenceImageHints.length, sceneDescriptions.length, selectedBrandVoiceTemplate, selectedBrandVoiceTemplateMeta.hint, selectedBrandVoiceTemplateMeta.label, selectedStoryboardTemplateMeta.label, subtitleLines.length, subtitleText, targetMarket])
  const blockingReviewItems = adReviewChecklist.filter((item) => item.blocking && !item.passed)
  const advisoryReviewItems = adReviewChecklist.filter((item) => !item.blocking && !item.passed)
  const reviewReady = reviewConfirmed && blockingReviewItems.length === 0
  const currentVersionSummary = useMemo(() => ({
    promptLength: adPrompt.trim().length,
    subtitleCount: splitSubtitleScript(subtitleText).length,
    sceneCount: parseLines(sceneDescriptionsText).length,
    referenceCount: parseLines(referenceImageHintsText).length,
    market: getTargetMarketLabelSafe(targetMarket),
    brandVoice: BRAND_VOICE_TEMPLATES.find((item) => item.key === selectedBrandVoiceTemplate)?.label ?? BRAND_VOICE_TEMPLATES[0].label,
    storyboard: STORYBOARD_TEMPLATES.find((item) => item.key === selectedStoryboardTemplate)?.label ?? STORYBOARD_TEMPLATES[0].label,
  }), [adPrompt, referenceImageHintsText, sceneDescriptionsText, selectedBrandVoiceTemplate, selectedStoryboardTemplate, subtitleText, targetMarket])
  const selectedHistorySummary = useMemo(() => {
    if (!selectedHistoryEntry) return null
    return {
      promptLength: selectedHistoryEntry.state.adPrompt.trim().length,
      subtitleCount: splitSubtitleScript(selectedHistoryEntry.state.subtitleText).length,
      sceneCount: parseLines(selectedHistoryEntry.state.sceneDescriptionsText).length,
      referenceCount: parseLines(selectedHistoryEntry.state.referenceImageHintsText).length,
      market: getTargetMarketLabelSafe(selectedHistoryEntry.state.targetMarket),
      brandVoice: BRAND_VOICE_TEMPLATES.find((item) => item.key === selectedHistoryEntry.state.selectedBrandVoiceTemplate)?.label ?? BRAND_VOICE_TEMPLATES[0].label,
      storyboard: STORYBOARD_TEMPLATES.find((item) => item.key === selectedHistoryEntry.state.selectedStoryboardTemplate)?.label ?? STORYBOARD_TEMPLATES[0].label,
    }
  }, [selectedHistoryEntry])
  const adVideoDraftState = useMemo<AdVideoDraftSnapshot>(() => ({
    title,
    adPrompt,
    optimizedScript,
    imageUrlsText,
    sceneDescriptionsText,
    referenceImageHintsText,
    brandVoiceNotesText,
    targetMarket,
    subtitleLanguage,
    creativeMode,
    directorNote,
    subtitleText,
    selectedTemplate,
    selectedStoryboardTemplate,
    selectedBrandVoiceTemplate,
    selectedVideoModel,
    selectedStylePreset,
    selectedMotionMode,
    selectedVideoMode,
    clipDurationSec,
    autoOptimizeCopy,
    enableLocalCompression,
    maxImageSide,
    jpegQuality,
    autoAvoidLowHourEnabled,
    lowHourThreshold,
    autoRetryEnabled,
  }), [
    adPrompt,
    autoAvoidLowHourEnabled,
    autoOptimizeCopy,
    autoRetryEnabled,
    clipDurationSec,
    creativeMode,
    directorNote,
    enableLocalCompression,
    imageUrlsText,
    jpegQuality,
    lowHourThreshold,
    maxImageSide,
    optimizedScript,
    sceneDescriptionsText,
    referenceImageHintsText,
    brandVoiceNotesText,
    selectedMotionMode,
    selectedStylePreset,
    selectedTemplate,
    selectedStoryboardTemplate,
    selectedBrandVoiceTemplate,
    selectedVideoMode,
    selectedVideoModel,
    subtitleLanguage,
    subtitleText,
    targetMarket,
    title,
  ])

  const applyDraftSnapshot = (state: Partial<AdVideoDraftSnapshot>) => {
    if (typeof state.title === 'string') setTitle(state.title)
    if (typeof state.adPrompt === 'string') setAdPrompt(state.adPrompt)
    if (typeof state.optimizedScript === 'string') setOptimizedScript(state.optimizedScript)
    if (typeof state.imageUrlsText === 'string') setImageUrlsText(state.imageUrlsText)
    if (typeof state.sceneDescriptionsText === 'string') setSceneDescriptionsText(state.sceneDescriptionsText)
    if (typeof state.referenceImageHintsText === 'string') setReferenceImageHintsText(state.referenceImageHintsText)
    if (typeof state.brandVoiceNotesText === 'string') setBrandVoiceNotesText(state.brandVoiceNotesText)
    if (typeof state.targetMarket === 'string') setTargetMarket(state.targetMarket)
    if (typeof state.subtitleLanguage === 'string') setSubtitleLanguage(state.subtitleLanguage)
    if (typeof state.creativeMode === 'string') setCreativeMode(state.creativeMode)
    if (typeof state.directorNote === 'string') setDirectorNote(state.directorNote)
    if (typeof state.subtitleText === 'string') setSubtitleText(state.subtitleText)
    if (typeof state.selectedTemplate === 'string' && AD_TEMPLATES.some((item) => item.key === state.selectedTemplate)) {
      setSelectedTemplate(state.selectedTemplate as (typeof AD_TEMPLATES)[number]['key'])
    }
    if (typeof state.selectedStoryboardTemplate === 'string' && STORYBOARD_TEMPLATES.some((item) => item.key === state.selectedStoryboardTemplate)) {
      setSelectedStoryboardTemplate(state.selectedStoryboardTemplate)
    }
    if (typeof state.selectedBrandVoiceTemplate === 'string' && BRAND_VOICE_TEMPLATES.some((item) => item.key === state.selectedBrandVoiceTemplate)) {
      setSelectedBrandVoiceTemplate(state.selectedBrandVoiceTemplate as BrandVoiceTemplateKey)
    }
    if (typeof state.selectedVideoModel === 'string') setSelectedVideoModel(state.selectedVideoModel)
    if (typeof state.selectedStylePreset === 'string') setSelectedStylePreset(state.selectedStylePreset)
    if (state.selectedMotionMode && VIDEO_MOTION_OPTIONS.some((item) => item.key === state.selectedMotionMode)) {
      setSelectedMotionMode(state.selectedMotionMode as (typeof VIDEO_MOTION_OPTIONS)[number]['key'])
    }
    if (state.selectedVideoMode === 'frame_animation' || state.selectedVideoMode === 'api_generation') setSelectedVideoMode(state.selectedVideoMode)
    if (typeof state.clipDurationSec === 'number' && !Number.isNaN(state.clipDurationSec)) setClipDurationSec(state.clipDurationSec)
    if (typeof state.autoOptimizeCopy === 'boolean') setAutoOptimizeCopy(state.autoOptimizeCopy)
    if (typeof state.enableLocalCompression === 'boolean') setEnableLocalCompression(state.enableLocalCompression)
    if (typeof state.maxImageSide === 'number' && !Number.isNaN(state.maxImageSide)) setMaxImageSide(state.maxImageSide)
    if (typeof state.jpegQuality === 'number' && !Number.isNaN(state.jpegQuality)) setJpegQuality(state.jpegQuality)
    if (typeof state.autoAvoidLowHourEnabled === 'boolean') setAutoAvoidLowHourEnabled(state.autoAvoidLowHourEnabled)
    if (typeof state.lowHourThreshold === 'number' && !Number.isNaN(state.lowHourThreshold)) setLowHourThreshold(state.lowHourThreshold)
    if (typeof state.autoRetryEnabled === 'boolean') setAutoRetryEnabled(state.autoRetryEnabled)
  }

  const handleRestoreSavedDraft = () => {
    if (!pendingDraftRestore) return
    applyDraftSnapshot(pendingDraftRestore)
    clearAdVideoDraft()
    setPendingDraftRestore(null)
    toast({ title: '已恢复上次草稿', description: '草稿内容已回到当前页，不会跳转到旧项目。', variant: 'success' })
  }

  const handleDiscardSavedDraft = () => {
    clearAdVideoDraft()
    setPendingDraftRestore(null)
    toast({ title: '已清除上次草稿', description: '后续会使用当前页面的新编辑内容。', variant: 'success' })
  }

  useEffect(() => {
    if (typeof window === 'undefined') return

    const { savedAt, state } = readAdVideoDraft()
    setHistoryEntries(readAdVideoHistory())
    if (state) {
      setPendingDraftRestore(state)
      setDraftSavedAt(savedAt)
    }

    setDraftHydrated(true)
  }, [])

  useEffect(() => {
    if (!draftHydrated || typeof window === 'undefined') return
    const savedAt = writeAdVideoDraft(adVideoDraftState)
    setDraftSavedAt(savedAt)
  }, [adVideoDraftState, draftHydrated])

  useEffect(() => {
    if (!draftHydrated) return
    setReviewConfirmed(false)
  }, [
    adPrompt,
    clipDurationSec,
    creativeMode,
    directorNote,
    draftHydrated,
    imageUrlsText,
    localFiles.length,
    optimizedScript,
    brandVoiceNotesText,
    referenceImageHintsText,
    sceneDescriptionsText,
    selectedMotionMode,
    selectedStylePreset,
    selectedTemplate,
    selectedStoryboardTemplate,
    selectedBrandVoiceTemplate,
    selectedVideoMode,
    subtitleLanguage,
    subtitleText,
    targetMarket,
    title,
  ])

  useEffect(() => {
    if (!activeGenerationTaskId) return

    setGenerationTasks((prev) => prev.map((task) => {
      if (task.id !== activeGenerationTaskId) return task

      if (taskStatus === 'succeeded') {
        return {
          ...task,
          projectId: activeProjectId ?? task.projectId,
          status: 'succeeded',
          step: '生成完成，可预览或下载成片',
          outputUrl: taskOutputUrl || task.outputUrl,
          error: '',
        }
      }

      if (taskStatus === 'failed') {
        return {
          ...task,
          projectId: activeProjectId ?? task.projectId,
          status: 'failed',
          step: taskError || '生成失败',
          error: taskError || '生成失败',
        }
      }

      if (taskStatus === 'processing') {
        return {
          ...task,
          projectId: activeProjectId ?? task.projectId,
          status: 'running',
          step: '生成中，后台正在轮询任务状态',
        }
      }

      if (taskStatus === 'pending') {
        return {
          ...task,
          projectId: activeProjectId ?? task.projectId,
          status: 'running',
          step: '已提交，等待队列执行',
        }
      }

      return task
    }))
  }, [activeGenerationTaskId, activeProjectId, taskError, taskOutputUrl, taskStatus])

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

  const applyStoryboardTemplate = (templateKey: string) => {
    const template = STORYBOARD_TEMPLATES.find((item) => item.key === templateKey)
    if (!template) return

    setSelectedStoryboardTemplate(template.key)
    setSceneDescriptionsText(template.sceneLines.join('\n'))
    setSubtitleText(template.dialogueLines.join('\n'))
    setReferenceImageHintsText(template.referenceLines.join('\n'))
    toast({ title: `已应用分镜模板：${template.label}`, description: template.hint, variant: 'success' })
  }

  const getMarketBrief = () => buildMarketDirective(
    targetMarket,
    subtitleLanguage,
    creativeMode,
    directorNote,
    selectedBrandVoiceTemplate,
    brandVoiceNotesText,
  )

  const getTargetMarketLabel = () => TARGET_MARKET_OPTIONS.find((item) => item.key === targetMarket)?.label ?? TARGET_MARKET_OPTIONS[0].label

  const getSubtitleLanguageLabel = () => SUBTITLE_LANGUAGE_OPTIONS.find((item) => item.key === subtitleLanguage)?.label ?? SUBTITLE_LANGUAGE_OPTIONS[0].label

  const getCreativeModeLabel = () => CREATIVE_MODE_OPTIONS.find((item) => item.key === creativeMode)?.label ?? CREATIVE_MODE_OPTIONS[0].label

  const getBrandVoiceLabel = () => BRAND_VOICE_TEMPLATES.find((item) => item.key === selectedBrandVoiceTemplate)?.label ?? BRAND_VOICE_TEMPLATES[0].label

  const buildHistoryLabel = () => {
    const titleText = title.trim() || '未命名广告'
    return `${titleText} · ${getBrandVoiceLabel()} · ${getTargetMarketLabel()}`
  }

  const handleSaveHistorySnapshot = () => {
    const entry: AdVideoHistoryEntry = {
      id: `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
      savedAt: new Date().toISOString(),
      label: buildHistoryLabel(),
      state: adVideoDraftState,
    }
    const nextEntries = [entry, ...historyEntries].slice(0, 8)
    writeAdVideoHistory(nextEntries)
    setHistoryEntries(nextEntries)
    setSelectedHistoryEntryId(entry.id)
    toast({ title: '已保存本地版本', description: '可随时恢复到当前页面；不包含本地上传图片文件。', variant: 'success' })
  }

  const handleRestoreHistorySnapshot = (entry: AdVideoHistoryEntry) => {
    applyDraftSnapshot(entry.state)
    setSelectedHistoryEntryId(entry.id)
    toast({ title: '已恢复历史版本', description: '当前页面已切换到所选版本内容。', variant: 'success' })
  }

  const updateGenerationTask = (taskId: string, patch: Partial<AdGenerationTaskEntry>) => {
    setGenerationTasks((prev) => prev.map((item) => {
      if (item.id !== taskId) return item
      return {
        ...item,
        ...patch,
        updatedAt: new Date().toISOString(),
      }
    }))
  }

  const buildGenerationTaskLabel = () => {
    const trimmedTitle = title.trim() || '未命名广告'
    return `${trimmedTitle} · ${getBrandVoiceLabel()} · ${selectedStoryboardTemplateMeta.label}`
  }

  const createGenerationTaskEntry = (): AdGenerationTaskEntry => {
    const now = new Date().toISOString()
    return {
      id: `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
      createdAt: now,
      updatedAt: now,
      label: buildGenerationTaskLabel(),
      status: 'queued',
      step: '等待提交',
      title: title.trim() || '未命名广告',
      marketLabel: getTargetMarketLabel(),
      brandVoiceLabel: getBrandVoiceLabel(),
      storyboardLabel: selectedStoryboardTemplateMeta.label,
      subtitleCount: splitSubtitleScript(subtitleText).length,
      imageCount: Math.max(imageUrls.length, localFiles.length),
    }
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
    adTaskStatusRef.current = 'pending'
    setActiveProjectId(ctx.projectId)
    setActiveTaskId(null)
    setActiveTaskStartedAt(startedAt)
    setTaskStatus('pending')
    setTaskError('')
    setTaskOutputUrl('')
    setTaskClipProgress({ done: 0, total: 0 })
    appendAdTaskLog(`已发起视频生成请求：项目 ${ctx.projectId} / 模型 ${ctx.modelName}`, 'info')

    const effectiveSubtitleText = ctx.subtitleText.trim() || ctx.prompt.trim()
    const subtitlesEnabled = ctx.dialogues.some((line) => line.trim().length > 0) || effectiveSubtitleText.length > 0

    await videoAPI.generate(ctx.projectId, {
      image_urls: ctx.imageUrls,
      scene_descriptions: ctx.sceneDescriptions,
      scene_description: ctx.prompt,
      style_preset: ctx.stylePreset,
      motion_mode: ctx.motionMode,
      video_mode: ctx.videoMode,
      model_name: ctx.modelName || undefined,
      clip_duration_sec: ctx.clipDurationSec,
      subtitle_text: effectiveSubtitleText,
      dialogues: ctx.dialogues,
      render_config: {
        config_version: 1,
        target_market: ctx.targetMarket,
        subtitle_language: ctx.subtitleLanguage,
        creative_mode: ctx.creativeMode,
        director_note: ctx.directorNote,
        brand_voice_template: ctx.brandVoiceTemplate,
        brand_voice_notes: ctx.brandVoiceNotes,
        storyboard_template: ctx.storyboardTemplate,
        reference_image_hints: ctx.referenceImageHints,
        workflow_mode: 'guided-ad-video',
        dialogues: ctx.dialogues,
        generate_audio: subtitlesEnabled,
        subtitle_style: {
          font_name: ctx.subtitleLanguage === 'en-US' ? 'Inter' : 'Noto Sans CJK SC',
          font_size: ctx.subtitleLanguage === 'bilingual' ? 40 : 44,
          outline_width: 3,
          alignment: 2,
          margin_v: 48,
          bold: true,
        },
      },
    })

    setLastGenerationContext({ ...ctx, startedAt })
    appendAdTaskLog('视频生成请求已提交，等待后台创建任务', 'progress')
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


  const runCopyOptimization = async (options?: { preserveLogs?: boolean }): Promise<OptimizedAdResult | null> => {
    const premise = adPrompt.trim()
    if (premise.length < 10) {
      toast({ title: '请先输入足够详细的广告文案', variant: 'destructive' })
      return null
    }

    if (!options?.preserveLogs) {
      resetAdTaskLogs('开始文案优化任务')
    }
    appendAdTaskLog('正在创建文案优化任务', 'info')

    const marketBrief = getMarketBrief()
    const marketLabel = getTargetMarketLabel()
    const subtitleLanguageLabel = getSubtitleLanguageLabel()
    const creativeModeLabel = getCreativeModeLabel()
    const brandVoiceLabel = getBrandVoiceLabel()
    const subtitleLineHint = subtitleLines.length > 0 ? `当前已填写 ${subtitleLines.length} 条字幕/台词` : '当前还未填写字幕/台词，请生成时自动补齐'

    setOptimizingCopy(true)
    try {
      const taskRes = await taskAPI.create({
        task_type: 'script_quick_generate',
        payload: {
          mode: 'script',
          premise,
          genre: '广告短片',
          platform: `${marketLabel}短视频投放`,
          delivery_format: '分镜脚本+逐句口播+字幕分行+结尾CTA',
          episode_duration: '45-60秒',
          tone: creativeMode === 'script-preserved'
            ? '保留原文卖点和节奏，减少导演改写'
            : creativeMode === 'director-led'
              ? '强化镜头感、节奏感和转化导向，但保持市场一致性'
              : '本地化、口语化、直给卖点，避免泛化广告腔',
          requirements: [
            '输出可直接用于广告视频生成，包含镜头建议、产品卖点、情绪转折和行动号召',
            `目标市场：${marketLabel}`,
            `字幕语言：${subtitleLanguageLabel}`,
            `创意模式：${creativeModeLabel}`,
            `品牌语气：${brandVoiceLabel}`,
            subtitleLineHint,
            marketBrief,
          ].join('\n'),
          target_words: 750,
          chapter_count: 5,
        },
      }) as unknown as { data?: { id?: number } }

      const optimizeTaskId = Number(taskRes?.data?.id ?? 0)
      if (!optimizeTaskId) throw new Error('文案优化任务创建失败')
      setActiveOptimizeTaskId(optimizeTaskId)
      appendAdTaskLog(`文案优化任务已创建 #${optimizeTaskId}`, 'progress')

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
            try {
              const progressResp = await taskAPI.getProgress(optimizeTaskId) as unknown as {
                data?: TaskProgressRecord[]
              }
              ingestBackendProgress(progressResp?.data ?? [])
            } catch {
              // ignore transient progress fetch errors
            }

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
              appendAdTaskLog('文案优化完成', 'success')
              resolve({
                title: String(taskResult.title ?? '').trim(),
                content: String(taskResult.content ?? '').trim(),
                outline: Array.isArray(taskResult.outline) ? taskResult.outline : [],
                tags: Array.isArray(taskResult.tags) ? taskResult.tags : [],
              })
            } else if (task.status === 'failed') {
              clearInterval(timer)
              appendAdTaskLog(task.error_msg || '文案优化失败', 'error')
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
      if (!subtitleText.trim() && result.content) {
        setSubtitleText(result.content)
      }
      toast({ title: '文案优化完成', description: '已自动回填优化结果与分镜建议', variant: 'success' })
      return result
    } catch (err: unknown) {
      appendAdTaskLog(err instanceof Error ? err.message : '文案优化失败', 'error')
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
      const savedAt = writeAdVideoDraft(adVideoDraftState)
      setDraftSavedAt(savedAt)

      toast({
        title: '广告草稿已保存',
        description: '你可以继续在当前页面调整市场、字幕和导演备注，然后直接生成视频',
        variant: 'success',
      })
    } catch {
      toast({ title: '保存草稿失败，请稍后重试', variant: 'destructive' })
      setCreatingByText(false)
      return
    }

    setCreatingByText(false)
  }

  const handleGenerateByImages = async () => {
    const basePrompt = adPrompt.trim()
    if (basePrompt.length < 10) {
      toast({ title: '请先输入广告文案，用于场景描述和视频语义', variant: 'destructive' })
      return
    }

    const optimized = autoOptimizeCopy ? await runCopyOptimization({ preserveLogs: true }) : null
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

    if (!reviewConfirmed) {
      toast({
        title: '请先完成本地审核确认',
        description: '确认市场、台词和分镜无误后，再提交生成。',
        variant: 'destructive',
      })
      return
    }

    resetAdTaskLogs('开始创建广告任务')
    appendAdTaskLog(`广告文案已确认，准备提交 ${getTargetMarketLabel()} 任务`, 'info')

    const generationTask = createGenerationTaskEntry()
    setGenerationTasks((prev) => [generationTask, ...prev].slice(0, 8))
    setActiveGenerationTaskId(generationTask.id)
    setCreatingByImages(true)

    try {
      updateGenerationTask(generationTask.id, {
        status: autoOptimizeCopy ? 'optimizing' : 'uploading',
        step: autoOptimizeCopy ? '文案优化中，正在准备广告语' : '正在准备素材并提交任务',
      })
      appendAdTaskLog(autoOptimizeCopy ? '开始自动优化广告文案' : '跳过自动优化，直接准备素材', 'progress')

      const marketBrief = getMarketBrief()
      const subtitleSourceText = subtitleText.trim() || trimmedPrompt
      const generationPrompt = [trimmedPrompt, marketBrief].filter(Boolean).join('\n\n')
      const projectTitle = buildProjectTitle()
      const referenceSourceLines = referenceImageHints.length > 0 ? referenceImageHints : selectedStoryboardTemplateMeta.referenceLines
      const initialReferenceHints = distributeDialogues(referenceSourceLines, Math.max(imageUrls.length, localFiles.length, 1))
      updateGenerationTask(generationTask.id, {
        status: 'uploading',
        step: '正在上传素材并创建项目',
      })
      appendAdTaskLog('正在上传素材并创建项目', 'progress')
      const createRes = (await projectAPI.create({
        title: projectTitle,
        description: `由视频广告生成器创建；目标市场：${getTargetMarketLabel()}；字幕语言：${getSubtitleLanguageLabel()}；创意模式：${getCreativeModeLabel()}；品牌语气：${getBrandVoiceLabel()}；分镜模板：${selectedStoryboardTemplateMeta.label}`,
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
          target_market: targetMarket,
          subtitle_language: subtitleLanguage,
          creative_mode: creativeMode,
          director_note: directorNote.trim(),
          brand_voice_template: selectedBrandVoiceTemplate,
          brand_voice_notes: brandVoiceNotesText.trim(),
          storyboard_template: selectedStoryboardTemplate,
          reference_image_hints: initialReferenceHints,
        },
      } as never)) as { data: { id: number } }

      const projectId = createRes.data.id
      appendAdTaskLog(`项目已创建，ID ${projectId}`, 'success')
      setAutoRetryAttempts(0)
      autoRetryingRef.current = false
      setSessionAnchorAt(new Date().toISOString())
      setRetryHistory([])
      setBatchSubmittedCount(0)

      const scriptFile = new File([subtitleSourceText], 'ad-script.txt', {
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
      appendAdTaskLog(`素材准备完成，共 ${finalImageUrls.length} 张图片`, 'success')

      const perClipReferenceHints = distributeDialogues(referenceSourceLines, finalImageUrls.length)
      const perClipDialogues = distributeDialogues(splitSubtitleScript(subtitleSourceText), finalImageUrls.length)

      const resolvedDescriptions = finalImageUrls.map((_, index) => {
        const line = sceneDescriptions[index] ?? perClipDialogues[index] ?? optimized?.outline?.[index] ?? sceneDescriptions[sceneDescriptions.length - 1] ?? trimmedPrompt
        return line.trim()
      })

      await triggerVideoGeneration({
        projectId,
        projectTitle,
        prompt: generationPrompt,
        imageUrls: finalImageUrls,
        sceneDescriptions: resolvedDescriptions,
        modelName: chooseModelForSubmission(selectedVideoModel || (availableVideoModels[0]?.model_key ?? '')),
        stylePreset: selectedStylePreset,
        motionMode: selectedMotionMode,
        videoMode: selectedVideoMode,
        clipDurationSec,
        targetMarket,
        subtitleLanguage,
        creativeMode,
        directorNote: directorNote.trim(),
        subtitleText: subtitleSourceText,
        dialogues: perClipDialogues,
        storyboardTemplate: selectedStoryboardTemplate,
        referenceImageHints: perClipReferenceHints,
        brandVoiceTemplate: selectedBrandVoiceTemplate,
        brandVoiceNotes: brandVoiceNotesText.trim(),
      })
      appendAdTaskLog(`视频生成任务已提交，项目 ${projectId} 正在后台处理`, 'success')

      updateGenerationTask(generationTask.id, {
        projectId,
        status: 'running',
        step: '已提交成功，后台正在生成并轮询状态',
      })

      toast({
        title: '广告视频生成已启动',
        description: `任务已加入异步列表：${projectTitle}`,
        variant: 'success',
      })
    } catch {
      appendAdTaskLog('广告任务启动失败', 'error')
      updateGenerationTask(generationTask.id, {
        status: 'failed',
        step: '生成任务提交失败',
        error: '启动生成失败，请稍后重试',
      })
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
    appendAdTaskLog(`开始批量提交 ${models.length} 个视频版本`, 'info')
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
          subtitle_text: lastGenerationContext.subtitleText,
          dialogues: lastGenerationContext.dialogues,
          render_config: {
            config_version: 1,
            target_market: lastGenerationContext.targetMarket,
            subtitle_language: lastGenerationContext.subtitleLanguage,
            creative_mode: lastGenerationContext.creativeMode,
            director_note: lastGenerationContext.directorNote,
            brand_voice_template: lastGenerationContext.brandVoiceTemplate,
            brand_voice_notes: lastGenerationContext.brandVoiceNotes,
            storyboard_template: lastGenerationContext.storyboardTemplate,
            reference_image_hints: lastGenerationContext.referenceImageHints,
            workflow_mode: 'guided-ad-video',
            dialogues: lastGenerationContext.dialogues,
            generate_audio: lastGenerationContext.dialogues.some((line) => line.trim().length > 0),
            subtitle_style: {
              font_name: lastGenerationContext.subtitleLanguage === 'en-US' ? 'Inter' : 'Noto Sans CJK SC',
              font_size: lastGenerationContext.subtitleLanguage === 'bilingual' ? 40 : 44,
              outline_width: 3,
              alignment: 2,
              margin_v: 48,
              bold: true,
            },
          },
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
      appendAdTaskLog(`批量提交成功，已新增 ${submitted} 个版本`, 'success')
      toast({ title: '批量版本生成已提交', description: `已提交 ${submitted} 个模型版本`, variant: 'success' })
    } catch {
      appendAdTaskLog('批量提交失败，请稍后重试', 'error')
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
        target_market: lastGenerationContext.targetMarket,
        subtitle_language: lastGenerationContext.subtitleLanguage,
        creative_mode: lastGenerationContext.creativeMode,
        director_note: lastGenerationContext.directorNote,
        brand_voice_template: lastGenerationContext.brandVoiceTemplate,
        brand_voice_notes: lastGenerationContext.brandVoiceNotes,
        subtitle_text: lastGenerationContext.subtitleText,
        dialogues: lastGenerationContext.dialogues,
        model_name: lastGenerationContext.modelName,
        style_preset: lastGenerationContext.stylePreset,
        motion_mode: lastGenerationContext.motionMode,
        video_mode: lastGenerationContext.videoMode,
        clip_duration_sec: lastGenerationContext.clipDurationSec,
        prompt: lastGenerationContext.prompt,
        storyboard_template: lastGenerationContext.storyboardTemplate,
        reference_image_hints: lastGenerationContext.referenceImageHints,
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
        `- 目标市场: ${TARGET_MARKET_OPTIONS.find((item) => item.key === lastGenerationContext.targetMarket)?.label ?? TARGET_MARKET_OPTIONS[0].label}`,
        `- 字幕语言: ${SUBTITLE_LANGUAGE_OPTIONS.find((item) => item.key === lastGenerationContext.subtitleLanguage)?.label ?? SUBTITLE_LANGUAGE_OPTIONS[0].label}`,
        `- 创意模式: ${CREATIVE_MODE_OPTIONS.find((item) => item.key === lastGenerationContext.creativeMode)?.label ?? CREATIVE_MODE_OPTIONS[0].label}`,
        `- 品牌语气: ${BRAND_VOICE_TEMPLATES.find((item) => item.key === lastGenerationContext.brandVoiceTemplate)?.label ?? BRAND_VOICE_TEMPLATES[0].label}`,
        `- 模型: ${lastGenerationContext.modelName}`,
        `- 分镜模板: ${lastGenerationContext.storyboardTemplate}`,
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
        '## 字幕 / 口播台词',
        '',
        lastGenerationContext.subtitleText || '- 未填写',
        '',
        '## 品牌语气',
        '',
        `- 模板：${BRAND_VOICE_TEMPLATES.find((item) => item.key === lastGenerationContext.brandVoiceTemplate)?.label ?? BRAND_VOICE_TEMPLATES[0].label}`,
        `- 说明：${lastGenerationContext.brandVoiceNotes || '未填写'}`,
        '',
        '## 分镜描述',
        '',
        ...lastGenerationContext.sceneDescriptions.map((item, idx) => `${idx + 1}. ${item}`),
        '',
        '## 镜头参考图提示',
        '',
        ...(lastGenerationContext.referenceImageHints.length > 0
          ? lastGenerationContext.referenceImageHints.map((item, idx) => `${idx + 1}. ${item}`)
          : ['- 未填写'] ),
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
        if (!activeTaskId) {
          setActiveTaskId(target.id)
          appendAdTaskLog(`已定位视频任务 #${target.id}`, 'info')
        }

        const clipsTotal = target.clips?.length ?? target.image_urls?.length ?? 0
        const clipsDone = target.clips?.filter((clip) => clip.status === 'succeeded').length ?? 0
        setTaskClipProgress({ done: clipsDone, total: clipsTotal })

        if (target.status !== adTaskStatusRef.current) {
          adTaskStatusRef.current = target.status as typeof adTaskStatusRef.current
          const statusMessageMap: Record<string, string> = {
            pending: '视频任务已进入队列',
            processing: '视频任务正在生成',
            succeeded: '视频任务已完成',
            failed: '视频任务已失败',
          }
          const statusMessage = statusMessageMap[target.status] ?? `视频任务状态更新为 ${target.status}`
          appendAdTaskLog(statusMessage, target.status === 'failed' ? 'error' : target.status === 'succeeded' ? 'success' : 'progress')
        }

        if (target.status === 'succeeded') {
          setTaskStatus('succeeded')
          const outputUrl = String(target.result_url || target.hls_url || '').trim()
          setTaskOutputUrl(outputUrl)
          if (outputUrl) appendAdTaskLog(`已生成输出链接：${outputUrl}`, 'success')
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
          appendAdTaskLog(errorMessage, 'error')

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
                appendAdTaskLog(`主模型失败，自动切换到 ${backupModel} 重试`, 'warning')
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
              先写广告文案，再在本页完善市场、分镜和台词，然后保存草稿或直接生成视频。
            </p>
          </div>

          <div className="grid gap-3 sm:grid-cols-2">
            <div className="rounded-2xl border border-white/10 bg-white/10 p-4 backdrop-blur">
              <p className="text-xs uppercase tracking-[0.2em] text-surface-300">方式 A</p>
              <p className="mt-2 text-base font-semibold text-white">本地草稿完善</p>
              <p className="mt-1 text-xs text-surface-400">所有修改保留在当前页</p>
            </div>
            <div className="rounded-2xl border border-white/10 bg-white/10 p-4 backdrop-blur">
              <p className="text-xs uppercase tracking-[0.2em] text-surface-300">方式 B</p>
              <p className="mt-2 text-base font-semibold text-white">指定图片直出视频</p>
              <p className="mt-1 text-xs text-surface-400">创建项目后直接触发视频任务</p>
            </div>
          </div>
        </div>
      </div>

      {pendingDraftRestore ? (
        <div className="rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-900">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div>
              <p className="font-medium">检测到上次草稿</p>
              <p className="mt-1 text-xs text-amber-700">这是本地草稿恢复，不会自动跳转到旧项目；你可以选择恢复或丢弃。</p>
            </div>
            <div className="flex items-center gap-2">
              <Button type="button" size="sm" onClick={handleRestoreSavedDraft}>
                恢复草稿
              </Button>
              <Button type="button" size="sm" variant="outline" onClick={handleDiscardSavedDraft}>
                丢弃草稿
              </Button>
            </div>
          </div>
        </div>
      ) : null}

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
				onClick={() => { void runCopyOptimization() }}
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

          <div className="space-y-3 rounded-xl border border-surface-200 bg-white p-4">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <div>
                <p className="text-sm font-medium text-surface-800">市场 / 台词 / 导演备注</p>
                <p className="text-xs text-surface-500">这些设置会进入文案优化、字幕烧录和视频生成提示词，直接影响市场匹配和口播一致性。</p>
              </div>
              <span className="rounded-full border border-cyan-200 bg-cyan-50 px-2.5 py-1 text-[11px] font-medium text-cyan-700">
                指导式生成
              </span>
            </div>

            <div className="grid gap-4 md:grid-cols-3">
              <div className="space-y-1">
                <Label className="text-xs text-surface-700">目标市场</Label>
                <Select value={targetMarket} onValueChange={setTargetMarket}>
                  <SelectTrigger>
                    <SelectValue placeholder="选择市场" />
                  </SelectTrigger>
                  <SelectContent>
                    {TARGET_MARKET_OPTIONS.map((option) => (
                      <SelectItem key={option.key} value={option.key}>
                        {option.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              <div className="space-y-1">
                <Label className="text-xs text-surface-700">字幕语言</Label>
                <Select value={subtitleLanguage} onValueChange={setSubtitleLanguage}>
                  <SelectTrigger>
                    <SelectValue placeholder="选择字幕语言" />
                  </SelectTrigger>
                  <SelectContent>
                    {SUBTITLE_LANGUAGE_OPTIONS.map((option) => (
                      <SelectItem key={option.key} value={option.key}>
                        {option.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              <div className="space-y-1">
                <Label className="text-xs text-surface-700">创意模式</Label>
                <Select value={creativeMode} onValueChange={setCreativeMode}>
                  <SelectTrigger>
                    <SelectValue placeholder="选择创意模式" />
                  </SelectTrigger>
                  <SelectContent>
                    {CREATIVE_MODE_OPTIONS.map((option) => (
                      <SelectItem key={option.key} value={option.key}>
                        {option.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            </div>

            <div className="grid gap-4 md:grid-cols-2">
              <div className="space-y-2">
                <Label htmlFor="subtitle-text">字幕 / 口播台词</Label>
                <Textarea
                  id="subtitle-text"
                  rows={7}
                  placeholder="每行一句，生成时会自动分配到各镜头；支持中文、英文或中英双语。"
                  value={subtitleText}
                  onChange={(event) => setSubtitleText(event.target.value)}
                />
                <p className="text-xs text-surface-500">
                  当前已识别 {subtitleLines.length} 条台词；生成时会同步进入字幕和支持原生音频的模型输入。
                </p>
              </div>

              <div className="space-y-2">
                <Label htmlFor="director-note">导演备注 / 禁止项</Label>
                <Textarea
                  id="director-note"
                  rows={7}
                  placeholder="例如：不要把品牌卖点改掉；前 5 秒必须交代市场利益点；字幕必须跟口播逐句对应。"
                  value={directorNote}
                  onChange={(event) => setDirectorNote(event.target.value)}
                />
                <div className="rounded-lg border border-surface-200 bg-surface-50 px-3 py-2 text-xs text-surface-600">
                  当前市场约束会自动进入文案优化与视频生成提示词，避免导演式自动改写把市场带偏。
                </div>
              </div>
            </div>
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

          <div className="space-y-4 rounded-xl border border-surface-200 bg-white p-4">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <div>
                <p className="text-sm font-medium text-surface-800">本地审核与分镜预览</p>
                <p className="text-xs text-surface-500">先检查广告文案、素材、台词和分镜，再勾选确认后生成。编辑卡片会同步回上方文本区。</p>
              </div>
              <span className={[
                'rounded-full border px-2.5 py-1 text-[11px] font-medium',
                reviewReady
                  ? 'border-emerald-200 bg-emerald-50 text-emerald-700'
                  : reviewConfirmed
                    ? 'border-amber-200 bg-amber-50 text-amber-700'
                    : 'border-surface-200 bg-surface-50 text-surface-600',
              ].join(' ')}>
                {reviewReady ? '已确认，可生成' : reviewConfirmed ? '等待补全项' : '等待审核确认'}
              </span>
            </div>

            <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
              {adReviewChecklist.map((item) => (
                <div
                  key={item.key}
                  className={[
                    'rounded-xl border p-3',
                    item.passed
                      ? 'border-emerald-200 bg-emerald-50/60'
                      : item.blocking
                        ? 'border-rose-200 bg-rose-50/60'
                        : 'border-amber-200 bg-amber-50/60',
                  ].join(' ')}
                >
                  <div className="flex items-center justify-between gap-2">
                    <p className="text-xs font-medium text-surface-800">{item.label}</p>
                    <span className={[
                      'rounded-full border px-2 py-0.5 text-[11px] font-medium',
                      item.passed
                        ? 'border-emerald-200 bg-white/80 text-emerald-700'
                        : item.blocking
                          ? 'border-rose-200 bg-white/80 text-rose-700'
                          : 'border-amber-200 bg-white/80 text-amber-700',
                    ].join(' ')}>
                      {item.passed ? '通过' : item.blocking ? '缺失' : '待优化'}
                    </span>
                  </div>
                  <p className="mt-2 text-xs leading-5 text-surface-600">{item.detail}</p>
                </div>
              ))}
            </div>

            {blockingReviewItems.length > 0 ? (
              <p className="text-xs text-rose-600">先补全：{blockingReviewItems.map((item) => item.label).join('、')}</p>
            ) : advisoryReviewItems.length > 0 ? (
              <p className="text-xs text-amber-600">建议先优化：{advisoryReviewItems.map((item) => item.label).join('、')}</p>
            ) : (
              <p className="text-xs text-emerald-600">当前审核项已满足，可以进入生成。</p>
            )}

            <div className="flex items-start gap-3 rounded-xl border border-cyan-200 bg-cyan-50/60 p-3">
              <Switch checked={reviewConfirmed} onCheckedChange={setReviewConfirmed} />
              <div className="space-y-1">
                <p className="text-sm font-medium text-cyan-900">我已确认市场、台词和分镜无误</p>
                <p className="text-xs text-cyan-700">勾选后才能提交生成；修改任意输入后会自动取消确认。</p>
              </div>
            </div>

            <div className="space-y-3 rounded-xl border border-cyan-100 bg-cyan-50/40 p-4">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <div>
                  <p className="text-sm font-medium text-cyan-900">分镜模板库</p>
                  <p className="text-xs text-cyan-700">一键套用镜头结构、台词节奏和参考图提示，再在卡片里做局部微调。</p>
                </div>
                <span className="rounded-full border border-cyan-200 bg-white px-2.5 py-1 text-[11px] font-medium text-cyan-700">
                  当前：{selectedStoryboardTemplateMeta.label}
                </span>
              </div>

              <div className="flex flex-wrap gap-2">
                {STORYBOARD_TEMPLATES.map((template) => {
                  const active = selectedStoryboardTemplate === template.key
                  return (
                    <button
                      key={template.key}
                      type="button"
                      onClick={() => applyStoryboardTemplate(template.key)}
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

              <div className="rounded-lg border border-cyan-100 bg-white px-3 py-3 text-xs text-cyan-700">
                <p className="font-medium text-cyan-900">{selectedStoryboardTemplateMeta.label}</p>
                <p className="mt-1 leading-5">{selectedStoryboardTemplateMeta.hint}</p>
                <p className="mt-2 text-[11px] leading-5 text-cyan-600">
                  场景建议 {selectedStoryboardTemplateMeta.sceneLines.length} 条 · 台词建议 {selectedStoryboardTemplateMeta.dialogueLines.length} 条 · 参考图建议 {selectedStoryboardTemplateMeta.referenceLines.length} 条
                </p>
              </div>
            </div>

            <div className="space-y-3 rounded-xl border border-violet-100 bg-violet-50/40 p-4">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <div>
                  <p className="text-sm font-medium text-violet-900">品牌语气模板</p>
                  <p className="text-xs text-violet-700">模板会进入文案优化和视频生成提示词，并影响当前文案的表达方向。</p>
                </div>
                <span className="rounded-full border border-violet-200 bg-white px-2.5 py-1 text-[11px] font-medium text-violet-700">
                  当前：{getBrandVoiceLabel()}
                </span>
              </div>

              <div className="flex flex-wrap gap-2">
                {BRAND_VOICE_TEMPLATES.map((template) => {
                  const active = selectedBrandVoiceTemplate === template.key
                  return (
                    <button
                      key={template.key}
                      type="button"
                      onClick={() => setSelectedBrandVoiceTemplate(template.key)}
                      className={[
                        'rounded-full border px-3 py-1.5 text-xs font-medium transition-colors',
                        active
                          ? 'border-violet-300 bg-violet-50 text-violet-800'
                          : 'border-surface-200 bg-white text-surface-600 hover:border-violet-200 hover:bg-violet-50/40',
                      ].join(' ')}
                    >
                      {template.label}
                    </button>
                  )
                })}
              </div>

              <div className="grid gap-3 md:grid-cols-2">
                <div className="rounded-lg border border-surface-200 bg-white p-3">
                  <p className="text-xs font-medium text-surface-700">切换前预览</p>
                  <div className="mt-2 max-h-36 overflow-y-auto whitespace-pre-wrap text-xs leading-5 text-surface-600">
                    {optimizedScript.trim() || adPrompt.trim() || '请先填写广告文案，以便对比品牌语气模板。'}
                  </div>
                </div>
                <div className="rounded-lg border border-violet-200 bg-white p-3">
                  <p className="text-xs font-medium text-violet-800">切换后预览</p>
                  <div className="mt-2 max-h-36 overflow-y-auto whitespace-pre-wrap text-xs leading-5 text-violet-700">
                    {brandVoiceBrief}
                  </div>
                </div>
              </div>

              <div className="space-y-1">
                <Label className="text-xs text-violet-800">品牌语气补充说明</Label>
                <Textarea
                  rows={3}
                  value={brandVoiceNotesText}
                  onChange={(event) => setBrandVoiceNotesText(event.target.value)}
                  placeholder="例如：更克制、更高级，不要太热闹；品牌口径要统一；避免过度促销腔。"
                />
                <p className="text-[11px] leading-5 text-violet-600">这里写更细的口吻要求，模板会保留，你可以只改局部表达。</p>
              </div>

              <div className="rounded-lg border border-violet-200 bg-violet-50/60 px-3 py-2 text-xs text-violet-700">
                <p className="font-medium text-violet-900">{selectedBrandVoiceTemplateMeta.label}</p>
                <p className="mt-1 leading-5">{selectedBrandVoiceTemplateMeta.hint}</p>
                <p className="mt-2 text-[11px] leading-5 text-violet-600">{selectedBrandVoiceTemplateMeta.contrast}</p>
              </div>
            </div>

            <div className="space-y-3">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <div>
                  <p className="text-sm font-medium text-surface-800">分镜可视化编辑</p>
                  <p className="text-xs text-surface-500">每张卡片都对应一段镜头，修改会同步回上方分镜描述和字幕文本。</p>
                </div>
                <div className="flex flex-wrap items-center gap-2">
                  <span className="rounded-full border border-cyan-200 bg-cyan-50 px-2.5 py-1 text-[11px] font-medium text-cyan-700">
                    {storyboardPreview.length} 个镜头
                  </span>
                  <Button
                    type="button"
                    size="sm"
                    variant="outline"
                    onClick={fillReferenceHintsForAll}
                    disabled={referenceHintGeneratingAll || referenceHintGeneratingIndex !== null || storyboardPreview.length === 0}
                    className="h-8 gap-2 border-cyan-200 bg-cyan-50 text-cyan-800 hover:bg-cyan-100"
                  >
                    {referenceHintGeneratingAll ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Sparkles className="h-3.5 w-3.5" />}
                    AI 补全全部参考图
                  </Button>
                </div>
              </div>

              <div className="grid gap-4 xl:grid-cols-2">
                {storyboardPreview.map((shot) => (
                  <div key={shot.index} className="rounded-2xl border border-surface-200 bg-surface-50 p-4">
                    <div className="flex items-start justify-between gap-3">
                      <div>
                        <p className="text-xs uppercase tracking-[0.2em] text-surface-500">镜头 {shot.index + 1}</p>
                        <p className="mt-1 text-sm font-medium text-surface-800">{shot.imageSource}</p>
                      </div>
                      <div className="flex flex-col items-end gap-1">
                        <span className="rounded-full border border-surface-200 bg-white px-2 py-1 text-[11px] font-medium text-surface-600">
                          {shot.dialogue ? '台词已填' : '待补台词'}
                        </span>
                        <span className="rounded-full border border-surface-200 bg-white px-2 py-1 text-[11px] font-medium text-surface-600">
                          {shot.referenceHint ? '参考图已填' : '待补参考图'}
                        </span>
                      </div>
                    </div>
                    <div className="mt-3 space-y-3">
                      <div className="space-y-1">
                        <Label className="text-xs text-surface-700">分镜描述</Label>
                        <Textarea
                          rows={3}
                          value={shot.scene}
                          onChange={(event) => setSceneDescriptionsText((prev) => updateLineAtIndex(prev, shot.index, event.target.value))}
                        />
                      </div>
                      <div className="space-y-1">
                        <div className="flex items-center justify-between gap-2">
                          <Label className="text-xs text-surface-700">镜头参考图</Label>
                          <Button
                            type="button"
                            size="sm"
                            variant="ghost"
                            onClick={() => fillReferenceHintAtIndex(shot)}
                            disabled={referenceHintGeneratingAll || referenceHintGeneratingIndex === shot.index}
                            className="h-7 gap-1 px-2 text-[11px] text-cyan-700 hover:bg-cyan-50 hover:text-cyan-900"
                          >
                            {referenceHintGeneratingIndex === shot.index ? (
                              <Loader2 className="h-3 w-3 animate-spin" />
                            ) : (
                              <Sparkles className="h-3 w-3" />
                            )}
                            AI 补全
                          </Button>
                        </div>
                        <Input
                          value={shot.referenceHint}
                          placeholder={selectedStoryboardTemplateMeta.referenceLines[shot.index] ?? selectedStoryboardTemplateMeta.referenceLines[selectedStoryboardTemplateMeta.referenceLines.length - 1] ?? '例如：白底产品特写 / 手持使用场景'}
                          onChange={(event) => setReferenceImageHintsText((prev) => updateLineAtIndex(prev, shot.index, event.target.value))}
                        />
                        <p className="text-[11px] leading-5 text-surface-500">可填参考图风格、构图、主体或检索关键词，不需要真实上传图片。</p>
                      </div>
                      <div className="space-y-1">
                        <Label className="text-xs text-surface-700">字幕 / 口播</Label>
                        <Textarea
                          rows={3}
                          value={shot.dialogue}
                          onChange={(event) => setSubtitleText((prev) => updateLineAtIndex(prev, shot.index, event.target.value))}
                        />
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          </div>

          <div className="grid gap-3 sm:grid-cols-2">
            <Button
              type="button"
              onClick={handleCreateFromText}
              disabled={creatingByText || creatingByImages}
              className="h-11 gap-2"
            >
              {creatingByText ? <Loader2 className="h-4 w-4 animate-spin" /> : <Wand2 className="h-4 w-4" />}
              保存草稿并继续编辑
            </Button>
            <Button
              type="button"
              variant="outline"
              onClick={handleGenerateByImages}
              disabled={creatingByImages || creatingByText || !reviewReady}
              className="h-11 gap-2 border-cyan-200 bg-cyan-50 text-cyan-800 hover:bg-cyan-100"
            >
              {creatingByImages ? <Loader2 className="h-4 w-4 animate-spin" /> : <ImageIcon className="h-4 w-4" />}
              审核通过后异步生成
              <ArrowRight className="h-4 w-4" />
            </Button>
          </div>

          <GenerationQueuePanel
            generationTasks={generationTasks}
            activeGenerationTaskId={activeGenerationTaskId}
            onOpenProject={(projectId) => router.push(`/projects/${projectId}`)}
            onOpenOutput={(outputUrl) => window.open(outputUrl, '_blank', 'noopener,noreferrer')}
          />

          <LocalHistoryPanel
            historyEntries={historyEntries}
            selectedHistoryEntryId={selectedHistoryEntryId}
            selectedHistoryEntry={selectedHistoryEntry}
            currentVersionSummary={currentVersionSummary}
            selectedHistorySummary={selectedHistorySummary}
            onSave={handleSaveHistorySnapshot}
            onSelect={setSelectedHistoryEntryId}
            onRestore={(entry) => handleRestoreHistorySnapshot(entry as AdVideoHistoryEntry)}
          />

          <div className="flex flex-wrap items-center justify-between gap-2 rounded-xl border border-dashed border-surface-200 bg-surface-50 px-4 py-3 text-xs text-surface-500">
            <span>{draftSavedAt ? `草稿已保存于 ${new Date(draftSavedAt).toLocaleString('zh-CN')}` : '草稿会自动保存，避免编辑中途丢失'}</span>
            <span>当前流程已与旧项目创建页解耦，修改会保留在本页。</span>
          </div>

          {activeProjectId ? (
            <div className="rounded-xl border border-cyan-200 bg-cyan-50/60 p-4">
              <CurrentTaskPanel
                activeProjectId={activeProjectId}
                activeTaskId={activeTaskId}
                taskStatus={taskStatus}
                taskError={taskError}
                taskOutputUrl={taskOutputUrl}
                taskClipProgress={taskClipProgress}
                autoRetryAttempts={autoRetryAttempts}
                adTaskLogs={adTaskLogs}
                activeOptimizeTaskId={activeOptimizeTaskId}
                manualRerunLoading={manualRerunLoading}
                exportingPackage={exportingPackage}
                onOpenProject={(projectId) => router.push(`/projects/${projectId}`)}
                onOpenOutput={openOutput}
                onRerunAnotherVersion={handleRerunAnotherVersion}
                onExportPackage={handleExportPackage}
              />

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
