import {
  EnableSharing,
  DisableSharing,
  Status,
  SetSharedHosts,
  ListClients,
  RevokeClient,
  RespondPairing,
} from '../../../wailsjs/go/wails/ShareService'

export interface ShareStatus {
  enabled: boolean
  pin: string
  port: number
  instanceName: string
  sharedHostIds: string[]
}

export interface ShareClient {
  id: string
  name: string
  pairedAt: number
  lastSeenAt?: number
}

// asShareStatus normalizes sharedHostIds -- Go's nil slice marshals to
// JSON `null`, not `[]`, whenever no host has ever been shared.
function asShareStatus(dto: ShareStatus): ShareStatus {
  return { ...dto, sharedHostIds: dto.sharedHostIds ?? [] }
}

export async function enableSharing(hostIds: string[]): Promise<ShareStatus> {
  return asShareStatus(await EnableSharing(hostIds))
}

export async function disableSharing(): Promise<void> {
  await DisableSharing()
}

export async function getShareStatus(): Promise<ShareStatus> {
  return asShareStatus(await Status())
}

export async function setSharedHosts(hostIds: string[]): Promise<void> {
  await SetSharedHosts(hostIds)
}

export async function listShareClients(): Promise<ShareClient[]> {
  const clients = await ListClients()
  return clients ?? []
}

export async function revokeShareClient(id: string): Promise<void> {
  await RevokeClient(id)
}

export async function respondPairing(requestId: string, approve: boolean): Promise<void> {
  await RespondPairing(requestId, approve)
}
