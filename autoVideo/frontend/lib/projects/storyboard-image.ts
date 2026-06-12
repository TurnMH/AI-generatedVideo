import { storyboardAPI } from '@/lib/api'
import type { Storyboard } from '@/types'

export async function triggerStoryboardImageGeneration(
  projectId: number,
  sb: Pick<Storyboard, 'id' | 'status'>,
  modelName?: string,
) {
  if (sb.status === 'failed' || sb.status === 'pending' || sb.status === 'paused') {
    await storyboardAPI.retry(projectId, sb.id, modelName)
    return
  }
  await storyboardAPI.generate(projectId, sb.id, modelName)
}

export function canTriggerStoryboardImage(sb: Pick<Storyboard, 'status' | 'image_url'>) {
  return sb.status === 'failed'
    || sb.status === 'pending'
    || sb.status === 'paused'
    || sb.status === 'completed'
    || (sb.status === 'generating' && !sb.image_url)
}
