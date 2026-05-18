import axios from 'axios'
import type {
  Model,
  Project,
  Episode,
  Asset,
  Storyboard,
  StoryboardConfig,
  CharacterData,
  ImageModelCapabilitiesResponse,
} from '@/types'

function normalizeBaseURL(value?: string) {
  const trimmed = value?.trim()
  if (!trimmed || trimmed === '/') return ''
  return trimmed.replace(/\/+$/, '')
}

function buildURL(baseURL: string, path: string) {
  return `${baseURL}${path}`
}

function getDirectGatewayBaseURL() {
  if (typeof window === 'undefined') {
    return 'http://localhost:8000'
  }
  return `${window.location.protocol}//${window.location.hostname}:8000`
}

const BASE_URL = normalizeBaseURL(process.env.NEXT_PUBLIC_API_URL)
const SCRIPT_BASE_URL = normalizeBaseURL(process.env.NEXT_PUBLIC_SCRIPT_API_URL) || BASE_URL

const api = axios.create({ baseURL: BASE_URL || undefined, timeout: 30000 })
const scriptApi = axios.create({ baseURL: SCRIPT_BASE_URL || undefined, timeout: 30000 })

function applyInterceptors(client: typeof api) {
  client.interceptors.request.use((config) => {
    if (typeof window !== 'undefined') {
      const token = localStorage.getItem('access_token')
      if (token) {
        config.headers.Authorization = `Bearer ${token}`
        // Extract user_id from JWT for services that need X-User-ID header
        try {
          const payload = JSON.parse(atob(token.split('.')[1]))
          const uid = payload.user_id || payload.sub
          if (uid) config.headers['X-User-ID'] = String(uid)
        } catch { /* ignore decode errors */ }
      }
    }
    return config
  })

  client.interceptors.response.use(
    (res) => res.data,
    async (err) => {
      if (err.response?.status === 401) {
        const refreshToken =
          typeof window !== 'undefined' ? localStorage.getItem('refresh_token') : null
        if (refreshToken) {
          try {
            const res = await axios.post(buildURL(BASE_URL, '/api/v1/auth/token/refresh'), {
              refresh_token: refreshToken,
            })
            localStorage.setItem('access_token', res.data.data.access_token)
            localStorage.setItem('refresh_token', res.data.data.refresh_token)
            err.config.headers.Authorization = `Bearer ${res.data.data.access_token}`
            return client(err.config)
          } catch {
            localStorage.clear()
            const returnUrl = encodeURIComponent(window.location.pathname + window.location.search)
            window.location.href = `/login?redirect=${returnUrl}`
          }
        } else {
          if (typeof window !== 'undefined') {
            const returnUrl = encodeURIComponent(window.location.pathname + window.location.search)
            window.location.href = `/login?redirect=${returnUrl}`
          }
        }
      }
      return Promise.reject(err)
    }
  )
}

applyInterceptors(api)
applyInterceptors(scriptApi)

function getCurrentUserIdFromToken(): number {
  if (typeof window === 'undefined') return 0
  try {
    const token = localStorage.getItem('access_token')
    if (!token) return 0
    const payload = JSON.parse(atob(token.split('.')[1]))
    const raw = payload.user_id || payload.sub
    const userId = Number(raw)
    return Number.isFinite(userId) && userId > 0 ? userId : 0
  } catch {
    return 0
  }
}

export const authAPI = {
  login: (email: string, password: string) =>
    api.post('/api/v1/auth/login/password', { email, password }),
  register: (data: { username: string; email: string; password: string }) =>
    api.post('/api/v1/auth/register', data),
  me: () => api.get('/api/v1/auth/me'),
  logout: () => api.post('/api/v1/auth/logout'),
  refreshToken: (refreshToken: string) =>
    api.post('/api/v1/auth/token/refresh', { refresh_token: refreshToken }),
  listAPIKeys: () => api.get('/api/v1/auth/api-keys'),
  addAPIKey: (data: { provider: string; alias?: string; key: string; base_url?: string; model_scope?: string }) =>
    api.post('/api/v1/auth/api-keys', data),
  deleteAPIKey: (id: number) => api.delete(`/api/v1/auth/api-keys/${id}`),
  listSystemAPIKeys: () => api.get('/api/v1/auth/system-api-keys'),
}

