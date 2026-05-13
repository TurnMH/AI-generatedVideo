import type { ReactNode } from 'react'

import { generateStaticIdParam } from '@/lib/static-export'

export function generateStaticParams() {
  return generateStaticIdParam()
}

export default function ProjectIdLayout({ children }: { children: ReactNode }) {
  return children
}