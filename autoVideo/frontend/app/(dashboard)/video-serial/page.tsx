'use client'

import { useState, useMemo } from 'react'
import { useRouter } from 'next/navigation'
import useSWR from 'swr'
import { format } from 'date-fns'
import {
  Plus,
  FolderOpen,
  Trash2,
  ArrowRight,
  Search,
  Pause,
  Play,
  Copy,
  MoreVertical,
  HardDrive,
  ArrowUpDown,
  Type,
  Image,
  Video,
  Mic,
  Sparkles,
  Clock3,
  CheckCircle2,
} from 'lucide-react'
import { projectAPI } from '@/lib/api'
import { matchesProjectMedia, PROJECT_MEDIA_META, stripProjectMediaTags } from '@/lib/project-media'
import type { Project } from '@/types'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardFooter, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
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
import { EmptyState } from '@/components/common/EmptyState'
import { PipelineProgress } from '@/components/projects/list/PipelineProgress'
import { ProjectStatusBadge as StatusBadge } from '@/components/projects/list/ProjectStatusBadge'
import { formatBytes } from '@/lib/projects/utils'

type SortField = 'created_at' | 'updated_at' | 'title'
type SortDir = 'asc' | 'desc'

const MODEL_ICONS: Record<string, React.ReactNode> = {
  text:  <Type className="h-3 w-3" />,
  image: <Image className="h-3 w-3" />,
  video: <Video className="h-3 w-3" />,
  tts:   <Mic className="h-3 w-3" />,
}

const META = PROJECT_MEDIA_META.video_serial

