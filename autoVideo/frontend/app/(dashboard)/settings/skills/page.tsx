'use client'

import { useState } from 'react'
import useSWR from 'swr'
import { skillAPI } from '@/lib/api'
import type { Skill } from '@/types'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent } from '@/components/ui/card'
import {
  Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle
} from '@/components/ui/dialog'
import {
  AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent,
  AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle, AlertDialogTrigger
} from '@/components/ui/alert-dialog'
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue
} from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import { useToast } from '@/components/ui/toast'
import { Plus, Trash2, Pencil, Sword, Compass, MessageCircle, Star, Search, RefreshCw, RotateCcw } from 'lucide-react'
import { DashboardHero } from '@/components/layout/dashboard-hero'
import { DashboardEmptyState } from '@/components/layout/dashboard-empty-state'

// project_id = 0 means global/template skills (not project-specific)
const GLOBAL_PROJECT_ID = 0

const SKILL_TYPES = [
  { value: 'combat',      label: '战斗',    color: 'bg-red-100 text-red-700',      icon: Sword },
  { value: 'exploration', label: '探索',    color: 'bg-green-100 text-green-700',  icon: Compass },
  { value: 'social',      label: '对话',    color: 'bg-blue-100 text-blue-700',    icon: MessageCircle },
  { value: 'special',     label: '特技',    color: 'bg-purple-100 text-purple-700',icon: Star },
]

const USE_CASES = [
  { value: 'storyboard',      label: '分镜生成' },
  { value: 'storyboard_prep', label: '分镜预处理' },
  { value: 'extraction',      label: '资源提取' },
  { value: 'prompt',          label: '提示词优化' },
  { value: 'writing',         label: '剧本写作' },
]

function SkillTypeBadge({ type }: { type: string }) {
  const t = SKILL_TYPES.find(t => t.value === type)
  if (!t) return <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-gray-100 text-gray-700">{type || '未分类'}</span>
  const Icon = t.icon
  return (
    <span className={`inline-flex items-center gap-1 px-2.5 py-0.5 rounded-full text-xs font-medium ${t.color}`}>
      <Icon className="h-3 w-3" />{t.label}
    </span>
  )
}

function UseCaseBadge({ useCase }: { useCase: string }) {
  const u = USE_CASES.find(u => u.value === useCase)
  return (
    <span className="inline-flex items-center px-2 py-0.5 rounded text-xs bg-surface-100 text-surface-700 border border-border/50">
      {u?.label || useCase || '通用'}
    </span>
  )
}

interface FormState {
  name: string
  skill_type: string
  use_case: string
  description: string
  is_active: boolean
}

const EMPTY_FORM: FormState = {
  name: '', skill_type: 'combat', use_case: 'storyboard',
  description: '', is_active: true,
}

