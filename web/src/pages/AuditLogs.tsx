import { useState, useEffect, useCallback } from 'react'
import { useTranslation } from 'react-i18next'
import { Search, Filter } from 'lucide-react'
import { Card } from '../components/ui/Card'
import { Button } from '../components/ui/Button'
import { Pagination } from '../components/ui/Pagination'
import { useToast } from '../components/ui/Toast'
import { useUIConfig } from '../contexts/UIConfigContext'
import { api, type AuditLog } from '../api/client'

const actionLabels: Record<string, string> = {
  CREATE: '新建',
  UPDATE: '更新',
  DELETE: '删除',
}
const actionColors: Record<string, string> = {
  CREATE: 'bg-green-500/10 text-green-400 border-green-500/30',
  UPDATE: 'bg-blue-500/10 text-blue-400 border-blue-500/30',
  DELETE: 'bg-red-500/10 text-red-400 border-red-500/30',
}
const resourceLabels: Record<string, string> = {
  host: '主机',
  profile: '引导配置',
  settings: '系统设置',
  dns_record: 'DNS 记录',
  bmc_config: 'BMC 配置',
  dhcp_reservation: 'DHCP 预留',
  answer_template: '应答模板',
  install_task: '安装任务',
  netboot_overlay: '网络引导覆盖',
  blacklist: '黑名单',
  whitelist: '白名单',
  unauthorized_device: '未授权设备',
  os_image: '系统镜像',
  lease: 'DHCP 租约',
  wol_schedule: 'WOL 调度',
  wol_history: 'WOL 记录',
  wol: '网络唤醒',
  file: '文件',
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
  const [searchText, setSearchText] = useState('')

  const loadLogs = useCallback(async () => {
    setLoading(true)
    try {
      const params: Record<string, string> = { page: String(page), size: String(pageSize) }
      if (actionFilter) params.action = actionFilter
      if (resourceFilter) params.resource = resourceFilter
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
  }, [page, pageSize, actionFilter, resourceFilter, searchText])

  useEffect(() => { loadLogs() }, [loadLogs])

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-lg font-bold text-[var(--text-primary)]">{t('audit.title', '审计日志')}</h1>
      </div>

      {/* Filters */}
      <div className="flex items-center gap-3 mb-5 flex-wrap">
        <div className="relative">
          <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-[var(--text-muted)]" />
          <input
            value={searchText}
            onChange={e => { setSearchText(e.target.value); setPage(1) }}
            placeholder={t('audit.searchPlaceholder', '搜索...')}
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
            {a ? actionLabels[a] || a : t('audit.allActions', '全部')}
          </button>
        ))}

        <select
          value={resourceFilter}
          onChange={e => { setResourceFilter(e.target.value); setPage(1) }}
          className="px-3 py-1.5 text-xs bg-[var(--bg-card)] border border-[var(--bg-border)] rounded-lg text-[var(--text-primary)] outline-none focus:border-blue-500"
        >
          <option value="">{t('audit.allResources', '所有资源')}</option>
          {Object.entries(resourceLabels).map(([key, label]) => (
            <option key={key} value={key}>{label}</option>
          ))}
        </select>

        {(actionFilter || resourceFilter || searchText) && (
          <Button variant="secondary" size="sm" onClick={() => { setActionFilter(''); setResourceFilter(''); setSearchText(''); setPage(1) }}>
            <Filter size={12} />
            {t('audit.clearFilters', '清除筛选')}
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
          <p className="text-sm text-[var(--text-muted)] text-center py-12">{t('audit.noLogs', '暂无审计日志')}</p>
        ) : (
          <div className="overflow-x-auto">
            <div className="flex items-center gap-3 px-5 py-2 text-[11px] font-semibold text-[var(--text-muted)] uppercase tracking-wider border-b border-[var(--bg-border)] whitespace-nowrap">
              <span className="w-40 shrink-0">{t('audit.time', '时间')}</span>
              <span className="w-16 shrink-0">{t('audit.action', '操作')}</span>
              <span className="w-24 shrink-0">{t('audit.resource', '资源')}</span>
              <span className="w-40 shrink-0">{t('audit.resourceId', '目标')}</span>
              <span className="flex-1 min-w-0">{t('audit.detail', '变更内容')}</span>
              <span className="w-20 shrink-0">{t('audit.remoteIp', '来源')}</span>
            </div>
            <div className="divide-y divide-[var(--bg-border)]">
              {logs.map((l, i) => (
                <div key={l.id || i} className="flex items-center gap-3 px-5 py-2 hover:bg-[var(--bg-hover)]/50 transition-colors text-xs whitespace-nowrap">
                  <span className="w-40 shrink-0 text-[var(--text-muted)] font-mono text-[11px]">{new Date(l.timestamp).toLocaleString()}</span>
                  <span className={`w-16 shrink-0 px-2 py-0.5 rounded-md border text-[10px] font-bold ${actionColors[l.action] || ''}`}>
                    {actionLabels[l.action] || l.action}
                  </span>
                  <span className="w-24 shrink-0 text-[var(--text-secondary)]">
                    {resourceLabels[l.resource] || l.resource}
                  </span>
                  <span className="w-40 shrink-0 font-medium text-[var(--text-primary)] truncate" title={l.resource_id}>
                    {l.resource_id || '-'}
                  </span>
                  <span className="flex-1 min-w-0 text-[var(--text-muted)] truncate" title={l.detail}>
                    {l.detail || '-'}
                  </span>
                  <span className="w-20 shrink-0 text-[var(--text-secondary)]">{l.remote_ip || '-'}</span>
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
