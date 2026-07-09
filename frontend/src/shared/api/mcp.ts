import {
  KillControl,
  ListClients,
  ListDelegations,
  RespondCommandApproval,
  RespondConnectApproval,
  RespondConnectionScopeApproval,
  RespondControlApproval,
  RespondPairing,
  RevokeClient,
} from '../../../wailsjs/go/wails/MCPApprovalService'

export interface MCPClient {
  id: string
  name: string
  pairedAt: number
  lastSeenAt?: number
  revoked: boolean
}

export interface Delegation {
  sessionId: string
  clientId: string
  aiCreated: boolean
  grantedAt: number
}

export async function respondPairing(requestId: string, approve: boolean): Promise<void> {
  await RespondPairing(requestId, approve)
}

export async function listMCPClients(): Promise<MCPClient[]> {
  const clients = await ListClients()
  return clients ?? []
}

export async function revokeMCPClient(clientId: string): Promise<void> {
  await RevokeClient(clientId)
}

export async function respondConnectApproval(requestId: string, approve: boolean): Promise<void> {
  await RespondConnectApproval(requestId, approve)
}

export async function respondControlApproval(requestId: string, approve: boolean): Promise<void> {
  await RespondControlApproval(requestId, approve)
}

export async function respondConnectionScopeApproval(requestId: string, approve: boolean): Promise<void> {
  await RespondConnectionScopeApproval(requestId, approve)
}

export async function respondCommandApproval(requestId: string, approve: boolean): Promise<void> {
  await RespondCommandApproval(requestId, approve)
}

export async function killControl(sessionId: string): Promise<void> {
  await KillControl(sessionId)
}

export async function listDelegations(): Promise<Delegation[]> {
  const delegations = await ListDelegations()
  return delegations ?? []
}
