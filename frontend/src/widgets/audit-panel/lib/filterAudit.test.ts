import { describe, expect, it } from 'vitest'
import { filterEvents } from './filterAudit'
import type { AuditEvent } from '../../../shared/api/audit'

function makeEvent(overrides: Partial<AuditEvent> = {}): AuditEvent {
  return {
    id: 1,
    timestamp: 1000,
    clientId: 'c1',
    sessionId: 's1',
    kind: 'command',
    target: '',
    originalCmd: 'ls -la',
    guardedCmd: 'ls -la',
    risk: 'low',
    uncertain: false,
    reasons: [],
    approver: '',
    decision: 'auto',
    ...overrides,
  }
}

const NO_FILTER = { text: '', client: '', decision: '' }

describe('filterEvents', () => {
  it('returns every event when the filter is empty', () => {
    const events = [makeEvent({ id: 1 }), makeEvent({ id: 2, clientId: 'c2' })]
    expect(filterEvents(events, NO_FILTER)).toEqual(events)
  })

  it('filters by exact clientId match', () => {
    const events = [makeEvent({ id: 1, clientId: 'c1' }), makeEvent({ id: 2, clientId: 'c2' })]
    expect(filterEvents(events, { ...NO_FILTER, client: 'c2' })).toEqual([events[1]])
  })

  it('filters by exact decision match', () => {
    const events = [makeEvent({ id: 1, decision: 'auto' }), makeEvent({ id: 2, decision: 'rejected' })]
    expect(filterEvents(events, { ...NO_FILTER, decision: 'rejected' })).toEqual([events[1]])
  })

  it('filters text case-insensitively against originalCmd', () => {
    const events = [makeEvent({ id: 1, originalCmd: 'rm -rf /tmp' }), makeEvent({ id: 2, originalCmd: 'ls -la' })]
    expect(filterEvents(events, { ...NO_FILTER, text: 'RM -RF' })).toEqual([events[0]])
  })

  it('filters text against guardedCmd when it differs from originalCmd', () => {
    const events = [makeEvent({ id: 1, originalCmd: 'rm x', guardedCmd: 'rm -i x' })]
    expect(filterEvents(events, { ...NO_FILTER, text: '-i' })).toEqual(events)
  })

  it('filters text against target (e.g. a connect event\'s host name)', () => {
    const events = [makeEvent({ id: 1, kind: 'connect', originalCmd: '', target: 'web-01' })]
    expect(filterEvents(events, { ...NO_FILTER, text: 'web-01' })).toEqual(events)
  })

  it('combines client, decision, and text filters (AND semantics)', () => {
    const match = makeEvent({ id: 1, clientId: 'c1', decision: 'auto', originalCmd: 'ls -la' })
    const wrongClient = makeEvent({ id: 2, clientId: 'c2', decision: 'auto', originalCmd: 'ls -la' })
    const wrongDecision = makeEvent({ id: 3, clientId: 'c1', decision: 'rejected', originalCmd: 'ls -la' })
    const events = [match, wrongClient, wrongDecision]

    expect(filterEvents(events, { text: 'ls', client: 'c1', decision: 'auto' })).toEqual([match])
  })

  it('returns an empty array when nothing matches', () => {
    const events = [makeEvent()]
    expect(filterEvents(events, { ...NO_FILTER, text: 'nonexistent' })).toEqual([])
  })
})
