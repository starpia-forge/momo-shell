import { beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'
import type { Host } from '../../../entities/host'
import i18n from '../../../shared/i18n'

beforeAll(async () => {
  await i18n.changeLanguage('en')
})

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
  topics: { sessionState: (id: string) => `session:state:${id}`, transferTask: () => 'transfer:task' },
}))
vi.mock('../../../shared/api/events', () => eventsMocks)

const localfsMocks = vi.hoisted(() => ({
  copyLocal: vi.fn(),
  detectLocalConflicts: vi.fn(),
}))
vi.mock('../../../shared/api/localfs', () => localfsMocks)

const transferApiMocks = vi.hoisted(() => ({
  detectUploadConflicts: vi.fn(),
  uploadFiles: vi.fn(),
  downloadFiles: vi.fn(),
}))
vi.mock('../../../shared/api/transfer', () => transferApiMocks)

const { useSftpStore, initTransferWatcher } = await import('./store')

// initTransferWatcher subscribes to transfer:task exactly once (module-level
// guard), so the callback is captured the first time any test triggers it
// and reused afterward -- capturing must happen before vi.resetAllMocks()
// wipes the mockImplementation used to grab it.
let capturedTaskCb: ((task: Record<string, unknown>) => void) | null = null
function transferTaskCb(): (task: Record<string, unknown>) => void {
  if (!capturedTaskCb) {
    eventsMocks.subscribe.mockImplementation((topic: string, cb: (task: Record<string, unknown>) => void) => {
      if (topic === 'transfer:task') capturedTaskCb = cb
      return vi.fn()
    })
    initTransferWatcher()
  }
  return capturedTaskCb!
}

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
    pendingConflict: null,
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
    expect(state.gateError).toBe('Connection lost')
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

describe('paste (same-side)', () => {
  it('copies within the local side and keeps the clipboard for repeat pastes', async () => {
    paneOpsMocks.localOps.copyWithin.mockResolvedValue(undefined)
    paneOpsMocks.localOps.list.mockResolvedValue([])
    useSftpStore.setState({ local: { ...EMPTY_PANE, path: '/dst' }, clipboard: { side: 'local', op: 'copy', paths: ['/a.txt'] } })

    await useSftpStore.getState().paste('local')

    expect(paneOpsMocks.localOps.copyWithin).toHaveBeenCalledWith(['/a.txt'], '/dst')
    expect(useSftpStore.getState().clipboard).toEqual({ side: 'local', op: 'copy', paths: ['/a.txt'] })
  })

  it('moves within the remote side and clears the clipboard', async () => {
    const remoteOps = fakeRemoteOps({ moveWithin: vi.fn().mockResolvedValue(undefined), list: vi.fn().mockResolvedValue([]) })
    useSftpStore.setState({
      sessionId: 's1',
      remoteOps,
      remote: { ...EMPTY_PANE, path: '/dst' },
      clipboard: { side: 'remote', op: 'move', paths: ['/a.txt'] },
    })

    await useSftpStore.getState().paste('remote')

    expect(remoteOps.moveWithin).toHaveBeenCalledWith(['/a.txt'], '/dst')
    expect(useSftpStore.getState().clipboard).toBeNull()
  })
})

describe('paste / transferSelection (cross-side)', () => {
  function setup() {
    const remoteOps = fakeRemoteOps({ baseName: vi.fn((p: string) => p.split('/').pop()) })
    useSftpStore.setState({
      sessionId: 's1',
      remoteOps,
      local: { ...EMPTY_PANE, path: '/local/dst' },
      remote: { ...EMPTY_PANE, path: '/remote/dst' },
    })
    return remoteOps
  }

  it('uploads local -> remote immediately when there are no conflicts', async () => {
    setup()
    transferApiMocks.detectUploadConflicts.mockResolvedValue([])
    transferApiMocks.uploadFiles.mockResolvedValue(['t1'])

    await useSftpStore.getState().transferSelection('local', ['/local/a.txt'])

    expect(transferApiMocks.uploadFiles).toHaveBeenCalledWith('s1', ['/local/a.txt'], '/remote/dst', 'overwrite')
    expect(useSftpStore.getState().pendingConflict).toBeNull()
  })

  it('surfaces a pendingConflict and resumes with the chosen policy', async () => {
    setup()
    transferApiMocks.detectUploadConflicts.mockResolvedValue(['a.txt'])
    transferApiMocks.uploadFiles.mockResolvedValue(['t1'])

    await useSftpStore.getState().transferSelection('local', ['/local/a.txt'])

    const conflict = useSftpStore.getState().pendingConflict
    expect(conflict?.names).toEqual(['a.txt'])
    expect(transferApiMocks.uploadFiles).not.toHaveBeenCalled()

    conflict!.resume('rename')
    await Promise.resolve()

    expect(transferApiMocks.uploadFiles).toHaveBeenCalledWith('s1', ['/local/a.txt'], '/remote/dst', 'rename')
    expect(useSftpStore.getState().pendingConflict).toBeNull()
  })

  it('paste of a cross-side move clears the clipboard right away (deletion is deferred to the done event)', async () => {
    setup()
    localfsMocks.detectLocalConflicts.mockResolvedValue([])
    transferApiMocks.downloadFiles.mockResolvedValue(['t2'])
    useSftpStore.setState({ clipboard: { side: 'remote', op: 'move', paths: ['/remote/a.txt'] } })

    await useSftpStore.getState().paste('local')

    expect(transferApiMocks.downloadFiles).toHaveBeenCalledWith('s1', ['/remote/a.txt'], '/local/dst', 'overwrite')
    expect(useSftpStore.getState().clipboard).toBeNull()
  })
})

