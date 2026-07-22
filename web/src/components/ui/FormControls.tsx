import { type FC, type InputHTMLAttributes, type SelectHTMLAttributes, type TextareaHTMLAttributes } from 'react'

const base = 'w-full bg-[var(--bg-input)] border border-[var(--bg-border)] text-[var(--text-primary)] outline-none transition-all placeholder-[var(--text-muted)] focus:border-blue-500 focus:ring-2 focus:ring-blue-500/10'

const sizeClasses = {
  md: 'rounded-lg px-3.5 py-2 text-sm',
  sm: 'rounded-lg px-3 py-2 text-xs',
  xs: 'rounded px-2 py-1 text-xs',
} as const

type Size = keyof typeof sizeClasses

interface InputProps extends Omit<InputHTMLAttributes<HTMLInputElement>, 'size'> {
  size?: Size
}

export const Input: FC<InputProps> = ({ size = 'md', className = '', ...props }) => (
  <input className={`${base} ${sizeClasses[size]} ${className}`} {...props} />
)

interface SelectProps extends Omit<SelectHTMLAttributes<HTMLSelectElement>, 'size'> {
  size?: Size
}

const chevronBg = "url(\"data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='14' height='14' viewBox='0 0 24 24' fill='none' stroke='%2394a3b8' stroke-width='2' stroke-linecap='round' stroke-linejoin='round'%3E%3Cpath d='m6 9 6 6 6-6'/%3E%3C/svg%3E\")"

export const Select: FC<SelectProps> = ({ size = 'md', className = '', style, children, ...props }) => (
  <select
    className={`${base} ${sizeClasses[size]} appearance-none cursor-pointer pr-8 bg-no-repeat bg-[right_0.6rem_center] ${className}`}
    style={{ backgroundImage: chevronBg, ...style }}
    {...props}
  >
    {children}
  </select>
)

interface TextareaProps extends TextareaHTMLAttributes<HTMLTextAreaElement> {
  size?: Size
}

export const Textarea: FC<TextareaProps> = ({ size = 'md', className = '', ...props }) => (
  <textarea className={`${base} ${sizeClasses[size]} resize-y ${className}`} {...props} />
)
