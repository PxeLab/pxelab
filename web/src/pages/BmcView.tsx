import { useState, useEffect, useRef } from 'react'
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
  { value: 'pxe', label: 'PXE 网络启动' },
  { value: 'disk', label: '本地硬盘' },
  { value: 'cdrom', label: '光驱' },
  { value: 'bios', label: 'BIOS 设置' },
]

export default function BmcView() {
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
      showError(err.message || '加载失败')
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
    confirm('删除配置', `确定要删除 ${cfg.name || cfg.host} 的 BMC 配置吗？`, async () => {
      try {
        await api.deleteBMCConfig(cfg.id!)
        success('已删除')
        loadConfigs()
      } catch (err: any) {
        showError(err.message || '删除失败')
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
      showError(err.message || '操作失败')
    }
  }

  async function handleSetBootDevice() {
    if (!bootDeviceCfg?.id) return
    setBootDeviceLoading(true)
    try {
      await api.bmcSetBootDevice(bootDeviceCfg.id, bootDevice)
      success('引导设备已设置')
      setBootDeviceCfg(null)
    } catch (err: any) {
      showError(err.message || '设置失败')
    } finally {
      setBootDeviceLoading(false)
    }
  }

  async function batchAction(action: string) {
    const ids = Array.from(selectedIds)
    if (ids.length === 0) { showError('请先选择设备'); return }
    try {
      if (action === 'on') await api.bmcBatchPowerOn(ids)
      else if (action === 'off') await api.bmcBatchPowerOff(ids)
      else if (action === 'restart') await api.bmcBatchRestart(ids)
      success('批量操作已执行')
      setTimeout(loadConfigs, 2000)
    } catch (err: any) {
      showError(err.message || '批量操作失败')
    }
  }

  async function handleCsvImport() {
    if (!csvText.trim()) { showError('请粘贴 CSV 内容'); return }
    setCsvImporting(true)
    try {
      const res = await api.bmcImportCSV(csvText)
      success(`导入完成：成功 ${res.data.success}，失败 ${res.data.failed}`)
      setCsvOpen(false)
      setCsvText('')
      loadConfigs()
    } catch (err: any) {
      showError(err.message || '导入失败')
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
    if (s === 'on') return '开机'
    if (s === 'off') return '关机'
    return '未知'
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center py-16">
        <div className="flex flex-col items-center gap-3">
          <div className="animate-spin w-6 h-6 border-2 border-blue-500 border-t-transparent rounded-full" />
          <span className="text-sm text-[var(--text-muted)]">加载中...</span>
        </div>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-bold text-[var(--text-primary)]">带外管理</h1>
        <div className="flex items-center gap-2">
          <Button variant="ghost" size="sm" onClick={loadConfigs}>
            <RefreshCw size={14} />
          </Button>
          <Button variant="secondary" size="sm" onClick={() => setCsvOpen(true)}>
            <Upload size={14} />
            导入 CSV
          </Button>
          <Button variant="primary" size="sm" onClick={openCreate}>
            <Plus size={14} />
            新建配置
          </Button>
        </div>
      </div>

      {selectedIds.size > 0 && (
        <div className="flex items-center gap-2 px-4 py-2 rounded-lg bg-blue-500/5 border border-blue-500/10">
          <span className="text-xs text-[var(--text-muted)]">已选择 {selectedIds.size} 项</span>
          <div className="flex-1" />
          <Button variant="primary" size="sm" onClick={() => batchAction('on')}>
            <Power size={12} /> 批量开机
          </Button>
          <Button variant="danger" size="sm" onClick={() => batchAction('off')}>
            <PowerOff size={12} /> 批量关机
          </Button>
          <Button variant="secondary" size="sm" onClick={() => batchAction('restart')}>
            <RotateCcw size={12} /> 批量重启
          </Button>
        </div>
      )}

      <Card padding={false}>
        {configs.length === 0 ? (
          <div className="flex items-center justify-center py-16">
            <div className="text-center">
              <p className="text-sm text-[var(--text-muted)] mb-4">暂无 BMC 配置</p>
              <Button variant="primary" size="sm" onClick={openCreate}>
                <Plus size={14} /> 新建配置
              </Button>
            </div>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full">
              <thead>
                <tr className="border-b border-[var(--bg-border)]">
                  <th className="px-4 py-3 w-10">
                    <input type="checkbox" checked={selectedIds.size === configs.length && configs.length > 0} onChange={selectAll} className="accent-blue-500 cursor-pointer" />
                  </th>
                  <th className="text-left px-4 py-3 text-xs font-semibold text-[var(--text-muted)] uppercase tracking-wider">设备名</th>
                  <th className="text-left px-4 py-3 text-xs font-semibold text-[var(--text-muted)] uppercase tracking-wider">BMC 地址</th>
                  <th className="text-left px-4 py-3 text-xs font-semibold text-[var(--text-muted)] uppercase tracking-wider">协议</th>
                  <th className="text-left px-4 py-3 text-xs font-semibold text-[var(--text-muted)] uppercase tracking-wider">品牌 / 型号</th>
                  <th className="text-left px-4 py-3 text-xs font-semibold text-[var(--text-muted)] uppercase tracking-wider">SN</th>
                  <th className="text-left px-4 py-3 text-xs font-semibold text-[var(--text-muted)] uppercase tracking-wider">电源状态</th>
                  <th className="text-right px-4 py-3 text-xs font-semibold text-[var(--text-muted)] uppercase tracking-wider">操作</th>
                </tr>
              </thead>
              <tbody>
                {configs.map(cfg => (
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
                          title="打开 BMC Web 界面"
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
                      <div className="flex items-center gap-1.5">
                        <span className={`inline-block w-2 h-2 rounded-full ${statusColor(cfg)}`} style={{
                          backgroundColor: statusDotColor(cfg) === 'green' ? '#22c55e' : statusDotColor(cfg) === 'red' ? '#ef4444' : '#6b7280'
                        }} />
                        <span className="text-xs">{statusText(cfg)}</span>
                      </div>
                    </td>
                    <td className="px-4 py-3">
                      <div className="flex items-center justify-end gap-1">
                        <button onClick={() => powerAction(cfg, 'on')} className="p-1.5 rounded hover:bg-green-500/10 text-[var(--text-muted)] hover:text-green-400 transition-colors" title="开机">
                          <Play size={14} />
                        </button>
                        <button onClick={() => powerAction(cfg, 'off')} className="p-1.5 rounded hover:bg-red-500/10 text-[var(--text-muted)] hover:text-red-400 transition-colors" title="关机">
                          <Square size={14} />
                        </button>
                        <button onClick={() => powerAction(cfg, 'restart')} className="p-1.5 rounded hover:bg-yellow-500/10 text-[var(--text-muted)] hover:text-yellow-400 transition-colors" title="重启">
                          <RotateCcw size={14} />
                        </button>
                        <button
                          onClick={() => { setBootDeviceCfg(cfg); setBootDevice('pxe') }}
                          className="p-1.5 rounded hover:bg-blue-500/10 text-[var(--text-muted)] hover:text-blue-400 transition-colors"
                          title="引导设备"
                        >
                          <Disc size={14} />
                        </button>
                        <button
                          onClick={() => refreshSingle(cfg)}
                          disabled={refreshingIds.has(cfg.id!)}
                          className="p-1.5 rounded hover:bg-blue-500/10 text-[var(--text-muted)] hover:text-blue-400 transition-colors disabled:opacity-40"
                          title="刷新设备信息"
                        >
                          <RefreshCw size={14} className={refreshingIds.has(cfg.id!) ? 'animate-spin' : ''} />
                        </button>
                        <button onClick={() => openEdit(cfg)} className="p-1.5 rounded hover:bg-blue-500/10 text-[var(--text-muted)] hover:text-blue-400 transition-colors" title="编辑">✎</button>
                        <button onClick={() => handleDelete(cfg)} className="p-1.5 rounded hover:bg-red-500/10 text-[var(--text-muted)] hover:text-red-400 transition-colors" title="删除">
                          <Trash2 size={14} />
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Card>

      <BMCConfigForm
        open={formOpen}
        onClose={() => { setFormOpen(false); setEditConfig(null) }}
        onSaved={() => { success('配置已保存'); loadConfigs() }}
        editConfig={editConfig}
      />

      <Modal
        open={!!bootDeviceCfg}
        onClose={() => setBootDeviceCfg(null)}
        title={`设置引导设备 — ${bootDeviceCfg?.host || ''}`}
        width="400px"
        footer={
          <>
            <Button variant="secondary" size="sm" onClick={() => setBootDeviceCfg(null)}>取消</Button>
            <Button variant="primary" size="sm" onClick={handleSetBootDevice} disabled={bootDeviceLoading}>
              {bootDeviceLoading ? '设置中...' : '确定'}
            </Button>
          </>
        }
      >
        <select
          value={bootDevice}
          onChange={e => setBootDevice(e.target.value)}
          className="w-full bg-[var(--bg-base)] border border-[var(--bg-border)] rounded-lg px-3 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500"
        >
          {BOOT_DEVICES.map(d => (
            <option key={d.value} value={d.value}>{d.label}</option>
          ))}
        </select>
      </Modal>

      <Modal
        open={csvOpen}
        onClose={() => setCsvOpen(false)}
        title="导入 CSV"
        width="560px"
        footer={
          <>
            <Button variant="secondary" size="sm" onClick={() => setCsvOpen(false)}>取消</Button>
            <Button variant="primary" size="sm" onClick={handleCsvImport} disabled={csvImporting}>
              {csvImporting ? '导入中...' : '导入'}
            </Button>
          </>
        }
      >
        <div className="space-y-3">
          <p className="text-xs text-[var(--text-muted)]">
            格式：每行一个配置，用逗号分隔：host,port,username,password,protocol
          </p>
          <textarea
            value={csvText}
            onChange={e => setCsvText(e.target.value)}
            placeholder={`10.0.0.1,623,admin,admin,ipmi\n10.0.0.2,443,admin,admin,redfish`}
            rows={8}
            className="w-full bg-[var(--bg-base)] border border-[var(--bg-border)] rounded-lg px-3 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500 font-mono resize-none"
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
            <Button variant="secondary" size="sm" onClick={() => setConfirmOpen(false)}>取消</Button>
            <Button variant="primary" size="sm" onClick={async () => { setConfirmOpen(false); await confirmAction() }}>确认</Button>
          </>
        }
      >
        <p className="text-sm text-[var(--text-muted)]">{confirmMsg}</p>
      </Modal>
    </div>
  )
}
