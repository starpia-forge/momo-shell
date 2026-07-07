import { cn } from '../../lib/cn'

export interface SegmentedOption<T extends string> {
  value: T
  label: string
}

interface SegmentedControlProps<T extends string> {
  value: T
  options: SegmentedOption<T>[]
  onChange: (value: T) => void
  className?: string
}

export function SegmentedControl<T extends string>({ value, options, onChange, className }: SegmentedControlProps<T>) {
  return (
    <div className={cn('segmented-control flex items-center gap-0.5 p-0.75 rounded-md bg-inputbg border border-line', className)}>
      {options.map((opt) => (
        <button
          key={opt.value}
          type="button"
          onClick={() => onChange(opt.value)}
          className={cn(
            'flex-1 rounded-sm px-3 py-1.5 text-[12.5px] cursor-pointer whitespace-nowrap',
            opt.value === value ? 'bg-surface2 text-fg font-medium' : 'text-fg2',
          )}
        >
          {opt.label}
        </button>
      ))}
    </div>
  )
}
