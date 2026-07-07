import { useEffect, useState } from 'react'
import { cn } from '../../../shared/lib/cn'
import { Dialog, Button, TextInput } from '../../../shared/ui'
import { setHostSecret, testConnection, browseForKeyFile, type AuthType, type Host, type TestResult } from '../../../shared/api/host'
import { useHostStore } from '../../../entities/host'

interface HostFormDialogProps {
  open: boolean
  onClose: () => void
  /** Editing an existing host. Omit (with cloneFrom unset too) to create a fresh one. */
  hostId?: string
  /** Prefill fields from an existing host without editing it (host-sidebar's "복제"). */
  cloneFrom?: Host
  /** Called with the saved host right after a successful save (SFTP page's
   * ConnectGate uses this to connect immediately after registering a new host). */
  onSaved?: (host: Host) => void
}

interface FormState {
  name: string
  address: string
  port: string
  labels: string[]
  labelDraft: string
  username: string
  authType: AuthType
  keyPath: string
  secret: string
  secretTouched: boolean
  showSecret: boolean
}

function emptyForm(): FormState {
  return {
    name: '',
    address: '',
    port: '22',
    labels: [],
    labelDraft: '',
    username: '',
    authType: 'password',
    keyPath: '',
    secret: '',
    secretTouched: false,
    showSecret: false,
  }
}

function formFromHost(h: Host): FormState {
  return {
    name: h.name,
    address: h.address,
    port: String(h.port),
    labels: h.labels,
    labelDraft: '',
    username: h.username,
    authType: h.authType,
    keyPath: h.keyPath ?? '',
    secret: '',
    secretTouched: false,
    showSecret: false,
  }
}

