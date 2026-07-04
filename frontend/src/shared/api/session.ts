import {
  CreateLocalSession,
  CreateSSHSession,
  RespondHostKey,
  WriteSession,
  ResizeSession,
  CloseSession,
} from '../../../wailsjs/go/wails/SessionService'
import { bytesToB64 } from '../lib/base64'

export interface SessionInfo {
  id: string
  kind: string
  shell?: string
  hostId?: string
  cols: number
  rows: number
}

export interface CreateLocalSessionOpts {
  shell?: string
  cwd?: string
  cols: number
  rows: number
}

export async function createLocalSession(opts: CreateLocalSessionOpts): Promise<SessionInfo> {
  return CreateLocalSession({
    shell: opts.shell ?? '',
    cwd: opts.cwd ?? '',
    cols: opts.cols,
    rows: opts.rows,
  })
}

export interface CreateSSHSessionOpts {
  hostId: string
  cols: number
  rows: number
}

export async function createSSHSession(opts: CreateSSHSessionOpts): Promise<SessionInfo> {
  return CreateSSHSession({
    hostId: opts.hostId,
    cols: opts.cols,
    rows: opts.rows,
  })
}

export type HostKeyDecision = 'trust' | 'once' | 'cancel'

export async function respondHostKey(sessionId: string, decision: HostKeyDecision): Promise<void> {
  await RespondHostKey(sessionId, decision)
}

export async function writeSession(id: string, data: Uint8Array): Promise<void> {
  await WriteSession(id, bytesToB64(data))
}

export async function resizeSession(id: string, cols: number, rows: number): Promise<void> {
  await ResizeSession(id, cols, rows)
}

export async function closeSession(id: string): Promise<void> {
  await CloseSession(id)
}
