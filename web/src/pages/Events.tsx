import { useState, useEffect, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import { Pause, Play, Maximize, Minimize } from 'lucide-react'
import { Card } from '../components/ui/Card'
import { Tag } from '../components/ui/Tag'
import { StatusDot } from '../components/ui/StatusDot'
import { Button } from '../components/ui/Button'
import { Pagination } from '../components/ui/Pagination'
import { PageHeader } from '../components/ui/PageHeader'
import { useToast } from '../components/ui/Toast'
import { api, type Event } from '../api/client'
import { useUIConfig } from '../contexts/UIConfigContext'
import { useSSE } from '../hooks/useSSE'
import { useFullscreen } from '../hooks/useFullscreen'

const filterChips = [
  { key: '', label: 'events.all', color: 'blue' as const },
  { key: 'dhcp', label: 'DHCP', color: 'cyan' as const },
  { key: 'tftp', label: 'TFTP', color: 'orange' as const },
  { key: 'http', label: 'HTTP', color: 'purple' as const },
  { key: 'boot', label: 'BOOT', color: 'green' as const },
  { key: 'wol', label: 'WOL', color: 'yellow' as const },
  { key: 'ipmi', label: 'IPMI', color: 'yellow' as const },
]

export default function Events() {
  const { t } = useTranslation()
  const { error: toastError } = useToast()
  const [events, setEvents] = useState<Event[]>([])
  const [loading, setLoading] = useState(true)
  const [page, setPage] = useState(1)
  const [total, setTotal] = useState(0)
  const [filter, setFilter] = useState('')
  const scrollRef = useRef<HTMLDivElement>(null)
  const { ref: fullscreenRef, isFullscreen, toggle: toggleFullscreen } = useFullscreen<HTMLDivElement>()

  const { pageSize } = useUIConfig()

  const { connected, pause, resume, paused } = useSSE<Event>(
    '/api/v1/events/stream',
    (evt) => {
      if (filter && !evt.type.toLowerCase().includes(filter)) return
      setEvents(prev => [evt, ...prev].slice(0, pageSize * 3))
    },
    { bufferLimit: 100 },
  )

  useEffect(() => {
    loadEvents()
  }, [page, filter])

  async function loadEvents() {
    setLoading(true)
    try {
      const params: Record<string, string> = { page: String(page), size: String(pageSize) }
      if (filter) params.type = filter
      const res = await api.getEvents(params)
      setEvents(res.data.events)
      setTotal(res.data.meta.total)
    } catch { toastError(t('events.loadFailed')) }
    finally { setLoading(false) }
  }

  const eventIcon = (type: string) => {
    const lowerType = type.toLowerCase()
    if (lowerType.includes('dhcp')) return { label: 'DHCP', color: 'cyan' as const }
    if (lowerType.includes('tftp')) return { label: 'TFTP', color: 'orange' as const }
    if (lowerType.includes('http')) return { label: 'HTTP', color: 'purple' as const }
    if (lowerType.includes('boot')) return { label: 'BOOT', color: 'green' as const }
    if (lowerType.includes('wol')) return { label: 'WOL', color: 'yellow' as const }
    if (lowerType.includes('ipmi')) return { label: 'IPMI', color: 'yellow' as const }
    return { label: 'EVENT', color: 'blue' as const }
  }

  return (
    <div>
      <PageHeader
        title={t('events.title')}
        actions={
          <>
            <span className={`inline-flex items-center gap-1.5 px-2.5 py-1.5 rounded-full text-xs font-mono font-semibold ${paused ? 'bg-accent-yellow/10 text-accent-yellow' : connected ? 'bg-accent-green/10 text-accent-green' : 'bg-accent-red/10 text-accent-red'}`}>
              <StatusDot color={paused ? 'yellow' : connected ? 'green' : 'red'} pulse={!paused && connected} />
              {paused ? t('events.paused') : connected ? t('events.live') : t('settings.disconnected')}
              <span className="text-[var(--text-muted)] font-normal mx-0.5">·</span>
              <span className="text-[var(--text-muted)] font-normal">{total}+</span>
            </span>
            <Button variant={paused ? 'primary' : 'secondary'} size="sm" onClick={paused ? resume : pause}>
              {paused ? <Play size={14} /> : <Pause size={14} />}
              {paused ? t('events.resume') : t('events.pause')}
            </Button>
            <span title={isFullscreen ? t('common.exitFullscreen') : t('common.fullscreen')}>
              <Button variant="ghost" size="sm" onClick={toggleFullscreen}>
                {isFullscreen ? <Minimize size={14} /> : <Maximize size={14} />}
              </Button>
            </span>
          </>
        }
      />

      {/* Filter chips */}
      <div className="flex gap-2 flex-wrap mb-5">
        {filterChips.map(chip => (
          <button
            key={chip.key}
            onClick={() => { setFilter(chip.key); setPage(1) }}
            className={`inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg border text-xs font-semibold transition-all ${
              filter === chip.key
                ? 'bg-blue-500/10 text-blue-400 border-blue-500/30'
                : 'border-[var(--bg-border)] bg-[var(--bg-card)] text-[var(--text-secondary)] hover:border-[var(--text-muted)]'
            }`}
          >
            <StatusDot color={chip.color} />
            {chip.key === '' ? t('events.all') : chip.label}
          </button>
        ))}
      </div>

      {/* Event list */}
      <div
        ref={el => { scrollRef.current = el; fullscreenRef.current = el }}
        className={isFullscreen ? 'h-full overflow-y-auto bg-[var(--background)] p-4' : ''}
      >
        <Card padding={false}>
          {loading ? (
            <div className="p-5 space-y-3">
              {Array.from({ length: 5 }).map((_, i) => (
                <div key={i} className="h-12 bg-[var(--bg-card)] rounded animate-shimmer" />
              ))}
            </div>
          ) : events.length === 0 ? (
            <p className="text-sm text-[var(--text-muted)] text-center py-12">{t('events.noEvents')}</p>
          ) : (
            <div className="overflow-x-auto">
              {/* Header */}
              <div className="flex items-center gap-3 px-5 py-2 text-[11px] font-semibold text-[var(--text-muted)] uppercase tracking-wider border-b border-[var(--bg-border)] whitespace-nowrap">
                <span className="w-7 shrink-0" />
                <span className="w-14 shrink-0">{t('events.type', 'Type')}</span>
                <span className="w-36 shrink-0">{t('events.time', 'Time')}</span>
                <span className="w-12 shrink-0 text-center">{t('events.tag', 'Tag')}</span>
                <span className="flex-1 min-w-0">{t('events.message', 'Message')}</span>
                <span className="w-44 shrink-0">{t('events.mac', 'MAC')}</span>
                <span className="w-32 shrink-0">{t('events.ip', 'IP')}</span>
              </div>
              <div className="divide-y divide-[var(--bg-border)]">
                {events.map((e, i) => {
                  const ic = eventIcon(e.type)
                  return (
                    <div key={e.id || i} className="flex items-center gap-3 px-5 py-2 hover:bg-[var(--bg-hover)]/50 transition-colors text-xs whitespace-nowrap">
                      <div className={`w-7 h-7 rounded-md flex items-center justify-center text-[10px] font-bold shrink-0 ${
                        ic.color === 'cyan' ? 'bg-cyan-500/10 text-cyan-400' :
                        ic.color === 'orange' ? 'bg-orange-500/10 text-orange-400' :
                        ic.color === 'purple' ? 'bg-purple-500/10 text-purple-400' :
                        ic.color === 'green' ? 'bg-accent-green/10 text-accent-green' :
                        ic.color === 'yellow' ? 'bg-accent-yellow/10 text-accent-yellow' :
                        'bg-blue-500/10 text-blue-400'
                      }`}>{ic.label}</div>
                      <span className="w-14 shrink-0 font-semibold text-[var(--text-primary)]">{e.type}</span>
                      <span className="w-36 shrink-0 text-[var(--text-muted)] font-mono">{new Date(e.timestamp).toLocaleString()}</span>
                      <Tag color={ic.color}>{ic.label}</Tag>
                      <span className="flex-1 min-w-0 text-[var(--text-muted)] truncate">{e.message}</span>
                      <span className="w-44 shrink-0 text-[var(--text-secondary)] font-mono truncate">{e.mac || '-'}</span>
                      <span className="w-32 shrink-0 text-[var(--text-secondary)] font-mono truncate">{e.ip || '-'}</span>
                    </div>
                  )
                })}
              </div>
            </div>
          )}
          <div className="px-5 py-3 border-t border-[var(--bg-border)]">
            <Pagination page={page} total={total} size={pageSize} onChange={setPage} />
          </div>
        </Card>
      </div>
    </div>
  )
}
