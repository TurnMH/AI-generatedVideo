'use client'

import { useState, useCallback, useEffect, useMemo } from 'react'
import useSWR from 'swr'
import {
  MessageSquare,
  ImageIcon,
  Video,
  Volume2,
  Plus,
  Trash2,
  Star,
  Pencil,
  Zap,
  Scale,
  Gem,
  Activity,
  Loader2,
  Search,
  X,
  ExternalLink,
  CheckCircle2,
  CircleAlert,
  CloudDownload,
  Link2,
  AlertTriangle,
} from 'lucide-react'
import { authAPI, modelAPI } from '@/lib/api'
import type { APIKey, Model, ModelType, SpeedRating, SystemAPIKey } from '@/types'
import {
  OFFICIAL_MODEL_CATALOG,
  buildModelPayloadFromCatalog,
  type CatalogStatus,
  type IntegrationStatus,
  type OfficialModelCatalogItem,
} from '@/lib/official-model-catalog'
import {
  buildModelFeasibilityAssessments,
  buildProductFeasibilityAssessments,
  getAssessmentAvailabilityLabels,
  getModelCapabilityLabels,
  getProviderLabel,
  getProductAvailabilityLabels,
  getProductCapabilityLabels,
  getValidationStatusMeta,
  type ModelFeasibilityAssessment,
  type ProductFeasibilityAssessment,
} from '@/lib/model-feasibility'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Card, CardContent, CardHeader } from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
  DialogDescription,
} from '@/components/ui/dialog'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Separator } from '@/components/ui/separator'
import { useToast } from '@/components/ui/toast'
import { DashboardHero } from '@/components/layout/dashboard-hero'
import { DashboardEmptyState } from '@/components/layout/dashboard-empty-state'

// ─── Constants ────────────────────────────────────────────────────────────────

const MODEL_TYPES: { value: ModelType; label: string; icon: React.ElementType }[] = [
  { value: 'llm', label: '文本模型', icon: MessageSquare },
  { value: 'image', label: '图像模型', icon: ImageIcon },
  { value: 'video', label: '视频模型', icon: Video },
  { value: 'audio', label: '语音模型', icon: Volume2 },
]

const SPEED_CONFIG: Record<SpeedRating, { label: string; color: string; icon: React.ElementType }> = {
  fast: { label: '快速', color: 'bg-emerald-100 text-emerald-800', icon: Zap },
  balanced: { label: '均衡', color: 'bg-amber-100 text-amber-800', icon: Scale },
  quality: { label: '高质量', color: 'bg-purple-100 text-purple-800', icon: Gem },
  slow: { label: '慢速', color: 'bg-red-100 text-red-800', icon: Scale },
}

const EMPTY_FORM: Partial<Model> = {
  name: '',
  model_key: '',
  type: 'llm',
  provider: '',
  speed_rating: 'balanced',
  capability_tags: [],
  supports_consistency: false,
  consistency_method: 'none',
  is_active: true,
  is_default: false,
  input_price: 0,
  output_price: 0,
  cost_per_unit: 0,
  unit: '',
  supported_ratios: [],
  description: '',
  api_endpoint: '',
  api_key_ref: '',
  context_window: undefined,
  max_resolution: '',
  video_mode: undefined,
  priority: 0,
}

type HealthMap = Record<string, 'healthy' | 'unhealthy' | 'unknown'>

const CATALOG_STATUS_CONFIG: Record<CatalogStatus, { label: string; color: string }> = {
  default: { label: '官方主推', color: 'bg-emerald-100 text-emerald-800' },
  latest: { label: '最新', color: 'bg-blue-100 text-blue-800' },
  stable: { label: '稳定', color: 'bg-sky-100 text-sky-800' },
  preview: { label: '预览', color: 'bg-amber-100 text-amber-800' },
  snapshot: { label: '快照', color: 'bg-indigo-100 text-indigo-800' },
  legacy: { label: '旧版', color: 'bg-zinc-100 text-zinc-800' },
  deprecated: { label: '弃用', color: 'bg-red-100 text-red-800' },
  recommended: { label: '推荐', color: 'bg-violet-100 text-violet-800' },
}

const INTEGRATION_STATUS_CONFIG: Record<IntegrationStatus, { label: string; color: string; description: string }> = {
  native: {
    label: '原生可用',
    color: 'bg-emerald-100 text-emerald-800',
    description: '当前仓库已有明确代码路径或生成器实现。',
  },
  config_only: {
    label: '可入库待打通',
    color: 'bg-blue-100 text-blue-800',
    description: '当前可作为模型目录管理，但业务链路尚未完全按 model_key 消费。',
  },
  catalog_only: {
    label: '仅目录展示',
    color: 'bg-zinc-100 text-zinc-700',
    description: '官方已展示，但当前项目没有对应适配器或调用链路。',
  },
}

// ─── Sub-components ───────────────────────────────────────────────────────────

function HealthDot({ status }: { status?: 'healthy' | 'unhealthy' | 'unknown' }) {
  const cls =
    status === 'healthy'
      ? 'bg-green-500 shadow-green-500/40'
      : status === 'unhealthy'
        ? 'bg-red-500 shadow-red-500/40'
        : 'bg-zinc-300'
  const title =
    status === 'healthy' ? '运行正常' : status === 'unhealthy' ? '连接异常' : '状态未知'
  return (
    <span
      className={`inline-block h-2.5 w-2.5 shrink-0 rounded-full shadow-sm ${cls}`}
      title={title}
    />
  )
}

function SpeedBadge({ rating }: { rating: SpeedRating }) {
  const cfg = SPEED_CONFIG[rating]
  const Icon = cfg.icon
  return (
    <Badge className={`gap-1 ${cfg.color}`}>
      <Icon className="h-3 w-3" />
      {cfg.label}
    </Badge>
  )
}

function TagList({ tags }: { tags: string[] }) {
  if (!tags.length) return null
  return (
    <div className="flex flex-wrap gap-1">
      {tags.map((tag) => (
        <Badge key={tag} variant="outline" className="text-[11px] font-normal">
          {tag}
        </Badge>
      ))}
    </div>
  )
}

// ─── Toggle Switch (inline, no external dep) ─────────────────────────────────

function Toggle({
  checked,
  onChange,
  disabled,
}: {
  checked: boolean
  onChange: (v: boolean) => void
  disabled?: boolean
}) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      disabled={disabled}
      onClick={() => onChange(!checked)}
      className={`relative inline-flex h-6 w-11 shrink-0 items-center rounded-full transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50 ${
        checked ? 'bg-primary-600' : 'bg-zinc-200'
      }`}
    >
      <span
        className={`pointer-events-none inline-block h-4 w-4 rounded-full bg-white shadow-sm transition-transform ${
          checked ? 'translate-x-6' : 'translate-x-1'
        }`}
      />
    </button>
  )
}

// ─── Model Card ───────────────────────────────────────────────────────────────

