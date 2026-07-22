import { type FC, createContext, useContext, useState, useCallback, useRef, type ReactNode } from 'react'
import { CheckCircle2, XCircle, Info, AlertTriangle, X, Loader2 } from 'lucide-react'

type ToastType = 'success' | 'error' | 'info' | 'warning' | 'loading'

interface Toast {
  id: number
  type: ToastType
  message: string
}

interface PromiseMessages {
  loading: string
  success: string
  error?: string
}

interface ToastContextValue {
  toast: (message: string, type?: ToastType) => void
  success: (message: string) => void
  error: (message: string) => void
  info: (message: string) => void
  warning: (message: string) => void
  showToast: (type: ToastType, message: string) => void
  dismiss: (id: number) => void
  promise: <T>(p: Promise<T>, messages: PromiseMessages) => Promise<T>
}

const ToastContext = createContext<ToastContextValue | null>(null)

export function useToast(): ToastContextValue {
  const ctx = useContext(ToastContext)
  if (!ctx) {
    throw new Error('useToast must be used within ToastProvider')
  }
  return ctx
}

const iconMap: Record<ToastType, FC<{ className?: string; size?: number }>> = {
  success: CheckCircle2,
  error: XCircle,
  info: Info,
  warning: AlertTriangle,
  loading: Loader2,
}

const colorMap: Record<ToastType, string> = {
  success: 'border-accent-green/30',
  error: 'border-accent-red/30',
  info: 'border-blue-500/30',
  warning: 'border-accent-yellow/30',
  loading: 'border-blue-500/30',
}

const iconColorMap: Record<ToastType, string> = {
  success: 'text-accent-green',
  error: 'text-accent-red',
  info: 'text-blue-400',
  warning: 'text-accent-yellow',
  loading: 'text-blue-400 animate-spin',
}

const DEFAULT_DURATION = 3500

let nextId = 0

export const ToastProvider: FC<{ children: ReactNode }> = ({ children }) => {
  const [toasts, setToasts] = useState<Toast[]>([])
  const timersRef = useRef(new Map<number, ReturnType<typeof setTimeout>>())

  const scheduleRemoval = useCallback((id: number, duration: number) => {
    const existing = timersRef.current.get(id)
    if (existing) clearTimeout(existing)
    timersRef.current.set(id, setTimeout(() => {
      timersRef.current.delete(id)
      setToasts(prev => prev.filter(t => t.id !== id))
    }, duration))
  }, [])

  const dismiss = useCallback((id: number) => {
    const timer = timersRef.current.get(id)
    if (timer) {
      clearTimeout(timer)
      timersRef.current.delete(id)
    }
    setToasts(prev => prev.filter(t => t.id !== id))
  }, [])

  const addToast = useCallback((type: ToastType, message: string): number => {
    const id = nextId++
    setToasts(prev => [...prev, { id, type, message }])
    if (type !== 'loading') scheduleRemoval(id, DEFAULT_DURATION)
    return id
  }, [scheduleRemoval])

  const update = useCallback((id: number, type: ToastType, message: string) => {
    setToasts(prev => prev.map(t => (t.id === id ? { ...t, type, message } : t)))
    if (type !== 'loading') scheduleRemoval(id, DEFAULT_DURATION)
  }, [scheduleRemoval])

  const toast = useCallback((message: string, type: ToastType = 'info') => { addToast(type, message) }, [addToast])
  const success = useCallback((message: string) => { addToast('success', message) }, [addToast])
  const error = useCallback((message: string) => { addToast('error', message) }, [addToast])
  const info = useCallback((message: string) => { addToast('info', message) }, [addToast])
  const warning = useCallback((message: string) => { addToast('warning', message) }, [addToast])
  const showToast = useCallback((type: ToastType, message: string) => { addToast(type, message) }, [addToast])

  const promise = useCallback(<T,>(p: Promise<T>, messages: PromiseMessages): Promise<T> => {
    const id = addToast('loading', messages.loading)
    return p.then(
      result => {
        update(id, 'success', messages.success)
        return result
      },
      err => {
        update(id, 'error', messages.error || err?.message || String(err))
        throw err
      },
    )
  }, [addToast, update])

  return (
    <ToastContext.Provider value={{ toast, success, error, info, warning, showToast, dismiss, promise }}>
      {children}
      <div className="fixed bottom-5 right-5 z-[200] flex flex-col gap-2">
        {toasts.map(t => {
          const Icon = iconMap[t.type]
          return (
            <div
              key={t.id}
              className={`flex items-center gap-2.5 px-4 py-3 rounded-lg bg-[var(--bg-card)] border ${colorMap[t.type]} shadow-lg text-sm text-[var(--text-primary)] min-w-[300px]`}
            >
              <Icon size={16} className={`shrink-0 ${iconColorMap[t.type]}`} />
              <span className="flex-1">{t.message}</span>
              <button onClick={() => dismiss(t.id)} className="text-[var(--text-muted)] hover:text-[var(--text-primary)]">
                <X size={14} />
              </button>
            </div>
          )
        })}
      </div>
    </ToastContext.Provider>
  )
}
