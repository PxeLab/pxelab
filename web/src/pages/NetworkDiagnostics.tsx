import { useState, useEffect, useRef, useCallback } from 'react'
import { useTranslation } from 'react-i18next'
import { Card } from '../components/ui/Card'
import { Button } from '../components/ui/Button'
import { StatusDot } from '../components/ui/StatusDot'
import { useToast } from '../components/ui/Toast'
import { api, type PingPacket, type PingResult, type TracerouteResult, type NetworkInterface } from '../api/client'

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
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-bold text-[var(--text-primary)]">{t('network.title')}</h1>
      </div>

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

    if (continuous) {
      // SSE streaming mode
      const es = api.networkPingStream(opts, (pkt) => {
        if ('type' in pkt && pkt.type === 'summary') {
          setLoading(false)
        } else {
          setPackets(prev => [...prev, pkt as PingPacket])
        }
      })
      abortRef.current = es
    } else {
      try {
        const res = await api.networkPing(opts)
        setResult(res.data)
        setPackets(res.data.packets)
        if (res.data.reachable) {
          toastSuccess(t('network.pingSuccess'))
        }
      } catch (err: any) {
        toastError(err.message || t('network.pingFailed'))
      } finally {
        setLoading(false)
      }
    }
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

  return (
    <Card>
      <div className="p-5">
        {/* All controls in one row */}
        <div className="flex flex-wrap gap-3 items-end">
          <div className="w-48">
            <label className="text-[11px] font-medium text-[var(--text-muted)] mb-1 block">{t('network.target')}</label>
            <input
              type="text"
              value={host}
              onChange={e => setHost(e.target.value)}
              placeholder={t('network.hostPlaceholder')}
              className="w-full px-2.5 py-1.5 rounded-lg bg-[var(--bg-input)] border border-[var(--bg-border)] text-[var(--text-primary)] text-sm font-mono focus:outline-none focus:border-blue-500 focus:ring-1 focus:ring-blue-500/30"
              onKeyDown={e => e.key === 'Enter' && !loading && doPing()}
            />
          </div>
          <div className="w-16">
            <label className="text-[11px] font-medium text-[var(--text-muted)] mb-1 block">{t('network.count')}</label>
            <input
              type="number"
              value={count}
              onChange={e => setCount(Math.max(1, parseInt(e.target.value) || 1))}
              disabled={continuous}
              className="w-full px-2.5 py-1.5 rounded-lg bg-[var(--bg-input)] border border-[var(--bg-border)] text-[var(--text-primary)] text-sm font-mono disabled:opacity-40 focus:outline-none focus:border-blue-500"
            />
          </div>
          <div className="w-44">
            <label className="text-[11px] font-medium text-[var(--text-muted)] mb-1 block">{t('network.interface')}</label>
            <select
              value={iface}
              onChange={e => setIface(e.target.value)}
              className="w-full px-2.5 py-1.5 rounded-lg bg-[var(--bg-input)] border border-[var(--bg-border)] text-[var(--text-primary)] text-sm focus:outline-none focus:border-blue-500"
            >
              <option value="">{t('network.auto')}</option>
              {interfaces.map(iface => (
                <option key={iface.name} value={iface.name}>
                  {iface.name} ({iface.ips[0]})
                </option>
              ))}
            </select>
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
            {loading && continuous && (
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
                  <div className="text-green-400 font-semibold">{result.received}/{result.sent}</div>
                </div>
                <div className="p-3 rounded-lg bg-[var(--bg-base)]">
                  <div className="text-[var(--text-muted)] text-xs">{t('network.lost')}</div>
                  <div className="text-red-400 font-semibold">{result.lost}</div>
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
          <div className="overflow-x-auto max-h-[400px] overflow-y-auto">
            <table className="w-full text-xs">
              <thead className="sticky top-0 bg-[var(--bg-card)]">
                <tr className="text-[var(--text-muted)] border-b border-[var(--bg-border)]">
                  <th className="text-left py-2 px-2">#</th>
                  <th className="text-left py-2 px-2">{t('network.rtt')}</th>
                  <th className="text-left py-2 px-2">TTL</th>
                  <th className="text-left py-2 px-2">Bytes</th>
                  <th className="text-left py-2 px-2">{t('network.status')}</th>
                </tr>
              </thead>
              <tbody>
                {packets.map((pkt, i) => (
                  <tr key={i} className="border-b border-[var(--bg-border)] last:border-0 hover:bg-[var(--bg-hover)]/30">
                    <td className="py-1.5 px-2 font-mono text-[var(--text-primary)]">{pkt.seq}</td>
                    <td className="py-1.5 px-2 font-mono text-[var(--text-primary)]">{pkt.error ? '-' : formatRTT(pkt.rtt)}</td>
                    <td className="py-1.5 px-2 text-[var(--text-secondary)]">{pkt.ttl || '-'}</td>
                    <td className="py-1.5 px-2 text-[var(--text-secondary)]">{pkt.bytes || '-'}</td>
                    <td className="py-1.5 px-2">
                      {pkt.error ? (
                        <span className="text-red-400">{pkt.error}</span>
                      ) : (
                        <StatusDot color="green" />
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
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
  const [result, setResult] = useState<TracerouteResult | null>(null)

  const doTraceroute = async () => {
    if (!host.trim()) {
      toastError(t('network.hostRequired'))
      return
    }
    setLoading(true)
    setResult(null)
    try {
      const res = await api.networkTraceroute({
        host,
        max_hops: maxHops,
        timeout_ms: timeoutMs,
        probes,
        interface: iface || undefined,
      })
      setResult(res.data)
    } catch (err: any) {
      toastError(err.message || t('network.tracerouteFailed'))
    } finally {
      setLoading(false)
    }
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

  return (
    <Card>
      <div className="p-5">
        {/* All controls in one row */}
        <div className="flex flex-wrap gap-3 items-end">
          <div className="w-48">
            <label className="text-[11px] font-medium text-[var(--text-muted)] mb-1 block">{t('network.target')}</label>
            <input
              type="text"
              value={host}
              onChange={e => setHost(e.target.value)}
              placeholder={t('network.hostPlaceholder')}
              className="w-full px-2.5 py-1.5 rounded-lg bg-[var(--bg-input)] border border-[var(--bg-border)] text-[var(--text-primary)] text-sm font-mono focus:outline-none focus:border-blue-500 focus:ring-1 focus:ring-blue-500/30"
              onKeyDown={e => e.key === 'Enter' && !loading && doTraceroute()}
            />
          </div>
          <div className="w-44">
            <label className="text-[11px] font-medium text-[var(--text-muted)] mb-1 block">{t('network.interface')}</label>
            <select
              value={iface}
              onChange={e => setIface(e.target.value)}
              className="w-full px-2.5 py-1.5 rounded-lg bg-[var(--bg-input)] border border-[var(--bg-border)] text-[var(--text-primary)] text-sm focus:outline-none focus:border-blue-500"
            >
              <option value="">{t('network.auto')}</option>
              {interfaces.map(iface => (
                <option key={iface.name} value={iface.name}>
                  {iface.name} ({iface.ips[0]})
                </option>
              ))}
            </select>
          </div>
          <button
            onClick={() => setAdvanced(!advanced)}
            className="text-xs text-blue-400 hover:text-blue-300 flex items-center gap-1 pb-0.5"
          >
            <span className={`transition-transform ${advanced ? 'rotate-90' : ''}`}>▶</span>
            {t('network.advancedOptions')}
          </button>
          <div className="ml-auto">
            <Button onClick={doTraceroute} disabled={loading || !host.trim()}>
              {loading ? '...' : t('network.tracerouteButton')}
            </Button>
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

        {/* Result */}
        {result && (
          <div className="space-y-3">
            <div className="flex items-center gap-4 text-xs text-[var(--text-muted)]">
              <span>{result.ip}</span>
              <span>{result.hops.length} {t('network.hops')}</span>
            </div>

            <div className="overflow-x-auto">
              <table className="w-full text-xs">
                <thead>
                  <tr className="text-[var(--text-muted)] border-b border-[var(--bg-border)]">
                    <th className="text-left py-2 px-2">#</th>
                    <th className="text-left py-2 px-2">{t('network.ip')}</th>
                    <th className="text-left py-2 px-2">{t('network.host')}</th>
                    <th className="text-right py-2 px-2">1</th>
                    {probes > 1 && <th className="text-right py-2 px-2">2</th>}
                    {probes > 2 && <th className="text-right py-2 px-2">3</th>}
                    <th className="text-right py-2 px-2">{t('network.avgRTT')}</th>
                  </tr>
                </thead>
                <tbody>
                  {result.hops.map((hop, i) => (
                    <tr key={i} className="border-b border-[var(--bg-border)] last:border-0 hover:bg-[var(--bg-hover)]/30">
                      <td className="py-2 px-2 font-mono text-[var(--text-primary)]">{hop.ttl}</td>
                      <td className="py-2 px-2 font-mono text-[var(--text-primary)]">
                        {hop.timeout ? '*' : hop.ip}
                      </td>
                      <td className="py-2 px-2 text-[var(--text-muted)] truncate max-w-[200px]">
                        {hop.timeout ? '' : hop.host || '-'}
                      </td>
                      {hop.rtt ? (
                        <td className="py-2 px-2 text-right font-mono text-[var(--text-primary)]">{formatRTT(hop.rtt)}</td>
                      ) : (
                        <td className="py-2 px-2 text-right text-red-400">*</td>
                      )}
                      {probes > 1 && (
                        <td className="py-2 px-2 text-right font-mono text-[var(--text-primary)]">
                          {hop.rtts?.[1] ? formatRTT(hop.rtts[1]) : hop.timeout ? '*' : '-'}
                        </td>
                      )}
                      {probes > 2 && (
                        <td className="py-2 px-2 text-right font-mono text-[var(--text-primary)]">
                          {hop.rtts?.[2] ? formatRTT(hop.rtts[2]) : hop.timeout ? '*' : '-'}
                        </td>
                      )}
                      <td className="py-2 px-2 text-right font-mono text-[var(--text-primary)]">
                        {hop.timeout ? '' : formatRTT(hopAvgRTT(hop.rtts))}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        )}
      </div>
    </Card>
  )
}
