import { useState, useEffect, useCallback } from 'react'
import { useTranslation } from 'react-i18next'
import { Save, RefreshCw, Plus, Trash2, Edit2, FolderOpen, Server } from 'lucide-react'
import { Card } from '../components/ui/Card'
import { Button } from '../components/ui/Button'
import { Modal } from '../components/ui/Modal'
import { ConfirmDialog } from '../components/ui/ConfirmDialog'
import { useToast } from '../components/ui/Toast'
import { SettingsField, SettingsInput } from '../components/settings/SettingsField'
import { api, type NFSSettingsData, type NFSMountPointData } from '../api/client'

// Empty mount point for new entries
function newMountPoint(): NFSMountPointData {
  return { label: '', export_path: '/', local_dir: '', read_only: true, allow_ips: [] }
}

export default function SettingsNFS() {
  const { t } = useTranslation()
  const { success, error: showError } = useToast()
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [config, setConfig] = useState<NFSSettingsData>({
    enabled: false,
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

  useEffect(() => { loadSettings() }, [])

  async function loadSettings() {
    setLoading(true)
    try {
      const res = await api.getNFSSettings()
      const d = res.data
      setConfig({
        enabled: d.enabled ?? false,
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
    setModalOpen(true)
  }

  // Open modal for editing existing mount point
  function handleEdit(index: number) {
    setEditingIndex(index)
    setModalData({ ...config.mount_points[index] })
    setModalOpen(true)
  }

  // Save from modal
  function handleModalSave() {
    // Validate required fields
    if (!modalData.export_path || !modalData.local_dir) {
      showError(t('settings.nfsMountPointRequired'))
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

            {config.mount_points.length === 0 || (config.mount_points.length === 1 && !config.mount_points[0].local_dir) ? (
              <div className="text-center py-12 text-[var(--text-muted)]">
                <FolderOpen size={32} className="mx-auto mb-3 opacity-50" />
                <p className="text-sm">{t('settings.nfsNoMountPoints')}</p>
              </div>
            ) : (
              <div className="space-y-3">
                {config.mount_points.map((mp, idx) => (
                  <div
                    key={idx}
                    className="bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded-lg p-4 hover:border-[var(--text-muted)] transition-colors"
                  >
                    <div className="flex items-center justify-between">
                      {/* Left: Mount point info */}
                      <div className="flex-1 min-w-0">
                        <div className="flex items-center gap-2 mb-1">
                          <span className="font-semibold text-[var(--text-primary)]">
                            {mp.label || mp.export_path}
                          </span>
                          <span className={`px-2 py-0.5 text-xs rounded-full ${
                            mp.read_only
                              ? 'bg-blue-500/20 text-blue-400'
                              : 'bg-amber-500/20 text-amber-400'
                          }`}>
                            {mp.read_only ? t('settings.nfsReadOnlyYes') : t('settings.nfsReadOnlyNo')}
                          </span>
                        </div>
                        <div className="flex items-center gap-4 text-xs text-[var(--text-muted)]">
                          <span className="flex items-center gap-1">
                            <span className="text-[var(--text-secondary)]">{t('settings.nfsExportPath')}:</span>
                            <code className="font-mono bg-[var(--bg-card)] px-1.5 py-0.5 rounded">{mp.export_path}</code>
                          </span>
                          <span className="flex items-center gap-1">
                            <span className="text-[var(--text-secondary)]">{t('settings.nfsLocalDir')}:</span>
                            <code className="font-mono bg-[var(--bg-card)] px-1.5 py-0.5 rounded truncate max-w-[300px]">{mp.local_dir}</code>
                          </span>
                          {mp.allow_ips && mp.allow_ips.length > 0 && (
                            <span className="flex items-center gap-1">
                              <span className="text-[var(--text-secondary)]">{t('settings.nfsAllowIPs')}:</span>
                              <span className="font-mono bg-[var(--bg-card)] px-1.5 py-0.5 rounded">
                                {mp.allow_ips.length > 2
                                  ? `${mp.allow_ips[0]}, +${mp.allow_ips.length - 1}`
                                  : mp.allow_ips.join(', ')}
                              </span>
                            </span>
                          )}
                        </div>
                      </div>

                      {/* Right: Actions */}
                      <div className="flex items-center gap-1 ml-4">
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => handleEdit(idx)}
                          className="text-[var(--text-muted)] hover:text-[var(--text-primary)]"
                        >
                          <Edit2 size={14} />
                        </Button>
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => handleDeleteClick(idx)}
                          className="text-red-400 hover:text-red-300"
                        >
                          <Trash2 size={14} />
                        </Button>
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            )}
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
          <SettingsField label={t('settings.nfsMountPointLabel')} help={t('settings.nfsMountPointLabelHelp')}>
            <SettingsInput
              value={modalData.label}
              onChange={v => setModalData({ ...modalData, label: v })}
              placeholder={t('settings.nfsMountPointLabelPlaceholder')}
            />
          </SettingsField>

          <SettingsField label={t('settings.nfsExportPath')} help={t('settings.nfsExportPathHelp')}>
            <SettingsInput
              value={modalData.export_path}
              onChange={v => setModalData({ ...modalData, export_path: v })}
              placeholder={t('settings.nfsExportPathPlaceholder')}
            />
          </SettingsField>

          <SettingsField label={t('settings.nfsLocalDir')} help={t('settings.nfsLocalDirHelp')}>
            <SettingsInput
              value={modalData.local_dir}
              onChange={v => setModalData({ ...modalData, local_dir: v })}
              placeholder={t('settings.nfsLocalDirPlaceholder')}
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
            <div>
              <div className="text-sm font-medium text-[var(--text-primary)]">{t('settings.nfsReadOnly')}</div>
              <div className="text-xs text-[var(--text-muted)]">{t('settings.nfsReadOnlyHelp')}</div>
            </div>
            <label className="relative inline-flex items-center cursor-pointer">
              <input
                type="checkbox"
                checked={modalData.read_only}
                onChange={e => setModalData({ ...modalData, read_only: e.target.checked })}
                className="sr-only peer"
              />
              <div className="w-11 h-6 bg-gray-200 peer-focus:ring-2 peer-focus:ring-blue-300 rounded-full peer peer-checked:after:translate-x-full rtl:peer-checked:after:-translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:start-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-blue-600 dark:bg-gray-600 dark:peer-checked:bg-blue-600"></div>
            </label>
          </div>
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
    </div>
  )
}
