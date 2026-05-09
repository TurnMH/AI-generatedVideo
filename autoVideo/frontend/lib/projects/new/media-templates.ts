import type { ProjectMediaKind } from '@/lib/project-media'

export type MediaStarterTemplate = {
  key: string
  label: string
  desc: string
  title: string
  description: string
  styleTags: string[]
  targetEpisodes?: number
  storyboardAspectRatio?: string
  storyboardResolution?: string
  consistencyStrength?: number
}

export const MEDIA_STARTER_TEMPLATES: Record<ProjectMediaKind, MediaStarterTemplate[]> = {
  video: [
    {
      key: 'video-explainer-comic',
      label: '解说漫视频',
      desc: '适合漫画风旁白讲解、剧情盘点和竖屏内容输出。',
      title: '西游记人物剧情解说漫',
      description: '面向竖屏解说视频的漫画风项目，强调旁白驱动、字幕清晰、角色表演稳定和连续讲述节奏。',
      styleTags: ['解说漫', '漫画', '动漫', '竖屏', '旁白'],
      targetEpisodes: 10,
      storyboardAspectRatio: '9:16',
      storyboardResolution: '1080x1920',
      consistencyStrength: 84,
    },
  ],
  video_serial: [],
  comics: [
    {
      key: 'comics-guofeng',
      label: '国风条漫',
      desc: '适合神话、仙侠、东方题材的连续漫画项目。',
      title: '西游记国风条漫',
      description: '面向条漫阅读的东方神话漫画项目，强调角色辨识、长卷叙事和画格连续性。',
      styleTags: ['国风', '漫画', '仙侠', '长卷叙事'],
      targetEpisodes: 12,
      storyboardAspectRatio: '3:4',
      storyboardResolution: '1024x1536',
      consistencyStrength: 88,
    },
    {
      key: 'comics-action',
      label: '热血战斗漫画',
      desc: '适合高能打斗、速度感强的漫画镜头。',
      title: '热血战斗漫画企划',
      description: '聚焦动作桥段和角色演出，适合战斗追逐、冲突升级与漫画分镜表达。',
      styleTags: ['漫画', '热血', '动作', '夸张透视'],
      targetEpisodes: 8,
      storyboardAspectRatio: '3:4',
      storyboardResolution: '1024x1536',
      consistencyStrength: 82,
    },
  ],
  music: [
    {
      key: 'music-bgm',
      label: '剧情 BGM',
      desc: '适合视频配乐、情绪铺垫和氛围音乐产出。',
      title: '剧情向 BGM 配乐项目',
      description: '用于持续产出剧情段落配乐、氛围铺底和节奏推进音乐的项目。',
      styleTags: ['BGM', '配乐', '情绪推进', '氛围'],
    },
    {
      key: 'music-theme-song',
      label: '主题曲',
      desc: '适合片头、片尾和角色主题歌创作。',
      title: '主题曲创作项目',
      description: '面向片头片尾、角色主题曲和带人声音乐创作的长期项目。',
      styleTags: ['主题曲', '歌词', '片尾', '旋律感'],
    },
  ],
  image: [
    {
      key: 'image-key-visual',
      label: '主视觉 / 海报',
      desc: '适合封面、海报和宣传图产出。',
      title: '主视觉海报概念项目',
      description: '用于主视觉海报、封面KV和宣传素材打样，强调构图、气质和品牌感。',
      styleTags: ['海报', '主视觉', '宣传图', '高质感'],
      storyboardAspectRatio: '3:4',
      storyboardResolution: '1024x1536',
      consistencyStrength: 80,
    },
    {
      key: 'image-character-sheet',
      label: '人物设定图',
      desc: '适合角色立绘、设定图和形象参考。',
      title: '角色设定图项目',
      description: '用于角色立绘、服装设定、形象统一和后续资源图延展。',
      styleTags: ['人物设定', '立绘', '角色统一', '参考图'],
      storyboardAspectRatio: '1:1',
      storyboardResolution: '1024x1024',
      consistencyStrength: 90,
    },
  ],
}

export const MEDIA_STYLE_OPTIONS: Record<ProjectMediaKind, string[]> = {
  video: ['国风', '史诗', '写实', '动漫', '悬疑', '治愈', '都市夜景', '纪录片', '科幻', '复古胶片', '广告感', '战争'],
  video_serial: ['国风', '史诗', '写实', '动漫', '悬疑', '治愈', '都市夜景', '纪录片', '科幻', '复古胶片', '广告感', '战争'],
  comics: ['漫画', '动漫', '国风', '热血', '仙侠', '黑白网点', '夸张透视', '条漫', '角色演出', '校园', '奇幻', '悬疑'],
  music: ['BGM', '主题曲', '国风', '史诗', '热血', '治愈', '悬疑', '电子', '钢琴', '弦乐', '片尾', '战斗'],
  image: ['海报', '主视觉', '人物设定', '场景概念', '写实', '水墨', '插画', '国风', '广告感', '极简', '赛博朋克', '封面'],
}

