import { type FC, type ReactNode, useState } from 'react'
import { useNavigate, useLocation } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { useTheme } from '../../hooks/useTheme'
import { ThemeToggle } from '../ThemeToggle'
import { LangSwitch } from '../LangSwitch'
import { StatusDot } from '../ui/StatusDot'
import {
  LayoutDashboard, Server, FileCode, FolderOpen, Activity, Settings,
  Monitor, Globe, ShieldCheck, Network, Menu,
} from 'lucide-react'

interface NavItem {
  path: string
  label: string
  icon: FC<{ size?: number; className?: string }>
  badge?: string
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
      { path: '/dns/records', label: 'DNS 记录', icon: Globe },
      { path: '/access-control', label: '访问控制', icon: ShieldCheck },
      { path: '/leases', label: 'nav.leases', icon: Network },
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
    label: 'nav.section.system',
    items: [
      { path: '/services', label: 'nav.services', icon: Monitor },
      { path: '/settings', label: 'nav.settings', icon: Settings },
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

  const isActive = (path: string) => {
    if (path === '/') return location.pathname === '/'
    return location.pathname.startsWith(path)
  }

  const pageTitle = () => {
    for (const section of navSections) {
      for (const item of section.items) {
        if (isActive(item.path)) return t(item.label)
      }
    }
    return t('nav.dashboard')
  }

  return (
    <div className="flex min-h-screen bg-[var(--bg-base)] text-[var(--text-primary)]">
      {/* Sidebar */}
      <aside
        className={`fixed top-0 left-0 bottom-0 w-[260px] bg-[var(--bg-elevated)] border-r border-[var(--bg-border)] z-50 flex flex-col transition-transform duration-200 ${
          sidebarOpen ? 'translate-x-0' : '-translate-x-full'
        } lg:translate-x-0`}
      >
        <div className="flex items-center gap-3 px-5 py-5 border-b border-[var(--bg-border)]">
          <div className="w-9 h-9 rounded-lg bg-gradient-to-br from-[#0f172a] via-[#1e293b] to-[#0f172a] flex items-center justify-center font-extrabold text-sm text-blue-500 border border-[var(--bg-border)] shrink-0">
            PX
          </div>
          <span className="text-base font-bold tracking-tight">
            Pxe<span className="text-[var(--text-muted)] font-medium">Go</span>
          </span>
        </div>

        <nav className="flex-1 px-2.5 py-3 flex flex-col gap-1">
          {navSections.map((section) => (
            <div key={section.label}>
              <div className="px-2.5 py-1.5 text-[10px] uppercase tracking-wider text-[var(--text-muted)] font-semibold">
                {t(section.label)}
              </div>
              {section.items.map((item) => {
                const Icon = item.icon
                const active = isActive(item.path)
                return (
                  <button
                    key={item.path}
                    onClick={() => { navigate(item.path); setSidebarOpen(false) }}
                    className={`w-full flex items-center gap-2.5 px-3 py-2 rounded-lg text-sm font-medium transition-all duration-200 text-left relative ${
                      active
                        ? 'bg-blue-500/10 text-blue-400'
                        : 'text-[var(--text-secondary)] hover:bg-[var(--bg-card)] hover:text-[var(--text-primary)]'
                    }`}
                  >
                    {active && (
                      <span className="absolute left-[-10px] top-1/2 -translate-y-1/2 w-[3px] h-5 bg-blue-500 rounded-r-full" />
                    )}
                    <Icon size={16} className="shrink-0 opacity-70" />
                    <span className="flex-1">{t(item.label)}</span>
                    {item.badge && (
                      <span className={`text-[11px] font-mono px-2 py-0.5 rounded-full ${
                        active ? 'bg-blue-500/15 text-blue-400' : 'bg-[var(--bg-card)] text-[var(--text-muted)]'
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

        <div className="px-2.5 py-3 border-t border-[var(--bg-border)]">
          <button
            onClick={() => navigate('/settings')}
            className="w-full flex items-center gap-2.5 px-3 py-2 rounded-lg text-sm text-[var(--text-secondary)] hover:bg-[var(--bg-card)] hover:text-[var(--text-primary)] transition-colors"
          >
            <span className="text-sm opacity-70">{'ℹ'}</span>
            <span className="flex-1">{t('nav.about', '关于')}</span>
            <span className="text-[11px] font-mono text-[var(--text-muted)]">v0.1.0</span>
          </button>
        </div>
      </aside>

      {/* Overlay for mobile */}
      {sidebarOpen && (
        <div className="fixed inset-0 bg-black/50 z-40 lg:hidden" onClick={() => setSidebarOpen(false)} />
      )}

      {/* Main */}
      <main className="flex-1 lg:ml-[260px] min-h-screen">
        {/* Top Bar */}
        <header className="h-16 border-b border-[var(--bg-border)] flex items-center justify-between px-4 lg:px-8 bg-[var(--bg-elevated)] sticky top-0 z-30">
          <div className="flex items-center gap-4">
            <button
              onClick={() => setSidebarOpen(true)}
              className="lg:hidden p-2 text-[var(--text-secondary)] hover:text-[var(--text-primary)]"
            >
              <Menu size={20} />
            </button>
            <div>
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
