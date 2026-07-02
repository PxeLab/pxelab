import { type FC, type InputHTMLAttributes, type SelectHTMLAttributes } from 'react'

const inputBase = 'w-full bg-[var(--bg-input)] border border-[var(--bg-border)] rounded-lg px-3.5 py-2 text-sm text-[var(--text-primary)] outline-none transition-all placeholder-[var(--text-muted)] focus:border-blue-500 focus:ring-2 focus:ring-blue-500/10'

interface InputProps extends InputHTMLAttributes<HTMLInputElement> {}

export const Input: FC<InputProps> = ({ className = '', ...props }) => (
  <input className={`${inputBase} ${className}`} {...props} />
)

interface SelectProps extends SelectHTMLAttributes<HTMLSelectElement> {}

export const Select: FC<SelectProps> = ({ className = '', children, ...props }) => (
  <select className={`${inputBase} appearance-none cursor-pointer ${className}`} {...props}>
    {children}
  </select>
)

interface TextareaProps extends React.TextareaHTMLAttributes<HTMLTextAreaElement> {}

export const Textarea: FC<TextareaProps> = ({ className = '', ...props }) => (
  <textarea className={`${inputBase} resize-y ${className}`} {...props} />
)
