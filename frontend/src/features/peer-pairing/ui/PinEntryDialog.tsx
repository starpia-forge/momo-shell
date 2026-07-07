import { useState } from 'react'
import { Dialog, Button, PinInput } from '../../../shared/ui'
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
        className="flex flex-col gap-4 w-100"
        onSubmit={(e) => {
          e.preventDefault()
          void submit()
        }}
      >
        <p className="m-0 text-[13px] text-fg2 leading-relaxed">상대 화면에 표시된 PIN을 입력하세요.</p>
        <PinInput value={pin} onChange={setPin} autoFocus error={error} />
        {pending && <p className="m-0 text-[12px] text-fg2 text-center">상대방의 승인을 기다리는 중...</p>}
        <div className="flex justify-end gap-2.5">
          <Button type="button" onClick={onClose}>
            취소
          </Button>
          <Button type="submit" variant="primary" disabled={pending || pin.length !== 6}>
            연결
          </Button>
        </div>
      </form>
    </Dialog>
  )
}
