import type { ButtonHTMLAttributes } from 'react'
import { cn } from '../../lib/cn'

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: 'primary' | 'default' | 'danger' | 'outline-accent'
  size?: 'md' | 'sm'
}

const VARIANT: Record<NonNullable<ButtonProps['variant']>, string> = {
  default: 'bg-transparent border border-line text-fg2 hover:text-fg',
  primary: 'bg-accent border border-accent text-on-accent font-bold hover:brightness-105',
  danger: 'bg-transparent border border-line text-red hover:border-red',
  'outline-accent': 'bg-transparent border-[1.5px] border-accent text-accent-text font-bold',
}

const SIZE: Record<NonNullable<ButtonProps['size']>, string> = {
  md: 'px-4.5 py-2 text-[13px]',
  sm: 'px-3.25 py-1.25 text-[12px]',
}

export function Button({ variant = 'default', size = 'md', className, ...rest }: ButtonProps) {
  return (
    <button
      className={cn(
        'btn rounded-md cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed',
        SIZE[size],
        VARIANT[variant],
        className,
      )}
      {...rest}
    />
  )
}
