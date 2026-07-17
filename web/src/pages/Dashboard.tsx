import { useState, useEffect, useRef, useCallback } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { Activity, Server, Zap, ChevronRight, Wifi, BarChart3, Users, FileText } from 'lucide-react'
import { StatusDot } from '../components/ui/StatusDot'
import { Card } from '../components/ui/Card'
import { api, type ServiceStatus, type Host, type Event, type MetricsSnapshot, type TimeBucket } from '../api/client'
import { XAxis, YAxis, Tooltip, ResponsiveContainer, PieChart, Pie, Cell, BarChart, Bar, type PieLabelRenderProps } from 'recharts'

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

const PIE_COLORS = ['#22d3ee', '#f59e0b', '#a78bfa', '#34d399', '#f472b6', '#f97316', '#06b6d4', '#84cc16']

export default function Dashboard() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [status, setStatus] = useState<ServiceStatus | null>(null)
  const [hosts, setHosts] = useState<Host[]>([])
  const [events, setEvents] = useState<Event[]>([])
  const [metrics, setMetrics] = useState<MetricsSnapshot | null>(null)
  const [dnsCount, setDnsCount] = useState(0)
  const [loading, setLoading] = useState(true)
  const intervalRef = useRef<ReturnType<typeof setInterval>>(undefined)

  const load = useCallback(async () => {
    try {
      const [s, h, e, m, d] = await Promise.all([
        api.getStatus(),
        api.getHosts({ page: '1', size: '8' }),
        api.getEvents({ page: '1', size: '12' }),
        api.getMetrics(),
        api.getDNSRecords().catch(() => ({ data: { records: [] } })),
      ])
      setStatus(s.data)
      setHosts(h.data.hosts ?? [])
      setEvents(e.data.events ?? [])
      setMetrics(m.data)
      setDnsCount(d.data.records.length)
    } catch (err) {
      console.error('Failed to load dashboard', err)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { load(); intervalRef.current = window.setInterval(load, 5000); return () => clearInterval(intervalRef.current) }, [load])

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

  const dm = (k: string) => metrics?.services[k]?.metrics
  const dhcpData = metrics?.services.dhcp?.dhcp
  const httpData = metrics?.services.http?.http

  const todayBoots = events.filter(e => {
    const d = new Date(e.timestamp)
    const now = new Date()
    return d.toDateString() === now.toDateString()
  }).length

  const statCards = [
    { label: t('dashboard.stats.onlineHosts'), value: hosts.length, color: 'green', icon: Server, desc: '在线主机' },
    { label: '运行服务', value: services.filter(s => serviceStatus(s.key) === 'running').length, color: 'blue', icon: Wifi, desc: `共 ${services.length} 个` },
    { label: '活跃租约', value: dhcpData?.activeLeases ?? 0, color: 'cyan', icon: Users, desc: `发现 ${dhcpData?.discovers ?? 0}` },
    { label: 'DNS 记录', value: dnsCount, color: 'violet', icon: FileText, desc: '解析记录' },
    { label: '今日启动', value: todayBoots, color: 'orange', icon: Activity, desc: 'PXE 启动事件' },
  ]

  const archPieData = dhcpData?.archBreakdown
    ? Object.entries(dhcpData.archBreakdown).map(([name, value]) => ({ name, value }))
    : []

  const httpStatusData = httpData
    ? [
        { name: '2xx', value: httpData.status2xx, fill: '#34d399' },
        { name: '3xx', value: httpData.status3xx, fill: '#22d3ee' },
        { name: '4xx', value: httpData.status4xx, fill: '#f59e0b' },
        { name: '5xx', value: httpData.status5xx, fill: '#ef4444' },
      ]
    : []

  const eventIcon = (type: string) => {
    const t = type.toLowerCase()
    if (t.includes('dhcp')) return { label: 'DHCP', color: 'bg-cyan-500/10 text-cyan-400' }
    if (t.includes('tftp')) return { label: 'TFTP', color: 'bg-orange-500/10 text-orange-400' }
    if (t.includes('http')) return { label: 'HTTP', color: 'bg-purple-500/10 text-purple-400' }
    if (t.includes('boot')) return { label: 'BOOT', color: 'bg-green-500/10 text-green-400' }
    if (t.includes('ipmi')) return { label: 'IPMI', color: 'bg-yellow-500/10 text-yellow-400' }
    if (t.includes('dns')) return { label: 'DNS', color: 'bg-violet-500/10 text-violet-400' }
    return { label: 'EVT', color: 'bg-blue-500/10 text-blue-400' }
  }

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
                const colorMap: Record<string, string> = { running: 'bg-green-500', stopped: 'bg-gray-400', error: 'bg-red-500' }
                return <span key={svc.key} className={`w-2 h-2 rounded-full ${colorMap[st]}`} />
              })}
            </div>
          </div>
          <div className="grid grid-cols-2 sm:grid-cols-5 gap-3">
            {services.map(svc => {
              const st = serviceStatus(svc.key)
              const color = st === 'running' ? 'green' : st === 'error' ? 'red' : 'yellow'
              const m = dm(svc.key)
              return (
                <div key={svc.name} className="flex items-center gap-3 px-3 py-2.5 rounded-xl bg-[var(--bg-base)]/50 border border-[var(--bg-border)]/50">
                  <StatusDot color={color as any} pulse={st === 'running'} />
                  <div className="min-w-0 flex-1">
                    <div className="text-sm font-semibold text-[var(--text-primary)]">{svc.name}</div>
                    <div className="text-[10px] font-mono text-[var(--text-muted)]">{svc.port}</div>
                    {m && <div className="text-[9px] font-mono text-[var(--text-muted)] mt-0.5">{avgRate(m.requestRate)}/s · {fmtBytes(m.bytesOut + m.bytesIn)}</div>}
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
                      s.color === 'green' ? 'bg-green-500/10 text-green-400' :
                      s.color === 'blue' ? 'bg-blue-500/10 text-blue-400' :
                      s.color === 'cyan' ? 'bg-cyan-500/10 text-cyan-400' :
                      s.color === 'violet' ? 'bg-violet-500/10 text-violet-400' :
                      'bg-orange-500/10 text-orange-400'
                    }`}>
                      <Icon size={16} />
                    </span>
                  </div>
                  <div className="text-2xl font-bold tracking-tight text-[var(--text-primary)]">{s.value}</div>
                  {s.desc && <div className="text-[10px] text-[var(--text-muted)] mt-0.5">{s.desc}</div>}
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
              <div className="flex justify-between"><span className="text-[var(--text-muted)]">Offer/Ack/Nak</span><span className="font-mono text-[var(--text-primary)]">{dhcpData.offers}/{dhcpData.acks}/{dhcpData.naks}</span></div>
              <div className="flex justify-between"><span className="text-[var(--text-muted)]">Discover/Request</span><span className="font-mono text-[var(--text-primary)]">{dhcpData.discovers}/{dhcpData.requests}</span></div>
              <div className="flex justify-between"><span className="text-[var(--text-muted)]">Decline</span><span className="font-mono text-red-400">{dhcpData.declines}</span></div>
              <div className="flex justify-between"><span className="text-[var(--text-muted)]">未授权</span><span className="font-mono text-yellow-400">{dhcpData.unauthorized}</span></div>
            </div>
          ) : (
            <p className="text-xs text-[var(--text-muted)]">{serviceStatus('dhcp') === 'stopped' ? '服务未运行' : '暂无数据'}</p>
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
              <div className="flex justify-between"><span className="text-[var(--text-muted)]">请求</span><span className="font-mono text-[var(--text-primary)]">{dm('tftp')?.requests ?? 0}</span></div>
              <div className="flex justify-between"><span className="text-[var(--text-muted)]">错误</span><span className="font-mono text-red-400">{dm('tftp')?.errors ?? 0}</span></div>
              <div className="flex justify-between"><span className="text-[var(--text-muted)]">下发流量</span><span className="font-mono text-[var(--text-primary)]">{fmtBytes(dm('tftp')?.bytesOut ?? 0)}</span></div>
              <div className="flex justify-between"><span className="text-[var(--text-muted)]">速率</span><span className="font-mono text-[var(--text-primary)]">{avgRate(dm('tftp')?.requestRate ?? [])}/s</span></div>
            </div>
          ) : (
            <p className="text-xs text-[var(--text-muted)]">{serviceStatus('tftp') === 'stopped' ? '服务未运行' : '暂无数据'}</p>
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
              <div className="flex justify-between"><span className="text-[var(--text-muted)]">解析记录</span><span className="font-mono text-[var(--text-primary)]">{dnsCount}</span></div>
              <div className="flex justify-between"><span className="text-[var(--text-muted)]">查询次数</span><span className="font-mono text-[var(--text-primary)]">{dm('dns')?.requests ?? 0}</span></div>
              <div className="flex justify-between"><span className="text-[var(--text-muted)]">错误</span><span className="font-mono text-red-400">{dm('dns')?.errors ?? 0}</span></div>
              <div className="flex justify-between"><span className="text-[var(--text-muted)]">查询速率</span><span className="font-mono text-[var(--text-primary)]">{avgRate(dm('dns')?.requestRate ?? [])}/s</span></div>
            </div>
          ) : (
            <p className="text-xs text-[var(--text-muted)]">{serviceStatus('dns') === 'stopped' ? '服务未运行' : '暂无数据'}</p>
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
              <div className="flex justify-between"><span className="text-[var(--text-muted)]">请求</span><span className="font-mono text-[var(--text-primary)]">{dm('http')?.requests ?? 0}</span></div>
              <div className="flex justify-between"><span className="text-[var(--text-muted)]">活跃连接</span><span className="font-mono text-[var(--text-primary)]">{dm('http')?.activeConns ?? 0}</span></div>
              <div className="flex justify-between"><span className="text-[var(--text-muted)]">2xx/4xx/5xx</span><span className="font-mono text-[var(--text-primary)]">{httpData?.status2xx ?? 0}/{httpData?.status4xx ?? 0}/{httpData?.status5xx ?? 0}</span></div>
              <div className="flex justify-between"><span className="text-[var(--text-muted)]">拒绝</span><span className="font-mono text-yellow-400">{dm('http')?.rejected ?? 0}</span></div>
            </div>
          ) : (
            <p className="text-xs text-[var(--text-muted)]">{serviceStatus('http') === 'stopped' ? '服务未运行' : '暂无数据'}</p>
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
              <div className="flex justify-between"><span className="text-[var(--text-muted)]">请求</span><span className="font-mono text-[var(--text-primary)]">{dm('nfs')?.requests ?? 0}</span></div>
              <div className="flex justify-between"><span className="text-[var(--text-muted)]">活跃连接</span><span className="font-mono text-[var(--text-primary)]">{dm('nfs')?.activeConns ?? 0}</span></div>
              <div className="flex justify-between"><span className="text-[var(--text-muted)]">传输流量</span><span className="font-mono text-[var(--text-primary)]">{fmtBytes((dm('nfs')?.bytesOut ?? 0) + (dm('nfs')?.bytesIn ?? 0))}</span></div>
              <div className="flex justify-between"><span className="text-[var(--text-muted)]">速率</span><span className="font-mono text-[var(--text-primary)]">{avgRate(dm('nfs')?.requestRate ?? [])}/s</span></div>
            </div>
          ) : (
            <p className="text-xs text-[var(--text-muted)]">{serviceStatus('nfs') === 'stopped' ? '服务未运行' : '暂无数据'}</p>
          )}
        </div>
      </div>

      {/* Charts Row */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-5">
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
              {dhcpData.platformBreakdown && Object.keys(dhcpData.platformBreakdown).length > 0 && (
                <div className="h-28">
                  <ResponsiveContainer width="100%" height="100%">
                    <PieChart>
                      <Pie data={Object.entries(dhcpData.platformBreakdown).map(([name, value]) => ({ name, value }))} dataKey="value" nameKey="name" cx="50%" cy="50%" outerRadius={40} innerRadius={25}>
                        {Object.entries(dhcpData.platformBreakdown).map((_, i) => <Cell key={i} fill={PIE_COLORS[i % PIE_COLORS.length]} />)}
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
