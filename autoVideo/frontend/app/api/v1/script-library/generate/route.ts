import { NextRequest, NextResponse } from 'next/server'

// Allow up to 120 seconds for AI script generation
export const maxDuration = 120

const UPSTREAM = (process.env.API_PROXY_TARGET || 'http://localhost:8000').replace(/\/+$/, '')

export async function POST(req: NextRequest) {
  const body = await req.text()
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
  }
  const auth = req.headers.get('Authorization')
  if (auth) headers['Authorization'] = auth

  const upstream = `${UPSTREAM}/api/v1/script-library/generate`
  const controller = new AbortController()
  const timer = setTimeout(() => controller.abort(), 115_000)

  try {
    const res = await fetch(upstream, {
      method: 'POST',
      headers,
      body,
      signal: controller.signal,
    })
    clearTimeout(timer)
    const data = await res.text()
    return new NextResponse(data, {
      status: res.status,
      headers: { 'Content-Type': 'application/json' },
    })
  } catch (err: unknown) {
    clearTimeout(timer)
    const message = err instanceof Error ? err.message : 'unknown error'
    return NextResponse.json({ code: 504, message: `gateway timeout: ${message}` }, { status: 504 })
  }
}
