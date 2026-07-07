import { useEffect, useRef } from 'react'
import { cn } from '../../lib/cn'

export interface ContextMenuItem {
  label: string
  onClick: () => void
  danger?: boolean
  /** Render a divider above this item. */
  divider?: boolean
}

interface ContextMenuProps {
  x: number
  y: number
  items: ContextMenuItem[]
  onClose: () => void
}

export function ContextMenu({ x, y, items, onClose }: ContextMenuProps) {
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const onPointerDown = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) onClose()
    }
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    document.addEventListener('mousedown', onPointerDown)
    document.addEventListener('keydown', onKeyDown)
    return () => {
      document.removeEventListener('mousedown', onPointerDown)
      document.removeEventListener('keydown', onKeyDown)
    }
  }, [onClose])

  return (
    <div
      className="context-menu fixed z-200 flex min-w-[150px] flex-col gap-0 rounded-lg border border-line bg-surface2 p-1.5 shadow-menu"
      ref={ref}
      style={{ left: x, top: y }}
    >
      {items.map((item) => (
        <div key={item.label}>
          {item.divider && <div className="h-px bg-line my-1.25 mx-2" />}
          <button
            className={cn(
              'w-full rounded-md px-3 py-2 text-left text-[12.5px] cursor-pointer hover:bg-accent/14',
              item.danger ? 'text-red' : 'text-fg',
            )}
            onClick={() => {
              item.onClick()
              onClose()
            }}
          >
            {item.label}
          </button>
        </div>
      ))}
    </div>
  )
}
