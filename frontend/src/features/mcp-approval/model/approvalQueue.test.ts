import { beforeEach, describe, expect, it, vi } from 'vitest'
import { scheduleApprovalTTL, useApprovalQueueStore, type ApprovalRequest } from './approvalQueue'

const CONNECT: ApprovalRequest = { kind: 'connect', payload: { requestId: 'r1', clientId: 'c1', hostName: 'web-1' } }

beforeEach(() => {
  useApprovalQueueStore.setState({ requests: {} })
  vi.useRealTimers()
})

describe('setRequest / clearRequest', () => {
  it('adds a request keyed by requestId', () => {
    useApprovalQueueStore.getState().setRequest('r1', CONNECT)
    expect(useApprovalQueueStore.getState().requests).toEqual({ r1: CONNECT })
  })

  it('clearRequest removes only the matching entry', () => {
    const other: ApprovalRequest = { kind: 'control', payload: { requestId: 'r2', clientId: 'c1', sessionId: 's1' } }
    useApprovalQueueStore.getState().setRequest('r1', CONNECT)
    useApprovalQueueStore.getState().setRequest('r2', other)

    useApprovalQueueStore.getState().clearRequest('r1')

    expect(useApprovalQueueStore.getState().requests).toEqual({ r2: other })
  })

  it('clearRequest on an unknown id is a no-op', () => {
    useApprovalQueueStore.getState().setRequest('r1', CONNECT)
    useApprovalQueueStore.getState().clearRequest('does-not-exist')
    expect(useApprovalQueueStore.getState().requests).toEqual({ r1: CONNECT })
  })
})

describe('scheduleApprovalTTL', () => {
  it('clears the request after the TTL elapses', () => {
    vi.useFakeTimers()
    useApprovalQueueStore.getState().setRequest('r1', CONNECT)

    scheduleApprovalTTL('r1')
    vi.advanceTimersByTime(65_000)

    expect(useApprovalQueueStore.getState().requests).toEqual({})
    vi.useRealTimers()
  })

  it('does not clear before the TTL elapses', () => {
    vi.useFakeTimers()
    useApprovalQueueStore.getState().setRequest('r1', CONNECT)

    scheduleApprovalTTL('r1')
    vi.advanceTimersByTime(64_000)

    expect(useApprovalQueueStore.getState().requests).toEqual({ r1: CONNECT })
    vi.useRealTimers()
  })
})
