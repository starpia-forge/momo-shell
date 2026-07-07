import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Dialog, Button, PinInput } from '../../../shared/ui'
import { pairWithPeer, describeShareError } from '../../../shared/api/share'
import { loadPeers } from '../../../entities/peer'

interface PinEntryDialogProps {
  peerId: string
  peerName: string
  onClose: () => void
}

export function PinEntryDialog({ peerId, peerName, onClose }: PinEntryDialogProps) {
  const { t } = useTranslation()
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
      setError(described === t('shareErrors.generic') ? t('peerPairing.pinInvalidOrRejected') : described)
      setPending(false)
    }
  }

  return (
    <Dialog open onClose={onClose} title={t('credentialDialog.title', { name: peerName })}>
      <form
        className="flex flex-col gap-4 w-100"
        onSubmit={(e) => {
          e.preventDefault()
          void submit()
        }}
      >
        <p className="m-0 text-[13px] text-fg2 leading-relaxed">{t('peerPairing.enterPinHint')}</p>
        <PinInput value={pin} onChange={setPin} autoFocus error={error} />
        {pending && <p className="m-0 text-[12px] text-fg2 text-center">{t('peerPairing.waitingApproval')}</p>}
        <div className="flex justify-end gap-2.5">
          <Button type="button" onClick={onClose}>
            {t('common.cancel')}
          </Button>
          <Button type="submit" variant="primary" disabled={pending || pin.length !== 6}>
            {t('common.connect')}
          </Button>
        </div>
      </form>
    </Dialog>
  )
}
