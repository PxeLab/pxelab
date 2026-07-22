import { useState, useEffect, useCallback } from 'react'
import { RefreshCw, Trash2, AlertTriangle } from 'lucide-react'
import { Card } from '../../components/ui/Card'
import { Button } from '../../components/ui/Button'
import { Modal } from '../../components/ui/Modal'
import { Tag, type TagColor } from '../../components/ui/Tag'
import { DataTable, type Column } from '../../components/ui/DataTable'
import { useToast } from '../../components/ui/Toast'
import { api, type Lease, type LeaseStats } from '../../api/client'
import { useTranslation } from 'react-i18next'

export function LeasesTab() {
  const { t } = useTranslation()
  const { success, error } = useToast()

  const [leases, setLeases] = useState<Lease[]>([])
  const [stats, setStats] = useState<LeaseStats[]>([])
  const [loading, setLoading] = useState(true)
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [deleteTarget, setDeleteTarget] = useState<Lease | null>(null)
  const [batchDeleteOpen, setBatchDeleteOpen] = useState(false)
  const [pruneOpen, setPruneOpen] = useState(false)
  const [pruning, setPruning] = useState(false)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const [leaseRes, statsRes] = await Promise.all([
        api.getLeases(),
        api.getLeaseStats(),
      ])
      setLeases(leaseRes.data || [])
      setStats(statsRes.data || [])
    } catch (err: any) {
      error(err.message || t('common.error'))
    } finally {
      setLoading(false)
    }
  }, [error, t])

  useEffect(() => { load() }, [load])

  function toggleSelect(mac: string) {
    setSelected(prev => {
      const next = new Set(prev)
      if (next.has(mac)) next.delete(mac)
      else next.add(mac)
      return next
    })
  }

  function toggleSelectAll() {
    if (selected.size === leases.length) {
      setSelected(new Set())
    } else {
      setSelected(new Set(leases.map(l => l.mac)))
    }
  }

  async function handleDelete(mac: string) {
    setDeleteTarget(null)
    try {
      await api.deleteLease(mac)
      success(t('leases.deleteSuccess'))
      load()
    } catch (err: any) {
      error(err.message)
    }
  }

  async function handleBatchDelete() {
    setBatchDeleteOpen(false)
    const macs = Array.from(selected)
    if (macs.length === 0) return
    try {
      const res = await api.batchDeleteLeases(macs)
      if (res.data?.failed_count && res.data.failed_count > 0) {
        error(t('leases.batchDeleteFail', { success: res.data.success_count, failed: res.data.failed_count }))
      } else {
        success(t('leases.batchDeleteSuccess', { count: res.data?.success_count ?? macs.length }))
      }
      setSelected(new Set())
      load()
    } catch (err: any) {
      error(err.message)
    }
  }

  async function handlePrune() {
    setPruneOpen(false)
    setPruning(true)
    try {
      await api.pruneLeases()
      success(t('leases.pruneSuccess'))
      load()
    } catch (err: any) {
      error(err.message)
    } finally {
      setPruning(false)
    }
  }

  const totalLeases = leases.length
  const activeLeases = leases.filter(l => new Date(l.expires_at) > new Date()).length
  const expiredLeases = totalLeases - activeLeases
  const hasExpired = expiredLeases > 0

  const statusColor = (expiresAt: string): TagColor => {
    return new Date(expiresAt) > new Date() ? 'green' : 'red'
  }

  const statusLabel = (expiresAt: string): string => {
    return new Date(expiresAt) > new Date() ? t('leases.statusActive') : t('leases.statusExpired')
  }

  const formatTime = (s: string) => {
    const d = new Date(s)
    return d.toLocaleString()
  }

  const columns: Column<Lease>[] = [
    {
      key: '_select', label: '', width: '40px',
      render: (item) => (
        <input
          type="checkbox"
          checked={selected.has(item.mac)}
          onChange={() => toggleSelect(item.mac)}
          className="w-4 h-4 rounded border-[var(--bg-border)] bg-[var(--bg-input)] accent-blue-500 cursor-pointer"
        />
      ),
    },
    { key: 'mac', label: t('leases.mac'), render: (r) => <span className="font-mono text-xs text-[var(--text-primary)]">{r.mac}</span> },
    { key: 'ip', label: t('leases.ip'), render: (r) => <span className="font-mono text-xs">{r.ip}</span> },
    { key: 'subnet_id', label: t('leases.subnet'), render: (r) => <span className="text-xs">{r.subnet_id}</span> },
    { key: 'hostname', label: t('leases.hostname'), render: (r) => r.hostname ? <span className="text-xs">{r.hostname}</span> : <span className="text-xs text-[var(--text-muted)]">—</span> },
    { key: 'status', label: t('leases.status'), render: (r) => <Tag color={statusColor(r.expires_at)}>{statusLabel(r.expires_at)}</Tag> },
    { key: 'expires_at', label: t('leases.expiresAt'), render: (r) => <span className="text-xs">{formatTime(r.expires_at)}</span> },
    { key: 'created_at', label: t('leases.createdAt'), render: (r) => <span className="text-xs text-[var(--text-muted)]">{formatTime(r.created_at)}</span> },
    {
      key: '_actions', label: '', width: '60px',
      render: (r) => (
        <Button variant="ghost" size="sm" onClick={() => setDeleteTarget(r)}>
          <Trash2 size={13} />
        </Button>
      ),
    },
  ]

  return (
    <div>
      {/* Summary Cards */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4 mb-6">
        <Card>
          <div className="text-center">
            <p className="text-xs text-[var(--text-muted)] mb-1">{t('leases.totalSubnets')}</p>
            <p className="text-2xl font-bold text-[var(--text-primary)]">{stats.length}</p>
          </div>
        </Card>
        <Card>
          <div className="text-center">
            <p className="text-xs text-[var(--text-muted)] mb-1">{t('leases.totalLeases')}</p>
            <p className="text-2xl font-bold text-[var(--text-primary)]">{totalLeases}</p>
          </div>
        </Card>
        <Card>
          <div className="text-center">
            <p className="text-xs text-[var(--text-muted)] mb-1">{t('leases.statusActive')}</p>
            <p className="text-2xl font-bold text-accent-green">{activeLeases}</p>
          </div>
        </Card>
        <Card>
          <div className="text-center">
            <p className="text-xs text-[var(--text-muted)] mb-1">{t('leases.statusExpired')}</p>
            <p className="text-2xl font-bold text-accent-red">{expiredLeases}</p>
          </div>
        </Card>
      </div>

      {/* Subnet Usage */}
      {stats.length > 0 && (
        <div className="mb-6">
          <h2 className="text-sm font-bold text-[var(--text-primary)] mb-3">{t('leases.subnetOverview')}</h2>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
            {stats.map((s) => (
              <Card key={s.subnet_id} padding={false}>
                <div className="p-4">
                  <div className="flex items-center justify-between mb-2">
                    <div>
                      <span className="text-sm font-semibold text-[var(--text-primary)]">{s.subnet_id}</span>
                      <Tag color={s.dhcp_mode === 'server' ? 'blue' : 'purple'} className="ml-2">
                        {s.dhcp_mode === 'server' ? t('leases.dhcpModeFull') : s.dhcp_mode === 'proxy' ? t('leases.dhcpModeProxy') : t('leases.dhcpModeOff')}
                      </Tag>
                    </div>
                  </div>
                  {s.pool_size > 0 ? (
                    <>
                      <div className="flex items-center gap-4 text-xs text-[var(--text-muted)] mb-2">
                        <span>{t('leases.poolSize')}: <strong className="text-[var(--text-primary)]">{s.pool_size}</strong></span>
                        <span>{t('leases.allocated')}: <strong className="text-[var(--text-primary)]">{s.allocated}</strong></span>
                        <span>{t('leases.available')}: <strong className="text-accent-green">{s.available}</strong></span>
                        <span>{t('leases.usage')}: <strong className={s.usage_pct > 80 ? 'text-accent-red' : s.usage_pct > 50 ? 'text-accent-yellow' : 'text-blue-500'}>{s.usage_pct.toFixed(1)}%</strong></span>
                      </div>
                      <div className="w-full h-2 bg-[var(--bg-card)] rounded-full overflow-hidden border border-[var(--bg-border)]">
                        <div
                          className={`h-full rounded-full transition-all duration-300 ${
                            s.usage_pct > 80 ? 'bg-accent-red' : s.usage_pct > 50 ? 'bg-accent-yellow' : 'bg-blue-500'
                          }`}
                          style={{ width: `${Math.min(s.usage_pct, 100)}%` }}
                        />
                      </div>
                    </>
                  ) : (
                    <div className="text-xs text-[var(--text-muted)]">
                      {t('leases.activeOnly')}: <strong className="text-[var(--text-primary)]">{s.active_only}</strong>
                    </div>
                  )}
                </div>
              </Card>
            ))}
          </div>
        </div>
      )}

      {/* Toolbar */}
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-sm font-bold text-[var(--text-primary)]">{t('leases.leaseRecords')}</h2>
        <div className="flex items-center gap-2">
          <Button
            variant="ghost"
            size="sm"
            onClick={() => setPruneOpen(true)}
            disabled={!hasExpired || pruning}
          >
            <AlertTriangle size={13} className="mr-1" />
            {t('leases.pruneExpired')}
          </Button>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => setBatchDeleteOpen(true)}
            disabled={selected.size === 0}
          >
            <Trash2 size={13} className="mr-1" />
            {t('leases.deleteSelected')} ({selected.size})
          </Button>
          <Button variant="ghost" size="sm" onClick={load}>
            <RefreshCw size={13} className="mr-1" />
            {t('common.refresh')}
          </Button>
        </div>
      </div>

      {/* Lease Table */}
      {leases.length > 0 && (
        <div className="mb-4">
          <label className="flex items-center gap-2 text-xs text-[var(--text-muted)] cursor-pointer">
            <input
              type="checkbox"
              checked={selected.size === leases.length && leases.length > 0}
              onChange={toggleSelectAll}
              className="w-4 h-4 rounded border-[var(--bg-border)] bg-[var(--bg-input)] accent-blue-500"
            />
            {t('common.selectAll')}
          </label>
        </div>
      )}
      <Card padding={false}>
        <DataTable
          columns={columns}
          data={leases}
          loading={loading}
          emptyText={t('leases.empty')}
          rowKey={(r) => r.mac}
        />
      </Card>

      {/* Delete Single Modal */}
      <Modal
        open={!!deleteTarget}
        onClose={() => setDeleteTarget(null)}
        title={t('common.delete')}
        width="400px"
        footer={
          <>
            <Button variant="secondary" onClick={() => setDeleteTarget(null)}>{t('common.cancel')}</Button>
            <Button variant="primary" onClick={() => deleteTarget && handleDelete(deleteTarget.mac)}>{t('common.delete')}</Button>
          </>
        }
      >
        <p className="text-sm text-[var(--text-secondary)]">
          {t('leases.deleteConfirm')}
        </p>
        {deleteTarget && (
          <div className="mt-3 p-3 bg-[var(--bg-card)] rounded-lg border border-[var(--bg-border)]">
            <p className="text-xs font-mono text-[var(--text-primary)]">{deleteTarget.mac}</p>
            <p className="text-xs text-[var(--text-muted)]">{deleteTarget.ip} · {deleteTarget.subnet_id}</p>
          </div>
        )}
      </Modal>

      {/* Batch Delete Modal */}
      <Modal
        open={batchDeleteOpen}
        onClose={() => setBatchDeleteOpen(false)}
        title={t('common.delete')}
        width="400px"
        footer={
          <>
            <Button variant="secondary" onClick={() => setBatchDeleteOpen(false)}>{t('common.cancel')}</Button>
            <Button variant="primary" onClick={handleBatchDelete}>{t('common.delete')}</Button>
          </>
        }
      >
        <p className="text-sm text-[var(--text-secondary)]">
          {t('leases.deleteConfirmMulti', { count: selected.size })}
        </p>
      </Modal>

      {/* Prune Modal */}
      <Modal
        open={pruneOpen}
        onClose={() => setPruneOpen(false)}
        title={t('leases.pruneExpired')}
        width="400px"
        footer={
          <>
            <Button variant="secondary" onClick={() => setPruneOpen(false)}>{t('common.cancel')}</Button>
            <Button variant="primary" onClick={handlePrune}>{t('leases.pruneExpired')}</Button>
          </>
        }
      >
        <p className="text-sm text-[var(--text-secondary)]">
          {t('leases.pruneConfirm')}
        </p>
        <p className="text-xs text-[var(--text-muted)] mt-2">
          {t('leases.expired')}: {expiredLeases} {t('common.items')}
        </p>
      </Modal>
    </div>
  )
}
