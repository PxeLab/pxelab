import { useState, useEffect, useCallback, useMemo } from 'react'
import { useSearchParams } from 'react-router-dom'
import { RefreshCw, Trash2, AlertTriangle, Plus, Pencil } from 'lucide-react'
import { Card } from '../components/ui/Card'
import { Button } from '../components/ui/Button'
import { Modal } from '../components/ui/Modal'
import { Tag, type TagColor } from '../components/ui/Tag'
import { DataTable, type Column } from '../components/ui/DataTable'
import { useToast } from '../components/ui/Toast'
import { SettingsField, SettingsInput } from '../components/settings/SettingsField'
import { api, type InterfaceSettings, type InterfaceInfo, type Lease, type LeaseStats, type DHCPReservation } from '../api/client'
import { useTranslation } from 'react-i18next'

// ── Shared types ──

interface SubnetConfig {
  cidr: string; dhcpMode: string; pools: string[]; gateway: string; dnsServers: string; leaseTime: string; nextServer: string; chainToIPXE: boolean
}

interface InterfaceConfig {
  name: string; ip: string; bootloader: string
  subnets: SubnetConfig[]
}

const defaultIface: InterfaceConfig = {
  name: '', ip: '', bootloader: 'ipxe',
  subnets: [{ cidr: '', dhcpMode: 'full', pools: [''], gateway: '', dnsServers: '', leaseTime: '3600', nextServer: '', chainToIPXE: false }],
}

function validateIP(ip: string): boolean {
  if (!ip) return true
  const parts = ip.split('.')
  return parts.length === 4 && parts.every(p => {
    const n = parseInt(p)
    return n >= 0 && n <= 255 && String(n) === p
  })
}

function validateCIDR(cidr: string): boolean {
  if (!cidr) return true
  const parts = cidr.split('/')
  if (parts.length !== 2) return false
  const ips = parts[0].split('.')
  const mask = parseInt(parts[1])
  return ips.length === 4 && ips.every(p => {
    const n = parseInt(p)
    return n >= 0 && n <= 255 && String(n) === p
  }) && !isNaN(mask) && mask >= 0 && mask <= 32
}

function ipToInt(ip: string): number {
  const parts = ip.trim().split('.')
  if (parts.length !== 4) return NaN
  return parts.reduce((acc, oct) => {
    const n = parseInt(oct)
    if (isNaN(n) || n < 0 || n > 255) return NaN
    return (acc << 8) + n
  }, 0) >>> 0
}

function ipInCIDR(ip: string, cidr: string): boolean {
  const [netIP, maskStr] = cidr.split('/')
  const mask = parseInt(maskStr)
  if (isNaN(mask) || mask < 0 || mask > 32) return false
  const ipInt = ipToInt(ip)
  const netInt = ipToInt(netIP)
  if (isNaN(ipInt) || isNaN(netInt)) return false
  const maskInt = mask === 0 ? 0 : (~0 << (32 - mask)) >>> 0
  return (ipInt & maskInt) === (netInt & maskInt)
}

// ── Interface Edit Modal ──

