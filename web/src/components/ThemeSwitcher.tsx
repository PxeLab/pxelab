import { useState, useRef, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { Palette, Sun, Moon, Monitor } from 'lucide-react'
import { PALETTES, type Palette as PaletteType } from '../hooks/usePalette'
import { RADIUS_PRESETS, type RadiusPreset } from '../hooks/useRadius'
import type { Theme } from '../hooks/useTheme'

const PALETTE_KEYS = Object.keys(PALETTES) as PaletteType[]
const THEME_OPTIONS: { key: Theme; icon: typeof Sun; labelKey: string }[] = [
  { key: 'light', icon: Sun, labelKey: 'themeSwitcher.light' },
  { key: 'dark', icon: Moon, labelKey: 'themeSwitcher.dark' },
  { key: 'system', icon: Monitor, labelKey: 'themeSwitcher.system' },
]
const RADIUS_KEYS = Object.keys(RADIUS_PRESETS) as RadiusPreset[]

interface ThemeSwitcherProps {
  theme: Theme
  palette: PaletteType
  onSetTheme: (t: Theme, origin?: { x: number; y: number }) => void
  onChangePalette: (p: PaletteType) => void
  radius: RadiusPreset
  onChangeRadius: (r: RadiusPreset) => void
  macosCards?: boolean
  onToggleMacOS?: () => void
}

export function ThemeSwitcher({ theme, palette, onSetTheme, onChangePalette, radius, onChangeRadius, macosCards, onToggleMacOS }: ThemeSwitcherProps) {
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
        className="rounded-lg p-2 text-[var(--foreground-muted)] hover:bg-[var(--card)] hover:text-[var(--foreground)] transition-colors"
        title={t('themeSwitcher.title')}
      >
        <Palette size={18} />
      </button>
      {open && (
        <div className="absolute right-0 top-full mt-2 w-[240px] z-50 rounded-xl bg-[var(--popover)] border border-[var(--border)] shadow-xl overflow-hidden animate-fade-in">
          {/* Palette grid */}
          <div className="px-3 py-3 border-b border-[var(--border)]">
            <div className="text-[11px] font-semibold text-[var(--foreground-secondary)] mb-2.5">{t('themeSwitcher.palette')}</div>
            <div className="grid grid-cols-6 gap-2">
              {PALETTE_KEYS.map(key => (
                <button
                  key={key}
                  onClick={() => onChangePalette(key)}
                  className={`w-7 h-7 rounded-full transition-all duration-200 hover:scale-110 hover:shadow-md ${
                    palette === key ? 'ring-2 ring-offset-2 ring-offset-[var(--popover)] scale-110' : 'opacity-60 hover:opacity-100'
                  }`}
                  style={{ backgroundColor: PALETTES[key].primary }}
                  title={t(`themeSwitcher.palette_${key}`)}
                />
              ))}
            </div>
          </div>
          {/* Theme mode: light / dark / system */}
          <div className="px-3 py-2.5 border-b border-[var(--border)]">
            <div className="text-[11px] font-semibold text-[var(--foreground-secondary)] mb-2">{t('themeSwitcher.mode')}</div>
            <div className="flex rounded-lg bg-[var(--input)] p-0.5 gap-0.5">
              {THEME_OPTIONS.map(({ key, icon: Icon, labelKey }) => (
                <button
                  key={key}
                  onClick={(e) => onSetTheme(key, { x: e.clientX, y: e.clientY })}
                  title={t(labelKey)}
                  className={`flex-1 flex items-center justify-center gap-1 px-2 py-1.5 rounded-md text-xs transition-colors ${
                    theme === key
                      ? 'bg-[var(--card)] text-[var(--foreground)] shadow-sm'
                      : 'text-[var(--foreground-muted)] hover:text-[var(--foreground-secondary)]'
                  }`}
                >
                  <Icon size={13} />
                  {t(labelKey)}
                </button>
              ))}
            </div>
          </div>
          {/* Radius */}
          <div className="px-3 py-2.5 border-b border-[var(--border)]">
            <div className="text-[11px] font-semibold text-[var(--foreground-secondary)] mb-2">{t('themeSwitcher.radius')}</div>
            <div className="flex rounded-lg bg-[var(--input)] p-0.5 gap-0.5">
              {RADIUS_KEYS.map(key => (
                <button
                  key={key}
                  onClick={() => onChangeRadius(key)}
                  title={t(`themeSwitcher.radius_${key}`)}
                  className={`flex-1 px-1 py-1.5 rounded-md text-[10px] font-medium transition-colors ${
                    radius === key
                      ? 'bg-[var(--card)] text-[var(--foreground)] shadow-sm'
                      : 'text-[var(--foreground-muted)] hover:text-[var(--foreground-secondary)]'
                  }`}
                >
                  {t(`themeSwitcher.radius_${key}`)}
                </button>
              ))}
            </div>
          </div>
          {/* macOS Card toggle */}
          <div className="px-3 py-2.5">
            <button
              onClick={() => { onToggleMacOS?.(); setOpen(false) }}
              className={`w-full flex items-center justify-between px-3 py-2 rounded-lg text-sm transition-colors ${
                macosCards ? 'bg-blue-500/10 text-blue-400' : 'text-[var(--foreground-secondary)] hover:bg-[var(--hover)] hover:text-[var(--foreground)]'
              }`}
            >
              <span className="text-xs font-medium">macOS Cards</span>
              <span className={`text-[10px] px-2 py-0.5 rounded-full font-medium ${
                macosCards ? 'bg-blue-500/20 text-blue-400' : 'bg-[var(--input)] text-[var(--foreground-muted)]'
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