export const projectAPI = {
  list: (params?: { keyword?: string; status?: string; project_type?: string; page?: number; page_size?: number }) =>
    api.get('/api/v1/projects', { params }),
  get: (id: number) => api.get(`/api/v1/projects/${id}`),
  create: (data: Partial<Project>) => api.post('/api/v1/projects', data),
  update: (id: number, data: Partial<Project>) =>
    api.put(`/api/v1/projects/${id}`, data),
  delete: (id: number) => api.delete(`/api/v1/projects/${id}`),
  pause: (id: number) => api.post(`/api/v1/projects/${id}/pause`),
  resume: (id: number) => api.post(`/api/v1/projects/${id}/resume`),
  clone: (id: number) => api.post(`/api/v1/projects/${id}/clone`),
  // Episodes
  listEpisodes: (id: number) => api.get(`/api/v1/projects/${id}/episodes`),
  createEpisode: (id: number, data: { episode_number: number; title: string; summary?: string; script_excerpt?: string }) =>
    api.post(`/api/v1/projects/${id}/episodes`, data),
  generateEpisodes: (
    id: number,
    keywords?: { characters?: string[]; locations?: string[]; events?: string[]; props?: string[]; split_keywords?: string[] },
    options?: { force?: boolean; autoStoryboard?: boolean }
  ) =>
    api.post(`/api/v1/projects/${id}/episodes/generate`, {
      ...(keywords ? { keywords } : {}),
      ...(options?.force ? { force: true } : {}),
      ...(options?.autoStoryboard ? { auto_storyboard: true } : {}),
    }),
  updateEpisode: (id: number, eid: number, data: Partial<Episode>) =>
    api.put(`/api/v1/projects/${id}/episodes/${eid}`, data),
  extractStoryboards: (id: number) =>
    api.post(`/api/v1/projects/${id}/episodes/extract-storyboards`),
  extractEpisodeStoryboards: (id: number, eid: number, skipAssetRefresh?: boolean) =>
    api.post(`/api/v1/projects/${id}/episodes/${eid}/extract-storyboards`, {}, {
      ...(skipAssetRefresh ? { headers: { 'X-Autovideo-Skip-Asset-Refresh': 'true' } } : {}),
    }),
  deleteEpisode: (id: number, eid: number) =>
    api.delete(`/api/v1/projects/${id}/episodes/${eid}`),
  // Script optimization & AI review
  optimizeEpisode: (id: number, eid: number) =>
    api.post(`/api/v1/projects/${id}/episodes/${eid}/optimize`),
  applyOptimizedText: (id: number, eid: number) =>
    api.post(`/api/v1/projects/${id}/episodes/${eid}/apply-optimized`),
  reviewEpisode: (id: number, eid: number) =>
    api.post(`/api/v1/projects/${id}/episodes/${eid}/review`),
  autoOptimizeReview: (id: number, eid: number) =>
    api.post(`/api/v1/projects/${id}/episodes/${eid}/auto-optimize-review`),
  batchOptimize: (id: number) =>
    api.post(`/api/v1/projects/${id}/episodes/batch-optimize`),
  batchReview: (id: number) =>
    api.post(`/api/v1/projects/${id}/episodes/batch-review`),
  // Script versions
  getScriptVersions: (id: number) => api.get(`/api/v1/projects/${id}/script/versions`),
  switchScriptVersion: (id: number, versionId: number) =>
    api.post(`/api/v1/projects/${id}/script/switch/${versionId}`),
  uploadScript: (id: number, file: File) => {
    const form = new FormData()
    form.append('file', file)
    return api.post(`/api/v1/projects/${id}/script`, form, {
      headers: { 'Content-Type': 'multipart/form-data' },
    })
  },
  getProgress: (id: number) => api.get(`/api/v1/projects/${id}/progress`),
}

export const taskAPI = {
  list: (params?: {
    status?: string
    type?: string
    project_id?: number
    page?: number
    page_size?: number
  }) => api.get('/api/v1/tasks', { params }),
  get: (id: number) => api.get(`/api/v1/tasks/${id}`),
  cancel: (id: number) => api.post(`/api/v1/tasks/${id}/cancel`),
  create: (data: { task_type: string; payload: object; priority?: number }) =>
    api.post('/api/v1/tasks', data),
}

export const chatAPI = {
  send: (messages: { role: string; content: string }[], modelName?: string) =>
    api.post('/api/v1/chat', { messages, model_name: modelName }),
  sendGemini: (messages: { role: string; content: string }[]) =>
    api.post('/api/v1/chat/gemini', { messages }),
}

export const modelAPI = {
  list: (params?: { type?: string; provider?: string; speed_rating?: string; enabled?: string; sort_by?: string }) =>
    api.get('/api/v1/models', { params }),
  get: (id: number) => api.get(`/api/v1/models/${id}`),
  create: (data: Partial<Model>) => api.post('/api/v1/models', data),
  update: (id: number, data: Partial<Model>) => api.put(`/api/v1/models/${id}`, data),
  delete: (id: number) => api.delete(`/api/v1/models/${id}`),
  toggle: (id: number) => api.patch(`/api/v1/models/${id}/toggle`),
  setDefault: (id: number) => api.patch(`/api/v1/models/${id}/default`),
  test: (id: number) => api.post(`/api/v1/models/${id}/test`),
  health: () => api.get('/api/v1/models/health'),
  imageCapabilities: () =>
    api.get<ImageModelCapabilitiesResponse>('/api/v1/images/model-capabilities'),
  geminiChannels: () =>
    api.get<GeminiChannelStatus[]>('/api/v1/images/gemini-channels'),
}

export interface GeminiChannelStatus {
  index: number
  base: string
  key_mask: string
  valid: boolean
  error?: string
}

