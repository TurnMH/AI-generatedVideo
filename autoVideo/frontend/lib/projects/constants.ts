export const FALLBACK_VOICE_OPTIONS = [
  { value: 'default', label: '云剑 (男·激昂)' },
  { value: 'male2', label: '云希 (男·活泼)' },
  { value: 'male3', label: '云扬 (男·专业)' },
  { value: 'female1', label: '晓晓 (女·温暖)' },
  { value: 'female2', label: '晓伊 (女·活泼)' },
  { value: 'dialect', label: '晓北 (东北方言)' },
  { value: 'dialect-shaanxi', label: '晓妮 (陕西方言)' },
  { value: 'cantonese-female1', label: '晓佳 (粤语·女)' },
  { value: 'cantonese-female2', label: '晓曼 (粤语·女)' },
  { value: 'cantonese-male1', label: '云龙 (粤语·男)' },
  { value: 'taiwan-female1', label: '晓臻 (台湾普通话·女)' },
  { value: 'taiwan-female2', label: '晓雨 (台湾普通话·女)' },
  { value: 'taiwan-male1', label: '云哲 (台湾普通话·男)' },
]

export const GENERATION_STAGE_HINTS: Record<string, string[]> = {
  queued: ['正在排队分配算力', '已进入生成队列，马上开始', '当前任务较多，正在等待执行'],
  submitting: ['正在提交生成请求', '正在同步最新描述到图片服务', '马上开始创建图片任务'],
  submitted: ['任务已创建，等待图片服务接单', '图片服务已收到请求', '正在准备渲染环境'],
  rendering: ['AI 正在细化画面构图', '正在绘制人物与场景细节', '正在生成高质量图片版本'],
  processing: ['图片服务处理中', '正在整理生成结果', '请稍候，结果即将返回'],
  completed: ['图片已生成完成', '新版本已可查看', '可以从候选图中选择更满意的一张'],
  failed: ['本次生成失败，请稍后重试', '图片服务返回失败状态', '你也可以调整描述后再次生成'],
}
