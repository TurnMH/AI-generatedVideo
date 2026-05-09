
'use client'

import React, { useEffect, useMemo, useRef, useState } from 'react'
import useSWR from 'swr'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Button } from '@/components/ui/button'
import { Image as ImageIcon, LayoutGrid, Mic, Video, Sparkles, RefreshCw, Loader2, CheckCircle2, AlertCircle, Play, Pause, RotateCcw, Bot } from 'lucide-react'
import { assetAPI, projectAPI, storyboardAPI } from '@/lib/api'
import { useToast } from '@/components/ui/toast'
import type { Project, Episode, Asset, Storyboard } from '@/types'

import { AssetsTab } from './tabs/AssetsTab'
import { StoryboardTab } from './tabs/StoryboardTab'
import { DubbingTab } from './tabs/DubbingTab'
import { VideoTab } from './tabs/VideoTab'

interface EpisodeWorkspaceProps {
  projectId: number
  episodeId: number
  episode?: Episode
  project: Project
  initialTab?: 'assets' | 'storyboard' | 'dubbing' | 'video'
  initialAwaitingAutoStoryboard?: boolean
}

export function EpisodeWorkspace({ projectId, episodeId, episode, project, initialTab = 'assets', initialAwaitingAutoStoryboard = false }: EpisodeWorkspaceProps) {
  const [activeTab, setActiveTab] = useState(initialTab)
  const [isExtracting, setIsExtracting] = useState(false)
  const [isExtractingStoryboards, setIsExtractingStoryboards] = useState(false)
  const [awaitingAutoStoryboard, setAwaitingAutoStoryboard] = useState(initialAwaitingAutoStoryboard)
  const autoSwitchedRef = useRef(false)
  const { toast } = useToast()
  const isSerial = project.project_type === 'video_serial'

  const { data: assetsData, mutate: mutateAssets } = useSWR(
    ['episode-workspace-assets', projectId, episodeId],
    () => assetAPI.list(projectId, { episode_id: episodeId }) as unknown as Promise<{ data: Asset[] }>,
    {
      refreshInterval: (data) => {
        const items = (data as { data?: Asset[] })?.data ?? []
        return awaitingAutoStoryboard || items.some((asset) => ['extracting', 'pending', 'generating'].includes(asset.status)) ? 3000 : 0
      },
    }
  )
  const episodeAssets = assetsData?.data ?? []

  const { data: storyboardsData, mutate: mutateStoryboards } = useSWR(
    ['episode-workspace-storyboards', projectId, episodeId],
    () => storyboardAPI.listAll(projectId, { episode_id: episodeId }) as Promise<{ data: Storyboard[] }>,
    {
      refreshInterval: (data) => {
        const items = (data as { data?: Storyboard[] })?.data ?? []
        return awaitingAutoStoryboard || items.some((sb) => ['pending', 'generating', 'paused'].includes(sb.status)) ? 3000 : 0
      },
    }
  )
  const episodeStoryboards = storyboardsData?.data ?? []

  const assetStats = useMemo(() => {
    const extracting = episodeAssets.some((asset) => asset.status === 'extracting')
    const visibleAssets = episodeAssets.filter((asset) => asset.name !== '__extracting__' && asset.status !== 'extracting')
    const completed = visibleAssets.filter((asset) => asset.status === 'completed').length
    const paused = visibleAssets.filter((asset) => asset.status === 'paused').length
    const generating = visibleAssets.filter((asset) => asset.status === 'generating' || asset.status === 'pending').length
    const active = paused + generating
    const failed = visibleAssets.filter((asset) => asset.status === 'failed' || asset.status === 'qa_failed').length
    return {
      total: visibleAssets.length,
      completed,
      active,
      paused,
      generating,
      failed,
      extracting,
    }
  }, [episodeAssets])

  const storyboardStats = useMemo(() => {
    const completed = episodeStoryboards.filter((sb) => sb.status === 'completed' && sb.image_url).length
    const paused = episodeStoryboards.filter((sb) => sb.status === 'paused').length
    const pending = episodeStoryboards.filter((sb) => sb.status === 'pending').length
    const generating = episodeStoryboards.filter((sb) => sb.status === 'generating').length
    const active = paused + generating
    const failed = episodeStoryboards.filter((sb) => sb.status === 'failed').length
    return {
      total: episodeStoryboards.length,
      completed,
      pending,
      active,
      paused,
      generating,
      failed,
    }
  }, [episodeStoryboards])

  const serialStoryboardStats = useMemo(() => {
    const sceneGroups = new Set(
      episodeStoryboards
        .map((storyboard) => storyboard.scene_group_key)
        .filter((groupKey): groupKey is string => Boolean(groupKey))
    ).size
    const firstClips = episodeStoryboards.filter((storyboard) => storyboard.scene_group_key && storyboard.is_scene_first_clip)
    const firstClipReady = firstClips.filter((storyboard) => storyboard.status === 'completed' && storyboard.image_url).length

    return {
      sceneGroups,
      firstClipTotal: firstClips.length,
      firstClipReady,
    }
  }, [episodeStoryboards])

  const storyboardStageLabel = isSerial ? '镜头拆分与首帧' : '镜头拆分与出图'
  const storyboardWorkspaceLabel = isSerial ? '镜头工作台' : '分镜工作台'
  const hasRenderableStoryboard = isSerial ? serialStoryboardStats.firstClipReady > 0 : storyboardStats.completed > 0

  const pipelineStatus = useMemo(() => {
    if (assetStats.extracting) {
      return {
        tone: 'amber',
        title: '资源提取中',
        description: '系统正在分析本集角色、场景与道具，请稍候。',
      }
    }
    if (awaitingAutoStoryboard && storyboardStats.total === 0 && episode?.status !== 'scene_splitting') {
      return {
        tone: 'blue',
        title: '资源提取完成，正在自动开启镜头拆分',
        description: isSerial ? '系统会继续拆分镜头并生成串行场景分组，无需再手动点一次。' : '系统会继续拆分镜头条目，无需再手动点一次。',
      }
    }
    if (isExtractingStoryboards || episode?.status === 'scene_splitting') {
      return {
        tone: 'blue',
        title: isSerial ? '镜头拆分中，等待首帧生成' : '镜头拆分中，等待出图',
        description: isSerial
          ? `当前已识别 ${serialStoryboardStats.sceneGroups || 0} 个场景组，拆分完成后会继续进入首帧生成。`
          : `当前正在拆分镜头条目，拆分完成后即可启动分镜图片生成。`,
      }
    }
    if (storyboardStats.generating > 0 || storyboardStats.paused > 0) {
      return {
        tone: 'blue',
        title: isSerial ? '首帧生成中' : '分镜图片生成中',
        description: isSerial
          ? `当前已识别 ${serialStoryboardStats.sceneGroups || 0} 个场景组，首帧就绪 ${serialStoryboardStats.firstClipReady}/${serialStoryboardStats.firstClipTotal || 0}。`
          : `当前集已出图 ${storyboardStats.completed}/${storyboardStats.total || 0}，可继续在镜头工作台查看新增结果。`,
      }
    }
    if (storyboardStats.total > 0 && storyboardStats.pending > 0 && storyboardStats.completed === 0) {
      return {
        tone: 'violet',
        title: isSerial ? '镜头已拆分，待生成首帧' : '镜头已拆分，待生成图片',
        description: isSerial
          ? `当前集已拆分 ${storyboardStats.total} 条镜头，场景组 ${serialStoryboardStats.sceneGroups || 0}，尚未开始首帧生成。`
          : `当前集已拆分 ${storyboardStats.total} 条镜头，尚未开始分镜图片生成。`,
      }
    }
    if (storyboardStats.total > 0) {
      return {
        tone: 'green',
        title: isSerial ? '镜头已拆分' : '镜头与图片已就绪',
        description: isSerial
          ? `当前集已拆分 ${storyboardStats.total} 条镜头，场景组 ${serialStoryboardStats.sceneGroups || 0}，首帧 ${serialStoryboardStats.firstClipReady}/${serialStoryboardStats.firstClipTotal || 0}。`
          : `当前集已累计 ${storyboardStats.completed}/${storyboardStats.total} 条可用分镜图片。`,
      }
    }
    if (assetStats.active > 0) {
      return {
        tone: 'blue',
        title: '资源生成中',
        description: `当前集已完成 ${assetStats.completed}/${assetStats.total} 个资源，正在生成剩余资源。`,
      }
    }
    if (assetStats.total > 0) {
      return {
        tone: 'violet',
        title: '资源已就绪',
        description: `当前集已识别 ${assetStats.completed}/${assetStats.total} 个资源，可直接进入${storyboardStageLabel}。`,
      }
    }
    return {
      tone: 'slate',
      title: '等待开始',
      description: `你可以先提取本集资源，系统会在完成后自动衔接${storyboardStageLabel}。`,
    }
  }, [assetStats, awaitingAutoStoryboard, storyboardStats, isExtractingStoryboards, episode?.status, isSerial, serialStoryboardStats, storyboardStageLabel])

  type WorkflowStepStatus = 'done' | 'current' | 'pending' | 'failed' | 'skipped'
  type WorkflowStepKey = 'assets' | 'storyboard' | 'dubbing' | 'video'
  type WorkflowStep = {
    key: WorkflowStepKey
    label: string
    status: WorkflowStepStatus
    statusLabel: string
    hint: string
  }

  const workflowSteps = useMemo<WorkflowStep[]>(() => {
    const dubbingEnabled = project.enable_dubbing || project.enable_subtitle

    const assetStepStatus = assetStats.failed > 0 && assetStats.completed === 0 && !assetStats.extracting && assetStats.active === 0
      ? 'failed'
      : assetStats.extracting || assetStats.active > 0 || isExtracting
        ? 'current'
        : assetStats.total > 0 && assetStats.failed === 0
          ? 'done'
          : assetStats.total > 0
            ? 'current'
            : 'pending'

    const storyboardStepStatus = storyboardStats.failed > 0 && storyboardStats.completed === 0 && storyboardStats.active === 0 && storyboardStats.pending === 0
      ? 'failed'
      : awaitingAutoStoryboard || isExtractingStoryboards || episode?.status === 'scene_splitting' || storyboardStats.active > 0
        ? 'current'
        : storyboardStats.total > 0 && storyboardStats.pending === 0 && storyboardStats.failed === 0
          ? 'done'
          : storyboardStats.total > 0
            ? 'pending'
            : 'pending'

    return [
      {
        key: 'assets',
        label: '资源提取',
        status: assetStepStatus,
        statusLabel: assetStats.extracting || isExtracting
          ? '提取中'
          : assetStats.active > 0
            ? '生成中'
            : assetStepStatus === 'done'
              ? '已完成'
              : assetStepStatus === 'failed'
                ? '异常'
                : '待开始',
        hint: assetStats.extracting || isExtracting
          ? `提取中...`
          : assetStats.active > 0
            ? `生成中 ${assetStats.completed}/${assetStats.total || '?'}`
            : assetStats.failed > 0
              ? `${assetStats.completed}/${assetStats.total}，失败 ${assetStats.failed}`
              : assetStats.total > 0
                ? `${assetStats.completed}/${assetStats.total} 个资源`
                : '尚未开始',
      },
      {
        key: 'storyboard',
        label: storyboardStageLabel,
        status: storyboardStepStatus,
        statusLabel: awaitingAutoStoryboard
          ? '排队中'
          : isExtractingStoryboards || episode?.status === 'scene_splitting'
            ? '拆分中'
            : storyboardStats.generating > 0 || storyboardStats.paused > 0
              ? (isSerial ? '首帧生成中' : '出图中')
            : storyboardStepStatus === 'done'
              ? '已完成'
              : storyboardStepStatus === 'failed'
                ? '异常'
                : storyboardStats.total > 0
                  ? (isSerial ? '待首帧' : '待出图')
                  : assetStats.total > 0
                    ? '待开始'
                  : '待资源',
        hint: awaitingAutoStoryboard
          ? '资源完成后自动开启'
          : isExtractingStoryboards || episode?.status === 'scene_splitting'
            ? '正在拆分镜头条目'
            : storyboardStats.generating > 0 || storyboardStats.paused > 0
            ? isSerial
              ? `场景组 ${serialStoryboardStats.sceneGroups || 0}，首帧 ${serialStoryboardStats.firstClipReady}/${serialStoryboardStats.firstClipTotal || 0}`
              : `出图中 ${storyboardStats.completed}/${storyboardStats.total || '?'}`
            : storyboardStats.failed > 0
              ? `${storyboardStats.completed}/${storyboardStats.total}，失败 ${storyboardStats.failed}`
              : storyboardStats.total > 0
                ? isSerial
                  ? `${storyboardStats.total} 条镜头，待生成首帧`
                  : `${storyboardStats.total} 条镜头，待生成图片`
                : assetStats.total > 0
                  ? '可手动启动'
                  : '依赖资源结果',
      },
      {
        key: 'dubbing',
        label: '语音合成',
        status: dubbingEnabled ? (storyboardStats.total > 0 ? 'pending' : 'pending') : 'skipped',
        statusLabel: !dubbingEnabled
          ? '未启用'
          : storyboardStats.total > 0
            ? '可开始'
            : '待分镜',
        hint: !dubbingEnabled
          ? '项目未启用'
          : storyboardStats.total > 0
            ? '分镜已就绪'
            : '需先完成分镜',
      },
      {
        key: 'video',
        label: '视频成片',
        status: storyboardStats.total > 0 ? 'pending' : 'pending',
        statusLabel: hasRenderableStoryboard
          ? '可开始'
          : '待前序',
        hint: hasRenderableStoryboard
          ? isSerial ? '首帧已就绪，可继续串行视频' : '分镜图片已就绪，可继续成片'
          : '需先完成前序步骤',
      },
    ]
  }, [assetStats, storyboardStats, awaitingAutoStoryboard, isExtracting, isExtractingStoryboards, episode?.status, project.enable_dubbing, project.enable_subtitle, storyboardStageLabel, isSerial, serialStoryboardStats, hasRenderableStoryboard])

  useEffect(() => {
    autoSwitchedRef.current = false
    setAwaitingAutoStoryboard(false)
    setActiveTab(initialTab)
  }, [episodeId, initialTab])

  useEffect(() => {
    const storyboardStarted = episode?.status === 'scene_splitting' || storyboardStats.total > 0 || storyboardStats.active > 0
    if (!awaitingAutoStoryboard || !storyboardStarted) return

    setAwaitingAutoStoryboard(false)
    setActiveTab('storyboard')
    if (!autoSwitchedRef.current) {
      autoSwitchedRef.current = true
      toast({
        title: '已自动开启本集镜头拆分',
        description: `资源提取完成，已切换到${storyboardWorkspaceLabel}。`,
        variant: 'success',
      })
    }
  }, [awaitingAutoStoryboard, episode?.status, storyboardStats.total, storyboardStats.active, toast, storyboardWorkspaceLabel])

  // 资源全部完成且处于等待自动拆分状态时，自动触发镜头拆分
  const autoExtractTriggeredRef = useRef(false)
  useEffect(() => {
    if (!awaitingAutoStoryboard) {
      autoExtractTriggeredRef.current = false
      return
    }
    if (autoExtractTriggeredRef.current) return
    if (assetStats.extracting || assetStats.active > 0 || assetStats.total === 0) return
    if (storyboardStats.total > 0 || episode?.status === 'scene_splitting') return
    // 资源已全部完成，自动启动镜头拆分
    autoExtractTriggeredRef.current = true
    handleExtractStoryboards()
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [awaitingAutoStoryboard, assetStats.extracting, assetStats.active, assetStats.total, storyboardStats.total, episode?.status])

  const handleExtractAssets = async () => {
    setIsExtracting(true)
    setAwaitingAutoStoryboard(true)
    autoSwitchedRef.current = false
    setActiveTab('assets')
    try {
      await assetAPI.extractEpisode(projectId, episodeId)
      await mutateAssets()
      toast({ title: '任务已提交', description: `正在提取本集资源，完成后会自动开启${storyboardStageLabel}。` })
    } catch (error: any) {
      setAwaitingAutoStoryboard(false)
      toast({
        title: '提取失败',
        description: error?.response?.data?.error || '服务器发生错误',
        variant: 'destructive',
      })
    } finally {
      setIsExtracting(false)
    }
  }

  const handleExtractStoryboards = () => {
    setIsExtractingStoryboards(true)
    setAwaitingAutoStoryboard(false)
    setActiveTab('storyboard')
    // 资源已全部完成时跳过后端的资产预刷新，避免触发不必要的重新提取
    const assetsReady = assetStats.total > 0 && assetStats.active === 0 && !assetStats.extracting
    ;(projectAPI.extractEpisodeStoryboards(projectId, episodeId, assetsReady) as Promise<unknown>)
      .then(() => {
        void mutateStoryboards()
        // 延迟重置，给轮询时间发现 scene_splitting 状态后接管
        setTimeout(() => setIsExtractingStoryboards(false), 5000)
      })
      .catch((error: any) => {
        setIsExtractingStoryboards(false)
        toast({
          title: '本集分镜提取失败',
          description: error?.response?.data?.error || error?.response?.data?.message || '服务器发生错误',
          variant: 'destructive',
        })
      })
    toast({
      title: '镜头拆分已开始',
      description: isSerial ? '正在为本集拆分镜头并生成场景分组，可在下方镜头工作台查看进度。' : '正在为本集拆分镜头条目，可在下方镜头工作台查看进度。',
      variant: 'success',
    })
  }

  const toneClass = {
    amber: 'border-amber-200 bg-amber-50 text-amber-800',
    blue: 'border-blue-200 bg-blue-50 text-blue-800',
    green: 'border-emerald-200 bg-emerald-50 text-emerald-800',
    violet: 'border-violet-200 bg-violet-50 text-violet-800',
    slate: 'border-surface-200 bg-surface-50 text-surface-700',
  }[pipelineStatus.tone]


  const workflowStatusLabel = {
    done: '已完成',
    current: '处理中',
    pending: '待开始',
    failed: '异常',
    skipped: '未启用',
  } as const

  const workflowStatusClass = {
    done: 'border-emerald-200 bg-emerald-50 text-emerald-700',
    current: 'border-blue-200 bg-blue-50 text-blue-700',
    pending: 'border-surface-200 bg-surface-50 text-surface-600',
    failed: 'border-red-200 bg-red-50 text-red-700',
    skipped: 'border-surface-200 bg-surface-100 text-surface-400',
  } as const

  const workflowStepTheme: Record<WorkflowStepKey, {
    tab: string
    title: string
    hint: string
    click: string
    currentBadge: string
    currentDot: string
    currentStrip: string
    pendingStrip: string
    pulse: string
    icon: string
  }> = {
    assets: {
      tab: 'hover:border-amber-300 hover:bg-amber-50/40 data-[state=active]:border-amber-400 data-[state=active]:bg-amber-50/30',
      title: 'group-hover:text-amber-800 group-data-[state=active]:text-amber-900',
      hint: 'group-hover:text-amber-700 group-data-[state=active]:text-amber-700',
      click: 'text-amber-600',
      currentBadge: 'border-amber-200 bg-amber-50 text-amber-700',
      currentDot: 'bg-amber-500',
      currentStrip: 'bg-amber-500',
      pendingStrip: 'bg-amber-200 group-data-[state=active]:bg-amber-300',
      pulse: 'bg-amber-300',
      icon: 'bg-amber-50 text-amber-600 group-data-[state=active]:bg-amber-100 group-data-[state=active]:text-amber-700',
    },
    storyboard: {
      tab: 'hover:border-blue-300 hover:bg-blue-50/40 data-[state=active]:border-blue-400 data-[state=active]:bg-blue-50/30',
      title: 'group-hover:text-blue-800 group-data-[state=active]:text-blue-900',
      hint: 'group-hover:text-blue-700 group-data-[state=active]:text-blue-700',
      click: 'text-blue-600',
      currentBadge: 'border-blue-200 bg-blue-50 text-blue-700',
      currentDot: 'bg-blue-500',
      currentStrip: 'bg-blue-500',
      pendingStrip: 'bg-blue-200 group-data-[state=active]:bg-blue-300',
      pulse: 'bg-blue-300',
      icon: 'bg-blue-50 text-blue-600 group-data-[state=active]:bg-blue-100 group-data-[state=active]:text-blue-700',
    },
    dubbing: {
      tab: 'hover:border-cyan-300 hover:bg-cyan-50/40 data-[state=active]:border-cyan-400 data-[state=active]:bg-cyan-50/30',
      title: 'group-hover:text-cyan-800 group-data-[state=active]:text-cyan-900',
      hint: 'group-hover:text-cyan-700 group-data-[state=active]:text-cyan-700',
      click: 'text-cyan-600',
      currentBadge: 'border-cyan-200 bg-cyan-50 text-cyan-700',
      currentDot: 'bg-cyan-500',
      currentStrip: 'bg-cyan-500',
      pendingStrip: 'bg-cyan-200 group-data-[state=active]:bg-cyan-300',
      pulse: 'bg-cyan-300',
      icon: 'bg-cyan-50 text-cyan-600 group-data-[state=active]:bg-cyan-100 group-data-[state=active]:text-cyan-700',
    },
    video: {
      tab: 'hover:border-emerald-300 hover:bg-emerald-50/40 data-[state=active]:border-emerald-400 data-[state=active]:bg-emerald-50/30',
      title: 'group-hover:text-emerald-800 group-data-[state=active]:text-emerald-900',
      hint: 'group-hover:text-emerald-700 group-data-[state=active]:text-emerald-700',
      click: 'text-emerald-600',
      currentBadge: 'border-emerald-200 bg-emerald-50 text-emerald-700',
      currentDot: 'bg-emerald-500',
      currentStrip: 'bg-emerald-500',
      pendingStrip: 'bg-emerald-200 group-data-[state=active]:bg-emerald-300',
      pulse: 'bg-emerald-300',
      icon: 'bg-emerald-50 text-emerald-600 group-data-[state=active]:bg-emerald-100 group-data-[state=active]:text-emerald-700',
    },
  }

  const activeWorkflowTheme = workflowStepTheme[activeTab]

  const sidebarTheme: Record<WorkflowStepKey, {
    card: string
    cardStrip: string
    iconWrap: string
    icon: string
    title: string
    desc: string
    panel: string
    panelTitle: string
    panelDesc: string
    primaryButton: string
    secondaryButton: string
    subtle: string
  }> = {
    assets: {
      card: 'border-amber-200 bg-gradient-to-br from-amber-50 via-white to-amber-50/70',
      cardStrip: 'bg-amber-400',
      iconWrap: 'bg-amber-100/80',
      icon: 'text-amber-700',
      title: 'text-amber-900',
      desc: 'text-amber-800/90',
      panel: 'border-amber-200/80 bg-amber-50/35',
      panelTitle: 'text-amber-900',
      panelDesc: 'text-amber-700',
      primaryButton: 'border-amber-500 bg-amber-500 text-white hover:bg-amber-600 hover:border-amber-600',
      secondaryButton: 'border-amber-200 bg-white text-amber-800 hover:bg-amber-50 hover:border-amber-300',
      subtle: 'text-amber-700',
    },
    storyboard: {
      card: 'border-blue-200 bg-gradient-to-br from-blue-50 via-white to-blue-50/70',
      cardStrip: 'bg-blue-400',
      iconWrap: 'bg-blue-100/80',
      icon: 'text-blue-700',
      title: 'text-blue-900',
      desc: 'text-blue-800/90',
      panel: 'border-blue-200/80 bg-blue-50/35',
      panelTitle: 'text-blue-900',
      panelDesc: 'text-blue-700',
      primaryButton: 'border-blue-500 bg-blue-500 text-white hover:bg-blue-600 hover:border-blue-600',
      secondaryButton: 'border-blue-200 bg-white text-blue-800 hover:bg-blue-50 hover:border-blue-300',
      subtle: 'text-blue-700',
    },
    dubbing: {
      card: 'border-cyan-200 bg-gradient-to-br from-cyan-50 via-white to-cyan-50/70',
      cardStrip: 'bg-cyan-400',
      iconWrap: 'bg-cyan-100/80',
      icon: 'text-cyan-700',
      title: 'text-cyan-900',
      desc: 'text-cyan-800/90',
      panel: 'border-cyan-200/80 bg-cyan-50/35',
      panelTitle: 'text-cyan-900',
      panelDesc: 'text-cyan-700',
      primaryButton: 'border-cyan-500 bg-cyan-500 text-white hover:bg-cyan-600 hover:border-cyan-600',
      secondaryButton: 'border-cyan-200 bg-white text-cyan-800 hover:bg-cyan-50 hover:border-cyan-300',
      subtle: 'text-cyan-700',
    },
    video: {
      card: 'border-emerald-200 bg-gradient-to-br from-emerald-50 via-white to-emerald-50/70',
      cardStrip: 'bg-emerald-400',
      iconWrap: 'bg-emerald-100/80',
      icon: 'text-emerald-700',
      title: 'text-emerald-900',
      desc: 'text-emerald-800/90',
      panel: 'border-emerald-200/80 bg-emerald-50/35',
      panelTitle: 'text-emerald-900',
      panelDesc: 'text-emerald-700',
      primaryButton: 'border-emerald-500 bg-emerald-500 text-white hover:bg-emerald-600 hover:border-emerald-600',
      secondaryButton: 'border-emerald-200 bg-white text-emerald-800 hover:bg-emerald-50 hover:border-emerald-300',
      subtle: 'text-emerald-700',
    },
  }

  const activeSidebarTheme = sidebarTheme[activeTab]

  const pipelineToneBadgeClass = {
    amber: 'border-amber-200 bg-amber-100 text-amber-800',
    blue: 'border-blue-200 bg-blue-100 text-blue-800',
    green: 'border-emerald-200 bg-emerald-100 text-emerald-800',
    violet: 'border-violet-200 bg-violet-100 text-violet-800',
    slate: 'border-surface-200 bg-surface-100 text-surface-700',
  }[pipelineStatus.tone]

  const pipelineToneLabel = {
    amber: '处理中',
    blue: '进行中',
    green: '已就绪',
    violet: '可继续',
    slate: '待开始',
  }[pipelineStatus.tone]

  const contentShellTheme: Record<WorkflowStepKey, {
    frame: string
    strip: string
    metaPill: string
    metaCount: string
    contentWrap: string
  }> = {
    assets: {
      frame: 'border-amber-200/80 bg-gradient-to-br from-white via-white to-amber-50/35',
      strip: 'bg-amber-400',
      metaPill: 'border-amber-200 bg-amber-50/70 text-amber-800',
      metaCount: 'text-amber-700',
      contentWrap: 'border-amber-200/70 bg-amber-50/20',
    },
    storyboard: {
      frame: 'border-blue-200/80 bg-gradient-to-br from-white via-white to-blue-50/35',
      strip: 'bg-blue-400',
      metaPill: 'border-blue-200 bg-blue-50/70 text-blue-800',
      metaCount: 'text-blue-700',
      contentWrap: 'border-blue-200/70 bg-blue-50/20',
    },
    dubbing: {
      frame: 'border-cyan-200/80 bg-gradient-to-br from-white via-white to-cyan-50/35',
      strip: 'bg-cyan-400',
      metaPill: 'border-cyan-200 bg-cyan-50/70 text-cyan-800',
      metaCount: 'text-cyan-700',
      contentWrap: 'border-cyan-200/70 bg-cyan-50/20',
    },
    video: {
      frame: 'border-emerald-200/80 bg-gradient-to-br from-white via-white to-emerald-50/35',
      strip: 'bg-emerald-400',
      metaPill: 'border-emerald-200 bg-emerald-50/70 text-emerald-800',
      metaCount: 'text-emerald-700',
      contentWrap: 'border-emerald-200/70 bg-emerald-50/20',
    },
  }

  const activeContentShellTheme = contentShellTheme[activeTab]

  const storyboardImageLabel = isSerial ? '首帧图片' : '分镜图片'

  const resourceButtonDisabled = isExtracting || assetStats.extracting
  const storyboardButtonDisabled = isExtractingStoryboards || assetStats.extracting || awaitingAutoStoryboard ||
    episode?.status === 'scene_splitting' || storyboardStats.active > 0

  const [generateTrigger, setGenerateTrigger] = useState(0)
  const [regenerateTrigger, setRegenerateTrigger] = useState(0)
  const [sbGenerateTrigger, setSbGenerateTrigger] = useState(0)
  const [sbRegenerateTrigger, setSbRegenerateTrigger] = useState(0)
  const [sbPauseTrigger, setSbPauseTrigger] = useState(0)
  const [sbResumeTrigger, setSbResumeTrigger] = useState(0)
  const [sbAuditTrigger, setSbAuditTrigger] = useState(0)
  const [autoMatchingVoices, setAutoMatchingVoices] = useState(false)
  const [pausingGeneration, setPausingGeneration] = useState(false)
  const [resumingGeneration, setResumingGeneration] = useState(false)

  const handleAutoMatchVoices = async () => {
    setAutoMatchingVoices(true)
    try {
      const res = await assetAPI.autoMatchVoices(projectId) as unknown as { data?: { updated?: number } }
      const updated = res?.data?.updated ?? 0
      toast({
        title: updated > 0 ? `自动匹配完成，已为 ${updated} 个人物分配音色` : '所有人物已有音色绑定，无需匹配',
        variant: updated > 0 ? 'success' : 'default',
      })
      mutateAssets()
    } catch {
      toast({ title: '自动匹配音色失败', variant: 'destructive' })
    } finally {
      setAutoMatchingVoices(false)
    }
  }

  const handlePauseGeneration = async () => {
    setPausingGeneration(true)
    try {
      const res = await assetAPI.pauseGeneration(projectId) as unknown as { data?: { paused?: number } }
      toast({ title: `已暂停资源生成（${res?.data?.paused ?? 0} 项）`, variant: 'success' })
      mutateAssets()
    } catch {
      toast({ title: '暂停资源生成失败', variant: 'destructive' })
    } finally {
      setPausingGeneration(false)
    }
  }

  const handleResumeGeneration = async () => {
    setResumingGeneration(true)
    try {
      const res = await assetAPI.resumeGeneration(projectId) as unknown as { data?: { triggered?: number } }
      const triggered = res?.data?.triggered ?? 0
      toast({ title: triggered > 0 ? `已继续资源生成（${triggered} 项）` : '当前没有已暂停的资源', variant: triggered > 0 ? 'success' : 'default' })
      mutateAssets()
    } catch {
      toast({ title: '继续资源生成失败', variant: 'destructive' })
    } finally {
      setResumingGeneration(false)
    }
  }

  const episodeSummary = episode?.summary?.trim() || '暂无单集摘要，可先从资源提取开始推进当前集制作。'
  const updatedAtLabel = episode?.updated_at
    ? new Date(episode.updated_at).toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
    : '—'

  return (
    <div className="space-y-6">
      <Tabs value={activeTab} onValueChange={(value) => setActiveTab(value as 'assets' | 'storyboard' | 'dubbing' | 'video')} className="w-full">
        <TabsList className="grid h-auto w-full grid-cols-4 gap-1.5 rounded-2xl bg-surface-100 p-1.5">
          {workflowSteps.map((step) => (
            <TabsTrigger
              key={step.key}
              value={step.key}
              className={`group relative flex cursor-pointer flex-col gap-1.5 overflow-hidden rounded-xl border border-surface-200/80 bg-white px-3 py-2.5 text-left transition-all duration-200 hover:-translate-y-px hover:shadow-sm data-[state=active]:shadow-md ${workflowStepTheme[step.key].tab}`}
            >
              {/* 顶部状态色条 - 始终可见 */}
              <span className={`absolute inset-x-0 top-0 h-[3px] rounded-t-xl transition-colors duration-300 ${
                step.status === 'done' ? 'bg-emerald-400' :
                step.status === 'current' ? workflowStepTheme[step.key].currentStrip :
                step.status === 'failed' ? 'bg-red-400' :
                step.status === 'skipped' ? 'bg-surface-150' :
                workflowStepTheme[step.key].pendingStrip
              }`} />

              {/* 第一行：标题 + 状态，两端对齐 */}
              <div className="flex w-full items-center justify-between gap-2">
                <div className="flex min-w-0 items-center gap-2">
                  <div className="relative flex h-6 w-6 shrink-0 items-center justify-center">
                  {step.status === 'current' && (
                    <span className={`absolute inset-0 animate-ping rounded-full opacity-30 ${workflowStepTheme[step.key].pulse}`} />
                  )}
                  <span className={`relative flex h-6 w-6 items-center justify-center rounded-full transition-colors duration-200 ${
                    step.status === 'done' ? 'bg-emerald-100 text-emerald-600' :
                    step.status === 'current' ? workflowStepTheme[step.key].icon :
                    step.status === 'failed' ? 'bg-red-100 text-red-600' :
                    step.status === 'skipped' ? 'bg-surface-100 text-surface-300' :
                    workflowStepTheme[step.key].icon
                  }`}>
                    {step.key === 'assets' && <ImageIcon className="h-3 w-3" />}
                    {step.key === 'storyboard' && <LayoutGrid className="h-3 w-3" />}
                    {step.key === 'dubbing' && <Mic className="h-3 w-3" />}
                    {step.key === 'video' && <Video className="h-3 w-3" />}
                  </span>
                </div>
                  <span className={`truncate text-xs font-semibold text-surface-700 transition-colors duration-200 ${workflowStepTheme[step.key].title}`}>
                    {step.label}
                  </span>
                </div>

                {/* 状态徽章 */}
                <span className={`inline-flex shrink-0 items-center gap-1 rounded-full border px-2 py-0.5 text-[10px] font-semibold transition-colors duration-200 ${step.status === 'current' ? workflowStepTheme[step.key].currentBadge : workflowStatusClass[step.status]}`}>
                  {step.status === 'current' && <span className={`h-1.5 w-1.5 animate-pulse rounded-full ${workflowStepTheme[step.key].currentDot}`} />}
                  {step.status === 'done' && <CheckCircle2 className="h-3 w-3" />}
                  {step.status === 'failed' && <AlertCircle className="h-3 w-3" />}
                  {step.statusLabel || workflowStatusLabel[step.status]}
                </span>
              </div>

              {/* 第二行：说明 + 可点击提示，两端对齐 */}
              <div className="flex w-full items-center justify-between gap-2">
                <span className={`min-w-0 flex-1 truncate text-[11px] text-surface-500 transition-colors duration-200 ${workflowStepTheme[step.key].hint}`}>
                  {step.hint}
                </span>
                <span className={`shrink-0 text-[10px] font-medium group-data-[state=active]:hidden ${workflowStepTheme[step.key].click}`}>点击进入</span>
                <span className="hidden shrink-0 text-[10px] font-medium text-surface-400 group-data-[state=active]:inline">当前</span>
              </div>
            </TabsTrigger>
          ))}
        </TabsList>

        <div className={`relative mt-6 overflow-hidden rounded-3xl border p-5 shadow-sm ${activeContentShellTheme.frame}`}>
          <span className={`absolute inset-x-0 top-0 h-1 ${activeContentShellTheme.strip}`} />
          <div className="grid gap-5 xl:grid-cols-[minmax(0,1.5fr)_360px]">
            <div className="space-y-5">
              <div>
                <div className="flex flex-wrap items-center gap-2 text-xs text-surface-500">
                  <span className={`rounded-full border px-2.5 py-1 ${activeContentShellTheme.metaPill}`}>
                    第 {episode?.episode_number ?? episodeId} 集
                  </span>
                  <span className={`rounded-full border px-2.5 py-1 ${activeContentShellTheme.metaPill}`}>
                    更新时间 {updatedAtLabel}
                  </span>
                  <span className={`rounded-full border px-2.5 py-1 ${activeContentShellTheme.metaPill}`}>
                    关键词 <span className={`font-semibold ${activeContentShellTheme.metaCount}`}>{episode?.keywords?.length ?? 0}</span>
                  </span>
                </div>
                <h3 className="mt-3 text-xl font-semibold text-surface-900">
                  {episode?.title || `第 ${episode?.episode_number ?? episodeId} 集`}
                </h3>
                <p className="mt-2 text-sm leading-6 text-surface-600">{episodeSummary}</p>
              </div>

              <div className={`relative overflow-hidden rounded-2xl border px-4 py-3 ${activeSidebarTheme.card}`}>
                <span className={`absolute inset-x-0 top-0 h-1 ${activeSidebarTheme.cardStrip}`} />
                <div className="mb-3 flex items-center justify-between gap-3">
                  <span className={`inline-flex items-center gap-2 rounded-full border px-2.5 py-1 text-[11px] font-semibold ${pipelineToneBadgeClass}`}>
                    <span className={`h-1.5 w-1.5 rounded-full ${pipelineStatus.tone === 'green' ? 'bg-emerald-500' : pipelineStatus.tone === 'amber' ? 'bg-amber-500' : pipelineStatus.tone === 'violet' ? 'bg-violet-500' : pipelineStatus.tone === 'slate' ? 'bg-surface-400' : 'bg-blue-500'}`} />
                    {pipelineToneLabel}
                  </span>
                  <span className={`text-[11px] font-medium ${activeSidebarTheme.subtle}`}>
                    当前阶段
                  </span>
                </div>
                <div className="flex items-start gap-3">
                  <div className={`mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-full ${activeSidebarTheme.iconWrap}`}>
                  {pipelineStatus.tone === 'green' ? (
                    <CheckCircle2 className={`h-4 w-4 ${activeSidebarTheme.icon}`} />
                  ) : pipelineStatus.tone === 'slate' ? (
                    <Sparkles className={`h-4 w-4 ${activeSidebarTheme.icon}`} />
                  ) : (
                    <Loader2 className={`h-4 w-4 animate-spin ${activeSidebarTheme.icon}`} />
                  )}
                  </div>
                  <div>
                    <p className={`text-sm font-semibold ${activeSidebarTheme.title}`}>{pipelineStatus.title}</p>
                    <p className={`mt-1 text-xs ${activeSidebarTheme.desc}`}>{pipelineStatus.description}</p>
                  </div>
                </div>
              </div>

            </div>

            <div>
              <div className={`rounded-2xl border p-4 ${activeSidebarTheme.panel}`}>
                <div className="mb-4 flex items-start justify-between gap-3">
                  <div>
                    <p className={`text-sm font-semibold ${activeSidebarTheme.panelTitle}`}>当前阶段操作</p>
                    <p className={`mt-1 text-xs ${activeSidebarTheme.panelDesc}`}>
                      {activeTab === 'assets'
                        ? '继续补齐本集角色、场景和道具资源。'
                        : activeTab === 'storyboard'
                          ? `继续推进镜头拆分与${storyboardImageLabel}生成。`
                          : activeTab === 'dubbing'
                            ? '当前阶段请在下方工作台处理中配音与字幕。'
                            : '当前阶段请在下方工作台继续合成视频成片。'}
                    </p>
                  </div>
                  <span className={`rounded-full px-2 py-1 text-[10px] font-semibold ${activeWorkflowTheme.currentBadge}`}>
                    {workflowSteps.find((step) => step.key === activeTab)?.label}
                  </span>
                </div>
              {activeTab === 'assets' && (
                <div className="flex flex-col gap-2">
                  <Button
                    size="sm"
                    onClick={() => setGenerateTrigger((t) => t + 1)}
                    disabled={resourceButtonDisabled}
                    className={`w-full ${activeSidebarTheme.primaryButton}`}
                  >
                    {resourceButtonDisabled ? <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" /> : <Sparkles className="mr-1.5 h-3.5 w-3.5" />}
                    一键生成全部
                  </Button>
                  <Button size="sm" variant="outline" onClick={handleAutoMatchVoices} disabled={autoMatchingVoices} className={`w-full ${activeSidebarTheme.secondaryButton}`}>
                    {autoMatchingVoices ? <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" /> : <Mic className="mr-1.5 h-3.5 w-3.5" />}
                    自动匹配音色
                  </Button>
                  {assetStats.paused > 0 ? (
                    <Button size="sm" variant="outline" onClick={handleResumeGeneration} disabled={resumingGeneration} className={`w-full ${activeSidebarTheme.secondaryButton}`}>
                      {resumingGeneration ? <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" /> : <Play className="mr-1.5 h-3.5 w-3.5" />}
                      继续生成 ({assetStats.paused})
                    </Button>
                  ) : assetStats.generating > 0 ? (
                    <Button size="sm" variant="outline" onClick={handlePauseGeneration} disabled={pausingGeneration} className={`w-full ${activeSidebarTheme.secondaryButton}`}>
                      {pausingGeneration ? <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" /> : <Pause className="mr-1.5 h-3.5 w-3.5" />}
                      暂停生成
                    </Button>
                  ) : null}
                  <Button size="sm" variant="outline" onClick={() => setRegenerateTrigger((t) => t + 1)} className={`w-full ${activeSidebarTheme.secondaryButton}`}>
                    <RotateCcw className="mr-1.5 h-3.5 w-3.5" />
                    当前集重新生成
                  </Button>
                </div>
              )}
              {activeTab === 'storyboard' && (
                <div className="flex flex-col gap-2">
                  <Button
                    size="sm"
                    onClick={() => setSbGenerateTrigger((t) => t + 1)}
                    disabled={storyboardButtonDisabled}
                    className={`w-full ${activeSidebarTheme.primaryButton}`}
                  >
                    {storyboardButtonDisabled ? <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" /> : <Sparkles className="mr-1.5 h-3.5 w-3.5" />}
                    生成本集{storyboardImageLabel}
                  </Button>
                  <Button size="sm" variant="outline" onClick={() => setSbAuditTrigger((t) => t + 1)} className={`w-full ${activeSidebarTheme.secondaryButton}`}>
                    <Bot className="mr-1.5 h-3.5 w-3.5" />
                    AI 补全缺失信息
                  </Button>
                  {storyboardStats.paused > 0 ? (
                    <Button size="sm" variant="outline" onClick={() => setSbResumeTrigger((t) => t + 1)} className={`w-full ${activeSidebarTheme.secondaryButton}`}>
                      <Play className="mr-1.5 h-3.5 w-3.5" />
                      继续生成 ({storyboardStats.paused})
                    </Button>
                  ) : storyboardStats.generating > 0 ? (
                    <Button size="sm" variant="outline" onClick={() => setSbPauseTrigger((t) => t + 1)} className={`w-full ${activeSidebarTheme.secondaryButton}`}>
                      <Pause className="mr-1.5 h-3.5 w-3.5" />
                      暂停生成
                    </Button>
                  ) : null}
                  <Button size="sm" variant="outline" onClick={() => setSbRegenerateTrigger((t) => t + 1)} className={`w-full ${activeSidebarTheme.secondaryButton}`}>
                    <RotateCcw className="mr-1.5 h-3.5 w-3.5" />
                    重新生成本集
                  </Button>
                </div>
              )}
              {(activeTab === 'dubbing' || activeTab === 'video') && (
                <div className={`rounded-xl border border-dashed px-3 py-4 text-sm ${activeSidebarTheme.secondaryButton}`}>
                  当前阶段的详细操作已放在下方工作台中，进入对应内容区即可继续处理。
                </div>
              )}
              </div>
            </div>
          </div>
        </div>

        <div className="mt-6">
          <TabsContent value="assets" className="mt-0">
            <div className={`rounded-3xl border p-3 ${contentShellTheme.assets.contentWrap}`}>
              <AssetsTab
                projectId={projectId}
                project={project}
                episodeId={episodeId}
                onExtractEpisodeAssets={handleExtractAssets}
                isExtractingEpisodeAssets={resourceButtonDisabled}
                generateTrigger={generateTrigger}
                regenerateTrigger={regenerateTrigger}
                hideActionBar
              />
            </div>
          </TabsContent>
          <TabsContent value="storyboard" className="mt-0">
            <div className={`rounded-3xl border p-3 ${contentShellTheme.storyboard.contentWrap}`}>
              <StoryboardTab
                projectId={projectId}
                project={project}
                episodeId={episodeId}
                onExtractStoryboards={handleExtractStoryboards}
                isExtractingStoryboards={isExtractingStoryboards || episode?.status === 'scene_splitting'}
                awaitingAutoStoryboard={awaitingAutoStoryboard}
                storyboardButtonDisabled={storyboardButtonDisabled}
                hideActionBar
                sbGenerateTrigger={sbGenerateTrigger}
                sbRegenerateTrigger={sbRegenerateTrigger}
                sbPauseTrigger={sbPauseTrigger}
                sbResumeTrigger={sbResumeTrigger}
                sbAuditTrigger={sbAuditTrigger}
              />
            </div>
          </TabsContent>
          <TabsContent value="dubbing" className="mt-0">
            <div className={`rounded-3xl border p-3 ${contentShellTheme.dubbing.contentWrap}`}>
              <DubbingTab projectId={projectId} project={project} mutateProject={() => {}} episodeId={episodeId} />
            </div>
          </TabsContent>
          <TabsContent value="video" className="mt-0">
            <div className={`rounded-3xl border p-3 ${contentShellTheme.video.contentWrap}`}>
              <VideoTab projectId={projectId} project={project} episodeId={episodeId} />
            </div>
          </TabsContent>
        </div>
      </Tabs>
    </div>
  )
}

