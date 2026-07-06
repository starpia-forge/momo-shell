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
      <div className="flex-none w-55 bg-surface border-r border-line flex flex-col">
        <div className="px-3 py-2.5 text-[14px] font-semibold border-b border-line">설정</div>
        <div className="flex flex-col p-2 gap-1">
          {TABS.map((t) => (
            <button
              key={t.id}
              className={cn(
                'text-[13px] text-left px-3 py-2 rounded border-none cursor-pointer',
                tab === t.id ? 'bg-canvas text-fg' : 'text-muted bg-transparent',
              )}
              onClick={() => setTab(t.id)}
            >
              {t.label}
            </button>
          ))}
        </div>
      </div>
      <div className="flex-1 overflow-y-auto p-5">
        <div className="max-w-[640px]">
          {tab === 'appearance' && <AppearanceTab />}
          {tab === 'terminal' && <TerminalTab />}
          {tab === 'sharing' && <SharingTab />}
          {tab === 'about' && <AboutTab />}
        </div>
      </div>
    </div>
  )
}
