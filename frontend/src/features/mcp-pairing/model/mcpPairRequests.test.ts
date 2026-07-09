import { beforeEach, describe, expect, it } from 'vitest'
import { useMCPPairRequestStore } from './mcpPairRequests'

function makeRequest(overrides: Partial<{ requestId: string; clientName: string }> = {}) {
  return {
    requestId: overrides.requestId ?? 'req-1',
    clientName: overrides.clientName ?? 'claude-desktop',
  }
}

beforeEach(() => {
  useMCPPairRequestStore.setState({ requests: {} })
})

describe('setRequest/clearRequest', () => {
  it('adds and removes a request by id', () => {
    const req = makeRequest()
    useMCPPairRequestStore.getState().setRequest(req.requestId, req)
    expect(useMCPPairRequestStore.getState().requests[req.requestId]).toEqual(req)

    useMCPPairRequestStore.getState().clearRequest(req.requestId)
    expect(useMCPPairRequestStore.getState().requests[req.requestId]).toBeUndefined()
  })

  it('keeps distinct requests separate', () => {
    useMCPPairRequestStore.getState().setRequest('a', makeRequest({ requestId: 'a' }))
    useMCPPairRequestStore.getState().setRequest('b', makeRequest({ requestId: 'b' }))
    expect(Object.keys(useMCPPairRequestStore.getState().requests)).toHaveLength(2)

    useMCPPairRequestStore.getState().clearRequest('a')
    expect(Object.keys(useMCPPairRequestStore.getState().requests)).toEqual(['b'])
  })
})
