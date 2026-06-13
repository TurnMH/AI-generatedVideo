import type { WorkflowStepKey } from '@/lib/projects/episode-workspace-workflow-steps'

export type WorkflowStepTheme = {
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
}

export type SidebarTheme = {
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
}

export type ContentShellTheme = {
  frame: string
  strip: string
  metaPill: string
  metaCount: string
  contentWrap: string
}

export const workflowStepTheme: Record<WorkflowStepKey, WorkflowStepTheme> = {
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

export const sidebarTheme: Record<WorkflowStepKey, SidebarTheme> = {
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

export const contentShellTheme: Record<WorkflowStepKey, ContentShellTheme> = {
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
