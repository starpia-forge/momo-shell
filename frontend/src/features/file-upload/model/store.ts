import { create } from 'zustand'

export interface PendingDrop {
  sessionId: string
  paths: string[]
  /** Editable destination directory, defaulted from the session's OSC 7 cwd or SFTP home. */
  cwd: string
}

interface FileUploadStore {
  pending: PendingDrop | null
  setPending: (pending: PendingDrop | null) => void
}

export const useFileUploadStore = create<FileUploadStore>((set) => ({
  pending: null,
  setPending: (pending) => set({ pending }),
}))
