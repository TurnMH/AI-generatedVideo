import * as React from 'react'
import { cn } from '@/lib/utils'

export interface InputProps extends React.InputHTMLAttributes<HTMLInputElement> {
  error?: boolean
}

const Input = React.forwardRef<HTMLInputElement, InputProps>(
  ({ className, type, error, ...props }, ref) => {
    return (
      <input
        type={type}
        className={cn(
          'flex h-10 w-full rounded-xl border bg-white/80 px-3 py-2 text-sm text-surface-900 caret-surface-900 ring-offset-white file:border-0 file:bg-transparent file:text-sm file:font-medium file:text-surface-900 placeholder:text-surface-500 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50 focus-visible:ring-offset-2 focus-visible:border-primary-300 disabled:cursor-not-allowed disabled:opacity-50 transition-all duration-200',
          error
            ? 'border-red-400 focus-visible:ring-red-500/50'
            : 'border-surface-200 hover:border-surface-300',
          className
        )}
        ref={ref}
        {...props}
      />
    )
  }
)
Input.displayName = 'Input'

export { Input }
