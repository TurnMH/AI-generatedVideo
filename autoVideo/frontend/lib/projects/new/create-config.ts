import type { ProjectMediaKind } from '@/lib/project-media'
import { VIDEO_STYLE_PRESETS, VIDEO_MOTION_OPTIONS } from '@/lib/video-style-config'

export const STYLE_OPTIONS = [
  '漫画', '写实', '水彩', '赛博朋克', '古风', '油画',
  '像素风', '扁平插画', '3D渲染', '动漫', '水墨', '极简',
  '国潮', '仙侠', '武侠', '宫廷', '民国', '悬疑',
  '惊悚', '治愈', '青春', '都市夜景', '科幻', '末日废土',
  '蒸汽朋克', '奇幻', '纪录片', '广告感', '复古胶片', '童话',
  '萌宠', '史诗', '黑色电影', '日系', '韩系', '欧美电影感',
  '女性向', '商战', '职场', '历史正剧', '战争', '志怪',
  '探险', '公路片', '美食', '旅行', '体育', '机甲',
  '恐怖', '暗黑奇幻', '疗愈系', '手账插画', '儿童动画', '舞台剧感',
  '英伦', '美剧感', '西部片', '黑帮', '超级英雄', '吸血鬼',
  '狼人', '魔法学院', '哥特', '废墟都市', '北欧冷调', '洛丽塔暗黑',
]

export const ASPECT_RATIOS = ['16:9', '9:16', '4:3', '3:4', '1:1']
export const RESOLUTIONS = ['1920x1080', '1080x1920', '1280x720', '720x1280', '1024x1024']
export const WATERMARK_POSITIONS = [
  { value: 'top-left', label: '左上' },
  { value: 'top-right', label: '右上' },
  { value: 'bottom-left', label: '左下' },
  { value: 'bottom-right', label: '右下' },
  { value: 'center', label: '居中' },
] as const

export const VIDEO_STYLE_CREATION_SPOTLIGHT_KEYS = new Set<string>([
  'anime-2d',
  'anime-3d',
  'live-action-film',
  'live-action-short',
])
export const VIDEO_STYLE_PRESET_LABELS: Record<string, string> = Object.fromEntries(
  VIDEO_STYLE_PRESETS.map((style) => [style.key, style.label])
)
export const VIDEO_MOTION_MODE_LABELS: Record<string, string> = Object.fromEntries(
  VIDEO_MOTION_OPTIONS.map((mode) => [mode.key, mode.label])
)
export const VIDEO_RUNTIME_KEY_LABELS: Record<string, string> = {
  'comfyui-video': 'ComfyUI 本地视频',
  wan: '通义万象',
  'hubagi-TC-GV': 'Hubagi TC-GV',
  'hubagi-voe3.1': 'Veo 3.1',
  sora2: 'Sora-2',
}

export type CreatePageDisplayConfig = {
  stepLabels: [string, string, string]
  stepDescriptions: [string, string, string]
  flowHint: string
  afterCreateTitle: string
  afterCreateHint: string
  titlePlaceholder: string
  descriptionPlaceholder: string
  targetEpisodesLabel: string
  targetEpisodesHint?: string
  scriptLabel: string
  scriptUploadTitle: string
  scriptUploadHint: string
  storyboardSectionTitle: string
  consistencyLabel: string
  consistencyHint: string
  styleTagHint: string
  showPresetTemplates: boolean
  showTargetEpisodes: boolean
  showVideoMode: boolean
  showVideoStyle: boolean
  showVideoMotion: boolean
  showDubSubtitle: boolean
  showWatermark: boolean
  showStoryboardConfig: boolean
  showStoryboardDuration: boolean
  showConsistency: boolean
  showCostEstimation: boolean
  modelSelectorKeys: Array<'text' | 'image' | 'video' | 'tts'>
}

