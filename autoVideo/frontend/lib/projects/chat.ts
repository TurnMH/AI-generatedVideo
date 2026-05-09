import type { ChatMessage } from '@/types'

export type LegacyChatMessage = ChatMessage & {
  message?: string
  role?: string
  content?: string
  timestamp?: string
}

export function getChatRole(message: LegacyChatMessage): 'user' | 'assistant' {
  return message.role === 'user' ? 'user' : 'assistant'
}

export function getChatContent(message: LegacyChatMessage): string {
  return message.content?.trim() || message.message?.trim() || ''
}

export function getChatImageUrl(message: LegacyChatMessage): string | undefined {
  return message.image_url
}

export function getChatImageModel(message: LegacyChatMessage): string | undefined {
  const m = (message as unknown as { image_model?: string }).image_model
  return typeof m === 'string' && m.trim() !== '' ? m : undefined
}
