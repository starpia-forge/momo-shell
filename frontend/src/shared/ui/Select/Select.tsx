import { cn } from '../../lib/cn'

export interface SelectOption<T extends string> {
  value: T
  label: string
}

interface SelectProps<T extends string> {
  value: T
  options: SelectOption<T>[]
  onChange: (value: T) => void
  className?: string
  id?: string
  'aria-label'?: string
}

export function Select<T extends string>({ value, options, onChange, className, id, ...rest }: SelectProps<T>) {
  return (
    <div className="select relative inline-block">
      <select
        id={id}
        value={value}
        onChange={(e) => onChange(e.target.value as T)}
        className={cn(
          'appearance-none rounded-md border border-line bg-inputbg text-fg text-[13px] px-3.5 py-2.5 pr-9 cursor-pointer focus:outline-none focus:border-accent w-full',
          className,
        )}
        {...rest}
      >
        {options.map((opt) => (
          <option key={opt.value} value={opt.value}>
            {opt.label}
          </option>
        ))}
      </select>
      <svg
        className="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-fg3"
        width="16"
        height="16"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="2"
      >
        <path d="M6 9l6 6 6-6" strokeLinecap="round" strokeLinejoin="round" />
      </svg>
    </div>
  )
}
