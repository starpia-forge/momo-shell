import { useState } from 'react'
import { Dialog, Button, SegmentedControl, TextInput } from '../../../shared/ui'
import { browseForKeyFile, type AuthType, type Host } from '../../../shared/api/host'
import type { SharedHost } from '../../../shared/api/share'
import { connectSharedHost } from '../lib/connectSharedHost'

const AUTH_OPTIONS: { value: AuthType; label: string }[] = [
  { value: 'password', label: '비밀번호' },
  { value: 'privateKey', label: 'SSH 키' },
  { value: 'agent', label: 'Agent' },
]

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
        className="flex flex-col gap-4 w-90 text-[13px]"
        onSubmit={(e) => {
          e.preventDefault()
          void submit()
        }}
      >
        <p className="m-0 text-fg2 font-mono text-[12px]">
          {sharedHost.address}:{sharedHost.port}
        </p>

        <TextInput label="사용자명" value={username} onChange={(e) => setUsername(e.target.value)} />

        <div className="flex flex-col gap-1.75">
          <span className="text-[12.5px] font-medium text-fg2">인증</span>
          <SegmentedControl value={authType} onChange={setAuthType} options={AUTH_OPTIONS} />
        </div>

        {authType === 'password' && (
          <TextInput label="비밀번호" type="password" value={secret} onChange={(e) => setSecret(e.target.value)} autoFocus />
        )}

        {authType === 'privateKey' && (
          <>
            <div className="flex items-end gap-2 [&>*:first-child]:flex-1">
              <TextInput label="키 파일*" value={keyPath} readOnly error={keyPathError} />
              <Button type="button" onClick={handleBrowseKeyFile}>
                찾아보기
              </Button>
            </div>
            <TextInput label="패스프레이즈" type="password" value={secret} onChange={(e) => setSecret(e.target.value)} />
          </>
        )}

        <label className="flex items-center gap-1.5 cursor-pointer">
          <input type="checkbox" checked={saveAsHost} onChange={(e) => setSaveAsHost(e.target.checked)} />
          내 호스트로 저장하며 연결
        </label>

        {error && <div className="text-red text-[12px]">{error}</div>}

        <div className="flex justify-end gap-2.5">
          <Button type="button" onClick={onClose}>
            취소
          </Button>
          <Button type="submit" variant="primary" disabled={!isValid || connecting}>
            {connecting ? '연결 중...' : '연결'}
          </Button>
        </div>
      </form>
    </Dialog>
  )
}
