'use client'

import { useState } from 'react'
import useSWR from 'swr'
import { projectAPI, storageAPI } from '@/lib/api'
import { Project, StorageDetails } from '@/types'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Progress } from '@/components/ui/progress'
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
import { useToast } from '@/components/ui/toast'
import {
  HardDrive,
  FolderOpen,
  Film,
  Image as ImageIcon,
  FileText,
  Music,
  Trash2,
  RefreshCw,
  ChevronRight,
  ChevronDown,
  BarChart3,
} from 'lucide-react'
import { DashboardHero } from '@/components/layout/dashboard-hero'
import { DashboardEmptyState } from '@/components/layout/dashboard-empty-state'

// ─────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${(bytes / Math.pow(k, i)).toFixed(1)} ${sizes[i]}`
}

const CATEGORY_CONFIG: Record<string, { label: string; icon: React.ElementType; color: string; barColor: string }> = {
  video:     { label: '视频',   icon: Film,      color: 'text-primary-600',   barColor: 'bg-primary-500' },
  image:     { label: '图像',   icon: ImageIcon,  color: 'text-purple-600', barColor: 'bg-purple-500' },
  audio:     { label: '音频',   icon: Music,      color: 'text-green-600',  barColor: 'bg-green-500' },
  script:    { label: '剧本',   icon: FileText,   color: 'text-orange-600', barColor: 'bg-orange-500' },
  storyboard:{ label: '分镜',   icon: BarChart3,  color: 'text-pink-600',   barColor: 'bg-pink-500' },
  other:     { label: '其他',   icon: FolderOpen, color: 'text-surface-600',   barColor: 'bg-surface-400' },
}

// ─────────────────────────────────────────────
// Project Storage Card
// ─────────────────────────────────────────────

