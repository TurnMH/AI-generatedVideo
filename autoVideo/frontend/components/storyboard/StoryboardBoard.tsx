'use client'
import { useState } from 'react'
import {
  DndContext,
  closestCenter,
  KeyboardSensor,
  PointerSensor,
  useSensor,
  useSensors,
  type DragEndEvent,
} from '@dnd-kit/core'
import {
  arrayMove,
  SortableContext,
  sortableKeyboardCoordinates,
  rectSortingStrategy,
  useSortable,
} from '@dnd-kit/sortable'
import { CSS } from '@dnd-kit/utilities'
import type { Scene } from '@/types'
import { SceneCard } from './SceneCard'
import { PromptEditDialog } from './PromptEditDialog'
import { Button } from '@/components/ui/button'
import { sceneAPI } from '@/lib/api'

interface StoryboardBoardProps {
  scenes: Scene[]
  episodeId: number
  onScenesChange: (scenes: Scene[]) => void
}

function SortableScene({
  scene,
  isSelected,
  onSelect,
  onEditPrompt,
  onRegenerate,
  onPreview,
}: {
  scene: Scene
  isSelected: boolean
  onSelect: (id: number) => void
  onEditPrompt: (scene: Scene) => void
  onRegenerate: (scene: Scene) => void
  onPreview: (scene: Scene) => void
}) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } =
    useSortable({ id: scene.id })

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.5 : 1,
    zIndex: isDragging ? 10 : undefined,
  }

  return (
    <div ref={setNodeRef} style={style} {...attributes} {...listeners}>
      <SceneCard
        scene={scene}
        isSelected={isSelected}
        onSelect={onSelect}
        onEditPrompt={onEditPrompt}
        onRegenerate={onRegenerate}
        onPreview={onPreview}
      />
    </div>
  )
}

export function StoryboardBoard({
  scenes: initialScenes,
  episodeId,
  onScenesChange,
}: StoryboardBoardProps) {
  const [scenes, setScenes] = useState<Scene[]>(initialScenes)
  const [selected, setSelected] = useState<Set<number>>(new Set())
  const [editingScene, setEditingScene] = useState<Scene | null>(null)
  const [previewScene, setPreviewScene] = useState<Scene | null>(null)
  const [batchGenerating, setBatchGenerating] = useState(false)

  const sensors = useSensors(
    useSensor(PointerSensor),
    useSensor(KeyboardSensor, {
      coordinateGetter: sortableKeyboardCoordinates,
    })
  )

  const handleDragEnd = async (event: DragEndEvent) => {
    const { active, over } = event
    if (!over || active.id === over.id) return

    const oldIndex = scenes.findIndex((s) => s.id === active.id)
    const newIndex = scenes.findIndex((s) => s.id === over.id)
    const reordered = arrayMove(scenes, oldIndex, newIndex).map((s, i) => ({
      ...s,
      scene_order: i + 1,
    }))
    setScenes(reordered)
    onScenesChange(reordered)

    try {
      await sceneAPI.reorder(
        episodeId,
        reordered.map((s) => ({ id: s.id, scene_order: s.scene_order }))
      )
    } catch {
      // revert on error
      setScenes(initialScenes)
    }
  }

  const toggleSelect = (id: number) => {
    setSelected((prev) => {
      const next = new Set(prev)
      next.has(id) ? next.delete(id) : next.add(id)
      return next
    })
  }

  const handleBatchGenerate = async () => {
    setBatchGenerating(true)
    try {
      await sceneAPI.batchGenerate(Array.from(selected))
      setSelected(new Set())
    } catch {
      // ignore
    } finally {
      setBatchGenerating(false)
    }
  }

  const handleRegenerate = async (scene: Scene) => {
    try {
      await sceneAPI.generateImage(scene.id)
    } catch {
      // ignore
    }
  }

  const handlePromptSave = (sceneId: number, newPrompt: string) => {
    setScenes((prev) =>
      prev.map((s) => (s.id === sceneId ? { ...s, prompt_draft: newPrompt } : s))
    )
  }

  return (
    <div className="space-y-4">
      {/* Batch action bar */}
      {selected.size > 0 && (
        <div className="flex items-center gap-3 rounded-lg border border-primary-200 bg-primary-50 px-4 py-2">
          <span className="text-sm text-primary-700">已选 {selected.size} 个场景</span>
          <Button size="sm" onClick={handleBatchGenerate} disabled={batchGenerating} title="批量生成已选场景的图像">
            {batchGenerating ? '生成中…' : '批量生成'}
          </Button>
          <Button size="sm" variant="outline" onClick={() => setSelected(new Set())} title="取消选择所有场景">
            取消选择
          </Button>
        </div>
      )}

      {/* DnD grid */}
      <DndContext
        sensors={sensors}
        collisionDetection={closestCenter}
        onDragEnd={handleDragEnd}
      >
        <SortableContext
          items={scenes.map((s) => s.id)}
          strategy={rectSortingStrategy}
        >
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
            {scenes.map((scene) => (
              <SortableScene
                key={scene.id}
                scene={scene}
                isSelected={selected.has(scene.id)}
                onSelect={toggleSelect}
                onEditPrompt={setEditingScene}
                onRegenerate={handleRegenerate}
                onPreview={setPreviewScene}
              />
            ))}
          </div>
        </SortableContext>
      </DndContext>

      {/* Prompt edit dialog */}
      <PromptEditDialog
        scene={editingScene}
        onClose={() => setEditingScene(null)}
        onSave={handlePromptSave}
      />

      {/* Image preview dialog */}
      {previewScene?.image_url && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/80"
          onClick={() => setPreviewScene(null)}
        >
          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img
            src={previewScene.image_url}
            alt="preview"
            className="max-h-[90vh] max-w-[90vw] rounded-lg object-contain"
          />
        </div>
      )}
    </div>
  )
}
