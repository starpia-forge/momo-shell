import { create } from 'zustand'
import type { ShareStatus, ShareClient } from '../../../shared/api/share'

interface SharePanelStore {
  status: ShareStatus | null
  clients: ShareClient[]
  setStatus: (status: ShareStatus) => void
  setClients: (clients: ShareClient[]) => void
  removeClient: (id: string) => void
}

export const useSharePanelStore = create<SharePanelStore>((set) => ({
  status: null,
  clients: [],
  setStatus: (status) => set({ status }),
  setClients: (clients) => set({ clients }),
  removeClient: (id) => set((s) => ({ clients: s.clients.filter((c) => c.id !== id) })),
}))
