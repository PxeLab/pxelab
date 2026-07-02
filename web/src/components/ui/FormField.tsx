import { type FC, type ReactNode } from 'react'

interface Props {
  label: string
  children: ReactNode
  className?: string
  help?: string
}

export const FormField: FC<Props> = ({ label, children, className = '', help }) => (
  <div className={className}>
    <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1.5">{label}</label>
    {children}
    {help && <p className="text-xs text-[var(--text-muted)] mt-1">{help}</p>}
  </div>
)
