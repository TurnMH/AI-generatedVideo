import { generateStaticIdParam } from '@/lib/static-export'

import ProjectDetailPageClient from './page-client'

export function generateStaticParams() {
  return generateStaticIdParam()
}

export default function ProjectDetailPage() {
  return <ProjectDetailPageClient />
}