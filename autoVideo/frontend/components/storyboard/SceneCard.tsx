'use client'
import Image from 'next/image'
import { Edit2, RefreshCw, ZoomIn } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import type { Scene } from '@/types'

interface SceneCardProps {
  scene: Scene
  isSelected?: boolean
  onSelect?: (id: number) => void
  onEditPrompt?: (scene: Scene) => void
  onRegenerate?: (scene: Scene) => void
  onPreview?: (scene: Scene) => void
}

const emotionVariant: Record<
  string,
  'default' | 'secondary' | 'success' | 'destructive' | 'warning' | 'outline'
> = {
  happy: 'success',
  sad: 'secondary',
  angry: 'destructive',
  excited: 'warning',
  calm: 'outline',
  tense: 'default',
}

export function SceneCard({
  scene,
  isSelected,
  onSelect,
  onEditPrompt,
  onRegenerate,
  onPreview,
}: SceneCardProps) {
  return (
    <div
      className={`group relative flex flex-col overflow-hidden rounded-lg border bg-white shadow-sm transition-all hover:shadow-md ${
        isSelected ? 'ring-2 ring-primary-500 ring-offset-1' : 'border-surface-200'
      }`}
    >
      {/* Select checkbox */}
      <div
        className="absolute left-2 top-2 z-10"
        onClick={(e) => {
          e.stopPropagation()
          onSelect?.(scene.id)
        }}
      >
        <input
          type="checkbox"
          checked={isSelected}
          onChange={() => onSelect?.(scene.id)}
          className="h-4 w-4 cursor-pointer rounded border-surface-300 accent-blue-600"
        />
      </div>

      {/* Scene number */}
      <div className="absolute right-2 top-2 z-10">
        <span className="rounded bg-black/50 px-1.5 py-0.5 text-xs font-medium text-white">
          #{scene.scene_order}
        </span>
      </div>

      {/* Image */}
      <div className="relative aspect-video w-full overflow-hidden bg-surface-100">
        {scene.image_url ? (
          <Image
            src={scene.image_url}
            alt={`Scene ${scene.scene_order}`}
            fill
            className="object-cover"
          />
        ) : (
          <div className="flex h-full items-center justify-center">
            <div className="text-center text-surface-300">
              <div className="mb-1 text-3xl">🎬</div>
              <p className="text-xs">待生成</p>
            </div>
          </div>
        )}

        {/* Action overlay */}
        <div className="absolute inset-0 flex items-center justify-center gap-2 bg-black/40 opacity-0 transition-opacity group-hover:opacity-100">
          <button
            onClick={(e) => {
              e.stopPropagation()
              onEditPrompt?.(scene)
            }}
            className="rounded-md bg-white/90 p-1.5 text-surface-700 hover:bg-white"
            title="编辑 Prompt"
          >
            <Edit2 className="h-4 w-4" />
          </button>
          <button
            onClick={(e) => {
              e.stopPropagation()
              onRegenerate?.(scene)
            }}
            className="rounded-md bg-white/90 p-1.5 text-surface-700 hover:bg-white"
            title="重新生成"
          >
            <RefreshCw className="h-4 w-4" />
          </button>
          {scene.image_url && (
            <button
              onClick={(e) => {
                e.stopPropagation()
                onPreview?.(scene)
              }}
              className="rounded-md bg-white/90 p-1.5 text-surface-700 hover:bg-white"
              title="预览大图"
            >
              <ZoomIn className="h-4 w-4" />
            </button>
          )}
        </div>
      </div>

      {/* Content */}
      <div className="flex flex-1 flex-col gap-2 p-3">
        {/* Emotion + setting */}
        <div className="flex items-center gap-2">
          <Badge variant={emotionVariant[scene.emotion] ?? 'secondary'}>
            {scene.emotion}
          </Badge>
          <span className="truncate text-xs text-surface-400">{scene.setting}</span>
        </div>

        {/* Description */}
        <p className="line-clamp-2 text-xs text-surface-700">{scene.description}</p>

        {/* Characters */}
        {scene.characters.length > 0 && (
          <div className="flex flex-wrap gap-1">
            {scene.characters.map((c) => (
              <span
                key={c}
                className="rounded bg-surface-100 px-1.5 py-0.5 text-xs text-surface-600"
              >
                {c}
              </span>
            ))}
          </div>
        )}

        {/* Prompt */}
        <p className="line-clamp-1 text-xs italic text-surface-400">{scene.prompt_draft}</p>
      </div>
    </div>
  )
}
