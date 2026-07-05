import { subscribe, topics } from '../../../shared/api/events'
import { loadPeers } from './actions'

// Loads the peer list immediately, then keeps it in sync with backend-
// driven changes (mDNS discovery updates, the 30s paired-peer sync loop).
// Call once from the widget that renders the shared-hosts section.
export function registerPeerUpdates(): () => void {
  void loadPeers()
  return subscribe(topics.sharePeersUpdated(), () => void loadPeers())
}
