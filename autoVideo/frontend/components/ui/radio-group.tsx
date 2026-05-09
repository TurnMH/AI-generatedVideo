'use client'
import * as React from 'react'
import { cn } from '@/lib/utils'

interface RadioGroupProps extends React.HTMLAttributes<HTMLDivElement> {
  value?: string
  onValueChange?: (value: string) => void
  defaultValue?: string
}

const RadioGroupContext = React.createContext<{
  value?: string
  onValueChange?: (value: string) => void
}>({})

const RadioGroup = React.forwardRef<HTMLDivElement, RadioGroupProps>(
  ({ className, value, onValueChange, defaultValue, children, ...props }, ref) => {
    const [internalValue, setInternalValue] = React.useState(defaultValue || '')
    const currentValue = value !== undefined ? value : internalValue
    const handleChange = onValueChange || setInternalValue

    return (
      <RadioGroupContext.Provider value={{ value: currentValue, onValueChange: handleChange }}>
        <div ref={ref} role="radiogroup" className={cn('grid gap-2', className)} {...props}>
          {children}
        </div>
      </RadioGroupContext.Provider>
    )
  }
)
RadioGroup.displayName = 'RadioGroup'

interface RadioGroupItemProps extends React.HTMLAttributes<HTMLButtonElement> {
  value: string
  disabled?: boolean
}

const RadioGroupItem = React.forwardRef<HTMLButtonElement, RadioGroupItemProps>(
  ({ className, value, children, disabled, ...props }, ref) => {
    const context = React.useContext(RadioGroupContext)
    const checked = context.value === value

    return (
      <button
        ref={ref}
        type="button"
        role="radio"
        aria-checked={checked}
        disabled={disabled}
        data-state={checked ? 'checked' : 'unchecked'}
        className={cn(
          'aspect-square h-4 w-4 rounded-full border border-surface-300 text-primary-600 ring-offset-white focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50',
          checked && 'border-blue-600',
          className
        )}
        onClick={() => context.onValueChange?.(value)}
        {...props}
      >
        {checked && (
          <span className="flex items-center justify-center">
            <span className="h-2.5 w-2.5 rounded-full bg-primary-600" />
          </span>
        )}
      </button>
    )
  }
)
RadioGroupItem.displayName = 'RadioGroupItem'

export { RadioGroup, RadioGroupItem }
