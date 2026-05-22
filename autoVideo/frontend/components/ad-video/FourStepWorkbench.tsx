'use client'

export type FourStepWorkbenchStatus = 'todo' | 'active' | 'done' | 'failed'

export type FourStepWorkbenchItem = {
  key: string
  stepNo: number
  title: string
  status: FourStepWorkbenchStatus
  summary: string
  nextAction: string
  artifact?: string
}

interface FourStepWorkbenchProps {
  items: FourStepWorkbenchItem[]
}

const statusLabel: Record<FourStepWorkbenchStatus, string> = {
  todo: '待开始',
  active: '进行中',
  done: '已完成',
  failed: '失败',
}

const statusTone: Record<FourStepWorkbenchStatus, string> = {
  todo: 'border-surface-200 bg-surface-50 text-surface-600',
  active: 'border-cyan-200 bg-cyan-50 text-cyan-800',
  done: 'border-emerald-200 bg-emerald-50 text-emerald-700',
  failed: 'border-rose-200 bg-rose-50 text-rose-700',
}

const cardTone: Record<FourStepWorkbenchStatus, string> = {
  todo: 'border-surface-200 bg-white',
  active: 'border-cyan-200 bg-cyan-50/40',
  done: 'border-emerald-200 bg-emerald-50/50',
  failed: 'border-rose-200 bg-rose-50/50',
}

export function FourStepWorkbench({ items }: FourStepWorkbenchProps) {
  return (
    <section className="space-y-3 rounded-2xl border border-surface-200 bg-white p-4 shadow-sm">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <p className="text-sm font-medium text-surface-900">四步骤工作台</p>
          <p className="mt-1 text-xs text-surface-500">把图片、视频、合成拆开看；每一步都显示当前状态、产物和下一步。</p>
        </div>
        <span className="rounded-full border border-surface-200 bg-surface-50 px-2.5 py-1 text-[11px] font-medium text-surface-600">
          手动流程
        </span>
      </div>

      <div className="grid gap-3 xl:grid-cols-4 md:grid-cols-2">
        {items.map((item) => (
          <article key={item.key} className={[
            'rounded-2xl border p-4 shadow-sm transition-colors',
            cardTone[item.status],
          ].join(' ')}>
            <div className="flex items-start justify-between gap-3">
              <div className="flex h-8 w-8 items-center justify-center rounded-full border border-surface-200 bg-white text-xs font-semibold text-surface-700">
                {item.stepNo}
              </div>
              <span className={[
                'rounded-full border px-2 py-0.5 text-[11px] font-medium',
                statusTone[item.status],
              ].join(' ')}>
                {statusLabel[item.status]}
              </span>
            </div>

            <p className="mt-3 text-sm font-medium text-surface-900">{item.title}</p>
            <p className="mt-2 min-h-[60px] text-xs leading-5 text-surface-600">{item.summary}</p>

            <div className="mt-3 rounded-xl border border-surface-200 bg-white/80 px-3 py-2">
              <p className="text-[11px] font-medium text-surface-700">下一步</p>
              <p className="mt-1 text-[11px] leading-5 text-surface-500">{item.nextAction}</p>
            </div>

            <div className="mt-3 rounded-xl border border-dashed border-surface-200 bg-surface-50 px-3 py-2">
              <p className="text-[11px] font-medium text-surface-700">当前产物</p>
              <p className="mt-1 text-[11px] leading-5 text-surface-500">{item.artifact || '当前还没有可展示产物'}</p>
            </div>
          </article>
        ))}
      </div>
    </section>
  )
}
