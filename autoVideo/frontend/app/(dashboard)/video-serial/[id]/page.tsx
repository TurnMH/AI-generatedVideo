import { generateStaticIdParam } from '@/lib/static-export'

import SerialProjectDetailPageClient from './page-client'

export function generateStaticParams() {
  return generateStaticIdParam()
}

export default function SerialProjectDetailPage() {
  return <SerialProjectDetailPageClient />
}