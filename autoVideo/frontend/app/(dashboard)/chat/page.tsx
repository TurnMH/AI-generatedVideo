'use client'

import { useEffect, useMemo, useRef, useState } from 'react'
import dynamic from 'next/dynamic'
import useSWR from 'swr'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import {
  Bot,
  Send,
  Loader2,
  Plus,
  Trash2,
  MessageSquare,
  Copy,
  Check,
  Sparkles,
  Paperclip,
  Download,
  FileText,
  X as XIcon,
  AlertCircle,
  RotateCcw,
  Clock,
  Image as ImageIcon,
} from 'lucide-react'
import { chatAPI, modelAPI } from '@/lib/api'
import type { Model } from '@/types'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Badge } from '@/components/ui/badge'
import { useToast } from '@/components/ui/toast'

// ─── Dynamic / utility ────────────────────────────────────────────────────────

const MermaidChart = dynamic(() => import('@/components/chat/MermaidChart'), { ssr: false })

function sanitizeSvg(raw: string): string {
  return raw
    .replace(/<script[\s\S]*?<\/script>/gi, '')
    .replace(/\son\w+\s*=/gi, ' data-removed=')
    .replace(/javascript:/gi, '')
}

// ─── Types ────────────────────────────────────────────────────────────────────

type Role = 'user' | 'assistant' | 'system'

// A single output part for Gemini multimodal responses
type GeminiPart = { type: 'text'; text: string } | { type: 'image'; mime_type: string; data: string }

type ChatMessage = {
  role: Role
  content: string
  isError?: boolean
  retryContent?: string
  // Gemini multimodal parts (present when using 图文模式)
  parts?: GeminiPart[]
}

type AttachedFile = {
  name: string
  content: string
  size: number
  truncated: boolean
}

type Conversation = {
  id: string
  title: string
  messages: ChatMessage[]
  modelId: string
  createdAt: string
}

// ─── File extraction ──────────────────────────────────────────────────────────

const MAX_FILE_CHARS = 15000

const TEXT_EXTS = new Set([
  'txt','md','markdown','json','jsonl','csv','tsv',
  'js','jsx','ts','tsx','mjs','cjs',
  'py','go','java','rs','rb','php','swift','kt','c','cpp','h','hpp',
  'html','htm','css','scss','less','svg',
  'yaml','yml','toml','ini','env','sh','bash','zsh','bat','ps1',
  'sql','xml','log','conf','cfg',
])

async function extractFileContent(file: File): Promise<string> {
  const ext = file.name.split('.').pop()?.toLowerCase() ?? ''

  // Text-based files
  if (TEXT_EXTS.has(ext) || file.type.startsWith('text/')) {
    return new Promise((resolve, reject) => {
      const reader = new FileReader()
      reader.onload = e => resolve(e.target?.result as string ?? '')
      reader.onerror = reject
      reader.readAsText(file, 'UTF-8')
    })
  }

  // PDF
  if (ext === 'pdf' || file.type === 'application/pdf') {
    const pdfjsLib = await import('pdfjs-dist')
    pdfjsLib.GlobalWorkerOptions.workerSrc =
      `https://unpkg.com/pdfjs-dist@${pdfjsLib.version}/build/pdf.worker.min.mjs`
    const buffer = await file.arrayBuffer()
    const pdf = await pdfjsLib.getDocument({ data: buffer }).promise
    const pages: string[] = []
    const limit = Math.min(pdf.numPages, 60)
    for (let i = 1; i <= limit; i++) {
      const page = await pdf.getPage(i)
      const content = await page.getTextContent()
      pages.push(content.items.map((it: any) => it.str ?? '').join(' '))
    }
    return pages.join('\n\n')
  }

  throw new Error(`不支持的文件类型 .${ext}，请上传文本文件或 PDF`)
}

// ─── LocalStorage ─────────────────────────────────────────────────────────────

const STORAGE_KEY = 'ai_chat_conversations_v1'

function loadConversations(): Conversation[] {
  if (typeof window === 'undefined') return []
  try { return JSON.parse(localStorage.getItem(STORAGE_KEY) ?? '[]') } catch { return [] }
}

