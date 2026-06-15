import { type FC, type ReactNode } from 'react'

interface Props {
  title?: string
  children: ReactNode
  className?: string
  hover?: boolean
  footer?: ReactNode
  padding?: boolean
}

export const Card: FC<Props> = ({ title, children, className = '', hover = false, footer, padding = true }) => (
  <div className={`bg-[#16181f] border border-[#232738] rounded-xl overflow-hidden transition-all duration-200 ${hover ? 'hover:border-[#2e3245] hover:shadow-[0_1px_3px_rgba(0,0,0,0.3)]' : ''} ${className}`}>
    {title && (
      <div className="flex items-center justify-between px-5 py-4 border-b border-[#232738]">
        <span className="text-sm font-bold text-[#e8eaed] tracking-tight">{title}</span>
      </div>
    )}
    <div className={padding ? 'p-5' : ''}>{children}</div>
    {footer && <div className="px-5 py-3 border-t border-[#232738] text-xs text-[#6b7294]">{footer}</div>}
  </div>
)
