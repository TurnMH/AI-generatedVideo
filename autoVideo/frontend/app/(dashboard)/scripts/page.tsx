'use client'

import React, { useState } from 'react'
import useSWR from 'swr'
import { useRouter } from 'next/navigation'
import {
  BookOpen,
  Upload,
  Star,
  FileText,
  Plus,
  Search,
  Tag,
  User,
  Hash,
  ExternalLink,
  Wand2,
  Sparkles,
  Loader2,
  ScrollText,
  Library,
  Clapperboard,
  Film,
  List,
  MessageSquare,
  Copy,
  Download,
  RefreshCw,
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Separator } from '@/components/ui/separator'
import { useToast } from '@/components/ui/toast'
import { DashboardHero } from '@/components/layout/dashboard-hero'
import { DashboardEmptyState } from '@/components/layout/dashboard-empty-state'
import { modelAPI, scriptLibraryAPI } from '@/lib/api'
import { savePendingProjectDraft } from '@/lib/project-draft'
import type { Model, ScriptLibraryItem } from '@/types'

type TabKey = 'all' | 'uploaded' | 'showcase'
type AIGenerateMode = 'script' | 'novel_outline' | 'novel_chapter' | 'adaptation' | 'episode_outline' | 'scene_script' | 'dialogue_polish'

type AIFormState = {
  title: string
  genre: string
  platform: string
  delivery_format: string
  episode_duration: string
  reference_style: string
  premise: string
  character_setup: string
  world_setting: string
  outline: string
  chapter_brief: string
  source_text: string
  target_words: string
  chapter_count: string
  audience: string
  tone: string
  requirements: string
}

type AIResult = {
  title: string
  description: string
  genre: string
  tags: string[]
  outline: string[]
  content: string
  suggested_episodes: number
  word_count: number
  ending_state?: string
}

const AI_MODE_OPTIONS: { key: AIGenerateMode; label: string; desc: string; icon: React.ElementType }[] = [
  { key: 'script', label: 'AI 创作剧本', desc: '生成可直接用于项目的视频/分集剧本。', icon: Clapperboard },
  { key: 'novel_outline', label: '小说大纲', desc: '生成人设、世界观、主线与阶段推进。', icon: Library },
  { key: 'novel_chapter', label: '小说章节', desc: '围绕某章目标直接写正文。', icon: ScrollText },
  { key: 'adaptation', label: '小说改编剧本', desc: '把小说内容改成影视化剧本。', icon: Wand2 },
  { key: 'episode_outline', label: '分集大纲', desc: '按集拆分剧情推进，适合短剧/番剧策划。', icon: List },
  { key: 'scene_script', label: '场景脚本', desc: '细化到场次、动作、对白与镜头提示。', icon: Film },
  { key: 'dialogue_polish', label: '对白润色', desc: '对现有内容做口语化、角色化和情绪强化。', icon: MessageSquare },
]

const AI_PRESETS: { label: string; mode: AIGenerateMode; form: Partial<AIFormState> }[] = [
  {
    label: '短剧反转流',
    mode: 'episode_outline',
    form: {
      genre: '都市悬疑短剧',
      platform: '竖屏短剧',
      delivery_format: '分集钩子强、每集结尾留悬念',
      episode_duration: '1-3分钟/集',
      tone: '反转密集、冲突强、节奏快',
      target_words: '2500',
      chapter_count: '20',
    },
  },
  {
    label: '动画番剧',
    mode: 'scene_script',
    form: {
      genre: '热血冒险动画',
      platform: '横屏动画 / 番剧',
      delivery_format: '场景驱动、镜头感强',
      episode_duration: '12分钟/集',
      reference_style: '日漫分镜节奏、少年成长线',
      tone: '热血、幽默、成长',
      target_words: '3500',
      chapter_count: '12',
    },
  },
  {
    label: '网文长篇',
    mode: 'novel_outline',
    form: {
      genre: '玄幻成长',
      platform: '网络小说',
      delivery_format: '升级流、阶段目标明确',
      episode_duration: '单章 2000-3000 字',
      tone: '爽点稳定、悬念持续',
      target_words: '4000',
      chapter_count: '100',
    },
  },
  {
    label: '小说改编影视化',
    mode: 'adaptation',
    form: {
      platform: '短剧 / 漫改视频',
      delivery_format: '镜头化、对白化、冲突前置',
      episode_duration: '2-5分钟/集',
      tone: '强画面感、强节奏',
    },
  },
]

const DEFAULT_AI_FORM: AIFormState = {
  title: '',
  genre: '',
  platform: '短视频 / 剧情视频',
  delivery_format: '可直接用于分集、分场或后续拆解',
  episode_duration: '2-5分钟/集',
  reference_style: '',
  premise: '',
  character_setup: '',
  world_setting: '',
  outline: '',
  chapter_brief: '',
  source_text: '',
  target_words: '3000',
  chapter_count: '12',
  audience: '短剧 / 动画 / 漫改用户',
  tone: '强情节、强画面感、节奏清晰',
  requirements: '',
}

