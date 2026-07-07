import { useRef, type ChangeEvent, type KeyboardEvent } from 'react'
import { cn } from '../../lib/cn'

interface PinInputProps {
  value: string
  onChange: (value: string) => void
  length?: number
  autoFocus?: boolean
  error?: string
}

/** Six-box numeric PIN entry (pairing PIN) -- each box holds one digit,
 * with auto-advance on entry and auto-back on backspace. */
export function PinInput({ value, onChange, length = 6, autoFocus, error }: PinInputProps) {
  const refs = useRef<(HTMLInputElement | null)[]>([])

  function setDigit(index: number, digit: string) {
    const chars = value.split('')
    chars[index] = digit
    onChange(
      chars
        .join('')
        .slice(0, length)
        .replace(/\s/g, ''),
    )
  }

  function handleChange(index: number, e: ChangeEvent<HTMLInputElement>) {
    const raw = e.target.value.replace(/\D/g, '')
    if (!raw) {
      setDigit(index, '')
      return
    }
    setDigit(index, raw.slice(-1))
    if (index < length - 1) refs.current[index + 1]?.focus()
  }

  function handleKeyDown(index: number, e: KeyboardEvent<HTMLInputElement>) {
    if (e.key === 'Backspace' && !value[index] && index > 0) {
      refs.current[index - 1]?.focus()
    }
  }

  return (
    <div className="flex flex-col gap-2">
      <div className="flex justify-center gap-2">
        {Array.from({ length }).map((_, i) => (
          <input
            key={i}
            ref={(el) => {
              refs.current[i] = el
            }}
            className={cn(
              'w-10.5 h-13 rounded-md border bg-inputbg text-center font-mono text-[22px] font-bold text-fg focus:outline-none focus:border-accent',
              error ? 'border-red' : 'border-line',
            )}
            value={value[i] ?? ''}
            onChange={(e) => handleChange(i, e)}
            onKeyDown={(e) => handleKeyDown(i, e)}
            inputMode="numeric"
            maxLength={1}
            autoFocus={autoFocus && i === 0}
          />
        ))}
      </div>
      {error && <span className="text-[12px] text-red text-center">{error}</span>}
    </div>
  )
}
