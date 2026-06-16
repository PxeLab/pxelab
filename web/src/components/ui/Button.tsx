import { type FC, type ReactNode } from 'react'

interface Props {
  children: ReactNode
  onClick?: () => void
  variant?: 'primary' | 'secondary' | 'danger' | 'ghost'
  size?: 'sm' | 'md'
  className?: string
  disabled?: boolean
  type?: 'button' | 'submit'
}

const variantClasses = {
  primary: 'bg-blue-500 text-white hover:bg-blue-600',
  secondary: 'bg-[var(--bg-input)] text-[var(--text-primary)] border border-[var(--bg-border)] hover:bg-[var(--bg-hover)] hover:border-[#2e3245]',
  danger: 'bg-red-500/10 text-red-400 border border-red-500/20 hover:bg-red-500/20',
  ghost: 'bg-transparent text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-card)]',
}

export const Button: FC<Props> = ({ children, onClick, variant = 'secondary', size = 'md', className = '', disabled, type = 'button' }) => (
  <button
    type={type}
    onClick={onClick}
    disabled={disabled}
    className={`inline-flex items-center gap-1.5 rounded-lg font-semibold font-sans transition-all duration-200 disabled:opacity-40 ${variantClasses[variant]} ${size === 'sm' ? 'px-3 py-1.5 text-xs' : 'px-4 py-2 text-sm'} ${className}`}
  >
    {children}
  </button>
)
