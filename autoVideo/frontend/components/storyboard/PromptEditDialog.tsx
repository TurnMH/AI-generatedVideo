'use client'
import { useState, useEffect } from 'react'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { sceneAPI } from '@/lib/api'
import type { Scene } from '@/types'
import { LoadingSpinner } from '@/components/common/LoadingSpinner'

interface PromptEditDialogProps {
  scene: Scene | null
  onClose: () => void
  onSave: (sceneId: number, newPrompt: string) => void
}

export function PromptEditDialog({ scene, onClose, onSave }: PromptEditDialogProps) {
  const [prompt, setPrompt] = useState('')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    if (scene) {
      setPrompt(scene.prompt_draft)
      setError('')
    }
  }, [scene])

  const handleSave = async () => {
    if (!scene) return
    setSaving(true)
    setError('')
    try {
      await sceneAPI.update(scene.script_id, scene.id, { prompt_draft: prompt })
      onSave(scene.id, prompt)
      onClose()
    } catch {
      setError('保存失败，请重试')
    } finally {
      setSaving(false)
    }
  }

  return (
    <Dialog open={!!scene} onOpenChange={() => onClose()}>
      <DialogContent className="max-w-xl">
        <DialogHeader>
          <DialogTitle>
            编辑 Prompt — 场景 #{scene?.scene_order}
          </DialogTitle>
        </DialogHeader>

        <div className="space-y-3 py-2">
          {scene && (
            <p className="text-xs text-surface-500">{scene.description}</p>
          )}
          <textarea
            value={prompt}
            onChange={(e) => setPrompt(e.target.value)}
            rows={8}
            placeholder="输入生成 prompt..."
            className="w-full rounded-md border border-surface-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary-500"
          />
          {error && <p className="text-xs text-red-600">{error}</p>}
        </div>

        <DialogFooter className="gap-2">
          <Button variant="outline" onClick={onClose} disabled={saving} title="放弃编辑">
            取消
          </Button>
          <Button onClick={handleSave} disabled={saving || !prompt.trim()} title="保存 Prompt 改动">
            {saving ? <LoadingSpinner size="sm" /> : '保存'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
