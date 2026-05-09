'use client'
import * as React from 'react'
import { cn } from '@/lib/utils'

interface SliderProps {
  className?: string
  value?: number[]
  onValueChange?: (value: number[]) => void
  defaultValue?: number[]
  min?: number
  max?: number
  step?: number
  disabled?: boolean
}

const Slider = React.forwardRef<HTMLDivElement, SliderProps>(
  (
    {
      className,
      value,
      onValueChange,
      defaultValue = [50],
      min = 0,
      max = 100,
      step = 1,
      disabled,
      ...props
    },
    ref
  ) => {
    const [internalValue, setInternalValue] = React.useState(defaultValue)
    const currentValue = value !== undefined ? value : internalValue

    const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
      const newValue = [Number(e.target.value)]
      if (value === undefined) setInternalValue(newValue)
      onValueChange?.(newValue)
    }

    const percent = ((currentValue[0] - min) / (max - min)) * 100

    return (
      <div ref={ref} className={cn('relative flex w-full touch-none select-none items-center', className)} {...props}>
        <div className="relative h-2 w-full rounded-full bg-surface-200">
          <div
            className="absolute h-full rounded-full bg-primary-600"
            style={{ width: `${percent}%` }}
          />
        </div>
        <input
          type="range"
          min={min}
          max={max}
          step={step}
          value={currentValue[0]}
          onChange={handleChange}
          disabled={disabled}
          className="absolute inset-0 h-full w-full cursor-pointer opacity-0 disabled:cursor-not-allowed"
        />
        <div
          className="absolute h-5 w-5 rounded-full border-2 border-blue-600 bg-white shadow transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 disabled:pointer-events-none disabled:opacity-50"
          style={{ left: `calc(${percent}% - 10px)` }}
        />
      </div>
    )
  }
)
Slider.displayName = 'Slider'

export { Slider }
