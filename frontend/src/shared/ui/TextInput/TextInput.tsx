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
        <label className="text-[12px] text-muted" htmlFor={id}>
          {label}
        </label>
      )}
      <input
        id={id}
        className={cn(
          'rounded border px-2 py-1.5 bg-canvas text-fg text-[13px] focus:outline-none focus:border-accent',
          error ? 'border-danger' : 'border-line',
          className,
        )}
        {...rest}
      />
      {error && <span className="text-[12px] text-danger">{error}</span>}
    </div>
  )
}
