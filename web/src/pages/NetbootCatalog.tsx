import { useState, useEffect, useCallback } from 'react'
import { useSearchParams, useNavigate } from 'react-router-dom'
import { Search, RefreshCw, HardDrive, Globe, Monitor, Cpu, Package, Wrench, Server, CheckCircle2, XCircle, Settings2, Trash2, Plus } from 'lucide-react'
import { useToast } from '../components/ui/Toast'
import { Button } from '../components/ui/Button'
import { Modal } from '../components/ui/Modal'
import { Pagination } from '../components/ui/Pagination'
import { api, type NetbootDistro, type NetbootVersion, type NetbootOverlay } from '../api/client'
import { useUIConfig } from '../contexts/UIConfigContext'

type Tab = 'all' | string

const GROUP_LABELS: Record<string, string> = {
  linux: 'Linux',
  'linux-i386': 'Linux (32-bit)',
  'linux-arm64': 'Linux ARM64',
  bsd: 'BSD',
  live: 'Live CD',
  'live-arm': 'Live CD ARM64',
  tools: '工具',
  unix: 'Unix',
  dos: 'DOS',
  windows: 'Windows',
}

const GROUP_ICONS: Record<string, React.ReactNode> = {
  linux: <HardDrive size={16} />,
  'linux-i386': <Cpu size={16} />,
  'linux-arm64': <Cpu size={16} />,
  bsd: <Globe size={16} />,
  live: <Monitor size={16} />,
  'live-arm': <Monitor size={16} />,
  tools: <Wrench size={16} />,
  unix: <Server size={16} />,
  dos: <Monitor size={16} />,
  windows: <Monitor size={16} />,
}

const ANSWER_TYPES = [
  { value: '', label: '无' },
  { value: 'subiquity', label: 'Subiquity (Ubuntu 20.04+)' },
  { value: 'preseed', label: 'Preseed (Ubuntu/Debian)' },
  { value: 'kickstart', label: 'Kickstart (RHEL/CentOS/Rocky)' },
  { value: 'autoyast', label: 'AutoYaST (SUSE)' },
  { value: 'autounattend', label: 'Autounattend (Windows)' },
  { value: 'winpeshl', label: 'Winpeshl (Windows PE)' },
]

function archBadge(arch: string) {
  if (!arch) return null
  const colors: Record<string, string> = {
    amd64: 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300',
    x86_64: 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300',
    i386: 'bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-400',
    arm64: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300',
    aarch64: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300',
  }
  return (
    <span className={`inline-block text-[10px] px-1.5 py-0.5 rounded font-medium ${colors[arch] || 'bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-400'}`}>
      {arch}
    </span>
  )
}

function bootTypeTag(t: string | undefined) {
  if (!t) return null
  const colors: Record<string, string> = {
    kernel: 'bg-cyan-100 text-cyan-700 dark:bg-cyan-900/30 dark:text-cyan-300',
    memdisk: 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-300',
    memtest: 'bg-pink-100 text-pink-700 dark:bg-pink-900/30 dark:text-pink-300',
    sanboot: 'bg-indigo-100 text-indigo-700 dark:bg-indigo-900/30 dark:text-indigo-300',
    wimboot: 'bg-violet-100 text-violet-700 dark:bg-violet-900/30 dark:text-violet-300',
  }
  return (
    <span className={`inline-block text-[10px] px-1.5 py-0.5 rounded font-mono font-medium ${colors[t] || 'bg-gray-100 text-gray-600'}`}>
      {t}
    </span>
  )
}

function installTypeTag(t: string | undefined) {
  if (!t) return null
  const colors: Record<string, string> = {
    legacy: 'bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-400',
    subiquity: 'bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-300',
    live: 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-300',
    direct: 'bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-300',
    bsd: 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300',
  }
  return (
    <span className={`inline-block text-[10px] px-1.5 py-0.5 rounded font-medium ${colors[t] || ''}`}>
      {t}
    </span>
  )
}

