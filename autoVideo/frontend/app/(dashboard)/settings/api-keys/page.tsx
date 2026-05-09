'use client'

import { useState } from 'react'
import useSWR from 'swr'
import { authAPI, modelAPI, GeminiChannelStatus } from '@/lib/api'
import { APIKey, SystemAPIKey } from '@/types'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { useToast } from '@/components/ui/toast'
import { Plus, Trash2, Key, Globe, ChevronRight, CheckCircle2, AlertCircle, Eye, EyeOff, Loader2, RefreshCw, XCircle, Cpu } from 'lucide-react'
import { DashboardHero } from '@/components/layout/dashboard-hero'
import { DashboardEmptyState } from '@/components/layout/dashboard-empty-state'

// ─── 已验证可用的 Gemini 代理渠道参考库（2026-05-03 验证） ────────────────────
interface KnownChannel {
  id: string
  label: string
  base: string
  models: string[]
  verifiedAt: string
  expired?: boolean
  note?: string
}

const KNOWN_GEMINI_CHANNELS: KnownChannel[] = [
  {
    id: 'amutes',
    label: '阿姆 (amutes)',
    base: 'https://www.amutes.com',
    models: ['gemini-3.1-flash-image-preview', 'gemini-3-pro-image-preview'],
    verifiedAt: '2026-05-03',
  },
  {
    id: 'mingyu2',
    label: '明语2',
    base: 'http://104.238.220.180:61109',
    models: ['gemini-3.1-flash-image-preview'],
    verifiedAt: '2026-05-03',
  },
  {
    id: 'quickquickai',
    label: '盛总 (quickquickai)',
    base: 'https://www.quickquickai.com',
    models: ['gemini-3.1-flash-image-preview', 'gemini-3-pro-image-preview'],
    verifiedAt: '2026-05-03',
  },
  {
    id: 'polo',
    label: 'polo',
    base: 'https://fast.poloai.top',
    models: ['gemini-2.5-flash-image', 'gemini-3-pro-image-preview'],
    verifiedAt: '2026-05-03',
  },
  {
    id: 'midsummer',
    label: '仲夏 (midsummer)',
    base: 'https://new.midsummer.work',
    models: ['gemini-2.5-flash-image-preview', 'gemini-3-pro-image-preview'],
    verifiedAt: '2026-05-03',
  },
  {
    id: 'echo',
    label: 'echo',
    base: 'https://api.ai-echomind.com',
    models: ['gemini-3.1-flash-image-preview'],
    verifiedAt: '2026-05-03',
  },
  {
    id: 'yunzhi',
    label: '云智',
    base: 'https://api.openxs.top',
    models: ['gemini-3.1-flash-image-preview'],
    verifiedAt: '2026-05-03',
  },
  {
    id: 'ppchat',
    label: 'ppchat.vip',
    base: 'https://api.ppchat.vip',
    models: ['Gemini 模型已下架，现仅支持 GPT-Image'],
    verifiedAt: '2026-05-03',
    expired: true,
    note: '原 Gemini key 已失效，已从 config.local.yaml 移除',
  },
]

// ─── Gemini 渠道管理面板 ────────────────────────────────────────────────────

