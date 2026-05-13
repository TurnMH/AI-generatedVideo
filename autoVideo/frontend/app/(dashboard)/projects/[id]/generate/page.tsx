import { generateStaticIdParam } from '@/lib/static-export'

import GeneratePageClient from './page-client'

export function generateStaticParams() {
  return generateStaticIdParam()
}

export default function GeneratePage() {
  return <GeneratePageClient />
}