export default function SkillsPage() {
  const { toast } = useToast()

  const [filterType, setFilterType] = useState('all')
  const [filterUseCase, setFilterUseCase] = useState('all')
  const [search, setSearch] = useState('')

  // Always load global skills (project_id = 0)
  const { data: skillsData, mutate } = useSWR(
    ['skills', GLOBAL_PROJECT_ID, filterType, filterUseCase],
    () => skillAPI.list(GLOBAL_PROJECT_ID, filterType === 'all' ? undefined : filterType, filterUseCase === 'all' ? undefined : filterUseCase)
  )
  const skills: Skill[] = skillsData?.data ?? []

  const [dialogOpen, setDialogOpen] = useState(false)
  const [editingSkill, setEditingSkill] = useState<Skill | null>(null)
  const [form, setForm] = useState<FormState>(EMPTY_FORM)
  const [saving, setSaving] = useState(false)

  function openCreate() {
    setEditingSkill(null)
    setForm(EMPTY_FORM)
    setDialogOpen(true)
  }

  function openEdit(skill: Skill) {
    setEditingSkill(skill)
    setForm({
      name: skill.name,
      skill_type: skill.skill_type,
      use_case: skill.use_case,
      description: skill.description,
      is_active: skill.is_active,
    })
    setDialogOpen(true)
  }

  async function handleSave() {
    if (!form.name.trim()) return
    setSaving(true)
    try {
      const payload = {
        project_id: GLOBAL_PROJECT_ID,
        name: form.name.trim(),
        skill_type: form.skill_type,
        use_case: form.use_case,
        description: form.description,
        is_active: form.is_active,
      }
      if (editingSkill) {
        await skillAPI.update(editingSkill.id, payload)
        toast({ title: 'Skill 已更新', variant: 'success' })
      } else {
        await skillAPI.create(payload)
        toast({ title: 'Skill 已创建', variant: 'success' })
      }
      mutate()
      setDialogOpen(false)
    } catch {
      toast({ title: '保存失败', variant: 'destructive' })
    } finally {
      setSaving(false)
    }
  }

  async function handleDelete(id: number) {
    try {
      await skillAPI.delete(id)
      toast({ title: 'Skill 已删除' })
      mutate()
    } catch {
      toast({ title: '删除失败', variant: 'destructive' })
    }
  }

  async function handleReseed() {
    try {
      await skillAPI.reseed(GLOBAL_PROJECT_ID)
      toast({ title: '默认 Skill 已恢复', variant: 'success' })
      mutate()
    } catch {
      toast({ title: '恢复默认失败', variant: 'destructive' })
    }
  }

  const filtered = skills.filter(s =>
    !search || s.name.toLowerCase().includes(search.toLowerCase()) ||
    s.description.toLowerCase().includes(search.toLowerCase())
  )

  return (
    <div className="flex flex-col h-full">
      <DashboardHero
        badge="Skill 管理"
        title="Skill 管理"
        description="管理角色能力标签，按角色、能力类型和使用场景分类"
        actions={
          <div className="flex gap-2">
            <Button variant="outline" size="sm" onClick={handleReseed} title="恢复默认技能">
              <RotateCcw className="h-4 w-4 mr-1.5" />恢复默认
            </Button>
            <Button onClick={openCreate} size="sm">
              <Plus className="h-4 w-4 mr-1.5" />新增 Skill
            </Button>
          </div>
        }
      />

      <div className="flex-1 overflow-auto p-6">
        {/* Filters */}
        <div className="flex flex-wrap gap-3 mb-6">
          <Select value={filterType} onValueChange={setFilterType}>
            <SelectTrigger className="w-36">
              <SelectValue placeholder="能力类型" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">全部类型</SelectItem>
              {SKILL_TYPES.map(t => <SelectItem key={t.value} value={t.value}>{t.label}</SelectItem>)}
            </SelectContent>
          </Select>

          <Select value={filterUseCase} onValueChange={setFilterUseCase}>
            <SelectTrigger className="w-36">
              <SelectValue placeholder="使用场景" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">全部场景</SelectItem>
              {USE_CASES.map(u => <SelectItem key={u.value} value={u.value}>{u.label}</SelectItem>)}
            </SelectContent>
          </Select>

          <div className="relative flex-1 min-w-48">
            <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
            <Input
              className="pl-8"
              placeholder="搜索 Skill 名称 / 描述…"
              value={search}
              onChange={e => setSearch(e.target.value)}
            />
          </div>

          <Button variant="ghost" size="sm" onClick={() => mutate()} title="刷新">
            <RefreshCw className="h-4 w-4" />
          </Button>
        </div>

        {filtered.length === 0 ? (
          <DashboardEmptyState
            icon={<Star className="h-10 w-10 text-muted-foreground" />}
            title="暂无 Skill"
            description="点击「恢复默认」加载系统默认技能，或点击「新增 Skill」手动创建"
            action={
              <div className="flex gap-2">
                <Button variant="outline" onClick={handleReseed} size="sm"><RotateCcw className="h-4 w-4 mr-1.5" />恢复默认</Button>
                <Button onClick={openCreate} size="sm"><Plus className="h-4 w-4 mr-1.5" />新增 Skill</Button>
              </div>
            }
          />
        ) : (
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {filtered.map(skill => (
                <Card key={skill.id} className={`border border-border/50 hover:border-border transition-colors ${!skill.is_active ? 'opacity-50' : ''}`}>
                  <CardContent className="p-4">
                    <div className="flex items-start justify-between gap-2 mb-2">
                      <div className="flex flex-wrap gap-1.5">
                        <SkillTypeBadge type={skill.skill_type} />
                        <UseCaseBadge useCase={skill.use_case} />
                      </div>
                      <div className="flex gap-1 shrink-0">
                        <Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => openEdit(skill)}>
                          <Pencil className="h-3.5 w-3.5" />
                        </Button>
                        <AlertDialog>
                          <AlertDialogTrigger asChild>
                            <Button variant="ghost" size="icon" className="h-7 w-7 text-destructive hover:text-destructive">
                              <Trash2 className="h-3.5 w-3.5" />
                            </Button>
                          </AlertDialogTrigger>
                          <AlertDialogContent>
                            <AlertDialogHeader>
                              <AlertDialogTitle>删除 Skill</AlertDialogTitle>
                              <AlertDialogDescription>确定要删除「{skill.name}」吗？此操作不可撤销。</AlertDialogDescription>
                            </AlertDialogHeader>
                            <AlertDialogFooter>
                              <AlertDialogCancel>取消</AlertDialogCancel>
                              <AlertDialogAction onClick={() => handleDelete(skill.id)}>删除</AlertDialogAction>
                            </AlertDialogFooter>
                          </AlertDialogContent>
                        </AlertDialog>
                      </div>
                    </div>
                    <p className="font-medium text-sm">{skill.name}</p>
                    {skill.description && (
                      <p className="text-xs text-muted-foreground mt-1.5 line-clamp-2">{skill.description}</p>
                    )}
                  </CardContent>
                </Card>
            ))}
          </div>
        )}
      </div>

      {/* Create / Edit Dialog */}
      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>{editingSkill ? '编辑 Skill' : '新增 Skill'}</DialogTitle>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <div className="space-y-1.5">
              <Label>Skill 名称 *</Label>
              <Input value={form.name} onChange={e => setForm(f => ({ ...f, name: e.target.value }))} placeholder="如：近战格斗、隐身侦察…" />
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label>能力类型</Label>
                <Select value={form.skill_type} onValueChange={v => setForm(f => ({ ...f, skill_type: v }))}>
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>
                    {SKILL_TYPES.map(t => <SelectItem key={t.value} value={t.value}>{t.label}</SelectItem>)}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1.5">
                <Label>使用场景</Label>
                <Select value={form.use_case} onValueChange={v => setForm(f => ({ ...f, use_case: v }))}>
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>
                    {USE_CASES.map(u => <SelectItem key={u.value} value={u.value}>{u.label}</SelectItem>)}
                  </SelectContent>
                </Select>
              </div>
            </div>
            <div className="space-y-1.5">
              <Label>描述</Label>
              <Textarea
                rows={3}
                value={form.description}
                onChange={e => setForm(f => ({ ...f, description: e.target.value }))}
                placeholder="描述该 Skill 的作用和触发条件…"
              />
            </div>
            <div className="flex items-center gap-2">
              <input
                type="checkbox"
                id="skill-active"
                checked={form.is_active}
                onChange={e => setForm(f => ({ ...f, is_active: e.target.checked }))}
                className="rounded"
              />
              <Label htmlFor="skill-active">启用</Label>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDialogOpen(false)}>取消</Button>
            <Button onClick={handleSave} disabled={saving || !form.name.trim()}>
              {saving ? '保存中…' : '保存'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
