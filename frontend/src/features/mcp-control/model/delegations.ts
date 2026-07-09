import { create } from 'zustand'
import type { ConnectionScope, Delegation } from '../../../shared/api/mcp'
import type { MCPCommandStatePayload } from '../../../shared/api/events'

interface DelegationStore {
  delegations: Record<string, Delegation>
  cmdStates: Record<string, MCPCommandStatePayload>
  scopes: ConnectionScope[]
  open: boolean
  toggleOpen: () => void
  close: () => void
  replaceAll: (delegations: Delegation[]) => void
  setDelegation: (delegation: Delegation) => void
  removeDelegation: (sessionId: string) => void
  setCmdState: (payload: MCPCommandStatePayload) => void
  setScopes: (scopes: ConnectionScope[]) => void
}

export const useDelegationStore = create<DelegationStore>((set) => ({
  delegations: {},
  cmdStates: {},
  scopes: [],
  open: false,
  toggleOpen: () => set((s) => ({ open: !s.open })),
  close: () => set({ open: false }),
  replaceAll: (delegations) => set({ delegations: Object.fromEntries(delegations.map((d) => [d.sessionId, d])) }),
  setDelegation: (delegation) => set((s) => ({ delegations: { ...s.delegations, [delegation.sessionId]: delegation } })),
  removeDelegation: (sessionId) =>
    set((s) => {
      const rest = { ...s.delegations }
      delete rest[sessionId]
      const restCmdStates = { ...s.cmdStates }
      delete restCmdStates[sessionId]
      return { delegations: rest, cmdStates: restCmdStates }
    }),
  setCmdState: (payload) => set((s) => ({ cmdStates: { ...s.cmdStates, [payload.sessionId]: payload } })),
  setScopes: (scopes) => set({ scopes }),
}))
