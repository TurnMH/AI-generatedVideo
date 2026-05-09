'use client'

import { useState } from 'react'
import { FileText, Copy, Check, RefreshCw } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs'
import { Badge } from '@/components/ui/badge'

// ─── Helpers ──────────────────────────────────────────────────────────────────

function useCopy() {
  const [copied, setCopied] = useState(false)
  const copy = (text: string) => {
    navigator.clipboard.writeText(text).then(() => {
      setCopied(true)
      setTimeout(() => setCopied(false), 1500)
    })
  }
  return { copied, copy }
}

function IOPanel({
  input, output, onInput, action, actionLabel, onSwap, placeholder, monoOutput = true,
}: {
  input: string
  output: string
  onInput: (v: string) => void
  action: () => void
  actionLabel: string
  onSwap?: () => void
  placeholder?: string
  monoOutput?: boolean
}) {
  const { copied, copy } = useCopy()
  return (
    <div className="grid grid-cols-2 gap-3">
      <div className="space-y-2">
        <p className="text-xs font-medium text-surface-500 uppercase tracking-wide">输入</p>
        <Textarea
          value={input}
          onChange={(e) => onInput(e.target.value)}
          placeholder={placeholder ?? '在此输入内容…'}
          className="h-48 resize-none font-mono text-sm"
        />
        <div className="flex gap-2">
          <Button size="sm" onClick={action} className="gap-1.5">
            <RefreshCw className="h-3.5 w-3.5" />
            {actionLabel}
          </Button>
          {onSwap && (
            <Button size="sm" variant="outline" onClick={onSwap} className="gap-1.5">
              ⇄ 互换
            </Button>
          )}
        </div>
      </div>
      <div className="space-y-2">
        <p className="text-xs font-medium text-surface-500 uppercase tracking-wide">输出</p>
        <div className="relative">
          <Textarea
            readOnly
            value={output}
            className={`h-48 resize-none text-sm ${monoOutput ? 'font-mono' : ''} bg-surface-50`}
          />
          {output && (
            <button
              onClick={() => copy(output)}
              className="absolute right-2 top-2 rounded-md border border-surface-200 bg-white px-2 py-1 text-xs text-surface-500 hover:text-indigo-600 transition-colors flex items-center gap-1"
            >
              {copied ? <Check className="h-3 w-3 text-green-500" /> : <Copy className="h-3 w-3" />}
              {copied ? '已复制' : '复制'}
            </button>
          )}
        </div>
      </div>
    </div>
  )
}

// ─── JSON tool ────────────────────────────────────────────────────────────────

function JsonTool() {
  const [input, setInput] = useState('')
  const [output, setOutput] = useState('')
  const [error, setError] = useState('')
  const [indent, setIndent] = useState(2)

  function format() {
    setError('')
    try {
      const parsed = JSON.parse(input)
      setOutput(JSON.stringify(parsed, null, indent))
    } catch (e: any) {
      setError(e.message); setOutput('')
    }
  }
  function minify() {
    setError('')
    try {
      setOutput(JSON.stringify(JSON.parse(input)))
    } catch (e: any) {
      setError(e.message); setOutput('')
    }
  }

  return (
    <div className="space-y-3">
      <div className="flex items-center gap-2">
        <span className="text-sm text-surface-600">缩进空格：</span>
        {[2, 4].map((n) => (
          <button key={n} onClick={() => setIndent(n)}
            className={`rounded border px-2 py-0.5 text-xs font-mono
              ${indent === n ? 'border-indigo-400 bg-indigo-50 text-indigo-700' : 'border-surface-200 text-surface-600'}`}>
            {n}
          </button>
        ))}
      </div>
      <IOPanel
        input={input} output={output} onInput={setInput}
        action={format} actionLabel="格式化"
        placeholder='{"key": "value"}'
      />
      <div className="flex gap-2">
        <Button size="sm" variant="outline" onClick={minify}>压缩 / Minify</Button>
      </div>
      {error && <p className="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-600">{error}</p>}
    </div>
  )
}

// ─── Base64 tool ──────────────────────────────────────────────────────────────

