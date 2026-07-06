import { type FC, type ReactNode, useLayoutEffect, useCallback, useRef } from 'react'
import { createPortal } from 'react-dom'
import { X } from 'lucide-react'

interface Props {
  open: boolean
  onClose: () => void
  title: string
  children: ReactNode
  footer?: ReactNode
  width?: string
  disableBackdropClose?: boolean
}

const FOCUSABLE = 'a[href], button:not([disabled]), textarea:not([disabled]), input:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex="-1"])'

export const Modal: FC<Props> = ({ open, onClose, title, children, footer, width = '480px', disableBackdropClose }) => {
  const contentRef = useRef<HTMLDivElement>(null)
  const onCloseRef = useRef(onClose)

  // Keep onClose ref in sync without putting side effects in the render phase
  useLayoutEffect(() => { onCloseRef.current = onClose })

  // Stable callback ref — tracks the content element for the focus trap.
  // Does NOT auto-focus; let each page opt in via autoFocus on the desired field.
  const setContentRef = useCallback((node: HTMLDivElement | null) => {
    contentRef.current = node
  }, [])

  const handleKey = useCallback((e: KeyboardEvent) => {
    if (e.key === 'Escape') { onCloseRef.current(); return }
    if (e.key === 'Tab' && contentRef.current) {
      const els = contentRef.current.querySelectorAll<HTMLElement>(FOCUSABLE)
      if (els.length === 0) return
      const first = els[0]
      const last = els[els.length - 1]
      if (e.shiftKey) {
        if (document.activeElement === first) { e.preventDefault(); last.focus() }
      } else {
        if (document.activeElement === last) { e.preventDefault(); first.focus() }
      }
    }
  }, [])

  useLayoutEffect(() => {
    if (!open) {
      document.removeEventListener('keydown', handleKey)
      return
    }
    document.addEventListener('keydown', handleKey)
    return () => document.removeEventListener('keydown', handleKey)
  }, [open, handleKey])

  if (!open) return null

  return createPortal(
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm animate-overlay-in"
      onClick={(e) => { if (!disableBackdropClose && e.target === e.currentTarget) onClose() }}
    >
      <div
        ref={setContentRef}
        className="bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded-2xl shadow-xl max-h-[80vh] overflow-y-auto animate-modal-in"
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
    </div>,
    document.body
  )
}
