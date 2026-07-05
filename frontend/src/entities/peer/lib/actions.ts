import { listPeers } from '../../../shared/api/share'
import { usePeerStore } from '../model/store'

export async function loadPeers(): Promise<void> {
  const peers = await listPeers()
  usePeerStore.getState().setPeers(peers)
}
