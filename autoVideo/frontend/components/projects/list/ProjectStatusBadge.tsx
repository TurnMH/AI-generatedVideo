'use client'

import type { ProjectStatus } from '@/types'
import { Badge } from '@/components/ui/badge'

export const PROJECT_LIST_STATUS_MAP: Record<
  string,
  { label: string; variant: 'default' | 'secondary' | 'destructive' | 'outline' | 'success' | 'warning'; animated?: boolean }
> = {
  draft:                  { label: '草稿',       variant: 'secondary' },
  script_processing:      { label: '剧本处理中', variant: 'default',     animated: true },
  script_ready:           { label: '剧本就绪',   variant: 'success' },
  asset_generating:       { label: '资源生成中', variant: 'default',     animated: true },
  asset_ready:            { label: '资源就绪',   variant: 'success' },
  storyboard_generating:  { label: '分镜生成中', variant: 'default',     animated: true },
  storyboard_ready:       { label: '分镜就绪',   variant: 'success' },
  video_generating:       { label: '视频生成中', variant: 'default',     animated: true },
  completed:              { label: '已完成',     variant: 'success' },
  paused:                 { label: '已暂停',     variant: 'warning' },
  failed:                 { label: '失败',       variant: 'destructive' },
}

export function ProjectStatusBadge({ status }: { status: ProjectStatus }) {
  const info = PROJECT_LIST_STATUS_MAP[status] || { label: status, variant: 'secondary' as const }
  return (
    <Badge variant={info.variant} className={info.animated ? 'animate-pulse' : ''}>
      {info.label}
    </Badge>
  )
}
