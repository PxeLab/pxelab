import { type FC } from 'react'

interface Props {
  icon?: string
  title: string
  description?: string
  action?: { label: string; onClick: () => void }
}

export const EmptyState: FC<Props> = ({ icon = '📦', title, description, action }) => (
  <div className="text-center py-12 text-[#6b7294]">
    <div className="text-4xl mb-3 opacity-50">{icon}</div>
    <h3 className="text-base text-[#9aa0ab] mb-1.5">{title}</h3>
    {description && <p className="text-sm mb-4">{description}</p>}
    {action && (
      <button
        onClick={action.onClick}
        className="inline-flex items-center gap-1.5 px-4 py-2 rounded-lg text-sm font-semibold bg-blue-500 text-white hover:bg-blue-600 transition-colors"
      >
        {action.label}
      </button>
    )}
  </div>
)
