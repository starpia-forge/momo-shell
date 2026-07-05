import { useEffect } from 'react'
import { Dialog, Button } from '../../../shared/ui'
import { respondPairing } from '../../../shared/api/share'
import { subscribe, topics, type SharePairRequestPayload } from '../../../shared/api/events'
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

  const requests = usePairRequestStore((s) => s.requests)
  const entry = Object.entries(requests)[0]
  if (!entry) return null
  const [requestId, payload] = entry

  async function respond(approve: boolean) {
    await respondPairing(requestId, approve)
    usePairRequestStore.getState().clearRequest(requestId)
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
