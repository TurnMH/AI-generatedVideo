import type { VideoProductionMode } from '@/lib/projects/new/production-mode'
import { AUTO_EPISODE_SPLIT_HINT } from '@/lib/projects/episode-split'

export const VIDEO_CREATE_STEP_COUNT = 2

export const VIDEO_CREATE_STEPS = ['写什么内容', '确认并创建'] as const

export type QuickGenreOption = {
  presetKey: string
  label: string
  hint: string
  /** 选中后会影响的后续环节，用用户能看懂的说法 */
  effect: string
}

export const GENRE_PICKER_INTRO = {
  title: '题材风格（选填）',
  desc: '决定成片「长什么样、怎么讲」，不是决定分几集。上传剧本后会按内容自动分集。',
  selectedTitle: '选中后会影响这些环节',
  effects: ['画面画风与镜头气质', '横屏 / 竖屏与每段时长', '分镜拆分与旁白节奏', '角色稳定度与配音字幕'],
} as const

export const SCRIPT_AUTO_SPLIT_NOTE = AUTO_EPISODE_SPLIT_HINT

export const VIDEO_QUICK_GENRES: Record<VideoProductionMode, QuickGenreOption[]> = {
  script_drama: [
    {
      presetKey: 'myth-guofeng',
      label: '国风神话',
      hint: '西游、仙侠、史诗',
      effect: '水墨国风、电影感横屏、偏史诗运镜',
    },
    {
      presetKey: 'xianxia-fantasy',
      label: '仙侠玄幻',
      hint: '修仙、宗门、秘境',
      effect: '东方奇幻画风、宏大场景、剧情向分镜',
    },
    {
      presetKey: 'urban-romance',
      label: '都市情感',
      hint: '恋爱、夜景、人物戏',
      effect: '现代写实、竖屏友好、情绪特写偏多',
    },
    {
      presetKey: 'suspense-investigation',
      label: '悬疑推理',
      hint: '案件、反转、压迫感',
      effect: '暗调氛围、悬念节奏、镜头偏紧',
    },
    {
      presetKey: 'short-drama-reversal',
      label: '爽剧反转',
      hint: '打脸、逆袭、高能',
      effect: '快节奏剪辑感、冲突镜头多、对白密集',
    },
    {
      presetKey: 'palace-intrigue',
      label: '古装宫斗',
      hint: '权谋、后宫、礼仪',
      effect: '华美服化、室内调度、人物关系戏',
    },
    {
      presetKey: 'campus-youth',
      label: '校园青春',
      hint: '成长、友情、初恋',
      effect: '明亮清新、生活化场景、轻快运镜',
    },
    {
      presetKey: 'anime-action',
      label: '热血动漫',
      hint: '战斗、冒险、少年',
      effect: '漫画动感、动作分镜多、节奏更快',
    },
  ],
  commentary_comic: [
    {
      presetKey: 'explainer-comic',
      label: '漫画解说',
      hint: '剧情盘点、角色解析',
      effect: '竖屏解说漫、旁白细拆、漫画风角色演出',
    },
    {
      presetKey: 'documentary',
      label: '知识讲解',
      hint: '设定介绍、世界观',
      effect: '讲解感旁白、信息点清晰、画面偏说明性',
    },
    {
      presetKey: 'myth-guofeng',
      label: '神话盘点',
      hint: '西游、封神、神仙谱',
      effect: '国风漫画解说、史诗旁白、角色关系梳理',
    },
    {
      presetKey: 'xianxia-fantasy',
      label: '仙侠解说',
      hint: '修仙体系、宗门势力',
      effect: '玄幻设定讲解、境界/法宝盘点',
    },
    {
      presetKey: 'historical-epic',
      label: '历史科普',
      hint: '朝代、人物、大事件',
      effect: '纪实讲解感、时间线叙述、资料型画面',
    },
    {
      presetKey: 'anime-action',
      label: '热血番解说',
      hint: '战斗名场面、战力排行',
      effect: '动漫风、高能段落强调、节奏更快',
    },
    {
      presetKey: 'horror-thriller',
      label: '悬疑解说',
      hint: '案件复盘、细思极恐',
      effect: '暗色氛围、悬念旁白、压迫感镜头',
    },
    {
      presetKey: 'female-romance',
      label: '情感解说',
      hint: 'CP 梳理、虐甜剧情',
      effect: '人物情绪特写、关系线旁白、偏竖屏',
    },
    {
      presetKey: 'food-lifestyle',
      label: '生活盘点',
      hint: '日常、好物、轻松讲',
      effect: '轻快旁白、生活场景、节奏舒缓',
    },
    {
      presetKey: 'hard-sci-fi',
      label: '科幻解说',
      hint: '世界观、设定、时间线',
      effect: '未来感画面、设定拆解、逻辑型旁白',
    },
  ],
}

export const FRIENDLY_ASPECT_OPTIONS = [
  { value: '16:9', label: '横屏', hint: '电影感、剧情向' },
  { value: '9:16', label: '竖屏', hint: '短视频、解说漫' },
  { value: '1:1', label: '方形', hint: '社交封面' },
] as const

export const FRIENDLY_DURATION_OPTIONS = [4, 5, 6] as const

export const FRIENDLY_CONSISTENCY_OPTIONS = [
  { value: 70, label: '灵活' },
  { value: 82, label: '均衡' },
  { value: 92, label: '稳定' },
] as const

export function resolutionForAspect(aspect: string): string {
  switch (aspect) {
    case '9:16':
      return '1080x1920'
    case '1:1':
      return '1024x1024'
    case '4:3':
      return '1280x960'
    case '3:4':
      return '1024x1536'
    default:
      return '1920x1080'
  }
}

export function nearestConsistencyPreset(value: number): number {
  return FRIENDLY_CONSISTENCY_OPTIONS.reduce((best, item) =>
    Math.abs(item.value - value) < Math.abs(best - value) ? item.value : best,
  FRIENDLY_CONSISTENCY_OPTIONS[1].value)
}
