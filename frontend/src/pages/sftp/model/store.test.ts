import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Host } from '../../../entities/host'

const paneOpsMocks = vi.hoisted(() => ({
  localOps: {
    side: 'local' as const,
    list: vi.fn(),
    homeDir: vi.fn(),
    join: vi.fn(),
    parent: vi.fn(),
    baseName: vi.fn(),
    mkdir: vi.fn(),
    rename: vi.fn(),
    remove: vi.fn(),
    stat: vi.fn(),
    copyWithin: vi.fn(),
    moveWithin: vi.fn(),
  },
  makeRemoteOps: vi.fn(),
}))
vi.mock('./paneOps', () => paneOpsMocks)

const sessionConnectMocks = vi.hoisted(() => ({ connectHost: vi.fn() }))
vi.mock('../../../features/session-connect', () => sessionConnectMocks)

const sessionMocks = vi.hoisted(() => ({ disposeSession: vi.fn() }))
vi.mock('../../../entities/session', () => sessionMocks)

const eventsMocks = vi.hoisted(() => ({
  subscribe: vi.fn(),
  topics: { sessionState: (id: string) => `session:state:${id}` },
}))
vi.mock('../../../shared/api/events', () => eventsMocks)

const { useSftpStore } = await import('./store')

function fakeRemoteOps(overrides: Record<string, unknown> = {}) {
  return {
    side: 'remote' as const,
    list: vi.fn().mockResolvedValue([]),
    homeDir: vi.fn().mockResolvedValue('/home'),
    join: vi.fn(),
    parent: vi.fn(),
    baseName: vi.fn(),
    mkdir: vi.fn(),
    rename: vi.fn(),
    remove: vi.fn(),
    stat: vi.fn(),
    copyWithin: vi.fn(),
    moveWithin: vi.fn(),
    ...overrides,
  }
}

const EMPTY_PANE = { path: null, entries: [], loading: false, error: null, selected: null }
const HOST: Host = {
  id: 'h1',
  name: 'web',
  address: '10.0.0.5',
  port: 22,
  labels: [],
  username: 'u',
  authType: 'password',
  source: 'local',
  createdAt: 0,
  updatedAt: 0,
}

beforeEach(() => {
  vi.resetAllMocks()
  useSftpStore.setState({
    sessionId: null,
    remoteOps: null,
    hostLabel: '',
    connecting: false,
    gateError: null,
    local: EMPTY_PANE,
    remote: EMPTY_PANE,
    clipboard: null,
  })
})

describe('connect', () => {
  it('sets sessionId and populates the remote pane at home on success', async () => {
    sessionConnectMocks.connectHost.mockResolvedValue('s1')
    eventsMocks.subscribe.mockReturnValue(vi.fn())
    const entries = [{ name: 'a', path: '/home/a', size: 0, mode: 0, modeText: '', modTime: 0, isDir: false }]
    paneOpsMocks.makeRemoteOps.mockReturnValue(fakeRemoteOps({ homeDir: vi.fn().mockResolvedValue('/home'), list: vi.fn().mockResolvedValue(entries) }))

    await useSftpStore.getState().connect(HOST)

    const state = useSftpStore.getState()
    expect(state.sessionId).toBe('s1')
    expect(state.gateError).toBeNull()
    expect(state.connecting).toBe(false)
    expect(state.remote.path).toBe('/home')
    expect(state.remote.entries).toEqual(entries)
    expect(state.hostLabel).toBe('web (10.0.0.5)')
  })

  it('sets gateError and leaves sessionId null on failure', async () => {
    sessionConnectMocks.connectHost.mockRejectedValue(new Error('boom'))

    await useSftpStore.getState().connect(HOST)

    const state = useSftpStore.getState()
    expect(state.sessionId).toBeNull()
    expect(state.gateError).toBe('boom')
    expect(state.connecting).toBe(false)
  })
})

