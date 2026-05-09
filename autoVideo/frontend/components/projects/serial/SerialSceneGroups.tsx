'use client'

import { useState, useMemo } from 'react'
import useSWR from 'swr'
import { Film, ChevronDown, ChevronRight, ImageIcon, ArrowRight } from 'lucide-react'
import { storyboardAPI } from '@/lib/api'
import type { Storyboard } from '@/types'
import { Badge } from '@/components/ui/badge'

interface SerialSceneGroupsProps {
  projectId: number
  episodeId?: number
}

interface SceneGroup {
  key: string
  storyboards: Storyboard[]
}

export function SerialSceneGroups({ projectId, episodeId }: SerialSceneGroupsProps) {
  const [expanded, setExpanded] = useState<Set<string>>(new Set())

  const { data } = useSWR(
    ['storyboards-serial', projectId, episodeId ?? 'all'],
    async () => {
      const params: { episode_id?: number } = {}
      if (episodeId) params.episode_id = episodeId
      const res = await storyboardAPI.listAll(projectId, params) as unknown as { data?: Storyboard[] }
      return res.data ?? []
    }
  )

  const storyboards: Storyboard[] = data ?? []

  // Only show storyboards that have a scene_group_key
  const groups = useMemo<SceneGroup[]>(() => {
    const withKey = storyboards.filter((s) => s.scene_group_key)
    if (withKey.length === 0) return []
    const map = new Map<string, Storyboard[]>()
    for (const sb of withKey) {
      const key = sb.scene_group_key!
      if (!map.has(key)) map.set(key, [])
      map.get(key)!.push(sb)
    }
    // Sort each group by sequence_number
    return Array.from(map.entries())
      .sort(([a], [b]) => a.localeCompare(b))
      .map(([key, items]) => ({
        key,
        storyboards: items.sort((a, b) => a.sequence_number - b.sequence_number),
      }))
  }, [storyboards])

  if (groups.length === 0) return null

  function toggleExpand(key: string) {
    setExpanded((prev) => {
      const next = new Set(prev)
      if (next.has(key)) next.delete(key)
      else next.add(key)
      return next
    })
  }

  return (
    <div className="space-y-3">
      <div className="flex items-center gap-2">
        <Film className="h-4 w-4 text-indigo-500" />
        <h3 className="text-sm font-semibold text-surface-800">场景分组（串行）</h3>
        <Badge variant="outline" className="text-[10px]">{groups.length} 个场景</Badge>
      </div>
      <p className="text-xs text-surface-400">
        以下分镜按场景分组，同一场景的分镜将串行生成，前一帧的末帧作为下一帧的起始图。
      </p>

      <div className="space-y-2">
        {groups.map((g) => {
          const isOpen = expanded.has(g.key)
          const firstWithFrame = g.storyboards.find((s) => s.end_frame_image_url)
          return (
            <div key={g.key} className="rounded-xl border border-surface-200 bg-white overflow-hidden">
              {/* Group header */}
              <div
                className="flex items-center gap-3 px-4 py-3 cursor-pointer hover:bg-surface-50 transition-colors"
                onClick={() => toggleExpand(g.key)}
              >
                {isOpen ? (
                  <ChevronDown className="h-4 w-4 text-surface-400 shrink-0" />
                ) : (
                  <ChevronRight className="h-4 w-4 text-surface-400 shrink-0" />
                )}
                <span className="flex-1 text-sm font-medium text-surface-700 truncate">
                  {g.key.replace(/_/g, ' ')}
                </span>
                <Badge variant="secondary" className="text-[10px] shrink-0">
                  {g.storyboards.length} 个分镜
                </Badge>
                {firstWithFrame && (
                  <Badge className="bg-emerald-50 text-emerald-700 border-emerald-200 text-[10px] shrink-0">
                    有末帧
                  </Badge>
                )}
              </div>

              {/* Storyboard strip */}
              {isOpen && (
                <div className="border-t border-surface-100 px-4 py-3 bg-surface-50">
                  <div className="flex items-center gap-2 overflow-x-auto pb-2">
                    {g.storyboards.map((sb, idx) => (
                      <div key={sb.id} className="flex items-center gap-2 shrink-0">
                        {/* Storyboard card */}
                        <div className="w-28 space-y-1">
                          <div className="relative aspect-video rounded-lg overflow-hidden bg-surface-100">
                            {sb.image_url ? (
                              <img src={sb.image_url} alt={`#${sb.sequence_number}`} className="h-full w-full object-cover" />
                            ) : (
                              <div className="flex h-full items-center justify-center">
                                <ImageIcon className="h-5 w-5 text-surface-300" />
                              </div>
                            )}
                            {sb.is_scene_first_clip && (
                              <span className="absolute left-1 top-1 rounded-sm bg-indigo-500 px-1 py-0.5 text-[9px] text-white font-medium">首帧</span>
                            )}
                          </div>
                          <div className="flex items-center justify-between">
                            <span className="text-[10px] text-surface-500">#{sb.sequence_number}</span>
                            {sb.end_frame_image_url && (
                              <span className="text-[9px] text-emerald-600 font-medium">末帧✓</span>
                            )}
                          </div>
                          {sb.end_frame_image_url && (
                            <div className="aspect-video w-full rounded overflow-hidden border border-emerald-200">
                              <img src={sb.end_frame_image_url} alt="末帧" className="h-full w-full object-cover" />
                            </div>
                          )}
                        </div>
                        {/* Arrow between clips */}
                        {idx < g.storyboards.length - 1 && (
                          <ArrowRight className="h-4 w-4 text-surface-300 shrink-0" />
                        )}
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </div>
          )
        })}
      </div>
    </div>
  )
}
