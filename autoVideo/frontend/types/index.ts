// ─── User ────────────────────────────────────────────────────
export interface User {
  id: number
  username: string
  email: string
  avatar_url: string
  role: 'user' | 'creator' | 'admin'
  status: string
}

// ─── Model ───────────────────────────────────────────────────
export type ModelType = 'llm' | 'image' | 'video' | 'audio'
export type SpeedRating = 'fast' | 'balanced' | 'quality' | 'slow'
export type ConsistencyMethod = 'ip_adapter' | 'lora' | 'reference_image' | 'none'
export type VideoModelMode = 'frame_animation' | 'api_generation' | 'both'

export interface Model {
  id: number
  name: string
  model_key: string
  type: ModelType
  provider: string
  context_window?: number
  input_price?: number
  output_price?: number
  speed_rating: SpeedRating
  capability_tags: string[]
  supports_consistency: boolean
  consistency_method: ConsistencyMethod
  video_mode?: VideoModelMode
  max_resolution?: string
  supported_ratios: string[]
  is_active: boolean
  is_default: boolean
  api_endpoint?: string
  api_key_ref?: string
  description?: string
  failure_reason?: string
  config?: Record<string, unknown>
  cost_per_unit: number
  unit: string
  priority: number
  health_status?: 'healthy' | 'unhealthy' | 'unknown'
  created_at: string
  updated_at: string
}

// ─── Project ─────────────────────────────────────────────────
export type ProjectStatus =
  | 'draft'
  | 'script_processing'
  | 'script_ready'
  | 'asset_generating'
  | 'asset_ready'
  | 'storyboard_generating'
  | 'storyboard_ready'
  | 'video_generating'
  | 'completed'
  | 'paused'
  | 'failed'

export interface StageProgress {
  status: 'pending' | 'running' | 'done' | 'failed' | 'skipped'
  total?: number
  completed?: number
  current?: number
}

export interface ProjectProgress {
  stage?: 'episode_splitting' | 'scene_splitting' | 'script_prepping' | 'idle'
  episode_split?: StageProgress
  scene_split?: StageProgress
  message?: string
  phase_label?: string
  next_step?: string
  current_episode?: number
  total_episodes?: number
  started_at?: string
  updated_at?: string
  // Legacy fields for backward compat
  script?: StageProgress
  asset?: StageProgress
  storyboard?: StageProgress
  dubbing?: StageProgress
  video?: StageProgress
}

export interface StoryboardConfig {
  aspect_ratio: string
  resolution: string
  duration: number
  camera_movement: string
  video_mode: string
  style_preset?: string
  motion_mode?: string
  video_model?: string  // runtime model key (e.g. "kling", "vidu", "wan") for director duration guidance
}

export interface WatermarkConfig {
  enabled: boolean
  type: 'text' | 'image'
  text?: string
  image_url?: string
  position: 'top-left' | 'top-right' | 'bottom-left' | 'bottom-right' | 'center'
  opacity: number
  size: 'small' | 'medium' | 'large'
  apply_to: ('video' | 'storyboard_image' | 'export')[]
}

export type ProjectType = 'video' | 'video_serial' | 'comics' | 'music' | 'image'

export interface Project {
  id: number
  user_id: number
  title: string
  description: string
  project_type: ProjectType
  cover_url: string
  logo_url: string
  script_file_url: string
  script_text: string
  script_file_size: number
  script_versions: ScriptVersionRef[]
  status: ProjectStatus
  progress: ProjectProgress
  target_episodes: number
  style_tags: string[]
  text_model_id?: number
  image_model_id?: number
  video_model_id?: number
  tts_model_id?: number
  enable_dubbing: boolean
  enable_subtitle: boolean
  video_mode: 'frame_animation' | 'api_generation'
  storyboard_config: StoryboardConfig
  watermark_config: WatermarkConfig
  consistency_strength: number
  image_style_prefix: string
  storage_used_bytes: number
  keyword_library?: {
    characters: string[]
    locations: string[]
    events: string[]
    props: string[]
    split_keywords?: string[]
  }
  created_at: string
  updated_at: string
  episodes?: Episode[]
}

export interface ScriptVersionRef {
  url: string
  size: number
  created_at: string
}

