import { type ReactNode } from 'react'

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
  columns, data, loading, onRowClick, sortField, sortDir, onSort, emptyText = '暂无数据', rowKey,
}: Props<T>) {
  if (loading) {
    return (
      <div className="overflow-x-auto">
        <table className="w-full text-sm border-collapse">
          <thead>
            <tr>
              {columns.map(col => (
                <th key={col.key} className="text-left px-4 py-2.5 text-xs font-semibold uppercase tracking-wide text-[#6b7294] border-b border-[#232738] whitespace-nowrap" style={{ width: col.width }}>
                  {col.label}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {Array.from({ length: 5 }).map((_, i) => (
              <tr key={i}>
                {columns.map(col => (
                  <td key={col.key} className="px-4 py-3 border-b border-[#232738]">
                    <div className="h-4 bg-[#16181f] rounded animate-[shimmer_1.5s_infinite] bg-gradient-to-r from-[#16181f] via-[#1c1f2c] to-[#16181f] bg-[length:200%_100%]" />
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
      <div className="text-center py-12 text-[#6b7294]">
        <p className="text-sm">{emptyText}</p>
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
                className={`text-left px-4 py-2.5 text-xs font-semibold uppercase tracking-wide text-[#6b7294] border-b border-[#232738] whitespace-nowrap ${col.sortable ? 'cursor-pointer hover:text-[#9aa0ab] select-none' : ''}`}
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
              className={`${onRowClick ? 'cursor-pointer' : ''} hover:bg-white/[0.02]`}
              onClick={() => onRowClick?.(item)}
            >
              {columns.map(col => (
                <td key={col.key} className={`px-4 py-3 border-b border-[#232738] text-[#9aa0ab] ${col.className || ''}`}>
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
