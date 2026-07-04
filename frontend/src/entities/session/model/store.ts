import { create } from 'zustand'

export type SessionState = 'connecting' | 'starting' | 'running' | 'closed' | 'error'

export interface SessionMeta {
  id: string
  shell: string
  cols: number
  rows: number
  state: SessionState
  exitCode?: number
  error?: string
}

interface SessionStore {
  sessions: Record<string, SessionMeta>
  upsert: (meta: SessionMeta) => void
  setState: (id: string, state: SessionState, extra?: { exitCode?: number; error?: string }) => void
  remove: (id: string) => void
}

export const useSessionStore = create<SessionStore>((set) => ({
  sessions: {},
  upsert: (meta) => set((s) => ({ sessions: { ...s.sessions, [meta.id]: meta } })),
  setState: (id, state, extra) =>
    set((s) => {
      const existing = s.sessions[id]
      if (!existing) return s
      return { sessions: { ...s.sessions, [id]: { ...existing, state, ...extra } } }
    }),
  remove: (id) =>
    set((s) => {
      const rest = { ...s.sessions }
      delete rest[id]
      return { sessions: rest }
    }),
}))
