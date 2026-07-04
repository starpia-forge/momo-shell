import { EventsOn } from '../../../wailsjs/runtime/runtime'

export const topics = {
  sessionData: (id: string) => `session:data:${id}`,
  sessionState: (id: string) => `session:state:${id}`,
  sessionClosed: (id: string) => `session:closed:${id}`,
  sessionHostKey: (id: string) => `session:hostkey:${id}`,
  historyAppended: () => `history:appended`,
  transferTask: () => `transfer:task`,
  transferProgress: (taskId: string) => `transfer:progress:${taskId}`,
  transferZmodem: (sessionId: string) => `transfer:zmodem:${sessionId}`,
  osFileDrop: () => `os:filedrop`,
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

export interface TransferTaskPayload {
  id: string
  sessionId: string
  kind: 'upload' | 'download'
  state: 'queued' | 'running' | 'done' | 'failed' | 'canceled'
  src: string
  dst: string
  currentFile?: string
  bytes: number
  total: number
  offset: number
  error?: string
}

export interface TransferProgressPayload {
  bytes: number
  total: number
  rate: number
  state: string
  file: string
}

/** Wails' native OS file drop -- x/y are webview-relative client coordinates. */
export interface FileDropPayload {
  x: number
  y: number
  paths: string[]
}

export interface TransferZmodemPayload {
  direction: 'upload' | 'download'
  phase: 'detected' | 'active' | 'done' | 'failed' | 'canceled'
  taskId?: string
}

// subscribe wraps EventsOn with a typed callback and returns the unsubscribe
// function directly (Wails' EventsOn already returns one).
export function subscribe<T>(topic: string, cb: (payload: T) => void): () => void {
  return EventsOn(topic, (payload: T) => cb(payload))
}
