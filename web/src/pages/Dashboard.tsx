import { useState, useEffect, useRef, useCallback } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { Activity, Server, Zap, ChevronRight, Wifi, AlertTriangle, BarChart3, Users, XCircle } from 'lucide-react'
import { StatusDot } from '../components/ui/StatusDot'
import { Card } from '../components/ui/Card'
import { api, type ServiceStatus, type Host, type Event, type MetricsSnapshot, type MetricServiceData, type TimeBucket } from '../api/client'
import { LineChart, Line, XAxis, YAxis, Tooltip, ResponsiveContainer, PieChart, Pie, Cell, BarChart, Bar, type PieLabelRenderProps } from 'recharts'

interface ServiceMetricWithKey extends MetricServiceData {
  key: string
}

function lastRate(buckets: TimeBucket[]): number {
  if (buckets.length < 2) return 0
  return Math.round(buckets[buckets.length - 1].v * 10) / 10
}

const PIE_COLORS = ['#22d3ee', '#f59e0b', '#a78bfa', '#34d399', '#f472b6', '#f97316', '#06b6d4', '#84cc16']

const CHART_STROKES: Record<string, string> = {
  dhcp: '#22d3ee',
  tftp: '#f59e0b',
  dns: '#a78bfa',
  http: '#34d399',
  nfs: '#f472b6',
}

