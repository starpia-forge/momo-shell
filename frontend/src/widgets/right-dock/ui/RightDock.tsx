import type { ReactNode } from 'react'
import { useRightDockStore } from '../model/store'
import { cn } from '../../../shared/lib/cn'

interface RightDockProps {
  historyPanel: ReactNode
  filesPanel: ReactNode
  sharePanel: ReactNode
}

/** Hosts HISTORY, FILES, and SHARE as mutually-exclusive tabs in the
 * fixed-width right rail -- neither panel widget manages its own
 * visibility anymore. */
export function RightDock({ historyPanel, filesPanel, sharePanel }: RightDockProps) {
  const open = useRightDockStore((s) => s.open)
  const active = useRightDockStore((s) => s.active)

  if (!open) return null

  return (
    <div className="flex flex-col w-70 flex-none h-full bg-surface border-l border-line text-fg">
      <div className="flex flex-none border-b border-line">
        <button
          className={cn(
            'flex-1 p-2 text-[11px] font-semibold tracking-wider text-fg2 bg-transparent border-none cursor-pointer',
            active === 'history' && 'bg-canvas text-fg',
          )}
          onClick={() => useRightDockStore.getState().show('history')}
        >
          히스토리
        </button>
        <button
          className={cn(
            'flex-1 p-2 text-[11px] font-semibold tracking-wider text-fg2 bg-transparent border-none cursor-pointer',
            active === 'files' && 'bg-canvas text-fg',
          )}
          onClick={() => useRightDockStore.getState().show('files')}
        >
          파일
        </button>
        <button
          className={cn(
            'flex-1 p-2 text-[11px] font-semibold tracking-wider text-fg2 bg-transparent border-none cursor-pointer',
            active === 'share' && 'bg-canvas text-fg',
          )}
          onClick={() => useRightDockStore.getState().show('share')}
        >
          공유
        </button>
      </div>
      <div className="flex-1 min-h-0 flex flex-col">{active === 'history' ? historyPanel : active === 'files' ? filesPanel : sharePanel}</div>
    </div>
  )
}
