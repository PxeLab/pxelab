import { type FC, type ReactNode } from 'react'

export type TagColor = 'blue' | 'green' | 'yellow' | 'red' | 'purple' | 'cyan' | 'orange'
type TagVariant = 'filled' | 'subtle'

const colorClasses: Record<TagVariant, Record<TagColor, string>> = {
  filled: {
    blue: 'bg-blue-500 text-white',
    green: 'bg-green-500 text-white',
    yellow: 'bg-yellow-500 text-black',
    red: 'bg-red-500 text-white',
    purple: 'bg-purple-500 text-white',
    cyan: 'bg-cyan-500 text-white',
    orange: 'bg-orange-500 text-white',
  },
  subtle: {
    blue: 'bg-blue-500/10 text-blue-400',
    green: 'bg-green-500/10 text-green-400',
    yellow: 'bg-yellow-500/10 text-yellow-400',
    red: 'bg-red-500/10 text-red-400',
    purple: 'bg-purple-500/10 text-purple-400',
    cyan: 'bg-cyan-500/10 text-cyan-400',
    orange: 'bg-orange-500/10 text-orange-400',
  },
}

interface TagProps {
  color?: TagColor
  variant?: TagVariant
  children: ReactNode
  className?: string
}

export const Tag: FC<TagProps> = ({ color = 'blue', variant = 'subtle', children, className = '' }) => (
  <span className={`inline-flex items-center gap-1 px-2 py-0.5 rounded text-xs font-mono font-semibold ${colorClasses[variant][color]} ${className}`}>
    {children}
  </span>
)
