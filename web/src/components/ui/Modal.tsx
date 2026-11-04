import { type FC, type ReactNode, useLayoutEffect, useCallback, useRef, useId, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
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

// 只把真正可见的控件算进焦点环：弹窗里常有条件渲染/隐藏字段
const focusableIn = (root: HTMLElement): HTMLElement[] =>
  Array.from(root.querySelectorAll<HTMLElement>(FOCUSABLE)).filter(el => el.getClientRects().length > 0)

export const Modal: FC<Props> = ({ open, onClose, title, children, footer, width = '480px', disableBackdropClose }) => {
  const { t } = useTranslation()
  const contentRef = useRef<HTMLDivElement>(null)
  const onCloseRef = useRef(onClose)
  const restoreRef = useRef<HTMLElement | null>(null)
  const titleId = useId()

  // Keep onClose ref in sync without putting side effects in the render phase
  useLayoutEffect(() => { onCloseRef.current = onClose })

  // Stable callback ref — tracks the content element for the focus trap.
  // Does NOT auto-focus; let each page opt in via autoFocus on the desired field.
  const setContentRef = useCallback((node: HTMLDivElement | null) => {
    contentRef.current = node
  }, [])

  const handleKey = useCallback((e: KeyboardEvent) => {
    if (e.key === 'Escape') { onCloseRef.current(); return }
    if (e.key !== 'Tab' || !contentRef.current) return
    const els = focusableIn(contentRef.current)
    if (els.length === 0) return
    const first = els[0]
    const last = els[els.length - 1]
    const active = document.activeElement
    // 焦点还在弹窗外（例如 autoFocus 未命中）时，先把 Tab 拉回弹窗
    if (!contentRef.current.contains(active)) {
      e.preventDefault()
      first.focus()
      return
    }
    if (e.shiftKey) {
      if (active === first) { e.preventDefault(); last.focus() }
    } else if (active === last) {
      e.preventDefault(); first.focus()
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

  // 打开时聚焦弹窗内首个控件（页面已用 autoFocus 时尊重它），关闭时把焦点还给触发元素
  useLayoutEffect(() => {
    if (!open) return
    restoreRef.current = document.activeElement instanceof HTMLElement ? document.activeElement : null
    const content = contentRef.current
    if (!content) return
    if (!content.contains(document.activeElement)) {
      const target = focusableIn(content)[0] ?? content
      target.focus()
    }
    return () => {
      const el = restoreRef.current
      if (el && document.contains(el)) el.focus()
    }
  }, [open])

  // 背景 inert：读屏与 Tab 都不会进入遮罩后的页面
  useEffect(() => {
    if (!open) return
    const root = document.getElementById('root')
    root?.setAttribute('inert', '')
    return () => root?.removeAttribute('inert')
  }, [open])

  // 弹窗打开时锁住页面滚动，避免背景跟随滚轮移动
  useEffect(() => {
    if (!open) return
    const prev = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => { document.body.style.overflow = prev }
  }, [open])

  if (!open) return null

  return createPortal(
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm animate-overlay-in"
      onClick={(e) => { if (!disableBackdropClose && e.target === e.currentTarget) onClose() }}
    >
      <div
        ref={setContentRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        tabIndex={-1}
        className="bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded-2xl shadow-xl max-h-[80vh] overflow-y-auto animate-modal-in outline-none"
        style={{ width, maxWidth: '90vw' }}
      >
        <div className="flex items-center justify-between px-6 py-5 border-b border-[var(--bg-border)]">
          <h2 id={titleId} className="text-base font-bold text-[var(--text-primary)]">{title}</h2>
          <button
            onClick={onClose}
            aria-label={t('common.close')}
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
