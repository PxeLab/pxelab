import { useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { Save, RefreshCw, Plus, Trash2 } from 'lucide-react'
import { Card } from '../components/ui/Card'
import { Button } from '../components/ui/Button'
import { Toggle } from '../components/ui/Toggle'
import { useToast } from '../components/ui/Toast'
import { SettingsField, SettingsInput } from '../components/settings/SettingsField'
import { api, type NFSSettingsData, type NFSMountPointData } from '../api/client'

function newMountPoint(): NFSMountPointData {
  return { label: '', export_path: '/', local_dir: '', read_only: true, allow_ips: [] }
}

export default function SettingsNFS() {
  const { t } = useTranslation()
  const { success, error: showError } = useToast()
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [config, setConfig] = useState<NFSSettingsData>({
    enabled: false, port: 2049, mount_points: [newMountPoint()],
  })

  useEffect(() => { loadSettings() }, [])

  async function loadSettings() {
    setLoading(true)
    try {
      const res = await api.getNFSSettings()
      const d = res.data
      setConfig({
        enabled: d.enabled ?? false,
        port: d.port || 2049,
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

  function updateMountPoint(index: number, patch: Partial<NFSMountPointData>) {
    const mps = [...config.mount_points]
    mps[index] = { ...mps[index], ...patch }
    setConfig({ ...config, mount_points: mps })
  }

  function removeMountPoint(index: number) {
    const mps = config.mount_points.filter((_, i) => i !== index)
    setConfig({ ...config, mount_points: mps.length > 0 ? mps : [newMountPoint()] })
  }

  function addMountPoint() {
    setConfig({ ...config, mount_points: [...config.mount_points, newMountPoint()] })
  }

  return (
    <div>
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

      <Card>
        {loading ? (
          <div className="flex items-center justify-center py-16">
            <div className="animate-spin w-6 h-6 border-2 border-blue-500 border-t-transparent rounded-full" />
            <span className="ml-3 text-sm text-[var(--text-muted)]">{t('settings.loading')}</span>
          </div>
        ) : (
          <div className="space-y-4">
            <div className="flex items-center gap-4">
              <SettingsField label={t('settings.port')}>
                <SettingsInput value={String(config.port)} onChange={v => setConfig({...config, port: parseInt(v) || 2049})} />
              </SettingsField>
            </div>

            <div className="bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded px-3 py-2 text-xs text-[var(--text-muted)] leading-relaxed">
              <span className="font-semibold text-[var(--text-primary)]">rpcbind</span> — {t('settings.nfsRpcbindPort')}
            </div>

            <div className="space-y-3">
              <div className="flex items-center justify-between">
                <h2 className="text-sm font-semibold text-[var(--text-primary)]">{t('settings.nfsMountPoints')}</h2>
                <Button variant="ghost" size="sm" onClick={addMountPoint}>
                  <Plus size={14} /> {t('settings.nfsMountPointAdd')}
                </Button>
              </div>

              {config.mount_points.map((mp, idx) => (
                <div key={idx} className="bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded-lg p-4 space-y-3">
                  <div className="flex items-center justify-between">
                    <span className="text-xs font-semibold text-[var(--text-secondary)]">
                      {t('settings.nfsMountPoint')} {idx + 1}
                    </span>
                    {config.mount_points.length > 1 && (
                      <Button variant="ghost" size="sm" onClick={() => removeMountPoint(idx)} className="text-red-400 hover:text-red-300">
                        <Trash2 size={14} />
                      </Button>
                    )}
                  </div>

                  <div className="grid grid-cols-2 gap-3">
                    <SettingsField label={t('settings.nfsMountPointLabel')} help={t('settings.nfsMountPointLabelHelp')}>
                      <SettingsInput
                        value={mp.label}
                        onChange={v => updateMountPoint(idx, { label: v })}
                        placeholder={t('settings.nfsMountPointLabelPlaceholder')}
                      />
                    </SettingsField>
                    <SettingsField label={t('settings.nfsExportPath')} help={t('settings.nfsExportPathHelp')}>
                      <SettingsInput
                        value={mp.export_path}
                        onChange={v => updateMountPoint(idx, { export_path: v })}
                        placeholder={t('settings.nfsExportPathPlaceholder')}
                      />
                    </SettingsField>
                  </div>

                  <SettingsField label={t('settings.nfsLocalDir')} help={t('settings.nfsLocalDirHelp')}>
                    <SettingsInput
                      value={mp.local_dir}
                      onChange={v => updateMountPoint(idx, { local_dir: v })}
                      placeholder={t('settings.nfsLocalDirPlaceholder')}
                    />
                  </SettingsField>

                  <Toggle
                    checked={mp.read_only}
                    onChange={v => updateMountPoint(idx, { read_only: v })}
                    label={t('settings.nfsReadOnly')}
                  />
                  <p className="text-xs text-[var(--text-muted)] -mt-2">{t('settings.nfsReadOnlyHelp')}</p>

                  <SettingsField label={t('settings.nfsAllowIPs')} help={t('settings.nfsAllowIPsHelp')}>
                    <textarea
                      className="w-full bg-[var(--bg-input)] border border-[var(--bg-border)] rounded-lg px-3 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500 font-mono resize-y"
                      rows={3}
                      value={(mp.allow_ips || []).join('\n')}
                      onChange={e => updateMountPoint(idx, {
                        allow_ips: e.target.value.split('\n').map(s => s.trim()).filter(Boolean),
                      })}
                      placeholder={t('settings.nfsAllowIPsPlaceholder')}
                    />
                    <p className="text-xs text-[var(--text-muted)] mt-1">{t('settings.nfsAllowRestartHint')}</p>
                  </SettingsField>
                </div>
              ))}
            </div>
          </div>
        )}
      </Card>
    </div>
  )
}
