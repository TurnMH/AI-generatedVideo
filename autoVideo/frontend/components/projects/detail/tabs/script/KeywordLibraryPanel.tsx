'use client'

import type { Project } from '@/types'

type KeywordLibraryPanelProps = {
  keywordLibrary: NonNullable<Project['keyword_library']>
}

export function KeywordLibraryPanel({ keywordLibrary }: KeywordLibraryPanelProps) {
  if (
    !(keywordLibrary.characters?.length > 0 || keywordLibrary.locations?.length > 0)
  ) {
    return null
  }

  return (
    <div className="mb-4 rounded-lg border border-purple-100 bg-purple-50 px-4 py-3">
      <p className="mb-2 text-xs font-semibold text-purple-700">📚 关键词库</p>
      <div className="space-y-1.5">
        {keywordLibrary.characters?.length > 0 && (
          <div className="flex flex-wrap gap-1">
            <span className="text-[11px] text-purple-500 shrink-0">人物：</span>
            {keywordLibrary.characters.slice(0, 20).map((k) => (
              <span key={k} className="rounded bg-purple-100 px-1.5 py-0.5 text-[10px] text-purple-700">{k}</span>
            ))}
          </div>
        )}
        {keywordLibrary.locations?.length > 0 && (
          <div className="flex flex-wrap gap-1">
            <span className="text-[11px] text-purple-500 shrink-0">地点：</span>
            {keywordLibrary.locations.slice(0, 20).map((k) => (
              <span key={k} className="rounded bg-primary-100 px-1.5 py-0.5 text-[10px] text-primary-700">{k}</span>
            ))}
          </div>
        )}
        {keywordLibrary.events?.length > 0 && (
          <div className="flex flex-wrap gap-1">
            <span className="text-[11px] text-purple-500 shrink-0">事件：</span>
            {keywordLibrary.events.slice(0, 15).map((k) => (
              <span key={k} className="rounded bg-orange-100 px-1.5 py-0.5 text-[10px] text-orange-700">{k}</span>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
