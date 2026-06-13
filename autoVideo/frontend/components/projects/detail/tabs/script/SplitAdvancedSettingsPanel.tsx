'use client'

import { ChevronDown, Search, X } from 'lucide-react'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import type { Model } from '@/types'
import { AUTO_EPISODE_SPLIT_HINT } from '@/lib/projects/episode-split'
import type { EpisodeCountRecommendation } from '@/lib/projects/comic'
import { getProviderLabel } from '@/lib/model-feasibility'
import { getSplitModelRemark } from '@/lib/projects/models'
import { getProjectModelAvailability } from './model-availability'

type ModelAvailability = { label: string; color: string }

type SplitAdvancedSettingsPanelProps = {
  textModelsLoading: boolean
  splitModels: Model[]
  usesAutoEpisodeSplit: boolean
  splitSettingsDirty: boolean
  selectedSplitModelAvailability: ModelAvailability | null
  effectiveSplitModel: Model | null
  selectedSplitModelProvider: string | null
  hasValidTargetEpisodes: boolean
  parsedTargetEpisodes: number
  recommendedEpisodeCount: EpisodeCountRecommendation | null
  draftTargetEpisodes: string
  onDraftTargetEpisodesChange: (value: string) => void
  onMarkSplitSettingsDirty: () => void
  showSplitAdvancedSettings: boolean
  onToggleAdvancedSettings: () => void
  savingSplitModel: boolean
  isProcessing: boolean
  shouldShowSplitSearch: boolean
  splitModelSearch: string
  onSplitModelSearchChange: (value: string) => void
  draftSplitModelId: string
  onSplitModelChange: (value: string) => void
  filteredSplitModels: Model[]
  splitModelCapabilities: string[]
  selectedSplitModelRemark: string
  textModelHealthMap: Record<string, 'healthy' | 'unhealthy' | 'unknown'>
}

