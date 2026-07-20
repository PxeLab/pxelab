import { useState, useEffect, useCallback, useMemo } from 'react'
import { createPortal } from 'react-dom'
import { useTranslation } from 'react-i18next'
import { X, Save, Copy, Check, RotateCw, Settings as SettingsIcon, Monitor, FileCode, Activity, HardDrive } from 'lucide-react'
import { Toggle } from '../../components/ui/Toggle'
import { Button } from '../../components/ui/Button'
import { useToast } from '../../components/ui/Toast'
import { useUIConfig } from '../../contexts/UIConfigContext'
import {
  getGeneralSettings, updateGeneralSettings,
  getNetbootSettings, updateNetbootSettings,
  getInterfaces, getServices, updateAutoStart,
  getCacheStats,
  getLoggingSettings, updateLoggingSettings,
  getLogFiles, getLogDiskUsage, cleanupLogs,
  type GeneralSettings, type NetbootSettingsData, type InterfaceInfo, type ServiceInfo, type CacheStats,
  type LoggingSettings, type LogFileInfo,
} from '../../api/client'

interface Props {
  open: boolean
  onClose: () => void
}

type Section = 'general' | 'boot' | 'netboot' | 'services' | 'logging'

interface NavItem {
  key: Section
  label: string
  icon: typeof SettingsIcon
}

