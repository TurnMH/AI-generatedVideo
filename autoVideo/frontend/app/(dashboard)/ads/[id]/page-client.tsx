
'use client'

import React from 'react'
import { useParams, usePathname } from 'next/navigation'

import ProjectDetailPageClient from '@/app/(dashboard)/projects/[id]/page-client'
import { resolveProjectIdParam } from '@/lib/project-route'

export default function AdProjectDetailPageClient() {
  const params = useParams()
  const pathname = usePathname()
  const projectId = resolveProjectIdParam(params.id, pathname, 'ads')

  return <ProjectDetailPageClient key={projectId ?? 'ad-project-detail'} />
}
