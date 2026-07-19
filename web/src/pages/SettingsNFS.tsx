import { useState, useEffect, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import { Save, RefreshCw, Plus, Trash2, Edit2, FolderOpen, Server, Users, CheckCircle, XCircle, Loader2 } from 'lucide-react'
import { Card } from '../components/ui/Card'
import { Button } from '../components/ui/Button'
import { Modal } from '../components/ui/Modal'
import { ConfirmDialog } from '../components/ui/ConfirmDialog'
import { Toggle } from '../components/ui/Toggle'
import { useToast } from '../components/ui/Toast'
import { SettingsField, SettingsInput } from '../components/settings/SettingsField'
import { api, type NFSSettingsData, type NFSMountPointData, type NFSClientInfo } from '../api/client'
import { DataTable, type Column } from '../components/ui/DataTable'

// Empty mount point for new entries
function newMountPoint(): NFSMountPointData {
  return { label: '', export_path: '/', local_dir: '', read_only: true, allow_ips: [], connection_count: 0 }
}

export default function SettingsNFS() {
  const { t } = useTranslation()
  const { success, error: showError } = useToast()
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [config, setConfig] = useState<NFSSettingsData>({
    enabled: false,
    running: false,
    port: 2049,
    rpcbind_port: 111,
    version: 'NFSv3',
    mount_points: [newMountPoint()],
  })

  // Modal state
  const [modalOpen, setModalOpen] = useState(false)
  const [editingIndex, setEditingIndex] = useState<number | null>(null)
  const [modalData, setModalData] = useState<NFSMountPointData>(newMountPoint())

  // Delete confirm state
  const [deleteIndex, setDeleteIndex] = useState<number | null>(null)

  // Client details modal
  const [clientsModalIndex, setClientsModalIndex] = useState<number | null>(null)

  // Browse directory modal
  const [browseOpen, setBrowseOpen] = useState(false)
  const [browsePath, setBrowsePath] = useState('/')
  const [browseEntries, setBrowseEntries] = useState<Array<{ name: string; path: string; is_dir: boolean }>>([])
  const [browseLoading, setBrowseLoading] = useState(false)
  const [browseError, setBrowseError] = useState<string | null>(null)

  async function loadBrowseDir(path: string) {
    setBrowseLoading(true)
    setBrowseError(null)
    try {
      const res = await api.browseNFSPath(path)
      setBrowsePath(res.data.current)
      setBrowseEntries(res.data.entries || [])
    } catch (err: any) {
      setBrowseError(err.message || 'Failed to list directory')
      setBrowseEntries([])
    } finally {
      setBrowseLoading(false)
    }
  }

  function handleBrowseOpen() {
    // Start from current local_dir or root
    const localDir = modalData.local_dir
    if (localDir) {
      // Navigate to parent of current path
      // Handle both Unix (/foo/bar) and Windows (C:\foo\bar) separators
      const sep = localDir.includes('\\') ? '\\' : '/'
      const parts = localDir.split(sep).filter(Boolean)
      if (parts.length <= 1) {
        // At drive root or Unix root — show drives / root
        loadBrowseDir('')
      } else {
        // Go up one level
        const parent = parts.length > 1
          ? (localDir.startsWith('/') ? '/' : parts[0] + sep) + parts.slice(1, -1).join(sep)
          : ''
        loadBrowseDir(parent)
      }
    } else {
      loadBrowseDir('')
    }
    setBrowseOpen(true)
  }

  function handleBrowseSelect(dirPath: string) {
    setModalData({ ...modalData, local_dir: dirPath })
    handleLocalDirChange(dirPath)
    setBrowseOpen(false)
  }

  // Path validation state (for edit modal local_dir)
  const [pathValidation, setPathValidation] = useState<{ exists: boolean; writable: boolean } | null>(null)
  const [validatingPath, setValidatingPath] = useState(false)
  const pathValidationTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

  // Debounced path validation
  function handleLocalDirChange(val: string) {
    setModalData({ ...modalData, local_dir: val })
    setPathValidation(null)
    if (pathValidationTimer.current) clearTimeout(pathValidationTimer.current)
    if (!val.trim()) return
    pathValidationTimer.current = setTimeout(async () => {
      setValidatingPath(true)
      try {
        const res = await api.validateNFSPath(val.trim())
        setPathValidation({ exists: res.data.exists, writable: res.data.writable })
      } catch {
        setPathValidation(null)
      } finally {
        setValidatingPath(false)
      }
    }, 600)
  }

  // Clean up timer on unmount
  useEffect(() => {
    return () => { if (pathValidationTimer.current) clearTimeout(pathValidationTimer.current) }
  }, [])

  useEffect(() => { loadSettings() }, [])

  async function loadSettings() {
    setLoading(true)
    try {
      const res = await api.getNFSSettings()
      const d = res.data
      setConfig({
        enabled: d.enabled ?? false,
        running: d.running ?? false,
        port: d.port || 2049,
        rpcbind_port: d.rpcbind_port || 111,
        version: d.version || 'NFSv3',
        mount_points: (d.mount_points && d.mount_points.length > 0)
          ? d.mount_points
          : [newMountPoint()],
      })
    } catch (err: any) {
      showError(err.message || t('settings.loadFailed'))
    } finally {
      setLoading(false)
    }
  }

  async function handleSave() {
    setSaving(true)
    try {
      await api.updateNFSSettings(config)
      success(t('settings.saved'))
    } catch (err: any) {
      showError(err.message || t('settings.saveFailed'))
    } finally {
      setSaving(false)
    }
  }

  // Open modal for adding new mount point
  function handleAdd() {
    setEditingIndex(null)
    setModalData(newMountPoint())
    setPathValidation(null)
    setModalOpen(true)
  }

  // Open modal for editing existing mount point
  function handleEdit(index: number) {
    setEditingIndex(index)
    setModalData({ ...config.mount_points[index] })
    setPathValidation(null)
    setModalOpen(true)
  }

  // Save from modal
  function handleModalSave() {
    // Validate required fields
    if (!modalData.label?.trim()) {
      showError(t('settings.nfsLabelRequired'))
      return
    }
    if (!modalData.export_path) {
      showError(t('settings.nfsExportPathRequired'))
      return
    }
    if (!modalData.export_path.startsWith('/')) {
      showError(t('settings.nfsExportPathMustStartSlash'))
      return
    }
    if (!modalData.local_dir) {
      showError(t('settings.nfsLocalDirRequired'))
      return
    }

    // Check for duplicate export_path (exclude current editing index)
    const dupIndex = config.mount_points.findIndex(
      (mp, i) => mp.export_path === modalData.export_path && i !== editingIndex
    )
    if (dupIndex !== -1) {
      showError(t('settings.nfsExportPathDuplicate', { label: config.mount_points[dupIndex].label || config.mount_points[dupIndex].export_path }))
      return
    }

    const mps = [...config.mount_points]
    if (editingIndex !== null) {
      // Edit existing
      mps[editingIndex] = modalData
    } else {
      // Add new
      mps.push(modalData)
    }
    setConfig({ ...config, mount_points: mps })
    setModalOpen(false)
  }

  // Open delete confirm
  function handleDeleteClick(index: number) {
    setDeleteIndex(index)
  }

  // Confirm delete
  function handleDeleteConfirm() {
    if (deleteIndex === null) return
    const mps = config.mount_points.filter((_, i) => i !== deleteIndex)
    setConfig({ ...config, mount_points: mps.length > 0 ? mps : [newMountPoint()] })
    setDeleteIndex(null)
  }

  // Filter valid mount points (non-empty local_dir)
  const validMountPoints = config.mount_points.filter(mp => mp.local_dir)

  // Table columns for mount points
  const mpColumns: Column<NFSMountPointData & { _index: number }>[] = [
    {
      key: 'label',
      label: t('settings.nfsMountPointLabel'),
      render: (mp) => (
        <div className="flex items-center gap-2">
          <span className="font-medium text-[var(--text-primary)]">{mp.label || mp.export_path}</span>
          {(mp.connection_count || 0) > 0 && (
            <button
              onClick={(e) => { e.stopPropagation(); setClientsModalIndex(mp._index) }}
              className="px-1.5 py-0.5 text-xs rounded-full bg-green-500/20 text-green-400 flex items-center gap-1 hover:bg-green-500/30 transition-colors cursor-pointer"
            >
              <Users size={10} />
              {mp.connection_count}
            </button>
          )}
        </div>
      ),
    },
    {
      key: 'export_path',
      label: t('settings.nfsExportPath'),
      render: (mp) => (
        <code className="font-mono text-xs bg-[var(--bg-card)] px-1.5 py-0.5 rounded">{mp.export_path}</code>
      ),
    },
    {
      key: 'local_dir',
      label: t('settings.nfsLocalDir'),
      render: (mp) => (
        <code className="font-mono text-xs bg-[var(--bg-card)] px-1.5 py-0.5 rounded truncate max-w-[300px] inline-block">{mp.local_dir}</code>
      ),
    },
    {
      key: 'read_only',
      label: t('settings.nfsReadOnly'),
      width: '100px',
      render: (mp) => (
        <span className={`px-2 py-0.5 text-xs rounded-full ${
          mp.read_only
            ? 'bg-blue-500/20 text-blue-400'
            : 'bg-amber-500/20 text-amber-400'
        }`}>
          {mp.read_only ? t('settings.nfsReadOnlyYes') : t('settings.nfsReadOnlyNo')}
        </span>
      ),
    },
    {
      key: 'allow_ips',
      label: t('settings.nfsAllowIPs'),
      render: (mp) => {
        if (!mp.allow_ips || mp.allow_ips.length === 0) {
          return <span className="text-xs text-[var(--text-muted)]">—</span>
        }
        return (
          <span className="font-mono text-xs bg-[var(--bg-card)] px-1.5 py-0.5 rounded">
            {mp.allow_ips.length > 2
              ? `${mp.allow_ips[0]}, +${mp.allow_ips.length - 1}`
              : mp.allow_ips.join(', ')}
          </span>
        )
      },
    },
    {
      key: 'actions',
      label: '',
      width: '80px',
      render: (mp) => (
        <div className="flex items-center gap-1 justify-end">
          <Button
            variant="ghost"
            size="sm"
            onClick={() => handleEdit(mp._index)}
            className="text-[var(--text-muted)] hover:text-[var(--text-primary)]"
          >
            <Edit2 size={14} />
          </Button>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => handleDeleteClick(mp._index)}
            className="text-red-400 hover:text-red-300"
          >
            <Trash2 size={14} />
          </Button>
        </div>
      ),
    },
  ]

  // DataTable data with original index preserved
  const mpTableData = validMountPoints.map((mp) => ({
    ...mp,
    _index: config.mount_points.indexOf(mp),
  }))

  return (
    <div>
      {/* Header */}
      <div className="flex items-center justify-between mb-4">
        <h1 className="text-lg font-bold text-[var(--text-primary)]">{t('settings.nfsTitle')}</h1>
        <div className="flex gap-2">
          <Button variant="secondary" size="sm" disabled={loading} onClick={loadSettings}>
            <RefreshCw size={14} /> {t('settings.refresh')}
          </Button>
          <Button variant="primary" size="sm" disabled={saving} onClick={handleSave}>
            <Save size={14} /> {saving ? t('settings.saving') : t('settings.save')}
          </Button>
        </div>
      </div>

      {loading ? (
        <Card>
          <div className="flex items-center justify-center py-16">
            <div className="animate-spin w-6 h-6 border-2 border-blue-500 border-t-transparent rounded-full" />
            <span className="ml-3 text-sm text-[var(--text-muted)]">{t('settings.loading')}</span>
          </div>
        </Card>
      ) : (
        <>
          {/* Status Bar */}
          <Card className="mb-4">
            <div className="flex items-center gap-6">
              <div className="flex items-center gap-2">
                <Server size={16} className="text-[var(--text-muted)]" />
                <span className="text-xs text-[var(--text-muted)]">{t('settings.nfsNfsPort')}:</span>
                <span className="text-sm font-semibold text-[var(--text-primary)]">{config.port}/TCP</span>
              </div>
              <div className="flex items-center gap-2">
                <Server size={16} className="text-[var(--text-muted)]" />
                <span className="text-xs text-[var(--text-muted)]">{t('settings.nfsRpcbindPortLabel')}:</span>
                <span className="text-sm font-semibold text-[var(--text-primary)]">{config.rpcbind_port}/UDP+TCP</span>
              </div>
              <div className="flex items-center gap-2">
                <span className="text-xs text-[var(--text-muted)]">{t('settings.nfsVersion')}:</span>
                <span className="text-sm font-semibold text-[var(--text-primary)]">{config.version}</span>
              </div>
              <div className="flex items-center gap-2">
                <FolderOpen size={16} className="text-[var(--text-muted)]" />
                <span className="text-xs text-[var(--text-muted)]">{t('settings.nfsMountPoints')}:</span>
                <span className="text-sm font-semibold text-[var(--text-primary)]">{config.mount_points.length}</span>
              </div>
              <div className="flex items-center gap-2">
                <Users size={16} className="text-[var(--text-muted)]" />
                <span className="text-xs text-[var(--text-muted)]">{t('settings.nfsTotalConnections')}:</span>
                <span className="text-sm font-semibold text-[var(--text-primary)]">
                  {config.mount_points.reduce((sum, mp) => sum + (mp.connection_count || 0), 0)}
                </span>
              </div>
              <div className="ml-auto flex items-center gap-2">
                <div className={`w-2 h-2 rounded-full ${config.running ? 'bg-green-500' : 'bg-gray-400'}`} />
                <span className="text-xs text-[var(--text-muted)]">{t(`settings.${config.running ? 'nfsRunning' : 'nfsStopped'}`)}</span>
              </div>
            </div>
          </Card>

          {/* Mount Points List */}
          <Card>
            <div className="flex items-center justify-between mb-4">
              <h2 className="text-sm font-semibold text-[var(--text-primary)]">{t('settings.nfsMountPoints')}</h2>
              <Button variant="primary" size="sm" onClick={handleAdd}>
                <Plus size={14} /> {t('settings.nfsAddMountPoint')}
              </Button>
            </div>

            <DataTable
              columns={mpColumns}
              data={mpTableData}
              emptyText={t('settings.nfsNoMountPoints')}
              rowKey={(mp) => `${mp._index}`}
            />
          </Card>
        </>
      )}

      {/* Add/Edit Modal */}
      <Modal
        open={modalOpen}
        onClose={() => setModalOpen(false)}
        title={editingIndex !== null ? t('settings.nfsEditMountPoint') : t('settings.nfsAddMountPoint')}
        width="560px"
        footer={
          <>
            <Button variant="secondary" size="sm" onClick={() => setModalOpen(false)}>
              {t('common.cancel')}
            </Button>
            <Button variant="primary" size="sm" onClick={handleModalSave}>
              {t('common.save')}
            </Button>
          </>
        }
      >
        <div className="space-y-4">
          <SettingsField label={t('settings.nfsLocalDir')} help={t('settings.nfsLocalDirHelp')}>
            <div className="relative">
              <div className="flex gap-2">
                <div className="relative flex-1">
                  <SettingsInput
                    value={modalData.local_dir}
                    onChange={handleLocalDirChange}
                    placeholder={t('settings.nfsLocalDirPlaceholder')}
                  />
                  {(validatingPath || pathValidation) && (
                    <div className="absolute right-3 top-1/2 -translate-y-1/2 flex items-center gap-1.5 pointer-events-none">
                      {validatingPath ? (
                        <Loader2 size={14} className="animate-spin text-[var(--text-muted)]" />
                      ) : pathValidation?.exists ? (
                        <span className="flex items-center gap-1 text-xs text-green-400">
                          <CheckCircle size={12} />
                          {t('settings.nfsPathExists')}{pathValidation.writable ? ` · ${t('settings.nfsPathWritable')}` : ` · ${t('settings.nfsPathNotWritable')}`}
                        </span>
                      ) : (
                        <span className="flex items-center gap-1 text-xs text-amber-400">
                          <XCircle size={12} />
                          {t('settings.nfsPathNotExist')}
                        </span>
                      )}
                    </div>
                  )}
                </div>
                <Button variant="secondary" size="sm" onClick={handleBrowseOpen} className="shrink-0">
                  <FolderOpen size={14} /> {t('settings.nfsBrowse')}
                </Button>
              </div>
            </div>
          </SettingsField>

          <SettingsField label={t('settings.nfsExportPath')} help={t('settings.nfsExportPathHelp')}>
            <SettingsInput
              value={modalData.export_path}
              onChange={v => setModalData({ ...modalData, export_path: v })}
              placeholder={t('settings.nfsExportPathPlaceholder')}
            />
          </SettingsField>

          <SettingsField label={t('settings.nfsMountPointLabel')} help={t('settings.nfsMountPointLabelHelp')}>
            <SettingsInput
              value={modalData.label}
              onChange={v => setModalData({ ...modalData, label: v })}
              placeholder={t('settings.nfsMountPointLabelPlaceholder')}
            />
          </SettingsField>

          <SettingsField label={t('settings.nfsAllowIPs')} help={t('settings.nfsAllowIPsHelp')}>
            <textarea
              className="w-full bg-[var(--bg-input)] border border-[var(--bg-border)] rounded-lg px-3 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500 font-mono resize-y"
              rows={3}
              value={(modalData.allow_ips || []).join('\n')}
              onChange={e => setModalData({
                ...modalData,
                allow_ips: e.target.value.split('\n').map(s => s.trim()).filter(Boolean),
              })}
              placeholder={t('settings.nfsAllowIPsPlaceholder')}
            />
            <p className="text-xs text-[var(--text-muted)] mt-1">{t('settings.nfsAllowRestartHint')}</p>
          </SettingsField>

          <div className="flex items-center justify-between py-2">
            <Toggle
              checked={modalData.read_only}
              onChange={v => setModalData({ ...modalData, read_only: v })}
              label={t('settings.nfsReadOnly')}
            />
          </div>
          <p className="text-xs text-[var(--text-muted)] -mt-2">{t('settings.nfsReadOnlyHelp')}</p>
        </div>
      </Modal>

      {/* Delete Confirm */}
      <ConfirmDialog
        open={deleteIndex !== null}
        onClose={() => setDeleteIndex(null)}
        onConfirm={handleDeleteConfirm}
        title={t('settings.nfsDeleteMountPoint')}
        message={deleteIndex !== null
          ? t('settings.nfsDeleteMountPointConfirm', {
              path: config.mount_points[deleteIndex]?.export_path || ''
            })
          : ''}
        danger
      />

      {/* Client Details Modal */}
      <Modal
        open={clientsModalIndex !== null}
        onClose={() => setClientsModalIndex(null)}
        title={t('settings.nfsClientDetails')}
        width="640px"
      >
        {clientsModalIndex !== null && (() => {
          const mp = config.mount_points[clientsModalIndex]
          const clients: NFSClientInfo[] = mp?.clients || []
          if (clients.length === 0) {
            return <p className="text-sm text-[var(--text-muted)] py-8 text-center">{t('settings.nfsNoConnections')}</p>
          }
          return (
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-[var(--bg-border)]">
                    <th className="text-left py-2 px-3 text-xs font-semibold text-[var(--text-muted)]">{t('settings.nfsClientIP')}</th>
                    <th className="text-left py-2 px-3 text-xs font-semibold text-[var(--text-muted)]">{t('settings.nfsClientConnectedAt')}</th>
                    <th className="text-left py-2 px-3 text-xs font-semibold text-[var(--text-muted)]">{t('settings.nfsClientLastActivity')}</th>
                    <th className="text-left py-2 px-3 text-xs font-semibold text-[var(--text-muted)]">{t('settings.nfsClientDuration')}</th>
                  </tr>
                </thead>
                <tbody>
                  {clients.map((c, i) => {
                    const connectedAt = new Date(c.connected_at)
                    const lastActivity = new Date(c.last_activity)
                    const now = new Date()
                    const durationMs = now.getTime() - connectedAt.getTime()
                    const durationMin = Math.floor(durationMs / 60000)
                    const durationStr = durationMin < 60
                      ? `${durationMin}m`
                      : `${Math.floor(durationMin / 60)}h ${durationMin % 60}m`
                    return (
                      <tr key={i} className="border-b border-[var(--bg-border)] last:border-b-0">
                        <td className="py-2 px-3 font-mono text-[var(--text-primary)]">{c.ip}</td>
                        <td className="py-2 px-3 text-[var(--text-secondary)]">{connectedAt.toLocaleString()}</td>
                        <td className="py-2 px-3 text-[var(--text-secondary)]">{lastActivity.toLocaleString()}</td>
                        <td className="py-2 px-3 text-[var(--text-secondary)]">{durationStr}</td>
                      </tr>
                    )
                  })}
                </tbody>
              </table>
            </div>
          )
        })()}
      </Modal>

      {/* Browse Directory Modal */}
      <Modal
        open={browseOpen}
        onClose={() => setBrowseOpen(false)}
        title={t('settings.nfsBrowse')}
        width="560px"
        footer={
          <Button variant="secondary" size="sm" onClick={() => setBrowseOpen(false)}>
            {t('common.cancel')}
          </Button>
        }
      >
        <div className="space-y-3">
          {/* Current path display + navigate up */}
          <div className="flex items-center gap-2">
            <button
              disabled={!browsePath || browsePath === '/'}
              onClick={() => {
                if (!browsePath) return
                const sep = browsePath.includes('\\') ? '\\' : '/'
                const parts = browsePath.split(sep).filter(Boolean)
                if (parts.length <= 1) {
                  // At drive root or Unix root — go back to drive list / root
                  loadBrowseDir('')
                } else {
                  const parent = parts.length > 1
                    ? (browsePath.startsWith('/') ? '/' : parts[0] + sep) + parts.slice(1, -1).join(sep)
                    : ''
                  loadBrowseDir(parent)
                }
              }}
              className="px-2 py-1 text-xs rounded bg-[var(--bg-input)] text-[var(--text-secondary)] hover:bg-[var(--bg-border)] disabled:opacity-40 disabled:cursor-not-allowed"
            >
              ↑
            </button>
            <code className="flex-1 text-xs font-mono bg-[var(--bg-input)] px-2 py-1 rounded text-[var(--text-primary)] truncate">
              {browsePath || t('settings.nfsBrowseDrives')}
            </code>
            <Button variant="primary" size="sm" onClick={() => handleBrowseSelect(browsePath)}>
              {t('settings.nfsSelectThisDir')}
            </Button>
          </div>

          {/* Directory list */}
          <div className="border border-[var(--bg-border)] rounded-lg max-h-[320px] overflow-y-auto">
            {browseLoading ? (
              <div className="flex items-center justify-center py-8">
                <div className="animate-spin w-5 h-5 border-2 border-blue-500 border-t-transparent rounded-full" />
              </div>
            ) : browseError ? (
              <div className="py-8 text-center text-sm text-amber-400">{browseError}</div>
            ) : browseEntries.length === 0 ? (
              <div className="py-8 text-center text-sm text-[var(--text-muted)]">{t('settings.nfsBrowseEmpty')}</div>
            ) : (
              <div>
                {browseEntries.map((entry) => (
                  <button
                    key={entry.path}
                    onClick={() => loadBrowseDir(entry.path)}
                    className="w-full flex items-center gap-2 px-3 py-2 text-left text-sm hover:bg-[var(--bg-input)] transition-colors border-b border-[var(--bg-border)] last:border-b-0"
                  >
                    <FolderOpen size={14} className="text-amber-400 shrink-0" />
                    <span className="text-[var(--text-primary)]">{entry.name}</span>
                    <span className="ml-auto text-xs text-[var(--text-muted)] font-mono">{entry.path}</span>
                  </button>
                ))}
              </div>
            )}
          </div>
        </div>
      </Modal>
    </div>
  )
}
