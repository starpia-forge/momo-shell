import { create } from 'zustand'

// Tab.id starts out equal to sessionId (Phase 2 is one pane per tab, so
// there's no reason to mint a separate identifier at creation time), but
// they diverge on reconnect: id stays put as the tab's stable identity
// while sessionId is repointed at the freshly-opened session.
export interface Tab {
  id: string
  kind: 'local' | 'ssh'
  hostId?: string
  sessionId: string
  title: string
  subtitle: string
}

/** Which of the center-slot views WorkspacePage renders. */
export type Screen = 'home' | 'settings' | 'sftp' | 'workspace'

interface TabBarStore {
  tabs: Tab[]
  activeId: string | null
  newTabPopoverOpen: boolean
  screen: Screen
  addTab: (tab: Tab) => void
  removeTab: (id: string) => void
  setActive: (id: string) => void
  reorder: (fromIndex: number, toIndex: number) => void
  openNewTabPopover: () => void
  closeNewTabPopover: () => void
  activateByIndex: (index: number) => void
  replaceSession: (tabId: string, sessionId: string) => void
  showHome: () => void
  showSettings: () => void
  showSftp: () => void
}

export const useTabStore = create<TabBarStore>((set) => ({
  tabs: [],
  activeId: null,
  newTabPopoverOpen: false,
  screen: 'home',
  addTab: (tab) =>
    set((s) => ({ tabs: [...s.tabs, tab], activeId: tab.id, newTabPopoverOpen: false, screen: 'workspace' })),
  removeTab: (id) =>
    set((s) => {
      const idx = s.tabs.findIndex((t) => t.id === id)
      const tabs = s.tabs.filter((t) => t.id !== id)
      let activeId = s.activeId
      if (activeId === id) {
        activeId = tabs[idx]?.id ?? tabs[idx - 1]?.id ?? null
      }
      return { tabs, activeId, screen: tabs.length === 0 ? 'home' : s.screen }
    }),
  setActive: (id) => set({ activeId: id, screen: 'workspace' }),
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
      return tab ? { activeId: tab.id, screen: 'workspace' } : {}
    }),
  replaceSession: (tabId, sessionId) =>
    set((s) => ({
      tabs: s.tabs.map((t) => (t.id === tabId ? { ...t, sessionId } : t)),
    })),
  showHome: () => set({ screen: 'home' }),
  showSettings: () => set({ screen: 'settings' }),
  showSftp: () => set({ screen: 'sftp' }),
}))