function ModelCard({
  model,
  health,
  onToggle,
  onSetDefault,
  onEdit,
  onTest,
  onDelete,
  testingId,
}: {
  model: Model
  health?: 'healthy' | 'unhealthy' | 'unknown'
  onToggle: (id: number) => void
  onSetDefault: (id: number) => void
  onEdit: (model: Model) => void
  onTest: (id: number) => void
  onDelete: (id: number) => void
  testingId: number | null
}) {
  const isTesting = testingId === model.id

  return (
    <Card className="group relative overflow-hidden border-zinc-200 transition-shadow hover:shadow-md">
      {model.is_default && (
        <div className="absolute right-0 top-0 rounded-bl-lg bg-amber-400 px-2 py-0.5">
          <Star className="h-3.5 w-3.5 text-white" fill="currentColor" />
        </div>
      )}

      <CardHeader className="pb-3">
        <div className="flex items-start justify-between gap-3">
          <div className="flex items-center gap-2.5 min-w-0">
            <HealthDot status={health} />
            <div className="min-w-0">
              <h3 className="truncate text-sm font-semibold text-zinc-900">{model.name}</h3>
              <p className="truncate text-xs text-zinc-500">{model.model_key}</p>
            </div>
          </div>
          <Toggle checked={model.is_active} onChange={() => onToggle(model.id)} />
        </div>
      </CardHeader>

      <CardContent className="space-y-3 pt-0">
        {/* Provider + Speed */}
        <div className="flex flex-wrap items-center gap-2">
          <Badge variant="secondary">{model.provider}</Badge>
          <SpeedBadge rating={model.speed_rating} />
        </div>

        {/* Capability tags */}
        <TagList tags={model.capability_tags ?? []} />

        {/* Type-specific info */}
        {model.type === 'image' && model.supports_consistency && (
          <div className="flex items-center gap-1.5 text-xs text-zinc-600">
            <span className="text-emerald-600">✓</span>
            <span>一致性: {model.consistency_method}</span>
          </div>
        )}
        {model.type === 'video' && model.video_mode && (
          <div className="text-xs text-zinc-600">
            模式: {model.video_mode === 'frame_animation' ? '帧动画' : model.video_mode === 'api_generation' ? 'API生成' : '混合'}
          </div>
        )}
        {model.type === 'llm' && model.context_window && (
          <div className="text-xs text-zinc-600">
            上下文: {(model.context_window / 1000).toFixed(0)}K tokens
          </div>
        )}
        {model.max_resolution && (
          <div className="text-xs text-zinc-600">最大分辨率: {model.max_resolution}</div>
        )}
        {model.supported_ratios?.length > 0 && (
          <div className="flex flex-wrap gap-1">
            {model.supported_ratios.map((r) => (
              <span key={r} className="rounded bg-zinc-100 px-1.5 py-0.5 text-[11px] text-zinc-600">
                {r}
              </span>
            ))}
          </div>
        )}

        {/* Pricing */}
        <Separator />
        <div className="flex items-baseline justify-between text-xs">
          <span className="text-zinc-500">价格</span>
          <span className="font-medium text-zinc-700">
            {model.cost_per_unit > 0 ? `¥${model.cost_per_unit}` : model.input_price ? `¥${model.input_price}` : '免费'}
            {model.unit ? ` / ${model.unit}` : ''}
          </span>
        </div>

        {/* Actions */}
        <Separator />
        <div className="flex flex-wrap items-center gap-1.5">
          {!model.is_default && (
            <Button
              variant="ghost"
              size="sm"
              className="h-7 gap-1 text-xs text-zinc-600 hover:text-amber-600"
              onClick={() => onSetDefault(model.id)}
              title="将此模型设为该类型的默认模型"
            >
              <Star className="h-3 w-3" />
              设为默认
            </Button>
          )}
          <Button
            variant="ghost"
            size="sm"
            className="h-7 gap-1 text-xs text-zinc-600"
            onClick={() => onEdit(model)}
            title="编辑模型名称、参数等配置"
          >
            <Pencil className="h-3 w-3" />
            编辑
          </Button>
          <Button
            variant="ghost"
            size="sm"
            className="h-7 gap-1 text-xs text-zinc-600"
            onClick={() => onTest(model.id)}
            disabled={isTesting}
            title="发送测试请求验证模型连通性"
          >
            {isTesting ? <Loader2 className="h-3 w-3 animate-spin" /> : <Activity className="h-3 w-3" />}
            测试
          </Button>
          <Button
            variant="ghost"
            size="sm"
            className="h-7 gap-1 text-xs text-red-500 hover:bg-red-50 hover:text-red-700 ml-auto"
            onClick={() => onDelete(model.id)}
            title="删除此模型"
          >
            <Trash2 className="h-3 w-3" />
          </Button>
        </div>
      </CardContent>
    </Card>
  )
}

// ─── Unavailable Reason Helper ────────────────────────────────────────────────

function getUnavailableReason(model: Model, health?: 'healthy' | 'unhealthy' | 'unknown'): string | null {
  const failureReason = model.failure_reason?.trim()

  if (!model.is_active) return failureReason || '已手动禁用'
  if (failureReason) return failureReason
  if (health === 'unhealthy') return '连接异常'
  if (model.type === 'image') return null
  if (!model.api_endpoint) return '未配置 Endpoint'
  if (!model.api_key_ref) return '未配置 API Key'
  return null
}

// ─── Model Table ──────────────────────────────────────────────────────────────

