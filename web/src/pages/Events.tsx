import { useState, useEffect, useRef, useCallback } from 'react'
import { useTranslation } from 'react-i18next'
import { Pause, Play } from 'lucide-react'
import { Card } from '../components/ui/Card'
import { Tag } from '../components/ui/Tag'
import { StatusDot } from '../components/ui/StatusDot'
import { Button } from '../components/ui/Button'
import { Pagination } from '../components/ui/Pagination'
import { useToast } from '../components/ui/Toast'
import { api, type Event } from '../api/client'
import { useUIConfig } from '../contexts/UIConfigContext'

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
  const [paused, setPaused] = useState(false)
  const [liveEvents, setLiveEvents] = useState<Event[]>([])
  const scrollRef = useRef<HTMLDivElement>(null)
  const esRef = useRef<EventSource | null>(null)

  const { pageSize } = useUIConfig()

  useEffect(() => {
    loadEvents()
  }, [page, filter])

  useEffect(() => {
    connectSSE()
    return () => { esRef.current?.close() }
  }, [filter])

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

  function connectSSE() {
    esRef.current?.close()
    const es = new EventSource('/api/v1/events/stream')
    es.onmessage = (e) => {
      try {
        const evt: Event = JSON.parse(e.data)
        if (filter && !evt.type.toLowerCase().includes(filter)) return
        if (paused) {
          setLiveEvents(prev => [evt, ...prev].slice(0, 100))
          return
        }
        setEvents(prev => [evt, ...prev].slice(0, pageSize * 3))
      } catch { /* ignore */ }
    }
    es.onerror = () => {
      es.close()
      setTimeout(connectSSE, 3000)
    }
    esRef.current = es
  }

  const togglePause = useCallback(() => {
    if (paused) {
      setPaused(false)
      const batch = liveEvents
      setLiveEvents([])
      setEvents(prev => [...batch, ...prev].slice(0, pageSize * 3))
    } else {
      setPaused(true)
    }
  }, [paused, liveEvents])

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
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-lg font-bold text-[var(--text-primary)]">{t('events.title')}</h1>
        <div className="flex items-center gap-3">
          <span className={`inline-flex items-center gap-1.5 px-2.5 py-1.5 rounded-full text-xs font-mono font-semibold ${!paused ? 'bg-green-500/10 text-green-400' : 'bg-yellow-500/10 text-yellow-400'}`}>
            <StatusDot color={!paused ? 'green' : 'yellow'} pulse={!paused} />
            {!paused ? t('events.live') : t('events.paused')}
            <span className="text-[var(--text-muted)] font-normal mx-0.5">·</span>
            <span className="text-[var(--text-muted)] font-normal">{total}+</span>
          </span>
          <Button variant={paused ? 'primary' : 'secondary'} size="sm" onClick={togglePause}>
            {paused ? <Play size={14} /> : <Pause size={14} />}
            {paused ? t('events.resume') : t('events.pause')}
          </Button>
        </div>
      </div>

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
      <div ref={scrollRef}>
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
                        ic.color === 'green' ? 'bg-green-500/10 text-green-400' :
                        ic.color === 'yellow' ? 'bg-yellow-500/10 text-yellow-400' :
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