function InterfaceEditModal({
  open, onClose, form, setForm, onSave, saving, availableIfaces,
}: {
  open: boolean; onClose: () => void
  form: InterfaceConfig; setForm: (f: InterfaceConfig) => void
  onSave: () => void; saving: boolean
  availableIfaces: InterfaceInfo[]
}) {
  const { t } = useTranslation()
  return (
    <Modal open={open} onClose={onClose} title={t('settings.interfaceConfig')} width="640px" disableBackdropClose
      footer={
        <>
          <Button variant="secondary" size="sm" onClick={onClose} disabled={saving}>{t('common.cancel')}</Button>
          <Button variant="primary" size="sm" onClick={onSave} disabled={saving}>{saving ? t('settings.saving') : t('common.save')}</Button>
        </>
      }
    >
      <div className="space-y-4 max-h-[60vh] overflow-y-auto pr-1">
        <div className="grid grid-cols-2 gap-4">
          <SettingsField label={t('settings.interfaceName')}>
            <select
              className="w-full bg-[var(--bg-input)] border border-[var(--bg-border)] rounded-lg px-3.5 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500 appearance-none"
              value={form.name}
              onChange={e => {
                const sel = availableIfaces.find(x => x.name === e.target.value)
                const pool = sel?.ipv4?.[0] || ''
                const cidr = pool ? pool.replace(/\.\d+$/, '.0/24') : ''
                const subnets = form.subnets.map((sn, si) => ({
                  ...sn, cidr: si === 0 ? cidr : sn.cidr, dnsServers: pool || sn.dnsServers
                }))
                setForm({...form, name: e.target.value, ip: pool, subnets})
              }}
            >
              <option value="">{t('settings.selectNic')}</option>
              {availableIfaces.map(ai => (
                <option key={ai.name} value={ai.name}>
                  {ai.name} {ai.ipv4?.length ? `(${ai.ipv4[0]})` : ''} {!ai.up ? `[${t('settings.disconnected')}]` : ''}
                </option>
              ))}
            </select>
          </SettingsField>
          <SettingsField label={t('settings.ipAddress')}>
            <SettingsInput value={form.ip} onChange={v => setForm({...form, ip: v})} placeholder="192.168.1.100" />
          </SettingsField>
        </div>
        <div className="grid grid-cols-2 gap-4">
          <SettingsField label={t('settings.bootloader')}>
            <select className="w-full bg-[var(--bg-input)] border border-[var(--bg-border)] rounded-lg px-3.5 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500 appearance-none"
              value={form.bootloader} onChange={e => setForm({...form, bootloader: e.target.value})}>
              <option value="ipxe">{t('settings.ipxeFull')}</option>
              <option value="undionly">{t('settings.ipxeUndi')}</option>
              <option value="pxelinux">PXELinux</option>
              <option value="grub2">GRUB2</option>
            </select>
          </SettingsField>
        </div>

        {form.subnets.map((s, si) => {
          const subnetMode = s.dhcpMode || 'full'
          const isOffSubnet = subnetMode === 'off'
          const isProxySubnet = subnetMode === 'proxy'
          const disableFields = isOffSubnet || isProxySubnet
          return (
            <div key={si} className="border border-[var(--bg-border)] rounded-lg p-4 space-y-3">
              <div className="flex items-center justify-between">
                <span className="text-sm font-medium text-[var(--text-primary)]">{t('settings.subnetNumber', { num: si + 1 })}</span>
                {form.subnets.length > 1 && (
                  <button onClick={() => setForm({...form, subnets: form.subnets.filter((_, j) => j !== si)})}
                    className="text-xs text-red-400 hover:text-red-300 transition-colors">{t('settings.subnetRemove')}</button>
                )}
              </div>
              <div className="grid grid-cols-2 gap-4">
                <SettingsField label={t('settings.subnetCidr')}>
                  <SettingsInput value={s.cidr} onChange={v => {
                    const sn = [...form.subnets]; sn[si] = {...sn[si], cidr: v}; setForm({...form, subnets: sn})
                  }} placeholder="192.168.1.0/24" />
                </SettingsField>
                <SettingsField label={t('settings.dhcpMode')}>
                  <select className="w-full bg-[var(--bg-input)] border border-[var(--bg-border)] rounded-lg px-3.5 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500 appearance-none"
                    value={s.dhcpMode} onChange={e => {
                      const sn = [...form.subnets]; sn[si] = {...sn[si], dhcpMode: e.target.value}; setForm({...form, subnets: sn})
                    }}>
                    <option value="full">{t('settings.dhcpModeFull')}</option>
                    <option value="proxy">{t('settings.dhcpModeProxy')}</option>
                    <option value="off">{t('settings.dhcpModeOff')}</option>
                  </select>
                </SettingsField>
              </div>
              {disableFields ? (
                <p className="text-xs text-[var(--text-muted)] italic">
                  {isOffSubnet ? t('settings.dhcpOffHelp') : t('settings.dhcpProxyHelp')}
                </p>
              ) : (
                <>
                  <div className="space-y-2">
                    <label className="block text-xs font-semibold text-[var(--text-secondary)]">{t('settings.addressPool')}</label>
                    {s.pools.map((pool, pi) => (
                      <div key={pi} className="flex items-center gap-2">
                        <input type="text" value={pool} onChange={e => {
                          const sn = [...form.subnets]; const pools = [...sn[si].pools]; pools[pi] = e.target.value
                          sn[si] = {...sn[si], pools}; setForm({...form, subnets: sn})
                        }} placeholder="192.168.1.100-192.168.1.200"
                          className="flex-1 bg-[var(--bg-input)] border border-[var(--bg-border)] rounded-lg px-3.5 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500 placeholder-[var(--text-muted)]" />
                        <button onClick={() => {
                          const sn = [...form.subnets]; sn[si] = {...sn[si], pools: sn[si].pools.filter((_, j) => j !== pi)}
                          setForm({...form, subnets: sn})
                        }} className="p-2 rounded-lg border border-[var(--bg-border)] hover:bg-[var(--bg-hover)] text-[var(--text-muted)] hover:text-red-400 transition-colors text-xs font-bold">✕</button>
                      </div>
                    ))}
                    <button onClick={() => {
                      const sn = [...form.subnets]; sn[si] = {...sn[si], pools: [...sn[si].pools, '']}
                      setForm({...form, subnets: sn})
                    }} className="text-xs text-blue-400 hover:text-blue-300 transition-colors">{t('settings.addAddressRange')}</button>
                  </div>
                  <div className="grid grid-cols-2 gap-4">
                    <SettingsField label={t('settings.gateway')}>
                      <SettingsInput value={s.gateway} onChange={v => {
                        const sn = [...form.subnets]; sn[si] = {...sn[si], gateway: v}; setForm({...form, subnets: sn})
                      }} placeholder="192.168.1.1" />
                    </SettingsField>
                    <SettingsField label={t('settings.dnsServer')}>
                      <SettingsInput value={s.dnsServers} onChange={v => {
                        const sn = [...form.subnets]; sn[si] = {...sn[si], dnsServers: v}; setForm({...form, subnets: sn})
                      }} placeholder={form.ip || t('settings.dnsServerPlaceholder')} />
                    </SettingsField>
                  </div>
                  <div className="grid grid-cols-2 gap-4">
                    <SettingsField label={t('settings.leaseTimeSeconds')}>
                      <SettingsInput value={s.leaseTime} onChange={v => {
                        const sn = [...form.subnets]; sn[si] = {...sn[si], leaseTime: v}; setForm({...form, subnets: sn})
                      }} />
                    </SettingsField>
                    <SettingsField label={t('settings.nextServer')}>
                      <SettingsInput value={s.nextServer} onChange={v => {
                        const sn = [...form.subnets]; sn[si] = {...sn[si], nextServer: v}; setForm({...form, subnets: sn})
                      }} placeholder={t('settings.nextServerPlaceholder')} />
                    </SettingsField>
                  </div>
                  <label className="flex items-center gap-2.5 text-sm text-[var(--text-secondary)] cursor-pointer pt-1">
                    <input type="checkbox" checked={s.chainToIPXE} onChange={e => {
                      const sn = [...form.subnets]; sn[si] = {...sn[si], chainToIPXE: e.target.checked}; setForm({...form, subnets: sn})
                    }} disabled={form.bootloader !== 'pxelinux' && form.bootloader !== 'grub2'} className="rounded border-[var(--bg-border)] w-4 h-4" />
                    <span className={form.bootloader !== 'pxelinux' && form.bootloader !== 'grub2' ? 'opacity-40' : ''}>{t('settings.chainToIpxe')}</span>
                    {(form.bootloader === 'pxelinux' || form.bootloader === 'grub2') && s.chainToIPXE && (
                      <span className="text-xs text-blue-400">{t('settings.chainToIpxeHint')}</span>
                    )}
                  </label>
                </>
              )}
            </div>
          )
        })}
        <Button variant="secondary" size="sm" onClick={() => setForm({...form, subnets: [...form.subnets, { cidr: '', dhcpMode: 'full', pools: [''], gateway: '', dnsServers: form.ip || '', leaseTime: '3600', nextServer: form.ip || '', chainToIPXE: false }]})}>
          {t('settings.addSubnet')}
        </Button>


      </div>
    </Modal>
  )
}

