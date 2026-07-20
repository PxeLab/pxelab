import { useState, useEffect, useCallback } from 'react'
import { useTranslation } from 'react-i18next'
import { Search, Filter } from 'lucide-react'
import { Card } from '../components/ui/Card'
import { Button } from '../components/ui/Button'
import { Pagination } from '../components/ui/Pagination'
import { useToast } from '../components/ui/Toast'
import { useUIConfig } from '../contexts/UIConfigContext'
import { api, type AuditLog } from '../api/client'

const actionColors: Record<string, string> = {
  CREATE: 'bg-green-500/10 text-green-400 border-green-500/30',
  UPDATE: 'bg-blue-500/10 text-blue-400 border-blue-500/30',
  DELETE: 'bg-red-500/10 text-red-400 border-red-500/30',
}

export default function AuditLogs() {
  const { t } = useTranslation()
  const { error: toastError } = useToast()
  const { pageSize } = useUIConfig()
  const [logs, setLogs] = useState<AuditLog[]>([])
  const [loading, setLoading] = useState(true)
  const [page, setPage] = useState(1)
  const [total, setTotal] = useState(0)
  const [actionFilter, setActionFilter] = useState('')
  const [resourceFilter, setResourceFilter] = useState('')
  const [ipFilter, setIpFilter] = useState('')
  const [searchText, setSearchText] = useState('')

  const loadLogs = useCallback(async () => {
    setLoading(true)
    try {
      const params: Record<string, string> = { page: String(page), size: String(pageSize) }
      if (actionFilter) params.action = actionFilter
      if (resourceFilter) params.resource = resourceFilter
      if (ipFilter) params.remote_ip = ipFilter
      const res = await api.getAuditLogs(params)
      let items: AuditLog[] = res.data.logs || []
      if (searchText) {
        const q = searchText.toLowerCase()
        items = items.filter(l =>
          l.resource?.toLowerCase().includes(q) ||
          l.resource_id?.toLowerCase().includes(q) ||
          l.remote_ip?.toLowerCase().includes(q) ||
          l.detail?.toLowerCase().includes(q)
        )
      }
      setLogs(items)
      setTotal(res.data.meta?.total || items.length)
    } catch { toastError(t('events.loadFailed')) }
    finally { setLoading(false) }
  }, [page, pageSize, actionFilter, resourceFilter, ipFilter, searchText])

  useEffect(() => { loadLogs() }, [loadLogs])

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-lg font-bold text-[var(--text-primary)]">{t('audit.title', 'Audit Logs')}</h1>
      </div>

      {/* Filters */}
      <div className="flex items-center gap-3 mb-5 flex-wrap">
        <div className="relative">
          <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-[var(--text-muted)]" />
          <input
            value={searchText}
            onChange={e => { setSearchText(e.target.value); setPage(1) }}
            placeholder={t('audit.searchPlaceholder', 'Search...')}
            className="pl-8 pr-3 py-1.5 text-xs bg-[var(--bg-card)] border border-[var(--bg-border)] rounded-lg text-[var(--text-primary)] outline-none focus:border-blue-500 w-48"
          />
        </div>

        {(['', 'CREATE', 'UPDATE', 'DELETE'] as const).map(a => (
          <button
            key={a}
            onClick={() => { setActionFilter(a); setPage(1) }}
            className={`px-3 py-1.5 rounded-lg border text-xs font-semibold transition-all ${
              actionFilter === a
                ? a ? `${actionColors[a]} border` : 'bg-blue-500/10 text-blue-400 border-blue-500/30'
                : 'border-[var(--bg-border)] bg-[var(--bg-card)] text-[var(--text-secondary)] hover:border-[var(--text-muted)]'
            }`}
          >
            {a || t('audit.allActions', 'All')}
          </button>
        ))}

        <input
          value={resourceFilter}
          onChange={e => { setResourceFilter(e.target.value); setPage(1) }}
          placeholder={t('audit.resourceFilter', 'Resource...')}
          className="px-3 py-1.5 text-xs bg-[var(--bg-card)] border border-[var(--bg-border)] rounded-lg text-[var(--text-primary)] outline-none focus:border-blue-500 w-36"
        />

        <input
          value={ipFilter}
          onChange={e => { setIpFilter(e.target.value); setPage(1) }}
          placeholder={t('audit.ipFilter', 'IP...')}
          className="px-3 py-1.5 text-xs bg-[var(--bg-card)] border border-[var(--bg-border)] rounded-lg text-[var(--text-primary)] outline-none focus:border-blue-500 w-32"
        />

        {(actionFilter || resourceFilter || ipFilter) && (
          <Button variant="secondary" size="sm" onClick={() => { setActionFilter(''); setResourceFilter(''); setIpFilter(''); setSearchText(''); setPage(1) }}>
            <Filter size={12} />
            {t('audit.clearFilters', 'Clear')}
          </Button>
        )}
      </div>

      <Card padding={false}>
        {loading ? (
          <div className="p-5 space-y-3">
            {Array.from({ length: 5 }).map((_, i) => (
              <div key={i} className="h-10 bg-[var(--bg-card)] rounded animate-shimmer" />
            ))}
          </div>
        ) : logs.length === 0 ? (
          <p className="text-sm text-[var(--text-muted)] text-center py-12">{t('audit.noLogs', 'No audit logs')}</p>
        ) : (
          <div className="overflow-x-auto">
            <div className="flex items-center gap-3 px-5 py-2 text-[11px] font-semibold text-[var(--text-muted)] uppercase tracking-wider border-b border-[var(--bg-border)] whitespace-nowrap">
              <span className="w-36 shrink-0">{t('audit.time', 'Time')}</span>
              <span className="w-20 shrink-0">{t('audit.action', 'Action')}</span>
              <span className="w-32 shrink-0">{t('audit.resource', 'Resource')}</span>
              <span className="w-36 shrink-0">{t('audit.resourceId', 'Resource ID')}</span>
              <span className="w-28 shrink-0">{t('audit.remoteIp', 'IP')}</span>
              <span className="flex-1 min-w-0">{t('audit.detail', 'Detail')}</span>
            </div>
            <div className="divide-y divide-[var(--bg-border)]">
              {logs.map((l, i) => (
                <div key={l.id || i} className="flex items-center gap-3 px-5 py-2 hover:bg-[var(--bg-hover)]/50 transition-colors text-xs whitespace-nowrap">
                  <span className="w-36 shrink-0 text-[var(--text-muted)] font-mono">{new Date(l.timestamp).toLocaleString()}</span>
                  <span className={`w-20 shrink-0 px-2 py-0.5 rounded-md border text-[10px] font-bold ${actionColors[l.action] || ''}`}>{l.action}</span>
                  <span className="w-32 shrink-0 font-semibold text-[var(--text-primary)] truncate">{l.resource}</span>
                  <span className="w-36 shrink-0 text-[var(--text-secondary)] font-mono truncate">{l.resource_id || '-'}</span>
                  <span className="w-28 shrink-0 text-[var(--text-secondary)] font-mono truncate">{l.remote_ip || '-'}</span>
                  <span className="flex-1 min-w-0 text-[var(--text-muted)] truncate">{l.detail || '-'}</span>
                </div>
              ))}
            </div>
          </div>
        )}
        <div className="px-5 py-3 border-t border-[var(--bg-border)]">
          <Pagination page={page} total={total} size={pageSize} onChange={setPage} />
        </div>
      </Card>
    </div>
  )
}
