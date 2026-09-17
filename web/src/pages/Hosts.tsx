import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { Plus, Search, Rocket } from 'lucide-react'
import { Card } from '../components/ui/Card'
import { DataTable, type Column } from '../components/ui/DataTable'
import { Pagination } from '../components/ui/Pagination'
import { Button } from '../components/ui/Button'
import { Modal } from '../components/ui/Modal'
import { ConfirmDialog } from '../components/ui/ConfirmDialog'
import { PageHeader } from '../components/ui/PageHeader'
import { useToast } from '../components/ui/Toast'
import { Input, Select } from '../components/ui/FormControls'
import { ChecklistPicker } from '../components/ui/ChecklistPicker'
import { Toggle } from '../components/ui/Toggle'
import { api, type Host, type Profile, type Baseline, type scriptDTO, type DriverPackage, type PxeBootRecord, type AnswerTemplate, type BatchSkipped } from '../api/client'
import { getPxeBootRecords, claimPxeBootRecord, deletePxeBootRecord, clearPxeBootRecords } from '../api/pxeboot'
import { buildReadySystems, defaultHostname, type MatchedSystem } from '../utils/readySystems'
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
  const [createShowDrivers, setCreateShowDrivers] = useState(false)
  const [clearMode, setClearMode] = useState<'' | 'unknown' | 'all'>('')
  const [newHost, setNewHost] = useState<{
    name: string; mac: string; ip: string; sn: string; profile_id: string
    baseline_ids: string[]; script_ids: number[]; driver_packs: string[]
  }>({ name: '', mac: '', ip: '', sn: '', profile_id: '', baseline_ids: [], script_ids: [], driver_packs: [] })
  const [profiles, setProfiles] = useState<Profile[]>([])
  const [allBaselines, setAllBaselines] = useState<Baseline[]>([])
  const [allScripts, setAllScripts] = useState<scriptDTO[]>([])
  const [allDriverPacks, setAllDriverPacks] = useState<DriverPackage[]>([])
  const [confirmDelete, setConfirmDelete] = useState<string | null>(null)
  const [sortField, setSortField] = useState('')
  const [sortDir, setSortDir] = useState<'asc' | 'desc'>('asc')

  // R7 批量装机：多选 + 批量弹窗
  const [selHosts, setSelHosts] = useState<Set<string>>(new Set())
  const [selRecords, setSelRecords] = useState<Set<string>>(new Set())
  const [showBatch, setShowBatch] = useState(false)
  const [batchSource, setBatchSource] = useState<'hosts' | 'records'>('hosts')
  const [batchSystems, setBatchSystems] = useState<MatchedSystem[]>([])
  const [batchTemplates, setBatchTemplates] = useState<AnswerTemplate[]>([])
  const [batchSysKey, setBatchSysKey] = useState('')
  const [batchTplOverride, setBatchTplOverride] = useState<number | null>(null)
  const [batchBusy, setBatchBusy] = useState(false)
  const [batchError, setBatchError] = useState('')
  const [batchSkipped, setBatchSkipped] = useState<BatchSkipped[]>([])

  const { pageSize } = useUIConfig()

  function loadRecords() {
    getPxeBootRecords().then(res => setRecords(res.data?.records ?? [])).catch(() => {})
  }

  function openCreate(prefillMac?: string) {
    setNewHost({ name: '', mac: prefillMac ?? '', ip: '', sn: '', profile_id: '', baseline_ids: [], script_ids: [], driver_packs: [] })
    setCreateShowBaselines(false)
    setCreateShowScripts(false)
    setCreateShowDrivers(false)
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
    api.getDriverPackages().then(res => setAllDriverPacks(res.data ?? [])).catch(() => {})
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
        driver_packs: newHost.driver_packs,
      })
      if (pendingClaimMac) {
        try {
          await claimPxeBootRecord(pendingClaimMac, res.data.id)
          loadRecords()
        } catch { /* 认领失败不阻塞保存 */ }
      }
      success(t('hosts.created'))
      setShowModal(false)
      setNewHost({ name: '', mac: '', ip: '', sn: '', profile_id: '', baseline_ids: [], script_ids: [], driver_packs: [] })
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

  // ── R7 批量装机 ──

  function toggleSet(prev: Set<string>, v: string): Set<string> {
    const next = new Set(prev)
    if (next.has(v)) next.delete(v)
    else next.add(v)
    return next
  }

  const unclaimedRecords = records.filter(r => !r.claimed_host_id)

  const batchMachines: { mac: string; name?: string; ip?: string }[] =
    batchSource === 'hosts'
      ? hosts.filter(h => selHosts.has(h.id)).map(h => ({ mac: h.mac, name: h.name, ip: h.ip || undefined }))
      : unclaimedRecords.filter(r => selRecords.has(r.mac)).map(r => ({ mac: r.mac, ip: r.ip || undefined }))

  async function openBatch(source: 'hosts' | 'records') {
    setBatchSource(source)
    setBatchSysKey('')
    setBatchTplOverride(null)
    setBatchError('')
    setBatchSkipped([])
    setShowBatch(true)
    const [profRes, catRes, fsRes, tmplRes] = await Promise.all([
      api.getProfiles().catch(() => null),
      api.getNetbootCatalog().catch(() => null),
      api.getNetbootFileStatus().catch(() => null),
      api.getAnswerTemplates().catch(() => null),
    ])
    const tmpls = tmplRes?.data?.templates || []
    setBatchTemplates(tmpls)
    setBatchSystems(buildReadySystems(profRes?.data || [], catRes?.data?.distros || [], fsRes?.data || [], tmpls))
  }

  const batchSys = batchSystems.find(s => s.key === batchSysKey)
  const batchTemplate = batchSys
    ? (batchTplOverride != null ? batchTemplates.find(tp => tp.id === batchTplOverride) : batchSys.defaultTemplate)
    : undefined

  async function submitBatch() {
    if (!batchSys || !batchTemplate || batchMachines.length === 0) return
    setBatchBusy(true)
    setBatchError('')
    setBatchSkipped([])
    try {
      const res = await api.createInstallTaskBatch({
        hosts: batchMachines.map(m => ({ mac: m.mac, name: m.name || defaultHostname(m.mac), ip: m.ip })),
        distro_name: batchSys.distroName,
        version_codename: batchSys.versionCodename,
        arch: batchSys.arch,
        answer_template_id: batchTemplate.id ?? null,
        profile_id: batchSys.profile.id,
      })
      const skipped = res.data?.skipped || []
      if (skipped.length > 0) {
        setBatchSkipped(skipped)
        setBatchBusy(false)
        return
      }
      success(t('batchInstall.created', { count: res.data?.tasks?.length ?? batchMachines.length }))
      setShowBatch(false)
      setSelHosts(new Set())
      setSelRecords(new Set())
      navigate('/install-tasks?view=batch')
    } catch (err: any) {
      setBatchError(err.message || t('batchInstall.failed'))
    } finally {
      setBatchBusy(false)
    }
  }

  // 初始化脚本（P1：继承自 Profile 的基线 + 主机追加的基线/脚本）
  const selectedProfile = profiles.find(p => p.id === newHost.profile_id)
  const inheritedBaselineIds: string[] = selectedProfile?.baselines ?? []

  const recColumns: Column<PxeBootRecord>[] = [
    { key: 'sel', width: '32px', label: (
      <input
        type="checkbox"
        className="accent-blue-500 align-middle"
        checked={unclaimedRecords.length > 0 && unclaimedRecords.every(r => selRecords.has(r.mac))}
        onChange={() => {
          const all = unclaimedRecords.every(r => selRecords.has(r.mac))
          setSelRecords(prev => {
            const next = new Set(prev)
            unclaimedRecords.forEach(r => { if (all) next.delete(r.mac); else next.add(r.mac) })
            return next
          })
        }}
      />
    ), render: (r) => r.claimed_host_id ? null : (
      <input
        type="checkbox"
        className="accent-blue-500 align-middle"
        checked={selRecords.has(r.mac)}
        onChange={() => setSelRecords(prev => toggleSet(prev, r.mac))}
      />
    ) },
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
    { key: 'sel', width: '32px', label: (
      <input
        type="checkbox"
        className="accent-blue-500 align-middle"
        checked={hosts.length > 0 && hosts.every(h => selHosts.has(h.id))}
        onChange={() => {
          const all = hosts.every(h => selHosts.has(h.id))
          setSelHosts(prev => {
            const next = new Set(prev)
            hosts.forEach(h => { if (all) next.delete(h.id); else next.add(h.id) })
            return next
          })
        }}
        onClick={e => e.stopPropagation()}
      />
    ), render: (h) => (
      <input
        type="checkbox"
        className="accent-blue-500 align-middle"
        checked={selHosts.has(h.id)}
        onChange={() => setSelHosts(prev => toggleSet(prev, h.id))}
        onClick={e => e.stopPropagation()}
      />
    ) },
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
        {selHosts.size > 0 && (
          <Button variant="primary" size="sm" onClick={() => openBatch('hosts')}>
            <Rocket size={14} /> {t('batchInstall.button')} ({selHosts.size})
          </Button>
        )}
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
              {selRecords.size > 0 && (
                <Button variant="primary" size="sm" onClick={() => openBatch('records')}>
                  <Rocket size={14} /> {t('batchInstall.button')} ({selRecords.size})
                </Button>
              )}
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
        onClose={() => { setShowModal(false); setNewHost({ name: '', mac: '', ip: '', sn: '', profile_id: '', baseline_ids: [], script_ids: [], driver_packs: [] }) }}
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
            {/* R6 驱动包：Windows 首登时拉取并 pnputil 安装 */}
            <div>
              <Toggle
                checked={createShowDrivers}
                onChange={v => {
                  setCreateShowDrivers(v)
                  if (!v) setNewHost({ ...newHost, driver_packs: [] })
                }}
                label={t('hosts.addDriverPacks')}
              />
              <p className="text-[11px] text-[var(--text-muted)] mt-0.5">{t('hosts.driverPacksHint')}</p>
              {createShowDrivers && (
                <div className="mt-1.5">
                  <ChecklistPicker
                    items={allDriverPacks.map(p => ({ id: p.name, name: p.name, badge: `${p.inf_files} INF` }))}
                    selected={newHost.driver_packs}
                    onToggle={id => setNewHost({ ...newHost, driver_packs: toggleArr(newHost.driver_packs, id) })}
                    emptyText={t('hosts.noDriverPacks')}
                  />
                </div>
              )}
            </div>
          </div>
        </div>
      </Modal>

      {/* R7 批量装机弹窗 */}
      <Modal
        open={showBatch}
        onClose={() => setShowBatch(false)}
        title={t('batchInstall.title')}
        footer={
          <>
            <Button variant="secondary" onClick={() => setShowBatch(false)}>{t('common.cancel')}</Button>
            <Button variant="primary" onClick={submitBatch} disabled={batchBusy || !batchSys || !batchTemplate || batchMachines.length === 0}>
              {batchBusy ? t('common.processing') : t('batchInstall.submit')}
            </Button>
          </>
        }
      >
        <div className="space-y-4">
          {batchError && (
            <div className="px-3 py-2 rounded-lg bg-accent-red/10 border border-accent-red/30 text-accent-red text-xs">{batchError}</div>
          )}
          <div>
            <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1.5">
              {t('batchInstall.machines')} ({batchMachines.length})
            </label>
            <div className="max-h-32 overflow-y-auto rounded-lg border border-[var(--bg-border)] px-3 py-2 space-y-1">
              {batchMachines.map(m => (
                <div key={m.mac} className="flex items-center gap-2 text-xs font-mono text-[var(--text-primary)]">
                  <span>{m.mac}</span>
                  {m.name && <span className="text-[var(--text-muted)]">{m.name}</span>}
                  {m.ip && <span className="text-[var(--text-muted)]">{m.ip}</span>}
                </div>
              ))}
            </div>
          </div>
          {batchSystems.length === 0 ? (
            <div className="flex items-center gap-3 px-3 py-2.5 rounded-lg bg-accent-yellow/10 border border-accent-yellow/30">
              <span className="text-xs text-accent-yellow flex-1">{t('batchInstall.noReadySystems')}</span>
              <Button size="sm" onClick={() => navigate('/store')}>{t('quickInstall.goStore')}</Button>
            </div>
          ) : (
            <>
              <div>
                <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1.5">{t('batchInstall.system')}</label>
                <Select value={batchSysKey} onChange={e => { setBatchSysKey(e.target.value); setBatchTplOverride(null) }}>
                  <option value="">{t('batchInstall.selectSystem')}</option>
                  {batchSystems.map(s => (
                    <option key={s.key} value={s.key}>{s.profile.name} — {s.distroName} {s.versionName} ({s.arch})</option>
                  ))}
                </Select>
              </div>
              <div>
                <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1.5">{t('quickInstall.sumTemplate')}</label>
                <Select
                  value={batchTplOverride ?? ''}
                  onChange={e => setBatchTplOverride(e.target.value ? Number(e.target.value) : null)}
                  disabled={!batchSys}
                >
                  <option value="">
                    {batchSys?.defaultTemplate
                      ? t('batchInstall.useDefault', { name: `${batchSys.defaultTemplate.name} (${batchSys.defaultTemplate.type})` })
                      : t('quickInstall.useDefault')}
                  </option>
                  {batchTemplates.map(tp => (
                    <option key={tp.id} value={tp.id}>{tp.name} ({tp.type})</option>
                  ))}
                </Select>
                {batchSys && !batchTemplate && (
                  <div className="mt-2 flex items-center gap-2">
                    <span className="text-xs text-accent-red flex-1">{t('quickInstall.missingTemplateWarn')}</span>
                    <Button size="sm" onClick={() => navigate('/answer-templates')}>{t('quickInstall.goAnswerTemplates')}</Button>
                  </div>
                )}
              </div>
            </>
          )}
          {batchSkipped.length > 0 && (
            <div className="rounded-lg border border-accent-yellow/30 bg-accent-yellow/5 px-3 py-2 space-y-1">
              <p className="text-xs font-semibold text-accent-yellow">{t('batchInstall.skippedTitle')}</p>
              {batchSkipped.map(s => (
                <div key={s.mac} className="text-xs font-mono text-[var(--text-muted)]">{s.mac} — {s.reason}</div>
              ))}
              <div className="pt-1">
                <Button size="sm" onClick={() => navigate('/install-tasks?view=batch')}>{t('quickInstall.viewTasks')}</Button>
              </div>
            </div>
          )}
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

