import { create } from 'zustand'
import type { Host } from '../../../entities/host'
import { disposeSession } from '../../../entities/session'
import { connectHost } from '../../../features/session-connect'
import i18n from '../../../shared/i18n'
import { copyLocal, detectLocalConflicts } from '../../../shared/api/localfs'
import { subscribe, topics, type SessionStatePayload, type TransferTaskPayload } from '../../../shared/api/events'
import { detectUploadConflicts, downloadFiles, uploadFiles, type ConflictPolicy, type RemoteEntry } from '../../../shared/api/transfer'
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

/** A same-name collision detected at a transfer's destination -- surfaced
 * to the UI as a ConflictDialog; `resume` re-runs the transfer with the
 * user's chosen policy. */
export interface PendingConflict {
  names: string[]
  resume: (policy: ConflictPolicy) => void
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
  pendingConflict: PendingConflict | null

  connect: (host: Host) => Promise<void>
  disconnect: () => void
  initLocal: () => Promise<void>
  refreshLocal: (path: string) => Promise<void>
  refreshRemote: (path: string) => Promise<void>
  select: (side: ClipboardSide, path: string | null) => void
  setClipboard: (clipboard: FsClipboard | null) => void
  clearPendingConflict: () => void
  /** Uploads/downloads paths from `fromSide` into the opposite pane's
   * current directory (drag-drop or the "업로드"/"다운로드" menu item). */
  transferSelection: (fromSide: ClipboardSide, paths: string[]) => Promise<void>
  /** Applies the clipboard (set via setClipboard) to targetSide: same-side
   * copy/move, or a cross-side transfer for the opposite side. */
  paste: (targetSide: ClipboardSide) => Promise<void>
  /** An OS file drop landing directly on a pane (not via the clipboard). */
  dropIncoming: (side: ClipboardSide, paths: string[]) => Promise<void>
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

// Cross-side move deletes their source only once the transfer task that
// moved them reports "done" -- keyed by task ID (not path/order) since a
// conflict policy of "skip" can shrink Upload/Download's returned task list
// relative to the original path list. The event's own `src` field (not our
// original call args) is what gets deleted, so ordering never matters.
const pendingMoveDeletes = new Map<string, ClipboardSide>()
let transferWatcherStarted = false

/** Starts the page-lifetime watcher that refreshes a pane once its transfer
 * lands and deletes cross-side move sources once their task completes.
 * Idempotent -- safe to call from SftpPage's mount effect every time. */
export function initTransferWatcher(): void {
  if (transferWatcherStarted) return
  transferWatcherStarted = true
  subscribe<TransferTaskPayload>(topics.transferTask(), (task) => {
    const state = useSftpStore.getState()
    if (task.sessionId !== state.sessionId) return // not this page's session

    if (task.state === 'done') {
      const destSide: ClipboardSide = task.kind === 'upload' ? 'remote' : 'local'
      if (state[destSide].path === task.dst) {
        void (destSide === 'local' ? state.refreshLocal(task.dst) : state.refreshRemote(task.dst))
      }
      if (pendingMoveDeletes.has(task.id)) {
        const side = pendingMoveDeletes.get(task.id)!
        pendingMoveDeletes.delete(task.id)
        const ops = side === 'local' ? localOps : useSftpStore.getState().remoteOps
        ops?.remove(task.src).catch((err) => {
          useSftpStore.setState((s) => ({
            [side]: { ...s[side], error: i18n.t('sftp.errors.deleteSourceFailed', { message: errorMessage(err) }) },
          }))
        })
      }
    } else if (task.state === 'failed' || task.state === 'canceled') {
      pendingMoveDeletes.delete(task.id) // preserve the source on a failed/canceled move
    }
  })
}

/** Starts (or resumes past a conflict) an upload/download of `paths` from
 * `fromSide` into the opposite pane's current directory. */
async function startCrossSideTransfer(
  get: () => SftpStore,
  set: (partial: Partial<SftpStore> | ((s: SftpStore) => Partial<SftpStore>)) => void,
  fromSide: ClipboardSide,
  paths: string[],
  isMove: boolean,
  policy: ConflictPolicy
): Promise<void> {
  const state = get()
  const sessionId = state.sessionId
  if (!sessionId) return
  try {
    const taskIds =
      fromSide === 'local'
        ? await uploadFiles(sessionId, paths, state.remote.path ?? '', policy)
        : await downloadFiles(sessionId, paths, state.local.path ?? '', policy)
    if (isMove) taskIds.forEach((id) => pendingMoveDeletes.set(id, fromSide))
  } catch (err) {
    set((s) => ({ [fromSide]: { ...s[fromSide], error: errorMessage(err) } }))
  }
}

/** Shared by transferSelection and cross-side paste: checks the
 * destination for name collisions, surfacing a PendingConflict if any
 * exist instead of transferring immediately. */
async function transferWithConflictCheck(
  get: () => SftpStore,
  set: (partial: Partial<SftpStore> | ((s: SftpStore) => Partial<SftpStore>)) => void,
  fromSide: ClipboardSide,
  paths: string[],
  isMove: boolean
): Promise<void> {
  const state = get()
  const sessionId = state.sessionId
  if (!sessionId) return
  const toSide: ClipboardSide = fromSide === 'local' ? 'remote' : 'local'
  const dstDir = state[toSide].path
  if (dstDir === null) return

  const conflicts =
    fromSide === 'local'
      ? await detectUploadConflicts(sessionId, paths, dstDir)
      : await detectLocalConflicts(paths.map((p) => state.remoteOps!.baseName(p)), dstDir)

  if (conflicts.length === 0) {
    await startCrossSideTransfer(get, set, fromSide, paths, isMove, 'overwrite')
    return
  }
  set({
    pendingConflict: {
      names: conflicts,
      resume: (policy) => {
        set({ pendingConflict: null })
        void startCrossSideTransfer(get, set, fromSide, paths, isMove, policy)
      },
    },
  })
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
  pendingConflict: null,

  connect: async (host) => {
    set({ connecting: true, gateError: null, hostLabel: `${host.name} (${host.address})` })
    try {
      const sessionId = await connectHost(host.id)
      stopWatchingSession()
      unsubSessionState = subscribe<SessionStatePayload>(topics.sessionState(sessionId), (p) => {
        if (p.state === 'closed' || p.state === 'error') {
          stopWatchingSession()
          set({ sessionId: null, remoteOps: null, remote: EMPTY_PANE, gateError: p.error ?? i18n.t('sftp.gate.disconnected') })
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

  clearPendingConflict: () => set({ pendingConflict: null }),

  transferSelection: async (fromSide, paths) => {
    await transferWithConflictCheck(get, set, fromSide, paths, false)
  },

  paste: async (targetSide) => {
    const state = get()
    const clipboard = state.clipboard
    if (!clipboard) return
    const dstDir = state[targetSide].path
    if (dstDir === null) return

    if (clipboard.side === targetSide) {
      const ops = targetSide === 'local' ? localOps : state.remoteOps
      if (!ops) return
      try {
        if (clipboard.op === 'copy') await ops.copyWithin(clipboard.paths, dstDir)
        else await ops.moveWithin(clipboard.paths, dstDir)
        await (targetSide === 'local' ? get().refreshLocal(dstDir) : get().refreshRemote(dstDir))
        if (clipboard.op === 'move') set({ clipboard: null })
      } catch (err) {
        set((s) => ({ [targetSide]: { ...s[targetSide], error: errorMessage(err) } }))
      }
      return
    }

    await transferWithConflictCheck(get, set, clipboard.side, clipboard.paths, clipboard.op === 'move')
    if (clipboard.op === 'move') set({ clipboard: null })
  },

  dropIncoming: async (side, paths) => {
    const state = get()
    const dstDir = state[side].path
    if (dstDir === null) return
    if (side === 'local') {
      try {
        await copyLocal(paths, dstDir)
        await get().refreshLocal(dstDir)
      } catch (err) {
        set((s) => ({ local: { ...s.local, error: errorMessage(err) } }))
      }
      return
    }
    await transferWithConflictCheck(get, set, 'local', paths, false)
  },
}))
