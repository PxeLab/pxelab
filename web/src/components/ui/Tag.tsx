import { type FC, type ReactNode } from 'react'

export type TagColor = 'blue' | 'green' | 'yellow' | 'red' | 'purple' | 'cyan' | 'orange'
type TagVariant = 'filled' | 'subtle'

const colorClasses: Record<TagVariant, Record<TagColor, string>> = {
  filled: {
    blue: 'bg-blue-500 text-white',
    green: 'bg-accent-green text-white',
    yellow: 'bg-accent-yellow text-black',
    red: 'bg-accent-red text-white',
    purple: 'bg-purple-500 text-white',
    cyan: 'bg-cyan-500 text-white',
    orange: 'bg-orange-500 text-white',
  },
  subtle: {
    blue: 'bg-blue-500/10 text-blue-400',
    green: 'bg-accent-green/10 text-accent-green',
    yellow: 'bg-accent-yellow/10 text-accent-yellow',
    red: 'bg-accent-red/10 text-accent-red',
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
  <span className={`inline-flex items-center gap-1 px-2 py-0.5 rounded-md text-xs font-mono font-semibold ${colorClasses[variant][color]} ${className}`}>
    {children}
  </span>
)