function saveConversations(list: Conversation[]) {
  if (typeof window === 'undefined') return
  localStorage.setItem(STORAGE_KEY, JSON.stringify(list.slice(0, 30)))
}

function newConversation(modelId: string): Conversation {
  return {
    id: String(Date.now()),
    title: '新对话',
    messages: [],
    modelId,
    createdAt: new Date().toISOString(),
  }
}

// ─── System prompt ───────────────────────────────────────────────────────────

const SYSTEM_PROMPT = `你是一个运行在浏览器端的 AI 助手。

【运行环境】
- 环境：现代 Web 浏览器（支持 HTML5、ES2022+、CSS3）
- 渲染：用户界面支持 Markdown 渲染，包括标题、列表、代码块（带语法高亮）、表格、粗体/斜体、链接等
- 可执行：你可以输出可直接在浏览器控制台运行的 JavaScript 代码
- 可展示：你可以输出 HTML 片段、SVG 图形、CSS 样式表

【输出建议】
- 回答技术问题时，优先给出可运行的代码示例
- 需要展示数据时，使用 Markdown 表格
- 步骤说明使用有序列表，要点使用无序列表
- 代码块请注明语言（\`\`\`javascript、\`\`\`html、\`\`\`css 等）以便语法高亮
- 如果用户要求可视化内容，可输出 HTML+CSS+JS 的完整片段

请用中文回复，保持友好、专业、简洁。`

// ─── Suggested prompts ────────────────────────────────────────────────────────

const SUGGESTIONS = [
  '帮我写一段产品宣传文案，要有感染力',
  '解释一下什么是扩散模型（Diffusion Model）',
  '帮我构思一个赛博朋克风格的科幻短片剧情',
  '给出 5 个适合用于 AI 图片生成的场景描述',
  '用中文写一首关于秋天的现代诗',
]

// ─── Message bubble ───────────────────────────────────────────────────────────

// ─── Shared Markdown components ──────────────────────────────────────────────

const markdownComponents = {
  code({ node, className, children, ...props }: any) {
    const lang = (className ?? '').replace('language-', '').toLowerCase()
    const code = String(children).replace(/\n$/, '')
    if (!className) {
      return (
        <code className="rounded bg-surface-100 px-1 py-0.5 font-mono text-xs text-indigo-700" {...props}>
          {children}
        </code>
      )
    }
    if (lang === 'mermaid') return <MermaidChart code={code} />
    if (lang === 'svg') {
      return (
        <div className="my-2 overflow-x-auto rounded-xl border border-surface-200 bg-white p-4">
          <div dangerouslySetInnerHTML={{ __html: sanitizeSvg(code) }} />
        </div>
      )
    }
    return (
      <pre className="overflow-x-auto rounded-xl bg-surface-900 px-4 py-3 text-xs">
        <code className={`${className} font-mono text-surface-100`} {...props}>{children}</code>
      </pre>
    )
  },
  table({ children }: any) {
    return <div className="overflow-x-auto"><table className="w-full border-collapse text-xs">{children}</table></div>
  },
  th({ children }: any) {
    return <th className="border border-surface-200 bg-surface-50 px-3 py-1.5 text-left font-semibold">{children}</th>
  },
  td({ children }: any) {
    return <td className="border border-surface-200 px-3 py-1.5">{children}</td>
  },
  a({ href, children }: any) {
    return <a href={href} target="_blank" rel="noopener noreferrer" className="text-indigo-600 underline hover:text-indigo-800">{children}</a>
  },
}

// ─── Message bubble ───────────────────────────────────────────────────────────

