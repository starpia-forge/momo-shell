import { useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { Dialog, Button } from '../../../shared/ui'
import { respondPairing } from '../../../shared/api/share'
import {
  subscribe,
  topics,
  type SharePairRequestPayload,
  type SharePairRequestResolvedPayload,
} from '../../../shared/api/events'
import { usePairRequestStore } from '../model/pairRequests'

// Mounted once (see pages/workspace), same one-at-a-time shape as
// HostKeyPrompt -- concurrent pairing requests are rare enough that
// queuing them isn't worth the extra state.
export function PairApprovalDialog() {
  const { t } = useTranslation()
  useEffect(() => {
    return subscribe<SharePairRequestPayload>(topics.sharePairRequest(), (payload) => {
      usePairRequestStore.getState().setRequest(payload.requestId, payload)
    })
  }, [])

  // The backend resolves a request (answered, timed out, or the requester
  // disconnected) independently of whatever this dialog does -- without
  // this, a request that times out server-side while still showing here
  // would never clear, since respond() below only clears on its own call.
  useEffect(() => {
    return subscribe<SharePairRequestResolvedPayload>(topics.sharePairRequestResolved(), (payload) => {
      usePairRequestStore.getState().clearRequest(payload.requestId)
    })
  }, [])

  const requests = usePairRequestStore((s) => s.requests)
  const entry = Object.entries(requests)[0]
  if (!entry) return null
  const [requestId, payload] = entry

  async function respond(approve: boolean) {
    try {
      await respondPairing(requestId, approve)
    } finally {
      usePairRequestStore.getState().clearRequest(requestId)
    }
  }

  return (
    <Dialog open onClose={() => respond(false)} title={t('peerPairing.requestTitle')}>
      <div className="flex flex-col gap-4 w-100">
        <p className="m-0 text-[13px] text-fg2 leading-relaxed">
          <strong className="text-fg">{payload.clientName}</strong>
          {t('peerPairing.requestBody', { address: payload.remoteAddr })}
        </p>
        <div className="flex justify-end gap-2.5">
          <Button onClick={() => respond(false)}>{t('peerPairing.reject')}</Button>
          <Button variant="primary" onClick={() => respond(true)}>
            {t('peerPairing.approve')}
          </Button>
        </div>
      </div>
    </Dialog>
  )
}
