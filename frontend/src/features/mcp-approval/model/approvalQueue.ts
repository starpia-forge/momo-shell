import { create } from 'zustand'
import type {
  MCPCommandApprovalPayload,
  MCPConnectApprovalPayload,
  MCPConnectionScopeApprovalPayload,
  MCPControlApprovalPayload,
} from '../../../shared/api/events'

// A discriminated union so one dialog/store can hold any of the 4 approval
// kinds (mcpPairRequests.ts' single-payload-type shape doesn't fit -- these
// 4 kinds have different payloads).
export type ApprovalRequest =
  | { kind: 'connect'; payload: MCPConnectApprovalPayload }
  | { kind: 'control'; payload: MCPControlApprovalPayload }
  | { kind: 'scope'; payload: MCPConnectionScopeApprovalPayload }
  | { kind: 'command'; payload: MCPCommandApprovalPayload }

interface ApprovalQueueStore {
  requests: Record<string, ApprovalRequest>
  setRequest: (requestId: string, request: ApprovalRequest) => void
  clearRequest: (requestId: string) => void
}

export const useApprovalQueueStore = create<ApprovalQueueStore>((set) => ({
  requests: {},
  setRequest: (requestId, request) => set((s) => ({ requests: { ...s.requests, [requestId]: request } })),
  clearRequest: (requestId) =>
    set((s) => {
      const rest = { ...s.requests }
      delete rest[requestId]
      return { requests: rest }
    }),
}))

// Unlike mcp:pair-request, the connect/control/scope/command approval flows
// have no server-side "resolved" companion event (aicontrol.Service just
// times out its own wait -- see connect.go/control.go/run.go's
// awaitXApproval, each with its own *ApprovalTimeout). Without this, a
// request answered by the timeout would leave its dialog entry stuck
// forever. 65s gives the backend's 60s timeout (connect/control/scope) or
// command's own window room to fire first in the common case; either way
// this is just a client-side safety net, not the source of truth --
// respond() below still races it via clearRequest's idempotent delete.
export const APPROVAL_REQUEST_TTL_MS = 65_000

export function scheduleApprovalTTL(requestId: string): void {
  setTimeout(() => useApprovalQueueStore.getState().clearRequest(requestId), APPROVAL_REQUEST_TTL_MS)
}
