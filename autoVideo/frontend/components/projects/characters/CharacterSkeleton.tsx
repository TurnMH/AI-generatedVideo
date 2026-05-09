export function CharacterSkeleton() {
  return (
    <div className="animate-pulse rounded-2xl border border-surface-200 bg-white p-4">
      <div className="mb-3 flex items-center gap-3">
        <div className="h-12 w-12 rounded-full bg-surface-100" />
        <div className="flex-1 space-y-1.5">
          <div className="h-4 w-2/3 rounded bg-surface-100" />
          <div className="h-3 w-1/3 rounded bg-surface-100" />
        </div>
      </div>
      <div className="space-y-1.5">
        <div className="h-3 w-full rounded bg-surface-100" />
        <div className="h-3 w-4/5 rounded bg-surface-100" />
      </div>
    </div>
  )
}
