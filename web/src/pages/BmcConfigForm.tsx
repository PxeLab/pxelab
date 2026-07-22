import { useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { Monitor, Loader2, Search, ExternalLink } from 'lucide-react'
import { Button } from '../components/ui/Button'
import { Modal } from '../components/ui/Modal'
import { Input, Select } from '../components/ui/FormControls'
import { api, type BMCConfig as BMCConfigType, type BMCProbeResult } from '../api/client'

interface Props {
  open: boolean
  onClose: () => void
  onSaved: () => void
  editConfig?: BMCConfigType | null
}

const PROTOCOL_OPTIONS = [
  { value: 'ipmi', label: 'IPMI' },
  { value: 'redfish', label: 'Redfish' },
]

export default function BMCConfigForm({ open, onClose, onSaved, editConfig }: Props) {
  const { t } = useTranslation()
  const isEdit = !!editConfig

  const [host, setHost] = useState(editConfig?.host || '')
  const [protocol, setProtocol] = useState(editConfig?.protocol || 'ipmi')
  const [port, setPort] = useState(editConfig?.port || 623)

  useEffect(() => {
    if (editConfig) return
    setPort(protocol === 'redfish' ? 443 : 623)
  }, [protocol, editConfig])
  const [username, setUsername] = useState(editConfig?.username || '')
  const [password, setPassword] = useState('')

  const [probeResult, setProbeResult] = useState<BMCProbeResult | null>(editConfig ? {
    name: editConfig.name || '',
    vendor: editConfig.vendor || '',
    model: editConfig.model || '',
    serial: editConfig.serial || '',
    mac: editConfig.mac || '',
  } : null)
  const [probeLoading, setProbeLoading] = useState(false)
  const [probeError, setProbeError] = useState('')
  const [probeSkipped, setProbeSkipped] = useState(false)

  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')

  async function handleProbe() {
    if (!host || !username || !password) {
      setProbeError(t('bmc.probeError'))
      return
    }
    setProbeLoading(true)
    setProbeError('')
    setProbeResult(null)
    setProbeSkipped(false)
    try {
      const res = await api.probeBMC({ host, port, username, password, protocol })
      setProbeResult(res.data)
    } catch (err: any) {
      setProbeError(err.message || t('bmc.probeFailed'))
    } finally {
      setProbeLoading(false)
    }
  }

  function skipProbe() {
    setProbeSkipped(true)
    setProbeResult({ name: host, vendor: '', model: '', serial: '', mac: '' })
  }

  async function handleSave() {
    if (!host || !username) {
      setError(t('bmc.addrUserRequired'))
      return
    }
    if (!isEdit && !password) {
      setError(t('bmc.passwordRequired'))
      return
    }
    setSaving(true)
    setError('')
    try {
      const data: Partial<BMCConfigType> = { host, port, username, password, protocol }
      if (probeResult) {
        data.name = probeResult.name
        data.vendor = probeResult.vendor
        data.model = probeResult.model
        data.serial = probeResult.serial
        data.mac = probeResult.mac
      }
      if (isEdit && editConfig?.id) {
        await api.updateBMCConfig(editConfig.id, data)
      } else {
        await api.createBMCConfig(data)
      }
      onSaved()
      onClose()
    } catch (err: any) {
      setError(err.message || t('bmc.saveFailed'))
    } finally {
      setSaving(false)
    }
  }

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={isEdit ? t('bmc.editConfig') : t('bmc.newConfigForm')}
      width="520px"
      footer={
        <>
          <Button variant="secondary" size="sm" onClick={onClose}>{t('bmc.cancel')}</Button>
          <Button variant="primary" size="sm" onClick={handleSave} disabled={saving}>
            {saving ? t('bmc.saving') : t('bmc.save')}
          </Button>
        </>
      }
    >
      <div className="space-y-4">
        <div>
          <label className="block text-xs font-semibold text-[var(--text-muted)] mb-1.5">{t('bmc.addr')} *</label>
          <Input
            type="text" value={host} onChange={e => setHost(e.target.value)}
            placeholder="10.0.0.1"
            className="font-mono"
          />
        </div>

        <div className="grid grid-cols-[1fr_120px] gap-3">
          <div>
            <label className="block text-xs font-semibold text-[var(--text-muted)] mb-1.5">{t('bmc.protocol')}</label>
            <Select
              value={protocol} onChange={e => setProtocol(e.target.value)}
            >
              {PROTOCOL_OPTIONS.map(o => (
                <option key={o.value} value={o.value}>{o.label}</option>
              ))}
            </Select>
          </div>
          <div>
            <label className="block text-xs font-semibold text-[var(--text-muted)] mb-1.5">{t('bmc.port')}</label>
            <Input
              type="number" value={port} onChange={e => setPort(Number(e.target.value))}
              className="font-mono"
            />
          </div>
        </div>

        <div className="grid grid-cols-2 gap-3">
          <div>
            <label className="block text-xs font-semibold text-[var(--text-muted)] mb-1.5">{t('bmc.username')} *</label>
            <Input
              type="text" value={username} onChange={e => setUsername(e.target.value)}
              placeholder="admin"
            />
          </div>
          <div>
            <label className="block text-xs font-semibold text-[var(--text-muted)] mb-1.5">
              {t('bmc.password')}{isEdit ? '' : ' *'}
            </label>
            <Input
              type="password" value={password} onChange={e => setPassword(e.target.value)}
              placeholder={isEdit ? t('bmc.editPasswordHint') : ''}
            />
          </div>
        </div>

        {!probeSkipped && !probeResult && (
          <div className="flex items-center gap-2">
            <Button variant="secondary" size="sm" onClick={handleProbe} disabled={probeLoading}>
              {probeLoading ? <Loader2 size={14} className="animate-spin" /> : <Search size={14} />}
              {probeLoading ? t('bmc.probing') : t('bmc.detectDevice')}
            </Button>
            <button
              onClick={skipProbe}
              className="text-xs text-[var(--text-muted)] hover:text-[var(--text-primary)] underline"
            >
              {t('bmc.skipProbe')}
            </button>
          </div>
        )}

        {probeError && (
          <div className="flex items-start gap-2 p-3 rounded-lg bg-accent-red/5 border border-accent-red/10">
            <span className="text-xs text-accent-red flex-1">{probeError}</span>
            <button onClick={() => { setProbeError(''); skipProbe() }} className="text-xs text-[var(--text-muted)] hover:text-[var(--text-primary)] underline shrink-0">
              {t('bmc.skipProbe')}
            </button>
          </div>
        )}

        {(probeResult || probeSkipped) && (
          <div className="p-3 rounded-lg bg-blue-500/5 border border-blue-500/10 space-y-2">
            <div className="flex items-center gap-2 text-xs font-semibold text-blue-400">
              <Monitor size={14} />
              {t('bmc.deviceInfo')}
            </div>
            <div className="grid grid-cols-2 gap-2 text-xs">
              <div>
                <span className="text-[var(--text-muted)]">{t('bmc.deviceName')}</span>
                <Input size="xs"
                  type="text" value={probeResult?.name || ''}
                  onChange={e => setProbeResult(prev => prev ? { ...prev, name: e.target.value } : { name: e.target.value, vendor: '', model: '', serial: '', mac: '' })}
                  className="mt-1"
                />
              </div>
              <div>
                <span className="text-[var(--text-muted)]">{t('bmc.brand')}</span>
                <Input size="xs"
                  type="text" value={probeResult?.vendor || ''}
                  onChange={e => setProbeResult(prev => prev ? { ...prev, vendor: e.target.value } : null)}
                  className="mt-1"
                />
              </div>
              <div>
                <span className="text-[var(--text-muted)]">{t('bmc.model')}</span>
                <Input size="xs"
                  type="text" value={probeResult?.model || ''}
                  onChange={e => setProbeResult(prev => prev ? { ...prev, model: e.target.value } : null)}
                  className="mt-1"
                />
              </div>
              <div>
                <span className="text-[var(--text-muted)]">SN</span>
                <Input size="xs"
                  type="text" value={probeResult?.serial || ''}
                  onChange={e => setProbeResult(prev => prev ? { ...prev, serial: e.target.value } : null)}
                  className="mt-1"
                />
              </div>
              <div className="col-span-2">
                <span className="text-[var(--text-muted)]">MAC</span>
                <Input size="xs"
                  type="text" value={probeResult?.mac || ''}
                  onChange={e => setProbeResult(prev => prev ? { ...prev, mac: e.target.value } : null)}
                  className="mt-1 font-mono"
                />
              </div>
            </div>
          </div>
        )}

        {host && (
          <div className="text-xs text-[var(--text-muted)] flex items-center gap-1">
            <ExternalLink size={12} />
            {t('bmc.webInterface')}：
            <a
              href={`https://${host}`}
              target="_blank"
              rel="noopener noreferrer"
              className="text-blue-400 hover:text-blue-300 underline"
            >
              https://{host}
            </a>
          </div>
        )}

        {error && (
          <div className="p-2 rounded bg-accent-red/5 border border-accent-red/10 text-xs text-accent-red">
            {error}
          </div>
        )}
      </div>
    </Modal>
  )
}