export default function SettingsModal({ open, onClose }: Props) {
  const { t } = useTranslation()
  const { success, error: showError } = useToast()
  const { setPageSize } = useUIConfig()
  const [section, setSection] = useState<Section>('general')
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [tokenCopied, setTokenCopied] = useState(false)
  const [availableIfaces, setAvailableIfaces] = useState<InterfaceInfo[]>([])
  const [general, setGeneral] = useState<GeneralSettings | null>(null)
  const [netboot, setNetboot] = useState<NetbootSettingsData | null>(null)
  const [services, setServices] = useState<ServiceInfo[]>([])
  const [logging, setLogging] = useState<LoggingSettings | null>(null)
  const [originalDataDir, setOriginalDataDir] = useState('')

  useEffect(() => {
    if (!open) return
    setGeneral(null)
    setNetboot(null)
    setLogging(null)
    setServices([])
    setLoading(true)
    setTokenCopied(false)
    setSection('general')
    Promise.all([getGeneralSettings(), getNetbootSettings(), getInterfaces(), getServices(), getLoggingSettings()])
      .then(([g, nb, ifaces, svc, log]) => {
        const gen = g.data as unknown as GeneralSettings
        setGeneral(gen)
        setOriginalDataDir(gen.data_dir || '')
        setNetboot(nb.data as unknown as NetbootSettingsData)
        setAvailableIfaces(ifaces.data as unknown as InterfaceInfo[])
        setServices(svc.data as unknown as ServiceInfo[])
        setLogging(log.data as unknown as LoggingSettings)
      })
      .catch(console.error)
      .finally(() => setLoading(false))
  }, [open])

  const regenerateToken = useCallback(() => {
    if (!general) return
    const newToken = Array.from({ length: 32 }, () => Math.random().toString(36)[2]).join('')
    setGeneral({ ...general, token: newToken })
  }, [general])

  const copyToken = useCallback(async () => {
    if (!general) return
    try {
      await navigator.clipboard.writeText(general.token)
      setTokenCopied(true)
      setTimeout(() => setTokenCopied(false), 2000)
    } catch {}
  }, [general])

  const handleSave = async () => {
    setSaving(true)
    try {
      const promises: Promise<unknown>[] = []
      if (section === 'general' && general) promises.push(updateGeneralSettings(general))
      if (section === 'netboot' && netboot) promises.push(updateNetbootSettings(netboot))
      if (section === 'logging' && logging) promises.push(updateLoggingSettings(logging))
      await Promise.all(promises)
      if (section === 'general' && general?.page_size) setPageSize(general.page_size)
      success(t('settings.saved'))
    } catch (e: any) {
      showError(e?.message || t('settings.saveFailed'))
    } finally {
      setSaving(false)
    }
  }

  if (!open) return null

  const navItems: NavItem[] = [
    { key: 'general', label: t('nav.settings.general'), icon: SettingsIcon },
    { key: 'boot', label: t('nav.settings.bootMenu'), icon: FileCode },
    { key: 'netboot', label: t('nav.settings.netboot'), icon: Monitor },
    { key: 'services', label: t('settings.modalServiceAutoStart'), icon: Activity },
    { key: 'logging', label: t('settings.logging', '日志管理'), icon: HardDrive },
  ]

  return createPortal(
    <div className="fixed inset-0 z-[100] flex items-center justify-center bg-black/60 backdrop-blur-sm animate-overlay-in" onClick={onClose}>
      <div className="flex w-full h-full md:w-[75vw] md:h-[80vh] md:max-w-[900px] md:rounded-xl bg-[var(--bg-elevated)] border border-[var(--bg-border)] overflow-hidden" onClick={e => e.stopPropagation()}>
        {/* Sidebar */}
        <aside className="w-[160px] shrink-0 bg-[var(--bg-base)] border-r border-[var(--bg-border)] flex flex-col overflow-hidden">
          <div className="flex items-center h-14 px-4 border-b border-[var(--bg-border)] shrink-0">
            <span className="font-bold text-sm">{t('settings.title')}</span>
          </div>
          <nav className="flex-1 overflow-y-auto min-h-0 p-2 space-y-0.5">
            {navItems.map(item => {
              const Icon = item.icon
              const active = section === item.key
              return (
                <button key={item.key} onClick={() => setSection(item.key)}
                  className={`w-full flex items-center gap-2.5 px-3 py-2 rounded-lg text-sm font-medium transition-colors ${
                    active ? 'bg-blue-500/10 text-blue-400' : 'text-[var(--text-secondary)] hover:bg-[var(--bg-card)] hover:text-[var(--text-primary)]'
                  }`}>
                  <Icon size={16} className="shrink-0 opacity-70" />
                  <span>{item.label}</span>
                </button>
              )
            })}
          </nav>
        </aside>

        {/* Content */}
        <div className="flex-1 flex flex-col min-w-0 overflow-hidden">
          {/* Header */}
          <div className="flex items-center justify-between h-14 px-5 border-b border-[var(--bg-border)] shrink-0">
            <span className="font-bold text-sm">{navItems.find(n => n.key === section)?.label}</span>
            <div className="flex items-center gap-2">
              <Button variant="primary" size="sm" onClick={handleSave} disabled={saving}>
                <Save size={14} />
                {saving ? t('settings.loading') : t('settings.save')}
              </Button>
              <button onClick={onClose} className="p-1.5 rounded-lg hover:bg-[var(--bg-card)] text-[var(--text-muted)] hover:text-[var(--text-primary)] transition-colors">
                <X size={18} />
              </button>
            </div>
          </div>

          {/* Body */}
          <div className="flex-1 overflow-y-auto">
            {loading ? (
              <div className="flex items-center justify-center py-16">
                <div className="animate-spin w-6 h-6 border-2 border-blue-500 border-t-transparent rounded-full" />
                <span className="ml-3 text-sm text-[var(--text-muted)]">{t('settings.loading')}</span>
              </div>
            ) : (
              <>
                {section === 'general' && general && <GeneralForm config={general} onChange={setGeneral} tokenCopied={tokenCopied} onCopy={copyToken} onRegenerate={regenerateToken} ifaces={availableIfaces} originalDataDir={originalDataDir} />}
                {section === 'boot' && general && <BootForm config={general} onChange={setGeneral} />}
                {section === 'netboot' && netboot && <NetbootForm data={netboot} onChange={setNetboot} />}
                {section === 'services' && services.length > 0 && <ServicesForm services={services} onReload={async () => { try { const res = await getServices(); setServices(res.data as unknown as ServiceInfo[]) } catch {} }} />}
                {section === 'logging' && logging && <LoggingForm config={logging} onChange={setLogging} />}
              </>
            )}
          </div>
        </div>
      </div>
    </div>,
    document.body
  )
}

