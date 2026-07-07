import { cn } from '../../lib/cn'

export type DotStatus = 'running' | 'connecting' | 'error' | 'idle'

const COLOR: Record<DotStatus, string> = {
  running: 'bg-green',
  connecting: 'bg-amber animate-pulse-dot',
  error: 'bg-red',
  idle: 'bg-fg3',
}

interface StatusDotProps {
  status: DotStatus
  size?: number
  className?: string
}

export function StatusDot({ status, size = 7, className }: StatusDotProps) {
  return (
    <span
      className={cn('status-dot inline-block flex-none rounded-full', COLOR[status], className)}
      style={{ width: size, height: size }}
    />
  )
}
