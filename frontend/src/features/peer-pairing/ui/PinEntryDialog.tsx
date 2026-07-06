import { useState } from 'react'
import { Dialog, Button, TextInput } from '../../../shared/ui'
import { pairWithPeer, describeShareError } from '../../../shared/api/share'
import { loadPeers } from '../../../entities/peer'

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
    } catch (err) {
      const described = describeShareError(err)
      setError(described === '요청이 실패했습니다' ? 'PIN이 올바르지 않거나 요청이 거부되었습니다' : described)
      setPending(false)
    }
  }

  return (
    <Dialog open onClose={onClose} title={`${peerName}에 연결`}>
      <form
        className="flex flex-col gap-3 w-65"
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
        {pending && <p className="m-0 text-[12px] text-muted">상대방의 승인을 기다리는 중...</p>}
        <div className="flex justify-end gap-2">
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
