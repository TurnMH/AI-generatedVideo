'use client'

import { useState } from 'react'
import useSWR from 'swr'
import {
  Plus,
  ChevronDown,
  ChevronRight,
  Trash2,
  Edit,
  Users,
  X,
  Loader2,
} from 'lucide-react'
import { characterGroupAPI } from '@/lib/api'
import type { CharacterGroup, CharacterData } from '@/types'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Badge } from '@/components/ui/badge'
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { useToast } from '@/components/ui/toast'

interface CharacterGroupPanelProps {
  projectId: number
  characters: CharacterData[]
}

interface GroupForm {
  name: string
  description: string
}

const EMPTY_FORM: GroupForm = { name: '', description: '' }

export function CharacterGroupPanel({ projectId, characters }: CharacterGroupPanelProps) {
  const { toast } = useToast()
  const [expanded, setExpanded] = useState<Set<number>>(new Set())
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editTarget, setEditTarget] = useState<CharacterGroup | null>(null)
  const [form, setForm] = useState<GroupForm>(EMPTY_FORM)
  const [saving, setSaving] = useState(false)
  const [deleteTarget, setDeleteTarget] = useState<CharacterGroup | null>(null)
  const [deleting, setDeleting] = useState(false)

  // Assign variant dialog
  const [assignGroupId, setAssignGroupId] = useState<number | null>(null)
  const [assignAssetId, setAssignAssetId] = useState<string>('')
  const [assignVariantName, setAssignVariantName] = useState('')
  const [assigning, setAssigning] = useState(false)

  const { data: groupsRaw, mutate } = useSWR(
    ['character-groups', projectId],
    () => characterGroupAPI.list(projectId) as unknown as Promise<{ data: CharacterGroup[] }>
  )
  const groups: CharacterGroup[] = (groupsRaw as { data?: CharacterGroup[] })?.data ?? []

  function toggleExpand(id: number) {
    setExpanded((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  function openCreate() {
    setEditTarget(null)
    setForm(EMPTY_FORM)
    setDialogOpen(true)
  }

  function openEdit(g: CharacterGroup) {
    setEditTarget(g)
    setForm({ name: g.name, description: g.description ?? '' })
    setDialogOpen(true)
  }

  async function handleSave() {
    if (!form.name.trim()) {
      toast({ title: '请输入组名', variant: 'destructive' })
      return
    }
    setSaving(true)
    try {
      if (editTarget) {
        await characterGroupAPI.update(projectId, editTarget.id, { name: form.name.trim(), description: form.description.trim() })
        toast({ title: '角色组已更新' })
      } else {
        await characterGroupAPI.create(projectId, { name: form.name.trim(), description: form.description.trim() })
        toast({ title: '角色组已创建' })
      }
      setDialogOpen(false)
      mutate()
    } catch {
      toast({ title: '保存失败', variant: 'destructive' })
    } finally {
      setSaving(false)
    }
  }

  async function handleDelete() {
    if (!deleteTarget) return
    setDeleting(true)
    try {
      await characterGroupAPI.delete(projectId, deleteTarget.id)
      toast({ title: '角色组已删除' })
      setDeleteTarget(null)
      mutate()
    } catch {
      toast({ title: '删除失败', variant: 'destructive' })
    } finally {
      setDeleting(false)
    }
  }

  async function handleAssign() {
    if (!assignGroupId || !assignAssetId) return
    setAssigning(true)
    try {
      await characterGroupAPI.assignVariant(projectId, assignGroupId, Number(assignAssetId), assignVariantName)
      toast({ title: '已添加变体' })
      setAssignGroupId(null)
      setAssignAssetId('')
      setAssignVariantName('')
      mutate()
    } catch {
      toast({ title: '添加失败', variant: 'destructive' })
    } finally {
      setAssigning(false)
    }
  }

  async function handleRemoveVariant(groupId: number, assetId: number) {
    try {
      await characterGroupAPI.removeVariant(projectId, groupId, assetId)
      toast({ title: '已移除变体' })
      mutate()
    } catch {
      toast({ title: '移除失败', variant: 'destructive' })
    }
  }

  return (
    <div className="space-y-4">
      {/* Section header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <Users className="h-4 w-4 text-indigo-500" />
          <h3 className="text-sm font-semibold text-surface-800">角色组（串行生成）</h3>
          <Badge variant="outline" className="text-[10px]">{groups.length}</Badge>
        </div>
        <Button size="sm" variant="outline" onClick={openCreate}>
          <Plus className="mr-1.5 h-3.5 w-3.5" />
          新建角色组
        </Button>
      </div>

      {/* Groups list */}
      {groups.length === 0 ? (
        <div className="rounded-xl border border-dashed border-surface-200 py-8 text-center">
          <Users className="mx-auto h-8 w-8 text-surface-300" />
          <p className="mt-2 text-sm text-surface-400">还没有角色组</p>
          <p className="mt-1 text-xs text-surface-400">新建角色组后，可将角色变体关联到组内，用于串行视频生成</p>
        </div>
      ) : (
        <div className="space-y-2">
          {groups.map((g) => {
            const isOpen = expanded.has(g.id)
            return (
              <div key={g.id} className="rounded-xl border border-surface-200 bg-white overflow-hidden">
                {/* Group row */}
                <div
                  className="flex items-center gap-3 px-4 py-3 cursor-pointer hover:bg-surface-50 transition-colors"
                  onClick={() => toggleExpand(g.id)}
                >
                  {isOpen ? (
                    <ChevronDown className="h-4 w-4 text-surface-400 shrink-0" />
                  ) : (
                    <ChevronRight className="h-4 w-4 text-surface-400 shrink-0" />
                  )}
                  <span className="flex-1 text-sm font-medium text-surface-800">{g.name}</span>
                  {g.description && (
                    <span className="text-xs text-surface-400 max-w-[200px] truncate hidden sm:block">
                      {g.description}
                    </span>
                  )}
                  <Badge variant="secondary" className="text-[10px]">
                    {g.variants?.length ?? 0} 个变体
                  </Badge>
                  <div className="flex items-center gap-1" onClick={(e) => e.stopPropagation()}>
                    <Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => openEdit(g)}>
                      <Edit className="h-3.5 w-3.5" />
                    </Button>
                    <Button variant="ghost" size="icon" className="h-7 w-7 text-red-500 hover:text-red-600" onClick={() => setDeleteTarget(g)}>
                      <Trash2 className="h-3.5 w-3.5" />
                    </Button>
                  </div>
                </div>

                {/* Variants */}
                {isOpen && (
                  <div className="border-t border-surface-100 px-4 py-3 bg-surface-50 space-y-2">
                    {(g.variants ?? []).length === 0 ? (
                      <p className="text-xs text-surface-400">暂无变体，点击「添加变体」关联角色</p>
                    ) : (
                      <div className="flex flex-wrap gap-2">
                        {(g.variants ?? []).map((v) => (
                          <div
                            key={v.id}
                            className="flex items-center gap-1.5 rounded-lg border border-surface-200 bg-white px-2.5 py-1.5 text-xs"
                          >
                            {v.reference_image_url && (
                              <img src={v.reference_image_url} alt={v.name} className="h-6 w-6 rounded-md object-cover" />
                            )}
                            <span className="text-surface-700">{v.name}</span>
                            <button
                              className="ml-0.5 text-surface-400 hover:text-red-500"
                              onClick={() => handleRemoveVariant(g.id, v.id)}
                            >
                              <X className="h-3 w-3" />
                            </button>
                          </div>
                        ))}
                      </div>
                    )}
                    <Button
                      size="sm"
                      variant="outline"
                      className="text-xs"
                      onClick={() => { setAssignGroupId(g.id); setAssignAssetId(''); setAssignVariantName('') }}
                    >
                      <Plus className="mr-1 h-3 w-3" />
                      添加变体
                    </Button>
                  </div>
                )}
              </div>
            )
          })}
        </div>
      )}

      {/* Create / Edit dialog */}
      <Dialog open={dialogOpen} onOpenChange={(o) => !o && setDialogOpen(false)}>
        <DialogContent className="max-w-sm">
          <DialogHeader>
            <DialogTitle>{editTarget ? '编辑角色组' : '新建角色组'}</DialogTitle>
          </DialogHeader>
          <div className="space-y-4 pt-2">
            <div className="space-y-1.5">
              <Label>组名 *</Label>
              <Input
                placeholder="如：主角场景A"
                value={form.name}
                onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
              />
            </div>
            <div className="space-y-1.5">
              <Label>描述（可选）</Label>
              <Textarea
                placeholder="用于串行生成时的备注"
                rows={2}
                value={form.description}
                onChange={(e) => setForm((f) => ({ ...f, description: e.target.value }))}
              />
            </div>
            <div className="flex gap-2 pt-1">
              <Button variant="outline" className="flex-1" onClick={() => setDialogOpen(false)} disabled={saving}>取消</Button>
              <Button className="flex-1" onClick={handleSave} disabled={saving}>
                {saving && <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />}
                {editTarget ? '保存' : '创建'}
              </Button>
            </div>
          </div>
        </DialogContent>
      </Dialog>

      {/* Assign variant dialog */}
      <Dialog open={assignGroupId !== null} onOpenChange={(o) => !o && setAssignGroupId(null)}>
        <DialogContent className="max-w-sm">
          <DialogHeader>
            <DialogTitle>添加变体角色</DialogTitle>
          </DialogHeader>
          <div className="space-y-4 pt-2">
            <div className="space-y-1.5">
              <Label>选择角色 *</Label>
              <Select value={assignAssetId} onValueChange={setAssignAssetId}>
                <SelectTrigger>
                  <SelectValue placeholder="请选择角色" />
                </SelectTrigger>
                <SelectContent>
                  {characters.map((c) => (
                    <SelectItem key={c.id} value={String(c.id)}>{c.name}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-1.5">
              <Label>变体名称（可选）</Label>
              <Input
                placeholder="如：正装版、战斗版"
                value={assignVariantName}
                onChange={(e) => setAssignVariantName(e.target.value)}
              />
            </div>
            <div className="flex gap-2 pt-1">
              <Button variant="outline" className="flex-1" onClick={() => setAssignGroupId(null)} disabled={assigning}>取消</Button>
              <Button className="flex-1" onClick={handleAssign} disabled={assigning || !assignAssetId}>
                {assigning && <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />}
                添加
              </Button>
            </div>
          </div>
        </DialogContent>
      </Dialog>

      {/* Delete confirm */}
      <AlertDialog open={!!deleteTarget} onOpenChange={(o) => !o && setDeleteTarget(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>删除角色组</AlertDialogTitle>
            <AlertDialogDescription>
              确定要删除「{deleteTarget?.name}」吗？组内变体关联将一并清除，但角色本身不会被删除。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deleting}>取消</AlertDialogCancel>
            <AlertDialogAction className="bg-red-600 hover:bg-red-700" onClick={handleDelete} disabled={deleting}>
              {deleting && <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />}
              删除
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
