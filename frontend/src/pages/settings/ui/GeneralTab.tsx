import { useTranslation } from 'react-i18next'
import { useSettingsStore, type Language } from '../../../entities/settings'
import { Select, useToastStore, type SelectOption } from '../../../shared/ui'

export function GeneralTab() {
  const { t } = useTranslation()
  const language = useSettingsStore((s) => s.language)

  const options: SelectOption<Language>[] = [
    { value: 'system', label: t('settings.general.languageSystem') },
    { value: 'ko', label: '한국어' },
    { value: 'en', label: 'English' },
    { value: 'zh', label: '简体中文' },
    { value: 'ja', label: '日本語' },
  ]

  function handleLanguageChange(value: Language) {
    useSettingsStore
      .getState()
      .setLanguage(value)
      .catch(() => useToastStore.getState().push(t('settings.general.languageChangeFailed'), 'error'))
  }

  return (
    <section className="flex flex-col gap-2">
      <h2 className="text-[20px] font-bold mb-1">{t('settings.general.title')}</h2>
      <div className="flex items-center justify-between py-3.5 border-b border-line">
        <div className="flex flex-col gap-1">
          <span className="text-[14px] font-medium">{t('settings.general.language')}</span>
          <span className="text-[12px] text-fg3">{t('settings.general.languageDesc')}</span>
        </div>
        <Select className="w-55" value={language} options={options} onChange={handleLanguageChange} />
      </div>
    </section>
  )
}