function Base64Tool() {
  const [input, setInput] = useState('')
  const [output, setOutput] = useState('')
  const [error, setError] = useState('')
  const [mode, setMode] = useState<'encode' | 'decode'>('encode')

  function run() {
    setError('')
    try {
      if (mode === 'encode') {
        setOutput(btoa(unescape(encodeURIComponent(input))))
      } else {
        setOutput(decodeURIComponent(escape(atob(input.trim()))))
      }
    } catch (e: any) {
      setError('解码失败：输入内容不是合法的 Base64')
      setOutput('')
    }
  }
  function swap() {
    setInput(output); setOutput(''); setMode(mode === 'encode' ? 'decode' : 'encode')
  }

  return (
    <div className="space-y-3">
      <div className="flex gap-2">
        {(['encode', 'decode'] as const).map((m) => (
          <button key={m} onClick={() => setMode(m)}
            className={`rounded border px-3 py-1 text-sm font-medium transition-colors
              ${mode === m ? 'border-indigo-400 bg-indigo-50 text-indigo-700' : 'border-surface-200 text-surface-600 hover:border-indigo-300'}`}>
            {m === 'encode' ? '编码（→ Base64）' : '解码（Base64 →）'}
          </button>
        ))}
      </div>
      <IOPanel
        input={input} output={output} onInput={setInput}
        action={run} actionLabel={mode === 'encode' ? '编码' : '解码'}
        onSwap={swap}
        placeholder={mode === 'encode' ? '输入文本内容…' : '输入 Base64 字符串…'}
      />
      {error && <p className="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-600">{error}</p>}
    </div>
  )
}

// ─── URL encode tool ──────────────────────────────────────────────────────────

function UrlTool() {
  const [input, setInput] = useState('')
  const [output, setOutput] = useState('')
  const [error, setError] = useState('')
  const [mode, setMode] = useState<'encode' | 'decode'>('encode')

  function run() {
    setError('')
    try {
      if (mode === 'encode') {
        setOutput(encodeURIComponent(input))
      } else {
        setOutput(decodeURIComponent(input))
      }
    } catch (e: any) {
      setError('解码失败：输入不是合法的 URL 编码')
      setOutput('')
    }
  }
  function swap() { setInput(output); setOutput(''); setMode(mode === 'encode' ? 'decode' : 'encode') }

  return (
    <div className="space-y-3">
      <div className="flex gap-2">
        {(['encode', 'decode'] as const).map((m) => (
          <button key={m} onClick={() => setMode(m)}
            className={`rounded border px-3 py-1 text-sm font-medium transition-colors
              ${mode === m ? 'border-indigo-400 bg-indigo-50 text-indigo-700' : 'border-surface-200 text-surface-600 hover:border-indigo-300'}`}>
            {m === 'encode' ? '编码（URL Encode）' : '解码（URL Decode）'}
          </button>
        ))}
      </div>
      <IOPanel
        input={input} output={output} onInput={setInput}
        action={run} actionLabel={mode === 'encode' ? '编码' : '解码'}
        onSwap={swap}
        placeholder={mode === 'encode' ? '例：你好 世界' : '例：%E4%BD%A0%E5%A5%BD'}
      />
      {error && <p className="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-600">{error}</p>}
    </div>
  )
}

// ─── Line ending / encoding tool ─────────────────────────────────────────────