export const pipelineAPI = {
  start: (projectId: number, scriptId: number, config: object) =>
    api.post('/api/v1/pipeline/start', { project_id: projectId, script_id: scriptId, config }),
  status: (pipelineId: string) => api.get(`/api/v1/pipeline/${pipelineId}/status`),
  pause: (pipelineId: string) => api.post(`/api/v1/pipeline/${pipelineId}/pause`),
  abort: (pipelineId: string) => api.post(`/api/v1/pipeline/${pipelineId}/abort`),
}

export interface ScriptRecord {
  id: number
  project_id: number
  title: string
  version: number
  parse_status: string
  created_at: string
  updated_at: string
}

export const scriptAPI = {
  listByProject: (projectId: number, params?: { page?: number; page_size?: number }) =>
    scriptApi.get('/api/v1/scripts', {
      params: { project_id: projectId, page: 1, page_size: 20, ...params },
    }) as Promise<{ data: { items: ScriptRecord[]; total: number; page: number; page_size: number } }>,
}

export const sceneAPI = {
  list: (scriptId: number) => api.get(`/api/v1/scripts/${scriptId}/scenes`),
  update: (scriptId: number, sceneId: number, data: { prompt_draft?: string; scene_order?: number; description?: string; status?: string }) =>
    api.put(`/api/v1/scripts/${scriptId}/scenes/${sceneId}`, data),
  reorder: (scriptId: number, orders: { id: number; scene_order: number }[]) =>
    api.post(`/api/v1/scripts/${scriptId}/scenes/reorder`, { orders }),
  generateImage: (id: number) => api.post(`/api/v1/scenes/${id}/generate-image`),
  batchGenerate: (ids: number[]) =>
    api.post('/api/v1/scenes/batch-generate', { scene_ids: ids }),
}

export const assetAPI = {
  list: (projectId: number, params?: { type?: string; episode_id?: number; keyword?: string }) =>
    api.get(`/api/v1/projects/${projectId}/assets`, { params }),
  listPaginated: (projectId: number, params?: { type?: string; status?: string; episode_id?: number; keyword?: string; page?: number; page_size?: number }) =>
    api.get(`/api/v1/projects/${projectId}/assets`, { params: { page: 1, page_size: 50, ...params } }),
  get: (projectId: number, id: number) =>
    api.get(`/api/v1/projects/${projectId}/assets/${id}`),
  create: (projectId: number, data: Partial<Asset>) =>
    api.post(`/api/v1/projects/${projectId}/assets`, data),
  update: (projectId: number, id: number, data: Partial<Asset>) =>
    api.patch(`/api/v1/projects/${projectId}/assets/${id}`, data),
  delete: (projectId: number, id: number) =>
    api.delete(`/api/v1/projects/${projectId}/assets/${id}`),
  deleteAll: (projectId: number) =>
    api.delete(`/api/v1/projects/${projectId}/assets`),
  extract: (projectId: number, force?: boolean) =>
    api.post(`/api/v1/projects/${projectId}/assets/extract${force ? '?force=true' : ''}`, {}, { timeout: 180000 }),
  extractEpisode: (projectId: number, episodeId: number) =>
    api.post(`/api/v1/projects/${projectId}/assets/extract-episode/${episodeId}`, {}, { timeout: 180000 }),
  generateBatch: (
    projectId: number,
    assetIds: number[],
    opts?: { modelName?: string; modelNames?: string[]; promptSuffix?: string; promptSuffixes?: Record<string, string>; force?: boolean },
  ) => {
    const body: Record<string, unknown> = { asset_ids: assetIds }
    if (opts?.modelName) body.model_name = opts.modelName
    if (opts?.modelNames?.length) body.model_names = opts.modelNames
    if (opts?.promptSuffix) body.prompt_suffix = opts.promptSuffix
    if (opts?.promptSuffixes && Object.keys(opts.promptSuffixes).length > 0) body.prompt_suffixes = opts.promptSuffixes
    if (opts?.force) body.force = true
    return api.post(`/api/v1/projects/${projectId}/assets/generate-batch`, body)
  },
  generate: (projectId: number, id: number, modelName?: string, promptSuffix?: string, stylePreset?: string) => {
    const body: Record<string, string> = {}
    if (modelName) body.model_name = modelName
    if (promptSuffix) body.prompt_suffix = promptSuffix
    if (stylePreset) body.style_preset = stylePreset
    return api.post(`/api/v1/projects/${projectId}/assets/${id}/generate`, body)
  },
  retry: (projectId: number, id: number, modelName?: string) =>
    api.post(`/api/v1/projects/${projectId}/assets/${id}/retry`, modelName ? { model_name: modelName } : {}),
  regenPanel: (projectId: number, id: number, panel: 'closeup' | 'front' | 'side' | 'back', opts?: { promptOverride?: string; modelName?: string }) =>
    api.post(`/api/v1/projects/${projectId}/assets/${id}/regen-panel`, {
      panel,
      ...(opts?.promptOverride ? { prompt_override: opts.promptOverride } : {}),
      ...(opts?.modelName ? { model_name: opts.modelName } : {}),
    }),
  reset: (projectId: number, id: number) =>
    api.post(`/api/v1/projects/${projectId}/assets/${id}/reset`),
  retryFailed: (projectId: number, episodeId?: number, modelName?: string) =>
    api.post(`/api/v1/projects/${projectId}/assets/retry-failed${episodeId ? `?episode_id=${episodeId}` : ''}`, modelName ? { model_name: modelName } : {}),
  generateAll: (
    projectId: number,
    episodeId?: number,
    modelName?: string,
    promptSuffix?: string,
    force?: boolean,
    opts?: { modelNames?: string[]; promptSuffixes?: Record<string, string> },
  ) => {
    const body: Record<string, unknown> = {}
    if (modelName) body.model_name = modelName
    if (opts?.modelNames?.length) body.model_names = opts.modelNames
    if (promptSuffix) body.prompt_suffix = promptSuffix
    if (opts?.promptSuffixes && Object.keys(opts.promptSuffixes).length > 0) body.prompt_suffixes = opts.promptSuffixes
    if (force) body.force = true
    return api.post(`/api/v1/projects/${projectId}/assets/generate-all${episodeId ? `?episode_id=${episodeId}` : ''}`, body)
  },
  pauseGeneration: (projectId: number) =>
    api.post(`/api/v1/projects/${projectId}/assets/pause-generation`),
  resumeGeneration: (projectId: number) =>
    api.post(`/api/v1/projects/${projectId}/assets/resume-generation`),
  backfillEpisodes: (projectId: number) =>
    api.post(`/api/v1/projects/${projectId}/assets/backfill-episodes`),
  autoMatchVoices: (projectId: number) =>
    api.post(`/api/v1/projects/${projectId}/assets/auto-match-voices`),
  upload: (projectId: number, id: number, file: File) => {
    const form = new FormData()
    form.append('file', file)
    return api.post(`/api/v1/projects/${projectId}/assets/${id}/upload`, form, {
      headers: { 'Content-Type': 'multipart/form-data' },
    })
  },
  chat: (projectId: number, id: number, message: string, modelName?: string, skillContext?: string) =>
    api.post(`/api/v1/projects/${projectId}/assets/${id}/chat`, {
      role: 'user',
      content: message,
      ...(modelName ? { model_name: modelName } : {}),
      ...(skillContext ? { skill_context: skillContext } : {}),
    }),
  updateConsistency: (projectId: number, config: { consistency_strength: number }) =>
    api.patch(`/api/v1/projects/${projectId}/consistency-config`, config),
  modelStatus: () =>
    api.get<{ models: { key: string; available: boolean }[] }>('/api/v1/images/model-status'),
}

