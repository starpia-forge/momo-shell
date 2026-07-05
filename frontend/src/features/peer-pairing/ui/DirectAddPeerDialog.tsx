import { useState } from 'react'
import { Dialog, Button, TextInput } from '../../../shared/ui'
import { addPeerByAddress, describeShareError } from '../../../shared/api/share'
import { loadPeers } from '../../../entities/peer'
import './DirectAddPeerDialog.css'

interface DirectAddPeerDialogProps {
  onClose: () => void
}

const DEFAULT_PORT = '47800'

export function DirectAddPeerDialog({ onClose }: DirectAddPeerDialogProps) {
  const [address, setAddress] = useState('')
  const [port, setPort] = useState(DEFAULT_PORT)
  const [pin, setPin] = useState('')
  const [error, setError] = useState<string>()
  const [pending, setPending] = useState(false)

  const portNum = Number(port)
  const isValid = address.trim() !== '' && Number.isInteger(portNum) && portNum > 0 && portNum <= 65535 && pin.length === 6

  async function submit() {
    if (!isValid) return
    setPending(true)
    setError(undefined)
    try {
      await addPeerByAddress(address.trim(), portNum, pin)
      await loadPeers()
      onClose()
    } catch (err) {
      const described = describeShareError(err)
      setError(described === '요청이 실패했습니다' ? 'PIN이 올바르지 않거나 요청이 거부되었습니다' : described)
      setPending(false)
    }
  }

  return (
    <Dialog open onClose={onClose} title="IP로 피어 추가">
      <form
        className="direct-add-peer"
        onSubmit={(e) => {
          e.preventDefault()
          void submit()
        }}
      >
        <div className="direct-add-peer__row">
          <TextInput label="주소" value={address} onChange={(e) => setAddress(e.target.value)} autoFocus placeholder="10.0.1.5" />
          <TextInput label="포트" value={port} onChange={(e) => setPort(e.target.value)} />
        </div>
        <TextInput
          label="PIN"
          value={pin}
          onChange={(e) => setPin(e.target.value.replace(/\D/g, '').slice(0, 6))}
          maxLength={6}
          inputMode="numeric"
          error={error}
        />
        <div className="direct-add-peer__actions">
          <Button type="submit" variant="primary" disabled={!isValid || pending}>
            연결
          </Button>
          <Button type="button" onClick={onClose}>
            취소
          </Button>
        </div>
      </form>
    </Dialog>
  )
}
