'use client'

import { useState, useRef, useEffect, useMemo } from 'react'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { Loader2, RefreshCw, Upload, Trash2, Clock, CheckCircle2, Film } from 'lucide-react'
import { useToast } from '@/components/ui/toast'
import { storageAPI, videoAPI, projectAPI } from '@/lib/api'
import { useAuthStore } from '@/lib/store/auth'
function isHttpUrl(url: string) { try { new URL(url); return true; } catch { return false; } }

function isSupportedVideoFile(file: File) {
  const t = file.type.toLowerCase()
  if (t.startsWith('video/')) return true
  const n = file.name.toLowerCase()
  return n.endsWith('.mp4') || n.endsWith('.mov') || n.endsWith('.webm') || n.endsWith('.mkv') || n.endsWith('.avi') || n.endsWith('.m4v')
}

type ExtractRecord = {
  id: string
  timestamp: number
  videoUrl: string
  fileName?: string
  status: 'processing' | 'success' | 'failed'
  progress?: number
  extractedNarration?: string
  extractedVisualText?: string
  summary?: string
  frameUrl?: string
  frameUrls?: string[]
  frameItems?: Array<{
    frameUrl: string
    extractedText: string
    summary: string
  }>
  error?: string
  language?: string
}

const HISTORY_KEY = 'video-tools-extract-history'
const STALE_PROCESSING_MS = 10 * 60 * 1000
const ACTIVE_EXTRACT_KEY = 'video-tools-active-extract'

function recoverStaleHistory(records: ExtractRecord[]) {
  const now = Date.now()
  let changed = false

  const next = records.map((record) => {
    if (record.status !== 'processing') {
      return record
    }

    const age = now - record.timestamp
    if (age < STALE_PROCESSING_MS) {
      return record
    }

    changed = true
    return {
      ...record,
      status: 'failed',
      error: record.error || '上次提取已超时或被刷新中断，请重新提交',
    }
  })

  return { next, changed }
}

