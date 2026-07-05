import { create } from 'zustand'
import type { SharePairRequestPayload } from '../../../shared/api/events'

interface PairRequestStore {
  requests: Record<string, SharePairRequestPayload>
  setRequest: (requestId: string, payload: SharePairRequestPayload) => void
  clearRequest: (requestId: string) => void
}

export const usePairRequestStore = create<PairRequestStore>((set) => ({
  requests: {},
  setRequest: (requestId, payload) => set((s) => ({ requests: { ...s.requests, [requestId]: payload } })),
  clearRequest: (requestId) =>
    set((s) => {
      const rest = { ...s.requests }
      delete rest[requestId]
      return { requests: rest }
    }),
}))
