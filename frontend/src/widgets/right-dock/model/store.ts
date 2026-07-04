import { create } from 'zustand'

export type RightDockTab = 'history' | 'files'

interface RightDockStore {
  open: boolean
  active: RightDockTab
  /** Opens the dock to `tab` (switching tabs if already open). */
  show: (tab: RightDockTab) => void
  /** Opens to `tab` if closed or on another tab; closes if already showing `tab`. */
  toggle: (tab: RightDockTab) => void
  close: () => void
}

export const useRightDockStore = create<RightDockStore>((set) => ({
  open: false,
  active: 'history',
  show: (tab) => set({ open: true, active: tab }),
  toggle: (tab) => set((s) => (s.open && s.active === tab ? { open: false } : { open: true, active: tab })),
  close: () => set({ open: false }),
}))
