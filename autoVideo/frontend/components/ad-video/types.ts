import type { VIDEO_MOTION_OPTIONS } from '@/lib/video-style-config'

export type OptimizedAdResult = {
  title: string
  content: string
  outline: string[]
  tags: string[]
}

export type VideoTaskSnapshot = {
  id: number
  status: string
  model_name?: string
  result_url?: string
  hls_url?: string
  error_msg?: string
  created_at?: string
  updated_at?: string
  clips?: Array<{ status?: string }>
  image_urls?: string[]
}

export type TaskProgressRecord = {
  id?: number
  task_id: number
  progress: number
  message: string
  status: string
  timestamp: number
  created_at?: string
}

export type GenerationContext = {
  projectId: number
  projectTitle: string
  prompt: string
  imageUrls: string[]
  sceneDescriptions: string[]
  storyboardTemplate: string
  referenceImageHints: string[]
  brandVoiceTemplate: string
  brandVoiceNotes: string
  modelName: string
  stylePreset: string
  motionMode: (typeof VIDEO_MOTION_OPTIONS)[number]['key']
  videoMode: 'frame_animation' | 'api_generation'
  clipDurationSec: number
  targetMarket: string
  subtitleLanguage: string
  creativeMode: string
  directorNote: string
  subtitleText: string
  dialogues: string[]
  startedAt: string
}

export type RetryRecord = {
  timestamp: string
  fromModel: string
  toModel: string
  reason: string
  status: 'submitted' | 'failed'
}

export type AdTaskLogEntry = {
  at: string
  level: 'info' | 'progress' | 'success' | 'warning' | 'error'
  message: string
}

export type AdVideoDraftSnapshot = {
  title: string
  adPrompt: string
  optimizedScript: string
  imageUrlsText: string
  sceneDescriptionsText: string
  referenceImageHintsText: string
  brandVoiceNotesText: string
  targetMarket: string
  subtitleLanguage: string
  creativeMode: string
  directorNote: string
  subtitleText: string
  selectedTemplate: string
  selectedStoryboardTemplate: string
  selectedBrandVoiceTemplate: string
  selectedVideoModel: string
  selectedImageModel: string
  selectedReferenceHintModel: string
  selectedStylePreset: string
  selectedMotionMode: (typeof VIDEO_MOTION_OPTIONS)[number]['key']
  selectedVideoMode: 'frame_animation' | 'api_generation'
  clipDurationSec: number
  autoOptimizeCopy: boolean
  enableLocalCompression: boolean
  maxImageSide: number
  jpegQuality: number
  autoAvoidLowHourEnabled: boolean
  lowHourThreshold: number
  autoRetryEnabled: boolean
  reviewedStoryboardShotKeys: string[]
}

export type AdReviewChecklistItem = {
  key: string
  label: string
  passed: boolean
  detail: string
  blocking: boolean
}

export type AdVideoHistoryEntry = {
  id: string
  savedAt: string
  label: string
  state: AdVideoDraftSnapshot
}

export type AdGenerationTaskStatus = 'queued' | 'optimizing' | 'uploading' | 'submitting' | 'running' | 'succeeded' | 'failed'

export type AdGenerationTaskEntry = {
  id: string
  createdAt: string
  updatedAt: string
  label: string
  status: AdGenerationTaskStatus
  step: string
  projectId?: number
  outputUrl?: string
  error?: string
  title: string
  marketLabel: string
  brandVoiceLabel: string
  storyboardLabel: string
  subtitleCount: number
  imageCount: number
}

export type StoryboardPreviewStatus = 'blocked' | 'attention' | 'ready' | 'pending' | 'generating' | 'succeeded' | 'failed' | 'submitted'

export type StoryboardPreviewTone = 'slate' | 'amber' | 'blue' | 'emerald'

export type StoryboardPreviewItem = {
  index: number
  shotKey: string
  scene: string
  sceneResolved: string
  scenePlaceholder: string
  dialogue: string
  dialogueResolved: string
  dialoguePlaceholder: string
  referenceHint: string
  referenceHintResolved: string
  referenceHintPlaceholder: string
  imageSource: string
  imagePreviewUrl?: string
  imageStatusLabel: string
  hasDialogue: boolean
  hasReferenceHint: boolean
  storyboardStatus: StoryboardPreviewStatus
  storyboardStatusLabel: string
  storyboardStatusDetail: string
  storyboardStatusTone: StoryboardPreviewTone
  storyboardBlockingItems: string[]
  frontendBlockingDetail?: string
  backendFailureDetail?: string
  isReviewed?: boolean
  realStoryboardId?: number
  realStoryboardSequence?: number
  realStoryboardStatus?: string
  realStoryboardUpdatedAt?: string
  realStoryboardError?: string
}

export type BrandVoiceTemplateKey = string

export type AssetLike = {
  url?: string
  file_url?: string
  thumbnail_url?: string
  metadata?: unknown
} | null | undefined
