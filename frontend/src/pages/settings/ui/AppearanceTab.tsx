import { useTranslation } from 'react-i18next'
import { cn } from '../../../shared/lib/cn'
import { useSettingsStore, type Accent, type Theme } from '../../../entities/settings'
import { useToastStore } from '../../../shared/ui'

const ACCENT_OPTIONS: { value: Accent; hex: string }[] = [
  { value: 'pink', hex: '#febecb' },
  { value: 'orange', hex: '#f0a05a' },
  { value: 'purple', hex: '#c9a0ff' },
  { value: 'blue', hex: '#8fb4e8' },
]

export function AppearanceTab() {
  const { t } = useTranslation()
  const theme = useSettingsStore((s) => s.theme)
  const accent = useSettingsStore((s) => s.accent)

  const THEME_OPTIONS: { value: Theme; label: string }[] = [
    { value: 'dark', label: t('settings.appearance.themeDark') },
    { value: 'light', label: t('settings.appearance.themeLight') },
  ]

  function handleThemeChange(value: Theme) {
    useSettingsStore
      .getState()
      .setTheme(value)
      .catch(() => useToastStore.getState().push(t('settings.appearance.themeChangeFailed'), 'error'))
  }

  function handleAccentChange(value: Accent) {
    useSettingsStore
      .getState()
      .setAccent(value)
      .catch(() => useToastStore.getState().push(t('settings.appearance.accentChangeFailed'), 'error'))
  }

  return (
    <section className="flex flex-col gap-9">
      <div className="flex flex-col gap-2">
        <h2 className="text-[20px] font-bold">{t('settings.appearance.title')}</h2>
        <p className="text-[13px] text-fg2">{t('settings.appearance.themeDesc')}</p>
      </div>

      <div className="flex gap-5">
        {THEME_OPTIONS.map((opt) => {
          const selected = theme === opt.value
          const light = opt.value === 'light'
          return (
            <button
              key={opt.value}
              className="w-62.5 flex flex-col gap-3 bg-transparent border-none cursor-pointer text-left"
              onClick={() => handleThemeChange(opt.value)}
            >
              <div
                className={cn(
                  'h-40 rounded-xl overflow-hidden flex flex-col border-2',
                  selected ? 'border-accent shadow-[0_0_0_4px_rgba(254,190,203,0.15)]' : 'border-line',
                  light ? 'bg-[#f2f0ec]' : 'bg-[#17181c]',
                )}
              >
                <div className={cn('h-6.5 flex items-center gap-1.25 px-2.5 border-b', light ? 'bg-white border-[#e0dcd4]' : 'bg-[#1e2026] border-[#2e323b]')}>
                  <span className="w-3 h-3 rounded-[4px]" style={{ background: light ? '#e79aab' : '#febecb' }} />
                  <span className={cn('w-8.5 h-2 rounded-full', light ? 'bg-[#eceae4]' : 'bg-[#262a32]')} />
                  <span className={cn('w-8.5 h-2 rounded-full', light ? 'bg-[#eceae4]' : 'bg-[#262a32]')} />
                </div>
                <div className="flex-1 flex gap-2 p-2.5">
                  <div className={cn('w-13 rounded-md', light ? 'bg-white' : 'bg-[#1e2026]')} />
                  <div className={cn('flex-1 rounded-md p-2 flex flex-col gap-1.25 border', light ? 'bg-[#fdfcfb] border-[#e0dcd4]' : 'bg-[#101116] border-transparent')}>
                    <span className="w-[70%] h-1.75 rounded-full" style={{ background: light ? '#dcd8d0' : '#2e323b' }} />
                    <span className="w-[50%] h-1.75 rounded-full" style={{ background: light ? '#dcd8d0' : '#2e323b' }} />
                    <span className="w-[60%] h-1.75 rounded-full opacity-60" style={{ background: light ? '#e79aab' : '#febecb' }} />
                  </div>
                </div>
              </div>
              <div className="flex items-center gap-2.25">
                <span
                  className={cn(
                    'w-4.25 h-4.25 rounded-full border-2 flex items-center justify-center',
                    selected ? 'border-accent' : 'border-fg3',
                  )}
                >
                  {selected && <span className="w-2 h-2 rounded-full bg-accent" />}
                </span>
                <span className={cn('text-[13.5px]', selected ? 'font-bold' : 'text-fg2')}>{opt.label}</span>
              </div>
            </button>
          )
        })}
      </div>

      <div className="h-px bg-line" />

      <div className="flex items-center justify-between">
        <div className="flex flex-col gap-1">
          <span className="text-[14px] font-medium">{t('settings.appearance.accentLabel')}</span>
          <span className="text-[12px] text-fg3">{t('settings.appearance.accentDesc')}</span>
        </div>
        <div className="flex items-center gap-3">
          {ACCENT_OPTIONS.map((opt) => {
            const selected = accent === opt.value
            return (
              <button
                key={opt.value}
                className={cn(
                  'w-8.5 h-8.5 rounded-full flex items-center justify-center cursor-pointer border-2',
                  selected ? 'border-accent' : 'border-transparent',
                )}
                onClick={() => handleAccentChange(opt.value)}
                aria-label={opt.value}
              >
                <span className="w-6 h-6 rounded-full" style={{ background: opt.hex }} />
              </button>
            )
          })}
        </div>
      </div>
    </section>
  )
}
