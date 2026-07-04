import { create } from 'zustand'

// Only one pane's search overlay can be open at a time -- opening it for a
// new leaf implicitly closes whichever one was open before.
interface TerminalSearchStore {
  openForLeafId: string | null
  openFor: (leafId: string) => void
  close: () => void
}

export const useTerminalSearchStore = create<TerminalSearchStore>((set) => ({
  openForLeafId: null,
  openFor: (leafId) => set({ openForLeafId: leafId }),
  close: () => set({ openForLeafId: null }),
}))
