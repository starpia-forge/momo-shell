import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Dialog, Button, SegmentedControl, TextInput } from '../../../shared/ui'
import { browseForKeyFile, type AuthType, type Host } from '../../../shared/api/host'
import type { SharedHost } from '../../../shared/api/share'
import { connectSharedHost } from '../lib/connectSharedHost'

interface CredentialDialogProps {
  sharedHost: SharedHost
  onClose: () => void
  onConnected: (result: { sessionId: string; host?: Host }) => void
}

export function CredentialDialog({ sharedHost, onClose, onConnected }: CredentialDialogProps) {
  const { t } = useTranslation()
  const AUTH_OPTIONS: { value: AuthType; label: string }[] = [
    { value: 'password', label: t('hostForm.authPassword') },
    { value: 'privateKey', label: t('hostForm.authKey') },
    { value: 'agent', label: t('hostForm.authAgent') },
  ]
  const [username, setUsername] = useState(sharedHost.username)
  const [authType, setAuthType] = useState<AuthType>('password')
  const [keyPath, setKeyPath] = useState('')
  const [secret, setSecret] = useState('')
  const [saveAsHost, setSaveAsHost] = useState(true)
  const [connecting, setConnecting] = useState(false)
  const [error, setError] = useState<string>()

  const keyPathError = authType === 'privateKey' && keyPath.trim() === '' ? t('hostForm.errorKeyPathRequired') : undefined
  const isValid = !keyPathError

  async function handleBrowseKeyFile() {
    const path = await browseForKeyFile()
    if (path) setKeyPath(path)
  }

  async function submit() {
    if (!isValid) return
    setConnecting(true)
    setError(undefined)
    try {
      const result = await connectSharedHost(sharedHost, {
        username,
        authType,
        keyPath: authType === 'privateKey' ? keyPath : undefined,
        secret,
        saveAsHost,
      })
      onConnected(result)
    } catch {
      setError(t('credentialDialog.connectFailed'))
      setConnecting(false)
    }
  }

  return (
    <Dialog open onClose={onClose} title={t('credentialDialog.title', { name: sharedHost.name })}>
      <form
        className="flex flex-col gap-4 w-90 text-[13px]"
        onSubmit={(e) => {
          e.preventDefault()
          void submit()
        }}
      >
        <p className="m-0 text-fg2 font-mono text-[12px]">
          {sharedHost.address}:{sharedHost.port}
        </p>

        <TextInput label={t('credentialDialog.username')} value={username} onChange={(e) => setUsername(e.target.value)} />

        <div className="flex flex-col gap-1.75">
          <span className="text-[12.5px] font-medium text-fg2">{t('hostForm.auth')}</span>
          <SegmentedControl value={authType} onChange={setAuthType} options={AUTH_OPTIONS} />
        </div>

        {authType === 'password' && (
          <TextInput label={t('hostForm.authPassword')} type="password" value={secret} onChange={(e) => setSecret(e.target.value)} autoFocus />
        )}

        {authType === 'privateKey' && (
          <>
            <div className="flex items-end gap-2 [&>*:first-child]:flex-1">
              <TextInput label={t('hostForm.labelKeyFile')} value={keyPath} readOnly error={keyPathError} />
              <Button type="button" onClick={handleBrowseKeyFile}>
                {t('hostForm.browse')}
              </Button>
            </div>
            <TextInput label={t('hostForm.labelPassphrase')} type="password" value={secret} onChange={(e) => setSecret(e.target.value)} />
          </>
        )}

        <label className="flex items-center gap-1.5 cursor-pointer">
          <input type="checkbox" checked={saveAsHost} onChange={(e) => setSaveAsHost(e.target.checked)} />
          {t('credentialDialog.saveAsHost')}
        </label>

        {error && <div className="text-red text-[12px]">{error}</div>}

        <div className="flex justify-end gap-2.5">
          <Button type="button" onClick={onClose}>
            {t('common.cancel')}
          </Button>
          <Button type="submit" variant="primary" disabled={!isValid || connecting}>
            {connecting ? t('common.connecting') : t('common.connect')}
          </Button>
        </div>
      </form>
    </Dialog>
  )
}
