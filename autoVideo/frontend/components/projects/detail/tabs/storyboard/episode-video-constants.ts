export const VIDEO_STYLE_LABELS: Record<string, string> = {
  'anime-2d': '2维动漫',
  'anime-3d': '3维动漫',
  'live-action-film': '真人电影',
  'live-action-short': '真人短剧',
}

export const VIDEO_MOTION_LABELS = {
  gentle: '柔和',
  dynamic: '动感',
  cinematic: '电影感',
} as const

export const VIDEO_FRAME_SIZE_OPTIONS = [
  { key: 'portrait-9-16', label: '竖屏 9:16', desc: '适合短视频、手机全屏与人物主体。' },
  { key: 'landscape-16-9', label: '横屏 16:9', desc: '适合剧情镜头、横版预告与通用视频。' },
  { key: 'square-1-1', label: '方形 1:1', desc: '适合封面感画面与居中构图。' },
  { key: 'ultrawide-21-9', label: '宽银幕 21:9', desc: '适合史诗场景与电影化大场面。' },
] as const

export const VIDEO_SUBJECT_SIZE_OPTIONS = [
  { key: 'close-up', label: '特写 / 大主体', desc: '人物或主体更大，强调表情与细节。' },
  { key: 'medium-shot', label: '中景 / 平衡', desc: '主体与环境平衡，更适合常规叙事。' },
  { key: 'wide-shot', label: '远景 / 大场景', desc: '主体相对更小，突出场景与空间关系。' },
] as const

export const VIDEO_CLARITY_OPTIONS = [
  { key: 'standard', label: '标准清晰', desc: '细节自然，适合常规生成。' },
  { key: 'high', label: '高清细节', desc: '更清楚的边缘与材质层次。' },
  { key: 'ultra', label: '超清锐利', desc: '尽量强化精细纹理和清晰度。' },
] as const

export type VideoFrameSizeKey = (typeof VIDEO_FRAME_SIZE_OPTIONS)[number]['key']
export type VideoSubjectSizeKey = (typeof VIDEO_SUBJECT_SIZE_OPTIONS)[number]['key']
export type VideoClarityKey = (typeof VIDEO_CLARITY_OPTIONS)[number]['key']
export type VideoMotionKey = keyof typeof VIDEO_MOTION_LABELS
