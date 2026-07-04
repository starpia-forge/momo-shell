import { EventsOn } from '../../../wailsjs/runtime/runtime'

export const topics = {
  sessionData: (id: string) => `session:data:${id}`,
  sessionState: (id: string) => `session:state:${id}`,
  sessionClosed: (id: string) => `session:closed:${id}`,
  sessionHostKey: (id: string) => `session:hostkey:${id}`,
  historyAppended: () => `history:appended`,
}

export interface SessionStatePayload {
  state: 'connecting' | 'starting' | 'running' | 'closed' | 'error'
  error?: string
}

export interface SessionClosedPayload {
  exitCode?: number
}

export interface SessionHostKeyPayload {
  address: string
  port: number
  algo: string
  fingerprint: string
}

export interface HistoryAppendedPayload {
  id: number
  hostId?: string
  command: string
  executedAt: number
}

// subscribe wraps EventsOn with a typed callback and returns the unsubscribe
// function directly (Wails' EventsOn already returns one).
export function subscribe<T>(topic: string, cb: (payload: T) => void): () => void {
  return EventsOn(topic, (payload: T) => cb(payload))
}
