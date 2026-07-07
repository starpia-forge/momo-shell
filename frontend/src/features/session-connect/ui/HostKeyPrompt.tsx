import { Dialog, Button } from '../../../shared/ui'
import { useHostKeyPromptStore } from '../../../entities/session'
import { respondHostKey } from '../../../shared/api/session'

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
    <Dialog open onClose={() => respond('cancel')} title="처음 연결하는 호스트입니다">
      <div className="flex flex-col gap-4 w-110">
        <p className="m-0 text-[13px] text-fg2 leading-relaxed">
          <strong className="text-fg">
            {payload.address}:{payload.port}
          </strong>{' '}
          ({payload.algo})의 신원을 확인할 수 없습니다. 아래 핑거프린트가 서버 관리자에게 받은 값과 일치하는지 확인하세요.
        </p>
        <p className="m-0 font-mono text-[12px] px-4 py-3.5 bg-inputbg border border-line rounded-md break-all text-fg2 leading-relaxed">
          {payload.fingerprint}
        </p>
        <div className="flex justify-end gap-2.5">
          <Button onClick={() => respond('cancel')}>취소</Button>
          <Button onClick={() => respond('once')}>이번만</Button>
          <Button variant="primary" onClick={() => respond('trust')}>
            신뢰하고 연결
          </Button>
        </div>
      </div>
    </Dialog>
  )
}
