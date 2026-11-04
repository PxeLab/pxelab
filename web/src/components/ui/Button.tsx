import { type FC, type ReactNode } from 'react'

interface Props {
  children: ReactNode
  onClick?: () => void
  variant?: 'primary' | 'secondary' | 'danger' | 'ghost'
  size?: 'sm' | 'md'
  className?: string
  disabled?: boolean
  type?: 'button' | 'submit'
  /** 纯图标按钮必须提供可读名称，读屏与键盘用户才知道按钮做什么 */
  'aria-label'?: string
  'aria-expanded'?: boolean
}

const variantClasses = {
  primary: 'bg-blue-500 text-white hover:bg-blue-600 hover:shadow-[var(--glow-blue)] active:scale-[0.97]',
  secondary: 'bg-[var(--bg-input)] text-[var(--text-primary)] border border-[var(--bg-border)] hover:bg-[var(--bg-hover)] hover:border-[var(--text-muted)] hover:shadow-sm active:scale-[0.98]',
  danger: 'bg-accent-red/10 text-danger border border-accent-red/20 hover:bg-accent-red/20 hover:border-accent-red/30 active:scale-[0.98]',
  ghost: 'bg-transparent text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-card)] active:scale-[0.98]',
}

export const Button: FC<Props> = ({ children, onClick, variant = 'secondary', size = 'md', className = '', disabled, type = 'button', ...aria }) => (
  <button
    type={type}
    onClick={onClick}
    disabled={disabled}
    {...aria}
    className={`inline-flex items-center gap-1.5 rounded-lg font-semibold transition-all duration-200 disabled:opacity-40 disabled:active:scale-100 ${variantClasses[variant]} ${size === 'sm' ? 'px-3 py-1.5 text-xs' : 'px-4 py-2 text-sm'} ${className}`}
  >
    {children}
  </button>
)
