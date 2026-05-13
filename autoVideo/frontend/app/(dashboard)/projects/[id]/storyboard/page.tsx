import { generateStaticIdParam } from '@/lib/static-export'

import StoryboardPageClient from './page-client'

export function generateStaticParams() {
  return generateStaticIdParam()
}

export default function StoryboardPage() {
  return <StoryboardPageClient />
}