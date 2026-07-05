import { create } from 'zustand'
import type { PeerView } from '../../../shared/api/share'

interface PeerStore {
  peers: PeerView[]
  setPeers: (peers: PeerView[]) => void
}

export const usePeerStore = create<PeerStore>((set) => ({
  peers: [],
  setPeers: (peers) => set({ peers }),
}))
