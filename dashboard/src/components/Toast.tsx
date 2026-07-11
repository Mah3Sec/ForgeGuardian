import { createContext, useCallback, useContext, useState, type ReactNode } from 'react'
import { CheckCircle, XCircle, Info, X } from 'lucide-react'
import { cn } from './ui/utils'

export type ToastVariant = 'success' | 'error' | 'info'

interface ToastProps {
  message: string
  variant?: ToastVariant
  onDismiss: () => void
}

const variantConfig: Record<ToastVariant, { icon: typeof CheckCircle; color: string }> = {
  success: { icon: CheckCircle, color: 'var(--success)' },
  error: { icon: XCircle, color: 'var(--critical)' },
  info: { icon: Info, color: 'var(--primary-blue)' },
}

export function Toast({ message, variant = 'info', onDismiss }: ToastProps) {
  const { icon: Icon, color } = variantConfig[variant]

  return (
    <div
      className="fixed bottom-6 right-6 z-50 flex items-center gap-3 rounded-lg border border-border-color bg-surface px-4 py-3 shadow-lg"
      role="status"
    >
      <Icon size={18} style={{ color }} />
      <span className="text-sm text-text-primary">{message}</span>
      <button
        type="button"
        onClick={onDismiss}
        className={cn('ml-2 text-text-muted hover:text-text-primary')}
        aria-label="Dismiss"
      >
        <X size={14} />
      </button>
    </div>
  )
}

interface ToastState {
  message: string
  variant: ToastVariant
}

interface ToastContextValue {
  show: (message: string, variant?: ToastVariant) => void
}

const ToastContext = createContext<ToastContextValue | null>(null)

// Deliberately minimal: single-toast-at-a-time. A new `show()` call replaces
// whatever toast is currently visible rather than queueing/stacking — that's
// sufficient for the current app's needs (one action -> one confirmation).
export function ToastProvider({ children }: { children: ReactNode }) {
  const [toast, setToast] = useState<ToastState | null>(null)

  const show = useCallback((message: string, variant: ToastVariant = 'info') => {
    setToast({ message, variant })
  }, [])

  const dismiss = useCallback(() => setToast(null), [])

  return (
    <ToastContext.Provider value={{ show }}>
      {children}
      {toast && <Toast message={toast.message} variant={toast.variant} onDismiss={dismiss} />}
    </ToastContext.Provider>
  )
}

export function useToast(): ToastContextValue {
  const ctx = useContext(ToastContext)
  if (!ctx) {
    throw new Error('useToast must be used within a ToastProvider')
  }
  return ctx
}