function GeminiChannelsPanel() {
  const {
    data: liveChannels,
    isLoading,
    mutate: refresh,
  } = useSWR<GeminiChannelStatus[]>('images/gemini-channels', () =>
    modelAPI.geminiChannels().then((r: { data: GeminiChannelStatus[] }) => r.data)
  )

  const configuredBases = (liveChannels ?? []).map((c) => c.base.replace(/\/$/, ''))

  return (
    <div className="space-y-5">
      {/* 当前配置渠道（实时探测） */}
      <Card>
        <CardHeader className="pb-3 flex flex-row items-center justify-between">
          <div>
            <CardTitle className="text-base">当前配置渠道</CardTitle>
            <CardDescription className="mt-0.5">
              来自 config.local.yaml 的 <code className="text-xs bg-muted px-1 rounded">gemini_bases / gemini_keys</code>，实时探测可用性
            </CardDescription>
          </div>
          <Button variant="ghost" size="sm" onClick={() => refresh()} disabled={isLoading} title="刷新探测">
            {isLoading ? <Loader2 className="h-4 w-4 animate-spin" /> : <RefreshCw className="h-4 w-4" />}
          </Button>
        </CardHeader>
        <CardContent className="space-y-2.5">
          {isLoading && (
            <div className="flex items-center gap-2 py-4 text-muted-foreground text-sm">
              <Loader2 className="h-4 w-4 animate-spin" />
              探测中…
            </div>
          )}
          {!isLoading && (!liveChannels || liveChannels.length === 0) && (
            <DashboardEmptyState
              icon={<Cpu className="mx-auto mb-2 h-7 w-7 opacity-40" />}
              title="未配置 Gemini 渠道"
              description="在 config.local.yaml 的 gemini_bases / gemini_keys 中添加渠道后重启服务"
              className="rounded-[24px] border-0 bg-transparent p-0 shadow-none"
              innerClassName="rounded-[24px] border border-dashed border-surface-200 py-8 text-center text-muted-foreground"
            />
          )}
          {(liveChannels ?? []).map((ch) => (
            <div
              key={ch.index}
              className="flex items-start gap-3 rounded-lg border border-border/50 p-3.5"
            >
              <div className="mt-0.5">
                {ch.valid ? (
                  <CheckCircle2 className="h-5 w-5 text-emerald-500" />
                ) : (
                  <XCircle className="h-5 w-5 text-red-500" />
                )}
              </div>
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-2 flex-wrap">
                  <span className="font-mono text-xs bg-muted px-1.5 py-0.5 rounded truncate max-w-[280px]">
                    {ch.base}
                  </span>
                  <Badge
                    className={
                      ch.valid
                        ? 'bg-emerald-100 text-emerald-800 text-xs'
                        : 'bg-red-100 text-red-800 text-xs'
                    }
                  >
                    {ch.valid ? '✓ 可用' : '✗ 已失效'}
                  </Badge>
                </div>
                <p className="text-xs text-muted-foreground mt-1 font-mono">
                  Key: {ch.key_mask}
                </p>
                {ch.error && (
                  <p className="text-xs text-red-500 mt-1">{ch.error}</p>
                )}
              </div>
              <div className="shrink-0 text-xs text-muted-foreground">#{ch.index}</div>
            </div>
          ))}
        </CardContent>
      </Card>

      {/* 已验证渠道参考库 */}
      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-base">已验证渠道库</CardTitle>
          <CardDescription>
            2026-05-03 全量 DB 校验结果。过期渠道已标注，可用渠道可选择添加到 config.local.yaml。
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-2.5">
          {KNOWN_GEMINI_CHANNELS.map((kc) => {
            const isConfigured = configuredBases.some(
              (b) => b.toLowerCase() === kc.base.replace(/\/$/, '').toLowerCase()
            )
            return (
              <div
                key={kc.id}
                className={`flex items-start gap-3 rounded-lg border p-3.5 transition-colors ${
                  kc.expired
                    ? 'border-red-200 bg-red-50/50 opacity-75'
                    : isConfigured
                      ? 'border-emerald-200 bg-emerald-50/30'
                      : 'border-border/50 hover:border-border'
                }`}
              >
                <div className="mt-0.5">
                  {kc.expired ? (
                    <XCircle className="h-5 w-5 text-red-400" />
                  ) : (
                    <CheckCircle2 className="h-5 w-5 text-emerald-500" />
                  )}
                </div>
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2 flex-wrap">
                    <span className={`font-medium text-sm ${kc.expired ? 'line-through text-muted-foreground' : ''}`}>
                      {kc.label}
                    </span>
                    {kc.expired && (
                      <Badge className="bg-red-100 text-red-700 text-xs">已失效</Badge>
                    )}
                    {!kc.expired && isConfigured && (
                      <Badge className="bg-emerald-100 text-emerald-700 text-xs">✓ 已配置</Badge>
                    )}
                    {!kc.expired && !isConfigured && (
                      <Badge variant="outline" className="text-xs text-muted-foreground">未配置</Badge>
                    )}
                  </div>
                  <p className="font-mono text-xs text-muted-foreground mt-0.5">{kc.base}</p>
                  <div className="flex flex-wrap gap-1 mt-1.5">
                    {kc.models.map((m) => (
                      <span
                        key={m}
                        className={`text-xs px-1.5 py-0.5 rounded font-mono ${
                          kc.expired ? 'bg-red-100 text-red-500 line-through' : 'bg-muted text-muted-foreground'
                        }`}
                      >
                        {m}
                      </span>
                    ))}
                  </div>
                  {kc.note && (
                    <p className="text-xs text-muted-foreground mt-1 italic">{kc.note}</p>
                  )}
                </div>
                <div className="shrink-0 text-xs text-muted-foreground whitespace-nowrap pt-0.5">
                  验证于 {kc.verifiedAt}
                </div>
              </div>
            )
          })}
        </CardContent>
      </Card>
    </div>
  )
}

