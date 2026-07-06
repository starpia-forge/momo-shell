import { useEffect } from 'react'
import { cn } from '../../lib/cn'
import { useToastStore } from './toastStore'

const AUTO_DISMISS_MS = 4000

function ToastHostItem({ id, message, onClick }: { id: string; message: string; onClick?: () => void }) {
  useEffect(() => {
    const timer = setTimeout(() => useToastStore.getState().remove(id), AUTO_DISMISS_MS)
    return () => clearTimeout(timer)
  }, [id])

  return (
    <div
      className={cn(
        'toast-host__item max-w-80 overflow-hidden text-ellipsis whitespace-nowrap rounded px-3.5 py-2 bg-surface border border-line text-fg text-[12px] shadow-float',
        onClick && 'cursor-pointer',
      )}
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
    <div className="toast-host fixed right-4 bottom-4 z-500 flex flex-col-reverse items-end gap-2">
      {toasts.map((t) => (
        <ToastHostItem key={t.id} id={t.id} message={t.message} onClick={t.onClick} />
      ))}
    </div>
  )
}
