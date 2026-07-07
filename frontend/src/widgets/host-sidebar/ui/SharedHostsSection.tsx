import { useEffect, useState, type MouseEvent } from 'react'
import { usePeerStore, registerPeerUpdates, loadPeers, type PeerView } from '../../../entities/peer'
import { removePeer, fetchSharedHosts, importSharedHost, describeShareError, type SharedHost } from '../../../shared/api/share'
import { PinEntryDialog, DirectAddPeerDialog } from '../../../features/peer-pairing'
import { CredentialDialog } from '../../../features/shared-host-connect'
import { useHostStore, type Host } from '../../../entities/host'
import { cn } from '../../../shared/lib/cn'
import { ContextMenu, useToastStore, type ContextMenuItem } from '../../../shared/ui'

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
            .catch((err) => useToastStore.getState().push(describeShareError(err), 'error')),
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
            .then(() => useToastStore.getState().push(`${host.name}을(를) 내 호스트로 가져왔습니다`, 'success'))
            .catch((err) => useToastStore.getState().push(describeShareError(err), 'error')),
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
    <div className="flex-none border-t border-line max-h-60 flex flex-col">
      <div className="flex items-center justify-between px-2 py-1 text-fg2 text-[12px]">
        <span>공유 호스트</span>
        <button className="border-none bg-transparent text-accent cursor-pointer text-[12px]" onClick={() => setAddingByAddress(true)}>
          IP로 추가
        </button>
      </div>
      {peers.length > 0 && (
        <div className="overflow-y-auto">
          {peers.map((peer) => (
            <div key={peer.id}>
              <div
                className={cn(
                  'flex items-center gap-1.5 px-2 py-1.5 cursor-pointer bg-accent/6 hover:bg-canvas',
                  !peer.online && 'text-fg2 opacity-60',
                )}
                onClick={() => peer.paired && toggle(peer.id)}
                onContextMenu={(e) => peer.paired && openPeerMenu(e, peer)}
              >
                <span className="flex-none">📡</span>
                <span className="flex-1 min-w-0 overflow-hidden text-ellipsis whitespace-nowrap text-[12px]">
                  {peer.name}
                  {peer.paired ? ` (${peer.hosts.length})` : ''}
                </span>
                {!peer.online && peer.lastSyncAt && (
                  <span className="flex-none text-[10px] text-fg2">마지막 동기화 {new Date(peer.lastSyncAt * 1000).toLocaleString()}</span>
                )}
                {!peer.paired && (
                  <button
                    className="flex-none border border-line bg-canvas text-accent rounded px-1.5 py-0.5 text-[11px] cursor-pointer"
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
                <ul className="m-0 py-0 pr-2 pb-1 pl-6 flex flex-col gap-0.5">
                  {peer.hosts.length === 0 && <li className="text-[11px] text-fg2 py-1">공유된 호스트가 없습니다</li>}
                  {peer.hosts.map((h, index) => (
                    <li
                      key={`${h.address}:${h.port}`}
                      className="flex items-center gap-1.5 text-[12px] text-fg2"
                      onDoubleClick={() => setConnecting(h)}
                      onContextMenu={(e) => openHostMenu(e, peer.id, index, h)}
                    >
                      <span className="flex-none text-accent">⇢</span>
                      <span className="overflow-hidden text-ellipsis whitespace-nowrap">{h.name}</span>
                      <span className="flex-none text-[11px]">{h.address}</span>
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