function LineEndingTool() {
  const [input, setInput] = useState('')
  const [output, setOutput] = useState('')
  const { copied, copy } = useCopy()

  function convert(to: 'lf' | 'crlf' | 'cr') {
    const stripped = input.replace(/\r\n/g, '\n').replace(/\r/g, '\n')
    if (to === 'lf') setOutput(stripped)
    else if (to === 'crlf') setOutput(stripped.replace(/\n/g, '\r\n'))
    else setOutput(stripped.replace(/\n/g, '\r'))
  }

  function detectLineEnding(text: string): string {
    const crlf = (text.match(/\r\n/g) ?? []).length
    const cr = (text.replace(/\r\n/g, '').match(/\r/g) ?? []).length
    const lf = (text.replace(/\r\n/g, '').match(/\n/g) ?? []).length
    if (crlf > 0 && lf === 0 && cr === 0) return `CRLF (\\r\\n) — Windows 风格，共 ${crlf} 处`
    if (lf > 0 && crlf === 0 && cr === 0) return `LF (\\n) — Unix/Linux/macOS 风格，共 ${lf} 处`
    if (cr > 0 && crlf === 0 && lf === 0) return `CR (\\r) — 旧 Mac 风格，共 ${cr} 处`
    if (crlf > 0 || lf > 0 || cr > 0) return `混合换行符（CRLF:${crlf} LF:${lf} CR:${cr}）`
    return '无换行符 / 空内容'
  }

  return (
    <div className="space-y-3">
      <Textarea
        value={input}
        onChange={(e) => setInput(e.target.value)}
        placeholder="粘贴文本内容…"
        className="h-36 resize-none font-mono text-sm"
      />
      {input && (
        <div className="rounded-lg bg-surface-50 border border-surface-200 px-3 py-2 text-sm">
          <span className="text-surface-500">检测到：</span>
          <span className="font-mono text-surface-800">{detectLineEnding(input)}</span>
        </div>
      )}
      <div className="flex gap-2 flex-wrap">
        <Button size="sm" onClick={() => convert('lf')} variant="outline">转为 LF（Unix）</Button>
        <Button size="sm" onClick={() => convert('crlf')} variant="outline">转为 CRLF（Windows）</Button>
        <Button size="sm" onClick={() => convert('cr')} variant="outline">转为 CR（旧 Mac）</Button>
      </div>
      {output && (
        <div className="space-y-2">
          <div className="relative">
            <Textarea readOnly value={output} className="h-36 resize-none font-mono text-sm bg-surface-50" />
            <button
              onClick={() => copy(output)}
              className="absolute right-2 top-2 rounded-md border border-surface-200 bg-white px-2 py-1 text-xs text-surface-500 hover:text-indigo-600 transition-colors flex items-center gap-1"
            >
              {copied ? <Check className="h-3 w-3 text-green-500" /> : <Copy className="h-3 w-3" />}
              {copied ? '已复制' : '复制'}
            </button>
          </div>
          <p className="text-xs text-surface-400">转换后：{detectLineEnding(output)}</p>
        </div>
      )}
      <div className="rounded-lg bg-blue-50 px-4 py-3 text-xs text-blue-700 space-y-1">
        <p className="font-medium">兼容性说明</p>
        <p>• <strong>LF (\\n)</strong>：Linux/macOS/Git 默认，建议代码文件统一使用</p>
        <p>• <strong>CRLF (\\r\\n)</strong>：Windows 记事本、Excel CSV 默认，跨平台协作时常见</p>
        <p>• <strong>混合换行符</strong>可能导致代码 diff 异常、CSV 解析错误</p>
      </div>
    </div>
  )
}

// ─── Count tool ───────────────────────────────────────────────────────────────

function CountTool() {
  const [input, setInput] = useState('')

  const chars = input.length
  const charsNoSpace = input.replace(/\s/g, '').length
  const lines = input === '' ? 0 : input.split('\n').length
  const words = input.trim() === '' ? 0 : input.trim().split(/\s+/).length
  const bytes = new Blob([input]).size
  const cnChars = (input.match(/[\u4e00-\u9fff]/g) ?? []).length

  return (
    <div className="space-y-3">
      <Textarea
        value={input}
        onChange={(e) => setInput(e.target.value)}
        placeholder="粘贴或输入文本…"
        className="h-48 resize-none text-sm"
      />
      <div className="grid grid-cols-3 gap-3">
        {[
          ['字符数', chars],
          ['不含空格', charsNoSpace],
          ['行数', lines],
          ['单词数', words],
          ['汉字数', cnChars],
          ['字节数 (UTF-8)', bytes],
        ].map(([label, val]) => (
          <div key={label as string} className="rounded-lg border border-surface-200 bg-white p-3 text-center">
            <p className="text-2xl font-bold text-indigo-600">{val as number}</p>
            <p className="text-xs text-surface-500 mt-1">{label as string}</p>
          </div>
        ))}
      </div>
    </div>
  )
}

// ─── HTML Entity tool ───────────────────────────────────────────────────────────────

