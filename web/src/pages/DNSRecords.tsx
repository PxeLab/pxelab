import { useState, useEffect } from 'react'
import { Plus, Eye, Trash2 } from 'lucide-react'
import { Card } from '../components/ui/Card'
import { DataTable, type Column } from '../components/ui/DataTable'
import { Button } from '../components/ui/Button'
import { Modal } from '../components/ui/Modal'
import { Tag, type TagColor } from '../components/ui/Tag'
import { Toggle } from '../components/ui/Toggle'
import { useToast } from '../components/ui/Toast'
import { api, type DNSRecord } from '../api/client'

const typeColors: Record<string, TagColor> = {
  A: 'blue', AAAA: 'purple', CNAME: 'green', TXT: 'cyan', MX: 'orange',
}

export default function DNSRecords() {
  const { success, error } = useToast()
  const [records, setRecords] = useState<DNSRecord[]>([])
  const [localDomain, setLocalDomain] = useState('')
  const [loading, setLoading] = useState(true)
  const [showModal, setShowModal] = useState(false)
  const [editing, setEditing] = useState<DNSRecord | null>(null)
  const [saving, setSaving] = useState(false)
  const [deleteTarget, setDeleteTarget] = useState<DNSRecord | null>(null)
  const [form, setForm] = useState<DNSRecord>({ name: '', type: 'A', value: '', ttl: 300, enabled: true, subnet: '' })
  const [subnetOptions, setSubnetOptions] = useState<string[]>([])

  useEffect(() => { load(); loadSubnets() }, [])

  async function load() {
    setLoading(true)
    try {
      const res = await api.getDNSRecords()
      setRecords(res.data?.records || [])
      if (res.data?.local_domain) setLocalDomain(res.data.local_domain)
    } catch { error('加载失败') }
    finally { setLoading(false) }
  }

  async function loadSubnets() {
    try {
      const res = await api.getInterfaceSettings()
      const subnets: string[] = []
      for (const iface of res.data?.interfaces || []) {
        for (const s of iface.subnets || []) {
          if (s.cidr && !subnets.includes(s.cidr)) subnets.push(s.cidr)
        }
      }
      setSubnetOptions(subnets)
    } catch { /* ignore */ }
  }

  function openCreate() {
    setEditing(null)
    setForm({ name: '', type: 'A', value: '', ttl: 300, enabled: true })
    setShowModal(true)
  }

  function openEdit(r: DNSRecord) {
    setEditing(r)
    setForm({ ...r })
    setShowModal(true)
  }

  async function handleSave() {
    if (!form.name.trim() || !form.value.trim()) {
      error('名称和值不能为空')
      return
    }
    setSaving(true)
    try {
      if (editing && editing.id) {
        await api.updateDNSRecord(editing.id, form)
        success('已更新')
      } else {
        await api.createDNSRecord(form)
        success('已创建')
      }
      setShowModal(false)
      load()
    } catch (err: any) { error(err.message) }
    finally { setSaving(false) }
  }

  async function handleDelete(id: number) {
    setDeleteTarget(null)
    try { await api.deleteDNSRecord(id); success('已删除'); load() }
    catch (err: any) { error(err.message) }
  }

  async function toggleEnabled(r: DNSRecord) {
    try {
      await api.updateDNSRecord(r.id!, { ...r, enabled: !r.enabled })
      setRecords(prev => prev.map(x => x.id === r.id ? { ...x, enabled: !x.enabled } : x))
    } catch (err: any) { error(err.message) }
  }

  const columns: Column<DNSRecord>[] = [
    { key: 'name', label: '名称', render: (r) => (
      <div>
        <span className="font-medium text-[var(--text-primary)]">{r.name === '@' ? localDomain : r.name}</span>
        {localDomain && r.name !== '@' && <span className="text-[10px] text-[var(--text-muted)] ml-1">.{localDomain}</span>}
      </div>
    ) },
    { key: 'type', label: '类型', render: (r) => <Tag color={typeColors[r.type] || 'blue'}>{r.type}</Tag> },
    { key: 'value', label: '值', render: (r) => <span className="font-mono text-xs text-[var(--text-primary)]">{r.value}</span> },
    { key: 'ttl', label: 'TTL', render: (r) => <span className="font-mono text-xs text-[var(--text-muted)]">{r.ttl}s</span> },
    { key: 'subnet', label: '子网', render: (r) => <span className="text-xs text-[var(--text-muted)]">{r.subnet || '全部'}</span> },
    { key: 'enabled', label: '启用', render: (r) => <Toggle checked={r.enabled} onChange={() => toggleEnabled(r)} /> },
    { key: 'actions', label: '', render: (r) => (
      <div className="flex gap-1">
        <Button variant="ghost" size="sm" onClick={() => openEdit(r)}><Eye size={13} /></Button>
        <Button variant="ghost" size="sm" onClick={() => r.id && setDeleteTarget(r)}><Trash2 size={13} /></Button>
      </div>
    ), width: '80px' },
  ]

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <div></div>
        <Button variant="primary" onClick={openCreate}>
          <Plus size={14} /> 添加记录
        </Button>
      </div>

      {localDomain && (
        <p className="text-xs text-[var(--text-muted)] mb-3">
          本地域名：<code className="text-[11px] bg-[var(--bg-card)] px-1.5 py-0.5 rounded font-mono text-[var(--text-secondary)]">{localDomain}</code>
        </p>
      )}

      <Card padding={false}>
        <DataTable columns={columns} data={records} loading={loading} emptyText="暂无 DNS 记录" />
      </Card>

      <Modal
        open={showModal}
        onClose={() => setShowModal(false)}
        title={editing ? '编辑 DNS 记录' : '添加 DNS 记录'}
        width="500px"
        footer={
          <>
            <Button variant="secondary" onClick={() => setShowModal(false)} disabled={saving}>取消</Button>
            <Button variant="primary" onClick={handleSave} disabled={saving}>{saving ? '保存中...' : '保存'}</Button>
          </>
        }
      >
        <div className="space-y-4">
          <div>
            <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1">名称</label>
            <div className="flex items-center gap-2">
              <input className="flex-1 bg-[var(--bg-input)] border border-[var(--bg-border)] rounded-lg px-3.5 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500" value={form.name} onChange={e => setForm({...form, name: e.target.value})} placeholder="例如: pxe-server" />
              {localDomain && (
                <span className="text-sm text-[var(--text-muted)] font-mono whitespace-nowrap">
                  .{localDomain}
                </span>
              )}
            </div>
          </div>
          <div className="grid grid-cols-3 gap-4">
            <div>
              <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1">类型</label>
              <select className="w-full bg-[var(--bg-input)] border border-[var(--bg-border)] rounded-lg px-3.5 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500 appearance-none" value={form.type} onChange={e => setForm({...form, type: e.target.value})}>
                <option value="A">A</option>
                <option value="AAAA">AAAA</option>
                <option value="CNAME">CNAME</option>
                <option value="TXT">TXT</option>
                <option value="MX">MX</option>
              </select>
            </div>
            <div>
              <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1">TTL</label>
              <input type="number" className="w-full bg-[var(--bg-input)] border border-[var(--bg-border)] rounded-lg px-3.5 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500" value={form.ttl} onChange={e => setForm({...form, ttl: parseInt(e.target.value) || 300})} />
            </div>
            <div>
              <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1">子网</label>
              <select className="w-full bg-[var(--bg-input)] border border-[var(--bg-border)] rounded-lg px-3.5 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500 appearance-none" value={form.subnet || ''} onChange={e => setForm({...form, subnet: e.target.value})}>
                <option value="">全部</option>
                {subnetOptions.map(s => <option key={s} value={s}>{s}</option>)}
              </select>
            </div>
            <div>
              <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1">值</label>
              <input className="w-full bg-[var(--bg-input)] border border-[var(--bg-border)] rounded-lg px-3.5 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500 font-mono" value={form.value} onChange={e => setForm({...form, value: e.target.value})} placeholder={form.type === 'A' ? '192.168.1.100' : ''} />
            </div>
          </div>
          <label className="flex items-center gap-2.5 cursor-pointer pt-1">
            <button type="button" onClick={() => setForm({...form, enabled: !form.enabled})} className={`relative w-10 h-5.5 rounded-full transition-colors ${form.enabled ? 'bg-blue-500' : 'bg-[var(--bg-border)]'}`}>
              <span className={`absolute top-0.5 left-0.5 w-4.5 h-4.5 rounded-full bg-white transition-transform ${form.enabled ? 'translate-x-4.5' : ''}`} />
            </button>
            <span className="text-sm text-[var(--text-secondary)]">启用</span>
          </label>
        </div>
      </Modal>

      <Modal
        open={!!deleteTarget}
        onClose={() => setDeleteTarget(null)}
        title="确认删除"
        width="400px"
        footer={
          <>
            <Button variant="secondary" onClick={() => setDeleteTarget(null)}>取消</Button>
            <Button variant="primary" onClick={() => deleteTarget?.id && handleDelete(deleteTarget.id)}>确定删除</Button>
          </>
        }
      >
        <p className="text-sm text-[var(--text-secondary)]">
          确定要删除 DNS 记录 <span className="font-semibold text-[var(--text-primary)]">{deleteTarget?.name}</span>（{deleteTarget?.type}）吗？此操作不可撤销。
        </p>
      </Modal>
    </div>
  )
}
