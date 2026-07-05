import { useEffect, useState, type MouseEvent } from 'react'
import { usePeerStore, registerPeerUpdates, loadPeers, type PeerView } from '../../../entities/peer'
import { removePeer, fetchSharedHosts, importSharedHost, describeShareError, type SharedHost } from '../../../shared/api/share'
import { PinEntryDialog, DirectAddPeerDialog } from '../../../features/peer-pairing'
import { CredentialDialog } from '../../../features/shared-host-connect'
import { useHostStore, type Host } from '../../../entities/host'
import { ContextMenu, useToastStore, type ContextMenuItem } from '../../../shared/ui'
import './SharedHostsSection.css'

interface SharedHostsSectionProps {
  onConnect: (host: Host, sessionId: string) => void
  onConnectShared: (name: string, address: string, sessionId: string) => void
}

export function SharedHostsSection({ onConnect, onConnectShared }: SharedHostsSectionProps) {
  const peers = usePeerStore((s) => s.peers)
  const [expanded, setExpanded] = useState<Record<string, boolean>>({})
  const [pairing, setPairing] = useState<{ id: string; name: string } | null>(null)
  const [addingByAddress, setAddingByAddress] = useState(false)
  const [menu, setMenu] = useState<{ x: number; y: number; peer: PeerView } | null>(null)
  const [hostMenu, setHostMenu] = useState<{ x: number; y: number; peerId: string; index: number; host: SharedHost } | null>(null)
  const [connecting, setConnecting] = useState<SharedHost | null>(null)

  useEffect(() => registerPeerUpdates(), [])

  function toggle(id: string) {
    setExpanded((s) => ({ ...s, [id]: !s[id] }))
  }

  function openPeerMenu(e: MouseEvent, peer: PeerView) {
    e.preventDefault()
    setMenu({ x: e.clientX, y: e.clientY, peer })
  }

  function peerMenuItems(peer: PeerView): ContextMenuItem[] {
    return [
      {
        label: '새로고침',
        onClick: () =>
          void fetchSharedHosts(peer.id)
            .then(loadPeers)
            .catch((err) => useToastStore.getState().push(describeShareError(err))),
      },
      { label: '삭제', danger: true, onClick: () => void removePeer(peer.id).then(loadPeers) },
    ]
  }

  function openHostMenu(e: MouseEvent, peerId: string, index: number, host: SharedHost) {
    e.preventDefault()
    e.stopPropagation()
    setHostMenu({ x: e.clientX, y: e.clientY, peerId, index, host })
  }

  function hostMenuItems(peerId: string, index: number, host: SharedHost): ContextMenuItem[] {
    return [
      { label: '연결', onClick: () => setConnecting(host) },
      {
        label: '내 호스트로 가져오기',
        onClick: () =>
          void importSharedHost(peerId, index)
            .then(() => useHostStore.getState().load())
            .then(() => useToastStore.getState().push(`${host.name}을(를) 내 호스트로 가져왔습니다`))
            .catch((err) => useToastStore.getState().push(describeShareError(err))),
      },
    ]
  }

  function handleConnected(result: { sessionId: string; host?: Host }) {
    setConnecting(null)
    if (result.host) {
      onConnect(result.host, result.sessionId)
    } else if (connecting) {
      onConnectShared(connecting.name, connecting.address, result.sessionId)
    }
  }

  return (
    <div className="shared-hosts">
      <div className="shared-hosts__toolbar">
        <span>공유 호스트</span>
        <button className="shared-hosts__add-by-address" onClick={() => setAddingByAddress(true)}>
          IP로 추가
        </button>
      </div>
      {peers.length > 0 && (
        <div className="shared-hosts__list">
          {peers.map((peer) => (
            <div key={peer.id} className="shared-hosts__peer">
              <div
                className={['shared-hosts__peer-header', !peer.online && 'shared-hosts__peer-header--offline']
                  .filter(Boolean)
                  .join(' ')}
                onClick={() => peer.paired && toggle(peer.id)}
                onContextMenu={(e) => peer.paired && openPeerMenu(e, peer)}
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
                  {peer.hosts.map((h, index) => (
                    <li
                      key={`${h.address}:${h.port}`}
                      className="shared-hosts__host-item"
                      onDoubleClick={() => setConnecting(h)}
                      onContextMenu={(e) => openHostMenu(e, peer.id, index, h)}
                    >
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
      )}

      {menu && <ContextMenu x={menu.x} y={menu.y} items={peerMenuItems(menu.peer)} onClose={() => setMenu(null)} />}
      {hostMenu && (
        <ContextMenu
          x={hostMenu.x}
          y={hostMenu.y}
          items={hostMenuItems(hostMenu.peerId, hostMenu.index, hostMenu.host)}
          onClose={() => setHostMenu(null)}
        />
      )}
      {pairing && <PinEntryDialog peerId={pairing.id} peerName={pairing.name} onClose={() => setPairing(null)} />}
      {addingByAddress && <DirectAddPeerDialog onClose={() => setAddingByAddress(false)} />}
      {connecting && <CredentialDialog sharedHost={connecting} onClose={() => setConnecting(null)} onConnected={handleConnected} />}
    </div>
  )
}
