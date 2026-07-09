import { beforeEach, describe, expect, it } from 'vitest'
import { useDelegationStore } from './delegations'
import type { Delegation } from '../../../shared/api/mcp'

function makeDelegation(overrides: Partial<Delegation> = {}): Delegation {
  return { sessionId: 's1', clientId: 'c1', aiCreated: false, grantedAt: 1000, ...overrides }
}

beforeEach(() => {
  useDelegationStore.setState({ delegations: {}, open: false })
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