export const CREATE_PAGE_DISPLAY: Record<ProjectMediaKind, CreatePageDisplayConfig> = {
  video: {
    stepLabels: ['基本信息', '模型与功能', '剧本与配置'],
    stepDescriptions: ['完善基础设定', '选择模型与能力', '确认脚本与分镜配置'],
    flowHint: '信息、模型、剧本配置',
    afterCreateTitle: '进入视频项目',
    afterCreateHint: '继续生成分集、分镜、配音和视频',
    titlePlaceholder: '例如：西游记热血预告片',
    descriptionPlaceholder: '描述题材、节奏、画风和目标产出，例如国风神话、强节奏、适合短视频连载。',
    targetEpisodesLabel: '目标剧本分集数',
    targetEpisodesHint: '用于后续自动分集、分镜和视频生成估算。',
    scriptLabel: '上传剧本（可选）',
    scriptUploadTitle: '点击上传剧本文件',
    scriptUploadHint: '支持 .txt、.md、.docx 格式',
    storyboardSectionTitle: '分镜默认设置',
    consistencyLabel: '角色与素材一致性',
    consistencyHint: '数值越高越强调角色/场景的一致外观，数值越低越允许画面更灵活变化。',
    styleTagHint: '建议优先标注题材、画风和镜头气质，便于后续分镜与视频风格统一。',
    showPresetTemplates: true,
    showTargetEpisodes: true,
    showVideoMode: true,
    showVideoStyle: true,
    showVideoMotion: true,
    showDubSubtitle: true,
    showWatermark: true,
    showStoryboardConfig: true,
    showStoryboardDuration: true,
    showConsistency: true,
    showCostEstimation: true,
    modelSelectorKeys: ['text', 'image', 'video', 'tts'],
  },
  comics: {
    stepLabels: ['基本信息', '模型与画格', '剧本与漫画配置'],
    stepDescriptions: ['完善漫画项目设定', '选择拆稿与出图模型', '确认剧本分集与漫画画格设置'],
    flowHint: '项目信息、模型、漫画配置',
    afterCreateTitle: '进入漫画工作台',
    afterCreateHint: '继续做剧本分集、漫画分镜和漫画生成',
    titlePlaceholder: '例如：西游记国风条漫',
    descriptionPlaceholder: '描述条漫题材、角色风格和阅读气质，例如东方神话、长卷分镜、角色表情明确。',
    targetEpisodesLabel: '目标剧本分集数',
    targetEpisodesHint: '漫画项目会复用这些剧本分集来拆成漫画画格。',
    scriptLabel: '上传漫画脚本 / 小说（可选）',
    scriptUploadTitle: '点击上传漫画脚本或小说',
    scriptUploadHint: '支持 .txt、.md、.docx 格式',
    storyboardSectionTitle: '漫画画格默认设置',
    consistencyLabel: '角色与画面一致性',
    consistencyHint: '用于稳定漫画角色和场景外观，减少跨画格漂移。',
    styleTagHint: '建议优先标注漫画画风、题材和角色气质，便于后续拆格和画面统一。',
    showPresetTemplates: false,
    showTargetEpisodes: true,
    showVideoMode: false,
    showVideoStyle: false,
    showVideoMotion: false,
    showDubSubtitle: false,
    showWatermark: false,
    showStoryboardConfig: true,
    showStoryboardDuration: true,
    showConsistency: true,
    showCostEstimation: false,
    modelSelectorKeys: ['text', 'image'],
  },
  music: {
    stepLabels: ['基本信息', '模型与创作能力', '歌词与素材'],
    stepDescriptions: ['完善音乐项目设定', '选择文案与声音模型', '确认歌词、灵感文案与创作素材'],
    flowHint: '信息、模型、歌词素材',
    afterCreateTitle: '进入音乐项目',
    afterCreateHint: '继续生成主题曲、BGM 和音频素材',
    titlePlaceholder: '例如：西游主题战斗配乐',
    descriptionPlaceholder: '描述音乐用途、氛围和受众，例如热血战斗 BGM、东方史诗、片尾主题歌。',
    targetEpisodesLabel: '目标段落数',
    scriptLabel: '上传歌词 / 文案（可选）',
    scriptUploadTitle: '点击上传歌词或创作文案',
    scriptUploadHint: '支持 .txt、.md、.docx 格式',
    storyboardSectionTitle: '默认生成设置',
    consistencyLabel: '内容一致性',
    consistencyHint: '用于保持歌词主题、角色主题或音乐概念的一致性。',
    styleTagHint: '建议优先标注音乐用途、情绪和风格，例如国风、热血、片尾、史诗。',
    showPresetTemplates: false,
    showTargetEpisodes: false,
    showVideoMode: false,
    showVideoStyle: false,
    showVideoMotion: false,
    showDubSubtitle: false,
    showWatermark: false,
    showStoryboardConfig: false,
    showStoryboardDuration: false,
    showConsistency: false,
    showCostEstimation: false,
    modelSelectorKeys: ['text', 'tts'],
  },
  image: {
    stepLabels: ['基本信息', '模型与生成能力', '文案与画面偏好'],
    stepDescriptions: ['完善图片项目设定', '选择文案和出图模型', '确认提示文案与默认画面参数'],
    flowHint: '信息、模型、画面偏好',
    afterCreateTitle: '进入图片项目',
    afterCreateHint: '继续做资源图生成、改图和图片工作流',
    titlePlaceholder: '例如：西游主视觉海报概念图',
    descriptionPlaceholder: '描述图片用途、主体和质感，例如角色海报、封面主视觉、概念设定图。',
    targetEpisodesLabel: '目标批次数',
    scriptLabel: '上传提示文案 / 参考描述（可选）',
    scriptUploadTitle: '点击上传提示文案或参考描述',
    scriptUploadHint: '支持 .txt、.md、.docx 格式',
    storyboardSectionTitle: '默认画面设置',
    consistencyLabel: '主体与风格一致性',
    consistencyHint: '用于稳定人物、道具或场景在多次出图中的风格表现。',
    styleTagHint: '建议优先标注画风、主体和用途，例如海报、国风、人物设定、写实。',
    showPresetTemplates: false,
    showTargetEpisodes: false,
    showVideoMode: false,
    showVideoStyle: false,
    showVideoMotion: false,
    showDubSubtitle: false,
    showWatermark: false,
    showStoryboardConfig: true,
    showStoryboardDuration: false,
    showConsistency: true,
    showCostEstimation: false,
    modelSelectorKeys: ['text', 'image'],
  },
  video_serial: {
    stepLabels: ['基本信息', '模型与生成能力', '文案与画面偏好'],
    stepDescriptions: ['完善串行视频项目设定', '选择文案和视频模型', '确认提示文案与默认画面参数'],
    flowHint: '信息、模型、画面偏好',
    afterCreateTitle: '进入串行视频项目',
    afterCreateHint: '配置角色组、分镜场景，开始串行视频生成',
    titlePlaceholder: '例如：连续场景短视频系列',
    descriptionPlaceholder: '描述视频主题、角色和场景，例如同一角色在不同场景下的连续动作视频。',
    targetEpisodesLabel: '目标集数',
    scriptLabel: '上传剧本 / 分镜脚本（可选）',
    scriptUploadTitle: '点击上传剧本或分镜脚本',
    scriptUploadHint: '支持 .txt、.md、.docx 格式',
    storyboardSectionTitle: '默认分镜设置',
    consistencyLabel: '角色与场景一致性',
    consistencyHint: '串行视频需要较高一致性，建议设置 85 以上以保证角色跨帧连贯。',
    styleTagHint: '建议标注视频风格、主体动作和场景类型，例如写实、动漫、室内、追逐。',
    showPresetTemplates: true,
    showTargetEpisodes: true,
    showVideoMode: true,
    showVideoStyle: true,
    showVideoMotion: true,
    showDubSubtitle: true,
    showWatermark: true,
    showStoryboardConfig: true,
    showStoryboardDuration: true,
    showConsistency: true,
    showCostEstimation: true,
    modelSelectorKeys: ['text', 'image', 'video', 'tts'],
  },
}
