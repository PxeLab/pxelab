import { useState, useEffect, useRef, useCallback } from 'react'
import { useTranslation } from 'react-i18next'
import { Card } from '../components/ui/Card'
import { Button } from '../components/ui/Button'
import { StatusDot } from '../components/ui/StatusDot'
import { PageHeader } from '../components/ui/PageHeader'
import { useToast } from '../components/ui/Toast'
import { Input, Select } from '../components/ui/FormControls'
import { DataTable, type Column } from '../components/ui/DataTable'
import { api, type PingPacket, type PingResult, type TracerouteHop, type NetworkInterface } from '../api/client'
import { RefreshCw } from 'lucide-react'

type Tab = 'ping' | 'traceroute'

export default function NetworkDiagnostics() {
  const { t } = useTranslation()
  const [tab, setTab] = useState<Tab>('ping')
  const [interfaces, setInterfaces] = useState<NetworkInterface[]>([])

  useEffect(() => {
    api.getNetworkInterfaces().then(res => setInterfaces(res.data || [])).catch(() => {})
  }, [])

  return (
    <div className="space-y-6">
      <PageHeader title={t('network.title')} className="mb-0" />

      {/* ── Tabs ── */}
      <div className="flex gap-1 border-b border-[var(--bg-border)]">
        {(['ping', 'traceroute'] as Tab[]).map(key => (
          <button
            key={key}
            onClick={() => setTab(key)}
            className={`px-4 py-2.5 text-sm font-medium border-b-2 transition-colors -mb-[1px] ${
              tab === key
                ? 'border-blue-500 text-blue-400'
                : 'border-transparent text-[var(--text-muted)] hover:text-[var(--text-primary)]'
            }`}
          >
            {key === 'ping' ? 'Ping' : 'Traceroute'}
          </button>
        ))}
      </div>

      {tab === 'ping' && <PingPanel interfaces={interfaces} />}
      {tab === 'traceroute' && <TraceroutePanel interfaces={interfaces} />}
    </div>
  )
}

