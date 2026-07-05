import { useEffect } from 'react'
import { useToastStore } from './toastStore'
import './ToastHost.css'

const AUTO_DISMISS_MS = 4000

function ToastHostItem({ id, message, onClick }: { id: string; message: string; onClick?: () => void }) {
  useEffect(() => {
    const timer = setTimeout(() => useToastStore.getState().remove(id), AUTO_DISMISS_MS)
    return () => clearTimeout(timer)
  }, [id])

  return (
    <div
      className="toast-host__item"
      role={onClick ? 'button' : undefined}
      onClick={() => {
        onClick?.()
        useToastStore.getState().remove(id)
      }}
    >
      {message}
    </div>
  )
}

/** App-wide stacked notification queue, separate from the single fixed-
 * position <Toast> used inline by panels (FileBrowserPanel, HistoryPanel). */
export function ToastHost() {
  const toasts = useToastStore((s) => s.toasts)
  if (toasts.length === 0) return null

  return (
    <div className="toast-host">
      {toasts.map((t) => (
        <ToastHostItem key={t.id} id={t.id} message={t.message} onClick={t.onClick} />
      ))}
    </div>
  )
}
