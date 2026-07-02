import { type FC, type ReactNode, useState, useEffect, useCallback, useRef } from 'react'
import { useNavigate, useLocation } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { useTheme } from '../../hooks/useTheme'
import { usePalette } from '../../hooks/usePalette'
import { ThemeSwitcher } from '../ThemeSwitcher'
import { LangSwitch } from '../LangSwitch'
import { StatusDot } from '../ui/StatusDot'
import { getServices, startService, stopService, restartService, batchService, type ServiceInfo } from '../../api/client'
import SettingsModal from './SettingsModal'
import {
  LayoutDashboard, Server, FileCode, Activity, Settings,
  Monitor, ShieldCheck, Network, Menu, ChevronRight, ChevronLeft,
  HardDrive, Cpu,
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
    label: 'nav.section.manage',
    items: [
      { path: '/hosts', label: 'nav.hosts', icon: Server },
      { path: '/profiles', label: 'nav.profiles', icon: FileCode },
      { path: '/answer-templates', label: 'nav.answerTemplates', icon: FileCode },
      { path: '/access-control', label: 'nav.accessControl', icon: ShieldCheck },
      { path: '/install-tasks', label: 'nav.installTasks', icon: HardDrive },
      { path: '/bmc', label: 'nav.bmc', icon: Cpu },
    ] as NavItem[],
  },
  {
    label: 'nav.section.monitor',
    items: [
      { path: '/events', label: 'nav.events', icon: Activity },
      { path: '/logs', label: 'nav.logs', icon: Activity },
    ] as NavItem[],
  },
  {
    label: 'nav.section.settings',
    items: [
      { path: '/services/dhcp', label: 'nav.settings.dhcp', icon: Network },
      { path: '/services/tftp', label: 'nav.settings.tftp', icon: Monitor },
      { path: '/services/dns', label: 'nav.settings.dns', icon: Monitor },
      { path: '/netboot-catalog', label: 'nav.netboot', icon: Monitor },
    ] as NavItem[],
  },
]

interface Props {
  children: ReactNode
}

