import { useState, useEffect, useCallback } from 'react'
import { useTranslation } from 'react-i18next'
import { X, Save, Copy, Check, RotateCw, Settings as SettingsIcon, Monitor, FileCode, Activity } from 'lucide-react'
import { Toggle } from '../../components/ui/Toggle'
import { useToast } from '../../components/ui/Toast'
import { useUIConfig } from '../../contexts/UIConfigContext'
import {
  getGeneralSettings, updateGeneralSettings,
  getNetbootSettings, updateNetbootSettings,
  getInterfaces, getServices, updateAutoStart,
  type GeneralSettings, type NetbootSettingsData, type InterfaceInfo, type ServiceInfo,
} from '../../api/client'

interface Props {
  open: boolean
  onClose: () => void
}

type Section = 'general' | 'boot' | 'netboot' | 'services'

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
  const [originalDataDir, setOriginalDataDir] = useState('')

  useEffect(() => {
    if (!open) return
    setLoading(true)
    setTokenCopied(false)
    Promise.all([getGeneralSettings(), getNetbootSettings(), getInterfaces(), getServices()])
      .then(([g, nb, ifaces, svc]) => {
        const gen = g.data as unknown as GeneralSettings
        setGeneral(gen)
        setOriginalDataDir(gen.data_dir || '')
        setNetboot(nb.data as unknown as NetbootSettingsData)
        setAvailableIfaces(ifaces.data as unknown as InterfaceInfo[])
        setServices(svc.data as unknown as ServiceInfo[])
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
      if (general) promises.push(updateGeneralSettings(general))
      if (netboot) promises.push(updateNetbootSettings(netboot))
      await Promise.all(promises)
      if (general?.page_size) setPageSize(general.page_size)
      success('设置已保存')
    } catch (e: any) {
      showError(e?.message || '保存失败')
    } finally {
      setSaving(false)
    }
  }

  if (!open) return null

  const navItems: NavItem[] = [
    { key: 'general', label: t('nav.settings.general'), icon: SettingsIcon },
    { key: 'boot', label: t('nav.settings.bootMenu'), icon: FileCode },
    { key: 'netboot', label: t('nav.settings.netboot'), icon: Monitor },
    { key: 'services', label: '服务自动启动', icon: Activity },
  ]

  return (
    <div className="fixed inset-0 z-[100] flex items-center justify-center bg-black/60" onClick={onClose}>
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
              <button onClick={handleSave} disabled={saving}
                className="inline-flex items-center gap-1.5 px-4 py-1.5 rounded-lg text-xs font-medium bg-blue-500 text-white hover:bg-blue-600 transition-colors disabled:opacity-50">
                <Save size={14} />
                {saving ? t('common.loading') : t('settings.save')}
              </button>
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
                <span className="ml-3 text-sm text-[var(--text-muted)]">加载中...</span>
              </div>
            ) : (
              <>
                {section === 'general' && general && <GeneralForm config={general} onChange={setGeneral} tokenCopied={tokenCopied} onCopy={copyToken} onRegenerate={regenerateToken} ifaces={availableIfaces} originalDataDir={originalDataDir} />}
                {section === 'boot' && general && <BootForm config={general} onChange={setGeneral} />}
                {section === 'netboot' && netboot && <NetbootForm data={netboot} onChange={setNetboot} />}
                {section === 'services' && services.length > 0 && <ServicesForm services={services} onReload={async () => { try { const res = await getServices(); setServices(res.data as unknown as ServiceInfo[]) } catch {} }} />}
              </>
            )}
          </div>
        </div>
      </div>
    </div>
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

  return (
    <div className="p-6 space-y-4">
      <h3 className="text-sm font-semibold text-[var(--text-primary)]">基本设置</h3>
      <SettingsField label={t('common.serverName', '服务器名称')}>
        <input className={inputCls} value={config.server_name} onChange={e => onChange({...config, server_name: e.target.value})} />
      </SettingsField>
      <SettingsField label={t('common.logLevel', '日志级别')}>
        <select className={selectCls} value={config.log_level} onChange={e => onChange({...config, log_level: e.target.value})}>
          <option>info</option><option>debug</option><option>warn</option><option>error</option>
        </select>
      </SettingsField>
      <SettingsField label="每页条数">
        <input type="number" min={5} max={500} value={config.page_size}
          onChange={e => onChange({...config, page_size: parseInt(e.target.value) || 50})}
          className="w-full bg-[var(--bg-card)] border border-[var(--bg-border)] rounded-lg px-3.5 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500" />
        <p className="text-xs text-[var(--text-muted)] mt-1">列表页和表格每页显示的数据条数，默认 50。</p>
      </SettingsField>
      <SettingsField label={t('settings.dataDir')}>
        <div className="flex gap-2">
          <input className={`${inputCls} flex-1`} value={config.data_dir} onChange={e => onChange({...config, data_dir: e.target.value, migrate_boot: undefined})} />
        </div>
        {originalDataDir && config.data_dir !== originalDataDir && (
          <label className="flex items-center gap-2 mt-2 cursor-pointer" onClick={e => e.stopPropagation()}>
            <input type="checkbox" checked={!!config.migrate_boot} onChange={e => onChange({...config, migrate_boot: e.target.checked})}
              className="rounded border-[var(--bg-border)] bg-[var(--bg-input)]" />
            <span className="text-xs text-[var(--text-muted)]">同时将旧目录中的启动文件迁移到新目录</span>
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
            <option value="">选择网卡</option>
            <option value="*">所有接口 (0.0.0.0:{config.listen_addr.split(':')[1] || '8080'})</option>
            {ifaces.filter(ai => ai.up && ai.ipv4?.length > 0).map(ai => (
              <option key={ai.name} value={ai.name}>{ai.name} ({ai.ipv4[0]})</option>
            ))}
          </select>
        </div>
        <p className="text-xs text-[var(--text-muted)] mt-1">修改后需要重启 HTTP 服务才能生效。默认 127.0.0.1:8080（仅本机访问）；设为 0.0.0.0:8080 允许远程访问（需要登录认证）。</p>
      </SettingsField>

      <SettingsField label="API 认证令牌">
        <div className="flex items-center gap-2">
          <input type="text" readOnly
            value={config.token.length === 32 && /^[0-9a-f]+$/i.test(config.token)
              ? config.token.slice(0, 4) + '...' + config.token.slice(-4)
              : config.token || '未设置'}
            className="flex-1 bg-[var(--bg-card)] border border-[var(--bg-border)] rounded-lg px-3.5 py-2 text-sm text-[var(--text-primary)] font-mono outline-none select-all" />
          <button onClick={onCopy} className="p-2 rounded-lg border border-[var(--bg-border)] hover:bg-[var(--bg-card)] transition-colors text-[var(--text-secondary)]" title="复制">
            {tokenCopied ? <Check size={16} className="text-green-500" /> : <Copy size={16} />}
          </button>
          <button onClick={onRegenerate} className="p-2 rounded-lg border border-[var(--bg-border)] hover:bg-[var(--bg-card)] transition-colors text-[var(--text-secondary)]" title="重新生成">
            <RotateCw size={16} />
          </button>
        </div>
        <p className="text-xs text-[var(--text-muted)] mt-1">用于 API 请求的身份验证。将令牌输入登录页即可获取会话令牌。</p>

        {config.token && !config.token.includes('...') && (
          <div className="mt-2 px-3 py-2 bg-amber-500/10 border border-amber-500/30 rounded-lg text-xs text-amber-400">
            ⚠ 新令牌已生成！请立即复制并保存。保存配置后令牌将仅显示掩码。
          </div>
        )}
      </SettingsField>

      <div className="pt-2">
        <label className="flex items-center justify-between gap-4 py-2 border-b border-[var(--bg-border)]">
          <span className="text-sm text-[var(--text-secondary)]">{t('settings.autoOpen', '启动时自动打开浏览器（app 模式）')}</span>
          <Toggle checked={config.app_mode} onChange={v => onChange({...config, app_mode: v})} />
        </label>
        <label className="flex items-center justify-between gap-4 py-2 border-b border-[var(--bg-border)]">
          <span className="text-sm text-[var(--text-secondary)]">启用全局白名单</span>
          <Toggle checked={config.whitelist_enabled} onChange={v => onChange({...config, whitelist_enabled: v})} />
        </label>
      </div>
    </div>
  )
}

// ── Boot Customization ──

function BootForm({ config, onChange }: { config: GeneralSettings; onChange: (c: GeneralSettings) => void }) {
  const inputCls = 'w-full bg-[var(--bg-card)] border border-[var(--bg-border)] rounded-lg px-3.5 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500'
  return (
    <div className="p-6 space-y-4">
      <h3 className="text-sm font-semibold text-[var(--text-primary)]">引导定制</h3>

      <SettingsField label="自定义 iPXE 脚本">
        <p className="text-xs text-[var(--text-muted)] mb-2">仅当客户端使用 <strong>iPXE</strong> 引导时生效（pxelinux/GRUB2 不受此影响）。留空则使用下方可视化配置的引导逻辑。</p>
        <textarea value={config.script_template} onChange={e => onChange({...config, script_template: e.target.value})}
          rows={6} spellCheck={false}
          className="w-full bg-[var(--bg-card)] border border-[var(--bg-border)] rounded-lg px-4 py-3 text-sm font-mono text-[var(--text-primary)] outline-none focus:border-blue-500"
          placeholder={'#!ipxe\nmenu PxeGo Boot Menu\nitem shell iPXE Shell\nitem local Boot Local Disk\n\nchoose selected || shell\ngoto ${selected}'} />
      </SettingsField>

      <div className="pt-4 border-t border-[var(--bg-border)]">
        <h4 className="text-xs font-semibold text-[var(--text-primary)] mb-2">默认引导菜单</h4>
        <p className="text-xs text-[var(--text-muted)] mb-3">引导项在「<a href="/profiles" className="text-blue-400 hover:text-blue-300">引导配置</a>」中管理。</p>
        <SettingsField label="超时时间（秒）">
          <input className={inputCls} type="number" value={config.default_menu.timeout}
            onChange={e => onChange({...config, default_menu: {...config.default_menu, timeout: parseInt(e.target.value) || 0}})} />
        </SettingsField>
        <label className="flex items-center justify-between gap-4 py-2 border-b border-[var(--bg-border)]">
          <span className="text-sm text-[var(--text-secondary)]">列出所有引导配置作为菜单项</span>
          <Toggle checked={config.default_menu.list_all_profiles}
            onChange={v => onChange({...config, default_menu: {...config.default_menu, list_all_profiles: v}})} />
        </label>
        <p className="text-xs text-[var(--text-muted)] mt-1">关闭时只显示默认引导配置的项，开启时列出所有引导配置，默认配置排第一。</p>
      </div>
    </div>
  )
}

// ── Netboot ──

function NetbootForm({ data, onChange }: { data: NetbootSettingsData; onChange: (d: NetbootSettingsData) => void }) {
  const inputCls = 'w-full bg-[var(--bg-card)] border border-[var(--bg-border)] rounded-lg px-3.5 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500'
  return (
    <div className="p-6 space-y-4">
      <h3 className="text-sm font-semibold">Netboot 设置</h3>
      <SettingsField label="启用 Netboot 引导菜单">
        <Toggle checked={data.enabled} onChange={v => onChange({...data, enabled: v})} />
      </SettingsField>
      <p className="text-xs text-[var(--text-muted)] -mt-2">启用后，PXE 引导菜单将显示「[Netboot] 网络安装操作系统目录」选项。</p>
      <SettingsField label="HTTPS 代理">
        <Toggle checked={data.proxy_https} onChange={v => onChange({...data, proxy_https: v})} />
        <p className="text-xs text-[var(--text-muted)] mt-1">开启后自动将 HTTPS 引导 URL 通过本地 HTTP 代理拉取，适用于不支持 HTTPS 的 iPXE 固件。</p>
      </SettingsField>
      <SettingsField label="菜单标题">
        <input className={inputCls} value={data.catalog_display.title}
          onChange={e => onChange({...data, catalog_display: {...data.catalog_display, title: e.target.value}})} />
      </SettingsField>

      <div className="pt-4 border-t border-[var(--bg-border)]">
        <h4 className="text-xs font-semibold text-[var(--text-primary)] mb-2">引导菜单跳转</h4>
        <p className="text-xs text-[var(--text-muted)] mb-3">无匹配的 Profile 时自动跳转到 Netboot 引导菜单。</p>
        <SettingsField label="启用跳转">
          <Toggle checked={data.catalog_redirect?.enabled ?? false}
            onChange={v => onChange({...data, catalog_redirect: {...data.catalog_redirect, enabled: v}})} />
        </SettingsField>
        <SettingsField label="目标 URL">
          <div className="flex flex-col gap-2">
            <input className={`${inputCls} font-mono`} value={data.catalog_redirect.target_url}
              onChange={e => onChange({...data, catalog_redirect: {...data.catalog_redirect, target_url: e.target.value}})} />
            <div className="flex gap-2">
              {[
                { label: '本地 Netboot', url: 'http://{{.URL}}/netboot/menu.ipxe?arch=${arch}&platform=${platform}' },
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
            <p className="text-[11px] text-[var(--text-muted)]">支持 <code className="text-[10px] bg-[var(--bg-card)] px-1 py-0.5 rounded font-mono">{`{{.URL}}`}</code> 变量替换。iPXE 不支持 HTTPS。</p>
          </div>
        </SettingsField>
        <SettingsField label="前置脚本">
          <textarea rows={3} spellCheck={false}
            className="w-full bg-[var(--bg-card)] border border-[var(--bg-border)] rounded-lg px-3.5 py-2 text-sm font-mono text-[var(--text-primary)] outline-none focus:border-blue-500"
            value={data.catalog_redirect.preamble}
            onChange={e => onChange({...data, catalog_redirect: {...data.catalog_redirect, preamble: e.target.value}})}
            placeholder="# 可选：在跳转前执行 dhcp、设置变量等" />
        </SettingsField>
      </div>
    </div>
  )
}

// ── Service Auto-Start ──

function ServicesForm({ services, onReload }: { services: ServiceInfo[]; onReload: () => Promise<void> }) {
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
      <h3 className="text-sm font-semibold text-[var(--text-primary)]">服务自动启动</h3>
      <p className="text-xs text-[var(--text-muted)]">开启后，对应服务将在 PxeGo 下次启动时自动运行（不影响当前运行状态）。</p>

      {/* 全局服务 */}
      {['http','tftp','dns'].filter(k => services.some(s => s.name === k)).map(k => {
        const svc = services.find(s => s.name === k)!
        return <ServiceAutoStartRow key={svc.name} svc={svc} operating={operating} onToggle={toggleAutoStart} />
      })}

      {/* 接口级服务 */}
      {['dhcp/','proxy/'].filter(prefix => services.some(s => s.name.startsWith(prefix))).length > 0 && (
        <>
          <div className="pt-2 pb-1">
            <span className="text-[11px] font-semibold uppercase tracking-wider text-[var(--text-muted)]">接口级服务（每个网卡独立配置）</span>
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
          {svc.auto_start ? '已开启' : '已关闭'}
        </span>
        <button
          onClick={() => onToggle(svc.name, !svc.auto_start)}
          disabled={isOperating || svc.protected}
          className={`relative w-10 h-5 rounded-full transition-colors ${isOperating ? 'opacity-50' : ''} ${svc.protected ? 'cursor-not-allowed opacity-60' : ''} ${svc.auto_start ? 'bg-blue-500' : 'bg-gray-500/30'}`}
        >
          <span className={`absolute top-0.5 left-0.5 w-4 h-4 rounded-full bg-white transition-transform ${svc.auto_start ? 'translate-x-5' : 'translate-x-0'}`} />
        </button>
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
