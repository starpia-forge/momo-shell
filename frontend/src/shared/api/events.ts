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
  sharePairRequest: () => `share:pair-request`,
  sharePairRequestResolved: () => `share:pair-request-resolved`,
  sharePeersUpdated: () => `share:peers-updated`,
  mcpPairRequest: () => `mcp:pair-request`,
  mcpPairRequestResolved: () => `mcp:pair-request-resolved`,
  mcpConnectApproval: () => `mcp:connect-approval`,
  mcpControlApproval: () => `mcp:control-approval`,
  mcpConnectionScopeApproval: () => `mcp:connection-scope-approval`,
  mcpCommandApproval: () => `mcp:cmd-approval`,
  mcpDelegation: () => `mcp:delegation`,
  mcpCommandState: () => `mcp:cmd-state`,
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

/** Published when an incoming /pair request is awaiting the local user's
 * approve/deny decision via respondPairing. */
export interface SharePairRequestPayload {
  requestId: string
  clientName: string
  remoteAddr: string
}

/** Published once a share:pair-request is no longer pending (answered,
 * timed out, or the requester disconnected) -- lets the approval dialog
 * dismiss itself even if respondPairing was never called for it. */
export interface SharePairRequestResolvedPayload {
  requestId: string
}

/** Published when an incoming MCP client's pair frame is awaiting the local
 * user's approve/deny decision via respondPairing (mcp). No PIN or remote
 * address -- MCP pairing is a local-only IPC transport (doc 20 D1). */
export interface MCPPairRequestPayload {
  requestId: string
  clientName: string
}

/** Published once an mcp:pair-request is no longer pending (answered, timed
 * out, or the client disconnected) -- SharePairRequestResolvedPayload's
 * mirror. */
export interface MCPPairRequestResolvedPayload {
  requestId: string
}

/** Published when connect_host is awaiting the local user's approve/deny
 * decision via respondConnectApproval (no active ConnectionScope covers the
 * requested host, so this isn't an auto-grant). */
export interface MCPConnectApprovalPayload {
  requestId: string
  clientId: string
  hostName: string
}

/** Published when RequestControl (taking over an existing, human-opened
 * session) is awaiting the local user's approve/deny decision via
 * respondControlApproval. */
export interface MCPControlApprovalPayload {
  requestId: string
  clientId: string
  sessionId: string
}

/** Published when a fan-out RequestConnectionScope batch grant is awaiting
 * the local user's approve/deny decision via
 * respondConnectionScopeApproval -- one dialog covers every host in the
 * batch. */
export interface MCPConnectionScopeApprovalPayload {
  requestId: string
  clientId: string
  hostNames: string[]
}

/** Published when a run_command call resolved to medium/high risk (or an
 * uncertain static analysis) and is awaiting the local user's approve/deny
 * decision via respondCommandApproval. guardedCmd is the actual string that
 * will execute if approved (inline-guarded); command is the original for
 * display. */
export interface MCPCommandApprovalPayload {
  requestId: string
  clientId: string
  sessionId: string
  command: string
  risk: string
  uncertain: boolean
  reasons: string[]
  guardedCmd: string
}

/** Published on every delegation lifecycle transition (grant/release/
 * kill/expire) so the frontend's control panel can stay in sync without
 * polling -- state is "delegated" on grant, "none" on every kind of
 * teardown (reason distinguishes released/killed/expired/granted). */
export interface MCPDelegationPayload {
  sessionId: string
  clientId: string
  state: 'delegated' | 'none'
  reason: 'granted' | 'released' | 'killed' | 'expired'
  aiCreated: boolean
}

/** Published on every in-flight AI command's lifecycle transition (D1) --
 * running while executing, tui while an alt-screen/REPL program has taken
 * over, done once it exits (exitCode set). No awaiting_input state; gate
 * states (pending approval etc.) are handled by the separate
 * mcp:cmd-approval flow, not this stream. */
export interface MCPCommandStatePayload {
  sessionId: string
  seq: number
  state: 'running' | 'tui' | 'done'
  exitCode?: number
}

// subscribe wraps EventsOn with a typed callback and returns the unsubscribe
// function directly (Wails' EventsOn already returns one).
export function subscribe<T>(topic: string, cb: (payload: T) => void): () => void {
  return EventsOn(topic, (payload: T) => cb(payload))
}
