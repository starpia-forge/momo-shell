import {
  CreateLocalSession,
  CreateSSHSession,
  CreateSSHDirectSession,
  RespondHostKey,
  WriteSession,
  ResizeSession,
  CloseSession,
} from '../../../wailsjs/go/wails/SessionService'
import { bytesToB64 } from '../lib/base64'
import type { AuthType } from './host'

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

export interface CreateSSHDirectSessionOpts {
  name: string
  address: string
  port: number
  username: string
  authType: AuthType
  keyPath?: string
  secret: string
  cols: number
  rows: number
}

// createSSHDirectSession connects to a host with no saved Host row (e.g. a
// peer's shared host) -- credentials are supplied here and used only for
// this dial, never persisted.
export async function createSSHDirectSession(opts: CreateSSHDirectSessionOpts): Promise<SessionInfo> {
  return CreateSSHDirectSession({
    name: opts.name,
    address: opts.address,
    port: opts.port,
    username: opts.username,
    authType: opts.authType,
    keyPath: opts.keyPath ?? '',
    secret: opts.secret,
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
