'use client'
import { useEffect, useRef, useCallback } from 'react'
import { useTaskStore } from './store/task'
import type { TaskProgress } from '@/types'

const RECONNECT_DELAY = 5000
const MAX_RECONNECT_DELAY = 30000

function normalizeWSBaseURL(value?: string) {
  const trimmed = value?.trim()
  if (!trimmed || trimmed === '/') return ''
  return trimmed.replace(/\/+$/, '')
}

function currentWSOrigin() {
  if (typeof window === 'undefined') return 'ws://localhost:8000'
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${protocol}//${window.location.host}`
}

function resolveWSBaseURL(value?: string) {
  return normalizeWSBaseURL(value) || currentWSOrigin()
}

// ─── Standardized Event Types ────────────────────────────────

export type WSEventType =
  | 'task:progress'
  | 'task:completed'
  | 'task:failed'
  | 'asset:progress'
  | 'asset:completed'
  | 'storyboard:progress'
  | 'storyboard:completed'
  | 'dubbing:progress'
  | 'dubbing:completed'
  | 'video:progress'
  | 'video:completed'
  | 'project:status'
  | 'storage:updated'

export interface WSMessage {
  type: WSEventType
  task_id?: number
  project_id?: number
  progress?: number
  message?: string
  status?: string
  timestamp: number
  data?: Record<string, unknown>
}

type WSEventHandler = (msg: WSMessage) => void

// ─── Global WebSocket Manager ────────────────────────────────

class WebSocketManager {
  private connections = new Map<string, WebSocket>()
  private handlers = new Map<string, Set<WSEventHandler>>()
  private reconnectAttempts = new Map<string, number>()
  private reconnectTimers = new Map<string, ReturnType<typeof setTimeout>>()
  private active = new Map<string, boolean>()

  connect(key: string, url: string) {
    if (this.connections.has(key)) return
    this.active.set(key, true)
    this._doConnect(key, url)
  }

  private _doConnect(key: string, url: string) {
    if (!this.active.get(key)) return

    const token = typeof window !== 'undefined' ? localStorage.getItem('access_token') : null
    const ws = new WebSocket(`${url}${url.includes('?') ? '&' : '?'}token=${token ?? ''}`)

    ws.onopen = () => {
      this.reconnectAttempts.set(key, 0)
    }

    ws.onmessage = (e) => {
      try {
        const msg = JSON.parse(e.data) as WSMessage
        this.emit(key, msg)
        // Also emit to wildcard handlers
        this.emit('*', msg)
      } catch {
        // ignore malformed
      }
    }

    ws.onclose = () => {
      this.connections.delete(key)
      if (this.active.get(key)) {
        const attempts = this.reconnectAttempts.get(key) ?? 0
        const delay = Math.min(RECONNECT_DELAY * Math.pow(1.5, attempts), MAX_RECONNECT_DELAY)
        this.reconnectAttempts.set(key, attempts + 1)
        const timer = setTimeout(() => this._doConnect(key, url), delay)
        this.reconnectTimers.set(key, timer)
      }
    }

    ws.onerror = () => ws.close()

    this.connections.set(key, ws)
  }

  disconnect(key: string) {
    this.active.set(key, false)
    const timer = this.reconnectTimers.get(key)
    if (timer) clearTimeout(timer)
    this.reconnectTimers.delete(key)
    const ws = this.connections.get(key)
    if (ws) ws.close()
    this.connections.delete(key)
    this.handlers.delete(key)
    this.reconnectAttempts.delete(key)
  }

  on(key: string, handler: WSEventHandler) {
    if (!this.handlers.has(key)) this.handlers.set(key, new Set())
    this.handlers.get(key)!.add(handler)
    return () => {
      this.handlers.get(key)?.delete(handler)
    }
  }

  private emit(key: string, msg: WSMessage) {
    this.handlers.get(key)?.forEach((fn) => fn(msg))
  }

  disconnectAll() {
    for (const key of this.connections.keys()) {
      this.disconnect(key)
    }
  }
}

export const wsManager = new WebSocketManager()

// ─── Hooks ───────────────────────────────────────────────────

export function useTaskProgress(taskId: number | null) {
  const { updateTaskProgress } = useTaskStore()

  useEffect(() => {
    if (!taskId) return
    const key = `task-${taskId}`

    wsManager.connect(key, `${resolveWSBaseURL(process.env.NEXT_PUBLIC_WS_URL)}/ws/tasks/${taskId}`)

    const unsub = wsManager.on(key, (msg) => {
      updateTaskProgress(taskId, msg as unknown as TaskProgress)
    })

    return () => {
      unsub()
      wsManager.disconnect(key)
    }
  }, [taskId, updateTaskProgress])
}

export function useProjectProgress(projectId: number | null) {
  const { updateTaskProgress } = useTaskStore()

  useEffect(() => {
    if (!projectId) return
    const key = `project-${projectId}`

    wsManager.connect(key, `${resolveWSBaseURL(process.env.NEXT_PUBLIC_WS_URL)}/ws/projects/${projectId}`)

    const unsub = wsManager.on(key, (msg) => {
      if (msg.task_id) updateTaskProgress(msg.task_id, msg as unknown as TaskProgress)
    })

    return () => {
      unsub()
      wsManager.disconnect(key)
    }
  }, [projectId, updateTaskProgress])
}

export function useNotifications(onMessage?: WSEventHandler) {
  useEffect(() => {
    const key = 'notifications'
    wsManager.connect(
      key,
      `${resolveWSBaseURL(process.env.NEXT_PUBLIC_NOTIFY_WS_URL || process.env.NEXT_PUBLIC_WS_URL)}/ws/connect`
    )

    let unsub: (() => void) | undefined
    if (onMessage) {
      unsub = wsManager.on(key, onMessage)
    }

    return () => {
      unsub?.()
      wsManager.disconnect(key)
    }
  }, [onMessage])
}

export function useGlobalEvents(handler: WSEventHandler) {
  useEffect(() => {
    const unsub = wsManager.on('*', handler)
    return unsub
  }, [handler])
}
