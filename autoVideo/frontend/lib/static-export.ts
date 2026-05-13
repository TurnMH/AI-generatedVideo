export const STATIC_EXPORT_DYNAMIC_ID = '__dynamic__'

export function generateStaticIdParam() {
  return [{ id: STATIC_EXPORT_DYNAMIC_ID }]
}