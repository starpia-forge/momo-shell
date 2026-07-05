import { create } from 'zustand'

export interface ToastItem {
  id: string
  message: string
  onClick?: () => void
}

interface ToastStore {
  toasts: ToastItem[]
  push: (message: string, onClick?: () => void) => void
  remove: (id: string) => void
}

let nextId = 0

export const useToastStore = create<ToastStore>((set) => ({
  toasts: [],
  push: (message, onClick) => {
    nextId += 1
    const id = `toast-${nextId}`
    set((s) => ({ toasts: [...s.toasts, { id, message, onClick }] }))
  },
  remove: (id) => set((s) => ({ toasts: s.toasts.filter((t) => t.id !== id) })),
}))
