import { cn } from '../../lib/cn'

interface ProgressBarProps {
  /** 0-100 */
  value: number
  direction?: 'up' | 'down'
  className?: string
}

export function ProgressBar({ value, direction = 'up', className }: ProgressBarProps) {
  const clamped = Math.max(0, Math.min(100, value))
  return (
    <div className={cn('progress-bar h-1.5 rounded-full bg-track overflow-hidden', className)}>
      <div
        className={cn('h-full rounded-full', direction === 'up' ? 'bg-accent' : 'bg-blue')}
        style={{ width: `${clamped}%` }}
      />
    </div>
  )
}
