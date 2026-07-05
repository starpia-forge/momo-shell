import type { ReactNode } from 'react'
import { useRightDockStore } from '../model/store'
import './RightDock.css'

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
    <div className="right-dock">
      <div className="right-dock__tabs">
        <button
          className={['right-dock__tab', active === 'history' && 'right-dock__tab--active'].filter(Boolean).join(' ')}
          onClick={() => useRightDockStore.getState().show('history')}
        >
          히스토리
        </button>
        <button
          className={['right-dock__tab', active === 'files' && 'right-dock__tab--active'].filter(Boolean).join(' ')}
          onClick={() => useRightDockStore.getState().show('files')}
        >
          파일
        </button>
        <button
          className={['right-dock__tab', active === 'share' && 'right-dock__tab--active'].filter(Boolean).join(' ')}
          onClick={() => useRightDockStore.getState().show('share')}
        >
          공유
        </button>
      </div>
      <div className="right-dock__body">{active === 'history' ? historyPanel : active === 'files' ? filesPanel : sharePanel}</div>
    </div>
  )
}
