'use client'

import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import Link from 'next/link'
import { format } from 'date-fns'
import useSWR from 'swr'
import {
  Image as ImageIcon,
  Plus,
  Wand2,
  Send,
  Sparkles,
  Loader2,
  CheckCircle2,
  History,
  Trash2,
  X,
  RotateCcw,
  AlertCircle,
  Maximize2,
} from 'lucide-react'
import { assetAPI, modelAPI, projectAPI } from '@/lib/api'
import { PROJECT_MEDIA_META } from '@/lib/project-media'
import type { Asset, AssetType, ChatMessage, Model, Project } from '@/types'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Badge } from '@/components/ui/badge'
import { useToast } from '@/components/ui/toast'

// ─── Types ────────────────────────────────────────────────────────────────────

type GenerationResult = {
  modelKey: string
  modelName: string
  assetId: number
  status: 'pending' | 'generating' | 'completed' | 'failed'
  imageUrl: string
  generatedImages: string[]
  agentHistory: ChatMessage[]
}

type GenerationSession = {
  id: string
  prompt: string
  createdAt: string
  projectId: number
  assetIds: Record<string, number>
}

// ─── LocalStorage ─────────────────────────────────────────────────────────────

const SESSIONS_KEY = 'img_gen_sessions_v1'

function loadSessions(): GenerationSession[] {
  if (typeof window === 'undefined') return []
  try { return JSON.parse(localStorage.getItem(SESSIONS_KEY) ?? '[]') } catch { return [] }
}

function saveSessions(sessions: GenerationSession[]) {
  if (typeof window === 'undefined') return
  localStorage.setItem(SESSIONS_KEY, JSON.stringify(sessions.slice(0, 50)))
}

// Maps frontend style keys to backend canonical style_preset values understood by image-service.
const STYLE_PRESET_BACKEND_MAP: Record<string, string> = {
  realistic: 'live-action-short',
  anime: 'anime-2d',
  illustration: 'illustration',
  'chinese-ink': 'chinese-ink',
  cyberpunk: 'cyberpunk',
}

// ─── Style Presets ────────────────────────────────────────────────────────────

const STYLE_PRESETS = [
  {
    key: 'realistic',
    label: '真人写实',
    description: '逼真摄影质感',
    keywords: 'photorealistic, 8K ultra detail, cinematic lighting, professional photography, RAW photo, sharp focus, highly detailed',
  },
  {
    key: 'anime',
    label: '动漫二次元',
    description: '日系动漫风格',
    keywords: 'anime style illustration, 2D art, vibrant colors, Japanese animation, manga aesthetic, clean line art',
  },
  {
    key: 'illustration',
    label: '插画风',
    description: '数字绘画艺术',
    keywords: 'digital illustration, concept art, painterly style, soft color palette, artistic rendering',
  },
  {
    key: 'chinese-ink',
    label: '国风水墨',
    description: '中国传统画风',
    keywords: 'Chinese ink wash painting, traditional brush strokes, ancient Chinese aesthetic, elegant composition',
  },
  {
    key: 'cyberpunk',
    label: '赛博朋克',
    description: '霓虹未来感',
    keywords: 'cyberpunk style, neon lights, holographic displays, futuristic cityscape, dark atmosphere, sci-fi aesthetic',
  },
] as const
type StylePresetKey = typeof STYLE_PRESETS[number]['key'] | ''

function enhancePromptClientSide(userPrompt: string, styleKey: StylePresetKey): string {
  const clean = userPrompt.trim()
  if (!clean) return clean
  const style = STYLE_PRESETS.find(s => s.key === styleKey)
  const parts: string[] = [clean]
  if (style) parts.push(style.keywords)
  parts.push('highly detailed, masterpiece quality, best quality')
  return parts.join(', ')
}

// ─── Asset helpers ────────────────────────────────────────────────────────────

function getSelectedImageUrl(asset: Asset): string {
  const meta = asset.metadata ?? {}
  const preferred = typeof meta.selected_generated_image_url === 'string' ? meta.selected_generated_image_url.trim() : ''
  if (preferred) return preferred
  if (asset.image_url?.trim()) return asset.image_url.trim()
  const raw = Array.isArray(meta.generated_images) ? meta.generated_images as any[] : []
  for (const item of raw) { if (item?.url) return item.url }
  return ''
}

function getGeneratedImages(asset: Asset): string[] {
  const meta = asset.metadata ?? {}
  const raw = Array.isArray(meta.generated_images) ? meta.generated_images as any[] : []
  const urls: string[] = []
  for (const item of raw) { if (item?.url && !urls.includes(item.url)) urls.push(item.url) }
  const primary = asset.image_url?.trim()
  if (primary && !urls.includes(primary)) urls.unshift(primary)
  return urls
}