export const storyboardAPI = {
  list: (projectId: number, params?: { episode_id?: number; status?: string; keyword?: string; page?: number; page_size?: number; include_versions?: boolean }) =>
    api.get(`/api/v1/projects/${projectId}/storyboards`, { params }),
  listAll: async (projectId: number, params?: { episode_id?: number; status?: string; keyword?: string; include_versions?: boolean }) => {
    const pageSize = params?.include_versions ? 100 : 500
    let page = 1
    let total = Number.POSITIVE_INFINITY
    const items: Storyboard[] = []

    while (items.length < total) {
      const res = await api.get(`/api/v1/projects/${projectId}/storyboards`, {
        params: { ...params, page, page_size: pageSize },
      }) as {
        data?: Storyboard[] | { items?: Storyboard[] }
        page_info?: { total?: number }
      }

      const pageItems = Array.isArray(res.data) ? res.data : (res.data?.items ?? [])
      items.push(...pageItems)
      total = res.page_info?.total ?? pageItems.length

      if (pageItems.length === 0 || items.length >= total) {
        break
      }

      page += 1
    }

    return { data: items }
  },
  get: (projectId: number, id: number) =>
    api.get(`/api/v1/projects/${projectId}/storyboards/${id}`),
  stats: (projectId: number) =>
    api.get(`/api/v1/projects/${projectId}/storyboards/stats`),
  episodeStats: (projectId: number) =>
    api.get(`/api/v1/projects/${projectId}/storyboards/episode-stats`),
  create: (projectId: number, data: Partial<Storyboard>) =>
    api.post(`/api/v1/projects/${projectId}/storyboards`, data),
  update: (projectId: number, id: number, data: Partial<Storyboard>) =>
    api.patch(`/api/v1/projects/${projectId}/storyboards/${id}`, data),
  generate: (projectId: number, id: number, modelName?: string) =>
    api.post(`/api/v1/projects/${projectId}/storyboards/${id}/generate`, modelName ? { model_name: modelName } : {}),
  retry: (projectId: number, id: number, modelName?: string) =>
    api.post(`/api/v1/projects/${projectId}/storyboards/${id}/retry`, modelName ? { model_name: modelName } : {}),
  retryFailed: (projectId: number, modelName?: string, episodeId?: number, options?: { modelNames?: string[] }) =>
    api.post(`/api/v1/projects/${projectId}/storyboards/retry-failed`, {
      ...(modelName ? { model_name: modelName } : {}),
      ...(options?.modelNames?.length ? { model_names: options.modelNames } : {}),
      ...(episodeId ? { episode_id: episodeId } : {}),
    }),
  generateAll: (projectId: number, episodeId?: number, modelName?: string, force?: boolean, options?: { modelNames?: string[] }) =>
    api.post(`/api/v1/projects/${projectId}/storyboards/generate-all`, {
      ...(episodeId !== undefined ? { episode_id: episodeId } : {}),
      ...(modelName ? { model_name: modelName } : {}),
      ...(options?.modelNames?.length ? { model_names: options.modelNames } : {}),
      ...(force ? { force: true } : {}),
    }),
  auditContinuity: (projectId: number, episodeId?: number) =>
    api.post(`/api/v1/projects/${projectId}/storyboards/audit-continuity`, {
      ...(episodeId !== undefined ? { episode_id: episodeId } : {}),
    }),
  pauseGeneration: (projectId: number, episodeId?: number) =>
    api.post(`/api/v1/projects/${projectId}/storyboards/pause-generation`, episodeId ? { episode_id: episodeId } : {}),
  resumeGeneration: (projectId: number, episodeId?: number) =>
    api.post(`/api/v1/projects/${projectId}/storyboards/resume-generation`, episodeId ? { episode_id: episodeId } : {}),
  switchVersion: (projectId: number, id: number, versionId: number) =>
    api.post(`/api/v1/projects/${projectId}/storyboards/${id}/switch-version`, { version_id: versionId }),
  deleteVersion: (projectId: number, id: number, vid: number) =>
    api.delete(`/api/v1/projects/${projectId}/storyboards/${id}/versions/${vid}`),
  delete: (projectId: number, id: number) =>
    api.delete(`/api/v1/projects/${projectId}/storyboards/${id}`),
  void: (projectId: number, id: number) =>
    api.post(`/api/v1/projects/${projectId}/storyboards/${id}/void`),
  chat: (projectId: number, id: number, message: string) =>
    api.post(`/api/v1/projects/${projectId}/storyboards/${id}/chat`, { role: 'user', content: message }),
  updateConfig: (projectId: number, config: Partial<StoryboardConfig>) =>
    api.patch(`/api/v1/projects/${projectId}/storyboards/config`, config),
  exportURL: (projectId: number) =>
    `/api/v1/projects/${projectId}/storyboards/export`,
}

