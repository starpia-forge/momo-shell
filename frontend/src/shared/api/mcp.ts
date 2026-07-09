import { ListClients, RespondPairing, RevokeClient } from '../../../wailsjs/go/wails/MCPApprovalService'

export interface MCPClient {
  id: string
  name: string
  pairedAt: number
  lastSeenAt?: number
  revoked: boolean
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
