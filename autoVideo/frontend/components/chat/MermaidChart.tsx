'use client'

import { useEffect, useRef, useState } from 'react'

let mermaidInitialized = false

async function ensureMermaid() {
  const mermaid = (await import('mermaid')).default
  if (!mermaidInitialized) {
    mermaid.initialize({
      startOnLoad: false,
      theme: 'neutral',
      fontFamily: 'ui-sans-serif, system-ui, sans-serif',
      fontSize: 13,
    })
    mermaidInitialized = true
  }
  return mermaid
}

export default function MermaidChart({ code }: { code: string }) {
  const containerRef = useRef<HTMLDivElement>(null)
  const [error, setError] = useState<string | null>(null)
  const [rendered, setRendered] = useState(false)

  useEffect(() => {
    let cancelled = false
    setError(null)
    setRendered(false)

    ensureMermaid().then(async (mermaid) => {
      if (cancelled || !containerRef.current) return
      try {
        const id = `mermaid-${Math.random().toString(36).slice(2)}`
        const { svg } = await mermaid.render(id, code.trim())
        if (cancelled || !containerRef.current) return
        containerRef.current.innerHTML = svg
        setRendered(true)
      } catch (e: any) {
        if (!cancelled) setError(e?.message ?? 'Mermaid 渲染失败')
      }
    })

    return () => { cancelled = true }
  }, [code])

  if (error) {
    return (
      <div className="rounded-xl border border-red-200 bg-red-50 p-3">
        <p className="mb-2 text-xs font-medium text-red-600">Mermaid 渲染错误</p>
        <pre className="overflow-x-auto text-xs text-red-500">{code}</pre>
        <p className="mt-1 text-xs text-red-400">{error}</p>
      </div>
    )
  }

  return (
    <div className="my-2 overflow-x-auto rounded-xl border border-surface-200 bg-white p-4">
      <div ref={containerRef} className={rendered ? '' : 'animate-pulse text-xs text-surface-400'}>
        {!rendered && '渲染图表中…'}
      </div>
    </div>
  )
}