// ─── Episode ─────────────────────────────────────────────────
export interface Episode {
  id: number
  project_id: number
  episode_number: number
  title: string
  summary: string
  script_excerpt: string
  word_count: number
  estimated_duration: number
  status: 'draft' | 'generating' | 'done' | 'pending' | 'script_prepping' | 'scene_splitting' | 'scene_ready' | 'failed'
  version: number
  keywords: string[]
  created_at: string
  updated_at: string
  // Script optimization
  optimize_status?: '' | 'optimizing' | 'done' | 'failed'
  optimized_text?: string
  original_excerpt?: string
  // AI review
  review_status?: '' | 'reviewing' | 'done' | 'failed'
  review_result?: {
    score: {
      completeness: number
      integrity: number
      consistency: number
      transitions: number
      dialog_quality: number
    }
    issues: Array<{
      severity: 'critical' | 'warning' | 'info'
      type: string
      description: string
      suggestion: string
    }>
    overall: string
    strengths: string
  } | null
}

// ─── Asset ───────────────────────────────────────────────────
export type AssetType = 'character' | 'scene' | 'prop' | 'image'
export type AssetStatus = 'pending' | 'generating' | 'extracting' | 'paused' | 'completed' | 'failed' | 'qa_failed'

export interface Asset {
  id: number
  project_id: number
  type: AssetType
  name: string
  description: string
  image_url: string
  panel_images?: string[]
  composite_image_url?: string
  seed?: number
  panel_validation?: {
    panels: Array<{
      panel: string    // closeup | front | side | back
      pass: boolean
      score: number
      issues?: string[]
    }>
    overall_pass: boolean
    summary: string
    model: string
    checked_at: string
  }
  consistency_ref: Record<string, unknown>
  metadata: Record<string, unknown>
  status: AssetStatus | 'partial'
  is_locked: boolean
  is_manual: boolean
  prompt_used: string
  error_msg: string
  agent_history: ChatMessage[]
  episode_ids: number[]
  voice_model?: string
  created_at: string
  updated_at: string
}

export interface ChatMessage {
  role: 'user' | 'assistant'
  content: string
  timestamp: string
  image_url?: string
}

// ─── Storyboard ──────────────────────────────────────────────
export type StoryboardStatus = 'pending' | 'generating' | 'paused' | 'completed' | 'failed' | 'voided'

export interface StoryboardVersion {
  id: number
  storyboard_id: number
  version_number: number
  image_url: string
  oss_key: string
  size_bytes: number
  prompt_used: string
  is_current: boolean
  created_at: string
}

export interface Storyboard {
  id: number
  project_id: number
  episode_id?: number
  sequence_number: number
  scene_description: string
  characters: string[]
  location: string
  camera_movement: string
  mood?: string
  duration: number
  aspect_ratio: string
  resolution: string
  video_mode?: string
  dialogue: string
  current_version: number
  image_url: string
  prompt_used: string
  status: StoryboardStatus
  error_msg?: string
  is_voided: boolean
  is_manual_edited: boolean
  agent_history: ChatMessage[]
  asset_ids: number[]
  scene_group_key?: string
  is_scene_first_clip?: boolean
  end_frame_image_url?: string
  versions?: StoryboardVersion[]
  created_at: string
  updated_at: string
}

// ─── CharacterGroup ──────────────────────────────────────────
export interface CharacterGroup {
  id: number
  project_id: number
  name: string
  description?: string
  voice_model?: string
  voice_sample_url?: string
  sort_order: number
  created_at: string
  updated_at: string
  variants?: CharacterData[]
}

// ─── Script Version ──────────────────────────────────────────
export interface ScriptVersion {
  id: number
  project_id: number
  version_number: number
  file_url: string
  oss_key: string
  file_size: number
  is_current: boolean
  created_at: string
}

// ─── Script Library ──────────────────────────────────────────
export type ScriptLibrarySource = 'uploaded' | 'showcase'

export interface ScriptLibraryItem {
  id: number
  title: string
  author: string
  description: string
  cover_url: string
  file_url: string
  file_size: number
  word_count: number
  genre: string
  tags: string[]
  source: ScriptLibrarySource
  project_id?: number
  user_id: number
  is_public: boolean
  sort_order: number
  created_at: string
  updated_at: string
}

// ─── Video ───────────────────────────────────────────────────
export interface Video {
  id: number
  project_id: number
  episode_id?: number
  status: 'pending' | 'generating' | 'completed' | 'failed'
  progress: number
  file_url: string
  hls_url: string
  duration: number
  file_size: number
  render_log: string
  created_at: string
  updated_at: string
}

// ─── Storage ─────────────────────────────────────────────────
export interface CategorySummary {
  bytes: number
  count: number
}

export interface StorageFile {
  id: number
  category: string
  label: string
  size_bytes: number
  created_at: string
  is_current: boolean
  deletable: boolean
}

export interface StorageDetails {
  total_bytes: number
  categories: Record<string, CategorySummary>
  files: StorageFile[]
}