// ── General Settings ──

function GeneralForm({ config, onChange, tokenCopied, onCopy, onRegenerate, ifaces, originalDataDir }: {
  config: GeneralSettings
  onChange: (c: GeneralSettings) => void
  tokenCopied: boolean
  onCopy: () => void
  onRegenerate: () => void
  ifaces: InterfaceInfo[]
  originalDataDir?: string
}) {
  const { t } = useTranslation()
  const inputCls = 'w-full bg-[var(--bg-card)] border border-[var(--bg-border)] rounded-lg px-3.5 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500'
  const selectCls = 'w-full bg-[var(--bg-card)] border border-[var(--bg-border)] rounded-lg px-3.5 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500 appearance-none cursor-pointer'

  const serverNameValid = useMemo(() => {
    const v = config.server_name
    return v === '' || /^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$/.test(v)
  }, [config.server_name])

  return (
    <div className="p-6 space-y-4">
      <h3 className="text-sm font-semibold text-[var(--text-primary)]">{t('settings.modalBasicSettings')}</h3>
      <SettingsField label={t('common.serverName', '服务器名称')}>
        <input className={inputCls} value={config.server_name} onChange={e => onChange({...config, server_name: e.target.value})} />
        {!serverNameValid && (
          <p className="text-xs text-amber-400 mt-1">{t('common.serverNameHint')}</p>
        )}
      </SettingsField>
      <SettingsField label={t('common.logLevel', '日志级别')}>
        <select className={selectCls} value={config.log_level} onChange={e => onChange({...config, log_level: e.target.value})}>
          <option>info</option><option>debug</option><option>warn</option><option>error</option>
        </select>
      </SettingsField>
      <SettingsField label={t('settings.modalPerPage')}>
        <input type="number" min={5} max={500} value={config.page_size}
          onChange={e => onChange({...config, page_size: parseInt(e.target.value) || 50})}
          className="w-full bg-[var(--bg-card)] border border-[var(--bg-border)] rounded-lg px-3.5 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500" />
        <p className="text-xs text-[var(--text-muted)] mt-1">{t('settings.modalPerPageHelp')}</p>
      </SettingsField>
      <SettingsField label={t('settings.dataDir')}>
        <div className="flex gap-2">
          <input className={`${inputCls} flex-1`} value={config.data_dir} onChange={e => onChange({...config, data_dir: e.target.value, migrate_boot: undefined})} />
        </div>
        {originalDataDir && config.data_dir !== originalDataDir && (
          <label className="flex items-center gap-2 mt-2 cursor-pointer" onClick={e => e.stopPropagation()}>
            <input type="checkbox" checked={!!config.migrate_boot} onChange={e => onChange({...config, migrate_boot: e.target.checked})}
              className="rounded border-[var(--bg-border)] bg-[var(--bg-input)]" />
            <span className="text-xs text-[var(--text-muted)]">{t('settings.modalMigrateBoot')}</span>
          </label>
        )}
      </SettingsField>
      <SettingsField label={t('common.mode', '运行模式')}>
        <select className={selectCls} value={config.app_mode ? 'app' : 'server'} onChange={e => onChange({...config, app_mode: e.target.value === 'app'})}>
          <option value="server">server</option>
          <option value="app">app</option>
        </select>
      </SettingsField>

      <SettingsField label={t('settings.listenAddr', '管理接口监听地址')}>
        <div className="flex gap-2">
          <input className={`${inputCls} flex-1 font-mono min-w-0`} value={config.listen_addr} onChange={e => onChange({...config, listen_addr: e.target.value})} placeholder="127.0.0.1:8080" />
          <select className={selectCls + ' flex-1 min-w-0'} value="" onChange={e => {
            const v = e.target.value
            if (!v) return
            const port = config.listen_addr.split(':')[1] || '8080'
            if (v === '*') {
              onChange({...config, listen_addr: '0.0.0.0:' + port})
            } else {
              const sel = ifaces.find(x => x.name === v)
              if (sel?.ipv4?.[0]) {
                onChange({...config, listen_addr: sel.ipv4[0] + ':' + port})
              }
            }
          }}>
            <option value="">{t('settings.modalSelectInterface')}</option>
            <option value="*">{t('settings.allInterfaces')} (0.0.0.0:{config.listen_addr.split(':')[1] || '8080'})</option>
            {ifaces.filter(ai => ai.up && ai.ipv4?.length > 0).map(ai => (
              <option key={ai.name} value={ai.name}>{ai.name} ({ai.ipv4[0]})</option>
            ))}
          </select>
        </div>
        <p className="text-xs text-[var(--text-muted)] mt-1">{t('settings.modalListenAddrHelp')}</p>
      </SettingsField>

      <SettingsField label={t('settings.authToken')}>
        <div className="flex items-center gap-2">
          <input type="text" readOnly
            value={config.token.length === 32 && /^[0-9a-f]+$/i.test(config.token)
              ? config.token.slice(0, 4) + '...' + config.token.slice(-4)
              : config.token || t('settings.notSet')}
            className="flex-1 bg-[var(--bg-card)] border border-[var(--bg-border)] rounded-lg px-3.5 py-2 text-sm text-[var(--text-primary)] font-mono outline-none select-all" />
          <button onClick={onCopy} className="p-2 rounded-lg border border-[var(--bg-border)] hover:bg-[var(--bg-card)] transition-colors text-[var(--text-secondary)]" title={t('settings.modalCopy')}>
            {tokenCopied ? <Check size={16} className="text-green-500" /> : <Copy size={16} />}
          </button>
          <button onClick={onRegenerate} className="p-2 rounded-lg border border-[var(--bg-border)] hover:bg-[var(--bg-card)] transition-colors text-[var(--text-secondary)]" title={t('settings.modalRegenerate')}>
            <RotateCw size={16} />
          </button>
        </div>
        <p className="text-xs text-[var(--text-muted)] mt-1">{t('settings.modalTokenHelp')}</p>

        {config.token && !config.token.includes('...') && (
          <div className="mt-2 px-3 py-2 bg-amber-500/10 border border-amber-500/30 rounded-lg text-xs text-amber-400">
            {t('settings.modalNewTokenWarning')}
          </div>
        )}
      </SettingsField>

      <div className="pt-2">
        <label className="flex items-center justify-between gap-4 py-2 border-b border-[var(--bg-border)]">
          <span className="text-sm text-[var(--text-secondary)]">{t('settings.autoOpen', '启动时自动打开浏览器（app 模式）')}</span>
          <Toggle checked={config.app_mode} onChange={v => onChange({...config, app_mode: v})} />
        </label>
        <label className="flex items-center justify-between gap-4 py-2 border-b border-[var(--bg-border)]">
          <span className="text-sm text-[var(--text-secondary)]">{t('settings.modalEnableWhitelist')}</span>
          <Toggle checked={config.whitelist_enabled} onChange={v => onChange({...config, whitelist_enabled: v})} />
        </label>
      </div>
    </div>
  )
}

