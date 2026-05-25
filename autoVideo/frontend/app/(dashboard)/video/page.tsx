'use client'

import { useEffect, useMemo, useState } from 'react'
import useSWR from 'swr'
import Link from 'next/link'
import { useSearchParams } from 'next/navigation'
import { Film, ImagePlus, Layers3, ArrowRightLeft, ScanFace, Sparkles, Volume2, CheckCircle2, XCircle, Loader2 } from 'lucide-react'
import { videoAPI } from '@/lib/api'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { cn } from '@/lib/utils'

type ModelParamValue = {
  value: string
  label: string
}

type ModelParamOption = {
  key: string
  label: string
  default: string
  values?: ModelParamValue[]
}

type VideoModelStatus = {
  key: string
  available: boolean
  native_audio?: boolean
  params?: ModelParamOption[]
}

type ManualMenuKey = 'text' | 'image' | 'reference' | 'start-end' | 'face-swap'

type ManualMenuDef = {
  key: ManualMenuKey
  label: string
  description: string
  icon: React.ComponentType<{ className?: string }>
}

const MANUAL_MENU_ITEMS: ManualMenuDef[] = [
  { key: 'text', label: '文生视频', description: '仅输入提示词生成视频，优先展示支持纯文本生成的模型。', icon: Film },
  { key: 'image', label: '图生视频', description: '上传单张首帧图片驱动视频生成。', icon: ImagePlus },
  { key: 'reference', label: '融合生视频', description: '基于参考图/角色图做主体一致性生成。', icon: Layers3 },
  { key: 'start-end', label: '首尾针视频', description: '同时指定首帧与尾帧，生成过渡视频。', icon: ArrowRightLeft },
  { key: 'face-swap', label: 'AI 换脸', description: '展示具备参考图/人物一致性基础能力的模型，作为换脸入口预留。', icon: ScanFace },
]

const TEXT_MODELS = new Set(['wan', 'wan-t2v', 'vidu', 'vidu-offpeak'])
const START_END_MODELS = new Set(['doubao', 'doubao-seedance', 'suanneng', 'vidu', 'vidu-offpeak', 'kling'])
const REFERENCE_MODELS = new Set(['doubao', 'doubao-seedance', 'suanneng', 'vidu', 'vidu-offpeak', 'kling', 'wan'])
const FACE_SWAP_MODELS = new Set(['doubao', 'doubao-seedance', 'suanneng', 'vidu', 'vidu-offpeak', 'kling', 'wan', 'comfyui-video'])

function hasGenerateMode(model: VideoModelStatus, mode: string) {
  return (model.params || []).some((param) =>
    param.key === 'generate_mode' && (param.values || []).some((item) => item.value === mode)
  )
}

function getModelDisplayName(key: string) {
  const map: Record<string, string> = {
    'wan': 'Wan 图生视频',
    'wan-t2v': 'Wan 文生视频',
    'vidu': 'Vidu',
    'vidu-offpeak': 'Vidu（离峰）',
    'kling': 'Kling',
    'tencent-vclm': 'Tencent VCLM / Kling 路由',
    'doubao': 'Doubao',
    'doubao-seedance': 'Doubao Seedance',
    'suanneng': '算能',
    'hubagi-voe3.1': 'Veo 3.1（Hubagi）',
    'hubagi-TC-GV': 'TC-GV（Hubagi）',
    'sora2': 'Sora 2',
    'comfyui-video': 'ComfyUI Video',
    'runninghub': 'RunningHub',
    'cogvideo': 'CogVideo',
    'baidu-bce': 'Baidu BCE',
    'gaga': 'Gaga',
    'aiping': '爱评',
  }
  return map[key] || key
}

function inferModelCategories(model: VideoModelStatus): ManualMenuKey[] {
  const categories = new Set<ManualMenuKey>()
  const key = model.key

  if (TEXT_MODELS.has(key) || hasGenerateMode(model, 'text2video')) {
    categories.add('text')
  }
  if (!TEXT_MODELS.has(key) || key === 'wan') {
    categories.add('image')
  }
  if (REFERENCE_MODELS.has(key) || hasGenerateMode(model, 'reference2video')) {
    categories.add('reference')
  }
  if (START_END_MODELS.has(key) || hasGenerateMode(model, 'startEnd2video')) {
    categories.add('start-end')
  }
  if (FACE_SWAP_MODELS.has(key) || hasGenerateMode(model, 'reference2video')) {
    categories.add('face-swap')
  }

  return Array.from(categories)
}

