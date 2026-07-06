import { create } from 'zustand'
import type { Host } from '../../../entities/host'
import { disposeSession } from '../../../entities/session'
import { connectHost } from '../../../features/session-connect'
import { subscribe, topics, type SessionStatePayload } from '../../../shared/api/events'
import type { RemoteEntry } from '../../../shared/api/transfer'
import { localOps, makeRemoteOps, type PaneOps } from './paneOps'

export interface PaneState {
  path: string | null // null until the pane's first directory resolves
  entries: RemoteEntry[]
  loading: boolean
  error: string | null
  selected: string | null // selected entry's path
}

export type ClipboardSide = 'local' | 'remote'

export interface FsClipboard {
  side: ClipboardSide
  op: 'copy' | 'move'
  paths: string[]
}

const EMPTY_PANE: PaneState = { path: null, entries: [], loading: false, error: null, selected: null }

function sortEntries(entries: RemoteEntry[]): RemoteEntry[] {
  return [...entries].sort((a, b) => (a.isDir === b.isDir ? a.name.localeCompare(b.name) : a.isDir ? -1 : 1))
}

interface SftpStore {
  sessionId: string | null
  remoteOps: PaneOps | null
  hostLabel: string
  connecting: boolean
  gateError: string | null
  local: PaneState
  remote: PaneState
  clipboard: FsClipboard | null

  connect: (host: Host) => Promise<void>
  disconnect: () => void
  initLocal: () => Promise<void>
  refreshLocal: (path: string) => Promise<void>
  refreshRemote: (path: string) => Promise<void>
  select: (side: ClipboardSide, path: string | null) => void
  setClipboard: (clipboard: FsClipboard | null) => void
}

// Module-scope (not store state) since it's an implementation detail of
// connect/disconnect lifecycle management, not something a component reads.
let unsubSessionState: (() => void) | null = null

function stopWatchingSession(): void {
  unsubSessionState?.()
  unsubSessionState = null
}

function errorMessage(err: unknown): string {
  return err instanceof Error ? err.message : String(err)
}

export const useSftpStore = create<SftpStore>((set, get) => ({
  sessionId: null,
  remoteOps: null,
  hostLabel: '',
  connecting: false,
  gateError: null,
  local: EMPTY_PANE,
  remote: EMPTY_PANE,
  clipboard: null,

  connect: async (host) => {
    set({ connecting: true, gateError: null, hostLabel: `${host.name} (${host.address})` })
    try {
      const sessionId = await connectHost(host.id)
      stopWatchingSession()
      unsubSessionState = subscribe<SessionStatePayload>(topics.sessionState(sessionId), (p) => {
        if (p.state === 'closed' || p.state === 'error') {
          stopWatchingSession()
          set({ sessionId: null, remoteOps: null, remote: EMPTY_PANE, gateError: p.error ?? '연결이 끊어졌습니다' })
        }
      })
      const ops = makeRemoteOps(sessionId)
      const home = await ops.homeDir()
      set({ sessionId, remoteOps: ops })
      await get().refreshRemote(home)
    } catch (err) {
      set({ sessionId: null, remoteOps: null, gateError: errorMessage(err) })
    } finally {
      set({ connecting: false })
    }
  },

  disconnect: () => {
    stopWatchingSession()
    const { sessionId, clipboard } = get()
    if (sessionId) disposeSession(sessionId)
    set({
      sessionId: null,
      remoteOps: null,
      remote: EMPTY_PANE,
      clipboard: clipboard?.side === 'remote' ? null : clipboard,
    })
  },

  initLocal: async () => {
    if (get().local.path !== null) return
    try {
      const home = await localOps.homeDir()
      await get().refreshLocal(home)
    } catch (err) {
      set((s) => ({ local: { ...s.local, error: errorMessage(err), loading: false } }))
    }
  },

  refreshLocal: async (path) => {
    set((s) => ({ local: { ...s.local, loading: true } }))
    try {
      const entries = sortEntries(await localOps.list(path))
      set({ local: { path, entries, loading: false, error: null, selected: null } })
    } catch (err) {
      set((s) => ({ local: { ...s.local, loading: false, error: errorMessage(err) } }))
    }
  },

  refreshRemote: async (path) => {
    const ops = get().remoteOps
    if (!ops) return
    set((s) => ({ remote: { ...s.remote, loading: true } }))
    try {
      const entries = sortEntries(await ops.list(path))
      set({ remote: { path, entries, loading: false, error: null, selected: null } })
    } catch (err) {
      set((s) => ({ remote: { ...s.remote, loading: false, error: errorMessage(err) } }))
    }
  },

  select: (side, path) => set((s) => ({ [side]: { ...s[side], selected: path } }) as Pick<SftpStore, 'local' | 'remote'>),

  setClipboard: (clipboard) => set({ clipboard }),
}))