export const storageAPI = {
  getDetails: (projectId: number) =>
    api.get(`/api/v1/projects/${projectId}/storage`),
  upload: (
    projectId: number,
    file: File,
    opts?: {
      bucket?: string
      category?: string
      userId?: number
      onProgress?: (percent: number) => void
    }
  ) => {
    const form = new FormData()
    form.append('file', file)
    form.append('bucket', opts?.bucket || 'videos')
    form.append('category', opts?.category || 'other')
    form.append('project_id', String(projectId))
    form.append('user_id', String(opts?.userId || getCurrentUserIdFromToken()))
    return api.post<{
      code: number
      data: {
        cdn_url: string
        object_key: string
        file_size: number
        file_id: number
      }
    }>('/api/v1/storage/upload', form, {
      headers: { 'Content-Type': 'multipart/form-data' },
      timeout: 180000,
      onUploadProgress: (evt) => {
        const total = Number(evt.total || 0)
        if (!opts?.onProgress || total <= 0) return
        const percent = Math.max(0, Math.min(100, Math.round((evt.loaded / total) * 100)))
        opts.onProgress(percent)
      },
    })
  },
  deleteFiles: (projectId: number, fileIds: number[]) =>
    api.delete(`/api/v1/projects/${projectId}/storage/files`, { data: { file_ids: fileIds } }),
  cleanHistory: (projectId: number) =>
    api.post(`/api/v1/projects/${projectId}/storage/clean-history`),
  getBulkTotals: (projectIds: number[]) =>
    api.get<{ data: Record<string, number> }>(`/api/v1/storage/projects/totals`, {
      params: { ids: projectIds.join(',') },
    }),
}

