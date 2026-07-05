import { beforeEach, describe, expect, it } from 'vitest'
import type { PeerView } from '../../../shared/api/share'
import { usePeerStore } from './store'

function makePeer(overrides: Partial<PeerView> = {}): PeerView {
  return {
    id: 'peer-1',
    name: 'Starpia-PC',
    address: '10.0.1.5',
    port: 47800,
    paired: true,
    online: true,
    hosts: [],
    ...overrides,
  }
}

beforeEach(() => {
  usePeerStore.setState({ peers: [] })
})

describe('setPeers', () => {
  it('overwrites the peer list wholesale', () => {
    usePeerStore.getState().setPeers([makePeer({ id: 'a' }), makePeer({ id: 'b' })])
    expect(usePeerStore.getState().peers.map((p) => p.id)).toEqual(['a', 'b'])

    usePeerStore.getState().setPeers([makePeer({ id: 'c' })])
    expect(usePeerStore.getState().peers.map((p) => p.id)).toEqual(['c'])
  })
})
