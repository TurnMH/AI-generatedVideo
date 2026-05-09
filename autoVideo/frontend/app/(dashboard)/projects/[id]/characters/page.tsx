'use client'

import React, { useState } from 'react'
import { useParams, useRouter } from 'next/navigation'
import useSWR from 'swr'
import {
  ArrowLeft,
  Plus,
  Sparkles,
  Users,
  User,
  Edit,
  Trash2,
  ImageIcon,
  Loader2,
} from 'lucide-react'
import { projectAPI, characterAPI } from '@/lib/api'
import type { Project, CharacterData } from '@/types'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent } from '@/components/ui/card'
import { ZoomableImage } from '@/components/ui/image-lightbox'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
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
import { useToast } from '@/components/ui/toast'

import { STYLE_PRESETS, STYLE_BADGE_CLASS, EMPTY_CHARACTER_FORM as EMPTY_FORM, type CharacterFormState as FormState } from '@/lib/projects/characters/form'
import { CharacterSkeleton } from '@/components/projects/characters/CharacterSkeleton'
import { CharacterGroupPanel } from '@/components/projects/characters/CharacterGroupPanel'

export default function CharactersPage() {
  const router = useRouter()
  const params = useParams()
  const projectId = Number(params.id)
  const { toast } = useToast()

  const [dialogOpen, setDialogOpen] = useState(false)
  const [editTarget, setEditTarget] = useState<CharacterData | null>(null)
  const [form, setForm] = useState<FormState>(EMPTY_FORM)
  const [saving, setSaving] = useState(false)
  const [deleteTarget, setDeleteTarget] = useState<CharacterData | null>(null)
  const [deleting, setDeleting] = useState(false)

  const { data: projectRaw } = useSWR(
    ['project', projectId],
    () => projectAPI.get(projectId) as unknown as Promise<{ data: Project }>
  )
  const project = (projectRaw as { data?: Project })?.data

  const { data: charsRaw, mutate: mutateChars, isLoading } = useSWR(
    ['characters', projectId],
    () => characterAPI.list(projectId) as unknown as Promise<{ data: CharacterData[] }>
  )
  const characters: CharacterData[] = (charsRaw as { data?: CharacterData[] })?.data ?? []

  function openCreate() {
    setEditTarget(null)
    setForm(EMPTY_FORM)
    setDialogOpen(true)
  }

  function openEdit(ch: CharacterData) {
    setEditTarget(ch)
    setForm({
      name: ch.name,
      role_desc: ch.role_desc ?? '',
      appearance_desc: ch.appearance_desc ?? '',
      reference_image_url: ch.reference_image_url ?? '',
      style_preset: ch.style_preset ?? '',
    })
    setDialogOpen(true)
  }

  async function handleSave() {
    if (!form.name.trim()) {
      toast({ title: '请输入角色名字', variant: 'destructive' })
      return
    }
    setSaving(true)
    try {
      if (editTarget) {
        await characterAPI.update(projectId, editTarget.id, form)
        toast({ title: '角色已更新' })
      } else {
        await characterAPI.create(projectId, form)
        toast({ title: '角色已创建' })
      }
      setDialogOpen(false)
      mutateChars()
    } catch {
      toast({ title: editTarget ? '更新失败' : '创建失败', variant: 'destructive' })
    } finally {
      setSaving(false)
    }
  }

  async function handleDelete() {
    if (!deleteTarget) return
    setDeleting(true)
    try {
      await characterAPI.delete(projectId, deleteTarget.id)
      toast({ title: '角色已删除' })
      setDeleteTarget(null)
      mutateChars()
    } catch {
      toast({ title: '删除失败', variant: 'destructive' })
    } finally {
      setDeleting(false)
    }
  }

  return (
    <div className="space-y-6">
      {/* Dark gradient header */}
      <div className="overflow-hidden rounded-[28px] border border-surface-200/70 bg-gradient-to-br from-slate-950 via-violet-950 to-slate-900 p-6 text-white shadow-sm">
        <div className="flex flex-col gap-6 lg:flex-row lg:items-start lg:justify-between">
          <div className="max-w-2xl">
            <div className="mb-4 inline-flex items-center gap-2 rounded-full border border-white/10 bg-white/10 px-3 py-1.5 text-xs font-medium text-surface-100 backdrop-blur">
              <Sparkles className="h-3.5 w-3.5 text-primary-300" />
              角色设定工作区
            </div>
            <div className="flex items-start gap-4">
              <Button
                variant="ghost"
                size="icon"
                className="mt-0.5 shrink-0 rounded-2xl border border-white/10 bg-white/10 text-white hover:bg-white/15 hover:text-white"
                onClick={() => router.push(`/video/${projectId}`)}
              >
                <ArrowLeft className="h-4 w-4" />
              </Button>
              <div>
                <h2 className="text-2xl font-semibold tracking-tight text-white">角色管理</h2>
                <p className="mt-2 text-sm leading-6 text-surface-300">
                  {project?.title
                    ? `${project.title} · 管理项目中的角色设定与形象参考。`
                    : '管理项目中的角色设定与参考图。'}
                </p>
              </div>
            </div>
          </div>

          <div className="grid gap-3 sm:grid-cols-2">
            <div className="rounded-2xl border border-white/10 bg-white/10 p-4 backdrop-blur">
              <div className="flex items-center gap-2 text-surface-300">
                <Users className="h-4 w-4 text-cyan-300" />
                <span className="text-xs uppercase tracking-[0.2em]">角色总数</span>
              </div>
              <p className="mt-3 text-2xl font-semibold text-white">
                {isLoading ? '…' : characters.length}
              </p>
              <p className="mt-1 text-xs text-surface-400">已创建角色卡</p>
            </div>
            <div className="rounded-2xl border border-white/10 bg-white/10 p-4 backdrop-blur">
              <div className="flex items-center gap-2 text-surface-300">
                <ImageIcon className="h-4 w-4 text-violet-300" />
                <span className="text-xs uppercase tracking-[0.2em]">参考图</span>
              </div>
              <p className="mt-3 text-2xl font-semibold text-white">
                {isLoading ? '…' : characters.filter((c) => c.reference_image_url).length}
              </p>
              <p className="mt-1 text-xs text-surface-400">已设置参考图</p>
            </div>
          </div>
        </div>
      </div>

      {/* Content area */}
      <div className="rounded-[28px] border border-surface-200 bg-gradient-to-br from-white to-surface-50 p-4 shadow-sm">
        <div className="mb-4 flex items-center justify-between px-2">
          <h3 className="text-sm font-semibold text-surface-700">
            {isLoading ? '加载中…' : `${characters.length} 个角色`}
          </h3>
          <Button size="sm" onClick={openCreate} className="gap-1.5">
            <Plus className="h-3.5 w-3.5" />
            新建角色
          </Button>
        </div>

        {isLoading ? (
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {Array.from({ length: 3 }).map((_, i) => (
              <CharacterSkeleton key={i} />
            ))}
          </div>
        ) : characters.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-16 text-center">
            <div className="mb-4 flex h-16 w-16 items-center justify-center rounded-full bg-surface-100">
              <Users className="h-8 w-8 text-surface-400" />
            </div>
            <h3 className="mb-1 text-base font-semibold text-surface-700">暂无角色</h3>
            <p className="mb-4 text-sm text-surface-400">点击「新建角色」添加第一个角色</p>
            <Button size="sm" onClick={openCreate} className="gap-1.5">
              <Plus className="h-3.5 w-3.5" />
              新建角色
            </Button>
          </div>
        ) : (
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {characters.map((ch) => (
              <Card
                key={ch.id}
                className="group relative overflow-hidden rounded-2xl border border-surface-200 bg-white shadow-none transition-shadow hover:shadow-md"
              >
                <CardContent className="p-4">
                  <div className="flex items-start gap-3">
                    {ch.reference_image_url ? (
                      <ZoomableImage
                        src={ch.reference_image_url}
                        alt={ch.name}
                        className="h-12 w-12 shrink-0 rounded-full object-cover"
                      />
                    ) : (
                      <div className="flex h-12 w-12 shrink-0 items-center justify-center rounded-full bg-gradient-to-br from-violet-100 to-indigo-100">
                        <User className="h-6 w-6 text-violet-500" />
                      </div>
                    )}
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center gap-2">
                        <h4 className="truncate text-sm font-semibold text-surface-800">{ch.name}</h4>
                        {ch.style_preset && (
                          <Badge
                            className={`shrink-0 px-1.5 py-0 text-[10px] ${STYLE_BADGE_CLASS[ch.style_preset] ?? 'bg-surface-100 text-surface-600'}`}
                          >
                            {STYLE_PRESETS.find((s) => s.value === ch.style_preset)?.label ?? ch.style_preset}
                          </Badge>
                        )}
                      </div>
                      {ch.role_desc && (
                        <p className="mt-0.5 line-clamp-1 text-xs text-surface-500">{ch.role_desc}</p>
                      )}
                    </div>
                  </div>
                  {ch.appearance_desc && (
                    <p className="mt-3 line-clamp-2 text-xs leading-5 text-surface-500">
                      {ch.appearance_desc}
                    </p>
                  )}
                  <div className="mt-3 flex gap-2 opacity-0 transition-opacity group-hover:opacity-100">
                    <Button
                      size="sm"
                      variant="outline"
                      className="h-7 flex-1 gap-1 rounded-lg text-xs"
                      onClick={() => openEdit(ch)}
                    >
                      <Edit className="h-3 w-3" />
                      编辑
                    </Button>
                    <Button
                      size="sm"
                      variant="outline"
                      className="h-7 flex-1 gap-1 rounded-lg text-xs text-red-600 hover:border-red-300 hover:text-red-700"
                      onClick={() => setDeleteTarget(ch)}
                    >
                      <Trash2 className="h-3 w-3" />
                      删除
                    </Button>
                  </div>
                </CardContent>
              </Card>
            ))}
          </div>
        )}
      </div>

      {/* Character Groups (for serial video) */}
      <div className="rounded-[20px] border border-surface-200 bg-white p-5">
        <CharacterGroupPanel projectId={projectId} characters={characters} />
      </div>

      {/* Create / Edit dialog */}
      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{editTarget ? '编辑角色' : '新建角色'}</DialogTitle>
          </DialogHeader>
          <div className="space-y-4 pt-2">
            <div className="space-y-1.5">
              <Label htmlFor="char-name">
                名字 <span className="text-red-500">*</span>
              </Label>
              <Input
                id="char-name"
                placeholder="请输入角色名字"
                value={form.name}
                onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="char-role">角色简介</Label>
              <Textarea
                id="char-role"
                placeholder="角色的身份、性格和背景…"
                rows={2}
                value={form.role_desc}
                onChange={(e) => setForm((f) => ({ ...f, role_desc: e.target.value }))}
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="char-appearance">外貌描述</Label>
              <Textarea
                id="char-appearance"
                placeholder="外貌特征、服装、发型等…"
                rows={2}
                value={form.appearance_desc}
                onChange={(e) => setForm((f) => ({ ...f, appearance_desc: e.target.value }))}
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="char-ref">参考图 URL</Label>
              <Input
                id="char-ref"
                placeholder="https://..."
                value={form.reference_image_url}
                onChange={(e) => setForm((f) => ({ ...f, reference_image_url: e.target.value }))}
              />
            </div>
            <div className="space-y-1.5">
              <Label>风格预设</Label>
              <Select
                value={form.style_preset}
                onValueChange={(v) => setForm((f) => ({ ...f, style_preset: v }))}
              >
                <SelectTrigger>
                  <SelectValue placeholder="选择风格…" />
                </SelectTrigger>
                <SelectContent>
                  {STYLE_PRESETS.map((s) => (
                    <SelectItem key={s.value} value={s.value}>
                      {s.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className="flex gap-2 pt-2">
              <Button
                variant="outline"
                className="flex-1"
                onClick={() => setDialogOpen(false)}
                disabled={saving}
              >
                取消
              </Button>
              <Button className="flex-1" onClick={handleSave} disabled={saving}>
                {saving && <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />}
                {editTarget ? '保存' : '创建'}
              </Button>
            </div>
          </div>
        </DialogContent>
      </Dialog>

      {/* Delete confirm dialog */}
      <AlertDialog open={!!deleteTarget} onOpenChange={(o) => !o && setDeleteTarget(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>删除角色</AlertDialogTitle>
            <AlertDialogDescription>
              确定要删除「{deleteTarget?.name}」吗？此操作不可恢复。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deleting}>取消</AlertDialogCancel>
            <AlertDialogAction
              className="bg-red-600 hover:bg-red-700"
              onClick={handleDelete}
              disabled={deleting}
            >
              {deleting && <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />}
              删除
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
