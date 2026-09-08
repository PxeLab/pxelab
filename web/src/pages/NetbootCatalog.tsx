import { useState, useEffect, useCallback } from 'react'
import { useSearchParams, useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { Search, RefreshCw, HardDrive, Globe, Monitor, Cpu, Package, Wrench, Server, CheckCircle2, XCircle, Settings2, Trash2, Plus } from 'lucide-react'
import { useToast } from '../components/ui/Toast'
import { Button } from '../components/ui/Button'
import { Modal } from '../components/ui/Modal'
import { ConfirmDialog } from '../components/ui/ConfirmDialog'
import { PageHeader } from '../components/ui/PageHeader'
import { Pagination } from '../components/ui/Pagination'
import { DataTable, type Column } from '../components/ui/DataTable'
import { Input, Select } from '../components/ui/FormControls'
import { api, type NetbootDistro, type NetbootVersion, type NetbootOverlay } from '../api/client'
import { useUIConfig } from '../contexts/UIConfigContext'

type Tab = 'all' | string

type CatalogRow = { distro: NetbootDistro; version: NetbootVersion }

const GROUP_LABEL_KEYS: Record<string, string> = {
  linux: 'netbootCatalog.groups.linux',
  'linux-i386': 'netbootCatalog.groups.linuxI386',
  'linux-arm64': 'netbootCatalog.groups.linuxArm64',
  bsd: 'netbootCatalog.groups.bsd',
  live: 'netbootCatalog.groups.live',
  'live-arm': 'netbootCatalog.groups.liveArm',
  tools: 'netbootCatalog.groups.tools',
  unix: 'netbootCatalog.groups.unix',
  dos: 'netbootCatalog.groups.dos',
  windows: 'netbootCatalog.groups.windows',
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
  { value: '', label: '' },
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
    arm64: 'bg-accent-green/10 text-accent-green',
    aarch64: 'bg-accent-green/10 text-accent-green',
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
    memdisk: 'bg-accent-yellow/10 text-accent-yellow',
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
    live: 'bg-accent-green/10 text-accent-green',
    direct: 'bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-300',
    bsd: 'bg-accent-red/10 text-accent-red',
  }
  return (
    <span className={`inline-block text-[10px] px-1.5 py-0.5 rounded font-medium ${colors[t] || ''}`}>
      {t}
    </span>
  )
}

