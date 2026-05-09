import type { Project } from '@/types'

export type ProjectMediaKind = 'video' | 'video_serial' | 'comics' | 'music' | 'image'

export const PROJECT_MEDIA_TAGS: Record<ProjectMediaKind, string> = {
  video: 'media:video',
  video_serial: 'media:video_serial',
  comics: 'media:comics',
  music: 'media:music',
  image: 'media:image',
}

const INTERNAL_MEDIA_TAG_SET = new Set<string>(Object.values(PROJECT_MEDIA_TAGS))

export const PROJECT_MEDIA_META: Record<
  ProjectMediaKind,
  {
    label: string
    listTitle: string
    countLabel: string
    listHref: string
    createHref: string
    createLabel: string
    createTitle: string
    createDescription: string
    emptyTitle: string
    emptyDescription: string
    selectLabel: string
    selectPlaceholder: string
  }
> = {
  video: {
    label: '视频',
    listTitle: '我的视频',
    countLabel: '个视频项目',
    listHref: '/projects',
    createHref: '/projects/new?media=video',
    createLabel: '新建视频项目',
    createTitle: '新建视频项目',
    createDescription: '创建一个新的 AI 视频项目',
    emptyTitle: '还没有视频项目',
    emptyDescription: '点击「新建视频项目」开始创建您的第一个 AI 视频项目',
    selectLabel: '选择视频项目',
    selectPlaceholder: '请选择视频项目',
  },
  video_serial: {
    label: '视频（串行）',
    listTitle: '我的串行视频',
    countLabel: '个串行视频项目',
    listHref: '/video-serial',
    createHref: '/video-serial/new',
    createLabel: '新建串行视频项目',
    createTitle: '新建串行视频项目',
    createDescription: '创建支持场景内串行生成、末帧约束的 AI 视频项目',
    emptyTitle: '还没有串行视频项目',
    emptyDescription: '点击「新建串行视频项目」开始创建多场景连贯视频',
    selectLabel: '选择串行视频项目',
    selectPlaceholder: '请选择串行视频项目',
  },
  comics: {
    label: '漫画',
    listTitle: '漫画项目列表',
    countLabel: '个漫画项目',
    listHref: '/comics',
    createHref: '/projects/new?media=comics',
    createLabel: '新建漫画项目',
    createTitle: '新建漫画项目',
    createDescription: '创建一个用于漫画分镜与漫画生成的独立项目',
    emptyTitle: '还没有可用于漫画生成的项目',
    emptyDescription: '先创建一个漫画项目，再来这里拆分画格和生成漫画。',
    selectLabel: '选择漫画项目',
    selectPlaceholder: '请选择漫画项目',
  },
  music: {
    label: '音乐',
    listTitle: '音乐项目列表',
    countLabel: '个音乐项目',
    listHref: '/music',
    createHref: '/projects/new?media=music',
    createLabel: '新建音乐项目',
    createTitle: '新建音乐项目',
    createDescription: '创建一个用于 AI 配乐与主题曲生成的独立项目',
    emptyTitle: '还没有音乐项目',
    emptyDescription: '先创建一个音乐项目，再来这里生成 BGM 或主题曲。',
    selectLabel: '选择音乐项目',
    selectPlaceholder: '请选择音乐项目',
  },
  image: {
    label: '图片',
    listTitle: '图片项目列表',
    countLabel: '个图片项目',
    listHref: '/images',
    createHref: '/projects/new?media=image',
    createLabel: '新建图片项目',
    createTitle: '新建图片项目',
    createDescription: '创建一个用于图片优化、改图和动图生成的独立项目',
    emptyTitle: '还没有图片项目',
    emptyDescription: '先创建一个图片项目，再来这里做图片优化、改图和生成动图。',
    selectLabel: '选择图片项目',
    selectPlaceholder: '请选择图片项目',
  },
}

export function normalizeProjectMediaKind(raw?: string | null): ProjectMediaKind {
  if (raw === 'comics' || raw === 'music' || raw === 'image' || raw === 'video' || raw === 'video_serial') {
    return raw
  }
  return 'video'
}

export function ensureProjectMediaTag(styleTags: string[] = [], media: ProjectMediaKind): string[] {
  const visibleTags = styleTags.filter((tag) => !INTERNAL_MEDIA_TAG_SET.has(tag))
  return [...visibleTags, PROJECT_MEDIA_TAGS[media]]
}

export function stripProjectMediaTags(styleTags: string[] = []): string[] {
  return styleTags.filter((tag) => !INTERNAL_MEDIA_TAG_SET.has(tag))
}

export function getProjectMediaKind(project: Pick<Project, 'style_tags' | 'project_type'> | { style_tags?: string[]; project_type?: string }): ProjectMediaKind {
  if (project.project_type === 'comics' || project.project_type === 'music' || project.project_type === 'image' || project.project_type === 'video' || project.project_type === 'video_serial') {
    return project.project_type
  }
  const tags = project.style_tags ?? []
  if (tags.includes(PROJECT_MEDIA_TAGS.comics)) return 'comics'
  if (tags.includes(PROJECT_MEDIA_TAGS.music)) return 'music'
  if (tags.includes(PROJECT_MEDIA_TAGS.image)) return 'image'
  return 'video'
}

export function matchesProjectMedia(project: Pick<Project, 'style_tags' | 'project_type'> | { style_tags?: string[]; project_type?: string }, media: ProjectMediaKind): boolean {
  return getProjectMediaKind(project) === media
}