describe('transfer watcher', () => {
  beforeEach(() => {
    transferTaskCb() // ensure the watcher is registered before each test touches it
  })

  it('refreshes the destination pane once its transfer reports done', async () => {
    const remoteOps = fakeRemoteOps({ list: vi.fn().mockResolvedValue([{ name: 'x', path: '/r/x', size: 0, mode: 0, modeText: '', modTime: 0, isDir: false }]) })
    useSftpStore.setState({ sessionId: 's1', remoteOps, remote: { ...EMPTY_PANE, path: '/r' } })

    transferTaskCb()({ id: 't1', sessionId: 's1', kind: 'upload', state: 'done', src: '/local/x', dst: '/r' })
    await Promise.resolve()
    await Promise.resolve()

    expect(remoteOps.list).toHaveBeenCalledWith('/r')
  })

  it('ignores a done event for a different session', async () => {
    const remoteOps = fakeRemoteOps()
    useSftpStore.setState({ sessionId: 's1', remoteOps, remote: { ...EMPTY_PANE, path: '/r' } })

    transferTaskCb()({ id: 't1', sessionId: 'other', kind: 'upload', state: 'done', src: '/local/x', dst: '/r' })
    await Promise.resolve()

    expect(remoteOps.list).not.toHaveBeenCalled()
  })

  it('deletes a cross-side move source only once its task reports done, using the task-reported src', async () => {
    const remoteOps = fakeRemoteOps({ baseName: vi.fn((p: string) => p.split('/').pop()) })
    useSftpStore.setState({
      sessionId: 's1',
      remoteOps,
      local: { ...EMPTY_PANE, path: '/local/dst' },
      remote: { ...EMPTY_PANE, path: '/remote/dst' },
    })
    transferApiMocks.detectUploadConflicts.mockResolvedValue([])
    transferApiMocks.uploadFiles.mockResolvedValue(['t3'])
    paneOpsMocks.localOps.remove.mockResolvedValue(undefined)

    // A cross-side move: local -> remote, deferred delete on the local source.
    useSftpStore.setState({ clipboard: { side: 'local', op: 'move', paths: ['/local/dst/a.txt'] } })
    await useSftpStore.getState().paste('remote')

    transferTaskCb()({ id: 't3', sessionId: 's1', kind: 'upload', state: 'done', src: '/local/dst/a.txt', dst: '/remote/dst' })
    await Promise.resolve()
    await Promise.resolve()

    expect(paneOpsMocks.localOps.remove).toHaveBeenCalledWith('/local/dst/a.txt')
  })

  it('preserves the source when a move task fails', async () => {
    const remoteOps = fakeRemoteOps()
    useSftpStore.setState({
      sessionId: 's1',
      remoteOps,
      local: { ...EMPTY_PANE, path: '/local/dst' },
      remote: { ...EMPTY_PANE, path: '/remote/dst' },
    })
    transferApiMocks.detectUploadConflicts.mockResolvedValue([])
    transferApiMocks.uploadFiles.mockResolvedValue(['t4'])
    useSftpStore.setState({ clipboard: { side: 'local', op: 'move', paths: ['/local/dst/b.txt'] } })

    await useSftpStore.getState().paste('remote')
    transferTaskCb()({ id: 't4', sessionId: 's1', kind: 'upload', state: 'failed', src: '/local/dst/b.txt', dst: '/remote/dst' })
    await Promise.resolve()

    expect(paneOpsMocks.localOps.remove).not.toHaveBeenCalled()
  })
})
