import { create } from 'zustand'

export type ToastVariant = 'success' | 'error' | 'info'

export interface ToastItem {
  id: string
  message: string
  variant: ToastVariant
  onClick?: () => void
}

interface ToastStore {
  toasts: ToastItem[]
  push: (message: string, variant?: ToastVariant, onClick?: () => void) => void
  remove: (id: string) => void
}

let nextId = 0

export const useToastStore = create<ToastStore>((set) => ({
  toasts: [],
  push: (message, variant = 'info', onClick) => {
    nextId += 1
    const id = `toast-${nextId}`
    set((s) => ({ toasts: [...s.toasts, { id, message, variant, onClick }] }))
  },
  remove: (id) => set((s) => ({ toasts: s.toasts.filter((t) => t.id !== id) })),
}))