export function HostFormDialog({ open, onClose, hostId, cloneFrom, onSaved }: HostFormDialogProps) {
  const existing = useHostStore((s) => (hostId ? s.hosts[hostId] : undefined))
  const save = useHostStore((s) => s.save)
  const [form, setForm] = useState<FormState>(emptyForm)
  const [savedId, setSavedId] = useState<string | undefined>(hostId)
  const [testResult, setTestResult] = useState<TestResult | null>(null)
  const [testing, setTesting] = useState(false)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (!open) return
    setSavedId(hostId)
    setTestResult(null)
    if (existing) {
      setForm(formFromHost(existing))
    } else if (cloneFrom) {
      setForm({ ...formFromHost(cloneFrom), name: `${cloneFrom.name} 복사본` })
    } else {
      setForm(emptyForm())
    }
    // Only re-seed when the dialog transitions to open, not on every keystroke.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open])

  const port = Number(form.port)
  const errors = {
    name: form.name.trim() === '' ? '이름을 입력하세요' : undefined,
    address: form.address.trim() === '' ? '주소를 입력하세요' : undefined,
    username: form.username.trim() === '' ? '사용자명을 입력하세요' : undefined,
    port: !Number.isInteger(port) || port < 1 || port > 65535 ? '1-65535 사이의 포트' : undefined,
    keyPath: form.authType === 'privateKey' && form.keyPath.trim() === '' ? '키 파일을 선택하세요' : undefined,
  }
  const isValid = Object.values(errors).every((e) => !e)

  async function persist(): Promise<Host> {
    const host = await save({
      id: savedId,
      name: form.name.trim(),
      address: form.address.trim(),
      port,
      labels: form.labels,
      username: form.username.trim(),
      authType: form.authType,
      keyPath: form.authType === 'privateKey' ? form.keyPath : undefined,
    })
    if (form.secretTouched && form.secret !== '') {
      await setHostSecret(host.id, form.secret)
    }
    setSavedId(host.id)
    return host
  }

  async function handleTestConnection() {
    if (!isValid) return
    setTesting(true)
    setTestResult(null)
    try {
      const host = await persist()
      setTestResult(await testConnection(host.id))
    } finally {
      setTesting(false)
    }
  }

  async function handleSave() {
    if (!isValid) return
    setSaving(true)
    try {
      const host = await persist()
      onSaved?.(host)
      onClose()
    } finally {
      setSaving(false)
    }
  }

  function addLabel() {
    const label = form.labelDraft.trim()
    if (label === '' || form.labels.includes(label)) {
      setForm((f) => ({ ...f, labelDraft: '' }))
      return
    }
    setForm((f) => ({ ...f, labels: [...f.labels, label], labelDraft: '' }))
  }

  function removeLabel(label: string) {
    setForm((f) => ({ ...f, labels: f.labels.filter((l) => l !== label) }))
  }

  async function handleBrowseKeyFile() {
    const path = await browseForKeyFile()
    if (path) setForm((f) => ({ ...f, keyPath: path }))
  }

  return (
    <Dialog open={open} onClose={onClose} title={hostId ? '호스트 편집' : '호스트 추가'}>
      <div className="host-form flex flex-col gap-3 w-90">
        <TextInput
          label="이름*"
          value={form.name}
          onChange={(e) => setForm((f) => ({ ...f, name: e.target.value }))}
          error={errors.name}
        />
        <div className="flex gap-2 items-end *:flex-1">
          <TextInput
            label="주소*"
            value={form.address}
            onChange={(e) => setForm((f) => ({ ...f, address: e.target.value }))}
            error={errors.address}
          />
          <TextInput
            label="포트"
            value={form.port}
            onChange={(e) => setForm((f) => ({ ...f, port: e.target.value }))}
            error={errors.port}
          />
        </div>

        <div className="flex flex-col gap-1">
          <span className="text-[12px] text-fg2">라벨</span>
          <div className="flex flex-wrap gap-1.5 items-center">
            {form.labels.map((label) => (
              <span
                key={label}
                className="inline-flex items-center gap-1 px-1.5 py-0.5 rounded-full bg-canvas border border-line text-[12px]"
              >
                {label}
                <button
                  type="button"
                  className="border-none bg-transparent text-fg2 cursor-pointer text-[13px] leading-none"
                  onClick={() => removeLabel(label)}
                  aria-label={`${label} 제거`}
                >
                  ×
                </button>
              </span>
            ))}
            <input
              className="flex-1 min-w-20 border-none bg-transparent text-fg text-[12px] outline-none"
              value={form.labelDraft}
              placeholder="+ 추가"
              onChange={(e) => setForm((f) => ({ ...f, labelDraft: e.target.value }))}
              onKeyDown={(e) => {
                if (e.key === 'Enter') {
                  e.preventDefault()
                  addLabel()
                }
              }}
              onBlur={addLabel}
            />
          </div>
        </div>

        <TextInput
          label="사용자명*"
          value={form.username}
          onChange={(e) => setForm((f) => ({ ...f, username: e.target.value }))}
          error={errors.username}
        />

        <div className="flex flex-col gap-1">
          <span className="text-[12px] text-fg2">인증 방식</span>
          <div className="flex gap-3">
            {(['password', 'privateKey', 'agent'] as const).map((type) => (
              <label key={type} className="flex items-center gap-1 text-[13px]">
                <input
                  type="radio"
                  name="authType"
                  checked={form.authType === type}
                  onChange={() => setForm((f) => ({ ...f, authType: type }))}
                />
                {type === 'password' ? '비밀번호' : type === 'privateKey' ? 'SSH 키' : 'Agent'}
              </label>
            ))}
          </div>
        </div>

        {form.authType === 'password' && (
          <div className="flex gap-2 items-end">
            <TextInput
              label="비밀번호"
              type={form.showSecret ? 'text' : 'password'}
              value={form.secret}
              onChange={(e) => setForm((f) => ({ ...f, secret: e.target.value, secretTouched: true }))}
              placeholder={hostId ? '변경하지 않으려면 비워두세요' : ''}
            />
            <label className="flex items-center gap-1 text-[12px] whitespace-nowrap">
              <input
                type="checkbox"
                checked={form.showSecret}
                onChange={(e) => setForm((f) => ({ ...f, showSecret: e.target.checked }))}
              />
              표시
            </label>
          </div>
        )}

        {form.authType === 'privateKey' && (
          <>
            <div className="flex gap-2 items-end">
              <TextInput label="키 파일*" value={form.keyPath} readOnly error={errors.keyPath} />
              <Button type="button" onClick={handleBrowseKeyFile}>
                찾아보기
              </Button>
            </div>
            <TextInput
              label="패스프레이즈"
              type={form.showSecret ? 'text' : 'password'}
              value={form.secret}
              onChange={(e) => setForm((f) => ({ ...f, secret: e.target.value, secretTouched: true }))}
              placeholder={hostId ? '변경하지 않으려면 비워두세요' : ''}
            />
          </>
        )}

        {testResult && (
          <div className={cn('text-[12px] px-2 py-1.5 rounded', testResult.ok ? 'bg-green/15 text-green' : 'bg-red/15 text-red')}>
            {testResult.ok
              ? '연결 성공'
              : `실패 (${testResult.stage === 'tcp' ? '주소 불가' : testResult.stage === 'handshake' ? '호스트키 불일치' : '인증 실패'})${
                  testResult.message ? `: ${testResult.message}` : ''
                }`}
          </div>
        )}

        <div className="flex justify-between items-center mt-2">
          <Button type="button" onClick={handleTestConnection} disabled={!isValid || testing}>
            {testing ? '테스트 중...' : '연결 테스트'}
          </Button>
          <div className="flex gap-2">
            <Button type="button" onClick={onClose}>
              취소
            </Button>
            <Button type="button" variant="primary" onClick={handleSave} disabled={!isValid || saving}>
              저장
            </Button>
          </div>
        </div>
      </div>
    </Dialog>
  )
}
