import type { InputHTMLAttributes } from 'react'
import './TextInput.css'

interface TextInputProps extends InputHTMLAttributes<HTMLInputElement> {
  label?: string
  error?: string
}

export function TextInput({ label, error, id, className, ...rest }: TextInputProps) {
  return (
    <div className="text-input">
      {label && (
        <label className="text-input__label" htmlFor={id}>
          {label}
        </label>
      )}
      <input id={id} className={['text-input__field', error && 'text-input__field--error', className].filter(Boolean).join(' ')} {...rest} />
      {error && <span className="text-input__error">{error}</span>}
    </div>
  )
}