function capabilityHints(model: VideoModelStatus, categories: ManualMenuKey[]) {
  const hints: string[] = []
  if (categories.includes('text')) hints.push('支持文生视频')
  if (categories.includes('image')) hints.push('支持图生视频')
  if (categories.includes('reference')) hints.push('支持参考图/融合生成')
  if (categories.includes('start-end')) hints.push('支持首尾帧过渡')
  if (model.native_audio) hints.push('支持原生音频')
  if ((model.params || []).some((p) => p.key === 'aspect_ratio')) hints.push('可选画幅比例')
  if ((model.params || []).some((p) => p.key === 'resolution')) hints.push('可选分辨率')
  return hints
}

export default function VideoManualPage() {
  const searchParams = useSearchParams()
  const tabParam = searchParams.get('tab')
  const [activeMenu, setActiveMenu] = useState<ManualMenuKey>('text')
  const { data, isLoading } = useSWR('video-model-status', () => videoAPI.modelStatus())

  const models: VideoModelStatus[] = useMemo(() => {
    const list = (data as { models?: VideoModelStatus[] } | undefined)?.models || []
    return list
  }, [data])

  const grouped = useMemo(() => {
    const next: Record<ManualMenuKey, VideoModelStatus[]> = {
      text: [], image: [], reference: [], 'start-end': [], 'face-swap': [],
    }
    for (const model of models) {
      for (const category of inferModelCategories(model)) {
        next[category].push(model)
      }
    }
    return next
  }, [models])

  const activeModels = grouped[activeMenu] || []
  const activeMeta = MANUAL_MENU_ITEMS.find((item) => item.key === activeMenu) || MANUAL_MENU_ITEMS[0]

  useEffect(() => {
    const allowed = new Set<ManualMenuKey>(['text', 'image', 'reference', 'start-end', 'face-swap'])
    if (tabParam && allowed.has(tabParam as ManualMenuKey)) {
      setActiveMenu(tabParam as ManualMenuKey)
    }
  }, [tabParam])

  useEffect(() => {
    if (activeModels.length === 0) {
      const fallback = MANUAL_MENU_ITEMS.find((item) => (grouped[item.key] || []).length > 0)
      if (fallback && fallback.key !== activeMenu) {
        setActiveMenu(fallback.key)
      }
    }
  }, [activeMenu, activeModels.length, grouped])

  return (
    <div className="space-y-6">
      <div className="flex items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold text-white">手动创建视频</h1>
          <p className="mt-2 text-sm text-slate-300">
            按现有 video-service 运行态模型能力分组展示。这里只做能力导航与模型筛选入口，不伪造后端尚未落地的独立换脸链路。
          </p>
        </div>
        <Link
          href="/projects"
          className="inline-flex items-center gap-2 rounded-lg border border-white/10 bg-white/5 px-3 py-2 text-sm text-slate-200 transition hover:bg-white/10"
        >
          去项目生成链
        </Link>
      </div>

      <div className="grid gap-4 lg:grid-cols-[300px,minmax(0,1fr)]">
        <Card className="border-white/10 bg-slate-900/60 text-slate-100">
          <CardHeader>
            <CardTitle>手动创建视频</CardTitle>
            <CardDescription className="text-slate-400">二级菜单</CardDescription>
          </CardHeader>
          <CardContent className="space-y-2">
            {MANUAL_MENU_ITEMS.map((item) => {
              const Icon = item.icon
              const count = grouped[item.key]?.length || 0
              const active = item.key === activeMenu
              return (
                <button
                  key={item.key}
                  type="button"
                  onClick={() => setActiveMenu(item.key)}
                  className={cn(
                    'w-full rounded-xl border px-4 py-3 text-left transition',
                    active
                      ? 'border-cyan-400/50 bg-cyan-400/10 shadow-[0_0_0_1px_rgba(34,211,238,0.15)]'
                      : 'border-white/8 bg-white/[0.03] hover:bg-white/[0.06]'
                  )}
                >
                  <div className="flex items-center justify-between gap-3">
                    <div className="flex items-center gap-3">
                      <span className="flex h-9 w-9 items-center justify-center rounded-lg bg-white/5">
                        <Icon className="h-4 w-4" />
                      </span>
                      <div>
                        <div className="font-medium text-slate-100">{item.label}</div>
                        <div className="mt-1 text-xs text-slate-400">{item.description}</div>
                      </div>
                    </div>
                    <span className="rounded-full bg-white/8 px-2 py-1 text-xs text-slate-300">{count}</span>
                  </div>
                </button>
              )
            })}
          </CardContent>
        </Card>

        <div className="space-y-4">
          <Card className="border-white/10 bg-slate-900/60 text-slate-100">
            <CardHeader>
              <div className="flex items-center gap-3">
                <span className="flex h-10 w-10 items-center justify-center rounded-xl bg-cyan-400/10 text-cyan-300">
                  <activeMeta.icon className="h-5 w-5" />
                </span>
                <div>
                  <CardTitle>{activeMeta.label}</CardTitle>
                  <CardDescription className="text-slate-400">{activeMeta.description}</CardDescription>
                </div>
              </div>
            </CardHeader>
            <CardContent>
              {isLoading ? (
                <div className="flex items-center gap-2 text-sm text-slate-300">
                  <Loader2 className="h-4 w-4 animate-spin" /> 正在加载视频模型能力…
                </div>
              ) : activeModels.length === 0 ? (
                <div className="rounded-xl border border-amber-400/20 bg-amber-400/5 p-4 text-sm text-amber-100">
                  当前分类下没有可识别模型。若你希望这里不仅展示，还直接进入独立生成表单，我下一步可以继续把对应表单和任务创建链接上。
                </div>
              ) : (
                <div className="grid gap-4 xl:grid-cols-2">
                  {activeModels.map((model) => {
                    const categories = inferModelCategories(model)
                    const hints = capabilityHints(model, categories)
                    return (
                      <div key={`${activeMenu}-${model.key}`} className="rounded-2xl border border-white/10 bg-white/[0.03] p-4">
                        <div className="flex items-start justify-between gap-3">
                          <div>
                            <h3 className="text-base font-semibold text-white">{getModelDisplayName(model.key)}</h3>
                            <p className="mt-1 text-xs text-slate-400">模型键：{model.key}</p>
                          </div>
                          <span className={cn(
                            'inline-flex items-center gap-1 rounded-full px-2.5 py-1 text-xs',
                            model.available
                              ? 'bg-emerald-400/10 text-emerald-300'
                              : 'bg-rose-400/10 text-rose-300'
                          )}>
                            {model.available ? <CheckCircle2 className="h-3.5 w-3.5" /> : <XCircle className="h-3.5 w-3.5" />}
                            {model.available ? '可用' : '不可用'}
                          </span>
                        </div>

                        <div className="mt-3 flex flex-wrap gap-2">
                          {hints.map((hint) => (
                            <span key={hint} className="rounded-full bg-slate-800 px-2.5 py-1 text-[11px] text-slate-300">
                              {hint}
                            </span>
                          ))}
                          {hints.length === 0 && (
                            <span className="rounded-full bg-slate-800 px-2.5 py-1 text-[11px] text-slate-400">未暴露额外前端参数</span>
                          )}
                        </div>

                        {!!model.params?.length && (
                          <div className="mt-4 space-y-2">
                            <div className="text-xs font-medium uppercase tracking-wider text-slate-500">可配置参数</div>
                            <div className="space-y-2">
                              {model.params.map((param) => (
                                <div key={`${model.key}-${param.key}`} className="rounded-xl bg-slate-950/50 p-3">
                                  <div className="flex items-center justify-between gap-2 text-sm">
                                    <span className="text-slate-200">{param.label}</span>
                                    <span className="text-xs text-slate-500">默认：{param.default || '-'}</span>
                                  </div>
                                  {!!param.values?.length && (
                                    <div className="mt-2 flex flex-wrap gap-2">
                                      {param.values.map((value) => (
                                        <span key={`${param.key}-${value.value}`} className="rounded-md border border-white/10 px-2 py-1 text-[11px] text-slate-300">
                                          {value.label}
                                        </span>
                                      ))}
                                    </div>
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
              )}
            </CardContent>
          </Card>

          <Card className="border-white/10 bg-slate-900/60 text-slate-100">
            <CardHeader>
              <CardTitle>当前说明</CardTitle>
              <CardDescription className="text-slate-400">真实能力边界按现有后端代码收口</CardDescription>
            </CardHeader>
            <CardContent className="space-y-3 text-sm text-slate-300">
              <p>1. 文生视频当前明确识别到的真实链路主要是 <span className="text-cyan-300">Wan T2V</span> 与 <span className="text-cyan-300">Vidu text2video</span>。</p>
              <p>2. 图生视频、融合生视频、首尾针视频的能力识别主要来自 video-service generator 的 <code className="rounded bg-slate-950 px-1 py-0.5 text-xs">ParamOptions()</code> 与生成模式分支。</p>
              <p>3. AI 换脸目前前端先作为能力归类入口展示，底层仍是参考图/人物一致性链路，不在这里伪装成已完成的独立 face-swap 后端。</p>
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  )
}
