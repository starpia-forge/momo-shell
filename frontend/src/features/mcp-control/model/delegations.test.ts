import { beforeEach, describe, expect, it } from 'vitest'
import { useDelegationStore } from './delegations'
import type { ConnectionScope, Delegation } from '../../../shared/api/mcp'
import type { MCPCommandStatePayload } from '../../../shared/api/events'

function makeDelegation(overrides: Partial<Delegation> = {}): Delegation {
  return { sessionId: 's1', clientId: 'c1', aiCreated: false, grantedAt: 1000, ...overrides }
}

function makeCmdState(overrides: Partial<MCPCommandStatePayload> = {}): MCPCommandStatePayload {
  return { sessionId: 's1', seq: 1, state: 'running', ...overrides }
}

function makeScope(overrides: Partial<ConnectionScope> = {}): ConnectionScope {
  return { scopeId: 'scope-1', clientId: 'c1', hostNames: ['web-1'], maxConcurrent: 1, activeCount: 0, ...overrides }
}

beforeEach(() => {
  useDelegationStore.setState({ delegations: {}, cmdStates: {}, scopes: [], open: false })
})

describe('setDelegation / removeDelegation', () => {
  it('adds a delegation keyed by sessionId', () => {
    useDelegationStore.getState().setDelegation(makeDelegation())
    expect(useDelegationStore.getState().delegations).toEqual({ s1: makeDelegation() })
  })

  it('removeDelegation removes only the matching entry', () => {
    const other = makeDelegation({ sessionId: 's2' })
    useDelegationStore.getState().setDelegation(makeDelegation())
    useDelegationStore.getState().setDelegation(other)

    useDelegationStore.getState().removeDelegation('s1')

    expect(useDelegationStore.getState().delegations).toEqual({ s2: other })
  })

  it('removeDelegation on an unknown sessionId is a no-op', () => {
    useDelegationStore.getState().setDelegation(makeDelegation())
    useDelegationStore.getState().removeDelegation('does-not-exist')
    expect(useDelegationStore.getState().delegations).toEqual({ s1: makeDelegation() })
  })

  it('removeDelegation also clears that session\'s cmdState', () => {
    useDelegationStore.getState().setDelegation(makeDelegation())
    useDelegationStore.getState().setCmdState(makeCmdState({ sessionId: 's1' }))
    useDelegationStore.getState().setCmdState(makeCmdState({ sessionId: 's2' }))

    useDelegationStore.getState().removeDelegation('s1')

    expect(Object.keys(useDelegationStore.getState().cmdStates)).toEqual(['s2'])
  })
})

describe('replaceAll', () => {
  it('replaces the entire map, keyed by sessionId', () => {
    useDelegationStore.getState().setDelegation(makeDelegation({ sessionId: 'stale' }))

    useDelegationStore.getState().replaceAll([makeDelegation({ sessionId: 's1' }), makeDelegation({ sessionId: 's2' })])

    expect(Object.keys(useDelegationStore.getState().delegations).sort()).toEqual(['s1', 's2'])
  })

  it('replaces with an empty map when given no delegations', () => {
    useDelegationStore.getState().setDelegation(makeDelegation())
    useDelegationStore.getState().replaceAll([])
    expect(useDelegationStore.getState().delegations).toEqual({})
  })
})

describe('toggleOpen / close', () => {
  it('toggleOpen flips the open flag', () => {
    expect(useDelegationStore.getState().open).toBe(false)
    useDelegationStore.getState().toggleOpen()
    expect(useDelegationStore.getState().open).toBe(true)
    useDelegationStore.getState().toggleOpen()
    expect(useDelegationStore.getState().open).toBe(false)
  })

  it('close sets open to false unconditionally', () => {
    useDelegationStore.setState({ open: true })
    useDelegationStore.getState().close()
    expect(useDelegationStore.getState().open).toBe(false)
  })
})

describe('setCmdState', () => {
  it('adds a cmd-state keyed by sessionId', () => {
    useDelegationStore.getState().setCmdState(makeCmdState())
    expect(useDelegationStore.getState().cmdStates).toEqual({ s1: makeCmdState() })
  })

  it('upserts -- a later event for the same session replaces the prior state', () => {
    useDelegationStore.getState().setCmdState(makeCmdState({ state: 'running', seq: 1 }))
    useDelegationStore.getState().setCmdState(makeCmdState({ state: 'done', seq: 2, exitCode: 0 }))

    expect(useDelegationStore.getState().cmdStates.s1).toEqual({ sessionId: 's1', seq: 2, state: 'done', exitCode: 0 })
  })

  it('tracks distinct sessions independently', () => {
    useDelegationStore.getState().setCmdState(makeCmdState({ sessionId: 's1' }))
    useDelegationStore.getState().setCmdState(makeCmdState({ sessionId: 's2', state: 'tui' }))

    expect(Object.keys(useDelegationStore.getState().cmdStates).sort()).toEqual(['s1', 's2'])
  })
})

describe('setScopes', () => {
  it('replaces the scope list wholesale', () => {
    useDelegationStore.getState().setScopes([makeScope()])
    expect(useDelegationStore.getState().scopes).toEqual([makeScope()])

    useDelegationStore.getState().setScopes([])
    expect(useDelegationStore.getState().scopes).toEqual([])
  })
})
