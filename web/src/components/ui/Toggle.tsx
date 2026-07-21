import { type FC } from 'react'

interface ToggleProps {
  checked: boolean
  onChange: (v: boolean) => void
  label?: string
  disabled?: boolean
}

export const Toggle: FC<ToggleProps> = ({ checked, onChange, label, disabled }) => (
  <label className={`inline-flex items-center gap-2.5 ${disabled ? 'opacity-50 pointer-events-none cursor-not-allowed' : 'cursor-pointer'}`}>
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      disabled={disabled}
      onClick={() => { if (!disabled) onChange(!checked) }}
      className={`relative inline-flex h-5 w-9 items-center rounded-full transition-all duration-300 ${checked ? 'bg-blue-500 shadow-[var(--glow-blue)]' : 'bg-[var(--bg-border)]'}`}
    >
      <span
        className={`inline-block h-3.5 w-3.5 transform rounded-full bg-white shadow-sm transition-all duration-300 ${
          checked ? 'translate-x-4' : 'translate-x-1'
        }`}
      />
    </button>
    {label && <span className="text-sm text-[var(--text-secondary)]">{label}</span>}
  </label>
)
