import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { Activity, Server, Clock, Zap, ChevronRight, Wifi } from 'lucide-react'
import { StatusDot } from '../components/ui/StatusDot'
import { Card } from '../components/ui/Card'
import { api, type ServiceStatus, type Host, type Event } from '../api/client'

export default function Dashboard() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [status, setStatus] = useState<ServiceStatus | null>(null)
  const [hosts, setHosts] = useState<Host[]>([])
  const [events, setEvents] = useState<Event[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    async function load() {
      try {
        const [s, h, e] = await Promise.all([
          api.getStatus(),
          api.getHosts({ page: '1', size: '8' }),
          api.getEvents({ page: '1', size: '8' }),
        ])
        setStatus(s.data)
        setHosts(h.data.hosts ?? [])
        setEvents(e.data.events ?? [])
      } catch (err) {
        console.error('Failed to load dashboard', err)
      } finally {
        setLoading(false)
      }
    }
    load()
  }, [])

  const serviceColor = (svc: string) => {
    if (!status?.services) return 'yellow'
    const s = status.services[svc] || ''
    if (s === 'running') return 'green'
    if (s === 'error') return 'red'
    return 'yellow'
  }

  const serviceStatus = (svc: string): 'running' | 'stopped' | 'error' => {
    if (!status?.services) return 'stopped'
    const s = status.services[svc] || ''
    if (s === 'running') return 'running'
    if (s === 'error') return 'error'
    return 'stopped'
  }

  const services = [
    { name: 'DHCP', port: ':67', key: 'dhcp' },
    { name: 'TFTP', port: ':69', key: 'tftp' },
    { name: 'HTTP', port: ':8080', key: 'http' },
    { name: 'DNS', port: ':53', key: 'dns' },
  ]

  const stats = [
    { label: t('dashboard.stats.onlineHosts'), value: hosts.length, color: 'green' as const, icon: Server, gradient: 'from-green-500/20 to-emerald-500/5' },
    { label: t('dashboard.stats.runningServices'), value: services.filter(s => serviceStatus(s.key) === 'running').length, color: 'blue' as const, icon: Wifi, gradient: 'from-blue-500/20 to-cyan-500/5' },
    { label: t('dashboard.stats.totalEvents'), value: events.length, color: 'orange' as const, icon: Activity, gradient: 'from-orange-500/20 to-amber-500/5' },
    { label: t('dashboard.stats.uptime'), value: status ? `${Math.floor(status.uptime / 60)}m` : '—', color: 'purple' as const, icon: Clock, gradient: 'from-purple-500/20 to-pink-500/5' },
  ]

  const eventIcon = (type: string) => {
    const t = type.toLowerCase()
    if (t.includes('dhcp')) return { label: 'DHCP', color: 'cyan' as const }
    if (t.includes('tftp')) return { label: 'TFTP', color: 'orange' as const }
    if (t.includes('http')) return { label: 'HTTP', color: 'purple' as const }
    if (t.includes('boot')) return { label: 'BOOT', color: 'green' as const }
    if (t.includes('ipmi')) return { label: 'IPMI', color: 'yellow' as const }
    return { label: 'EVENT', color: 'blue' as const }
  }

  const statusColorMap: Record<string, string> = {
    running: 'bg-green-500',
    stopped: 'bg-gray-400',
    error: 'bg-red-500',
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-2xl font-bold text-[var(--text-primary)] tracking-tight">{t('dashboard.title')}</h1>
        <p className="text-sm text-[var(--text-muted)] mt-1">{t('dashboard.description')}</p>
      </div>

      {/* Service Status Bar - Glass Card */}
      <div className="relative overflow-hidden rounded-2xl border border-[var(--bg-border)] bg-[var(--bg-card)] shadow-sm">
        <div className="absolute inset-0 bg-gradient-to-r from-blue-500/5 via-transparent to-purple-500/5" />
        <div className="relative px-5 py-4">
          <div className="flex items-center justify-between mb-3">
            <div className="flex items-center gap-2">
              <Zap size={14} className="text-blue-400" />
              <span className="text-xs font-semibold uppercase tracking-wider text-[var(--text-muted)]">{t('dashboard.serviceStatus')}</span>
            </div>
            <div className="flex items-center gap-1.5">
              {services.map(svc => {
                const st = serviceStatus(svc.key)
                return (
                  <span key={svc.key} className={`w-2 h-2 rounded-full ${statusColorMap[st]}`} />
                )
              })}
            </div>
          </div>
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
            {services.map(svc => {
              const st = serviceStatus(svc.key)
              const color = serviceColor(svc.key)
              return (
                <div key={svc.name} className="flex items-center gap-3 px-3 py-2.5 rounded-xl bg-[var(--bg-base)]/50 border border-[var(--bg-border)]/50">
                  <StatusDot color={color as any} pulse={st === 'running'} />
                  <div className="min-w-0">
                    <div className="text-sm font-semibold text-[var(--text-primary)]">{svc.name}</div>
                    <div className="text-[10px] font-mono text-[var(--text-muted)]">{svc.port}</div>
                  </div>
                  <span className={`ml-auto text-[10px] font-semibold uppercase tracking-wide ${
                    st === 'running' ? 'text-green-400' : st === 'error' ? 'text-red-400' : 'text-[var(--text-muted)]'
                  }`}>
                    {st === 'running' ? t('common.running') : st === 'error' ? t('common.error') : t('common.stopped')}
                  </span>
                </div>
              )
            })}
          </div>
        </div>
      </div>

      {/* Stats Grid - Gradient Cards */}
      {loading ? (
        <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
          {Array.from({ length: 4 }).map((_, i) => (
            <div key={i} className="rounded-2xl border border-[var(--bg-border)] bg-[var(--bg-card)] p-5 animate-shimmer" />
          ))}
        </div>
      ) : (
        <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
          {stats.map((s, i) => {
            const Icon = s.icon
            return (
              <div key={i} className={`group relative overflow-hidden rounded-2xl border border-[var(--bg-border)] bg-[var(--bg-card)] p-5 shadow-sm hover:shadow-lg hover:-translate-y-0.5 transition-all duration-300`}>
                <div className={`absolute inset-0 bg-gradient-to-br ${s.gradient} opacity-0 group-hover:opacity-100 transition-opacity duration-500`} />
                <div className="relative">
                  <div className="flex items-center justify-between mb-4">
                    <span className="text-xs font-semibold uppercase tracking-wider text-[var(--text-muted)]">{s.label}</span>
                    <span className={`w-10 h-10 rounded-xl flex items-center justify-center transition-all duration-300 group-hover:scale-110 ${
                      s.color === 'green' ? 'bg-green-500/10 text-green-400' :
                      s.color === 'blue' ? 'bg-blue-500/10 text-blue-400' :
                      s.color === 'orange' ? 'bg-orange-500/10 text-orange-400' :
                      'bg-purple-500/10 text-purple-400'
                    }`}>
                      <Icon size={18} />
                    </span>
                  </div>
                  <div className="text-3xl font-bold tracking-tight text-[var(--text-primary)]">{s.value}</div>
                </div>
              </div>
            )
          })}
        </div>
      )}

      {/* Main Content: Events + Hosts */}
      <div className="grid grid-cols-1 lg:grid-cols-5 gap-5">
        {/* Recent Events - Takes 3 cols */}
        <div className="lg:col-span-3">
          <Card
            padding={false}
            title={
              <div className="flex items-center gap-2">
                <Activity size={14} className="text-blue-400" />
                <span>{t('dashboard.recentEvents')}</span>
              </div>
            }
            footer={
              <button onClick={() => navigate('/events')} className="flex items-center gap-1 text-blue-400 hover:text-blue-300 text-xs font-medium transition-colors">
                {t('common.viewAll')} <ChevronRight size={12} />
              </button>
            }
          >
            {events.length === 0 ? (
              <p className="text-sm text-[var(--text-muted)] text-center py-8">{t('dashboard.noEvents')}</p>
            ) : (
              <div>
                {events.slice(0, 6).map((e, i) => {
                  const ic = eventIcon(e.type)
                  return (
                    <div key={i} className="flex items-center gap-3 px-5 py-3 border-b border-[var(--bg-border)] last:border-b-0 hover:bg-[var(--bg-hover)]/30 transition-colors group">
                      <div className={`w-8 h-8 rounded-lg flex items-center justify-center text-[10px] font-bold shrink-0 ${
                        ic.color === 'cyan' ? 'bg-cyan-500/10 text-cyan-400' :
                        ic.color === 'orange' ? 'bg-orange-500/10 text-orange-400' :
                        ic.color === 'purple' ? 'bg-purple-500/10 text-purple-400' :
                        ic.color === 'green' ? 'bg-green-500/10 text-green-400' :
                        ic.color === 'yellow' ? 'bg-yellow-500/10 text-yellow-400' :
                        'bg-blue-500/10 text-blue-400'
                      }`}>{ic.label}</div>
                      <div className="flex-1 min-w-0">
                        <div className="flex items-center gap-2">
                          <span className="text-xs font-semibold text-[var(--text-primary)]">{e.type}</span>
                          {e.mac && <span className="text-[10px] font-mono text-[var(--text-muted)]">{e.mac}</span>}
                        </div>
                        <p className="text-[11px] text-[var(--text-muted)] truncate">{e.message}</p>
                      </div>
                      <span className="text-[10px] text-[var(--text-muted)] font-mono shrink-0 opacity-0 group-hover:opacity-100 transition-opacity">
                        {new Date(e.timestamp).toLocaleTimeString()}
                      </span>
                    </div>
                  )
                })}
              </div>
            )}
          </Card>
        </div>

        {/* Recent Hosts - Takes 2 cols */}
        <div className="lg:col-span-2">
          <Card
            padding={false}
            title={
              <div className="flex items-center gap-2">
                <Server size={14} className="text-green-400" />
                <span>{t('dashboard.recentHosts')}</span>
              </div>
            }
            footer={
              <button onClick={() => navigate('/hosts')} className="flex items-center gap-1 text-blue-400 hover:text-blue-300 text-xs font-medium transition-colors">
                {t('common.viewAll')} <ChevronRight size={12} />
              </button>
            }
          >
            {hosts.length === 0 ? (
              <p className="text-sm text-[var(--text-muted)] text-center py-8">{t('hosts.empty')}</p>
            ) : (
              <div>
                {hosts.slice(0, 6).map(h => (
                  <div
                    key={h.id}
                    onClick={() => navigate('/hosts/' + h.id)}
                    className="flex items-center gap-3 px-5 py-3 border-b border-[var(--bg-border)] last:border-b-0 hover:bg-[var(--bg-hover)]/30 transition-colors cursor-pointer group"
                  >
                    <StatusDot color={h.last_online ? 'green' : 'red'} />
                    <div className="flex-1 min-w-0">
                      <div className="text-sm font-medium text-[var(--text-primary)] truncate">{h.name || h.mac}</div>
                      <div className="text-[10px] font-mono text-[var(--text-muted)]">{h.ip}</div>
                    </div>
                    <ChevronRight size={14} className="text-[var(--text-muted)] opacity-0 group-hover:opacity-100 transition-opacity" />
                  </div>
                ))}
              </div>
            )}
          </Card>
        </div>
      </div>
    </div>
  )
}
