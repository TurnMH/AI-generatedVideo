import { STATIC_EXPORT_DYNAMIC_ID } from '@/lib/static-export'

import VideoHistoryTaskPageClient from './page-client'

export function generateStaticParams() {
  return [{ taskId: STATIC_EXPORT_DYNAMIC_ID }]
}

export default function VideoHistoryTaskPage() {
  return <VideoHistoryTaskPageClient />
}
