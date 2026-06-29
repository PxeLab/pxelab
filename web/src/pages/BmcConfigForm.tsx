import { useState } from 'react'
import { Monitor, Loader2, Search, ExternalLink } from 'lucide-react'
import { Button } from '../components/ui/Button'
import { Modal } from '../components/ui/Modal'
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

const BOOT_MODE_OPTIONS = [
  { value: 'auto', label: '自动探测' },
  { value: 'uefi', label: 'UEFI' },
  { value: 'legacy', label: 'Legacy' },
]

export default function BMCConfigForm({ open, onClose, onSaved, editConfig }: Props) {
  const isEdit = !!editConfig

  const [host, setHost] = useState(editConfig?.host || '')
  const [port, setPort] = useState(editConfig?.port || 623)
  const [username, setUsername] = useState(editConfig?.username || '')
  const [password, setPassword] = useState('')
  const [protocol, setProtocol] = useState(editConfig?.protocol || 'ipmi')

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

  const [bootMode, setBootMode] = useState(editConfig?.boot_mode || 'auto')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')

  async function handleProbe() {
    if (!host || !username || !password) {
      setProbeError('请先填写 BMC 地址、用户名和密码')
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
      setProbeError(err.message || '探测失败')
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
      setError('BMC 地址和用户名不能为空')
      return
    }
    if (!isEdit && !password) {
      setError('新建配置必须填写密码')
      return
    }
    setSaving(true)
    setError('')
    try {
      const data: any = { host, port, username, password, protocol, boot_mode: bootMode }
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
      setError(err.message || '保存失败')
    } finally {
      setSaving(false)
    }
  }

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={isEdit ? '编辑 BMC 配置' : '新建 BMC 配置'}
      width="520px"
      footer={
        <>
          <Button variant="secondary" size="sm" onClick={onClose}>取消</Button>
          <Button variant="primary" size="sm" onClick={handleSave} disabled={saving}>
            {saving ? '保存中...' : '保存'}
          </Button>
        </>
      }
    >
      <div className="space-y-4">
        <div className="grid grid-cols-[1fr_100px] gap-3">
          <div>
            <label className="block text-xs font-semibold text-[var(--text-muted)] mb-1.5">BMC 地址 *</label>
            <input
              type="text" value={host} onChange={e => setHost(e.target.value)}
              placeholder="10.0.0.1"
              className="w-full bg-[var(--bg-base)] border border-[var(--bg-border)] rounded-lg px-3 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500 font-mono"
            />
          </div>
          <div>
            <label className="block text-xs font-semibold text-[var(--text-muted)] mb-1.5">端口</label>
            <input
              type="number" value={port} onChange={e => setPort(Number(e.target.value))}
              className="w-full bg-[var(--bg-base)] border border-[var(--bg-border)] rounded-lg px-3 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500 font-mono"
            />
          </div>
        </div>

        <div>
          <label className="block text-xs font-semibold text-[var(--text-muted)] mb-1.5">协议</label>
          <select
            value={protocol} onChange={e => setProtocol(e.target.value)}
            className="w-full bg-[var(--bg-base)] border border-[var(--bg-border)] rounded-lg px-3 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500"
          >
            {PROTOCOL_OPTIONS.map(o => (
              <option key={o.value} value={o.value}>{o.label}</option>
            ))}
          </select>
        </div>

        <div className="grid grid-cols-2 gap-3">
          <div>
            <label className="block text-xs font-semibold text-[var(--text-muted)] mb-1.5">用户名 *</label>
            <input
              type="text" value={username} onChange={e => setUsername(e.target.value)}
              placeholder="admin"
              className="w-full bg-[var(--bg-base)] border border-[var(--bg-border)] rounded-lg px-3 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500"
            />
          </div>
          <div>
            <label className="block text-xs font-semibold text-[var(--text-muted)] mb-1.5">
              密码{isEdit ? '' : ' *'}
            </label>
            <input
              type="password" value={password} onChange={e => setPassword(e.target.value)}
              placeholder={isEdit ? '留空则不修改' : ''}
              className="w-full bg-[var(--bg-base)] border border-[var(--bg-border)] rounded-lg px-3 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500"
            />
          </div>
        </div>

        {!probeSkipped && !probeResult && (
          <div className="flex items-center gap-2">
            <Button variant="secondary" size="sm" onClick={handleProbe} disabled={probeLoading}>
              {probeLoading ? <Loader2 size={14} className="animate-spin" /> : <Search size={14} />}
              {probeLoading ? '探测中...' : '检测设备'}
            </Button>
            <button
              onClick={skipProbe}
              className="text-xs text-[var(--text-muted)] hover:text-[var(--text-primary)] underline"
            >
              跳过检测
            </button>
          </div>
        )}

        {probeError && (
          <div className="flex items-start gap-2 p-3 rounded-lg bg-red-500/5 border border-red-500/10">
            <span className="text-xs text-red-400 flex-1">{probeError}</span>
            <button onClick={() => { setProbeError(''); skipProbe() }} className="text-xs text-[var(--text-muted)] hover:text-[var(--text-primary)] underline shrink-0">
              跳过检测
            </button>
          </div>
        )}

        {(probeResult || probeSkipped) && (
          <div className="p-3 rounded-lg bg-blue-500/5 border border-blue-500/10 space-y-2">
            <div className="flex items-center gap-2 text-xs font-semibold text-blue-400">
              <Monitor size={14} />
              设备信息（自动探测）
            </div>
            <div className="grid grid-cols-2 gap-2 text-xs">
              <div>
                <span className="text-[var(--text-muted)]">设备名</span>
                <input
                  type="text" value={probeResult?.name || ''}
                  onChange={e => setProbeResult(prev => prev ? { ...prev, name: e.target.value } : { name: e.target.value, vendor: '', model: '', serial: '', mac: '' })}
                  className="w-full bg-[var(--bg-base)] border border-[var(--bg-border)] rounded px-2 py-1 text-[var(--text-primary)] outline-none focus:border-blue-500 mt-1"
                />
              </div>
              <div>
                <span className="text-[var(--text-muted)]">品牌</span>
                <input
                  type="text" value={probeResult?.vendor || ''}
                  onChange={e => setProbeResult(prev => prev ? { ...prev, vendor: e.target.value } : null)}
                  className="w-full bg-[var(--bg-base)] border border-[var(--bg-border)] rounded px-2 py-1 text-[var(--text-primary)] outline-none focus:border-blue-500 mt-1"
                />
              </div>
              <div>
                <span className="text-[var(--text-muted)]">型号</span>
                <input
                  type="text" value={probeResult?.model || ''}
                  onChange={e => setProbeResult(prev => prev ? { ...prev, model: e.target.value } : null)}
                  className="w-full bg-[var(--bg-base)] border border-[var(--bg-border)] rounded px-2 py-1 text-[var(--text-primary)] outline-none focus:border-blue-500 mt-1"
                />
              </div>
              <div>
                <span className="text-[var(--text-muted)]">SN</span>
                <input
                  type="text" value={probeResult?.serial || ''}
                  onChange={e => setProbeResult(prev => prev ? { ...prev, serial: e.target.value } : null)}
                  className="w-full bg-[var(--bg-base)] border border-[var(--bg-border)] rounded px-2 py-1 text-[var(--text-primary)] outline-none focus:border-blue-500 mt-1"
                />
              </div>
              <div className="col-span-2">
                <span className="text-[var(--text-muted)]">MAC</span>
                <input
                  type="text" value={probeResult?.mac || ''}
                  onChange={e => setProbeResult(prev => prev ? { ...prev, mac: e.target.value } : null)}
                  className="w-full bg-[var(--bg-base)] border border-[var(--bg-border)] rounded px-2 py-1 text-[var(--text-primary)] outline-none focus:border-blue-500 mt-1 font-mono"
                />
              </div>
            </div>
          </div>
        )}

        <div>
          <label className="block text-xs font-semibold text-[var(--text-muted)] mb-1.5">引导模式</label>
          <select
            value={bootMode} onChange={e => setBootMode(e.target.value)}
            className="w-full bg-[var(--bg-base)] border border-[var(--bg-border)] rounded-lg px-3 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500"
          >
            {BOOT_MODE_OPTIONS.map(o => (
              <option key={o.value} value={o.value}>{o.label}</option>
            ))}
          </select>
        </div>

        {host && (
          <div className="text-xs text-[var(--text-muted)] flex items-center gap-1">
            <ExternalLink size={12} />
            BMC Web 界面：
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
          <div className="p-2 rounded bg-red-500/5 border border-red-500/10 text-xs text-red-400">
            {error}
          </div>
        )}
      </div>
    </Modal>
  )
}
