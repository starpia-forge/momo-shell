import { useEffect, useState, type MouseEvent } from 'react'
import { usePeerStore, registerPeerUpdates, loadPeers, type PeerView } from '../../../entities/peer'
import { removePeer, fetchSharedHosts, importSharedHost, describeShareError, type SharedHost } from '../../../shared/api/share'
import { PinEntryDialog, DirectAddPeerDialog } from '../../../features/peer-pairing'
import { CredentialDialog } from '../../../features/shared-host-connect'
import { useHostStore, type Host } from '../../../entities/host'
import { ContextMenu, useToastStore, type ContextMenuItem } from '../../../shared/ui'

interface SharedHostsGridProps {
  onConnect: (host: Host, sessionId: string) => void
  onConnectShared: (name: string, address: string, sessionId: string) => void
}

export function SharedHostsGrid({ onConnect, onConnectShared }: SharedHostsGridProps) {
  const peers = usePeerStore((s) => s.peers)
  const [pairing, setPairing] = useState<{ id: string; name: string } | null>(null)
  const [addingByAddress, setAddingByAddress] = useState(false)
  const [menu, setMenu] = useState<{ x: number; y: number; peer: PeerView } | null>(null)
  const [hostMenu, setHostMenu] = useState<{ x: number; y: number; peerId: string; index: number; host: SharedHost } | null>(null)
  const [connecting, setConnecting] = useState<SharedHost | null>(null)

  useEffect(() => registerPeerUpdates(), [])

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
    <section className="home-section">
      <div className="home-section__header">
        <span className="home-section__title">공유 호스트</span>
        <div className="home-section__actions">
          <button className="home-section__link-btn" onClick={() => setAddingByAddress(true)}>
            IP로 추가
          </button>
        </div>
      </div>

      {peers.length === 0 && <div className="home-section__empty">발견된 공유 피어가 없습니다</div>}

      {peers.map((peer) => (
        <div key={peer.id} className="home-peer-group">
          <div
            className={['home-peer-group__header', !peer.online && 'home-peer-group__header--offline'].filter(Boolean).join(' ')}
            onContextMenu={(e) => peer.paired && openPeerMenu(e, peer)}
          >
            <span className="home-peer-group__icon">📡</span>
            <span className="home-peer-group__name">
              {peer.name}
              {peer.paired ? ` (${peer.hosts.length})` : ''}
            </span>
            {!peer.online && peer.lastSyncAt && (
              <span className="home-peer-group__sync">마지막 동기화 {new Date(peer.lastSyncAt * 1000).toLocaleString()}</span>
            )}
            {!peer.paired && (
              <button className="home-section__link-btn" onClick={() => setPairing({ id: peer.id, name: peer.name })}>
                연결
              </button>
            )}
          </div>

          {peer.paired && (
            <div className="home-grid">
              {peer.hosts.length === 0 && <div className="home-section__empty">공유된 호스트가 없습니다</div>}
              {peer.hosts.map((h, index) => (
                <div
                  key={`${h.address}:${h.port}`}
                  className="home-card"
                  onDoubleClick={() => setConnecting(h)}
                  onContextMenu={(e) => openHostMenu(e, peer.id, index, h)}
                >
                  <div className="home-card__header">
                    <span className="home-card__badge">⇢</span>
                    <span className="home-card__name">{h.name}</span>
                  </div>
                  <div className="home-card__address">{h.address}</div>
                  <button className="home-card__connect" onClick={() => setConnecting(h)}>
                    연결
                  </button>
                </div>
              ))}
            </div>
          )}
        </div>
      ))}

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
    </section>
  )
}
