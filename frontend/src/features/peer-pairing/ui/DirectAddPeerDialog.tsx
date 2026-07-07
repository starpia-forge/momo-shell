import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Dialog, Button, PinInput, TextInput } from '../../../shared/ui'
import { addPeerByAddress, describeShareError } from '../../../shared/api/share'
import { loadPeers } from '../../../entities/peer'

interface DirectAddPeerDialogProps {
  onClose: () => void
}

const DEFAULT_PORT = '47800'

export function DirectAddPeerDialog({ onClose }: DirectAddPeerDialogProps) {
  const { t } = useTranslation()
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
      setError(described === t('shareErrors.generic') ? t('peerPairing.pinInvalidOrRejected') : described)
      setPending(false)
    }
  }

  return (
    <Dialog open onClose={onClose} title={t('peerPairing.addByAddressTitle')}>
      <form
        className="flex flex-col gap-4 w-100"
        onSubmit={(e) => {
          e.preventDefault()
          void submit()
        }}
      >
        <div className="flex gap-3 [&>*:first-child]:flex-2 [&>*:last-child]:flex-1">
          <TextInput label={t('peerPairing.addressLabel')} value={address} onChange={(e) => setAddress(e.target.value)} autoFocus placeholder="10.0.1.5" />
          <TextInput label={t('hostForm.labelPort')} className="font-mono" value={port} onChange={(e) => setPort(e.target.value)} />
        </div>
        <PinInput value={pin} onChange={setPin} error={error} />
        {pending && <p className="m-0 text-[12px] text-fg2 text-center">{t('peerPairing.waitingApproval')}</p>}
        <div className="flex justify-end gap-2.5">
          <Button type="button" onClick={onClose}>
            {t('common.cancel')}
          </Button>
          <Button type="submit" variant="primary" disabled={!isValid || pending}>
            {t('common.connect')}
          </Button>
        </div>
      </form>
    </Dialog>
  )
}
