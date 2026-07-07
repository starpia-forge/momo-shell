import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { useRightDockStore, type RightDockTab } from '../model/store'
import { cn } from '../../../shared/lib/cn'
import { IconButton } from '../../../shared/ui'

interface RightDockProps {
  historyPanel: ReactNode
  filesPanel: ReactNode
  sharePanel: ReactNode
}

/** Hosts HISTORY, FILES, and SHARE as mutually-exclusive panels behind a
 * always-visible icon rail -- the rail stays put (46px) whether or not the
 * 300px panel is open, so it never fights the terminal for who owns that
 * strip of the window. */
export function RightDock({ historyPanel, filesPanel, sharePanel }: RightDockProps) {
  const { t } = useTranslation()
  const open = useRightDockStore((s) => s.open)
  const active = useRightDockStore((s) => s.active)

  const RAIL: { tab: RightDockTab; glyph: string; label: string }[] = [
    { tab: 'history', glyph: '≡', label: t('rightDock.history') },
    { tab: 'files', glyph: '▤', label: t('rightDock.files') },
    { tab: 'share', glyph: '⇡', label: t('settings.sharing.title') },
  ]

  return (
    <div className="flex-none flex h-full border-l border-line">
      {open && (
        <div className="w-75 flex-none flex flex-col bg-surface border-r border-line text-fg min-h-0">
          {active === 'history' ? historyPanel : active === 'files' ? filesPanel : sharePanel}
        </div>
      )}
      <div className="w-11.5 flex-none flex flex-col items-center pt-3.5 gap-2 bg-surface">
        {RAIL.map((r) => (
          <IconButton
            key={r.tab}
            size={34}
            className={cn(
              'text-[14px] font-mono',
              open && active === r.tab && 'bg-accent/16 border border-accent text-accent-text',
            )}
            onClick={() => useRightDockStore.getState().toggle(r.tab)}
            aria-label={r.label}
          >
            {r.glyph}
          </IconButton>
        ))}
      </div>
    </div>
  )
}
