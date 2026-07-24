import { useState, useEffect, useRef, useCallback, useMemo } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { Activity, Server, Zap, ChevronRight, Wifi, BarChart3, Users, FileText, ArrowUpRight, ArrowDownRight } from 'lucide-react'
import { StatusDot } from '../components/ui/StatusDot'
import { Card } from '../components/ui/Card'
import { PageHeader } from '../components/ui/PageHeader'
import { api, type Host, type Event, type MetricsSnapshot, type TimeBucket, getServices, type ServiceInfo } from '../api/client'
import { XAxis, YAxis, Tooltip, ResponsiveContainer, PieChart, Pie, Cell, BarChart, Bar, LineChart, Line, CartesianGrid, type PieLabelRenderProps } from 'recharts'

function avgRate(buckets: TimeBucket[]): string {
  if (buckets.length < 2) return '0'
  const recent = buckets.slice(-6)
  const avg = recent.reduce((a, b) => a + b.v, 0) / recent.length
  return avg < 10 ? avg.toFixed(1) : Math.round(avg).toString()
}

function fmtBytes(n: number): string {
  if (n < 1024) return `${n}B`
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)}KB`
  return `${(n / 1024 / 1024).toFixed(1)}MB`
}

function bwRate(buckets: TimeBucket[]): string {
  if (buckets.length < 3) return '0/s'
  const recent = buckets.slice(-6)
  const total = recent.reduce((a, b) => a + b.v, 0)
  const bytesPerSec = Math.round(total / 60)
  if (bytesPerSec < 1024) return `${bytesPerSec}B/s`
  if (bytesPerSec < 1024 * 1024) return `${(bytesPerSec / 1024).toFixed(1)}KB/s`
  return `${(bytesPerSec / 1024 / 1024).toFixed(1)}MB/s`
}

function fmtUptime(startedAt: string | null): string {
  if (!startedAt) return '—'
  const ms = Date.now() - new Date(startedAt).getTime()
  if (ms < 0) return '—'
  const sec = Math.floor(ms / 1000)
  if (sec < 60) return `${sec}s`
  const min = Math.floor(sec / 60)
  if (min < 60) return `${min}m ${sec % 60}s`
  const hr = Math.floor(min / 60)
  if (hr < 24) return `${hr}h ${min % 60}m`
  const day = Math.floor(hr / 24)
  return `${day}d ${hr % 24}h`
}

// 分类色板：尽量用语义 accent 变量；pink/lime 无对应 accent 变量，保留 hex
const PIE_COLORS = [
  'var(--color-accent-cyan)', 'var(--color-accent-yellow)', 'var(--color-accent-purple)', 'var(--color-accent-green)',
  '#f472b6', 'var(--color-accent-orange)', 'var(--color-accent-blue)', '#84cc16',
]

const HTTP_RANGE_CFG: Record<string, { seconds: number }> = {
  '5m': { seconds: 300 },
  '30m': { seconds: 1800 },
  '1h': { seconds: 3600 },
}

function ChartTooltip({ active, payload, label, formatter }: any) {
  if (!active || !payload?.length) return null
  return (
    <div style={{ background: 'var(--popover)', border: '1px solid var(--border)', borderRadius: 8, fontSize: 12, padding: '8px 12px' }}>
      <p style={{ margin: 0, marginBottom: 4, fontWeight: 600, color: 'var(--foreground)' }}>{label}</p>
      {payload.map((entry: any, i: number) => (
        <p key={i} style={{ margin: 0, color: 'var(--foreground)' }}>
          {entry.name}: {formatter ? formatter(entry.value) : typeof entry.value === 'number' ? entry.value.toLocaleString() : entry.value}
        </p>
      ))}
    </div>
  )
}

export default function Dashboard() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [hosts, setHosts] = useState<Host[]>([])
  const [events, setEvents] = useState<Event[]>([])
  const [metrics, setMetrics] = useState<MetricsSnapshot | null>(null)
  const [dnsCount, setDnsCount] = useState(0)
  const [serviceList, setServiceList] = useState<ServiceInfo[]>([])
  const [loading, setLoading] = useState(true)
  const [trafficRange, setTrafficRange] = useState<'5m' | '30m' | '1h'>('5m')
  const [httpRange, setHttpRange] = useState<'5m' | '30m' | '1h'>('5m')
  const httpDeltasRef = useRef<{ time: number; status2xx: number; status3xx: number; status4xx: number; status5xx: number }[]>([])
  const lastHttpRef = useRef<{ status2xx: number; status3xx: number; status4xx: number; status5xx: number } | null>(null)
  const intervalRef = useRef<ReturnType<typeof setInterval>>(undefined)

  const load = useCallback(async () => {
    try {
      const [h, e, m, d, svcs] = await Promise.all([
        api.getHosts({ page: '1', size: '8' }),
        api.getEvents({ page: '1', size: '12' }),
        api.getMetrics(),
        api.getDNSRecords().catch(() => ({ data: { records: [] } })),
        getServices().catch(() => ({ data: [] as ServiceInfo[] })),
      ])
      setHosts(h.data.hosts ?? [])
      setEvents(e.data.events ?? [])
      setMetrics(m.data)
      setDnsCount(d.data.records.length)
      setServiceList(svcs.data)

      // Compute HTTP status deltas for time-range filtering
      const httpMetrics = m.data.services.http?.http
      if (httpMetrics) {
        const prev = lastHttpRef.current
        if (prev !== null) {
          const entry = {
            time: Date.now(),
            status2xx: Math.max(0, httpMetrics.status2xx - prev.status2xx),
            status3xx: Math.max(0, httpMetrics.status3xx - prev.status3xx),
            status4xx: Math.max(0, httpMetrics.status4xx - prev.status4xx),
            status5xx: Math.max(0, httpMetrics.status5xx - prev.status5xx),
          }
          httpDeltasRef.current.push(entry)
          // Prune entries older than 1h
          const cutoff = Date.now() - 3600000
          httpDeltasRef.current = httpDeltasRef.current.filter(d => d.time > cutoff)
        }
        lastHttpRef.current = {
          status2xx: httpMetrics.status2xx,
          status3xx: httpMetrics.status3xx,
          status4xx: httpMetrics.status4xx,
          status5xx: httpMetrics.status5xx,
        }
      }
    } catch (err) {
      console.error('Failed to load dashboard', err)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { load(); intervalRef.current = window.setInterval(load, 5000); return () => clearInterval(intervalRef.current) }, [load])

  const serviceStatus = (svc: string): 'running' | 'stopped' | 'error' => {
    if (!serviceList.length) return 'stopped'
    // 对于 dhcp，匹配所有 dhcp/ 开头的服务（dhcp/eth0 等），任一 running 即 running
    if (svc === 'dhcp') {
      const dhcpSvcs = serviceList.filter(s => s.name.startsWith('dhcp/'))
      if (dhcpSvcs.some(s => s.status === 'running')) return 'running'
      if (dhcpSvcs.some(s => s.status === 'error')) return 'error'
      return dhcpSvcs.length ? 'stopped' : 'stopped'
    }
    const s = serviceList.find(svcItem => svcItem.name === svc)
    if (!s) return 'stopped'
    return s.status
  }

  const services = [
    { name: 'DHCP', port: ':67', key: 'dhcp' },
    { name: 'TFTP', port: ':69', key: 'tftp' },
    { name: 'HTTP', port: ':8080', key: 'http' },
    { name: 'DNS', port: ':53', key: 'dns' },
    { name: 'NFS', port: ':2049', key: 'nfs' },
  ]

  const dm = (k: string) => metrics?.services[k]?.metrics
  const dhcpData = metrics?.services.dhcp?.dhcp

  const todayBoots = events.filter(e => {
    if ((e.type || '').toUpperCase() !== 'BOOT') return false
    const d = new Date(e.timestamp)
    const now = new Date()
    return d.toDateString() === now.toDateString()
  }).length

  // 在线判定：最近 10 分钟内有 last_online 记录
  const ONLINE_WINDOW_MS = 10 * 60 * 1000
  const onlineHosts = hosts.filter(h => {
    if (!h.last_online) return false
    return Date.now() - new Date(h.last_online).getTime() < ONLINE_WINDOW_MS
  }).length

  const statCards = [
    { label: t('dashboard.stats.onlineHosts'), value: onlineHosts, color: 'green', icon: Server },
    { label: t('dashboard.stats.runningServices'), value: serviceList.filter(s => s.status === 'running').length, color: 'blue', icon: Wifi },
    { label: t('dashboard.activeLeases'), value: dhcpData?.activeLeases ?? 0, color: 'cyan', icon: Users },
    { label: t('dashboard.dnsRecords'), value: dnsCount, color: 'violet', icon: FileText },
    { label: t('dashboard.todayBoots'), value: todayBoots, color: 'orange', icon: Activity },
  ]

  const archPieData = dhcpData?.archBreakdown
    ? Object.entries(dhcpData.archBreakdown).map(([name, value]) => ({ name, value }))
    : []

  const trafficServices = ['tftp', 'http', 'nfs']
  const trafficColors: Record<string, string> = { tftp: 'var(--color-accent-yellow)', http: 'var(--color-accent-purple)', nfs: 'var(--color-accent-green)' }
  const trafficLabels: Record<string, string> = { tftp: 'TFTP', http: 'HTTP', nfs: 'NFS' }

  const trafficRangeCfg: Record<string, { seconds: number; agg: number }> = {
    '5m': { seconds: 300, agg: 10 },
    '30m': { seconds: 1800, agg: 60 },
    '1h': { seconds: 3600, agg: 120 },
  }

  const trafficChartData = useMemo(() => {
    const now = Date.now() / 1000
    const { seconds: maxAge, agg } = trafficRangeCfg[trafficRange] ?? trafficRangeCfg['5m']
    const buckets: Record<number, Record<string, number>> = {}
    trafficServices.forEach(svc => {
      const bw = metrics?.services[svc]?.metrics?.bandwidth ?? []
      bw.forEach(b => {
        if (now - b.t > maxAge) return
        const key = Math.floor(b.t / agg) * agg
        if (!buckets[key]) buckets[key] = {}
        buckets[key][svc] = (buckets[key][svc] || 0) + b.v
      })
    })
    return Object.entries(buckets)
      .sort(([a], [b]) => Number(a) - Number(b))
      .map(([t, svcData]) => ({
        time: new Date(Number(t) * 1000).toLocaleTimeString(),
        tftp: +(svcData.tftp ?? 0),
        http: +(svcData.http ?? 0),
        nfs: +(svcData.nfs ?? 0),
      }))
  }, [metrics, trafficRange])

  const httpRecent = useMemo(() => {
    const now = Date.now()
    const maxAge = (HTTP_RANGE_CFG[httpRange] ?? HTTP_RANGE_CFG['5m']).seconds * 1000
    const cutoff = now - maxAge
    const relevant = httpDeltasRef.current.filter(d => d.time > cutoff)
    if (relevant.length > 0) {
      return {
        status2xx: relevant.reduce((a, d) => a + d.status2xx, 0),
        status3xx: relevant.reduce((a, d) => a + d.status3xx, 0),
        status4xx: relevant.reduce((a, d) => a + d.status4xx, 0),
        status5xx: relevant.reduce((a, d) => a + d.status5xx, 0),
      }
    }
    // Fall back to cumulative until first delta is available
    const init = metrics?.services.http?.http
    if (init) {
      return {
        status2xx: init.status2xx,
        status3xx: init.status3xx,
        status4xx: init.status4xx,
        status5xx: init.status5xx,
      }
    }
    return { status2xx: 0, status3xx: 0, status4xx: 0, status5xx: 0 }
  }, [metrics, httpRange])

  const httpStatusData = httpRecent.status2xx > 0 || httpRecent.status3xx > 0 || httpRecent.status4xx > 0 || httpRecent.status5xx > 0
    ? [
        { name: '2xx', value: httpRecent.status2xx, fill: 'var(--color-accent-green)' },
        { name: '3xx', value: httpRecent.status3xx, fill: 'var(--color-accent-cyan)' },
        { name: '4xx', value: httpRecent.status4xx, fill: 'var(--color-accent-yellow)' },
        { name: '5xx', value: httpRecent.status5xx, fill: 'var(--color-accent-red)' },
      ]
    : []

  const eventIcon = (type: string) => {
    const t = type.toLowerCase()
    if (t.includes('dhcp')) return { label: 'DHCP', color: 'bg-cyan-500/10 text-cyan-400' }
    if (t.includes('tftp')) return { label: 'TFTP', color: 'bg-orange-500/10 text-orange-400' }
    if (t.includes('http')) return { label: 'HTTP', color: 'bg-purple-500/10 text-purple-400' }
    if (t.includes('boot')) return { label: 'BOOT', color: 'bg-accent-green/10 text-accent-green' }
    if (t.includes('ipmi')) return { label: 'IPMI', color: 'bg-accent-yellow/10 text-accent-yellow' }
    if (t.includes('dns')) return { label: 'DNS', color: 'bg-violet-500/10 text-violet-400' }
    return { label: 'EVT', color: 'bg-blue-500/10 text-blue-400' }
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <PageHeader
        title={t('dashboard.title')}
        description={t('dashboard.description')}
        size="lg"
        className="mb-0"
      />

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
                const colorMap: Record<string, string> = { running: 'bg-accent-green', stopped: 'bg-gray-400', error: 'bg-accent-red' }
                return <span key={svc.key} className={`w-2 h-2 rounded-full ${colorMap[st]}`} />
              })}
            </div>
          </div>
          <div className="grid grid-cols-2 sm:grid-cols-5 gap-3">
            {services.map(svc => {
              const st = serviceStatus(svc.key)
              const color = st === 'running' ? 'green' : st === 'error' ? 'red' : 'yellow'
              const svcInfo = serviceList.find(s => s.name === svc.key) || (svc.key === 'dhcp' ? serviceList.find(s => s.name.startsWith('dhcp/')) : undefined)
              const protocol = svcInfo?.protocol?.toUpperCase() || ''
              const uptime = fmtUptime(svcInfo?.started_at ?? null)
              const startTime = svcInfo?.started_at ? new Date(svcInfo.started_at).toLocaleString() : '—'
              return (
                <div key={svc.name} className="flex items-center gap-3 px-3 py-2.5 rounded-xl bg-[var(--bg-base)]/50 border border-[var(--bg-border)]/50">
                  <StatusDot color={color as any} pulse={st === 'running'} />
                  <div className="min-w-0 flex-1">
                    <div className="text-sm font-semibold text-[var(--text-primary)]">{svc.name}</div>
                    <div className="text-[10px] font-mono text-[var(--text-muted)]">{svc.port}{protocol ? `/${protocol}` : ''}</div>
                    {st === 'running' && (
                      <div className="text-[9px] font-mono text-[var(--text-muted)] mt-0.5">{t('dashboard.startedAt')} {startTime}</div>
                    )}
                  </div>
                  <div className="flex flex-col items-end gap-0.5">
                    <span className={`text-[10px] font-semibold tracking-wide ${
                      st === 'running' ? 'text-accent-green' : st === 'error' ? 'text-accent-red' : 'text-[var(--text-muted)]'
                    }`}>
                      {st === 'running' ? t('common.running') : st === 'error' ? t('common.error') : t('common.stopped')}
                    </span>
                    {st === 'running' && (
                      <span className="text-[9px] font-mono text-[var(--text-muted)]">{uptime}</span>
                    )}
                  </div>
                </div>
              )
            })}
          </div>
        </div>
      </div>

      {/* Stats Cards */}
      {loading ? (
        <div className="grid grid-cols-2 lg:grid-cols-5 gap-3">
          {Array.from({ length: 5 }).map((_, i) => (
            <div key={i} className="rounded-2xl border border-[var(--bg-border)] bg-[var(--bg-card)] p-4 animate-shimmer" />
          ))}
        </div>
      ) : (
        <div className="grid grid-cols-2 lg:grid-cols-5 gap-3">
          {statCards.map((s, i) => {
            const Icon = s.icon
            return (
              <div key={i} className="group relative overflow-hidden rounded-2xl border border-[var(--bg-border)] bg-[var(--bg-card)] p-4 shadow-sm hover:shadow-lg hover:-translate-y-0.5 transition-all duration-300">
                <div className="relative">
                  <div className="flex items-center justify-between mb-2">
                    <span className="text-[10px] font-semibold uppercase tracking-wider text-[var(--text-muted)]">{s.label}</span>
                    <span className={`w-8 h-8 rounded-lg flex items-center justify-center transition-all duration-300 group-hover:scale-110 ${
                      s.color === 'green' ? 'bg-accent-green/10 text-accent-green' :
                      s.color === 'blue' ? 'bg-blue-500/10 text-blue-400' :
                      s.color === 'cyan' ? 'bg-cyan-500/10 text-cyan-400' :
                      s.color === 'violet' ? 'bg-violet-500/10 text-violet-400' :
                      'bg-orange-500/10 text-orange-400'
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

      {/* Per-Service Detail Cards */}
      <div className="grid grid-cols-1 lg:grid-cols-5 gap-3">
        {/* DHCP */}
        <div className="rounded-2xl border border-[var(--bg-border)] bg-[var(--bg-card)] p-4 shadow-sm">
          <div className="flex items-center justify-between mb-3">
            <span className="text-xs font-semibold text-[var(--text-primary)]">DHCP</span>
            <StatusDot color={serviceStatus('dhcp') === 'running' ? 'green' : 'red'} pulse />
          </div>
          {dhcpData ? (
            <div className="space-y-1.5 text-xs">
              <div className="flex justify-between"><span className="text-[var(--text-muted)]">{t('dashboard.offerAckNak')}</span><span className="font-mono text-[var(--text-primary)]">{dhcpData.offers}/{dhcpData.acks}/{dhcpData.naks}</span></div>
              <div className="flex justify-between"><span className="text-[var(--text-muted)]">{t('dashboard.discoverRequest')}</span><span className="font-mono text-[var(--text-primary)]">{dhcpData.discovers}/{dhcpData.requests}</span></div>
              <div className="flex justify-between"><span className="text-[var(--text-muted)]">{t('dashboard.decline')}</span><span className="font-mono text-accent-red">{dhcpData.declines}</span></div>
              <div className="flex justify-between"><span className="text-[var(--text-muted)]">{t('dashboard.unauthorized')}</span><span className="font-mono text-accent-yellow">{dhcpData.unauthorized}</span></div>
            </div>
          ) : (
            <p className="text-xs text-[var(--text-muted)]">{serviceStatus('dhcp') === 'stopped' ? t('dashboard.serviceNotRunning') : t('dashboard.noMetrics')}</p>
          )}
        </div>

        {/* TFTP */}
        <div className="rounded-2xl border border-[var(--bg-border)] bg-[var(--bg-card)] p-4 shadow-sm">
          <div className="flex items-center justify-between mb-3">
            <span className="text-xs font-semibold text-[var(--text-primary)]">TFTP</span>
            <StatusDot color={serviceStatus('tftp') === 'running' ? 'green' : 'red'} pulse />
          </div>
          {dm('tftp') ? (
            <div className="space-y-1.5 text-xs">
              <div className="flex justify-between"><span className="text-[var(--text-muted)]">{t('dashboard.labelRequests')}</span><span className="font-mono text-[var(--text-primary)]">{dm('tftp')?.requests ?? 0}</span></div>
              <div className="flex justify-between"><span className="text-[var(--text-muted)]">{t('dashboard.labelErrors')}</span><span className="font-mono text-accent-red">{dm('tftp')?.errors ?? 0}</span></div>
              <div className="flex justify-between"><span className="text-[var(--text-muted)]">{t('dashboard.labelTrafficOut')}</span><span className="font-mono text-[var(--text-primary)]">{fmtBytes(dm('tftp')?.bytesOut ?? 0)}</span></div>
              <div className="flex justify-between"><span className="text-[var(--text-muted)]">{t('dashboard.labelRate')}</span><span className="font-mono text-[var(--text-primary)]">{avgRate(dm('tftp')?.requestRate ?? [])}/s</span></div>
            </div>
          ) : (
            <p className="text-xs text-[var(--text-muted)]">{serviceStatus('tftp') === 'stopped' ? t('dashboard.serviceNotRunning') : t('dashboard.noMetrics')}</p>
          )}
        </div>

        {/* DNS */}
        <div className="rounded-2xl border border-[var(--bg-border)] bg-[var(--bg-card)] p-4 shadow-sm">
          <div className="flex items-center justify-between mb-3">
            <span className="text-xs font-semibold text-[var(--text-primary)]">DNS</span>
            <StatusDot color={serviceStatus('dns') === 'running' ? 'green' : 'red'} pulse />
          </div>
          {dm('dns') ? (
            <div className="space-y-1.5 text-xs">
              <div className="flex justify-between"><span className="text-[var(--text-muted)]">{t('dashboard.labelDnsRecords')}</span><span className="font-mono text-[var(--text-primary)]">{dnsCount}</span></div>
              <div className="flex justify-between"><span className="text-[var(--text-muted)]">{t('dashboard.labelQueryCount')}</span><span className="font-mono text-[var(--text-primary)]">{dm('dns')?.requests ?? 0}</span></div>
              <div className="flex justify-between"><span className="text-[var(--text-muted)]">{t('dashboard.labelErrors')}</span><span className="font-mono text-accent-red">{dm('dns')?.errors ?? 0}</span></div>
              <div className="flex justify-between"><span className="text-[var(--text-muted)]">{t('dashboard.labelQueryRate')}</span><span className="font-mono text-[var(--text-primary)]">{avgRate(dm('dns')?.requestRate ?? [])}/s</span></div>
            </div>
          ) : (
            <p className="text-xs text-[var(--text-muted)]">{serviceStatus('dns') === 'stopped' ? t('dashboard.serviceNotRunning') : t('dashboard.noMetrics')}</p>
          )}
        </div>

        {/* HTTP */}
        <div className="rounded-2xl border border-[var(--bg-border)] bg-[var(--bg-card)] p-4 shadow-sm">
          <div className="flex items-center justify-between mb-3">
            <span className="text-xs font-semibold text-[var(--text-primary)]">HTTP</span>
            <StatusDot color={serviceStatus('http') === 'running' ? 'green' : 'red'} pulse />
          </div>
          {dm('http') ? (
            <div className="space-y-1.5 text-xs">
              <div className="flex justify-between"><span className="text-[var(--text-muted)]">{t('dashboard.labelRequests')}</span><span className="font-mono text-[var(--text-primary)]">{dm('http')?.requests ?? 0}</span></div>
              <div className="flex justify-between"><span className="text-[var(--text-muted)]">{t('dashboard.labelActiveConns')}</span><span className="font-mono text-[var(--text-primary)]">{dm('http')?.activeConns ?? 0}</span></div>
              <div className="flex justify-between"><span className="text-[var(--text-muted)]">{t('dashboard.labelStatusCodes')}</span><span className="font-mono text-[var(--text-primary)]">{httpRecent.status2xx}/{httpRecent.status4xx}/{httpRecent.status5xx}</span></div>
              <div className="flex justify-between"><span className="text-[var(--text-muted)]">{t('dashboard.labelRejected')}</span><span className="font-mono text-accent-yellow">{dm('http')?.rejected ?? 0}</span></div>
            </div>
          ) : (
            <p className="text-xs text-[var(--text-muted)]">{serviceStatus('http') === 'stopped' ? t('dashboard.serviceNotRunning') : t('dashboard.noMetrics')}</p>
          )}
        </div>

        {/* NFS */}
        <div className="rounded-2xl border border-[var(--bg-border)] bg-[var(--bg-card)] p-4 shadow-sm">
          <div className="flex items-center justify-between mb-3">
            <span className="text-xs font-semibold text-[var(--text-primary)]">NFS</span>
            <StatusDot color={serviceStatus('nfs') === 'running' ? 'green' : 'red'} pulse />
          </div>
          {dm('nfs') ? (
            <div className="space-y-1.5 text-xs">
              <div className="flex justify-between"><span className="text-[var(--text-muted)]">{t('dashboard.labelRequests')}</span><span className="font-mono text-[var(--text-primary)]">{dm('nfs')?.requests ?? 0}</span></div>
              <div className="flex justify-between"><span className="text-[var(--text-muted)]">{t('dashboard.labelActiveConns')}</span><span className="font-mono text-[var(--text-primary)]">{dm('nfs')?.activeConns ?? 0}</span></div>
              <div className="flex justify-between"><span className="text-[var(--text-muted)]">{t('dashboard.labelTransferTraffic')}</span><span className="font-mono text-[var(--text-primary)]">{fmtBytes((dm('nfs')?.bytesOut ?? 0) + (dm('nfs')?.bytesIn ?? 0))}</span></div>
              <div className="flex justify-between"><span className="text-[var(--text-muted)]">{t('dashboard.labelRate')}</span><span className="font-mono text-[var(--text-primary)]">{avgRate(dm('nfs')?.requestRate ?? [])}/s</span></div>
            </div>
          ) : (
            <p className="text-xs text-[var(--text-muted)]">{serviceStatus('nfs') === 'stopped' ? t('dashboard.serviceNotRunning') : t('dashboard.noMetrics')}</p>
          )}
        </div>
      </div>

      {/* Charts Row */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-5">
        <div className="rounded-2xl border border-[var(--bg-border)] bg-[var(--bg-card)] p-5 shadow-sm">
          <div className="flex items-center gap-2 mb-4">
            <Server size={14} className="text-cyan-400" />
            <span className="text-xs font-semibold uppercase tracking-wider text-[var(--text-muted)]">{t('dashboard.chartDhcpArch')}</span>
          </div>
          {archPieData.length > 0 ? (
            <div className="h-52">
              <ResponsiveContainer width="100%" height="100%">
                <PieChart>
                  <Pie data={archPieData} dataKey="value" nameKey="name" cx="50%" cy="50%" outerRadius={70} label={({ name, percent }: PieLabelRenderProps) => `${name ?? ''} ${((percent ?? 0) * 100).toFixed(0)}%`}>
                    {archPieData.map((_, i) => <Cell key={i} fill={PIE_COLORS[i % PIE_COLORS.length]} />)}
                  </Pie>
                  <Tooltip content={<ChartTooltip />} />
                </PieChart>
              </ResponsiveContainer>
            </div>
          ) : (
            <p className="text-sm text-[var(--text-muted)] text-center py-12">{t('common.noData')}</p>
          )}
        </div>

        <div className="rounded-2xl border border-[var(--bg-border)] bg-[var(--bg-card)] p-5 shadow-sm">
          <div className="flex items-center justify-between mb-4">
            <div className="flex items-center gap-2">
              <BarChart3 size={14} className="text-purple-400" />
              <span className="text-xs font-semibold uppercase tracking-wider text-[var(--text-muted)]">{t('dashboard.chartHttpStatus')}</span>
            </div>
            <div className="flex items-center gap-1 bg-[var(--bg-base)]/50 rounded-lg p-0.5">
              {[['5m', t('dashboard.range5m')], ['30m', t('dashboard.range30m')], ['1h', t('dashboard.range1h')]].map(([key, label]) => (
                <button key={key} onClick={() => setHttpRange(key as any)} className={`px-2 py-0.5 text-[10px] font-medium rounded-md transition-colors ${httpRange === key ? 'bg-blue-500/20 text-blue-400' : 'text-[var(--text-muted)] hover:text-[var(--text-primary)]'}`}>
                  {label}
                </button>
              ))}
            </div>
          </div>
          {httpStatusData.some(d => d.value > 0) ? (
            <div className="h-52">
              <ResponsiveContainer width="100%" height="100%">
                <BarChart data={httpStatusData} margin={{ top: 5, right: 10, left: 0, bottom: 5 }}>
                  <XAxis dataKey="name" tick={{ fontSize: 11, fill: 'var(--foreground-muted)' }} axisLine={false} tickLine={false} />
                  <YAxis tick={{ fontSize: 10, fill: 'var(--foreground-muted)' }} axisLine={false} tickLine={false} allowDecimals={false} />
                  <Tooltip content={<ChartTooltip />} />
                  <Bar dataKey="value" name={t('dashboard.httpCount')} radius={[4, 4, 0, 0]}>
                    {httpStatusData.map((d, i) => <Cell key={i} fill={d.fill} />)}
                  </Bar>
                </BarChart>
              </ResponsiveContainer>
            </div>
          ) : (
            <p className="text-sm text-[var(--text-muted)] text-center py-12">{t('common.noData')}</p>
          )}
          <div className="flex flex-wrap gap-3 mt-2 text-xs text-[var(--text-muted)]">
            {httpStatusData.map(d => (
              <span key={d.name} className="flex items-center gap-1">
                <span className="w-2.5 h-2.5 rounded" style={{ background: d.fill }} /> {d.name}: {d.value}
              </span>
            ))}
          </div>
        </div>

        <div className="rounded-2xl border border-[var(--bg-border)] bg-[var(--bg-card)] p-5 shadow-sm">
          <div className="flex items-center justify-between mb-4">
            <div className="flex items-center gap-2">
              <BarChart3 size={14} className="text-accent-green" />
              <span className="text-xs font-semibold uppercase tracking-wider text-[var(--text-muted)]">{t('dashboard.chartTraffic')}</span>
            </div>
            <div className="flex items-center gap-1 bg-[var(--bg-base)]/50 rounded-lg p-0.5">
              {[['5m', t('dashboard.range5m')], ['30m', t('dashboard.range30m')], ['1h', t('dashboard.range1h')]].map(([key, label]) => (
                <button key={key} onClick={() => setTrafficRange(key as any)} className={`px-2 py-0.5 text-[10px] font-medium rounded-md transition-colors ${trafficRange === key ? 'bg-blue-500/20 text-blue-400' : 'text-[var(--text-muted)] hover:text-[var(--text-primary)]'}`}>
                  {label}
                </button>
              ))}
            </div>
          </div>
          {trafficChartData.length > 0 ? (
            <>
              <div className="h-40">
                <ResponsiveContainer width="100%" height="100%">
                  <LineChart data={trafficChartData} margin={{ top: 5, right: 5, left: -20, bottom: 0 }}>
                    <CartesianGrid strokeDasharray="3 3" stroke="var(--border)" strokeOpacity={0.5} />
                    <XAxis dataKey="time" tick={{ fontSize: 9, fill: 'var(--foreground-muted)' }} axisLine={false} tickLine={false} interval="preserveStartEnd" />
                    <YAxis tick={{ fontSize: 9, fill: 'var(--foreground-muted)' }} axisLine={false} tickLine={false} tickFormatter={v => v >= 1024 * 1024 ? `${(v / 1024 / 1024).toFixed(0)}MB` : v >= 1024 ? `${(v / 1024).toFixed(0)}KB` : `${v}B`} />
                    <Tooltip content={<ChartTooltip formatter={(v: any) => fmtBytes(Math.round(Number(v)))} />} />
                    {trafficServices.map(svc => (
                      <Line key={svc} type="monotone" dataKey={svc} name={trafficLabels[svc]} stroke={trafficColors[svc]} strokeWidth={1.5} dot={false} />
                    ))}
                  </LineChart>
                </ResponsiveContainer>
              </div>
              <div className="grid grid-cols-3 gap-2 mt-3">
                {trafficServices.map(svc => {
                  const m = metrics?.services[svc]?.metrics
                  return (
                    <div key={svc} className="bg-[var(--bg-base)]/50 rounded-xl p-2.5">
                      <div className="flex items-center gap-1.5 mb-1.5">
                        <span className="w-2 h-2 rounded-full" style={{ background: trafficColors[svc] }} />
                        <span className="text-[10px] font-semibold text-[var(--text-muted)]">{trafficLabels[svc]}</span>
                      </div>
                      <div className="text-xs font-mono text-[var(--text-primary)] font-semibold">{bwRate(m?.bandwidth ?? [])}</div>
                      <div className="flex items-center gap-2 mt-1 text-[10px] font-mono text-[var(--text-muted)]">
                        <span className="flex items-center gap-0.5"><ArrowUpRight size={10} className="text-accent-green" />{fmtBytes(m?.bytesOut ?? 0)}</span>
                        <span className="flex items-center gap-0.5"><ArrowDownRight size={10} className="text-blue-400" />{fmtBytes(m?.bytesIn ?? 0)}</span>
                      </div>
                    </div>
                  )
                })}
              </div>
            </>
          ) : (
            <p className="text-sm text-[var(--text-muted)] text-center py-12">{t('common.noData')}</p>
          )}
        </div>
      </div>

      {/* Events */}
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
                  <div className={`w-7 h-7 rounded-lg flex items-center justify-center text-[9px] font-bold shrink-0 ${ic.color}`}>{ic.label}</div>
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
  )
}
