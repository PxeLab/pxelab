import { useTranslation } from 'react-i18next'
import { Button } from '../../components/ui/Button'
import { Modal } from '../../components/ui/Modal'
import { SettingsField, SettingsInput } from '../../components/settings/SettingsField'
import { Input, Select } from '../../components/ui/FormControls'
import { type InterfaceInfo } from '../../api/client'
import { type InterfaceConfig } from './utils'

export function InterfaceEditModal({
  open, onClose, form, setForm, onSave, saving, availableIfaces, globalWhitelistEnabled,
}: {
  open: boolean; onClose: () => void
  form: InterfaceConfig; setForm: (f: InterfaceConfig) => void
  onSave: () => void; saving: boolean
  availableIfaces: InterfaceInfo[]
  globalWhitelistEnabled?: boolean
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
            <Select
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
            </Select>
          </SettingsField>
          <SettingsField label={t('settings.ipAddress')}>
            <SettingsInput value={form.ip} onChange={v => setForm({...form, ip: v})} placeholder="192.168.1.100" />
          </SettingsField>
        </div>

        {form.subnets.map((s, si) => {
          const subnetMode = s.dhcpMode || 'server'
          const isOffSubnet = subnetMode === 'off'
          const isProxySubnet = subnetMode === 'proxy'
          const disableFields = isOffSubnet || isProxySubnet
          return (
            <div key={si} className="border border-[var(--bg-border)] rounded-lg p-4 space-y-3">
              <div className="flex items-center justify-between">
                <span className="text-sm font-medium text-[var(--text-primary)]">{t('settings.subnetNumber', { num: si + 1 })}</span>
                {form.subnets.length > 1 && (
                  <button onClick={() => setForm({...form, subnets: form.subnets.filter((_, j) => j !== si)})}
                    className="text-xs text-accent-red transition-colors">{t('settings.subnetRemove')}</button>
                )}
              </div>
              <div className="grid grid-cols-2 gap-4">
                <SettingsField label={t('settings.subnetCidr')}>
                  <SettingsInput value={s.cidr} onChange={v => {
                    const sn = [...form.subnets]; sn[si] = {...sn[si], cidr: v}; setForm({...form, subnets: sn})
                  }} placeholder="192.168.1.0/24" />
                </SettingsField>
                <SettingsField label={t('settings.dhcpMode')}>
                  <Select
                    value={s.dhcpMode} onChange={e => {
                      const sn = [...form.subnets]; sn[si] = {...sn[si], dhcpMode: e.target.value}; setForm({...form, subnets: sn})
                    }}>
                    <option value="server">{t('settings.dhcpModeFull')}</option>
                    <option value="proxy">{t('settings.dhcpModeProxy')}</option>
                    <option value="off">{t('settings.dhcpModeOff')}</option>
                  </Select>
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
                        <Input type="text" value={pool} onChange={e => {
                          const sn = [...form.subnets]; const pools = [...sn[si].pools]; pools[pi] = e.target.value
                          sn[si] = {...sn[si], pools}; setForm({...form, subnets: sn})
                        }} placeholder="192.168.1.100-192.168.1.200"
                          className="flex-1" />
                        <button onClick={() => {
                          const sn = [...form.subnets]; sn[si] = {...sn[si], pools: sn[si].pools.filter((_, j) => j !== pi)}
                          setForm({...form, subnets: sn})
                        }} className="p-2 rounded-lg border border-[var(--bg-border)] hover:bg-[var(--bg-hover)] text-[var(--text-muted)] hover:text-accent-red transition-colors text-xs font-bold">✕</button>
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
                    }} className="rounded border-[var(--bg-border)] w-4 h-4" />
                    <span>{t('settings.chainToIpxe')}</span>
                    {s.chainToIPXE && (
                      <span className="text-xs text-blue-400">{t('settings.chainToIpxeHint')}</span>
                    )}
                  </label>
                  <label className="flex items-center gap-2.5 text-sm text-[var(--text-secondary)] cursor-pointer">
                    <input type="checkbox" checked={globalWhitelistEnabled || s.whitelistEnabled}
                      disabled={globalWhitelistEnabled}
                      onChange={e => {
                        const sn = [...form.subnets]; sn[si] = {...sn[si], whitelistEnabled: e.target.checked}; setForm({...form, subnets: sn})
                      }} className="rounded border-[var(--bg-border)] w-4 h-4" />
                    <span>{t('settings.enableSubnetWhitelist')}</span>
                    {globalWhitelistEnabled && (
                      <span className="text-xs text-blue-400">{t('settings.subnetWhitelistForced')}</span>
                    )}
                  </label>
                </>
              )}
            </div>
          )
        })}
        <Button variant="secondary" size="sm" onClick={() => setForm({...form, subnets: [...form.subnets, { cidr: '', dhcpMode: 'server', pools: [''], gateway: '', dnsServers: form.ip || '', leaseTime: '3600', nextServer: form.ip || '', chainToIPXE: false, whitelistEnabled: !!globalWhitelistEnabled }]})}>
          {t('settings.addSubnet')}
        </Button>


      </div>
    </Modal>
  )
}
