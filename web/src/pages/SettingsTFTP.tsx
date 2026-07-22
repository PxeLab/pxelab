import { useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { Save, RefreshCw } from 'lucide-react'
import { Card } from '../components/ui/Card'
import { Button } from '../components/ui/Button'
import { PageHeader } from '../components/ui/PageHeader'
import { useToast } from '../components/ui/Toast'
import { SettingsField, SettingsInput } from '../components/settings/SettingsField'
import { api, type TFTPSettingsData } from '../api/client'

export default function SettingsTFTP() {
  const { t } = useTranslation()
  const { success, error: showError } = useToast()
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [config, setConfig] = useState<TFTPSettingsData>({
    enabled: true, port: 69, timeout: 5, root: '',
    pxe_config_file: 'pxelinux.cfg/default',
    grub_config_file: 'grub2/grub.cfg',
  })

  useEffect(() => { loadSettings() }, [])

  async function loadSettings() {
    setLoading(true)
    try {
      const res = await api.getTFTPSettings()
      const d = res.data
      setConfig({
        enabled: true,
        port: d.port || 69,
        timeout: d.timeout || 5,
        root: d.root || '',
        pxe_config_file: d.pxe_config_file || 'pxelinux.cfg/default',
        grub_config_file: d.grub_config_file || 'grub2/grub.cfg',
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
      await api.updateTFTPSettings({ ...config, enabled: true })
      success(t('settings.saved'))
    } catch (err: any) {
      showError(err.message || t('settings.saveFailed'))
    } finally {
      setSaving(false)
    }
  }

  return (
    <div>
      <PageHeader
        title={t('settings.tftpTitle')}
        actions={
          <>
            <Button variant="secondary" size="sm" disabled={loading} onClick={loadSettings}>
              <RefreshCw size={14} /> {t('settings.refresh')}
            </Button>
            <Button variant="primary" size="sm" disabled={saving} onClick={handleSave}>
              <Save size={14} /> {saving ? t('settings.saving') : t('settings.save')}
            </Button>
          </>
        }
      />

      <Card>
        {loading ? (
          <div className="flex items-center justify-center py-16">
            <div className="animate-spin w-6 h-6 border-2 border-blue-500 border-t-transparent rounded-full" />
            <span className="ml-3 text-sm text-[var(--text-muted)]">{t('settings.loading')}</span>
          </div>
        ) : (
          <div className="grid grid-cols-2 gap-4">
            <SettingsField label={t('settings.portLabel', '端口')} help={t('settings.portHelp', 'TFTP 服务监听端口，默认 69')}>
              <SettingsInput
                type="number"
                min={1}
                max={65535}
                value={String(config.port)}
                onChange={v => setConfig({...config, port: parseInt(v) || 69})}
              />
            </SettingsField>
            <SettingsField label={t('settings.timeoutLabel', '传输超时（秒）')} help={t('settings.timeoutHelp', 'TFTP 传输超时时间，默认 5 秒')}>
              <SettingsInput
                type="number"
                min={1}
                max={120}
                value={String(config.timeout)}
                onChange={v => setConfig({...config, timeout: parseInt(v) || 5})}
              />
            </SettingsField>
            <SettingsField label={t('settings.tftpRoot', '根目录')} help={t('settings.tftpRootHelp', 'TFTP 文件根目录')}>
              <SettingsInput value={config.root} onChange={v => setConfig({...config, root: v})} />
            </SettingsField>
            <SettingsField label={t('settings.pxeConfigPath')} help={t('settings.pxeConfigHelp')}>
              <SettingsInput value={config.pxe_config_file} onChange={v => setConfig({...config, pxe_config_file: v})} placeholder="pxelinux.cfg/default" />
            </SettingsField>
            <SettingsField label={t('settings.grub2ConfigPath')} help={t('settings.grub2ConfigHelp')}>
              <SettingsInput value={config.grub_config_file} onChange={v => setConfig({...config, grub_config_file: v})} placeholder="grub2/grub.cfg" />
            </SettingsField>
          </div>
        )}
      </Card>
    </div>
  )
}