// ── Boot Customization ──

function BootForm({ config, onChange }: { config: GeneralSettings; onChange: (c: GeneralSettings) => void }) {
  const { t } = useTranslation()
  const inputCls = 'w-full bg-[var(--bg-card)] border border-[var(--bg-border)] rounded-lg px-3.5 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500'
  return (
    <div className="p-6 space-y-4">
      <h3 className="text-sm font-semibold text-[var(--text-primary)]">{t('settings.modalBootCustomization')}</h3>

      <SettingsField label={t('settings.modalCustomIpxeScript')}>
        <p className="text-xs text-[var(--text-muted)] mb-2">{t('settings.modalCustomIpxeHelp')}</p>
        <textarea value={config.script_template} onChange={e => onChange({...config, script_template: e.target.value})}
          rows={6} spellCheck={false}
          className="w-full bg-[var(--bg-card)] border border-[var(--bg-border)] rounded-lg px-4 py-3 text-sm font-mono text-[var(--text-primary)] outline-none focus:border-blue-500"
          placeholder={'#!ipxe\nmenu PxeLab Boot Menu\nitem shell iPXE Shell\nitem local Boot Local Disk\n\nchoose selected || shell\ngoto ${selected}'} />
      </SettingsField>

      <div className="pt-4 border-t border-[var(--bg-border)]">
        <h4 className="text-xs font-semibold text-[var(--text-primary)] mb-2">{t('settings.modalDefaultBootMenu')}</h4>
        <p className="text-xs text-[var(--text-muted)] mb-3">{t('settings.modalBootMenuHelp')}</p>
        <SettingsField label={t('settings.modalTimeout')}>
          <input className={inputCls} type="number" value={config.default_menu.timeout}
            onChange={e => onChange({...config, default_menu: {...config.default_menu, timeout: parseInt(e.target.value) || 0}})} />
        </SettingsField>
        <label className="flex items-center justify-between gap-4 py-2 border-b border-[var(--bg-border)]">
          <span className="text-sm text-[var(--text-secondary)]">{t('settings.modalListAllProfiles')}</span>
          <Toggle checked={config.default_menu.list_all_profiles}
            onChange={v => onChange({...config, default_menu: {...config.default_menu, list_all_profiles: v}})} />
        </label>
        <p className="text-xs text-[var(--text-muted)] mt-1">{t('settings.modalListAllProfilesHelp')}</p>
      </div>
    </div>
  )
}

