import { useState } from 'react'
import { cn } from '../../../shared/lib/cn'
import { AppearanceTab } from './AppearanceTab'
import { TerminalTab } from './TerminalTab'
import { SharingTab } from './SharingTab'
import { AboutTab } from './AboutTab'

type SettingsTab = 'appearance' | 'terminal' | 'sharing' | 'about'

const TABS: { id: SettingsTab; label: string }[] = [
  { id: 'appearance', label: '모양' },
  { id: 'terminal', label: '터미널' },
  { id: 'sharing', label: '공유' },
  { id: 'about', label: '정보' },
]

export function SettingsPage() {
  const [tab, setTab] = useState<SettingsTab>('appearance')

  return (
    <div className="settings-page flex-1 min-w-0 h-full flex bg-canvas text-fg">
      <div className="flex-none w-60 bg-surface border-r border-line flex flex-col py-7 px-4 gap-1">
        {TABS.map((t) => (
          <button
            key={t.id}
            className={cn(
              'text-[13.5px] text-left px-3.5 py-2.75 rounded-md border-none cursor-pointer',
              tab === t.id ? 'bg-surface2 font-bold text-fg' : 'text-fg2 bg-transparent',
            )}
            onClick={() => setTab(t.id)}
          >
            {t.label}
          </button>
        ))}
      </div>
      <div className="flex-1 overflow-y-auto py-12 px-16">
        <div className="max-w-190 flex flex-col gap-9">
          {tab === 'appearance' && <AppearanceTab />}
          {tab === 'terminal' && <TerminalTab />}
          {tab === 'sharing' && <SharingTab />}
          {tab === 'about' && <AboutTab />}
        </div>
      </div>
    </div>
  )
}