function ProjectStorageCard({ project, realBytes }: { project: Project; realBytes: number }) {
  const { toast } = useToast()
  const [expanded, setExpanded] = useState(false)
  const [cleaning, setCleaning] = useState(false)

  const { data, mutate } = useSWR<{ data: StorageDetails }>(
    expanded ? `storage/${project.id}` : null,
    () => storageAPI.getDetails(project.id) as unknown as Promise<{ data: StorageDetails }>
  )

  const details = data?.data
  // Use accurate bytes from storage-service; realBytes is passed from parent (T8)
  const totalBytes = details ? details.total_bytes : realBytes

  const handleCleanHistory = async () => {
    setCleaning(true)
    try {
      await storageAPI.cleanHistory(project.id)
      toast({ title: '清理完成', description: '历史版本文件已清理' })
      mutate()
    } catch {
      toast({ title: '清理失败', variant: 'destructive' })
    } finally {
      setCleaning(false)
    }
  }

  return (
    <Card className="border border-border/50">
      <CardContent className="p-0">
        {/* Header row */}
        <button
          className="w-full p-4 flex items-center gap-4 hover:bg-muted/40 transition-colors text-left"
          onClick={() => setExpanded(!expanded)}
          title={expanded ? '收起存储详情' : '展开查看存储详情'}
        >
          <div className="w-9 h-9 bg-muted rounded-lg flex items-center justify-center flex-shrink-0">
            <FolderOpen className="w-5 h-5 text-muted-foreground" />
          </div>
          <div className="flex-1 min-w-0">
            <div className="flex items-center gap-2">
              <p className="font-medium text-sm truncate">{project.title}</p>
              <Badge variant="outline" className="text-xs flex-shrink-0">{project.status}</Badge>
            </div>
            <div className="flex items-center gap-3 mt-1">
              <span className="text-xs text-muted-foreground">{formatBytes(totalBytes)}</span>
              {totalBytes > 0 && (
                <div className="flex-1 max-w-40">
                  <Progress
                    value={Math.min((totalBytes / (500 * 1024 * 1024)) * 100, 100)}
                    className="h-1.5"
                  />
                </div>
              )}
            </div>
          </div>
          {expanded
            ? <ChevronDown className="w-4 h-4 text-muted-foreground flex-shrink-0" />
            : <ChevronRight className="w-4 h-4 text-muted-foreground flex-shrink-0" />
          }
        </button>

        {/* Expanded details */}
        {expanded && (
          <div className="border-t px-4 pb-4 pt-3 space-y-4">
            {!details ? (
              <p className="text-sm text-muted-foreground">加载中...</p>
            ) : (
              <>
                {/* Category breakdown */}
                <div className="space-y-2.5">
                  <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide">分类占用</p>
                  {Object.entries(details.categories ?? {}).map(([cat, summary]) => {
                    const cfg = CATEGORY_CONFIG[cat] ?? CATEGORY_CONFIG.other
                    const Icon = cfg.icon
                    const pct = details.total_bytes > 0
                      ? (summary.bytes / details.total_bytes) * 100
                      : 0
                    return (
                      <div key={cat} className="flex items-center gap-3">
                        <div className={`w-7 h-7 rounded flex items-center justify-center bg-muted ${cfg.color}`}>
                          <Icon className="w-3.5 h-3.5" />
                        </div>
                        <div className="flex-1">
                          <div className="flex justify-between text-xs mb-1">
                            <span>{cfg.label} <span className="text-muted-foreground">({summary.count} 个文件)</span></span>
                            <span className="font-medium">{formatBytes(summary.bytes)}</span>
                          </div>
                          <div className="h-1.5 bg-muted rounded-full overflow-hidden">
                            <div
                              className={`h-full rounded-full transition-all ${cfg.barColor}`}
                              style={{ width: `${pct}%` }}
                            />
                          </div>
                        </div>
                      </div>
                    )
                  })}
                </div>

                {/* Deletable history files */}
                {details.files?.filter(f => !f.is_current && f.deletable).length > 0 && (
                  <div className="pt-1">
                    <div className="flex items-center justify-between mb-2">
                      <p className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
                        历史版本文件 ({details.files.filter(f => !f.is_current && f.deletable).length} 个)
                      </p>
                      <AlertDialog>
                        <AlertDialogTrigger asChild>
                          <Button variant="outline" size="sm" className="h-7 text-xs gap-1.5" title="清理历史版本文件以释放存储空间">
                            <Trash2 className="w-3 h-3" />
                            一键清理
                          </Button>
                        </AlertDialogTrigger>
                        <AlertDialogContent>
                          <AlertDialogHeader>
                            <AlertDialogTitle>清理历史版本</AlertDialogTitle>
                            <AlertDialogDescription>
                              将删除 {details.files.filter(f => !f.is_current && f.deletable).length} 个历史版本文件，
                              释放约 {formatBytes(details.files.filter(f => !f.is_current && f.deletable).reduce((a, f) => a + f.size_bytes, 0))} 空间。
                              当前使用中的文件不受影响。
                            </AlertDialogDescription>
                          </AlertDialogHeader>
                          <AlertDialogFooter>
                            <AlertDialogCancel>取消</AlertDialogCancel>
                            <AlertDialogAction onClick={handleCleanHistory} disabled={cleaning}>
                              {cleaning ? '清理中...' : '确认清理'}
                            </AlertDialogAction>
                          </AlertDialogFooter>
                        </AlertDialogContent>
                      </AlertDialog>
                    </div>
                    <div className="space-y-1.5">
                      {details.files
                        .filter(f => !f.is_current && f.deletable)
                        .slice(0, 5)
                        .map(file => (
                          <div key={file.id} className="flex items-center justify-between text-xs text-muted-foreground py-1 border-b border-border/40 last:border-0">
                            <span className="truncate flex-1">{file.label}</span>
                            <span className="ml-3 flex-shrink-0">{formatBytes(file.size_bytes)}</span>
                          </div>
                        ))}
                      {details.files.filter(f => !f.is_current && f.deletable).length > 5 && (
                        <p className="text-xs text-muted-foreground text-center pt-1">
                          还有 {details.files.filter(f => !f.is_current && f.deletable).length - 5} 个文件...
                        </p>
                      )}
                    </div>
                  </div>
                )}
              </>
            )}
          </div>
        )}
      </CardContent>
    </Card>
  )
}

// ─────────────────────────────────────────────
// Main Page
// ─────────────────────────────────────────────

