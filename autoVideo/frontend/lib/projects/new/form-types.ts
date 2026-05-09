export interface FormData {
  title: string
  description: string
  logoFile: File | null
  logoPreview: string
  style_tags: string[]
  target_episodes: number
  text_model_id: number | undefined
  image_model_id: number | undefined
  video_model_id: number | undefined
  video_style_preset: string
  liveaction_region: string
  liveaction_era: string
  liveaction_ethnicity: string
  tts_model_id: number | undefined
  video_mode: 'frame_animation' | 'api_generation'
  enable_dubbing: boolean
  enable_subtitle: boolean
  watermark_enabled: boolean
  watermark_type: 'text' | 'image'
  watermark_text: string
  watermark_position: 'top-left' | 'top-right' | 'bottom-left' | 'bottom-right' | 'center'
  watermark_opacity: number
  scriptFile: File | null
  scriptPreview: string
  storyboard_aspect_ratio: string
  storyboard_resolution: string
  storyboard_duration: number
  consistency_strength: number
  video_motion_mode: string
}

export const initialFormData: FormData = {
  title: '',
  description: '',
  logoFile: null,
  logoPreview: '',
  style_tags: [],
  target_episodes: 1,
  text_model_id: undefined,
  image_model_id: undefined,
  video_model_id: undefined,
  video_style_preset: 'anime-2d',
  liveaction_region: '',
  liveaction_era: '',
  liveaction_ethnicity: '',
  tts_model_id: undefined,
  video_mode: 'api_generation',
  enable_dubbing: true,
  enable_subtitle: true,
  watermark_enabled: false,
  watermark_type: 'text',
  watermark_text: '',
  watermark_position: 'bottom-right',
  watermark_opacity: 80,
  scriptFile: null,
  scriptPreview: '',
  storyboard_aspect_ratio: '16:9',
  storyboard_resolution: '1920x1080',
  storyboard_duration: 5,
  consistency_strength: 75,
  video_motion_mode: 'gentle',
}