// ─── Canvas pan/zoom hook ─────────────────────────────────────────────────────

function useCanvas() {
  const [offset, setOffset] = useState({ x: 48, y: 40 })
  const [scale, setScale] = useState(1)
  const isPanning = useRef(false)
  const lastMouse = useRef({ x: 0, y: 0 })

  const onMouseDown = useCallback((e: React.MouseEvent<HTMLDivElement>) => {
    if ((e.target as HTMLElement).closest('[data-no-pan]')) return
    isPanning.current = true
    lastMouse.current = { x: e.clientX, y: e.clientY }
    e.preventDefault()
  }, [])

  const onMouseMove = useCallback((e: React.MouseEvent<HTMLDivElement>) => {
    if (!isPanning.current) return
    const dx = e.clientX - lastMouse.current.x
    const dy = e.clientY - lastMouse.current.y
    lastMouse.current = { x: e.clientX, y: e.clientY }
    setOffset(prev => ({ x: prev.x + dx, y: prev.y + dy }))
  }, [])

  const onMouseUp = useCallback(() => { isPanning.current = false }, [])

  const onWheel = useCallback((e: React.WheelEvent<HTMLDivElement>) => {
    e.preventDefault()
    setScale(prev => Math.max(0.35, Math.min(1.8, prev * (e.deltaY > 0 ? 0.95 : 1.05))))
  }, [])

  const reset = useCallback(() => { setOffset({ x: 48, y: 40 }); setScale(1) }, [])

  return { offset, scale, onMouseDown, onMouseMove, onMouseUp, onWheel, reset }
}

// ─── Lightbox ─────────────────────────────────────────────────────────────────

function Lightbox({ url, onClose }: { url: string; onClose: () => void }) {
  useEffect(() => {
    const handler = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [onClose])

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/85 backdrop-blur-sm"
      onClick={onClose}
    >
      <button
        className="absolute right-4 top-4 rounded-full bg-white/10 p-2 text-white hover:bg-white/20 transition-colors"
        onClick={onClose}
      >
        <X className="h-6 w-6" />
      </button>
      <img
        src={url}
        alt=""
        draggable={false}
        className="max-h-[90vh] max-w-[90vw] rounded-2xl object-contain shadow-2xl"
        onClick={e => e.stopPropagation()}
      />
    </div>
  )
}

// ─── ModelCard component ──────────────────────────────────────────────────────

function ModelCard({
  result,
  isSelected,
  onSelect,
  onZoom,
}: {
  result: GenerationResult
  isSelected: boolean
  onSelect: () => void
  onZoom: (url: string) => void
}) {
  const isActive = result.status === 'generating' || result.status === 'pending'
  const isFailed = result.status === 'failed'

  return (
    <div
      data-no-pan
      onClick={onSelect}
      className={`group/card w-56 flex-shrink-0 cursor-pointer select-none rounded-2xl border-2 bg-white shadow-sm transition-all ${
        isSelected
          ? 'border-indigo-400 shadow-indigo-100 shadow-lg ring-1 ring-indigo-200'
          : 'border-surface-200 hover:border-indigo-200 hover:shadow-md'
      }`}
    >
      {/* Header */}
      <div className={`flex items-center justify-between rounded-t-xl px-3 py-2 ${isSelected ? 'bg-indigo-50' : 'bg-surface-50'}`}>
        <span className="truncate text-xs font-semibold text-surface-700">{result.modelName}</span>
        <div className="flex shrink-0 items-center gap-1.5">
          {isActive && <Loader2 className="h-3.5 w-3.5 animate-spin text-indigo-500" />}
          {isFailed && <AlertCircle className="h-3.5 w-3.5 text-red-400" />}
          {result.status === 'completed' && <CheckCircle2 className="h-3.5 w-3.5 text-emerald-500" />}
          {isSelected && (
            <span className="rounded-full bg-indigo-500 px-1.5 py-0.5 text-[9px] text-white">已选</span>
          )}
        </div>
      </div>

      {/* Image area */}
      <div className="relative bg-surface-50" style={{ height: 200 }}>
        {result.imageUrl ? (
          <>
            <img
              src={result.imageUrl}
              alt={result.modelName}
              draggable={false}
              className="h-full w-full cursor-zoom-in object-cover"
              onClick={e => { e.stopPropagation(); onZoom(result.imageUrl) }}
            />
            <button
              data-no-pan
              className="absolute right-2 top-2 rounded-full bg-black/40 p-1 text-white opacity-0 transition-opacity group-hover/card:opacity-100 hover:bg-black/60"
              onClick={e => { e.stopPropagation(); onZoom(result.imageUrl) }}
              title="查看大图"
            >
              <Maximize2 className="h-3.5 w-3.5" />
            </button>
          </>
        ) : (
          <div className="flex h-full items-center justify-center">
            {isActive ? (
              <div className="text-center">
                <Loader2 className="mx-auto h-7 w-7 animate-spin text-indigo-300" />
                <p className="mt-2 text-[11px] text-surface-400">生成中…</p>
              </div>
            ) : isFailed ? (
              <div className="text-center">
                <AlertCircle className="mx-auto h-7 w-7 text-red-300" />
                <p className="mt-2 text-[11px] text-red-400">生成失败</p>
              </div>
            ) : (
              <p className="text-[11px] text-surface-300">等待中…</p>
            )}
          </div>
        )}

        {/* Versions badge */}
        {result.generatedImages.length > 1 && (
          <div className="absolute bottom-2 right-2 rounded-full bg-black/50 px-2 py-0.5 text-[10px] text-white">
            {result.generatedImages.length} 版本
          </div>
        )}

        {/* Selected overlay */}
        {isSelected && (
          <div className="absolute inset-0 flex items-end justify-center bg-indigo-600/5 pb-3">
            <div className="rounded-full bg-indigo-600/90 px-3 py-1 text-[11px] text-white backdrop-blur-sm">
              <Wand2 className="mr-1 inline h-3 w-3" />右侧面板修改
            </div>
          </div>
        )}
      </div>

      {/* Thumbnail strip */}
      {result.generatedImages.length > 1 && (
        <div className="flex gap-1.5 overflow-x-auto p-2">
          {result.generatedImages.slice(0, 4).map(url => (
            <img
              key={url}
              src={url}
              alt=""
              draggable={false}
              className={`h-11 w-11 flex-shrink-0 rounded-lg border object-cover ${
                url === result.imageUrl ? 'border-indigo-400' : 'border-surface-200'
              }`}
            />
          ))}
          {result.generatedImages.length > 4 && (
            <div className="flex h-11 w-11 flex-shrink-0 items-center justify-center rounded-lg border border-surface-200 bg-surface-50 text-[10px] text-surface-400">
              +{result.generatedImages.length - 4}
            </div>
          )}
        </div>
      )}

      {/* Modification count */}
      {result.agentHistory.filter(m => m.role === 'user').length > 0 && (
        <div className="px-3 pb-2 text-[10px] text-surface-400">
          已修改 {result.agentHistory.filter(m => m.role === 'user').length} 次
        </div>
      )}
    </div>
  )
}

