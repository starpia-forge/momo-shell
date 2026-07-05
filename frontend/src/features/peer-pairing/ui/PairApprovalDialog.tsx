import { useEffect } from 'react'
import { Dialog, Button } from '../../../shared/ui'
import { respondPairing } from '../../../shared/api/share'
import {
  subscribe,
  topics,
  type SharePairRequestPayload,
  type SharePairRequestResolvedPayload,
} from '../../../shared/api/events'
import { usePairRequestStore } from '../model/pairRequests'
import './PairApprovalDialog.css'

// Mounted once (see pages/workspace), same one-at-a-time shape as
// HostKeyPrompt -- concurrent pairing requests are rare enough that
// queuing them isn't worth the extra state.
export function PairApprovalDialog() {
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
    <Dialog open onClose={() => respond(false)} title="공유 연결 요청">
      <div className="pair-approval">
        <p>
          <strong>{payload.clientName}</strong>({payload.remoteAddr})에서 호스트 공유 연결을 요청했습니다.
        </p>
        <div className="pair-approval__actions">
          <Button variant="primary" onClick={() => respond(true)}>
            허용
          </Button>
          <Button onClick={() => respond(false)}>거부</Button>
        </div>
      </div>
    </Dialog>
  )
}
