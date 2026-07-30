import { type FC, type ReactNode, useState, useEffect, useCallback, useMemo, useRef } from 'react'
import { useNavigate, useLocation } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { useTheme } from '../../hooks/useTheme'
import { usePalette } from '../../hooks/usePalette'
import { useRadius } from '../../hooks/useRadius'
import { ThemeSwitcher } from '../ThemeSwitcher'
import { LangSwitch } from '../LangSwitch'
import { StatusDot } from '../ui/StatusDot'
import { useToast } from '../ui/Toast'
import { getServices, startService, stopService, restartService, batchService, getVersionInfo, downloadUpdate, type ServiceInfo, type VersionInfo } from '../../api/client'
import SettingsModal from './SettingsModal'
import { CommandPalette } from '../CommandPalette'
import { NotificationCenter } from '../NotificationCenter'
import {
  LayoutDashboard, Server, FileCode, Activity, Settings,
  ShieldCheck, Network, Menu, ChevronRight, ChevronLeft,
  HardDrive, Cpu, Wifi, Disc, Bell, ScrollText, Search, Package,
} from 'lucide-react'

interface NavItem {
  path?: string
  label: string
  icon?: FC<{ size?: number; className?: string }>
  badge?: string
  children?: NavItem[]
}

const navSections = [
  {
    label: 'nav.section.overview',
    items: [
      { path: '/', label: 'nav.dashboard', icon: LayoutDashboard },
    ] as NavItem[],
  },
  {
    label: 'nav.section.basic',
    items: [
      {
        label: 'nav.serviceConfig',
        icon: Settings,
        children: [
          { path: '/services/dhcp', label: 'nav.settings.dhcp' },
          { path: '/services/dns', label: 'nav.settings.dns' },
          { path: '/services/nfs', label: 'nav.settings.nfs' },
          { path: '/services/tftp', label: 'nav.settings.tftp' },
          { path: '/boot-settings', label: 'nav.bootSettings' },
          { path: '/netboot-catalog', label: 'nav.netboot' },
        ] as NavItem[],
      },
      { path: '/files', label: 'nav.files', icon: HardDrive },
      { path: '/profiles', label: 'nav.profiles', icon: FileCode },
      { path: '/answer-templates', label: 'nav.answerTemplates', icon: FileCode },
      { path: '/os-images', label: 'nav.osImages', icon: Disc },
    ] as NavItem[],
  },
  {
    label: 'nav.section.manage',
    items: [
      { path: '/hosts', label: 'nav.hosts', icon: Server },
      { path: '/access-control', label: 'nav.accessControl', icon: ShieldCheck },
      { path: '/baselines', label: 'nav.baselines', icon: FileCode },
      { path: '/scripts', label: 'nav.scripts', icon: FileCode },
      { path: '/install-tasks', label: 'nav.installTasks', icon: HardDrive },
      { path: '/bmc', label: 'nav.bmc', icon: Cpu },
      { path: '/wol', label: 'nav.wol', icon: Wifi },
      { path: '/network', label: 'nav.network', icon: Network },
      { path: '/store', label: 'nav.store', icon: Package },
    ] as NavItem[],
  },
  {
    label: 'nav.section.monitor',
    items: [
      { path: '/events', label: 'nav.events', icon: Bell },
      { path: '/audit-logs', label: 'nav.auditLogs', icon: ScrollText },
      { path: '/logs', label: 'nav.logs', icon: Activity },
    ] as NavItem[],
  },
]

const RECENT_NAV_KEY = 'pxelab-recent-nav'
const RECENT_NAV_LIMIT = 5

interface RecentNavEntry {
  path: string
  labelKey: string
}

function loadRecentNav(): RecentNavEntry[] {
  try {
    const raw = JSON.parse(localStorage.getItem(RECENT_NAV_KEY) || '[]')
    return Array.isArray(raw) ? raw.filter(r => r && typeof r.path === 'string' && typeof r.labelKey === 'string') : []
  } catch {
    return []
  }
}

