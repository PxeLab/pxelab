import { type FC } from 'react'
import { useTranslation } from 'react-i18next'
import { ChevronLeft, ChevronRight } from 'lucide-react'

interface PaginationProps {
  page: number
  total: number
  size: number
  onChange: (page: number) => void
}

export const Pagination: FC<PaginationProps> = ({ page, total, size, onChange }) => {
  const { t } = useTranslation()
  const totalPages = Math.ceil(total / size)
  if (totalPages <= 1) return null

  const pages: number[] = []
  const start = Math.max(1, page - 2)
  const end = Math.min(totalPages, page + 2)
  for (let i = start; i <= end; i++) pages.push(i)

  const from = (page - 1) * size + 1
  const to = Math.min(page * size, total)

  return (
    <div className="flex items-center justify-between pt-4 text-xs text-[var(--text-muted)]">
      <span>
        {t('pagination.showing', { from, to, total })}
      </span>
      <div className="flex gap-1">
        <button
          onClick={() => onChange(page - 1)}
          disabled={page <= 1}
          className="w-8 h-8 flex items-center justify-center rounded-lg border border-[var(--bg-border)] bg-transparent text-[var(--text-secondary)] disabled:opacity-40 hover:bg-[var(--bg-hover)] hover:border-[var(--text-muted)] transition-all"
        >
          <ChevronLeft size={14} />
        </button>
        {pages.map(p => (
          <button
            key={p}
            onClick={() => onChange(p)}
            className={`w-8 h-8 flex items-center justify-center rounded-lg border text-xs font-mono transition-all duration-200 ${
              p === page
                ? 'bg-blue-500 border-blue-500 text-white shadow-[var(--glow-blue)]'
                : 'border-[var(--bg-border)] bg-transparent text-[var(--text-secondary)] hover:bg-[var(--bg-hover)] hover:border-[var(--text-muted)]'
            }`}
          >
            {p}
          </button>
        ))}
        <button
          onClick={() => onChange(page + 1)}
          disabled={page >= totalPages}
          className="w-8 h-8 flex items-center justify-center rounded-lg border border-[var(--bg-border)] bg-transparent text-[var(--text-secondary)] disabled:opacity-40 hover:bg-[var(--bg-hover)] hover:border-[var(--text-muted)] transition-all"
        >
          <ChevronRight size={14} />
        </button>
      </div>
    </div>
  )
}
