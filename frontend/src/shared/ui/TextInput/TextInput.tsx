import type { InputHTMLAttributes } from 'react'
import { cn } from '../../lib/cn'

interface TextInputProps extends InputHTMLAttributes<HTMLInputElement> {
  label?: string
  error?: string
}

export function TextInput({ label, error, id, className, ...rest }: TextInputProps) {
  return (
    <div className="text-input flex flex-col gap-1">
      {label && (
        <label className="text-[12.5px] font-medium text-fg2" htmlFor={id}>
          {label}
        </label>
      )}
      <input
        id={id}
        className={cn(
          'rounded-md border px-3.5 py-2.5 bg-inputbg text-fg text-[13px] focus:outline-none focus:border-accent',
          error ? 'border-red' : 'border-line',
          className,
        )}
        {...rest}
      />
      {error && <span className="text-[12px] text-red">{error}</span>}
    </div>
  )
}