function PingPanel({ interfaces }: { interfaces: NetworkInterface[] }) {
  const { t } = useTranslation()
  const { error: toastError, success: toastSuccess } = useToast()
  const [host, setHost] = useState('')
  const [count, setCount] = useState(4)
  const [continuous, setContinuous] = useState(false)
  const [size, setSize] = useState(64)
  const [ttl, setTtl] = useState(64)
  const [intervalMs, setIntervalMs] = useState(1000)
  const [timeoutMs, setTimeoutMs] = useState(2000)
  const [iface, setIface] = useState('')
  const [advanced, setAdvanced] = useState(false)
  const [loading, setLoading] = useState(false)
  const [result, setResult] = useState<PingResult | null>(null)
  const [packets, setPackets] = useState<PingPacket[]>([])
  const abortRef = useRef<{ close(): void } | null>(null)

  const doPing = useCallback(async () => {
    if (!host.trim()) {
      toastError(t('network.hostRequired'))
      return
    }
    setLoading(true)
    setResult(null)
    setPackets([])

    const opts = {
      host,
      count: continuous ? 0 : count,
      timeout_ms: timeoutMs,
      interval_ms: intervalMs,
      size,
      ttl,
      interface: iface || undefined,
    }

    // 单次与连续统一走 SSE 流式：包到达即渲染；单次在 summary 时收尾
    const es = api.networkPingStream(opts, (pkt) => {
      if ('type' in pkt && pkt.type === 'error') {
        setLoading(false)
        toastError((pkt as { message?: string }).message || t('network.pingFailed'))
        abortRef.current = null
        return
      }
      if ('type' in pkt && pkt.type === 'summary') {
        setLoading(false)
        if (!continuous) {
          const s = pkt as any
          setResult({ host, packets: [], ...s } as unknown as PingResult)
          if (s.received > 0) toastSuccess(t('network.pingSuccess'))
          abortRef.current = null
        }
      } else {
        // 持续模式只保留最近 500 条，防止无限增长
        setPackets(prev => continuous && prev.length >= 500 ? [...prev.slice(-499), pkt as PingPacket] : [...prev, pkt as PingPacket])
      }
    }, (err) => {
      setLoading(false)
      toastError(err.message || t('network.pingFailed'))
      abortRef.current = null
    })
    abortRef.current = es
  }, [host, count, continuous, size, ttl, intervalMs, timeoutMs, iface])

  const stopPing = () => {
    abortRef.current?.close()
    abortRef.current = null
    setLoading(false)
  }

  const formatRTT = (rtt: number) => {
    if (!rtt) return '-'
    if (rtt < 1) return `${(rtt * 1000).toFixed(0)}µs`
    return `${rtt.toFixed(2)}ms`
  }

  const packetColumns: Column<PingPacket>[] = [
    { key: 'seq', label: '#', render: pkt => <span className="font-mono text-[var(--text-primary)]">{pkt.seq}</span> },
    { key: 'rtt', label: t('network.rtt'), render: pkt => <span className="font-mono text-[var(--text-primary)]">{pkt.error ? '-' : formatRTT(pkt.rtt)}</span> },
    { key: 'ttl', label: 'TTL', render: pkt => pkt.ttl || '-' },
    { key: 'bytes', label: 'Bytes', render: pkt => pkt.bytes || '-' },
    { key: 'status', label: t('network.status'), render: pkt => pkt.error ? (
      <span className="text-accent-red">{pkt.error}</span>
    ) : (
      <StatusDot color="green" />
    ) },
  ]

  return (
    <Card>
      <div className="p-5">
        {/* All controls in one row */}
        <div className="flex flex-wrap gap-3 items-end">
          <div className="w-48">
            <label className="text-[11px] font-medium text-[var(--text-muted)] mb-1 block">{t('network.target')}</label>
            <Input
              size="xs"
              className="px-2.5 py-1.5 rounded-lg font-mono"
              type="text"
              value={host}
              onChange={e => setHost(e.target.value)}
              placeholder={t('network.hostPlaceholder')}
              onKeyDown={e => e.key === 'Enter' && !loading && doPing()}
            />
          </div>
          <div className="w-16">
            <label className="text-[11px] font-medium text-[var(--text-muted)] mb-1 block">{t('network.count')}</label>
            <Input
              size="xs"
              className="px-2.5 py-1.5 rounded-lg font-mono disabled:opacity-40"
              type="number"
              value={count}
              onChange={e => setCount(Math.max(1, parseInt(e.target.value) || 1))}
              disabled={continuous}
            />
          </div>
          <div className="w-44">
            <label className="text-[11px] font-medium text-[var(--text-muted)] mb-1 block">{t('network.interface')}</label>
            <Select
              size="xs"
              className="px-2.5 py-1.5 rounded-lg"
              value={iface}
              onChange={e => setIface(e.target.value)}
            >
              <option value="">{t('network.auto')}</option>
              {interfaces.map(iface => (
                <option key={iface.name} value={iface.name}>
                  {iface.name} ({iface.ips[0]})
                </option>
              ))}
            </Select>
          </div>
          <label className="flex items-center gap-1.5 cursor-pointer select-none pb-0.5">
            <input
              type="checkbox"
              checked={continuous}
              onChange={e => setContinuous(e.target.checked)}
              className="w-3.5 h-3.5 rounded border-[var(--bg-border)] bg-[var(--bg-input)] text-blue-500 focus:ring-blue-500/30"
            />
            <span className="text-xs text-[var(--text-secondary)]">{t('network.continuous')}</span>
          </label>
          <button
            onClick={() => setAdvanced(!advanced)}
            className="text-xs text-blue-400 hover:text-blue-300 flex items-center gap-1 pb-0.5"
          >
            <span className={`transition-transform ${advanced ? 'rotate-90' : ''}`}>▶</span>
            {t('network.advancedOptions')}
          </button>
          <div className="flex gap-2 ml-auto">
            <Button onClick={doPing} disabled={loading || !host.trim()}>
              {t('network.pingButton')}
            </Button>
            {loading && (
              <Button onClick={stopPing} variant="danger">
                {t('network.stop')}
              </Button>
            )}
          </div>
        </div>

        {/* Advanced Options */}
        {advanced && (
          <div className="flex flex-wrap gap-4 p-3 rounded-lg bg-[var(--bg-base)] border border-[var(--bg-border)]">
            <div>
              <label className="text-xs font-medium text-[var(--text-muted)] mb-1 block">{t('network.packetSize')}</label>
              <input
                type="number"
                value={size}
                onChange={e => setSize(Math.max(0, parseInt(e.target.value) || 0))}
                className="w-24 px-3 py-1.5 rounded bg-[var(--bg-input)] border border-[var(--bg-border)] text-[var(--text-primary)] text-xs font-mono focus:outline-none focus:border-blue-500"
              />
              <span className="text-[10px] text-[var(--text-muted)] ml-1">bytes</span>
            </div>
            <div>
              <label className="text-xs font-medium text-[var(--text-muted)] mb-1 block">TTL</label>
              <input
                type="number"
                value={ttl}
                onChange={e => setTtl(Math.max(1, Math.min(255, parseInt(e.target.value) || 64)))}
                className="w-20 px-3 py-1.5 rounded bg-[var(--bg-input)] border border-[var(--bg-border)] text-[var(--text-primary)] text-xs font-mono focus:outline-none focus:border-blue-500"
              />
            </div>
            <div>
              <label className="text-xs font-medium text-[var(--text-muted)] mb-1 block">{t('network.interval')}</label>
              <input
                type="number"
                value={intervalMs}
                onChange={e => setIntervalMs(Math.max(100, parseInt(e.target.value) || 1000))}
                className="w-24 px-3 py-1.5 rounded bg-[var(--bg-input)] border border-[var(--bg-border)] text-[var(--text-primary)] text-xs font-mono focus:outline-none focus:border-blue-500"
              />
              <span className="text-[10px] text-[var(--text-muted)] ml-1">ms</span>
            </div>
            <div>
              <label className="text-xs font-medium text-[var(--text-muted)] mb-1 block">{t('network.timeout')}</label>
              <input
                type="number"
                value={timeoutMs}
                onChange={e => setTimeoutMs(Math.max(500, parseInt(e.target.value) || 2000))}
                className="w-24 px-3 py-1.5 rounded bg-[var(--bg-input)] border border-[var(--bg-border)] text-[var(--text-primary)] text-xs font-mono focus:outline-none focus:border-blue-500"
              />
              <span className="text-[10px] text-[var(--text-muted)] ml-1">ms</span>
            </div>
          </div>
        )}

        {/* Stats */}
        {(result || packets.length > 0) && (
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 text-sm">
            {result && (
              <>
                <div className="p-3 rounded-lg bg-[var(--bg-base)]">
                  <div className="text-[var(--text-muted)] text-xs">{t('network.ip')}</div>
                  <div className="text-[var(--text-primary)] font-mono text-xs mt-0.5">{result.ip}</div>
                </div>
                <div className="p-3 rounded-lg bg-[var(--bg-base)]">
                  <div className="text-[var(--text-muted)] text-xs">{t('network.received')}</div>
                  <div className="text-accent-green font-semibold">{result.received}/{result.sent}</div>
                </div>
                <div className="p-3 rounded-lg bg-[var(--bg-base)]">
                  <div className="text-[var(--text-muted)] text-xs">{t('network.lost')}</div>
                  <div className="text-accent-red font-semibold">{result.lost}</div>
                </div>
                <div className="p-3 rounded-lg bg-[var(--bg-base)]">
                  <div className="text-[var(--text-muted)] text-xs">{t('network.avgRTT')}</div>
                  <div className="text-[var(--text-primary)] font-semibold">{formatRTT(result.avg_rtt)}</div>
                </div>
              </>
            )}
            {!result && packets.length > 0 && (
              <div className="col-span-2 sm:col-span-4 p-3 rounded-lg bg-[var(--bg-base)]">
                <div className="text-[var(--text-muted)] text-xs">{t('network.packetsReceived')}</div>
                <div className="text-[var(--text-primary)] font-semibold">{packets.filter(p => !p.error).length}</div>
              </div>
            )}
          </div>
        )}

        {/* Packet Table */}
        {packets.length > 0 && (
          <div className="max-h-[400px] overflow-y-auto">
            <DataTable columns={packetColumns} data={packets} stickyHeader />
          </div>
        )}
      </div>
    </Card>
  )
}

