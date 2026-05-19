'use client'

import { Button } from '@/components/ui/button'

type HistoryEntry = {
  id: string
  savedAt: string
  label: string
  state: {
    targetMarket: string
    selectedBrandVoiceTemplate: string
    selectedStoryboardTemplate: string
    adPrompt: string
    subtitleText: string
    sceneDescriptionsText: string
    referenceImageHintsText: string
  }
}

type VersionSummary = {
  promptLength: number
  subtitleCount: number
  sceneCount: number
  referenceCount: number
  market: string
  brandVoice: string
  storyboard: string
}

interface LocalHistoryPanelProps {
  historyEntries: HistoryEntry[]
  selectedHistoryEntryId: string
  selectedHistoryEntry: HistoryEntry | null
  currentVersionSummary: VersionSummary
  selectedHistorySummary: VersionSummary | null
  onSave: () => void
  onSelect: (entryId: string) => void
  onRestore: (entry: HistoryEntry) => void
}

export function LocalHistoryPanel({
  historyEntries,
  selectedHistoryEntryId,
  selectedHistoryEntry,
  currentVersionSummary,
  selectedHistorySummary,
  onSave,
  onSelect,
  onRestore,
}: LocalHistoryPanelProps) {
  return (
    <div className="space-y-4 rounded-xl border border-surface-200 bg-white p-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div>
          <p className="text-sm font-medium text-surface-800">本地版本记录</p>
          <p className="text-xs text-surface-500">这是本地草稿快照，不是项目跳转记录。只用于回退当前页内容。</p>
        </div>
        <div className="flex items-center gap-2">
          <span className="rounded-full border border-surface-200 bg-surface-50 px-2.5 py-1 text-[11px] font-medium text-surface-600">
            已存 {historyEntries.length} 个版本
          </span>
          <Button type="button" size="sm" variant="outline" onClick={onSave}>
            保存当前版本
          </Button>
        </div>
      </div>

      {historyEntries.length > 0 ? (
        <div className="grid gap-4 xl:grid-cols-[1.15fr_1fr]">
          <div className="space-y-2 rounded-xl border border-surface-200 bg-surface-50 p-3">
            {historyEntries.map((entry) => {
              const isActive = selectedHistoryEntryId === entry.id
              return (
                <div
                  key={entry.id}
                  className={[
                    'rounded-lg border px-3 py-3',
                    isActive ? 'border-cyan-200 bg-white' : 'border-surface-200 bg-white/70',
                  ].join(' ')}
                >
                  <div className="flex flex-wrap items-start justify-between gap-2">
                    <div>
                      <p className="text-sm font-medium text-surface-800">{entry.label}</p>
                      <p className="mt-1 text-[11px] text-surface-500">
                        {new Date(entry.savedAt).toLocaleString('zh-CN')}
                      </p>
                    </div>
                    <div className="flex items-center gap-2">
                      <Button type="button" size="sm" variant="outline" onClick={() => onSelect(entry.id)}>
                        查看对比
                      </Button>
                      <Button type="button" size="sm" variant="outline" onClick={() => onRestore(entry)}>
                        恢复到当前页
                      </Button>
                    </div>
                  </div>
                  <div className="mt-2 flex flex-wrap gap-2 text-[11px] text-surface-500">
                    <span className="rounded-full border border-surface-200 bg-surface-50 px-2 py-0.5">市场 {entry.state.targetMarket}</span>
                    <span className="rounded-full border border-surface-200 bg-surface-50 px-2 py-0.5">语气 {entry.state.selectedBrandVoiceTemplate}</span>
                    <span className="rounded-full border border-surface-200 bg-surface-50 px-2 py-0.5">分镜 {entry.state.selectedStoryboardTemplate}</span>
                  </div>
                </div>
              )
            })}
          </div>

          <div className="rounded-xl border border-cyan-200 bg-cyan-50/40 p-3">
            <p className="text-sm font-medium text-cyan-900">当前与历史版本对比</p>
            {selectedHistoryEntry && selectedHistorySummary ? (
              <div className="mt-3 grid gap-3 md:grid-cols-2">
                <div className="rounded-lg border border-surface-200 bg-white p-3 text-xs text-surface-600">
                  <p className="font-medium text-surface-800">当前版本</p>
                  <p className="mt-2">广告文案字数：{currentVersionSummary.promptLength}</p>
                  <p className="mt-1">台词条数：{currentVersionSummary.subtitleCount}</p>
                  <p className="mt-1">分镜条数：{currentVersionSummary.sceneCount}</p>
                  <p className="mt-1">参考图提示：{currentVersionSummary.referenceCount}</p>
                  <p className="mt-1">目标市场：{currentVersionSummary.market}</p>
                  <p className="mt-1">品牌语气：{currentVersionSummary.brandVoice}</p>
                  <p className="mt-1">分镜模板：{currentVersionSummary.storyboard}</p>
                </div>
                <div className="rounded-lg border border-cyan-200 bg-white p-3 text-xs text-cyan-700">
                  <p className="font-medium text-cyan-900">历史版本</p>
                  <p className="mt-2">广告文案字数：{selectedHistorySummary.promptLength}</p>
                  <p className="mt-1">台词条数：{selectedHistorySummary.subtitleCount}</p>
                  <p className="mt-1">分镜条数：{selectedHistorySummary.sceneCount}</p>
                  <p className="mt-1">参考图提示：{selectedHistorySummary.referenceCount}</p>
                  <p className="mt-1">目标市场：{selectedHistorySummary.market}</p>
                  <p className="mt-1">品牌语气：{selectedHistorySummary.brandVoice}</p>
                  <p className="mt-1">分镜模板：{selectedHistorySummary.storyboard}</p>
                </div>
              </div>
            ) : (
              <p className="mt-3 text-xs text-cyan-700">先选择一个历史版本进行对比。</p>
            )}
          </div>
        </div>
      ) : (
        <p className="text-xs text-surface-500">当前还没有保存任何本地版本。</p>
      )}
    </div>
  )
}
