import { create } from 'zustand'

// Tab.id doubles as the session ID -- Phase 2 is one pane per tab, so
// there's no reason to mint a separate identifier yet (Phase 3's split
// layouts are what actually need tab != pane).
export interface Tab {
  id: string
  kind: 'local' | 'ssh'
  hostId?: string
  sessionId: string
  title: string
  subtitle: string
}

interface TabBarStore {
  tabs: Tab[]
  activeId: string | null
  newTabPopoverOpen: boolean
  addTab: (tab: Tab) => void
  removeTab: (id: string) => void
  setActive: (id: string) => void
  reorder: (fromIndex: number, toIndex: number) => void
  openNewTabPopover: () => void
  closeNewTabPopover: () => void
  activateByIndex: (index: number) => void
}

export const useTabStore = create<TabBarStore>((set) => ({
  tabs: [],
  activeId: null,
  newTabPopoverOpen: false,
  addTab: (tab) => set((s) => ({ tabs: [...s.tabs, tab], activeId: tab.id, newTabPopoverOpen: false })),
  removeTab: (id) =>
    set((s) => {
      const idx = s.tabs.findIndex((t) => t.id === id)
      const tabs = s.tabs.filter((t) => t.id !== id)
      let activeId = s.activeId
      if (activeId === id) {
        activeId = tabs[idx]?.id ?? tabs[idx - 1]?.id ?? null
      }
      return { tabs, activeId }
    }),
  setActive: (id) => set({ activeId: id }),
  reorder: (fromIndex, toIndex) =>
    set((s) => {
      if (fromIndex === toIndex) return s
      const tabs = [...s.tabs]
      const [moved] = tabs.splice(fromIndex, 1)
      tabs.splice(toIndex, 0, moved)
      return { tabs }
    }),
  openNewTabPopover: () => set({ newTabPopoverOpen: true }),
  closeNewTabPopover: () => set({ newTabPopoverOpen: false }),
  activateByIndex: (index) =>
    set((s) => {
      const tab = s.tabs[index]
      return tab ? { activeId: tab.id } : {}
    }),
}))
