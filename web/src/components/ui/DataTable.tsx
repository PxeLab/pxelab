import { type KeyboardEvent as ReactKeyboardEvent, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

export interface Column<T> {
  key: string
  label: ReactNode
  render?: (item: T) => ReactNode
  sortable?: boolean
  width?: string
  className?: string
}

interface Props<T> {
  columns: Column<T>[]
  data: T[]
  loading?: boolean
  onRowClick?: (item: T) => void
  sortField?: string
  sortDir?: 'asc' | 'desc'
  onSort?: (field: string) => void
  emptyText?: string
  rowKey?: (item: T) => string
  /** 表头吸顶：需配合外层滚动容器（如 max-h + overflow-y-auto）使用 */
  stickyHeader?: boolean
}

export function DataTable<T extends Record<string, any>>({
  columns, data, loading, onRowClick, sortField, sortDir, onSort, emptyText, rowKey, stickyHeader,
}: Props<T>) {
  const { t } = useTranslation()
  const displayEmpty = emptyText || t('common.noData')

  // col.className 自带对齐类时不再附加 text-left——构建产物中 text-center 排在
  // text-left 之前，靠类名顺序无法覆盖，只能在 th 上去掉冲突的基类
  const thAlign = (cls?: string) => (cls && /\btext-(center|right|justify|start|end)\b/.test(cls) ? '' : 'text-left')

  const thAriaSort = (key: string): 'ascending' | 'descending' | undefined => {
    if (sortField !== key) return undefined
    return sortDir === 'desc' ? 'descending' : 'ascending'
  }

  // 排序表头用键盘同样可触发：Enter/Space 等价于点击
  const sortKeyDown = (col: Column<T>) => (e: ReactKeyboardEvent) => {
    if (!col.sortable) return
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault()
      onSort?.(col.key)
    }
  }

  if (loading) {
    return (
      <div className="overflow-x-auto">
        <table className="w-full text-sm border-collapse">
          <thead>
            <tr>
              {columns.map(col => (
                <th key={col.key} className={`${thAlign(col.className)} px-4 py-2.5 text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)] border-b border-[var(--bg-border)] whitespace-nowrap ${col.className || ''}`} style={{ width: col.width }}>
                  {col.label}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {Array.from({ length: 5 }).map((_, i) => (
              <tr key={i}>
                {columns.map(col => (
                  <td key={col.key} className="px-4 py-3 border-b border-[var(--bg-border)]">
                    <div className="h-4 bg-[var(--bg-card)] rounded animate-shimmer" />
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    )
  }

  if (data.length === 0) {
    return (
      <div className="overflow-x-auto">
        <table className="w-full text-sm border-collapse">
          <thead>
            <tr>
              {columns.map(col => (
                <th key={col.key} className={`${thAlign(col.className)} px-4 py-2.5 text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)] border-b border-[var(--bg-border)] whitespace-nowrap ${col.className || ''}`} style={{ width: col.width }}>
                  {col.label}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            <tr>
              <td colSpan={columns.length} className="text-center py-12 text-[var(--text-muted)]">
                <p className="text-sm">{displayEmpty}</p>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    )
  }

  const sortIndicator = (key: string) => {
    if (sortField !== key) return ''
    return sortDir === 'asc' ? ' ↑' : ' ↓'
  }

  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm border-collapse">
        <thead>
          <tr>
            {columns.map(col => (
              <th
                key={col.key}
                aria-sort={thAriaSort(col.key)}
                tabIndex={col.sortable ? 0 : undefined}
                onKeyDown={sortKeyDown(col)}
                className={`${thAlign(col.className)} px-4 py-2.5 text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)] border-b border-[var(--bg-border)] whitespace-nowrap ${col.sortable ? 'cursor-pointer hover:text-[var(--text-secondary)] select-none focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-[-2px] focus-visible:outline-blue-500' : ''} ${stickyHeader ? 'sticky top-0 bg-[var(--card)] z-10' : ''} ${col.className || ''}`}
                style={{ width: col.width }}
                onClick={() => col.sortable && onSort?.(col.key)}
              >
                {col.label}{sortIndicator(col.key)}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {data.map((item, i) => (
            <tr
              key={rowKey?.(item) ?? i}
              tabIndex={onRowClick ? 0 : undefined}
              onKeyDown={onRowClick ? (e) => {
                if (e.key === 'Enter' || e.key === ' ') {
                  e.preventDefault()
                  onRowClick(item)
                }
              } : undefined}
              className={`group ${onRowClick ? 'cursor-pointer focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-[-2px] focus-visible:outline-blue-500' : ''} hover:bg-[var(--bg-hover)]/60 hover:shadow-[inset_3px_0_0_var(--bg-border)] transition-all duration-150`}
              onClick={() => onRowClick?.(item)}
            >
              {columns.map(col => (
                <td key={col.key} className={`px-4 py-3 border-b border-[var(--bg-border)] text-[var(--text-secondary)] ${col.className || ''}`}>
                  {col.render ? col.render(item) : String(item[col.key] ?? '—')}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