describe('session loss', () => {
  it('a closed session:state event resets the remote pane, sets gateError, and unsubscribes', async () => {
    sessionConnectMocks.connectHost.mockResolvedValue('s1')
    let capturedCb: ((p: { state: string; error?: string }) => void) | undefined
    const unsub = vi.fn()
    eventsMocks.subscribe.mockImplementation((_topic: string, cb: (p: { state: string; error?: string }) => void) => {
      capturedCb = cb
      return unsub
    })
    paneOpsMocks.makeRemoteOps.mockReturnValue(fakeRemoteOps())

    await useSftpStore.getState().connect(HOST)
    expect(useSftpStore.getState().sessionId).toBe('s1')

    capturedCb?.({ state: 'closed' })

    const state = useSftpStore.getState()
    expect(state.sessionId).toBeNull()
    expect(state.remoteOps).toBeNull()
    expect(state.remote.path).toBeNull()
    expect(state.gateError).toBe('연결이 끊어졌습니다')
    expect(unsub).toHaveBeenCalled()
  })
})

describe('disconnect', () => {
  async function connectFirst() {
    sessionConnectMocks.connectHost.mockResolvedValue('s1')
    eventsMocks.subscribe.mockReturnValue(vi.fn())
    paneOpsMocks.makeRemoteOps.mockReturnValue(fakeRemoteOps())
    await useSftpStore.getState().connect(HOST)
  }

  it('disposes the session, unsubscribes, and clears the remote pane', async () => {
    const unsub = vi.fn()
    eventsMocks.subscribe.mockReturnValue(unsub)
    sessionConnectMocks.connectHost.mockResolvedValue('s1')
    paneOpsMocks.makeRemoteOps.mockReturnValue(fakeRemoteOps())
    await useSftpStore.getState().connect(HOST)

    useSftpStore.getState().disconnect()

    expect(sessionMocks.disposeSession).toHaveBeenCalledWith('s1')
    expect(unsub).toHaveBeenCalled()
    const state = useSftpStore.getState()
    expect(state.sessionId).toBeNull()
    expect(state.remote.path).toBeNull()
  })

  it('drops a remote-side clipboard but keeps a local-side one', async () => {
    await connectFirst()
    useSftpStore.setState({ clipboard: { side: 'remote', op: 'copy', paths: ['/a'] } })
    useSftpStore.getState().disconnect()
    expect(useSftpStore.getState().clipboard).toBeNull()

    await connectFirst()
    useSftpStore.setState({ clipboard: { side: 'local', op: 'copy', paths: ['/a'] } })
    useSftpStore.getState().disconnect()
    expect(useSftpStore.getState().clipboard).toEqual({ side: 'local', op: 'copy', paths: ['/a'] })
  })
})

describe('refreshLocal', () => {
  it('sets local.error on failure', async () => {
    paneOpsMocks.localOps.list.mockRejectedValue(new Error('denied'))

    await useSftpStore.getState().refreshLocal('/nope')

    const state = useSftpStore.getState()
    expect(state.local.error).toBe('denied')
    expect(state.local.loading).toBe(false)
  })

  it('populates entries sorted dirs-first on success', async () => {
    paneOpsMocks.localOps.list.mockResolvedValue([
      { name: 'b.txt', path: '/b.txt', size: 1, mode: 0, modeText: '', modTime: 0, isDir: false },
      { name: 'a-dir', path: '/a-dir', size: 0, mode: 0, modeText: '', modTime: 0, isDir: true },
    ])

    await useSftpStore.getState().refreshLocal('/')

    const state = useSftpStore.getState()
    expect(state.local.path).toBe('/')
    expect(state.local.entries.map((e) => e.name)).toEqual(['a-dir', 'b.txt'])
    expect(state.local.error).toBeNull()
  })
})

describe('select / setClipboard', () => {
  it('select updates only the given side', () => {
    useSftpStore.getState().select('local', '/a.txt')
    expect(useSftpStore.getState().local.selected).toBe('/a.txt')
    expect(useSftpStore.getState().remote.selected).toBeNull()
  })

  it('setClipboard stores the clipboard payload', () => {
    useSftpStore.getState().setClipboard({ side: 'local', op: 'move', paths: ['/a.txt'] })
    expect(useSftpStore.getState().clipboard).toEqual({ side: 'local', op: 'move', paths: ['/a.txt'] })
  })
})