function ModelTable({
  models,
  healthMap,
  onToggle,
  onSetDefault,
  onEdit,
  onTest,
  onDelete,
  testingId,
}: {
  models: Model[]
  healthMap: HealthMap
  onToggle: (id: number) => void
  onSetDefault: (id: number) => void
  onEdit: (model: Model) => void
  onTest: (id: number) => void
  onDelete: (id: number) => void
  testingId: number | null
}) {
  return (
    <div className="overflow-x-auto rounded-xl border border-zinc-200 bg-white">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b border-zinc-100 bg-zinc-50">
            <th className="px-4 py-3 text-left text-xs font-medium text-zinc-500 w-6"></th>
            <th className="px-4 py-3 text-left text-xs font-medium text-zinc-500">名称 / Key</th>
            <th className="px-4 py-3 text-left text-xs font-medium text-zinc-500">服务商</th>
            <th className="px-4 py-3 text-left text-xs font-medium text-zinc-500">速度</th>
            <th className="px-4 py-3 text-left text-xs font-medium text-zinc-500">访问链接</th>
            <th className="px-4 py-3 text-left text-xs font-medium text-zinc-500">可用状态</th>
            <th className="px-4 py-3 text-right text-xs font-medium text-zinc-500">操作</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-zinc-100">
          {models.map((model) => {
            const health = healthMap[model.name]
            const reason = getUnavailableReason(model, health)
            const isAvailable = reason === null
            const isTesting = testingId === model.id

            return (
              <tr
                key={model.id}
                className={`group transition-colors hover:bg-zinc-50 ${!model.is_active ? 'opacity-60' : ''}`}
              >
                {/* Health dot */}
                <td className="px-4 py-3">
                  <div className="flex items-center gap-1.5">
                    <HealthDot status={health} />
                    {model.is_default && (
                      <Star className="h-3 w-3 text-amber-400" fill="currentColor" />
                    )}
                  </div>
                </td>

                {/* Name + Key */}
                <td className="px-4 py-3">
                  <div className="font-medium text-zinc-900">{model.name}</div>
                  <div className="mt-0.5 font-mono text-[11px] text-zinc-400">{model.model_key}</div>
                  {model.capability_tags?.length > 0 && (
                    <div className="mt-1 flex flex-wrap gap-1">
                      {model.capability_tags.slice(0, 2).map((tag) => (
                        <Badge key={tag} variant="outline" className="text-[10px] font-normal px-1 py-0">
                          {tag}
                        </Badge>
                      ))}
                      {model.capability_tags.length > 2 && (
                        <Badge variant="outline" className="text-[10px] font-normal px-1 py-0">
                          +{model.capability_tags.length - 2}
                        </Badge>
                      )}
                    </div>
                  )}
                </td>

                {/* Provider */}
                <td className="px-4 py-3">
                  <Badge variant="secondary" className="font-normal">{model.provider}</Badge>
                </td>

                {/* Speed */}
                <td className="px-4 py-3">
                  <SpeedBadge rating={model.speed_rating} />
                </td>

                {/* Access URL (T1) */}
                <td className="px-4 py-3 max-w-[200px]">
                  {model.api_endpoint ? (
                    <a
                      href={model.api_endpoint}
                      target="_blank"
                      rel="noreferrer"
                      className="inline-flex items-center gap-1 truncate text-xs text-primary-600 hover:text-primary-800 hover:underline"
                      title={model.api_endpoint}
                    >
                      <Link2 className="h-3 w-3 shrink-0" />
                      <span className="truncate max-w-[160px]">{model.api_endpoint}</span>
                    </a>
                  ) : (
                    <span className="text-xs text-zinc-300 italic">未配置</span>
                  )}
                </td>

                {/* Availability Status (T1) */}
                <td className="px-4 py-3">
                  {isAvailable ? (
                    <Badge className="bg-emerald-100 text-emerald-800 gap-1">
                      <CheckCircle2 className="h-3 w-3" />
                      可用
                    </Badge>
                  ) : (
                    <div className="space-y-1">
                      <Badge className="bg-red-100 text-red-700 gap-1">
                        <AlertTriangle className="h-3 w-3" />
                        不可用
                      </Badge>
                      {reason && (
                        <div className="text-[11px] text-zinc-400">{reason}</div>
                      )}
                    </div>
                  )}
                </td>

                {/* Actions */}
                <td className="px-4 py-3">
                  <div className="flex items-center justify-end gap-1">
                    <Toggle
                      checked={model.is_active}
                      onChange={() => onToggle(model.id)}
                    />
                    {!model.is_default && (
                      <Button
                        variant="ghost"
                        size="sm"
                        className="h-7 w-7 p-0 text-zinc-400 hover:text-amber-500"
                        onClick={() => onSetDefault(model.id)}
                        title="设为默认"
                      >
                        <Star className="h-3.5 w-3.5" />
                      </Button>
                    )}
                    <Button
                      variant="ghost"
                      size="sm"
                      className="h-7 w-7 p-0 text-zinc-400 hover:text-zinc-700"
                      onClick={() => onEdit(model)}
                      title="编辑"
                    >
                      <Pencil className="h-3.5 w-3.5" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="sm"
                      className="h-7 w-7 p-0 text-zinc-400 hover:text-zinc-700"
                      onClick={() => onTest(model.id)}
                      disabled={isTesting}
                      title="测试连接"
                    >
                      {isTesting ? (
                        <Loader2 className="h-3.5 w-3.5 animate-spin" />
                      ) : (
                        <Activity className="h-3.5 w-3.5" />
                      )}
                    </Button>
                    <Button
                      variant="ghost"
                      size="sm"
                      className="h-7 w-7 p-0 text-red-400 hover:bg-red-50 hover:text-red-600"
                      onClick={() => onDelete(model.id)}
                      title="删除"
                    >
                      <Trash2 className="h-3.5 w-3.5" />
                    </Button>
                  </div>
                </td>
              </tr>
            )
          })}
        </tbody>
      </table>
    </div>
  )
}

function OfficialCatalogCard({
  assessment,
  onUpsert,
  busy,
}: {
  assessment: ModelFeasibilityAssessment
  onUpsert: (item: OfficialModelCatalogItem) => void
  busy: boolean
}) {
  const { item } = assessment
  const statusCfg = CATALOG_STATUS_CONFIG[item.catalog_status]
  const integrationCfg = INTEGRATION_STATUS_CONFIG[item.integration_status]
  const validationCfg = getValidationStatusMeta(assessment.status)
  const capabilityLabels = getModelCapabilityLabels(item)
  const availabilityLabels = getAssessmentAvailabilityLabels(assessment)

  return (
    <Card className="border-zinc-200">
      <CardHeader className="space-y-3 pb-3">
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0">
            <div className="flex items-center gap-2">
              <h3 className="truncate text-sm font-semibold text-zinc-900">{item.name}</h3>
              {assessment.is_imported ? (
                <Badge className="bg-emerald-100 text-emerald-800">已入库</Badge>
              ) : (
                <Badge variant="outline">未入库</Badge>
              )}
            </div>
            <p className="mt-1 truncate font-mono text-xs text-zinc-500">{item.model_key}</p>
          </div>
          {assessment.is_imported ? (
            <CheckCircle2 className="h-4 w-4 shrink-0 text-emerald-500" />
          ) : (
            <CircleAlert className="h-4 w-4 shrink-0 text-zinc-400" />
          )}
        </div>
        <div className="flex flex-wrap gap-2">
          <Badge variant="outline">{assessment.product_label}</Badge>
          <Badge variant="secondary">{item.provider_label}</Badge>
          <Badge className={statusCfg.color}>{statusCfg.label}</Badge>
          <Badge className={integrationCfg.color}>{integrationCfg.label}</Badge>
          <Badge className={validationCfg.color}>{validationCfg.label}</Badge>
        </div>
      </CardHeader>

      <CardContent className="space-y-3 pt-0">
        <p className="text-xs leading-5 text-zinc-600">{item.description}</p>

        <div className="space-y-1 text-xs text-zinc-500">
          <div>{integrationCfg.description}</div>
          {item.context_window ? <div>上下文: {(item.context_window / 1000).toFixed(0)}K tokens</div> : null}
          {item.max_resolution ? <div>最大分辨率: {item.max_resolution}</div> : null}
          {item.runtime_alias ? <div>当前运行别名: {item.runtime_alias}</div> : null}
        </div>

        <div className="space-y-2">
          <div className="text-[11px] font-medium uppercase tracking-wide text-zinc-500">生成能力</div>
          <div className="flex flex-wrap gap-1">
            {capabilityLabels.map((label) => (
              <Badge key={label} variant="outline" className="text-[11px] font-normal">
                {label}
              </Badge>
            ))}
          </div>
        </div>

        <div className="space-y-2 rounded-md bg-zinc-50 p-3">
          <div className="text-[11px] font-medium uppercase tracking-wide text-zinc-500">可用部分</div>
          {availabilityLabels.length > 0 ? (
            <div className="flex flex-wrap gap-1">
              {availabilityLabels.map((label) => (
                <Badge key={label} variant="outline" className="text-[11px] font-normal">
                  {label}
                </Badge>
              ))}
            </div>
          ) : (
            <div className="text-xs text-zinc-500">当前项目还没有这条模型的可用链路。</div>
          )}
          <div className="pt-1 text-[11px] font-medium uppercase tracking-wide text-zinc-500">可行性说明</div>
          <ul className="space-y-1 text-xs leading-5 text-zinc-600">
            {assessment.reasons.slice(0, 3).map((reason) => (
              <li key={reason}>• {reason}</li>
            ))}
          </ul>
          {assessment.coverage.length > 0 ? (
            <div className="flex flex-wrap gap-1">
              {assessment.coverage.map((entry) => (
                <Badge key={`${entry.kind}:${entry.provider}:${entry.scope_match}:${entry.base_url ?? ''}`} variant="outline" className="text-[11px] font-normal">
                  {entry.label}
                  {entry.scope_match === 'exact' ? ' · 精确命中' : ' · 同供应商'}
                </Badge>
              ))}
            </div>
          ) : null}
        </div>

        <div className="flex items-center justify-between gap-3 border-t border-zinc-100 pt-3">
          <a
            href={item.source_url}
            target="_blank"
            rel="noreferrer"
            className="inline-flex items-center gap-1 text-xs text-primary-600 hover:text-primary-700"
          >
            {item.source_label}
            <ExternalLink className="h-3.5 w-3.5" />
          </a>
          <Button size="sm" variant={assessment.is_imported ? 'outline' : 'default'} onClick={() => onUpsert(item)} disabled={busy}>
            {busy ? <Loader2 className="mr-2 h-3.5 w-3.5 animate-spin" /> : <CloudDownload className="mr-2 h-3.5 w-3.5" />}
            {assessment.is_imported ? '更新入库' : '加入模型库'}
          </Button>
        </div>
      </CardContent>
    </Card>
  )
}

