import { type FC } from 'react'

interface LoadingSpinnerProps {
  text?: string
  className?: string
}

export const LoadingSpinner: FC<LoadingSpinnerProps> = ({ text, className = '' }) => {
  return (
    <div className={`flex items-center justify-center py-12 ${className}`}>
      <div className="animate-spin w-6 h-6 border-2 border-blue-500 border-t-transparent rounded-full" />
      {text && <span className="ml-3 text-sm text-[var(--text-muted)]">{text}</span>}
    </div>
  )
}

interface ErrorBannerProps {
  message: string
  onDismiss?: () => void
  className?: string
}

export const ErrorBanner: FC<ErrorBannerProps> = ({ message, onDismiss, className = '' }) => (
  <div className={`px-4 py-3 rounded-lg bg-red-500/10 border border-red-500/20 text-sm text-red-400 ${className}`}>
    {message}
    {onDismiss && (
      <button onClick={onDismiss} className="float-right text-red-400/60 hover:text-red-400 transition-colors">✕</button>
    )}
  </div>
)

interface SkeletonProps {
  rows?: number
  className?: string
}

export const Skeleton: FC<SkeletonProps> = ({ rows = 5, className = '' }) => (
  <div className={`space-y-3 ${className}`}>
    {Array.from({ length: rows }).map((_, i) => (
      <div key={i} className="h-10 rounded-lg bg-[var(--bg-hover)] animate-pulse" />
    ))}
  </div>
)
