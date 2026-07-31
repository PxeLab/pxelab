import { useState, useEffect } from 'react'
import { Plus, Pencil, Trash2 } from 'lucide-react'
import { Card } from '../../components/ui/Card'
import { Button } from '../../components/ui/Button'
import { Modal } from '../../components/ui/Modal'
import { Tag } from '../../components/ui/Tag'
import { DataTable, type Column } from '../../components/ui/DataTable'
import { useToast } from '../../components/ui/Toast'
import { api, type InterfaceSettings, type InterfaceInfo } from '../../api/client'
import { useTranslation } from 'react-i18next'
import {
  defaultIface, validateIP, validateCIDR, ipInCIDR,
  type SubnetConfig, type InterfaceConfig,
} from './utils'
import { InterfaceEditModal } from './InterfaceEditModal'

export function DHCPConfigTab() {
  const { t } = useTranslation()
  const { success, error: showError } = useToast()
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [availableIfaces, setAvailableIfaces] = useState<InterfaceInfo[]>([])
  const [interfaces, setInterfaces] = useState<InterfaceConfig[]>([])

  // Modal state
  const [showModal, setShowModal] = useState(false)
  const [editIndex, setEditIndex] = useState<number | null>(null)
  const [editForm, setEditForm] = useState<InterfaceConfig>({ ...defaultIface })
  const [modalSaving, setModalSaving] = useState(false)

  // Global whitelist state
  const [globalWhitelist, setGlobalWhitelist] = useState(false)

  // Delete confirm
  const [deleteTarget, setDeleteTarget] = useState<number | null>(null)

  useEffect(() => { loadSettings() }, [])

  async function loadSettings() {
    setLoading(true)
    try {
      const [res, ifaceRes, settingsRes] = await Promise.all([api.getInterfaceSettings(), api.getInterfaces(), api.getSettings()])
      setGlobalWhitelist(settingsRes.data.whitelist_enabled)
      setAvailableIfaces(ifaceRes.data)
      const ifaces = res.data.interfaces || []
      if (ifaces.length > 0) {
        setInterfaces(ifaces.map((ir: InterfaceSettings) => {
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
                whitelistEnabled: s.whitelist_enabled || false,
              }))
            : [{ cidr: ir.subnet || '', dhcpMode: 'server', pools: ir.pools?.length ? ir.pools : [''], gateway: ir.gateway || '', dnsServers: ir.dns_servers || '', leaseTime: String(ir.lease_time || 3600), nextServer: ir.next_server || '', chainToIPXE: false, whitelistEnabled: false }]
          return {
            name: ir.name || '',
            ip: ir.ip || '',
            subnets,
          }
        }))
      }
    } catch (err: any) {
      showError(err.message || t('settings.loadInterfaceFailed'))
    } finally {
      setLoading(false)
    }
  }

  function openEdit(i: number) {
    const iface = interfaces[i]
    if (globalWhitelist) {
      iface.subnets = iface.subnets.map(s => ({ ...s, whitelistEnabled: true }))
    }
    setEditIndex(i)
    setEditForm({ ...iface })
    setShowModal(true)
  }

  function openCreate() {
    setEditIndex(null)
    setEditForm({ ...defaultIface })
    setShowModal(true)
  }

  async function handleModalSave() {
    const errs = validateFormItem(editForm)
    if (errs.length > 0) { showError(errs.join('\n')); return }
    setModalSaving(true)
    try {
      const next = editIndex !== null
        ? interfaces.map((iface, i) => i === editIndex ? editForm : iface)
        : [...interfaces, editForm]
      setInterfaces(next)
      setShowModal(false)
      setSaving(true)
      const payload: InterfaceSettings[] = next.filter(iface => iface.name).map(iface => ({
        name: iface.name, ip: iface.ip,
        subnets: iface.subnets.map(s => ({
          cidr: s.cidr, dhcp_mode: s.dhcpMode, pools: s.pools.filter(p => p && p.includes('-')),
          gateway: s.gateway, dns_servers: s.dnsServers, lease_time: parseInt(s.leaseTime) || 3600, next_server: s.nextServer, chain_to_ipxe: s.chainToIPXE, whitelist_enabled: s.whitelistEnabled,
        })),
        subnet: iface.subnets[0]?.cidr || '', pools: [], gateway: iface.subnets[0]?.gateway || '',
        dns_servers: iface.subnets[0]?.dnsServers || '', lease_time: parseInt(iface.subnets[0]?.leaseTime || '3600') || 3600,
        next_server: iface.subnets[0]?.nextServer || '',
      }))
      await api.updateInterfaceSettings({ interfaces: payload })
      success(t('settings.saved'))
    } catch (err: any) { showError(err.message || t('settings.saveFailed')) }
    finally { setModalSaving(false); setSaving(false) }
  }

  async function handleDelete(i: number) {
    setDeleteTarget(null)
    const next = interfaces.filter((_, j) => j !== i)
    setInterfaces(next)
      setSaving(true)
      try {
        const payload: InterfaceSettings[] = next.filter(iface => iface.name).map(iface => ({
          name: iface.name, ip: iface.ip,
          subnets: iface.subnets.map(s => ({
            cidr: s.cidr, dhcp_mode: s.dhcpMode, pools: s.pools.filter(p => p && p.includes('-')),
            gateway: s.gateway, dns_servers: s.dnsServers, lease_time: parseInt(s.leaseTime) || 3600, next_server: s.nextServer, chain_to_ipxe: s.chainToIPXE, whitelist_enabled: s.whitelistEnabled,
          })),
          subnet: iface.subnets[0]?.cidr || '', pools: [], gateway: iface.subnets[0]?.gateway || '',
          dns_servers: iface.subnets[0]?.dnsServers || '', lease_time: parseInt(iface.subnets[0]?.leaseTime || '3600') || 3600,
          next_server: iface.subnets[0]?.nextServer || '',
        }))
        await api.updateInterfaceSettings({ interfaces: payload })
        success(t('settings.deleted'))
    } catch (err: any) { showError(err.message || t('settings.deleteFailed')) }
    finally { setSaving(false) }
  }

  function validateFormItem(iface: InterfaceConfig): string[] {
    const errs: string[] = []
    for (let si = 0; si < iface.subnets.length; si++) {
      const s = iface.subnets[si]
      const mode = s.dhcpMode || 'server'
      if (mode === 'server') {
        if (s.cidr && !validateCIDR(s.cidr)) errs.push(`${t('settings.subnetNumberLabel')} #${si + 1}: ${t('settings.cidrInvalid')}`)
        for (let pi = 0; pi < s.pools.length; pi++) {
          const pool = s.pools[pi]
          if (!pool) continue
          const parts = pool.split('-')
          if (parts.length !== 2 || !validateIP(parts[0].trim()) || !validateIP(parts[1].trim())) {
            errs.push(`${t('settings.subnetNumberLabel')} #${si + 1}: ${t('settings.addressPoolInvalid')}`); continue
          }
          if (s.cidr) {
            const startIP = parts[0].trim(), endIP = parts[1].trim()
            if (!ipInCIDR(startIP, s.cidr)) errs.push(`${t('settings.subnetNumberLabel')} #${si + 1}: ${t('settings.addressPoolStartInvalid', { ip: startIP, cidr: s.cidr })}`)
            if (!ipInCIDR(endIP, s.cidr)) errs.push(`${t('settings.subnetNumberLabel')} #${si + 1}: ${t('settings.addressPoolEndInvalid', { ip: endIP, cidr: s.cidr })}`)
          }
        }
        if (s.gateway && !validateIP(s.gateway)) errs.push(`${t('settings.subnetNumberLabel')} #${si + 1}: ${t('settings.gatewayInvalid')}`)
      }
    }
    return errs
  }

  const columns: Column<{ idx: number; iface: InterfaceConfig }>[] = [
    { key: 'idx', label: '#', width: '40px', render: (r) => <span className="text-xs text-[var(--text-muted)]">{r.idx + 1}</span> },
    { key: 'name', label: t('settings.interfaceName'), render: (r) => <span className="font-medium text-[var(--text-primary)] text-sm">{r.iface.name || <span className="text-[var(--text-muted)] italic">{t('common.notConfigured')}</span>}</span> },
    { key: 'ip', label: 'IP', render: (r) => <span className="font-mono text-xs">{r.iface.ip || '—'}</span> },
    { key: 'subnets', label: t('settings.subnet'), render: (r) => (
      r.iface.subnets.length > 0
        ? <div className="space-y-1">
            {r.iface.subnets.map((s, i) => (
              <div key={i} className="flex items-center gap-1.5 text-xs whitespace-nowrap">
                <span className="font-mono text-[var(--text-primary)]">{s.cidr || '—'}</span>
                <Tag color={s.dhcpMode === 'server' ? 'blue' : s.dhcpMode === 'proxy' ? 'purple' : 'cyan'}>
                  {s.dhcpMode === 'server' ? 'server' : s.dhcpMode === 'proxy' ? 'proxy' : 'off'}
                </Tag>
              </div>
            ))}
          </div>
        : <span className="text-xs text-[var(--text-muted)]">—</span>
    )},
    { key: 'chainToIPXE', label: t('settings.chainToIpxe'), render: (r) => (
      r.iface.subnets.length > 0
        ? <div className="space-y-1">
            {r.iface.subnets.map((s, i) => (
              <div key={i} className="flex items-center gap-1.5 text-xs whitespace-nowrap">
                <span className="font-mono text-[var(--text-muted)]">{s.cidr || '—'}</span>
                <Tag color={s.chainToIPXE ? 'green' : 'red'}>{s.chainToIPXE ? t('common.yes') : t('common.no')}</Tag>
              </div>
            ))}
          </div>
        : <span className="text-xs text-[var(--text-muted)]">—</span>
    ) },
    { key: 'whitelist', label: t('settings.whitelist'), render: (r) => {
      return r.iface.subnets.length > 0
        ? <div className="space-y-1">
            {r.iface.subnets.map((s, i) => {
              const wlOn = globalWhitelist || s.whitelistEnabled
              return (
                <div key={i} className="flex items-center gap-1.5 text-xs whitespace-nowrap">
                  <span className="font-mono text-[var(--text-muted)]">{s.cidr || '—'}</span>
                  <Tag color={globalWhitelist ? 'blue' : wlOn ? 'green' : 'red'}>
                    {globalWhitelist ? t('settings.whitelistGlobal') : wlOn ? t('common.yes') : t('common.no')}
                  </Tag>
                </div>
              )
            })}
          </div>
        : <span className="text-xs text-[var(--text-muted)]">—</span>
    }},
    { key: '_actions', label: '', width: '80px', render: (r) => (
      <div className="flex gap-1" onClick={e => e.stopPropagation()}>
        <Button variant="ghost" size="sm" onClick={() => openEdit(r.idx)}><Pencil size={13} /></Button>
        <Button variant="ghost" size="sm" onClick={() => setDeleteTarget(r.idx)}><Trash2 size={13} /></Button>
      </div>
    )},
  ]

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center gap-2">
          {saving && <span className="text-xs text-[var(--text-muted)] animate-pulse">{t('settings.saving')}</span>}
        </div>
        <Button variant="primary" size="sm" onClick={openCreate}>
          <Plus size={14} /> {t('settings.newInterface')}
        </Button>
      </div>

      <Card padding={false}>
        <DataTable columns={columns} data={interfaces.map((iface, i) => ({ idx: i, iface }))}
          loading={loading} onRowClick={(r) => openEdit(r.idx)}
          emptyText={t('settings.dhcpEmpty')} rowKey={(r) => r.iface.name || String(r.idx)} />
      </Card>

      <InterfaceEditModal open={showModal} onClose={() => setShowModal(false)}
        form={editForm} setForm={setEditForm} onSave={handleModalSave} saving={modalSaving} availableIfaces={availableIfaces}
        globalWhitelistEnabled={globalWhitelist}
      />

      <Modal open={!!deleteTarget} onClose={() => setDeleteTarget(null)} title={t('settings.confirmDeleteTitle')} width="400px" disableBackdropClose
        footer={
          <>
            <Button variant="secondary" onClick={() => setDeleteTarget(null)}>{t('common.cancel')}</Button>
            <Button variant="primary" onClick={() => deleteTarget !== null && handleDelete(deleteTarget)}>{t('common.delete')}</Button>
          </>
        }
      >
        <p className="text-sm text-[var(--text-secondary)]">
          {t('settings.deleteInterfaceConfirm', { name: deleteTarget !== null ? interfaces[deleteTarget]?.name || `#${deleteTarget + 1}` : '' })}
        </p>
      </Modal>
    </div>
  )
}
