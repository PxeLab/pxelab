import { useState, useEffect, useRef, useCallback } from 'react'
import { useTranslation } from 'react-i18next'
import { Play, Pause, Trash2, Columns, LayoutGrid, Maximize, Minimize } from 'lucide-react'
import { Button } from '../components/ui/Button'
import { PageHeader } from '../components/ui/PageHeader'
import { getBaseURL } from '../api/client'
import { useSSE } from '../hooks/useSSE'
import { useFullscreen } from '../hooks/useFullscreen'

type LogLevel = 'debug' | 'info' | 'warn' | 'error'
type LayoutMode = 1 | 2 | 4

interface LogEntry {
  time: string
  level: LogLevel
  message: string
  service?: string
  mac?: string
  ip?: string
  attrs?: Record<string, unknown>
}

interface PanelConfig {
  id: number
  service: string
  level: LogLevel | ''
  logs: LogEntry[]
  paused: boolean
}

const SERVICES = [
  { value: '', label: 'logs.allServices' },
  { value: 'DHCP', label: 'DHCP' },
  { value: 'TFTP', label: 'TFTP' },
  { value: 'HTTP', label: 'HTTP' },
  { value: 'DNS', label: 'DNS' },
  { value: 'NFS', label: 'NFS' },
  { value: 'IPMI', label: 'IPMI' },
  { value: 'BOOT', label: 'BOOT' },
]

const LEVELS: { value: LogLevel | ''; label: string; color: string }[] = [
  { value: '', label: 'logs.allLevels', color: '' },
  { value: 'debug', label: 'DEBUG', color: 'text-gray-500' },
  { value: 'info', label: 'INFO', color: 'text-blue-400' },
  { value: 'warn', label: 'WARN', color: 'text-accent-yellow' },
  { value: 'error', label: 'ERROR', color: 'text-accent-red' },
]

const LEVEL_COLORS: Record<string, string> = {
  debug: 'bg-gray-500/20 text-gray-400',
  info: 'bg-blue-500/20 text-blue-400',
  warn: 'bg-accent-yellow/20 text-accent-yellow',
  error: 'bg-accent-red/20 text-accent-red',
}

const SERVICE_COLORS: Record<string, string> = {
  DHCP: 'bg-purple-500/20 text-purple-400',
  TFTP: 'bg-cyan-500/20 text-cyan-400',
  HTTP: 'bg-accent-green/20 text-accent-green',
  DNS: 'bg-orange-500/20 text-orange-400',
  NFS: 'bg-blue-500/20 text-blue-400',
  IPMI: 'bg-pink-500/20 text-pink-400',
  BOOT: 'bg-accent-yellow/20 text-accent-yellow',
}

function formatTime(iso: string): string {
  try {
    const d = new Date(iso)
    return d.toLocaleString()
  } catch {
    return iso
  }
}

function createPanel(id: number, defaultService = ''): PanelConfig {
  return { id, service: defaultService, level: '', logs: [], paused: false }
}

// 多面板布局时各面板默认选中的服务（面板 0 起依次分配）
const LAYOUT_SERVICES = ['HTTP', 'TFTP', 'DHCP', 'DNS']

