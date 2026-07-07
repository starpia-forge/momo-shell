import { useEffect } from 'react'
import { cn } from '../../lib/cn'
import { useToastStore, type ToastVariant } from './toastStore'

const AUTO_DISMISS_MS = 4000

const ICON: Record<ToastVariant, { glyph: string; tint: string }> = {
  success: { glyph: '✓', tint: 'bg-green/16 text-green' },
  error: { glyph: '!', tint: 'bg-red/16 text-red' },
  info: { glyph: 'i', tint: 'bg-blue/16 text-blue' },
}

function ToastHostItem({
  id,
  message,
  variant,
  onClick,
}: {
  id: string
  message: string
  variant: ToastVariant
  onClick?: () => void
}) {
  useEffect(() => {
    const timer = setTimeout(() => useToastStore.getState().remove(id), AUTO_DISMISS_MS)
    return () => clearTimeout(timer)
  }, [id])

  const icon = ICON[variant]

  return (
    <div
      className={cn(
        'toast-host__item flex items-center gap-3 max-w-80 rounded-lg px-4.5 py-3.5 bg-surface2 border border-line text-fg text-[13px] shadow-menu',
        onClick && 'cursor-pointer',
      )}
      role={onClick ? 'button' : undefined}
      onClick={() => {
        onClick?.()
        useToastStore.getState().remove(id)
      }}
    >
      <span
        className={cn(
          'flex-none flex items-center justify-center w-5.5 h-5.5 rounded-full text-[12px]',
          icon.tint,
        )}
      >
        {icon.glyph}
      </span>
      <span className="flex-1 overflow-hidden text-ellipsis whitespace-nowrap">{message}</span>
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
        <ToastHostItem key={t.id} id={t.id} message={t.message} variant={t.variant} onClick={t.onClick} />
      ))}
    </div>
  )
}
