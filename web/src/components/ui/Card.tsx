import { type FC, type ReactNode } from 'react'

interface Props {
  title?: ReactNode
  children: ReactNode
  className?: string
  hover?: boolean
  footer?: ReactNode
  padding?: boolean
}

export const Card: FC<Props> = ({ title, children, className = '', hover = false, footer, padding = true }) => (
  <div
    className={`group bg-[var(--bg-card)] border border-[var(--bg-border)] rounded-xl overflow-hidden transition-all duration-300 ${
      hover
        ? 'shadow-sm hover:shadow-lg hover:border-blue-500/20 hover:-translate-y-0.5'
        : 'shadow-sm'
    } ${className}`}
  >
    {title && (
      <div className="flex items-center justify-between px-5 py-4 border-b border-[var(--bg-border)]">
        <span className="text-sm font-bold text-[var(--text-primary)] tracking-tight">{title}</span>
      </div>
    )}
    <div className={padding ? 'p-5' : ''}>{children}</div>
    {footer && <div className="px-5 py-3 border-t border-[var(--bg-border)] text-xs text-[var(--text-muted)]">{footer}</div>}
  </div>
)
