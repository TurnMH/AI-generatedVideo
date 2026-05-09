'use client'

import { useEffect, useMemo, useState } from 'react'
import Link from 'next/link'
import useSWR from 'swr'
import { BookOpen, Plus, Sparkles } from 'lucide-react'
import { projectAPI } from '@/lib/api'
import { matchesProjectMedia, PROJECT_MEDIA_META } from '@/lib/project-media'
import type { Project } from '@/types'
import { Button } from '@/components/ui/button'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { ComicsWorkspace } from '@/components/comics/ComicsWorkspace'
import { DashboardHero } from '@/components/layout/dashboard-hero'
import { DashboardEmptyState } from '@/components/layout/dashboard-empty-state'

export default function ComicsPage() {
  const [selectedProjectId, setSelectedProjectId] = useState<string>('')
  const [preferredProjectId, setPreferredProjectId] = useState<string | null>(null)

  useEffect(() => {
    if (typeof window === 'undefined') return
    setPreferredProjectId(new URLSearchParams(window.location.search).get('project'))
  }, [])

  const { data: projectsRaw, isLoading: projectsLoading } = useSWR(
    ['comics-projects'],
    () => projectAPI.list({ project_type: 'comics', page: 1, page_size: 100 }) as unknown as Promise<{ data?: { items?: Project[] } | Project[] }>
  )

  const projectsData = (projectsRaw as { data?: { items?: Project[] } | Project[] })?.data
  const projects = useMemo(
    () => (Array.isArray(projectsData) ? projectsData : (projectsData?.items ?? [])).filter((project) => matchesProjectMedia(project, 'comics')),
    [projectsData]
  )

  useEffect(() => {
    if (projects.length === 0) return
    if (preferredProjectId && projects.some((project) => String(project.id) === preferredProjectId)) {
      if (selectedProjectId !== preferredProjectId) {
        setSelectedProjectId(preferredProjectId)
      }
      return
    }
    if (!selectedProjectId || !projects.some((project) => String(project.id) === selectedProjectId)) {
      setSelectedProjectId(String(projects[0].id))
    }
  }, [preferredProjectId, projects, selectedProjectId])

  const numericProjectId = Number(selectedProjectId)
  const { data: projectRaw, isLoading: projectLoading } = useSWR(
    Number.isFinite(numericProjectId) && numericProjectId > 0 ? ['comics-project', numericProjectId] : null,
    () => projectAPI.get(numericProjectId) as unknown as Promise<{ data: Project }>
  )
  const project = (projectRaw as { data?: Project })?.data

  return (
    <div className="space-y-6">
      <DashboardHero
        badge="漫画生成工作台"
        badgeIcon={<BookOpen className="h-3.5 w-3.5 text-violet-300" />}
        title="根据剧本生成漫画"
        description="这里是独立的漫画项目列表与工作台，只展示漫画项目，不再和视频项目混用展示。"
        gradientClassName="from-slate-950 via-violet-950 to-slate-900"
        actions={
          <Button asChild className="bg-white text-slate-900 hover:bg-slate-100">
            <Link href={PROJECT_MEDIA_META.comics.createHref}>
              <Plus className="mr-1.5 h-4 w-4" /> {PROJECT_MEDIA_META.comics.createLabel}
            </Link>
          </Button>
        }
        stats={[
          {
            icon: <BookOpen className="h-4 w-4 text-violet-300" />,
            label: '漫画项目',
            value: projects.length,
            description: '独立漫画内容生产中的全部项目',
          },
          {
            icon: <Sparkles className="h-4 w-4 text-cyan-300" />,
            label: '当前项目',
            value: project?.title || '未选择',
            description: '快速切换并进入漫画生成工作台',
          },
          {
            icon: <Plus className="h-4 w-4 text-amber-300" />,
            label: '创建入口',
            value: PROJECT_MEDIA_META.comics.countLabel,
            description: '新建漫画项目后即可进入后续生成与预览流程',
          },
        ]}
      />

      <div className="rounded-[24px] border border-surface-200 bg-white/85 p-4 shadow-sm backdrop-blur">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h2 className="text-base font-semibold text-surface-900">{PROJECT_MEDIA_META.comics.listTitle}</h2>
            <p className="text-sm text-surface-500">共 {projects.length} {PROJECT_MEDIA_META.comics.countLabel}，按项目维度管理分镜和漫画生成。</p>
          </div>
          {project ? (
            <div className="rounded-lg bg-violet-50 px-3 py-2 text-xs text-violet-700">
              当前漫画项目：<span className="font-medium">{project.title}</span>
            </div>
          ) : null}
        </div>

        {!projectsLoading && projects.length > 0 ? (
          <div className="mt-4 grid gap-3 md:grid-cols-2 xl:grid-cols-3">
            {projects.map((item) => {
              const active = String(item.id) === selectedProjectId
              return (
                <button
                  key={item.id}
                  type="button"
                  onClick={() => setSelectedProjectId(String(item.id))}
                  className={`rounded-xl border p-4 text-left transition-colors ${
                    active
                      ? 'border-violet-300 bg-violet-50'
                      : 'border-surface-200 bg-surface-50 hover:border-surface-300 hover:bg-white'
                  }`}
                >
                  <div className="flex items-center justify-between gap-2">
                    <p className="line-clamp-1 text-sm font-medium text-surface-800">{item.title}</p>
                    {active ? <span className="rounded-full bg-violet-100 px-2 py-0.5 text-[10px] text-violet-700">当前</span> : null}
                  </div>
                  <p className="mt-2 line-clamp-2 text-xs leading-5 text-surface-500">
                    {item.description || '用于漫画分镜拆分、漫画生成和预览输出。'}
                  </p>
                </button>
              )
            })}
          </div>
        ) : null}

        <div className="mt-4 flex flex-wrap items-center gap-3">
          <div className="min-w-[220px] flex-1">
            <p className="mb-2 text-xs font-medium text-surface-500">快速切换漫画项目</p>
            <Select value={selectedProjectId} onValueChange={setSelectedProjectId}>
              <SelectTrigger>
                <SelectValue placeholder={projectsLoading ? '项目加载中...' : PROJECT_MEDIA_META.comics.selectPlaceholder} />
              </SelectTrigger>
              <SelectContent>
                {projects.map((item) => (
                  <SelectItem key={item.id} value={String(item.id)}>{item.title}</SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          {project ? (
            <div className="rounded-lg bg-violet-50 px-3 py-2 text-xs text-violet-700">
              <span className="font-medium">当前项目：</span>{project.title}
              {project.description ? <span className="ml-2 text-violet-500">{project.description}</span> : null}
            </div>
          ) : null}
        </div>
      </div>

      {!projectsLoading && projects.length === 0 ? (
        <DashboardEmptyState
          icon={<BookOpen className="mb-3 h-10 w-10 text-surface-300" />}
          title={PROJECT_MEDIA_META.comics.emptyTitle}
          description={PROJECT_MEDIA_META.comics.emptyDescription}
          action={
            <Button asChild>
              <Link href={PROJECT_MEDIA_META.comics.createHref}>
                <Sparkles className="mr-1.5 h-4 w-4" /> 去创建漫画项目
              </Link>
            </Button>
          }
          innerClassName="flex flex-col items-center justify-center rounded-[24px] border border-dashed border-surface-200 bg-[radial-gradient(circle_at_top_left,_rgba(139,92,246,0.08),_transparent_30%),radial-gradient(circle_at_bottom_right,_rgba(99,102,241,0.08),_transparent_28%)] px-6 py-16 text-center"
        />
      ) : projectLoading && selectedProjectId ? (
        <div className="space-y-4 py-6">{[1, 2, 3].map((i) => <div key={i} className="h-20 animate-pulse rounded-lg bg-surface-100" />)}</div>
      ) : project ? (
        <ComicsWorkspace projectId={project.id} project={project} />
      ) : (
        <div className="rounded-xl border border-dashed border-surface-200 bg-surface-50 px-6 py-16 text-center text-sm text-surface-500">
          请选择一个漫画项目进入漫画工作台。
        </div>
      )}
    </div>
  )
}
