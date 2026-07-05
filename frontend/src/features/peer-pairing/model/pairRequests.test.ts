import { beforeEach, describe, expect, it } from 'vitest'
import { usePairRequestStore } from './pairRequests'

function makeRequest(overrides: Partial<{ requestId: string; clientName: string; remoteAddr: string }> = {}) {
  return {
    requestId: overrides.requestId ?? 'req-1',
    clientName: overrides.clientName ?? 'kim-laptop',
    remoteAddr: overrides.remoteAddr ?? '10.0.1.9:52344',
  }
}

beforeEach(() => {
  usePairRequestStore.setState({ requests: {} })
})

describe('setRequest/clearRequest', () => {
  it('adds and removes a request by id', () => {
    const req = makeRequest()
    usePairRequestStore.getState().setRequest(req.requestId, req)
    expect(usePairRequestStore.getState().requests[req.requestId]).toEqual(req)

    usePairRequestStore.getState().clearRequest(req.requestId)
    expect(usePairRequestStore.getState().requests[req.requestId]).toBeUndefined()
  })

  it('keeps distinct requests separate', () => {
    usePairRequestStore.getState().setRequest('a', makeRequest({ requestId: 'a' }))
    usePairRequestStore.getState().setRequest('b', makeRequest({ requestId: 'b' }))
    expect(Object.keys(usePairRequestStore.getState().requests)).toHaveLength(2)

    usePairRequestStore.getState().clearRequest('a')
    expect(Object.keys(usePairRequestStore.getState().requests)).toEqual(['b'])
  })
})
