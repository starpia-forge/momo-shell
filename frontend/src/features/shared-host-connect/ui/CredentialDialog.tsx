import { useState } from 'react'
import { Dialog, Button, TextInput } from '../../../shared/ui'
import { browseForKeyFile, type AuthType, type Host } from '../../../shared/api/host'
import type { SharedHost } from '../../../shared/api/share'
import { connectSharedHost } from '../lib/connectSharedHost'
import './CredentialDialog.css'

interface CredentialDialogProps {
  sharedHost: SharedHost
  onClose: () => void
  onConnected: (result: { sessionId: string; host?: Host }) => void
}

export function CredentialDialog({ sharedHost, onClose, onConnected }: CredentialDialogProps) {
  const [username, setUsername] = useState(sharedHost.username)
  const [authType, setAuthType] = useState<AuthType>('password')
  const [keyPath, setKeyPath] = useState('')
  const [secret, setSecret] = useState('')
  const [saveAsHost, setSaveAsHost] = useState(true)
  const [connecting, setConnecting] = useState(false)
  const [error, setError] = useState<string>()

  const keyPathError = authType === 'privateKey' && keyPath.trim() === '' ? '키 파일을 선택하세요' : undefined
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
      setError('연결에 실패했습니다')
      setConnecting(false)
    }
  }

  return (
    <Dialog open onClose={onClose} title={`${sharedHost.name}에 연결`}>
      <form
        className="credential-dialog"
        onSubmit={(e) => {
          e.preventDefault()
          void submit()
        }}
      >
        <p className="credential-dialog__target">
          {sharedHost.address}:{sharedHost.port}
        </p>

        <TextInput label="사용자명" value={username} onChange={(e) => setUsername(e.target.value)} />

        <div className="credential-dialog__field">
          <span className="credential-dialog__label">인증 방식</span>
          <div className="credential-dialog__radios">
            {(['password', 'privateKey', 'agent'] as const).map((type) => (
              <label key={type} className="credential-dialog__radio">
                <input type="radio" name="authType" checked={authType === type} onChange={() => setAuthType(type)} />
                {type === 'password' ? '비밀번호' : type === 'privateKey' ? 'SSH 키' : 'Agent'}
              </label>
            ))}
          </div>
        </div>

        {authType === 'password' && (
          <TextInput label="비밀번호" type="password" value={secret} onChange={(e) => setSecret(e.target.value)} autoFocus />
        )}

        {authType === 'privateKey' && (
          <>
            <div className="credential-dialog__row">
              <TextInput label="키 파일*" value={keyPath} readOnly error={keyPathError} />
              <Button type="button" onClick={handleBrowseKeyFile}>
                찾아보기
              </Button>
            </div>
            <TextInput label="패스프레이즈" type="password" value={secret} onChange={(e) => setSecret(e.target.value)} />
          </>
        )}

        <label className="credential-dialog__checkbox">
          <input type="checkbox" checked={saveAsHost} onChange={(e) => setSaveAsHost(e.target.checked)} />
          내 호스트로 저장하며 연결
        </label>

        {error && <div className="credential-dialog__error">{error}</div>}

        <div className="credential-dialog__actions">
          <Button type="submit" variant="primary" disabled={!isValid || connecting}>
            {connecting ? '연결 중...' : '연결'}
          </Button>
          <Button type="button" onClick={onClose}>
            취소
          </Button>
        </div>
      </form>
    </Dialog>
  )
}
