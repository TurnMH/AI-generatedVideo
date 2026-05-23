import { generateStaticIdParam } from '@/lib/static-export'

import AdProjectDetailPageClient from './page-client'

export function generateStaticParams() {
  return generateStaticIdParam()
}

export default function AdProjectDetailPage() {
  return <AdProjectDetailPageClient />
}
