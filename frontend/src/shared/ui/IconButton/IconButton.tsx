import type { ButtonHTMLAttributes } from 'react'
import { cn } from '../../lib/cn'

interface IconButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  active?: boolean
  size?: number
}

export function IconButton({ active, size = 32, className, style, ...rest }: IconButtonProps) {
  return (
    <button
      className={cn(
        'icon-button flex-none flex items-center justify-center rounded-md text-fg2 cursor-pointer hover:bg-surface2 hover:text-fg disabled:opacity-50 disabled:cursor-not-allowed',
        active && 'bg-surface2 text-fg',
        className,
      )}
      style={{ width: size, height: size, ...style }}
      {...rest}
    />
  )
}
