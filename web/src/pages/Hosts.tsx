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
import { api, type Host } from '../api/client'
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
  const [newHost, setNewHost] = useState({ name: '', mac: '', ip: '', profile_id: '' })
  const [profiles, setProfiles] = useState<{ id: string; name: string }[]>([])
  const [confirmDelete, setConfirmDelete] = useState<string | null>(null)

  const { pageSize } = useUIConfig()

  useEffect(() => {
    loadHosts()
  }, [page, search])

  useEffect(() => {
    api.getProfiles().then(res => {
      setProfiles(res.data.map(p => ({ id: p.id, name: p.name })))
    }).catch(() => {})
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

  async function handleCreate() {
    try {
      await api.createHost(newHost)
      success(t('hosts.created'))
      setShowModal(false)
      setNewHost({ name: '', mac: '', ip: '', profile_id: '' })
      loadHosts()
    } catch (err: any) {
      error(err.message)
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
      loadHosts()
    } catch (err: any) {
      error(err.message)
    } finally {
      setConfirmDelete(null)
    }
  }

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
          <Button variant="primary" size="sm" onClick={() => setShowModal(true)}>
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
          data={hosts}
          loading={loading}
          onRowClick={(h) => navigate('/hosts/' + h.id)}
          emptyText={t('hosts.empty', '暂无主机')}
        />
        <div className="px-5 py-3">
          <Pagination page={page} total={total} size={pageSize} onChange={setPage} />
        </div>
      </Card>

      <Modal
        open={showModal}
        onClose={() => setShowModal(false)}
        title={t('hosts.addHost')}
        footer={
          <>
            <Button variant="secondary" onClick={() => setShowModal(false)}>{t('common.cancel')}</Button>
            <Button variant="primary" onClick={handleCreate}>{t('common.create')}</Button>
          </>
        }
      >
        <div className="space-y-4">
          <div>
            <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1.5">MAC {t('hosts.columns.mac')}</label>
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