export const videoAPI = {
  list: (projectId: number, params?: { episode_id?: number; page?: number; page_size?: number }) =>
    api.get(`/api/v1/projects/${projectId}/videos`, { params }),
  stats: (projectId: number) =>
    api.get(`/api/v1/projects/${projectId}/videos/stats`),
  generate: (projectId: number, data: VideoGenerateRequest) =>
    api.post(`/api/v1/projects/${projectId}/videos/generate`, data),
  generateBatch: (projectId: number, data: VideoGenerateBatchRequest) =>
    api.post(`/api/v1/projects/${projectId}/videos/generate-batch`, data),
  retry: (projectId: number, videoId: number, modelName?: string) =>
    api.post(`/api/v1/projects/${projectId}/videos/${videoId}/retry`, { model_name: modelName }),
  retryClip: (projectId: number, videoId: number, clipId: number, modelName?: string) =>
    api.post(`/api/v1/projects/${projectId}/videos/${videoId}/clips/${clipId}/retry`, { model_name: modelName }),
  retryFailed: (projectId: number, modelName?: string) =>
    api.post(`/api/v1/projects/${projectId}/videos/retry-failed`, { model_name: modelName }),
  pause: (projectId: number, videoId: number) =>
    api.post(`/api/v1/projects/${projectId}/videos/${videoId}/pause`),
  resume: (projectId: number, videoId: number) =>
    api.post(`/api/v1/projects/${projectId}/videos/${videoId}/resume`),
  export: (projectId: number, videoId: number) =>
    api.get(`/api/v1/projects/${projectId}/videos/${videoId}/export`),
  listTasks: (projectId: number, params?: { episode_id?: number; page?: number; page_size?: number }) =>
    api.get(`/api/v1/projects/${projectId}/videos`, { params }),
  getTask: (taskId: number) =>
    api.get(`/api/v1/videos/tasks/${taskId}`),
  deleteTask: (taskId: number) =>
    api.delete(`/api/v1/videos/tasks/${taskId}`),
  /** Cancel a pending/processing task (calls DELETE /api/v1/videos/tasks/:id) */
  cancelVideoTask: (taskId: number) =>
    api.delete(`/api/v1/videos/tasks/${taskId}`),
  /** Retry a failed task via POST /api/v1/projects/:pid/videos/:vid/retry */
  retryVideoTask: (projectId: number, taskId: number, modelName?: string) =>
    api.post(`/api/v1/projects/${projectId}/videos/${taskId}/retry`, { model_name: modelName }),
  compose: (taskId: number) =>
    api.post(`/api/v1/videos/tasks/${taskId}/compose`),
  modelStatus: () =>
    api.get<{ models: { key: string; available: boolean; native_audio?: boolean; params?: { key: string; label: string; default: string; values: { value: string; label: string }[] }[] }[] }>('/api/v1/videos/model-status'),
  getShotsMetadata: (projectId: number, episodeId: number) =>
    api.get(`/api/v1/projects/${projectId}/episodes/${episodeId}/videos/shots-metadata`),
  triggerClipPipeline: (projectId: number, episodeId: number, scriptText: string) =>
    api.post(`/api/v1/projects/${projectId}/episodes/${episodeId}/videos/clip-trigger`, { script_text: scriptText }),
  extractContent: (projectId: number, payload: { video_url: string; language?: string; only_audio?: boolean }) =>
    api.post<{
      code: number
      data: {
        video_url: string
        frame_url: string
        language: string
        narration_text: string
        extracted_text: string
        summary: string
        audio_enabled: boolean
        vision_enabled: boolean
        vision_model: string
      }
    }>(
      process.env.NODE_ENV === 'production'
        ? `/api/v1/projects/${projectId}/videos/content-extract`
        : `${getDirectGatewayBaseURL()}/api/v1/projects/${projectId}/videos/content-extract`,
      payload,
      { timeout: 300000 },
    ),
}

export interface VideoRenderConfig extends Record<string, unknown> {
  config_version?: number
}

export interface VideoEpisodeGenerateRequest {
  episode_id: number
  image_urls: string[]
  scene_descriptions?: string[]
  dialogues?: string[]
  durations?: number[]
  camera_movements?: string[]
  moods?: string[]
  scene_characters?: string[][]
  scene_asset_ids?: number[][]
  audio_url?: string
  scene_description?: string
  scene_group_keys?: string[]
}

export interface VideoGenerateRequest {
  episode_id?: number
  image_urls: string[]
  scene_descriptions?: string[]
  dialogues?: string[]
  durations?: number[]
  camera_movements?: string[]
  moods?: string[]
  scene_characters?: string[][]
  scene_asset_ids?: number[][]
  style_preset?: string
  motion_mode?: string
  model_name?: string
  video_mode?: string
  export_format?: string
  audio_url?: string
  subtitle_text?: string
  scene_description?: string
  render_config?: VideoRenderConfig
  clip_duration_sec?: number
  serial_scene?: boolean
  scene_group_keys?: string[]
}

export interface VideoGenerateBatchRequest {
  episodes: VideoEpisodeGenerateRequest[]
  style_preset?: string
  motion_mode?: string
  model_name?: string
  video_mode?: string
  export_format?: string
  render_config?: VideoRenderConfig
  clip_duration_sec?: number
  serial_scene?: boolean
}

export interface DubbingTask {
  id: number
  project_id: number
  episode_id: number
  storyboard_id?: number | null
  user_id: number
  task_type: 'dubbing' | 'subtitle'
  voice_model: string
  voice_rate: string
  voice_pitch: string
  voice_volume: string
  status: 'pending' | 'processing' | 'succeeded' | 'failed'
  chunks_done: number
  chunks_total: number
  audio_url: string
  subtitle_url: string
  duration_sec: number
  error_msg: string
  created_at: string
  updated_at: string
}

