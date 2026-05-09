export function TabSkeleton() {
  return (
    <div className="space-y-4 py-6">
      {[1, 2, 3].map((i) => (
        <div key={i} className="h-20 animate-pulse rounded-lg bg-surface-100" />
      ))}
    </div>
  )
}
