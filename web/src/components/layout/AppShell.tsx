import { type FC, type ReactNode, useState } from 'react'
import { useNavigate, useLocation } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { useTheme } from '../../hooks/useTheme'
import { ThemeToggle } from '../ThemeToggle'
import { LangSwitch } from '../LangSwitch'
import { StatusDot } from '../ui/StatusDot'
import SettingsModal from './SettingsModal'
import {
  LayoutDashboard, Server, FileCode, FolderOpen, Activity, Settings,
  Monitor, ShieldCheck, Network, Menu, ChevronRight,
  PanelLeftClose, PanelLeftOpen, HardDrive, Cpu,
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
      { path: '/files', label: 'nav.files', icon: FolderOpen },
      { path: '/netboot-catalog', label: 'nav.netboot', icon: Server },
      { path: '/answer-templates', label: '应答模板', icon: FileCode },
      { path: '/access-control', label: '访问控制', icon: ShieldCheck },
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
      { path: '/settings/dhcp', label: 'nav.settings.dhcp', icon: Network },
      { path: '/settings/tftp', label: 'nav.settings.tftp', icon: Monitor },
      { path: '/settings/dns', label: 'nav.settings.dns', icon: Monitor },
      { path: '/settings/netboot', label: 'nav.settings.netboot', icon: Monitor },
      { path: '/services', label: 'nav.services', icon: Monitor },
    ] as NavItem[],
  },
]

interface Props {
  children: ReactNode
}

