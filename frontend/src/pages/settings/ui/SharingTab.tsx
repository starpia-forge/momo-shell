import { useEffect, useState, type KeyboardEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { getDeviceName, setDeviceName, describeShareError } from '../../../shared/api/share'
import { TextInput, useToastStore } from '../../../shared/ui'

export function SharingTab() {
  const { t } = useTranslation()
  const [name, setName] = useState('')

  useEffect(() => {
    void getDeviceName().then(setName)
  }, [])

  function commit() {
    setDeviceName(name.trim())
      .then(() => useToastStore.getState().push(t('settings.sharing.deviceNameSaved'), 'success'))
      .catch((err) => useToastStore.getState().push(describeShareError(err), 'error'))
  }

  function handleKeyDown(e: KeyboardEvent<HTMLInputElement>) {
    if (e.key === 'Enter') e.currentTarget.blur()
  }

  return (
    <section className="flex flex-col gap-2">
      <h2 className="text-[20px] font-bold mb-1">{t('settings.sharing.title')}</h2>
      <div className="flex items-center justify-between py-3.5 border-b border-line">
        <div className="flex flex-col gap-1">
          <span className="text-[14px] font-medium">{t('settings.sharing.deviceName')}</span>
          <span className="text-[12px] text-fg3">{t('settings.sharing.deviceNameDesc')}</span>
        </div>
        <TextInput className="w-55" value={name} onChange={(e) => setName(e.target.value)} onBlur={commit} onKeyDown={handleKeyDown} />
      </div>
    </section>
  )
}