export const dubbingAPI = {
  generate: (
    projectId: number,
    episodeId: number,
    text: string,
    voiceModel?: string,
    options?: { voice_rate?: string; voice_pitch?: string; voice_volume?: string }
  ) =>
    api.post(`/api/v1/projects/${projectId}/dubbing/generate`, {
      episode_id: episodeId,
      text,
      voice_model: voiceModel || 'default',
      voice_rate: options?.voice_rate || '+0%',
      voice_pitch: options?.voice_pitch || '+0Hz',
      voice_volume: options?.voice_volume || '+0%',
    }),
  generateSubtitle: (
    projectId: number,
    episodeId: number,
    text: string,
    options?: { voice_model?: string; voice_rate?: string; voice_pitch?: string; voice_volume?: string }
  ) =>
    api.post(`/api/v1/projects/${projectId}/subtitle/generate`, {
      episode_id: episodeId,
      text,
      voice_model: options?.voice_model || 'default',
      voice_rate: options?.voice_rate || '+0%',
      voice_pitch: options?.voice_pitch || '+0Hz',
      voice_volume: options?.voice_volume || '+0%',
    }),
  generateBatch: (
    projectId: number,
    items: {
      episode_id: number
      text: string
      voice_model?: string
      voice_rate?: string
      voice_pitch?: string
      voice_volume?: string
    }[]
  ) =>
    api.post(`/api/v1/projects/${projectId}/dubbing/generate-batch`, { items }),
  generateSubtitleBatch: (
    projectId: number,
    items: {
      episode_id: number
      text?: string
      audio_url?: string
      voice_model?: string
      voice_rate?: string
      voice_pitch?: string
      voice_volume?: string
    }[]
  ) =>
    api.post(`/api/v1/projects/${projectId}/subtitle/generate-batch`, { items }),
  listTasks: (projectId: number) =>
    api.get<{ code: number; data: DubbingTask[] }>(`/api/v1/projects/${projectId}/dubbing/tasks`),
  getTask: (projectId: number, taskId: number) =>
    api.get<{ code: number; data: DubbingTask }>(`/api/v1/projects/${projectId}/dubbing/tasks/${taskId}`),
  retryTask: (projectId: number, taskId: number, text?: string) =>
    api.post(`/api/v1/projects/${projectId}/dubbing/tasks/${taskId}/retry`, text ? { text } : {}),
  retryTasksBatch: (
    projectId: number,
    items: { task_id: number; text?: string }[]
  ) =>
    api.post(`/api/v1/projects/${projectId}/dubbing/tasks/retry-batch`, { items }),
  listVoices: () =>
    api.get<{ code: number; data: { voices: { key: string; name: string; label: string }[] } }>('/api/v1/voices'),
  generateForStoryboard: (
    projectId: number,
    storyboardId: number,
    episodeId: number,
    text: string,
    voiceModel?: string,
    options?: { voice_rate?: string; voice_pitch?: string; voice_volume?: string }
  ) =>
    api.post(`/api/v1/projects/${projectId}/storyboards/${storyboardId}/dubbing`, {
      episode_id: episodeId,
      text,
      voice_model: voiceModel || 'default',
      voice_rate: options?.voice_rate || '+0%',
      voice_pitch: options?.voice_pitch || '+0Hz',
      voice_volume: options?.voice_volume || '+0%',
    }),
  listStoryboardTasks: (projectId: number) =>
    api.get<{ code: number; data: DubbingTask[] }>(`/api/v1/projects/${projectId}/dubbing/storyboard-tasks`),
}

export const scriptLibraryAPI = {
  list: (source?: string) =>
    scriptApi.get('/api/v1/script-library', { params: source ? { source } : {} }),
  create: (data: {
    title: string
    author?: string
    description?: string
    genre?: string
    tags?: string[]
    source?: string
    cover_url?: string
    file_url?: string
    file_size?: number
    word_count?: number
    project_id?: number
    is_public?: boolean
  }) => scriptApi.post('/api/v1/script-library', data),
  generateAI: (data: {
    mode: 'script' | 'novel_outline' | 'novel_chapter' | 'adaptation' | 'episode_outline' | 'scene_script' | 'dialogue_polish'
    model_name?: string
    title?: string
    genre?: string
    platform?: string
    delivery_format?: string
    episode_duration?: string
    reference_style?: string
    premise?: string
    character_setup?: string
    world_setting?: string
    outline?: string
    chapter_brief?: string
    source_text?: string
    target_words?: number
    chapter_count?: number
    audience?: string
    tone?: string
    requirements?: string
  }) => scriptApi.post('/api/v1/script-library/generate', data),
  update: (id: number, data: Record<string, unknown>) =>
    scriptApi.put(`/api/v1/script-library/${id}`, data),
  delete: (id: number) => scriptApi.delete(`/api/v1/script-library/${id}`),
}

