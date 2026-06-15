import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { Plus, Search } from 'lucide-react'
import { Card } from '../components/ui/Card'
import { DataTable, type Column } from '../components/ui/DataTable'
import { Pagination } from '../components/ui/Pagination'
import { Button } from '../components/ui/Button'
import { Modal } from '../components/ui/Modal'
import { useToast } from '../components/ui/Toast'
import { api, type Host } from '../api/client'

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

  const size = 10

  useEffect(() => {
    loadHosts()
  }, [page, search])

  async function loadHosts() {
    setLoading(true)
    try {
      const res = await api.getHosts({ page: String(page), size: String(size), search })
      setHosts(res.data.hosts)
      setTotal(res.data.meta.total)
    } catch (err: any) {
      error(err.message || '加载失败')
    } finally {
      setLoading(false)
    }
  }

  async function handleCreate() {
    try {
      await api.createHost(newHost)
      success('主机已创建')
      setShowModal(false)
      setNewHost({ name: '', mac: '', ip: '', profile_id: '' })
      loadHosts()
    } catch (err: any) {
      error(err.message)
    }
  }

  async function handleDelete(id: string) {
    if (!confirm(t('hosts.deleteConfirm'))) return
    try {
      await api.deleteHost(id)
      success('主机已删除')
      loadHosts()
    } catch (err: any) {
      error(err.message)
    }
  }

  const columns: Column<Host>[] = [
    { key: 'mac', label: t('hosts.columns.mac'), render: (h) => <span className="font-mono text-xs text-[#e8eaed]">{h.mac}</span>, sortable: true },
    { key: 'name', label: t('hosts.columns.hostname'), render: (h) => <span className="font-medium text-[#e8eaed]">{h.name || '—'}</span> },
    { key: 'ip', label: t('hosts.columns.ip'), render: (h) => <span className="font-mono text-xs">{h.ip}</span> },
    { key: 'boot_count', label: t('hosts.columns.bootCount'), render: (h) => <span className="font-mono text-xs">{h.boot_count}</span> },
    { key: 'last_online', label: t('hosts.columns.lastOnline'), render: (h) => (
      <span className="font-mono text-xs text-[#6b7294]">{h.last_online ? new Date(h.last_online).toLocaleString() : '—'}</span>
    )},
    { key: 'actions', label: '', render: (h) => (
      <div onClick={e => e.stopPropagation()}><Button variant="ghost" size="sm" onClick={() => handleDelete(h.id)}>{t('common.delete')}</Button></div>
    ), width: '60px' },
  ]

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center gap-3">
          <div className="relative">
            <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-[#6b7294]" />
            <input
              className="w-[280px] bg-[#1a1d2e] border border-[#232738] rounded-lg py-2 pl-9 pr-3 text-sm text-[#e8eaed] placeholder-[#6b7294] outline-none focus:border-blue-500 focus:ring-3 focus:ring-blue-500/10 transition-all"
              placeholder={t('hosts.search', '搜索 MAC / 主机名 / IP ...')}
              value={search}
              onChange={(e) => { setSearch(e.target.value); setPage(1) }}
            />
          </div>
          <span className="text-sm text-[#6b7294]">{t('common.total', '共')} {total} {t('common.items', '条')}</span>
        </div>
        <Button variant="primary" onClick={() => setShowModal(true)}>
          <Plus size={14} /> {t('hosts.addHost')}
        </Button>
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
          <Pagination page={page} total={total} size={size} onChange={setPage} />
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
            <label className="block text-xs font-semibold text-[#9aa0ab] mb-1.5">MAC {t('hosts.columns.mac')}</label>
            <input className="w-full bg-[#1a1d2e] border border-[#232738] rounded-lg px-3.5 py-2 text-sm text-[#e8eaed] outline-none focus:border-blue-500 focus:ring-3 focus:ring-blue-500/10" placeholder="00:11:22:33:44:55" value={newHost.mac} onChange={e => setNewHost({...newHost, mac: e.target.value})} />
          </div>
          <div>
            <label className="block text-xs font-semibold text-[#9aa0ab] mb-1.5">{t('hosts.columns.hostname')}</label>
            <input className="w-full bg-[#1a1d2e] border border-[#232738] rounded-lg px-3.5 py-2 text-sm text-[#e8eaed] outline-none focus:border-blue-500 focus:ring-3 focus:ring-blue-500/10" placeholder="node-01" value={newHost.name} onChange={e => setNewHost({...newHost, name: e.target.value})} />
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-xs font-semibold text-[#9aa0ab] mb-1.5">IP {t('hosts.columns.ip')}</label>
              <input className="w-full bg-[#1a1d2e] border border-[#232738] rounded-lg px-3.5 py-2 text-sm text-[#e8eaed] outline-none focus:border-blue-500 focus:ring-3 focus:ring-blue-500/10" placeholder="192.168.1.100" value={newHost.ip} onChange={e => setNewHost({...newHost, ip: e.target.value})} />
            </div>
            <div>
              <label className="block text-xs font-semibold text-[#9aa0ab] mb-1.5">{t('profiles.title')}</label>
              <input className="w-full bg-[#1a1d2e] border border-[#232738] rounded-lg px-3.5 py-2 text-sm text-[#e8eaed] outline-none focus:border-blue-500 focus:ring-3 focus:ring-blue-500/10" placeholder="profile-id" value={newHost.profile_id} onChange={e => setNewHost({...newHost, profile_id: e.target.value})} />
            </div>
          </div>
        </div>
      </Modal>
    </div>
  )
}