export default function StoragePage() {
  const { data: projectsData, mutate } = useSWR(
    'projects/storage-list',
    () => projectAPI.list({ page_size: 100 }) as unknown as Promise<{ data: Project[] }>
  )

  const projects: Project[] = projectsData?.data ?? []
  const projectIds = projects.map((p) => p.id)

  // Fetch accurate storage totals from storage-service files table (T8)
  const { data: totalsData, mutate: mutateTotals } = useSWR(
    projectIds.length > 0 ? ['storage/bulk-totals', projectIds.join(',')] : null,
    () => storageAPI.getBulkTotals(projectIds) as unknown as Promise<{ data: Record<string, number> }>
  )
  const totalsMap: Record<number, number> = {}
  if (totalsData?.data) {
    for (const [k, v] of Object.entries(totalsData.data)) {
      totalsMap[Number(k)] = v
    }
  }

  // Use accurate bytes from storage-service; fall back to project field only if not yet loaded
  const getProjectBytes = (p: Project) =>
    totalsMap[p.id] !== undefined ? totalsMap[p.id] : (p.storage_used_bytes ?? 0)

  const totalBytes = projects.reduce((sum, p) => sum + getProjectBytes(p), 0)
  const totalProjects = projects.length

  const storageLimit = 10 * 1024 * 1024 * 1024 // 10 GB example quota
  const usagePercent = Math.min((totalBytes / storageLimit) * 100, 100)

  const handleRefresh = () => {
    mutate()
    mutateTotals()
  }

  return (
    <div className="mx-auto max-w-4xl space-y-6 p-6">
      <DashboardHero
        badge="存储监控中心"
        badgeIcon={<HardDrive className="h-3.5 w-3.5 text-sky-300" />}
        title="存储管理"
        description="查看所有项目的存储用量，追踪空间分布，并清理历史版本文件释放配额。"
        gradientClassName="from-slate-950 via-sky-950 to-slate-900"
        actions={
          <Button variant="outline" className="border-white/10 bg-white/10 text-white hover:bg-white/15 hover:text-white" onClick={handleRefresh} title="刷新存储用量信息">
            <RefreshCw className="mr-2 h-4 w-4" />
            刷新
          </Button>
        }
        stats={[
          {
            icon: <HardDrive className="h-4 w-4 text-sky-300" />,
            label: '总用量',
            value: formatBytes(totalBytes),
            description: '当前所有项目已消耗的存储空间',
          },
          {
            icon: <FolderOpen className="h-4 w-4 text-cyan-300" />,
            label: '项目总数',
            value: totalProjects,
            description: '已纳入存储统计的项目数量',
          },
          {
            icon: <BarChart3 className="h-4 w-4 text-emerald-300" />,
            label: '剩余配额',
            value: formatBytes(Math.max(storageLimit - totalBytes, 0)),
            description: `${usagePercent.toFixed(1)}% 已使用`,
          },
        ]}
      />

      {/* Overview */}
      <div className="grid grid-cols-3 gap-4">
        <Card className="col-span-2">
          <CardContent className="p-5">
            <div className="flex items-center gap-3 mb-4">
              <div className="w-10 h-10 bg-primary-100 rounded-xl flex items-center justify-center">
                <HardDrive className="w-5 h-5 text-primary-600" />
              </div>
              <div>
                <p className="text-2xl font-bold">{formatBytes(totalBytes)}</p>
                <p className="text-xs text-muted-foreground">总存储用量</p>
              </div>
            </div>
            <div className="space-y-1.5">
              <div className="flex justify-between text-xs text-muted-foreground">
                <span>已用 {formatBytes(totalBytes)}</span>
                <span>配额 {formatBytes(storageLimit)}</span>
              </div>
              <Progress value={usagePercent} className="h-2.5" />
              <p className="text-xs text-muted-foreground">{usagePercent.toFixed(1)}% 已使用</p>
            </div>
          </CardContent>
        </Card>

        <div className="space-y-3">
          <Card>
            <CardContent className="p-4">
              <p className="text-2xl font-bold">{totalProjects}</p>
              <p className="text-xs text-muted-foreground mt-0.5">项目总数</p>
            </CardContent>
          </Card>
          <Card>
            <CardContent className="p-4">
              <p className="text-2xl font-bold text-orange-600">
                {formatBytes(Math.max(storageLimit - totalBytes, 0))}
              </p>
              <p className="text-xs text-muted-foreground mt-0.5">剩余配额</p>
            </CardContent>
          </Card>
        </div>
      </div>

      {/* Category overview (stacked bar) */}
      {projects.length > 0 && (
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-base">存储分布</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="flex items-center gap-4 flex-wrap">
              {Object.entries(CATEGORY_CONFIG).map(([cat, cfg]) => (
                <div key={cat} className="flex items-center gap-1.5 text-xs">
                  <div className={`w-2.5 h-2.5 rounded-sm ${cfg.barColor}`} />
                  <span className="text-muted-foreground">{cfg.label}</span>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      )}

      {/* Per-project list */}
      <div>
        <div className="flex items-center justify-between mb-3">
          <h2 className="text-base font-semibold">项目存储明细</h2>
          <span className="text-xs text-muted-foreground">点击展开查看详情</span>
        </div>

        {projects.length === 0 ? (
          <Card>
            <CardContent className="py-3">
              <DashboardEmptyState
                icon={<HardDrive className="mx-auto mb-3 h-10 w-10 opacity-30" />}
                title="暂无项目"
                description="创建项目并产出资源后，这里会自动统计对应空间占用。"
                className="rounded-[24px] border-0 bg-transparent p-0 shadow-none"
                innerClassName="rounded-[24px] border border-dashed border-surface-200 bg-[radial-gradient(circle_at_top_left,_rgba(14,165,233,0.08),_transparent_30%),radial-gradient(circle_at_bottom_right,_rgba(16,185,129,0.08),_transparent_28%)] py-12 text-center text-muted-foreground"
              />
            </CardContent>
          </Card>
        ) : (
          <div className="space-y-2">
            {[...projects]
              .sort((a, b) => getProjectBytes(b) - getProjectBytes(a))
              .map(p => (
                <ProjectStorageCard key={p.id} project={p} realBytes={getProjectBytes(p)} />
              ))
            }
          </div>
        )}
      </div>
    </div>
  )
}
