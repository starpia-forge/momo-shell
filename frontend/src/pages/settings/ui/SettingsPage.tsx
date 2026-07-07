import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { cn } from '../../../shared/lib/cn'
import { GeneralTab } from './GeneralTab'
import { AppearanceTab } from './AppearanceTab'
import { TerminalTab } from './TerminalTab'
import { SharingTab } from './SharingTab'
import { AboutTab } from './AboutTab'

type SettingsTab = 'general' | 'appearance' | 'terminal' | 'sharing' | 'about'

const TABS = [
  { id: 'general', labelKey: 'settings.tabs.general' },
  { id: 'appearance', labelKey: 'settings.tabs.appearance' },
  { id: 'terminal', labelKey: 'settings.tabs.terminal' },
  { id: 'sharing', labelKey: 'settings.tabs.sharing' },
  { id: 'about', labelKey: 'settings.tabs.about' },
] as const satisfies { id: SettingsTab; labelKey: string }[]

export function SettingsPage() {
  const { t } = useTranslation()
  const [tab, setTab] = useState<SettingsTab>('general')

  return (
    <div className="settings-page flex-1 min-w-0 h-full flex bg-canvas text-fg">
      <div className="flex-none w-60 bg-surface border-r border-line flex flex-col py-7 px-4 gap-1">
        {TABS.map((tabItem) => (
          <button
            key={tabItem.id}
            className={cn(
              'text-[13.5px] text-left px-3.5 py-2.75 rounded-md border-none cursor-pointer',
              tab === tabItem.id ? 'bg-surface2 font-bold text-fg' : 'text-fg2 bg-transparent',
            )}
            onClick={() => setTab(tabItem.id)}
          >
            {t(tabItem.labelKey)}
          </button>
        ))}
      </div>
      <div className="flex-1 overflow-y-auto py-12 px-16">
        <div className="max-w-190 flex flex-col gap-9">
          {tab === 'general' && <GeneralTab />}
          {tab === 'appearance' && <AppearanceTab />}
          {tab === 'terminal' && <TerminalTab />}
          {tab === 'sharing' && <SharingTab />}
          {tab === 'about' && <AboutTab />}
        </div>
      </div>
    </div>
  )
}
