import { type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

export interface Column<T> {
  key: string
  label: string
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
}

export function DataTable<T extends Record<string, any>>({
  columns, data, loading, onRowClick, sortField, sortDir, onSort, emptyText, rowKey,
}: Props<T>) {
  const { t } = useTranslation()
  const displayEmpty = emptyText || t('common.noData')

  if (loading) {
    return (
      <div className="overflow-x-auto">
        <table className="w-full text-sm border-collapse">
          <thead>
            <tr>
              {columns.map(col => (
                <th key={col.key} className="text-left px-4 py-2.5 text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)] border-b border-[var(--bg-border)] whitespace-nowrap" style={{ width: col.width }}>
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
                <th key={col.key} className="text-left px-4 py-2.5 text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)] border-b border-[var(--bg-border)] whitespace-nowrap" style={{ width: col.width }}>
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
                className={`text-left px-4 py-2.5 text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)] border-b border-[var(--bg-border)] whitespace-nowrap ${col.sortable ? 'cursor-pointer hover:text-[var(--text-secondary)] select-none' : ''}`}
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
              className={`${onRowClick ? 'cursor-pointer' : ''} hover:bg-[var(--bg-hover)]/60 hover:shadow-[inset_3px_0_0_var(--bg-border)] transition-all duration-150`}
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