function MessageBubble({ msg, idx, onRetry }: { msg: ChatMessage; idx: number; onRetry?: () => void }) {
  const [copied, setCopied] = useState(false)

  const handleCopy = () => {
    navigator.clipboard.writeText(msg.content)
    setCopied(true)
    setTimeout(() => setCopied(false), 1500)
  }

  if (msg.role === 'user') {
    return (
      <div className="flex justify-end px-4">
        <div className="max-w-[75%] rounded-2xl rounded-tr-sm bg-indigo-600 px-4 py-3 text-sm leading-6 text-white shadow-sm">
          {msg.content}
        </div>
      </div>
    )
  }

  // Error bubble
  if (msg.isError) {
    return (
      <div className="flex items-start gap-3 px-4">
        <div className="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-red-100">
          <AlertCircle className="h-4 w-4 text-red-500" />
        </div>
        <div className="flex-1 min-w-0">
          <div className="rounded-2xl rounded-tl-sm bg-red-50 px-4 py-3 text-sm leading-6 text-red-700 ring-1 ring-red-200/60 shadow-sm">
            {msg.content}
          </div>
          {onRetry && (
            <button
              onClick={onRetry}
              className="mt-1.5 flex items-center gap-1.5 rounded-full border border-red-200 bg-white px-3 py-1 text-xs text-red-600 transition-colors hover:bg-red-50"
            >
              <RotateCcw className="h-3 w-3" />
              重试
            </button>
          )}
        </div>
      </div>
    )
  }

  return (
    <div className="flex items-start gap-3 px-4">
      <div className="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-gradient-to-br from-indigo-500 to-violet-500 shadow-sm">
        <Bot className="h-4 w-4 text-white" />
      </div>
      <div className="group/bubble flex-1 min-w-0">
        <div className="rounded-2xl rounded-tl-sm bg-white px-4 py-3 text-sm leading-6 text-surface-700 shadow-sm ring-1 ring-surface-200/60 prose prose-sm prose-indigo max-w-none">
          {/* Gemini multimodal parts */}
          {msg.parts && msg.parts.length > 0 ? (
            <div className="flex flex-col gap-3">
              {msg.parts.map((part, i) =>
                part.type === 'image' ? (
                  <img
                    key={i}
                    src={`data:${part.mime_type};base64,${part.data}`}
                    alt="AI 生成图片"
                    className="max-w-full rounded-xl border border-surface-200 shadow-sm"
                  />
                ) : (
                  <ReactMarkdown
                    key={i}
                    remarkPlugins={[remarkGfm]}
                    components={markdownComponents}
                  >
                    {part.text}
                  </ReactMarkdown>
                )
              )}
            </div>
          ) : (
            <ReactMarkdown
              remarkPlugins={[remarkGfm]}
              components={markdownComponents}
            >
              {msg.content}
            </ReactMarkdown>
          )}
        </div>
        <button
          onClick={handleCopy}
          className="mt-1.5 flex items-center gap-1 rounded-full border border-surface-200 bg-white px-2.5 py-0.5 text-[11px] text-surface-400 opacity-0 transition-all group-hover/bubble:opacity-100 hover:border-indigo-200 hover:text-indigo-600"
        >
          {copied ? <Check className="h-3 w-3 text-green-500" /> : <Copy className="h-3 w-3" />}
          {copied ? '已复制' : '复制'}
        </button>
      </div>
    </div>
  )
}

// ─── Main page ────────────────────────────────────────────────────────────────