// ── Netboot ──

function NetbootForm({ data, onChange }: { data: NetbootSettingsData; onChange: (d: NetbootSettingsData) => void }) {
  const { t } = useTranslation()
  const inputCls = 'w-full bg-[var(--bg-card)] border border-[var(--bg-border)] rounded-lg px-3.5 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500'
  const [cacheStats, setCacheStats] = useState<CacheStats | null>(null)
  const [statsLoading, setStatsLoading] = useState(false)

  const fetchCacheStats = useCallback(async () => {
    if (!data.cache_enabled) { setCacheStats(null); return }
    setStatsLoading(true)
    try {
      const res = await getCacheStats()
      setCacheStats(res.data as unknown as CacheStats)
    } catch { setCacheStats(null) }
    setStatsLoading(false)
  }, [data.cache_enabled])

  useEffect(() => { fetchCacheStats() }, [fetchCacheStats])

  const fmtBytes = (b: number) => {
    if (b === 0) return '0 B'
    const u = ['B', 'KB', 'MB', 'GB']
    const i = Math.floor(Math.log(b) / Math.log(1024))
    return (b / Math.pow(1024, i)).toFixed(1) + ' ' + u[i]
  }

  return (
    <div className="p-6 space-y-4">
      <h3 className="text-sm font-semibold">{t('settings.modalNetbootSettings')}</h3>
      <SettingsField label={t('settings.modalEnableNetboot')}>
        <Toggle checked={data.enabled} onChange={v => onChange({...data, enabled: v})} />
      </SettingsField>
      <p className="text-xs text-[var(--text-muted)] -mt-2">{t('settings.modalNetbootHelp')}</p>
      <SettingsField label={t('settings.modalHttpsProxy')}>
        <Toggle checked={data.proxy_https} onChange={v => onChange({...data, proxy_https: v})} />
        <p className="text-xs text-[var(--text-muted)] mt-1">{t('settings.modalHttpsProxyHelp')}</p>
      </SettingsField>
      <SettingsField label={t('settings.modalCache')}>
        <div className="flex flex-col gap-1">
          <Toggle checked={data.cache_enabled} onChange={v => onChange({...data, cache_enabled: v})} />
          <p className="text-xs text-[var(--text-muted)]">{t('settings.modalCacheHelp')}</p>
          {data.cache_enabled && cacheStats && (
            <div className="mt-1 space-y-0.5">
              <div className="text-xs text-[var(--text-muted)]">
                {t('settings.cachePath')}: <code className="text-[var(--text-secondary)]">{cacheStats.path}</code>
              </div>
              <div className="text-xs text-[var(--text-muted)]">
                {t('settings.cacheStats')}: {cacheStats.file_count} {t('settings.cacheFiles')} · {fmtBytes(cacheStats.size_bytes)}
              </div>
            </div>
          )}
          {data.cache_enabled && statsLoading && (
            <span className="text-xs text-[var(--text-muted)]">{t('settings.loading')}</span>
          )}
        </div>
      </SettingsField>
      <SettingsField label={t('settings.modalMenuTitle')}>
        <input className={inputCls} value={data.catalog_display.title}
          onChange={e => onChange({...data, catalog_display: {...data.catalog_display, title: e.target.value}})} />
      </SettingsField>

      <div className="pt-4 border-t border-[var(--bg-border)]">
        <h4 className="text-xs font-semibold text-[var(--text-primary)] mb-2">{t('settings.modalRedirectSection')}</h4>
        <p className="text-xs text-[var(--text-muted)] mb-3">{t('settings.modalRedirectHelp')}</p>
        <SettingsField label={t('settings.modalEnableRedirect')}>
          <Toggle checked={data.catalog_redirect?.enabled ?? false}
            onChange={v => onChange({...data, catalog_redirect: {...data.catalog_redirect, enabled: v}})} />
        </SettingsField>
        <SettingsField label={t('settings.modalTargetUrl')}>
          <div className="flex flex-col gap-2">
            <input className={`${inputCls} font-mono`} value={data.catalog_redirect.target_url}
              onChange={e => onChange({...data, catalog_redirect: {...data.catalog_redirect, target_url: e.target.value}})} />
            <div className="flex gap-2">
              {[
                { label: t('settings.modalLocalNetboot'), url: 'http://{{.URL}}/netboot/menu.ipxe?arch=${arch}&platform=${platform}' },
                { label: 'netboot.xyz', url: 'http://boot.netboot.xyz/menu.ipxe' },
              ].map(p => (
                <button key={p.label}
                  onClick={() => onChange({...data, catalog_redirect: {...data.catalog_redirect, target_url: p.url}})}
                  className={`text-[11px] px-2.5 py-1 rounded-lg border font-medium transition-colors ${
                    data.catalog_redirect.target_url === p.url
                      ? 'bg-blue-500/15 text-blue-400 border-blue-500/30'
                      : 'border-[var(--bg-border)] text-[var(--text-secondary)] hover:border-blue-500/30 hover:text-blue-400'
                  }`}
                >
                  {p.label}
                </button>
              ))}
            </div>
            <p className="text-[11px] text-[var(--text-muted)]">{t('settings.modalRedirectUrlHelp')}</p>
          </div>
        </SettingsField>
        <SettingsField label={t('settings.modalPreamble')}>
          <textarea rows={3} spellCheck={false}
            className="w-full bg-[var(--bg-card)] border border-[var(--bg-border)] rounded-lg px-3.5 py-2 text-sm font-mono text-[var(--text-primary)] outline-none focus:border-blue-500"
            value={data.catalog_redirect.preamble}
            onChange={e => onChange({...data, catalog_redirect: {...data.catalog_redirect, preamble: e.target.value}})}
            placeholder={t('settings.modalPreamblePlaceholder')} />
        </SettingsField>
      </div>
    </div>
  )
}

