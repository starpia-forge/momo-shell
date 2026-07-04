import { create } from 'zustand'
import type { HistoryEntry } from '../../../shared/api'

export type HistoryScopeMode = 'all' | 'current'

interface HistoryPanelStore {
  open: boolean
  entries: HistoryEntry[]
  filter: string
  scopeMode: HistoryScopeMode
  toggle: () => void
  setFilter: (text: string) => void
  setScopeMode: (mode: HistoryScopeMode) => void
  setEntries: (entries: HistoryEntry[]) => void
  /** Adds a new entry, or moves an existing one (same ID, bumped timestamp) to the front. */
  prepend: (entry: HistoryEntry) => void
  removeEntry: (id: number) => void
}

export const useHistoryPanelStore = create<HistoryPanelStore>((set) => ({
  open: false,
  entries: [],
  filter: '',
  scopeMode: 'all',
  toggle: () => set((s) => ({ open: !s.open })),
  setFilter: (filter) => set({ filter }),
  setScopeMode: (scopeMode) => set({ scopeMode }),
  setEntries: (entries) => set({ entries }),
  prepend: (entry) => set((s) => ({ entries: [entry, ...s.entries.filter((e) => e.id !== entry.id)] })),
  removeEntry: (id) => set((s) => ({ entries: s.entries.filter((e) => e.id !== id) })),
}))
