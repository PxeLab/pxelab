import { type FC } from 'react'
import { Button } from './Button'

interface Props {
  icon?: string
  title: string
  description?: string
  action?: { label: string; onClick: () => void }
}

export const EmptyState: FC<Props> = ({ icon = '📦', title, description, action }) => (
  <div className="text-center py-12 text-[var(--text-muted)]">
    <div className="text-4xl mb-3 opacity-50">{icon}</div>
    <h3 className="text-base text-[var(--text-secondary)] mb-1.5">{title}</h3>
    {description && <p className="text-sm mb-4">{description}</p>}
    {action && (
      <Button variant="primary" size="md" onClick={action.onClick}>
        {action.label}
      </Button>
    )}
  </div>
)
