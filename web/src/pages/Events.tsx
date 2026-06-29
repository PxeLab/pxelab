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

const filterChips = [
  { key: '', label: 'events.all', color: 'blue' as const },
  { key: 'dhcp', label: 'DHCP', color: 'cyan' as const },
  { key: 'tftp', label: 'TFTP', color: 'orange' as const },
  { key: 'http', label: 'HTTP', color: 'purple' as const },
  { key: 'boot', label: 'BOOT', color: 'green' as const },
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

  const size = 20

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
      const params: Record<string, string> = { page: String(page), size: String(size) }
      if (filter) params.type = filter
      const res = await api.getEvents(params)
      setEvents(res.data.events)
      setTotal(res.data.meta.total)
    } catch { toastError('加载事件失败') }
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
        setEvents(prev => [evt, ...prev].slice(0, size * 3))
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
      setEvents(prev => [...batch, ...prev].slice(0, size * 3))
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
    if (lowerType.includes('ipmi')) return { label: 'IPMI', color: 'yellow' as const }
    return { label: 'EVENT', color: 'blue' as const }
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-lg font-bold text-[var(--text-primary)]">事件</h1>
        <div className="flex gap-2">
          <Button variant={paused ? 'primary' : 'secondary'} size="sm" onClick={togglePause}>
            {paused ? <Play size={14} /> : <Pause size={14} />}
            {paused ? t('events.resume', '继续') : t('events.pause', '暂停')}
          </Button>
        </div>
      </div>
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center gap-3">
          <span className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-mono font-semibold ${!paused ? 'bg-green-500/10 text-green-400' : 'bg-yellow-500/10 text-yellow-400'}`}>
            <StatusDot color={!paused ? 'green' : 'yellow'} pulse={!paused} />
            {!paused ? t('events.live', '实时') : t('events.paused', '已暂停')}
          </span>
          <span className="text-xs text-[var(--text-muted)]">{t('common.total', '共')} {total}+ {t('common.items', '条')}</span>
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
                : 'border-[var(--bg-border)] bg-[var(--bg-card)] text-[var(--text-secondary)] hover:border-[#2e3245]'
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
                <div key={i} className="h-12 bg-[var(--bg-card)] rounded animate-[shimmer_1.5s_infinite] bg-gradient-to-r from-[var(--bg-card)] via-[var(--bg-hover)] to-[var(--bg-card)] bg-[length:200%_100%]" />
              ))}
            </div>
          ) : events.length === 0 ? (
            <p className="text-sm text-[var(--text-muted)] text-center py-12">{t('events.noEvents')}</p>
          ) : (
            <div className="divide-y divide-[#232738]">
              {events.map((e, i) => {
                const ic = eventIcon(e.type)
                return (
                  <div key={e.id || i} className="flex items-start gap-3 px-5 py-3 hover:bg-white/[0.01] transition-colors">
                    <div className={`w-7 h-7 rounded-md flex items-center justify-center text-[10px] font-bold shrink-0 mt-0.5 ${
                      ic.color === 'cyan' ? 'bg-cyan-500/10 text-cyan-400' :
                      ic.color === 'orange' ? 'bg-orange-500/10 text-orange-400' :
                      ic.color === 'purple' ? 'bg-purple-500/10 text-purple-400' :
                      ic.color === 'green' ? 'bg-green-500/10 text-green-400' :
                      ic.color === 'yellow' ? 'bg-yellow-500/10 text-yellow-400' :
                      'bg-blue-500/10 text-blue-400'
                    }`}>{ic.label}</div>
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-2 mb-0.5">
                        <span className="text-xs font-semibold text-[var(--text-primary)]">{e.type}</span>
                        <Tag color={ic.color}>{ic.label}</Tag>
                        <span className="text-[11px] text-[var(--text-muted)] font-mono ml-auto shrink-0">
                          {new Date(e.timestamp).toLocaleTimeString()}
                        </span>
                      </div>
                      <p className="text-xs text-[var(--text-muted)]">
                        {e.message}
                        {e.mac && <> — <strong className="text-[var(--text-secondary)] font-semibold">{e.mac}</strong></>}
                        {e.ip && <> · {e.ip}</>}
                      </p>
                    </div>
                  </div>
                )
              })}
            </div>
          )}
          <div className="px-5 py-3 border-t border-[var(--bg-border)]">
            <Pagination page={page} total={total} size={size} onChange={setPage} />
          </div>
        </Card>
      </div>
    </div>
  )
}