function HtmlTool() {
  const [input, setInput] = useState('')
  const [output, setOutput] = useState('')
  const [mode, setMode] = useState<'encode' | 'decode'>('encode')

  function encode(t: string) {
    return t.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;').replace(/'/g, '&#39;')
  }
  function decode(t: string) {
    const doc = new DOMParser().parseFromString(t, 'text/html')
    return doc.documentElement.textContent ?? ''
  }
  function run() { setOutput(mode === 'encode' ? encode(input) : decode(input)) }
  function swap() { setInput(output); setOutput(''); setMode(m => m === 'encode' ? 'decode' : 'encode') }

  return (
    <div className="space-y-3">
      <div className="flex gap-2">
        {(['encode', 'decode'] as const).map((m) => (
          <button key={m} onClick={() => setMode(m)}
            className={`rounded border px-3 py-1 text-sm font-medium transition-colors ${mode === m ? 'border-indigo-400 bg-indigo-50 text-indigo-700' : 'border-surface-200 text-surface-600 hover:border-indigo-300'}`}>
            {m === 'encode' ? '编码（转义 HTML 实体）' : '解码（还原 HTML 实体）'}
          </button>
        ))}
      </div>
      <IOPanel
        input={input} output={output} onInput={setInput}
        action={run} actionLabel={mode === 'encode' ? '编码' : '解码'}
        onSwap={swap}
        placeholder={mode === 'encode' ? '例：<div class="box">Hello & World</div>' : '例：&lt;div&gt;Hello &amp; World&lt;/div&gt;'}
      />
    </div>
  )
}

// ─── Case conversion tool ───────────────────────────────────────────────────────────────

function toCamel(s: string) { return s.replace(/[\s_-]+(\w)/g, (_, c) => c.toUpperCase()).replace(/^[A-Z]/, c => c.toLowerCase()) }
function toPascal(s: string) { return s.replace(/[\s_-]+(\w)/g, (_, c) => c.toUpperCase()).replace(/^[a-z]/, c => c.toUpperCase()) }
function toSnake(s: string) { return s.replace(/([A-Z])/g, '_$1').replace(/[\s-]+/g, '_').replace(/__+/g, '_').replace(/^_/, '').toLowerCase() }
function toKebab(s: string) { return s.replace(/([A-Z])/g, '-$1').replace(/[\s_]+/g, '-').replace(/--+/g, '-').replace(/^-/, '').toLowerCase() }
function toTitle(s: string) { return s.toLowerCase().replace(/(?:^|[\s_-])\S/g, (c) => c.toUpperCase()) }