function ServiceDropdown() {
  const { t } = useTranslation()
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

  const load = useCallback(async () => {
    try {
      const res = await getServices()
      setServices(res.data)
    } catch { /* ignore */ }
  }, [])

  useEffect(() => { load(); const iv = setInterval(load, 5000); return () => clearInterval(iv) }, [load])

  const running = services.filter(s => s.status === 'running').length
  const stopped = services.filter(s => s.status === 'stopped').length
  const errors = services.filter(s => s.status === 'error').length

  const runOp = async (name: string, op: 'start' | 'stop' | 'restart') => {
    setOperating(prev => new Set(prev).add(name + op))
    try {
      if (op === 'start') await startService(name)
      else if (op === 'stop') await stopService(name)
      else await restartService(name)
      await load()
    } catch { /* ignore */ }
    setOperating(prev => { const next = new Set(prev); next.delete(name + op); return next })
  }

  const batchAll = async (op: 'start' | 'stop' | 'restart') => {
    const targets = services.filter(s => !s.protected && (op === 'start' ? s.status !== 'running' : true))
    if (targets.length === 0) return
    const names = targets.map(s => s.name)
    names.forEach(n => setOperating(prev => new Set(prev).add(n + op)))
    try {
      await batchService(op, names)
      await load()
    } catch { /* ignore */ }
    names.forEach(n => setOperating(prev => { const next = new Set(prev); next.delete(n + op); return next }))
  }

  const anyOperating = (op: string) => services.some(s => operating.has(s.name + op))

  return (
    <div className="relative" onMouseEnter={hoverOpen} onMouseLeave={hoverClose}>
      <button
        className="hidden sm:inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full text-xs font-mono font-semibold bg-[var(--bg-card)] border border-[var(--bg-border)] text-[var(--text-secondary)] hover:bg-[var(--bg-hover)] hover:shadow-sm transition-all duration-200"
      >
        <div className="flex items-center gap-2.5 py-0.5">
          <span className="flex items-center gap-1"><StatusDot color="green" /><span className="text-green-400 font-bold">{running}</span><span className="text-[10px] text-[var(--text-muted)]">{t('common.running')}</span></span>
          <span className="flex items-center gap-1"><StatusDot color="yellow" /><span className="text-yellow-400 font-bold">{stopped}</span><span className="text-[10px] text-[var(--text-muted)]">{t('common.stopped')}</span></span>
          <span className="flex items-center gap-1"><StatusDot color="red" /><span className="text-red-400 font-bold">{errors}</span><span className="text-[10px] text-[var(--text-muted)]">{t('common.error')}</span></span>
        </div>
      </button>
      {open && (
        <div onMouseEnter={hoverOpen} onMouseLeave={hoverClose}>
          <div className="absolute right-0 top-full mt-2 w-[520px] z-50 rounded-xl bg-[var(--bg-elevated)] border border-[var(--bg-border)] shadow-xl overflow-hidden animate-fade-in">
            {/* 一键操作 */}
            <div className="px-4 py-3 border-b border-[var(--bg-border)] bg-[var(--bg-base)]/50">
              <div className="flex items-center gap-1.5">
                <button onClick={() => batchAll('start')} disabled={anyOperating('start')}
                  className="px-2.5 py-1 text-[10px] font-medium rounded bg-green-500/15 text-green-400 hover:bg-green-500/25 disabled:opacity-40 transition-colors">
                  {anyOperating('start') ? '...' : t('common.startAll')}
                </button>
                <button onClick={() => batchAll('stop')} disabled={anyOperating('stop')}
                  className="px-2.5 py-1 text-[10px] font-medium rounded bg-red-500/15 text-red-400 hover:bg-red-500/25 disabled:opacity-40 transition-colors">
                  {anyOperating('stop') ? '...' : t('common.stopAll')}
                </button>
                <button onClick={() => batchAll('restart')} disabled={anyOperating('restart')}
                  className="px-2.5 py-1 text-[10px] font-medium rounded bg-orange-500/15 text-orange-400 hover:bg-orange-500/25 disabled:opacity-40 transition-colors">
                  {anyOperating('restart') ? '...' : t('common.restartAll')}
                </button>
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
                            <button onClick={() => runOp(svc.name, 'start')} disabled={operating.has(svc.name + 'start')}
                              className="px-2 py-0.5 text-[10px] font-medium rounded bg-green-500/15 text-green-400 hover:bg-green-500/25 disabled:opacity-40">
                              {operating.has(svc.name + 'start') ? '...' : t('common.start')}
                            </button>
                          )}
                          {svc.status === 'running' && (
                            <button onClick={() => runOp(svc.name, 'stop')} disabled={operating.has(svc.name + 'stop')}
                              className="px-2 py-0.5 text-[10px] font-medium rounded bg-red-500/15 text-red-400 hover:bg-red-500/25 disabled:opacity-40">
                              {operating.has(svc.name + 'stop') ? '...' : t('common.stop')}
                            </button>
                          )}
                          <button onClick={() => runOp(svc.name, 'restart')} disabled={operating.has(svc.name + 'restart')}
                            className="px-2 py-0.5 text-[10px] font-medium rounded bg-orange-500/15 text-orange-400 hover:bg-orange-500/25 disabled:opacity-40">
                            {operating.has(svc.name + 'restart') ? '...' : t('common.restart')}
                          </button>
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
  const { theme, toggleTheme } = useTheme()
  const { palette, setPalette } = usePalette()
  const [sidebarOpen, setSidebarOpen] = useState(false)
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false)
  const [settingsModalOpen, setSettingsModalOpen] = useState(false)
  const [expandedSections, setExpandedSections] = useState<Record<string, boolean>>(() => {
    const initial: Record<string, boolean> = {}
    for (const section of navSections) {
      for (const item of section.items) {
        if (item.children?.some(child => child.path && location.pathname.startsWith(child.path))) {
          initial[item.label] = true
        }
      }
    }
    return initial
  })

  const isActive = (path: string) => {
    if (path === '/') return location.pathname === '/'
    if (path === '/services') return location.pathname === '/services'
    return location.pathname.startsWith(path)
  }

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
    <div className="flex min-h-screen bg-[var(--bg-base)] text-[var(--text-primary)]">
      {/* Sidebar */}
      <aside
        className={`${
          sidebarOpen ? 'translate-x-0' : '-translate-x-full'
        } lg:translate-x-0 fixed top-0 left-0 bottom-0 bg-[var(--bg-elevated)]/80 backdrop-blur-2xl border-r border-[var(--bg-border)] z-50 flex flex-col overflow-hidden transition-all duration-300 ${
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
                  const expanded = expandedSections[item.label] ?? false
                  const hasActiveChild = item.children.some(child => child.path && isActive(child.path))
                  const Icon = item.icon
                  return (
                    <div key={item.label}>
                      <button
                        onClick={() => setExpandedSections(prev => ({...prev, [item.label]: !expanded}))}
                        className={`w-full flex items-center py-2 rounded-lg text-sm font-medium transition-all duration-200 text-left ${
                          sidebarCollapsed ? 'justify-center gap-0 px-3'
                        : 'gap-2.5 pl-5 pr-3 ' + (hasActiveChild
                            ? 'text-blue-400'
                            : 'text-[var(--text-secondary)] hover:bg-[var(--bg-card)] hover:text-[var(--text-primary)]')
                        }`}
                        title={sidebarCollapsed ? t(item.label) : undefined}
                      >
                        {Icon && <Icon size={16} className="shrink-0 opacity-70" />}
                        <span className={`${sidebarCollapsed ? 'hidden' : ''}`}>{t(item.label)}</span>
                        <ChevronRight size={14} className={`shrink-0 transition-transform duration-200 ${expanded ? 'rotate-90' : ''} ${sidebarCollapsed ? 'hidden' : ''}`} />
                      </button>
                      {expanded && !sidebarCollapsed && (
                        <div className="ml-3 mt-0.5 space-y-0.5 border-l border-[var(--bg-border)] pl-2">
                          {item.children.map(child => {
                            const childActive = child.path && isActive(child.path)
                            return (
                              <button
                                key={child.path}
                                onClick={() => { child.path && navigate(child.path); setSidebarOpen(false) }}
                                aria-current={childActive ? 'page' : undefined}
                                className={`w-full flex items-center gap-2.5 pl-7 pr-3 py-1.5 rounded-lg text-xs font-medium transition-all duration-200 text-left ${
                                  childActive
                                    ? 'bg-blue-500/10 text-blue-400'
                                    : 'text-[var(--text-secondary)] hover:bg-[var(--bg-card)] hover:text-[var(--text-primary)]'
                                }`}
                              >
                                <span>{t(child.label)}</span>
                              </button>
                            )
                          })}
                        </div>
                      )}
                    </div>
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

      {/* Overlay for mobile */}
      {sidebarOpen && (
        <div className="fixed inset-0 bg-black/50 z-40 lg:hidden" onClick={() => setSidebarOpen(false)} />
      )}

      {/* Main */}
      <main className={`flex-1 min-h-screen min-w-0 transition-all duration-300 ${sidebarCollapsed ? 'lg:ml-[64px]' : 'lg:ml-[200px]'}`}>
        {/* Top Bar */}
        <header className="h-16 border-b border-[var(--bg-border)] flex items-center justify-between px-4 lg:px-8 bg-[var(--bg-elevated)]/80 backdrop-blur-xl sticky top-0 z-30">
          <div className="absolute inset-x-0 bottom-0 h-[1px] bg-gradient-to-r from-transparent via-blue-500/30 to-transparent" />
          <div className="flex items-center gap-4">
            <button
              onClick={() => setSidebarOpen(true)}
              className="lg:hidden p-2 text-[var(--text-secondary)] hover:text-[var(--text-primary)]"
            >
              <Menu size={20} />
            </button>
            <div className="hidden">
              <h1 className="text-lg font-bold tracking-tight">{pageTitle()}</h1>
            </div>
          </div>
          <div className="flex items-center gap-3">
            <ServiceDropdown />
            <LangSwitch />
            <ThemeSwitcher theme={theme} palette={palette} onToggleTheme={toggleTheme} onChangePalette={setPalette} />
          </div>
        </header>

        <div className="p-4 lg:p-8">
          {children}
        </div>
      </main>
    </div>
  )
}
