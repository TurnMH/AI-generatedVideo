import type { Episode } from '@/types'

export type EpisodeCountRecommendation = {
  count: number
  reason: string
}

export const COMIC_STYLE_PRESETS = [
  { key: 'story-manga', label: '剧情漫画', desc: '保留人物表演与镜头叙事，适合连续剧情。', suffix: '剧情漫画分镜，黑白网点，清晰线稿，电影感构图，角色表情明确' },
  { key: 'action-comic', label: '热血战斗', desc: '强调速度线、冲击力与夸张动作。', suffix: '热血战斗漫画分镜，强透视，速度线，动作张力，夸张漫画构图' },
  { key: 'guofeng-comic', label: '国风条漫', desc: '适合神话、仙侠、古风题材。', suffix: '国风漫画分镜，东方服饰，长卷式构图，层次云雾，细腻线稿' },
] as const

export type ComicStylePresetKey = (typeof COMIC_STYLE_PRESETS)[number]['key']

export function recommendEpisodeCount(scriptText: string): EpisodeCountRecommendation | null {
  const normalized = scriptText.replace(/\r/g, '').trim()
  if (!normalized) return null

  const chapterRe = /^[　 \t]*(第[零一二三四五六七八九十百千万\d]+[回章节集卷幕]|Chapter\s+\d+|CHAPTER\s+\d+|序[章言幕]|楔\s*子|引\s*子|尾\s*声|终\s*章|番\s*外)[　 \t]*(.*)$/gm
  const chapterMatches = normalized.match(chapterRe) ?? []
  if (chapterMatches.length >= 2) {
    return {
      count: Math.min(200, chapterMatches.length),
      reason: `识别到 ${chapterMatches.length} 个章节标题`,
    }
  }

  const runeCount = Array.from(normalized).length
  const estimated = Math.max(1, Math.min(200, Math.floor(runeCount / 2000) || 1))
  return {
    count: estimated,
    reason: `按正文长度约 ${runeCount.toLocaleString()} 字估算`,
  }
}

export function splitEpisodeIntoComicPanels(episode: Episode, panelCount: number, styleKey: ComicStylePresetKey) {
  const source = `${episode.summary || ''}\n${episode.script_excerpt || ''}`.trim() || episode.title
  const normalized = source
    .replace(/\r/g, '')
    .replace(/\n{2,}/g, '\n')
    .trim()
  const rawSegments = normalized
    .split(/[\n。！？!?；;]+/)
    .map((segment) => segment.trim())
    .filter(Boolean)
  const style = COMIC_STYLE_PRESETS.find((preset) => preset.key === styleKey) ?? COMIC_STYLE_PRESETS[0]

  if (rawSegments.length === 0) {
    return [
      {
        scene_description: `${episode.title}。${style.suffix}`,
        dialogue: episode.summary || episode.title,
      },
    ]
  }

  const limitedSegments = rawSegments.slice(0, Math.max(panelCount * 2, panelCount))
  const actualPanels = Math.min(panelCount, Math.max(1, limitedSegments.length))
  const chunkSize = Math.max(1, Math.ceil(limitedSegments.length / actualPanels))

  return Array.from({ length: actualPanels }, (_, index) => {
    const chunk = limitedSegments.slice(index * chunkSize, (index + 1) * chunkSize)
    const combined = chunk.join('，').trim()
    const dialogue = combined.length > 80 ? `${combined.slice(0, 80)}...` : combined
    const panelLead = index === 0 ? `第${episode.episode_number}集开场` : `第${episode.episode_number}集第${index + 1}格`
    return {
      scene_description: `${panelLead}：${combined || episode.title}。${style.suffix}`,
      dialogue,
    }
  })
}
