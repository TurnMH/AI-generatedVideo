export const LOCATION_VIEW_OPTIONS = [
  { value: '', label: '自动推断' },
  { value: 'exterior', label: '外景' },
  { value: 'interior', label: '内景' },
  { value: 'entrance', label: '门口/过渡' },
  { value: 'aerial', label: '俯视/航拍' },
] as const

export type LocationViewValue = (typeof LOCATION_VIEW_OPTIONS)[number]['value']

export function locationViewLabel(value?: string): string {
  const normalized = (value || '').trim().toLowerCase()
  const found = LOCATION_VIEW_OPTIONS.find((opt) => opt.value === normalized)
  if (found) return found.label
  switch (normalized) {
    case 'outdoor':
    case 'outside':
      return '外景'
    case 'indoor':
    case 'inside':
      return '内景'
    case 'doorway':
    case 'threshold':
      return '门口/过渡'
    case 'overhead':
    case 'bird':
      return '俯视/航拍'
    default:
      return normalized ? value!.trim() : '自动推断'
  }
}

export type SceneSpatialMetadata = {
  view_type?: string
  location_hub?: string
  location_zone?: string
}

export function readSceneSpatialMetadata(metadata?: Record<string, unknown> | null): SceneSpatialMetadata {
  if (!metadata) return {}
  const viewType = typeof metadata.view_type === 'string' ? metadata.view_type.trim() : ''
  const locationHub = typeof metadata.location_hub === 'string' ? metadata.location_hub.trim() : ''
  const locationZone = typeof metadata.location_zone === 'string' ? metadata.location_zone.trim() : ''
  return {
    ...(viewType ? { view_type: viewType } : {}),
    ...(locationHub ? { location_hub: locationHub } : {}),
    ...(locationZone ? { location_zone: locationZone } : {}),
  }
}

/** Infer a display label when storyboard has no explicit location_zone. */
export function inferLocationViewHint(text: string): string {
  const combined = text.toLowerCase()
  if (/门口|门槛|入口|doorway|entrance|threshold/.test(combined)) return 'entrance'
  if (/店内|店里|铺内|室内|内景|屋内|后厨|厨房|inside|interior|indoor/.test(combined)) return 'interior'
  if (/门外|店外|室外|外景|街头|街道|outside|exterior|outdoor|street/.test(combined)) return 'exterior'
  return ''
}
