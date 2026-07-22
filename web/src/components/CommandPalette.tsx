import { type FC, useEffect, useState } from 'react'
import { createPortal } from 'react-dom'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { Command } from 'cmdk'
import { Search, Compass, History, Server } from 'lucide-react'
import { getHosts, type Host } from '../api/client'

export interface PaletteNavItem {
  path: string
  label: string
  group: string
}

export interface PaletteRecentItem {
  path: string
  label: string
}

interface Props {
  open: boolean
  onClose: () => void
  navItems: PaletteNavItem[]
  recent: PaletteRecentItem[]
}

const itemClass =
  'flex items-center gap-2.5 px-3 py-2 rounded-lg text-sm cursor-pointer text-[var(--foreground-secondary)] data-[selected=true]:bg-[var(--hover)] data-[selected=true]:text-[var(--foreground)] transition-colors'

const groupClass =
  '[&_[cmdk-group-heading]]:px-3 [&_[cmdk-group-heading]]:pt-2.5 [&_[cmdk-group-heading]]:pb-1 [&_[cmdk-group-heading]]:text-[10px] [&_[cmdk-group-heading]]:uppercase [&_[cmdk-group-heading]]:tracking-wider [&_[cmdk-group-heading]]:font-semibold [&_[cmdk-group-heading]]:text-[var(--foreground-muted)]'

export const CommandPalette: FC<Props> = ({ open, onClose, navItems, recent }) => {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [query, setQuery] = useState('')
  const [hosts, setHosts] = useState<Host[]>([])
  const [loading, setLoading] = useState(false)

  // 远程主机搜索：≥2 字符，300ms 防抖
  useEffect(() => {
    if (!open) return
    const q = query.trim()
    if (q.length < 2) {
      setHosts([])
      setLoading(false)
      return
    }
    setLoading(true)
    const timer = setTimeout(async () => {
      try {
        const res = await getHosts({ search: q, size: 5 })
        setHosts(res.data.hosts ?? [])
      } catch {
        setHosts([])
      } finally {
        setLoading(false)
      }
    }, 300)
    return () => clearTimeout(timer)
  }, [query, open])

  // Esc 关闭（面板打开期间）
  useEffect(() => {
    if (!open) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [open, onClose])

  if (!open) return null

  const go = (path: string) => {
    navigate(path)
    onClose()
  }

  const showHosts = query.trim().length >= 2

  return createPortal(
    <div
      className="fixed inset-0 z-50 flex items-start justify-center pt-[14vh] bg-black/60 backdrop-blur-sm animate-overlay-in"
      onClick={(e) => { if (e.target === e.currentTarget) onClose() }}
    >
      <div className="w-[560px] max-w-[92vw] bg-[var(--popover)] border border-[var(--border)] rounded-2xl shadow-xl overflow-hidden animate-modal-in">
        <Command label={t('commandPalette.placeholder')} loop>
          <div className="flex items-center gap-2.5 px-4 border-b border-[var(--border)]">
            <Search size={16} className="shrink-0 text-[var(--foreground-muted)]" />
            <Command.Input
              autoFocus
              value={query}
              onValueChange={setQuery}
              placeholder={t('commandPalette.placeholder')}
              className="flex-1 py-3.5 bg-transparent outline-none text-sm text-[var(--foreground)] placeholder-[var(--foreground-muted)]"
            />
          </div>
          <Command.List className="max-h-[360px] overflow-y-auto p-2">
            <Command.Empty className="py-10 text-center text-sm text-[var(--foreground-muted)]">
              {loading ? t('commandPalette.loading') : t('commandPalette.empty')}
            </Command.Empty>

            {recent.length > 0 && (
              <Command.Group heading={t('commandPalette.recent')} className={groupClass}>
                {recent.map(r => (
                  <Command.Item key={`recent-${r.path}`} value={`${r.label} ${r.path}`} onSelect={() => go(r.path)} className={itemClass}>
                    <History size={15} className="shrink-0 opacity-60" />
                    <span className="flex-1 truncate">{r.label}</span>
                    <span className="text-[10px] font-mono text-[var(--foreground-muted)] truncate">{r.path}</span>
                  </Command.Item>
                ))}
              </Command.Group>
            )}

            <Command.Group heading={t('commandPalette.navigation')} className={groupClass}>
              {navItems.map(item => (
                <Command.Item key={item.path} value={`${item.label} ${item.group} ${item.path}`} onSelect={() => go(item.path)} className={itemClass}>
                  <Compass size={15} className="shrink-0 opacity-60" />
                  <span className="flex-1 truncate">{item.label}</span>
                  <span className="text-[10px] text-[var(--foreground-muted)] truncate">{item.group}</span>
                </Command.Item>
              ))}
            </Command.Group>

            {showHosts && hosts.length > 0 && (
              <Command.Group heading={t('commandPalette.hosts')} className={groupClass}>
                {hosts.map(h => (
                  <Command.Item
                    key={`host-${h.id}`}
                    value={h.name}
                    keywords={[h.mac, h.ip]}
                    onSelect={() => go(`/hosts/${h.id}`)}
                    className={itemClass}
                  >
                    <Server size={15} className="shrink-0 opacity-60" />
                    <span className="flex-1 truncate">{h.name}</span>
                    <span className="text-[10px] font-mono text-[var(--foreground-muted)]">{h.mac}</span>
                  </Command.Item>
                ))}
              </Command.Group>
            )}
          </Command.List>
        </Command>
      </div>
    </div>,
    document.body
  )
}
