import { STATIC_EXPORT_DYNAMIC_ID } from '@/lib/static-export'

import AdVideoHistoryPageClient from './page-client'

export function generateStaticParams() {
  return [{ projectId: STATIC_EXPORT_DYNAMIC_ID }]
}

export default function AdVideoHistoryPage() {
  return <AdVideoHistoryPageClient />
}
