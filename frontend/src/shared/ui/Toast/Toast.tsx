import { useEffect } from 'react'

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

  return (
    <div className="toast fixed right-4 bottom-4 z-500 rounded-lg px-4.5 py-3.5 bg-surface2 border border-line text-fg text-[13px] shadow-menu">
      {message}
    </div>
  )
}
