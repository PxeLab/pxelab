import { useState, useEffect, useRef, useCallback, useMemo } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import {
  Activity, Server, ChevronRight, Wifi, Users, FileText, Rocket, RefreshCw, Info,
  Router, Share2, Download, Globe, Network, HardDrive,
} from 'lucide-react'
import { Button } from '../components/ui/Button'
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

function RangeTabs({ value, onChange }: { value: '5m' | '30m' | '1h'; onChange: (v: '5m' | '30m' | '1h') => void }) {
  const { t } = useTranslation()
  const labels = { '5m': 'range5m', '30m': 'range30m', '1h': 'range1h' } as const
  return (
    <div className="inline-flex h-[30px] items-center rounded-[5px] border border-[var(--bg-border)] bg-[var(--bg-base)]/60 p-0.5">
      {(['5m', '30m', '1h'] as const).map(key => (
        <button
          key={key}
          type="button"
          onClick={() => onChange(key)}
          aria-pressed={value === key}
          className={`h-6 rounded-[3px] px-2 text-xs font-medium transition-colors ${
            value === key
              ? 'bg-[var(--bg-card)] text-info shadow-sm'
              : 'text-[var(--text-muted)] hover:text-[var(--text-primary)]'
          }`}
        >
          {t(`dashboard.${labels[key]}`)}
        </button>
      ))}
    </div>
  )
}

const SERVICE_ICONS: Record<string, typeof Server> = {
  dhcp: Router,
  proxy: Share2,
  tftp: Download,
  http: Globe,
  dns: Network,
  nfs: HardDrive,
}

