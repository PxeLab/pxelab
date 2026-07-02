import { useState, useEffect, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import {
  Plus, Trash2, RefreshCw, Upload, ExternalLink,
  Power, PowerOff, RotateCcw, Disc,
  Play, Square,
} from 'lucide-react'
import { Button } from '../components/ui/Button'
import { Card } from '../components/ui/Card'
import { Modal } from '../components/ui/Modal'
import { useToast } from '../components/ui/Toast'
import { api, type BMCConfig as BMCConfigType } from '../api/client'
import BMCConfigForm from './BmcConfigForm'

function bmcWebURL(host: string): string {
  return `https://${host}`
}

const BOOT_DEVICES = [
  { value: 'pxe', key: 'bmc.pxe' },
  { value: 'disk', key: 'bmc.disk' },
  { value: 'cdrom', key: 'bmc.cdrom' },
  { value: 'bios', key: 'bmc.bios' },
]

export default function BmcView() {
  const { t } = useTranslation()
  const { success, error: showError } = useToast()

  const [configs, setConfigs] = useState<BMCConfigType[]>([])
  const [loading, setLoading] = useState(true)
  const [statusMap, setStatusMap] = useState<Record<number, string>>({})
  const [selectedIds, setSelectedIds] = useState<Set<number>>(new Set())

  const [formOpen, setFormOpen] = useState(false)
  const [editConfig, setEditConfig] = useState<BMCConfigType | null>(null)

  const [bootDeviceCfg, setBootDeviceCfg] = useState<BMCConfigType | null>(null)
  const [bootDevice, setBootDevice] = useState('pxe')
  const [bootDeviceLoading, setBootDeviceLoading] = useState(false)

  const [csvOpen, setCsvOpen] = useState(false)
  const [csvText, setCsvText] = useState('')
  const [csvImporting, setCsvImporting] = useState(false)

  const [confirmOpen, setConfirmOpen] = useState(false)
  const [confirmTitle, setConfirmTitle] = useState('')
  const [confirmMsg, setConfirmMsg] = useState('')
  const [confirmAction, setConfirmAction] = useState<() => Promise<void>>(async () => {})

  const [refreshingIds, setRefreshingIds] = useState<Set<number>>(new Set())
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const configsRef = useRef(configs)
  configsRef.current = configs

  useEffect(() => { loadConfigs(); return () => { if (pollRef.current) clearInterval(pollRef.current) } }, [])

  async function loadConfigs() {
    setLoading(true)
    try {
      const res = await api.getBMCConfigs()
      setConfigs(res.data)
      refreshAllStatus(res.data)
      if (pollRef.current) clearInterval(pollRef.current)
      pollRef.current = setInterval(() => refreshAllStatus(configsRef.current), 10000)
    } catch (err: any) {
      showError(err.message || t('bmc.loadFailed'))
    } finally {
      setLoading(false)
    }
  }

  async function refreshAllStatus(cfgs: BMCConfigType[]) {
    const ids = cfgs.map(c => c.id!).filter(Boolean)
    if (ids.length === 0) return
    try {
      const res = await api.bmcBatchStatus(ids)
      const map: Record<number, string> = {}
      for (const item of (res.data || [])) {
        if (item.config_id && item.state) map[item.config_id] = item.state
      }
      setStatusMap(prev => ({ ...prev, ...map }))
    } catch { /* silent */ }
  }

  async function refreshSingle(cfg: BMCConfigType) {
    if (!cfg.id) return
    setRefreshingIds(prev => new Set(prev).add(cfg.id!))
    try {
      const res = await api.refreshBMCConfig(cfg.id)
      if (res.data?.refreshed) loadConfigs()
    } catch { /* ignore */ }
    setRefreshingIds(prev => { const s = new Set(prev); s.delete(cfg.id!); return s })
  }

  function openEdit(cfg: BMCConfigType) {
    setEditConfig(cfg)
    setFormOpen(true)
  }

  function openCreate() {
    setEditConfig(null)
    setFormOpen(true)
  }

  function confirm(title: string, msg: string, action: () => Promise<void>) {
    setConfirmTitle(title)
    setConfirmMsg(msg)
    setConfirmAction(() => action)
    setConfirmOpen(true)
  }

  async function handleDelete(cfg: BMCConfigType) {
    confirm(t('bmc.delete'), `${t('bmc.confirmDelete')} ${cfg.name || cfg.host}`, async () => {
      try {
        await api.deleteBMCConfig(cfg.id!)
        success(t('bmc.deleteSuccess'))
        loadConfigs()
      } catch (err: any) {
        showError(err.message || t('bmc.deleteFailed'))
      }
    })
  }

  async function powerAction(cfg: BMCConfigType, action: string) {
    try {
      if (action === 'on') await api.bmcPowerOn(cfg.id!)
      else if (action === 'off') await api.bmcPowerOff(cfg.id!)
      else if (action === 'restart') await api.bmcRestart(cfg.id!)
      setTimeout(async () => {
        try {
          const res = await api.bmcPowerStatus(cfg.id!)
          setStatusMap(prev => ({ ...prev, [cfg.id!]: res.data.status }))
        } catch { /* */ }
      }, 2000)
    } catch (err: any) {
      showError(err.message || t('bmc.powerFailed'))
    }
  }

  async function handleSetBootDevice() {
    if (!bootDeviceCfg?.id) return
    setBootDeviceLoading(true)
    try {
      await api.bmcSetBootDevice(bootDeviceCfg.id, bootDevice)
      success(t('bmc.bootDeviceSuccess'))
      setBootDeviceCfg(null)
    } catch (err: any) {
      showError(err.message || t('bmc.bootDeviceFailed'))
    } finally {
      setBootDeviceLoading(false)
    }
  }

  async function batchAction(action: string) {
    const ids = Array.from(selectedIds)
    if (ids.length === 0) { showError(t('bmc.selectDevice')); return }
    try {
      if (action === 'on') await api.bmcBatchPowerOn(ids)
      else if (action === 'off') await api.bmcBatchPowerOff(ids)
      else if (action === 'restart') await api.bmcBatchRestart(ids)
      success(t('bmc.batchExecuted'))
      setTimeout(loadConfigs, 2000)
    } catch (err: any) {
      showError(err.message || t('bmc.batchFailed'))
    }
  }

  async function batchPXEBoot() {
    const ids = Array.from(selectedIds)
    if (ids.length === 0) { showError(t('bmc.selectDevice')); return }
    let successCount = 0
    for (const id of ids) {
      try {
        await api.bmcSetBootDevice(id, 'pxe')
        successCount++
      } catch { /* skip failed */ }
    }
    success(`${successCount}/${ids.length} ${t('bmc.bootDeviceSuccess')}`)
    loadConfigs()
  }

  async function handleCsvImport() {
    if (!csvText.trim()) { showError(t('bmc.pasteCsv')); return }
    setCsvImporting(true)
    try {
      const res = await api.bmcImportCSV(csvText)
      success(t('bmc.importResult', { success: res.data.success, failed: res.data.failed }))
      setCsvOpen(false)
      setCsvText('')
      loadConfigs()
    } catch (err: any) {
      showError(err.message || t('bmc.importFailed'))
    } finally {
      setCsvImporting(false)
    }
  }

  const toggleSelect = (id: number) => {
    setSelectedIds(prev => { const s = new Set(prev); if (s.has(id)) s.delete(id); else s.add(id); return s })
  }

  const selectAll = () => {
    if (selectedIds.size === configs.length) setSelectedIds(new Set())
    else setSelectedIds(new Set(configs.map(c => c.id!).filter(Boolean)))
  }

  const statusColor = (cfg: BMCConfigType) => {
    const s = statusMap[cfg.id!]
    if (s === 'on') return 'text-green-400'
    if (s === 'off') return 'text-red-400'
    return 'text-[var(--text-muted)]'
  }

  const statusDotColor = (cfg: BMCConfigType) => {
    const s = statusMap[cfg.id!]
    if (s === 'on') return 'green'
    if (s === 'off') return 'red'
    return 'gray'
  }

  const statusText = (cfg: BMCConfigType) => {
    const s = statusMap[cfg.id!]
    if (s === 'on') return t('bmc.powerOn')
    if (s === 'off') return t('bmc.powerOff')
    return t('common.unknown')
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center py-16">
        <div className="flex flex-col items-center gap-3">
          <div className="animate-spin w-6 h-6 border-2 border-blue-500 border-t-transparent rounded-full" />
          <span className="text-sm text-[var(--text-muted)]">{t('bmc.loading')}</span>
        </div>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-bold text-[var(--text-primary)]">{t('bmc.title')}</h1>
        <div className="flex items-center gap-2">
          <Button variant="ghost" size="sm" onClick={loadConfigs}>
            <RefreshCw size={14} />
          </Button>
          <Button variant="secondary" size="sm" onClick={() => setCsvOpen(true)}>
            <Upload size={14} />
            {t('bmc.importCsv')}
          </Button>
          <Button variant="primary" size="sm" onClick={openCreate}>
            <Plus size={14} />
            {t('bmc.newConfig')}
          </Button>
        </div>
      </div>

      {selectedIds.size > 0 && (
        <div className="flex items-center gap-2 px-4 py-2 rounded-lg bg-blue-500/5 border border-blue-500/10">
          <span className="text-xs text-[var(--text-muted)]">{t('bmc.selectedCount', { count: selectedIds.size })}</span>
          <div className="flex-1" />
          <Button variant="secondary" size="sm" onClick={() => batchAction('restart')}>
            <RotateCcw size={12} /> {t('bmc.batchRestart')}
          </Button>
          <Button variant="primary" size="sm" onClick={() => batchAction('on')}>
            <Power size={12} /> {t('bmc.batchPowerOn')}
          </Button>
          <Button variant="danger" size="sm" onClick={() => batchAction('off')}>
            <PowerOff size={12} /> {t('bmc.batchPowerOff')}
          </Button>
          <Button variant="secondary" size="sm" onClick={batchPXEBoot}>
            <Disc size={12} /> {t('bmc.bootPxe')}
          </Button>
        </div>
      )}

      <Card padding={false}>
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead>
              <tr className="border-b border-[var(--bg-border)]">
                <th className="px-4 py-3 w-10">
                  <input type="checkbox" checked={selectedIds.size === configs.length && configs.length > 0} onChange={selectAll} className="accent-blue-500 cursor-pointer" />
                </th>
                <th className="text-left px-4 py-3 text-xs font-semibold text-[var(--text-muted)] uppercase tracking-wider">{t('bmc.colName')}</th>
                <th className="text-left px-4 py-3 text-xs font-semibold text-[var(--text-muted)] uppercase tracking-wider">{t('bmc.colHost')}</th>
                <th className="text-left px-4 py-3 text-xs font-semibold text-[var(--text-muted)] uppercase tracking-wider">{t('bmc.colProtocol')}</th>
                <th className="text-left px-4 py-3 text-xs font-semibold text-[var(--text-muted)] uppercase tracking-wider">{t('bmc.colBrand')}</th>
                <th className="text-left px-4 py-3 text-xs font-semibold text-[var(--text-muted)] uppercase tracking-wider">{t('bmc.colSn')}</th>
                <th className="text-left px-4 py-3 text-xs font-semibold text-[var(--text-muted)] uppercase tracking-wider">{t('bmc.colBootMode')}</th>
                <th className="text-left px-4 py-3 text-xs font-semibold text-[var(--text-muted)] uppercase tracking-wider">{t('bmc.colNextBoot')}</th>
                <th className="text-left px-4 py-3 text-xs font-semibold text-[var(--text-muted)] uppercase tracking-wider">{t('bmc.colPower')}</th>
                <th className="text-right px-4 py-3 text-xs font-semibold text-[var(--text-muted)] uppercase tracking-wider">{t('bmc.colActions')}</th>
              </tr>
            </thead>
            <tbody>
              {configs.length === 0 ? (
                <tr>
                  <td colSpan={10} className="px-4 py-16 text-center text-sm text-[var(--text-muted)]">
                    {t('bmc.noConfigs')}
                  </td>
                </tr>
              ) : (
                configs.map(cfg => (
                  <tr key={cfg.id} className="border-b border-[var(--bg-border)] last:border-0 hover:bg-[var(--bg-card)]/50 transition-colors">
                    <td className="px-4 py-3">
                      <input type="checkbox" checked={selectedIds.has(cfg.id!)} onChange={() => toggleSelect(cfg.id!)} className="accent-blue-500 cursor-pointer" />
                    </td>
                    <td className="px-4 py-3">
                      <span className="text-sm font-medium">{cfg.name || '—'}</span>
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-2">
                        <code className="text-sm font-mono text-blue-400">{cfg.host}:{cfg.port}</code>
                        <a
                          href={bmcWebURL(cfg.host)}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="p-1 rounded hover:bg-blue-500/10 text-[var(--text-muted)] hover:text-blue-400 transition-colors"
                          title={t('bmc.openWeb')}
                        >
                          <ExternalLink size={12} />
                        </a>
                      </div>
                    </td>
                    <td className="px-4 py-3">
                      <span className="text-xs font-mono px-2 py-0.5 rounded-full bg-blue-500/5 text-blue-400 border border-blue-500/10">
                        {cfg.protocol?.toUpperCase()}
                      </span>
                    </td>
                    <td className="px-4 py-3">
                      <span className="text-xs text-[var(--text-secondary)]">
                        {[cfg.vendor, cfg.model].filter(Boolean).join(' / ') || '—'}
                      </span>
                    </td>
                    <td className="px-4 py-3">
                      <code className="text-xs font-mono text-[var(--text-muted)]">{cfg.serial || '—'}</code>
                    </td>
                    <td className="px-4 py-3">
                      <span className="text-xs px-2 py-0.5 rounded-full bg-purple-500/5 text-purple-400 border border-purple-500/10">
                        {cfg.boot_mode || 'auto'}
                      </span>
                    </td>
                    <td className="px-4 py-3">
                      <span className="text-xs">{cfg.next_boot_device ? t(BOOT_DEVICES.find(d => d.value === cfg.next_boot_device)?.key || '') || cfg.next_boot_device : '—'}</span>
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex items-center gap-1.5">
                        <span className={`inline-block w-2 h-2 rounded-full ${statusColor(cfg)}`} style={{
                          backgroundColor: statusDotColor(cfg) === 'green' ? '#22c55e' : statusDotColor(cfg) === 'red' ? '#ef4444' : '#6b7280'
                        }} />
                        <span className="text-xs">{statusText(cfg)}</span>
                      </div>
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex items-center justify-end gap-1">
                        <button onClick={() => powerAction(cfg, 'on')} className="p-1.5 rounded hover:bg-green-500/10 text-[var(--text-muted)] hover:text-green-400 transition-colors" title={t('bmc.powerOn')}>
                          <Play size={14} />
                        </button>
                        <button onClick={() => powerAction(cfg, 'off')} className="p-1.5 rounded hover:bg-red-500/10 text-[var(--text-muted)] hover:text-red-400 transition-colors" title={t('bmc.powerOff')}>
                          <Square size={14} />
                        </button>
                        <button onClick={() => powerAction(cfg, 'restart')} className="p-1.5 rounded hover:bg-yellow-500/10 text-[var(--text-muted)] hover:text-yellow-400 transition-colors" title={t('bmc.powerRestart')}>
                          <RotateCcw size={14} />
                        </button>
                        <button
                          onClick={() => { setBootDeviceCfg(cfg); setBootDevice('pxe') }}
                          className="p-1.5 rounded hover:bg-blue-500/10 text-[var(--text-muted)] hover:text-blue-400 transition-colors"
                          title={t('bmc.bootDevice')}
                        >
                          <Disc size={14} />
                        </button>
                        <button
                          onClick={() => refreshSingle(cfg)}
                          disabled={refreshingIds.has(cfg.id!)}
                          className="p-1.5 rounded hover:bg-blue-500/10 text-[var(--text-muted)] hover:text-blue-400 transition-colors disabled:opacity-40"
                          title={t('bmc.refreshDevice')}
                        >
                          <RefreshCw size={14} className={refreshingIds.has(cfg.id!) ? 'animate-spin' : ''} />
                        </button>
                        <button onClick={() => openEdit(cfg)} className="p-1.5 rounded hover:bg-blue-500/10 text-[var(--text-muted)] hover:text-blue-400 transition-colors" title={t('bmc.edit')}>✎</button>
                        <button onClick={() => handleDelete(cfg)} className="p-1.5 rounded hover:bg-red-500/10 text-[var(--text-muted)] hover:text-red-400 transition-colors" title={t('bmc.delete')}>
                          <Trash2 size={14} />
                        </button>
                      </div>
                    </td>
                  </tr>
                )))}
              </tbody>
            </table>
        </div>
      </Card>

      <BMCConfigForm
        open={formOpen}
        onClose={() => { setFormOpen(false); setEditConfig(null) }}
        onSaved={() => { success(t('bmc.saved')); loadConfigs() }}
        editConfig={editConfig}
      />

      <Modal
        open={!!bootDeviceCfg}
        onClose={() => setBootDeviceCfg(null)}
        title={`${t('bmc.setBootDevice')} — ${bootDeviceCfg?.host || ''}`}
        width="400px"
        footer={
          <>
            <Button variant="secondary" size="sm" onClick={() => setBootDeviceCfg(null)}>{t('bmc.cancel')}</Button>
            <Button variant="primary" size="sm" onClick={handleSetBootDevice} disabled={bootDeviceLoading}>
              {bootDeviceLoading ? t('bmc.setting') : t('bmc.confirm')}
            </Button>
          </>
        }
      >
        <select
          value={bootDevice}
          onChange={e => setBootDevice(e.target.value)}
          className="w-full bg-[var(--bg-input)] border border-[var(--bg-border)] rounded-lg px-3 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500"
        >
          {BOOT_DEVICES.map(d => (
            <option key={d.value} value={d.value}>{t(d.key)}</option>
          ))}
        </select>
      </Modal>

      <Modal
        open={csvOpen}
        onClose={() => setCsvOpen(false)}
        title={t('bmc.importCsv')}
        width="560px"
        footer={
          <>
            <Button variant="secondary" size="sm" onClick={() => setCsvOpen(false)}>{t('bmc.cancel')}</Button>
            <Button variant="primary" size="sm" onClick={handleCsvImport} disabled={csvImporting}>
              {csvImporting ? t('bmc.importing') : t('bmc.importCsv')}
            </Button>
          </>
        }
      >
        <div className="space-y-3">
          <p className="text-xs text-[var(--text-muted)]">
            {t('bmc.csvHint')}
          </p>
          <textarea
            value={csvText}
            onChange={e => setCsvText(e.target.value)}
            placeholder={`10.0.0.1,623,admin,admin,ipmi\n10.0.0.2,443,admin,admin,redfish`}
            rows={8}
            className="w-full bg-[var(--bg-input)] border border-[var(--bg-border)] rounded-lg px-3 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500 font-mono resize-none"
          />
        </div>
      </Modal>

      <Modal
        open={confirmOpen}
        onClose={() => setConfirmOpen(false)}
        title={confirmTitle}
        width="420px"
        footer={
          <>
            <Button variant="secondary" size="sm" onClick={() => setConfirmOpen(false)}>{t('bmc.cancel')}</Button>
            <Button variant="primary" size="sm" onClick={async () => { setConfirmOpen(false); await confirmAction() }}>{t('bmc.confirm')}</Button>
          </>
        }
      >
        <p className="text-sm text-[var(--text-muted)]">{confirmMsg}</p>
      </Modal>
    </div>
  )
}
