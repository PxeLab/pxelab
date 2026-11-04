import { useState, useEffect, useRef, useCallback, useMemo } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { Activity, Server, Zap, ChevronRight, Wifi, BarChart3, Users, FileText, ArrowUpRight, ArrowDownRight, Rocket } from 'lucide-react'
import { StatusDot } from '../components/ui/StatusDot'
import { Button } from '../components/ui/Button'
import { Card } from '../components/ui/Card'
import { PageHeader } from '../components/ui/PageHeader'
import { api, type Event, type MetricsSnapshot, type TimeBucket, getServices, type ServiceInfo } from '../api/client'
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
  const [events, setEvents] = useState<Event[]>([])
  const [metrics, setMetrics] = useState<MetricsSnapshot | null>(null)
  const [dnsCount, setDnsCount] = useState(0)
  const [serviceList, setServiceList] = useState<ServiceInfo[]>([])
  const [onlineHosts, setOnlineHosts] = useState(0)
  const [todayBoots, setTodayBoots] = useState(0)
  const [loading, setLoading] = useState(true)
  const [trafficRange, setTrafficRange] = useState<'5m' | '30m' | '1h'>('5m')
  const [httpRange, setHttpRange] = useState<'5m' | '30m' | '1h'>('5m')
  const httpDeltasRef = useRef<{ time: number; requests: number; rejected: number; status2xx: number; status3xx: number; status4xx: number; status5xx: number }[]>([])
  const lastHttpRef = useRef<{ requests: number; rejected: number; status2xx: number; status3xx: number; status4xx: number; status5xx: number } | null>(null)
  const intervalRef = useRef<ReturnType<typeof setInterval>>(undefined)

  const load = useCallback(async () => {
    try {
      // 今日 0 点（本地时区）的 Unix 秒，用于真实统计今日启动次数
      const dayStart = Math.floor(new Date().setHours(0, 0, 0, 0) / 1000)
      const [h, e, m, d, svcs, bootCount] = await Promise.all([
        api.getHosts({ page: '1', size: '8', online_within: 600 }),
        api.getEvents({ page: '1', size: '12' }),
        api.getMetrics(),
        api.getDNSRecords().catch(() => ({ data: { records: [] } })),
        getServices().catch(() => ({ data: [] as ServiceInfo[] })),
        api.getEvents({ type: 'BOOT', from: String(dayStart), page: '1', size: '1' }).catch(() => null),
      ])
      setEvents(e.data.events ?? [])
      setMetrics(m.data)
      setDnsCount(d.data.records.length)
      setServiceList(svcs.data)
      setOnlineHosts(h.data.online ?? 0)
      setTodayBoots(bootCount?.data.meta.total ?? 0)

      // 记录 HTTP 请求/拒绝/状态码的增量，供时间窗口统计使用
      const httpSvc = m.data.services.http
      const httpMetrics = httpSvc?.http
      if (httpMetrics) {
        const requests = httpSvc?.metrics.requests ?? 0
        const rejected = httpSvc?.metrics.rejected ?? 0
        const prev = lastHttpRef.current
        if (prev !== null) {
          const entry = {
            time: Date.now(),
            requests: Math.max(0, requests - prev.requests),
            rejected: Math.max(0, rejected - prev.rejected),
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
          requests,
          rejected,
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

  // 服务名支持 "dhcp/eth0"、"proxy/eth0" 这类多实例命名；任一实例运行即视为运行
  const serviceMatches = (key: string) => serviceList.filter(s => s.name === key || s.name.startsWith(key + '/'))
  const serviceStatus = (key: string): 'running' | 'stopped' | 'error' => {
    const matches = serviceMatches(key)
    if (matches.some(s => s.status === 'running')) return 'running'
    if (matches.some(s => s.status === 'error')) return 'error'
    return 'stopped'
  }

  const services = [
    { name: 'DHCP', port: ':67', key: 'dhcp' },
    { name: 'ProxyDHCP', port: ':4011', key: 'proxy' },
    { name: 'TFTP', port: ':69', key: 'tftp' },
    { name: 'HTTP', port: ':8080', key: 'http' },
    { name: 'DNS', port: ':53', key: 'dns' },
    { name: 'NFS', port: ':2049', key: 'nfs' },
  ]

  // 未在详情卡中展示的运行服务（如 SMB），单独提示，避免 KPI 与卡片数量对不上
  const otherRunningServices = serviceList.filter(s =>
    s.status === 'running' && !services.some(svc => s.name === svc.key || s.name.startsWith(svc.key + '/'))
  )

  const dm = (k: string) => metrics?.services[k]?.metrics
  const dhcpData = metrics?.services.dhcp?.dhcp

  // 只统计所选窗口内的增量；首次轮询尚无增量时返回 null，界面显示 "—" 而不是把累计值伪装成窗口值
  const httpRecent = useMemo(() => {
    const now = Date.now()
    const maxAge = (HTTP_RANGE_CFG[httpRange] ?? HTTP_RANGE_CFG['5m']).seconds * 1000
    const cutoff = now - maxAge
    const relevant = httpDeltasRef.current.filter(d => d.time > cutoff)
    if (relevant.length === 0) return null
    return {
      requests: relevant.reduce((a, d) => a + d.requests, 0),
      rejected: relevant.reduce((a, d) => a + d.rejected, 0),
      status2xx: relevant.reduce((a, d) => a + d.status2xx, 0),
      status3xx: relevant.reduce((a, d) => a + d.status3xx, 0),
      status4xx: relevant.reduce((a, d) => a + d.status4xx, 0),
      status5xx: relevant.reduce((a, d) => a + d.status5xx, 0),
    }
  }, [metrics, httpRange])

  const httpStatusData = httpRecent && (httpRecent.status2xx > 0 || httpRecent.status3xx > 0 || httpRecent.status4xx > 0 || httpRecent.status5xx > 0)
    ? [
        { name: '2xx', value: httpRecent.status2xx, fill: 'var(--color-accent-green)' },
        { name: '3xx', value: httpRecent.status3xx, fill: 'var(--color-accent-cyan)' },
        { name: '4xx', value: httpRecent.status4xx, fill: 'var(--color-accent-yellow)' },
        { name: '5xx', value: httpRecent.status5xx, fill: 'var(--color-accent-red)' },
      ]
    : []

  // 每个服务在总览表中只保留最能说明问题的一组指标
  type MetricPair = { label: string; value: string; tone?: 'danger' | 'warn' }
  const serviceMetrics = (key: string): MetricPair[] => {
    const m = dm(key)
    switch (key) {
      case 'dhcp':
        return dhcpData ? [
          { label: t('dashboard.offerAckNak'), value: `${dhcpData.offers}/${dhcpData.acks}/${dhcpData.naks}` },
          { label: t('dashboard.discoverRequest'), value: `${dhcpData.discovers}/${dhcpData.requests}` },
          { label: t('dashboard.decline'), value: String(dhcpData.declines), tone: 'danger' },
          { label: t('dashboard.unauthorized'), value: String(dhcpData.unauthorized), tone: 'warn' },
        ] : []
      case 'tftp':
        return m ? [
          { label: t('dashboard.labelRequests'), value: String(m.requests ?? 0) },
          { label: t('dashboard.labelErrors'), value: String(m.errors ?? 0), tone: 'danger' },
          { label: t('dashboard.labelTrafficOut'), value: fmtBytes(m.bytesOut ?? 0) },
          { label: t('dashboard.labelRate'), value: `${avgRate(m.requestRate ?? [])}/s` },
        ] : []
      case 'http':
        return m ? [
          { label: t('dashboard.labelRequests'), value: httpRecent ? httpRecent.requests.toLocaleString() : '—' },
          { label: t('dashboard.labelActiveConns'), value: String(m.activeConns ?? 0) },
          { label: t('dashboard.labelStatusCodes'), value: httpRecent ? `${httpRecent.status2xx}/${httpRecent.status4xx}/${httpRecent.status5xx}` : '—' },
          { label: t('dashboard.labelRejected'), value: httpRecent ? String(httpRecent.rejected) : '—', tone: 'warn' },
        ] : []
      case 'dns':
        return m ? [
          { label: t('dashboard.labelDnsRecords'), value: String(dnsCount) },
          { label: t('dashboard.labelQueryCount'), value: String(m.requests ?? 0) },
          { label: t('dashboard.labelErrors'), value: String(m.errors ?? 0), tone: 'danger' },
          { label: t('dashboard.labelQueryRate'), value: `${avgRate(m.requestRate ?? [])}/s` },
        ] : []
      case 'nfs':
        return m ? [
          { label: t('dashboard.labelRequests'), value: String(m.requests ?? 0) },
          { label: t('dashboard.labelActiveConns'), value: String(m.activeConns ?? 0) },
          { label: t('dashboard.labelTransferTraffic'), value: fmtBytes((m.bytesOut ?? 0) + (m.bytesIn ?? 0)) },
          { label: t('dashboard.labelRate'), value: `${avgRate(m.requestRate ?? [])}/s` },
        ] : []
      default:
        return []
    }
  }

  // 一行一个服务：状态、端口、运行时长、关键指标同屏，替代原来「状态条 + 详情卡」两块重复信息
  const serviceRows = [
    ...services.map(svc => {
      const info = serviceMatches(svc.key)[0]
      return {
        key: svc.key,
        name: svc.name,
        port: `${svc.port}${info?.protocol ? `/${info.protocol.toUpperCase()}` : ''}`,
        status: serviceStatus(svc.key),
        uptime: fmtUptime(info?.started_at ?? null),
        startTime: info?.started_at ? new Date(info.started_at).toLocaleString() : '—',
        errorMsg: info?.error_msg || '',
        metrics: serviceMetrics(svc.key),
      }
    }),
    ...otherRunningServices.map(svc => ({
      key: svc.name,
      name: svc.display || svc.name,
      port: svc.port ? `:${svc.port}${svc.protocol ? `/${svc.protocol.toUpperCase()}` : ''}` : '',
      status: 'running' as const,
      uptime: fmtUptime(svc.started_at),
      startTime: svc.started_at ? new Date(svc.started_at).toLocaleString() : '—',
      errorMsg: '',
      metrics: [] as MetricPair[],
    })),
  ]

  const statCards = [
    { label: t('dashboard.stats.onlineHosts'), value: onlineHosts, color: 'green', icon: Server },
    { label: t('dashboard.stats.runningServices'), value: serviceRows.filter(r => r.status === 'running').length, color: 'blue', icon: Wifi },
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

  const eventIcon = (type: string) => {
    const t = type.toLowerCase()
    if (t.includes('dhcp')) return { label: 'DHCP', color: 'bg-cyan-500/10 text-cyan-text' }
    if (t.includes('tftp')) return { label: 'TFTP', color: 'bg-orange-500/10 text-orange-text' }
    if (t.includes('http')) return { label: 'HTTP', color: 'bg-purple-500/10 text-purple-text' }
    if (t.includes('boot')) return { label: 'BOOT', color: 'bg-accent-green/10 text-success' }
    if (t.includes('ipmi')) return { label: 'IPMI', color: 'bg-accent-yellow/10 text-warning' }
    if (t.includes('dns')) return { label: 'DNS', color: 'bg-violet-500/10 text-violet-400' }
    return { label: 'EVT', color: 'bg-blue-500/10 text-info' }
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <PageHeader
        title={t('dashboard.title')}
        description={t('dashboard.description')}
        size="lg"
        className="mb-0"
        actions={
          <Button variant="primary" onClick={() => navigate('/quick-install')}>
            <Rocket size={14} />
            {t('quickInstall.entryButton')}
          </Button>
        }
      />

      {/* Stats Cards */}
      {loading ? (
        <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-5 gap-3">
          {Array.from({ length: 5 }).map((_, i) => (
            <div key={i} className="rounded-2xl border border-[var(--bg-border)] bg-[var(--bg-card)] p-4 h-[106px] animate-shimmer" />
          ))}
        </div>
      ) : (
        <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-5 gap-3">
          {statCards.map((s, i) => {
            const Icon = s.icon
            return (
              <div key={i} className="group relative overflow-hidden rounded-2xl border border-[var(--bg-border)] bg-[var(--bg-card)] p-4 shadow-sm hover:shadow-lg hover:-translate-y-0.5 transition-all duration-300">
                <div className="relative">
                  <div className="flex items-center justify-between mb-2">
                    <span className="text-[11px] font-semibold uppercase tracking-wider text-[var(--text-muted)]">{s.label}</span>
                    <span className={`w-8 h-8 rounded-lg flex items-center justify-center transition-all duration-300 group-hover:scale-110 ${
                      s.color === 'green' ? 'bg-accent-green/10 text-success' :
                      s.color === 'blue' ? 'bg-blue-500/10 text-info' :
                      s.color === 'cyan' ? 'bg-cyan-500/10 text-cyan-text' :
                      s.color === 'violet' ? 'bg-violet-500/10 text-violet-400' :
                      'bg-orange-500/10 text-orange-text'
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

      {/* Service Overview */}
      <div className="overflow-hidden rounded-2xl border border-[var(--bg-border)] bg-[var(--bg-card)] shadow-sm">
        <div className="flex items-center justify-between px-5 py-3 border-b border-[var(--bg-border)]">
          <div className="flex items-center gap-2">
            <Zap size={14} className="text-info" />
            <span className="text-xs font-semibold uppercase tracking-wider text-[var(--text-muted)]">{t('dashboard.serviceOverview')}</span>
          </div>
          <button onClick={() => navigate('/services')} className="flex items-center gap-1 text-xs font-medium text-info hover:opacity-80 transition-colors">
            {t('nav.services')} <ChevronRight size={12} />
          </button>
        </div>
        <div className="hidden md:grid grid-cols-[minmax(0,1.2fr)_minmax(0,0.9fr)_minmax(0,2.4fr)] gap-4 px-5 py-1.5 text-[11px] font-semibold uppercase tracking-wider text-[var(--text-muted)] border-b border-[var(--bg-border)] bg-[var(--bg-base)]/30">
          <span>{t('dashboard.colService')}</span>
          <span>{t('dashboard.colStatus')}</span>
          <span>{t('dashboard.colMetrics')}</span>
        </div>
        {loading ? (
          <div>
            {services.map(svc => (
              <div key={svc.key} className="h-11 border-b border-[var(--bg-border)] last:border-b-0 animate-shimmer" />
            ))}
          </div>
        ) : (
          <div>
            {serviceRows.map(row => (
              <div key={row.key} className="grid grid-cols-1 md:grid-cols-[minmax(0,1.2fr)_minmax(0,0.9fr)_minmax(0,2.4fr)] md:items-center gap-1 md:gap-4 px-5 py-2.5 border-b border-[var(--bg-border)] last:border-b-0">
                <div className="flex items-center gap-2.5 min-w-0">
                  <StatusDot color={row.status === 'running' ? 'green' : row.status === 'error' ? 'red' : 'yellow'} pulse={row.status === 'running'} />
                  <span className="text-sm font-semibold text-[var(--text-primary)]">{row.name}</span>
                  <span className="text-[11px] font-mono text-[var(--text-muted)]">{row.port}</span>
                </div>
                <div className="flex items-center gap-2 min-w-0 pl-5 md:pl-0">
                  <span className={`text-[11px] font-semibold tracking-wide ${
                    row.status === 'running' ? 'text-success' : row.status === 'error' ? 'text-danger' : 'text-[var(--text-muted)]'
                  }`}>
                    {row.status === 'running' ? t('common.running') : row.status === 'error' ? t('common.error') : t('common.stopped')}
                  </span>
                  {row.status === 'running' && (
                    <span className="text-[11px] font-mono text-[var(--text-muted)]" title={`${t('dashboard.startedAt')} ${row.startTime}`}>{row.uptime}</span>
                  )}
                  {row.status === 'error' && row.errorMsg && (
                    <span className="text-[11px] font-mono text-danger/80 truncate" title={row.errorMsg}>{row.errorMsg}</span>
                  )}
                </div>
                <div className="flex flex-wrap items-center gap-x-4 gap-y-0.5 pl-5 md:pl-0 text-xs font-mono text-[var(--text-primary)]">
                  {row.metrics.length > 0 ? row.metrics.map(m => (
                    <span key={m.label} className="whitespace-nowrap">
                      <span className="text-[var(--text-muted)]">{m.label}</span>{' '}
                      <span className={m.tone === 'danger' ? 'text-danger' : m.tone === 'warn' ? 'text-warning' : ''}>{m.value}</span>
                    </span>
                  )) : (
                    <span className="text-[var(--text-muted)]">{row.status === 'stopped' ? t('dashboard.serviceNotRunning') : t('dashboard.noMetrics')}</span>
                  )}
                </div>
              </div>
            ))}
          </div>
        )}
        {dm('http') && (
          <p className="px-5 py-2 text-[11px] leading-snug text-[var(--text-muted)] border-t border-[var(--bg-border)]">{t('dashboard.httpWindowHint')}</p>
        )}
      </div>

      {/* Events */}
      <Card
        padding={false}
        title={
          <div className="flex items-center gap-2">
            <Activity size={14} className="text-info" />
            <span>{t('dashboard.recentEvents')}</span>
          </div>
        }
        footer={
          <button onClick={() => navigate('/events')} className="flex items-center gap-1 text-info hover:opacity-80 text-xs font-medium transition-colors">
            {t('common.viewAll')} <ChevronRight size={12} />
          </button>
        }
      >
        {events.length === 0 ? (
          <p className="text-sm text-[var(--text-muted)] text-center py-6">{t('dashboard.noEvents')}</p>
        ) : (
          <div>
            {events.slice(0, 6).map((e, i) => {
              const ic = eventIcon(e.type)
              return (
                <div key={i} className="flex items-center gap-3 px-5 py-2.5 border-b border-[var(--bg-border)] last:border-b-0 hover:bg-[var(--bg-hover)]/30 transition-colors group">
                  <div className={`w-7 h-7 rounded-lg flex items-center justify-center text-[10px] font-bold shrink-0 ${ic.color}`}>{ic.label}</div>
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2">
                      <span className="text-xs font-semibold text-[var(--text-primary)]">{e.type}</span>
                      {e.mac && <span className="text-[11px] font-mono text-[var(--text-muted)]">{e.mac}</span>}
                    </div>
                    <p className="text-[11px] text-[var(--text-muted)] truncate">{e.message}</p>
                  </div>
                  <span className="text-[11px] text-[var(--text-muted)] font-mono shrink-0 opacity-0 group-hover:opacity-100 transition-opacity">
                    {new Date(e.timestamp).toLocaleTimeString()}
                  </span>
                </div>
              )
            })}
          </div>
        )}
      </Card>

      {/* Charts Row */}
      <div className={`grid grid-cols-1 gap-5 ${archPieData.length > 0 ? 'lg:grid-cols-3' : 'lg:grid-cols-2'}`}>
        {archPieData.length > 0 && (
          <div className="rounded-2xl border border-[var(--bg-border)] bg-[var(--bg-card)] p-5 shadow-sm">
            <div className="flex items-center gap-2 mb-4">
              <Server size={14} className="text-cyan-text" />
              <span className="text-xs font-semibold uppercase tracking-wider text-[var(--text-muted)]">{t('dashboard.chartDhcpArch')}</span>
            </div>
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
          </div>
        )}

        <div className="rounded-2xl border border-[var(--bg-border)] bg-[var(--bg-card)] p-5 shadow-sm">
          <div className="flex items-center justify-between mb-4">
            <div className="flex items-center gap-2">
              <BarChart3 size={14} className="text-purple-text" />
              <span className="text-xs font-semibold uppercase tracking-wider text-[var(--text-muted)]">{t('dashboard.chartHttpStatus')}</span>
            </div>
            <div className="flex items-center gap-1 bg-[var(--bg-base)]/50 rounded-lg p-0.5">
              {[['5m', t('dashboard.range5m')], ['30m', t('dashboard.range30m')], ['1h', t('dashboard.range1h')]].map(([key, label]) => (
                <button key={key} onClick={() => setHttpRange(key as any)} className={`px-2 py-0.5 text-[11px] font-medium rounded-md transition-colors ${httpRange === key ? 'bg-blue-500/20 text-info' : 'text-[var(--text-muted)] hover:text-[var(--text-primary)]'}`}>
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
                  <YAxis tick={{ fontSize: 11, fill: 'var(--foreground-muted)' }} axisLine={false} tickLine={false} allowDecimals={false} />
                  <Tooltip content={<ChartTooltip />} />
                  <Bar dataKey="value" name={t('dashboard.httpCount')} radius={[4, 4, 0, 0]}>
                    {httpStatusData.map((d, i) => <Cell key={i} fill={d.fill} />)}
                  </Bar>
                </BarChart>
              </ResponsiveContainer>
            </div>
          ) : (
            <p className="text-xs text-[var(--text-muted)] text-center py-6">{t('common.noData')}</p>
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
              <BarChart3 size={14} className="text-success" />
              <span className="text-xs font-semibold uppercase tracking-wider text-[var(--text-muted)]">{t('dashboard.chartTraffic')}</span>
            </div>
            <div className="flex items-center gap-1 bg-[var(--bg-base)]/50 rounded-lg p-0.5">
              {[['5m', t('dashboard.range5m')], ['30m', t('dashboard.range30m')], ['1h', t('dashboard.range1h')]].map(([key, label]) => (
                <button key={key} onClick={() => setTrafficRange(key as any)} className={`px-2 py-0.5 text-[11px] font-medium rounded-md transition-colors ${trafficRange === key ? 'bg-blue-500/20 text-info' : 'text-[var(--text-muted)] hover:text-[var(--text-primary)]'}`}>
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
                    <XAxis dataKey="time" tick={{ fontSize: 11, fill: 'var(--foreground-muted)' }} axisLine={false} tickLine={false} interval="preserveStartEnd" />
                    <YAxis tick={{ fontSize: 11, fill: 'var(--foreground-muted)' }} axisLine={false} tickLine={false} tickFormatter={v => v >= 1024 * 1024 ? `${(v / 1024 / 1024).toFixed(0)}MB` : v >= 1024 ? `${(v / 1024).toFixed(0)}KB` : `${v}B`} />
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
                        <span className="text-[11px] font-semibold text-[var(--text-muted)]">{trafficLabels[svc]}</span>
                      </div>
                      <div className="text-xs font-mono text-[var(--text-primary)] font-semibold">{bwRate(m?.bandwidth ?? [])}</div>
                      <div className="flex items-center gap-2 mt-1 text-[11px] font-mono text-[var(--text-muted)]">
                        <span className="flex items-center gap-0.5"><ArrowUpRight size={10} className="text-success" />{fmtBytes(m?.bytesOut ?? 0)}</span>
                        <span className="flex items-center gap-0.5"><ArrowDownRight size={10} className="text-info" />{fmtBytes(m?.bytesIn ?? 0)}</span>
                      </div>
                    </div>
                  )
                })}
              </div>
            </>
          ) : (
            <p className="text-xs text-[var(--text-muted)] text-center py-6">{t('common.noData')}</p>
          )}
        </div>
      </div>

    </div>
  )
}