export function SplitAdvancedSettingsPanel({
  textModelsLoading,
  splitModels,
  usesAutoEpisodeSplit,
  splitSettingsDirty,
  selectedSplitModelAvailability,
  effectiveSplitModel,
  selectedSplitModelProvider,
  hasValidTargetEpisodes,
  parsedTargetEpisodes,
  recommendedEpisodeCount,
  draftTargetEpisodes,
  onDraftTargetEpisodesChange,
  onMarkSplitSettingsDirty,
  showSplitAdvancedSettings,
  onToggleAdvancedSettings,
  savingSplitModel,
  isProcessing,
  shouldShowSplitSearch,
  splitModelSearch,
  onSplitModelSearchChange,
  draftSplitModelId,
  onSplitModelChange,
  filteredSplitModels,
  splitModelCapabilities,
  selectedSplitModelRemark,
  textModelHealthMap,
}: SplitAdvancedSettingsPanelProps) {
  return (
    <div className="mb-4 rounded-xl border border-primary-100 bg-primary-50/50 p-4">
      <div>
        <div className="rounded-lg border border-white/80 bg-white/80 p-4 shadow-sm">
          {textModelsLoading ? (
            <div className="h-10 animate-pulse rounded-md bg-surface-100" />
          ) : splitModels.length === 0 ? (
            <p className="text-xs text-surface-400">暂无可选文本模型</p>
          ) : (
            <div className="space-y-4">
              <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
                <div className="min-w-0 flex-1 space-y-3">
                  <div className="flex flex-wrap items-center gap-2">
                    <p className="text-sm font-semibold text-surface-900">分集高级设置</p>
                    <Badge variant="outline" className="text-[11px]">
                      {splitSettingsDirty ? '待应用' : '按当前配置执行'}
                    </Badge>
                    {selectedSplitModelAvailability ? (
                      <span className={`rounded-full px-2 py-0.5 text-[10px] font-medium ${selectedSplitModelAvailability.color}`}>
                        {selectedSplitModelAvailability.label}
                      </span>
                    ) : null}
                  </div>
                  <p className="text-xs leading-5 text-surface-500">
                    {usesAutoEpisodeSplit
                      ? '这一块默认收起。视频项目会按剧本自动分集，通常只需确认分集模型即可。'
                      : '这一块默认收起。只有在需要更换分集模型、调整目标集数或手动覆盖推荐值时，再展开修改即可。'}
                  </p>

                  <div className="grid gap-2 md:grid-cols-3">
                    <div className="rounded-xl border border-surface-200 bg-surface-50/70 px-3 py-2">
                      <p className="text-[11px] text-surface-400">当前模型</p>
                      <p className="truncate text-sm font-medium text-surface-900">{effectiveSplitModel?.name || '未选择分集模型'}</p>
                      <p className="truncate text-[11px] text-surface-500">{selectedSplitModelProvider || '展开后可切换模型'}</p>
                    </div>
                    <div className="rounded-xl border border-surface-200 bg-surface-50/70 px-3 py-2">
                      <p className="text-[11px] text-surface-400">{usesAutoEpisodeSplit ? '分集方式' : '目标分集数'}</p>
                      <p className="text-sm font-medium text-surface-900">
                        {usesAutoEpisodeSplit
                          ? (recommendedEpisodeCount ? `按剧本自动拆分（约 ${recommendedEpisodeCount.count} 集）` : '按剧本自动拆分')
                          : (hasValidTargetEpisodes ? `${parsedTargetEpisodes} 集` : '未设置')}
                      </p>
                      <p className="text-[11px] text-surface-500">
                        {usesAutoEpisodeSplit
                          ? AUTO_EPISODE_SPLIT_HINT
                          : '启动自动分集时会按这里的集数执行'}
                      </p>
                    </div>
                    <div className="rounded-xl border border-surface-200 bg-surface-50/70 px-3 py-2">
                      <p className="text-[11px] text-surface-400">{usesAutoEpisodeSplit ? '拆分依据' : '智能建议'}</p>
                      <p className="text-sm font-medium text-surface-900">
                        {recommendedEpisodeCount ? `预计 ${recommendedEpisodeCount.count} 集` : '暂无预估'}
                      </p>
                      <p className="truncate text-[11px] text-surface-500">{recommendedEpisodeCount?.reason || '上传并解析剧本后自动计算'}</p>
                    </div>
                  </div>
                </div>

                <div className="flex shrink-0 flex-wrap items-center gap-2">
                  {!usesAutoEpisodeSplit && recommendedEpisodeCount && draftTargetEpisodes !== String(recommendedEpisodeCount.count) ? (
                    <Button
                      type="button"
                      size="sm"
                      variant="outline"
                      onClick={() => {
                        onDraftTargetEpisodesChange(String(recommendedEpisodeCount.count))
                        onMarkSplitSettingsDirty()
                      }}
                      disabled={savingSplitModel || isProcessing}
                    >
                      采用推荐
                    </Button>
                  ) : null}
                  <Button
                    type="button"
                    size="sm"
                    variant="outline"
                    onClick={onToggleAdvancedSettings}
                  >
                    <ChevronDown className={`mr-1.5 h-3.5 w-3.5 transition-transform ${showSplitAdvancedSettings ? 'rotate-180' : ''}`} />
                    {showSplitAdvancedSettings ? '收起设置' : '调整设置'}
                  </Button>
                </div>
              </div>

              {showSplitAdvancedSettings ? (
                <div className="space-y-4 border-t border-surface-100 pt-4">
                  {shouldShowSplitSearch ? (
                    <div className="space-y-1.5">
                      <div className="flex items-center justify-between gap-2">
                        <Label className="text-xs font-medium text-surface-700">搜索筛选</Label>
                        <span className="text-[11px] text-surface-400">支持中文搜索</span>
                      </div>
                      <div className="relative">
                        <Search className="pointer-events-none absolute left-3 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-surface-400" />
                        <Input
                          value={splitModelSearch}
                          onChange={(event) => onSplitModelSearchChange(event.target.value)}
                          placeholder="搜索模型、供应商、能力或备注"
                          className="bg-white pl-8 pr-8"
                          disabled={savingSplitModel || isProcessing}
                        />
                        {splitModelSearch ? (
                          <button
                            type="button"
                            className="absolute right-2 top-1/2 -translate-y-1/2 rounded p-1 text-surface-400 transition hover:bg-surface-100 hover:text-surface-600"
                            onClick={() => onSplitModelSearchChange('')}
                            disabled={savingSplitModel || isProcessing}
                            aria-label="清空模型搜索"
                          >
                            <X className="h-3.5 w-3.5" />
                          </button>
                        ) : null}
                      </div>
                    </div>
                  ) : null}

                  <div className="space-y-1.5">
                    <Label className="text-xs font-medium text-surface-700">选择模型</Label>
                    <Select
                      value={draftSplitModelId}
                      onValueChange={onSplitModelChange}
                      disabled={savingSplitModel || isProcessing}
                    >
                      <SelectTrigger className="h-auto min-h-12 bg-white py-2.5">
                        {effectiveSplitModel ? (
                          <div className="min-w-0 flex-1 text-left">
                            <p className="truncate text-sm font-medium text-surface-900">{effectiveSplitModel.name}</p>
                            <p className="truncate text-[11px] text-surface-400">{selectedSplitModelRemark}</p>
                          </div>
                        ) : (
                          <span className="truncate text-surface-400">手动选择分集模型</span>
                        )}
                      </SelectTrigger>
                      <SelectContent>
                        {filteredSplitModels.length > 0 ? (
                          filteredSplitModels.map((model) => {
                            const availability = getProjectModelAvailability(model, textModelHealthMap)
                            const remark = getSplitModelRemark(model)
                            return (
                              <SelectItem
                                key={model.id}
                                value={model.id.toString()}
                                textValue={`${model.name} ${getProviderLabel(model.provider)} ${remark}`}
                              >
                                <div className="flex max-w-[360px] flex-col gap-1 py-0.5">
                                  <div className="flex min-w-0 items-center gap-2">
                                    <span className="truncate font-medium">{model.name}</span>
                                    <span className="text-xs text-surface-400">({getProviderLabel(model.provider)})</span>
                                    <span className={`rounded-full px-1.5 py-0.5 text-[10px] font-medium ${availability.color}`}>
                                      {availability.label}
                                    </span>
                                    {model.is_default ? (
                                      <Badge variant="outline" className="px-1 py-0 text-[10px]">
                                        默认
                                      </Badge>
                                    ) : null}
                                  </div>
                                  <p className="truncate text-[11px] leading-4 text-surface-500">{remark}</p>
                                </div>
                              </SelectItem>
                            )
                          })
                        ) : (
                          <div className="px-3 py-2 text-xs text-surface-400">
                            未找到匹配的模型，请尝试搜索模型名、供应商或“推理 / 长上下文”等中文关键词
                          </div>
                        )}
                      </SelectContent>
                    </Select>
                    {splitModelCapabilities.length > 0 ? (
                      <p className="text-[11px] leading-4 text-surface-500">
                        能力标签：{splitModelCapabilities.join(' · ')}
                      </p>
                    ) : null}
                  </div>

                  {usesAutoEpisodeSplit ? (
                    <div className="rounded-xl border border-primary-100 bg-primary-50/50 px-3 py-2.5">
                      <p className="text-xs font-medium text-primary-800">按剧本自动分集</p>
                      <p className="mt-1 text-[11px] leading-5 text-primary-700">{AUTO_EPISODE_SPLIT_HINT}</p>
                      {recommendedEpisodeCount ? (
                        <p className="mt-2 text-[11px] leading-4 text-primary-600">
                          当前剧本预计约 {recommendedEpisodeCount.count} 集（{recommendedEpisodeCount.reason}），实际以拆分结果为准。
                        </p>
                      ) : null}
                    </div>
                  ) : (
                    <div className="space-y-1.5">
                      <Label className="text-xs font-medium text-surface-700">目标分集数</Label>
                      <Input
                        type="number"
                        min={1}
                        max={200}
                        step={1}
                        value={draftTargetEpisodes}
                        onChange={(event) => {
                          onDraftTargetEpisodesChange(event.target.value.replace(/[^\d]/g, ''))
                          onMarkSplitSettingsDirty()
                        }}
                        placeholder="填写需要拆分的分集数量，例如 12"
                        disabled={savingSplitModel || isProcessing}
                        className="bg-white"
                      />
                      {recommendedEpisodeCount ? (
                        <p className="text-[11px] leading-4 text-emerald-700">
                          推荐 {recommendedEpisodeCount.count} 段（{recommendedEpisodeCount.reason}）
                          {draftTargetEpisodes === String(recommendedEpisodeCount.count) ? '，当前已采用推荐值。' : '。'}
                        </p>
                      ) : null}
                    </div>
                  )}
                </div>
              ) : null}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
