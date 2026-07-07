import type { InputHTMLAttributes } from 'react'
import { cn } from '../../lib/cn'

interface SearchInputProps extends InputHTMLAttributes<HTMLInputElement> {
  containerClassName?: string
}

export function SearchInput({ containerClassName, className, ...rest }: SearchInputProps) {
  return (
    <div
      className={cn(
        'search-input flex items-center gap-2 h-8.5 px-3.5 rounded-md bg-inputbg border border-line',
        containerClassName,
      )}
    >
      <svg width="14" height="14" viewBox="0 0 14 14" fill="none" className="flex-none">
        <circle cx="6" cy="6" r="4.5" stroke="currentColor" strokeWidth="1.5" className="text-fg3" />
        <line x1="9.5" y1="9.5" x2="13" y2="13" stroke="currentColor" strokeWidth="1.5" className="text-fg3" />
      </svg>
      <input
        className={cn(
          'flex-1 min-w-0 bg-transparent text-fg text-[13px] placeholder:text-fg3 focus:outline-none',
          className,
        )}
        {...rest}
      />
    </div>
  )
}
