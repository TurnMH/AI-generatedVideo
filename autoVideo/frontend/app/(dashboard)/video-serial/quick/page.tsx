'use client'

import { useState, useRef, useEffect, useCallback } from 'react'
import { useRouter } from 'next/navigation'
import {
  Wand2,
  Sparkles,
  Loader2,
  Video,
  ChevronRight,
  Check,
  AlertCircle,
  Zap,
  RefreshCw,
  ArrowRight,
  Settings2,
  ChevronDown,
  Cpu,
  Image as ImageIcon,
  Volume2,
  VolumeX,
  Clock,
  CheckCircle2,
  XCircle,
  ExternalLink,
  Trash2,
  History,
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { useToast } from '@/components/ui/toast'
import { scriptLibraryAPI, modelAPI, taskAPI } from '@/lib/api'
import { savePendingProjectDraft } from '@/lib/project-draft'
import { cn } from '@/lib/utils'
import type { Model } from '@/types'

type Step = 'input' | 'optimizing' | 'preview' | 'creating' | 'done'

interface OptimizedResult {
  title: string
  content: string
  genre: string
  tags: string[]
  outline: string[]
  word_count: number
}

/* ─── Generation History ─────────────────────────────────── */
interface QuickGenRecord {
  id: string
  taskId?: number
  title: string
  premise: string
  status: 'pending' | 'succeeded' | 'failed'
  createdAt: string
  draftId?: string
  errorMsg?: string
  textModel?: string
  imageModel?: string
  enableVoice?: boolean
  result?: OptimizedResult
}

const HISTORY_KEY = 'autovideo:quick-serial-gen-history'
const MAX_HISTORY = 20

function loadHistory(): QuickGenRecord[] {
  if (typeof window === 'undefined') return []
  try {
    return JSON.parse(localStorage.getItem(HISTORY_KEY) ?? '[]') as QuickGenRecord[]
  } catch {
    return []
  }
}

function saveHistory(list: QuickGenRecord[]) {
  localStorage.setItem(HISTORY_KEY, JSON.stringify(list.slice(0, MAX_HISTORY)))
}

function upsertHistory(record: QuickGenRecord) {
  const list = loadHistory()
  const idx = list.findIndex((r) => r.id === record.id)
  if (idx >= 0) list[idx] = record
  else list.unshift(record)
  saveHistory(list)
}

/* ─── Constants ──────────────────────────────────────────── */
const QUICK_EXAMPLES = [
  '一个普通外卖员发现自己收到了来自未来的订单，开始了一段神奇的冒险',
  '总裁爱上了他的秘书，但秘书其实是竞争对手派来的卧底',
  '一只会说话的猫咪帮助主人找到了失散多年的家人',
  '退休警察在乡下养老，却意外发现了一起尘封二十年的悬案',
]

/* ─── Model Select ───────────────────────────────────────── */
function ModelSelect({
  label,
  icon: Icon,
  models,
  value,
  onChange,
  placeholder,
}: {
  label: string
  icon: React.ElementType
  models: Model[]
  value: string
  onChange: (v: string) => void
  placeholder: string
}) {
  return (
    <div className="flex flex-col gap-1.5">
      <label className="flex items-center gap-1.5 text-xs font-medium text-surface-400">
        <Icon className="h-3.5 w-3.5" />
        {label}
      </label>
      {models.length === 0 ? (
        <div className="rounded-lg border border-white/[0.08] bg-surface-700/30 px-3 py-2 text-sm text-surface-500">
          使用系统默认模型
        </div>
      ) : (
        <select
          value={value}
          onChange={(e) => onChange(e.target.value)}
          className="rounded-lg border border-white/[0.08] bg-surface-700/60 px-3 py-2 text-sm text-surface-100 outline-none transition focus:border-primary-500/50 focus:ring-1 focus:ring-primary-500/20"
        >
          <option value="">{placeholder}</option>
          {models.map((m) => (
            <option key={m.id} value={m.model_key}>
              {m.name}
              {m.is_default ? ' ★' : ''}
            </option>
          ))}
        </select>
      )}
    </div>
  )
}

/* ─── Result Echo Modal ──────────────────────────────────── */
function ResultEchoModal({
  record,
  onClose,
  onCreateProject,
}: {
  record: QuickGenRecord
  onClose: () => void
  onCreateProject: (record: QuickGenRecord) => void
}) {
  const r = record.result
  if (!r) return null
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4 backdrop-blur-sm" onClick={onClose}>
      <div
        className="relative w-full max-w-2xl max-h-[80vh] overflow-y-auto rounded-2xl border border-white/[0.12] bg-surface-900 p-6 shadow-2xl"
        onClick={(e) => e.stopPropagation()}
      >
        <button onClick={onClose} className="absolute right-4 top-4 rounded-lg p-1.5 text-surface-500 hover:bg-surface-700/60 hover:text-white">
          <XCircle className="h-5 w-5" />
        </button>
        <div className="mb-4 flex items-center gap-2">
          <CheckCircle2 className="h-5 w-5 text-emerald-400" />
          <h3 className="text-lg font-bold text-white">{r.title || record.title}</h3>
        </div>
        {(r.genre || r.tags?.length > 0) && (
          <div className="mb-4 flex flex-wrap gap-1.5">
            {r.genre && <span className="rounded-full border border-violet-500/30 bg-violet-500/10 px-2.5 py-0.5 text-xs text-violet-300">{r.genre}</span>}
            {r.tags?.map((t, i) => <span key={i} className="rounded-full border border-white/[0.08] bg-surface-700/50 px-2.5 py-0.5 text-xs text-surface-300">#{t}</span>)}
          </div>
        )}
        {r.outline?.length > 0 && (
          <div className="mb-4 rounded-xl border border-white/[0.08] bg-surface-800/60 p-4">
            <p className="mb-2 text-xs font-semibold uppercase tracking-wider text-surface-400">故事大纲（{r.outline.length} 集）</p>
            <ol className="space-y-1.5">
              {r.outline.map((ep, i) => (
                <li key={i} className="flex items-start gap-2 text-sm">
                  <span className="mt-0.5 flex h-4 w-4 flex-shrink-0 items-center justify-center rounded-full bg-primary-500/20 text-[10px] font-bold text-primary-400">{i + 1}</span>
                  <span className="text-surface-200">{ep}</span>
                </li>
              ))}
            </ol>
          </div>
        )}
        {r.content && (
          <div className="rounded-xl border border-white/[0.08] bg-surface-800/60 p-4">
            <p className="mb-2 flex items-center justify-between text-xs font-semibold uppercase tracking-wider text-surface-400">
              <span>剧本内容</span>
              <span className="normal-case text-surface-500">{r.word_count ?? r.content.length} 字</span>
            </p>
            <pre className="max-h-64 overflow-y-auto whitespace-pre-wrap text-xs leading-relaxed text-surface-300">
              {r.content}
            </pre>
          </div>
        )}
        <div className="mt-5 flex gap-3">
          <Button
            onClick={() => { onClose(); onCreateProject(record) }}
            className="flex-1 gap-2 bg-gradient-to-r from-violet-600 to-primary-600 hover:from-violet-500 hover:to-primary-500"
          >
            <ArrowRight className="h-4 w-4" />
            创建视频项目
          </Button>
          <Button variant="outline" onClick={onClose} className="border-white/[0.12] text-surface-300 hover:text-white">
            关闭
          </Button>
        </div>
      </div>
    </div>
  )
}

/* ─── History Item ───────────────────────────────────────── */
function HistoryItem({
  record,
  onNavigate,
  onDelete,
  onView,
}: {
  record: QuickGenRecord
  onNavigate: (r: QuickGenRecord) => void
  onDelete: (id: string) => void
  onView: (r: QuickGenRecord) => void
}) {
  const statusMap = {
    pending: { icon: Clock, color: 'text-amber-400', bg: 'bg-amber-500/10 border-amber-500/20', label: '生成中' },
    succeeded: { icon: CheckCircle2, color: 'text-emerald-400', bg: 'bg-emerald-500/10 border-emerald-500/20', label: '已完成' },
    failed: { icon: XCircle, color: 'text-red-400', bg: 'bg-red-500/10 border-red-500/20', label: '失败' },
  }
  const s = statusMap[record.status]
  const StatusIcon = s.icon

  return (
    <div className="group flex items-start gap-3 rounded-xl border border-white/[0.06] bg-surface-800/40 px-4 py-3 transition-all hover:border-white/[0.10] hover:bg-surface-800/60">
      <div className={cn('mt-0.5 flex-shrink-0 rounded-full border p-1', s.bg)}>
        {record.status === 'pending'
          ? <Loader2 className={cn('h-3.5 w-3.5 animate-spin', s.color)} />
          : <StatusIcon className={cn('h-3.5 w-3.5', s.color)} />
        }
      </div>
      <div className="min-w-0 flex-1">
        <div className="flex items-center justify-between gap-2">
          <p className="truncate text-sm font-medium text-surface-100">{record.title || '未命名项目'}</p>
          <span className={cn('flex-shrink-0 text-xs font-medium', s.color)}>{s.label}</span>
        </div>
        <p className="mt-0.5 truncate text-xs text-surface-500">{record.premise}</p>
        <div className="mt-1.5 flex items-center gap-3 text-xs text-surface-600">
          <span>{new Date(record.createdAt).toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })}</span>
          {record.textModel && <span>文本: {record.textModel}</span>}
          {record.imageModel && <span>图像: {record.imageModel}</span>}
          {record.enableVoice && <span className="text-violet-400">含语音</span>}
        </div>
        {record.errorMsg && (
          <p className="mt-1 text-xs text-red-400">{record.errorMsg}</p>
        )}
      </div>
      <div className="flex flex-shrink-0 items-center gap-1 opacity-0 transition-opacity group-hover:opacity-100">
        {record.status === 'succeeded' && record.result && (
          <button
            onClick={() => onView(record)}
            className="rounded-lg p-1.5 text-emerald-400 hover:bg-emerald-500/10"
            title="查看回显结果"
          >
            <ExternalLink className="h-3.5 w-3.5" />
          </button>
        )}
        {record.status === 'succeeded' && !record.result && record.draftId && (
          <button
            onClick={() => onNavigate(record)}
            className="rounded-lg p-1.5 text-surface-400 hover:bg-surface-700/60 hover:text-white"
            title="查看项目"
          >
            <ExternalLink className="h-3.5 w-3.5" />
          </button>
        )}
        <button
          onClick={() => onDelete(record.id)}
          className="rounded-lg p-1.5 text-surface-600 hover:bg-red-500/10 hover:text-red-400"
          title="删除记录"
        >
          <Trash2 className="h-3.5 w-3.5" />
        </button>
      </div>
    </div>
  )
}

