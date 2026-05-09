'use client'

import type { ProjectMediaKind } from '@/lib/project-media'

const PENDING_PROJECT_DRAFT_STORAGE_KEY = 'autovideo:pending-project-draft:'

export interface PendingProjectDraft {
  title?: string
  description?: string
  scriptContent?: string
  scriptFileName?: string
  scriptMimeType?: string
  targetEpisodes?: number
  styleTags?: string[]
  media?: ProjectMediaKind
}

function getDraftStorageKey(id: string): string {
  return `${PENDING_PROJECT_DRAFT_STORAGE_KEY}${id}`
}

function generateDraftId(): string {
  return `${Date.now()}-${Math.random().toString(36).slice(2, 10)}`
}

export function savePendingProjectDraft(draft: PendingProjectDraft): string {
  if (typeof window === 'undefined') {
    throw new Error('savePendingProjectDraft can only be used in the browser')
  }

  const id = generateDraftId()
  window.localStorage.setItem(getDraftStorageKey(id), JSON.stringify(draft))
  return id
}

export function consumePendingProjectDraft(id: string): PendingProjectDraft | null {
  if (typeof window === 'undefined') {
    return null
  }

  const storageKey = getDraftStorageKey(id)
  const raw = window.localStorage.getItem(storageKey)
  if (!raw) {
    return null
  }

  window.localStorage.removeItem(storageKey)

  try {
    const parsed = JSON.parse(raw) as PendingProjectDraft
    return parsed && typeof parsed === 'object' ? parsed : null
  } catch {
    return null
  }
}
