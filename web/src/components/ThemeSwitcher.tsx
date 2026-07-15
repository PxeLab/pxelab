import { useState, useRef, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { Palette, Sun, Moon } from 'lucide-react'
import type { Palette as PaletteType } from '../hooks/usePalette'

const SWATCHES: { key: PaletteType; color: string; ring: string }[] = [
  { key: 'ocean', color: '#3b82f6', ring: 'ring-blue-500' },
  { key: 'forest', color: '#14b8a6', ring: 'ring-teal-500' },
  { key: 'sunset', color: '#f59e0b', ring: 'ring-amber-500' },
  { key: 'violet', color: '#8b5cf6', ring: 'ring-violet-500' },
  { key: 'rose', color: '#f43f5e', ring: 'ring-rose-500' },
  { key: 'midnight', color: '#3b82f6', ring: 'ring-blue-500' },
]

interface ThemeSwitcherProps {
  theme: 'dark' | 'light'
  palette: PaletteType
  onToggleTheme: () => void
  onChangePalette: (p: PaletteType) => void
  macosCards?: boolean
  onToggleMacOS?: () => void
}

export function ThemeSwitcher({ theme, palette, onToggleTheme, onChangePalette, macosCards, onToggleMacOS }: ThemeSwitcherProps) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', handleClick)
    return () => document.removeEventListener('mousedown', handleClick)
  }, [])

  return (
    <div ref={ref} className="relative">
      <button
        onClick={() => setOpen(!open)}
        className="rounded-lg p-2 text-[var(--text-muted)] hover:bg-[var(--bg-card)] hover:text-[var(--text-primary)] transition-colors"
        title={t('themeSwitcher.title')}
      >
        <Palette size={18} />
      </button>
      {open && (
        <div className="absolute right-0 top-full mt-2 w-[240px] z-50 rounded-xl bg-[var(--bg-elevated)] border border-[var(--bg-border)] shadow-xl overflow-hidden animate-fade-in">
          {/* Palette grid */}
          <div className="px-3 py-3 border-b border-[var(--bg-border)]">
            <div className="text-[11px] font-semibold text-[var(--text-secondary)] mb-2.5">{t('themeSwitcher.palette')}</div>
            <div className="grid grid-cols-6 gap-2">
              {SWATCHES.map(s => (
                <button
                  key={s.key}
                  onClick={() => { onChangePalette(s.key); setOpen(false) }}
                  className={`w-7 h-7 rounded-full transition-all duration-200 hover:scale-110 hover:shadow-md ${
                    palette === s.key ? 'ring-2 ring-offset-2 ring-offset-[var(--bg-elevated)] scale-110' : 'opacity-60 hover:opacity-100'
                  }`}
                  style={{ backgroundColor: s.color }}
                  title={t(`themeSwitcher.palette_${s.key}`)}
                />
              ))}
            </div>
          </div>
          {/* Dark/light toggle */}
          <div className="px-3 py-2.5 border-b border-[var(--bg-border)]">
            <button
              onClick={() => { onToggleTheme(); setOpen(false) }}
              className="w-full flex items-center justify-between px-3 py-2 rounded-lg text-sm text-[var(--text-secondary)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)] transition-colors"
            >
              <span className="text-xs font-medium">{t('themeSwitcher.mode')}</span>
              <span className="flex items-center gap-1.5 text-xs">
                {theme === 'dark' ? (
                  <><Sun size={14} /> {t('themeSwitcher.light')}</>
                ) : (
                  <><Moon size={14} /> {t('themeSwitcher.dark')}</>
                )}
              </span>
            </button>
          </div>
          {/* macOS Card toggle */}
          <div className="px-3 py-2.5">
            <button
              onClick={() => { onToggleMacOS?.(); setOpen(false) }}
              className={`w-full flex items-center justify-between px-3 py-2 rounded-lg text-sm transition-colors ${
                macosCards ? 'bg-blue-500/10 text-blue-400' : 'text-[var(--text-secondary)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]'
              }`}
            >
              <span className="text-xs font-medium">macOS Cards</span>
              <span className={`text-[10px] px-2 py-0.5 rounded-full font-medium ${
                macosCards ? 'bg-blue-500/20 text-blue-400' : 'bg-[var(--bg-input)] text-[var(--text-muted)]'
              }`}>
                {macosCards ? 'ON' : 'OFF'}
              </span>
            </button>
          </div>
        </div>
      )}
    </div>
  )
}