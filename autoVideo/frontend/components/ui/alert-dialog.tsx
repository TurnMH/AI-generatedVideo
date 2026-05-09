'use client'
import * as React from 'react'
import { cn } from '@/lib/utils'

interface AlertDialogContextType {
  open: boolean
  onOpenChange: (open: boolean) => void
}

const AlertDialogContext = React.createContext<AlertDialogContextType>({
  open: false,
  onOpenChange: () => {},
})

function AlertDialog({
  children,
  open,
  onOpenChange,
}: {
  children: React.ReactNode
  open?: boolean
  onOpenChange?: (open: boolean) => void
}) {
  const [internalOpen, setInternalOpen] = React.useState(false)
  const isControlled = open !== undefined
  const isOpen = isControlled ? open : internalOpen
  const setOpen = isControlled ? onOpenChange! : setInternalOpen

  return (
    <AlertDialogContext.Provider value={{ open: isOpen, onOpenChange: setOpen }}>
      {children}
    </AlertDialogContext.Provider>
  )
}

function AlertDialogTrigger({
  children,
  asChild,
  ...props
}: React.ButtonHTMLAttributes<HTMLButtonElement> & { asChild?: boolean }) {
  const { onOpenChange } = React.useContext(AlertDialogContext)
  if (asChild && React.isValidElement(children)) {
    return React.cloneElement(children as React.ReactElement<Record<string, unknown>>, {
      onClick: (e: React.MouseEvent) => {
        onOpenChange(true);
        (children as React.ReactElement<{ onClick?: (e: React.MouseEvent) => void }>).props.onClick?.(e)
      },
    })
  }
  return (
    <button onClick={() => onOpenChange(true)} {...props}>
      {children}
    </button>
  )
}

function AlertDialogContent({
  children,
  className,
}: {
  children: React.ReactNode
  className?: string
}) {
  const { open, onOpenChange } = React.useContext(AlertDialogContext)
  if (!open) return null

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      <div
        className="fixed inset-0 bg-black/50 animate-in fade-in-0"
        onClick={() => onOpenChange(false)}
      />
      <div
        className={cn(
          'relative z-50 w-full max-w-lg rounded-lg bg-white p-6 shadow-lg animate-in fade-in-0 zoom-in-95',
          className
        )}
      >
        {children}
      </div>
    </div>
  )
}

function AlertDialogHeader({
  children,
  className,
}: {
  children: React.ReactNode
  className?: string
}) {
  return (
    <div className={cn('flex flex-col space-y-2 text-center sm:text-left', className)}>
      {children}
    </div>
  )
}

function AlertDialogFooter({
  children,
  className,
}: {
  children: React.ReactNode
  className?: string
}) {
  return (
    <div className={cn('mt-4 flex flex-col-reverse gap-2 sm:flex-row sm:justify-end', className)}>
      {children}
    </div>
  )
}

function AlertDialogTitle({
  children,
  className,
}: {
  children: React.ReactNode
  className?: string
}) {
  return <h2 className={cn('text-lg font-semibold', className)}>{children}</h2>
}

function AlertDialogDescription({
  children,
  className,
}: {
  children: React.ReactNode
  className?: string
}) {
  return <p className={cn('text-sm text-surface-500', className)}>{children}</p>
}

function AlertDialogAction({
  children,
  className,
  ...props
}: React.ButtonHTMLAttributes<HTMLButtonElement>) {
  const { onOpenChange } = React.useContext(AlertDialogContext)
  return (
    <button
      className={cn(
        'inline-flex h-10 items-center justify-center rounded-md bg-red-600 px-4 py-2 text-sm font-medium text-white hover:bg-red-700 focus:outline-none focus:ring-2 focus:ring-red-500 focus:ring-offset-2 disabled:pointer-events-none disabled:opacity-50',
        className
      )}
      onClick={(e) => {
        props.onClick?.(e)
        onOpenChange(false)
      }}
      {...props}
    >
      {children}
    </button>
  )
}

function AlertDialogCancel({
  children,
  className,
  ...props
}: React.ButtonHTMLAttributes<HTMLButtonElement>) {
  const { onOpenChange } = React.useContext(AlertDialogContext)
  return (
    <button
      className={cn(
        'inline-flex h-10 items-center justify-center rounded-md border border-surface-300 bg-white px-4 py-2 text-sm font-medium text-surface-700 hover:bg-surface-50 focus:outline-none focus:ring-2 focus:ring-surface-500 focus:ring-offset-2 disabled:pointer-events-none disabled:opacity-50',
        className
      )}
      onClick={(e) => {
        props.onClick?.(e)
        onOpenChange(false)
      }}
      {...props}
    >
      {children}
    </button>
  )
}

export {
  AlertDialog,
  AlertDialogTrigger,
  AlertDialogContent,
  AlertDialogHeader,
  AlertDialogFooter,
  AlertDialogTitle,
  AlertDialogDescription,
  AlertDialogAction,
  AlertDialogCancel,
}