export default function Logs() {
  const { t } = useTranslation()
  const [layout, setLayout] = useState<LayoutMode>(1)
  const [panels, setPanels] = useState<PanelConfig[]>([createPanel(0)])
  const [allPaused, setAllPaused] = useState(false)
  const bottomRefs = useRef<(HTMLDivElement | null)[]>([])
  const containerRefs = useRef<(HTMLDivElement | null)[]>([])
  const { ref: fullscreenRef, isFullscreen, toggle: toggleFullscreen } = useFullscreen<HTMLDivElement>()

  // SSE url 带面板 0 的过滤参数；url 变化时 useSSE 自动重连
  const params = new URLSearchParams()
  const svc = panels[0]?.service
  const lvl = panels[0]?.level
  if (svc) params.set('service', svc)
  if (lvl) params.set('level', lvl)
  const qs = params.toString()
  const streamUrl = getBaseURL() + '/api/v1/logs/stream' + (qs ? '?' + qs : '')

  useSSE<LogEntry>(streamUrl, (entry) => {
    setPanels(prev => prev.map(p => {
      if (p.paused || allPaused) return p

      // 按过滤条件判断
      if (p.service && entry.service !== p.service) return p
      if (p.level && entry.level !== p.level) return p

      const next = [...p.logs, entry]
      if (next.length > 500) next.splice(0, next.length - 500)
      return { ...p, logs: next }
    }))
  })

  // 自动滚动
  useEffect(() => {
    if (allPaused) return
    panels.forEach((_, i) => {
      const container = containerRefs.current[i]
      const bottom = bottomRefs.current[i]
      if (!container || !bottom) return
      // 只有用户已经在底部时才自动滚动
      if (container.scrollHeight - container.scrollTop - container.clientHeight < 50) {
        bottom.scrollIntoView({ behavior: 'smooth' })
      }
    })
  })

  // 布局变化时同步面板数量
  const changeLayout = useCallback((mode: LayoutMode) => {
    setLayout(mode)
    setPanels(prev => {
      const count = mode
      const next = [...prev]
      while (next.length < count) {
        const i = next.length
        const svc = mode > 1 ? LAYOUT_SERVICES[i] ?? '' : ''
        next.push(createPanel(i, svc))
      }
      return next.slice(0, count)
    })
    bottomRefs.current = bottomRefs.current.slice(0, mode)
  }, [])

  const clearPanel = useCallback((id: number) => {
    setPanels(prev => prev.map(p => p.id === id ? { ...p, logs: [] } : p))
  }, [])

  const togglePausePanel = useCallback((id: number) => {
    setPanels(prev => prev.map(p => p.id === id ? { ...p, paused: !p.paused } : p))
  }, [])

  const updatePanel = useCallback((id: number, upd: Partial<PanelConfig>) => {
    // 切换过滤条件时清空该面板的日志
    if ('service' in upd || 'level' in upd) {
      upd.logs = []
    }
    setPanels(prev => prev.map(p => p.id === id ? { ...p, ...upd } : p))
  }, [])

  const gridClasses = layout === 1 ? 'grid-cols-1' : layout === 2 ? 'grid-cols-1 lg:grid-cols-2' : 'grid-cols-1 sm:grid-cols-2 auto-rows-fr'

  return (
    <div>
      <PageHeader
        title={t('logs.title')}
        className="mb-4"
        actions={
          <>
            <span className="text-xs font-semibold text-[var(--text-secondary)] mr-1">{t('logs.layout')}</span>
            <Button
              variant={layout === 1 ? 'primary' : 'secondary'} size="sm"
              onClick={() => changeLayout(1)}
            >
              <Columns size={14} className="mr-1" /> 1
            </Button>
            <Button
              variant={layout === 2 ? 'primary' : 'secondary'} size="sm"
              onClick={() => changeLayout(2)}
            >
              <Columns size={14} className="mr-1" /> 2
            </Button>
            <Button
              variant={layout === 4 ? 'primary' : 'secondary'} size="sm"
              onClick={() => changeLayout(4)}
            >
              <LayoutGrid size={14} className="mr-1" /> 4
            </Button>
            <div className="w-px h-4 bg-[var(--bg-border)] mx-1" />
            <Button
              variant="secondary" size="sm"
              onClick={() => setAllPaused(!allPaused)}
            >
              {allPaused ? <Play size={14} className="mr-1" /> : <Pause size={14} className="mr-1" />}
              {allPaused ? t('logs.resumeAll') : t('logs.pauseAll')}
            </Button>
            <Button variant="ghost" size="sm" onClick={() => {
              setPanels(prev => prev.map(p => ({ ...p, logs: [] })))
            }}>
              <Trash2 size={14} className="mr-1" /> {t('logs.clearAll')}
            </Button>
            <span title={isFullscreen ? t('common.exitFullscreen') : t('common.fullscreen')}>
              <Button variant="ghost" size="sm" onClick={toggleFullscreen}>
                {isFullscreen ? <Minimize size={14} /> : <Maximize size={14} />}
              </Button>
            </span>
          </>
        }
      />

      {/* Log Panels */}
      <div ref={fullscreenRef} className={`grid ${gridClasses} gap-4 ${isFullscreen ? 'h-full bg-[var(--background)] p-4' : 'h-[calc(100vh-12rem)]'}`}>
        {panels.map((panel, idx) => (
          <div key={panel.id} className="flex flex-col min-h-0">
            {/* Panel Header */}
            <div className="flex items-center justify-between px-3 py-2 rounded-t-xl border border-[var(--bg-border)] border-b-0 bg-[var(--bg-card)] shrink-0">
              <div className="flex items-center gap-2">
                {/* Service filter */}
                <select
                  className="text-xs bg-[var(--bg-input)] border border-[var(--bg-border)] rounded px-1.5 py-1 text-[var(--text-primary)] outline-none"
                  value={panel.service}
                  onChange={e => updatePanel(panel.id, { service: e.target.value })}
                >
                  {SERVICES.map(s => (
                    <option key={s.value} value={s.value}>{t(s.label)}</option>
                  ))}
                </select>
                {/* Level filter */}
                <select
                  className="text-xs bg-[var(--bg-input)] border border-[var(--bg-border)] rounded px-1.5 py-1 text-[var(--text-primary)] outline-none"
                  value={panel.level}
                  onChange={e => updatePanel(panel.id, { level: e.target.value as LogLevel | '' })}
                >
                  {LEVELS.map(l => (
                    <option key={l.value} value={l.value}>{t(l.label)}</option>
                  ))}
                </select>
              </div>
              <div className="flex items-center gap-1">
                <span className="text-[11px] text-[var(--text-muted)] font-mono mr-1">{panel.logs.length}</span>
                <button
                  onClick={() => clearPanel(panel.id)}
                  className="p-1 rounded hover:bg-[var(--bg-hover)] text-[var(--text-muted)] hover:text-accent-red transition-colors"
                  title={t('logs.clear')}
                >
                  <Trash2 size={13} />
                </button>
                <button
                  onClick={() => togglePausePanel(panel.id)}
                  className="p-1 rounded hover:bg-[var(--bg-hover)] text-[var(--text-muted)] hover:text-[var(--text-primary)] transition-colors"
                  title={panel.paused ? t('logs.resume') : t('logs.pause')}
                >
                  {panel.paused ? <Play size={13} /> : <Pause size={13} />}
                </button>
              </div>
            </div>

            {/* Log Lines */}
            <div
              ref={el => { containerRefs.current[idx] = el }}
              className="flex-1 overflow-y-auto overflow-x-auto rounded-b-xl border border-[var(--bg-border)] bg-[var(--bg-card)] font-mono text-xs leading-relaxed"
            >
              {panel.logs.length === 0 ? (
                <div className="flex items-center justify-center h-full min-h-[100px] text-[var(--text-muted)] italic text-xs">
                  {t('logs.noLogs')}
                </div>
              ) : (
                panel.logs.map((entry, li) => (
                  <div key={li} className="flex items-center gap-2 px-3 py-0.5 hover:bg-[var(--bg-hover)] border-b border-[var(--bg-border)]/30 last:border-0 whitespace-nowrap">
                    <span className="text-[var(--text-muted)] shrink-0 tabular-nums">{formatTime(entry.time)}</span>
                    <span className={`shrink-0 font-semibold ${LEVEL_COLORS[entry.level] || ''} px-1 rounded text-[10px] leading-4`}>
                      {entry.level.toUpperCase()}
                    </span>
                    {entry.service && (
                      <span className={`shrink-0 font-semibold ${SERVICE_COLORS[entry.service] || ''} px-1 rounded text-[10px] leading-4`}>
                        {entry.service}
                      </span>
                    )}
                    <span className="text-[var(--text-primary)]">{entry.message}</span>
                    {(entry.mac || entry.ip || (entry.attrs && Object.keys(entry.attrs).length > 0)) && (
                      <span className="text-[11px] text-[var(--text-muted)] font-mono shrink-0">
                        {entry.mac && <span className="mr-2">mac={entry.mac}</span>}
                        {entry.ip && <span className="mr-2">ip={entry.ip}</span>}
                        {entry.attrs && Object.entries(entry.attrs).map(([k, v]) => (
                          <span key={k} className="mr-2">{k}={typeof v === 'string' ? v : JSON.stringify(v)}</span>
                        ))}
                      </span>
                    )}
                  </div>
                ))
              )}
              <div ref={el => { bottomRefs.current[idx] = el }} />
            </div>
          </div>
        ))}
      </div>
    </div>
  )
}
