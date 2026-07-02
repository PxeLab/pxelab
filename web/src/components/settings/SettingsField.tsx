import { type FC, type ReactNode } from 'react'

interface Props {
  label: string
  children: ReactNode
  help?: string
  disabled?: boolean
}

export const SettingsField: FC<Props> = ({ label, children, help, disabled }) => {
  return (
    <div className={disabled ? 'opacity-50 cursor-not-allowed' : ''}>
      <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1.5">
        {label}
        {help && (
          <span className="group relative inline-flex items-center ml-1.5 align-middle">
            <span className="inline-flex items-center justify-center w-3.5 h-3.5 rounded-full border border-[var(--text-muted)] text-[10px] text-[var(--text-muted)] cursor-help leading-none select-none">?</span>
            <span className="invisible group-hover:visible opacity-0 group-hover:opacity-100 transition-opacity absolute top-full left-0 mt-1.5 px-2.5 py-1.5 rounded-lg bg-[var(--bg-elevated)] border border-[var(--bg-border)] text-[11px] text-[var(--text-secondary)] whitespace-nowrap max-w-[320px] z-10 shadow-lg pointer-events-none">
              {help}
            </span>
          </span>
        )}
      </label>
      {children}
    </div>
  )
}

interface InputProps {
  value: string
  onChange: (v: string) => void
  placeholder?: string
  disabled?: boolean
  type?: string
  className?: string
  error?: boolean
}

export const SettingsInput: FC<InputProps> = ({ value, onChange, placeholder, disabled, type, className, error }) => {
  const errorBorder = error ? 'border-red-500/50 focus:border-red-500 focus:ring-red-500/10' : 'border-[var(--bg-border)] focus:border-blue-500 focus:ring-blue-500/10'
  const bgColor = 'bg-[var(--bg-input)]'
  return (
    <input
      type={type || 'text'}
      value={value}
      onChange={e => onChange(e.target.value)}
      placeholder={placeholder}
      disabled={disabled}
      className={`w-full ${bgColor} border rounded-lg px-3.5 py-2 text-sm text-[var(--text-primary)] outline-none transition-all placeholder-[var(--text-muted)] ${errorBorder} ${className || ''}`}
    />
  )
}
