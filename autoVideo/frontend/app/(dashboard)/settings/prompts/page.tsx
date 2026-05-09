'use client'

import { useState } from 'react'
import useSWR from 'swr'
import { promptTemplateAPI } from '@/lib/api'
import type { PromptTemplate } from '@/types'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
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
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { useToast } from '@/components/ui/toast'
import { Plus, Trash2, Pencil, Eye, Search, RefreshCw, Code2, RotateCcw } from 'lucide-react'
import { DashboardHero } from '@/components/layout/dashboard-hero'
import { DashboardEmptyState } from '@/components/layout/dashboard-empty-state'

const RESOURCE_TYPES = [
  { value: 'character',   label: '人物' },
  { value: 'scene',       label: '场景' },
  { value: 'item',        label: '物品' },
  { value: 'storyboard',  label: '分镜' },
]

const RESOURCE_COLORS: Record<string, string> = {
  character:  'bg-orange-100 text-orange-700',
  scene:      'bg-green-100 text-green-700',
  item:       'bg-blue-100 text-blue-700',
  storyboard: 'bg-purple-100 text-purple-700',
}

function ResourceTypeBadge({ type }: { type: string }) {
  const t = RESOURCE_TYPES.find(t => t.value === type)
  const color = RESOURCE_COLORS[type] ?? 'bg-surface-100 text-surface-700'
  return (
    <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${color}`}>
      {t?.label ?? type ?? '通用'}
    </span>
  )
}

interface FormState {
  name: string
  style_key: string
  description: string
  content: string
  resource_type: string
  model_binding: string
  version: string
  sort_order: string
  is_active: boolean
}

const EMPTY_FORM: FormState = {
  name: '', style_key: '', description: '', content: '',
  resource_type: 'character', model_binding: '', version: '', sort_order: '0', is_active: true,
}

function extractVarsFromContent(content: string): string[] {
  const matches = content.match(/\{([^{}]+)\}/g) ?? []
  const seen = new Set<string>()
  return matches
    .map(m => m.slice(1, -1))
    .filter(v => { if (seen.has(v)) return false; seen.add(v); return true })
}

export default function PromptsPage() {
  const { toast } = useToast()

  const [filterResourceType, setFilterResourceType] = useState('all')
  const [filterModelBinding, setFilterModelBinding] = useState('all')
  const [search, setSearch] = useState('')

  const { data: templatesData, mutate } = useSWR(
    ['prompt-templates', filterResourceType, filterModelBinding],
    () => promptTemplateAPI.list({
      resource_type: filterResourceType === 'all' ? undefined : filterResourceType,
      model_binding: filterModelBinding === 'all' ? undefined : filterModelBinding,
    })
  )
  const templates: PromptTemplate[] = templatesData?.data ?? []

  const [dialogOpen, setDialogOpen] = useState(false)
  const [editingTpl, setEditingTpl] = useState<PromptTemplate | null>(null)
  const [form, setForm] = useState<FormState>(EMPTY_FORM)
  const [saving, setSaving] = useState(false)

  // Preview state
  const [previewOpen, setPreviewOpen] = useState(false)
  const [previewTpl, setPreviewTpl] = useState<PromptTemplate | null>(null)
  const [previewVars, setPreviewVars] = useState<Record<string, string>>({})
  const [previewResult, setPreviewResult] = useState('')
  const [previewLoading, setPreviewLoading] = useState(false)

  function openCreate() {
    setEditingTpl(null)
    setForm(EMPTY_FORM)
    setDialogOpen(true)
  }

  function openEdit(tpl: PromptTemplate) {
    setEditingTpl(tpl)
    setForm({
      name: tpl.name,
      style_key: tpl.style_key,
      description: tpl.description,
      content: tpl.content,
      resource_type: tpl.resource_type,
      model_binding: tpl.model_binding,
      version: tpl.version,
      sort_order: String(tpl.sort_order),
      is_active: tpl.is_active,
    })
    setDialogOpen(true)
  }

  async function handleSave() {
    if (!form.name.trim() || !form.style_key.trim() || !form.content.trim()) return
    setSaving(true)
    try {
      const payload = {
        name: form.name.trim(),
        style_key: form.style_key.trim(),
        description: form.description,
        content: form.content,
        resource_type: form.resource_type,
        model_binding: form.model_binding,
        version: form.version,
        sort_order: Number(form.sort_order) || 0,
        is_active: form.is_active,
      }
      if (editingTpl) {
        await promptTemplateAPI.update(editingTpl.id, payload)
        toast({ title: '模板已更新', variant: 'success' })
      } else {
        await promptTemplateAPI.create(payload)
        toast({ title: '模板已创建', variant: 'success' })
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
      await promptTemplateAPI.delete(id)
      toast({ title: '模板已删除' })
      mutate()
    } catch {
      toast({ title: '删除失败', variant: 'destructive' })
    }
  }

  async function handleReseedDefaults() {
    try {
      await promptTemplateAPI.reseedDefaults()
      toast({ title: '默认提示词模板已恢复', variant: 'success' })
      mutate()
    } catch {
      toast({ title: '恢复默认失败', variant: 'destructive' })
    }
  }

  function openPreview(tpl: PromptTemplate) {
    setPreviewTpl(tpl)
    const vars = extractVarsFromContent(tpl.content)
    setPreviewVars(Object.fromEntries(vars.map(v => [v, ''])))
    setPreviewResult(tpl.content)
    setPreviewOpen(true)
  }

  async function runPreview() {
    if (!previewTpl) return
    setPreviewLoading(true)
    try {
      const res = await promptTemplateAPI.preview(previewTpl.id, previewVars)
      setPreviewResult(res.data?.preview ?? '')
    } catch {
      toast({ title: '预览失败', variant: 'destructive' })
    } finally {
      setPreviewLoading(false)
    }
  }

  const filtered = templates.filter(t =>
    !search ||
    t.name.toLowerCase().includes(search.toLowerCase()) ||
    t.style_key.toLowerCase().includes(search.toLowerCase()) ||
    t.description.toLowerCase().includes(search.toLowerCase())
  )

  const modelBindings = [...new Set(templates.map(t => t.model_binding).filter(Boolean))]

  return (
    <div className="flex flex-col h-full">
      <DashboardHero
        badge="提示词模板"
        title="提示词模板"
        description="管理图片资产和分镜生成的提示词模板，支持变量占位符和模型适配"
        actions={
          <div className="flex gap-2">
            <Button variant="outline" size="sm" onClick={handleReseedDefaults} title="恢复内置默认模板">
              <RotateCcw className="h-4 w-4 mr-1.5" />恢复默认
            </Button>
            <Button onClick={openCreate} size="sm">
              <Plus className="h-4 w-4 mr-1.5" />新增模板
            </Button>
          </div>
        }
      />

      <div className="flex-1 overflow-auto p-6">
        {/* Filters */}
        <div className="flex flex-wrap gap-3 mb-6">
          <Select value={filterResourceType} onValueChange={setFilterResourceType}>
            <SelectTrigger className="w-36">
              <SelectValue placeholder="资源类型" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">全部类型</SelectItem>
              {RESOURCE_TYPES.map(t => <SelectItem key={t.value} value={t.value}>{t.label}</SelectItem>)}
            </SelectContent>
          </Select>

          <Select value={filterModelBinding} onValueChange={setFilterModelBinding}>
            <SelectTrigger className="w-44">
              <SelectValue placeholder="模型适配" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">全部模型</SelectItem>
              {modelBindings.map(m => <SelectItem key={m} value={m}>{m}</SelectItem>)}
            </SelectContent>
          </Select>

          <div className="relative flex-1 min-w-48">
            <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
            <Input
              className="pl-8"
              placeholder="搜索模板名称 / 风格 Key / 描述…"
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
            icon={<Code2 className="h-10 w-10 text-muted-foreground" />}
            title="暂无提示词模板"
            description="点击「新增模板」创建第一个提示词模板"
            action={<Button onClick={openCreate} size="sm"><Plus className="h-4 w-4 mr-1.5" />新增模板</Button>}
          />
        ) : (
          <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
            {filtered.map(tpl => (
              <Card key={tpl.id} className={`border border-border/50 hover:border-border transition-colors ${!tpl.is_active ? 'opacity-50' : ''}`}>
                <CardHeader className="pb-2 pt-4 px-4">
                  <div className="flex items-start justify-between gap-2">
                    <div className="flex-1 min-w-0">
                      <div className="flex flex-wrap gap-1.5 mb-1.5">
                        <ResourceTypeBadge type={tpl.resource_type} />
                        {tpl.model_binding && (
                          <Badge variant="outline" className="text-xs">{tpl.model_binding}</Badge>
                        )}
                        {tpl.version && (
                          <span className="text-xs text-muted-foreground">v{tpl.version}</span>
                        )}
                        {!tpl.is_active && <Badge variant="secondary" className="text-xs">禁用</Badge>}
                      </div>
                      <CardTitle className="text-sm font-semibold truncate">{tpl.name}</CardTitle>
                      <p className="text-xs text-muted-foreground mt-0.5">Key: {tpl.style_key}</p>
                    </div>
                    <div className="flex gap-1 shrink-0">
                      <Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => openPreview(tpl)} title="预览">
                        <Eye className="h-3.5 w-3.5" />
                      </Button>
                      <Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => openEdit(tpl)}>
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
                            <AlertDialogTitle>删除模板</AlertDialogTitle>
                            <AlertDialogDescription>确定要删除「{tpl.name}」吗？此操作不可撤销。</AlertDialogDescription>
                          </AlertDialogHeader>
                          <AlertDialogFooter>
                            <AlertDialogCancel>取消</AlertDialogCancel>
                            <AlertDialogAction onClick={() => handleDelete(tpl.id)}>删除</AlertDialogAction>
                          </AlertDialogFooter>
                        </AlertDialogContent>
                      </AlertDialog>
                    </div>
                  </div>
                </CardHeader>
                <CardContent className="px-4 pb-4">
                  {tpl.description && (
                    <p className="text-xs text-muted-foreground mb-2">{tpl.description}</p>
                  )}
                  <div className="bg-surface-50 rounded p-2 border border-border/40">
                    <pre className="text-xs text-surface-700 whitespace-pre-wrap line-clamp-4 font-mono">{tpl.content}</pre>
                  </div>
                  {extractVarsFromContent(tpl.content).length > 0 && (
                    <div className="flex flex-wrap gap-1 mt-2">
                      <Code2 className="h-3 w-3 text-muted-foreground mt-0.5" />
                      {extractVarsFromContent(tpl.content).map(v => (
                        <span key={v} className="text-xs font-mono text-primary bg-primary/10 px-1.5 py-0.5 rounded">{`{${v}}`}</span>
                      ))}
                    </div>
                  )}
                </CardContent>
              </Card>
            ))}
          </div>
        )}
      </div>

      {/* Create / Edit Dialog */}
      <Dialog open={dialogOpen} onOpenChange={setDialogOpen}>
        <DialogContent className="max-w-2xl max-h-[90vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>{editingTpl ? '编辑模板' : '新增提示词模板'}</DialogTitle>
          </DialogHeader>
          <div className="space-y-4 py-2">
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label>模板名称 *</Label>
                <Input value={form.name} onChange={e => setForm(f => ({ ...f, name: e.target.value }))} placeholder="如：融生4.3 动漫人物" />
              </div>
              <div className="space-y-1.5">
                <Label>风格 Key *</Label>
                <Input value={form.style_key} onChange={e => setForm(f => ({ ...f, style_key: e.target.value }))} placeholder="如：animation_v43" />
              </div>
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label>资源类型</Label>
                <Select value={form.resource_type} onValueChange={v => setForm(f => ({ ...f, resource_type: v }))}>
                  <SelectTrigger><SelectValue /></SelectTrigger>
                  <SelectContent>
                    {RESOURCE_TYPES.map(t => <SelectItem key={t.value} value={t.value}>{t.label}</SelectItem>)}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1.5">
                <Label>绑定模型（可选）</Label>
                <Input value={form.model_binding} onChange={e => setForm(f => ({ ...f, model_binding: e.target.value }))} placeholder="如：xingtu-v3，留空表示通用" />
              </div>
            </div>
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label>版本号</Label>
                <Input value={form.version} onChange={e => setForm(f => ({ ...f, version: e.target.value }))} placeholder="如：1.1" />
              </div>
              <div className="space-y-1.5">
                <Label>排序权重</Label>
                <Input type="number" value={form.sort_order} onChange={e => setForm(f => ({ ...f, sort_order: e.target.value }))} />
              </div>
            </div>
            <div className="space-y-1.5">
              <Label>描述</Label>
              <Input value={form.description} onChange={e => setForm(f => ({ ...f, description: e.target.value }))} placeholder="简要描述此模板的适用风格和场景" />
            </div>
            <div className="space-y-1.5">
              <Label>模板内容 * <span className="text-xs text-muted-foreground ml-1">（使用 {'{变量名}'} 作为占位符）</span></Label>
              <Textarea
                rows={10}
                className="font-mono text-xs"
                value={form.content}
                onChange={e => setForm(f => ({ ...f, content: e.target.value }))}
                placeholder="输入提示词模板内容，支持 {角色名}、{场景名} 等变量占位符…"
              />
              {extractVarsFromContent(form.content).length > 0 && (
                <div className="flex flex-wrap gap-1 pt-1">
                  <span className="text-xs text-muted-foreground">检测到变量：</span>
                  {extractVarsFromContent(form.content).map(v => (
                    <span key={v} className="text-xs font-mono text-primary bg-primary/10 px-1.5 py-0.5 rounded">{`{${v}}`}</span>
                  ))}
                </div>
              )}
            </div>
            <div className="flex items-center gap-2">
              <input
                type="checkbox"
                id="tpl-active"
                checked={form.is_active}
                onChange={e => setForm(f => ({ ...f, is_active: e.target.checked }))}
                className="rounded"
              />
              <Label htmlFor="tpl-active">启用</Label>
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDialogOpen(false)}>取消</Button>
            <Button onClick={handleSave} disabled={saving || !form.name.trim() || !form.style_key.trim() || !form.content.trim()}>
              {saving ? '保存中…' : '保存'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Preview Dialog */}
      <Dialog open={previewOpen} onOpenChange={setPreviewOpen}>
        <DialogContent className="max-w-2xl max-h-[90vh] overflow-y-auto">
          <DialogHeader>
            <DialogTitle>预览：{previewTpl?.name}</DialogTitle>
          </DialogHeader>
          {previewTpl && (
            <Tabs defaultValue="preview">
              <TabsList>
                <TabsTrigger value="preview">预览结果</TabsTrigger>
                <TabsTrigger value="variables">填入变量</TabsTrigger>
              </TabsList>
              <TabsContent value="variables" className="space-y-3 pt-3">
                {extractVarsFromContent(previewTpl.content).length === 0 ? (
                  <p className="text-sm text-muted-foreground">该模板不含变量占位符。</p>
                ) : (
                  extractVarsFromContent(previewTpl.content).map(v => (
                    <div key={v} className="space-y-1">
                      <Label className="font-mono text-xs text-primary">{`{${v}}`}</Label>
                      <Input
                        value={previewVars[v] ?? ''}
                        onChange={e => setPreviewVars(prev => ({ ...prev, [v]: e.target.value }))}
                        placeholder={`填入 ${v} 的值…`}
                      />
                    </div>
                  ))
                )}
                <Button onClick={runPreview} disabled={previewLoading} className="w-full">
                  {previewLoading ? '生成中…' : '生成预览'}
                </Button>
              </TabsContent>
              <TabsContent value="preview" className="pt-3">
                <div className="bg-surface-50 rounded p-3 border border-border/40">
                  <pre className="text-xs whitespace-pre-wrap font-mono text-surface-800">
                    {previewResult || previewTpl.content}
                  </pre>
                </div>
              </TabsContent>
            </Tabs>
          )}
          <DialogFooter>
            <Button variant="outline" onClick={() => setPreviewOpen(false)}>关闭</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