export default function VideoSerialPage() {
  const router = useRouter()
  const { toast } = useToast()
  const [keyword, setKeyword] = useState('')
  const [statusFilter, setStatusFilter] = useState<string>('all')
  const [sortField, setSortField] = useState<SortField>('created_at')
  const [sortDir, setSortDir] = useState<SortDir>('desc')
  const [actionLoading, setActionLoading] = useState<string | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<Project | null>(null)

  const filters = { keyword, status: statusFilter }
  const { data, isLoading, mutate } = useSWR(
    ['projects-serial', filters],
    () =>
      projectAPI.list({
        keyword: keyword || undefined,
        status: statusFilter !== 'all' ? statusFilter : undefined,
        project_type: 'video_serial',
        page: 1,
        page_size: 100,
      }) as unknown as Promise<{ data: Project[] }>
  )

  const projectsData = (data as { data?: Project[] | { items?: Project[] } })?.data
  const projects = useMemo<Project[]>(
    () => (Array.isArray(projectsData) ? projectsData : (projectsData?.items ?? [])),
    [projectsData]
  )
  const serialProjects = useMemo(
    () => projects.filter((p) => matchesProjectMedia(p, 'video_serial')),
    [projects]
  )

  const sorted = useMemo(() => {
    const list = [...serialProjects]
    list.sort((a, b) => {
      let cmp = 0
      if (sortField === 'title') {
        cmp = a.title.localeCompare(b.title)
      } else {
        const da = new Date(a[sortField]).getTime()
        const db = new Date(b[sortField]).getTime()
        cmp = da - db
      }
      return sortDir === 'asc' ? cmp : -cmp
    })
    return list
  }, [serialProjects, sortField, sortDir])

  const projectStats = useMemo(() => {
    const processingCount = serialProjects.filter((p) =>
      ['script_processing', 'asset_generating', 'storyboard_generating', 'video_generating'].includes(p.status)
    ).length
    const completedCount = serialProjects.filter((p) => p.status === 'completed').length
    const draftCount = serialProjects.filter((p) => p.status === 'draft').length
    return { processingCount, completedCount, draftCount }
  }, [serialProjects])

  const handleAction = async (action: string, project: Project) => {
    if (action === 'delete') {
      setDeleteTarget(project)
      return
    }
    setActionLoading(`${action}-${project.id}`)
    try {
      if (action === 'pause') {
        await projectAPI.pause(project.id)
        toast({ title: '已暂停', description: `项目「${project.title}」已暂停` })
      } else if (action === 'resume') {
        await projectAPI.resume(project.id)
        toast({ title: '已恢复', description: `项目「${project.title}」已恢复运行` })
      } else if (action === 'clone') {
        await projectAPI.clone(project.id)
        toast({ title: '克隆成功', description: `项目「${project.title}」已克隆` })
      }
      await mutate()
    } catch {
      toast({ title: '操作失败', description: '请稍后重试', variant: 'destructive' })
    } finally {
      setActionLoading(null)
    }
  }

  const confirmDelete = async () => {
    if (!deleteTarget) return
    setActionLoading(`delete-${deleteTarget.id}`)
    try {
      await projectAPI.delete(deleteTarget.id)
      toast({ title: '已删除', description: `项目「${deleteTarget.title}」已删除` })
      await mutate()
    } catch {
      toast({ title: '删除失败', description: '请稍后重试', variant: 'destructive' })
    } finally {
      setActionLoading(null)
      setDeleteTarget(null)
    }
  }

  const toggleSort = (field: SortField) => {
    if (sortField === field) {
      setSortDir((d) => (d === 'asc' ? 'desc' : 'asc'))
    } else {
      setSortField(field)
      setSortDir('desc')
    }
  }

  const modelIcons = (project: Project) => {
    const icons: { type: string; id?: number }[] = []
    if (project.text_model_id) icons.push({ type: 'text', id: project.text_model_id })
    if (project.image_model_id) icons.push({ type: 'image', id: project.image_model_id })
    if (project.video_model_id) icons.push({ type: 'video', id: project.video_model_id })
    if (project.tts_model_id) icons.push({ type: 'tts', id: project.tts_model_id })
    return icons
  }

  return (
    <div className="space-y-6 animate-fadeIn">
      {/* Header */}
      <div className="overflow-hidden rounded-[28px] border border-surface-200/70 bg-gradient-to-br from-slate-950 via-indigo-950 to-slate-900 p-6 text-white shadow-sm">
        <div className="flex flex-col gap-6 lg:flex-row lg:items-start lg:justify-between">
          <div className="max-w-2xl">
            <div className="mb-4 inline-flex items-center gap-2 rounded-full border border-white/10 bg-white/10 px-3 py-1.5 text-xs font-medium text-surface-100 backdrop-blur">
              <Sparkles className="h-3.5 w-3.5 text-indigo-300" />
              AI 串行视频工作台
            </div>
            <h2 className="text-2xl font-semibold tracking-tight">{META.listTitle}</h2>
            <p className="mt-2 text-sm leading-6 text-surface-300">
              按场景分组，同场景分镜串行生成，末帧约束保证连贯性。
            </p>
          </div>
          <div className="flex flex-wrap gap-3">
            <Button
              className="bg-white text-slate-900 hover:bg-slate-100"
              onClick={() => router.push(META.createHref)}
            >
              <Plus className="mr-2 h-4 w-4" />
              {META.createLabel}
            </Button>
          </div>
        </div>

        <div className="mt-6 grid gap-3 sm:grid-cols-3">
          <div className="rounded-2xl border border-white/10 bg-white/10 p-4 backdrop-blur">
            <div className="flex items-center gap-2 text-surface-300">
              <FolderOpen className="h-4 w-4 text-cyan-300" />
              <span className="text-xs uppercase tracking-[0.2em]">项目总数</span>
            </div>
            <p className="mt-3 text-2xl font-semibold text-white">{serialProjects.length}</p>
            <p className="mt-1 text-xs text-surface-400">{META.countLabel}</p>
          </div>
          <div className="rounded-2xl border border-white/10 bg-white/10 p-4 backdrop-blur">
            <div className="flex items-center gap-2 text-surface-300">
              <Clock3 className="h-4 w-4 text-amber-300" />
              <span className="text-xs uppercase tracking-[0.2em]">处理中</span>
            </div>
            <p className="mt-3 text-2xl font-semibold text-white">{projectStats.processingCount}</p>
            <p className="mt-1 text-xs text-surface-400">正在生成</p>
          </div>
          <div className="rounded-2xl border border-white/10 bg-white/10 p-4 backdrop-blur">
            <div className="flex items-center gap-2 text-surface-300">
              <CheckCircle2 className="h-4 w-4 text-emerald-300" />
              <span className="text-xs uppercase tracking-[0.2em]">已完成</span>
            </div>
            <p className="mt-3 text-2xl font-semibold text-white">{projectStats.completedCount}</p>
            <p className="mt-1 text-xs text-surface-400">草稿 {projectStats.draftCount} 个</p>
          </div>
        </div>
      </div>

      {/* Filters */}
      <div className="flex flex-wrap items-center gap-3">
        <div className="relative flex-1 min-w-[200px] max-w-sm">
          <Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-surface-400" />
          <Input
            placeholder="搜索项目..."
            value={keyword}
            onChange={(e) => setKeyword(e.target.value)}
            className="pl-9"
          />
        </div>
        <Select value={statusFilter} onValueChange={setStatusFilter}>
          <SelectTrigger className="w-40">
            <SelectValue placeholder="状态筛选" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">全部状态</SelectItem>
            <SelectItem value="draft">草稿</SelectItem>
            <SelectItem value="storyboard_ready">分镜就绪</SelectItem>
            <SelectItem value="video_generating">视频生成中</SelectItem>
            <SelectItem value="completed">已完成</SelectItem>
            <SelectItem value="paused">已暂停</SelectItem>
            <SelectItem value="failed">失败</SelectItem>
          </SelectContent>
        </Select>

        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="outline" size="sm">
              <ArrowUpDown className="mr-2 h-4 w-4" />
              排序
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem onClick={() => toggleSort('created_at')}>
              创建时间 {sortField === 'created_at' && (sortDir === 'desc' ? '↓' : '↑')}
            </DropdownMenuItem>
            <DropdownMenuItem onClick={() => toggleSort('updated_at')}>
              更新时间 {sortField === 'updated_at' && (sortDir === 'desc' ? '↓' : '↑')}
            </DropdownMenuItem>
            <DropdownMenuItem onClick={() => toggleSort('title')}>
              标题 {sortField === 'title' && (sortDir === 'desc' ? '↓' : '↑')}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>

      {/* Grid */}
      {isLoading ? (
        <div className="grid grid-cols-1 gap-6 sm:grid-cols-2 lg:grid-cols-3">
          {Array.from({ length: 6 }).map((_, i) => (
            <div key={i} className="h-72 animate-pulse rounded-xl bg-gradient-to-br from-surface-100 to-surface-50" />
          ))}
        </div>
      ) : sorted.length === 0 ? (
        <EmptyState
          icon={FolderOpen}
          title={keyword || statusFilter !== 'all' ? '没有匹配的串行视频项目' : META.emptyTitle}
          description={
            keyword || statusFilter !== 'all'
              ? '尝试调整搜索词或状态筛选。'
              : META.emptyDescription
          }
          actionLabel={META.createLabel}
          onAction={() => router.push(META.createHref)}
        />
      ) : (
        <div className="grid grid-cols-1 gap-6 sm:grid-cols-2 lg:grid-cols-3">
          {sorted.map((project) => {
            const icons = modelIcons(project)
            return (
              <Card
                key={project.id}
                className="flex flex-col overflow-hidden transition-shadow hover:shadow-md cursor-pointer"
                onClick={() => router.push(`/video-serial/${project.id}`)}
              >
                <div className="relative aspect-video bg-gradient-to-br from-indigo-100 via-violet-100 to-blue-100">
                  {project.cover_url ? (
                    <img src={project.cover_url} alt={project.title} className="h-full w-full object-cover" />
                  ) : (
                    <div className="flex h-full items-center justify-center">
                      <span className="text-4xl drop-shadow-sm">🎞️</span>
                    </div>
                  )}
                  <div className="absolute right-2 top-2">
                    <StatusBadge status={project.status} />
                  </div>
                </div>

                <CardHeader className="pb-2">
                  <div className="flex items-start justify-between">
                    <CardTitle className="line-clamp-1 text-base">{project.title}</CardTitle>
                    <DropdownMenu>
                      <DropdownMenuTrigger asChild>
                        <Button variant="ghost" size="icon" className="h-7 w-7 shrink-0" onClick={(e) => e.stopPropagation()}>
                          <MoreVertical className="h-4 w-4" />
                        </Button>
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align="end" onClick={(e) => e.stopPropagation()}>
                        <DropdownMenuItem onClick={() => router.push(`/video-serial/${project.id}`)}>
                          <ArrowRight className="mr-2 h-4 w-4" /> 查看详情
                        </DropdownMenuItem>
                        {project.status !== 'paused' && project.status !== 'draft' && project.status !== 'completed' && project.status !== 'failed' && (
                          <DropdownMenuItem onClick={() => handleAction('pause', project)}>
                            <Pause className="mr-2 h-4 w-4" /> 暂停
                          </DropdownMenuItem>
                        )}
                        {project.status === 'paused' && (
                          <DropdownMenuItem onClick={() => handleAction('resume', project)}>
                            <Play className="mr-2 h-4 w-4" /> 继续
                          </DropdownMenuItem>
                        )}
                        <DropdownMenuItem onClick={() => handleAction('clone', project)}>
                          <Copy className="mr-2 h-4 w-4" /> 克隆
                        </DropdownMenuItem>
                        <DropdownMenuItem className="text-red-500 focus:text-red-500" onClick={() => handleAction('delete', project)}>
                          <Trash2 className="mr-2 h-4 w-4" /> 删除
                        </DropdownMenuItem>
                      </DropdownMenuContent>
                    </DropdownMenu>
                  </div>
                  {project.description && (
                    <p className="line-clamp-2 text-xs text-surface-500">{project.description}</p>
                  )}
                </CardHeader>

                <CardContent className="flex-1 space-y-3 pb-2">
                  <PipelineProgress progress={project.progress} />
                  <div className="flex items-center justify-between">
                    <div className="flex flex-wrap gap-1">
                      {stripProjectMediaTags(project.style_tags || []).slice(0, 3).map((tag) => (
                        <Badge key={tag} variant="outline" className="px-1.5 py-0 text-[10px]">{tag}</Badge>
                      ))}
                    </div>
                    {icons.length > 0 && (
                      <div className="flex items-center gap-1">
                        {icons.map((ic) => (
                          <span key={ic.type} className="flex h-5 w-5 items-center justify-center rounded-md bg-surface-100 text-surface-500">
                            {MODEL_ICONS[ic.type]}
                          </span>
                        ))}
                      </div>
                    )}
                  </div>
                  {project.storage_used_bytes > 0 && (
                    <div className="flex items-center gap-1 text-[11px] text-surface-400">
                      <HardDrive className="h-3 w-3" />
                      {formatBytes(project.storage_used_bytes)}
                    </div>
                  )}
                </CardContent>

                <CardFooter className="border-t border-surface-100 pb-3 pt-3">
                  <p className="text-xs text-surface-400">
                    {format(new Date(project.created_at), 'yyyy-MM-dd')}
                  </p>
                  <Button variant="default" size="sm" className="ml-auto" onClick={(e) => { e.stopPropagation(); router.push(`/video-serial/${project.id}`) }}>
                    进入 <ArrowRight className="ml-1 h-3.5 w-3.5" />
                  </Button>
                </CardFooter>
              </Card>
            )
          })}
        </div>
      )}

      <AlertDialog open={!!deleteTarget} onOpenChange={(open) => !open && setDeleteTarget(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>确认删除项目</AlertDialogTitle>
            <AlertDialogDescription>
              确定要删除「{deleteTarget?.title}」吗？此操作不可恢复。
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={actionLoading === `delete-${deleteTarget?.id}`}>取消</AlertDialogCancel>
            <AlertDialogAction onClick={confirmDelete} disabled={actionLoading === `delete-${deleteTarget?.id}`}>
              {actionLoading === `delete-${deleteTarget?.id}` ? '删除中...' : '确认删除'}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
