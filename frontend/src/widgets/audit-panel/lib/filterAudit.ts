import type { AuditEvent } from '../../../shared/api/audit'

export interface AuditFilter {
  text: string
  /** '' = every client. */
  client: string
  /** '' = every decision. */
  decision: string
}

/** Pure filter over the loaded audit timeline -- kept separate from the
 * store/component so the match logic is unit-testable without a DOM (this
 * frontend has no component-render test infra). Text matches the original
 * command, guarded command, and target (e.g. a connect event's host name). */
export function filterEvents(events: AuditEvent[], filter: AuditFilter): AuditEvent[] {
  const needle = filter.text.toLowerCase()
  return events.filter((e) => {
    if (filter.client && e.clientId !== filter.client) return false
    if (filter.decision && e.decision !== filter.decision) return false
    if (needle && !`${e.originalCmd} ${e.guardedCmd} ${e.target}`.toLowerCase().includes(needle)) return false
    return true
  })
}
