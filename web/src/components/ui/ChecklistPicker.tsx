import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Search } from 'lucide-react'

// ChecklistPicker 带搜索过滤的勾选列表：已选项置顶，数据多时可按名称过滤。
export function ChecklistPicker<T extends string | number>(props: {
  items: { id: T; name: string; badge?: string }[]
  selected: T[]
  disabledIds?: T[]
  onToggle: (id: T) => void
  emptyText: string
}) {
  const { t } = useTranslation()
  const [filter, setFilter] = useState('')
  const disabled = new Set(props.disabledIds ?? [])
  const q = filter.trim().toLowerCase()
  const visible = (q ? props.items.filter(i => i.name.toLowerCase().includes(q)) : props.items)
    .slice()
    .sort((a, b) => {
      const aOn = props.selected.includes(a.id) || disabled.has(a.id) ? 0 : 1
      const bOn = props.selected.includes(b.id) || disabled.has(b.id) ? 0 : 1
      return aOn - bOn
    })
  return (
    <div className="rounded border border-[var(--bg-border)]">
      <div className="flex items-center gap-2 px-2 py-1.5 border-b border-[var(--bg-border)]">
        <Search size={12} className="text-[var(--text-muted)] shrink-0" />
        <input
          className="flex-1 min-w-0 bg-transparent text-xs text-[var(--text-primary)] outline-none placeholder:text-[var(--text-muted)]"
          placeholder={t('common.search')}
          value={filter}
          onChange={e => setFilter(e.target.value)}
        />
        {props.selected.length > 0 && (
          <span className="shrink-0 px-1.5 py-px rounded bg-blue-500/10 text-blue-400 text-[10px] font-medium">
            {t('common.selectedCount', { count: props.selected.length })}
          </span>
        )}
      </div>
      <div className="max-h-36 overflow-y-auto p-2 space-y-1">
        {visible.map(item => {
          const isDisabled = disabled.has(item.id)
          const on = isDisabled || props.selected.includes(item.id)
          return (
            <label key={String(item.id)} className={`flex items-center gap-2 text-xs ${isDisabled ? 'opacity-60' : 'cursor-pointer hover:text-[var(--text-primary)]'}`}>
              <input
                type="checkbox"
                checked={on}
                disabled={isDisabled}
                onChange={() => props.onToggle(item.id)}
                className="rounded border-[var(--bg-border)] text-blue-500 focus:ring-blue-500/30"
              />
              <span className="text-[var(--text-muted)] truncate">{item.name}</span>
              {item.badge && (
                <span className="ml-auto px-1 py-px rounded text-[9px] uppercase text-[var(--text-muted)] bg-[var(--bg-hover)]">{item.badge}</span>
              )}
            </label>
          )
        })}
        {visible.length === 0 && (
          <p className="text-[11px] text-[var(--text-muted)]">{props.items.length === 0 ? props.emptyText : t('common.noMatch')}</p>
        )}
      </div>
    </div>
  )
}
