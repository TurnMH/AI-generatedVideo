export const DEFAULT_AD_TAGS = ['广告', '品牌宣传', '短视频营销']

export const AD_TEMPLATES = [
  {
    key: 'ecommerce-sale',
    label: '电商促销',
    hint: '强调优惠与转化，适合活动节点投放',
    promptSeed: '产品主打卖点清晰、限时促销、结尾强 CTA，节奏快，镜头以产品特写+真人使用场景为主。',
    style: 'live-action-short',
    motion: 'dynamic',
    duration: 4,
  },
  {
    key: 'brand-story',
    label: '品牌故事',
    hint: '强化品牌感与情绪价值，适合品牌曝光',
    promptSeed: '突出品牌理念与情绪共鸣，通过人物故事线带出产品价值，结尾口号有记忆点。',
    style: 'live-action-film',
    motion: 'cinematic',
    duration: 5,
  },
  {
    key: 'app-growth',
    label: '应用拉新',
    hint: '问题-解决方案-下载引导结构，适合信息流',
    promptSeed: '展示用户痛点与使用前后对比，强调功能亮点与一键下载，引导立即行动。',
    style: 'live-action-short',
    motion: 'gentle',
    duration: 3,
  },
] as const

export const STORYBOARD_TEMPLATES = [
  {
    key: 'product-reveal',
    label: '产品开场',
    hint: '适合先展示产品本体，再用使用场景和收尾 CTA 完成转化。',
    sceneLines: [
      '开场产品特写：直接展示品牌主视觉与核心卖点。',
      '功能细节镜头：突出材质、界面或使用方式。',
      '真实使用场景：让目标用户看到自己在画面里的样子。',
      '收尾 CTA：强化优惠、购买或下载行动。',
    ],
    dialogueLines: [
      '先把最强卖点讲出来。',
      '再补一条能感知到的功能优势。',
      '把用户放进真实使用场景里。',
      '最后明确行动号召。',
    ],
    referenceLines: [
      '白底产品特写 / 主视觉海报',
      '功能细节近景 / 包装或界面截图',
      '人物手持使用 / 场景化照片',
      '品牌结尾海报 / 优惠 CTA 图',
    ],
  },
  {
    key: 'pain-solution',
    label: '痛点解决',
    hint: '适合先抛出痛点，再给出解决方案和结果对比。',
    sceneLines: [
      '痛点开场：展示用户当前遇到的困扰。',
      '方案登场：让产品作为解决方案出现。',
      '结果对比：突出使用前后变化。',
      '行动号召：引导立即体验或购买。',
    ],
    dialogueLines: [
      '这个问题是不是你也遇到过？',
      '我们用这个方案直接解决。',
      '前后变化一眼就能看懂。',
      '现在就去试试。',
    ],
    referenceLines: [
      '问题场景抓拍 / 用户痛点画面',
      '产品解决方案图 / 功能演示截图',
      '前后对比拼图 / 结果对照图',
      '下载页 / 购买按钮 / 优惠弹窗',
    ],
  },
  {
    key: 'social-proof',
    label: '口碑转化',
    hint: '适合用评价、测评和真实反馈增强信任。',
    sceneLines: [
      '用户口碑开场：先给出好评或评分。',
      '真实测评镜头：展示产品在手里的状态。',
      '结果反馈：补充用户使用后的感受。',
      '品牌收尾：统一品牌信息与 CTA。',
    ],
    dialogueLines: [
      '大家都在夸的点，先看这里。',
      '实测一下，效果很直接。',
      '用户反馈和结果都很清晰。',
      '想要同款，马上行动。',
    ],
    referenceLines: [
      '评分截图 / 评论区高赞图',
      '实拍测评 / 近景手持图',
      '用户反馈截图 / 对比图',
      '品牌收口海报 / CTA 图',
    ],
  },
] as const

export const BRAND_VOICE_TEMPLATES = [
  {
    key: 'premium',
    label: '高端质感',
    hint: '适合强调质感、克制和品牌信任的广告。',
    directive: '品牌语气要克制、干净、稍有留白，突出高级感和可信度。',
    contrast: '更适合美妆、消费电子、高客单价品牌。',
  },
  {
    key: 'youthful',
    label: '年轻活力',
    hint: '适合轻快、社交感和即时反馈强的广告。',
    directive: '品牌语气要轻快、口语化、带一点社交感，结尾 CTA 要直接。',
    contrast: '更适合饮料、零食、APP 拉新和短视频投放。',
  },
  {
    key: 'expert',
    label: '专业可信',
    hint: '适合功能说明、工具类和知识型产品。',
    directive: '品牌语气要专业、清楚、避免夸张，用事实和功能点建立信任。',
    contrast: '更适合工具、科技、教育和 B2B 内容。',
  },
  {
    key: 'promo',
    label: '促销直给',
    hint: '适合活动投放、限时促销和转化导向广告。',
    directive: '品牌语气要直接、明确、转化导向强，少修辞，多利益点和行动号召。',
    contrast: '更适合活动节点、优惠券和强 CTA 场景。',
  },
] as const

export const TARGET_MARKET_OPTIONS = [
  {
    key: 'cn-mainland',
    label: '中国大陆',
    prompt: '使用大陆短视频广告口吻，优先本地化消费场景、直接卖点和明确 CTA，避免泛国际化表达。',
  },
  {
    key: 'global-en',
    label: '海外英语',
    prompt: 'Use natural market-local English copy, avoid literal translation, and keep the CTA concise and persuasive.',
  },
  {
    key: 'sea',
    label: '东南亚',
    prompt: '使用容易理解的本地化营销文案，强调价格感、利益点和直接转化，不要过度文艺化。',
  },
] as const

export const SUBTITLE_LANGUAGE_OPTIONS = [
  {
    key: 'zh-CN',
    label: '中文',
    prompt: '字幕、口播与镜头文案全部使用中文，句子短一点，便于烧录和 TTS 对齐。',
  },
  {
    key: 'en-US',
    label: '英文',
    prompt: 'Subtitle and spoken lines should be in natural English; avoid direct translation and keep the lines short.',
  },
  {
    key: 'bilingual',
    label: '中英双语',
    prompt: '字幕按中英双语输出，优先保证中文卖点不丢失，同时保留英文可投放版本。',
  },
] as const

export const CREATIVE_MODE_OPTIONS = [
  {
    key: 'market-first',
    label: '市场优先',
    prompt: '优先匹配目标市场，不要把脚本自动改成过于泛化的广告腔。',
  },
  {
    key: 'script-preserved',
    label: '文案保真',
    prompt: '尽量保留用户原文的卖点和节奏，只做必要的整理，不要重写核心卖点。',
  },
  {
    key: 'director-led',
    label: '导演强化',
    prompt: '允许强化镜头感和节奏，但不要偏离品牌信息和目标市场。',
  },
] as const

export const AD_VIDEO_DRAFT_STORAGE_KEY = 'autovideo:ad-video-draft:v1'
export const AD_VIDEO_HISTORY_STORAGE_KEY = 'autovideo:ad-video-history:v1'
