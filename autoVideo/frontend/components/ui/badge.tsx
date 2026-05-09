import * as React from 'react'
import { cva, type VariantProps } from 'class-variance-authority'
import { cn } from '@/lib/utils'

const badgeVariants = cva(
  'inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-semibold transition-colors',
  {
    variants: {
      variant: {
        default: 'bg-primary-100 text-primary-700 ring-1 ring-primary-200/50',
        secondary: 'bg-surface-100 text-surface-700 ring-1 ring-surface-200/50',
        destructive: 'bg-red-100 text-red-700 ring-1 ring-red-200/50',
        outline: 'border border-surface-200 text-surface-600 bg-white/50',
        success: 'bg-emerald-100 text-emerald-700 ring-1 ring-emerald-200/50',
        warning: 'bg-amber-100 text-amber-700 ring-1 ring-amber-200/50',
      },
    },
    defaultVariants: {
      variant: 'default',
    },
  }
)

export interface BadgeProps
  extends React.HTMLAttributes<HTMLDivElement>,
    VariantProps<typeof badgeVariants> {}

function Badge({ className, variant, ...props }: BadgeProps) {
  return <div className={cn(badgeVariants({ variant }), className)} {...props} />
}

export { Badge, badgeVariants }
