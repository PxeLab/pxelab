import { useState, useEffect, useCallback, useMemo } from 'react'
import { Plus, Pencil, Trash2 } from 'lucide-react'
import { Card } from '../../components/ui/Card'
import { Button } from '../../components/ui/Button'
import { Modal } from '../../components/ui/Modal'
import { DataTable, type Column } from '../../components/ui/DataTable'
import { useToast } from '../../components/ui/Toast'
import { SettingsField, SettingsInput } from '../../components/settings/SettingsField'
import { Select } from '../../components/ui/FormControls'
import { api, type InterfaceSettings, type InterfaceInfo, type Lease, type DHCPReservation } from '../../api/client'
import { useTranslation } from 'react-i18next'
import { type SubnetConfig, type InterfaceConfig } from './utils'

export function ReservationTab() {
  const { t } = useTranslation()
  const { success, error } = useToast()

  const [reservations, setReservations] = useState<DHCPReservation[]>([])
  const [leases, setLeases] = useState<Lease[]>([])
  const [availableIfaces, setAvailableIfaces] = useState<InterfaceInfo[]>([])
  const [ifaceSettings, setIfaceSettings] = useState<InterfaceConfig[]>([])
  const [loading, setLoading] = useState(true)
  const [filterSubnet, setFilterSubnet] = useState('')

  // Modal state
  const [showModal, setShowModal] = useState(false)
  const [editId, setEditId] = useState<number | null>(null)
  const [form, setForm] = useState<Partial<DHCPReservation>>({
    interface_name: '', subnet_cidr: '', mac: '', ip: '', hostname: '', description: '',
  })
  const [modalSaving, setModalSaving] = useState(false)

  // Delete confirm
  const [deleteTarget, setDeleteTarget] = useState<DHCPReservation | null>(null)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const [res, ifaceRes, ifaceSettingsRes, leaseRes] = await Promise.all([
        api.getDHCPReservations(),
        api.getInterfaces(),
        api.getInterfaceSettings(),
        api.getLeases(),
      ])
      setReservations(res.data?.reservations || [])
      setLeases(leaseRes.data || [])
      setAvailableIfaces(ifaceRes.data || [])
      // Build InterfaceConfig list from settings for subnet/pool lookup
      const settings = ifaceSettingsRes.data?.interfaces || []
      setIfaceSettings(settings.map((ir: InterfaceSettings) => {
        const subnets: SubnetConfig[] = ir.subnets && ir.subnets.length > 0
          ? ir.subnets.map(s => ({
              cidr: s.cidr || '',
              dhcpMode: s.dhcp_mode || 'server',
              pools: s.pools && s.pools.length > 0 ? s.pools : [''],
              gateway: s.gateway || '',
              dnsServers: s.dns_servers || '',
              leaseTime: String(s.lease_time || 3600),
              nextServer: s.next_server || '',
              chainToIPXE: s.chain_to_ipxe || false,
            }))
          : []
        return { name: ir.name || '', ip: ir.ip || '', bootloader: ir.bootloader || 'ipxe', subnets }
      }))
    } catch (err: any) {
      error(err.message || t('common.error'))
    } finally {
      setLoading(false)
    }
  }, [error, t])

  // Build a pool range lookup keyed by subnet CIDR
  const poolMap = useMemo(() => {
    const map: Record<string, string[]> = {}
    for (const iface of ifaceSettings) {
      for (const sn of iface.subnets) {
        if (sn.cidr && sn.pools.length > 0) {
          map[sn.cidr] = sn.pools.filter(p => p && p.includes('-'))
        }
      }
    }
    return map
  }, [ifaceSettings])

  const selectedPools = form.subnet_cidr ? poolMap[form.subnet_cidr] || [] : []

  // IP 冲突检测
  const ipConflict = useMemo<{ type: 'reservation' | 'lease' | null; detail: string }>(() => {
    if (!form.subnet_cidr || !form.ip) return { type: null, detail: '' }

    // 检查是否已被其他预留占用
    const dupRes = reservations.find(r =>
      r.subnet_cidr === form.subnet_cidr &&
      r.ip === form.ip &&
      r.id !== editId
    )
    if (dupRes) {
      const detail = dupRes.mac
        ? t('settings.ipConflictReservation', { mac: dupRes.mac })
        : t('settings.ipConflictReservationOnly')
      return { type: 'reservation', detail }
    }

    // 检查是否有活跃租约
    const activeLease = leases.find(l =>
      l.ip === form.ip &&
      new Date(l.expires_at) > new Date()
    )
    if (activeLease) {
      return { type: 'lease', detail: t('settings.ipConflictLease', { mac: activeLease.mac }) }
    }

    return { type: null, detail: '' }
  }, [form.subnet_cidr, form.ip, reservations, leases, editId, t])

  useEffect(() => { load() }, [load])

  const subnetOptions = [...new Set(reservations.map(r => r.subnet_cidr))].sort()
  const filtered = filterSubnet
    ? reservations.filter(r => r.subnet_cidr === filterSubnet)
    : reservations

  function openCreate() {
    setEditId(null)
    setForm({ interface_name: '', subnet_cidr: '', mac: '', ip: '', hostname: '', description: '' })
    setShowModal(true)
  }

  function openEdit(r: DHCPReservation) {
    setEditId(r.id ?? null)
    setForm({ ...r })
    setShowModal(true)
  }

  async function handleSave() {
    if (!form.ip) { error(t('reservations.ipRequired')); return }
    if (form.mac && !/^([0-9A-Fa-f]{2}[:-]){5}([0-9A-Fa-f]{2})$/.test(form.mac)) {
      error(t('reservations.macFormat')); return
    }
    setModalSaving(true)
    try {
      if (editId !== null) {
        await api.updateDHCPReservation(editId, form)
        success(t('settings.saved'))
      } else {
        await api.createDHCPReservation(form)
        success(t('settings.saved'))
      }
      setShowModal(false)
      load()
    } catch (err: any) {
      error(err.message)
    } finally {
      setModalSaving(false)
    }
  }

  async function handleDelete(r: DHCPReservation) {
    setDeleteTarget(null)
    if (r.id == null) return
    try {
      await api.deleteDHCPReservation(r.id)
      success(t('settings.deleted'))
      load()
    } catch (err: any) {
      error(err.message)
    }
  }

  const columns: Column<DHCPReservation>[] = [
    { key: 'mac', label: t('reservations.mac'), render: (r) => r.mac ? <span className="font-mono text-xs">{r.mac}</span> : <span className="text-xs text-[var(--text-muted)]">—</span> },
    { key: 'ip', label: t('reservations.ip'), render: (r) => <span className="font-mono text-xs text-[var(--text-primary)]">{r.ip}</span> },
    { key: 'hostname', label: t('reservations.hostname'), render: (r) => r.hostname ? <span className="text-xs">{r.hostname}</span> : <span className="text-xs text-[var(--text-muted)]">—</span> },
    { key: 'description', label: t('reservations.description'), render: (r) => r.description ? <span className="text-xs">{r.description}</span> : <span className="text-xs text-[var(--text-muted)]">—</span> },
    { key: 'interface_name', label: t('reservations.interface'), render: (r) => <span className="text-xs">{r.interface_name}</span> },
    { key: 'subnet_cidr', label: t('reservations.subnet'), render: (r) => <span className="font-mono text-xs">{r.subnet_cidr}</span> },
    { key: '_actions', label: '', width: '80px', render: (r) => (
      <div className="flex gap-1" onClick={e => e.stopPropagation()}>
        <Button variant="ghost" size="sm" onClick={() => openEdit(r)}><Pencil size={13} /></Button>
        <Button variant="ghost" size="sm" onClick={() => setDeleteTarget(r)}><Trash2 size={13} /></Button>
      </div>
    )},
  ]

  return (
    <div>
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-3">
          <select
            className="bg-[var(--bg-input)] border border-[var(--bg-border)] rounded-lg px-3 py-1.5 text-xs text-[var(--text-primary)] outline-none focus:border-blue-500 appearance-none cursor-pointer"
            value={filterSubnet}
            onChange={e => setFilterSubnet(e.target.value)}
          >
            <option value="">{t('reservations.filterAll')}</option>
            {subnetOptions.map(sn => (
              <option key={sn} value={sn}>{sn}</option>
            ))}
          </select>
        </div>
        <Button variant="primary" size="sm" onClick={openCreate}>
          <Plus size={14} /> {t('reservations.add')}
        </Button>
      </div>

      <Card padding={false}>
        <DataTable columns={columns} data={filtered}
          loading={loading} onRowClick={(r) => openEdit(r)}
          emptyText={t('reservations.empty')} rowKey={(r) => String(r.id ?? r.ip)} />
      </Card>

      {/* Add/Edit Modal */}
      <Modal open={showModal} onClose={() => setShowModal(false)}
        title={editId !== null ? t('reservations.edit') : t('reservations.add')} width="520px"
        footer={
          <>
            <Button variant="secondary" size="sm" onClick={() => setShowModal(false)} disabled={modalSaving}>{t('common.cancel')}</Button>
            <Button variant="primary" size="sm" onClick={handleSave} disabled={modalSaving}>{modalSaving ? t('settings.saving') : t('common.save')}</Button>
          </>
        }
      >
        <div className="space-y-4">
          <div className="grid grid-cols-2 gap-4">
            <SettingsField label={t('reservations.interface')}>
              <Select
                value={form.interface_name || ''}
                onChange={e => {
                  const iface = ifaceSettings.find(i => i.name === e.target.value)
                  const fullSubnet = iface?.subnets?.find(sn => sn.dhcpMode === 'server')
                  setForm({...form, interface_name: e.target.value, subnet_cidr: fullSubnet?.cidr || ''})
                }}
              >
                <option value="">{t('settings.selectSubnet')}</option>
                {availableIfaces.filter(i => i.up).map(ai => (
                  <option key={ai.name} value={ai.name}>{ai.name}</option>
                ))}
              </Select>
            </SettingsField>
            <SettingsField label={t('reservations.subnet')}>
              <Select
                value={form.subnet_cidr || ''}
                onChange={e => setForm({...form, subnet_cidr: e.target.value})}
              >
                <option value="">{t('settings.selectSubnet')}</option>
                {ifaceSettings
                  .filter(i => i.name === form.interface_name)
                  .flatMap(i => i.subnets)
                  .filter(sn => sn.cidr && sn.dhcpMode === 'server')
                  .map(sn => (
                    <option key={sn.cidr} value={sn.cidr}>
                      {sn.cidr}{t('settings.subnetFull')}
                    </option>
                  ))
                }
              </Select>
              {selectedPools.length > 0 && (
                <div className="mt-1.5 text-[11px] text-[var(--text-muted)] leading-relaxed">
                  <span className="font-medium text-[var(--text-secondary)]">{t('settings.addressPoolLabel')}</span>
                  {selectedPools.map((pool, i) => (
                    <span key={i} className="font-mono">{i > 0 ? '、' : ''}{pool}</span>
                  ))}
                </div>
              )}
            </SettingsField>
          </div>
          <div className="grid grid-cols-2 gap-4">
            <SettingsField label={t('reservations.mac')}>
              <SettingsInput value={form.mac || ''} onChange={v => setForm({...form, mac: v})} placeholder={`00:11:22:33:44:55 (${t('settings.optional')})`} />
            </SettingsField>
            <SettingsField label={t('reservations.ip')}>
              <SettingsInput value={form.ip || ''} onChange={v => setForm({...form, ip: v})} placeholder="192.168.1.100" />
              {form.ip && (
                <div className="mt-1.5 text-[11px] leading-relaxed">
                  {ipConflict.type ? (
                    <span className="text-accent-red">
                      <span className="font-medium">{t('settings.ipConflict')}</span>
                      {ipConflict.detail}
                    </span>
                  ) : (
                    <span className="text-accent-green">{t('settings.ipAvailable')}</span>
                  )}
                </div>
              )}
            </SettingsField>
          </div>
          <div className="grid grid-cols-2 gap-4">
            <SettingsField label={t('reservations.hostname')}>
              <SettingsInput value={form.hostname || ''} onChange={v => setForm({...form, hostname: v})} placeholder={`(${t('settings.optional')})`} />
            </SettingsField>
            <SettingsField label={t('reservations.description')}>
              <SettingsInput value={form.description || ''} onChange={v => setForm({...form, description: v})} placeholder={`(${t('settings.optional')})`} />
            </SettingsField>
          </div>
        </div>
      </Modal>

      {/* Delete Modal */}
      <Modal open={!!deleteTarget} onClose={() => setDeleteTarget(null)} title={t('common.delete')} width="400px"
        footer={
          <>
            <Button variant="secondary" onClick={() => setDeleteTarget(null)}>{t('common.cancel')}</Button>
            <Button variant="primary" onClick={() => deleteTarget && handleDelete(deleteTarget)}>{t('common.delete')}</Button>
          </>
        }
      >
        <p className="text-sm text-[var(--text-secondary)]">
          {t('reservations.deleteConfirm')}
        </p>
        {deleteTarget && (
          <div className="mt-3 p-3 bg-[var(--bg-card)] rounded-lg border border-[var(--bg-border)]">
            <p className="text-xs font-mono text-[var(--text-primary)]">{deleteTarget.ip} / {deleteTarget.mac || '—'}</p>
            <p className="text-xs text-[var(--text-muted)]">{deleteTarget.subnet_cidr}</p>
          </div>
        )}
      </Modal>
    </div>
  )
}
