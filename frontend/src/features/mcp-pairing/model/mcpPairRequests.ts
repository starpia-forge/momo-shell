import { create } from 'zustand'
import type { MCPPairRequestPayload } from '../../../shared/api/events'

interface MCPPairRequestStore {
  requests: Record<string, MCPPairRequestPayload>
  setRequest: (requestId: string, payload: MCPPairRequestPayload) => void
  clearRequest: (requestId: string) => void
}

export const useMCPPairRequestStore = create<MCPPairRequestStore>((set) => ({
  requests: {},
  setRequest: (requestId, payload) => set((s) => ({ requests: { ...s.requests, [requestId]: payload } })),
  clearRequest: (requestId) =>
    set((s) => {
      const rest = { ...s.requests }
      delete rest[requestId]
      return { requests: rest }
    }),
}))
