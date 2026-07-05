import { beforeEach, describe, expect, it } from 'vitest'
import type { ShareClient, ShareStatus } from '../../../shared/api/share'
import { useSharePanelStore } from './store'

function makeStatus(overrides: Partial<ShareStatus> = {}): ShareStatus {
  return {
    enabled: false,
    pin: '',
    port: 0,
    instanceName: 'Starpia-PC',
    sharedHostIds: [],
    ...overrides,
  }
}

function makeClient(overrides: Partial<ShareClient> = {}): ShareClient {
  return {
    id: 'c1',
    name: 'kim-laptop',
    pairedAt: 0,
    ...overrides,
  }
}

beforeEach(() => {
  useSharePanelStore.setState({ status: null, clients: [] })
})

describe('setStatus', () => {
  it('overwrites the status wholesale', () => {
    useSharePanelStore.getState().setStatus(makeStatus({ enabled: true, pin: '482917' }))
    expect(useSharePanelStore.getState().status).toEqual(makeStatus({ enabled: true, pin: '482917' }))
  })
})

describe('setClients/removeClient', () => {
  it('sets the client list and removes one by id', () => {
    useSharePanelStore.getState().setClients([makeClient({ id: 'c1' }), makeClient({ id: 'c2' })])
    expect(useSharePanelStore.getState().clients).toHaveLength(2)

    useSharePanelStore.getState().removeClient('c1')
    const remaining = useSharePanelStore.getState().clients
    expect(remaining).toHaveLength(1)
    expect(remaining[0].id).toBe('c2')
  })

  it('is a no-op when removing an unknown id', () => {
    useSharePanelStore.getState().setClients([makeClient({ id: 'c1' })])
    useSharePanelStore.getState().removeClient('nonexistent')
    expect(useSharePanelStore.getState().clients).toHaveLength(1)
  })
})
