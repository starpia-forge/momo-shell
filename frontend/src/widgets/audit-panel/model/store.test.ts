import { beforeEach, describe, expect, it } from 'vitest'
import { useAuditPanelStore } from './store'
import type { AuditEvent } from '../../../shared/api/audit'

function makeEvent(overrides: Partial<AuditEvent> = {}): AuditEvent {
  return {
    id: 1,
    timestamp: 1000,
    clientId: 'c1',
    sessionId: 's1',
    kind: 'command',
    target: '',
    originalCmd: 'ls',
    guardedCmd: 'ls',
    risk: 'low',
    uncertain: false,
    reasons: [],
    approver: '',
    decision: 'auto',
    ...overrides,
  }
}

beforeEach(() => {
  useAuditPanelStore.setState({ events: [], filterText: '', clientFilter: '', decisionFilter: '', replaySessionId: null })
})

describe('setEvents', () => {
  it('replaces the event list wholesale', () => {
    useAuditPanelStore.getState().setEvents([makeEvent()])
    expect(useAuditPanelStore.getState().events).toEqual([makeEvent()])

    useAuditPanelStore.getState().setEvents([])
    expect(useAuditPanelStore.getState().events).toEqual([])
  })
})

describe('filter setters', () => {
  it('setFilterText/setClientFilter/setDecisionFilter update independently', () => {
    useAuditPanelStore.getState().setFilterText('rm -rf')
    useAuditPanelStore.getState().setClientFilter('c1')
    useAuditPanelStore.getState().setDecisionFilter('rejected')

    expect(useAuditPanelStore.getState()).toMatchObject({
      filterText: 'rm -rf',
      clientFilter: 'c1',
      decisionFilter: 'rejected',
    })
  })
})

describe('openReplay / closeReplay', () => {
  it('openReplay sets the replay session id', () => {
    useAuditPanelStore.getState().openReplay('s1')
    expect(useAuditPanelStore.getState().replaySessionId).toBe('s1')
  })

  it('closeReplay clears it back to null', () => {
    useAuditPanelStore.getState().openReplay('s1')
    useAuditPanelStore.getState().closeReplay()
    expect(useAuditPanelStore.getState().replaySessionId).toBeNull()
  })

  it('openReplay on a new session overwrites the previous one', () => {
    useAuditPanelStore.getState().openReplay('s1')
    useAuditPanelStore.getState().openReplay('s2')
    expect(useAuditPanelStore.getState().replaySessionId).toBe('s2')
  })
})
