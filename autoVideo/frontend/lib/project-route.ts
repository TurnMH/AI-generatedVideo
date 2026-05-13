import { STATIC_EXPORT_DYNAMIC_ID } from '@/lib/static-export'

export function parseProjectIdParam(param: string | string[] | undefined): number | null {
  const rawValue = Array.isArray(param) ? param[0] : param
  const projectId = Number(rawValue)

  if (!Number.isFinite(projectId) || projectId <= 0) {
    return null
  }

  return projectId
}

export function parseProjectIdFromPathname(
  pathname: string | null | undefined,
  routeSegment: 'projects' | 'video-serial'
): number | null {
  if (!pathname) return null

  const segments = pathname.split('/').filter(Boolean)
  const routeIndex = segments.indexOf(routeSegment)
  const rawValue = routeIndex >= 0 ? segments[routeIndex + 1] : undefined

  return parseProjectIdParam(rawValue)
}

export function resolveProjectIdParam(
  param: string | string[] | undefined,
  pathname: string | null | undefined,
  routeSegment: 'projects' | 'video-serial'
): number | null {
  const parsed = parseProjectIdParam(param)
  if (parsed) return parsed

  const rawValue = Array.isArray(param) ? param[0] : param
  if (rawValue !== STATIC_EXPORT_DYNAMIC_ID) {
    return null
  }

  return parseProjectIdFromPathname(pathname, routeSegment)
}