export default function ScriptsPage() {
  const [tab, setTab] = useState<TabKey>('all')
  const [search, setSearch] = useState('')
  const [showCreate, setShowCreate] = useState(false)
  const [showAIStudio, setShowAIStudio] = useState(false)
  const [aiMode, setAIMode] = useState<AIGenerateMode>('script')
  const [aiForm, setAIForm] = useState<AIFormState>(DEFAULT_AI_FORM)
  const [aiResult, setAIResult] = useState<AIResult | null>(null)
  const [generatingAI, setGeneratingAI] = useState(false)
  const [savingAI, setSavingAI] = useState(false)
  const [creatingProject, setCreatingProject] = useState(false)
  const [selectedAIModelId, setSelectedAIModelId] = useState('')
  const router = useRouter()
  const { toast } = useToast()

  const { data, mutate } = useSWR(
    ['script-library', tab],
    () => scriptLibraryAPI.list(tab === 'all' ? undefined : tab) as unknown as Promise<{ data: ScriptLibraryItem[] }>,
  )
  const items = (data as { data?: ScriptLibraryItem[] })?.data ?? []
  const { data: aiModelsRaw } = useSWR(
    ['script-ai-models'],
    () => modelAPI.list({ type: 'llm', sort_by: 'priority' }) as unknown as Promise<{ data?: Model[] }>
  )
  const aiModels = ((aiModelsRaw as { data?: Model[] })?.data ?? [])
    .filter((model) => model.is_active)
    .sort((left, right) => {
      if (left.is_default !== right.is_default) return left.is_default ? -1 : 1
      if (left.priority !== right.priority) return left.priority - right.priority
      return left.name.localeCompare(right.name, 'zh-CN')
    })
  const selectedAIModel = aiModels.find((model) => String(model.id) === selectedAIModelId) ?? null
  const selectedAIModelName = selectedAIModel?.name

  React.useEffect(() => {
    if (!selectedAIModelId && aiModels.length > 0) {
      setSelectedAIModelId(String((aiModels.find((model) => model.is_default) ?? aiModels[0]).id))
    }
  }, [aiModels, selectedAIModelId])

  const filtered = search
    ? items.filter(
        (i) =>
          i.title.includes(search) ||
          i.author.includes(search) ||
          i.description.includes(search) ||
          i.tags?.some((t) => t.includes(search))
      )
    : items

  const uploaded = filtered.filter((i) => i.source === 'uploaded')
  const showcase = filtered.filter((i) => i.source === 'showcase')

  const resetAIStudio = () => {
    setAIForm(DEFAULT_AI_FORM)
    setAIResult(null)
    setAIMode('script')
  }

  const applyAIPreset = (preset: { mode: AIGenerateMode; form: Partial<AIFormState> }) => {
    setAIMode(preset.mode)
    setAIForm((prev) => ({ ...prev, ...preset.form }))
    setShowAIStudio(true)
  }

  const handleReuseInAIStudio = (item: ScriptLibraryItem, mode: AIGenerateMode = 'adaptation') => {
    const inferredRequirements = [
      item.description ? `参考简介：${item.description}` : '',
      item.tags?.length ? `参考标签：${item.tags.join('、')}` : '',
    ].filter(Boolean).join('\n')

    setAIMode(mode)
    setAIForm((prev) => ({
      ...prev,
      title: item.title || prev.title,
      genre: item.genre || prev.genre,
      premise: item.description || prev.premise,
      outline: item.description || prev.outline,
      requirements: inferredRequirements || prev.requirements,
      source_text: mode === 'adaptation' || mode === 'dialogue_polish' ? `${item.title}\n\n${item.description}`.trim() : prev.source_text,
    }))
    setShowAIStudio(true)
  }

  const createProjectDraftFromScript = async (item: ScriptLibraryItem) => {
    setCreatingProject(true)
    try {
      let scriptContent = ''
      if (item.file_url) {
        try {
          const response = await fetch(item.file_url)
          if (response.ok) {
            scriptContent = await response.text()
          }
        } catch {
          scriptContent = ''
        }
      }

      const fallbackScriptContent = [item.title, item.description].filter(Boolean).join('\n\n')
      const draftId = savePendingProjectDraft({
        title: item.title,
        description: item.description,
        scriptContent: scriptContent.trim() || fallbackScriptContent,
        scriptFileName: `${(item.title || 'script').replace(/[\\/:*?"<>|]+/g, '_')}.txt`,
        styleTags: item.tags,
        media: 'video',
      })

      toast({ title: `已带入「${item.title}」`, description: '可继续调整模型和剧本配置', variant: 'success' })
      router.push(`/video/new?media=video&draft=${encodeURIComponent(draftId)}`)
    } catch {
      toast({ title: '带入创建页失败', description: '请稍后重试', variant: 'destructive' })
    } finally {
      setCreatingProject(false)
    }
  }

  const handleCreateSubmit = async (e: React.FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    const form = new FormData(e.currentTarget)
    const title = form.get('title') as string
    if (!title.trim()) return
    try {
      await scriptLibraryAPI.create({
        title: title.trim(),
        author: (form.get('author') as string) || '',
        description: (form.get('description') as string) || '',
        genre: (form.get('genre') as string) || '',
        tags: (form.get('tags') as string)?.split(/[,，]/).map((t) => t.trim()).filter(Boolean) || [],
        source: 'showcase',
      })
      toast({ title: '添加成功', variant: 'success' })
      setShowCreate(false)
      mutate()
    } catch {
      toast({ title: '添加失败', variant: 'destructive' })
    }
  }

  const handleDelete = async (id: number) => {
    try {
      await scriptLibraryAPI.delete(id)
      toast({ title: '已删除', variant: 'success' })
      mutate()
    } catch {
      toast({ title: '删除失败', variant: 'destructive' })
    }
  }

  const handleAIGenerate = async () => {
    if (!aiForm.title.trim() && aiMode !== 'adaptation') {
      toast({ title: '请先填写标题或主题', variant: 'destructive' })
      return
    }
    if ((aiMode === 'adaptation' || aiMode === 'dialogue_polish') && !aiForm.source_text.trim()) {
      toast({ title: aiMode === 'dialogue_polish' ? '请粘贴需要润色的正文或对白' : '请粘贴小说/章节原文', variant: 'destructive' })
      return
    }

    setGeneratingAI(true)
    try {
      const res = await scriptLibraryAPI.generateAI({
        mode: aiMode,
        model_name: selectedAIModelName,
        ...aiForm,
        target_words: Number(aiForm.target_words) || 0,
        chapter_count: Number(aiForm.chapter_count) || 0,
      }) as unknown as { data?: AIResult }

      if (!res.data) throw new Error('empty result')
      setAIResult({
        ...res.data,
        title: res.data.title || aiForm.title,
        description: res.data.description || aiForm.premise,
        genre: res.data.genre || aiForm.genre,
        tags: res.data.tags ?? [],
        outline: res.data.outline ?? [],
        ending_state: res.data.ending_state || '',
      })
      toast({ title: 'AI 内容已生成', variant: 'success' })
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { message?: string } } })?.response?.data?.message
      toast({ title: 'AI 生成失败', description: msg || '请检查模型配置后重试', variant: 'destructive' })
    } finally {
      setGeneratingAI(false)
    }
  }

  const handleSaveAIToLibrary = async () => {
    if (!aiResult?.content.trim()) return
    setSavingAI(true)
    try {
      await scriptLibraryAPI.create({
        title: aiResult.title.trim(),
        author: 'AI 创作助手',
        description: aiResult.description.trim(),
        genre: aiResult.genre.trim(),
        tags: aiResult.tags,
        source: 'uploaded',
        word_count: aiResult.word_count || aiResult.content.length,
        is_public: false,
      })
      toast({ title: '已保存到剧本库', variant: 'success' })
      mutate()
    } catch {
      toast({ title: '保存失败', variant: 'destructive' })
    } finally {
      setSavingAI(false)
    }
  }

  const handleCreateProjectFromAI = async () => {
    if (!aiResult?.content.trim()) return
    setCreatingProject(true)
    try {
      const draftId = savePendingProjectDraft({
        title: aiResult.title.trim(),
        description: aiResult.description.trim(),
        targetEpisodes: aiResult.suggested_episodes || undefined,
        styleTags: aiResult.tags,
        scriptContent: aiResult.content,
        scriptFileName: `${(aiResult.title || 'ai-script').replace(/[\\/:*?"<>|]+/g, '_')}.txt`,
        scriptMimeType: 'text/plain;charset=utf-8',
        media: 'video',
      })

      toast({ title: `已带入「${aiResult.title}」`, description: '可继续调整模型和剧本配置', variant: 'success' })
      router.push(`/video/new?media=video&draft=${encodeURIComponent(draftId)}`)
    } catch {
      toast({ title: '带入创建页失败', variant: 'destructive' })
    } finally {
      setCreatingProject(false)
    }
  }

  const handleCopyAIResult = async () => {
    if (!aiResult?.content) return
    try {
      await navigator.clipboard.writeText(aiResult.content)
      toast({ title: '正文已复制', variant: 'success' })
    } catch {
      toast({ title: '复制失败', variant: 'destructive' })
    }
  }

  const handleDownloadAIResult = () => {
    if (!aiResult?.content) return
    const blob = new Blob([aiResult.content], { type: 'text/plain;charset=utf-8' })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    const safeTitle = (aiResult.title || 'ai-script').replace(/[\\/:*?"<>|]+/g, '_')
    link.href = url
    link.download = `${safeTitle}.txt`
    link.click()
    URL.revokeObjectURL(url)
  }

  const formatWordCount = (n: number) => {
    if (n >= 10000) return `${(n / 10000).toFixed(1)}万字`
    if (n > 0) return `${n}字`
    return ''
  }

  const SOURCE_BADGE: Record<string, { label: string; cls: string; icon: React.ElementType }> = {
    uploaded: { label: '我的上传', cls: 'bg-blue-100 text-blue-700', icon: Upload },
    showcase: { label: '推荐展示', cls: 'bg-amber-100 text-amber-700', icon: Star },
  }

  const TABS: { key: TabKey; label: string; icon: React.ElementType }[] = [
    { key: 'all', label: '全部', icon: BookOpen },
    { key: 'uploaded', label: '我的上传', icon: Upload },
    { key: 'showcase', label: '推荐展示', icon: Star },
  ]

  const renderCard = (item: ScriptLibraryItem) => {
    const badge = SOURCE_BADGE[item.source] ?? SOURCE_BADGE.uploaded
    const BadgeIcon = badge.icon
    return (
      <div
        key={item.id}
        className="group flex flex-col rounded-xl border bg-white shadow-sm transition-all hover:shadow-md"
      >
        {/* Cover area */}
        <div className="relative flex h-40 items-center justify-center rounded-t-xl bg-gradient-to-br from-surface-100 to-surface-50">
          {item.cover_url ? (
            <img src={item.cover_url} alt={item.title} className="h-full w-full rounded-t-xl object-cover" />
          ) : (
            <FileText className="h-12 w-12 text-surface-300" />
          )}
          {/* Source badge */}
          <span className={`absolute left-3 top-3 flex items-center gap-1 rounded-full px-2 py-0.5 text-[10px] font-semibold ${badge.cls}`}>
            <BadgeIcon className="h-3 w-3" />
            {badge.label}
          </span>
        </div>

        {/* Content */}
        <div className="flex flex-1 flex-col p-4">
          <h3 className="mb-1 text-sm font-bold text-surface-900 line-clamp-1">{item.title}</h3>
          {item.author && (
            <div className="mb-1.5 flex items-center gap-1 text-xs text-surface-500">
              <User className="h-3 w-3" /> {item.author}
            </div>
          )}
          <p className="mb-3 line-clamp-2 flex-1 text-xs text-surface-500">{item.description || '暂无简介'}</p>

          {/* Tags */}
          {item.tags && item.tags.length > 0 && (
            <div className="mb-3 flex flex-wrap gap-1">
              {item.tags.slice(0, 4).map((tag) => (
                <span key={tag} className="inline-flex items-center gap-0.5 rounded-full bg-surface-100 px-2 py-0.5 text-[10px] text-surface-600">
                  <Tag className="h-2.5 w-2.5" />{tag}
                </span>
              ))}
            </div>
          )}

          {/* Meta */}
          <div className="flex items-center gap-3 text-[10px] text-surface-400">
            {item.genre && (
              <span className="flex items-center gap-0.5">
                <Hash className="h-3 w-3" />{item.genre}
              </span>
            )}
            {item.word_count > 0 && <span>{formatWordCount(item.word_count)}</span>}
          </div>
        </div>

        {/* Actions */}
        <div className="space-y-2 border-t px-4 py-2.5">
          <Button size="sm" variant="outline" className="w-full" onClick={() => handleReuseInAIStudio(item, item.source === 'showcase' ? 'script' : 'adaptation')}>
            <Sparkles className="mr-1.5 h-3.5 w-3.5" />
            继续 AI 创作
          </Button>
          {item.source === 'showcase' ? (
            <Button size="sm" className="w-full" onClick={() => createProjectDraftFromScript(item)} disabled={creatingProject}>
              <ExternalLink className="mr-1.5 h-3.5 w-3.5" />
              使用此剧本创建项目
            </Button>
          ) : item.project_id ? (
            <Button size="sm" variant="outline" className="w-full" onClick={() => router.push(`/projects/${item.project_id}`)}>
              <ExternalLink className="mr-1.5 h-3.5 w-3.5" />
              查看项目
            </Button>
          ) : (
            <Button size="sm" variant="outline" className="w-full" onClick={() => createProjectDraftFromScript(item)} disabled={creatingProject}>
              <ExternalLink className="mr-1.5 h-3.5 w-3.5" />
              使用此内容创建项目
            </Button>
          )}
        </div>
      </div>
    )
  }

  return (
    <div className="mx-auto max-w-7xl px-6 py-8">
      {/* Header */}
      <DashboardHero
        badge="剧本内容工作台"
        badgeIcon={<Sparkles className="h-3.5 w-3.5 text-primary-300" />}
        title="剧本库"
        description="浏览推荐剧本、管理上传内容，或直接用 AI 创作剧本、小说大纲、章节正文和改编稿。"
        actions={
          <>
            <Button variant="outline" className="border-white/10 bg-white/10 text-white hover:bg-white/15 hover:text-white" onClick={() => setShowAIStudio(true)}>
              <Sparkles className="mr-1.5 h-4 w-4" />
              AI 创作
            </Button>
            <Button className="bg-white text-slate-900 hover:bg-slate-100" onClick={() => setShowCreate(true)}>
              <Plus className="mr-1.5 h-4 w-4" />
              添加剧本
            </Button>
          </>
        }
        stats={[
          {
            icon: <BookOpen className="h-4 w-4 text-cyan-300" />,
            label: '总条目',
            value: items.length,
            description: '当前剧本库中的全部内容',
          },
          {
            icon: <Upload className="h-4 w-4 text-violet-300" />,
            label: '我的上传',
            value: items.filter((i) => i.source === 'uploaded').length,
            description: '关联项目或个人沉淀的私有内容',
          },
          {
            icon: <Star className="h-4 w-4 text-amber-300" />,
            label: '推荐展示',
            value: items.filter((i) => i.source === 'showcase').length,
            description: '可直接复用并创建项目的参考脚本',
          },
        ]}
      />

      <div className="mb-6 grid gap-3 md:grid-cols-4">
        {AI_MODE_OPTIONS.map((option) => {
          const Icon = option.icon
          return (
            <button
              key={option.key}
              type="button"
              onClick={() => {
                setAIMode(option.key)
                setShowAIStudio(true)
              }}
              className="rounded-xl border bg-white p-4 text-left shadow-sm transition hover:border-primary-300 hover:shadow-md"
            >
              <div className="mb-2 flex items-center gap-2">
                <span className="rounded-lg bg-primary-50 p-2 text-primary-600">
                  <Icon className="h-4 w-4" />
                </span>
                <span className="text-sm font-semibold text-surface-900">{option.label}</span>
              </div>
              <p className="text-xs leading-5 text-surface-500">{option.desc}</p>
            </button>
          )
        })}
      </div>

      <div className="mb-6 flex flex-wrap gap-2">
        {AI_PRESETS.map((preset) => (
          <button
            key={preset.label}
            type="button"
            onClick={() => applyAIPreset(preset)}
            className="rounded-full border bg-white px-3 py-1.5 text-xs text-surface-600 transition hover:border-primary-300 hover:text-primary-700"
          >
            {preset.label}
          </button>
        ))}
      </div>

      {/* Search + Tabs */}
      <div className="mb-6 rounded-[24px] border border-surface-200 bg-white/80 p-4 shadow-sm backdrop-blur">
        <div className="mb-3">
          <p className="text-sm font-medium text-surface-900">检索与分类</p>
          <p className="mt-1 text-xs text-surface-500">按标题、作者、标签搜索，并在“全部 / 我的上传 / 推荐展示”之间快速切换。</p>
        </div>
        <div className="flex flex-wrap items-center gap-3">
        <div className="relative w-72">
          <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-surface-400" />
          <Input
            placeholder="搜索标题、作者、标签…"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="pl-9"
          />
        </div>
        <Separator orientation="vertical" className="h-8" />
        {TABS.map((t) => {
          const Icon = t.icon
          return (
            <Button
              key={t.key}
              size="sm"
              variant={tab === t.key ? 'default' : 'outline'}
              onClick={() => setTab(t.key)}
            >
              <Icon className="mr-1.5 h-3.5 w-3.5" />
              {t.label}
            </Button>
          )
        })}
        </div>
      </div>

      {/* Showcase section */}
      {(tab === 'all' || tab === 'showcase') && showcase.length > 0 && (
        <section className="mb-8">
          {tab === 'all' && (
            <h2 className="mb-4 flex items-center gap-2 text-lg font-semibold text-surface-800">
              <Star className="h-5 w-5 text-amber-500" />
              推荐展示
            </h2>
          )}
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
            {showcase.map(renderCard)}
          </div>
        </section>
      )}

      {/* Uploaded section */}
      {(tab === 'all' || tab === 'uploaded') && (
        <section className="mb-8">
          {tab === 'all' && uploaded.length > 0 && (
            <h2 className="mb-4 flex items-center gap-2 text-lg font-semibold text-surface-800">
              <Upload className="h-5 w-5 text-blue-500" />
              我的上传
            </h2>
          )}
          {uploaded.length > 0 ? (
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
              {uploaded.map(renderCard)}
            </div>
          ) : tab === 'uploaded' ? (
            <DashboardEmptyState
              icon={<Upload className="mb-3 h-12 w-12 text-surface-300" />}
              title="暂无上传的剧本"
              description="在项目中上传剧本后会自动出现在这里"
              innerClassName="flex flex-col items-center justify-center rounded-[24px] border border-dashed border-surface-200 bg-[radial-gradient(circle_at_top_left,_rgba(99,102,241,0.08),_transparent_30%),radial-gradient(circle_at_bottom_right,_rgba(236,72,153,0.08),_transparent_28%)] py-16 text-center"
            />
          ) : null}
        </section>
      )}

      {filtered.length === 0 && (
        <DashboardEmptyState
          icon={<BookOpen className="mb-3 h-12 w-12 text-surface-300" />}
          title={search ? '未找到匹配的剧本' : '剧本库为空'}
          description={search ? '试试调整关键词、标签或切换分类。' : '可以先添加展示剧本，或直接使用 AI 创作生成第一份内容。'}
          innerClassName="flex flex-col items-center justify-center rounded-[24px] border border-dashed border-surface-200 bg-[radial-gradient(circle_at_top_left,_rgba(99,102,241,0.08),_transparent_30%),radial-gradient(circle_at_bottom_right,_rgba(236,72,153,0.08),_transparent_28%)] py-16 text-center"
        />
      )}

      {/* Create dialog */}
      {showCreate && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50" onClick={() => setShowCreate(false)}>
          <form
            onSubmit={handleCreateSubmit}
            onClick={(e) => e.stopPropagation()}
            className="w-full max-w-md rounded-xl bg-white p-6 shadow-xl"
          >
            <h3 className="mb-4 text-lg font-bold">添加展示剧本</h3>
            <div className="space-y-3">
              <div>
                <label className="mb-1 block text-xs font-medium text-surface-600">标题 *</label>
                <Input name="title" required placeholder="剧本名称" />
              </div>
              <div>
                <label className="mb-1 block text-xs font-medium text-surface-600">作者</label>
                <Input name="author" placeholder="作者名" />
              </div>
              <div>
                <label className="mb-1 block text-xs font-medium text-surface-600">简介</label>
                <textarea name="description" className="w-full rounded-md border px-3 py-2 text-sm" rows={3} placeholder="剧本简介" />
              </div>
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="mb-1 block text-xs font-medium text-surface-600">类型</label>
                  <Input name="genre" placeholder="如：奇幻小说" />
                </div>
                <div>
                  <label className="mb-1 block text-xs font-medium text-surface-600">标签（逗号分隔）</label>
                  <Input name="tags" placeholder="冒险,魔法,热血" />
                </div>
              </div>
            </div>
            <div className="mt-5 flex justify-end gap-2">
              <Button type="button" variant="outline" onClick={() => setShowCreate(false)}>取消</Button>
              <Button type="submit">添加</Button>
            </div>
          </form>
        </div>
      )}

      {showAIStudio && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4" onClick={() => setShowAIStudio(false)}>
          <div
            onClick={(e) => e.stopPropagation()}
            className="flex max-h-[90vh] w-full max-w-6xl flex-col overflow-hidden rounded-2xl bg-white shadow-xl"
          >
            <div className="flex items-center justify-between border-b px-6 py-4">
              <div>
                <h3 className="text-lg font-bold text-surface-900">AI 剧本工作台</h3>
                <p className="mt-1 text-xs text-surface-500">支持剧本创作、小说大纲、章节正文，以及小说改编为剧本。</p>
              </div>
              <div className="flex items-center gap-2">
                <Button type="button" variant="outline" onClick={resetAIStudio}>
                  <RefreshCw className="mr-1.5 h-4 w-4" />
                  重置
                </Button>
                <Button type="button" variant="outline" onClick={() => setShowAIStudio(false)}>关闭</Button>
              </div>
            </div>

            <div className="grid min-h-0 flex-1 gap-0 lg:grid-cols-[360px_minmax(0,1fr)]">
              <div className="overflow-y-auto border-r bg-surface-50 p-4">
                <div className="grid gap-2">
                  {AI_MODE_OPTIONS.map((option) => {
                    const Icon = option.icon
                    const active = aiMode === option.key
                    return (
                      <button
                        key={option.key}
                        type="button"
                        onClick={() => setAIMode(option.key)}
                        className={`rounded-xl border px-3 py-3 text-left transition ${active ? 'border-primary-300 bg-primary-50' : 'border-surface-200 bg-white'}`}
                      >
                        <div className="mb-1 flex items-center gap-2">
                          <Icon className={`h-4 w-4 ${active ? 'text-primary-600' : 'text-surface-400'}`} />
                          <span className="text-sm font-semibold text-surface-900">{option.label}</span>
                        </div>
                        <p className="text-xs text-surface-500">{option.desc}</p>
                      </button>
                    )
                  })}
                </div>

                <div className="mt-4 space-y-3">
                  <div>
                    <label className="mb-1 block text-xs font-medium text-surface-600">范围预设</label>
                    <div className="flex flex-wrap gap-2">
                      {AI_PRESETS.filter((preset) => preset.mode === aiMode).map((preset) => (
                        <button
                          key={`${aiMode}-${preset.label}`}
                          type="button"
                          onClick={() => applyAIPreset(preset)}
                          className="rounded-full border bg-white px-3 py-1.5 text-[11px] text-surface-600 transition hover:border-primary-300 hover:text-primary-700"
                        >
                          {preset.label}
                        </button>
                      ))}
                    </div>
                  </div>
                  <div>
                    <label className="mb-1 block text-xs font-medium text-surface-600">生成模型</label>
                    <Select value={selectedAIModelId} onValueChange={setSelectedAIModelId}>
                      <SelectTrigger>
                        <SelectValue placeholder="选择文本模型" />
                      </SelectTrigger>
                      <SelectContent>
                        {aiModels.map((model) => (
                          <SelectItem key={model.id} value={String(model.id)}>
                            {model.name}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                    <p className="mt-1 text-[11px] text-surface-400">对白润色与 AI 创作都会使用这里选择的文本模型。</p>
                  </div>
                  <div>
                    <label className="mb-1 block text-xs font-medium text-surface-600">标题 / 主题</label>
                    <Input value={aiForm.title} onChange={(e) => setAIForm((prev) => ({ ...prev, title: e.target.value }))} placeholder="如：山海城迷局 / 末日列车 / 宫廷谜案" />
                  </div>
                  <div className="grid grid-cols-2 gap-3">
                    <div>
                      <label className="mb-1 block text-xs font-medium text-surface-600">类型</label>
                      <Input value={aiForm.genre} onChange={(e) => setAIForm((prev) => ({ ...prev, genre: e.target.value }))} placeholder="悬疑 / 玄幻 / 言情 / 科幻" />
                    </div>
                    <div>
                      <label className="mb-1 block text-xs font-medium text-surface-600">目标字数</label>
                      <Input value={aiForm.target_words} onChange={(e) => setAIForm((prev) => ({ ...prev, target_words: e.target.value }))} placeholder="3000" />
                    </div>
                  </div>
                  <div className="grid grid-cols-2 gap-3">
                    <div>
                      <label className="mb-1 block text-xs font-medium text-surface-600">适用平台</label>
                      <Input value={aiForm.platform} onChange={(e) => setAIForm((prev) => ({ ...prev, platform: e.target.value }))} placeholder="短剧 / 动画 / 漫改 / 有声书" />
                    </div>
                    <div>
                      <label className="mb-1 block text-xs font-medium text-surface-600">交付形式</label>
                      <Input value={aiForm.delivery_format} onChange={(e) => setAIForm((prev) => ({ ...prev, delivery_format: e.target.value }))} placeholder="分集大纲 / 场景脚本 / 长篇正文" />
                    </div>
                  </div>
                  <div className="grid grid-cols-2 gap-3">
                    <div>
                      <label className="mb-1 block text-xs font-medium text-surface-600">章节数</label>
                      <Input value={aiForm.chapter_count} onChange={(e) => setAIForm((prev) => ({ ...prev, chapter_count: e.target.value }))} placeholder="12" />
                    </div>
                    <div>
                      <label className="mb-1 block text-xs font-medium text-surface-600">受众</label>
                      <Input value={aiForm.audience} onChange={(e) => setAIForm((prev) => ({ ...prev, audience: e.target.value }))} placeholder="短剧 / 漫改 / 女性向 / 男频" />
                    </div>
                  </div>
                  <div className="grid grid-cols-2 gap-3">
                    <div>
                      <label className="mb-1 block text-xs font-medium text-surface-600">单集/单章长度</label>
                      <Input value={aiForm.episode_duration} onChange={(e) => setAIForm((prev) => ({ ...prev, episode_duration: e.target.value }))} placeholder="2-5分钟/集 或 2000字/章" />
                    </div>
                    <div>
                      <label className="mb-1 block text-xs font-medium text-surface-600">参考风格</label>
                      <Input value={aiForm.reference_style} onChange={(e) => setAIForm((prev) => ({ ...prev, reference_style: e.target.value }))} placeholder="国风、赛博、轻喜、黑色幽默" />
                    </div>
                  </div>
                  <div>
                    <label className="mb-1 block text-xs font-medium text-surface-600">基调与节奏</label>
                    <Input value={aiForm.tone} onChange={(e) => setAIForm((prev) => ({ ...prev, tone: e.target.value }))} placeholder="紧张、反转密集、情感浓烈" />
                  </div>
                  <div>
                    <label className="mb-1 block text-xs font-medium text-surface-600">故事设定 / 一句话梗概</label>
                    <Textarea value={aiForm.premise} onChange={(e) => setAIForm((prev) => ({ ...prev, premise: e.target.value }))} rows={4} placeholder="描述主角、冲突、目标和卖点" />
                  </div>
                  <div>
                    <label className="mb-1 block text-xs font-medium text-surface-600">角色设定</label>
                    <Textarea value={aiForm.character_setup} onChange={(e) => setAIForm((prev) => ({ ...prev, character_setup: e.target.value }))} rows={4} placeholder="主角、反派、配角关系与人物弧光" />
                  </div>
                  <div>
                    <label className="mb-1 block text-xs font-medium text-surface-600">世界观 / 背景</label>
                    <Textarea value={aiForm.world_setting} onChange={(e) => setAIForm((prev) => ({ ...prev, world_setting: e.target.value }))} rows={3} placeholder="时代、规则、空间、行业背景等" />
                  </div>
                  {(aiMode === 'novel_chapter' || aiMode === 'adaptation' || aiMode === 'episode_outline' || aiMode === 'scene_script') && (
                    <div>
                      <label className="mb-1 block text-xs font-medium text-surface-600">大纲 / 上下文</label>
                      <Textarea value={aiForm.outline} onChange={(e) => setAIForm((prev) => ({ ...prev, outline: e.target.value }))} rows={4} placeholder="已有大纲、上一章摘要、关键推进节点" />
                    </div>
                  )}
                  {(aiMode === 'novel_chapter' || aiMode === 'scene_script' || aiMode === 'episode_outline') && (
                    <div>
                      <label className="mb-1 block text-xs font-medium text-surface-600">{aiMode === 'novel_chapter' ? '本章目标' : aiMode === 'episode_outline' ? '每集推进目标' : '本场戏目标'}</label>
                      <Textarea value={aiForm.chapter_brief} onChange={(e) => setAIForm((prev) => ({ ...prev, chapter_brief: e.target.value }))} rows={4} placeholder={aiMode === 'novel_chapter' ? '这一章要推进什么事件、关系和悬念' : aiMode === 'episode_outline' ? '说明每集核心矛盾、钩子和结尾悬念' : '说明该场戏的冲突、转折与情绪目标'} />
                    </div>
                  )}
                  {(aiMode === 'adaptation' || aiMode === 'dialogue_polish') && (
                    <div>
                      <label className="mb-1 block text-xs font-medium text-surface-600">{aiMode === 'dialogue_polish' ? '待润色正文 / 对白' : '小说原文 / 章节内容'}</label>
                      <Textarea value={aiForm.source_text} onChange={(e) => setAIForm((prev) => ({ ...prev, source_text: e.target.value }))} rows={8} placeholder={aiMode === 'dialogue_polish' ? '粘贴对白、场景台词或待润色片段' : '粘贴要改编的小说正文、章节节选或原始大纲'} />
                    </div>
                  )}
                  <div>
                    <label className="mb-1 block text-xs font-medium text-surface-600">补充要求</label>
                    <Textarea value={aiForm.requirements} onChange={(e) => setAIForm((prev) => ({ ...prev, requirements: e.target.value }))} rows={3} placeholder="如：更强反转、更适合短视频节奏、对白更口语化等" />
                  </div>
                  <Button onClick={handleAIGenerate} disabled={generatingAI}>
                    {generatingAI ? <Loader2 className="mr-1.5 h-4 w-4 animate-spin" /> : <Wand2 className="mr-1.5 h-4 w-4" />}
                    生成内容
                  </Button>
                </div>
              </div>

              <div className="min-h-0 overflow-y-auto p-5">
                {aiResult ? (
                  <div className="space-y-4">
                    <div className="grid gap-3 md:grid-cols-3">
                      <div className="rounded-xl border bg-surface-50 p-3">
                        <p className="text-[11px] text-surface-400">推荐标题</p>
                        <Input value={aiResult.title} onChange={(e) => setAIResult((prev) => prev ? { ...prev, title: e.target.value } : prev)} className="mt-2" />
                      </div>
                      <div className="rounded-xl border bg-surface-50 p-3">
                        <p className="text-[11px] text-surface-400">类型</p>
                        <Input value={aiResult.genre} onChange={(e) => setAIResult((prev) => prev ? { ...prev, genre: e.target.value } : prev)} className="mt-2" />
                      </div>
                      <div className="rounded-xl border bg-surface-50 p-3">
                        <p className="text-[11px] text-surface-400">建议集数 / 字数</p>
                        <p className="mt-2 text-sm font-medium text-surface-800">{aiResult.suggested_episodes || 0} 集 · {aiResult.word_count || aiResult.content.length} 字</p>
                      </div>
                    </div>

                    <div className="rounded-xl border p-4">
                      <p className="mb-2 text-xs font-medium text-surface-600">简介</p>
                      <Textarea value={aiResult.description} onChange={(e) => setAIResult((prev) => prev ? { ...prev, description: e.target.value } : prev)} rows={3} />
                    </div>

                    <div className="rounded-xl border p-4">
                      <div className="mb-2 flex items-center justify-between">
                        <p className="text-xs font-medium text-surface-600">标签</p>
                        <p className="text-[11px] text-surface-400">可直接编辑，逗号分隔</p>
                      </div>
                      <Input
                        value={aiResult.tags.join('，')}
                        onChange={(e) => setAIResult((prev) => prev ? { ...prev, tags: e.target.value.split(/[,，]/).map((tag) => tag.trim()).filter(Boolean) } : prev)}
                      />
                    </div>

                    {aiResult.outline.length > 0 && (
                      <div className="rounded-xl border p-4">
                        <p className="mb-2 text-xs font-medium text-surface-600">结构大纲</p>
                        <div className="space-y-2">
                          {aiResult.outline.map((item, index) => (
                            <div key={`${index}-${item}`} className="rounded-lg bg-surface-50 px-3 py-2 text-sm text-surface-700">
                              <span className="mr-2 text-xs text-surface-400">#{index + 1}</span>
                              {item}
                            </div>
                          ))}
                        </div>
                      </div>
                    )}

                    <div className="rounded-xl border p-4">
                      <div className="mb-2 flex items-center justify-between">
                        <p className="text-xs font-medium text-surface-600">正文内容</p>
                        <p className="text-[11px] text-surface-400">可继续修改后保存或建项目</p>
                      </div>
                      <Textarea
                        value={aiResult.content}
                        onChange={(e) => setAIResult((prev) => prev ? { ...prev, content: e.target.value, word_count: e.target.value.replace(/\s/g, '').length } : prev)}
                        rows={24}
                        className="min-h-[520px] font-mono text-xs leading-6"
                      />
                    </div>

                    {aiResult.ending_state && (
                      <div className="rounded-xl border border-amber-200 bg-amber-50 p-4">
                        <div className="mb-2 flex items-center justify-between">
                          <p className="text-xs font-semibold text-amber-800">📌 结尾状态摘要（供续写衔接）</p>
                          <button
                            type="button"
                            className="rounded-md border border-amber-300 bg-white px-2.5 py-1 text-[11px] text-amber-700 hover:bg-amber-50 transition-colors"
                            onClick={() => setAIForm((prev) => ({ ...prev, outline: aiResult.ending_state ?? '' }))}
                          >
                            填入「大纲/上下文」续写
                          </button>
                        </div>
                        <p className="text-xs leading-5 text-amber-700 whitespace-pre-wrap">{aiResult.ending_state}</p>
                      </div>
                    )}

                    <div className="flex flex-wrap justify-end gap-2">
                      <Button type="button" variant="outline" onClick={handleCopyAIResult}>
                        <Copy className="mr-1.5 h-4 w-4" />
                        复制正文
                      </Button>
                      <Button type="button" variant="outline" onClick={handleDownloadAIResult}>
                        <Download className="mr-1.5 h-4 w-4" />
                        下载 TXT
                      </Button>
                      <Button variant="outline" onClick={handleSaveAIToLibrary} disabled={savingAI}>
                        {savingAI ? <Loader2 className="mr-1.5 h-4 w-4 animate-spin" /> : <BookOpen className="mr-1.5 h-4 w-4" />}
                        保存到剧本库
                      </Button>
                      <Button onClick={handleCreateProjectFromAI} disabled={creatingProject}>
                        {creatingProject ? <Loader2 className="mr-1.5 h-4 w-4 animate-spin" /> : <ExternalLink className="mr-1.5 h-4 w-4" />}
                        带入新建项目
                      </Button>
                    </div>
                  </div>
                ) : (
                  <div className="flex h-full min-h-[640px] flex-col items-center justify-center rounded-2xl border border-dashed border-surface-200 bg-surface-50 px-8 text-center">
                    <Sparkles className="mb-3 h-12 w-12 text-primary-300" />
                    <h4 className="text-base font-semibold text-surface-800">开始 AI 创作</h4>
                    <p className="mt-2 max-w-lg text-sm leading-6 text-surface-500">
                      左侧选择创作模式并填写设定后，可生成完整剧本、小说大纲、章节正文，或把小说原文改编成镜头化剧本。
                    </p>
                  </div>
                )}
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
