import { useState, useEffect, useRef, useCallback } from 'react'
import { Play, Pause, Trash2, Columns, LayoutGrid } from 'lucide-react'
import { Button } from '../components/ui/Button'
import { getBaseURL } from '../api/client'

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
  { value: '', label: '全部' },
  { value: 'DHCP', label: 'DHCP' },
  { value: 'TFTP', label: 'TFTP' },
  { value: 'HTTP', label: 'HTTP' },
  { value: 'DNS', label: 'DNS' },
  { value: 'IPMI', label: 'IPMI' },
  { value: 'BOOT', label: 'BOOT' },
]

const LEVELS: { value: LogLevel | ''; label: string; color: string }[] = [
  { value: '', label: '全部', color: '' },
  { value: 'debug', label: 'DEBUG', color: 'text-gray-500' },
  { value: 'info', label: 'INFO', color: 'text-blue-400' },
  { value: 'warn', label: 'WARN', color: 'text-yellow-500' },
  { value: 'error', label: 'ERROR', color: 'text-red-500' },
]

const LEVEL_COLORS: Record<string, string> = {
  debug: 'bg-gray-500/20 text-gray-400',
  info: 'bg-blue-500/20 text-blue-400',
  warn: 'bg-yellow-500/20 text-yellow-500',
  error: 'bg-red-500/20 text-red-500',
}

const SERVICE_COLORS: Record<string, string> = {
  DHCP: 'bg-purple-500/20 text-purple-400',
  TFTP: 'bg-cyan-500/20 text-cyan-400',
  HTTP: 'bg-green-500/20 text-green-400',
  DNS: 'bg-orange-500/20 text-orange-400',
  IPMI: 'bg-pink-500/20 text-pink-400',
  BOOT: 'bg-yellow-500/20 text-yellow-400',
}

function formatTime(iso: string): string {
  try {
    const d = new Date(iso)
    return d.toLocaleTimeString('zh-CN', { hour12: false })
  } catch {
    return iso
  }
}

function createPanel(id: number, defaultService = ''): PanelConfig {
  return { id, service: defaultService, level: '', logs: [], paused: false }
}

// 多面板布局时各面板默认选中的服务（面板 0 起依次分配）
const LAYOUT_SERVICES = ['DHCP', 'TFTP', 'HTTP', 'DNS', 'IPMI']

