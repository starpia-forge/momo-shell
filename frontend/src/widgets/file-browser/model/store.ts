import { create } from 'zustand'
import type { RemoteEntry } from '../../../shared/api'

interface SessionBrowseState {
  path: string | null // null until home dir resolves
  entries: RemoteEntry[]
  loading: boolean
  error: string | null
  selected: string | null // selected entry's path
}

export const EMPTY_BROWSE_STATE: SessionBrowseState = { path: null, entries: [], loading: false, error: null, selected: null }

interface FileBrowserStore {
  bySession: Record<string, SessionBrowseState>
  setPath: (sessionId: string, path: string) => void
  setEntries: (sessionId: string, entries: RemoteEntry[]) => void
  setLoading: (sessionId: string, loading: boolean) => void
  setError: (sessionId: string, error: string | null) => void
  setSelected: (sessionId: string, path: string | null) => void
}

function patch(state: FileBrowserStore, sessionId: string, partial: Partial<SessionBrowseState>): Pick<FileBrowserStore, 'bySession'> {
  const current = state.bySession[sessionId] ?? EMPTY_BROWSE_STATE
  return { bySession: { ...state.bySession, [sessionId]: { ...current, ...partial } } }
}

export const useFileBrowserStore = create<FileBrowserStore>((set) => ({
  bySession: {},
  setPath: (sessionId, path) => set((s) => patch(s, sessionId, { path, selected: null })),
  setEntries: (sessionId, entries) => set((s) => patch(s, sessionId, { entries, loading: false, error: null })),
  setLoading: (sessionId, loading) => set((s) => patch(s, sessionId, { loading })),
  setError: (sessionId, error) => set((s) => patch(s, sessionId, { error, loading: false })),
  setSelected: (sessionId, selected) => set((s) => patch(s, sessionId, { selected })),
}))

export function sessionBrowseState(sessionId: string): SessionBrowseState {
  return useFileBrowserStore.getState().bySession[sessionId] ?? EMPTY_BROWSE_STATE
}