function CaseTool() {
  const [input, setInput] = useState('')
  const { copy } = useCopy()

  const conversions = input.trim() === '' ? [] : [
    { label: 'camelCase', value: toCamel(input) },
    { label: 'PascalCase', value: toPascal(input) },
    { label: 'snake_case', value: toSnake(input) },
    { label: 'kebab-case', value: toKebab(input) },
    { label: 'Title Case', value: toTitle(input) },
    { label: 'UPPER CASE', value: input.toUpperCase() },
    { label: 'lower case', value: input.toLowerCase() },
    { label: 'CONSTANT_CASE', value: toSnake(input).toUpperCase() },
  ]

  return (
    <div className="space-y-3">
      <Textarea
        value={input}
        onChange={(e) => setInput(e.target.value)}
        placeholder="输入文字，例：hello world / helloWorld / hello_world"
        className="h-24 resize-none text-sm"
      />
      {conversions.length > 0 && (
        <div className="grid grid-cols-2 gap-2">
          {conversions.map(({ label, value }) => (
            <div key={label} className="flex items-center justify-between rounded-lg border border-surface-200 bg-white px-3 py-2">
              <div className="min-w-0 flex-1">
                <p className="text-xs text-surface-500">{label}</p>
                <p className="font-mono text-sm text-surface-800 mt-0.5 truncate">{value}</p>
              </div>
              <button onClick={() => copy(value)} className="ml-2 flex-shrink-0 rounded border border-surface-200 px-2 py-1 text-xs text-surface-500 hover:text-indigo-600 transition-colors">
                复制
              </button>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

// ─── Regex tester ─────────────────────────────────────────────────────────────────────

function RegexTool() {
  const [pattern, setPattern] = useState('')
  const [flagsState, setFlagsState] = useState({ g: true, i: false, m: false, s: false })
  const [testStr, setTestStr] = useState('')

  function toggleFlag(f: 'g' | 'i' | 'm' | 's') { setFlagsState(prev => ({ ...prev, [f]: !prev[f] })) }

  const flagStr = Object.entries(flagsState).filter(([, v]) => v).map(([k]) => k).join('')
  let regex: RegExp | null = null
  let regexError = ''
  if (pattern) {
    try { regex = new RegExp(pattern, flagStr) }
    catch (e: any) { regexError = e.message }
  }

  const gFlagStr = flagStr.includes('g') ? flagStr : flagStr + 'g'
  const matches = (regex && testStr) ? (() => {
    try { return [...testStr.matchAll(new RegExp(pattern, gFlagStr))] } catch { return [] }
  })() : []

  function renderHighlighted() {
    if (!pattern || !testStr || matches.length === 0) return null
    const parts: (string | JSX.Element)[] = []
    let last = 0
    for (const m of matches) {
      if (m.index === undefined || m[0].length === 0) continue
      if (m.index > last) parts.push(testStr.slice(last, m.index))
      parts.push(<mark key={`m${m.index}`} className="bg-yellow-200 text-yellow-900 rounded-sm px-0.5">{m[0]}</mark>)
      last = m.index + m[0].length
    }
    if (last < testStr.length) parts.push(testStr.slice(last))
    return parts.length > 0 ? parts : null
  }

  const highlighted = renderHighlighted()

  return (
    <div className="space-y-3">
      <div className="flex items-center gap-2">
        <span className="text-surface-400 font-mono text-lg">/</span>
        <input value={pattern} onChange={(e) => setPattern(e.target.value)}
          placeholder="正则表达式"
          className="flex-1 rounded-lg border border-surface-200 px-3 py-1.5 text-sm font-mono focus:outline-none focus:border-indigo-400"
        />
        <span className="text-surface-400 font-mono text-lg">/</span>
        <div className="flex gap-1">
          {(['g', 'i', 'm', 's'] as const).map(f => (
            <button key={f} onClick={() => toggleFlag(f)}
              className={`w-7 h-7 rounded border text-xs font-mono font-bold transition-colors ${(flagsState as any)[f] ? 'border-indigo-400 bg-indigo-50 text-indigo-700' : 'border-surface-200 text-surface-500 hover:border-indigo-300'}`}>
              {f}
            </button>
          ))}
        </div>
      </div>
      {regexError && <p className="rounded-lg bg-red-50 px-3 py-2 text-xs text-red-600 font-mono">{regexError}</p>}
      <Textarea value={testStr} onChange={(e) => setTestStr(e.target.value)} placeholder="输入测试字符串…" className="h-32 resize-none text-sm" />
      {highlighted && (
        <div className="rounded-lg border border-surface-200 bg-white p-3 space-y-2">
          <p className="text-xs text-surface-500">匹配高亮（共 {matches.length} 处）</p>
          <div className="font-mono text-sm whitespace-pre-wrap break-all leading-6">{highlighted}</div>
        </div>
      )}
      {matches.length > 0 && (
        <div className="space-y-1.5 max-h-48 overflow-y-auto">
          <p className="text-xs font-medium text-surface-600">匹配详情</p>
          {matches.slice(0, 30).map((m, i) => (
            <div key={i} className="rounded border border-surface-100 bg-surface-50 px-3 py-1.5 text-xs font-mono">
              <span className="text-surface-400">#{i + 1} @{m.index}　</span>
              <span className="text-indigo-700">{JSON.stringify(m[0])}</span>
              {m.slice(1).map((g, gi) => g !== undefined ? (
                <span key={gi} className="ml-2 text-green-600">${gi + 1}={JSON.stringify(g)}</span>
              ) : null)}
            </div>
          ))}
          {matches.length > 30 && <p className="text-xs text-surface-400 px-1">… 还有 {matches.length - 30} 处</p>}
        </div>
      )}
    </div>
  )
}

// ─── Timestamp converter ─────────────────────────────────────────────────────────────

function TimestampTool() {
  const [tsInput, setTsInput] = useState('')
  const [dateInput, setDateInput] = useState('')
  const [, setTick] = useState(0)
  const { copy } = useCopy()

  const now = Math.floor(Date.now() / 1000)
  const nowMs = Date.now()

  function parseTs(s: string): Date | null {
    const n = Number(s.trim())
    if (isNaN(n)) return null
    return new Date(s.trim().length >= 13 ? n : n * 1000)
  }

  const tsDate = tsInput ? parseTs(tsInput) : null
  const dateTs = (() => { if (!dateInput) return null; const d = new Date(dateInput); return isNaN(d.getTime()) ? null : d })()

  const fmts = (d: Date) => [
    ['本地时间', d.toLocaleString('zh-CN')],
    ['UTC', d.toUTCString()],
    ['ISO 8601', d.toISOString()],
    ['Unix (s)', String(Math.floor(d.getTime() / 1000))],
    ['Unix (ms)', String(d.getTime())],
  ]

  return (
    <div className="space-y-4">
      <div className="grid grid-cols-2 gap-3">
        <div className="rounded-xl border border-surface-200 bg-white p-4 space-y-3 shadow-sm">
          <p className="text-sm font-medium text-surface-700">时间戳 → 日期</p>
          <div className="flex gap-2">
            <input value={tsInput} onChange={(e) => setTsInput(e.target.value)} placeholder="例：1714924800"
              className="flex-1 rounded-lg border border-surface-200 px-3 py-1.5 text-sm font-mono focus:outline-none focus:border-indigo-400" />
            <Button size="sm" variant="outline" onClick={() => setTsInput(String(now))}>现在</Button>
          </div>
          {tsDate && !isNaN(tsDate.getTime()) ? (
            <div className="space-y-1.5">
              {fmts(tsDate).map(([k, v]) => (
                <div key={k} className="flex items-start justify-between gap-2">
                  <span className="text-xs text-surface-500 w-20 flex-shrink-0 pt-0.5">{k}</span>
                  <span className="flex-1 text-xs font-mono text-surface-800 break-all">{v}</span>
                  <button onClick={() => copy(v)} className="text-xs text-indigo-500 hover:text-indigo-700 flex-shrink-0">复制</button>
                </div>
              ))}
            </div>
          ) : tsInput ? <p className="text-xs text-red-500">无效时间戳</p> : null}
        </div>
        <div className="rounded-xl border border-surface-200 bg-white p-4 space-y-3 shadow-sm">
          <p className="text-sm font-medium text-surface-700">日期 → 时间戳</p>
          <input type="datetime-local" value={dateInput} onChange={(e) => setDateInput(e.target.value)}
            className="w-full rounded-lg border border-surface-200 px-3 py-1.5 text-sm focus:outline-none focus:border-indigo-400" />
          {dateTs && (
            <div className="space-y-1.5">
              {[['Unix (s)', String(Math.floor(dateTs.getTime() / 1000))], ['Unix (ms)', String(dateTs.getTime())]].map(([k, v]) => (
                <div key={k} className="flex items-center justify-between gap-2">
                  <span className="text-xs text-surface-500 w-20">{k}</span>
                  <span className="flex-1 text-sm font-mono text-surface-800">{v}</span>
                  <button onClick={() => copy(v)} className="text-xs text-indigo-500 hover:text-indigo-700">复制</button>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
      <div className="rounded-xl border border-surface-200 bg-surface-50 p-4 space-y-2">
        <div className="flex items-center justify-between">
          <p className="text-xs font-medium text-surface-600">当前时间</p>
          <Button size="sm" variant="outline" onClick={() => setTick(n => n + 1)} className="gap-1.5 h-7 px-2 text-xs">
            <RefreshCw className="h-3 w-3" /> 刷新
          </Button>
        </div>
        <div className="grid grid-cols-2 gap-2">
          {[['Unix (s)', String(now)], ['Unix (ms)', String(nowMs)], ['本地', new Date().toLocaleString('zh-CN')], ['ISO', new Date().toISOString()]].map(([k, v]) => (
            <div key={k} className="flex items-center justify-between rounded-lg border border-surface-200 bg-white px-3 py-1.5">
              <span className="text-xs text-surface-500">{k}</span>
              <div className="flex items-center gap-1.5">
                <span className="text-xs font-mono text-surface-800">{v}</span>
                <button onClick={() => copy(v)} className="text-[10px] text-indigo-500 hover:text-indigo-700">复制</button>
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}

// ─── Color converter ────────────────────────────────────────────────────────────────

function hexToRgb(hex: string): { r: number; g: number; b: number } | null {
  const m = /^#?([a-f\d]{2})([a-f\d]{2})([a-f\d]{2})$/i.exec(hex.trim())
  return m ? { r: parseInt(m[1], 16), g: parseInt(m[2], 16), b: parseInt(m[3], 16) } : null
}
function rgbToHsl(r: number, g: number, b: number) {
  r /= 255; g /= 255; b /= 255
  const max = Math.max(r, g, b), min = Math.min(r, g, b)
  let h = 0, s = 0; const l = (max + min) / 2
  if (max !== min) {
    const d = max - min; s = l > 0.5 ? d / (2 - max - min) : d / (max + min)
    if (max === r) h = ((g - b) / d + (g < b ? 6 : 0)) / 6
    else if (max === g) h = ((b - r) / d + 2) / 6
    else h = ((r - g) / d + 4) / 6
  }
  return { h: Math.round(h * 360), s: Math.round(s * 100), l: Math.round(l * 100) }
}

function ColorTool() {
  const [hex, setHex] = useState('#4f46e5')
  const { copy } = useCopy()
  const rgb = hexToRgb(hex)
  const hsl = rgb ? rgbToHsl(rgb.r, rgb.g, rgb.b) : null
  const safeHex = /^#[0-9a-f]{6}$/i.test(hex) ? hex : '#4f46e5'

  return (
    <div className="space-y-4">
      <div className="flex items-center gap-4">
        <div
          className="h-20 w-20 rounded-xl border-4 border-white shadow-md flex-shrink-0 transition-colors"
          style={{ backgroundColor: rgb ? `rgb(${rgb.r},${rgb.g},${rgb.b})` : '#ccc' }}
        />
        <div className="flex-1 space-y-2">
          <div className="flex items-center gap-2">
            <input type="color" value={safeHex} onChange={(e) => setHex(e.target.value)}
              className="h-9 w-10 rounded cursor-pointer border border-surface-200 p-0.5" />
            <input value={hex} onChange={(e) => setHex(e.target.value)} placeholder="#rrggbb"
              className="flex-1 rounded-lg border border-surface-200 px-3 py-1.5 text-sm font-mono uppercase focus:outline-none focus:border-indigo-400" />
          </div>
          <p className="text-xs text-surface-400">颜色选择器 或 直接输入 HEX（#RRGGBB）</p>
        </div>
      </div>
      {rgb && hsl && (
        <div className="space-y-2">
          {([
            ['HEX', hex.toUpperCase()],
            ['RGB', `rgb(${rgb.r}, ${rgb.g}, ${rgb.b})`],
            ['RGB 归一化', `(${(rgb.r / 255).toFixed(3)}, ${(rgb.g / 255).toFixed(3)}, ${(rgb.b / 255).toFixed(3)})`],
            ['HSL', `hsl(${hsl.h}, ${hsl.s}%, ${hsl.l}%)`],
            ['CSS变量', `--color: ${hex.toUpperCase()};`],
            ['rgba()', `rgba(${rgb.r}, ${rgb.g}, ${rgb.b}, 1.0)`],
          ] as [string, string][]).map(([label, value]) => (
            <div key={label} className="flex items-center justify-between rounded-lg border border-surface-200 bg-white px-3 py-2">
              <span className="text-xs text-surface-500 w-24 flex-shrink-0">{label}</span>
              <span className="flex-1 text-sm font-mono text-surface-800">{value}</span>
              <button onClick={() => copy(value)} className="text-xs text-indigo-500 hover:text-indigo-700 ml-2 flex-shrink-0">复制</button>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

// ─── Hash calculator ────────────────────────────────────────────────────────────────

async function computeHash(text: string) {
  const enc = new TextEncoder().encode(text)
  const results: Record<string, string> = {}
  for (const algo of ['SHA-1', 'SHA-256', 'SHA-512'] as const) {
    const buf = await crypto.subtle.digest(algo, enc)
    results[algo] = Array.from(new Uint8Array(buf)).map(b => b.toString(16).padStart(2, '0')).join('')
  }
  return results
}

function HashTool() {
  const [input, setInput] = useState('')
  const [hashes, setHashes] = useState<Record<string, string>>({})
  const [computing, setComputing] = useState(false)
  const { copy } = useCopy()

  async function run() {
    if (!input) return
    setComputing(true)
    try { setHashes(await computeHash(input)) }
    finally { setComputing(false) }
  }

  return (
    <div className="space-y-3">
      <Textarea value={input} onChange={(e) => setInput(e.target.value)}
        placeholder="输入要计算哈希的文本内容…"
        className="h-36 resize-none text-sm"
      />
      <Button onClick={run} disabled={!input || computing} className="gap-2">
        {computing && <RefreshCw className="h-4 w-4 animate-spin" />}
        {computing ? '计算中…' : '计算 Hash'}
      </Button>
      {Object.keys(hashes).length > 0 && (
        <div className="space-y-2">
          {Object.entries(hashes).map(([algo, hash]) => (
            <div key={algo} className="rounded-lg border border-surface-200 bg-white p-3">
              <div className="flex items-center justify-between mb-1.5">
                <span className="text-xs font-medium text-surface-600">{algo}</span>
                <button onClick={() => copy(hash)} className="text-xs text-indigo-500 hover:text-indigo-700">复制</button>
              </div>
              <p className="font-mono text-xs text-surface-800 break-all leading-5">{hash}</p>
            </div>
          ))}
          <p className="text-xs text-surface-400">注：SHA-1 存在碰撞风险，仅供文件校验，不建议用于安全场景</p>
        </div>
      )}
    </div>
  )
}

// ─── Page ─────────────────────────────────────────────────────────────────────

export default function TextToolsPage() {
  return (
    <div className="mx-auto max-w-3xl space-y-6">
      <div className="flex items-center gap-3">
        <div className="flex h-10 w-10 items-center justify-center rounded-xl bg-gradient-to-br from-violet-500 to-indigo-500 shadow-sm">
          <FileText className="h-5 w-5 text-white" />
        </div>
        <div>
          <h1 className="text-lg font-bold text-surface-900">文字工具</h1>
          <p className="text-sm text-surface-500">JSON · Base64 · URL · HTML实体 · 大小写 · 正则 · 时间戳 · 颜色 · Hash · 换行符 · 字数</p>
        </div>
      </div>

      <Tabs defaultValue="json">
        <div className="overflow-x-auto">
          <TabsList className="flex w-max min-w-full">
            <TabsTrigger value="json">JSON</TabsTrigger>
            <TabsTrigger value="base64">Base64</TabsTrigger>
            <TabsTrigger value="url">URL 编解码</TabsTrigger>
            <TabsTrigger value="html">HTML 实体</TabsTrigger>
            <TabsTrigger value="case">大小写</TabsTrigger>
            <TabsTrigger value="regex">正则测试</TabsTrigger>
            <TabsTrigger value="timestamp">时间戳</TabsTrigger>
            <TabsTrigger value="color">颜色转换</TabsTrigger>
            <TabsTrigger value="hash">Hash 计算</TabsTrigger>
            <TabsTrigger value="lineending">换行符</TabsTrigger>
            <TabsTrigger value="count">字数统计</TabsTrigger>
          </TabsList>
        </div>

        <TabsContent value="json" className="pt-2"><JsonTool /></TabsContent>
        <TabsContent value="base64" className="pt-2"><Base64Tool /></TabsContent>
        <TabsContent value="url" className="pt-2"><UrlTool /></TabsContent>
        <TabsContent value="html" className="pt-2"><HtmlTool /></TabsContent>
        <TabsContent value="case" className="pt-2"><CaseTool /></TabsContent>
        <TabsContent value="regex" className="pt-2"><RegexTool /></TabsContent>
        <TabsContent value="timestamp" className="pt-2"><TimestampTool /></TabsContent>
        <TabsContent value="color" className="pt-2"><ColorTool /></TabsContent>
        <TabsContent value="hash" className="pt-2"><HashTool /></TabsContent>
        <TabsContent value="lineending" className="pt-2"><LineEndingTool /></TabsContent>
        <TabsContent value="count" className="pt-2"><CountTool /></TabsContent>
      </Tabs>
    </div>
  )
}
