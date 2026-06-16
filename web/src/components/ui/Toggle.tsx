import { type FC } from 'react'

interface ToggleProps {
  checked: boolean
  onChange: (v: boolean) => void
  label?: string
  disabled?: boolean
}

export const Toggle: FC<ToggleProps> = ({ checked, onChange, label, disabled }) => (
  <label className={`inline-flex items-center gap-2.5 cursor-pointer ${disabled ? 'opacity-50' : ''}`}>
    <button
      type="button"
      disabled={disabled}
      onClick={() => onChange(!checked)}
      className={`relative inline-flex h-5 w-9 items-center rounded-full transition-colors ${checked ? 'bg-blue-500' : 'bg-[var(--bg-border)]'}`}
    >
      <span
        className={`inline-block h-3.5 w-3.5 transform rounded-full bg-white transition-transform ${
          checked ? 'translate-x-4' : 'translate-x-1'
        }`}
      />
    </button>
    {label && <span className="text-sm text-[var(--text-secondary)]">{label}</span>}
  </label>
)
