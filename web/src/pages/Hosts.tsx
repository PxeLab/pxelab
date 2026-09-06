import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { Plus, Search } from 'lucide-react'
import { Card } from '../components/ui/Card'
import { DataTable, type Column } from '../components/ui/DataTable'
import { Pagination } from '../components/ui/Pagination'
import { Button } from '../components/ui/Button'
import { Modal } from '../components/ui/Modal'
import { ConfirmDialog } from '../components/ui/ConfirmDialog'
import { PageHeader } from '../components/ui/PageHeader'
import { useToast } from '../components/ui/Toast'
import { Input } from '../components/ui/FormControls'
import { ChecklistPicker } from '../components/ui/ChecklistPicker'
import { Toggle } from '../components/ui/Toggle'
import { api, type Host, type Profile, type Baseline, type scriptDTO, type PxeBootRecord } from '../api/client'
import { getPxeBootRecords, claimPxeBootRecord, deletePxeBootRecord, clearPxeBootRecords } from '../api/pxeboot'
import { useUIConfig } from '../contexts/UIConfigContext'

export default function Hosts() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { success, error } = useToast()
  const [hosts, setHosts] = useState<Host[]>([])
  const [loading, setLoading] = useState(true)
  const [page, setPage] = useState(1)
  const [total, setTotal] = useState(0)
  const [search, setSearch] = useState('')
  const [showModal, setShowModal] = useState(false)
  const [view, setView] = useState<'hosts' | 'records'>('hosts')
  const [records, setRecords] = useState<PxeBootRecord[]>([])
  const [pendingClaimMac, setPendingClaimMac] = useState<string | null>(null)
  const [createShowBaselines, setCreateShowBaselines] = useState(false)
  const [createShowScripts, setCreateShowScripts] = useState(false)
  const [clearMode, setClearMode] = useState<'' | 'unknown' | 'all'>('')
  const [newHost, setNewHost] = useState<{
    name: string; mac: string; ip: string; sn: string; profile_id: string
    baseline_ids: string[]; script_ids: number[]
  }>({ name: '', mac: '', ip: '', sn: '', profile_id: '', baseline_ids: [], script_ids: [] })
  const [profiles, setProfiles] = useState<Profile[]>([])
  const [allBaselines, setAllBaselines] = useState<Baseline[]>([])
  const [allScripts, setAllScripts] = useState<scriptDTO[]>([])
  const [confirmDelete, setConfirmDelete] = useState<string | null>(null)
  const [sortField, setSortField] = useState('')
  const [sortDir, setSortDir] = useState<'asc' | 'desc'>('asc')

  const { pageSize } = useUIConfig()

  function loadRecords() {
    getPxeBootRecords().then(res => setRecords(res.data?.records ?? [])).catch(() => {})
  }

  function openCreate(prefillMac?: string) {
    setNewHost({ name: '', mac: prefillMac ?? '', ip: '', sn: '', profile_id: '', baseline_ids: [], script_ids: [] })
    setCreateShowBaselines(false)
    setCreateShowScripts(false)
    setPendingClaimMac(prefillMac ?? null)
    setCreateError('')
    setShowModal(true)
  }

  function toggleArr<T>(arr: T[], v: T): T[] {
    return arr.includes(v) ? arr.filter(x => x !== v) : [...arr, v]
  }

  useEffect(() => {
    loadHosts()
  }, [page, search])

  useEffect(() => {
    api.getProfiles().then(res => setProfiles(res.data)).catch(() => {})
    api.getBaselines().then(res => setAllBaselines(res.data ?? [])).catch(() => {})
    api.getScripts().then(res => setAllScripts(res.data ?? [])).catch(() => {})
  }, [])

  async function loadHosts() {
    setLoading(true)
    try {
      const res = await api.getHosts({ page: String(page), size: String(pageSize), search })
      setHosts(res.data.hosts ?? [])
      setTotal(res.data.meta.total)
    } catch (err: any) {
      error(err.message || t('hosts.loadError'))
    } finally {
      setLoading(false)
    }
  }

  const [createError, setCreateError] = useState('')

  const MAC_RE = /^([0-9a-fA-F]{2}[:-]){5}[0-9a-fA-F]{2}$/

  async function handleCreate() {
    const mac = newHost.mac.trim()
    if (!newHost.name.trim()) {
      setCreateError(t('hosts.nameRequired'))
      return
    }
    if (!MAC_RE.test(mac)) {
      setCreateError(t('hosts.invalidMac'))
      return
    }
    setCreateError('')
    try {
      const res = await api.createHost({
        name: newHost.name,
        mac,
        ip: newHost.ip || undefined,
        sn: newHost.sn || undefined,
        profile_id: newHost.profile_id || undefined,
        baseline_ids: newHost.baseline_ids,
        script_ids: newHost.script_ids,
      })
      if (pendingClaimMac) {
        try {
          await claimPxeBootRecord(pendingClaimMac, res.data.id)
          loadRecords()
        } catch { /* 认领失败不阻塞保存 */ }
      }
      success(t('hosts.created'))
      setShowModal(false)
      setNewHost({ name: '', mac: '', ip: '', sn: '', profile_id: '', baseline_ids: [], script_ids: [] })
      setPendingClaimMac(null)
      loadHosts()
    } catch (err: any) {
      setCreateError(err.message)
    }
  }

  async function ignoreRecord(mac: string) {
    try { await deletePxeBootRecord(mac); loadRecords() } catch (e: any) { error(e.message) }
  }

  async function doClearRecords() {
    try {
      if (clearMode === 'unknown') {
        for (const rec of records.filter(r => !r.claimed_host_id)) {
          await deletePxeBootRecord(rec.mac).catch(() => {})
        }
      } else {
        await clearPxeBootRecords()
      }
      loadRecords()
      success(t('hosts.recordsCleared'))
    } catch (e: any) { error(e.message) }
    finally { setClearMode('') }
  }

  async function handleDelete(id: string) {
    setConfirmDelete(id)
  }

  async function doDelete() {
    if (!confirmDelete) return
    try {
      await api.deleteHost(confirmDelete)
      success(t('hosts.deleted'))
      // 删除的是当前页最后一条时回退一页，避免停留在空页（setPage 触发 loadHosts）
      if (hosts.length === 1 && page > 1) {
        setPage(page - 1)
      } else {
        loadHosts()
      }
    } catch (err: any) {
      error(err.message)
    } finally {
      setConfirmDelete(null)
    }
  }

  const handleSort = (field: string) => {
    if (sortField === field) setSortDir(d => d === 'asc' ? 'desc' : 'asc')
    else { setSortField(field); setSortDir('asc') }
  }

  // 初始化脚本（P1：继承自 Profile 的基线 + 主机追加的基线/脚本）
  const selectedProfile = profiles.find(p => p.id === newHost.profile_id)
  const inheritedBaselineIds: string[] = selectedProfile?.baselines ?? []

  const recColumns: Column<PxeBootRecord>[] = [
    { key: 'mac', label: 'MAC', render: (r) => <span className="font-mono text-xs text-[var(--text-primary)]">{r.mac}</span> },
    { key: 'last_seen', label: t('hosts.recordsLastSeen'), render: (r) => <span className="font-mono text-xs text-[var(--text-muted)]">{new Date(r.last_seen).toLocaleString()}</span> },
    { key: 'count', label: t('hosts.recordsCount'), render: (r) => <span className="font-mono text-xs">{r.count}</span> },
    { key: 'loader', label: t('hosts.recordsLoader'), render: (r) => <span className="text-xs">{r.loader || '—'}</span> },
    { key: 'last_context', label: t('hosts.recordsContext'), render: (r) => <span className="text-xs text-[var(--text-muted)]">{r.last_context || '—'}</span> },
    { key: 'claimed', label: t('hosts.recordsClaimed'), render: (r) => (
      r.claimed_host_id
        ? <span className="text-xs text-accent-green">{r.claimed_host || r.claimed_host_id}</span>
        : <span className="text-xs text-accent-yellow">{t('hosts.recordsNotClaimed')}</span>
    ) },
    { key: 'actions', label: '', className: 'text-right', render: (r) => (
      <div className="flex items-center justify-end gap-2">
        {!r.claimed_host_id && (
          <Button variant="primary" size="sm" onClick={() => openCreate(r.mac)}>{t('hosts.recordsClaim')}</Button>
        )}
        <Button variant="ghost" size="sm" onClick={() => ignoreRecord(r.mac)}>{t('hosts.recordsIgnore')}</Button>
      </div>
    ) },
  ]

  const sortedHosts = sortField
    ? [...hosts].sort((a, b) => {
        const cmp = String(a[sortField as keyof Host] ?? '').localeCompare(String(b[sortField as keyof Host] ?? ''))
        return sortDir === 'asc' ? cmp : -cmp
      })
    : hosts

  const columns: Column<Host>[] = [
    { key: 'mac', label: t('hosts.columns.mac'), render: (h) => <span className="font-mono text-xs text-[var(--text-primary)]">{h.mac}</span>, sortable: true },
    { key: 'name', label: t('hosts.columns.hostname'), render: (h) => <span className="font-medium text-[var(--text-primary)]">{h.name || '—'}</span> },
    { key: 'ip', label: t('hosts.columns.ip'), render: (h) => <span className="font-mono text-xs">{h.ip}</span> },
    { key: 'boot_count', label: t('hosts.columns.bootCount'), render: (h) => <span className="font-mono text-xs">{h.boot_count}</span> },
    { key: 'last_online', label: t('hosts.columns.lastOnline'), render: (h) => (
      <span className="font-mono text-xs text-[var(--text-muted)]">{h.last_online ? new Date(h.last_online).toLocaleString() : '—'}</span>
    )},
    { key: 'actions', label: '', render: (h) => (
      <div onClick={e => e.stopPropagation()}><Button variant="ghost" size="sm" onClick={() => handleDelete(h.id)}>{t('common.delete')}</Button></div>
    ), width: '60px' },
  ]

  return (
    <div>
      <PageHeader
        title={t('hosts.title')}
        actions={
          <Button variant="primary" size="sm" onClick={() => openCreate()}>
            <Plus size={14} /> {t('hosts.addHost')}
          </Button>
        }
      />
      <div className="flex items-center gap-5 border-b border-[var(--bg-border)] mb-6">
        {(
          [
            { key: 'hosts', label: t('hosts.title') },
            { key: 'records', label: t('hosts.recordsTitle') },
          ] as { key: 'hosts' | 'records'; label: string }[]
        ).map(tab => {
          const active = view === tab.key
          return (
            <button
              key={tab.key}
              onClick={() => { setView(tab.key); if (tab.key === 'records') loadRecords() }}
              className={`-mb-px border-b-2 px-1 py-2 text-sm transition-colors ${
                active
                  ? 'border-blue-500 font-medium text-[var(--text-primary)]'
                  : 'border-transparent text-[var(--text-muted)] hover:text-[var(--text-primary)]'
              }`}
            >
              {tab.label}
              {tab.key === 'records' && records.filter(r => !r.claimed_host_id).length > 0 && (
                <span className="ml-1.5 px-1.5 py-px rounded-full bg-accent-red/15 text-accent-red text-[10px]">
                  {records.filter(r => !r.claimed_host_id).length}
                </span>
              )}
            </button>
          )
        })}
      </div>

      {view === 'hosts' && (
      <>
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center gap-3">
          <div className="relative">
            <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-[var(--text-muted)]" />
            <input
              className="w-[280px] bg-[var(--bg-input)] border border-[var(--bg-border)] rounded-lg py-2 pl-9 pr-3 text-sm text-[var(--text-primary)] placeholder-[var(--text-muted)] outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-500/10 transition-all"
              placeholder={t('hosts.search', '搜索 MAC / 主机名 / IP ...')}
              value={search}
              onChange={(e) => { setSearch(e.target.value); setPage(1) }}
            />
          </div>
          <span className="text-sm text-[var(--text-muted)]">{t('common.total', '共')} {total} {t('common.items', '条')}</span>
        </div>
      </div>

      <Card padding={false}>
        <DataTable
          columns={columns}
          data={sortedHosts}
          loading={loading}
          onRowClick={(h) => navigate('/hosts/' + h.id)}
          sortField={sortField}
          sortDir={sortDir}
          onSort={handleSort}
          emptyText={t('hosts.empty', '暂无主机')}
        />
        <div className="px-5 py-3">
          <Pagination page={page} total={total} size={pageSize} onChange={setPage} />
        </div>
      </Card>
      </>
      )}

      {/* PXE 引导记录面板 */}
      {view === 'records' && (
        <div className="space-y-5">
          <div className="flex items-center justify-between">
            <p className="text-sm text-[var(--text-muted)]">{t('hosts.recordsHint')}</p>
            <div className="flex gap-2">
              <Button variant="secondary" size="sm" onClick={() => setClearMode('unknown')} disabled={records.filter(r => !r.claimed_host_id).length === 0}>
                {t('hosts.recordsClearUnknown')}
              </Button>
              <Button variant="danger" size="sm" onClick={() => setClearMode('all')} disabled={records.length === 0}>
                {t('hosts.recordsClearAll')}
              </Button>
            </div>
          </div>

          {records.length === 0 ? (
            <Card>
              <div className="py-12 text-center text-sm text-[var(--text-muted)]">{t('hosts.recordsEmpty')}</div>
            </Card>
          ) : (
            <Card padding={false}>
              <DataTable
                columns={recColumns}
                data={records}
                rowKey={r => r.mac}
              />
            </Card>
          )}
        </div>
      )}

      {/* 清空引导记录确认 */}
      <ConfirmDialog
        open={clearMode !== ''}
        onClose={() => setClearMode('')}
        onConfirm={doClearRecords}
        title={t('hosts.recordsClearTitle')}
        message={clearMode === 'unknown' ? t('hosts.recordsClearUnknownConfirm') : t('hosts.recordsClearAllConfirm')}
      />

      <Modal
        open={showModal}
        onClose={() => { setShowModal(false); setNewHost({ name: '', mac: '', ip: '', sn: '', profile_id: '', baseline_ids: [], script_ids: [] }) }}
        title={t('hosts.addHost')}
        footer={
          <>
            <Button variant="secondary" onClick={() => setShowModal(false)}>{t('common.cancel')}</Button>
            <Button variant="primary" onClick={handleCreate}>{t('common.create')}</Button>
          </>
        }
      >
        <div className="space-y-4">
          {createError && (
            <div className="px-3 py-2 rounded-lg bg-accent-red/10 border border-accent-red/30 text-accent-red text-xs">{createError}</div>
          )}
          <div>
            <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1.5">{t('hosts.columns.mac')}</label>
            <Input placeholder="00:11:22:33:44:55" value={newHost.mac} onChange={e => setNewHost({...newHost, mac: e.target.value})} />
          </div>
          <div>
            <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1.5">{t('hosts.columns.hostname')}</label>
            <Input placeholder="node-01" value={newHost.name} onChange={e => setNewHost({...newHost, name: e.target.value})} />
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1.5">{t('hosts.columns.ip')}</label>
              <Input placeholder="192.168.1.100" value={newHost.ip} onChange={e => setNewHost({...newHost, ip: e.target.value})} />
            </div>
            <div>
              <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1.5">{t('profiles.title')}</label>
              <select
                className="w-full bg-[var(--bg-input)] border border-[var(--bg-border)] rounded-lg px-3.5 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-500/10"
                value={newHost.profile_id}
                onChange={e => setNewHost({...newHost, profile_id: e.target.value})}
              >
                <option value="">—</option>
                {profiles.map(p => (
                  <option key={p.id} value={p.id}>{p.name}</option>
                ))}
              </select>
            </div>
          </div>
          <div>
            <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1.5">SN</label>
            <Input placeholder={t('hosts.snPlaceholder', 'Serial Number（可选，配合 global.identity_attr=sn 使用）')} value={newHost.sn} onChange={e => setNewHost({ ...newHost, sn: e.target.value })} />
          </div>

          {/* 初始化脚本：勾选即装机后自动执行 */}
          <div className="rounded-lg border border-[var(--bg-border)] p-3 space-y-3">
            <div>
              <p className="text-xs font-semibold text-[var(--text-primary)]">{t('hosts.initSection')}</p>
              <p className="text-[11px] text-[var(--text-muted)] mt-0.5">{t('hosts.initHint')}</p>
            </div>
            {inheritedBaselineIds.length > 0 && (
              <div className="flex flex-wrap items-center gap-1.5">
                <span className="text-[10px] uppercase tracking-wide text-[var(--text-muted)]">{t('hosts.inheritedBaselines')}</span>
                {inheritedBaselineIds.map(id => {
                  const b = allBaselines.find(x => x.id === id)
                  return (
                    <span key={id} className="px-1.5 py-0.5 rounded bg-blue-500/10 text-blue-400 text-[10px] font-medium">{b ? b.name : id}</span>
                  )
                })}
              </div>
            )}
            <div>
              <Toggle
                checked={createShowBaselines}
                onChange={v => {
                  setCreateShowBaselines(v)
                  if (!v) setNewHost({ ...newHost, baseline_ids: [] })
                }}
                label={t('hosts.addBaselines')}
              />
              {createShowBaselines && (
                <div className="mt-1.5">
                  <ChecklistPicker
                    items={allBaselines.map(b => ({ id: b.id, name: b.name }))}
                    selected={newHost.baseline_ids}
                    disabledIds={inheritedBaselineIds}
                    onToggle={id => setNewHost({ ...newHost, baseline_ids: toggleArr(newHost.baseline_ids, id) })}
                    emptyText={t('baselines.noBaselines')}
                  />
                </div>
              )}
            </div>
            <div>
              <Toggle
                checked={createShowScripts}
                onChange={v => {
                  setCreateShowScripts(v)
                  if (!v) setNewHost({ ...newHost, script_ids: [] })
                }}
                label={t('hosts.addScripts')}
              />
              {createShowScripts && (
                <div className="mt-1.5">
                  <ChecklistPicker
                    items={allScripts.map(sc => ({ id: sc.id, name: sc.name, badge: sc.type }))}
                    selected={newHost.script_ids}
                    onToggle={id => setNewHost({ ...newHost, script_ids: toggleArr(newHost.script_ids, id) })}
                    emptyText={t('scripts.noScripts')}
                  />
                </div>
              )}
            </div>
          </div>
        </div>
      </Modal>
      <ConfirmDialog
        open={!!confirmDelete}
        onClose={() => setConfirmDelete(null)}
        onConfirm={doDelete}
        title={t('hosts.deleteTitle')}
        message={t('hosts.deleteConfirm')}
      />
    </div>
  )
}