export default function ChatPage() {
  const { toast } = useToast()
  const bottomRef = useRef<HTMLDivElement>(null)
  const textareaRef = useRef<HTMLTextAreaElement>(null)

  const [conversations, setConversations] = useState<Conversation[]>([])
  const [activeId, setActiveId] = useState<string | null>(null)
  const [input, setInput] = useState('')
  const [sending, setSending] = useState(false)
  const [sendingSeconds, setSendingSeconds] = useState(0)
  const [selectedModelId, setSelectedModelId] = useState('')
  const [attachedFiles, setAttachedFiles] = useState<AttachedFile[]>([])
  const [fileLoading, setFileLoading] = useState(false)
  const [geminiMode, setGeminiMode] = useState(false) // 图文模式：调用 Gemini multimodal API
  const fileInputRef = useRef<HTMLInputElement>(null)

  // Elapsed-time counter while waiting for AI reply
  useEffect(() => {
    if (!sending) { setSendingSeconds(0); return }
    const interval = setInterval(() => setSendingSeconds(s => s + 1), 1000)
    return () => clearInterval(interval)
  }, [sending])

  // Load from localStorage
  useEffect(() => {
    const saved = loadConversations()
    setConversations(saved)
    if (saved.length > 0) setActiveId(saved[0].id)
  }, [])

  // Models
  const { data: modelsRaw } = useSWR(
    ['chat-models'],
    () => modelAPI.list({ type: 'llm', sort_by: 'priority' }) as unknown as Promise<{ data?: Model[] }>
  )
  const models = useMemo<Model[]>(
    () => ((modelsRaw as any)?.data ?? []).filter((m: Model) => m.is_active && m.model_key),
    [modelsRaw]
  )

  useEffect(() => {
    if (!selectedModelId && models.length > 0) {
      const def = models.find(m => m.is_default) ?? models[0]
      setSelectedModelId(String(def.id))
    }
  }, [models, selectedModelId])

  // Sync Select display with the active conversation's stored model whenever the active
  // conversation or the model list changes (e.g. page load, sidebar click).
  useEffect(() => {
    if (!activeId || models.length === 0) return
    const conv = conversations.find(c => c.id === activeId)
    if (conv?.modelId && models.find(m => String(m.id) === conv.modelId)) {
      setSelectedModelId(conv.modelId)
    }
  }, [activeId, models])  // eslint-disable-line react-hooks/exhaustive-deps

  const activeConv = conversations.find(c => c.id === activeId) ?? null

  const updateConv = (id: string, updater: (c: Conversation) => Conversation) => {
    setConversations(prev => {
      const next = prev.map(c => c.id === id ? updater(c) : c)
      saveConversations(next)
      return next
    })
  }

  // Auto-scroll
  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [activeConv?.messages.length, sending])

  const handleNewChat = () => {
    const conv = newConversation(selectedModelId)
    const next = [conv, ...conversations]
    setConversations(next)
    saveConversations(next)
    setActiveId(conv.id)
    setInput('')
    textareaRef.current?.focus()
  }

  const handleDeleteConv = (id: string, e: React.MouseEvent) => {
    e.stopPropagation()
    const next = conversations.filter(c => c.id !== id)
    setConversations(next)
    saveConversations(next)
    if (activeId === id) setActiveId(next[0]?.id ?? null)
  }

  const handleFileSelect = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = Array.from(e.target.files ?? [])
    if (!files.length) return
    e.target.value = ''
    setFileLoading(true)
    const results: AttachedFile[] = []
    for (const file of files) {
      try {
        let raw = await extractFileContent(file)
        const truncated = raw.length > MAX_FILE_CHARS
        if (truncated) raw = raw.slice(0, MAX_FILE_CHARS)
        results.push({ name: file.name, content: raw, size: file.size, truncated })
      } catch (err: any) {
        toast({ title: `读取失败: ${file.name}`, description: err?.message, variant: 'destructive' })
      }
    }
    setAttachedFiles(prev => [...prev, ...results])
    setFileLoading(false)
  }

  const handleExport = () => {
    if (!activeConv || activeConv.messages.filter(m => m.role !== 'system').length === 0) return
    const lines = [
      `# ${activeConv.title}`,
      `> 导出时间：${new Date().toLocaleString('zh-CN')}`,
      '',
    ]
    for (const m of activeConv.messages) {
      if (m.role === 'system') continue
      lines.push(`## ${m.role === 'user' ? '🧑 用户' : '🤖 AI 助手'}`, '', m.content, '', '---', '')
    }
    const blob = new Blob([lines.join('\n')], { type: 'text/markdown;charset=utf-8' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `${activeConv.title.replace(/[/\\?%*:|"<>]/g, '-')}.md`
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    URL.revokeObjectURL(url)
  }

  const handleSend = async (text?: string) => {
    const content = (text ?? input).trim()
    if (!content || sending) return

    // Create a new conversation if none is active
    let convId = activeId
    if (!convId) {
      const conv = newConversation(selectedModelId)
      const next = [conv, ...conversations]
      setConversations(next)
      saveConversations(next)
      setActiveId(conv.id)
      convId = conv.id
    }

    // Build user message content — prepend file context if any
    const fileContext = attachedFiles.map(f =>
      `📄 **文件: ${f.name}**${f.truncated ? `（内容已截断至 ${MAX_FILE_CHARS} 字）` : ''}\n\`\`\`\n${f.content}\n\`\`\``
    ).join('\n\n')

    const fullContent = fileContext ? `${fileContext}\n\n---\n\n${content}` : content
    // Display message: show file names + user text (not raw content)
    const displayContent = attachedFiles.length
      ? `[📎 ${attachedFiles.map(f => f.name).join(', ')}]\n${content}`
      : content

    const userMsg: ChatMessage = { role: 'user', content: displayContent }
    setInput('')
    setAttachedFiles([])
    setSending(true)

    // Optimistically append user message
    updateConv(convId, c => {
      const msgs = [...c.messages, userMsg]
      return {
        ...c,
        messages: msgs,
        title: c.title === '新对话' ? content.slice(0, 30) : c.title,
      }
    })

    try {
      const currentConv = conversations.find(c => c.id === convId)
      // selectedModelId is always in sync with what the user sees in the Select —
      // use it as the authoritative source to avoid stale localStorage model IDs.
      const modelName = models.find(m => String(m.id) === selectedModelId)?.model_key
      // History uses display messages; for current turn, send full content with file context
      const history = [...(currentConv?.messages ?? [])]
      const apiMessages = [
        { role: 'system', content: SYSTEM_PROMPT },
        ...history.map(m => ({ role: m.role, content: m.content })),
        { role: 'user', content: fullContent },
      ]

      if (geminiMode) {
        const gRes = await chatAPI.sendGemini(apiMessages) as unknown as { data?: any }
        const parts: GeminiPart[] = (gRes.data as any)?.data?.parts ?? (gRes.data as any)?.parts ?? []
        const textFallback = parts.filter(p => p.type === 'text').map(p => (p as any).text).join('\n') || '（AI 未返回内容）'
        updateConv(convId, c => ({
          ...c,
          messages: [...c.messages, { role: 'assistant', content: textFallback, parts }],
        }))
      } else {
        const res = await chatAPI.send(apiMessages, modelName) as unknown as { data?: { data?: { reply: string }; reply?: string } }
        const reply = (res.data as any)?.data?.reply ?? (res.data as any)?.reply ?? '（AI 未返回内容）'
        updateConv(convId, c => ({
          ...c,
          messages: [...c.messages, { role: 'assistant', content: reply }],
        }))
      }
    } catch (err: any) {
      const status = err?.response?.status
      const isTimeout = status === 504 || (err?.message ?? '').toLowerCase().includes('timeout')
      const errText = isTimeout
        ? `⏱ 模型响应超时（已等待 ${sendingSeconds}s），请检查模型服务是否正常，然后点击「重试」。`
        : `请求失败：${err?.response?.data?.message ?? err?.message ?? '服务器错误'}`
      updateConv(convId!, c => ({
        ...c,
        messages: [...c.messages, { role: 'assistant', content: errText, isError: true, retryContent: fullContent }],
      }))
    } finally {
      setSending(false)
      textareaRef.current?.focus()
    }
  }

  /** Re-send when AI failed — removes the error bubble and calls the API again */
  const handleRetry = async (convId: string, errorMsg: ChatMessage) => {
    if (!errorMsg.retryContent || sending) return

    const currentConv = conversations.find(c => c.id === convId)
    const historyWithoutError = (currentConv?.messages ?? []).filter(m => m !== errorMsg)
    updateConv(convId, c => ({ ...c, messages: historyWithoutError }))

    setSending(true)
    try {
      const modelName = models.find(m => String(m.id) === selectedModelId)?.model_key
      const apiMessages = [
        { role: 'system', content: SYSTEM_PROMPT },
        ...historyWithoutError.map(m => ({ role: m.role, content: m.content })),
        { role: 'user', content: errorMsg.retryContent! },
      ]
      const res = await chatAPI.send(apiMessages, modelName) as unknown as { data?: any }
      const reply = (res.data as any)?.data?.reply ?? (res.data as any)?.reply ?? '（AI 未返回内容）'
      updateConv(convId, c => ({
        ...c,
        messages: [...c.messages, { role: 'assistant', content: reply }],
      }))
    } catch (err: any) {
      const status = err?.response?.status
      const isTimeout = status === 504 || (err?.message ?? '').toLowerCase().includes('timeout')
      const errText = isTimeout
        ? `⏱ 模型响应超时（已等待 ${sendingSeconds}s），请稍后重试。`
        : `请求失败：${err?.response?.data?.message ?? err?.message ?? '服务器错误'}`
      updateConv(convId, c => ({
        ...c,
        messages: [...c.messages, { role: 'assistant', content: errText, isError: true, retryContent: errorMsg.retryContent }],
      }))
    } finally {
      setSending(false)
      textareaRef.current?.focus()
    }
  }

  return (
    <div className="flex h-[calc(100vh-96px)] gap-0 overflow-hidden rounded-2xl border border-surface-200 bg-white shadow-sm">

      {/* ── Left sidebar: conversation list ── */}
      <div className="flex w-64 shrink-0 flex-col border-r border-surface-100 bg-surface-50/60">
        {/* Header */}
        <div className="flex shrink-0 items-center justify-between border-b border-surface-100 px-4 py-3.5">
          <div className="flex items-center gap-2">
            <MessageSquare className="h-4 w-4 text-indigo-500" />
            <span className="text-sm font-semibold text-surface-800">对话</span>
            {conversations.length > 0 && (
              <Badge variant="secondary" className="h-4 min-w-[1.25rem] px-1.5 text-[10px]">
                {conversations.length}
              </Badge>
            )}
          </div>
          <Button size="sm" variant="ghost" className="h-7 w-7 p-0" onClick={handleNewChat} title="新建对话">
            <Plus className="h-4 w-4" />
          </Button>
        </div>

        {/* Conversation list */}
        <div className="flex-1 overflow-y-auto p-2 space-y-0.5">
          {conversations.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-16 text-center">
              <MessageSquare className="mx-auto h-8 w-8 text-surface-300" />
              <p className="mt-3 text-sm text-surface-400">暂无对话</p>
              <p className="text-xs text-surface-300">点击右上角「+」新建</p>
            </div>
          ) : conversations.map(conv => (
            <button
              key={conv.id}
              onClick={() => setActiveId(conv.id)}
              className={`group/item w-full rounded-xl px-3 py-2.5 text-left transition-colors ${
                conv.id === activeId
                  ? 'bg-indigo-50 text-indigo-700'
                  : 'text-surface-600 hover:bg-surface-100'
              }`}
            >
              <div className="flex items-start justify-between gap-1">
                <p className="flex-1 truncate text-sm font-medium leading-tight">{conv.title}</p>
                <button
                  onClick={e => handleDeleteConv(conv.id, e)}
                  className="shrink-0 rounded p-0.5 text-surface-300 opacity-0 transition-opacity group-hover/item:opacity-100 hover:text-red-400"
                >
                  <Trash2 className="h-3.5 w-3.5" />
                </button>
              </div>
              <p className="mt-0.5 text-[11px] text-surface-400">
                {conv.messages.length} 条消息
              </p>
            </button>
          ))}
        </div>
      </div>

      {/* ── Right: chat area ── */}
      <div className="flex flex-1 flex-col overflow-hidden">

        {/* Top bar */}
        <div className="flex shrink-0 items-center justify-between border-b border-surface-100 px-5 py-3">
          <div className="flex items-center gap-2.5">
            <div className="flex h-8 w-8 items-center justify-center rounded-full bg-gradient-to-br from-indigo-500 to-violet-500">
              <Bot className="h-4 w-4 text-white" />
            </div>
            <div>
              <p className="text-sm font-semibold text-surface-800">AI 助手</p>
              <p className="text-[11px] text-surface-400">
                {activeConv ? `${activeConv.messages.length} 条对话` : '随时可以开始聊天'}
              </p>
            </div>
          </div>
          <div className="flex items-center gap-2">
            <Select value={selectedModelId} onValueChange={id => {
              setSelectedModelId(id)
              if (activeId) updateConv(activeId, c => ({ ...c, modelId: id }))
            }}>
              <SelectTrigger className="h-8 w-44 text-xs">
                <SelectValue placeholder="选择模型" />
              </SelectTrigger>
              <SelectContent>
                {models.map(m => (
                  <SelectItem key={m.id} value={String(m.id)}>
                    <span className="flex items-center gap-1.5">
                      {m.name}
                      {m.is_default && <span className="text-[10px] text-indigo-500">默认</span>}
                    </span>
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Button size="sm" variant="outline" onClick={handleNewChat} className="h-8 gap-1.5 text-xs">
              <Plus className="h-3.5 w-3.5" />
              新建
            </Button>
            <Button
              size="sm" variant="outline"
              onClick={handleExport}
              disabled={!activeConv || activeConv.messages.filter(m => m.role !== 'system').length === 0}
              className="h-8 gap-1.5 text-xs"
              title="导出对话为 Markdown"
            >
              <Download className="h-3.5 w-3.5" />
              导出
            </Button>
          </div>
        </div>

        {/* Messages */}
        <div className="flex-1 space-y-5 overflow-y-auto bg-surface-50/40 py-6">
          {!activeConv || activeConv.messages.length === 0 ? (
            <div className="flex h-full flex-col items-center justify-center px-8 text-center">
              <div className="flex h-16 w-16 items-center justify-center rounded-2xl bg-gradient-to-br from-indigo-500 to-violet-500 shadow-lg">
                <Sparkles className="h-8 w-8 text-white" />
              </div>
              <h2 className="mt-5 text-xl font-semibold text-surface-800">AI 助手</h2>
              <p className="mt-2 max-w-sm text-sm text-surface-400">
                我可以帮你写作、分析、创意策划、解答问题，或者聊任何感兴趣的话题。
              </p>
              <div className="mt-8 grid w-full max-w-lg grid-cols-1 gap-2">
                {SUGGESTIONS.map(s => (
                  <button
                    key={s}
                    onClick={() => handleSend(s)}
                    className="rounded-xl border border-surface-200 bg-white px-4 py-2.5 text-left text-sm text-surface-600 transition-colors hover:border-indigo-200 hover:bg-indigo-50 hover:text-indigo-700"
                  >
                    {s}
                  </button>
                ))}
              </div>
            </div>
          ) : (
            activeConv.messages.map((msg, i) => (
              <MessageBubble
                key={i}
                msg={msg}
                idx={i}
                onRetry={
                  msg.isError && msg.retryContent && activeId
                    ? () => handleRetry(activeId, msg)
                    : undefined
                }
              />
            ))
          )}
          {sending && (
            <div className="flex items-start gap-3 px-4">
              <div className="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-gradient-to-br from-indigo-500 to-violet-500 shadow-sm">
                <Bot className="h-4 w-4 text-white" />
              </div>
              <div className="rounded-2xl rounded-tl-sm bg-white px-4 py-3 shadow-sm ring-1 ring-surface-200/60">
                <div className="flex items-center gap-2">
                  <div className="flex items-center gap-1.5">
                    <span className="h-2 w-2 animate-bounce rounded-full bg-indigo-400 [animation-delay:0ms]" />
                    <span className="h-2 w-2 animate-bounce rounded-full bg-indigo-400 [animation-delay:150ms]" />
                    <span className="h-2 w-2 animate-bounce rounded-full bg-indigo-400 [animation-delay:300ms]" />
                  </div>
                  {sendingSeconds >= 5 && (
                    <span className="flex items-center gap-1 text-xs text-surface-400">
                      <Clock className="h-3 w-3" />
                      {sendingSeconds}s
                    </span>
                  )}
                </div>
              </div>
            </div>
          )}
          <div ref={bottomRef} />
        </div>

        {/* Input bar */}
        <div className="shrink-0 border-t border-surface-100 bg-white p-4">
          {/* Hidden file input */}
          <input
            ref={fileInputRef}
            type="file"
            multiple
            accept=".txt,.md,.json,.jsonl,.csv,.tsv,.js,.jsx,.ts,.tsx,.py,.go,.java,.rs,.rb,.php,.swift,.html,.css,.yaml,.yml,.toml,.sql,.xml,.log,.sh,.pdf"
            className="hidden"
            onChange={handleFileSelect}
          />

          {/* Attached file chips */}
          {attachedFiles.length > 0 && (
            <div className="mb-2 flex flex-wrap gap-1.5">
              {attachedFiles.map((f, i) => (
                <div key={i} className="flex items-center gap-1 rounded-lg border border-indigo-200 bg-indigo-50 px-2.5 py-1 text-xs text-indigo-700">
                  <FileText className="h-3 w-3 shrink-0" />
                  <span className="max-w-[140px] truncate">{f.name}</span>
                  {f.truncated && <span className="text-[10px] text-indigo-400">(截断)</span>}
                  <button
                    onClick={() => setAttachedFiles(prev => prev.filter((_, j) => j !== i))}
                    className="ml-0.5 rounded p-0.5 hover:bg-indigo-100"
                  >
                    <XIcon className="h-2.5 w-2.5" />
                  </button>
                </div>
              ))}
            </div>
          )}

          <div className="flex items-end gap-2 rounded-2xl border border-surface-200 bg-surface-50 px-3 py-3 focus-within:border-indigo-300 focus-within:bg-white transition-colors">
            {/* Paperclip button */}
            <button
              type="button"
              onClick={() => fileInputRef.current?.click()}
              disabled={fileLoading}
              className="mb-0.5 flex h-7 w-7 shrink-0 items-center justify-center rounded-lg text-surface-400 transition-colors hover:bg-surface-100 hover:text-indigo-500 disabled:opacity-40"
              title="上传文件（文本 / PDF）"
            >
              {fileLoading ? <Loader2 className="h-4 w-4 animate-spin" /> : <Paperclip className="h-4 w-4" />}
            </button>

            <Textarea
              ref={textareaRef}
              value={input}
              onChange={e => setInput(e.target.value)}
              rows={1}
              placeholder={attachedFiles.length ? '描述你想对这些文件做什么…' : '输入消息… (Enter 发送，Shift+Enter 换行)'}
              className="flex-1 resize-none border-0 bg-transparent p-0 text-sm shadow-none focus-visible:ring-0"
              style={{ maxHeight: 160, overflowY: 'auto' }}
              onKeyDown={e => {
                if (e.key === 'Enter' && !e.shiftKey) {
                  e.preventDefault()
                  handleSend()
                }
              }}
              onInput={e => {
                const el = e.currentTarget
                el.style.height = 'auto'
                el.style.height = Math.min(el.scrollHeight, 160) + 'px'
              }}
            />
            <Button
              size="sm"
              variant={geminiMode ? 'default' : 'outline'}
              onClick={() => setGeminiMode(v => !v)}
              title={geminiMode ? '当前：图文模式（Gemini）' : '切换到图文模式'}
              className="h-9 w-9 shrink-0 rounded-xl p-0"
            >
              <ImageIcon className="h-4 w-4" />
            </Button>
            <Button
              size="sm"
              onClick={() => handleSend()}
              disabled={sending || (!input.trim() && attachedFiles.length === 0)}
              className="h-9 w-9 shrink-0 rounded-xl p-0"
            >
              {sending ? <Loader2 className="h-4 w-4 animate-spin" /> : <Send className="h-4 w-4" />}
            </Button>
          </div>
          <p className="mt-2 text-center text-[11px] text-surface-300">
            支持上传文本文件 / PDF · 导出为 Markdown · AI 内容仅供参考
          </p>
        </div>
      </div>
    </div>
  )
}
