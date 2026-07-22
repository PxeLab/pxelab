import { useState, useEffect, useRef } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { Bell, CheckCheck, Info, CheckCircle2, AlertTriangle, XCircle } from 'lucide-react'
import { useNotifications, type Notification } from '../hooks/useNotifications'

const typeIcon: Record<Notification['type'], { icon: typeof Info; className: string }> = {
  info: { icon: Info, className: 'text-blue-400' },
  success: { icon: CheckCircle2, className: 'text-accent-green' },
  warning: { icon: AlertTriangle, className: 'text-accent-yellow' },
  error: { icon: XCircle, className: 'text-accent-red' },
}

export function NotificationCenter() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { notifications, unreadCount, markRead, markAllRead } = useNotifications()
  const [open, setOpen] = useState(false)
  const rootRef = useRef<HTMLDivElement>(null)

  // 点击面板外部关闭
  useEffect(() => {
    if (!open) return
    const onPointerDown = (e: MouseEvent) => {
      if (rootRef.current && !rootRef.current.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', onPointerDown)
    return () => document.removeEventListener('mousedown', onPointerDown)
  }, [open])

  const relativeTime = (time: string): string => {
    const ts = new Date(time).getTime()
    if (!ts || Number.isNaN(ts)) return ''
    const diff = Math.max(0, Date.now() - ts)
    const minutes = Math.floor(diff / 60_000)
    if (minutes < 1) return t('notifications.time.justNow')
    if (minutes < 60) return t('notifications.time.minutesAgo', { count: minutes })
    const hours = Math.floor(minutes / 60)
    if (hours < 24) return t('notifications.time.hoursAgo', { count: hours })
    return t('notifications.time.daysAgo', { count: Math.floor(hours / 24) })
  }

  const handleItemClick = (n: Notification) => {
    markRead(n.id)
    if (n.link) {
      navigate(n.link)
      setOpen(false)
    }
  }

  return (
    <div className="relative" ref={rootRef}>
      <button
        onClick={() => setOpen(v => !v)}
        className="relative p-2 rounded-lg text-[var(--foreground-secondary)] hover:bg-[var(--hover)] hover:text-[var(--foreground)] transition-colors"
        title={t('notifications.title')}
      >
        <Bell size={16} />
        {unreadCount > 0 && (
          <span className="absolute -top-0.5 -right-0.5 min-w-[16px] h-4 px-1 rounded-full bg-accent-red text-white text-[10px] font-bold leading-4 text-center">
            {unreadCount > 99 ? '99+' : unreadCount}
          </span>
        )}
      </button>

      {open && (
        <div className="absolute right-0 top-full mt-2 w-[380px] z-50 rounded-xl bg-[var(--popover)] border border-[var(--border)] shadow-xl overflow-hidden animate-fade-in">
          {/* 头部 */}
          <div className="flex items-center justify-between px-4 py-3 border-b border-[var(--border)] bg-[var(--background)]/50">
            <span className="text-sm font-semibold text-[var(--foreground)]">{t('notifications.title')}</span>
            <button
              onClick={markAllRead}
              disabled={unreadCount === 0}
              className="inline-flex items-center gap-1 text-xs text-blue-400 hover:text-blue-300 disabled:text-[var(--foreground-muted)] disabled:cursor-default transition-colors"
            >
              <CheckCheck size={13} />
              {t('notifications.markAllRead')}
            </button>
          </div>

          {/* 列表 */}
          <div className="max-h-[480px] overflow-y-auto">
            {notifications.length === 0 ? (
              <p className="text-sm text-[var(--foreground-muted)] text-center py-12">{t('notifications.empty')}</p>
            ) : (
              notifications.map(n => {
                const { icon: Icon, className: iconClass } = typeIcon[n.type] || typeIcon.info
                return (
                  <button
                    key={n.id}
                    onClick={() => handleItemClick(n)}
                    className={`relative w-full flex items-start gap-2.5 px-4 py-3 text-left hover:bg-[var(--hover)]/50 border-b border-[var(--border)] last:border-0 transition-colors ${n.link ? 'cursor-pointer' : 'cursor-default'}`}
                  >
                    {!n.read && (
                      <span className="absolute left-0 top-1/2 -translate-y-1/2 w-[3px] h-8 bg-blue-500 rounded-r-full" />
                    )}
                    <Icon size={15} className={`shrink-0 mt-0.5 ${iconClass}`} />
                    <div className="min-w-0 flex-1">
                      <div className="flex items-baseline justify-between gap-2">
                        <span className={`text-xs truncate ${n.read ? 'font-medium text-[var(--foreground-secondary)]' : 'font-bold text-[var(--foreground)]'}`}>
                          {n.title}
                        </span>
                        <span className="shrink-0 text-[10px] text-[var(--foreground-muted)]">{relativeTime(n.time)}</span>
                      </div>
                      <p className="text-xs text-[var(--foreground-muted)] truncate mt-0.5">{n.message}</p>
                    </div>
                  </button>
                )
              })
            )}
          </div>
        </div>
      )}
    </div>
  )
}
