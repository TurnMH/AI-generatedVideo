import { generateStaticIdParam } from '@/lib/static-export'

import SerialGeneratePageClient from './page-client'

export function generateStaticParams() {
  return generateStaticIdParam()
}

export default function SerialGeneratePage() {
  return <SerialGeneratePageClient />
}