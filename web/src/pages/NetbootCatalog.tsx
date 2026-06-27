import { useState, useEffect, useCallback } from 'react'
import { Search, HardDrive, Globe, Monitor, Cpu, Package, Wrench, Server, CheckCircle2, XCircle, ChevronRight, ChevronDown, Settings2 } from 'lucide-react'
import { Card } from '../components/ui/Card'
import { api, type NetbootDistro, type NetbootOverlay } from '../api/client'

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

export default function NetbootCatalog() {
  const [distros, setDistros] = useState<NetbootDistro[]>([])
  const [fileStatuses, setFileStatuses] = useState<Record<string, boolean>>({})
  const [overlays, setOverlays] = useState<Record<string, NetbootOverlay>>({})
  const [loading, setLoading] = useState(true)
  const [search, setSearch] = useState('')
  const [activeTab, setActiveTab] = useState<Tab>('all')
  const [expanded, setExpanded] = useState<Record<string, boolean>>({})
  const [detailTab, setDetailTab] = useState<Record<string, 'versions' | 'overlay'>>({})

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

  const groups = [...new Set(distros.filter(d => d.enabled).map(d => d.menu_group))].sort()
  const tabs: { key: Tab; label: string; filter: (d: NetbootDistro) => boolean }[] = [
    { key: 'all', label: '全部', filter: () => true },
    ...groups.map(g => ({
      key: g,
      label: GROUP_LABELS[g] || g,
      filter: (d: NetbootDistro) => d.menu_group === g,
    })),
  ]

  function filteredDistros() {
    const tab = tabs.find(t => t.key === activeTab)
    return distros.filter(d => {
      if (tab && !tab.filter(d)) return false
      if (search && !d.name.toLowerCase().includes(search.toLowerCase())) return false
      return d.enabled
    })
  }

  function toggleExpand(name: string) {
    setExpanded(prev => ({ ...prev, [name]: !prev[name] }))
  }

  function setDetailView(name: string, view: 'versions' | 'overlay') {
    setDetailTab(prev => ({ ...prev, [name]: view }))
  }

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

  // ── Overlay editing ──
  const [editingOverlay, setEditingOverlay] = useState<NetbootOverlay | null>(null)
  const [overlaySaving, setOverlaySaving] = useState<string | null>(null)

  function initOverlay(distro: string): NetbootOverlay {
    const existing = overlays[distro]
    if (existing) return JSON.parse(JSON.stringify(existing))
    return {
      distro_name: distro,
      enabled: true,
      mirror: '',
      local_base: '',
      kernel_params: '',
      version_overrides: [],
    }
  }

  async function saveOverlay(distro: string) {
    if (!editingOverlay) return
    setOverlaySaving(distro)
    try {
      const res = await api.upsertNetbootOverlay(distro, editingOverlay)
      setOverlays(prev => ({ ...prev, [distro]: res.data }))
      await loadData()
    } catch { /* ignore */ }
    setOverlaySaving(null)
  }

  function addVersionOverride(codename: string, arch: string) {
    if (!editingOverlay) return
    const vo = [...(editingOverlay.version_overrides || [])]
    vo.push({ codename, arch })
    setEditingOverlay({ ...editingOverlay, version_overrides: vo })
  }

  function removeVersionOverride(_distro: string, index: number) {
    if (!editingOverlay) return
    const vo = [...(editingOverlay.version_overrides || [])]
    vo.splice(index, 1)
    setEditingOverlay({ ...editingOverlay, version_overrides: vo })
  }

  /** Render overlay editor for a distro */
  function renderOverlayEditor(distro: NetbootDistro) {
    const ov = editingOverlay?.distro_name === distro.name
      ? editingOverlay
      : overlays[distro.name]
    const hasOverlay = !!ov?.id
    const isEnabled = ov?.enabled ?? false
    const isSaving = overlaySaving === distro.name

    return (
      <div className="p-4 space-y-4">
        {/* Enable toggle */}
        <div className="flex items-center justify-between">
          <label className="text-sm font-medium text-[var(--text-primary)]">启用覆盖配置</label>
          <button
            onClick={() => {
              const current = editingOverlay || initOverlay(distro.name)
              setEditingOverlay({ ...current, enabled: !current.enabled })
            }}
            className={`relative w-10 h-5 rounded-full transition-colors ${isEnabled ? 'bg-blue-500' : 'bg-gray-500/30'}`}
          >
            <span className={`absolute top-0.5 w-4 h-4 rounded-full bg-white transition-transform ${isEnabled ? 'translate-x-5' : 'translate-x-0.5'}`} />
          </button>
        </div>

        {isEnabled && (
          <>
            {/* Global fields */}
            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">镜像源 URL</label>
                <input type="text" value={editingOverlay?.mirror || ''}
                  onChange={e => setEditingOverlay(prev => prev ? { ...prev, mirror: e.target.value } : null)}
                  placeholder={distro.mirror || ''}
                  className="w-full px-3 py-2 text-xs rounded-lg border border-[var(--bg-border)] bg-[var(--bg-input)] text-[var(--text-primary)]" />
              </div>
              <div>
                <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">本地路径</label>
                <input type="text" value={editingOverlay?.local_base || ''}
                  onChange={e => setEditingOverlay(prev => prev ? { ...prev, local_base: e.target.value } : null)}
                  className="w-full px-3 py-2 text-xs rounded-lg border border-[var(--bg-border)] bg-[var(--bg-input)] text-[var(--text-primary)]" />
              </div>
            </div>
            <div>
              <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">全局 Kernel 参数</label>
              <input type="text" value={editingOverlay?.kernel_params || ''}
                onChange={e => setEditingOverlay(prev => prev ? { ...prev, kernel_params: e.target.value } : null)}
                className="w-full px-3 py-2 text-xs rounded-lg border border-[var(--bg-border)] bg-[var(--bg-input)] text-[var(--text-primary)]" />
            </div>

            {/* Version-level overrides */}
            <div>
              <div className="flex items-center justify-between mb-2">
                <label className="text-xs font-medium text-[var(--text-muted)]">版本级覆盖</label>
              </div>
              {distro.versions.filter(v => v.enabled).map((ver) => {
                const existingVo = (editingOverlay?.version_overrides || []).find(
                  vo => vo.codename === ver.codename && vo.arch === ver.arch
                )
                const voIdx = existingVo
                  ? (editingOverlay?.version_overrides || []).indexOf(existingVo)
                  : -1
                const expanded = voIdx >= 0

                return (
                  <div key={`${ver.codename}-${ver.arch}`} className="mb-2 p-3 rounded-lg bg-[var(--bg-muted)]/30 border border-[var(--bg-border)]">
                    <div className="flex items-center justify-between">
                      <span className="text-xs font-medium text-[var(--text-primary)]">{ver.name} ({ver.arch})</span>
                      {expanded ? (
                        <button onClick={() => removeVersionOverride(distro.name, voIdx)}
                          className="text-[10px] text-red-400 hover:text-red-300">移除覆盖</button>
                      ) : (
                        <button onClick={() => addVersionOverride(ver.codename, ver.arch)}
                          className="text-[10px] text-blue-400 hover:text-blue-300">+ 添加覆盖</button>
                      )}
                    </div>
                    {expanded && existingVo && (() => {
                      const ovIdx = voIdx
                      const current = (editingOverlay?.version_overrides || [])[ovIdx]
                      return (
                        <div className="mt-2 space-y-2">
                          <div className="grid grid-cols-2 gap-2">
                            <div>
                              <label className="block text-[10px] text-[var(--text-muted)]">Kernel URL</label>
                              <input type="text" value={current.remote_kernel || ''}
                                onChange={e => {
                                  const vo = [...(editingOverlay?.version_overrides || [])]
                                  vo[ovIdx] = { ...vo[ovIdx], remote_kernel: e.target.value }
                                  setEditingOverlay(prev => prev ? { ...prev, version_overrides: vo } : null)
                                }}
                                className="w-full px-2 py-1 text-[11px] rounded border border-[var(--bg-border)] bg-[var(--bg-input)] text-[var(--text-primary)]" />
                            </div>
                            <div>
                              <label className="block text-[10px] text-[var(--text-muted)]">Initrd URL</label>
                              <input type="text" value={current.remote_initrd || ''}
                                onChange={e => {
                                  const vo = [...(editingOverlay?.version_overrides || [])]
                                  vo[ovIdx] = { ...vo[ovIdx], remote_initrd: e.target.value }
                                  setEditingOverlay(prev => prev ? { ...prev, version_overrides: vo } : null)
                                }}
                                className="w-full px-2 py-1 text-[11px] rounded border border-[var(--bg-border)] bg-[var(--bg-input)] text-[var(--text-primary)]" />
                            </div>
                          </div>
                          <div>
                            <label className="block text-[10px] text-[var(--text-muted)]">Cmdline</label>
                            <input type="text" value={current.cmdline || ''}
                              onChange={e => {
                                const vo = [...(editingOverlay?.version_overrides || [])]
                                vo[ovIdx] = { ...vo[ovIdx], cmdline: e.target.value }
                                setEditingOverlay(prev => prev ? { ...prev, version_overrides: vo } : null)
                              }}
                              className="w-full px-2 py-1 text-[11px] rounded border border-[var(--bg-border)] bg-[var(--bg-input)] text-[var(--text-primary)]" />
                          </div>
                          <div className="grid grid-cols-2 gap-2">
                            <div>
                              <label className="block text-[10px] text-[var(--text-muted)]">Answer Param</label>
                              <input type="text" value={current.answer_param || ''}
                                onChange={e => {
                                  const vo = [...(editingOverlay?.version_overrides || [])]
                                  vo[ovIdx] = { ...vo[ovIdx], answer_param: e.target.value }
                                  setEditingOverlay(prev => prev ? { ...prev, version_overrides: vo } : null)
                                }}
                                placeholder="autoinstall ds=nocloud-net;s={{.AnswerURL}}"
                                className="w-full px-2 py-1 text-[11px] rounded border border-[var(--bg-border)] bg-[var(--bg-input)] text-[var(--text-primary)]" />
                            </div>
                            <div>
                              <label className="block text-[10px] text-[var(--text-muted)]">Answer 类型</label>
                              <select value={current.answer_type || ''}
                                onChange={e => {
                                  const vo = [...(editingOverlay?.version_overrides || [])]
                                  vo[ovIdx] = { ...vo[ovIdx], answer_type: e.target.value }
                                  setEditingOverlay(prev => prev ? { ...prev, version_overrides: vo } : null)
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

        {/* Save / Delete buttons */}
        <div className="flex justify-end gap-2 pt-2 border-t border-[var(--bg-border)]">
          {hasOverlay && (
            <button
              onClick={async () => {
                await api.deleteNetbootOverlay(distro.name)
                setEditingOverlay(null)
                setOverlays(prev => {
                  const next = { ...prev }
                  delete next[distro.name]
                  return next
                })
              }}
              className="px-3 py-1.5 text-xs font-medium rounded-lg bg-red-500/15 text-red-400 hover:bg-red-500/25 transition-colors"
            >
              删除覆盖
            </button>
          )}
          <button
            onClick={() => {
              setEditingOverlay(null)
              // Reset to the saved state
            }}
            className="px-3 py-1.5 text-xs font-medium rounded-lg bg-[var(--bg-hover)] text-[var(--text-secondary)] transition-colors"
          >
            取消
          </button>
          <button
            onClick={() => saveOverlay(distro.name)}
            disabled={isSaving}
            className="px-3 py-1.5 text-xs font-medium rounded-lg bg-blue-500/15 text-blue-400 hover:bg-blue-500/25 disabled:opacity-40 transition-colors"
          >
            {isSaving ? '保存中...' : '保存覆盖'}
          </button>
        </div>
      </div>
    )
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-xl font-semibold">OS 安装目录</h1>
        <div className="flex gap-2">
          <div className="relative">
            <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-[var(--text-muted)]" />
            <input
              type="text"
              placeholder="搜索..."
              value={search}
              onChange={e => setSearch(e.target.value)}
              className="pl-8 pr-4 py-1.5 text-sm border border-[var(--border-color)] rounded bg-[var(--bg-primary)]"
            />
          </div>
        </div>
      </div>

      <div className="flex gap-1 mb-4">
        {tabs.map(tab => (
          <button
            key={tab.key}
            onClick={() => setActiveTab(tab.key)}
            className={`px-3 py-1.5 text-sm rounded ${
              activeTab === tab.key
                ? 'bg-blue-500 text-white'
                : 'bg-[var(--bg-secondary)] text-[var(--text-muted)] hover:bg-[var(--bg-hover)]'
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
      ) : (
        <div className="space-y-2">
          {filteredDistros().map(distro => {
            const hasOverlay = !!overlays[distro.name]
            return (
              <Card key={distro.name} className="overflow-hidden">
                <button
                  onClick={() => {
                    const willExpand = !expanded[distro.name]
                    toggleExpand(distro.name)
                    if (willExpand) {
                      const ov = overlays[distro.name]
                      if (ov) setEditingOverlay(JSON.parse(JSON.stringify(ov)))
                      else setEditingOverlay(null)
                    }
                  }}
                  className="w-full flex items-center justify-between px-4 py-3 hover:bg-[var(--bg-hover)]"
                >
                  <div className="flex items-center gap-3">
                    {expanded[distro.name] ? <ChevronDown size={16} /> : <ChevronRight size={16} />}
                    {GROUP_ICONS[distro.menu_group] || <Package size={16} />}
                    <span className="font-medium">{distro.name}</span>
                    <span className="text-xs text-[var(--text-muted)]">{distro.versions.length} 个版本</span>
                    {hasOverlay && (
                      <span className="inline-block px-1.5 py-0.5 text-[10px] font-semibold rounded-full bg-blue-500/15 text-blue-400">
                        Overlay
                      </span>
                    )}
                  </div>
                  {distro.mirror && (
                    <span className="text-xs text-[var(--text-muted)] truncate max-w-[300px]">{distro.mirror}</span>
                  )}
                </button>

                {expanded[distro.name] && (
                  <div className="border-t border-[var(--border-color)]">
                    {/* Detail tabs */}
                    <div className="flex border-b border-[var(--border-color)]">
                      <button
                        onClick={() => setDetailView(distro.name, 'versions')}
                        className={`px-4 py-2 text-xs font-medium ${(detailTab[distro.name] || 'versions') === 'versions'
                          ? 'text-blue-400 border-b-2 border-blue-400'
                          : 'text-[var(--text-muted)] hover:text-[var(--text-primary)]'
                        }`}
                      >
                        版本
                      </button>
                      <button
                        onClick={() => {
                          setDetailView(distro.name, 'overlay')
                          if (!editingOverlay || editingOverlay.distro_name !== distro.name) {
                            setEditingOverlay(initOverlay(distro.name))
                          }
                        }}
                        className={`flex items-center gap-1 px-4 py-2 text-xs font-medium ${detailTab[distro.name] === 'overlay'
                          ? 'text-blue-400 border-b-2 border-blue-400'
                          : 'text-[var(--text-muted)] hover:text-[var(--text-primary)]'
                        }`}
                      >
                        <Settings2 size={12} />
                        覆盖配置
                        {hasOverlay && <span className="w-1.5 h-1.5 rounded-full bg-blue-400" />}
                      </button>
                    </div>

                    {/* Versions tab */}
                    {(detailTab[distro.name] || 'versions') === 'versions' && (
                      <table className="w-full text-sm">
                        <thead>
                          <tr className="bg-[var(--bg-secondary)]">
                            <th className="text-left px-4 py-2 font-medium text-[var(--text-muted)]">版本</th>
                            <th className="text-left px-4 py-2 font-medium text-[var(--text-muted)]">架构</th>
                            <th className="text-left px-4 py-2 font-medium text-[var(--text-muted)]">安装类型</th>
                            <th className="text-center px-4 py-2 font-medium text-[var(--text-muted)]">本地缓存</th>
                          </tr>
                        </thead>
                        <tbody>
                          {distro.versions.filter(v => v.enabled).map(ver => {
                            const statusKey = `${distro.name}/${ver.name}/${ver.arch}`
                            const hasLocal = fileStatuses[statusKey] || !!ver.local
                            return (
                              <tr key={`${distro.name}-${ver.codename}`} className="border-t border-[var(--border-color)]">
                                <td className="px-4 py-2 text-[var(--text-primary)]">{ver.name}</td>
                                <td className="px-4 py-2">{archBadge(ver.arch)}</td>
                                <td className="px-4 py-2">{installTypeTag(ver.install_type) || <span className="text-xs text-[var(--text-muted)]">—</span>}</td>
                                <td className="px-4 py-2 text-center">
                                  {hasLocal
                                    ? <CheckCircle2 size={16} className="text-green-500 inline" />
                                    : <XCircle size={16} className="text-red-400 inline" />
                                  }
                                </td>
                              </tr>
                            )
                          })}
                        </tbody>
                      </table>
                    )}

                    {/* Overlay config tab */}
                    {detailTab[distro.name] === 'overlay' && renderOverlayEditor(distro)}
                  </div>
                )}
              </Card>
            )
          })}

          {filteredDistros().length === 0 && (
            <div className="text-center py-16 text-[var(--text-muted)]">
              没有匹配的发行版
            </div>
          )}
        </div>
      )}
    </div>
  )
}
