import { type LucideIcon, Inbox } from 'lucide-react'
import { Button } from '@/components/ui/button'

interface EmptyStateProps {
  icon?: LucideIcon
  title: string
  description?: string
  actionLabel?: string
  onAction?: () => void
}

export function EmptyState({
  icon: Icon = Inbox,
  title,
  description,
  actionLabel,
  onAction,
}: EmptyStateProps) {
  return (
    <div className="flex flex-col items-center justify-center py-16 text-center animate-fadeIn">
      <div className="mb-5 flex h-20 w-20 items-center justify-center rounded-2xl bg-gradient-to-br from-primary-100 via-accent-100 to-primary-50 shadow-sm">
        <Icon className="h-9 w-9 text-primary-500" />
      </div>
      <h3 className="mb-2 text-base font-semibold text-surface-900">{title}</h3>
      {description && (
        <p className="mb-6 max-w-sm text-sm text-surface-500">{description}</p>
      )}
      {actionLabel && onAction && (
        <Button onClick={onAction} size="md">
          {actionLabel}
        </Button>
      )}
    </div>
  )
}