// ─── Task ────────────────────────────────────────────────────
export interface Task {
  id: number
  task_type: string
  status: 'pending' | 'running' | 'succeeded' | 'failed' | 'cancelled'
  priority: number
  progress?: number
  error_msg?: string
  created_at: string
  started_at?: string
  finished_at?: string
}

export interface TaskProgress {
  task_id: number
  progress: number
  message: string
  status: string
  timestamp: number
}

// ─── Scene (legacy, kept for backward compat) ────────────────
export interface Scene {
  id: number
  script_id: number
  episode_id: number
  scene_order: number
  description: string
  setting: string
  emotion: string
  characters: string[]
  prompt_draft: string
  status: string
  image_url?: string
}

// ─── Pipeline ────────────────────────────────────────────────
export type PipelineStage =
  | 'INIT'
  | 'SCRIPT_ANALYZING'
  | 'ASSETS_EXTRACTING'
  | 'STORYBOARD_GENERATING'
  | 'IMAGES_GENERATING'
  | 'IMAGES_REVIEWING'
  | 'VIDEOS_GENERATING'
  | 'COMPOSING'
  | 'AUTO_FIXING'
  | 'DONE'
  | 'FAILED'

export interface PipelineStatus {
  pipeline_id: string
  project_id: number
  current_stage: PipelineStage
  started_at: string
  finished_at?: string
  error?: string
}

export interface Pipeline {
  id: string
  project_id: number
  config: {
    style: string
    quality: string
    max_episodes: number
  }
  current_stage: PipelineStage
  stages_completed: PipelineStage[]
  started_at: string
  finished_at?: string
  error?: string
  logs: string[]
}

// ─── API Response ────────────────────────────────────────────
export interface APIResponse<T = unknown> {
  code: number
  message: string
  data: T
}

export interface PaginatedResponse<T> {
  items: T[]
  total: number
  page: number
  page_size: number
}

// ─── API Key ─────────────────────────────────────────────────
export interface APIKey {
  id: number
  user_id: number
  provider: string
  key_alias: string
  base_url: string
  model_scope: string
  is_system: boolean
  is_active: boolean
  created_at: string
}

export interface SystemAPIKey {
  id: number
  provider: string
  key_alias: string
  base_url: string
  model_scope: string
  is_active: boolean
  status: string
  created_at: string
}

// ─── Legacy types (kept for backward compat) ─────────────────
export interface Character {
  id: number
  project_id: number
  name: string
  description: string
  reference_images: string[]
  lora_model_url?: string
  created_at: string
}

// ─── CharacterData (matches character-service backend model) ─────────────────
export interface CharacterData {
  id: number
  project_id: number
  name: string
  role_desc?: string
  appearance_desc?: string
  reference_image_url?: string
  style_preset?: string
  lora_model_id?: string
  fixed_seed?: number
  created_at?: string
}

export interface ImageTask {
  id: number
  scene_id: number
  prompt: string
  model: string
  status: 'pending' | 'running' | 'succeeded' | 'failed'
  result_url?: string
  created_at: string
  finished_at?: string
}

export interface VideoTask {
  id: number
  scene_id: number
  image_url: string
  motion_prompt?: string
  model: string
  status: 'pending' | 'running' | 'succeeded' | 'failed'
  result_url?: string
  duration_sec?: number
  created_at: string
  finished_at?: string
}

export interface Skill {
  id: number
  project_id: number
  character_id?: number | null
  name: string
  skill_type: string   // combat|exploration|social|special
  use_case: string     // storyboard|extraction|prompt
  description: string
  is_active: boolean
  created_at: string
  updated_at: string
}

export interface PromptTemplate {
  id: number
  name: string
  style_key: string
  description: string
  content: string
  resource_type: string  // character|scene|item|storyboard
  model_binding: string
  version: string
  is_active: boolean
  sort_order: number
  created_at: string
  updated_at: string
}

// ─── Image Model Capabilities ────────────────────────────────
export type ImageSizeMode = 'arbitrary_wh' | 'enum_size' | 'passthrough'
export type ImageVerificationLevel = 'verified' | 'partial' | 'assumed'
export type ImageRatioIntent = 'square' | 'portrait' | 'landscape'

export interface ImageModelCapability {
  key: string
  available: boolean
  provider_key: string
  size_mode: ImageSizeMode
  verification: ImageVerificationLevel
  allowed_sizes?: string[]
  default_square?: string
  default_portrait?: string
  default_landscape?: string
  min_width?: number
  max_width?: number
  min_height?: number
  max_height?: number
  require_multiple?: number
  notes?: string
}

export interface TaskTypeCapability {
  task_type: string
  ratio_intent: ImageRatioIntent
}

export interface ImageModelCapabilitiesResponse {
  models: ImageModelCapability[]
  task_type_capabilities: TaskTypeCapability[]
}
