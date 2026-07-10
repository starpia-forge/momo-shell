import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { getAppInfo } from '../../../shared/api/settings'

export function AboutTab() {
  const { t } = useTranslation()
  const [version, setVersion] = useState('')

  useEffect(() => {
    void getAppInfo().then((info) => setVersion(info.version))
  }, [])

  return (
    <section className="flex flex-col gap-2">
      <h2 className="text-[20px] font-bold mb-1">{t('settings.about.title')}</h2>
      <div className="flex items-center justify-between py-3.5 border-b border-line">
        <span className="text-[14px] font-medium">MomoShell</span>
        <span className="text-[13px] text-fg2">{version}</span>
      </div>
    </section>
  )
}
