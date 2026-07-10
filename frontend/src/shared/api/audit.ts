import { LoadOutput, Query } from '../../../wailsjs/go/wails/AuditService'

export type AuditKind = 'connect' | 'command' | 'control'

// AuditEvent is E5's frontend-only audit-panel read model -- never reached
// through the MCP surface (AuditService isn't part of mcpipc/dispatch.go).
export interface AuditEvent {
  id: number
  timestamp: number
  clientId: string
  sessionId: string
  kind: AuditKind
  target: string
  originalCmd: string
  guardedCmd: string
  risk: string
  uncertain: boolean
  reasons: string[]
  approver: string
  decision: string
}

// AuditOutputSegment is one run of a command's E3-b captured original
// output, split at secret-hit boundaries (E5b). A non-redacted run carries
// only text; a redacted run carries type and the original value for the
// replay view's per-item unmask.
export interface AuditOutputSegment {
  text?: string
  redacted: boolean
  type?: string
  value?: string
}

function asAuditEvent(dto: Awaited<ReturnType<typeof Query>>[number]): AuditEvent {
  return {
    id: dto.id,
    timestamp: dto.timestamp,
    clientId: dto.clientId,
    sessionId: dto.sessionId,
    kind: dto.kind as AuditKind,
    target: dto.target ?? '',
    originalCmd: dto.originalCmd ?? '',
    guardedCmd: dto.guardedCmd ?? '',
    risk: dto.risk ?? '',
    uncertain: dto.uncertain ?? false,
    reasons: dto.reasons ?? [],
    approver: dto.approver ?? '',
    decision: dto.decision,
  }
}

/** Returns sessionId's audit trail in replay order (id-ascending), or every
 * session's events if sessionId is ''. */
export async function queryAudit(sessionId: string): Promise<AuditEvent[]> {
  const events = await Query(sessionId)
  return (events ?? []).map(asAuditEvent)
}

/** Loads auditId's captured original output, masked by default and split at
 * secret-hit boundaries for the replay view's per-item unmask (E5b). Empty
 * when nothing was ever captured, or retention already expired it. */
export async function loadAuditOutput(auditId: number): Promise<AuditOutputSegment[]> {
  const segments = await LoadOutput(auditId)
  return segments ?? []
}
