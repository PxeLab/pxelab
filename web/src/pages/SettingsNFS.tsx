import { useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { Save, RefreshCw } from 'lucide-react'
import { Card } from '../components/ui/Card'
import { Button } from '../components/ui/Button'
import { Toggle } from '../components/ui/Toggle'
import { useToast } from '../components/ui/Toast'
import { SettingsField, SettingsInput } from '../components/settings/SettingsField'
import { api, type NFSSettingsData } from '../api/client'

export default function SettingsNFS() {
  const { t } = useTranslation()
  const { success, error: showError } = useToast()
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [config, setConfig] = useState<NFSSettingsData>({
    enabled: false, port: 2049, root_dir: '', read_only: true, allow_ips: [],
  })

  useEffect(() => { loadSettings() }, [])

  async function loadSettings() {
    setLoading(true)
    try {
      const res = await api.getNFSSettings()
      const d = res.data
      setConfig({
        enabled: d.enabled ?? false,
        port: 2049,
        root_dir: d.root_dir || '',
        read_only: d.read_only ?? true,
        allow_ips: d.allow_ips || [],
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
            <div className="grid grid-cols-2 gap-4">
              <SettingsField label={t('settings.nfsRoot')} help={t('settings.nfsRootHelp')}>
                <SettingsInput value={config.root_dir} onChange={v => setConfig({...config, root_dir: v})} placeholder={t('settings.nfsRootPlaceholder')} />
              </SettingsField>
              <SettingsField label={t('settings.port')}>
                <SettingsInput value={String(config.port)} onChange={v => setConfig({...config, port: parseInt(v) || 2049})} />
              </SettingsField>
            </div>
            <Toggle checked={config.read_only} onChange={v => setConfig({...config, read_only: v})} label={t('settings.nfsReadOnly')} />
            <p className="text-xs text-[var(--text-muted)] -mt-2">{t('settings.nfsReadOnlyHelp')}</p>
            <div className="bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded px-3 py-2 text-xs text-[var(--text-muted)] leading-relaxed">
              <span className="font-semibold text-[var(--text-primary)]">rpcbind</span> — {t('settings.nfsRpcbindPort')}
            </div>
            <SettingsField label={t('settings.nfsAllowIPs')} help={t('settings.nfsAllowIPsHelp')}>
              <textarea
                className="w-full bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded px-2.5 py-1.5 text-xs text-[var(--text-primary)] outline-none focus:border-blue-500 font-mono resize-y"
                rows={4}
                value={(config.allow_ips || []).join('\n')}
                onChange={e => setConfig({...config, allow_ips: e.target.value.split('\n').map(s => s.trim()).filter(Boolean)})}
                placeholder={t('settings.nfsAllowIPsPlaceholder')}
              />
              <p className="text-xs text-[var(--text-muted)] mt-1">{t('settings.nfsAllowRestartHint')}</p>
            </SettingsField>
          </div>
        )}
      </Card>
    </div>
  )
}
