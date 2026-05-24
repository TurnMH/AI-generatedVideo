export const AD_CAMPAIGN_TYPE_OPTIONS = [
  { value: 'performance', label: '信息流转化广告' },
  { value: 'brand', label: '品牌质感广告' },
  { value: 'product', label: '产品展示广告' },
  { value: 'ugc', label: '口播 / UGC 广告' },
  { value: 'launch', label: '活动 / 上新预热广告' },
] as const

export const AD_OBJECTIVE_OPTIONS = [
  { value: 'conversion', label: '促进转化' },
  { value: 'click', label: '拉点击量' },
  { value: 'awareness', label: '品牌认知' },
  { value: 'lead', label: '留资获客' },
  { value: 'engagement', label: '互动传播' },
] as const

export const AD_DURATION_OPTIONS = [
  { value: '15s', label: '15 秒快转化', storyboardDuration: 4 },
  { value: '30s', label: '30 秒标准广告', storyboardDuration: 5 },
  { value: '60s', label: '60 秒完整讲述', storyboardDuration: 6 },
] as const

export type AdCampaignTemplate = {
  key: string
  label: string
  desc: string
  tags: readonly string[]
  campaignType: (typeof AD_CAMPAIGN_TYPE_OPTIONS)[number]['value']
  objective: (typeof AD_OBJECTIVE_OPTIONS)[number]['value']
  targetAudience: string
  cta: string
  durationPreference: (typeof AD_DURATION_OPTIONS)[number]['value']
  styleTags: readonly string[]
  videoStylePreset: string
  videoMotionMode: string
  storyboardAspectRatio: string
  storyboardResolution: string
  storyboardDuration: number
  consistencyStrength: number
  targetEpisodes: number
  enableDubbing: boolean
  enableSubtitle: boolean
  preferredVideoRuntimeKeys: readonly string[]
}

export const AD_CAMPAIGN_TEMPLATES: AdCampaignTemplate[] = [
  {
    key: 'ad-performance-hook',
    label: '强钩子转化广告',
    desc: '适合信息流投放、前三秒抓人、口播可念、卖点压缩和立即转化。',
    tags: ['信息流', '转化', '快节奏'],
    campaignType: 'performance',
    objective: 'conversion',
    targetAudience: '高意向新客 / 即将下单用户，偏好直接结论、明确利益点、短句口播',
    cta: '立即下单，马上获得核心优惠或结果',
    durationPreference: '15s',
    styleTags: ['广告', '信息流', '转化导向', '快节奏', '强钩子', '口播可念', '卖点前置', '明确CTA'],
    videoStylePreset: 'fashion-commercial',
    videoMotionMode: 'dynamic',
    storyboardAspectRatio: '9:16',
    storyboardResolution: '1080x1920',
    storyboardDuration: 4,
    consistencyStrength: 86,
    targetEpisodes: 6,
    enableDubbing: true,
    enableSubtitle: true,
    preferredVideoRuntimeKeys: ['hubagi-voe3.1', 'hubagi-TC-GV'],
  },
  {
    key: 'ad-brand-film',
    label: '品牌质感广告',
    desc: '适合品牌升级、主视觉大片，同时保留一句能被念出来的品牌主张。',
    tags: ['品牌', '质感', '主视觉'],
    campaignType: 'brand',
    objective: 'awareness',
    targetAudience: '品牌潜在人群 / 核心消费群，偏好质感表达但仍需要清晰记忆点',
    cta: '了解更多，记住品牌主张并进入下一步了解',
    durationPreference: '30s',
    styleTags: ['广告', '品牌宣传', '高质感', '产品主视觉', '品牌口号', '记忆点明确'],
    videoStylePreset: 'fashion-commercial',
    videoMotionMode: 'cinematic',
    storyboardAspectRatio: '16:9',
    storyboardResolution: '1920x1080',
    storyboardDuration: 5,
    consistencyStrength: 90,
    targetEpisodes: 8,
    enableDubbing: false,
    enableSubtitle: false,
    preferredVideoRuntimeKeys: ['hubagi-voe3.1', 'sora2'],
  },
  {
    key: 'ad-product-demo',
    label: '产品卖点展示',
    desc: '适合功能演示、核心卖点分段讲解、电商详情导流和逐句可念口播。',
    tags: ['产品展示', '卖点讲解', '电商'],
    campaignType: 'product',
    objective: 'click',
    targetAudience: '有明确需求的搜索 / 比价用户，希望快速听懂差异点与购买理由',
    cta: '立即查看详情，快速确认是否适合自己',
    durationPreference: '30s',
    styleTags: ['广告', '产品特写', '卖点展示', '电商', '功能讲解', '逐条口播'],
    videoStylePreset: 'realistic-drama',
    videoMotionMode: 'gentle',
    storyboardAspectRatio: '9:16',
    storyboardResolution: '1080x1920',
    storyboardDuration: 5,
    consistencyStrength: 88,
    targetEpisodes: 7,
    enableDubbing: true,
    enableSubtitle: true,
    preferredVideoRuntimeKeys: ['wan', 'hubagi-voe3.1'],
  },
  {
    key: 'ad-ugc-testimonial',
    label: '口播 / UGC 种草',
    desc: '适合用户证言、真人口播、种草安利和社媒原生广告，强调台词自然可念。',
    tags: ['UGC', '口播', '种草'],
    campaignType: 'ugc',
    objective: 'engagement',
    targetAudience: '社媒活跃人群 / 内容兴趣用户，偏好真实口语、短句表达、生活化分享',
    cta: '马上试试，先用一次就能感受到差异',
    durationPreference: '15s',
    styleTags: ['广告', '口播', '生活方式', '种草', '第一人称表达', '真实分享感'],
    videoStylePreset: 'live-action-short',
    videoMotionMode: 'gentle',
    storyboardAspectRatio: '9:16',
    storyboardResolution: '1080x1920',
    storyboardDuration: 4,
    consistencyStrength: 82,
    targetEpisodes: 5,
    enableDubbing: true,
    enableSubtitle: true,
    preferredVideoRuntimeKeys: ['hubagi-voe3.1', 'wan'],
  },
]

export function getAdOptionLabel(
  options: ReadonlyArray<{ value: string; label: string }>,
  value: string
): string {
  return options.find((item) => item.value === value)?.label ?? value
}
