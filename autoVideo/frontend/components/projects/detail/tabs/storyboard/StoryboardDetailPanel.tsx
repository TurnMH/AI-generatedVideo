'use client'

import {
  ChevronLeft,
  ChevronRight,
  Download,
  Image,
  LayoutGrid,
  Loader2,
  Mic,
  Send,
  X,
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { ZoomableImage } from '@/components/ui/image-lightbox'
import { StatusBadge } from '@/components/projects/detail/StatusBadge'
import { getProviderLabel } from '@/lib/model-feasibility'
import { formatDuration, formatChatTimestamp } from '@/lib/projects/utils'
import { getChatRole, getChatContent, getChatImageUrl } from '@/lib/projects/chat'
import type { LegacyChatMessage } from '@/lib/projects/chat'
import { canTriggerStoryboardImage } from '@/lib/projects/storyboard-image'
import { formatStoryboardErrorMessage } from '@/lib/projects/storyboard-status'
import type { ImageModelOption } from '@/lib/model-display'
import type { DubbingTask } from '@/lib/api'
import type { Asset, Project, Storyboard, StoryboardVersion } from '@/types'
import type { RefObject } from 'react'

export function StoryboardDetailPanel({
  project,
  selectedSb,
  onClose,
  storyboardItemLabel,
  storyboardImageLabel,
  storyboardGenerateLabel,
  selectedStoryboardVersion,
  selectedStoryboardPreviewUrl,
  selectedStoryboardMessageCount,
  versionIdx,
  onVersionIdxChange,
  sbDescLang,
  onSbDescLangChange,
  storyboardAssets,
  onCameraMovementChange,
  sbVoiceScope,
  onSbVoiceScopeChange,
  sbVoiceModel,
  onSbVoiceModelChange,
  sbVoiceRate,
  onSbVoiceRateChange,
  sbVoicePitch,
  onSbVoicePitchChange,
  sbVoiceVolume,
  onSbVoiceVolumeChange,
  sbVoiceOptions,
  generatingSbVoice,
  onGenerateVoice,
  storyboardTaskMap,
  modelOptions,
  imageModelAvailability,
  onGenerateOne,
  chatListRef,
  onChatListScroll,
  chatBottomRef,
  chatInput,
  onChatInputChange,
  chatLoading,
  onChat,
}: {
  project: Project
  selectedSb: Storyboard
  onClose: () => void
  storyboardItemLabel: string
  storyboardImageLabel: string
  storyboardGenerateLabel: string
  selectedStoryboardVersion: StoryboardVersion | undefined
  selectedStoryboardPreviewUrl: string
  selectedStoryboardMessageCount: number
  versionIdx: number
  onVersionIdxChange: (idx: number) => void
  sbDescLang: 'zh' | 'en'
  onSbDescLangChange: (lang: 'zh' | 'en') => void
  storyboardAssets: Asset[]
  onCameraMovementChange: (val: string) => Promise<void>
  sbVoiceScope: 'single' | 'episode'
  onSbVoiceScopeChange: (scope: 'single' | 'episode') => void
  sbVoiceModel: string
  onSbVoiceModelChange: (value: string) => void
  sbVoiceRate: string
  onSbVoiceRateChange: (value: string) => void
  sbVoicePitch: string
  onSbVoicePitchChange: (value: string) => void
  sbVoiceVolume: string
  onSbVoiceVolumeChange: (value: string) => void
  sbVoiceOptions: { value: string; label: string }[]
  generatingSbVoice: boolean
  onGenerateVoice: () => void
  storyboardTaskMap: Map<number, DubbingTask>
  modelOptions: ImageModelOption[]
  imageModelAvailability: Record<string, boolean>
  onGenerateOne: (sb: Storyboard, modelName?: string) => void
  chatListRef: RefObject<HTMLDivElement>
  onChatListScroll: () => void
  chatBottomRef: RefObject<HTMLDivElement>
  chatInput: string
  onChatInputChange: (value: string) => void
  chatLoading: boolean
  onChat: () => void
}) {
  const modelSections = [
    { label: '🌐 多模态推荐', filter: (m: ImageModelOption) => m.tags.includes('多模态') },
    { label: '🎨 高质量文生图', filter: (m: ImageModelOption) => m.tags.includes('高质量') && !m.tags.includes('多模态') },
    { label: '⚡ 高速 / 低成本', filter: (m: ImageModelOption) => !m.tags.includes('多模态') && !m.tags.includes('高质量') && !m.tags.includes('本地') },
    { label: '🖥️ 本地部署', filter: (m: ImageModelOption) => m.tags.includes('本地') },
  ]

  return (
    <div className="fixed inset-0 z-40 flex justify-end">
      <div className="absolute inset-0 bg-black/30" onClick={onClose} />
      <div className="relative z-50 flex h-full w-full max-w-6xl flex-col bg-white shadow-xl">
        <div className="flex items-center justify-between border-b px-4 py-3">
          <div>
            <h3 className="text-lg font-semibold">{`${storyboardItemLabel} #${selectedSb.sequence_number}`}</h3>
            <p className="text-xs text-surface-400">左侧查看版本与画面，右侧继续对话修改场景。</p>
          </div>
          <Button size="sm" variant="ghost" onClick={onClose}>
            <X className="h-4 w-4" />
          </Button>
        </div>
        <div className="flex min-h-0 flex-1 flex-col lg:flex-row">
          <div className="flex min-h-0 w-full flex-col border-b bg-surface-50/60 lg:w-[380px] lg:border-b-0 lg:border-r">
            <div className="min-h-0 flex-1 overflow-y-auto p-4">
              <div className="overflow-hidden rounded-2xl border border-surface-200 bg-white shadow-sm">
                <div className="relative aspect-video overflow-hidden bg-surface-100">
                  {selectedStoryboardPreviewUrl ? (
                    <img
                      src={selectedStoryboardPreviewUrl}
                      alt={selectedStoryboardVersion ? `V${selectedStoryboardVersion.version_number}` : `${storyboardItemLabel} #${selectedSb.sequence_number}`}
                      className="h-full w-full object-cover"
                    />
                  ) : (
                    <div className="flex h-full items-center justify-center text-surface-300">
                      <LayoutGrid className="h-10 w-10" />
                    </div>
                  )}
                  {selectedSb.status === 'generating' && (
                    <div className="absolute inset-0 flex items-center justify-center bg-surface-950/35">
                      <div className="rounded-full bg-white/90 px-3 py-1 text-[11px] font-medium text-surface-700 shadow-sm">
                        新版本生成中
                      </div>
                    </div>
                  )}
                </div>
                <div className="border-t border-surface-100 px-4 py-3">
                  <div className="flex items-center justify-between gap-3">
                    <div>
                      <p className="text-sm font-medium text-surface-800">
                        {selectedStoryboardVersion ? `版本 V${selectedStoryboardVersion.version_number}` : `当前${storyboardItemLabel}预览`}
                      </p>
                      <p className="text-[11px] text-surface-400">
                        {selectedSb.status === 'generating'
                          ? '生成完成后会自动刷新到这里。'
                          : selectedSb.versions && selectedSb.versions.length > 1
                            ? `共 ${selectedSb.versions.length} 个版本`
                            : '目前仅展示当前版本'}
                      </p>
                    </div>
                    <StatusBadge status={selectedSb.status} />
                  </div>
                  {selectedSb.versions && selectedSb.versions.length > 1 && (
                    <div className="mt-3 flex items-center justify-between gap-2">
                      <Button size="sm" variant="ghost" className="h-8 w-8 p-0" disabled={versionIdx === 0} onClick={() => onVersionIdxChange(versionIdx - 1)}>
                        <ChevronLeft className="h-4 w-4" />
                      </Button>
                      <span className="text-xs text-surface-500">
                        V{selectedSb.versions[versionIdx]?.version_number} ({versionIdx + 1}/{selectedSb.versions.length})
                      </span>
                      <Button size="sm" variant="ghost" className="h-8 w-8 p-0" disabled={versionIdx >= selectedSb.versions.length - 1} onClick={() => onVersionIdxChange(versionIdx + 1)}>
                        <ChevronRight className="h-4 w-4" />
                      </Button>
                    </div>
                  )}
                </div>
              </div>

              <div className="mt-4 grid grid-cols-3 gap-3 rounded-xl border border-surface-200 bg-white p-3 shadow-sm">
                <div className="text-center">
                  <p className="text-[10px] text-surface-400">时长</p>
                  <p className="text-sm font-medium">{formatDuration(selectedSb.duration)}</p>
                </div>
                <div className="text-center">
                  <p className="text-[10px] text-surface-400 mb-1">运镜</p>
                  <Select
                    value={selectedSb.camera_movement || 'static'}
                    onValueChange={onCameraMovementChange}
                  >
                    <SelectTrigger className="h-7 px-2 text-xs font-medium text-center border-surface-200">
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {([
                        { value: 'static', label: '静止镜头' },
                        { value: 'push-in', label: '缓慢推进' },
                        { value: 'pull-out', label: '缓慢拉远' },
                        { value: 'pan-left', label: '向左摇镜' },
                        { value: 'pan-right', label: '向右摇镜' },
                        { value: 'tracking', label: '跟随运镜' },
                        { value: 'handheld', label: '手持纪实' },
                      ] as const).map((m) => (
                        <SelectItem key={m.value} value={m.value} className="text-xs">{m.label}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
                <div className="text-center">
                  <p className="text-[10px] text-surface-400">比例</p>
                  <p className="text-sm font-medium">{selectedSb.aspect_ratio || '16:9'}</p>
                </div>
              </div>

              <div className="mt-4 space-y-3 rounded-xl border border-surface-200 bg-white p-4 shadow-sm">
                <div>
                  <div className="flex items-center justify-between">
                    <p className="text-xs font-medium text-surface-500">
                      {sbDescLang === 'zh' ? '场景描述' : '生成模型描述（英文）'}
                    </p>
                    <div className="flex items-center rounded-md border border-surface-200 bg-surface-50 p-0.5 text-[10px] font-semibold">
                      <button
                        className={`rounded px-1.5 py-0.5 transition-colors ${sbDescLang === 'zh' ? 'bg-white shadow-sm text-primary-700' : 'text-surface-400 hover:text-surface-600'}`}
                        onClick={() => onSbDescLangChange('zh')}
                      >中</button>
                      <button
                        className={`rounded px-1.5 py-0.5 transition-colors ${sbDescLang === 'en' ? 'bg-white shadow-sm text-primary-700' : 'text-surface-400 hover:text-surface-600'}`}
                        onClick={() => onSbDescLangChange('en')}
                      >EN</button>
                    </div>
                  </div>
                  {sbDescLang === 'zh' ? (
                    <p className="mt-1 whitespace-pre-wrap text-sm leading-6 text-surface-700">{selectedSb.scene_description || '暂无描述'}</p>
                  ) : (
                    <p className="mt-1 whitespace-pre-wrap break-all text-xs leading-5 text-surface-600">
                      {selectedSb.prompt_used || selectedSb.scene_description || '暂无描述'}
                    </p>
                  )}
                </div>
                {(selectedSb.characters?.length > 0 || selectedSb.location || (selectedSb.asset_ids?.length > 0)) && (
                  <div className="border-t border-surface-100 pt-3 space-y-2">
                    {selectedSb.characters?.length > 0 && (
                      <div className="flex items-start gap-2">
                        <span className="shrink-0 text-[10px] font-medium text-surface-400 w-10 mt-0.5">角色</span>
                        <div className="flex flex-wrap gap-1">
                          {selectedSb.characters.map((c) => (
                            <span key={c} className="rounded-full bg-primary-50 border border-primary-100 px-2 py-0.5 text-[11px] text-primary-700">👤 {c}</span>
                          ))}
                        </div>
                      </div>
                    )}
                    {selectedSb.location && (
                      <div className="flex items-center gap-2">
                        <span className="shrink-0 text-[10px] font-medium text-surface-400 w-10">场景</span>
                        <span className="rounded-full bg-surface-100 px-2 py-0.5 text-[11px] text-surface-700">📍 {selectedSb.location}</span>
                      </div>
                    )}
                    {selectedSb.asset_ids?.length > 0 && (() => {
                      const linkedAssets = storyboardAssets.filter((a) => selectedSb.asset_ids.includes(a.id))
                      if (linkedAssets.length === 0) return null
                      return (
                        <div className="flex items-start gap-2">
                          <span className="shrink-0 text-[10px] font-medium text-surface-400 w-10 mt-1">资源</span>
                          <div className="flex flex-wrap gap-1.5">
                            {linkedAssets.map((a) => (
                              <div key={a.id} className="flex items-center gap-1 rounded-lg border border-surface-200 bg-surface-50 px-1.5 py-1" title={a.name}>
                                {a.image_url ? (
                                  <img src={a.image_url} alt={a.name} className="h-7 w-7 rounded object-cover flex-shrink-0" />
                                ) : (
                                  <div className="h-7 w-7 rounded bg-surface-200 flex items-center justify-center text-[10px] text-surface-400">?</div>
                                )}
                                <div className="max-w-[80px]">
                                  <p className="truncate text-[10px] font-medium text-surface-700">{a.name}</p>
                                  <p className="text-[9px] text-surface-400">{a.type === 'character' ? '人物' : a.type === 'scene' ? '场景' : '物品'}</p>
                                </div>
                              </div>
                            ))}
                          </div>
                        </div>
                      )
                    })()}
                  </div>
                )}
                {selectedSb.dialogue && (
                  <div className="rounded-xl bg-primary-50 p-3 text-sm text-primary-900">
                    <p className="mb-1 text-xs font-medium text-primary-500">台词</p>
                    <p className="whitespace-pre-wrap leading-6">{selectedSb.dialogue}</p>
                  </div>
                )}

                {(project.enable_dubbing || project.enable_subtitle) && selectedSb.episode_id && selectedSb.dialogue && (
                  <div className="mt-3 rounded-xl border border-violet-200 bg-violet-50 p-3">
                    <div className="mb-2 flex items-center justify-between">
                      <div className="flex items-center gap-2">
                        <Mic className="h-3.5 w-3.5 text-violet-500" />
                        <span className="text-xs font-medium text-violet-700">语音生成</span>
                      </div>
                      <div className="flex items-center rounded-md border border-violet-200 bg-white p-0.5 text-[10px] font-medium">
                        <button
                          type="button"
                          className={`rounded px-2 py-0.5 transition-colors ${sbVoiceScope === 'single' ? 'bg-violet-500 text-white' : 'text-violet-600 hover:bg-violet-100'}`}
                          onClick={() => onSbVoiceScopeChange('single')}
                        >单帧</button>
                        <button
                          type="button"
                          className={`rounded px-2 py-0.5 transition-colors ${sbVoiceScope === 'episode' ? 'bg-violet-500 text-white' : 'text-violet-600 hover:bg-violet-100'}`}
                          onClick={() => onSbVoiceScopeChange('episode')}
                        >全集</button>
                      </div>
                    </div>
                    <p className="mb-2 text-[11px] text-violet-600">
                      {sbVoiceScope === 'single'
                        ? '仅使用本帧台词生成语音片段'
                        : '聚合本集所有分镜台词生成整集语音'}
                    </p>
                    <div className="mb-2 grid grid-cols-2 gap-1.5 sm:grid-cols-4">
                      <select
                        value={sbVoiceModel}
                        onChange={(e) => onSbVoiceModelChange(e.target.value)}
                        className="h-7 rounded-md border border-violet-200 bg-white px-2 text-[11px]"
                        title="音色"
                      >
                        {sbVoiceOptions.map(v => (
                          <option key={v.value} value={v.value}>{v.label}</option>
                        ))}
                      </select>
                      <select
                        value={sbVoiceRate}
                        onChange={(e) => onSbVoiceRateChange(e.target.value)}
                        className="h-7 rounded-md border border-violet-200 bg-white px-2 text-[11px]"
                        title="语速"
                      >
                        {[{value:'-30%',label:'慢 -30%'},{value:'-15%',label:'慢 -15%'},{value:'+0%',label:'正常'},{value:'+15%',label:'快 +15%'},{value:'+30%',label:'快 +30%'}].map(v => (
                          <option key={v.value} value={v.value}>{v.label}</option>
                        ))}
                      </select>
                      <select
                        value={sbVoicePitch}
                        onChange={(e) => onSbVoicePitchChange(e.target.value)}
                        className="h-7 rounded-md border border-violet-200 bg-white px-2 text-[11px]"
                        title="音调"
                      >
                        {[{value:'-10Hz',label:'低 -10Hz'},{value:'-5Hz',label:'低 -5Hz'},{value:'+0Hz',label:'正常'},{value:'+5Hz',label:'高 +5Hz'},{value:'+10Hz',label:'高 +10Hz'}].map(v => (
                          <option key={v.value} value={v.value}>{v.label}</option>
                        ))}
                      </select>
                      <select
                        value={sbVoiceVolume}
                        onChange={(e) => onSbVoiceVolumeChange(e.target.value)}
                        className="h-7 rounded-md border border-violet-200 bg-white px-2 text-[11px]"
                        title="音量"
                      >
                        {[{value:'-20%',label:'低 -20%'},{value:'-10%',label:'低 -10%'},{value:'+0%',label:'正常'},{value:'+10%',label:'高 +10%'},{value:'+20%',label:'高 +20%'}].map(v => (
                          <option key={v.value} value={v.value}>{v.label}</option>
                        ))}
                      </select>
                    </div>
                    <Button
                      size="sm"
                      className="w-full border-violet-300 bg-violet-600 text-white hover:bg-violet-700"
                      disabled={generatingSbVoice || (sbVoiceScope === 'single' && (storyboardTaskMap.get(selectedSb.id)?.status === 'pending' || storyboardTaskMap.get(selectedSb.id)?.status === 'processing'))}
                      onClick={onGenerateVoice}
                      title={sbVoiceScope === 'single' ? '使用本帧台词生成语音' : '为本集每个分镜分别生成语音'}
                    >
                      {generatingSbVoice ? (
                        <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
                      ) : (
                        <Mic className="mr-1.5 h-3.5 w-3.5" />
                      )}
                      {sbVoiceScope === 'single' ? '生成本帧语音' : '生成全部镜语音'}
                    </Button>
                    {sbVoiceScope === 'single' && (() => {
                      const sbTask = storyboardTaskMap.get(selectedSb.id)
                      if (!sbTask) return null
                      return (
                        <div className="mt-2">
                          {(sbTask.status === 'pending' || sbTask.status === 'processing') && (
                            <div className="flex items-center gap-1.5 text-[11px] text-amber-600">
                              <Loader2 className="h-3 w-3 animate-spin" />
                              {sbTask.status === 'pending' ? '排队等待中...' : `生成中 ${sbTask.chunks_total > 0 ? `(${sbTask.chunks_done}/${sbTask.chunks_total})` : ''}`}
                            </div>
                          )}
                          {sbTask.status === 'failed' && (
                            <p className="text-[11px] text-red-500">生成失败，可重新提交</p>
                          )}
                          {sbTask.status === 'succeeded' && sbTask.audio_url && (
                            <div className="space-y-1">
                              <p className="text-[10px] text-violet-500">语音已生成</p>
                              <audio controls className="h-8 w-full" src={sbTask.audio_url} />
                              <a href={sbTask.audio_url} target="_blank" rel="noopener noreferrer" className="inline-flex items-center gap-1 text-[10px] text-violet-500 hover:underline">
                                <Download className="h-3 w-3" /> 下载音频
                              </a>
                            </div>
                          )}
                        </div>
                      )
                    })()}
                  </div>
                )}
              </div>

              {canTriggerStoryboardImage(selectedSb) && (
                <div className={`mt-4 rounded-xl border p-4 shadow-sm ${selectedSb.status === 'failed' ? 'border-red-200 bg-red-50' : selectedSb.status === 'paused' ? 'border-yellow-200 bg-yellow-50' : selectedSb.status === 'completed' ? 'border-primary-200 bg-primary-50/40' : 'border-amber-200 bg-amber-50'}`}>
                  <p className={`mb-1 text-sm font-medium ${selectedSb.status === 'failed' ? 'text-red-700' : selectedSb.status === 'paused' ? 'text-yellow-800' : selectedSb.status === 'completed' ? 'text-primary-800' : 'text-amber-700'}`}>
                    {selectedSb.status === 'failed'
                      ? `${storyboardGenerateLabel}失败，可直接换模型重试`
                      : selectedSb.status === 'paused'
                        ? `${storyboardGenerateLabel}已暂停，可直接继续`
                        : selectedSb.status === 'completed'
                          ? `可单独重新生成当前${storyboardImageLabel}`
                          : `${storyboardImageLabel}待生成，可直接选择模型启动`}
                  </p>
                  <p className={`mb-3 text-[11px] ${selectedSb.status === 'failed' ? 'text-red-600' : selectedSb.status === 'paused' ? 'text-yellow-800/80' : selectedSb.status === 'completed' ? 'text-primary-700/80' : 'text-amber-700/80'}`}>
                    {selectedSb.status === 'failed'
                      ? `原因：${formatStoryboardErrorMessage(selectedSb.error_msg || '')}`
                      : selectedSb.status === 'paused'
                        ? `继续后，会从当前${storyboardItemLabel}队列重新拉起${storyboardImageLabel}生成。`
                        : selectedSb.status === 'completed'
                          ? `只会重新生成这一条${storyboardItemLabel}的图片，不会影响其他分镜。`
                          : `发送对话修改后，可在这里继续选择模型生成对应${storyboardImageLabel}。`}
                  </p>
                  <div className="max-h-[45vh] overflow-y-auto space-y-3 pr-0.5">
                    {modelSections.map(({ label: sectionLabel, filter }) => {
                      const sectionModels = modelOptions.filter(filter)
                      if (sectionModels.length === 0) return null
                      return (
                        <div key={sectionLabel}>
                          <p className="mb-1 text-[10px] font-semibold uppercase tracking-wide text-surface-400">{sectionLabel}</p>
                          <div className="grid grid-cols-2 gap-2">
                            {sectionModels.map((m) => {
                              const avail = imageModelAvailability[m.key]
                              const broken = !!m.failureReason
                              const globalIdx = modelOptions.findIndex(item => item.key === m.key)
                              return (
                                <button
                                  key={m.key}
                                  title={broken ? `已停用：${m.failureReason}` : undefined}
                                  disabled={broken}
                                  className={`flex items-start gap-2 rounded-lg border p-2.5 text-left transition-colors ${broken ? 'cursor-not-allowed border-red-200 bg-red-50 opacity-60' : avail === false ? 'border-surface-200 bg-surface-50 opacity-50 cursor-not-allowed' : 'border-surface-200 bg-white hover:border-primary-300 hover:bg-primary-50'}`}
                                  onClick={() => broken ? undefined : onGenerateOne(selectedSb, m.key)}
                                >
                                  <div className="mt-0.5 flex flex-col items-center gap-0.5">
                                    <span className="text-base leading-none">{m.icon}</span>
                                    <span className="rounded-full bg-surface-200 px-1 text-[8px] text-surface-500 font-bold">#{globalIdx + 1}</span>
                                  </div>
                                  <div className="flex-1 min-w-0">
                                    <div className="flex flex-wrap items-center gap-1">
                                      <span className="text-xs font-semibold text-surface-800">{m.label}</span>
                                      {!broken && m.speed === 'fast' && <span className="rounded bg-green-100 px-1 text-[9px] text-green-700">⚡快</span>}
                                      {!broken && m.quality === 'high' && <span className="rounded bg-blue-100 px-1 text-[9px] text-blue-700">★高质</span>}
                                      {!broken && avail === true && <span className="rounded bg-emerald-100 px-1 text-[9px] text-emerald-700">● 可用</span>}
                                      {!broken && avail === false && <span className="rounded bg-red-100 px-1 text-[9px] text-red-600">● 未配置</span>}
                                      {broken && <span className="rounded bg-red-100 px-1 text-[9px] text-red-600">⚠ 已停用</span>}
                                    </div>
                                    <p className="text-[10px] leading-none text-surface-400 mt-0.5">{getProviderLabel(m.provider)}</p>
                                    {broken ? (
                                      <p className="mt-0.5 text-[9px] leading-tight text-red-400">{m.failureReason}</p>
                                    ) : (
                                      <>
                                        <p className="mt-0.5 text-[10px] leading-tight text-surface-500">{m.desc}</p>
                                        <div className="mt-1 flex flex-wrap gap-0.5">
                                          {m.tags.map(t => (
                                            <span key={t} className="rounded-full bg-surface-100 px-1.5 text-[9px] text-surface-500">{t}</span>
                                          ))}
                                        </div>
                                      </>
                                    )}
                                  </div>
                                </button>
                              )
                            })}
                          </div>
                        </div>
                      )
                    })}
                  </div>
                </div>
              )}
            </div>
          </div>

          <div className="flex min-h-0 flex-1 flex-col">
            <div className="flex min-h-0 flex-1 flex-col p-5">
              <div className="mb-3 flex items-center justify-between">
                <h4 className="text-sm font-semibold text-surface-700">AI 对话记录</h4>
                <span className="text-[11px] text-surface-400">{selectedStoryboardMessageCount} 条消息</span>
              </div>
              <div
                ref={chatListRef}
                onScroll={onChatListScroll}
                className="min-h-0 flex-1 space-y-3 overflow-y-auto rounded-xl border border-surface-200 bg-surface-50/70 p-3 pr-2"
              >
                {selectedSb.status === 'generating' && (
                  <div className="rounded-2xl border border-primary-200 bg-primary-50/80 px-3 py-3 text-xs text-primary-900 shadow-sm">
                    <div className="flex items-center gap-2 font-medium">
                      <Loader2 className="h-3.5 w-3.5 animate-spin" />
                      <span>{`${storyboardItemLabel}正在生成新版本`}</span>
                    </div>
                    <p className="mt-2 text-[11px] text-primary-700/80">
                      当前画面会在生成完成后自动刷新到左侧版本预览，并保留在版本列表中。
                    </p>
                  </div>
                )}
                {selectedSb.status === 'generating' && (
                  <div className="flex justify-start">
                    <div className="max-w-[88%] rounded-2xl border border-surface-200 bg-white px-3 py-2.5 text-xs text-surface-700 shadow-sm">
                      <div className="mb-1 flex items-center gap-2 text-[10px] text-surface-400">
                        <Loader2 className="h-3 w-3 animate-spin" />
                        <span>AI 正在生成新的场景画面</span>
                      </div>
                      <div className="overflow-hidden rounded-xl border border-dashed border-primary-200 bg-primary-50/60">
                        {selectedStoryboardPreviewUrl ? (
                          <div className="relative">
                            <ZoomableImage src={selectedStoryboardPreviewUrl} alt="" className="h-36 w-full object-cover opacity-75" />
                            <div className="absolute inset-0 flex items-center justify-center bg-surface-950/35">
                              <div className="rounded-full bg-white/90 px-3 py-1 text-[11px] font-medium text-surface-700 shadow-sm">
                                {`新${storyboardImageLabel}生成中`}
                              </div>
                            </div>
                          </div>
                        ) : (
                          <div className="flex h-36 w-full flex-col items-center justify-center gap-2">
                            <Image className="h-8 w-8 text-primary-300" />
                            <span className="text-[11px] text-surface-500">{`待返回的新${storyboardImageLabel}将在这里展示`}</span>
                          </div>
                        )}
                      </div>
                      <div className="mt-2 text-[11px] leading-5 text-surface-500">
                        {`你可以继续补充镜头语言、角色动作和环境氛围；本轮完成后左侧会显示新的${storyboardImageLabel}版本。`}
                      </div>
                    </div>
                  </div>
                )}
                {selectedStoryboardMessageCount === 0 ? (
                  <p className="py-4 text-center text-xs text-surface-400">暂无对话记录</p>
                ) : (
                  selectedSb.agent_history.map((rawMsg, i) => {
                    const msg = rawMsg as LegacyChatMessage
                    const role = getChatRole(msg)
                    const content = getChatContent(msg)
                    const imageUrl = getChatImageUrl(msg)

                    return (
                      <div key={i} className={`flex ${role === 'user' ? 'justify-end' : 'justify-start'}`}>
                        <div className={`max-w-[88%] rounded-2xl px-3 py-2.5 text-xs shadow-sm ${
                          role === 'user'
                            ? 'bg-primary-500 text-white'
                            : 'border border-surface-200 bg-white text-surface-700'
                        }`}>
                          <div className={`mb-1 flex items-center justify-between gap-3 text-[10px] ${
                            role === 'user' ? 'text-primary-100' : 'text-surface-400'
                          }`}>
                            <span>{role === 'user' ? '你的修改要求' : 'AI 回应'}</span>
                            <span>{formatChatTimestamp(msg.timestamp)}</span>
                          </div>
                          <div className="whitespace-pre-wrap leading-5">{content}</div>
                          {imageUrl && <img src={imageUrl} alt="" className="mt-2 max-w-full rounded-lg border border-black/5" />}
                        </div>
                      </div>
                    )
                  })
                )}
                <div ref={chatBottomRef} />
              </div>
            </div>

            <div className="border-t bg-white/95 p-4 backdrop-blur-sm">
              <div className="rounded-xl border border-surface-200 bg-surface-50/70 p-3">
                <div className="mb-2 flex items-center justify-between">
                  <div>
                    <p className="text-sm font-medium text-surface-800">对话修改</p>
                    <p className="text-[11px] text-surface-400">告诉 AI 你想怎么改场景画面、角色动作、镜头语言或对白氛围。</p>
                  </div>
                </div>
                <Textarea
                  value={chatInput}
                  onChange={(e) => onChatInputChange(e.target.value)}
                  placeholder="例如：保持夜景基调不变，把镜头拉近到人物半身，增加风吹衣摆和火把反光。"
                  className="min-h-[110px] resize-none bg-white"
                  onKeyDown={(e) => {
                    if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
                      e.preventDefault()
                      onChat()
                    }
                  }}
                />
                <div className="mt-3 flex items-center justify-between gap-3">
                  <span className="text-[11px] text-surface-400">按 `Ctrl/Cmd + Enter` 快速发送</span>
                  <Button onClick={onChat} disabled={chatLoading || !chatInput.trim()}>
                    {chatLoading ? (
                      <Loader2 className="mr-1.5 h-4 w-4 animate-spin" />
                    ) : (
                      <Send className="mr-1.5 h-4 w-4" />
                    )}
                    发送修改要求
                  </Button>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