// ── Service Auto-Start ──

function ServicesForm({ services, onReload }: { services: ServiceInfo[]; onReload: () => Promise<void> }) {
  const { t } = useTranslation()
  const [operating, setOperating] = useState<Set<string>>(new Set())

  const toggleAutoStart = async (name: string, enabled: boolean) => {
    setOperating(prev => new Set(prev).add(name))
    try {
      await updateAutoStart(name, enabled)
      await onReload()
    } catch { /* ignore */ }
    setOperating(prev => { const next = new Set(prev); next.delete(name); return next })
  }

  return (
    <div className="p-6 space-y-4">
      <h3 className="text-sm font-semibold text-[var(--text-primary)]">{t('settings.modalServiceAutoStart')}</h3>
      <p className="text-xs text-[var(--text-muted)]">{t('settings.modalServiceAutoStartHelp')}</p>

      {/* 全局服务 */}
      {['http','tftp','dns','nfs'].filter(k => services.some(s => s.name === k)).map(k => {
        const svc = services.find(s => s.name === k)!
        return <ServiceAutoStartRow key={svc.name} svc={svc} operating={operating} onToggle={toggleAutoStart} />
      })}

      {/* 接口级服务 */}
      {['dhcp/','proxy/'].filter(prefix => services.some(s => s.name.startsWith(prefix))).length > 0 && (
        <>
          <div className="pt-2 pb-1">
            <span className="text-[11px] font-semibold uppercase tracking-wider text-[var(--text-muted)]">{t('settings.modalInterfaceServices')}</span>
          </div>
          {services.filter(s => s.name.startsWith('dhcp/') || s.name.startsWith('proxy/')).map(svc => (
            <ServiceAutoStartRow key={svc.name} svc={svc} operating={operating} onToggle={toggleAutoStart} />
          ))}
        </>
      )}
    </div>
  )
}