export const characterAPI = {
  list: (projectId: number) =>
    api.get('/api/v1/characters', { params: { project_id: projectId } }),
  create: (projectId: number, data: Omit<Partial<CharacterData>, 'id' | 'project_id' | 'created_at'>) =>
    api.post('/api/v1/characters', { project_id: projectId, ...data }),
  update: (projectId: number, id: number, data: Omit<Partial<CharacterData>, 'id' | 'project_id' | 'created_at'>) =>
    api.put(`/api/v1/characters/${id}`, { project_id: projectId, ...data }),
  delete: (_projectId: number, id: number) =>
    api.delete(`/api/v1/characters/${id}`),
}

export const characterGroupAPI = {
  list: (projectId: number) =>
    api.get(`/api/v1/projects/${projectId}/character-groups`),
  create: (projectId: number, data: { name: string; description?: string; sort_order?: number }) =>
    api.post(`/api/v1/projects/${projectId}/character-groups`, data),
  update: (projectId: number, groupId: number, data: { name?: string; description?: string; sort_order?: number }) =>
    api.put(`/api/v1/projects/${projectId}/character-groups/${groupId}`, data),
  delete: (projectId: number, groupId: number) =>
    api.delete(`/api/v1/projects/${projectId}/character-groups/${groupId}`),
  assignVariant: (projectId: number, groupId: number, assetId: number, variantName?: string) =>
    api.post(`/api/v1/projects/${projectId}/character-groups/${groupId}/variants`, { asset_id: assetId, variant_name: variantName ?? '' }),
  removeVariant: (projectId: number, groupId: number, assetId: number) =>
    api.delete(`/api/v1/projects/${projectId}/character-groups/${groupId}/variants/${assetId}`),
}

export const skillAPI = {
  list: (projectId: number, skillType?: string, useCase?: string) =>
    api.get('/api/v1/skills', { params: { project_id: projectId, skill_type: skillType, use_case: useCase } }),
  listByCharacter: (characterId: number) =>
    api.get(`/api/v1/characters/${characterId}/skills`),
  create: (data: { project_id: number; name: string; skill_type?: string; use_case?: string; description?: string; character_id?: number; is_active?: boolean }) =>
    api.post('/api/v1/skills', data),
  update: (id: number, data: Partial<{ name: string; skill_type: string; use_case: string; description: string; character_id: number | null; is_active: boolean }>) =>
    api.put(`/api/v1/skills/${id}`, data),
  delete: (id: number) => api.delete(`/api/v1/skills/${id}`),
  reseed: (projectId: number) => api.post(`/api/v1/skills/reseed?project_id=${projectId}`),
}

export const promptTemplateAPI = {
  list: (params?: { style_key?: string; resource_type?: string; model_binding?: string; active_only?: boolean }) =>
    api.get('/api/v1/prompt-templates', { params }),
  get: (id: number) => api.get(`/api/v1/prompt-templates/${id}`),
  create: (data: { name: string; style_key: string; content: string; description?: string; resource_type?: string; model_binding?: string; version?: string; sort_order?: number; is_active?: boolean }) =>
    api.post('/api/v1/prompt-templates', data),
  update: (id: number, data: Partial<{ name: string; style_key: string; description: string; content: string; resource_type: string; model_binding: string; version: string; sort_order: number; is_active: boolean }>) =>
    api.put(`/api/v1/prompt-templates/${id}`, data),
  delete: (id: number) => api.delete(`/api/v1/prompt-templates/${id}`),
  preview: (id: number, variables: Record<string, string>) =>
    api.post(`/api/v1/prompt-templates/${id}/preview`, { variables }),
  reseedDefaults: () => api.post('/api/v1/prompt-templates/reseed'),
}

export const utilsAPI = {
  translatePrompt: (text: string) =>
    api.post<{ translated: string }>('/api/v1/utils/translate', { text }),
}

export interface ProductionSkill {
  id: number
  project_id: number
  department: string
  name: string
  label_tag: string
  system_prompt: string
  is_active: boolean
  sort_order: number
  created_at: string
  updated_at: string
}

export const productionSkillAPI = {
  list: (projectId: number) =>
    api.get<{ items: ProductionSkill[]; total: number }>(`/api/v1/projects/${projectId}/production-skills`),
  create: (projectId: number, data: { department: string; name: string; label_tag: string; system_prompt?: string; is_active?: boolean; sort_order?: number }) =>
    api.post<ProductionSkill>(`/api/v1/projects/${projectId}/production-skills`, data),
  update: (projectId: number, id: number, data: Partial<{ name: string; label_tag: string; system_prompt: string; is_active: boolean; sort_order: number }>) =>
    api.put<ProductionSkill>(`/api/v1/projects/${projectId}/production-skills/${id}`, data),
  delete: (projectId: number, id: number) =>
    api.delete(`/api/v1/projects/${projectId}/production-skills/${id}`),
  seedDefaults: (projectId: number) =>
    api.post<{ items: ProductionSkill[]; seeded: boolean }>(`/api/v1/projects/${projectId}/production-skills/seed-defaults`),
  reseedDefaults: (projectId: number) =>
    api.post<{ items: ProductionSkill[]; reseeded: boolean }>(`/api/v1/projects/${projectId}/production-skills/reseed-defaults`),
}

export default api
