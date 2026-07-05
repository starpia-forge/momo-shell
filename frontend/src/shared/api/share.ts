import {
  EnableSharing,
  DisableSharing,
  Status,
  SetSharedHosts,
  ListClients,
  RevokeClient,
  RespondPairing,
  ListPeers,
  PairWithPeer,
  AddPeerByAddress,
  RemovePeer,
  FetchSharedHosts,
  ImportSharedHost,
} from '../../../wailsjs/go/wails/ShareService'
import { asHost, type Host } from './host'

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

export interface SharedHost {
  name: string
  address: string
  port: number
  labels: string[]
  username: string
}

export interface PeerView {
  id: string
  name: string
  address: string
  port: number
  paired: boolean
  online: boolean
  lastSyncAt?: number
  hosts: SharedHost[]
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

// asPeerView normalizes labels/hosts arrays the same way asShareStatus
// does for sharedHostIds -- Go's nil slice marshals to JSON `null`.
function asPeerView(dto: PeerView): PeerView {
  return { ...dto, hosts: (dto.hosts ?? []).map((h) => ({ ...h, labels: h.labels ?? [] })) }
}

export async function listPeers(): Promise<PeerView[]> {
  const peers = await ListPeers()
  return (peers ?? []).map(asPeerView)
}

export async function pairWithPeer(peerId: string, pin: string): Promise<void> {
  await PairWithPeer(peerId, pin)
}

export async function addPeerByAddress(address: string, port: number, pin: string): Promise<void> {
  await AddPeerByAddress(address, port, pin)
}

export async function removePeer(peerId: string): Promise<void> {
  await RemovePeer(peerId)
}

export async function fetchSharedHosts(peerId: string): Promise<SharedHost[]> {
  const hosts = await FetchSharedHosts(peerId)
  return (hosts ?? []).map((h) => ({ ...h, labels: h.labels ?? [] }))
}

export async function importSharedHost(peerId: string, index: number): Promise<Host> {
  return asHost(await ImportSharedHost(peerId, index))
}

// describeShareError translates a backend error's message (Go errors cross
// the Wails boundary as plain strings) into Korean UI copy. Matched by
// substring against known sentinel messages -- share.ErrLockedOut,
// out.ErrPeerCertMismatch, out.ErrPeerUnauthorized -- since Wails doesn't
// carry a structured error code, only err.Error().
export function describeShareError(err: unknown): string {
  const message = err instanceof Error ? err.message : String(err)
  if (message.includes('too many failed')) return '잠시 후 다시 시도하세요 (60초)'
  if (message.includes('certificate fingerprint changed')) return '피어의 인증서가 변경되었습니다. 다시 페어링하세요'
  if (message.includes('peer rejected our token')) return '피어가 연결을 거부했습니다. 다시 페어링하세요'
  return '요청이 실패했습니다'
}