function ServiceAutoStartRow({ svc, operating, onToggle }: { svc: ServiceInfo; operating: Set<string>; onToggle: (name: string, enabled: boolean) => Promise<void> }) {
  const { t } = useTranslation()
  const isOperating = operating.has(svc.name)
  return (
    <label className="flex items-center justify-between gap-4 py-2.5 border-b border-[var(--bg-border)]">
      <div className="flex items-center gap-2.5 min-w-0">
        <span className="text-sm text-[var(--text-primary)] font-medium truncate">{svc.display}</span>
        <span className="text-[10px] font-mono text-[var(--text-muted)] shrink-0">{svc.port}/{svc.protocol}</span>
        {svc.protected && (
          <span className="text-[10px] font-semibold text-blue-400 bg-blue-500/10 px-1.5 py-0.5 rounded-full">Core</span>
        )}
      </div>
      <div className="flex items-center gap-2 shrink-0">
        <span className={`text-xs ${svc.auto_start ? 'text-green-400' : 'text-[var(--text-muted)]'}`}>
          {svc.auto_start ? t('settings.modalEnabled') : t('settings.modalDisabled')}
        </span>
        <Toggle
          checked={svc.auto_start}
          onChange={() => onToggle(svc.name, !svc.auto_start)}
          disabled={isOperating || svc.protected}
        />
      </div>
    </label>
  )
}

function SettingsField({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="flex items-center justify-between gap-4 py-2 border-b border-[var(--bg-border)]">
      <span className="text-sm text-[var(--text-secondary)] shrink-0 w-32">{label}</span>
      <div className="flex-1">{children}</div>
    </label>
  )
}

// ── Logging Settings ──