function ProductSummaryCard({ assessment }: { assessment: ProductFeasibilityAssessment }) {
  const statusCfg = getValidationStatusMeta(assessment.status)
  const capabilityLabels = getProductCapabilityLabels(assessment)
  const availabilityLabels = getProductAvailabilityLabels(assessment)

  return (
    <Card className="border-zinc-200">
      <CardHeader className="space-y-2 pb-3">
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0">
            <h3 className="truncate text-sm font-semibold text-zinc-900">{assessment.product_label}</h3>
            <p className="mt-1 text-xs text-zinc-500">
              {assessment.provider_label} · {assessment.model_count} 个官方模型
            </p>
          </div>
          <Badge className={statusCfg.color}>{statusCfg.label}</Badge>
        </div>
        <div className="flex flex-wrap gap-2">
          <Badge variant="secondary">{getProviderLabel(assessment.provider)}</Badge>
          {assessment.native_count > 0 ? <Badge className="bg-emerald-100 text-emerald-800">原生 {assessment.native_count}</Badge> : null}
          {assessment.imported_count > 0 ? <Badge variant="outline">已入库 {assessment.imported_count}</Badge> : null}
        </div>
      </CardHeader>

      <CardContent className="space-y-3 pt-0">
        <div className="grid grid-cols-4 gap-2 text-center">
          <div className="rounded-md bg-zinc-50 p-2">
            <div className="text-base font-semibold text-zinc-900">{assessment.verified_count}</div>
            <div className="text-[11px] text-zinc-500">已验证</div>
          </div>
          <div className="rounded-md bg-zinc-50 p-2">
            <div className="text-base font-semibold text-zinc-900">{assessment.ready_count}</div>
            <div className="text-[11px] text-zinc-500">可接入</div>
          </div>
          <div className="rounded-md bg-zinc-50 p-2">
            <div className="text-base font-semibold text-zinc-900">{assessment.partial_count}</div>
            <div className="text-[11px] text-zinc-500">部分可行</div>
          </div>
          <div className="rounded-md bg-zinc-50 p-2">
            <div className="text-base font-semibold text-zinc-900">{assessment.blocked_count}</div>
            <div className="text-[11px] text-zinc-500">待补齐</div>
          </div>
        </div>

        <div className="space-y-1">
          <div className="text-[11px] font-medium uppercase tracking-wide text-zinc-500">生成能力</div>
          <div className="flex flex-wrap gap-1">
            {capabilityLabels.map((label) => (
              <Badge key={label} variant="outline" className="text-[11px] font-normal">
                {label}
              </Badge>
            ))}
          </div>
        </div>

        <div className="space-y-1">
          <div className="text-[11px] font-medium uppercase tracking-wide text-zinc-500">可用部分</div>
          <div className="flex flex-wrap gap-1">
            {availabilityLabels.map((label) => (
              <Badge key={label} variant="outline" className="text-[11px] font-normal">
                {label}
              </Badge>
            ))}
          </div>
        </div>

        {assessment.channels.length > 0 ? (
          <div className="space-y-1">
            <div className="text-[11px] font-medium uppercase tracking-wide text-zinc-500">命中渠道</div>
            <div className="flex flex-wrap gap-1">
              {assessment.channels.map((channel) => (
                <Badge key={channel} variant="outline" className="text-[11px] font-normal">
                  {channel}
                </Badge>
              ))}
            </div>
          </div>
        ) : (
          <div className="rounded-md bg-zinc-50 p-2 text-xs text-zinc-500">
            当前项目尚未发现该产品家族的可用渠道或运行实例。
          </div>
        )}

        <div className="flex flex-wrap gap-1">
          {assessment.models.slice(0, 4).map((model) => {
            const modelStatus = getValidationStatusMeta(model.status)
            return (
              <Badge key={model.item.model_key} className={`${modelStatus.color} text-[11px] font-normal`}>
                {model.item.name}
              </Badge>
            )
          })}
          {assessment.models.length > 4 ? (
            <Badge variant="outline" className="text-[11px] font-normal">
              +{assessment.models.length - 4} 个
            </Badge>
          ) : null}
        </div>
      </CardContent>
    </Card>
  )
}

// ─── Model Form Dialog ────────────────────────────────────────────────────────