// ─── Main Page ────────────────────────────────────────────────────────────────

export default function ImagesPage() {
  const { toast } = useToast()

  const [selectedProjectId, setSelectedProjectId] = useState('0')
  const [prompt, setPrompt] = useState('')
  const [stylePreset, setStylePreset] = useState<StylePresetKey>('')
  const [optimizing, setOptimizing] = useState(false)
  const [generating, setGenerating] = useState(false)
  const [lightboxUrl, setLightboxUrl] = useState<string | null>(null)
  const [sessions, setSessions] = useState<GenerationSession[]>([])
  const [activeSessionId, setActiveSessionId] = useState<string | null>(null)
  const [selectedModelKey, setSelectedModelKey] = useState<string | null>(null)
  const [chatInput, setChatInput] = useState('')
  const [sendingChat, setSendingChat] = useState(false)
  const [selectedChatModelId, setSelectedChatModelId] = useState('')
  const [assetRefreshInterval, setAssetRefreshInterval] = useState(0)
  const [showHistory, setShowHistory] = useState(false)
  const chatBottomRef = useRef<HTMLDivElement>(null)

  const canvas = useCanvas()

  useEffect(() => { setSessions(loadSessions()) }, [])

  // Projects
  const { data: projectsRaw } = useSWR(
    ['image-projects'],
    () => projectAPI.list({ project_type: 'image', page: 1, page_size: 100 }) as unknown as Promise<{ data?: { items?: Project[] } | Project[] }>
  )
  const projectsData = (projectsRaw as any)?.data
  const projects = useMemo<Project[]>(() => Array.isArray(projectsData) ? projectsData : (projectsData?.items ?? []), [projectsData])

  // 不再自动选第一个项目，保持「不选项目」状态，允许直接生成

  const numericProjectId = Number(selectedProjectId)

  // Image models
  const { data: imageModelsRaw } = useSWR(
    ['image-models'],
    () => modelAPI.list({ type: 'image', enabled: 'true', sort_by: 'priority' }) as unknown as Promise<{ data?: Model[] }>
  )
  const imageModels = useMemo<Model[]>(() => (imageModelsRaw as any)?.data ?? [], [imageModelsRaw])
  const activeImageModels = useMemo(() => imageModels.filter(m => m.is_active), [imageModels])

  // Chat models
  const { data: chatModelsRaw } = useSWR(
    ['image-chat-models'],
    () => modelAPI.list({ type: 'llm', sort_by: 'priority' }) as unknown as Promise<{ data?: Model[] }>
  )
  const chatModels = useMemo<Model[]>(() => ((chatModelsRaw as any)?.data ?? []).filter((m: Model) => m.is_active), [chatModelsRaw])

  useEffect(() => {
    if (!selectedChatModelId && chatModels.length > 0) {
      setSelectedChatModelId(String((chatModels.find(m => m.is_default) ?? chatModels[0]).id))
    }
  }, [chatModels, selectedChatModelId])

  // Assets (with polling)
  const { data: assetsRaw, mutate: mutateAssets } = useSWR(
    numericProjectId > 0 ? ['image-assets', numericProjectId] : null,
    () => assetAPI.listPaginated(numericProjectId, { page: 1, page_size: 200 }) as unknown as Promise<{ data?: { items?: Asset[] } }>,
    { refreshInterval: assetRefreshInterval }
  )
  const assets = useMemo<Asset[]>(() => (assetsRaw as any)?.data?.items ?? [], [assetsRaw])

  useEffect(() => {
    const active = assets.some(a => a.status === 'generating' || a.status === 'extracting')
    setAssetRefreshInterval(active ? 3000 : 0)
  }, [assets])

  // Active session results
  const activeSession = useMemo(() => sessions.find(s => s.id === activeSessionId), [sessions, activeSessionId])

  const sessionResults = useMemo((): GenerationResult[] => {
    if (!activeSession) return []
    return Object.entries(activeSession.assetIds).map(([modelKey, assetId]) => {
      const asset = assets.find(a => a.id === assetId)
      const model = imageModels.find(m => m.model_key === modelKey)
      const status: GenerationResult['status'] = !asset
        ? 'pending'
        : asset.status === 'completed'
          ? 'completed'
          : (asset.status === 'failed' || asset.status === 'qa_failed')
            ? 'failed'
            : asset.status === 'generating'
              ? 'generating'
              : 'pending'
      return {
        modelKey,
        modelName: model?.name ?? modelKey,
        assetId,
        status,
        imageUrl: asset ? getSelectedImageUrl(asset) : '',
        generatedImages: asset ? getGeneratedImages(asset) : [],
        agentHistory: (asset?.agent_history ?? []) as ChatMessage[],
      }
    })
  }, [activeSession, assets, imageModels])

  const selectedResult = selectedModelKey ? (sessionResults.find(r => r.modelKey === selectedModelKey) ?? null) : null
  const selectedAsset = selectedResult ? (assets.find(a => a.id === selectedResult.assetId) ?? null) : null

  useEffect(() => {
    chatBottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [selectedAsset?.agent_history?.length])

  // ── Generate all models ──────────────────────────────────────────────────

  const handleGenerate = async () => {
    if (!prompt.trim()) return toast({ title: '请输入图片描述', variant: 'destructive' })
    if (activeImageModels.length === 0) return toast({ title: '当前没有可用的图片模型', variant: 'destructive' })

    // Build style-enhanced prompt suffix for the backend pipeline
    const styleInfo = STYLE_PRESETS.find(s => s.key === stylePreset)
    const promptSuffix = styleInfo ? styleInfo.keywords : undefined
    const stylePresetBackend = STYLE_PRESET_BACKEND_MAP[stylePreset] ?? stylePreset
    const descriptionText = prompt.trim()

    setGenerating(true)
    const sessionId = String(Date.now())
    const assetIds: Record<string, number> = {}

    try {
      // 若未选项目，自动创建一个「快速图片」图片项目
      let targetProjectId = numericProjectId
      if (!targetProjectId) {
        const now = new Date()
        const label = `快速图片 ${now.getMonth() + 1}-${now.getDate()} ${String(now.getHours()).padStart(2, '0')}:${String(now.getMinutes()).padStart(2, '0')}`
        const created = await projectAPI.create({ title: label, project_type: 'image' }) as unknown as { data?: Project }
        targetProjectId = created?.data?.id ?? 0
        if (!targetProjectId) throw new Error('自动创建项目失败，请手动选择或新建项目')
        setSelectedProjectId(String(targetProjectId))
      }

      for (const model of activeImageModels) {
        const res = await assetAPI.create(targetProjectId, {
          type: 'image' as AssetType,
          name: `${descriptionText.slice(0, 28)} [${model.name}]`,
          description: descriptionText,
          is_manual: true,
        }) as unknown as { data?: Asset }
        const asset = res?.data
        if (!asset?.id) continue
        assetIds[model.model_key] = asset.id
      }

      if (Object.keys(assetIds).length === 0) throw new Error('所有模型创建资源失败')

      await Promise.allSettled(
        Object.entries(assetIds).map(([modelKey, assetId]) => {
          return assetAPI.generate(targetProjectId, assetId, modelKey, promptSuffix, stylePresetBackend)
        })
      )

      const newSession: GenerationSession = {
        id: sessionId,
        prompt: prompt.trim(),
        createdAt: new Date().toISOString(),
        projectId: targetProjectId,
        assetIds,
      }
      const next = [newSession, ...sessions]
      setSessions(next)
      saveSessions(next)
      setActiveSessionId(sessionId)
      setSelectedModelKey(null)
      canvas.reset()
      setAssetRefreshInterval(3000)
      await mutateAssets()
      toast({ title: `已向 ${Object.keys(assetIds).length} 个模型提交生成任务`, variant: 'success' })
    } catch (err: any) {
      toast({ title: '生成失败', description: err?.response?.data?.error ?? err?.message ?? '服务器错误', variant: 'destructive' })
    } finally {
      setGenerating(false)
    }
  }

  // ── Chat — only modifies selected model ─────────────────────────────────

  const handleChatSubmit = async () => {
    if (!selectedAsset || !chatInput.trim() || !numericProjectId || !selectedModelKey) return
    const outgoing = chatInput.trim()
    const imageDisplayName = imageModels.find(m => m.model_key === selectedModelKey)?.name
    const chatModelName = chatModels.find(m => String(m.id) === selectedChatModelId)?.name
    setSendingChat(true)
    setChatInput('')
    try {
      const res = await assetAPI.chat(numericProjectId, selectedAsset.id, outgoing, chatModelName) as unknown as { data: Asset }
      const nextAsset = res.data ?? selectedAsset
      await assetAPI.generate(numericProjectId, nextAsset.id, selectedModelKey)
      setAssetRefreshInterval(3000)
      await mutateAssets()
      toast({ title: `${imageDisplayName ?? '模型'} 正在根据要求重新生成`, variant: 'success' })
    } catch {
      setChatInput(outgoing)
      toast({ title: 'AI 改图失败', variant: 'destructive' })
    } finally {
      setSendingChat(false)
    }
  }

  // ── Session management ───────────────────────────────────────────────────

  const handleRestoreSession = (session: GenerationSession) => {
    setSelectedProjectId(String(session.projectId))
    setActiveSessionId(session.id)
    setPrompt(session.prompt)
    setSelectedModelKey(null)
    setShowHistory(false)
    canvas.reset()
  }

  const handleDeleteSession = (id: string, e: React.MouseEvent) => {
    e.stopPropagation()
    const next = sessions.filter(s => s.id !== id)
    setSessions(next)
    saveSessions(next)
    if (activeSessionId === id) {
      setActiveSessionId(next[0]?.id ?? null)
      setSelectedModelKey(null)
    }
  }

  // ─── Render ───────────────────────────────────────────────────────────────

  return (
    <div className="flex min-h-0 flex-col gap-4">

      {/* Lightbox */}
      {lightboxUrl && <Lightbox url={lightboxUrl} onClose={() => setLightboxUrl(null)} />}

      {/* ── Prompt bar ── */}
      <div className="rounded-2xl border border-surface-200 bg-white p-4 shadow-sm">
        <div className="flex flex-wrap items-start gap-3">
          <div className="min-w-0 flex-1">
            <Textarea
              value={prompt}
              onChange={e => setPrompt(e.target.value)}
              onKeyDown={e => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); handleGenerate() } }}
              rows={2}
              placeholder="描述你想生成的图片，例如：一个女人站在日落时分的海边，穿着白色连衣裙，远处帆船点点，光影柔和… (Enter 发送)"
              className="resize-none text-sm"
            />
            {/* Style preset chips */}
            <div className="mt-2 flex flex-wrap items-center gap-1.5">
              <span className="text-[11px] text-surface-400 shrink-0">画面风格：</span>
              <button
                type="button"
                onClick={() => setStylePreset('')}
                className={`rounded-full border px-2.5 py-0.5 text-[11px] font-medium transition-colors ${
                  stylePreset === ''
                    ? 'border-surface-400 bg-surface-700 text-white'
                    : 'border-surface-200 bg-white text-surface-500 hover:border-surface-300'
                }`}
              >
                不限
              </button>
              {STYLE_PRESETS.map(preset => (
                <button
                  key={preset.key}
                  type="button"
                  onClick={() => setStylePreset(prev => prev === preset.key ? '' : preset.key)}
                  title={preset.description}
                  className={`rounded-full border px-2.5 py-0.5 text-[11px] font-medium transition-colors ${
                    stylePreset === preset.key
                      ? 'border-indigo-400 bg-indigo-600 text-white'
                      : 'border-surface-200 bg-white text-surface-600 hover:border-indigo-200 hover:bg-indigo-50 hover:text-indigo-700'
                  }`}
                >
                  {preset.label}
                </button>
              ))}
              <button
                type="button"
                disabled={optimizing || !prompt.trim()}
                onClick={() => {
                  if (!prompt.trim()) return
                  setOptimizing(true)
                  setTimeout(() => {
                    setPrompt(enhancePromptClientSide(prompt, stylePreset))
                    setOptimizing(false)
                    toast({ title: '描述已优化', description: '已添加画质和风格关键词', variant: 'success' })
                  }, 300)
                }}
                className="ml-auto flex items-center gap-1 rounded-full border border-amber-200 bg-amber-50 px-2.5 py-0.5 text-[11px] font-medium text-amber-700 transition-colors hover:bg-amber-100 disabled:opacity-50"
              >
                {optimizing
                  ? <Loader2 className="h-3 w-3 animate-spin" />
                  : <Sparkles className="h-3 w-3" />}
                优化描述
              </button>
            </div>
          </div>

          <div className="flex flex-col gap-2">
            <Select value={selectedProjectId} onValueChange={setSelectedProjectId}>
              <SelectTrigger className="w-44">
                <SelectValue placeholder="不选项目（快速生成）" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="0">不选项目（快速生成）</SelectItem>
                {projects.map(p => (
                  <SelectItem key={p.id} value={String(p.id)}>{p.title}</SelectItem>
                ))}
              </SelectContent>
            </Select>
            <div className="flex gap-2">
              <Button onClick={handleGenerate} disabled={generating || !prompt.trim()} className="flex-1">
                {generating ? <Loader2 className="mr-1.5 h-4 w-4 animate-spin" /> : <Sparkles className="mr-1.5 h-4 w-4" />}
                {generating ? '生成中…' : '全模型生成'}
              </Button>
              <Button
                variant="outline"
                size="icon"
                onClick={() => setShowHistory(v => !v)}
                className={showHistory ? 'border-indigo-200 bg-indigo-50 text-indigo-700' : ''}
                title="历史生成记录"
              >
                <History className="h-4 w-4" />
              </Button>
              <Button variant="outline" size="icon" asChild title="新建图片项目">
                <Link href={PROJECT_MEDIA_META.image.createHref}>
                  <Plus className="h-4 w-4" />
                </Link>
              </Button>
            </div>
          </div>
        </div>

        {/* Active model tags */}
        {activeImageModels.length > 0 && (
          <div className="mt-3 flex flex-wrap items-center gap-1.5">
            <span className="text-xs text-surface-400">将同时使用：</span>
            {activeImageModels.map(m => (
              <span key={m.id} className="rounded-full border border-indigo-100 bg-indigo-50 px-2.5 py-0.5 text-[11px] font-medium text-indigo-600">
                {m.name}
              </span>
            ))}
            {stylePreset && (
              <span className="rounded-full border border-violet-200 bg-violet-50 px-2.5 py-0.5 text-[11px] font-medium text-violet-700">
                风格：{STYLE_PRESETS.find(s => s.key === stylePreset)?.label}
              </span>
            )}
            {sessions.length > 0 && (
              <span className="ml-auto text-[11px] text-surface-400">
                历史 {sessions.length} 条
              </span>
            )}
          </div>
        )}
      </div>

      {/* ── Main: History | Canvas | Right chat panel ── */}
      <div className="flex min-h-0 gap-4" style={{ height: 'calc(100vh - 280px)', minHeight: 520 }}>

        {/* History sidebar */}
        {showHistory && (
          <div className="flex w-64 shrink-0 flex-col overflow-hidden rounded-2xl border border-surface-200 bg-white shadow-sm">
            <div className="flex shrink-0 items-center justify-between border-b border-surface-200 px-4 py-3">
              <span className="text-sm font-semibold text-surface-800">历史生成</span>
              <div className="flex items-center gap-1.5">
                {sessions.length > 0 && (
                  <Badge variant="secondary" className="h-5 min-w-[1.25rem] px-1.5 text-[10px]">{sessions.length}</Badge>
                )}
                <Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => setShowHistory(false)}>
                  <X className="h-4 w-4" />
                </Button>
              </div>
            </div>
            <div className="flex-1 space-y-2 overflow-y-auto p-3">
              {sessions.length === 0 ? (
                <p className="py-10 text-center text-sm text-surface-400">暂无历史记录</p>
              ) : sessions.map(session => (
                <div
                  key={session.id}
                  onClick={() => handleRestoreSession(session)}
                  className={`group relative cursor-pointer rounded-xl border p-3 transition-colors ${
                    session.id === activeSessionId
                      ? 'border-indigo-300 bg-indigo-50'
                      : 'border-surface-200 bg-surface-50 hover:border-indigo-200 hover:bg-white'
                  }`}
                >
                  <p className="line-clamp-2 pr-6 text-sm font-medium text-surface-800">{session.prompt}</p>
                  <div className="mt-1.5 flex items-center justify-between">
                    <span className="text-[11px] text-surface-400">
                      {format(new Date(session.createdAt), 'MM-dd HH:mm')}
                    </span>
                    <span className="text-[11px] text-surface-400">
                      {Object.keys(session.assetIds).length} 模型
                    </span>
                  </div>
                  {session.id === activeSessionId && (
                    <Badge variant="outline" className="mt-1 h-4 px-1.5 text-[10px] text-indigo-600 border-indigo-200">当前</Badge>
                  )}
                  <Button
                    variant="ghost" size="icon"
                    className="absolute right-1.5 top-1.5 h-6 w-6 opacity-0 group-hover:opacity-100"
                    onClick={e => handleDeleteSession(session.id, e)}
                  >
                    <Trash2 className="h-3.5 w-3.5 text-red-400" />
                  </Button>
                </div>
              ))}
            </div>
          </div>
        )}

        {/* Canvas */}
        <div
          className="relative flex-1 overflow-hidden rounded-2xl border border-surface-200 bg-[radial-gradient(circle_at_1px_1px,_rgba(148,163,184,0.25)_1px,_transparent_0)] bg-[size:28px_28px] shadow-sm"
          style={{ cursor: 'grab', userSelect: 'none' }}
          onMouseDown={canvas.onMouseDown}
          onMouseMove={canvas.onMouseMove}
          onMouseUp={canvas.onMouseUp}
          onMouseLeave={canvas.onMouseUp}
          onWheel={canvas.onWheel}
        >
          {/* Canvas top-left info */}
          <div className="absolute left-3 top-3 z-10 flex items-center gap-2" data-no-pan>
            {activeSession && (
              <div className="max-w-sm truncate rounded-lg border border-surface-200 bg-white/90 px-3 py-1.5 text-xs text-surface-600 shadow-sm backdrop-blur-sm">
                <span className="font-medium">&quot;{activeSession.prompt}&quot;</span>
                <span className="ml-2 text-surface-400">{format(new Date(activeSession.createdAt), 'MM-dd HH:mm')}</span>
              </div>
            )}
          </div>

          <div className="absolute right-3 top-3 z-10" data-no-pan>
            <Button
              variant="outline" size="icon"
              className="h-8 w-8 bg-white/90 shadow-sm backdrop-blur-sm"
              onClick={canvas.reset}
              title="重置视图"
            >
              <RotateCcw className="h-3.5 w-3.5" />
            </Button>
          </div>

          <div className="absolute bottom-3 right-3 z-10 select-none text-[11px] text-surface-300" data-no-pan>
            滚轮缩放 · 拖拽平移
          </div>

          {/* Canvas inner */}
          <div
            style={{
              position: 'absolute',
              top: 0,
              left: 0,
              transform: `translate(${canvas.offset.x}px, ${canvas.offset.y}px) scale(${canvas.scale})`,
              transformOrigin: '0 0',
            }}
          >
            {!activeSession || sessionResults.length === 0 ? (
              <div className="flex items-center justify-center" style={{ width: '70vw', height: '50vh' }}>
                <div className="pointer-events-none select-none text-center">
                  <ImageIcon className="mx-auto h-16 w-16 text-surface-200" />
                  <p className="mt-4 text-lg font-medium text-surface-300">输入描述，点击「全模型生成」</p>
                  <p className="mt-1.5 text-sm text-surface-200">所有激活的 AI 图片模型将同时生成，并排展示对比结果</p>
                </div>
              </div>
            ) : (
              <div className="flex flex-wrap gap-5 p-6">
                {sessionResults.map(result => (
                  <ModelCard
                    key={result.modelKey}
                    result={result}
                    isSelected={selectedModelKey === result.modelKey}
                    onSelect={() => setSelectedModelKey(prev => prev === result.modelKey ? null : result.modelKey)}
                    onZoom={url => setLightboxUrl(url)}
                  />
                ))}
              </div>
            )}
          </div>
        </div>

        {/* Right: chat panel for selected model */}
        {selectedResult && selectedAsset && (
          <div
            className="flex w-72 shrink-0 flex-col overflow-hidden rounded-2xl border border-indigo-200 bg-white shadow-sm"
            data-no-pan
          >
            {/* Header */}
            <div className="flex shrink-0 items-center justify-between border-b border-surface-200 px-4 py-3">
              <div className="min-w-0">
                <p className="text-sm font-semibold text-surface-800">修改此图</p>
                <p className="truncate text-xs text-indigo-600">{selectedResult.modelName}</p>
              </div>
              <Button variant="ghost" size="icon" className="h-7 w-7 shrink-0" onClick={() => setSelectedModelKey(null)}>
                <X className="h-4 w-4" />
              </Button>
            </div>

            {/* Preview */}
            <div className="shrink-0 p-3">
              <div className="group/preview relative overflow-hidden rounded-xl border border-surface-200 bg-surface-50">
                {selectedResult.imageUrl ? (
                  <>
                    <img
                      src={selectedResult.imageUrl}
                      alt=""
                      className="w-full cursor-zoom-in object-cover"
                      style={{ maxHeight: 180 }}
                      onClick={() => setLightboxUrl(selectedResult.imageUrl)}
                    />
                    <button
                      className="absolute right-2 top-2 rounded-full bg-black/40 p-1 text-white opacity-0 transition-opacity group-hover/preview:opacity-100 hover:bg-black/60"
                      onClick={() => setLightboxUrl(selectedResult.imageUrl)}
                      title="查看大图"
                    >
                      <Maximize2 className="h-3.5 w-3.5" />
                    </button>
                  </>
                ) : (
                  <div className="flex h-32 items-center justify-center text-xs text-surface-400">
                    {selectedResult.status === 'generating'
                      ? <><Loader2 className="mr-1.5 h-4 w-4 animate-spin" />生成中…</>
                      : '暂无图片'}
                  </div>
                )}
              </div>

              {/* Thumbnails */}
              {selectedResult.generatedImages.length > 1 && (
                <div className="mt-2 flex gap-1.5 overflow-x-auto pb-1">
                  {selectedResult.generatedImages.map(url => (
                    <img
                      key={url}
                      src={url}
                      alt=""
                      draggable={false}
                      onClick={() => setLightboxUrl(url)}
                      className={`h-10 w-10 flex-shrink-0 cursor-zoom-in rounded-lg border object-cover ${
                        url === selectedResult.imageUrl ? 'border-indigo-400' : 'border-surface-200'
                      }`}
                    />
                  ))}
                </div>
              )}

              {/* Generating status */}
              {selectedResult.status === 'generating' && (
                <div className="mt-2 flex items-center gap-2 rounded-lg border border-blue-100 bg-blue-50 px-3 py-2 text-xs text-blue-700">
                  <Loader2 className="h-3.5 w-3.5 animate-spin" />
                  正在生成新版本，稍后可见
                </div>
              )}
            </div>

            {/* Chat history */}
            <div className="flex-1 space-y-2 overflow-y-auto px-3 pb-2 pt-1">
              <p className="border-b border-surface-100 pb-1 text-[11px] text-surface-400">
                对话记录 · 仅修改 {selectedResult.modelName}
              </p>
              {(selectedAsset.agent_history ?? []).length === 0 ? (
                <p className="py-6 text-center text-xs text-surface-400">
                  告诉 AI 如何修改这张图<br />
                  <span className="text-surface-300">例如：背景改成星空、人物表情更自然…</span>
                </p>
              ) : (
                (selectedAsset.agent_history ?? []).map((msg, i) => {
                  const content = msg.content?.trim() ?? ''
                  if (!content) return null
                  return (
                    <div key={i} className={`flex ${msg.role === 'user' ? 'justify-end' : 'justify-start'}`}>
                      <div className={`max-w-[88%] rounded-xl px-3 py-2 text-xs leading-5 ${
                        msg.role === 'user' ? 'bg-indigo-600 text-white' : 'bg-surface-100 text-surface-700'
                      }`}>
                        {content}
                      </div>
                    </div>
                  )
                })
              )}
              <div ref={chatBottomRef} />
            </div>

            {/* Chat input */}
            <div className="shrink-0 space-y-2 border-t border-surface-200 p-3">
              <Select value={selectedChatModelId} onValueChange={setSelectedChatModelId}>
                <SelectTrigger className="h-7 text-xs">
                  <SelectValue placeholder="对话模型" />
                </SelectTrigger>
                <SelectContent>
                  {chatModels.map(m => (
                    <SelectItem key={m.id} value={String(m.id)}>{m.name}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <div className="flex gap-2">
                <Textarea
                  value={chatInput}
                  onChange={e => setChatInput(e.target.value)}
                  rows={3}
                  placeholder="描述修改要求，例如：背景换成夜晚、光线更柔和…"
                  className="resize-none text-xs"
                  onKeyDown={e => {
                    if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) handleChatSubmit()
                  }}
                />
                <Button
                  size="sm"
                  onClick={handleChatSubmit}
                  disabled={sendingChat || !chatInput.trim() || selectedResult.status === 'generating'}
                  className="self-end"
                >
                  {sendingChat ? <Loader2 className="h-4 w-4 animate-spin" /> : <Send className="h-4 w-4" />}
                </Button>
              </div>
              <p className="text-[10px] text-surface-300">⌘ Enter 发送 · 仅修改 {selectedResult.modelName}</p>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