export default function Dashboard() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [events, setEvents] = useState<Event[]>([])
  const [eventTotal, setEventTotal] = useState(0)
  const [metrics, setMetrics] = useState<MetricsSnapshot | null>(null)
  const [dnsCount, setDnsCount] = useState(0)
  const [serviceList, setServiceList] = useState<ServiceInfo[]>([])
  const [onlineHosts, setOnlineHosts] = useState(0)
  const [todayBoots, setTodayBoots] = useState(0)
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)
  const [lastUpdated, setLastUpdated] = useState<Date | null>(null)
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
      setEventTotal(e.data.meta.total ?? 0)
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
      setLastUpdated(new Date())
    }
  }, [])

  const refresh = useCallback(async () => {
    setRefreshing(true)
    try { await load() } finally { setRefreshing(false) }
  }, [load])

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

  const runningServiceCount = serviceRows.filter(r => r.status === 'running').length

  const statCards = [
    { label: t('dashboard.stats.onlineHosts'), value: onlineHosts, icon: Server, hint: t('dashboard.kpiOnlineHint') },
    { label: t('dashboard.stats.runningServices'), value: runningServiceCount, icon: Wifi, hint: t('common.serviceCount', { count: runningServiceCount }) },
    { label: t('dashboard.activeLeases'), value: dhcpData?.activeLeases ?? 0, icon: Users, hint: t('dashboard.kpiLeaseHint') },
    { label: t('dashboard.dnsRecords'), value: dnsCount, icon: FileText, hint: t('dashboard.kpiDnsHint') },
    { label: t('dashboard.todayBoots'), value: todayBoots, icon: Activity, hint: t('dashboard.kpiBootsHint') },
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

  const fmtEventTime = (iso: string) => {
    try { return new Date(iso).toLocaleTimeString(undefined, { hour12: false }) } catch { return '—' }
  }

  const statusPill = (status: 'running' | 'stopped' | 'error') => {
    if (status === 'running') {
      return (
        <span className="inline-flex h-[22px] items-center gap-1.5 rounded-full border border-accent-green/25 bg-accent-green/10 px-2 text-[11.5px] font-medium text-success">
          <span className="h-[5px] w-[5px] rounded-full bg-accent-green" />
          {t('common.running')}
        </span>
      )
    }
    if (status === 'error') {
      return (
        <span className="inline-flex h-[22px] items-center gap-1.5 rounded-full border border-accent-red/25 bg-accent-red/10 px-2 text-[11.5px] font-medium text-danger">
          <span className="h-[5px] w-[5px] rounded-full bg-accent-red" />
          {t('common.error')}
        </span>
      )
    }
    return (
      <span className="inline-flex h-[22px] items-center gap-1.5 rounded-full border border-[var(--bg-border)] bg-[var(--bg-base)]/60 px-2 text-[11.5px] font-medium text-[var(--text-muted)]">
        <span className="h-[5px] w-[5px] rounded-full bg-[var(--text-muted)]" />
        {t('common.stopped')}
      </span>
    )
  }

  const metricList = (row: (typeof serviceRows)[number]) => (
    row.metrics.length > 0 ? (
      <div className="flex flex-wrap items-baseline gap-x-4 gap-y-1">
        {row.metrics.map(m => {
          const tone = m.tone === 'danger' ? 'text-danger' : m.tone === 'warn' ? 'text-warning' : 'text-[var(--text-primary)]'
          return (
            <span key={m.label} className="whitespace-nowrap">
              <span className="text-xs text-[var(--text-muted)]">{m.label}</span>{' '}
              <span className={'font-mono text-[13px] ' + tone}>{m.value}</span>
            </span>
          )
        })}
      </div>
    ) : (
      <span className="text-xs text-[var(--text-muted)]">
        {row.status === 'stopped' ? t('dashboard.serviceNotRunning') : t('dashboard.noMetrics')}
      </span>
    )
  )

  const rangeLabel = (r: '5m' | '30m' | '1h') =>
    t(r === '5m' ? 'dashboard.range5m' : r === '30m' ? 'dashboard.range30m' : 'dashboard.range1h')

  const levelBadgeClass = (level: string) => {
    const l = (level || 'INFO').toUpperCase()
    if (l === 'ERROR' || l === 'FATAL') return 'border-accent-red/25 bg-accent-red/10 text-danger'
    if (l === 'WARN' || l === 'WARNING') return 'border-accent-yellow/25 bg-accent-yellow/10 text-warning'
    return 'border-[var(--bg-border)] bg-[var(--bg-base)]/60 text-[var(--text-muted)]'
  }

  // 右侧已单独展示 MAC，消息末尾重复的 MAC 去掉，避免同一行出现两次
  const eventMessage = (message: string, mac?: string) => {
    const msg = (message || '').trimEnd()
    if (!mac || !msg.endsWith(mac)) return msg
    const stripped = msg.slice(0, -mac.length).replace(/[：:\s、-]+$/, '')
    return stripped || msg
  }

  return (
    <div className="space-y-[18px]">
      {/* Page header */}
      <div className="flex flex-wrap items-end justify-between gap-x-6 gap-y-3">
        <div className="min-w-0">
          <h1 className="text-2xl font-semibold tracking-tight text-[var(--text-primary)]">{t('dashboard.title')}</h1>
          <p className="mt-1 text-[13px] text-[var(--text-muted)]">{t('dashboard.description')}</p>
        </div>
        <div className="ml-auto flex flex-wrap items-center justify-end gap-x-4 gap-y-2">
          <button
            onClick={refresh}
            disabled={refreshing}
            aria-label={t('common.refresh')}
            className="inline-flex items-center gap-1.5 text-xs text-[var(--text-muted)] transition-colors hover:text-[var(--text-primary)] disabled:opacity-60"
          >
            <RefreshCw size={13} className={refreshing ? 'animate-spin' : ''} />
            {t('dashboard.lastUpdated')}
            <span className="font-mono text-[var(--text-primary)]">{lastUpdated ? lastUpdated.toLocaleTimeString() : '—'}</span>
          </button>
          <span className="hidden text-xs text-[var(--text-muted)] sm:inline">
            {t('dashboard.autoRefresh', { seconds: 5 })}
          </span>
          <Button variant="primary" className="bg-blue-600 hover:bg-blue-700" onClick={() => navigate('/quick-install')}>
            <Rocket size={14} />
            {t('quickInstall.entryButton')}
          </Button>
        </div>
      </div>

      {/* KPI */}
      {loading ? (
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-5">
          {Array.from({ length: 5 }).map((_, i) => (
            <div key={i} className="h-[96px] rounded-md border border-[var(--bg-border)] bg-[var(--bg-card)] animate-shimmer" />
          ))}
        </div>
      ) : (
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 md:grid-cols-5">
          {statCards.map((s, i) => {
            const Icon = s.icon
            return (
              <div key={i} className="min-w-0 rounded-md border border-[var(--bg-border)] bg-[var(--bg-card)] px-4 py-3 shadow-sm">
                <div className="flex items-start justify-between gap-2">
                  <span className="text-[12.5px] leading-snug text-[var(--text-muted)]">{s.label}</span>
                  <Icon size={16} className="mt-0.5 shrink-0 text-[var(--text-muted)]" />
                </div>
                <div className="mt-1.5 font-mono text-[29px] font-semibold leading-none tracking-tight text-[var(--text-primary)]">{s.value}</div>
                <div className="mt-1 truncate text-[11.5px] text-[var(--text-muted)]">{s.hint}</div>
              </div>
            )
          })}
        </div>
      )}

      {/* Service overview */}
      <section className="overflow-hidden rounded-md border border-[var(--bg-border)] bg-[var(--bg-card)] shadow-sm">
        <div className="flex min-h-[50px] flex-wrap items-center justify-between gap-x-4 gap-y-2 border-b border-[var(--bg-border)] px-4 py-2">
          <div className="flex flex-wrap items-baseline gap-2">
            <h2 className="text-[14.5px] font-semibold text-[var(--text-primary)]">{t('dashboard.serviceOverview')}</h2>
            <span className="text-xs text-[var(--text-muted)]">{t('common.serviceCount', { count: serviceRows.length })}</span>
          </div>
          <button
            onClick={() => navigate('/services')}
            className="inline-flex items-center gap-1 text-xs font-medium text-info transition-opacity hover:opacity-80"
          >
            {t('nav.services')} <ChevronRight size={13} />
          </button>
        </div>

        <div className="hidden h-9 grid-cols-[19%_12%_10%_11%_48%] items-center border-b border-[var(--bg-border)] bg-[var(--bg-base)]/40 px-4 text-[11.5px] font-semibold text-[var(--text-muted)] md:grid">
          <span>{t('dashboard.colService')}</span>
          <span>{t('dashboard.colPort')}</span>
          <span>{t('dashboard.colStatus')}</span>
          <span className="text-right pr-6">{t('dashboard.colUptime')}</span>
          <span className="pl-6">{t('dashboard.colMetrics')}</span>
        </div>

        {loading ? (
          <div>
            {services.map(svc => (
              <div key={svc.key} className="h-[50px] border-b border-[var(--bg-border)]/60 last:border-b-0 animate-shimmer" />
            ))}
          </div>
        ) : (
          <div>
            {serviceRows.map(row => {
              const Icon = SERVICE_ICONS[row.key] ?? Server
              return (
                <div key={row.key} className="border-b border-[var(--bg-border)]/60 last:border-b-0">
                  {/* Desktop */}
                  <div className="hidden min-h-[50px] grid-cols-[19%_12%_10%_11%_48%] items-center px-4 hover:bg-[var(--bg-base)]/40 md:grid">
                    <div className="flex min-w-0 items-center gap-2.5">
                      <Icon size={15} className="shrink-0 text-[var(--text-muted)]" />
                      <span className="truncate text-[13.5px] font-semibold text-[var(--text-primary)]">{row.name}</span>
                    </div>
                    <span className="whitespace-nowrap font-mono text-[12.5px] text-[var(--text-secondary)]">{row.port || '—'}</span>
                    <span>{statusPill(row.status)}</span>
                    <span className="pr-6 text-right font-mono text-[12.5px] text-[var(--text-secondary)]">{row.uptime}</span>
                    <div className="min-w-0 pl-6">{metricList(row)}</div>
                  </div>

                  {/* Mobile */}
                  <div className="px-4 py-3 md:hidden">
                    <div className="flex items-center justify-between gap-3">
                      <div className="flex min-w-0 items-center gap-2">
                        <Icon size={15} className="shrink-0 text-[var(--text-muted)]" />
                        <span className="truncate text-sm font-semibold text-[var(--text-primary)]">{row.name}</span>
                      </div>
                      {statusPill(row.status)}
                    </div>
                    <div className="mt-1 font-mono text-[11.5px] text-[var(--text-muted)]">
                      {row.port || '—'} · {row.uptime}
                    </div>
                    <div className="mt-1.5">{metricList(row)}</div>
                  </div>
                </div>
              )
            })}
          </div>
        )}

        {dm('http') && (
          <div className="flex items-start gap-1.5 border-t border-[var(--bg-border)] bg-[var(--bg-base)]/30 px-4 py-2.5 text-[11.5px] leading-snug text-[var(--text-muted)]">
            <span className="mt-0.5 shrink-0"><Info size={13} /></span>
            <span>{t('dashboard.httpWindowHint')}</span>
          </div>
        )}
      </section>

      {/* Events + charts */}
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-[minmax(0,1.45fr)_minmax(0,1fr)]">
        <section className="flex h-full min-h-0 flex-col overflow-hidden rounded-md border border-[var(--bg-border)] bg-[var(--bg-card)] shadow-sm">
          <div className="flex min-h-[50px] items-center justify-between gap-3 border-b border-[var(--bg-border)] px-4 py-2">
            <div className="flex items-baseline gap-2">
              <h2 className="text-[14.5px] font-semibold text-[var(--text-primary)]">{t('dashboard.recentEvents')}</h2>
              <span className="text-xs text-[var(--text-muted)]">{t('dashboard.eventCount', { count: eventTotal })}</span>
            </div>
            <button
              onClick={() => navigate('/events')}
              className="inline-flex items-center gap-1 text-xs font-medium text-info transition-opacity hover:opacity-80"
            >
              {t('common.viewAll')} <ChevronRight size={13} />
            </button>
          </div>

          {events.length === 0 ? (
            <div className="flex flex-1 items-center justify-center py-10 text-sm text-[var(--text-muted)]">{t('dashboard.noEvents')}</div>
          ) : (
            <div className="flex flex-1 flex-col">
              {events.slice(0, 8).map((e, i) => (
                <div key={i} className="flex min-h-[52px] max-h-[64px] flex-1 items-center gap-3 border-b border-[var(--bg-border)]/60 px-4 py-2 last:border-b-0 hover:bg-[var(--bg-base)]/40">
                  <span className="w-[52px] shrink-0 font-mono text-[11.5px] text-[var(--text-muted)]">{fmtEventTime(e.timestamp)}</span>
                  <span className="shrink-0 rounded border border-[var(--bg-border)] bg-[var(--bg-base)]/60 px-1.5 py-0.5 text-center font-mono text-[10.5px] uppercase text-[var(--text-secondary)]">
                    {e.type}
                  </span>
                  <div className="min-w-0 flex-1">
                    <div className="truncate text-[13px] text-[var(--text-primary)]">{eventMessage(e.message, e.mac)}</div>
                    {e.mac && <div className="truncate font-mono text-[11px] text-[var(--text-muted)] sm:hidden">{e.mac}</div>}
                  </div>
                  <div className="hidden w-[132px] shrink-0 flex-col items-end gap-1 sm:flex">
                    {e.mac && <span className="max-w-full truncate font-mono text-[11px] text-[var(--text-secondary)]">{e.mac}</span>}
                    <span className={'rounded border px-1.5 py-0.5 font-mono text-[10px] leading-none ' + levelBadgeClass(e.level)}>
                      {(e.level || 'INFO').toUpperCase()}
                    </span>
                  </div>
                </div>
              ))}
            </div>
          )}
        </section>

        <div className="flex flex-col gap-4">
          {/* HTTP status */}
          <section className="overflow-hidden rounded-md border border-[var(--bg-border)] bg-[var(--bg-card)] shadow-sm">
            <div className="flex min-h-[50px] flex-wrap items-center justify-between gap-2 border-b border-[var(--bg-border)] px-4 py-2">
              <h2 className="text-[14.5px] font-semibold text-[var(--text-primary)]">{t('dashboard.chartHttpStatus')}</h2>
              <RangeTabs value={httpRange} onChange={setHttpRange} />
            </div>
            <div className="p-4">
              <div className="mb-3 flex flex-wrap items-center gap-1.5 text-xs text-[var(--text-muted)]">
                <span>{t('dashboard.rangeSelected', { range: rangeLabel(httpRange) })}</span>
                <span className="opacity-50">·</span>
                <span>{t('dashboard.requestsInWindow', { count: httpRecent ? httpRecent.requests : 0 })}</span>
              </div>
              {httpStatusData.some(d => d.value > 0) ? (
                <div className="h-[140px]">
                  <ResponsiveContainer width="100%" height="100%">
                    <BarChart data={httpStatusData} margin={{ top: 8, right: 8, left: -18, bottom: 0 }}>
                      <CartesianGrid strokeDasharray="3 3" vertical={false} stroke="var(--border)" strokeOpacity={0.6} />
                      <XAxis dataKey="name" tick={{ fontSize: 11, fill: 'var(--foreground-muted)' }} axisLine={false} tickLine={false} />
                      <YAxis tick={{ fontSize: 11, fill: 'var(--foreground-muted)' }} axisLine={false} tickLine={false} allowDecimals={false} />
                      <Tooltip content={<ChartTooltip />} />
                      <Bar dataKey="value" name={t('dashboard.httpCount')} radius={[3, 3, 0, 0]} barSize={28} isAnimationActive={false}>
                        {httpStatusData.map((d, i) => <Cell key={i} fill={d.fill} />)}
                      </Bar>
                    </BarChart>
                  </ResponsiveContainer>
                </div>
              ) : (
                <div className="flex h-[140px] items-center justify-center text-xs text-[var(--text-muted)]">{t('common.noData')}</div>
              )}
              <div className="mt-3 flex flex-wrap gap-x-5 gap-y-1.5 text-xs text-[var(--text-muted)]">
                {httpStatusData.map(d => (
                  <span key={d.name} className="inline-flex items-center gap-1.5">
                    <span className="h-2 w-2 rounded-sm" style={{ background: d.fill }} />
                    {d.name}<span className="font-mono text-[var(--text-primary)]">{d.value}</span>
                  </span>
                ))}
              </div>
            </div>
          </section>

          {/* Traffic */}
          <section className="overflow-hidden rounded-md border border-[var(--bg-border)] bg-[var(--bg-card)] shadow-sm">
            <div className="flex min-h-[50px] flex-wrap items-center justify-between gap-2 border-b border-[var(--bg-border)] px-4 py-2">
              <h2 className="text-[14.5px] font-semibold text-[var(--text-primary)]">{t('dashboard.chartTraffic')}</h2>
              <RangeTabs value={trafficRange} onChange={setTrafficRange} />
            </div>
            <div className="p-4">
              <div className="mb-3 flex flex-wrap items-center gap-x-5 gap-y-1.5 text-xs">
                {trafficServices.map(svc => (
                  <span key={svc} className="inline-flex items-center gap-2 whitespace-nowrap">
                    <span className="h-[2px] w-4 rounded-full" style={{ background: trafficColors[svc] }} />
                    <span className="font-medium text-[var(--text-primary)]">{trafficLabels[svc]}</span>
                    <span className="font-mono text-[var(--text-muted)]">{bwRate(metrics?.services[svc]?.metrics?.bandwidth ?? [])}</span>
                  </span>
                ))}
              </div>
              {trafficChartData.length > 0 ? (
                <div className="h-[120px]">
                  <ResponsiveContainer width="100%" height="100%">
                    <LineChart data={trafficChartData} margin={{ top: 5, right: 8, left: -20, bottom: 0 }}>
                      <CartesianGrid strokeDasharray="3 3" stroke="var(--border)" strokeOpacity={0.6} />
                      <XAxis dataKey="time" tick={{ fontSize: 10.5, fill: 'var(--foreground-muted)' }} axisLine={false} tickLine={false} interval="preserveStartEnd" />
                      <YAxis tick={{ fontSize: 10.5, fill: 'var(--foreground-muted)' }} axisLine={false} tickLine={false} tickFormatter={v => v >= 1024 * 1024 ? (v / 1024 / 1024).toFixed(0) + 'MB' : v >= 1024 ? (v / 1024).toFixed(0) + 'KB' : v + 'B'} />
                      <Tooltip content={<ChartTooltip formatter={(v: any) => fmtBytes(Math.round(Number(v)))} />} />
                      {trafficServices.map(svc => (
                        <Line key={svc} type="monotone" dataKey={svc} name={trafficLabels[svc]} stroke={trafficColors[svc]} strokeWidth={1.5} dot={false} isAnimationActive={false} />
                      ))}
                    </LineChart>
                  </ResponsiveContainer>
                </div>
              ) : (
                <div className="flex h-[120px] items-center justify-center text-xs text-[var(--text-muted)]">{t('common.noData')}</div>
              )}
            </div>
          </section>

          {/* DHCP arch (only when present) */}
          {archPieData.length > 0 && (
            <section className="overflow-hidden rounded-md border border-[var(--bg-border)] bg-[var(--bg-card)] shadow-sm">
              <div className="flex min-h-[50px] items-center gap-2 border-b border-[var(--bg-border)] px-4 py-2">
                <h2 className="text-[14.5px] font-semibold text-[var(--text-primary)]">{t('dashboard.chartDhcpArch')}</h2>
              </div>
              <div className="h-[170px] p-2">
                <ResponsiveContainer width="100%" height="100%">
                  <PieChart>
                    <Pie data={archPieData} dataKey="value" nameKey="name" cx="50%" cy="50%" outerRadius={62} isAnimationActive={false} label={({ name, percent }: PieLabelRenderProps) => (name ?? '') + ' ' + (((percent ?? 0) * 100).toFixed(0)) + '%'}>
                      {archPieData.map((_, i) => <Cell key={i} fill={PIE_COLORS[i % PIE_COLORS.length]} />)}
                    </Pie>
                    <Tooltip content={<ChartTooltip />} />
                  </PieChart>
                </ResponsiveContainer>
              </div>
            </section>
          )}
        </div>
      </div>
    </div>
  )
}
