'use client'

import { useEffect, useMemo, useRef, useState } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { Eye, Upload, X, FileText, AlertTriangle } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'

// ─── Types ────────────────────────────────────────────────────────────────────

type DocType = 'pdf' | 'markdown' | 'csv' | 'tsv' | 'json' | 'html' | 'docx' | 'xlsx' | 'image' | 'code' | 'unknown'

type DocState = {
  file: File
  type: DocType
  text?: string
  objectUrl?: string
}

// ─── Constants ────────────────────────────────────────────────────────────────

const MAX_TEXT_SIZE = 5 * 1024 * 1024 // 5MB

const TYPE_LABELS: Record<DocType, string> = {
  pdf: 'PDF', markdown: 'Markdown', csv: 'CSV', tsv: 'TSV',
  json: 'JSON', html: 'HTML', docx: 'Word', xlsx: 'Excel',
  image: '图片', code: '代码/文本', unknown: '未知',
}

const TYPE_COLORS: Record<DocType, string> = {
  pdf: 'bg-red-50 text-red-700 border-red-200',
  markdown: 'bg-blue-50 text-blue-700 border-blue-200',
  csv: 'bg-green-50 text-green-700 border-green-200',
  tsv: 'bg-green-50 text-green-700 border-green-200',
  json: 'bg-amber-50 text-amber-700 border-amber-200',
  html: 'bg-orange-50 text-orange-700 border-orange-200',
  docx: 'bg-sky-50 text-sky-700 border-sky-200',
  xlsx: 'bg-emerald-50 text-emerald-700 border-emerald-200',
  image: 'bg-pink-50 text-pink-700 border-pink-200',
  code: 'bg-purple-50 text-purple-700 border-purple-200',
  unknown: 'bg-surface-100 text-surface-600 border-surface-200',
}

const ACCEPT = [
  '.pdf', '.md', '.mdx', '.markdown', '.csv', '.tsv', '.json',
  '.html', '.htm', '.docx', '.doc', '.xlsx', '.xls', '.ods', '.txt', '.log',
  '.js', '.ts', '.tsx', '.jsx', '.mjs', '.cjs',
  '.py', '.go', '.rs', '.java', '.c', '.cpp', '.h', '.hpp', '.cs', '.php', '.rb', '.swift', '.kt',
  '.css', '.scss', '.less', '.sql',
  '.sh', '.bash', '.zsh', '.fish',
  '.yaml', '.yml', '.toml', '.ini', '.conf', '.env',
  '.xml', '.svg', '.vue', '.svelte', '.graphql', '.proto',
  '.diff', '.patch', '.makefile', '.dockerfile', '.gitignore',
  'image/*',
].join(',')

// ─── Helpers ──────────────────────────────────────────────────────────────────

