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
import { api, type Host, type Profile, type Baseline, type scriptDTO } from '../api/client'
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

  function openCreate() {
    setNewHost({ name: '', mac: '', ip: '', sn: '', profile_id: '', baseline_ids: [], script_ids: [] })
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
      await api.createHost({
        name: newHost.name,
        mac,
        ip: newHost.ip || undefined,
        sn: newHost.sn || undefined,
        profile_id: newHost.profile_id || undefined,
        baseline_ids: newHost.baseline_ids,
        script_ids: newHost.script_ids,
      })
      success(t('hosts.created'))
      setShowModal(false)
      setNewHost({ name: '', mac: '', ip: '', sn: '', profile_id: '', baseline_ids: [], script_ids: [] })
      loadHosts()
    } catch (err: any) {
      setCreateError(err.message)
    }
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
  const isInherited = (id: string) => inheritedBaselineIds.includes(id)

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
          <Button variant="primary" size="sm" onClick={openCreate}>
            <Plus size={14} /> {t('hosts.addHost')}
          </Button>
        }
      />
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
              <p className="text-[11px] font-medium text-[var(--text-muted)] mb-1">{t('hosts.addBaselines')}</p>
              <div className="max-h-32 overflow-y-auto rounded border border-[var(--bg-border)] p-2 space-y-1">
                {allBaselines.map(b => {
                  const on = isInherited(b.id) || newHost.baseline_ids.includes(b.id)
                  return (
                    <label key={b.id} className={`flex items-center gap-2 text-xs ${isInherited(b.id) ? 'opacity-60' : 'cursor-pointer hover:text-[var(--text-primary)]'}`}>
                      <input
                        type="checkbox"
                        checked={on}
                        disabled={isInherited(b.id)}
                        onChange={() => setNewHost({ ...newHost, baseline_ids: toggleArr(newHost.baseline_ids, b.id) })}
                        className="rounded border-[var(--bg-border)] text-blue-500 focus:ring-blue-500/30"
                      />
                      <span className="text-[var(--text-muted)]">{b.name}</span>
                    </label>
                  )
                })}
                {allBaselines.length === 0 && <p className="text-[11px] text-[var(--text-muted)]">{t('baselines.noBaselines')}</p>}
              </div>
            </div>
            <div>
              <p className="text-[11px] font-medium text-[var(--text-muted)] mb-1">{t('hosts.addScripts')}</p>
              <div className="max-h-32 overflow-y-auto rounded border border-[var(--bg-border)] p-2 space-y-1">
                {allScripts.map(sc => {
                  const on = newHost.script_ids.includes(sc.id)
                  return (
                    <label key={sc.id} className="flex items-center gap-2 text-xs cursor-pointer hover:text-[var(--text-primary)]">
                      <input
                        type="checkbox"
                        checked={on}
                        onChange={() => setNewHost({ ...newHost, script_ids: toggleArr(newHost.script_ids, sc.id) })}
                        className="rounded border-[var(--bg-border)] text-blue-500 focus:ring-blue-500/30"
                      />
                      <span className="text-[var(--text-muted)]">{sc.name}</span>
                      <span className="ml-auto px-1 py-px rounded text-[9px] uppercase text-[var(--text-muted)] bg-[var(--bg-hover)]">{sc.type}</span>
                    </label>
                  )
                })}
                {allScripts.length === 0 && <p className="text-[11px] text-[var(--text-muted)]">{t('scripts.noScripts')}</p>}
              </div>
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