function TraceroutePanel({ interfaces }: { interfaces: NetworkInterface[] }) {
  const { t } = useTranslation()
  const { error: toastError } = useToast()
  const [host, setHost] = useState('')
  const [maxHops, setMaxHops] = useState(30)
  const [probes, setProbes] = useState(3)
  const [timeoutMs, setTimeoutMs] = useState(2000)
  const [iface, setIface] = useState('')
  const [advanced, setAdvanced] = useState(false)
  const [loading, setLoading] = useState(false)
  const [hops, setHops] = useState<TracerouteHop[]>([])
  const [targetIp, setTargetIp] = useState('')
  const abortRef = useRef<{ close(): void } | null>(null)

  const doTraceroute = () => {
    if (!host.trim()) {
      toastError(t('network.hostRequired'))
      return
    }
    setLoading(true)
    setHops([])
    setTargetIp('')
    // SSE 流式：逐跳到达即渲染
    const es = api.networkTracerouteStream({
      host,
      max_hops: maxHops,
      timeout_ms: timeoutMs,
      probes,
      interface: iface || undefined,
    }, (data) => {
      if ('type' in data && data.type === 'error') {
        setLoading(false)
        toastError(data.message || t('network.tracerouteFailed'))
        abortRef.current = null
        return
      }
      if ('type' in data && data.type === 'summary') {
        setTargetIp(data.ip)
        setLoading(false)
        abortRef.current = null
      } else {
        setHops(prev => [...prev, data as TracerouteHop])
      }
    }, (err) => {
      setLoading(false)
      toastError(err.message || t('network.tracerouteFailed'))
      abortRef.current = null
    })
    abortRef.current = es
  }

  const stopTraceroute = () => {
    abortRef.current?.close()
    abortRef.current = null
    setLoading(false)
  }

  const formatRTT = (rtt: number) => {
    if (!rtt) return '-'
    if (rtt < 1) return `${(rtt * 1000).toFixed(0)}µs`
    return `${rtt.toFixed(2)}ms`
  }

  const hopAvgRTT = (rtts: number[]) => {
    if (!rtts || rtts.length === 0) return 0
    return rtts.reduce((a, b) => a + b, 0) / rtts.length
  }

  const hopColumns: Column<TracerouteHop>[] = [
    { key: 'ttl', label: '#', render: hop => <span className="font-mono text-[var(--text-primary)]">{hop.ttl}</span> },
    { key: 'ip', label: t('network.ip'), render: hop => <span className="font-mono text-[var(--text-primary)]">{hop.timeout ? '*' : hop.ip}</span> },
    { key: 'host', label: t('network.host'), className: 'truncate max-w-[200px]', render: hop => (
      <span className="text-[var(--text-muted)]">{hop.timeout ? '' : hop.host || '-'}</span>
    ) },
    { key: 'rtt', label: '1', className: 'text-right font-mono text-[var(--text-primary)]', render: hop => hop.rtt ? (
      formatRTT(hop.rtt)
    ) : (
      <span className="text-accent-red">*</span>
    ) },
  ]
  if (probes > 1) {
    hopColumns.push({ key: 'rtt2', label: '2', className: 'text-right font-mono text-[var(--text-primary)]', render: hop => (hop.rtts?.[1] ? formatRTT(hop.rtts[1]) : hop.timeout ? '*' : '-') })
  }
  if (probes > 2) {
    hopColumns.push({ key: 'rtt3', label: '3', className: 'text-right font-mono text-[var(--text-primary)]', render: hop => (hop.rtts?.[2] ? formatRTT(hop.rtts[2]) : hop.timeout ? '*' : '-') })
  }
  hopColumns.push({ key: 'avg_rtt', label: t('network.avgRTT'), className: 'text-right font-mono text-[var(--text-primary)]', render: hop => (hop.timeout ? '' : formatRTT(hopAvgRTT(hop.rtts))) })

  return (
    <Card>
      <div className="p-5">
        {/* All controls in one row */}
        <div className="flex flex-wrap gap-3 items-end">
          <div className="w-48">
            <label className="text-[11px] font-medium text-[var(--text-muted)] mb-1 block">{t('network.target')}</label>
            <Input
              size="xs"
              className="px-2.5 py-1.5 rounded-lg font-mono"
              type="text"
              value={host}
              onChange={e => setHost(e.target.value)}
              placeholder={t('network.hostPlaceholder')}
              onKeyDown={e => e.key === 'Enter' && !loading && doTraceroute()}
            />
          </div>
          <div className="w-44">
            <label className="text-[11px] font-medium text-[var(--text-muted)] mb-1 block">{t('network.interface')}</label>
            <Select
              size="xs"
              className="px-2.5 py-1.5 rounded-lg"
              value={iface}
              onChange={e => setIface(e.target.value)}
            >
              <option value="">{t('network.auto')}</option>
              {interfaces.map(iface => (
                <option key={iface.name} value={iface.name}>
                  {iface.name} ({iface.ips[0]})
                </option>
              ))}
            </Select>
          </div>
          <button
            onClick={() => setAdvanced(!advanced)}
            className="text-xs text-blue-400 hover:text-blue-300 flex items-center gap-1 pb-0.5"
          >
            <span className={`transition-transform ${advanced ? 'rotate-90' : ''}`}>▶</span>
            {t('network.advancedOptions')}
          </button>
          <div className="ml-auto flex gap-2">
            <Button onClick={doTraceroute} disabled={loading || !host.trim()}>
              {loading ? '...' : t('network.tracerouteButton')}
            </Button>
            {loading && (
              <Button onClick={stopTraceroute} variant="danger">
                {t('network.stop')}
              </Button>
            )}
          </div>
        </div>

        {/* Advanced Options */}
        {advanced && (
          <div className="flex flex-wrap gap-4 p-3 rounded-lg bg-[var(--bg-base)] border border-[var(--bg-border)]">
            <div>
              <label className="text-xs font-medium text-[var(--text-muted)] mb-1 block">{t('network.maxHops')}</label>
              <input
                type="number"
                value={maxHops}
                onChange={e => setMaxHops(Math.max(1, Math.min(64, parseInt(e.target.value) || 30)))}
                className="w-20 px-3 py-1.5 rounded bg-[var(--bg-input)] border border-[var(--bg-border)] text-[var(--text-primary)] text-xs font-mono focus:outline-none focus:border-blue-500"
              />
            </div>
            <div>
              <label className="text-xs font-medium text-[var(--text-muted)] mb-1 block">{t('network.probes')}</label>
              <input
                type="number"
                value={probes}
                onChange={e => setProbes(Math.max(1, Math.min(10, parseInt(e.target.value) || 3)))}
                className="w-20 px-3 py-1.5 rounded bg-[var(--bg-input)] border border-[var(--bg-border)] text-[var(--text-primary)] text-xs font-mono focus:outline-none focus:border-blue-500"
              />
            </div>
            <div>
              <label className="text-xs font-medium text-[var(--text-muted)] mb-1 block">{t('network.timeout')}</label>
              <input
                type="number"
                value={timeoutMs}
                onChange={e => setTimeoutMs(Math.max(500, parseInt(e.target.value) || 2000))}
                className="w-24 px-3 py-1.5 rounded bg-[var(--bg-input)] border border-[var(--bg-border)] text-[var(--text-primary)] text-xs font-mono focus:outline-none focus:border-blue-500"
              />
              <span className="text-[10px] text-[var(--text-muted)] ml-1">ms</span>
            </div>
          </div>
        )}

        {/* Result（流式：hop 到达即追加） */}
        {(hops.length > 0 || targetIp) && (
          <div className="space-y-3">
            <div className="flex items-center gap-4 text-xs text-[var(--text-muted)]">
              {targetIp && <span>{targetIp}</span>}
              <span>{hops.length} {t('network.hops')}</span>
              {loading && <RefreshCw size={12} className="animate-spin" />}
            </div>

            <DataTable columns={hopColumns} data={hops} />
          </div>
        )}
      </div>
    </Card>
  )
}