export default function NetbootCatalog() {
  const [distros, setDistros] = useState<NetbootDistro[]>([])
  const [fileStatuses, setFileStatuses] = useState<Record<string, boolean>>({})
  const [overlays, setOverlays] = useState<Record<string, NetbootOverlay>>({})
  const [loading, setLoading] = useState(true)
  const [search, setSearch] = useState('')
  const [searchParams, setSearchParams] = useSearchParams()
  const activeTab = (searchParams.get('tab') as Tab) || 'all'
  // Overlay modal state
  const [overlayDistro, setOverlayDistro] = useState<NetbootDistro | null>(null)
  const [editingOverlay, setEditingOverlay] = useState<NetbootOverlay | null>(null)
  const [overlaySaving, setOverlaySaving] = useState(false)
  // Create profile dialog
  const [createTarget, setCreateTarget] = useState<{ distro: NetbootDistro; version: NetbootVersion } | null>(null)
  const [createName, setCreateName] = useState('')
  const [createLoading, setCreateLoading] = useState(false)
  // Pagination
  const [page, setPage] = useState(1)

  const navigate = useNavigate()
  const { success, error: toastError } = useToast()
  const { pageSize } = useUIConfig()

  const loadData = useCallback(async () => {
    setLoading(true)
    try {
      const [catRes, fileRes, overlayRes] = await Promise.all([
        api.getNetbootCatalog(),
        api.getNetbootFileStatus(),
        api.getNetbootOverlays(),
      ])
      setDistros(catRes.data?.distros || [])
      const statusMap: Record<string, boolean> = {}
      fileRes.data?.forEach(s => {
        statusMap[`${s.distro}/${s.version}/${s.arch}`] = s.has_local
      })
      setFileStatuses(statusMap)
      const overlayMap: Record<string, NetbootOverlay> = {}
      overlayRes.data?.overlays?.forEach(o => {
        overlayMap[o.distro_name] = o
      })
      setOverlays(overlayMap)
    } catch { /* ignore */ }
    setLoading(false)
  }, [])

  useEffect(() => { loadData() }, [loadData])
  useEffect(() => { setPage(1) }, [search, activeTab])

  const groups = [...new Set(distros.filter(d => d.enabled).map(d => d.menu_group))].sort()
  const tabs: { key: Tab; label: string; filter: (d: NetbootDistro) => boolean }[] = [
    { key: 'all', label: '全部', filter: () => true },
    ...groups.map(g => ({
      key: g,
      label: GROUP_LABELS[g] || g,
      filter: (d: NetbootDistro) => d.menu_group === g,
    })),
  ]

  function flatRows() {
    const tab = tabs.find(t => t.key === activeTab)
    const rows: { distro: NetbootDistro; version: NetbootVersion }[] = []
    for (const d of distros) {
      if (!d.enabled) continue
      if (tab && !tab.filter(d)) continue
      for (const v of d.versions) {
        if (!v.enabled) continue
        if (search && !d.name.toLowerCase().includes(search.toLowerCase()) && !v.name.toLowerCase().includes(search.toLowerCase())) continue
        rows.push({ distro: d, version: v })
      }
    }
    return rows
  }

  // ── Overlay modal ──
  function openOverlay(distro: NetbootDistro) {
    setOverlayDistro(distro)
    const ov = overlays[distro.name]
    setEditingOverlay(ov ? JSON.parse(JSON.stringify(ov)) : {
      distro_name: distro.name,
      enabled: true,
      mirror: '',
      local_base: '',
      kernel_params: '',
      version_overrides: [],
    })
  }

  async function saveOverlay() {
    if (!editingOverlay || !overlayDistro) return
    setOverlaySaving(true)
    try {
      const res = await api.upsertNetbootOverlay(overlayDistro.name, editingOverlay)
      setOverlays(prev => ({ ...prev, [overlayDistro.name]: res.data }))
      await loadData()
    } catch { /* ignore */ }
    setOverlaySaving(false)
  }

  function addVersionOverride(codename: string, arch: string) {
    if (!editingOverlay) return
    const vo = [...(editingOverlay.version_overrides || [])]
    vo.push({ codename, arch })
    setEditingOverlay({ ...editingOverlay, version_overrides: vo })
  }

  function removeVersionOverride(index: number) {
    if (!editingOverlay) return
    const vo = [...(editingOverlay.version_overrides || [])]
    vo.splice(index, 1)
    setEditingOverlay({ ...editingOverlay, version_overrides: vo })
  }

  function renderOverlayBody() {
    const distro = overlayDistro
    if (!distro || !editingOverlay) return null
    const ov = editingOverlay

    return (
      <div className="space-y-4">
        <div className="flex items-center justify-between">
          <label className="text-sm font-medium text-[var(--text-primary)]">启用覆盖配置</label>
          <button
            onClick={() => setEditingOverlay({ ...ov, enabled: !ov.enabled })}
            className={`relative w-10 h-5 rounded-full transition-colors ${ov.enabled ? 'bg-blue-500' : 'bg-gray-500/30'}`}
          >
            <span className={`absolute top-0.5 w-4 h-4 rounded-full bg-white transition-transform ${ov.enabled ? 'translate-x-5' : 'translate-x-0.5'}`} />
          </button>
        </div>

        {ov.enabled && (
          <>
            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">镜像源 URL</label>
                <input type="text" value={ov.mirror || ''}
                  onChange={e => setEditingOverlay({ ...ov, mirror: e.target.value })}
                  placeholder={distro.mirror || ''}
                  className="w-full px-3 py-2 text-xs rounded-lg border border-[var(--bg-border)] bg-[var(--bg-input)] text-[var(--text-primary)]" />
              </div>
              <div>
                <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">本地路径</label>
                <input type="text" value={ov.local_base || ''}
                  onChange={e => setEditingOverlay({ ...ov, local_base: e.target.value })}
                  className="w-full px-3 py-2 text-xs rounded-lg border border-[var(--bg-border)] bg-[var(--bg-input)] text-[var(--text-primary)]" />
              </div>
            </div>
            <div>
              <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">全局 Kernel 参数</label>
              <input type="text" value={ov.kernel_params || ''}
                onChange={e => setEditingOverlay({ ...ov, kernel_params: e.target.value })}
                className="w-full px-3 py-2 text-xs rounded-lg border border-[var(--bg-border)] bg-[var(--bg-input)] text-[var(--text-primary)]" />
            </div>

            <div>
              <div className="flex items-center justify-between mb-2">
                <label className="text-xs font-medium text-[var(--text-muted)]">版本级覆盖</label>
              </div>
              {distro.versions.filter(v => v.enabled).map((ver) => {
                const existingVo = (ov.version_overrides || []).find(
                  vo => vo.codename === ver.codename && vo.arch === ver.arch
                )
                const voIdx = existingVo ? (ov.version_overrides || []).indexOf(existingVo) : -1
                const isExpanded = voIdx >= 0

                return (
                  <div key={`${ver.codename}-${ver.arch}`} className="mb-2 p-3 rounded-lg bg-[var(--bg-muted)]/30 border border-[var(--bg-border)]">
                    <div className="flex items-center justify-between">
                      <span className="text-xs font-medium text-[var(--text-primary)]">{ver.name} ({ver.arch})</span>
                      {isExpanded ? (
                        <button onClick={() => removeVersionOverride(voIdx)}
                          className="text-[10px] text-red-400 hover:text-red-300">移除覆盖</button>
                      ) : (
                        <button onClick={() => addVersionOverride(ver.codename, ver.arch)}
                          className="text-[10px] text-blue-400 hover:text-blue-300">+ 添加覆盖</button>
                      )}
                    </div>
                    {isExpanded && existingVo && (() => {
                      const idx = voIdx
                      const current = (ov.version_overrides || [])[idx]
                      return (
                        <div className="mt-2 space-y-2">
                          <div className="grid grid-cols-2 gap-2">
                            <div>
                              <label className="block text-[10px] text-[var(--text-muted)]">Kernel URL</label>
                              <input type="text" value={current.remote_kernel || ''}
                                onChange={e => {
                                  const vo = [...(ov.version_overrides || [])]
                                  vo[idx] = { ...vo[idx], remote_kernel: e.target.value }
                                  setEditingOverlay({ ...ov, version_overrides: vo })
                                }}
                                className="w-full px-2 py-1 text-[11px] rounded border border-[var(--bg-border)] bg-[var(--bg-input)] text-[var(--text-primary)]" />
                            </div>
                            <div>
                              <label className="block text-[10px] text-[var(--text-muted)]">Initrd URL</label>
                              <input type="text" value={current.remote_initrd || ''}
                                onChange={e => {
                                  const vo = [...(ov.version_overrides || [])]
                                  vo[idx] = { ...vo[idx], remote_initrd: e.target.value }
                                  setEditingOverlay({ ...ov, version_overrides: vo })
                                }}
                                className="w-full px-2 py-1 text-[11px] rounded border border-[var(--bg-border)] bg-[var(--bg-input)] text-[var(--text-primary)]" />
                            </div>
                          </div>
                          <div>
                            <label className="block text-[10px] text-[var(--text-muted)]">Cmdline</label>
                            <input type="text" value={current.cmdline || ''}
                              onChange={e => {
                                const vo = [...(ov.version_overrides || [])]
                                vo[idx] = { ...vo[idx], cmdline: e.target.value }
                                setEditingOverlay({ ...ov, version_overrides: vo })
                              }}
                              className="w-full px-2 py-1 text-[11px] rounded border border-[var(--bg-border)] bg-[var(--bg-input)] text-[var(--text-primary)]" />
                          </div>
                          <div className="grid grid-cols-2 gap-2">
                            <div>
                              <label className="block text-[10px] text-[var(--text-muted)]">Answer Param</label>
                              <input type="text" value={current.answer_param || ''}
                                onChange={e => {
                                  const vo = [...(ov.version_overrides || [])]
                                  vo[idx] = { ...vo[idx], answer_param: e.target.value }
                                  setEditingOverlay({ ...ov, version_overrides: vo })
                                }}
                                placeholder="autoinstall ds=nocloud-net;s={{.AnswerURL}}"
                                className="w-full px-2 py-1 text-[11px] rounded border border-[var(--bg-border)] bg-[var(--bg-input)] text-[var(--text-primary)]" />
                            </div>
                            <div>
                              <label className="block text-[10px] text-[var(--text-muted)]">Answer 类型</label>
                              <select value={current.answer_type || ''}
                                onChange={e => {
                                  const vo = [...(ov.version_overrides || [])]
                                  vo[idx] = { ...vo[idx], answer_type: e.target.value }
                                  setEditingOverlay({ ...ov, version_overrides: vo })
                                }}
                                className="w-full px-2 py-1 text-[11px] rounded border border-[var(--bg-border)] bg-[var(--bg-input)] text-[var(--text-primary)]">
                                {ANSWER_TYPES.map(at => (
                                  <option key={at.value} value={at.value}>{at.label}</option>
                                ))}
                              </select>
                            </div>
                          </div>
                        </div>
                      )
                    })()}
                  </div>
                )
              })}
            </div>
          </>
        )}
      </div>
    )
  }

  const rows = flatRows()
  const pagedRows = rows.slice((page - 1) * pageSize, page * pageSize)

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-lg font-bold text-[var(--text-primary)]">OS 安装目录</h1>
        <Button variant="secondary" size="sm" onClick={loadData} disabled={loading}>
          <RefreshCw size={14} /> 刷新
        </Button>
      </div>

      <div className="flex items-center gap-4 mb-4">
        <div className="relative flex-1 max-w-sm">
          <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-[var(--text-muted)]" />
          <input
            type="text"
            placeholder="搜索发行版或版本..."
            value={search}
            onChange={e => setSearch(e.target.value)}
            className="w-full pl-8 pr-4 py-1.5 text-sm border border-[var(--border-color)] rounded bg-[var(--bg-primary)]"
          />
        </div>
      </div>
      <div className="flex gap-1 flex-wrap border-b border-[var(--bg-border)] mb-6">
        {tabs.map(tab => (
          <button
            key={tab.key}
            onClick={() => setSearchParams({ tab: tab.key })}
            className={`px-3 py-2.5 text-sm font-medium border-b-2 transition-colors -mb-[1px] ${
              activeTab === tab.key
                ? 'border-blue-500 text-blue-400'
                : 'border-transparent text-[var(--text-muted)] hover:text-[var(--text-primary)]'
            }`}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {loading ? (
        <div className="flex items-center justify-center py-16">
          <div className="animate-spin w-6 h-6 border-2 border-blue-500 border-t-transparent rounded-full" />
        </div>
      ) : rows.length === 0 ? (
        <div className="text-center py-16 text-[var(--text-muted)]">
          没有匹配的版本
        </div>
      ) : (
        <div>
          <div className="overflow-x-auto rounded-xl border border-[var(--bg-border)]">
            <table className="w-full text-sm">
              <thead>
                <tr className="bg-[var(--bg-secondary)]">
                  <th className="text-left px-3 py-2.5 font-medium text-[var(--text-muted)] text-[11px]">发行版</th>
                  <th className="text-left px-3 py-2.5 font-medium text-[var(--text-muted)] text-[11px]">版本</th>
                  <th className="text-left px-3 py-2.5 font-medium text-[var(--text-muted)] text-[11px]">架构</th>
                  <th className="text-left px-3 py-2.5 font-medium text-[var(--text-muted)] text-[11px]">引导</th>
                  <th className="text-left px-3 py-2.5 font-medium text-[var(--text-muted)] text-[11px]">安装</th>
                  <th className="text-left px-3 py-2.5 font-medium text-[var(--text-muted)] text-[11px]">Kernel</th>
                  <th className="text-left px-3 py-2.5 font-medium text-[var(--text-muted)] text-[11px]">Initrd</th>
                  <th className="text-left px-3 py-2.5 font-medium text-[var(--text-muted)] text-[11px]">Cmdline</th>
                  <th className="text-center px-3 py-2.5 font-medium text-[var(--text-muted)] text-[11px]">
                    <span title="Kernel/Initrd 文件是否已缓存到本地服务器">缓存</span>
                  </th>
                  <th className="text-center px-3 py-2.5 font-medium text-[var(--text-muted)] text-[11px]">操作</th>
                  <th className="text-center px-3 py-2.5 font-medium text-[var(--text-muted)] text-[11px]">覆盖</th>
                </tr>
              </thead>
              <tbody>
                {pagedRows.map(({ distro, version }) => {
                const statusKey = `${distro.name}/${version.name}/${version.arch}`
                const hasLocal = fileStatuses[statusKey] || !!version.local
                const kernelVal = version.remote?.kernel || version.local?.kernel || ''
                const initrdVal = version.remote?.initrd || version.local?.initrd || ''
                const hasOverlay = !!overlays[distro.name]
                return (
                  <tr key={`${distro.name}-${version.codename}-${version.arch}`} className="border-t border-[var(--bg-border)] hover:bg-[var(--bg-hover)]/30">
                    <td className="px-3 py-2.5 whitespace-nowrap">
                      <div className="flex items-center gap-2">
                        {GROUP_ICONS[distro.menu_group] || <Package size={16} />}
                        <span className="text-xs font-medium text-[var(--text-primary)]">{distro.name}</span>
                      </div>
                    </td>
                    <td className="px-3 py-2.5 text-[var(--text-primary)] text-xs whitespace-nowrap">{version.name}</td>
                    <td className="px-3 py-2.5 whitespace-nowrap">{archBadge(version.arch)}</td>
                    <td className="px-3 py-2.5 whitespace-nowrap">{bootTypeTag(version.boot_type)}</td>
                    <td className="px-3 py-2.5 whitespace-nowrap">{installTypeTag(version.install_type) || <span className="text-[10px] text-[var(--text-muted)]">—</span>}</td>
                    <td className="px-3 py-2.5 max-w-[200px]">
                      <span className="font-mono text-[10px] text-[var(--text-primary)] block truncate" title={kernelVal}>{kernelVal || <span className="text-[var(--text-muted)]">—</span>}</span>
                    </td>
                    <td className="px-3 py-2.5 max-w-[200px]">
                      <span className="font-mono text-[10px] text-[var(--text-primary)] block truncate" title={initrdVal}>{initrdVal || <span className="text-[var(--text-muted)]">—</span>}</span>
                    </td>
                    <td className="px-3 py-2.5 max-w-[200px]">
                      <span className="font-mono text-[10px] text-[var(--text-primary)] block truncate" title={version.cmdline || ''}>{version.cmdline || <span className="text-[var(--text-muted)]">—</span>}</span>
                    </td>
                    <td className="px-3 py-2.5 text-center">
                      {hasLocal
                        ? <CheckCircle2 size={14} className="text-green-500 inline" />
                        : <XCircle size={14} className="text-red-400/60 inline" />
                      }
                    </td>
                    <td className="px-3 py-2.5 text-center">
                      <button
                        onClick={() => {
                          setCreateTarget({ distro, version })
                          setCreateName(distro.name + ' ' + version.name)
                        }}
                        className="inline-flex items-center gap-1 px-2 py-1 text-[10px] font-medium rounded transition-colors bg-emerald-500/15 text-emerald-400 hover:bg-emerald-500/25"
                      >
                        <Plus size={12} /> 创建
                      </button>
                    </td>
                    <td className="px-3 py-2.5 text-center">
                      <button
                        onClick={() => openOverlay(distro)}
                        className={`inline-flex items-center gap-1 px-2 py-1 text-[10px] font-medium rounded transition-colors ${
                          hasOverlay
                            ? 'bg-blue-500/15 text-blue-400 hover:bg-blue-500/25'
                            : 'text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-hover)]'
                        }`}
                      >
                        <Settings2 size={12} />
                      </button>
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
          </div>
          <Pagination page={page} total={rows.length} size={pageSize} onChange={setPage} />
        </div>
      )}

      {/* Overlay Modal */}
      <Modal
        open={!!overlayDistro}
        onClose={() => { setOverlayDistro(null); setEditingOverlay(null) }}
        title={`覆盖配置 — ${overlayDistro?.name || ''}`}
        width="640px"
        footer={
          <>
            {overlays[overlayDistro?.name || '']?.id && (
              <button
                onClick={async () => {
                  if (!overlayDistro) return
                  await api.deleteNetbootOverlay(overlayDistro.name)
                  setOverlays(prev => {
                    const next = { ...prev }
                    delete next[overlayDistro.name]
                    return next
                  })
                  setOverlayDistro(null); setEditingOverlay(null)
                }}
                className="px-3 py-1.5 text-xs font-medium rounded-lg bg-red-500/15 text-red-400 hover:bg-red-500/25 transition-colors"
              >
                <Trash2 size={12} className="inline mr-1" />删除覆盖
              </button>
            )}
            <button
              onClick={() => { setOverlayDistro(null); setEditingOverlay(null) }}
              className="px-3 py-1.5 text-xs font-medium rounded-lg bg-[var(--bg-hover)] text-[var(--text-secondary)] transition-colors"
            >
              取消
            </button>
            <button
              onClick={saveOverlay}
              disabled={overlaySaving}
              className="px-3 py-1.5 text-xs font-medium rounded-lg bg-blue-500/15 text-blue-400 hover:bg-blue-500/25 disabled:opacity-40 transition-colors"
            >
              {overlaySaving ? '保存中...' : '保存覆盖'}
            </button>
          </>
        }
      >
        {renderOverlayBody()}
      </Modal>

      {/* Create Profile Dialog */}
      <Modal
        open={!!createTarget}
        onClose={() => setCreateTarget(null)}
        title="创建 Profile"
        width="400px"
        footer={
          <>
            <button
              onClick={() => setCreateTarget(null)}
              className="px-3 py-1.5 text-xs font-medium rounded-lg bg-[var(--bg-hover)] text-[var(--text-secondary)] transition-colors"
            >
              取消
            </button>
            <button
              onClick={async () => {
                if (!createTarget) return
                setCreateLoading(true)
                try {
                  await api.createProfileFromNetboot({
                    distro_name: createTarget.distro.name,
                    version_codename: createTarget.version.codename,
                    arch: createTarget.version.arch,
                    profile_name: createName,
                  })
                  success('Profile 创建成功')
                  setCreateTarget(null)
                  navigate('/profiles')
                } catch {
                  toastError('创建 Profile 失败')
                }
                setCreateLoading(false)
              }}
              disabled={createLoading || !createName.trim()}
              className="px-3 py-1.5 text-xs font-medium rounded-lg bg-blue-500/15 text-blue-400 hover:bg-blue-500/25 disabled:opacity-40 transition-colors"
            >
              {createLoading ? '创建中...' : '确定创建'}
            </button>
          </>
        }
      >
        <div className="space-y-4">
          <div>
            <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">Profile 名称</label>
            <input
              type="text"
              value={createName}
              onChange={e => setCreateName(e.target.value)}
              placeholder="输入 Profile 名称"
              className="w-full px-3 py-2 text-sm rounded-lg border border-[var(--bg-border)] bg-[var(--bg-input)] text-[var(--text-primary)]"
              autoFocus
            />
          </div>
          {createTarget && (
            <div className="text-xs text-[var(--text-muted)] space-y-1">
              <p>发行版: <span className="text-[var(--text-primary)]">{createTarget.distro.name}</span></p>
              <p>版本: <span className="text-[var(--text-primary)]">{createTarget.version.name} ({createTarget.version.codename})</span></p>
              <p>架构: <span className="text-[var(--text-primary)]">{createTarget.version.arch}</span></p>
              {createTarget.version.remote?.kernel && (
                <p className="truncate" title={createTarget.version.remote.kernel}>
                  Kernel: <span className="text-[var(--text-primary)]">{createTarget.version.remote.kernel}</span>
                </p>
              )}
            </div>
          )}
        </div>
      </Modal>
    </div>
  )
}