/* ─── Page ───────────────────────────────────────────────── */
export default function SerialQuickGeneratePage() {
  const router = useRouter()
  const { toast } = useToast()
  const textareaRef = useRef<HTMLTextAreaElement>(null)

  const [step, setStep] = useState<Step>('input')
  const [userInput, setUserInput] = useState('')
  const [optimized, setOptimized] = useState<OptimizedResult | null>(null)
  const [optimizedContent, setOptimizedContent] = useState('')
  const [optimizedTitle, setOptimizedTitle] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [progress, setProgress] = useState(0)
  const progressRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null)

  // Model selection
  const [showSettings, setShowSettings] = useState(false)
  const [textModels, setTextModels] = useState<Model[]>([])
  const [imageModels, setImageModels] = useState<Model[]>([])
  const [selectedTextModel, setSelectedTextModel] = useState('')
  const [selectedImageModel, setSelectedImageModel] = useState('')
  const [enableVoice, setEnableVoice] = useState(false)
  const [modelsLoading, setModelsLoading] = useState(false)

  // History
  const [history, setHistory] = useState<QuickGenRecord[]>([])
  const currentRecordId = useRef<string | null>(null)
  const [echoRecord, setEchoRecord] = useState<QuickGenRecord | null>(null)

  // Load models & history on mount
  useEffect(() => {
    setHistory(loadHistory())
    setModelsLoading(true)
    Promise.all([
      modelAPI.list({ type: 'llm', sort_by: 'priority' }),
      modelAPI.list({ type: 'image', sort_by: 'priority' }),
    ])
      .then(([llmRes, imgRes]) => {
        // axios interceptor returns res.data = { code, message, data: [...] }
        const allLlms: Model[] = (llmRes as unknown as { data: Model[] })?.data ?? []
        const imgs: Model[] = (imgRes as unknown as { data: Model[] })?.data ?? []
        // LLM models: do NOT pass model_name to script-service — its LLM channel pool
        // is configured independently and model_key from DB may not have a matching channel.
        // Always let script-service use its own default channel.
        setTextModels([])  // hide LLM dropdown
        setImageModels(imgs)
        const defImg = imgs.find((m) => m.is_default) ?? imgs[0]
        if (defImg?.model_key) setSelectedImageModel(defImg.model_key)
      })
      .catch(() => { /* silent — models just won't be pre-filled */ })
      .finally(() => setModelsLoading(false))
  }, [])

  // auto-grow textarea
  useEffect(() => {
    const el = textareaRef.current
    if (!el) return
    el.style.height = 'auto'
    el.style.height = `${el.scrollHeight}px`
  }, [userInput])

  const refreshHistory = useCallback(() => setHistory(loadHistory()), [])

  // cleanup on unmount
  useEffect(() => {
    return () => {
      if (progressRef.current) clearInterval(progressRef.current)
      if (pollRef.current) clearInterval(pollRef.current)
    }
  }, [])

  const startProgress = (from: number, to: number, durationMs: number) => {
    if (progressRef.current) clearInterval(progressRef.current)
    const steps = 60
    const increment = (to - from) / steps
    let current = from
    progressRef.current = setInterval(() => {
      current += increment
      if (current >= to) {
        current = to
        clearInterval(progressRef.current!)
      }
      setProgress(Math.round(current))
    }, durationMs / steps)
  }

  const handleOptimize = async () => {
    if (!userInput.trim()) {
      toast({ title: '请输入内容', description: '请先描述您想要生成的视频内容', variant: 'destructive' })
      return
    }
    setError(null)
    setStep('optimizing')
    setProgress(0)
    startProgress(0, 85, 12000)

    // Create history record
    const recordId = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
    currentRecordId.current = recordId
    const pendingRecord: QuickGenRecord = {
      id: recordId,
      title: '生成中…',
      premise: userInput.trim(),
      status: 'pending',
      createdAt: new Date().toISOString(),
      textModel: selectedTextModel || undefined,
      imageModel: selectedImageModel || undefined,
      enableVoice,
    }
    upsertHistory(pendingRecord)
    setHistory(loadHistory())

    try {
      // 1. 创建 Kafka 任务
      const taskRes = await taskAPI.create({
        task_type: 'script_quick_generate',
        payload: {
          mode: 'script',
          premise: userInput.trim(),
          genre: '短剧',
          platform: '竖屏短视频',
          delivery_format: '分集剧本，每集2-3分钟，每集结尾留悬念',
          episode_duration: '2-3分钟',
          tone: '节奏紧凑、情节吸引、易于理解',
          requirements: `剧情完整，人物立体，语言生动，适合短视频平台传播${enableVoice ? '，需要配音台词清晰' : ''}`,
          target_words: 1500,
          chapter_count: 5,
        },
      }) as unknown as { data: { id: number; task_type: string } }

      const taskId = taskRes?.data?.id
      if (!taskId) throw new Error('创建任务失败，未获取到任务ID')

      // 更新历史记录，存入 taskId
      upsertHistory({ ...pendingRecord, taskId })
      setHistory(loadHistory())

      // 2. 轮询任务状态（每 3 秒一次）
      await new Promise<void>((resolve, reject) => {
        if (pollRef.current) clearInterval(pollRef.current)
        let elapsed = 0
        pollRef.current = setInterval(async () => {
          elapsed += 3
          if (elapsed > 180) {
            clearInterval(pollRef.current!)
            reject(new Error('生成超时，请稍后重试'))
            return
          }
          try {
            const taskResp = await taskAPI.get(taskId) as unknown as {
              data: {
                id: number; status: string; error_msg?: string
                result?: {
                  title: string; description: string; genre: string
                  tags: string[]; outline: string[]; content: string
                  suggested_episodes: number; word_count: number
                }
              }
            }
            const task = taskResp.data
            if (task.status === 'succeeded') {
              clearInterval(pollRef.current!)
              resolve()

              const r = task.result
              if (!r) { reject(new Error('任务完成但结果为空')); return }

              const result: OptimizedResult = {
                title: r.title,
                content: r.content,
                genre: r.genre,
                tags: r.tags,
                outline: r.outline,
                word_count: r.word_count,
              }

              setOptimized(result)
              setOptimizedTitle(result.title || '我的视频项目')
              setOptimizedContent(result.content || '')
              setProgress(100)
              if (progressRef.current) clearInterval(progressRef.current)

              // 更新历史：存入结果 + 标记成功
              const updated: QuickGenRecord = {
                ...pendingRecord,
                taskId,
                title: result.title || '未命名项目',
                status: 'pending', // 仍为 pending，等待用户点击"创建项目"后才标为 succeeded
                result,
              }
              upsertHistory(updated)
              setHistory(loadHistory())

              setTimeout(() => setStep('preview'), 300)
            } else if (task.status === 'failed') {
              clearInterval(pollRef.current!)
              reject(new Error(task.error_msg || 'AI 生成失败'))
            }
          } catch (pollErr) {
            // 轮询网络错误不中断，继续重试
          }
        }, 3000)
      })
    } catch (err: unknown) {
      if (progressRef.current) clearInterval(progressRef.current)
      if (pollRef.current) clearInterval(pollRef.current!)
      const msg = err instanceof Error ? err.message : 'AI 优化失败，请重试'
      setError(msg)
      setStep('input')
      toast({ title: 'AI 优化失败', description: msg, variant: 'destructive' })

      upsertHistory({ ...pendingRecord, title: userInput.slice(0, 20) || '未命名', status: 'failed', errorMsg: msg })
      setHistory(loadHistory())
    }
  }

  const handleCreateProject = async () => {
    setStep('creating')
    startProgress(0, 90, 4000)

    try {
      const draftId = savePendingProjectDraft({
        title: optimizedTitle,
        description: optimized?.genre ?? '',
        scriptContent: optimizedContent,
        scriptFileName: `${optimizedTitle}.txt`,
        scriptMimeType: 'text/plain',
        targetEpisodes: optimized?.outline?.length ?? 5,
        styleTags: optimized?.tags ?? [],
        media: 'video_serial',
      })

      // Mark history as succeeded
      if (currentRecordId.current) {
        const rec = loadHistory().find((r) => r.id === currentRecordId.current)
        if (rec) {
          upsertHistory({ ...rec, status: 'succeeded', draftId, result: optimized ?? rec.result })
          setHistory(loadHistory())
        }
      }

      setProgress(100)
      if (progressRef.current) clearInterval(progressRef.current)
      setStep('done')

      setTimeout(() => {
        router.push(`/video-serial/new?draft=${draftId}`)
      }, 800)
    } catch (err: unknown) {
      if (progressRef.current) clearInterval(progressRef.current)
      const msg = err instanceof Error ? err.message : '创建项目失败'
      setError(msg)
      setStep('preview')
      toast({ title: '创建失败', description: msg, variant: 'destructive' })
    }
  }

  const handleReset = () => {
    setStep('input')
    setOptimized(null)
    setOptimizedContent('')
    setOptimizedTitle('')
    setError(null)
    setProgress(0)
    currentRecordId.current = null
  }

  const handleDeleteHistory = (id: string) => {
    const list = loadHistory().filter((r) => r.id !== id)
    saveHistory(list)
    setHistory(list)
  }

  const handleNavigateHistory = (record: QuickGenRecord) => {
    if (record.draftId) {
      router.push(`/video-serial/new?draft=${record.draftId}`)
    }
  }

  const handleViewHistory = (record: QuickGenRecord) => {
    setEchoRecord(record)
  }

  const handleCreateFromEcho = (record: QuickGenRecord) => {
    if (!record.result) return
    const r = record.result
    setOptimized(r)
    setOptimizedTitle(r.title || '我的视频项目')
    setOptimizedContent(r.content || '')
    currentRecordId.current = record.id
    setStep('preview')
  }

  const STEPS_LIST: Step[] = ['input', 'optimizing', 'preview', 'creating', 'done']
  const STEP_LABELS: Record<Step, string> = {
    input: '输入创意', optimizing: 'AI 优化', preview: '预览确认', creating: '创建项目', done: '完成',
  }

  return (
    <div className="mx-auto max-w-3xl space-y-8 pb-16">
      {/* Result Echo Modal */}
      {echoRecord && (
        <ResultEchoModal
          record={echoRecord}
          onClose={() => setEchoRecord(null)}
          onCreateProject={(rec) => {
            setEchoRecord(null)
            handleCreateFromEcho(rec)
          }}
        />
      )}
      {/* Header */}
      <div className="space-y-2">
        <div className="flex items-center gap-3">
          <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-gradient-to-br from-violet-500 to-primary-500 shadow-glow-sm">
            <Zap className="h-5 w-5 text-white" />
          </div>
          <div>
            <h1 className="text-2xl font-bold text-white">快速生成视频</h1>
            <p className="text-sm text-surface-400">输入您的创意，AI 自动优化并生成完整视频项目</p>
          </div>
        </div>

        {/* Step indicator */}
        <div className="flex items-center gap-2 pt-2">
          {STEPS_LIST.map((s, i) => {
            const stepIndex = STEPS_LIST.indexOf(step)
            const isDone = i < stepIndex
            const isActive = i === stepIndex
            return (
              <div key={s} className="flex items-center">
                <div className={cn(
                  'flex h-6 w-6 items-center justify-center rounded-full text-xs font-semibold transition-all',
                  isDone ? 'bg-emerald-500 text-white' :
                  isActive ? 'bg-primary-500 text-white ring-2 ring-primary-500/30' :
                  'bg-surface-700 text-surface-400'
                )}>
                  {isDone ? <Check className="h-3.5 w-3.5" /> : i + 1}
                </div>
                <span className={cn('ml-1.5 text-xs', isActive ? 'text-white font-medium' : 'text-surface-500')}>{STEP_LABELS[s]}</span>
                {i < 4 && <ChevronRight className="ml-2 h-3.5 w-3.5 text-surface-600" />}
              </div>
            )
          })}
        </div>
      </div>

      {/* ── Step: Input ─────────────────────────────────────── */}
      {step === 'input' && (
        <div className="space-y-5">
          {/* Textarea */}
          <div className="rounded-2xl border border-white/[0.08] bg-surface-800/60 p-6 backdrop-blur-sm">
            <label className="mb-3 flex items-center gap-2 text-sm font-medium text-surface-200">
              <Sparkles className="h-4 w-4 text-violet-400" />
              描述您的创意
            </label>
            <Textarea
              ref={textareaRef}
              value={userInput}
              onChange={(e) => setUserInput(e.target.value)}
              placeholder="用一两句话描述您想要的视频内容，例如：一个普通打工人意外获得了超能力……"
              className="min-h-[120px] resize-none border-white/[0.08] bg-surface-700/50 text-sm text-surface-100 placeholder:text-surface-500 focus:border-primary-500/50 focus:ring-primary-500/20"
              onKeyDown={(e) => { if (e.key === 'Enter' && e.metaKey) handleOptimize() }}
            />
            <p className="mt-2 text-right text-xs text-surface-500">{userInput.length} 字 · ⌘Enter 快速生成</p>
          </div>

          {/* Model Settings (collapsible) */}
          <div className="rounded-2xl border border-white/[0.08] bg-surface-800/40 backdrop-blur-sm">
            <button
              onClick={() => setShowSettings((v) => !v)}
              className="flex w-full items-center justify-between px-5 py-3.5 text-sm font-medium text-surface-300 hover:text-white"
            >
              <span className="flex items-center gap-2">
                <Settings2 className="h-4 w-4 text-surface-400" />
                生成设置
                {(selectedTextModel || selectedImageModel || enableVoice) && (
                  <span className="rounded-full bg-primary-500/20 px-2 py-0.5 text-[10px] font-semibold text-primary-300">已配置</span>
                )}
              </span>
              <ChevronDown className={cn('h-4 w-4 text-surface-500 transition-transform', showSettings && 'rotate-180')} />
            </button>

            {showSettings && (
              <div className="border-t border-white/[0.06] px-5 pb-5 pt-4">
                {modelsLoading ? (
                  <div className="flex items-center gap-2 text-xs text-surface-500">
                    <Loader2 className="h-3.5 w-3.5 animate-spin" /> 加载模型列表…
                  </div>
                ) : (
                  <div className="grid gap-4 sm:grid-cols-2">
                    {/* Text model */}
                    <ModelSelect
                      label="文本模型"
                      icon={Cpu}
                      models={textModels}
                      value={selectedTextModel}
                      onChange={setSelectedTextModel}
                      placeholder="默认文本模型"
                    />
                    {/* Image model */}
                    <ModelSelect
                      label="图像模型"
                      icon={ImageIcon}
                      models={imageModels}
                      value={selectedImageModel}
                      onChange={setSelectedImageModel}
                      placeholder="默认图像模型"
                    />
                  </div>
                )}

                {/* Voice toggle */}
                <div className="mt-4 flex items-center justify-between rounded-xl border border-white/[0.06] bg-surface-700/30 px-4 py-3">
                  <div className="flex items-center gap-2.5">
                    {enableVoice
                      ? <Volume2 className="h-4 w-4 text-violet-400" />
                      : <VolumeX className="h-4 w-4 text-surface-500" />
                    }
                    <div>
                      <p className="text-sm font-medium text-surface-200">AI 语音合成</p>
                      <p className="text-xs text-surface-500">为视频生成台词配音（TTS）</p>
                    </div>
                  </div>
                  <button
                    onClick={() => setEnableVoice((v) => !v)}
                    className={cn(
                      'relative h-6 w-11 rounded-full transition-colors',
                      enableVoice ? 'bg-violet-500' : 'bg-surface-600'
                    )}
                  >
                    <span className={cn(
                      'absolute top-0.5 h-5 w-5 rounded-full bg-white shadow transition-transform',
                      enableVoice ? 'translate-x-[22px]' : 'translate-x-0.5'
                    )} />
                  </button>
                </div>

                {/* Summary of selection */}
                {(selectedTextModel || selectedImageModel || enableVoice) && (
                  <div className="mt-3 flex flex-wrap gap-1.5">
                    {selectedTextModel && (
                      <span className="rounded-full border border-blue-500/20 bg-blue-500/10 px-2.5 py-0.5 text-xs text-blue-300">
                        文本: {textModels.find(m => m.model_key === selectedTextModel)?.name ?? selectedTextModel}
                      </span>
                    )}
                    {selectedImageModel && (
                      <span className="rounded-full border border-emerald-500/20 bg-emerald-500/10 px-2.5 py-0.5 text-xs text-emerald-300">
                        图像: {imageModels.find(m => m.model_key === selectedImageModel)?.name ?? selectedImageModel}
                      </span>
                    )}
                    {enableVoice && (
                      <span className="rounded-full border border-violet-500/20 bg-violet-500/10 px-2.5 py-0.5 text-xs text-violet-300">
                        语音已启用
                      </span>
                    )}
                  </div>
                )}
              </div>
            )}
          </div>

          {/* Examples */}
          <div className="space-y-2">
            <p className="text-xs font-medium uppercase tracking-wider text-surface-500">快捷示例</p>
            <div className="grid gap-2">
              {QUICK_EXAMPLES.map((ex, i) => (
                <button
                  key={i}
                  onClick={() => setUserInput(ex)}
                  className="rounded-xl border border-white/[0.06] bg-surface-800/40 px-4 py-3 text-left text-sm text-surface-300 transition-all hover:border-primary-500/30 hover:bg-surface-700/50 hover:text-white"
                >
                  <span className="mr-2 text-primary-400">→</span>
                  {ex}
                </button>
              ))}
            </div>
          </div>

          {error && (
            <div className="flex items-center gap-2 rounded-xl border border-red-500/20 bg-red-500/10 px-4 py-3 text-sm text-red-300">
              <AlertCircle className="h-4 w-4 flex-shrink-0" />
              {error}
            </div>
          )}

          <Button
            onClick={handleOptimize}
            disabled={!userInput.trim()}
            className="w-full gap-2 bg-gradient-to-r from-violet-600 to-primary-600 py-6 text-base font-semibold hover:from-violet-500 hover:to-primary-500"
          >
            <Wand2 className="h-5 w-5" />
            AI 智能优化并生成
          </Button>
        </div>
      )}

      {/* ── Step: Optimizing ────────────────────────────────── */}
      {step === 'optimizing' && (
        <div className="flex flex-col items-center justify-center space-y-6 py-16">
          <div className="relative flex h-20 w-20 items-center justify-center">
            <div className="absolute inset-0 animate-ping rounded-full bg-primary-500/20" />
            <div className="absolute inset-2 animate-spin rounded-full border-2 border-transparent border-t-primary-400" />
            <Sparkles className="h-8 w-8 text-primary-400" />
          </div>
          <div className="text-center">
            <p className="text-lg font-semibold text-white">AI 正在优化您的创意…</p>
            <p className="mt-1 text-sm text-surface-400">正在生成剧本、分析人物、构建故事结构</p>
            {selectedTextModel && (
              <p className="mt-1 text-xs text-surface-500">使用模型: {textModels.find(m => m.model_key === selectedTextModel)?.name ?? selectedTextModel}</p>
            )}
          </div>
          <div className="w-full max-w-xs">
            <div className="mb-1.5 flex justify-between text-xs text-surface-500">
              <span>处理中</span>
              <span>{progress}%</span>
            </div>
            <div className="h-2 overflow-hidden rounded-full bg-surface-700">
              <div
                className="h-full rounded-full bg-gradient-to-r from-violet-500 to-primary-500 transition-all duration-300"
                style={{ width: `${progress}%` }}
              />
            </div>
          </div>
        </div>
      )}

      {/* ── Step: Preview ───────────────────────────────────── */}
      {step === 'preview' && optimized && (
        <div className="space-y-5">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2 rounded-full bg-emerald-500/10 px-3 py-1.5">
              <Check className="h-3.5 w-3.5 text-emerald-400" />
              <span className="text-xs font-medium text-emerald-400">AI 优化完成</span>
            </div>
            <button
              onClick={handleReset}
              className="flex items-center gap-1.5 text-xs text-surface-500 hover:text-surface-300"
            >
              <RefreshCw className="h-3.5 w-3.5" />
              重新输入
            </button>
          </div>

          {/* Title */}
          <div className="rounded-2xl border border-white/[0.08] bg-surface-800/60 p-5">
            <label className="mb-2 block text-xs font-medium uppercase tracking-wider text-surface-400">项目标题</label>
            <input
              value={optimizedTitle}
              onChange={(e) => setOptimizedTitle(e.target.value)}
              className="w-full bg-transparent text-lg font-bold text-white outline-none placeholder:text-surface-500 focus:outline-none"
              placeholder="项目标题"
            />
          </div>

          {/* Meta */}
          {(optimized.genre || optimized.tags?.length > 0) && (
            <div className="flex flex-wrap gap-2">
              {optimized.genre && (
                <span className="rounded-full border border-violet-500/30 bg-violet-500/10 px-3 py-1 text-xs font-medium text-violet-300">{optimized.genre}</span>
              )}
              {optimized.tags?.map((tag, i) => (
                <span key={i} className="rounded-full border border-white/[0.08] bg-surface-700/50 px-3 py-1 text-xs text-surface-300">#{tag}</span>
              ))}
            </div>
          )}

          {/* Model config summary */}
          {(selectedTextModel || selectedImageModel || enableVoice) && (
            <div className="flex flex-wrap gap-1.5 rounded-xl border border-white/[0.06] bg-surface-800/40 px-4 py-3">
              <span className="mr-1 text-xs text-surface-500">生成配置：</span>
              {selectedTextModel && (
                <span className="rounded-full border border-blue-500/20 bg-blue-500/10 px-2 py-0.5 text-xs text-blue-300">
                  文本: {textModels.find(m => m.model_key === selectedTextModel)?.name ?? selectedTextModel}
                </span>
              )}
              {selectedImageModel && (
                <span className="rounded-full border border-emerald-500/20 bg-emerald-500/10 px-2 py-0.5 text-xs text-emerald-300">
                  图像: {imageModels.find(m => m.model_key === selectedImageModel)?.name ?? selectedImageModel}
                </span>
              )}
              {enableVoice && (
                <span className="rounded-full border border-violet-500/20 bg-violet-500/10 px-2 py-0.5 text-xs text-violet-300">AI 配音</span>
              )}
            </div>
          )}

          {/* Outline */}
          {optimized.outline?.length > 0 && (
            <div className="rounded-2xl border border-white/[0.08] bg-surface-800/60 p-5">
              <label className="mb-3 flex items-center gap-2 text-xs font-medium uppercase tracking-wider text-surface-400">
                <Video className="h-3.5 w-3.5" />
                故事大纲（{optimized.outline.length} 集）
              </label>
              <ol className="space-y-2">
                {optimized.outline.map((ep, i) => (
                  <li key={i} className="flex items-start gap-3 text-sm">
                    <span className="mt-0.5 flex h-5 w-5 flex-shrink-0 items-center justify-center rounded-full bg-primary-500/20 text-xs font-bold text-primary-400">{i + 1}</span>
                    <span className="text-surface-200">{ep}</span>
                  </li>
                ))}
              </ol>
            </div>
          )}

          {/* Script preview */}
          {optimizedContent && (
            <div className="rounded-2xl border border-white/[0.08] bg-surface-800/60 p-5">
              <label className="mb-3 flex items-center justify-between text-xs font-medium uppercase tracking-wider text-surface-400">
                <span>剧本内容预览</span>
                <span className="normal-case text-surface-500">{optimized.word_count ?? optimizedContent.length} 字</span>
              </label>
              <div className="max-h-48 overflow-y-auto">
                <pre className="whitespace-pre-wrap text-xs leading-relaxed text-surface-300">
                  {optimizedContent.slice(0, 800)}
                  {optimizedContent.length > 800 ? '\n\n…（更多内容将在项目中展开）' : ''}
                </pre>
              </div>
            </div>
          )}

          {error && (
            <div className="flex items-center gap-2 rounded-xl border border-red-500/20 bg-red-500/10 px-4 py-3 text-sm text-red-300">
              <AlertCircle className="h-4 w-4 flex-shrink-0" />
              {error}
            </div>
          )}

          <Button
            onClick={handleCreateProject}
            className="w-full gap-2 bg-gradient-to-r from-violet-600 to-primary-600 py-6 text-base font-semibold hover:from-violet-500 hover:to-primary-500"
          >
            <ArrowRight className="h-5 w-5" />
            创建视频项目并开始生成
          </Button>
        </div>
      )}

      {/* ── Step: Creating ──────────────────────────────────── */}
      {step === 'creating' && (
        <div className="flex flex-col items-center justify-center space-y-6 py-16">
          <div className="relative flex h-20 w-20 items-center justify-center">
            <div className="absolute inset-0 animate-ping rounded-full bg-emerald-500/20" />
            <div className="absolute inset-2 animate-spin rounded-full border-2 border-transparent border-t-emerald-400" />
            <Video className="h-8 w-8 text-emerald-400" />
          </div>
          <div className="text-center">
            <p className="text-lg font-semibold text-white">正在创建视频项目…</p>
            <p className="mt-1 text-sm text-surface-400">即将跳转到项目配置页面</p>
          </div>
          <div className="w-full max-w-xs">
            <div className="h-2 overflow-hidden rounded-full bg-surface-700">
              <div
                className="h-full rounded-full bg-gradient-to-r from-emerald-500 to-primary-500 transition-all duration-300"
                style={{ width: `${progress}%` }}
              />
            </div>
          </div>
        </div>
      )}

      {/* ── Step: Done ──────────────────────────────────────── */}
      {step === 'done' && (
        <div className="flex flex-col items-center justify-center space-y-6 py-16">
          <div className="flex h-20 w-20 items-center justify-center rounded-full bg-emerald-500/20 ring-4 ring-emerald-500/10">
            <Check className="h-10 w-10 text-emerald-400" />
          </div>
          <div className="text-center">
            <p className="text-xl font-bold text-white">项目已准备就绪！</p>
            <p className="mt-1 text-sm text-surface-400">正在跳转到项目配置页面…</p>
          </div>
          <Loader2 className="h-5 w-5 animate-spin text-surface-500" />
        </div>
      )}

      {/* ── Generation History ───────────────────────────────── */}
      <div className="space-y-3 border-t border-white/[0.06] pt-8">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <History className="h-4 w-4 text-surface-500" />
            <h2 className="text-sm font-semibold text-surface-300">生成历史</h2>
            {history.length > 0 && (
              <span className="rounded-full bg-surface-700/60 px-2 py-0.5 text-xs text-surface-500">{history.length}</span>
            )}
          </div>
          {history.some((r) => r.status === 'pending') && (
            <button
              onClick={refreshHistory}
              className="flex items-center gap-1.5 text-xs text-surface-500 hover:text-surface-300"
            >
              <RefreshCw className="h-3 w-3" />
              刷新
            </button>
          )}
        </div>

        {history.length === 0 ? (
          <div className="rounded-2xl border border-white/[0.06] bg-surface-800/30 py-10 text-center">
            <History className="mx-auto mb-2 h-8 w-8 text-surface-600" />
            <p className="text-sm text-surface-500">暂无生成记录</p>
            <p className="mt-1 text-xs text-surface-600">每次 AI 生成任务都会在这里留下记录</p>
          </div>
        ) : (
          <div className="space-y-2">
            {history.map((record) => (
              <HistoryItem
                key={record.id}
                record={record}
                onNavigate={handleNavigateHistory}
                onDelete={handleDeleteHistory}
                onView={handleViewHistory}
              />
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