function ModelFormDialog({
  open,
  onOpenChange,
  model,
  activeType,
  onSave,
  saving,
}: {
  open: boolean
  onOpenChange: (v: boolean) => void
  model: Partial<Model> | null
  activeType: ModelType
  onSave: (data: Partial<Model>) => void
  saving: boolean
}) {
  const isEdit = model !== null && 'id' in model && typeof model.id === 'number'
  const initial: Partial<Model> = model ?? { ...EMPTY_FORM, type: activeType }

  const [form, setForm] = useState<Partial<Model>>(initial)
  const [tagsInput, setTagsInput] = useState((initial.capability_tags ?? []).join(', '))
  const [ratiosInput, setRatiosInput] = useState((initial.supported_ratios ?? []).join(', '))

  // Reset form when dialog opens with different model
  useEffect(() => {
    const src = model ?? { ...EMPTY_FORM, type: activeType }
    setForm(src)
    setTagsInput((src.capability_tags ?? []).join(', '))
    setRatiosInput((src.supported_ratios ?? []).join(', '))
  }, [model, activeType, open])

  const patch = (updates: Partial<Model>) => setForm((prev) => ({ ...prev, ...updates }))

  const handleSubmit = () => {
    const tags = tagsInput
      .split(/[,，]/)
      .map((s) => s.trim())
      .filter(Boolean)
    const ratios = ratiosInput
      .split(/[,，]/)
      .map((s) => s.trim())
      .filter(Boolean)
    onSave({ ...form, capability_tags: tags, supported_ratios: ratios, type: activeType })
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[85vh] overflow-y-auto sm:max-w-xl">
        <DialogHeader>
          <DialogTitle>{isEdit ? '编辑模型' : '添加模型'}</DialogTitle>
          <DialogDescription>
            {isEdit ? `修改 ${model?.name} 的配置` : `添加新的${MODEL_TYPES.find((t) => t.value === activeType)?.label}`}
          </DialogDescription>
        </DialogHeader>

        <div className="grid gap-4 py-4">
          {/* Row 1: Name + Model Key */}
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1.5">
              <Label className="text-xs">名称 *</Label>
              <Input
                value={form.name ?? ''}
                onChange={(e) => patch({ name: e.target.value })}
                placeholder="GPT-4o"
              />
            </div>
            <div className="space-y-1.5">
              <Label className="text-xs">
                Model Key *{isEdit && <span className="ml-1 text-zinc-400">(不可修改)</span>}
              </Label>
              <Input
                value={form.model_key ?? ''}
                onChange={(e) => patch({ model_key: e.target.value })}
                placeholder="gpt-4o"
                disabled={isEdit}
              />
            </div>
          </div>

          {/* Row 2: Provider + Speed */}
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1.5">
              <Label className="text-xs">Provider *</Label>
              <Input
                value={form.provider ?? ''}
                onChange={(e) => patch({ provider: e.target.value })}
                placeholder="OpenAI"
              />
            </div>
            <div className="space-y-1.5">
              <Label className="text-xs">速度评级 *</Label>
              <Select
                value={form.speed_rating ?? 'balanced'}
                onValueChange={(v) => patch({ speed_rating: v as SpeedRating })}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="fast">快速 (Fast)</SelectItem>
                  <SelectItem value="balanced">均衡 (Balanced)</SelectItem>
                  <SelectItem value="quality">高质量 (Quality)</SelectItem>
                </SelectContent>
              </Select>
            </div>
          </div>

          {/* Row 3: Pricing */}
          <div className="grid grid-cols-3 gap-3">
            <div className="space-y-1.5">
              <Label className="text-xs">单价</Label>
              <Input
                type="number"
                step="0.001"
                value={form.cost_per_unit ?? 0}
                onChange={(e) => patch({ cost_per_unit: parseFloat(e.target.value) || 0 })}
              />
            </div>
            <div className="space-y-1.5">
              <Label className="text-xs">计价单位</Label>
              <Input
                value={form.unit ?? ''}
                onChange={(e) => patch({ unit: e.target.value })}
                placeholder="1K tokens"
              />
            </div>
            <div className="space-y-1.5">
              <Label className="text-xs">输入价格</Label>
              <Input
                type="number"
                step="0.001"
                value={form.input_price ?? 0}
                onChange={(e) => patch({ input_price: parseFloat(e.target.value) || 0 })}
              />
            </div>
          </div>

          {/* Row 4: text model specifics */}
          {activeType === 'llm' && (
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label className="text-xs">输出价格</Label>
                <Input
                  type="number"
                  step="0.001"
                  value={form.output_price ?? 0}
                  onChange={(e) => patch({ output_price: parseFloat(e.target.value) || 0 })}
                />
              </div>
              <div className="space-y-1.5">
                <Label className="text-xs">上下文窗口</Label>
                <Input
                  type="number"
                  value={form.context_window ?? ''}
                  onChange={(e) => patch({ context_window: parseInt(e.target.value) || undefined })}
                  placeholder="128000"
                />
              </div>
            </div>
          )}

          {/* Row: image model specifics */}
          {activeType === 'image' && (
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label className="text-xs">一致性支持</Label>
                <Select
                  value={form.supports_consistency ? 'yes' : 'no'}
                  onValueChange={(v) => patch({ supports_consistency: v === 'yes' })}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="yes">支持</SelectItem>
                    <SelectItem value="no">不支持</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1.5">
                <Label className="text-xs">一致性方法</Label>
                <Select
                  value={form.consistency_method ?? 'none'}
                  onValueChange={(v) => patch({ consistency_method: v as Model['consistency_method'] })}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="none">无</SelectItem>
                    <SelectItem value="ip_adapter">IP-Adapter</SelectItem>
                    <SelectItem value="lora">LoRA</SelectItem>
                    <SelectItem value="reference_image">参考图</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>
          )}

          {/* Row: video model specifics */}
          {activeType === 'video' && (
            <div className="space-y-1.5">
              <Label className="text-xs">视频模式</Label>
              <Select
                value={form.video_mode ?? 'api_generation'}
                onValueChange={(v) => patch({ video_mode: v as Model['video_mode'] })}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="frame_animation">帧动画</SelectItem>
                  <SelectItem value="api_generation">API 生成</SelectItem>
                  <SelectItem value="both">混合模式</SelectItem>
                </SelectContent>
              </Select>
            </div>
          )}

          {/* Row: Resolution + Ratios */}
          {(activeType === 'image' || activeType === 'video') && (
            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label className="text-xs">最大分辨率</Label>
                <Input
                  value={form.max_resolution ?? ''}
                  onChange={(e) => patch({ max_resolution: e.target.value })}
                  placeholder="2048×2048"
                />
              </div>
              <div className="space-y-1.5">
                <Label className="text-xs">支持比例 (逗号分隔)</Label>
                <Input
                  value={ratiosInput}
                  onChange={(e) => setRatiosInput(e.target.value)}
                  placeholder="16:9, 9:16, 1:1"
                />
              </div>
            </div>
          )}

          {/* Tags */}
          <div className="space-y-1.5">
            <Label className="text-xs">能力标签 (逗号分隔)</Label>
            <Input
              value={tagsInput}
              onChange={(e) => setTagsInput(e.target.value)}
              placeholder="reasoning, vision, consistency"
            />
          </div>

          {/* API config */}
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1.5">
              <Label className="text-xs">API Endpoint</Label>
              <Input
                value={form.api_endpoint ?? ''}
                onChange={(e) => patch({ api_endpoint: e.target.value })}
                placeholder="https://api.openai.com/v1"
              />
            </div>
            <div className="space-y-1.5">
              <Label className="text-xs">API Key 引用</Label>
              <Input
                value={form.api_key_ref ?? ''}
                onChange={(e) => patch({ api_key_ref: e.target.value })}
                placeholder="OPENAI_API_KEY"
              />
            </div>
          </div>

          {/* Priority */}
          <div className="grid grid-cols-2 gap-3">
            <div className="space-y-1.5">
              <Label className="text-xs">优先级</Label>
              <Input
                type="number"
                value={form.priority ?? 0}
                onChange={(e) => patch({ priority: parseInt(e.target.value) || 0 })}
              />
            </div>
          </div>

          {/* Description */}
          <div className="space-y-1.5">
            <Label className="text-xs">描述 / 备注</Label>
            <textarea
              className="flex min-h-[60px] w-full rounded-md border border-zinc-300 bg-white px-3 py-2 text-sm placeholder:text-zinc-400 focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50"
              value={form.description ?? ''}
              onChange={(e) => patch({ description: e.target.value })}
              placeholder="管理员备注..."
              rows={2}
            />
          </div>
        </div>

        <DialogFooter>
          <Button variant="ghost" onClick={() => onOpenChange(false)} disabled={saving}>
            取消
          </Button>
          <Button onClick={handleSubmit} disabled={saving || !form.name || !form.model_key || !form.provider}>
            {saving ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : null}
            {isEdit ? '保存' : '添加'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// ─── Delete Confirm Dialog ────────────────────────────────────────────────────

function DeleteConfirmDialog({
  open,
  onOpenChange,
  onConfirm,
  deleting,
}: {
  open: boolean
  onOpenChange: (v: boolean) => void
  onConfirm: () => void
  deleting: boolean
}) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-sm">
        <DialogHeader>
          <DialogTitle>确认删除</DialogTitle>
          <DialogDescription>删除后无法恢复。如果模型正被项目引用，建议禁用而非删除。</DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="ghost" onClick={() => onOpenChange(false)} disabled={deleting}>
            取消
          </Button>
          <Button variant="destructive" onClick={onConfirm} disabled={deleting}>
            {deleting ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : null}
            删除
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// ─── Main Page ────────────────────────────────────────────────────────────────

export default function ModelsPage() {
  const { toast } = useToast()
  const [activeType, setActiveType] = useState<ModelType>('llm')
  const [editingModel, setEditingModel] = useState<Model | null>(null)
  const [showAddDialog, setShowAddDialog] = useState(false)
  const [saving, setSaving] = useState(false)
  const [testingId, setTestingId] = useState<number | null>(null)
  const [deletingId, setDeletingId] = useState<number | null>(null)
  const [confirmDeleteId, setConfirmDeleteId] = useState<number | null>(null)
  const [catalogBusyKey, setCatalogBusyKey] = useState<string | null>(null)
  const [syncingCatalog, setSyncingCatalog] = useState(false)

  // Filters
  const [providerFilter, setProviderFilter] = useState<string>('')
  const [speedFilter, setSpeedFilter] = useState<string>('')
  const [enabledFilter, setEnabledFilter] = useState<string>('')
  const [searchQuery, setSearchQuery] = useState<string>('')

  // Data fetching
  const { data: modelsData, isLoading, mutate } = useSWR<{ data: Model[] }>(
    ['models', activeType],
    () => modelAPI.list({ type: activeType, sort_by: 'priority' }) as unknown as Promise<{ data: Model[] }>,
    { refreshInterval: 30000 }
  )
  const { data: healthData } = useSWR<{ data: HealthMap }>(
    'model-health',
    () => modelAPI.health() as unknown as Promise<{ data: HealthMap }>,
    { refreshInterval: 30000 }
  )
  const { data: systemKeysData } = useSWR<{ data: SystemAPIKey[] }>(
    'system-api-keys',
    () => authAPI.listSystemAPIKeys() as unknown as Promise<{ data: SystemAPIKey[] }>
  )
  const { data: userKeysData } = useSWR<{ data: APIKey[] }>(
    'user-api-keys',
    () => authAPI.listAPIKeys() as unknown as Promise<{ data: APIKey[] }>
  )

  const allModels: Model[] = (modelsData as { data?: Model[] | { items?: Model[] } })?.data
    ? Array.isArray((modelsData as any).data)
      ? (modelsData as any).data
      : (modelsData as any).data?.items ?? []
    : []

  const healthMap: HealthMap = (healthData as { data?: HealthMap })?.data ?? {}
  const systemKeys: SystemAPIKey[] = Array.isArray((systemKeysData as { data?: SystemAPIKey[] })?.data)
    ? ((systemKeysData as { data?: SystemAPIKey[] }).data ?? [])
    : []
  const userKeys: APIKey[] = Array.isArray((userKeysData as { data?: APIKey[] })?.data)
    ? ((userKeysData as { data?: APIKey[] }).data ?? [])
    : []
  const catalogModels = OFFICIAL_MODEL_CATALOG.filter((item) => item.type === activeType)
  const catalogAssessments = useMemo(
    () =>
      buildModelFeasibilityAssessments({
        catalog: catalogModels,
        runtimeModels: allModels,
        healthMap,
        systemKeys,
        userKeys,
      }),
    [allModels, catalogModels, healthMap, systemKeys, userKeys]
  )
  const productAssessments = useMemo(
    () => buildProductFeasibilityAssessments(catalogAssessments),
    [catalogAssessments]
  )

  // Derived: unique providers for filter
  const providers = Array.from(
    new Set([
      ...allModels.map((m) => m.provider).filter(Boolean),
      ...catalogModels.map((m) => m.provider).filter(Boolean),
    ])
  )

  const visibleCatalogAssessments = catalogAssessments.filter(({ item }) => {
    if (providerFilter && item.provider !== providerFilter) return false
    if (speedFilter && item.speed_rating !== speedFilter) return false
    return true
  })
  const visibleCatalogModels = visibleCatalogAssessments.map((entry) => entry.item)
  const visibleProductAssessments = productAssessments.filter((entry) => {
    if (providerFilter && entry.provider !== providerFilter) return false
    return true
  })

  const findExistingCatalogModel = (item: OfficialModelCatalogItem) =>
    allModels.find((m) => m.provider === item.provider && m.model_key === item.model_key)

  const importedCatalogCount = catalogModels.filter((item) => findExistingCatalogModel(item)).length
  const verifiedCatalogCount = catalogAssessments.filter((item) => item.status === 'verified').length
  const blockedCatalogCount = catalogAssessments.filter((item) => item.status === 'blocked').length

  // Filtered models
  const filteredModels = allModels.filter((m) => {
    if (providerFilter && m.provider !== providerFilter) return false
    if (speedFilter && m.speed_rating !== speedFilter) return false
    if (enabledFilter === 'enabled' && !m.is_active) return false
    if (enabledFilter === 'disabled' && m.is_active) return false
    if (searchQuery) {
      const q = searchQuery.toLowerCase()
      if (!m.name.toLowerCase().includes(q) && !m.model_key.toLowerCase().includes(q) && !m.provider.toLowerCase().includes(q)) return false
    }
    return true
  })

  // ── Handlers ──────────────────────────────────────────────────────────────

  const handleToggle = useCallback(
    async (id: number) => {
      try {
        await modelAPI.toggle(id)
        await mutate()
        toast({ title: '状态已切换', variant: 'success' })
      } catch {
        toast({ title: '操作失败', description: '切换模型状态时出错', variant: 'destructive' })
      }
    },
    [mutate, toast]
  )

  const handleSetDefault = useCallback(
    async (id: number) => {
      try {
        await modelAPI.setDefault(id)
        await mutate()
        toast({ title: '已设为默认模型', variant: 'success' })
      } catch {
        toast({ title: '操作失败', description: '设置默认模型时出错', variant: 'destructive' })
      }
    },
    [mutate, toast]
  )

  const handleTest = useCallback(
    async (id: number) => {
      setTestingId(id)
      try {
        await modelAPI.test(id)
        toast({ title: '连接测试通过', description: '模型响应正常', variant: 'success' })
      } catch {
        toast({ title: '连接测试失败', description: '无法连接到模型服务', variant: 'destructive' })
      } finally {
        setTestingId(null)
      }
    },
    [toast]
  )

  const handleDelete = useCallback(
    async (id: number) => {
      setDeletingId(id)
      try {
        await modelAPI.delete(id)
        await mutate()
        toast({ title: '模型已删除', variant: 'success' })
      } catch (err: any) {
        const msg = err?.response?.data?.detail ?? '删除模型时出错'
        toast({ title: '删除失败', description: msg, variant: 'destructive' })
      } finally {
        setDeletingId(null)
        setConfirmDeleteId(null)
      }
    },
    [mutate, toast]
  )

  const handleSave = useCallback(
    async (data: Partial<Model>) => {
      setSaving(true)
      try {
        if (editingModel?.id) {
          await modelAPI.update(editingModel.id, data)
          toast({ title: '模型已更新', variant: 'success' })
        } else {
          await modelAPI.create(data)
          toast({ title: '模型已添加', variant: 'success' })
        }
        await mutate()
        setEditingModel(null)
        setShowAddDialog(false)
      } catch (err: any) {
        const msg = err?.response?.data?.detail ?? '保存模型时出错'
        toast({ title: '保存失败', description: msg, variant: 'destructive' })
      } finally {
        setSaving(false)
      }
    },
    [editingModel, mutate, toast]
  )

  const handleCatalogUpsert = useCallback(
    async (item: OfficialModelCatalogItem) => {
      const existing = findExistingCatalogModel(item)
      const payload = buildModelPayloadFromCatalog(item)
      const busyKey = `${item.provider}:${item.model_key}`
      setCatalogBusyKey(busyKey)
      try {
        if (existing?.id) {
          await modelAPI.update(existing.id, payload)
          toast({ title: '官方模型已更新', description: `${item.name} 已同步到模型库`, variant: 'success' })
        } else {
          await modelAPI.create(payload)
          toast({ title: '官方模型已加入', description: `${item.name} 已写入模型库`, variant: 'success' })
        }
        await mutate()
      } catch (err: any) {
        const msg = err?.response?.data?.detail ?? '同步官方模型时出错'
        toast({ title: '同步失败', description: msg, variant: 'destructive' })
      } finally {
        setCatalogBusyKey(null)
      }
    },
    [mutate, toast, allModels]
  )

  const handleSyncCatalog = useCallback(async () => {
    if (visibleCatalogModels.length === 0) {
      toast({ title: '当前分类没有可同步的官方模型', variant: 'destructive' })
      return
    }

    setSyncingCatalog(true)
    let created = 0
    let updated = 0
    let failed = 0

    try {
      for (const item of visibleCatalogModels) {
        const existing = findExistingCatalogModel(item)
        const payload = buildModelPayloadFromCatalog(item)
        try {
          if (existing?.id) {
            await modelAPI.update(existing.id, payload)
            updated += 1
          } else {
            await modelAPI.create(payload)
            created += 1
          }
        } catch {
          failed += 1
        }
      }
      await mutate()
      toast({
        title: '官方目录同步完成',
        description: `新增 ${created} 个，更新 ${updated} 个，失败 ${failed} 个`,
        variant: failed > 0 ? 'destructive' : 'success',
      })
    } finally {
      setSyncingCatalog(false)
    }
  }, [mutate, toast, visibleCatalogModels, allModels])

  const handleTabChange = (value: string) => {
    setActiveType(value as ModelType)
    setProviderFilter('')
    setSpeedFilter('')
    setEnabledFilter('')
    setSearchQuery('')
  }

  // ── Render ────────────────────────────────────────────────────────────────

  return (
    <div className="space-y-6">
      <DashboardHero
        badge="AI 模型控制台"
        badgeIcon={<Activity className="h-3.5 w-3.5 text-violet-300" />}
        title="模型管理"
        description="管理平台使用的 AI 模型，并对照官方目录核验当前项目的接入范围与可行性。"
        gradientClassName="from-slate-950 via-violet-950 to-slate-900"
        contentClassName="max-w-3xl"
        stats={[
          {
            icon: <CloudDownload className="h-4 w-4 text-cyan-300" />,
            label: '官方目录',
            value: catalogModels.length,
            description: '当前分类官方公开模型数',
          },
          {
            icon: <CheckCircle2 className="h-4 w-4 text-emerald-300" />,
            label: '已入库',
            value: importedCatalogCount,
            description: '已与官方目录同步的模型',
          },
          {
            icon: <Activity className="h-4 w-4 text-amber-300" />,
            label: '产品家族',
            value: productAssessments.length,
            description: '按官方产品线归并后的列表数量',
          },
          {
            icon: <CircleAlert className="h-4 w-4 text-rose-300" />,
            label: '已验证 / 待补齐',
            value: `${verifiedCatalogCount} / ${blockedCatalogCount}`,
            description: '已验证健康状态 / 仍缺渠道或适配器',
          },
        ]}
      />

      {/* Tabs */}
      <Tabs value={activeType} onValueChange={handleTabChange}>
        <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
          <TabsList>
            {MODEL_TYPES.map(({ value, label, icon: Icon }) => (
              <TabsTrigger key={value} value={value} className="gap-1.5">
                <Icon className="h-4 w-4" />
                {label}
              </TabsTrigger>
            ))}
          </TabsList>

          <div className="flex flex-wrap items-center gap-2">
            <Button
              size="sm"
              variant="outline"
              onClick={handleSyncCatalog}
              disabled={syncingCatalog || visibleCatalogModels.length === 0}
              className="gap-1.5"
              title="按当前分类与筛选同步官方目录"
            >
              {syncingCatalog ? <Loader2 className="h-4 w-4 animate-spin" /> : <CloudDownload className="h-4 w-4" />}
              同步官方目录
            </Button>
            <Button size="sm" onClick={() => setShowAddDialog(true)} className="gap-1.5" title="添加新的 AI 模型">
              <Plus className="h-4 w-4" />
              添加模型
            </Button>
          </div>
        </div>

        <div className="mt-4 space-y-3">
          <div className="flex items-center justify-between">
            <div>
              <h3 className="text-sm font-semibold text-zinc-900">产品列表</h3>
              <p className="text-xs text-zinc-500">按官方产品线归并模型，并结合渠道配置与运行状态评估当前项目可行性。</p>
            </div>
            <div className="text-xs text-zinc-500">
              显示 {visibleProductAssessments.length} / {productAssessments.length}
            </div>
          </div>

          {visibleProductAssessments.length === 0 ? (
            <DashboardEmptyState
              icon={null}
              title="当前筛选条件下没有产品家族"
              className="rounded-[24px] border-0 bg-transparent p-0 shadow-none"
              innerClassName="rounded-[24px] border border-dashed border-zinc-300 bg-[radial-gradient(circle_at_top_left,_rgba(139,92,246,0.08),_transparent_30%),radial-gradient(circle_at_bottom_right,_rgba(59,130,246,0.08),_transparent_28%)] p-8 text-center text-sm text-zinc-500"
            />
          ) : (
            <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3">
              {visibleProductAssessments.map((assessment) => (
                <ProductSummaryCard key={assessment.product_key} assessment={assessment} />
              ))}
            </div>
          )}
        </div>

        {/* Filters */}
        <div className="mt-4 flex flex-wrap items-center gap-3">
          <div className="flex items-center gap-1.5 text-xs text-zinc-500">
            <Search className="h-3.5 w-3.5" />
            筛选:
          </div>

          {/* Name search (T1) */}
          <div className="relative">
            <Search className="absolute left-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-zinc-400" />
            <Input
              className="h-8 w-[180px] pl-8 text-xs"
              placeholder="搜索名称 / Key"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
            />
            {searchQuery && (
              <button
                className="absolute right-2 top-1/2 -translate-y-1/2 text-zinc-400 hover:text-zinc-600"
                onClick={() => setSearchQuery('')}
              >
                <X className="h-3 w-3" />
              </button>
            )}
          </div>

          <Select value={providerFilter || '_all'} onValueChange={(v) => setProviderFilter(v === '_all' ? '' : v)}>
            <SelectTrigger className="h-8 w-[140px] text-xs">
              <SelectValue placeholder="供应商" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="_all">全部供应商</SelectItem>
              {providers.map((p) => (
                <SelectItem key={p} value={p}>
                  {p}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>

          <Select value={speedFilter || '_all'} onValueChange={(v) => setSpeedFilter(v === '_all' ? '' : v)}>
            <SelectTrigger className="h-8 w-[120px] text-xs">
              <SelectValue placeholder="速度" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="_all">全部速度</SelectItem>
              <SelectItem value="fast">快速</SelectItem>
              <SelectItem value="balanced">均衡</SelectItem>
              <SelectItem value="quality">高质量</SelectItem>
            </SelectContent>
          </Select>

          <Select value={enabledFilter || '_all'} onValueChange={(v) => setEnabledFilter(v === '_all' ? '' : v)}>
            <SelectTrigger className="h-8 w-[120px] text-xs">
              <SelectValue placeholder="状态" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="_all">全部状态</SelectItem>
              <SelectItem value="enabled">可用</SelectItem>
              <SelectItem value="disabled">不可用</SelectItem>
            </SelectContent>
          </Select>

          {(providerFilter || speedFilter || enabledFilter || searchQuery) && (
            <Button
              variant="ghost"
              size="sm"
              className="h-8 gap-1 text-xs text-zinc-500"
              onClick={() => {
                setProviderFilter('')
                setSpeedFilter('')
                setEnabledFilter('')
                setSearchQuery('')
              }}
              title="清除所有筛选条件"
            >
              <X className="h-3 w-3" />
              清除
            </Button>
          )}
        </div>

        <div className="mt-4 space-y-3">
          <div className="flex items-center justify-between">
            <div>
              <h3 className="text-sm font-semibold text-zinc-900">官方目录</h3>
              <p className="text-xs text-zinc-500">按当前官方公开模型建立清单，并标注本项目内的真实可行性。</p>
            </div>
            <div className="text-xs text-zinc-500">
              显示 {visibleCatalogModels.length} / {catalogModels.length}
            </div>
          </div>

          {visibleCatalogModels.length === 0 ? (
            <DashboardEmptyState
              icon={null}
              title="当前筛选条件下没有官方模型"
              className="rounded-[24px] border-0 bg-transparent p-0 shadow-none"
              innerClassName="rounded-[24px] border border-dashed border-zinc-300 bg-[radial-gradient(circle_at_top_left,_rgba(139,92,246,0.08),_transparent_30%),radial-gradient(circle_at_bottom_right,_rgba(16,185,129,0.08),_transparent_28%)] p-8 text-center text-sm text-zinc-500"
            />
          ) : (
            <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3">
              {visibleCatalogAssessments.map((assessment) => {
                const busyKey = `${assessment.item.provider}:${assessment.item.model_key}`
                return (
                  <OfficialCatalogCard
                    key={busyKey}
                    assessment={assessment}
                    onUpsert={handleCatalogUpsert}
                    busy={syncingCatalog || catalogBusyKey === busyKey}
                  />
                )
              })}
            </div>
          )}
        </div>

        {/* Tab Content (shared across all types since data is driven by activeType) */}
        {MODEL_TYPES.map(({ value }) => (
          <TabsContent key={value} value={value} className="mt-4">
            <div className="mb-3 flex items-center justify-between">
              <div>
                <h3 className="text-sm font-semibold text-zinc-900">我的模型库</h3>
                <p className="text-xs text-zinc-500">当前已接入的模型列表，可直接启用/禁用或测试连接。</p>
              </div>
              <div className="text-xs text-zinc-500">
                显示 {filteredModels.length} / {allModels.length}
              </div>
            </div>
            {isLoading ? (
              <div className="flex h-48 items-center justify-center">
                <Loader2 className="h-6 w-6 animate-spin text-zinc-400" />
              </div>
            ) : filteredModels.length === 0 ? (
              <DashboardEmptyState
                icon={null}
                title={allModels.length === 0 ? '暂无模型' : '没有符合筛选条件的模型'}
                action={allModels.length === 0 ? (
                  <Button size="sm" variant="outline" onClick={() => setShowAddDialog(true)} className="gap-1.5" title="添加第一个模型">
                    <Plus className="h-3.5 w-3.5" />
                    添加第一个模型
                  </Button>
                ) : null}
                className="rounded-[24px] border-0 bg-transparent p-0 shadow-none"
                innerClassName="flex h-48 flex-col items-center justify-center gap-3 rounded-[24px] border border-dashed border-zinc-300 bg-[radial-gradient(circle_at_top_left,_rgba(139,92,246,0.08),_transparent_30%),radial-gradient(circle_at_bottom_right,_rgba(99,102,241,0.08),_transparent_28%)]"
              />
            ) : (
              <ModelTable
                models={filteredModels}
                healthMap={healthMap}
                onToggle={handleToggle}
                onSetDefault={handleSetDefault}
                onEdit={(m) => setEditingModel(m)}
                onTest={handleTest}
                onDelete={(id) => setConfirmDeleteId(id)}
                testingId={testingId}
              />
            )}
          </TabsContent>
        ))}
      </Tabs>

      {/* Edit Dialog */}
      <ModelFormDialog
        open={editingModel !== null}
        onOpenChange={(v) => { if (!v) setEditingModel(null) }}
        model={editingModel}
        activeType={activeType}
        onSave={handleSave}
        saving={saving}
      />

      {/* Add Dialog */}
      <ModelFormDialog
        open={showAddDialog}
        onOpenChange={setShowAddDialog}
        model={null}
        activeType={activeType}
        onSave={handleSave}
        saving={saving}
      />

      {/* Delete Confirmation */}
      <DeleteConfirmDialog
        open={confirmDeleteId !== null}
        onOpenChange={(v) => { if (!v) setConfirmDeleteId(null) }}
        onConfirm={() => confirmDeleteId !== null && handleDelete(confirmDeleteId)}
        deleting={deletingId !== null}
      />
    </div>
  )
}
