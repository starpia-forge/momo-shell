import { create } from 'zustand'
import type { Delegation } from '../../../shared/api/mcp'

interface DelegationStore {
  delegations: Record<string, Delegation>
  open: boolean
  toggleOpen: () => void
  close: () => void
  replaceAll: (delegations: Delegation[]) => void
  setDelegation: (delegation: Delegation) => void
  removeDelegation: (sessionId: string) => void
}

export const useDelegationStore = create<DelegationStore>((set) => ({
  delegations: {},
  open: false,
  toggleOpen: () => set((s) => ({ open: !s.open })),
  close: () => set({ open: false }),
  replaceAll: (delegations) => set({ delegations: Object.fromEntries(delegations.map((d) => [d.sessionId, d])) }),
  setDelegation: (delegation) => set((s) => ({ delegations: { ...s.delegations, [delegation.sessionId]: delegation } })),
  removeDelegation: (sessionId) =>
    set((s) => {
      const rest = { ...s.delegations }
      delete rest[sessionId]
      return { delegations: rest }
    }),
}))
