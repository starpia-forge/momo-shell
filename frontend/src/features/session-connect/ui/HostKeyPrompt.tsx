import { useTranslation } from 'react-i18next'
import { Dialog, Button } from '../../../shared/ui'
import { useHostKeyPromptStore } from '../../../entities/session'
import { respondHostKey } from '../../../shared/api/session'

// Mounted once (see pages/workspace). Renders at most one prompt at a time
// -- concurrent unrecognized host keys are rare enough that queuing them
// isn't worth the extra state.
export function HostKeyPrompt() {
  const { t } = useTranslation()
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
    <Dialog open onClose={() => respond('cancel')} title={t('hostKeyPrompt.title')}>
      <div className="flex flex-col gap-4 w-110">
        <p className="m-0 text-[13px] text-fg2 leading-relaxed">
          <strong className="text-fg">
            {payload.address}:{payload.port}
          </strong>
          {t('hostKeyPrompt.body', { algo: payload.algo })}
        </p>
        <p className="m-0 font-mono text-[12px] px-4 py-3.5 bg-inputbg border border-line rounded-md break-all text-fg2 leading-relaxed">
          {payload.fingerprint}
        </p>
        <div className="flex justify-end gap-2.5">
          <Button onClick={() => respond('cancel')}>{t('common.cancel')}</Button>
          <Button onClick={() => respond('once')}>{t('hostKeyPrompt.once')}</Button>
          <Button variant="primary" onClick={() => respond('trust')}>
            {t('hostKeyPrompt.trustAndConnect')}
          </Button>
        </div>
      </div>
    </Dialog>
  )
}