export const AppShell: FC<Props> = ({ children }) => {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const location = useLocation()
  const { theme, toggleTheme } = useTheme()
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
    if (path === '/settings') return location.pathname === '/settings'
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
        } lg:translate-x-0 lg:relative fixed top-0 left-0 bottom-0 bg-[var(--bg-elevated)] border-r border-[var(--bg-border)] z-50 flex flex-col overflow-hidden transition-all duration-200 ${
          sidebarCollapsed ? 'w-[64px]' : 'w-[260px]'
        }`}
      >
        <div className={`flex items-center gap-3 border-b border-[var(--bg-border)] h-16 ${sidebarCollapsed ? 'justify-center px-0' : 'px-5'}`}>
          <div className="w-9 h-9 rounded-lg bg-gradient-to-br from-[#0f172a] via-[#1e293b] to-[#0f172a] flex items-center justify-center font-extrabold text-sm text-blue-500 border border-[var(--bg-border)] shrink-0">
            PX
          </div>
          <span className={`text-base font-bold tracking-tight transition-opacity duration-200 ${sidebarCollapsed ? 'opacity-0 w-0 overflow-hidden' : ''}`}>
            Pxe<span className="text-[var(--text-muted)] font-medium">Go</span>
          </span>
        </div>

        <nav className="flex-1 overflow-y-auto min-h-0 px-2.5 py-3 flex flex-col gap-1">
          {navSections.map((section) => (
            <div key={section.label}>
              <div className={`px-2.5 py-1.5 text-[10px] uppercase tracking-wider text-[var(--text-muted)] font-semibold transition-opacity duration-200 ${sidebarCollapsed ? 'opacity-0 h-0 overflow-hidden py-0' : ''}`}>
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
                        className={`w-full flex items-center px-3 py-2 rounded-lg text-sm font-medium transition-all duration-200 text-left ${
                          sidebarCollapsed ? 'justify-center gap-0'
                        : 'gap-2.5 ' + (hasActiveChild
                            ? 'text-blue-400'
                            : 'text-[var(--text-secondary)] hover:bg-[var(--bg-card)] hover:text-[var(--text-primary)]')
                        }`}
                        title={sidebarCollapsed ? t(item.label) : undefined}
                      >
                        {Icon && <Icon size={16} className="shrink-0 opacity-70" />}
                        <span className={`flex-1 ${sidebarCollapsed ? 'hidden' : ''}`}>{t(item.label)}</span>
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
                                className={`w-full flex items-center gap-2.5 px-3 py-1.5 rounded-lg text-xs font-medium transition-all duration-200 text-left ${
                                  childActive
                                    ? 'bg-blue-500/10 text-blue-400'
                                    : 'text-[var(--text-secondary)] hover:bg-[var(--bg-card)] hover:text-[var(--text-primary)]'
                                }`}
                              >
                                <span className="flex-1">{t(child.label)}</span>
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
                    className={`w-full flex items-center px-3 py-2 rounded-lg text-sm font-medium transition-all duration-200 text-left relative ${
                      sidebarCollapsed ? 'justify-center gap-0'
                    : 'gap-2.5 ' + (active
                        ? 'bg-blue-500/10 text-blue-400'
                        : 'text-[var(--text-secondary)] hover:bg-[var(--bg-card)] hover:text-[var(--text-primary)]')
                    }`}
                    title={sidebarCollapsed ? t(item.label) : undefined}
                  >
                    {active && !sidebarCollapsed && (
                      <span className="absolute left-[-10px] top-1/2 -translate-y-1/2 w-[3px] h-5 bg-blue-500 rounded-r-full" />
                    )}
                    <Icon size={16} className="shrink-0 opacity-70" />
                    <span className={`flex-1 ${sidebarCollapsed ? 'hidden' : ''}`}>{t(item.label)}</span>
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
          <div className={`px-2.5 pt-3 mt-auto border-t border-[var(--bg-border)] transition-opacity duration-200 ${sidebarCollapsed ? 'opacity-0 h-0 overflow-hidden py-0 border-none' : ''}`}>
            <button
              onClick={() => setSettingsModalOpen(true)}
              className="w-full flex items-center gap-2.5 px-3 py-2 rounded-lg text-sm text-[var(--text-secondary)] hover:bg-[var(--bg-card)] hover:text-[var(--text-primary)] transition-colors"
            >
              <Settings size={16} className="opacity-70 shrink-0" />
              <span className="flex-1">{t('settings.title')}</span>
            </button>
          </div>
        </nav>
      </aside>

      <SettingsModal open={settingsModalOpen} onClose={() => setSettingsModalOpen(false)} />

      {/* Overlay for mobile */}
      {sidebarOpen && (
        <div className="fixed inset-0 bg-black/50 z-40 lg:hidden" onClick={() => setSidebarOpen(false)} />
      )}

      {/* Main */}
      <main className="flex-1 min-h-screen min-w-0">
        {/* Top Bar */}
        <header className="h-16 border-b border-[var(--bg-border)] flex items-center justify-between px-4 lg:px-8 bg-[var(--bg-elevated)] sticky top-0 z-30">
          <div className="flex items-center gap-4">
            <button
              onClick={() => setSidebarOpen(true)}
              className="lg:hidden p-2 text-[var(--text-secondary)] hover:text-[var(--text-primary)]"
            >
              <Menu size={20} />
            </button>
            <button
              onClick={() => setSidebarCollapsed(prev => !prev)}
              className="hidden lg:flex p-2 text-[var(--text-secondary)] hover:text-[var(--text-primary)]"
              title={sidebarCollapsed ? '展开侧边栏' : '折叠侧边栏'}
            >
              {sidebarCollapsed ? <PanelLeftOpen size={20} /> : <PanelLeftClose size={20} />}
            </button>
            <div className="hidden">
              <h1 className="text-lg font-bold tracking-tight">{pageTitle()}</h1>
            </div>
          </div>
          <div className="flex items-center gap-3">
            <span className="hidden sm:inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full text-xs font-mono font-semibold bg-[var(--bg-card)] border border-[var(--bg-border)] text-[var(--text-secondary)]">
              <StatusDot color="green" />
              All Services Running
            </span>
            <LangSwitch />
            <ThemeToggle theme={theme} onToggle={toggleTheme} />
          </div>
        </header>

        <div className="p-4 lg:p-8">
          {children}
        </div>
      </main>
    </div>
  )
}
