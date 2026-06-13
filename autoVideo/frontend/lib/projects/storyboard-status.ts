const STORYBOARD_IMAGE_ACTIVE_STATUSES = new Set(['generating', 'paused'])

/** Storyboard image generation is actively running. */
export function isStoryboardImageActive(status?: string | null): boolean {
  return STORYBOARD_IMAGE_ACTIVE_STATUSES.has(String(status ?? '').trim())
}

/** Storyboard structure exists but image generation has not started yet. */
export function isStoryboardAwaitingImage(status?: string | null): boolean {
  return String(status ?? '').trim() === 'pending'
}

export function formatStoryboardErrorMessage(msg: string): string {
  if (!msg) return '生成失败'
  if (msg.includes('moderation') || msg.includes('content_policy'))
    return '内容审核未通过 — 建议换用通义万相'
  if (msg.includes('timeout') || msg.includes('deadline'))
    return '生成超时 — 服务繁忙，请稍后重试'
  if (msg.includes('rate') || msg.includes('429'))
    return '请求过于频繁 — 请稍后重试'
  if (msg.includes('unreachable') || msg.includes('connection'))
    return '服务不可达 — 请检查服务状态'
  if (msg.includes('upload') || msg.includes('storage'))
    return '图片上传失败 — 存储服务异常'
  if (msg.length > 80) return msg.substring(0, 77) + '...'
  return msg
}
