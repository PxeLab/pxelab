import { type FC } from 'react'

type StatusColor = 'green' | 'yellow' | 'red' | 'blue' | 'purple' | 'orange' | 'cyan'

// 发光阴影：CSS 里只有 --glow-blue/--glow-green 两个 glow 变量，且语义是
// "大范围柔光"（带 blur/spread），与状态点的小光环效果不同；accent 色无对应
// glow 变量，故此处保留硬编码 rgba。
const colorMap: Record<StatusColor, string> = {
  green: 'bg-accent-green shadow-[0_0_8px_rgba(34,197,94,0.5)]',
  yellow: 'bg-accent-yellow shadow-[0_0_8px_rgba(234,179,8,0.5)]',
  red: 'bg-accent-red shadow-[0_0_8px_rgba(239,68,68,0.5)]',
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