interface Props {
  children: ReactNode
}

// start/stop/restart 操作按钮的配色，批量操作区和单项操作区共用
const opButtonColors = {
  green: 'bg-accent-green/15 text-accent-green hover:bg-accent-green/25',
  red: 'bg-accent-red/15 text-accent-red hover:bg-accent-red/25',
  orange: 'bg-orange-500/15 text-orange-400 hover:bg-orange-500/25',
} as const

function OpButton({ color, size = 'md', disabled, busy, label, onClick }: {
  color: keyof typeof opButtonColors
  size?: 'md' | 'sm'
  disabled: boolean
  busy: boolean
  label: string
  onClick: () => void
}) {
  return (
    <button onClick={onClick} disabled={disabled}
      className={`${size === 'sm' ? 'px-2 py-0.5' : 'px-2.5 py-1'} text-[10px] font-medium rounded ${opButtonColors[color]} disabled:opacity-40 transition-colors`}>
      {busy ? '...' : label}
    </button>
  )
}

function ServiceDropdown() {
  const { t } = useTranslation()
  const toast = useToast()
  const [services, setServices] = useState<ServiceInfo[]>([])
  const [open, setOpen] = useState(false)
  const [operating, setOperating] = useState<Set<string>>(new Set())
  const hoverTimer = useRef<ReturnType<typeof setTimeout>>(undefined)

  const hoverOpen = () => {
    clearTimeout(hoverTimer.current)
    setOpen(true)
  }
  const hoverClose = () => {
    hoverTimer.current = setTimeout(() => setOpen(false), 200)
  }
  // 触屏无 hover，点击切换展开/收起；与 hover 不冲突：hover 打开后点击即收起，
  // 收起期间鼠标仍在按钮上不会再次触发 mouseEnter
  const clickToggle = () => {
    clearTimeout(hoverTimer.current)
    setOpen(v => !v)
  }

  const load = useCallback(async () => {
    try {
      const res = await getServices()
      setServices(res.data)
    } catch (err) {
      // 轮询失败可忽略：下一次轮询会重试，不打断用户；仅记录日志便于排查
      console.warn('service status poll failed:', err)
    }
  }, [])

  useEffect(() => { load(); const iv = setInterval(load, 5000); return () => clearInterval(iv) }, [load])

  const running = services.filter(s => s.status === 'running').length
  const stopped = services.filter(s => s.status === 'stopped').length
  const errors = services.filter(s => s.status === 'error').length

  const runOp = async (name: string, op: 'start' | 'stop' | 'restart') => {
    setOperating(prev => new Set(prev).add(name + op))
    const opLabel = t(`common.${op}`)
    try {
      const p = op === 'start' ? startService(name) : op === 'stop' ? stopService(name) : restartService(name)
      await toast.promise(p, {
        loading: t('common.serviceOpLoading', { op: opLabel, name }),
        success: t('common.serviceOpSuccess', { name, op: opLabel.toLowerCase() }),
      })
      await load()
    } catch {
      // toast.promise 已把失败原因原地展示在通知上
    }
    setOperating(prev => { const next = new Set(prev); next.delete(name + op); return next })
  }

  const batchAll = async (op: 'start' | 'stop' | 'restart') => {
    const targets = services.filter(s => !s.protected && (op === 'start' ? s.status !== 'running' : true))
    if (targets.length === 0) return
    const names = targets.map(s => s.name)
    names.forEach(n => setOperating(prev => new Set(prev).add(n + op)))
    const opLabel = t(`common.${op}`)
    const name = t('common.serviceCount', { count: targets.length })
    try {
      await toast.promise(batchService(op, names), {
        loading: t('common.serviceOpLoading', { op: opLabel, name }),
        success: t('common.serviceOpSuccess', { name, op: opLabel.toLowerCase() }),
      })
      await load()
    } catch {
      // toast.promise 已把失败原因原地展示在通知上
    }
    names.forEach(n => setOperating(prev => { const next = new Set(prev); next.delete(n + op); return next }))
  }

  const anyOperating = (op: string) => services.some(s => operating.has(s.name + op))

  return (
    <div className="relative" onMouseEnter={hoverOpen} onMouseLeave={hoverClose}>
      <button
        onClick={clickToggle}
        className="hidden sm:inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full text-xs font-mono font-semibold bg-[var(--bg-card)] border border-[var(--bg-border)] text-[var(--text-secondary)] hover:bg-[var(--bg-hover)] hover:shadow-sm transition-all duration-200"
      >
        <div className="flex items-center gap-2.5 py-0.5">
          <span className="flex items-center gap-1"><StatusDot color="green" /><span className="text-accent-green font-bold">{running}</span><span className="text-[10px] text-[var(--text-muted)]">{t('common.running')}</span></span>
          <span className="flex items-center gap-1"><StatusDot color="yellow" /><span className="text-accent-yellow font-bold">{stopped}</span><span className="text-[10px] text-[var(--text-muted)]">{t('common.stopped')}</span></span>
          <span className="flex items-center gap-1"><StatusDot color="red" /><span className="text-accent-red font-bold">{errors}</span><span className="text-[10px] text-[var(--text-muted)]">{t('common.error')}</span></span>
        </div>
      </button>
      {open && (
        <div onMouseEnter={hoverOpen} onMouseLeave={hoverClose}>
          <div className="absolute right-0 top-full mt-2 w-[520px] z-50 rounded-xl bg-[var(--bg-elevated)] border border-[var(--bg-border)] shadow-xl overflow-hidden animate-fade-in">
            {/* 一键操作 */}
            <div className="px-4 py-3 border-b border-[var(--bg-border)] bg-[var(--bg-base)]/50">
              <div className="flex items-center gap-1.5">
                <OpButton color="green" onClick={() => batchAll('start')} disabled={anyOperating('start')}
                  busy={anyOperating('start')} label={t('common.startAll')} />
                <OpButton color="red" onClick={() => batchAll('stop')} disabled={anyOperating('stop')}
                  busy={anyOperating('stop')} label={t('common.stopAll')} />
                <OpButton color="orange" onClick={() => batchAll('restart')} disabled={anyOperating('restart')}
                  busy={anyOperating('restart')} label={t('common.restartAll')} />
              </div>
            </div>
            {/* 服务列表 */}
            <div className="max-h-[320px] overflow-y-auto">
              {services.map(svc => {
                const color = svc.status === 'running' ? 'green' : svc.status === 'error' ? 'red' : 'yellow'
                return (
                  <div key={svc.name} className="flex items-center justify-between px-4 py-2.5 hover:bg-[var(--bg-hover)]/30 border-b border-[var(--bg-border)] last:border-0">
                    <div className="flex items-center gap-2.5 min-w-0 flex-1">
                      <StatusDot color={color as any} />
                      <div className="min-w-0">
                        <span className="text-xs font-medium text-[var(--text-primary)]">{svc.display}</span>
                        <span className="text-[10px] text-[var(--text-muted)] ml-2 font-mono">{svc.port}/{svc.protocol}</span>
                      </div>
                    </div>
                    <div className="flex items-center gap-1 shrink-0">
                      {svc.protected ? (
                        <span className="text-[10px] text-blue-400 font-medium">Core</span>
                      ) : (
                        <>
                          {svc.status !== 'running' && (
                            <OpButton color="green" size="sm" onClick={() => runOp(svc.name, 'start')}
                              disabled={operating.has(svc.name + 'start')} busy={operating.has(svc.name + 'start')} label={t('common.start')} />
                          )}
                          {svc.status === 'running' && (
                            <OpButton color="red" size="sm" onClick={() => runOp(svc.name, 'stop')}
                              disabled={operating.has(svc.name + 'stop')} busy={operating.has(svc.name + 'stop')} label={t('common.stop')} />
                          )}
                          <OpButton color="orange" size="sm" onClick={() => runOp(svc.name, 'restart')}
                            disabled={operating.has(svc.name + 'restart')} busy={operating.has(svc.name + 'restart')} label={t('common.restart')} />
                        </>
                      )}
                    </div>
                  </div>
                )
              })}
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

export const AppShell: FC<Props> = ({ children }) => {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const location = useLocation()
  const { theme, setTheme } = useTheme()
  const { palette, setPalette } = usePalette()
  const { radius, setRadius } = useRadius()
  const [macosCards, setMacosCards] = useState(() => localStorage.getItem('PxeLab-macos-cards') === 'true')
  const [sidebarOpen, setSidebarOpen] = useState(false)
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false)
  const [settingsModalOpen, setSettingsModalOpen] = useState(false)
  const [paletteOpen, setPaletteOpen] = useState(false)
  const [recentNav, setRecentNav] = useState<RecentNavEntry[]>(loadRecentNav)
  useEffect(() => {
    document.documentElement.classList.toggle('theme-macos', macosCards)
    localStorage.setItem('PxeLab-macos-cards', String(macosCards))
  }, [macosCards])

  // ── 版本检查 ──
  const toast = useToast()
  const [versionInfo, setVersionInfo] = useState<VersionInfo | null>(null)
  const [downloading, setDownloading] = useState(false)

  useEffect(() => {
    getVersionInfo().then(res => {
      setVersionInfo(res.data)
    }).catch(() => {
      // 静默失败，不影响主界面
    })
  }, [])

  const handleDownload = useCallback(async () => {
    setDownloading(true)
    try {
      const res = await downloadUpdate()
      toast.success(t('version.downloadReady', { path: res.data.file_name }))
    } catch (e: any) {
      toast.error(e?.message || t('version.downloadFailed'))
    } finally {
      setDownloading(false)
    }
  }, [t, toast])

  // 全局 ⌘K / Ctrl+K 切换命令面板
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key.toLowerCase() === 'k' && (e.metaKey || e.ctrlKey)) {
        e.preventDefault()
        setPaletteOpen(v => !v)
      }
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [])

  // 记录路由访问到「最近访问」（只记录精确匹配的导航路径）
  useEffect(() => {
    const path = location.pathname
    let labelKey: string | undefined
    for (const section of navSections) {
      for (const item of section.items) {
        if (item.path === path) labelKey = item.label
        for (const child of item.children ?? []) {
          if (child.path === path) labelKey = child.label
        }
      }
    }
    if (!labelKey) return
    setRecentNav(prev => {
      const next = [{ path, labelKey }, ...prev.filter(r => r.path !== path)].slice(0, RECENT_NAV_LIMIT)
      localStorage.setItem(RECENT_NAV_KEY, JSON.stringify(next))
      return next
    })
  }, [location.pathname])

  // 命令面板的扁平化导航数据（含 children 二级项）
  const paletteNavItems = useMemo(() =>
    navSections.flatMap(section =>
      section.items.flatMap(item => [
        ...(item.path ? [{ path: item.path, label: t(item.label), group: t(section.label) }] : []),
        ...(item.children ?? [])
          .filter(child => child.path)
          .map(child => ({ path: child.path!, label: t(child.label), group: t(item.label) })),
      ])
    ), [t])

  const paletteRecentItems = useMemo(() =>
    recentNav.map(r => ({ path: r.path, label: t(r.labelKey) })),
  [recentNav, t])

  const isActive = (path: string) => {
    if (path === '/') return location.pathname === '/'
    if (path === '/services') return location.pathname === '/services'
    return location.pathname.startsWith(path)
  }

  const activeLevel3Parent = (() => {
    for (const section of navSections) {
      for (const item of section.items) {
        if (item.children?.some(child => child.path && isActive(child.path))) {
          return item
        }
      }
    }
    return undefined
  })()

  const pageTitle = () => {
    for (const section of navSections) {
      for (const item of section.items) {
        if (item.path && isActive(item.path)) return t(item.label)
        if (item.children) {
          for (const child of item.children) {
            if (child.path && isActive(child.path)) return t(child.label)
          }
        }
      }
    }
    return t('nav.dashboard')
  }

  return (
    <div className={`flex min-h-screen text-[var(--text-primary)] ${macosCards ? '' : 'bg-[var(--bg-base)]'}`}>
      {/* Sidebar */}
      <aside
        className={`${
          sidebarOpen ? 'translate-x-0' : '-translate-x-full'
        } lg:translate-x-0 fixed top-0 left-0 bottom-0 z-50 flex flex-col overflow-hidden transition-all duration-300 ${
          macosCards ? 'bg-[var(--bg-elevated)]/70 backdrop-blur-[40px] saturate-[1.5] border-r border-[var(--bg-border)]' : 'bg-[var(--bg-elevated)]/80 backdrop-blur-2xl border-r border-[var(--bg-border)]'
        } ${
          sidebarCollapsed ? 'w-[64px]' : 'w-[200px]'
        }`}
      >
        {/* Logo */}
        <div className={`flex flex-col border-b border-[var(--bg-border)]/50 ${sidebarCollapsed ? 'items-center py-3 gap-2' : 'px-4 h-16 flex-row items-center gap-3'}`}>
          <div className="w-9 h-9 rounded-xl bg-gradient-to-br from-blue-500 to-indigo-600 flex items-center justify-center font-extrabold text-sm text-white shadow-lg shadow-blue-500/30 shrink-0 transition-transform hover:scale-105">
            PX
          </div>
          {!sidebarCollapsed && (
            <>
              <span className="text-lg font-bold tracking-tight transition-all duration-300">
                Pxe<span className="text-blue-500">Go</span>
              </span>
              <button
                onClick={() => setSidebarCollapsed(true)}
                className="ml-auto p-1.5 rounded-md text-[var(--text-muted)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-secondary)] transition-colors"
                title={t('common.collapseSidebar')}
              >
                <ChevronLeft size={14} />
              </button>
            </>
          )}
        </div>

        {/* Nav */}
        <nav className="flex-1 overflow-y-auto min-h-0 px-2 py-3 flex flex-col gap-1">
          {navSections.map((section, sIdx) => (
            <div key={section.label}>
              {sIdx > 0 && !sidebarCollapsed && <div className="mx-2 my-2 border-t border-[var(--bg-border)]/50" />}
              <div className={`px-3 py-1.5 text-[10px] uppercase tracking-wider text-[var(--text-muted)] font-semibold transition-opacity duration-200 ${sidebarCollapsed ? 'opacity-0 h-0 overflow-hidden py-0' : ''}`}>
                {t(section.label)}
              </div>
              {section.items.map((item) => {
                if (item.children) {
                  const hasActiveChild = item.children.some(child => child.path && isActive(child.path))
                  const Icon = item.icon
                  const firstChildPath = item.children.find(c => c.path)?.path
                  return (
                    <button
                      key={item.label}
                      onClick={() => { firstChildPath && navigate(firstChildPath); setSidebarOpen(false) }}
                      className={`w-full flex items-center py-2 rounded-lg text-sm font-medium transition-all duration-200 text-left ${
                        sidebarCollapsed ? 'justify-center gap-0 px-3'
                        : 'gap-2.5 pl-5 pr-3 ' + (hasActiveChild
                            ? 'bg-blue-500/10 text-blue-400 shadow-sm'
                            : 'text-[var(--text-secondary)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)] hover:shadow-sm')
                      }`}
                      title={sidebarCollapsed ? t(item.label) : undefined}
                    >
                      {Icon && <Icon size={16} className="shrink-0 opacity-70" />}
                      <span className={`${sidebarCollapsed ? 'hidden' : ''}`}>{t(item.label)}</span>
                    </button>
                  )
                }

                const Icon = item.icon!
                const active = isActive(item.path!)
                return (
                  <button
                    key={item.path}
                    onClick={() => { item.path && navigate(item.path); setSidebarOpen(false) }}
                    aria-current={active ? 'page' : undefined}
                    className={`w-full flex items-center py-2 rounded-lg text-sm font-medium transition-all duration-200 text-left relative ${
                      sidebarCollapsed ? 'justify-center gap-0 px-3'
                    : 'gap-2.5 pl-5 pr-3 ' + (active
                        ? 'bg-blue-500/10 text-blue-400 shadow-sm'
                        : 'text-[var(--text-secondary)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)] hover:shadow-sm')
                    }`}
                    title={sidebarCollapsed ? t(item.label) : undefined}
                  >
                    {active && !sidebarCollapsed && (
                      <span className="absolute left-0 top-1/2 -translate-y-1/2 w-[3px] h-5 bg-blue-500 rounded-r-full shadow-lg shadow-blue-500/50" />
                    )}
                    <Icon size={16} className="shrink-0 opacity-70" />
                    <span className={`${sidebarCollapsed ? 'hidden' : ''}`}>{t(item.label)}</span>
                    {item.badge && (
                      <span className={`text-[11px] font-mono px-2 py-0.5 rounded-full ${
                        sidebarCollapsed ? 'hidden'
                      : active ? 'bg-blue-500/15 text-blue-400' : 'bg-[var(--bg-card)] text-[var(--text-muted)]'
                      }`}>
                        {item.badge}
                      </span>
                    )}
                  </button>
                )
              })}
            </div>
          ))}
        </nav>

        {/* Settings - always at bottom */}
        <div className="border-t border-[var(--bg-border)]/50 px-2 py-2.5">
          {sidebarCollapsed ? (
            <button
              onClick={() => setSidebarCollapsed(false)}
              className="w-full flex items-center justify-center px-3 py-2 rounded-lg text-sm text-[var(--text-muted)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-secondary)] transition-colors"
              title={t('common.expandSidebar')}
            >
              <ChevronRight size={16} />
            </button>
          ) : (
            <button
              onClick={() => setSettingsModalOpen(true)}
              className="w-full flex items-center justify-center gap-2.5 px-3 py-2 rounded-lg text-sm text-[var(--text-secondary)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)] transition-colors"
            >
              <Settings size={16} className="opacity-70 shrink-0" />
              <span>{t('settings.title')}</span>
            </button>
          )}
        </div>
      </aside>

      <SettingsModal open={settingsModalOpen} onClose={() => setSettingsModalOpen(false)} />

      <CommandPalette
        open={paletteOpen}
        onClose={() => setPaletteOpen(false)}
        navItems={paletteNavItems}
        recent={paletteRecentItems}
      />

      {/* Overlay for mobile */}
      {sidebarOpen && (
        <div className="fixed inset-0 bg-black/50 z-40 lg:hidden" onClick={() => setSidebarOpen(false)} />
      )}

      {/* Main */}
      <main className={`flex flex-col flex-1 min-h-screen min-w-0 transition-all duration-300 ${sidebarCollapsed ? 'lg:ml-[64px]' : 'lg:ml-[200px]'}`}>
        {/* Top Bar */}
        <header className="h-16 shrink-0 border-b border-[var(--bg-border)] flex items-center justify-between px-4 lg:px-8 bg-[var(--bg-elevated)]/80 backdrop-blur-xl sticky top-0 z-30">
          <div className="absolute inset-x-0 bottom-0 h-[1px] bg-gradient-to-r from-transparent via-blue-500/30 to-transparent" />
          <div className="flex items-center gap-4">
            <button
              onClick={() => setSidebarOpen(true)}
              className="lg:hidden p-2 text-[var(--text-secondary)] hover:text-[var(--text-primary)]"
            >
              <Menu size={20} />
            </button>
            <h1 className="lg:hidden text-lg font-bold tracking-tight text-[var(--text-primary)]">{pageTitle()}</h1>
          </div>
          <div className="flex items-center gap-3">
            <button
              onClick={() => setPaletteOpen(true)}
              className="flex items-center gap-2 px-3 py-1.5 rounded-lg bg-[var(--bg-card)] border border-[var(--bg-border)] text-[var(--text-muted)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-secondary)] transition-colors"
            >
              <Search size={14} />
              <span className="hidden md:inline text-xs">{t('commandPalette.placeholder')}</span>
              <kbd className="hidden md:inline-flex items-center px-1.5 py-0.5 rounded border border-[var(--bg-border)] bg-[var(--bg-elevated)] text-[10px] font-mono text-[var(--text-muted)]">⌘K</kbd>
            </button>
            <NotificationCenter />
            <ServiceDropdown />
            <LangSwitch />
            <ThemeSwitcher theme={theme} palette={palette} onSetTheme={setTheme} onChangePalette={setPalette} radius={radius} onChangeRadius={setRadius} macosCards={macosCards} onToggleMacOS={() => setMacosCards(v => !v)} />
          </div>
        </header>

        {/* Update banner */}
        {versionInfo?.check?.update_available && (
          <div className="bg-gradient-to-r from-blue-600/10 via-blue-500/10 to-indigo-600/10 border-b border-blue-500/20 px-4 lg:px-8 py-2 flex items-center justify-between gap-3">
            <div className="flex items-center gap-2 text-sm">
              <span className="relative flex h-2 w-2">
                <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-blue-400 opacity-75" />
                <span className="relative inline-flex rounded-full h-2 w-2 bg-blue-500" />
              </span>
              <span className="text-blue-400 font-medium">{t('version.updateAvailable', { version: versionInfo.check.latest_version })}</span>
              <a
                href={versionInfo.check.release_info?.release_notes_url || '#'}
                target="_blank"
                rel="noopener noreferrer"
                className="text-blue-400/70 hover:text-blue-300 underline underline-offset-2 text-xs ml-1"
              >
                {t('version.releaseNotes')}
              </a>
            </div>
            <div className="flex items-center gap-2">
              <button
                onClick={handleDownload}
                disabled={downloading}
                className="px-3 py-1 text-xs font-medium rounded-lg bg-blue-500/20 text-blue-300 hover:bg-blue-500/30 transition-colors disabled:opacity-50"
              >
                {downloading ? t('version.downloading') : t('version.download')}
              </button>
              <button
                onClick={() => setSettingsModalOpen(true)}
                className="text-blue-400/60 hover:text-blue-300 text-xs transition-colors"
              >
                {t('version.viewDetails')}
              </button>
            </div>
          </div>
        )}

        <div className="flex flex-1 min-h-0">
          {/* Level 3 Sub-nav */}
          {activeLevel3Parent && activeLevel3Parent.children && (
            <div className="w-[160px] shrink-0 border-r border-[var(--bg-border)] bg-[var(--bg-elevated)]/30 hidden lg:flex flex-col py-3 gap-0.5">
              <div className="px-4 pb-2 text-[10px] uppercase tracking-wider text-[var(--text-muted)] font-semibold">
                {t(activeLevel3Parent.label)}
              </div>
              {activeLevel3Parent.children.map(child => {
                const childActive = child.path && isActive(child.path)
                return (
                  <button
                    key={child.path}
                    onClick={() => { child.path && navigate(child.path) }}
                    aria-current={childActive ? 'page' : undefined}
                    className={`w-full flex items-center gap-2.5 px-4 py-2 text-sm font-medium transition-all duration-200 text-left relative ${
                      childActive
                        ? 'bg-blue-500/10 text-blue-400'
                        : 'text-[var(--text-secondary)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]'
                    }`}
                  >
                    {childActive && (
                      <span className="absolute left-0 top-1/2 -translate-y-1/2 w-[3px] h-5 bg-blue-500 rounded-r-full shadow-lg shadow-blue-500/50" />
                    )}
                    <span>{t(child.label)}</span>
                  </button>
                )
              })}
            </div>
          )}

          {/* Page content */}
          <div className="flex-1 p-4 lg:p-8 min-w-0">
            {children}
          </div>
        </div>
      </main>
    </div>
  )
}
