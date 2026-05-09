'use client'

import { useEffect } from 'react'
import { useRouter } from 'next/navigation'

export default function VideoSerialNewPage() {
  const router = useRouter()
  useEffect(() => {
    router.replace('/projects/new?media=video_serial')
  }, [router])
  return null
}
