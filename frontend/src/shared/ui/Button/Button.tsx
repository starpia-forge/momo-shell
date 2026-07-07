import type { ButtonHTMLAttributes } from 'react'
import { cn } from '../../lib/cn'

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: 'primary' | 'default' | 'danger'
}

const VARIANT: Record<NonNullable<ButtonProps['variant']>, string> = {
  default: 'bg-surface border-line text-fg',
  primary: 'bg-accent border-accent text-on-accent',
  danger: 'bg-red border-red text-on-accent',
}

export function Button({ variant = 'default', className, ...rest }: ButtonProps) {
  return (
    <button
      className={cn(
        'btn rounded px-3.5 py-1.5 border text-[13px] cursor-pointer hover:border-accent disabled:opacity-50 disabled:cursor-not-allowed',
        VARIANT[variant],
        className,
      )}
      {...rest}
    />
  )
}
