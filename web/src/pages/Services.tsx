import { useState, useEffect, useCallback, useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { StatusDot } from '../components/ui/StatusDot'
import { Card } from '../components/ui/Card'
import { Toggle } from '../components/ui/Toggle'
import { api, type ServiceInfo } from '../api/client'

const statusColor = (s: string) => {
  if (s === 'running') return 'green'
  if (s === 'error') return 'red'
  return 'yellow'
}

const statusLabel = (s: string, t: any) => {
  if (s === 'running') return t('services.statusRunning', '运行中')
  if (s === 'stopped') return t('services.statusStopped', '已停止')
  return t('services.statusError', '错误')
}

/** 服务分组定义 */
interface ServiceGroup {
  type: string
  services: ServiceInfo[]
  isPerNIC: boolean
}

export default function Services() {
  const { t } = useTranslation()
  const [services, setServices] = useState<ServiceInfo[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [operating, setOperating] = useState<Set<string>>(new Set())
  const [errorDetail, setErrorDetail] = useState<string | null>(null)

  const load = useCallback(async () => {
    try {
      const res = await api.getServices()
      setServices(res.data)
      setError('')
    } catch (err: any) {
      setError(err.message || '加载失败')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { load() }, [load])

  // Auto-refresh every 5s
  useEffect(() => {
    const interval = setInterval(load, 5000)
    return () => clearInterval(interval)
  }, [load])

  /** 按服务类型分组 */
  const groups = useMemo<ServiceGroup[]>(() => [
    { type: 'DHCP', services: services.filter(s => s.name.startsWith('dhcp/')), isPerNIC: true },
    { type: 'ProxyDHCP', services: services.filter(s => s.name.startsWith('proxy/')), isPerNIC: true },
    { type: 'TFTP', services: services.filter(s => s.name === 'tftp'), isPerNIC: false },
    { type: 'HTTP', services: services.filter(s => s.name === 'http'), isPerNIC: false },
    { type: 'DNS', services: services.filter(s => s.name === 'dns'), isPerNIC: false },
  ], [services])

  const toggleSelect = (name: string) => {
    const svc = services.find(s => s.name === name)
    if (svc?.protected) return
    setSelected(prev => {
      const next = new Set(prev)
      if (next.has(name)) next.delete(name)
      else next.add(name)
      return next
    })
  }

  const toggleAll = () => {
    const manageable = services.filter(s => !s.protected)
    if (selected.size === manageable.length && manageable.every(s => selected.has(s.name))) {
      setSelected(new Set())
    } else {
      setSelected(new Set(manageable.map(s => s.name)))
    }
  }

  const doOp = async (name: string, op: 'start' | 'stop' | 'restart') => {
    setOperating(prev => new Set(prev).add(name + op))
    try {
      if (op === 'start') await api.startService(name)
      else if (op === 'stop') await api.stopService(name)
      else await api.restartService(name)
      await load()
    } catch (err: any) {
      setError(err.message || '操作失败')
    } finally {
      setOperating(prev => {
        const next = new Set(prev)
        next.delete(name + op)
        return next
      })
    }
  }

  const doBatch = async (op: 'start' | 'stop' | 'restart') => {
    if (selected.size === 0) return
    const names = Array.from(selected)
    setOperating(prev => {
      const next = new Set(prev)
      names.forEach(n => next.add(n + op))
      return next
    })
    try {
      await api.batchService(op, names)
      await load()
      setSelected(new Set())
    } catch (err: any) {
      setError(err.message || '批量操作失败')
    } finally {
      setOperating(prev => {
        const next = new Set(prev)
        names.forEach(n => next.delete(n + op))
        return next
      })
    }
  }

  const toggleAutoStart = async (name: string, enabled: boolean) => {
    try {
      await api.updateAutoStart(name, enabled)
      await load()
    } catch (err: any) {
      setError(err.message || '操作失败')
    }
  }

  const isOperating = (name: string, op: string) => operating.has(name + op)
  const anySelected = selected.size > 0

  /** 渲染单个服务行 */
  const renderRow = (svc: ServiceInfo, indent?: boolean) => {
    const sel = selected.has(svc.name)
    const color = statusColor(svc.status)
    return (
      <tr key={svc.name} className={`hover:bg-white/[0.02] transition-colors ${sel ? 'bg-blue-500/5' : ''}`}>
        <td className="px-4 py-3 border-b border-[var(--bg-border)]">
          <input
            type="checkbox"
            checked={sel}
            disabled={svc.protected}
            onChange={() => toggleSelect(svc.name)}
            className="rounded border-[var(--bg-border)] bg-[var(--bg-input)] disabled:opacity-30"
          />
        </td>
        <td className="px-4 py-3 border-b border-[var(--bg-border)]">
          <div className="flex items-center gap-2">
            <span className={`font-medium text-[var(--text-primary)] ${indent ? 'ml-5 text-xs' : 'text-sm'}`}>
              {indent ? svc.display.replace(/^(DHCP|ProxyDHCP) /, '') : svc.display}
            </span>
            {svc.protected && (
              <span className="inline-block px-1.5 py-0.5 text-[10px] font-semibold uppercase tracking-wider rounded-full bg-blue-500/15 text-blue-400">
                Core
              </span>
            )}
          </div>
        </td>
        <td className="px-4 py-3 border-b border-[var(--bg-border)]">
          <div className="flex items-center gap-1.5">
            <StatusDot color={color as any} />
            <span className={`text-xs font-mono ${
              color === 'green' ? 'text-green-400' :
              color === 'red' ? 'text-red-400' :
              'text-yellow-400'
            }`}>{statusLabel(svc.status, t)}</span>
            {svc.status === 'error' && svc.error_msg && (
              <button
                onClick={() => setErrorDetail(svc.error_msg)}
                className="ml-1 text-[10px] text-red-400/60 hover:text-red-400 underline"
                title="查看错误详情"
              >
                ✕
              </button>
            )}
          </div>
        </td>
        <td className="px-4 py-3 border-b border-[var(--bg-border)]">
          <span className="text-xs font-mono text-[var(--text-muted)]">
            {svc.port}/{svc.protocol}
          </span>
        </td>
        <td className="px-4 py-3 border-b border-[var(--bg-border)]">
          <span className="text-xs font-mono text-[var(--text-muted)]">{svc.name}</span>
        </td>
        <td className="px-4 py-3 border-b border-[var(--bg-border)]">
          <Toggle
            checked={svc.auto_start}
            onChange={v => toggleAutoStart(svc.name, v)}
            disabled={svc.protected}
          />
        </td>
        <td className="px-4 py-3 border-b border-[var(--bg-border)] text-right">
          {svc.protected ? (
            <span className="inline-block px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wider rounded-full bg-blue-500/15 text-blue-400">
              Core
            </span>
          ) : (
            <div className="flex items-center justify-end gap-1.5">
              {svc.status !== 'running' && (
                <button
                  onClick={() => doOp(svc.name, 'start')}
                  disabled={isOperating(svc.name, 'start')}
                  className="px-2.5 py-1 text-xs font-medium rounded-lg bg-green-500/15 text-green-400 hover:bg-green-500/25 disabled:opacity-40 transition-colors"
                >
                  {isOperating(svc.name, 'start') ? '...' : t('services.start', '启动')}
                </button>
              )}
              {svc.status === 'running' && (
                <button
                  onClick={() => doOp(svc.name, 'stop')}
                  disabled={isOperating(svc.name, 'stop')}
                  className="px-2.5 py-1 text-xs font-medium rounded-lg bg-red-500/15 text-red-400 hover:bg-red-500/25 disabled:opacity-40 transition-colors"
                >
                  {isOperating(svc.name, 'stop') ? '...' : t('services.stop', '停止')}
                </button>
              )}
              <button
                onClick={() => doOp(svc.name, 'restart')}
                disabled={isOperating(svc.name, 'restart')}
                className="px-2.5 py-1 text-xs font-medium rounded-lg bg-orange-500/15 text-orange-400 hover:bg-orange-500/25 disabled:opacity-40 transition-colors"
              >
                {isOperating(svc.name, 'restart') ? '...' : t('services.restart', '重启')}
              </button>
            </div>
          )}
        </td>
      </tr>
    )
  }

  return (
    <div>
      {/* Header */}
      <div className="flex items-center justify-between mb-5">
        <div>
          <h2 className="text-base font-bold text-[var(--text-primary)]">{t('services.title', '服务管理')}</h2>
          <p className="text-xs text-[var(--text-muted)] mt-0.5">{t('services.subtitle', '查看和管理所有服务')}</p>
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={load}
            className="px-3 py-1.5 text-xs font-medium rounded-lg border border-[var(--bg-border)] bg-[var(--bg-card)] text-[var(--text-secondary)] hover:bg-[var(--bg-hover)] transition-colors"
          >
            {t('common.refresh', '刷新')}
          </button>
        </div>
      </div>

      {error && (
        <div className="mb-4 px-4 py-3 rounded-lg bg-red-500/10 border border-red-500/20 text-sm text-red-400">
          {error}
          <button onClick={() => setError('')} className="float-right text-red-400/60 hover:text-red-400">✕</button>
        </div>
      )}

      {/* Batch toolbar */}
      {anySelected && (
        <div className="mb-4 flex items-center gap-3 px-4 py-2.5 rounded-lg bg-blue-500/10 border border-blue-500/20">
          <span className="text-xs text-blue-400 font-medium">
            {t('services.selected', '已选择 {{count}} 个服务', { count: selected.size })}
          </span>
          <div className="flex gap-2 ml-auto">
            <button
              onClick={() => doBatch('start')}
              disabled={!anySelected}
              className="px-3 py-1 text-xs font-medium rounded-lg bg-green-500/15 text-green-400 hover:bg-green-500/25 disabled:opacity-40 transition-colors"
            >
              {t('services.batchStart', '批量启动')}
            </button>
            <button
              onClick={() => doBatch('stop')}
              disabled={!anySelected}
              className="px-3 py-1 text-xs font-medium rounded-lg bg-red-500/15 text-red-400 hover:bg-red-500/25 disabled:opacity-40 transition-colors"
            >
              {t('services.batchStop', '批量停止')}
            </button>
            <button
              onClick={() => doBatch('restart')}
              disabled={!anySelected}
              className="px-3 py-1 text-xs font-medium rounded-lg bg-orange-500/15 text-orange-400 hover:bg-orange-500/25 disabled:opacity-40 transition-colors"
            >
              {t('services.batchRestart', '批量重启')}
            </button>
          </div>
        </div>
      )}

      {/* Error detail modal */}
      {errorDetail && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50" onClick={() => setErrorDetail(null)}>
          <div className="max-w-lg w-full mx-4 rounded-xl bg-[var(--bg-card)] border border-[var(--bg-border)] p-5" onClick={e => e.stopPropagation()}>
            <h3 className="text-sm font-bold text-[var(--text-primary)] mb-3">错误详情</h3>
            <pre className="text-xs text-red-400 bg-red-500/5 rounded-lg p-3 overflow-auto max-h-48 whitespace-pre-wrap break-all">{errorDetail}</pre>
            <button onClick={() => setErrorDetail(null)} className="mt-3 px-3 py-1.5 text-xs font-medium rounded-lg bg-[var(--bg-hover)] text-[var(--text-secondary)]">关闭</button>
          </div>
        </div>
      )}

      {/* Service table */}
      <Card>
        {loading ? (
          <div className="py-12 text-center text-sm text-[var(--text-muted)]">{t('common.loading', '加载中...')}</div>
        ) : services.length === 0 ? (
          <div className="py-12 text-center text-sm text-[var(--text-muted)]">{t('services.empty', '暂无服务')}</div>
        ) : (
          <div className="overflow-x-auto -mx-5">
            <table className="w-full text-sm">
              <thead>
                <tr>
                  <th className="w-10 px-4 py-3 border-b border-[var(--bg-border)]">
                    <input
                      type="checkbox"
                      checked={services.length > 0 && services.filter(s => !s.protected).every(s => selected.has(s.name))}
                      onChange={toggleAll}
                      className="rounded border-[var(--bg-border)] bg-[var(--bg-input)]"
                    />
                  </th>
                  <th className="text-left px-4 py-3 text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)] border-b border-[var(--bg-border)]">
                    {t('services.name', '服务')}
                  </th>
                  <th className="text-left px-4 py-3 text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)] border-b border-[var(--bg-border)]">
                    {t('services.status', '状态')}
                  </th>
                  <th className="text-left px-4 py-3 text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)] border-b border-[var(--bg-border)]">
                    端口
                  </th>
                  <th className="text-left px-4 py-3 text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)] border-b border-[var(--bg-border)]">
                    {t('services.id', '标识')}
                  </th>
                  <th className="text-left px-4 py-3 text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)] border-b border-[var(--bg-border)]">
                    {t('services.autoStart', '自动启动')}
                  </th>
                  <th className="text-right px-4 py-3 text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)] border-b border-[var(--bg-border)]">
                    {t('services.actions', '操作')}
                  </th>
                </tr>
              </thead>
              <tbody>
                {groups.map(group => {
                  if (group.services.length === 0) return null
                  return [
                    <tr key={'h-' + group.type}>
                      <td colSpan={7} className="px-4 py-2 bg-[var(--bg-muted)]/30 border-b border-[var(--bg-border)]">
                        <span className="text-xs font-semibold uppercase tracking-wider text-[var(--text-muted)]">
                          {group.type}
                        </span>
                      </td>
                    </tr>,
                    ...group.services.map(svc => renderRow(svc, group.isPerNIC))
                  ]
                })}
              </tbody>
            </table>
          </div>
        )}
      </Card>
    </div>
  )
}
