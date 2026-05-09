export const STYLE_PRESETS = [
  { value: 'anime', label: '动漫' },
  { value: 'realistic', label: '写实' },
  { value: '3d', label: '3D' },
  { value: 'ink', label: '水墨' },
]

export const STYLE_BADGE_CLASS: Record<string, string> = {
  anime: 'bg-violet-100 text-violet-700',
  realistic: 'bg-blue-100 text-blue-700',
  '3d': 'bg-cyan-100 text-cyan-700',
  ink: 'bg-gray-100 text-gray-700',
}

export interface CharacterFormState {
  name: string
  role_desc: string
  appearance_desc: string
  reference_image_url: string
  style_preset: string
}

export const EMPTY_CHARACTER_FORM: CharacterFormState = {
  name: '',
  role_desc: '',
  appearance_desc: '',
  reference_image_url: '',
  style_preset: '',
}
