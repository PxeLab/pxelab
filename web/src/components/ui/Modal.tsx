import { type FC, type ReactNode, useEffect, useCallback } from 'react'
import { X } from 'lucide-react'

interface Props {
  open: boolean
  onClose: () => void
  title: string
  children: ReactNode
  footer?: ReactNode
  width?: string
}

export const Modal: FC<Props> = ({ open, onClose, title, children, footer, width = '480px' }) => {
  const handleKey = useCallback((e: KeyboardEvent) => {
    if (e.key === 'Escape') onClose()
  }, [onClose])

  useEffect(() => {
    if (open) document.addEventListener('keydown', handleKey)
    return () => document.removeEventListener('keydown', handleKey)
  }, [open, handleKey])

  if (!open) return null

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm"
      onClick={(e) => { if (e.target === e.currentTarget) onClose() }}
    >
      <div
        className="bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded-2xl shadow-[0_10px_40px_-10px_rgba(0,0,0,0.5)] max-h-[80vh] overflow-y-auto"
        style={{ width, maxWidth: '90vw' }}
      >
        <div className="flex items-center justify-between px-6 py-5 border-b border-[var(--bg-border)]">
          <h2 className="text-base font-bold text-[var(--text-primary)]">{title}</h2>
          <button
            onClick={onClose}
            className="w-7 h-7 flex items-center justify-center rounded-md text-[var(--text-muted)] hover:bg-[var(--bg-card)] hover:text-[var(--text-primary)] transition-colors"
          >
            <X size={16} />
          </button>
        </div>
        <div className="px-6 py-5">{children}</div>
        {footer && (
          <div className="flex justify-end gap-2 px-6 py-4 border-t border-[var(--bg-border)]">
            {footer}
          </div>
        )}
      </div>
    </div>
  )
}
