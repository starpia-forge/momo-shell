import { create } from 'zustand'
import type { AuditEvent } from '../../../shared/api/audit'

interface AuditPanelStore {
  events: AuditEvent[]
  filterText: string
  /** '' = every client. */
  clientFilter: string
  /** '' = every decision. */
  decisionFilter: string
  /** The session currently open in the replay dialog; null = closed. */
  replaySessionId: string | null
  setEvents: (events: AuditEvent[]) => void
  setFilterText: (text: string) => void
  setClientFilter: (clientId: string) => void
  setDecisionFilter: (decision: string) => void
  openReplay: (sessionId: string) => void
  closeReplay: () => void
}

export const useAuditPanelStore = create<AuditPanelStore>((set) => ({
  events: [],
  filterText: '',
  clientFilter: '',
  decisionFilter: '',
  replaySessionId: null,
  setEvents: (events) => set({ events }),
  setFilterText: (filterText) => set({ filterText }),
  setClientFilter: (clientFilter) => set({ clientFilter }),
  setDecisionFilter: (decisionFilter) => set({ decisionFilter }),
  openReplay: (sessionId) => set({ replaySessionId: sessionId }),
  closeReplay: () => set({ replaySessionId: null }),
}))
