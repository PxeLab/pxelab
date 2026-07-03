import { useState, useEffect, useCallback } from 'react'
import { useTranslation } from 'react-i18next'
import { Save, RefreshCw, Copy, Check, RotateCw } from 'lucide-react'
import { Card } from '../components/ui/Card'
import { Button } from '../components/ui/Button'
import { Toggle } from '../components/ui/Toggle'
import { useToast } from '../components/ui/Toast'
import { SettingsField, SettingsInput } from '../components/settings/SettingsField'
import { api, type GeneralSettings, type InterfaceInfo, type NetbootSettingsData } from '../api/client'
import { useUIConfig } from '../contexts/UIConfigContext'

export default function SettingsGeneral() {
  const { t } = useTranslation()
  const { success, error: showError } = useToast()
  const { setPageSize } = useUIConfig()
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [tokenCopied, setTokenCopied] = useState(false)
  const [availableIfaces, setAvailableIfaces] = useState<InterfaceInfo[]>([])
  const [originalDataDir, setOriginalDataDir] = useState('')
  const [migrateBoot, setMigrateBoot] = useState(false)
  const [netbootConfig, setNetbootConfig] = useState<NetbootSettingsData>({
    enabled: false,
    proxy_https: true,
    cache_enabled: true,
    catalog_redirect: { enabled: true, target_url: '', detect_arch: true, preamble: '' },
    catalog_display: { title: '' },
  })
  const [config, setConfig] = useState<GeneralSettings>({
    server_name: 'pxego',
    app_mode: false,
    token: '',
    token_set: false,
    listen_addr: '127.0.0.1:8080',
    log_level: 'info',
    data_dir: '',
    whitelist_enabled: false,
    script_template: '',
    page_size: 50,
    default_menu: {
      title: 'PxeGo Boot Menu',
      timeout: 10,
      default: 0,
      list_all_profiles: false,
      entries: [],
    },
  })

  useEffect(() => { loadSettings() }, [])

  async function loadSettings() {
    setLoading(true)
    try {
      const [res, ifaceRes, netbootRes] = await Promise.all([api.getGeneralSettings(), api.getInterfaces(), api.getNetbootSettings()])
      setAvailableIfaces(ifaceRes.data)
      const originalDir = res.data.data_dir || ''
      setOriginalDataDir(originalDir)
      setMigrateBoot(false)
      const d = res.data
      setConfig(prev => ({
        ...prev,
        server_name: d.server_name || 'pxego',
        log_level: d.log_level || prev.log_level,
        data_dir: d.data_dir || prev.data_dir,
        listen_addr: d.listen_addr || prev.listen_addr,
        token: d.token || prev.token,
        token_set: d.token_set ?? false,
        whitelist_enabled: d.whitelist_enabled ?? false,
        app_mode: d.app_mode ?? prev.app_mode,
        script_template: d.script_template ?? prev.script_template,
        page_size: d.page_size || prev.page_size,
        default_menu: d.default_menu ? {
          title: d.default_menu.title || prev.default_menu.title,
          timeout: d.default_menu.timeout ?? prev.default_menu.timeout,
          default: d.default_menu.default ?? prev.default_menu.default,
          list_all_profiles: d.default_menu.list_all_profiles ?? false,
          entries: d.default_menu.entries || [],
        } : prev.default_menu,
      }))
      const nd = netbootRes.data
      setNetbootConfig({
        enabled: nd.enabled ?? false,
        proxy_https: nd.proxy_https ?? true,
        cache_enabled: nd.cache_enabled ?? true,
        catalog_redirect: {
          enabled: nd.catalog_redirect?.enabled ?? true,
          target_url: nd.catalog_redirect?.target_url || 'http://{{.URL}}/netboot/menu.ipxe?arch=${arch}&platform=${platform}',
          detect_arch: nd.catalog_redirect?.detect_arch ?? true,
          preamble: nd.catalog_redirect?.preamble || '',
        },
        catalog_display: {
          title: nd.catalog_display?.title || '[Netboot] OS Install Catalog',
        },
      })
    } catch (err: any) {
      showError(err.message || t('settings.loadFailed'))
    } finally {
      setLoading(false)
    }
  }

  const regenerateToken = useCallback(async () => {
    const newToken = Array.from({ length: 32 }, () => Math.random().toString(36)[2]).join('')
    setConfig(prev => ({ ...prev, token: newToken }))
  }, [])

  const copyToken = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(config.token)
      setTokenCopied(true)
      setTimeout(() => setTokenCopied(false), 2000)
    } catch {}
  }, [config.token])

  async function handleSave() {
    setSaving(true)
    try {
      await Promise.all([
        api.updateGeneralSettings({ ...config, migrate_boot: migrateBoot }),
        api.updateNetbootSettings(netbootConfig),
      ])
      setPageSize(config.page_size)
      success(t('settings.saved'))
    } catch (err: any) {
      showError(err.message || t('settings.saveFailed'))
    } finally {
      setSaving(false)
    }
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-lg font-bold text-[var(--text-primary)]">{t('settings.general')}</h1>
        <div className="flex gap-2">
          <Button variant="secondary" size="sm" disabled={loading} onClick={loadSettings}>
            <RefreshCw size={14} /> {t('common.reload', '重载配置')}
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
            {/* ── 基本设置 ── */}
            <h3 className="text-sm font-semibold text-[var(--text-primary)]">{t('settings.basicSettings')}</h3>
            <SettingsField label={t('common.serverName', '服务器名称')}>
              <SettingsInput value={config.server_name} onChange={v => setConfig({...config, server_name: v})} />
            </SettingsField>
            <SettingsField label={t('common.logLevel', '日志级别')}>
              <select className="w-full bg-[var(--bg-input)] border border-[var(--bg-border)] rounded-lg px-3.5 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500 appearance-none"
                value={config.log_level} onChange={e => setConfig({...config, log_level: e.target.value})}>
                <option>info</option>
                <option>debug</option>
                <option>warn</option>
                <option>error</option>
              </select>
            </SettingsField>
            <SettingsField label={t('settings.perPage')}>
              <input type="number" min={5} max={500}
                value={config.page_size}
                onChange={e => setConfig({...config, page_size: parseInt(e.target.value) || 50})}
                className="w-full px-3.5 py-2 text-sm rounded-lg border border-[var(--bg-border)] bg-[var(--bg-input)] text-[var(--text-primary)] outline-none focus:border-blue-500" />
              <p className="text-xs text-[var(--text-muted)] mt-1">{t('settings.perPageHelp')}</p>
            </SettingsField>
            <SettingsField label={t('settings.dataDir')}>
              <div className="flex gap-2">
                <SettingsInput value={config.data_dir} onChange={v => { setConfig({...config, data_dir: v}); setMigrateBoot(false) }} className="flex-1" />
              </div>
              {originalDataDir && config.data_dir !== originalDataDir && (
                <label className="flex items-center gap-2 mt-2 cursor-pointer" onClick={e => e.stopPropagation()}>
                  <input type="checkbox" checked={migrateBoot} onChange={e => setMigrateBoot(e.target.checked)}
                    className="rounded border-[var(--bg-border)] bg-[var(--bg-input)]" />
                  <span className="text-xs text-[var(--text-muted)]">{t('settings.migrateBoot')}</span>
                </label>
              )}
            </SettingsField>
            <SettingsField label={t('common.mode', '运行模式')}>
              <select className="w-full bg-[var(--bg-input)] border border-[var(--bg-border)] rounded-lg px-3.5 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500 appearance-none"
                value={config.app_mode ? 'app' : 'server'} onChange={e => setConfig({...config, app_mode: e.target.value === 'app'})}>
                <option value="server">server</option>
                <option value="app">app</option>
              </select>
            </SettingsField>

            <SettingsField label={t('settings.listenAddr', '管理接口监听地址')}>
              <div className="flex gap-2">
                <SettingsInput value={config.listen_addr}
                  onChange={v => setConfig({...config, listen_addr: v})}
                  placeholder="127.0.0.1:8080"
                  className="flex-1 font-mono" />
                <select
                  className="bg-[var(--bg-card)] border border-[var(--bg-border)] rounded-lg px-3 py-2 text-xs text-[var(--text-primary)] outline-none focus:border-blue-500 appearance-none cursor-pointer min-w-[100px]"
                  value=""
                  onChange={e => {
                    const v = e.target.value
                    if (!v) return
                    const port = config.listen_addr.split(':')[1] || '8080'
                    if (v === '*') {
                      setConfig({...config, listen_addr: '0.0.0.0:' + port})
                    } else {
                      const sel = availableIfaces.find(x => x.name === v)
                      if (sel?.ipv4?.[0]) {
                        setConfig({...config, listen_addr: sel.ipv4[0] + ':' + port})
                      }
                    }
                  }}
                >
                  <option value="">{t('settings.selectInterface')}</option>
                  <option value="*">{t('settings.allInterfaces')} (0.0.0.0:{config.listen_addr.split(':')[1] || '8080'})</option>
                  {availableIfaces.filter(ai => ai.up && ai.ipv4?.length > 0).map(ai => (
                    <option key={ai.name} value={ai.name}>{ai.name} ({ai.ipv4[0]})</option>
                  ))}
                </select>
              </div>
              <p className="text-xs text-[var(--text-muted)] mt-1">{t('settings.listenAddrHelp')}</p>
            </SettingsField>

            {/* API 令牌 */}
            <SettingsField label={t('settings.authToken')}>
              <div className="flex items-center gap-2">
                <input
                  type="text"
                  value={config.token.length === 32 && /^[0-9a-f]+$/i.test(config.token)
                    ? config.token.slice(0, 4) + '...' + config.token.slice(-4)
                    : config.token || t('settings.notSet')}
                  readOnly
                  className="flex-1 bg-[var(--bg-input)] border border-[var(--bg-border)] rounded-lg px-3.5 py-2 text-sm text-[var(--text-primary)] font-mono outline-none select-all"
                />
                <button onClick={copyToken}
                  className="p-2 rounded-lg border border-[var(--bg-border)] hover:bg-[var(--bg-hover)] transition-colors text-[var(--text-secondary)]"
                  title={t('settings.copyToken')}>
                  {tokenCopied ? <Check size={16} className="text-green-500" /> : <Copy size={16} />}
                </button>
                <button onClick={regenerateToken}
                  className="p-2 rounded-lg border border-[var(--bg-border)] hover:bg-[var(--bg-hover)] transition-colors text-[var(--text-secondary)]"
                  title={t('settings.regenerate')}>
                  <RotateCw size={16} />
                </button>
              </div>
              {config.token && !config.token.includes('...') && (
                <div className="mt-2 px-3 py-2 bg-amber-500/10 border border-amber-500/30 rounded-lg text-xs text-amber-400">
                  {t('settings.newTokenWarning')}
                </div>
              )}
              <p className="text-xs text-[var(--text-muted)] mt-1">{t('settings.tokenHelp')}</p>
            </SettingsField>

            <div className="pt-2">
              <label className="flex items-center justify-between gap-4 py-2 border-b border-[var(--bg-border)]">
                <span className="text-sm text-[var(--text-secondary)]">{t('settings.autoOpen', '启动时自动打开浏览器（app 模式）')}</span>
                <Toggle checked={config.app_mode} onChange={v => setConfig({...config, app_mode: v})} />
              </label>
              <label className="flex items-center justify-between gap-4 py-2 border-b border-[var(--bg-border)]">
                <span className="text-sm text-[var(--text-secondary)]">{t('settings.enableWhitelist')}</span>
                <Toggle checked={config.whitelist_enabled} onChange={v => setConfig({...config, whitelist_enabled: v})} />
              </label>
            </div>

            {/* ── Netboot ── */}
            <div className="pt-4 border-t border-[var(--bg-border)]">
              <h3 className="text-sm font-semibold text-[var(--text-primary)] mb-3">Netboot</h3>
              <Toggle checked={netbootConfig.enabled} onChange={v => setNetbootConfig({...netbootConfig, enabled: v})} label={t('settings.enableNetbootMenu')} />
              <p className="text-xs text-[var(--text-muted)] mt-1 mb-4">
                {t('settings.netbootMenuHelp')}
              </p>
              <SettingsField label={t('settings.httpsProxy')}>
                <div className="flex items-center gap-3">
                  <Toggle checked={netbootConfig.proxy_https} onChange={v => setNetbootConfig({...netbootConfig, proxy_https: v})} />
                  <span className="text-xs text-[var(--text-muted)]">{t('settings.httpsProxyHelp')}</span>
                </div>
              </SettingsField>
              <SettingsField label={t('settings.menuTitle')}>
                <SettingsInput value={netbootConfig.catalog_display.title}
                  onChange={v => setNetbootConfig({...netbootConfig, catalog_display: {...netbootConfig.catalog_display, title: v}})} />
              </SettingsField>
              <p className="text-xs text-[var(--text-muted)] -mt-3">{t('settings.menuTitleHelp')}</p>

              <div className="pt-4 mt-4 border-t border-[var(--bg-border)]">
                <h4 className="text-sm font-medium text-[var(--text-primary)] mb-2">{t('settings.redirectSection')}</h4>
                <p className="text-xs text-[var(--text-muted)] mb-3">{t('settings.redirectHelp')}</p>
                <div className="flex flex-col gap-4">
                  <Toggle checked={netbootConfig.catalog_redirect.enabled}
                    onChange={v => setNetbootConfig({...netbootConfig, catalog_redirect: {...netbootConfig.catalog_redirect, enabled: v}})}
                    label={t('settings.enableRedirect')} />
                  <SettingsField label={t('settings.targetUrl')}>
                    <SettingsInput value={netbootConfig.catalog_redirect.target_url}
                      onChange={v => setNetbootConfig({...netbootConfig, catalog_redirect: {...netbootConfig.catalog_redirect, target_url: v}})}
                      className="font-mono" />
                    <div className="flex gap-2 mt-2">
                      {[
                        { label: t('settings.localNetboot'), url: 'http://{{.URL}}/netboot/menu.ipxe?arch=${arch}&platform=${platform}' },
                        { label: 'netboot.xyz', url: 'http://boot.netboot.xyz/menu.ipxe' },
                      ].map(p => (
                        <button key={p.label}
                          onClick={() => setNetbootConfig({...netbootConfig, catalog_redirect: {...netbootConfig.catalog_redirect, target_url: p.url}})}
                          className={`text-xs px-3 py-1.5 rounded-lg border font-medium transition-colors ${
                            netbootConfig.catalog_redirect.target_url === p.url
                              ? 'bg-blue-500/15 text-blue-400 border-blue-500/30'
                              : 'bg-[var(--bg-card)] text-[var(--text-secondary)] border-[var(--bg-border)] hover:border-blue-500/30 hover:text-blue-400'
                          }`}
                        >
                          {p.label}
                        </button>
                      ))}
                    </div>
                  </SettingsField>
                  <p className="text-xs text-[var(--text-muted)] -mt-3">{t('settings.redirectUrlHelp')}</p>
                  <SettingsField label={t('settings.preamble')}>
                    <textarea rows={3} spellCheck={false}
                      className="w-full bg-[var(--bg-input)] border border-[var(--bg-border)] rounded-lg px-4 py-3 text-sm font-mono text-[var(--text-primary)] outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-500/10 transition-all"
                      value={netbootConfig.catalog_redirect.preamble}
                      onChange={e => setNetbootConfig({...netbootConfig, catalog_redirect: {...netbootConfig.catalog_redirect, preamble: e.target.value}})}
                      placeholder={t('settings.preamblePlaceholder')} />
                  </SettingsField>
                </div>
              </div>
            </div>
          </div>
        )}
      </Card>
    </div>
  )
}
