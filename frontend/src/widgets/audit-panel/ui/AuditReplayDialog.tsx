import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Chip, Dialog } from '../../../shared/ui'
import { loadAuditOutput, queryAudit, type AuditEvent, type AuditOutputSegment } from '../../../shared/api/audit'
import { formatRelativeTime } from '../lib/formatRelativeTime'
import { useAuditPanelStore } from '../model/store'

const DECISION_TONE = (decision: string): 'green' | 'neutral' | 'red' =>
  decision === 'rejected' || decision === 'timeout' || decision === 'killed' ? 'red' : decision === 'auto' ? 'neutral' : 'green'

/** E5's replay view: the id-ASC decision trail for one session (US-5), plus
 * each command's E3-b captured original output loaded lazily and rendered
 * masked-by-default with per-item unmask (E5b). Mounted once alongside
 * AuditPanel; opens itself when useAuditPanelStore.replaySessionId is set. */
export function AuditReplayDialog() {
  const { t } = useTranslation()
  const sessionId = useAuditPanelStore((s) => s.replaySessionId)
  const [trail, setTrail] = useState<AuditEvent[]>([])
  const [outputs, setOutputs] = useState<Record<number, AuditOutputSegment[]>>({})
  const [revealed, setRevealed] = useState<Set<string>>(new Set())

  useEffect(() => {
    setOutputs({})
    setRevealed(new Set())
    if (!sessionId) {
      setTrail([])
      return
    }
    let cancelled = false
    void queryAudit(sessionId).then((events) => {
      if (!cancelled) setTrail(events)
    })
    return () => {
      cancelled = true
    }
  }, [sessionId])

  function handleLoadOutput(auditId: number) {
    if (outputs[auditId]) return
    void loadAuditOutput(auditId).then((segments) => {
      setOutputs((prev) => ({ ...prev, [auditId]: segments }))
    })
  }

  function toggleReveal(key: string) {
    setRevealed((prev) => {
      const next = new Set(prev)
      if (next.has(key)) next.delete(key)
      else next.add(key)
      return next
    })
  }

  return (
    <Dialog open={sessionId !== null} onClose={() => useAuditPanelStore.getState().closeReplay()} title={t('auditPanel.replayTitle')}>
      <div className="flex flex-col gap-2.5 w-[76vw] h-[76vh] overflow-y-auto">
        {trail.map((event) => (
          <div key={event.id} className="flex flex-col gap-1.5 px-2.5 py-2 rounded-md bg-inputbg">
            <div className="flex items-center gap-1.5">
              <Chip>{t(`auditPanel.kind_${event.kind}`, { defaultValue: event.kind })}</Chip>
              <Chip tone={DECISION_TONE(event.decision)}>{event.decision}</Chip>
              {event.risk && <span className="text-[11px] text-fg3">{event.risk}</span>}
              <span className="ml-auto flex-none text-[10.5px] text-fg3">{formatRelativeTime(event.timestamp)}</span>
            </div>

            {event.target && <div className="text-[12px] font-mono text-fg2">{event.target}</div>}

            {event.originalCmd && (
              <code className="block px-2.5 py-2 rounded-md bg-surface2 text-[12px] font-mono whitespace-pre-wrap break-all">
                {event.originalCmd}
              </code>
            )}
            {event.guardedCmd && event.guardedCmd !== event.originalCmd && (
              <div className="text-[11px] text-fg3">
                <div>{t('auditPanel.guardedCmd')}</div>
                <code className="block px-2.5 py-2 rounded-md bg-surface2 text-[12px] font-mono whitespace-pre-wrap break-all">
                  {event.guardedCmd}
                </code>
              </div>
            )}
            {event.reasons.length > 0 && (
              <ul className="m-0 pl-4.5 text-[12px] text-fg2 leading-relaxed">
                {event.reasons.map((reason) => (
                  <li key={reason}>{reason}</li>
                ))}
              </ul>
            )}

            <div className="text-[10.5px] text-fg3">
              {event.approver ? t('auditPanel.approvedBy', { approver: event.approver }) : t('auditPanel.autoRun')}
            </div>

            {event.kind === 'command' &&
              (!outputs[event.id] ? (
                <button
                  className="self-start border-none bg-transparent text-accent-text cursor-pointer text-[11px] px-0 py-1"
                  onClick={() => handleLoadOutput(event.id)}
                >
                  {t('auditPanel.loadOutput')}
                </button>
              ) : outputs[event.id].length === 0 ? (
                <div className="text-[11px] text-fg3">{t('auditPanel.noOutput')}</div>
              ) : (
                <pre className="m-0 max-h-70 overflow-auto px-2.5 py-2 rounded-md bg-surface2 text-[11.5px] font-mono whitespace-pre-wrap break-all">
                  {outputs[event.id].map((seg, i) => {
                    const key = `${event.id}:${i}`
                    if (!seg.redacted) return <span key={key}>{seg.text}</span>
                    return (
                      <span
                        key={key}
                        className="cursor-pointer rounded-sm bg-red/14 text-red px-1"
                        onClick={() => toggleReveal(key)}
                        title={t('auditPanel.unmaskHint')}
                      >
                        {revealed.has(key) ? seg.value : `[REDACTED:${seg.type}]`}
                      </span>
                    )
                  })}
                </pre>
              ))}
          </div>
        ))}
        {sessionId && trail.length === 0 && <div className="px-2.5 py-2 text-[12px] text-fg3">{t('auditPanel.empty')}</div>}
      </div>
    </Dialog>
  )
}