function LoggingForm({ config, onChange }: { config: LoggingSettings; onChange: (c: LoggingSettings) => void }) {
  const { t } = useTranslation()
  const { success, error: showError } = useToast()
  const inputCls = 'w-full bg-[var(--bg-card)] border border-[var(--bg-border)] rounded-lg px-3.5 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500'
  const [files, setFiles] = useState<LogFileInfo[]>([])
  const [diskSize, setDiskSize] = useState(0)
  const [logDir, setLogDir] = useState('')
  const [loadingFiles, setLoadingFiles] = useState(true)
  const [cleaning, setCleaning] = useState(false)

  const fetchLogInfo = useCallback(async () => {
    setLoadingFiles(true)
    try {
      const [filesRes, usageRes] = await Promise.all([getLogFiles(), getLogDiskUsage()])
      setFiles((filesRes.data as any)?.files || [])
      setDiskSize((usageRes.data as any)?.size_bytes || 0)
      setLogDir((usageRes.data as any)?.dir || '')
    } catch { /* ignore */ }
    setLoadingFiles(false)
  }, [])

  useEffect(() => { fetchLogInfo() }, [fetchLogInfo])

  const fmtBytes = (b: number) => {
    if (b === 0) return '0 B'
    const u = ['B', 'KB', 'MB', 'GB']
    const i = Math.floor(Math.log(b) / Math.log(1024))
    return (b / Math.pow(1024, i)).toFixed(1) + ' ' + u[i]
  }

  const handleCleanup = async () => {
    setCleaning(true)
    try {
      const res = await cleanupLogs(config.max_age_days || 30, config.max_backups || 5)
      const removed = (res.data as any)?.removed || 0
      success(t('settings.logCleanupDone', `已清理 ${removed} 个文件`))
      await fetchLogInfo()
    } catch (e: any) {
      showError(e?.message || t('settings.logCleanupFailed', '清理失败'))
    }
    setCleaning(false)
  }

  return (
    <div className="p-6 space-y-4">
      <h3 className="text-sm font-semibold text-[var(--text-primary)]">{t('settings.logging', '日志管理')}</h3>
      <p className="text-xs text-[var(--text-muted)]">{t('settings.loggingHelp', '配置日志文件轮转和清理策略，轮转参数修改后需重启服务生效')}</p>

      <SettingsField label={t('settings.logMaxSize', '单文件最大体积')}>
        <div className="flex items-center gap-2">
          <input type="number" min={1} value={config.max_size_mb}
            onChange={e => onChange({...config, max_size_mb: parseInt(e.target.value) || 100})}
            className={inputCls} />
          <span className="text-xs text-[var(--text-muted)] shrink-0">MB</span>
        </div>
      </SettingsField>

      <SettingsField label={t('settings.logMaxBackups', '保留轮转文件数')}>
        <input type="number" min={0} value={config.max_backups}
          onChange={e => onChange({...config, max_backups: parseInt(e.target.value) || 5})}
          className={inputCls} />
      </SettingsField>

      <SettingsField label={t('settings.logMaxAge', '日志保留天数')}>
        <input type="number" min={0} value={config.max_age_days}
          onChange={e => onChange({...config, max_age_days: parseInt(e.target.value) || 30})}
          className={inputCls} />
      </SettingsField>

      <label className="flex items-center justify-between gap-4 py-2 border-b border-[var(--bg-border)]">
        <span className="text-sm text-[var(--text-secondary)]">{t('settings.logCompress', '压缩旧日志')}</span>
        <Toggle checked={config.compress} onChange={v => onChange({...config, compress: v})} />
      </label>

      <SettingsField label={t('settings.logCleanupInterval', '自动清理间隔')}>
        <div className="flex items-center gap-2">
          <input type="number" min={0} value={config.cleanup_interval}
            onChange={e => onChange({...config, cleanup_interval: parseInt(e.target.value) || 24})}
            className={inputCls} />
          <span className="text-xs text-[var(--text-muted)] shrink-0">h</span>
        </div>
      </SettingsField>

      {/* 当前磁盘用量 */}
      <div className="pt-3 border-t border-[var(--bg-border)]">
        <h4 className="text-xs font-semibold text-[var(--text-primary)] mb-2">{t('settings.logDiskUsage', '磁盘用量')}</h4>
        {logDir && <p className="text-xs text-[var(--text-muted)] mb-2">{t('settings.logDir', '目录')}: <code className="text-[var(--text-secondary)]">{logDir}</code></p>}
        <p className="text-xs text-[var(--text-muted)] mb-3">{t('settings.logTotalSize', '总大小')}: <span className="text-[var(--text-secondary)] font-semibold">{fmtBytes(diskSize)}</span></p>

        {loadingFiles ? (
          <span className="text-xs text-[var(--text-muted)]">{t('settings.loading')}</span>
        ) : files.length > 0 ? (
          <div className="space-y-1">
            {files.map(f => (
              <div key={f.name} className="flex items-center justify-between text-xs py-1">
                <span className="text-[var(--text-secondary)] font-mono">{f.name}</span>
                <span className="text-[var(--text-muted)]">{fmtBytes(f.size)}</span>
              </div>
            ))}
          </div>
        ) : (
          <p className="text-xs text-[var(--text-muted)]">{t('settings.logNoFiles', '暂无日志文件')}</p>
        )}

        <Button variant="secondary" size="sm" className="mt-3" onClick={handleCleanup} disabled={cleaning}>
          {cleaning ? t('settings.loading') : t('settings.logCleanupNow', '立即清理')}
        </Button>
      </div>
    </div>
  )
}
