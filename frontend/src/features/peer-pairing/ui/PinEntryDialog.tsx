import { useState } from 'react'
import { Dialog, Button, TextInput } from '../../../shared/ui'
import { pairWithPeer } from '../../../shared/api/share'
import { loadPeers } from '../../../entities/peer'
import './PinEntryDialog.css'

interface PinEntryDialogProps {
  peerId: string
  peerName: string
  onClose: () => void
}

export function PinEntryDialog({ peerId, peerName, onClose }: PinEntryDialogProps) {
  const [pin, setPin] = useState('')
  const [error, setError] = useState<string>()
  const [pending, setPending] = useState(false)

  async function submit() {
    setPending(true)
    setError(undefined)
    try {
      await pairWithPeer(peerId, pin)
      await loadPeers()
      onClose()
    } catch {
      setError('PIN이 올바르지 않거나 요청이 거부되었습니다')
      setPending(false)
    }
  }

  return (
    <Dialog open onClose={onClose} title={`${peerName}에 연결`}>
      <form
        className="pin-entry"
        onSubmit={(e) => {
          e.preventDefault()
          void submit()
        }}
      >
        <TextInput
          label="PIN"
          value={pin}
          onChange={(e) => setPin(e.target.value.replace(/\D/g, '').slice(0, 6))}
          maxLength={6}
          inputMode="numeric"
          autoFocus
          error={error}
        />
        <div className="pin-entry__actions">
          <Button type="submit" variant="primary" disabled={pending || pin.length !== 6}>
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