function detectType(file: File): DocType {
  const ext = file.name.split('.').pop()?.toLowerCase() ?? ''
  const mime = file.type.toLowerCase()
  if (mime === 'application/pdf' || ext === 'pdf') return 'pdf'
  if (ext === 'md' || ext === 'mdx' || ext === 'markdown') return 'markdown'
  if (ext === 'csv' || mime === 'text/csv') return 'csv'
  if (ext === 'tsv') return 'tsv'
  if (ext === 'json' || mime === 'application/json') return 'json'
  if (ext === 'html' || ext === 'htm') return 'html'
  if (mime.startsWith('image/')) return 'image'
  if (ext === 'docx' || ext === 'doc') return 'docx'
  if (ext === 'xlsx' || ext === 'xls' || ext === 'ods') return 'xlsx'
  const codeExts = [
    'txt', 'log', 'js', 'ts', 'tsx', 'jsx', 'mjs', 'cjs',
    'py', 'go', 'rs', 'java', 'c', 'cpp', 'h', 'hpp', 'cs', 'php', 'rb', 'swift', 'kt',
    'css', 'scss', 'less', 'sql', 'sh', 'bash', 'zsh', 'fish',
    'yaml', 'yml', 'toml', 'ini', 'conf', 'env',
    'xml', 'svg', 'vue', 'svelte', 'graphql', 'proto',
    'diff', 'patch', 'makefile', 'dockerfile', 'gitignore',
  ]
  if (codeExts.includes(ext) || mime.startsWith('text/')) return 'code'
  return 'unknown'
}

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / 1024 / 1024).toFixed(2)} MB`
}

function extToLang(filename: string): string {
  const ext = filename.split('.').pop()?.toLowerCase() ?? ''
  const map: Record<string, string> = {
    js: 'JavaScript', ts: 'TypeScript', tsx: 'TSX', jsx: 'JSX', mjs: 'ES Module', cjs: 'CommonJS',
    py: 'Python', go: 'Go', rs: 'Rust', java: 'Java', c: 'C', cpp: 'C++',
    h: 'C Header', hpp: 'C++ Header', cs: 'C#', php: 'PHP', rb: 'Ruby', swift: 'Swift', kt: 'Kotlin',
    css: 'CSS', scss: 'SCSS', less: 'Less', sql: 'SQL',
    sh: 'Shell', bash: 'Bash', zsh: 'Zsh', fish: 'Fish',
    yaml: 'YAML', yml: 'YAML', toml: 'TOML', ini: 'INI', conf: '配置文件', env: 'ENV',
    xml: 'XML', svg: 'SVG', vue: 'Vue', svelte: 'Svelte', graphql: 'GraphQL', proto: 'Protobuf',
    txt: '纯文本', log: '日志', md: 'Markdown', diff: 'Diff', patch: 'Patch',
    makefile: 'Makefile', dockerfile: 'Dockerfile', gitignore: 'Gitignore',
  }
  return map[ext] ?? (ext ? ext.toUpperCase() : '文本')
}

// ─── CSV parser ───────────────────────────────────────────────────────────────

function parseCsv(text: string, sep = ','): string[][] {
  const rows: string[][] = []
  for (const line of text.split(/\r?\n/)) {
    if (line.trim() === '') continue
    const cells: string[] = []
    let cur = '', inQuote = false
    for (let i = 0; i < line.length; i++) {
      const ch = line[i]
      if (ch === '"') {
        if (inQuote && line[i + 1] === '"') { cur += '"'; i++ }
        else inQuote = !inQuote
      } else if (ch === sep && !inQuote) { cells.push(cur); cur = '' }
      else cur += ch
    }
    cells.push(cur)
    rows.push(cells)
  }
  return rows
}

// ─── JSON highlighter ─────────────────────────────────────────────────────────

function highlightJson(raw: string): string {
  const esc = raw.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
  return esc.replace(
    /("(\\u[a-fA-F0-9]{4}|\\[^u]|[^\\"])*"(\s*:)?|\b(true|false|null)\b|-?\d+(?:\.\d*)?(?:[eE][+-]?\d+)?)/g,
    (m) => {
      if (/^"/.test(m)) {
        if (/:$/.test(m)) return `<span class="text-sky-300">${m}</span>` // key
        return `<span class="text-emerald-300">${m}</span>` // string
      }
      if (/true|false/.test(m)) return `<span class="text-violet-400">${m}</span>`
      if (/null/.test(m)) return `<span class="text-rose-400">${m}</span>`
      return `<span class="text-amber-300">${m}</span>` // number
    },
  )
}

// ─── Renderers ────────────────────────────────────────────────────────────────

function PdfRenderer({ url }: { url: string }) {
  return (
    <iframe
      src={url}
      className="w-full rounded-xl border border-surface-200 shadow-sm"
      style={{ height: 'calc(100vh - 260px)', minHeight: '500px' }}
      title="PDF 预览"
    />
  )
}

function MarkdownRenderer({ text }: { text: string }) {
  return (
    <div className="rounded-xl border border-surface-200 bg-white p-6 shadow-sm overflow-auto" style={{ maxHeight: 'calc(100vh - 260px)' }}>
      <div className="prose prose-sm max-w-none">
        <ReactMarkdown remarkPlugins={[remarkGfm]}>{text}</ReactMarkdown>
      </div>
    </div>
  )
}

function CsvRenderer({ text, sep = ',' }: { text: string; sep?: string }) {
  const [page, setPage] = useState(0)
  const PAGE_SIZE = 100
  const rows = useMemo(() => parseCsv(text, sep), [text, sep])
  const header = rows[0] ?? []
  const dataRows = rows.slice(1)
  const totalPages = Math.max(1, Math.ceil(dataRows.length / PAGE_SIZE))
  const visible = dataRows.slice(page * PAGE_SIZE, (page + 1) * PAGE_SIZE)

  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between rounded-lg bg-surface-50 border border-surface-200 px-3 py-1.5 text-xs text-surface-500">
        <span>共 {dataRows.length} 行 · {header.length} 列 · 每页 {PAGE_SIZE} 行</span>
        {totalPages > 1 && (
          <div className="flex items-center gap-1">
            <button onClick={() => setPage(p => Math.max(0, p - 1))} disabled={page === 0}
              className="rounded border border-surface-200 bg-white px-2 py-0.5 disabled:opacity-30 hover:border-indigo-300 transition-colors">‹</button>
            <span className="px-1">{page + 1} / {totalPages}</span>
            <button onClick={() => setPage(p => Math.min(totalPages - 1, p + 1))} disabled={page >= totalPages - 1}
              className="rounded border border-surface-200 bg-white px-2 py-0.5 disabled:opacity-30 hover:border-indigo-300 transition-colors">›</button>
          </div>
        )}
      </div>
      <div className="overflow-auto rounded-xl border border-surface-200 shadow-sm" style={{ maxHeight: 'calc(100vh - 310px)' }}>
        <table className="w-full text-sm border-collapse">
          <thead className="sticky top-0 z-10">
            <tr className="bg-surface-100">
              <th className="border-b border-surface-200 px-3 py-2 text-right text-xs text-surface-400 font-normal w-12 select-none">#</th>
              {header.map((h, i) => (
                <th key={i} className="border-b border-surface-200 px-3 py-2 text-left text-xs font-semibold text-surface-700 whitespace-nowrap">
                  {h || <span className="text-surface-400">列{i + 1}</span>}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {visible.map((row, ri) => (
              <tr key={ri} className={`hover:bg-indigo-50/30 transition-colors ${ri % 2 === 0 ? 'bg-white' : 'bg-surface-50/60'}`}>
                <td className="border-b border-surface-100 px-3 py-1.5 text-right text-xs text-surface-400 select-none">
                  {page * PAGE_SIZE + ri + 1}
                </td>
                {header.map((_, ci) => (
                  <td key={ci} className="border-b border-surface-100 px-3 py-1.5 text-xs font-mono text-surface-700 max-w-xs truncate" title={row[ci] ?? ''}>
                    {row[ci] ?? ''}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}

function JsonRenderer({ text }: { text: string }) {
  const { pretty, error } = useMemo(() => {
    try {
      const obj = JSON.parse(text)
      const stats = (() => {
        const keys = JSON.stringify(obj).match(/"[^"]+"\s*:/g) ?? []
        return `${Array.isArray(obj) ? `数组 ${obj.length} 项` : `对象 ${Object.keys(obj).length} 个顶层键`}`
      })()
      return { pretty: JSON.stringify(obj, null, 2), stats, error: '' }
    } catch (e: any) {
      return { pretty: text, stats: '', error: e.message }
    }
  }, [text])

  return (
    <div className="space-y-2">
      {error && (
        <div className="flex items-start gap-2 rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700">
          <AlertTriangle className="h-4 w-4 mt-0.5 flex-shrink-0" />
          <span>JSON 解析失败：{error}（以原文显示）</span>
        </div>
      )}
      <pre
        className="overflow-auto rounded-xl border border-gray-700 bg-gray-950 p-5 text-sm font-mono leading-[1.65] shadow-sm"
        style={{ maxHeight: 'calc(100vh - 310px)' }}
        dangerouslySetInnerHTML={{ __html: highlightJson(pretty) }}
      />
    </div>
  )
}

function HtmlRenderer({ text, filename, isDocx }: { text: string; filename: string; isDocx?: boolean }) {
  const url = useMemo(() => URL.createObjectURL(new Blob([text], { type: 'text/html' })), [text])
  useEffect(() => () => URL.revokeObjectURL(url), [url])

  return (
    <div className="space-y-2">
      <div className="flex items-center gap-2 rounded-lg bg-amber-50 border border-amber-200 px-3 py-2 text-xs text-amber-700">
        <AlertTriangle className="h-3.5 w-3.5 flex-shrink-0" />
        {isDocx
          ? 'Word 文档已转换为 HTML 预览（mammoth.js），格式仅供参考，不完全还原原始排版'
          : '沙盒模式（sandbox），已禁止脚本执行、弹窗和外部请求'}
      </div>
      <iframe
        src={url}
        sandbox="allow-same-origin allow-forms"
        className="w-full rounded-xl border border-surface-200 bg-white shadow-sm"
        style={{ height: 'calc(100vh - 320px)', minHeight: '400px' }}
        title={filename}
      />
    </div>
  )
}

function XlsxRenderer({ text }: { text: string }) {
  const { sheetNames, sheets } = useMemo<{ sheetNames: string[]; sheets: Record<string, string[][]> }>(() => JSON.parse(text), [text])
  const [activeSheet, setActiveSheet] = useState(() => sheetNames[0] ?? '')
  const [page, setPage] = useState(0)
  const PAGE_SIZE = 100

  const rows: string[][] = sheets[activeSheet] ?? []
  const header = rows[0] ?? []
  const dataRows = rows.slice(1)
  const totalPages = Math.max(1, Math.ceil(dataRows.length / PAGE_SIZE))
  const visible = dataRows.slice(page * PAGE_SIZE, (page + 1) * PAGE_SIZE)

  function switchSheet(name: string) { setActiveSheet(name); setPage(0) }

  return (
    <div className="space-y-2">
      {/* Sheet tabs */}
      {sheetNames.length > 1 && (
        <div className="flex gap-1 overflow-x-auto pb-1">
          {sheetNames.map((name) => (
            <button
              key={name}
              onClick={() => switchSheet(name)}
              className={`flex-shrink-0 rounded-lg border px-3 py-1 text-xs font-medium transition-colors
                ${activeSheet === name ? 'border-emerald-400 bg-emerald-50 text-emerald-700' : 'border-surface-200 bg-white text-surface-600 hover:border-emerald-300'}`}
            >
              {name}
            </button>
          ))}
        </div>
      )}
      {/* Row/page info */}
      <div className="flex items-center justify-between rounded-lg bg-surface-50 border border-surface-200 px-3 py-1.5 text-xs text-surface-500">
        <span>共 {dataRows.length} 行 · {header.length} 列 · 工作表：{activeSheet}</span>
        {totalPages > 1 && (
          <div className="flex items-center gap-1">
            <button onClick={() => setPage(p => Math.max(0, p - 1))} disabled={page === 0}
              className="rounded border border-surface-200 bg-white px-2 py-0.5 disabled:opacity-30 hover:border-emerald-300 transition-colors">‹</button>
            <span className="px-1">{page + 1} / {totalPages}</span>
            <button onClick={() => setPage(p => Math.min(totalPages - 1, p + 1))} disabled={page >= totalPages - 1}
              className="rounded border border-surface-200 bg-white px-2 py-0.5 disabled:opacity-30 hover:border-emerald-300 transition-colors">›</button>
          </div>
        )}
      </div>
      {/* Table */}
      <div className="overflow-auto rounded-xl border border-surface-200 shadow-sm" style={{ maxHeight: 'calc(100vh - 340px)' }}>
        <table className="w-full text-sm border-collapse">
          <thead className="sticky top-0 z-10">
            <tr className="bg-emerald-50">
              <th className="border-b border-surface-200 px-3 py-2 text-right text-xs text-surface-400 font-normal w-12 select-none">#</th>
              {header.map((h, i) => (
                <th key={i} className="border-b border-surface-200 px-3 py-2 text-left text-xs font-semibold text-surface-700 whitespace-nowrap">
                  {String(h ?? '') || <span className="text-surface-400">{i + 1}</span>}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {visible.map((row, ri) => (
              <tr key={ri} className={`hover:bg-emerald-50/30 transition-colors ${ri % 2 === 0 ? 'bg-white' : 'bg-surface-50/60'}`}>
                <td className="border-b border-surface-100 px-3 py-1.5 text-right text-xs text-surface-400 select-none">
                  {page * PAGE_SIZE + ri + 1}
                </td>
                {header.map((_, ci) => (
                  <td key={ci} className="border-b border-surface-100 px-3 py-1.5 text-xs font-mono text-surface-700 max-w-xs truncate" title={String(row[ci] ?? '')}>
                    {String(row[ci] ?? '')}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}

function CodeRenderer({ text, filename }: { text: string; filename: string }) {
  const lang = extToLang(filename)
  const lines = text.split('\n')

  return (
    <div className="rounded-xl border border-gray-700 overflow-hidden shadow-sm">
      <div className="flex items-center justify-between bg-gray-900 px-4 py-2.5 border-b border-gray-700">
        <span className="text-xs font-medium text-gray-200">{filename}</span>
        <div className="flex items-center gap-3 text-xs text-gray-400">
          <span>{lang}</span>
          <span>{lines.length.toLocaleString()} 行</span>
          <span>{formatBytes(new Blob([text]).size)}</span>
        </div>
      </div>
      <div className="overflow-auto bg-gray-950" style={{ maxHeight: 'calc(100vh - 300px)' }}>
        <table className="w-full text-sm font-mono">
          <tbody>
            {lines.map((line, i) => (
              <tr key={i} className="hover:bg-white/[0.04] transition-colors group">
                <td className="select-none pr-4 pl-4 py-[2px] text-right text-gray-600 text-xs w-14 border-r border-gray-800 group-hover:text-gray-400">
                  {i + 1}
                </td>
                <td className="pl-4 pr-4 py-[2px] text-gray-100 whitespace-pre">{line || '\u00a0'}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}

function ImagePreviewRenderer({ url, filename, file }: { url: string; filename: string; file: File }) {
  const [dims, setDims] = useState<{ w: number; h: number } | null>(null)

  return (
    <div className="space-y-3">
      <div className="flex items-center gap-4 rounded-xl border border-surface-200 bg-white p-3 shadow-sm text-xs text-surface-600 flex-wrap">
        <span className="font-mono">{file.type || '未知 MIME'}</span>
        {dims && <span>{dims.w} × {dims.h} px</span>}
        {dims && <span>宽高比 {(() => { const g = (a: number, b: number): number => b === 0 ? a : g(b, a % b); const d = g(dims.w, dims.h); return `${dims.w / d}:${dims.h / d}` })()}</span>}
        <span>{formatBytes(file.size)}</span>
      </div>
      <div
        className="flex items-center justify-center rounded-xl border border-surface-200 bg-[repeating-conic-gradient(#e5e7eb_0%_25%,white_0%_50%)] bg-[length:20px_20px] overflow-hidden"
        style={{ minHeight: '300px', maxHeight: 'calc(100vh - 320px)' }}
      >
        <img
          src={url}
          alt={filename}
          className="max-w-full object-contain"
          style={{ maxHeight: 'calc(100vh - 340px)' }}
          onLoad={(e) => {
            const img = e.currentTarget
            setDims({ w: img.naturalWidth, h: img.naturalHeight })
          }}
        />
      </div>
    </div>
  )
}

// ─── Supported formats matrix ─────────────────────────────────────────────────

function SupportedFormats() {
  const groups = [
    {
      label: '文档', color: 'text-blue-600',
      items: ['PDF (.pdf)', 'Markdown (.md/.mdx)', 'HTML (.html/.htm)', 'Word (.docx/.doc)'],
    },
    {
      label: '数据', color: 'text-green-600',
      items: ['CSV (.csv)', 'TSV (.tsv)', 'JSON (.json)', 'Excel (.xlsx/.xls)'],
    },
    {
      label: '代码', color: 'text-purple-600',
      items: [
        'JS/TS/TSX/JSX', 'Python / Go / Rust',
        'Java / C / C++ / C#', 'CSS/SCSS/Less',
        'Shell / Bash / Zsh', 'YAML / TOML / INI',
        'SQL / XML / SVG', 'Vue / Svelte', 'GraphQL / Proto',
        '文本 / 日志',
      ],
    },
    {
      label: '图片', color: 'text-pink-600',
      items: ['JPG / PNG / WebP / GIF', 'BMP / AVIF / ICO / SVG'],
    },
  ]

  return (
    <div className="rounded-xl border border-surface-200 bg-white p-5 shadow-sm">
      <h3 className="mb-4 text-sm font-semibold text-surface-800">支持的文档格式</h3>
      <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
        {groups.map((g) => (
          <div key={g.label}>
            <p className={`mb-2 text-xs font-semibold ${g.color}`}>{g.label}</p>
            <ul className="space-y-1">
              {g.items.map((item) => (
                <li key={item} className="text-xs text-surface-600">· {item}</li>
              ))}
            </ul>
          </div>
        ))}
      </div>
    </div>
  )
}

// ─── Page ─────────────────────────────────────────────────────────────────────

export default function DocsPreviewPage() {
  const [doc, setDoc] = useState<DocState | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [dragging, setDragging] = useState(false)
  const inputRef = useRef<HTMLInputElement>(null)

  async function handleFile(file: File) {
    setLoading(true); setError('')
    if (doc?.objectUrl) URL.revokeObjectURL(doc.objectUrl)
    const type = detectType(file)
    try {
      if (type === 'pdf' || type === 'image') {
        setDoc({ file, type, objectUrl: URL.createObjectURL(file) })
      } else if (type === 'docx') {
        const arrayBuffer = await file.arrayBuffer()
        const mammothMod = await import('mammoth')
        const mammoth = (mammothMod as any).default ?? mammothMod
        const result = await mammoth.convertToHtml({ arrayBuffer })
        setDoc({ file, type, text: result.value })
      } else if (type === 'xlsx') {
        const arrayBuffer = await file.arrayBuffer()
        const XLSX = await import('xlsx')
        const workbook = XLSX.read(arrayBuffer, { type: 'array' })
        const sheetNames = workbook.SheetNames
        const sheets: Record<string, string[][]> = {}
        for (const name of sheetNames) {
          const ws = workbook.Sheets[name]
          sheets[name] = XLSX.utils.sheet_to_json<string[]>(ws, { header: 1 })
        }
        setDoc({ file, type, text: JSON.stringify({ sheetNames, sheets }) })
      } else {
        if (file.size > MAX_TEXT_SIZE) {
          setError(`文件过大（${formatBytes(file.size)}），文本预览限制 5 MB 以内`)
          setLoading(false); return
        }
        const text = await file.text()
        setDoc({ file, type, text })
      }
    } catch (e: any) {
      setError(`解析失败：${e.message}`)
    } finally {
      setLoading(false)
    }
  }

  function clear() {
    if (doc?.objectUrl) URL.revokeObjectURL(doc.objectUrl)
    setDoc(null); setError('')
  }

  function renderPreview() {
    if (!doc) return null
    if (doc.type === 'pdf' && doc.objectUrl) return <PdfRenderer url={doc.objectUrl} />
    if (doc.type === 'image' && doc.objectUrl) return <ImagePreviewRenderer url={doc.objectUrl} filename={doc.file.name} file={doc.file} />
    if (!doc.text) return null
    if (doc.type === 'markdown') return <MarkdownRenderer text={doc.text} />
    if (doc.type === 'csv') return <CsvRenderer text={doc.text} />
    if (doc.type === 'tsv') return <CsvRenderer text={doc.text} sep="\t" />
    if (doc.type === 'json') return <JsonRenderer text={doc.text} />
    if (doc.type === 'html') return <HtmlRenderer text={doc.text} filename={doc.file.name} />
    if (doc.type === 'docx') return <HtmlRenderer text={doc.text} filename={doc.file.name} isDocx />
    if (doc.type === 'xlsx') return <XlsxRenderer text={doc.text} />
    return <CodeRenderer text={doc.text} filename={doc.file.name} />
  }

  return (
    <div className="mx-auto max-w-5xl space-y-5">
      {/* Header */}
      <div className="flex items-center gap-3">
        <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-gradient-to-br from-teal-500 to-emerald-500 shadow-sm">
          <Eye className="h-5 w-5 text-white" />
        </div>
        <div>
          <h1 className="text-lg font-bold text-surface-900">文档预览</h1>
          <p className="text-sm text-surface-500">PDF · Word · Excel · Markdown · CSV/TSV · JSON · HTML · 代码 · 图片 — 本地处理，不上传服务器</p>
        </div>
      </div>

      {/* Hidden file input */}
      <input
        ref={inputRef}
        type="file"
        accept={ACCEPT}
        className="hidden"
        onChange={(e) => { const f = e.target.files?.[0]; if (f) { handleFile(f); e.target.value = '' } }}
      />

      {/* Drop zone (no file loaded) */}
      {!doc && !loading && (
        <div
          onDragOver={(e) => { e.preventDefault(); setDragging(true) }}
          onDragLeave={() => setDragging(false)}
          onDrop={(e) => { e.preventDefault(); setDragging(false); const f = e.dataTransfer.files[0]; if (f) handleFile(f) }}
          onClick={() => inputRef.current?.click()}
          className={`flex h-52 cursor-pointer flex-col items-center justify-center gap-3 rounded-2xl border-2 border-dashed transition-all
            ${dragging ? 'border-teal-400 bg-teal-50 scale-[1.01]' : 'border-surface-200 bg-surface-50 hover:border-teal-300 hover:bg-teal-50/40'}`}
        >
          <div className="flex h-14 w-14 items-center justify-center rounded-2xl bg-surface-100 shadow-sm">
            <Upload className="h-7 w-7 text-surface-400" />
          </div>
          <div className="text-center">
            <p className="text-sm font-semibold text-surface-700">拖入文件，或点击选择</p>
            <p className="mt-1 text-xs text-surface-400">PDF · Word · Excel · Markdown · CSV · JSON · HTML · 代码 · 图片</p>
          </div>
        </div>
      )}

      {loading && (
        <div className="flex h-32 items-center justify-center rounded-xl border border-surface-200 bg-surface-50">
          <p className="text-sm text-surface-500">加载中…</p>
        </div>
      )}

      {error && (
        <div className="flex items-start gap-2 rounded-xl bg-red-50 border border-red-200 px-4 py-3 text-sm text-red-700">
          <AlertTriangle className="h-4 w-4 mt-0.5 flex-shrink-0" />
          <span>{error}</span>
        </div>
      )}

      {/* File info bar + preview */}
      {doc && (
        <div className="space-y-3">
          {/* Toolbar */}
          <div className="flex items-center justify-between rounded-xl border border-surface-200 bg-white px-4 py-2.5 shadow-sm">
            <div className="flex items-center gap-3 min-w-0">
              <FileText className="h-4.5 w-4.5 text-teal-500 flex-shrink-0" />
              <span className="text-sm font-medium text-surface-800 truncate max-w-sm" title={doc.file.name}>
                {doc.file.name}
              </span>
              <Badge className={`text-[11px] flex-shrink-0 ${TYPE_COLORS[doc.type]}`}>
                {TYPE_LABELS[doc.type]}
              </Badge>
              <span className="text-xs text-surface-400 flex-shrink-0">{formatBytes(doc.file.size)}</span>
            </div>
            <div className="flex items-center gap-1.5 flex-shrink-0 ml-3">
              <Button size="sm" variant="outline" onClick={() => inputRef.current?.click()} className="gap-1.5 h-7 px-2.5 text-xs">
                <Upload className="h-3 w-3" /> 换文件
              </Button>
              <Button size="sm" variant="ghost" onClick={clear} className="h-7 w-7 p-0 text-surface-400 hover:text-red-500">
                <X className="h-4 w-4" />
              </Button>
            </div>
          </div>

          {/* Preview content */}
          {renderPreview()}
        </div>
      )}

      {/* Supported formats (show when no file) */}
      {!doc && !loading && <SupportedFormats />}
    </div>
  )
}
