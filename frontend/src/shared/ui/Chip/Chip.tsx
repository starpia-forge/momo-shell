import type { ReactNode } from 'react'
import { cn } from '../../lib/cn'

interface ChipProps {
  children: ReactNode
  tone?: 'neutral' | 'green' | 'red'
  className?: string
}

const TONE: Record<NonNullable<ChipProps['tone']>, string> = {
  neutral: 'bg-surface2 text-fg2',
  green: 'bg-green/14 text-green',
  red: 'bg-red/14 text-red',
}

export function Chip({ children, tone = 'neutral', className }: ChipProps) {
  return (
    <span className={cn('chip inline-flex items-center rounded-full px-2.5 py-0.75 text-[11px] font-medium', TONE[tone], className)}>
      {children}
    </span>
  )
}
