import { type FC } from 'react'

type StatusColor = 'green' | 'yellow' | 'red' | 'blue' | 'purple' | 'orange' | 'cyan'

const colorMap: Record<StatusColor, string> = {
  green: 'bg-green-500 shadow-[0_0_8px_rgba(34,197,94,0.5)]',
  yellow: 'bg-yellow-500 shadow-[0_0_8px_rgba(234,179,8,0.5)]',
  red: 'bg-red-500 shadow-[0_0_8px_rgba(239,68,68,0.5)]',
  blue: 'bg-blue-500 shadow-[0_0_8px_rgba(59,130,246,0.5)]',
  purple: 'bg-purple-500 shadow-[0_0_8px_rgba(168,85,247,0.5)]',
  orange: 'bg-orange-500 shadow-[0_0_8px_rgba(249,115,22,0.5)]',
  cyan: 'bg-cyan-500 shadow-[0_0_8px_rgba(6,182,212,0.5)]',
}

interface Props {
  color: StatusColor
  pulse?: boolean
  className?: string
}

export const StatusDot: FC<Props> = ({ color, pulse, className = '' }) => (
  <span
    className={`inline-block w-2 h-2 rounded-full flex-shrink-0 ${colorMap[color]} ${pulse ? 'animate-pulse' : ''} ${className}`}
  />
)