export default function NetbootCatalog() {
  const { t } = useTranslation()
  const [distros, setDistros] = useState<NetbootDistro[]>([])
  const [fileStatuses, setFileStatuses] = useState<Record<string, boolean>>({})
  const [overlays, setOverlays] = useState<Record<string, NetbootOverlay>>({})
  const [loading, setLoading] = useState(true)
  const [search, setSearch] = useState('')
  const [searchParams, setSearchParams] = useSearchParams()
  const activeTab = (searchParams.get('tab') as Tab) || 'all'
  // Overlay modal state
  const [overlayDistro, setOverlayDistro] = useState<NetbootDistro | null>(null)
  const [confirmDeleteOverlay, setConfirmDeleteOverlay] = useState(false)
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
    } catch (err: any) {
      toastError(err?.message || t('netbootCatalog.loadFailed'))
    }
    setLoading(false)
  }, [t, toastError])

  useEffect(() => { loadData() }, [loadData])
  useEffect(() => { setPage(1) }, [search, activeTab])

  const groups = [...new Set(distros.filter(d => d.enabled).map(d => d.menu_group))].sort()
  const tabs: { key: Tab; label: string; filter: (d: NetbootDistro) => boolean }[] = [
    { key: 'all', label: t('netbootCatalog.tabAll'), filter: () => true },
    ...groups.map(g => ({
      key: g,
      label: GROUP_LABEL_KEYS[g] ? t(GROUP_LABEL_KEYS[g]) : g,
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
      setOverlayDistro(null)
      setEditingOverlay(null)
    } catch (err: any) {
      // 保存失败时弹窗保持打开，便于修正后重试
      toastError(err?.message || t('netbootCatalog.overlaySaveFailed'))
    }
    setOverlaySaving(false)
  }

  async function deleteOverlay() {
    if (!overlayDistro) return
    try {
      await api.deleteNetbootOverlay(overlayDistro.name)
      setOverlays(prev => {
        const next = { ...prev }
        delete next[overlayDistro.name]
        return next
      })
      setOverlayDistro(null); setEditingOverlay(null)
    } catch (err: any) {
      toastError(err?.message || t('netbootCatalog.overlayDeleteFailed'))
    }
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
          <label className="text-sm font-medium text-[var(--text-primary)]">{t('netbootCatalog.overlayEnable')}</label>
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
                <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">{t('netbootCatalog.overlayMirror')}</label>
                <Input size="sm" type="text" value={ov.mirror || ''}
                  onChange={e => setEditingOverlay({ ...ov, mirror: e.target.value })}
                  placeholder={distro.mirror || ''} />
              </div>
              <div>
                <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">{t('netbootCatalog.overlayLocalPath')}</label>
                <Input size="sm" type="text" value={ov.local_base || ''}
                  onChange={e => setEditingOverlay({ ...ov, local_base: e.target.value })} />
              </div>
            </div>
            <div>
              <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">{t('netbootCatalog.overlayKernelParams')}</label>
              <Input size="sm" type="text" value={ov.kernel_params || ''}
                onChange={e => setEditingOverlay({ ...ov, kernel_params: e.target.value })} />
            </div>

            <div>
              <div className="flex items-center justify-between mb-2">
                <label className="text-xs font-medium text-[var(--text-muted)]">{t('netbootCatalog.overlayVersionOverrides')}</label>
              </div>
              {distro.versions.filter(v => v.enabled).map((ver) => {
                const existingVo = (ov.version_overrides || []).find(
                  vo => vo.codename === ver.codename && vo.arch === ver.arch
                )
                const voIdx = existingVo ? (ov.version_overrides || []).indexOf(existingVo) : -1
                const isExpanded = voIdx >= 0

                return (
                  <div key={`${ver.codename}-${ver.arch}`} className="mb-2 p-3 rounded-lg bg-[var(--hover)]/30 border border-[var(--bg-border)]">
                    <div className="flex items-center justify-between">
                      <span className="text-xs font-medium text-[var(--text-primary)]">{ver.name} ({ver.arch})</span>
                      {isExpanded ? (
                        <button onClick={() => removeVersionOverride(voIdx)}
                          className="text-[10px] text-accent-red">{t('netbootCatalog.overlayRemove')}</button>
                      ) : (
                        <button onClick={() => addVersionOverride(ver.codename, ver.arch)}
                          className="text-[10px] text-blue-400 hover:text-blue-300">{t('netbootCatalog.overlayAdd')}</button>
                      )}
                    </div>
                    {isExpanded && existingVo && (() => {
                      const idx = voIdx
                      const current = (ov.version_overrides || [])[idx]
                      return (
                        <div className="mt-2 space-y-2">
                          <div className="grid grid-cols-2 gap-2">
                            <div>
                              <label className="block text-[10px] text-[var(--text-muted)]">{t('netbootCatalog.overlayKernelUrl')}</label>
                              <Input size="xs" className="text-[11px]" type="text" value={current.remote_kernel || ''}
                                onChange={e => {
                                  const vo = [...(ov.version_overrides || [])]
                                  vo[idx] = { ...vo[idx], remote_kernel: e.target.value }
                                  setEditingOverlay({ ...ov, version_overrides: vo })
                                }} />
                            </div>
                            <div>
                              <label className="block text-[10px] text-[var(--text-muted)]">{t('netbootCatalog.overlayInitrdUrl')}</label>
                              <Input size="xs" className="text-[11px]" type="text" value={current.remote_initrd || ''}
                                onChange={e => {
                                  const vo = [...(ov.version_overrides || [])]
                                  vo[idx] = { ...vo[idx], remote_initrd: e.target.value }
                                  setEditingOverlay({ ...ov, version_overrides: vo })
                                }} />
                            </div>
                          </div>
                          <div>
                            <label className="block text-[10px] text-[var(--text-muted)]">{t('netbootCatalog.overlayCmdline')}</label>
                            <Input size="xs" className="text-[11px]" type="text" value={current.cmdline || ''}
                              onChange={e => {
                                const vo = [...(ov.version_overrides || [])]
                                vo[idx] = { ...vo[idx], cmdline: e.target.value }
                                setEditingOverlay({ ...ov, version_overrides: vo })
                              }} />
                          </div>
                          <div className="grid grid-cols-2 gap-2">
                            <div>
                              <label className="block text-[10px] text-[var(--text-muted)]">{t('netbootCatalog.overlayAnswerParam')}</label>
                              <Input size="xs" className="text-[11px]" type="text" value={current.answer_param || ''}
                                onChange={e => {
                                  const vo = [...(ov.version_overrides || [])]
                                  vo[idx] = { ...vo[idx], answer_param: e.target.value }
                                  setEditingOverlay({ ...ov, version_overrides: vo })
                                }}
                                placeholder="autoinstall ds=nocloud-net;s={{.AnswerURL}}" />
                            </div>
                            <div>
                              <label className="block text-[10px] text-[var(--text-muted)]">{t('netbootCatalog.overlayAnswerType')}</label>
                              <Select size="xs" className="text-[11px]" value={current.answer_type || ''}
                                onChange={e => {
                                  const vo = [...(ov.version_overrides || [])]
                                  vo[idx] = { ...vo[idx], answer_type: e.target.value }
                                  setEditingOverlay({ ...ov, version_overrides: vo })
                                }}>
                                {ANSWER_TYPES.map(at => (
                                  <option key={at.value} value={at.value}>{at.value === '' ? t('netbootCatalog.answerNone') : at.label}</option>
                                ))}
                              </Select>
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

  const rowKey = (r: CatalogRow) => `${r.distro.name}-${r.version.codename}-${r.version.arch}`

  const columns: Column<CatalogRow>[] = [
    {
      key: 'distro',
      label: t('netbootCatalog.colDistro'),
      render: ({ distro }) => (
        <div className="flex items-center gap-2">
          {GROUP_ICONS[distro.menu_group] || <Package size={16} />}
          <span className="text-xs font-medium text-[var(--text-primary)] whitespace-nowrap">{distro.name}</span>
        </div>
      ),
    },
    {
      key: 'version',
      label: t('netbootCatalog.colVersion'),
      render: ({ version }) => (
        <span className="text-[var(--text-primary)] text-xs whitespace-nowrap">{version.name}</span>
      ),
    },
    {
      key: 'arch',
      label: t('netbootCatalog.colArch'),
      className: 'whitespace-nowrap',
      render: ({ version }) => archBadge(version.arch),
    },
    {
      key: 'boot',
      label: t('netbootCatalog.colBoot'),
      className: 'whitespace-nowrap',
      render: ({ version }) => bootTypeTag(version.boot_type),
    },
    {
      key: 'install',
      label: t('netbootCatalog.colInstall'),
      className: 'whitespace-nowrap',
      render: ({ version }) => installTypeTag(version.install_type) || <span className="text-[10px] text-[var(--text-muted)]">—</span>,
    },
    {
      key: 'kernel',
      label: t('netbootCatalog.colKernel'),
      className: 'max-w-[200px]',
      render: ({ version }) => {
        const kernelVal = version.remote?.kernel || version.local?.kernel || ''
        return (
          <span className="font-mono text-[10px] text-[var(--text-primary)] block truncate" title={kernelVal}>{kernelVal || <span className="text-[var(--text-muted)]">—</span>}</span>
        )
      },
    },
    {
      key: 'initrd',
      label: t('netbootCatalog.colInitrd'),
      className: 'max-w-[200px]',
      render: ({ version }) => {
        const initrdVal = version.remote?.initrd || version.local?.initrd || ''
        return (
          <span className="font-mono text-[10px] text-[var(--text-primary)] block truncate" title={initrdVal}>{initrdVal || <span className="text-[var(--text-muted)]">—</span>}</span>
        )
      },
    },
    {
      key: 'cmdline',
      label: t('netbootCatalog.colCmdline'),
      className: 'max-w-[200px]',
      render: ({ version }) => (
        <span className="font-mono text-[10px] text-[var(--text-primary)] block truncate" title={version.cmdline || ''}>{version.cmdline || <span className="text-[var(--text-muted)]">—</span>}</span>
      ),
    },
    {
      key: 'cache',
      label: <span title={t('netbootCatalog.colCacheTitle')}>{t('netbootCatalog.colCache')}</span>,
      className: 'text-center',
      render: ({ distro, version }) => {
        const statusKey = `${distro.name}/${version.name}/${version.arch}`
        const hasLocal = fileStatuses[statusKey] || !!version.local
        if (hasLocal) {
          return <CheckCircle2 size={14} className="text-accent-green inline" />
        }
        const remoteOnly = !version.local && !!version.remote
        return (
          <span title={remoteOnly ? t('netbootCatalog.remoteOnlyTitle') : t('netbootCatalog.colCacheTitle')} className="inline-flex items-center gap-1 text-[9px] uppercase tracking-wide font-medium">
            <XCircle size={13} className={remoteOnly ? 'text-amber-400 inline' : 'text-accent-red/60 inline'} />
            {remoteOnly && <span className="text-amber-400 text-[10px]">{t('netbootCatalog.remoteOnly')}</span>}
          </span>
        )
      },
    },
    {
      key: 'actions',
      label: t('netbootCatalog.colActions'),
      className: 'text-center',
      render: ({ distro, version }) => (
        <button
          onClick={() => {
            setCreateTarget({ distro, version })
            setCreateName(distro.name + ' ' + version.name)
          }}
          className="inline-flex items-center gap-1 px-2 py-1 text-[10px] font-medium rounded transition-colors bg-accent-green/15 text-accent-green hover:bg-accent-green/25"
        >
          <Plus size={12} /> {t('netbootCatalog.create')}
        </button>
      ),
    },
    {
      key: 'overlay',
      label: t('netbootCatalog.colOverlay'),
      className: 'text-center',
      render: ({ distro }) => {
        const hasOverlay = !!overlays[distro.name]
        return (
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
        )
      },
    },
  ]

  return (
    <div>
      <PageHeader
        title={t('netbootCatalog.title')}
        description={t('netbootCatalog.description')}
        actions={
          <>
            <Button variant="secondary" size="sm" onClick={() => navigate('/store')}>
              <Package size={14} /> {t('netbootCatalog.importFromStore')}
            </Button>
            <Button variant="secondary" size="sm" onClick={loadData} disabled={loading}>
              <RefreshCw size={14} /> {t('netbootCatalog.refresh')}
            </Button>
          </>
        }
      />

      <div className="flex items-center gap-4 mb-4">
        <div className="relative flex-1 max-w-sm">
          <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-[var(--text-muted)]" />
          <input
            type="text"
            placeholder={t('netbootCatalog.searchPlaceholder')}
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
        <DataTable columns={columns} data={[]} loading rowKey={rowKey} />
      ) : rows.length === 0 ? (
        <div className="text-center py-16 text-[var(--text-muted)]">
          {t('netbootCatalog.noResults')}
        </div>
      ) : (
        <div>
          <div className="rounded-xl border border-[var(--border)] overflow-hidden">
            <DataTable columns={columns} data={pagedRows} rowKey={rowKey} />
          </div>
          <Pagination page={page} total={rows.length} size={pageSize} onChange={setPage} />
        </div>
      )}

      {/* Overlay Modal */}
      <Modal
        open={!!overlayDistro}
        onClose={() => { setOverlayDistro(null); setEditingOverlay(null) }}
        title={t('netbootCatalog.overlayTitle', { name: overlayDistro?.name || '' })}
        width="640px"
        footer={
          <>
            {overlays[overlayDistro?.name || '']?.id && (
              <button
                onClick={() => setConfirmDeleteOverlay(true)}
                className="px-3 py-1.5 text-xs font-medium rounded-lg bg-accent-red/15 text-accent-red hover:bg-accent-red/25 transition-colors"
              >
                <Trash2 size={12} className="inline mr-1" />{t('netbootCatalog.overlayDelete')}
              </button>
            )}
            <button
              onClick={() => { setOverlayDistro(null); setEditingOverlay(null) }}
              className="px-3 py-1.5 text-xs font-medium rounded-lg bg-[var(--bg-hover)] text-[var(--text-secondary)] transition-colors"
            >
              {t('netbootCatalog.cancel')}
            </button>
            <button
              onClick={saveOverlay}
              disabled={overlaySaving}
              className="px-3 py-1.5 text-xs font-medium rounded-lg bg-blue-500/15 text-blue-400 hover:bg-blue-500/25 disabled:opacity-40 transition-colors"
            >
              {overlaySaving ? t('netbootCatalog.saving') : t('netbootCatalog.overlaySave')}
            </button>
          </>
        }
      >
        {renderOverlayBody()}
      </Modal>

      <ConfirmDialog
        open={confirmDeleteOverlay}
        onClose={() => setConfirmDeleteOverlay(false)}
        onConfirm={async () => { setConfirmDeleteOverlay(false); await deleteOverlay() }}
        message={t('netbootCatalog.confirmDeleteOverlay', { name: overlayDistro?.name || '' })}
      />

      {/* Create Profile Dialog */}
      <Modal
        open={!!createTarget}
        onClose={() => setCreateTarget(null)}
        title={t('netbootCatalog.createProfileTitle')}
        width="400px"
        footer={
          <>
            <button
              onClick={() => setCreateTarget(null)}
              className="px-3 py-1.5 text-xs font-medium rounded-lg bg-[var(--bg-hover)] text-[var(--text-secondary)] transition-colors"
            >
              {t('netbootCatalog.cancel')}
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
                  success(t('netbootCatalog.createSuccess'))
                  setCreateTarget(null)
                  navigate('/profiles')
                } catch {
                  toastError(t('netbootCatalog.createFailed'))
                }
                setCreateLoading(false)
              }}
              disabled={createLoading || !createName.trim()}
              className="px-3 py-1.5 text-xs font-medium rounded-lg bg-blue-500/15 text-blue-400 hover:bg-blue-500/25 disabled:opacity-40 transition-colors"
            >
              {createLoading ? t('netbootCatalog.creating') : t('netbootCatalog.createConfirm')}
            </button>
          </>
        }
      >
        <div className="space-y-4">
          <div>
            <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">{t('netbootCatalog.profileName')}</label>
            <Input
              type="text"
              value={createName}
              onChange={e => {
                // iPXE BIOS menu font is ASCII-only — block non-ASCII input (e.g. Chinese)
                const v = e.target.value
                if (/^[\x20-\x7E]*$/.test(v)) setCreateName(v)
              }}
              placeholder={t('netbootCatalog.profileNamePlaceholder')}
              autoFocus
            />
          </div>
          {createTarget && (
            <div className="text-xs text-[var(--text-muted)] space-y-1">
              <p>{t('netbootCatalog.colDistro')}: <span className="text-[var(--text-primary)]">{createTarget.distro.name}</span></p>
              <p>{t('netbootCatalog.colVersion')}: <span className="text-[var(--text-primary)]">{createTarget.version.name} ({createTarget.version.codename})</span></p>
              <p>{t('netbootCatalog.colArch')}: <span className="text-[var(--text-primary)]">{createTarget.version.arch}</span></p>
              {createTarget.version.remote?.kernel && (
                <p className="truncate" title={createTarget.version.remote.kernel}>
                  {t('netbootCatalog.colKernel')}: <span className="text-[var(--text-primary)]">{createTarget.version.remote.kernel}</span>
                </p>
              )}
            </div>
          )}
        </div>
      </Modal>
    </div>
  )
}
