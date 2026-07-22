import { type FC, type ReactNode } from 'react'

interface Props {
  title: string
  description?: string
  actions?: ReactNode
  /**
   * 外层容器类名，替换默认的 `mb-6`。
   * 父容器已有间距（如 `space-y-6`）时传 `className="mb-0"`。
   */
  className?: string
  /** 标题字号：md = text-lg（默认），lg = text-2xl */
  size?: 'md' | 'lg'
}

export const PageHeader: FC<Props> = ({ title, description, actions, className, size = 'md' }) => (
  <div className={`flex items-center justify-between ${className ?? 'mb-6'}`}>
    <div>
      <h1 className={`${size === 'lg' ? 'text-2xl tracking-tight' : 'text-lg'} font-bold text-foreground`}>
        {title}
      </h1>
      {description && <p className="text-sm text-foreground-muted mt-1">{description}</p>}
    </div>
    {actions && <div className="flex items-center gap-2">{actions}</div>}
  </div>
)