// ── DHCP Settings Tab ──

function DHCPConfigTab() {
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

  // Delete confirm
  const [deleteTarget, setDeleteTarget] = useState<number | null>(null)

  useEffect(() => { loadSettings() }, [])

  async function loadSettings() {
    setLoading(true)
    try {
      const [res, ifaceRes] = await Promise.all([api.getInterfaceSettings(), api.getInterfaces()])
      setAvailableIfaces(ifaceRes.data)
      const ifaces = res.data.interfaces || []
      if (ifaces.length > 0) {
        setInterfaces(ifaces.map((ir: InterfaceSettings) => {
          const subnets: SubnetConfig[] = ir.subnets && ir.subnets.length > 0
            ? ir.subnets.map(s => ({
                cidr: s.cidr || '',
                dhcpMode: s.dhcp_mode || 'full',
                pools: s.pools && s.pools.length > 0 ? s.pools : [''],
                gateway: s.gateway || '',
                dnsServers: s.dns_servers || '',
                leaseTime: String(s.lease_time || 3600),
                nextServer: s.next_server || '',
                chainToIPXE: s.chain_to_ipxe || false,
              }))
            : [{ cidr: ir.subnet || '', dhcpMode: 'full', pools: ir.pools?.length ? ir.pools : [''], gateway: ir.gateway || '', dnsServers: ir.dns_servers || '', leaseTime: String(ir.lease_time || 3600), nextServer: ir.next_server || '', chainToIPXE: false }]
          return {
            name: ir.name || '',
            ip: ir.ip || '',
            bootloader: ir.bootloader || 'ipxe',
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
    setEditIndex(i)
    setEditForm({ ...interfaces[i] })
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
        name: iface.name, ip: iface.ip, bootloader: iface.bootloader,
        subnets: iface.subnets.map(s => ({
          cidr: s.cidr, dhcp_mode: s.dhcpMode, pools: s.pools.filter(p => p && p.includes('-')),
          gateway: s.gateway, dns_servers: s.dnsServers, lease_time: parseInt(s.leaseTime) || 3600, next_server: s.nextServer, chain_to_ipxe: s.chainToIPXE,
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
        name: iface.name, ip: iface.ip, bootloader: iface.bootloader,
        subnets: iface.subnets.map(s => ({
          cidr: s.cidr, dhcp_mode: s.dhcpMode, pools: s.pools.filter(p => p && p.includes('-')),
          gateway: s.gateway, dns_servers: s.dnsServers, lease_time: parseInt(s.leaseTime) || 3600, next_server: s.nextServer, chain_to_ipxe: s.chainToIPXE,
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
      const mode = s.dhcpMode || 'full'
      if (mode === 'full') {
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

  const bootloaderLabel: Record<string, string> = {
    ipxe: 'iPXE', undionly: 'iPXE(UNDI)', pxelinux: 'PXELinux', grub2: 'GRUB2',
  }

  const columns: Column<{ idx: number; iface: InterfaceConfig }>[] = [
    { key: 'idx', label: '#', width: '40px', render: (r) => <span className="text-xs text-[var(--text-muted)]">{r.idx + 1}</span> },
    { key: 'name', label: t('settings.interfaceName'), render: (r) => <span className="font-medium text-[var(--text-primary)] text-sm">{r.iface.name || <span className="text-[var(--text-muted)] italic">{t('common.notConfigured')}</span>}</span> },
    { key: 'bootloader', label: t('settings.bootloader'), render: (r) => <span className="text-xs">{bootloaderLabel[r.iface.bootloader] || r.iface.bootloader}</span> },
    { key: 'ip', label: 'IP', render: (r) => <span className="font-mono text-xs">{r.iface.ip || '—'}</span> },
    { key: 'subnets', label: t('settings.subnet'), render: (r) => (
      r.iface.subnets.length > 0
        ? <div className="space-y-1">
            {r.iface.subnets.map((s, i) => (
              <div key={i} className="flex items-center gap-1.5 text-xs whitespace-nowrap">
                <span className="font-mono text-[var(--text-primary)]">{s.cidr || '—'}</span>
                <Tag color={s.dhcpMode === 'full' ? 'blue' : s.dhcpMode === 'proxy' ? 'purple' : 'cyan'}>
                  {s.dhcpMode === 'full' ? 'full' : s.dhcpMode === 'proxy' ? 'proxy' : 'off'}
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

// ── DHCP Leases Tab ──

function LeasesTab() {
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
            <p className="text-2xl font-bold text-green-500">{activeLeases}</p>
          </div>
        </Card>
        <Card>
          <div className="text-center">
            <p className="text-xs text-[var(--text-muted)] mb-1">{t('leases.statusExpired')}</p>
            <p className="text-2xl font-bold text-red-500">{expiredLeases}</p>
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
                      <Tag color={s.dhcp_mode === 'full' ? 'blue' : 'purple'} className="ml-2">
                        {s.dhcp_mode === 'full' ? t('leases.dhcpModeFull') : s.dhcp_mode === 'proxy' ? t('leases.dhcpModeProxy') : t('leases.dhcpModeOff')}
                      </Tag>
                    </div>
                  </div>
                  {s.pool_size > 0 ? (
                    <>
                      <div className="flex items-center gap-4 text-xs text-[var(--text-muted)] mb-2">
                        <span>{t('leases.poolSize')}: <strong className="text-[var(--text-primary)]">{s.pool_size}</strong></span>
                        <span>{t('leases.allocated')}: <strong className="text-[var(--text-primary)]">{s.allocated}</strong></span>
                        <span>{t('leases.available')}: <strong className="text-green-500">{s.available}</strong></span>
                        <span>{t('leases.usage')}: <strong className={s.usage_pct > 80 ? 'text-red-500' : s.usage_pct > 50 ? 'text-yellow-500' : 'text-blue-500'}>{s.usage_pct.toFixed(1)}%</strong></span>
                      </div>
                      <div className="w-full h-2 bg-[var(--bg-card)] rounded-full overflow-hidden border border-[var(--bg-border)]">
                        <div
                          className={`h-full rounded-full transition-all duration-300 ${
                            s.usage_pct > 80 ? 'bg-red-500' : s.usage_pct > 50 ? 'bg-yellow-500' : 'bg-blue-500'
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

// ── Main Page ──


// ── DHCP Reservations Tab ──

function ReservationTab() {
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
              dhcpMode: s.dhcp_mode || 'full',
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
              <select
                className="w-full bg-[var(--bg-input)] border border-[var(--bg-border)] rounded-lg px-3.5 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500 appearance-none"
                value={form.interface_name || ''}
                onChange={e => {
                  const iface = ifaceSettings.find(i => i.name === e.target.value)
                  const fullSubnet = iface?.subnets?.find(sn => sn.dhcpMode === 'full')
                  setForm({...form, interface_name: e.target.value, subnet_cidr: fullSubnet?.cidr || ''})
                }}
              >
                <option value="">{t('settings.selectSubnet')}</option>
                {availableIfaces.filter(i => i.up).map(ai => (
                  <option key={ai.name} value={ai.name}>{ai.name}</option>
                ))}
              </select>
            </SettingsField>
            <SettingsField label={t('reservations.subnet')}>
              <select
                className="w-full bg-[var(--bg-input)] border border-[var(--bg-border)] rounded-lg px-3.5 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500 appearance-none"
                value={form.subnet_cidr || ''}
                onChange={e => setForm({...form, subnet_cidr: e.target.value})}
              >
                <option value="">{t('settings.selectSubnet')}</option>
                {ifaceSettings
                  .filter(i => i.name === form.interface_name)
                  .flatMap(i => i.subnets)
                  .filter(sn => sn.cidr && sn.dhcpMode === 'full')
                  .map(sn => (
                    <option key={sn.cidr} value={sn.cidr}>
                      {sn.cidr}{t('settings.subnetFull')}
                    </option>
                  ))
                }
              </select>
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
                    <span className="text-red-400">
                      <span className="font-medium">{t('settings.ipConflict')}</span>
                      {ipConflict.detail}
                    </span>
                  ) : (
                    <span className="text-green-500">{t('settings.ipAvailable')}</span>
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

export default function SettingsInterfaces() {
  const { t } = useTranslation()
  const [searchParams, setSearchParams] = useSearchParams()
  const activeTab = (searchParams.get('tab') as 'dhcp' | 'leases' | 'reservations') || 'dhcp'

  const tabs = [
    { key: 'dhcp' as const, label: t('settings.dhcpInterfaces') },
    { key: 'leases' as const, label: t('settings.dhcpLeases') },
    { key: 'reservations' as const, label: t('settings.dhcpReservations') },
  ]

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-lg font-bold text-[var(--text-primary)]">{t('settings.dhcpTitle')}</h1>
      </div>
      <div className="flex gap-1 mb-6 border-b border-[var(--bg-border)]">
        {tabs.map(tab => (
          <button key={tab.key}
            onClick={() => setSearchParams({ tab: tab.key })}
            className={`px-4 py-2.5 text-sm font-medium border-b-2 transition-colors -mb-[1px] ${
              activeTab === tab.key
                ? 'border-blue-500 text-blue-400'
                : 'border-transparent text-[var(--text-muted)] hover:text-[var(--text-primary)]'
            }`}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {activeTab === 'dhcp' ? <DHCPConfigTab /> : activeTab === 'leases' ? <LeasesTab /> : <ReservationTab />}
    </div>
  )
}
