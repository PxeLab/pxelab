import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { StatusDot } from '../components/ui/StatusDot'
import { Card } from '../components/ui/Card'
import { Tag } from '../components/ui/Tag'
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
          api.getHosts({ page: '1', size: '5' }),
          api.getEvents({ page: '1', size: '5' }),
        ])
        setStatus(s.data)
        setHosts(h.data.hosts)
        setEvents(e.data.events)
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

  const stats = [
    { label: t('dashboard.stats.onlineHosts'), value: hosts.length, color: 'green' as const, icon: '●' },
    { label: t('dashboard.stats.activeLeases'), value: status?.uptime ? '—' : '0', color: 'blue' as const, icon: '■' },
    { label: t('dashboard.stats.totalEvents'), value: events.length, color: 'orange' as const, icon: '≡' },
    { label: t('dashboard.stats.uptime'), value: status ? `${Math.floor(status.uptime / 60)}m` : '—', color: 'purple' as const, icon: '◆' },
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

  return (
    <div>
      {/* Service Bar */}
      <div className="flex gap-2 flex-wrap mb-7">
        {[
          { name: 'DHCP', port: ':67' },
          { name: 'TFTP', port: ':69' },
          { name: 'HTTP', port: ':8080' },
          { name: 'DNS', port: ':53' },
          { name: 'IPMI', port: 'Standby' },
        ].map(svc => (
          <div key={svc.name} className="inline-flex items-center gap-1.5 px-3.5 py-1.5 rounded-lg border border-[var(--bg-border)] bg-[var(--bg-card)] text-xs font-semibold">
            <StatusDot color={serviceColor(svc.name)} />
            <span className="text-[var(--text-secondary)] font-medium">{svc.name}</span>
            <span className="text-[var(--text-primary)] font-mono">{svc.port}</span>
          </div>
        ))}
      </div>

      {/* Stats Grid */}
      {loading ? (
        <div className="grid grid-cols-2 lg:grid-cols-4 gap-4 mb-7">
          {Array.from({ length: 4 }).map((_, i) => (
            <div key={i} className="bg-[var(--bg-card)] border border-[var(--bg-border)] rounded-xl p-5 animate-[shimmer_1.5s_infinite] bg-gradient-to-r from-[var(--bg-card)] via-[var(--bg-hover)] to-[var(--bg-card)] bg-[length:200%_100%]" />
          ))}
        </div>
      ) : (
        <div className="grid grid-cols-2 lg:grid-cols-4 gap-4 mb-7">
          {stats.map((s, i) => (
            <div key={i} className="bg-[var(--bg-card)] border border-[var(--bg-border)] rounded-xl p-5 hover:border-[#2e3245] transition-all">
              <div className="flex items-center justify-between mb-3">
                <span className="text-xs font-semibold uppercase tracking-wider text-[var(--text-muted)]">{s.label}</span>
                <span className={`w-8 h-8 rounded-lg flex items-center justify-center text-sm ${
                  s.color === 'green' ? 'bg-green-500/10 text-green-400' :
                  s.color === 'blue' ? 'bg-blue-500/10 text-blue-400' :
                  s.color === 'orange' ? 'bg-orange-500/10 text-orange-400' :
                  'bg-purple-500/10 text-purple-400'
                }`}>{s.icon}</span>
              </div>
              <div className="text-[28px] font-bold tracking-tight leading-none mb-1">{s.value}</div>
            </div>
          ))}
        </div>
      )}

      {/* Events + Hosts */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
        {/* Recent Events */}
        <Card title={t('dashboard.recentEvents')} footer={
          <button onClick={() => navigate('/events')} className="text-blue-400 hover:text-blue-300 transition-colors">
            {t('common.viewAll', '查看全部')} →
          </button>
        }>
          {events.length === 0 ? (
            <p className="text-sm text-[var(--text-muted)] text-center py-4">{t('dashboard.noEvents')}</p>
          ) : (
            <div className="flex flex-col -mx-5">
              {events.slice(0, 4).map((e, i) => {
                const ic = eventIcon(e.type)
                return (
                  <div key={i} className="flex items-start gap-3 px-5 py-2.5 border-b border-[var(--bg-border)] last:border-b-0">
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
                        <Tag color={ic.color} children={ic.label} />
                        <span className="text-[11px] text-[var(--text-muted)] font-mono ml-auto shrink-0">
                          {new Date(e.timestamp).toLocaleTimeString()}
                        </span>
                      </div>
                      <p className="text-xs text-[var(--text-muted)]">
                        {e.message} — <strong className="text-[var(--text-secondary)] font-semibold">{e.mac || ''}</strong>
                      </p>
                    </div>
                  </div>
                )
              })}
            </div>
          )}
        </Card>

        {/* Recent Hosts */}
        <Card title={t('dashboard.recentHosts', '最近上线的主机')} footer={
          <button onClick={() => navigate('/hosts')} className="text-blue-400 hover:text-blue-300 transition-colors">
            {t('common.viewAll', '查看全部')} →
          </button>
        }>
          {hosts.length === 0 ? (
            <p className="text-sm text-[var(--text-muted)] text-center py-4">{t('hosts.empty', '暂无主机')}</p>
          ) : (
            <div className="overflow-x-auto -mx-5">
              <table className="w-full text-sm">
                <thead>
                  <tr>
                    <th className="text-left px-4 py-2 text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)] border-b border-[var(--bg-border)]">MAC</th>
                    <th className="text-left px-4 py-2 text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)] border-b border-[var(--bg-border)]">{t('hosts.columns.hostname')}</th>
                    <th className="text-left px-4 py-2 text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)] border-b border-[var(--bg-border)]">IP</th>
                    <th className="text-left px-4 py-2 text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)] border-b border-[var(--bg-border)]">{t('hosts.columns.lastOnline')}</th>
                  </tr>
                </thead>
                <tbody>
                  {hosts.slice(0, 4).map(h => (
                    <tr key={h.id} onClick={() => navigate('/hosts/' + h.id)} className="cursor-pointer hover:bg-white/[0.02]">
                      <td className="px-4 py-3 border-b border-[var(--bg-border)]">
                        <span className="font-mono text-xs text-[var(--text-primary)]">{h.mac}</span>
                      </td>
                      <td className="px-4 py-3 border-b border-[var(--bg-border)]">
                        <span className="font-medium text-[var(--text-primary)]">{h.name || '—'}</span>
                      </td>
                      <td className="px-4 py-3 border-b border-[var(--bg-border)]">
                        <span className="font-mono text-xs text-[var(--text-secondary)]">{h.ip}</span>
                      </td>
                      <td className="px-4 py-3 border-b border-[var(--bg-border)]">
                        <span className="font-mono text-xs text-[var(--text-muted)]">
                          {h.last_online ? new Date(h.last_online).toLocaleString() : '—'}
                        </span>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </Card>
      </div>
    </div>
  )
}