export default function Logs() {
  const [layout, setLayout] = useState<LayoutMode>(1)
  const [panels, setPanels] = useState<PanelConfig[]>([createPanel(0)])
  const [allPaused, setAllPaused] = useState(false)
  const allPausedRef = useRef(allPaused) // 同步 allPaused 到 ref，避免闭包捕获过期值
  const [sseKey, setSseKey] = useState(0) // 递增后重连 SSE
  const eventSourceRef = useRef<EventSource | null>(null)
  const bottomRefs = useRef<(HTMLDivElement | null)[]>([])
  const autoScrollRef = useRef<(boolean)[]>([])

  autoScrollRef.current = panels.map((_, i) => autoScrollRef.current[i] ?? true)
  allPausedRef.current = allPaused // 同步到 ref，供 SSE 回调中读取

  // 连接 SSE（带过滤参数）
  useEffect(() => {
    let reconnectTimer: ReturnType<typeof setTimeout> | undefined

    function connect() {
      const params = new URLSearchParams()
      const svc = panels[0]?.service
      const lvl = panels[0]?.level
      if (svc) params.set('service', svc)
      if (lvl) params.set('level', lvl)
      const qs = params.toString()
      const url = getBaseURL() + '/api/v1/logs/stream' + (qs ? '?' + qs : '')

      if (eventSourceRef.current) {
        eventSourceRef.current.close()
      }

      const es = new EventSource(url)
      eventSourceRef.current = es

      es.onmessage = (e) => {
        try {
          const entry: LogEntry = JSON.parse(e.data)
          setPanels(prev => prev.map(p => {
            if (p.paused || allPausedRef.current) return p

            // 按过滤条件判断
            if (p.service && entry.service !== p.service) return p
            if (p.level && entry.level !== p.level) return p

            const next = [...p.logs, entry]
            if (next.length > 500) next.splice(0, next.length - 500)
            return { ...p, logs: next }
          }))
        } catch { /* ignore parse errors */ }
      }

      es.onerror = () => {
        es.close()
        reconnectTimer = setTimeout(connect, 3000)
      }
    }

    connect()

    return () => {
      clearTimeout(reconnectTimer)
      eventSourceRef.current?.close()
      eventSourceRef.current = null
    }
  }, [sseKey])

  // 自动滚动
  useEffect(() => {
    panels.forEach((_, i) => {
      if (autoScrollRef.current[i] && bottomRefs.current[i]) {
        bottomRefs.current[i]!.scrollIntoView({ behavior: 'smooth' })
      }
    })
  })

  // 布局变化时同步面板数量
  const changeLayout = useCallback((mode: LayoutMode) => {
    setLayout(mode)
    setPanels(prev => {
      const count = mode
      while (prev.length < count) {
        const i = prev.length
        const svc = mode > 1 ? LAYOUT_SERVICES[i] ?? '' : ''
        prev.push(createPanel(i, svc))
      }
      return prev.slice(0, count)
    })
    bottomRefs.current = bottomRefs.current.slice(0, mode)
    setSseKey(k => k + 1) // 面板 0 服务可能变化，重连 SSE
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
    // 第一个面板过滤变化时重连 SSE（让服务端历史过滤生效）
    if (id === 0 && ('service' in upd || 'level' in upd)) {
      setSseKey(k => k + 1)
    }
    setPanels(prev => prev.map(p => p.id === id ? { ...p, ...upd } : p))
  }, [])

  const gridClasses = layout === 1 ? 'grid-cols-1' : layout === 2 ? 'grid-cols-1 lg:grid-cols-2' : 'grid-cols-1 sm:grid-cols-2'

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-lg font-bold text-[var(--text-primary)]">日志</h1>
      </div>
      {/* Toolbar */}
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-2">
          <span className="text-xs font-semibold text-[var(--text-secondary)] mr-1">布局</span>
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
        </div>
        <div className="flex items-center gap-2">
          <Button
            variant="secondary" size="sm"
            onClick={() => setAllPaused(!allPaused)}
          >
            {allPaused ? <Play size={14} className="mr-1" /> : <Pause size={14} className="mr-1" />}
            {allPaused ? '恢复全部' : '暂停全部'}
          </Button>
          <Button variant="ghost" size="sm" onClick={() => {
            setAllPaused(false)
            setPanels(prev => prev.map(p => ({ ...p, logs: [], paused: false })))
            setSseKey(k => k + 1)
          }}>
            <Trash2 size={14} className="mr-1" /> 清空全部
          </Button>
        </div>
      </div>

      {/* Log Panels */}
      <div className={`grid ${gridClasses} gap-4 h-[calc(100vh-12rem)]`}>
        {panels.map((panel, idx) => (
          <div key={panel.id} className="flex flex-col min-h-0">
            {/* Panel Header */}
            <div className="flex items-center justify-between px-3 py-2 rounded-t-xl border border-[var(--bg-border)] border-b-0 bg-[var(--bg-card)] shrink-0">
              <div className="flex items-center gap-2">
                {/* Service filter */}
                <select
                  className="text-xs bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded px-1.5 py-1 text-[var(--text-primary)] outline-none"
                  value={panel.service}
                  onChange={e => updatePanel(panel.id, { service: e.target.value })}
                >
                  {SERVICES.map(s => (
                    <option key={s.value} value={s.value}>{s.label}</option>
                  ))}
                </select>
                {/* Level filter */}
                <select
                  className="text-xs bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded px-1.5 py-1 text-[var(--text-primary)] outline-none"
                  value={panel.level}
                  onChange={e => updatePanel(panel.id, { level: e.target.value as LogLevel | '' })}
                >
                  {LEVELS.map(l => (
                    <option key={l.value} value={l.value}>{l.label}</option>
                  ))}
                </select>
              </div>
              <div className="flex items-center gap-1">
                <span className="text-[11px] text-[var(--text-muted)] font-mono mr-1">{panel.logs.length}</span>
                <button
                  onClick={() => clearPanel(panel.id)}
                  className="p-1 rounded hover:bg-[var(--bg-hover)] text-[var(--text-muted)] hover:text-red-400 transition-colors"
                  title="清空"
                >
                  <Trash2 size={13} />
                </button>
                <button
                  onClick={() => togglePausePanel(panel.id)}
                  className="p-1 rounded hover:bg-[var(--bg-hover)] text-[var(--text-muted)] hover:text-[var(--text-primary)] transition-colors"
                  title={panel.paused ? '恢复' : '暂停'}
                >
                  {panel.paused ? <Play size={13} /> : <Pause size={13} />}
                </button>
              </div>
            </div>

            {/* Log Lines */}
            <div
              className="flex-1 overflow-y-auto rounded-b-xl border border-[var(--bg-border)] bg-[var(--bg-card)] font-mono text-xs leading-relaxed"
              onScroll={e => {
                const el = e.currentTarget
                autoScrollRef.current[idx] = el.scrollHeight - el.scrollTop - el.clientHeight < 50
              }}
            >
              {panel.logs.length === 0 ? (
                <div className="flex items-center justify-center h-full min-h-[100px] text-[var(--text-muted)] italic text-xs">
                  暂无日志，服务产生日志时实时显示
                </div>
              ) : (
                panel.logs.map((entry, li) => (
                  <div key={li} className="flex items-start gap-2 px-3 py-0.5 hover:bg-[var(--bg-hover)] border-b border-[var(--bg-border)]/30 last:border-0">
                    <span className="text-[var(--text-muted)] shrink-0 pt-0.5 tabular-nums">{formatTime(entry.time)}</span>
                    <span className={`shrink-0 font-semibold ${LEVEL_COLORS[entry.level] || ''} px-1 rounded text-[10px] leading-4`}>
                      {entry.level.toUpperCase()}
                    </span>
                    {entry.service && (
                      <span className={`shrink-0 font-semibold ${SERVICE_COLORS[entry.service] || ''} px-1 rounded text-[10px] leading-4`}>
                        {entry.service}
                      </span>
                    )}
                    <div className="flex-1 min-w-0">
                      <span className="text-[var(--text-primary)] break-all">{entry.message}</span>
                      {(entry.mac || entry.ip || (entry.attrs && Object.keys(entry.attrs).length > 0)) && (
                        <div className="text-[11px] text-[var(--text-muted)] leading-snug mt-0.5 flex flex-wrap gap-x-2 gap-y-0">
                          {entry.mac && <span className="font-mono">mac={entry.mac}</span>}
                          {entry.ip && <span className="font-mono">ip={entry.ip}</span>}
                          {entry.attrs && Object.entries(entry.attrs).map(([k, v]) => (
                            <span key={k} className="font-mono">{k}={typeof v === 'string' ? v : JSON.stringify(v)}</span>
                          ))}
                        </div>
                      )}
                    </div>
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
