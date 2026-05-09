'use client'
import * as React from 'react'
import * as ToastPrimitive from '@radix-ui/react-toast'
import { X } from 'lucide-react'
import { cn } from '@/lib/utils'

const ToastProvider = ToastPrimitive.Provider
const ToastViewport = React.forwardRef<
  React.ElementRef<typeof ToastPrimitive.Viewport>,
  React.ComponentPropsWithoutRef<typeof ToastPrimitive.Viewport>
>(({ className, ...props }, ref) => (
  <ToastPrimitive.Viewport
    ref={ref}
    className={cn(
      'fixed bottom-0 right-0 z-[100] flex max-h-screen w-full flex-col-reverse p-4 sm:max-w-[420px]',
      className
    )}
    {...props}
  />
))
ToastViewport.displayName = ToastPrimitive.Viewport.displayName

const Toast = React.forwardRef<
  React.ElementRef<typeof ToastPrimitive.Root>,
  React.ComponentPropsWithoutRef<typeof ToastPrimitive.Root> & {
    variant?: 'default' | 'destructive' | 'success'
  }
>(({ className, variant = 'default', ...props }, ref) => (
  <ToastPrimitive.Root
    ref={ref}
    className={cn(
      'group pointer-events-auto relative flex w-full items-center justify-between space-x-4 overflow-hidden rounded-xl border p-4 shadow-lg backdrop-blur-md transition-all',
      variant === 'destructive' && 'border-red-200/50 bg-red-50/90 text-red-900',
      variant === 'success' && 'border-emerald-200/50 bg-emerald-50/90 text-emerald-900',
      variant === 'default' && 'border-surface-200/50 bg-white/90 text-surface-900',
      className
    )}
    {...props}
  />
))
Toast.displayName = ToastPrimitive.Root.displayName

const ToastAction = React.forwardRef<
  React.ElementRef<typeof ToastPrimitive.Action>,
  React.ComponentPropsWithoutRef<typeof ToastPrimitive.Action>
>(({ className, ...props }, ref) => (
  <ToastPrimitive.Action
    ref={ref}
    className={cn(
      'inline-flex h-8 shrink-0 items-center justify-center rounded-lg border border-surface-200 bg-transparent px-3 text-sm font-medium transition-colors hover:bg-surface-100 focus:outline-none focus:ring-2 focus:ring-primary-500',
      className
    )}
    {...props}
  />
))
ToastAction.displayName = ToastPrimitive.Action.displayName

const ToastClose = React.forwardRef<
  React.ElementRef<typeof ToastPrimitive.Close>,
  React.ComponentPropsWithoutRef<typeof ToastPrimitive.Close>
>(({ className, ...props }, ref) => (
  <ToastPrimitive.Close
    ref={ref}
    className={cn(
      'absolute right-2 top-2 rounded-md p-1 opacity-0 transition-opacity hover:opacity-100 group-hover:opacity-100 focus:opacity-100 focus:outline-none focus:ring-2',
      className
    )}
    {...props}
  >
    <X className="h-4 w-4" />
  </ToastPrimitive.Close>
))
ToastClose.displayName = ToastPrimitive.Close.displayName

const ToastTitle = React.forwardRef<
  React.ElementRef<typeof ToastPrimitive.Title>,
  React.ComponentPropsWithoutRef<typeof ToastPrimitive.Title>
>(({ className, ...props }, ref) => (
  <ToastPrimitive.Title
    ref={ref}
    className={cn('text-sm font-semibold', className)}
    {...props}
  />
))
ToastTitle.displayName = ToastPrimitive.Title.displayName

const ToastDescription = React.forwardRef<
  React.ElementRef<typeof ToastPrimitive.Description>,
  React.ComponentPropsWithoutRef<typeof ToastPrimitive.Description>
>(({ className, ...props }, ref) => (
  <ToastPrimitive.Description
    ref={ref}
    className={cn('text-sm opacity-90', className)}
    {...props}
  />
))
ToastDescription.displayName = ToastPrimitive.Description.displayName

type ToastProps = React.ComponentPropsWithoutRef<typeof Toast>

export {
  ToastProvider,
  ToastViewport,
  Toast,
  ToastTitle,
  ToastDescription,
  ToastClose,
  ToastAction,
  type ToastProps,
}

// ─── useToast hook ────────────────────────────────────────────────────────────

interface ToastItem {
  id: string
  title?: string
  description?: string
  variant?: 'default' | 'destructive' | 'success'
  duration?: number
}

interface ToastState {
  toasts: ToastItem[]
}

type ToastAction2 =
  | { type: 'ADD'; toast: ToastItem }
  | { type: 'REMOVE'; id: string }

function toastReducer(state: ToastState, action: ToastAction2): ToastState {
  switch (action.type) {
    case 'ADD':
      return { toasts: [...state.toasts, action.toast] }
    case 'REMOVE':
      return { toasts: state.toasts.filter((t) => t.id !== action.id) }
    default:
      return state
  }
}

const ToastContext = React.createContext<{
  toasts: ToastItem[]
  toast: (opts: Omit<ToastItem, 'id'>) => void
  dismiss: (id: string) => void
} | null>(null)

export function ToastContextProvider({ children }: { children: React.ReactNode }) {
  const [state, dispatch] = React.useReducer(toastReducer, { toasts: [] })

  const toast = React.useCallback((opts: Omit<ToastItem, 'id'>) => {
    const id = Math.random().toString(36).slice(2)
    dispatch({ type: 'ADD', toast: { ...opts, id } })
    setTimeout(() => dispatch({ type: 'REMOVE', id }), opts.duration ?? 3500)
  }, [])

  const dismiss = React.useCallback((id: string) => {
    dispatch({ type: 'REMOVE', id })
  }, [])

  return (
    <ToastContext.Provider value={{ toasts: state.toasts, toast, dismiss }}>
      <ToastProvider>
        {children}
        {state.toasts.map((t) => (
          <Toast key={t.id} variant={t.variant}>
            {t.title && <ToastTitle>{t.title}</ToastTitle>}
            {t.description && <ToastDescription>{t.description}</ToastDescription>}
            <ToastClose onClick={() => dismiss(t.id)} />
          </Toast>
        ))}
        <ToastViewport />
      </ToastProvider>
    </ToastContext.Provider>
  )
}

export function useToast() {
  const ctx = React.useContext(ToastContext)
  if (!ctx) {
    // fallback when used outside provider
    return {
      toast: (opts: Omit<ToastItem, 'id'>) => {
        console.log('[toast]', opts.title, opts.description)
      },
      dismiss: (_id: string) => {},
      toasts: [] as ToastItem[],
    }
  }
  return ctx
}
