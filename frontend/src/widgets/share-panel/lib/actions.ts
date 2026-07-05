import {
  getShareStatus,
  enableSharing as apiEnableSharing,
  disableSharing as apiDisableSharing,
  setSharedHosts as apiSetSharedHosts,
  listShareClients,
  revokeShareClient as apiRevokeShareClient,
} from '../../../shared/api/share'
import { useSharePanelStore } from '../model/store'

export async function loadSharePanel(): Promise<void> {
  const [status, clients] = await Promise.all([getShareStatus(), listShareClients()])
  useSharePanelStore.getState().setStatus(status)
  useSharePanelStore.getState().setClients(clients)
}

export async function enableSharing(hostIds: string[]): Promise<void> {
  const status = await apiEnableSharing(hostIds)
  useSharePanelStore.getState().setStatus(status)
}

export async function disableSharing(): Promise<void> {
  await apiDisableSharing()
  useSharePanelStore.getState().setStatus(await getShareStatus())
}

export async function updateSharedHosts(hostIds: string[]): Promise<void> {
  await apiSetSharedHosts(hostIds)
  useSharePanelStore.getState().setStatus(await getShareStatus())
}

export async function revokeClient(id: string): Promise<void> {
  await apiRevokeShareClient(id)
  useSharePanelStore.getState().removeClient(id)
}