export default function Dashboard() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [status, setStatus] = useState<ServiceStatus | null>(null)
  const [hosts, setHosts] = useState<Host[]>([])
  const [events, setEvents] = useState<Event[]>([])
  const [metrics, setMetrics] = useState<MetricsSnapshot | null>(null)
  const [loading, setLoading] = useState(true)
  const intervalRef = useRef<ReturnType<typeof setInterval>>(undefined)

  const load = useCallback(async () => {
    try {
      const [s, h, e, m] = await Promise.all([
        api.getStatus(),
        api.getHosts({ page: '1', size: '8' }),
        api.getEvents({ page: '1', size: '12' }),
        api.getMetrics(),
      ])
      setStatus(s.data)
      setHosts(h.data.hosts ?? [])
      setEvents(e.data.events ?? [])
      setMetrics(m.data)
    } catch (err) {
      console.error('Failed to load dashboard', err)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { load(); intervalRef.current = window.setInterval(load, 5000); return () => clearInterval(intervalRef.current) }, [load])

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
    { name: 'NFS', port: ':2049', key: 'nfs' },
  ]

  const defaultMetrics = { metrics: { requests: 0, errors: 0, bytesIn: 0, bytesOut: 0, activeConns: 0, rejected: 0, requestRate: [], errorRate: [], bandwidth: [], latencyMs: [] } }
  const metricServices: ServiceMetricWithKey[] = services
    .map(s => ({ key: s.key, ...(metrics?.services[s.key] ?? defaultMetrics) }))
    .filter(s => s.metrics.requests > 0 || s.key === 'dhcp')

  const totalRequests = metricServices.reduce((a, s) => a + s.metrics.requests, 0)
  const totalErrors = metricServices.reduce((a, s) => a + s.metrics.errors, 0)
  const totalRejected = metricServices.reduce((a, s) => a + s.metrics.rejected, 0)
  const activeConnections = metricServices.reduce((a, s) => a + s.metrics.activeConns, 0)

  const dhcpData = metrics?.services.dhcp?.dhcp
  const httpData = metrics?.services.http?.http

  const eventIcon = (type: string) => {
    const t = type.toLowerCase()
    if (t.includes('dhcp')) return { label: 'DHCP', color: 'cyan' as const }
    if (t.includes('tftp')) return { label: 'TFTP', color: 'orange' as const }
    if (t.includes('http')) return { label: 'HTTP', color: 'purple' as const }
    if (t.includes('boot')) return { label: 'BOOT', color: 'green' as const }
    if (t.includes('ipmi')) return { label: 'IPMI', color: 'yellow' as const }
    if (t.includes('dns')) return { label: 'DNS', color: 'violet' as const }
    return { label: 'EVENT', color: 'blue' as const }
  }

  const eventColor = (ic: { label: string; color: string }) =>
    ic.color === 'cyan' ? 'bg-cyan-500/10 text-cyan-400' :
    ic.color === 'orange' ? 'bg-orange-500/10 text-orange-400' :
    ic.color === 'purple' ? 'bg-purple-500/10 text-purple-400' :
    ic.color === 'green' ? 'bg-green-500/10 text-green-400' :
    ic.color === 'yellow' ? 'bg-yellow-500/10 text-yellow-400' :
    ic.color === 'violet' ? 'bg-violet-500/10 text-violet-400' :
    'bg-blue-500/10 text-blue-400'

  const statusColorMap: Record<string, string> = {
    running: 'bg-green-500',
    stopped: 'bg-gray-400',
    error: 'bg-red-500',
  }

  const formatTS = (ts: number) => {
    const d = new Date(ts * 1000)
    return `${d.getHours().toString().padStart(2, '0')}:${d.getMinutes().toString().padStart(2, '0')}:${d.getSeconds().toString().padStart(2, '0')}`
  }

  const lineChartData = () => {
    const allBuckets: Record<string, Record<number, number>> = {}
    let allTimestamps = new Set<number>()
    for (const svc of metricServices) {
      for (const b of svc.metrics.requestRate) {
        if (!allBuckets[svc.key]) allBuckets[svc.key] = {}
        allBuckets[svc.key][b.t] = (allBuckets[svc.key][b.t] || 0) + b.v
        allTimestamps.add(b.t)
      }
    }
    return Array.from(allTimestamps).sort().map(ts => {
      const point: Record<string, number | string> = { time: formatTS(ts) }
      for (const svc of metricServices) {
        point[svc.key] = allBuckets[svc.key]?.[ts] ?? 0
      }
      return point
    })
  }

  const archPieData = dhcpData?.archBreakdown
    ? Object.entries(dhcpData.archBreakdown).map(([name, value]) => ({ name, value }))
    : []

  const platformPieData = dhcpData?.platformBreakdown
    ? Object.entries(dhcpData.platformBreakdown).map(([name, value]) => ({ name, value }))
    : []

  const httpStatusData = httpData
    ? [
        { name: '2xx', value: httpData.status2xx, fill: '#34d399' },
        { name: '3xx', value: httpData.status3xx, fill: '#22d3ee' },
        { name: '4xx', value: httpData.status4xx, fill: '#f59e0b' },
        { name: '5xx', value: httpData.status5xx, fill: '#ef4444' },
      ]
    : []

  const statCards = [
    { label: t('dashboard.stats.onlineHosts'), value: hosts.length, color: 'green' as const, icon: Server, gradient: 'from-green-500/20 to-emerald-500/5' },
    { label: t('dashboard.stats.runningServices'), value: services.filter(s => serviceStatus(s.key) === 'running').length, color: 'blue' as const, icon: Wifi, gradient: 'from-blue-500/20 to-cyan-500/5' },
    { label: '总请求', value: totalRequests.toLocaleString(), color: 'orange' as const, icon: BarChart3, gradient: 'from-orange-500/20 to-amber-500/5' },
    { label: '总错误', value: totalErrors.toLocaleString(), color: 'red' as const, icon: XCircle, gradient: 'from-red-500/20 to-rose-500/5' },
    { label: '总拒绝', value: totalRejected.toLocaleString(), color: 'yellow' as const, icon: AlertTriangle, gradient: 'from-yellow-500/20 to-amber-500/5' },
    { label: '活跃连接', value: activeConnections.toString(), color: 'purple' as const, icon: Users, gradient: 'from-purple-500/20 to-pink-500/5' },
  ]

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-2xl font-bold text-[var(--text-primary)] tracking-tight">{t('dashboard.title')}</h1>
        <p className="text-sm text-[var(--text-muted)] mt-1">{t('dashboard.description')}</p>
      </div>

      {/* Service Status Bar */}
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
                return <span key={svc.key} className={`w-2 h-2 rounded-full ${statusColorMap[st]}`} />
              })}
            </div>
          </div>
          <div className="grid grid-cols-2 sm:grid-cols-5 gap-3">
            {services.map(svc => {
              const st = serviceStatus(svc.key)
              const color = serviceColor(svc.key)
              const m = metrics?.services[svc.key]?.metrics
              return (
                <div key={svc.name} className="flex items-center gap-3 px-3 py-2.5 rounded-xl bg-[var(--bg-base)]/50 border border-[var(--bg-border)]/50">
                  <StatusDot color={color as any} pulse={st === 'running'} />
                  <div className="min-w-0 flex-1">
                    <div className="text-sm font-semibold text-[var(--text-primary)]">{svc.name}</div>
                    <div className="text-[10px] font-mono text-[var(--text-muted)]">{svc.port}</div>
                    {m && <div className="text-[9px] font-mono text-[var(--text-muted)] mt-0.5">{m.requests} req</div>}
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

      {/* Stats Cards */}
      {loading ? (
        <div className="grid grid-cols-2 lg:grid-cols-6 gap-3">
          {Array.from({ length: 6 }).map((_, i) => (
            <div key={i} className="rounded-2xl border border-[var(--bg-border)] bg-[var(--bg-card)] p-4 animate-shimmer" />
          ))}
        </div>
      ) : (
        <div className="grid grid-cols-2 lg:grid-cols-6 gap-3">
          {statCards.map((s, i) => {
            const Icon = s.icon
            return (
              <div key={i} className={`group relative overflow-hidden rounded-2xl border border-[var(--bg-border)] bg-[var(--bg-card)] p-4 shadow-sm hover:shadow-lg hover:-translate-y-0.5 transition-all duration-300`}>
                <div className={`absolute inset-0 bg-gradient-to-br ${s.gradient} opacity-0 group-hover:opacity-100 transition-opacity duration-500`} />
                <div className="relative">
                  <div className="flex items-center justify-between mb-3">
                    <span className="text-[10px] font-semibold uppercase tracking-wider text-[var(--text-muted)]">{s.label}</span>
                    <span className={`w-8 h-8 rounded-lg flex items-center justify-center transition-all duration-300 group-hover:scale-110 ${
                      s.color === 'green' ? 'bg-green-500/10 text-green-400' :
                      s.color === 'blue' ? 'bg-blue-500/10 text-blue-400' :
                      s.color === 'orange' ? 'bg-orange-500/10 text-orange-400' :
                      s.color === 'red' ? 'bg-red-500/10 text-red-400' :
                      s.color === 'yellow' ? 'bg-yellow-500/10 text-yellow-400' :
                      'bg-purple-500/10 text-purple-400'
                    }`}>
                      <Icon size={16} />
                    </span>
                  </div>
                  <div className="text-2xl font-bold tracking-tight text-[var(--text-primary)]">{s.value}</div>
                </div>
              </div>
            )
          })}
        </div>
      )}

      {/* Request Rate Line Chart */}
      <div className="rounded-2xl border border-[var(--bg-border)] bg-[var(--bg-card)] p-5 shadow-sm">
        <div className="flex items-center gap-2 mb-4">
          <Activity size={14} className="text-blue-400" />
          <span className="text-xs font-semibold uppercase tracking-wider text-[var(--text-muted)]">请求速率 (ops)</span>
        </div>
        <div className="h-64">
          <ResponsiveContainer width="100%" height="100%">
            <LineChart data={lineChartData()} margin={{ top: 5, right: 10, left: 0, bottom: 5 }}>
              <XAxis dataKey="time" tick={{ fontSize: 10, fill: '#94a3b8' }} axisLine={false} tickLine={false} interval="preserveStartEnd" />
              <YAxis tick={{ fontSize: 10, fill: '#94a3b8' }} axisLine={false} tickLine={false} allowDecimals={false} />
              <Tooltip
                contentStyle={{ background: '#1e293b', border: '1px solid #334155', borderRadius: 8, fontSize: 12 }}
                labelStyle={{ color: '#e2e8f0' }}
              />
              {metricServices.map(svc => (
                <Line key={svc.key} type="monotone" dataKey={svc.key} stroke={CHART_STROKES[svc.key] || '#22d3ee'} strokeWidth={2} dot={false} isAnimationActive={false} />
              ))}
            </LineChart>
          </ResponsiveContainer>
        </div>
        <div className="flex flex-wrap gap-4 mt-3 text-xs text-[var(--text-muted)]">
          {metricServices.map(svc => (
            <span key={svc.key} className="flex items-center gap-1.5">
              <span className="w-2.5 h-2.5 rounded-full" style={{ background: CHART_STROKES[svc.key] || '#22d3ee' }} />
              {svc.key.toUpperCase()} {lastRate(svc.metrics.requestRate)}/s
            </span>
          ))}
        </div>
      </div>

      {/* Charts Row: DHCP Arch + HTTP Status + Per-Service */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-5">
        {/* DHCP Architecture Pie */}
        <div className="rounded-2xl border border-[var(--bg-border)] bg-[var(--bg-card)] p-5 shadow-sm">
          <div className="flex items-center gap-2 mb-4">
            <Server size={14} className="text-cyan-400" />
            <span className="text-xs font-semibold uppercase tracking-wider text-[var(--text-muted)]">DHCP 架构分布</span>
          </div>
          {archPieData.length > 0 ? (
            <div className="h-52">
              <ResponsiveContainer width="100%" height="100%">
                <PieChart>
                  <Pie data={archPieData} dataKey="value" nameKey="name" cx="50%" cy="50%" outerRadius={70} label={({ name, percent }: PieLabelRenderProps) => `${name ?? ''} ${((percent ?? 0) * 100).toFixed(0)}%`}>
                    {archPieData.map((_, i) => <Cell key={i} fill={PIE_COLORS[i % PIE_COLORS.length]} />)}
                  </Pie>
                  <Tooltip />
                </PieChart>
              </ResponsiveContainer>
            </div>
          ) : (
            <p className="text-sm text-[var(--text-muted)] text-center py-12">暂无数据</p>
          )}
        </div>

        {/* HTTP Status Breakdown */}
        <div className="rounded-2xl border border-[var(--bg-border)] bg-[var(--bg-card)] p-5 shadow-sm">
          <div className="flex items-center gap-2 mb-4">
            <BarChart3 size={14} className="text-purple-400" />
            <span className="text-xs font-semibold uppercase tracking-wider text-[var(--text-muted)]">HTTP 状态码</span>
          </div>
          {httpStatusData.some(d => d.value > 0) ? (
            <div className="h-52">
              <ResponsiveContainer width="100%" height="100%">
                <BarChart data={httpStatusData} margin={{ top: 5, right: 10, left: 0, bottom: 5 }}>
                  <XAxis dataKey="name" tick={{ fontSize: 11, fill: '#94a3b8' }} axisLine={false} tickLine={false} />
                  <YAxis tick={{ fontSize: 10, fill: '#94a3b8' }} axisLine={false} tickLine={false} allowDecimals={false} />
                  <Tooltip contentStyle={{ background: '#1e293b', border: '1px solid #334155', borderRadius: 8, fontSize: 12 }} />
                  <Bar dataKey="value" name="count" radius={[4, 4, 0, 0]}>
                    {httpStatusData.map((d, i) => <Cell key={i} fill={d.fill} />)}
                  </Bar>
                </BarChart>
              </ResponsiveContainer>
            </div>
          ) : (
            <p className="text-sm text-[var(--text-muted)] text-center py-12">暂无数据</p>
          )}
          <div className="flex flex-wrap gap-3 mt-2 text-xs text-[var(--text-muted)]">
            {httpStatusData.map(d => (
              <span key={d.name} className="flex items-center gap-1">
                <span className="w-2.5 h-2.5 rounded" style={{ background: d.fill }} /> {d.name}: {d.value}
              </span>
            ))}
          </div>
        </div>

        {/* DHCP Active Leases & Unauthorized */}
        <div className="rounded-2xl border border-[var(--bg-border)] bg-[var(--bg-card)] p-5 shadow-sm">
          <div className="flex items-center gap-2 mb-4">
            <Users size={14} className="text-green-400" />
            <span className="text-xs font-semibold uppercase tracking-wider text-[var(--text-muted)]">DHCP 摘要</span>
          </div>
          {dhcpData ? (
            <div className="space-y-3">
              <div className="grid grid-cols-2 gap-2">
                <div className="bg-[var(--bg-base)]/50 rounded-xl p-3">
                  <div className="text-[10px] font-mono text-[var(--text-muted)]">活跃租约</div>
                  <div className="text-xl font-bold text-[var(--text-primary)]">{dhcpData.activeLeases}</div>
                </div>
                <div className="bg-[var(--bg-base)]/50 rounded-xl p-3">
                  <div className="text-[10px] font-mono text-[var(--text-muted)]">发现/请求</div>
                  <div className="text-xl font-bold text-[var(--text-primary)]">{dhcpData.discovers}/{dhcpData.requests}</div>
                </div>
                <div className="bg-[var(--bg-base)]/50 rounded-xl p-3">
                  <div className="text-[10px] font-mono text-[var(--text-muted)]">授权</div>
                  <div className="text-xl font-bold text-green-400">{dhcpData.acks}</div>
                </div>
                <div className="bg-[var(--bg-base)]/50 rounded-xl p-3">
                  <div className="text-[10px] font-mono text-[var(--text-muted)]">未授权</div>
                  <div className="text-xl font-bold text-red-400">{dhcpData.unauthorized}</div>
                </div>
              </div>
              {platformPieData.length > 0 && (
                <div className="h-28">
                  <ResponsiveContainer width="100%" height="100%">
                    <PieChart>
                      <Pie data={platformPieData} dataKey="value" nameKey="name" cx="50%" cy="50%" outerRadius={40} innerRadius={25}>
                        {platformPieData.map((_, i) => <Cell key={i} fill={PIE_COLORS[i % PIE_COLORS.length]} />)}
                      </Pie>
                    </PieChart>
                  </ResponsiveContainer>
                </div>
              )}
            </div>
          ) : (
            <p className="text-sm text-[var(--text-muted)] text-center py-12">暂无数据</p>
          )}
        </div>
      </div>

      {/* Events + Hosts */}
      <div className="grid grid-cols-1 lg:grid-cols-5 gap-5">
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
                {events.slice(0, 8).map((e, i) => {
                  const ic = eventIcon(e.type)
                  return (
                    <div key={i} className="flex items-center gap-3 px-5 py-2.5 border-b border-[var(--bg-border)] last:border-b-0 hover:bg-[var(--bg-hover)]/30 transition-colors group">
                      <div className={`w-7 h-7 rounded-lg flex items-center justify-center text-[9px] font-bold shrink-0 ${eventColor(ic)}`}>{ic.label}</div>
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