const PROVIDER_LABELS: Record<string, { label: string; color: string }> = {
  zhipu:      { label: '智谱 GLM',        color: 'bg-primary-100 text-primary-700' },
  dashscope:  { label: '阿里云 DashScope', color: 'bg-orange-100 text-orange-700' },
  wcnbai:     { label: 'wcnbai',          color: 'bg-green-100 text-green-700' },
  openai:     { label: 'OpenAI',          color: 'bg-emerald-100 text-emerald-700' },
  anthropic:  { label: 'Anthropic',       color: 'bg-red-100 text-red-700' },
  custom:     { label: '自定义',           color: 'bg-surface-100 text-surface-700' },
}

function maskKey(key: string): string {
  if (!key || key.length < 12) return '••••••••'
  return key.slice(0, 6) + '••••••••' + key.slice(-4)
}

function ProviderBadge({ provider }: { provider: string }) {
  const info = PROVIDER_LABELS[provider] ?? { label: provider, color: 'bg-surface-100 text-surface-700' }
  return (
    <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${info.color}`}>
      {info.label}
    </span>
  )
}

function SystemKeyCard({ k }: { k: SystemAPIKey }) {
  const modelList = k.model_scope ? k.model_scope.split(',').slice(0, 5) : []
  const extraCount = k.model_scope ? k.model_scope.split(',').length - 5 : 0

  return (
    <Card className="border border-border/50 hover:border-border transition-colors">
      <CardContent className="p-4">
        <div className="flex items-start justify-between gap-3">
          <div className="flex-1 min-w-0">
            <div className="flex items-center gap-2 mb-1.5">
              <ProviderBadge provider={k.provider} />
              <Badge variant={k.is_active ? 'default' : 'secondary'} className="text-xs">
                {k.is_active ? '可用' : '禁用'}
              </Badge>
              <Badge variant="outline" className="text-xs">系统预置</Badge>
            </div>
            <p className="font-medium text-sm truncate">{k.key_alias}</p>
            {k.base_url && (
              <p className="text-xs text-muted-foreground mt-0.5 flex items-center gap-1">
                <Globe className="w-3 h-3" />
                {k.base_url}
              </p>
            )}
            {modelList.length > 0 && (
              <div className="flex flex-wrap gap-1 mt-2">
                {modelList.map(m => (
                  <span key={m} className="text-xs bg-muted px-1.5 py-0.5 rounded font-mono">{m.trim()}</span>
                ))}
                {extraCount > 0 && (
                  <span className="text-xs text-muted-foreground px-1.5 py-0.5">+{extraCount} 个</span>
                )}
              </div>
            )}
          </div>
          <CheckCircle2 className="w-5 h-5 text-green-500 flex-shrink-0 mt-0.5" />
        </div>
      </CardContent>
    </Card>
  )
}

function UserKeyCard({ k, onDelete }: { k: APIKey; onDelete: (id: number) => void }) {
  const [showKey, setShowKey] = useState(false)

  return (
    <Card className="border border-border/50 hover:border-border transition-colors">
      <CardContent className="p-4">
        <div className="flex items-start justify-between gap-3">
          <div className="flex-1 min-w-0">
            <div className="flex items-center gap-2 mb-1.5">
              <ProviderBadge provider={k.provider} />
              <Badge variant={k.is_active ? 'default' : 'secondary'} className="text-xs">
                {k.is_active ? '可用' : '禁用'}
              </Badge>
            </div>
            <p className="font-medium text-sm">{k.key_alias || '未命名'}</p>
            {k.base_url && (
              <p className="text-xs text-muted-foreground mt-0.5 flex items-center gap-1">
                <Globe className="w-3 h-3" />
                {k.base_url}
              </p>
            )}
            <p className="text-xs text-muted-foreground mt-1">
              创建于 {new Date(k.created_at).toLocaleDateString('zh-CN')}
            </p>
          </div>
          <div className="flex items-center gap-1">
            <Button
              variant="ghost"
              size="sm"
              className="h-8 w-8 p-0"
              onClick={() => setShowKey(!showKey)}
              title={showKey ? '隐藏密钥' : '显示密钥'}
            >
              {showKey ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
            </Button>
            <AlertDialog>
              <AlertDialogTrigger asChild>
                <Button variant="ghost" size="sm" className="h-8 w-8 p-0 text-destructive hover:text-destructive" title="删除此 API 密钥">
                  <Trash2 className="w-4 h-4" />
                </Button>
              </AlertDialogTrigger>
              <AlertDialogContent>
                <AlertDialogHeader>
                  <AlertDialogTitle>删除 API Key</AlertDialogTitle>
                  <AlertDialogDescription>
                    确定要删除这个 API Key 吗？删除后相关功能可能无法使用。
                  </AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                  <AlertDialogCancel>取消</AlertDialogCancel>
                  <AlertDialogAction onClick={() => onDelete(k.id)} className="bg-destructive text-destructive-foreground">
                    删除
                  </AlertDialogAction>
                </AlertDialogFooter>
              </AlertDialogContent>
            </AlertDialog>
          </div>
        </div>
        {showKey && (
          <div className="mt-2 p-2 bg-muted rounded font-mono text-xs break-all text-muted-foreground">
            {maskKey('')}（密钥已加密存储，无法明文查看）
          </div>
        )}
      </CardContent>
    </Card>
  )
}

function AddKeyDialog({ onSuccess }: { onSuccess: () => void }) {
  const { toast } = useToast()
  const [open, setOpen] = useState(false)
  const [loading, setLoading] = useState(false)
  const [form, setForm] = useState({
    provider: '',
    alias: '',
    key: '',
    base_url: '',
    model_scope: '',
  })

  const handleSubmit = async () => {
    if (!form.provider || !form.key) {
      toast({ title: '请填写必填项', variant: 'destructive' })
      return
    }
    setLoading(true)
    try {
      await authAPI.addAPIKey({
        provider: form.provider,
        alias: form.alias,
        key: form.key,
        base_url: form.base_url,
        model_scope: form.model_scope,
      })
      toast({ title: 'API Key 已添加', description: `已添加 ${form.provider} 密钥` })
      setOpen(false)
      setForm({ provider: '', alias: '', key: '', base_url: '', model_scope: '' })
      onSuccess()
    } catch {
      toast({ title: '添加失败', variant: 'destructive' })
    } finally {
      setLoading(false)
    }
  }

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button title="添加新的 API 密钥">
          <Plus className="w-4 h-4 mr-2" />
          添加 API Key
        </Button>
      </DialogTrigger>
      <DialogContent className="max-w-md">
        <DialogHeader>
          <DialogTitle>添加 API Key</DialogTitle>
          <DialogDescription>
            添加您自己的 API Key，用于模型调用计费
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4 py-2">
          <div className="space-y-1.5">
            <Label>服务商 <span className="text-destructive">*</span></Label>
            <select
              className="w-full h-9 px-3 text-sm border rounded-md bg-background"
              value={form.provider}
              onChange={e => setForm(p => ({ ...p, provider: e.target.value }))}
            >
              <option value="">选择服务商...</option>
              <option value="easyart">星启 easyart</option>
              <option value="zhipu">智谱 GLM</option>
              <option value="dashscope">阿里云 DashScope</option>
              <option value="wcnbai">wcnbai</option>
              <option value="openai">OpenAI</option>
              <option value="anthropic">Anthropic</option>
              <option value="custom">自定义</option>
            </select>
          </div>
          <div className="space-y-1.5">
            <Label>别名</Label>
            <Input
              placeholder="如：我的 OpenAI Key"
              value={form.alias}
              onChange={e => setForm(p => ({ ...p, alias: e.target.value }))}
            />
          </div>
          <div className="space-y-1.5">
            <Label>API Key <span className="text-destructive">*</span></Label>
            <Input
              type="password"
              placeholder="sk-..."
              value={form.key}
              onChange={e => setForm(p => ({ ...p, key: e.target.value }))}
            />
          </div>
          <div className="space-y-1.5">
            <Label>Base URL</Label>
            <Input
              placeholder="https://api.example.com（可选）"
              value={form.base_url}
              onChange={e => setForm(p => ({ ...p, base_url: e.target.value }))}
            />
          </div>
          <div className="space-y-1.5">
            <Label>支持模型（逗号分隔）</Label>
            <Input
              placeholder="gpt-4o,gpt-4o-mini（可选）"
              value={form.model_scope}
              onChange={e => setForm(p => ({ ...p, model_scope: e.target.value }))}
            />
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => setOpen(false)}>取消</Button>
          <Button onClick={handleSubmit} disabled={loading}>
            {loading ? '添加中...' : '确认添加'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

export default function APIKeysPage() {
  const { toast } = useToast()
  const { data: userKeysData, mutate: mutateUser } = useSWR(
    'auth/api-keys',
    () => authAPI.listAPIKeys() as unknown as Promise<{ data: APIKey[] }>
  )
  const { data: sysKeysData } = useSWR(
    'auth/system-api-keys',
    () => authAPI.listSystemAPIKeys() as unknown as Promise<{ data: SystemAPIKey[] }>
  )

  const userKeys = userKeysData?.data ?? []
  const sysKeys = sysKeysData?.data ?? []

  const handleDelete = async (id: number) => {
    try {
      await authAPI.deleteAPIKey(id)
      toast({ title: 'API Key 已删除' })
      mutateUser()
    } catch {
      toast({ title: '删除失败', variant: 'destructive' })
    }
  }

  return (
    <div className="mx-auto max-w-4xl space-y-6 p-6">
      <DashboardHero
        badge="Key 管理中心"
        badgeIcon={<Key className="h-3.5 w-3.5 text-fuchsia-300" />}
        title="API Keys"
        description="管理平台 API 密钥，系统预置密钥为所有用户共享；个人密钥可用于独立接入和计费。"
        gradientClassName="from-slate-950 via-fuchsia-950 to-slate-900"
        actions={<AddKeyDialog onSuccess={() => mutateUser()} />}
        stats={[
          {
            icon: <Key className="h-4 w-4 text-fuchsia-300" />,
            label: '系统预置',
            value: sysKeys.length,
            description: '由管理员统一配置的共享密钥',
          },
          {
            icon: <Plus className="h-4 w-4 text-cyan-300" />,
            label: '我的密钥',
            value: userKeys.length,
            description: '个人新增的私有接入凭据',
          },
          {
            icon: <CheckCircle2 className="h-4 w-4 text-emerald-300" />,
            label: '可用密钥',
            value: sysKeys.filter(k => k.is_active).length + userKeys.filter(k => k.is_active).length,
            description: '当前处于启用状态的全部密钥',
          },
        ]}
      />

      {/* Stats */}
      <div className="grid grid-cols-3 gap-4">
        <Card>
          <CardContent className="p-4">
            <div className="flex items-center gap-3">
              <div className="w-9 h-9 bg-purple-100 rounded-lg flex items-center justify-center">
                <Key className="w-5 h-5 text-purple-600" />
              </div>
              <div>
                <p className="text-2xl font-bold">{sysKeys.length}</p>
                <p className="text-xs text-muted-foreground">系统预置</p>
              </div>
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="p-4">
            <div className="flex items-center gap-3">
              <div className="w-9 h-9 bg-primary-100 rounded-lg flex items-center justify-center">
                <Key className="w-5 h-5 text-primary-600" />
              </div>
              <div>
                <p className="text-2xl font-bold">{userKeys.length}</p>
                <p className="text-xs text-muted-foreground">我的密钥</p>
              </div>
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="p-4">
            <div className="flex items-center gap-3">
              <div className="w-9 h-9 bg-green-100 rounded-lg flex items-center justify-center">
                <CheckCircle2 className="w-5 h-5 text-green-600" />
              </div>
              <div>
                <p className="text-2xl font-bold">
                  {sysKeys.filter(k => k.is_active).length + userKeys.filter(k => k.is_active).length}
                </p>
                <p className="text-xs text-muted-foreground">可用密钥</p>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Tabs */}
      <Tabs defaultValue="system">
        <TabsList>
          <TabsTrigger value="system">
            系统预置
            <Badge variant="secondary" className="ml-1.5 text-xs">{sysKeys.length}</Badge>
          </TabsTrigger>
          <TabsTrigger value="mine">
            我的密钥
            <Badge variant="secondary" className="ml-1.5 text-xs">{userKeys.length}</Badge>
          </TabsTrigger>
          <TabsTrigger value="gemini">
            Gemini 渠道
            <Badge variant="secondary" className="ml-1.5 text-xs">图像生成</Badge>
          </TabsTrigger>
        </TabsList>

        <TabsContent value="system" className="mt-4">
          <Card>
            <CardHeader className="pb-3">
              <CardTitle className="text-base">系统预置 API Keys</CardTitle>
              <CardDescription>
                由平台管理员配置的共享密钥，已验证可用。所有用户均可使用这些密钥进行模型调用。
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              {sysKeys.length === 0 ? (
                <DashboardEmptyState
                  icon={<AlertCircle className="mx-auto mb-2 h-8 w-8 opacity-40" />}
                  title="暂无系统预置密钥"
                  description="系统级渠道尚未配置时，可先添加个人 API Key 使用。"
                  className="rounded-[24px] border-0 bg-transparent p-0 shadow-none"
                  innerClassName="rounded-[24px] border border-dashed border-surface-200 bg-[radial-gradient(circle_at_top_left,_rgba(217,70,239,0.08),_transparent_30%),radial-gradient(circle_at_bottom_right,_rgba(99,102,241,0.08),_transparent_28%)] py-12 text-center text-muted-foreground"
                />
              ) : (
                sysKeys.map(k => <SystemKeyCard key={k.id} k={k} />)
              )}
            </CardContent>
          </Card>

          {/* Quick reference */}
          <Card className="mt-4 bg-muted/50">
            <CardContent className="p-4">
              <h4 className="text-sm font-medium mb-3 flex items-center gap-1.5">
                <ChevronRight className="w-4 h-4" />
                支持的模型范围
              </h4>
              <div className="grid grid-cols-2 gap-3 text-xs text-muted-foreground">
                <div>
                  <p className="font-medium text-foreground mb-1">🤖 文本/推理</p>
                  <p>Gemini 系列、Claude 系列、GPT 系列</p>
                  <p>GLM-4 系列、Qwen3 系列</p>
                </div>
                <div>
                  <p className="font-medium text-foreground mb-1">🎨 图像生成</p>
                  <p>Gemini Image Preview</p>
                  <p>GPT-Image-1.5</p>
                </div>
                <div>
                  <p className="font-medium text-foreground mb-1">🎬 视频生成</p>
                  <p>Sora-2、Veo3、Veo3.1</p>
                  <p>WAN2.5 图生视频</p>
                </div>
                <div>
                  <p className="font-medium text-foreground mb-1">🔊 语音/TTS</p>
                  <p>MiniMax-M2.5</p>
                  <p>DashScope TTS</p>
                </div>
              </div>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="mine" className="mt-4">
          <Card>
            <CardHeader className="pb-3">
              <CardTitle className="text-base">我的 API Keys</CardTitle>
              <CardDescription>
                您个人添加的密钥，优先级高于系统预置密钥。密钥加密存储，无法明文查看。
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              {userKeys.length === 0 ? (
                <DashboardEmptyState
                  icon={<Key className="mx-auto mb-2 h-8 w-8 opacity-40" />}
                  title="暂无个人密钥"
                  description="添加您自己的 API Key 可以独立计费"
                  className="rounded-[24px] border-0 bg-transparent p-0 shadow-none"
                  innerClassName="rounded-[24px] border border-dashed border-surface-200 bg-[radial-gradient(circle_at_top_left,_rgba(59,130,246,0.08),_transparent_30%),radial-gradient(circle_at_bottom_right,_rgba(16,185,129,0.08),_transparent_28%)] py-12 text-center text-muted-foreground"
                />
              ) : (
                userKeys.map(k => (
                  <UserKeyCard key={k.id} k={k} onDelete={handleDelete} />
                ))
              )}
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="gemini" className="mt-4">
          <GeminiChannelsPanel />
        </TabsContent>
      </Tabs>
    </div>
  )
}
