import { useEffect } from 'react'
import './Toast.css'

interface ToastProps {
  message: string
  durationMs?: number
  onDismiss: () => void
}

/** Minimal fixed-position toast that auto-dismisses after durationMs. */
export function Toast({ message, durationMs = 1500, onDismiss }: ToastProps) {
  useEffect(() => {
    const timer = setTimeout(onDismiss, durationMs)
    return () => clearTimeout(timer)
  }, [durationMs, onDismiss])

  return <div className="toast">{message}</div>
}