export default function VideoExtractToolPage() {
  const { toast } = useToast()
  const [extractVideoUrl, setExtractVideoUrl] = useState('')
  const [extractVideoFile, setExtractVideoFile] = useState<File | null>(null)
  const extractVideoFileInputRef = useRef<HTMLInputElement>(null)

  const [extractLanguage, setExtractLanguage] = useState('auto')
  const [extractOnlyAudio, setExtractOnlyAudio] = useState(false)

  const [extracting, setExtracting] = useState(false)
  const [extractUploading, setExtractUploading] = useState(false)
  const [extractUploadProgress, setExtractUploadProgress] = useState(0)
  const activeRecordIdRef = useRef<string | null>(null)

  // 历史记录
  const [history, setHistory] = useState<ExtractRecord[]>([])

  useEffect(() => {
    try {
      const saved = localStorage.getItem(HISTORY_KEY)
      if (!saved) {
        return
      }

      const parsed = JSON.parse(saved)
      if (!Array.isArray(parsed)) {
        return
      }

      const typed = parsed.filter((item): item is ExtractRecord => Boolean(item && typeof item === 'object' && typeof item.id === 'string'))
      const activeRecordId = localStorage.getItem(ACTIVE_EXTRACT_KEY)
      const afterActiveRecovery = activeRecordId
        ? typed.map((record) => {
            if (record.id !== activeRecordId || record.status !== 'processing') {
              return record
            }
            return {
              ...record,
              status: 'failed',
              error: record.error || '页面刷新或关闭导致提取中断，请重新提交',
            }
          })
        : typed

      const { next, changed } = recoverStaleHistory(afterActiveRecovery)
      setHistory(next)
      if (changed) {
        localStorage.setItem(HISTORY_KEY, JSON.stringify(next))
        toast({ title: '已回收超时的提取记录', variant: 'default' })
      }
      if (activeRecordId) {
        localStorage.removeItem(ACTIVE_EXTRACT_KEY)
      }
    } catch {
      // ignore
    }
  }, [toast])

  useEffect(() => {
    const handlePageHide = () => {
      const activeRecordId = activeRecordIdRef.current || localStorage.getItem(ACTIVE_EXTRACT_KEY)
      if (!activeRecordId) {
        return
      }

      try {
        const saved = localStorage.getItem(HISTORY_KEY)
        if (!saved) return
        const parsed = JSON.parse(saved)
        if (!Array.isArray(parsed)) return

        const next = parsed.map((item) => {
          if (!item || typeof item !== 'object' || item.id !== activeRecordId) {
            return item
          }
          if ((item as ExtractRecord).status !== 'processing') {
            return item
          }
          return {
            ...(item as ExtractRecord),
            status: 'failed',
            error: (item as ExtractRecord).error || '页面刷新或关闭导致提取中断，请重新提交',
          }
        })

        localStorage.setItem(HISTORY_KEY, JSON.stringify(next))
      } catch {
        // ignore
      } finally {
        localStorage.removeItem(ACTIVE_EXTRACT_KEY)
      }
    }

    window.addEventListener('pagehide', handlePageHide)
    window.addEventListener('beforeunload', handlePageHide)
    return () => {
      window.removeEventListener('pagehide', handlePageHide)
      window.removeEventListener('beforeunload', handlePageHide)
    }
  }, [])

  const saveHistory = (newHistory: ExtractRecord[]) => {
    setHistory(newHistory)
    localStorage.setItem(HISTORY_KEY, JSON.stringify(newHistory))
  }

  const ensureProjectId = async (): Promise<number | null> => {
    try {
      const createRes = (await projectAPI.create({
        title: `视频内容提取-${new Date().toISOString().slice(0, 16).replace('T', ' ')}`,
        description: '工具区-视频内容提取专用项目',
        project_type: 'video',
      } as never)) as { data?: { id?: number } }

      const newId = createRes?.data?.id
      if (newId) return newId
      
      toast({ title: '无法创建临时项目，未能获取项目 ID', variant: 'destructive' })
      return null
    } catch (err) {
      console.error('Failed to create fallback project', err)
      toast({ title: '自动创建项目失败，请稍后重试', variant: 'destructive' })
      return null
    }
  }

  const handleExtract = async () => {
    const typedUrl = extractVideoUrl.trim()
    const localFile = extractVideoFile
    let videoUrl = typedUrl

    if (!videoUrl && !localFile) {
      toast({ title: '请先输入视频 URL，或选择一个本地视频文件', variant: 'destructive' })
      return
    }

    if (!videoUrl && localFile) {
      if (!isSupportedVideoFile(localFile)) {
        toast({ title: '仅支持常见视频文件（mp4/mov/webm/mkv/avi）', variant: 'destructive' })
        return
      }
      if (localFile.size > 1024 * 1024 * 512) {
        toast({ title: '本地视频过大（建议不超过 512MB）', variant: 'destructive' })
        return
      }
    }

    const projectId = await ensureProjectId()
    if (!projectId) return

    const recordId = Date.now().toString()
    activeRecordIdRef.current = recordId
    localStorage.setItem(ACTIVE_EXTRACT_KEY, recordId)
    const newRecord: ExtractRecord = {
      id: recordId,
      timestamp: Date.now(),
      videoUrl: typedUrl,
      fileName: localFile?.name,
      status: 'processing',
      progress: 0,
    }
    const currentHistory = [newRecord, ...history]
    saveHistory(currentHistory)

    setExtracting(true)
    setExtractUploadProgress(0)

    const updateRecord = (updates: Partial<ExtractRecord>) => {
      setHistory((prev) => {
        const next = prev.map((r) => (r.id === recordId ? { ...r, ...updates } : r))
        localStorage.setItem(HISTORY_KEY, JSON.stringify(next))
        return next
      })
    }

    try {
      if (!videoUrl && localFile) {
        setExtractUploading(true)
        updateRecord({ progress: 1 })
        
        let uploadRes: { data?: { cdn_url?: string } } | null = null
        try {
          uploadRes = await storageAPI.upload(projectId, localFile, {
            bucket: 'videos',
            category: 'tools-video-source',
            onProgress: (percent) => {
              setExtractUploadProgress(percent)
              updateRecord({ progress: percent })
            },
          }) as unknown as { data?: { cdn_url?: string } }
        } catch {
          throw new Error('UPLOAD_FAILED')
        }

        const uploadedUrl = String(uploadRes?.data?.cdn_url || '').trim()
        if (!isHttpUrl(uploadedUrl)) {
          throw new Error('本地视频上传成功，但未获取到可用链接')
        }
        videoUrl = uploadedUrl
        updateRecord({ videoUrl: uploadedUrl })
      }

      setExtractUploading(false)
      const res = await videoAPI.extractContent(projectId, {
        video_url: videoUrl,
        language: extractLanguage === 'auto' ? undefined : extractLanguage,
        only_audio: extractOnlyAudio,
      }) as unknown as {
        data?: {
          video_url?: string
          frame_url?: string
          frame_urls?: string[]
          frame_items?: Array<{
            frame_url?: string
            extracted_text?: string
            summary?: string
          }>
          language?: string
          narration_text?: string
          extracted_text?: string
          summary?: string
        }
      }

      const payload = res?.data ?? {}
      const frameUrls = Array.isArray(payload.frame_urls)
        ? payload.frame_urls.map((url) => String(url || '').trim()).filter(Boolean)
        : []
      const normalizedFrameItems = Array.isArray(payload.frame_items) && payload.frame_items.length > 0
        ? payload.frame_items.map((item, index) => ({
            frameUrl: String(item?.frame_url || frameUrls[index] || payload.frame_url || '').trim(),
            extractedText: String(item?.extracted_text || '').trim(),
            summary: String(item?.summary || '').trim(),
          })).filter((item) => item.frameUrl)
        : (payload.frame_url || frameUrls[0]
            ? [{
                frameUrl: String(payload.frame_url || frameUrls[0] || '').trim(),
                extractedText: String(payload.extracted_text || '').trim(),
                summary: String(payload.summary || '').trim(),
              }]
            : [])
      updateRecord({
        status: 'success',
        extractedNarration: String(payload.narration_text || '').trim(),
        extractedVisualText: String(payload.extracted_text || '').trim(),
        summary: String(payload.summary || '').trim(),
        frameUrl: String(payload.frame_url || frameUrls[0] || '').trim(),
        frameUrls,
        frameItems: normalizedFrameItems,
      })

      if (localFile) {
        setExtractVideoFile(null)
        if (extractVideoFileInputRef.current) extractVideoFileInputRef.current.value = ''
      }

      activeRecordIdRef.current = null
      localStorage.removeItem(ACTIVE_EXTRACT_KEY)
      
      toast({ title: '视频内容提取完成', variant: 'success' })
    } catch (error: any) {
      console.error(error)
      const errMsg = error.message === 'UPLOAD_FAILED' ? '本地文件上传失败，请稍后重试' : '提取失败，请稍后重试'
      updateRecord({ status: 'failed', error: errMsg })
      activeRecordIdRef.current = null
      localStorage.removeItem(ACTIVE_EXTRACT_KEY)
      toast({ title: errMsg, variant: 'destructive' })
    } finally {
      setExtracting(false)
      setExtractUploading(false)
    }
  }

  const handleCopy = (text: string, label: string) => {
    if (!text) return
    navigator.clipboard.writeText(text)
    toast({ title: `已复制${label}`, variant: 'success' })
  }

  const handleDeleteHistory = (id: string) => {
    const next = history.filter(r => r.id !== id)
    saveHistory(next)
  }

  return (
    <div className="mx-auto max-w-5xl space-y-6 pb-10">
      <div className="overflow-hidden rounded-[28px] border border-surface-200/70 bg-gradient-to-br from-slate-950 via-cyan-950 to-slate-900 p-6 text-white shadow-sm">
        <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
          <div>
            <div className="mb-3 inline-flex items-center gap-2 rounded-full border border-white/10 bg-white/10 px-3 py-1.5 text-xs font-medium text-surface-100 backdrop-blur">
              <Film className="h-3.5 w-3.5 text-cyan-300" />
              视频工具区
            </div>
            <h2 className="text-2xl font-semibold tracking-tight">视频内容提取</h2>
            <p className="mt-2 text-sm leading-6 text-surface-300">
              支持上传本地视频或输入视频链接，提取视频画面中的文字并转写音频解说文案。
            </p>
          </div>
        </div>
      </div>

      <Card className="overflow-hidden rounded-[24px] border-surface-200 shadow-sm">
        <CardContent className="space-y-6 bg-gradient-to-b from-white to-surface-50/60 pt-6 text-surface-900">
          <div className="space-y-3 rounded-xl border border-cyan-200 bg-cyan-50/50 p-5">
            <div className="mb-2">
              <p className="text-sm font-semibold text-cyan-900">新建提取任务</p>
            </div>
            <div className="grid gap-3 md:grid-cols-[1fr_180px_auto]">
              <Input
                placeholder="或者输入外链，例如: https://cdn.example.com/demo.mp4"
                value={extractVideoUrl}
                onChange={(e) => setExtractVideoUrl(e.target.value)}
                disabled={extracting}
              />
              <Select value={extractLanguage} onValueChange={setExtractLanguage} disabled={extracting}>
                <SelectTrigger>
                  <SelectValue placeholder="识别语言" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="zh">中文</SelectItem>
                  <SelectItem value="en">English</SelectItem>
                  <SelectItem value="ja">日本語</SelectItem>
                  <SelectItem value="ko">한국어</SelectItem>
                  <SelectItem value="auto">自动识别</SelectItem>
                </SelectContent>
              </Select>
              <Button
                type="button"
                className="gap-1.5 bg-cyan-600 hover:bg-cyan-700"
                onClick={handleExtract}
                disabled={extracting || extractUploading}
              >
                {extracting ? <Loader2 className="h-4 w-4 animate-spin" /> : <RefreshCw className="h-4 w-4" />}
                {extractUploading ? '上传中...' : '提交提取'}
              </Button>
            </div>
            
            <div className="flex flex-wrap items-center gap-2 mt-2">
              <input
                ref={extractVideoFileInputRef}
                type="file"
                accept="video/*,.mp4,.mov,.m4v,.webm,.mkv,.avi"
                className="hidden"
                onChange={(e) => setExtractVideoFile(e.target.files?.[0] ?? null)}
              />
              <Button
                type="button"
                variant="outline"
                size="sm"
                className="h-8 gap-1.5 border-cyan-300 text-cyan-800 hover:bg-cyan-100"
                onClick={() => extractVideoFileInputRef.current?.click()}
                disabled={extracting || extractUploading}
              >
                <Upload className="h-4 w-4" />
                选择本地视频
              </Button>
              {extractVideoFile ? (
                <span className="text-xs font-medium text-cyan-800">{extractVideoFile.name} ({(extractVideoFile.size / 1024 / 1024).toFixed(1)} MB)</span>
              ) : (
                <span className="text-xs text-surface-500">优先处理本地文件，如果不选则读取左侧 URL</span>
              )}
              {extractVideoFile && !extracting && (
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  className="h-8 px-2 text-xs text-surface-500 hover:text-red-500"
                  onClick={() => {
                    setExtractVideoFile(null)
                    if (extractVideoFileInputRef.current) extractVideoFileInputRef.current.value = ''
                  }}
                >
                  <Trash2 className="h-4 w-4" />
                </Button>
              )}
            </div>

            <div className="mt-4 flex items-center gap-2 text-sm text-cyan-900">
              <Switch checked={extractOnlyAudio} onCheckedChange={setExtractOnlyAudio} disabled={extracting} />
              <label>仅提取音频解说（跳过画面文案识别，速度更快）</label>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* 历史记录区 */}
      <h3 className="text-lg font-semibold text-surface-800 flex items-center gap-2 ml-1">
        <Clock className="w-5 h-5" /> 
        提取进度与历史
      </h3>

      <div className="space-y-4">
        {history.length === 0 ? (
          <div className="rounded-xl border border-dashed border-surface-200 p-8 text-center text-sm text-surface-500">
            暂无提取记录
          </div>
        ) : (
          history.map((record) => (
            <Card key={record.id} className="overflow-hidden">
              <div className="flex items-center justify-between bg-surface-50/50 px-4 py-3 border-b border-surface-100">
                <div className="flex items-center gap-3">
                  <div className="text-xs font-medium text-surface-500">
                    {new Date(record.timestamp).toLocaleString()}
                  </div>
                  <div className="text-sm font-medium text-surface-800 line-clamp-1 max-w-[300px]">
                    {record.fileName || record.videoUrl || '未知视频源'}
                  </div>
                  {record.status === 'processing' && (
                    <span className="inline-flex items-center gap-1 rounded-full bg-blue-100 px-2.5 py-0.5 text-xs font-medium text-blue-700">
                      <Loader2 className="h-3 w-3 animate-spin" /> 处理中
                    </span>
                  )}
                  {record.status === 'success' && (
                    <span className="inline-flex items-center gap-1 rounded-full bg-emerald-100 px-2.5 py-0.5 text-xs font-medium text-emerald-700">
                      <CheckCircle2 className="h-3 w-3" /> 已完成
                    </span>
                  )}
                  {record.status === 'failed' && (
                    <span className="inline-flex items-center gap-1 rounded-full bg-red-100 px-2.5 py-0.5 text-xs font-medium text-red-700">
                      提取失败
                    </span>
                  )}
                </div>
                <Button variant="ghost" size="sm" className="h-8 w-8 p-0 text-surface-400 hover:text-red-500" onClick={() => handleDeleteHistory(record.id)}>
                  <Trash2 className="h-4 w-4" />
                </Button>
              </div>

              <CardContent className="p-4 space-y-4">
                {record.status === 'processing' && (
                  <div className="space-y-2 py-4">
                    <div className="flex justify-between text-xs text-surface-500">
                      <span>处理进度</span>
                      <span>{record.progress ? `${record.progress}%` : '等待中...'}</span>
                    </div>
                    <div className="h-2 w-full overflow-hidden rounded-full bg-surface-100">
                      <div
                        className="h-full rounded-full bg-cyan-500 transition-all duration-300"
                        style={{ width: `${record.progress || (extracting ? 99 : 0)}%` }} // 假进度
                      />
                    </div>
                  </div>
                )}

                {record.status === 'failed' && (
                  <div className="rounded-lg bg-red-50 p-3 text-sm text-red-800">
                    失败原因：{record.error || '未知错误'}
                  </div>
                )}

                {record.status === 'success' && (
                  <div className="grid gap-4 md:grid-cols-2">
                    {/* 音频解说 */}
                    <div className="rounded-lg border border-surface-200 bg-white">
                      <div className="flex items-center justify-between border-b border-surface-100 bg-surface-50/50 px-3 py-2">
                        <span className="text-sm font-semibold text-surface-800">🗣️ 音频解说转写 {record.language ? `(${record.language})` : ""}</span>
                        <Button variant="ghost" size="sm" className="h-7 text-xs text-cyan-700 hover:text-cyan-800 hover:bg-cyan-50" onClick={() => handleCopy(record.extractedNarration || '', '音频解说')}>
                          复制
                        </Button>
                      </div>
                      <div className="p-3">
                        {record.extractedNarration ? (
                          <p className="whitespace-pre-wrap text-sm text-surface-700 leading-relaxed max-h-[300px] overflow-y-auto">
                            {record.extractedNarration}
                          </p>
                        ) : (
                          <p className="text-sm text-surface-400 italic">未能提取到任何音频解说内容</p>
                        )}
                      </div>
                    </div>

                    {/* 画面提取 */}
                    <div className="rounded-lg border border-surface-200 bg-white">
                      <div className="flex items-center justify-between border-b border-surface-100 bg-surface-50/50 px-3 py-2">
                        <span className="text-sm font-semibold text-surface-800">👀 多帧画面文案识别</span>
                        <Button variant="ghost" size="sm" className="h-7 text-xs text-cyan-700 hover:text-cyan-800 hover:bg-cyan-50" onClick={() => handleCopy(record.extractedVisualText || '', '画面文案')}>
                          复制
                        </Button>
                      </div>
                      <div className="p-3">
                        {record.extractedVisualText ? (
                          <p className="whitespace-pre-wrap text-sm text-surface-700 leading-relaxed max-h-[300px] overflow-y-auto">
                            {record.extractedVisualText}
                          </p>
                        ) : (
                          <p className="text-sm text-surface-400 italic">未启用或未提取到画面文字内容</p>
                        )}
                      </div>
                    </div>

                    {record.frameItems && record.frameItems.length > 0 && (
                      <div className="md:col-span-2 rounded-lg border border-surface-200 bg-white">
                        <div className="flex items-center justify-between border-b border-surface-100 bg-surface-50/50 px-3 py-2">
                          <span className="text-sm font-semibold text-surface-800">🎞️ 逐帧识别内容</span>
                          <span className="text-xs text-surface-500">{record.frameItems.length} 帧</span>
                        </div>
                        <div className="grid gap-3 p-3 md:grid-cols-2 xl:grid-cols-3">
                          {record.frameItems.map((item, index) => (
                            <div key={`${item.frameUrl}-${index}`} className="overflow-hidden rounded-lg border border-surface-200 bg-surface-50">
                              <a href={item.frameUrl} target="_blank" rel="noopener noreferrer" className="block">
                                <img src={item.frameUrl} alt={`frame-${index + 1}`} className="h-40 w-full object-cover" />
                              </a>
                              <div className="space-y-2 p-3">
                                <div className="flex items-center justify-between">
                                  <span className="text-xs font-medium text-surface-700">第 {index + 1} 帧</span>
                                  <Button variant="ghost" size="sm" className="h-7 px-2 text-xs text-cyan-700 hover:text-cyan-800 hover:bg-cyan-50" onClick={() => handleCopy(item.extractedText || item.summary || '', `第${index + 1}帧画面文案`)}>
                                    复制
                                  </Button>
                                </div>
                                <p className="max-h-28 overflow-y-auto whitespace-pre-wrap text-sm leading-6 text-surface-700">
                                  {item.extractedText || '未识别到可见文字'}
                                </p>
                                {item.summary && (
                                  <p className="text-xs leading-5 text-surface-500">
                                    {item.summary}
                                  </p>
                                )}
                              </div>
                            </div>
                          ))}
                        </div>
                      </div>
                    )}

                    {/* 附加信息 (摘要和外链) */}
                    {(record.summary || record.frameUrl) && (
                      <div className="md:col-span-2 rounded-lg bg-surface-50 p-3 space-y-2 border border-surface-100">
                        {record.summary && (
                          <div>
                            <span className="text-xs font-semibold text-surface-800 mr-2">智能摘要:</span>
                            <span className="text-xs text-surface-600">{record.summary}</span>
                          </div>
                        )}
                        {record.frameUrl && (
                          <div>
                            <span className="text-xs font-semibold text-surface-800 mr-2">关键帧画面:</span>
                            <a href={record.frameUrl} target="_blank" rel="noopener noreferrer" className="text-xs text-cyan-600 hover:underline">
                              点击查看图片
                            </a>
                          </div>
                        )}
                      </div>
                    )}
                  </div>
                )}
              </CardContent>
            </Card>
          ))
        )}
      </div>
    </div>
  )
}
