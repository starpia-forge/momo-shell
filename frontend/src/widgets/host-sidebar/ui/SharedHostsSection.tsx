import { useEffect, useState, type MouseEvent } from 'react'
import { usePeerStore, registerPeerUpdates, loadPeers, type PeerView } from '../../../entities/peer'
import { removePeer, fetchSharedHosts } from '../../../shared/api/share'
import { PinEntryDialog } from '../../../features/peer-pairing'
import { ContextMenu, type ContextMenuItem } from '../../../shared/ui'
import './SharedHostsSection.css'

// Read-only view of paired/discovered peers for M4 -- double-click connect
// and "가져오기" land in M5 once CreateSSHDirect/ImportSharedHost exist.
export function SharedHostsSection() {
  const peers = usePeerStore((s) => s.peers)
  const [expanded, setExpanded] = useState<Record<string, boolean>>({})
  const [pairing, setPairing] = useState<{ id: string; name: string } | null>(null)
  const [menu, setMenu] = useState<{ x: number; y: number; peer: PeerView } | null>(null)

  useEffect(() => registerPeerUpdates(), [])

  if (peers.length === 0) return null

  function toggle(id: string) {
    setExpanded((s) => ({ ...s, [id]: !s[id] }))
  }

  function openMenu(e: MouseEvent, peer: PeerView) {
    e.preventDefault()
    setMenu({ x: e.clientX, y: e.clientY, peer })
  }

  function menuItems(peer: PeerView): ContextMenuItem[] {
    return [
      { label: '새로고침', onClick: () => void fetchSharedHosts(peer.id).then(loadPeers) },
      { label: '삭제', danger: true, onClick: () => void removePeer(peer.id).then(loadPeers) },
    ]
  }

  return (
    <div className="shared-hosts">
      <div className="shared-hosts__toolbar">
        <span>공유 호스트</span>
      </div>
      <div className="shared-hosts__list">
        {peers.map((peer) => (
          <div key={peer.id} className="shared-hosts__peer">
            <div
              className={['shared-hosts__peer-header', !peer.online && 'shared-hosts__peer-header--offline']
                .filter(Boolean)
                .join(' ')}
              onClick={() => peer.paired && toggle(peer.id)}
              onContextMenu={(e) => peer.paired && openMenu(e, peer)}
            >
              <span className="shared-hosts__peer-icon">📡</span>
              <span className="shared-hosts__peer-name">
                {peer.name}
                {peer.paired ? ` (${peer.hosts.length})` : ''}
              </span>
              {!peer.online && peer.lastSyncAt && (
                <span className="shared-hosts__peer-sync">마지막 동기화 {new Date(peer.lastSyncAt * 1000).toLocaleString()}</span>
              )}
              {!peer.paired && (
                <button
                  className="shared-hosts__connect"
                  onClick={(e) => {
                    e.stopPropagation()
                    setPairing({ id: peer.id, name: peer.name })
                  }}
                >
                  연결
                </button>
              )}
            </div>
            {peer.paired && expanded[peer.id] && (
              <ul className="shared-hosts__host-list">
                {peer.hosts.length === 0 && <li className="shared-hosts__empty">공유된 호스트가 없습니다</li>}
                {peer.hosts.map((h) => (
                  <li key={`${h.address}:${h.port}`} className="shared-hosts__host-item">
                    <span className="shared-hosts__host-badge">⇢</span>
                    <span className="shared-hosts__host-name">{h.name}</span>
                    <span className="shared-hosts__host-address">{h.address}</span>
                  </li>
                ))}
              </ul>
            )}
          </div>
        ))}
      </div>

      {menu && <ContextMenu x={menu.x} y={menu.y} items={menuItems(menu.peer)} onClose={() => setMenu(null)} />}
      {pairing && <PinEntryDialog peerId={pairing.id} peerName={pairing.name} onClose={() => setPairing(null)} />}
    </div>
  )
}
