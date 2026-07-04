import { Dialog, Button } from '../../../shared/ui'
import { useHostKeyPromptStore } from '../../../entities/session'
import { respondHostKey } from '../../../shared/api/session'
import './HostKeyPrompt.css'

// Mounted once (see pages/workspace). Renders at most one prompt at a time
// -- concurrent unrecognized host keys are rare enough that queuing them
// isn't worth the extra state.
export function HostKeyPrompt() {
  const prompts = useHostKeyPromptStore((s) => s.prompts)
  const clearPrompt = useHostKeyPromptStore((s) => s.clearPrompt)

  const entry = Object.entries(prompts)[0]
  if (!entry) return null
  const [sessionId, payload] = entry

  async function respond(decision: 'trust' | 'once' | 'cancel') {
    await respondHostKey(sessionId, decision)
    clearPrompt(sessionId)
  }

  return (
    <Dialog open onClose={() => respond('cancel')} title="호스트 키 확인">
      <div className="host-key-prompt">
        <p>
          <strong>
            {payload.address}:{payload.port}
          </strong>{' '}
          ({payload.algo})의 키가 등록되어 있지 않습니다.
        </p>
        <p className="host-key-prompt__fingerprint">{payload.fingerprint}</p>
        <div className="host-key-prompt__actions">
          <Button variant="primary" onClick={() => respond('trust')}>
            신뢰하고 저장
          </Button>
          <Button onClick={() => respond('once')}>이번만</Button>
          <Button onClick={() => respond('cancel')}>취소</Button>
        </div>
      </div>
    </Dialog>
  )
}
