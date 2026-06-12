export type VideoProductionMode = 'script_drama' | 'commentary_comic'

export type VideoProductionModeOption = {
  key: VideoProductionMode
  label: string
  shortLabel: string
  desc: string
  pipeline: string[]
  scriptHint: string
  targetEpisodesHint: string
  storyboardHint: string
  defaultTargetEpisodes: number
  requiredStyleTag?: string
}

export const VIDEO_PRODUCTION_MODES: VideoProductionModeOption[] = [
  {
    key: 'script_drama',
    label: '剧本叙事视频',
    shortLabel: '剧情短剧',
    desc: '有完整故事、对白和场景，适合连续剧或剧情短片。',
    pipeline: ['上传剧本', '剧情分集', '剧本分镜', '分镜出图', '批量成片'],
    scriptHint: '上传完整剧本或小说章节，系统会按剧情起承转合自动分集。',
    targetEpisodesHint: '上传剧本后按章节或剧情结构自动分集，无需手动填写。',
    storyboardHint: '分镜按场景、动作链和对白回合拆分，适合连续叙事。',
    defaultTargetEpisodes: 10,
  },
  {
    key: 'commentary_comic',
    label: '解说漫视频',
    shortLabel: '解说漫',
    desc: '以旁白讲解为主，适合剧情盘点、角色解析、竖屏短视频。',
    pipeline: ['上传解说稿', '旁白分集', '解说分镜', '漫画风出图', '旁白成片'],
    scriptHint: '上传解说稿、盘点稿或带旁白的讲解脚本，系统会按旁白信息点拆分。',
    targetEpisodesHint: '上传解说稿后按旁白信息点自动分集，无需手动填写。',
    storyboardHint: '分镜按旁白句群与剧情节点拆分，默认竖屏 9:16。',
    defaultTargetEpisodes: 10,
    requiredStyleTag: '解说漫',
  },
]

export const AD_VIDEO_CREATE_HREF = '/ad-video'

export function getProductionModeOption(mode: VideoProductionMode): VideoProductionModeOption {
  return VIDEO_PRODUCTION_MODES.find((item) => item.key === mode) ?? VIDEO_PRODUCTION_MODES[0]
}

export function isCommentaryProductionMode(mode: VideoProductionMode): boolean {
  return mode === 'commentary_comic'
}

/** Preset keys primarily suited for each production mode. */
export const PRESET_KEYS_BY_PRODUCTION_MODE: Record<VideoProductionMode, string[] | null> = {
  script_drama: null,
  commentary_comic: [
    'explainer-comic',
    'documentary',
    'myth-guofeng',
    'xianxia-fantasy',
    'historical-epic',
    'anime-action',
    'horror-thriller',
    'female-romance',
    'food-lifestyle',
    'hard-sci-fi',
  ],
}

export function filterPresetsForProductionMode<T extends { key: string }>(
  presets: T[],
  mode: VideoProductionMode,
): T[] {
  const allowlist = PRESET_KEYS_BY_PRODUCTION_MODE[mode]
  if (!allowlist) {
    return presets.filter((preset) => preset.key !== 'explainer-comic')
  }
  const allowed = new Set(allowlist)
  return presets.filter((preset) => allowed.has(preset.key))
}

export function ensureProductionStyleTags(
  styleTags: string[],
  mode: VideoProductionMode,
): string[] {
  const cleaned = styleTags.filter((tag) => tag !== '解说漫')
  if (mode === 'commentary_comic') {
    return [...cleaned, '解说漫']
  }
  return cleaned
}

export function resolveVideoProductionModeLabel(project: {
  style_tags?: string[]
  storyboard_config?: { production_mode?: VideoProductionMode }
}): string | null {
  const configured = project.storyboard_config?.production_mode
  if (configured === 'commentary_comic' || configured === 'script_drama') {
    return getProductionModeOption(configured).shortLabel
  }
  if ((project.style_tags ?? []).includes('解说漫')) {
    return '解说漫'
  }
  return '剧情短剧'
}
