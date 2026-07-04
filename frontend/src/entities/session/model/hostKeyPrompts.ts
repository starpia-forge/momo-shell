import { create } from 'zustand'
import type { SessionHostKeyPayload } from '../../../shared/api/events'

interface HostKeyPromptStore {
  prompts: Record<string, SessionHostKeyPayload>
  setPrompt: (sessionId: string, payload: SessionHostKeyPayload) => void
  clearPrompt: (sessionId: string) => void
}

export const useHostKeyPromptStore = create<HostKeyPromptStore>((set) => ({
  prompts: {},
  setPrompt: (sessionId, payload) => set((s) => ({ prompts: { ...s.prompts, [sessionId]: payload } })),
  clearPrompt: (sessionId) =>
    set((s) => {
      const rest = { ...s.prompts }
      delete rest[sessionId]
      return { prompts: rest }
    }),
}))
