import { useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { Save, RefreshCw, Plus, Eye, Trash2 } from 'lucide-react'
import { Card } from '../components/ui/Card'
import { Button } from '../components/ui/Button'
import { Toggle } from '../components/ui/Toggle'
import { Modal } from '../components/ui/Modal'
import { Tag, type TagColor } from '../components/ui/Tag'
import { DataTable, type Column } from '../components/ui/DataTable'
import { useToast } from '../components/ui/Toast'
import { SettingsField, SettingsInput } from '../components/settings/SettingsField'
import { api, type DNSSettingsData, type DNSRecord } from '../api/client'

const typeColors: Record<string, TagColor> = {
  A: 'blue', AAAA: 'purple', CNAME: 'green', TXT: 'cyan', MX: 'orange',
}

export default function SettingsDNS() {
  const { t } = useTranslation()
  const { success, error } = useToast()

  // DNS settings
  const [settingsLoading, setSettingsLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [config, setConfig] = useState<DNSSettingsData>({
    enabled: false, port: 53, upstream: '', local_domain: 'PxeLab.local', default_record: false,
  })

  // DNS records
  const [records, setRecords] = useState<DNSRecord[]>([])
  const [localDomain, setLocalDomain] = useState('')
  const [recordsLoading, setRecordsLoading] = useState(true)
  const [showModal, setShowModal] = useState(false)
  const [editing, setEditing] = useState<DNSRecord | null>(null)
  const [formSaving, setFormSaving] = useState(false)
  const [deleteTarget, setDeleteTarget] = useState<DNSRecord | null>(null)
  const [form, setForm] = useState<DNSRecord>({ name: '', type: 'A', value: '', ttl: 300, enabled: true, subnet: '' })
  const [subnetOptions, setSubnetOptions] = useState<string[]>([])

  useEffect(() => {
    loadSettings()
    loadRecords()
    loadSubnets()
  }, [])

  async function loadSettings() {
    setSettingsLoading(true)
    try {
      const res = await api.getDNSSettings()
      const d = res.data
      setConfig({
        enabled: d.enabled ?? false,
        port: 53,
        upstream: d.upstream || '',
        local_domain: d.local_domain || 'PxeLab.local',
        default_record: d.default_record ?? false,
      })
    } catch (err: any) {
      error(err.message || t('settings.loadFailed'))
    } finally {
      setSettingsLoading(false)
    }
  }

  async function loadRecords() {
    setRecordsLoading(true)
    try {
      const res = await api.getDNSRecords()
      setRecords(res.data?.records || [])
      if (res.data?.local_domain) setLocalDomain(res.data.local_domain)
    } catch { error(t('settings.recordsLoadFailed')) }
    finally { setRecordsLoading(false) }
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
    } catch { /* 子网列表加载失败不影响主功能 */ }
  }

  async function handleSave() {
    setSaving(true)
    try {
      await api.updateDNSSettings({ ...config, enabled: true, port: 53 })
      success(t('settings.saved'))
    } catch (err: any) {
      error(err.message || t('settings.saveFailed'))
    } finally {
      setSaving(false)
    }
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

  async function handleFormSave() {
    if (!form.name.trim() || !form.value.trim()) {
      error(t('settings.nameAndValueRequired'))
      return
    }
    setFormSaving(true)
    try {
      if (editing && editing.id) {
        await api.updateDNSRecord(editing.id, form)
        success(t('settings.saved'))
      } else {
        await api.createDNSRecord(form)
        success(t('settings.saved'))
      }
      setShowModal(false)
      loadRecords()
    } catch (err: any) { error(err.message) }
    finally { setFormSaving(false) }
  }

  async function handleDelete(id: number) {
    setDeleteTarget(null)
    try { await api.deleteDNSRecord(id); success(t('settings.deleted')); loadRecords() }
    catch (err: any) { error(err.message) }
  }

  async function toggleEnabled(r: DNSRecord) {
    try {
      await api.updateDNSRecord(r.id!, { ...r, enabled: !r.enabled })
      setRecords(prev => prev.map(x => x.id === r.id ? { ...x, enabled: !x.enabled } : x))
    } catch (err: any) { error(err.message) }
  }

  const columns: Column<DNSRecord>[] = [
    { key: 'name', label: t('settings.name'), render: (r) => (
      <div>
        <span className="font-medium text-[var(--text-primary)]">{r.name === '@' ? localDomain : r.name}</span>
        {localDomain && r.name !== '@' && <span className="text-[10px] text-[var(--text-muted)] ml-1">.{localDomain}</span>}
      </div>
    ) },
    { key: 'type', label: t('settings.type'), render: (r) => <Tag color={typeColors[r.type] || 'blue'}>{r.type}</Tag> },
    { key: 'value', label: t('settings.value'), render: (r) => <span className="font-mono text-xs text-[var(--text-primary)]">{r.value}</span> },
    { key: 'ttl', label: t('settings.ttl'), render: (r) => <span className="font-mono text-xs text-[var(--text-muted)]">{r.ttl}s</span> },
    { key: 'subnet', label: t('settings.subnet'), render: (r) => <span className="text-xs text-[var(--text-muted)]">{r.subnet || t('common.none')}</span> },
    { key: 'enabled', label: t('settings.enabled'), render: (r) => <Toggle checked={r.enabled} onChange={() => toggleEnabled(r)} /> },
    { key: 'actions', label: '', render: (r) => (
      <div className="flex gap-1">
        <Button variant="ghost" size="sm" onClick={() => openEdit(r)}><Eye size={13} /></Button>
        <Button variant="ghost" size="sm" onClick={() => r.id && setDeleteTarget(r)}><Trash2 size={13} /></Button>
      </div>
    ), width: '80px' },
  ]

  return (
    <div>
      <div className="flex items-center justify-between mb-4">
        <h1 className="text-lg font-bold text-[var(--text-primary)]">{t('settings.dnsTitle')}</h1>
        <div className="flex gap-2">
          <Button variant="secondary" size="sm" disabled={settingsLoading} onClick={loadSettings}>
            <RefreshCw size={14} /> {t('common.reload', '重载配置')}
          </Button>
          <Button variant="primary" size="sm" disabled={saving} onClick={handleSave}>
            <Save size={14} /> {saving ? t('settings.saving') : t('settings.save')}
          </Button>
        </div>
      </div>

      <Card>
        {settingsLoading ? (
          <div className="flex items-center justify-center py-16">
            <div className="animate-spin w-6 h-6 border-2 border-blue-500 border-t-transparent rounded-full" />
            <span className="ml-3 text-sm text-[var(--text-muted)]">{t('settings.loading')}</span>
          </div>
        ) : (
          <div className="space-y-4">
            <div className="grid grid-cols-2 gap-4">
              <SettingsField label={t('settings.localDomain')} help={t('settings.localDomainHelp', { domain: config.local_domain || 'PxeLab.local' })}>
                <SettingsInput value={config.local_domain || ''} onChange={v => setConfig({...config, local_domain: v})} />
              </SettingsField>
              <SettingsField label={t('settings.dnsUpstream')} help={t('settings.upstreamDnsHelp')}>
                <SettingsInput value={config.upstream} onChange={v => setConfig({...config, upstream: v})} placeholder="8.8.8.8:53 1.1.1.1:53" />
              </SettingsField>
            </div>
            <Toggle checked={config.default_record ?? false} onChange={v => setConfig({...config, default_record: v})} label={t('settings.wildcardResolve')} />
            <p className="text-xs text-[var(--text-muted)] -mt-2">{t('settings.wildcardResolveHelp')}</p>
          </div>
        )}
      </Card>

      {/* DNS Records */}
      <div className="flex items-center justify-between mt-8 mb-4">
        <h2 className="text-sm font-semibold text-[var(--text-primary)]">{t('settings.dnsRecords')}</h2>
        <Button variant="primary" size="sm" onClick={openCreate}>
          <Plus size={14} /> {t('settings.addRecord')}
        </Button>
      </div>

      <Card padding={false}>
        <DataTable columns={columns} data={records} loading={recordsLoading} emptyText={t('settings.dnsEmpty')} />
      </Card>

      <Modal
        open={showModal}
        onClose={() => setShowModal(false)}
        title={editing ? t('settings.editDnsRecord') : t('settings.addDnsRecord')}
        width="500px"
        footer={
          <>
            <Button variant="secondary" onClick={() => setShowModal(false)} disabled={formSaving}>{t('common.cancel')}</Button>
            <Button variant="primary" onClick={handleFormSave} disabled={formSaving}>{formSaving ? t('settings.saving') : t('common.save')}</Button>
          </>
        }
      >
        <div className="space-y-4">
          <div>
            <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1">{t('settings.name')}</label>
            <div className="flex items-center gap-2">
              <input className="flex-1 bg-[var(--bg-input)] border border-[var(--bg-border)] rounded-lg px-3.5 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500" value={form.name} onChange={e => setForm({...form, name: e.target.value})} placeholder={t('settings.namePlaceholder')} />
              {localDomain && (
                <span className="text-sm text-[var(--text-muted)] font-mono whitespace-nowrap">.{localDomain}</span>
              )}
            </div>
          </div>
          <div className="grid grid-cols-3 gap-4">
            <div>
              <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1">{t('settings.type')}</label>
              <select className="w-full bg-[var(--bg-input)] border border-[var(--bg-border)] rounded-lg px-3.5 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500 appearance-none" value={form.type} onChange={e => setForm({...form, type: e.target.value})}>
                <option value="A">A</option>
                <option value="AAAA">AAAA</option>
                <option value="CNAME">CNAME</option>
                <option value="TXT">TXT</option>
                <option value="MX">MX</option>
              </select>
            </div>
            <div>
              <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1">{t('settings.ttl')}</label>
              <input type="number" className="w-full bg-[var(--bg-input)] border border-[var(--bg-border)] rounded-lg px-3.5 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500" value={form.ttl} onChange={e => setForm({...form, ttl: parseInt(e.target.value) || 300})} />
            </div>
            <div>
              <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1">{t('settings.subnet')}</label>
              <select className="w-full bg-[var(--bg-input)] border border-[var(--bg-border)] rounded-lg px-3.5 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500 appearance-none" value={form.subnet || ''} onChange={e => setForm({...form, subnet: e.target.value})}>
                <option value="">{t('common.none')}</option>
                {subnetOptions.map(s => <option key={s} value={s}>{s}</option>)}
              </select>
            </div>
            <div>
              <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1">{t('settings.value')}</label>
              <input className="w-full bg-[var(--bg-input)] border border-[var(--bg-border)] rounded-lg px-3.5 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500 font-mono" value={form.value} onChange={e => setForm({...form, value: e.target.value})} placeholder={form.type === 'A' ? '192.168.1.100' : ''} />
            </div>
          </div>
          <label className="flex items-center gap-2.5 cursor-pointer pt-1">
            <button type="button" onClick={() => setForm({...form, enabled: !form.enabled})}
              className={`relative w-10 h-5.5 rounded-full transition-colors ${form.enabled ? 'bg-blue-500' : 'bg-[var(--bg-border)]'}`}>
              <span className={`absolute top-0.5 left-0.5 w-4.5 h-4.5 rounded-full bg-white transition-transform ${form.enabled ? 'translate-x-4.5' : ''}`} />
            </button>
            <span className="text-sm text-[var(--text-secondary)]">{t('settings.enabled')}</span>
          </label>
        </div>
      </Modal>

      <Modal
        open={!!deleteTarget}
        onClose={() => setDeleteTarget(null)}
        title={t('settings.confirmDelete')}
        width="400px"
        footer={
          <>
            <Button variant="secondary" onClick={() => setDeleteTarget(null)}>{t('common.cancel')}</Button>
            <Button variant="primary" onClick={() => deleteTarget?.id && handleDelete(deleteTarget.id)}>{t('settings.confirmDeleteAction')}</Button>
          </>
        }
      >
        <p className="text-sm text-[var(--text-secondary)]">
          {t('settings.deleteDnsConfirm', { name: deleteTarget?.name, type: deleteTarget?.type })}
        </p>
      </Modal>
    </div>
  )
}
