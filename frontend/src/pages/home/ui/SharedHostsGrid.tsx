import { useEffect, useState, type MouseEvent } from 'react'
import { usePeerStore, registerPeerUpdates, loadPeers, type PeerView } from '../../../entities/peer'
import { removePeer, fetchSharedHosts, importSharedHost, describeShareError, type SharedHost } from '../../../shared/api/share'
import { PinEntryDialog, DirectAddPeerDialog } from '../../../features/peer-pairing'
import { CredentialDialog } from '../../../features/shared-host-connect'
import { useHostStore, type Host } from '../../../entities/host'
import { cn } from '../../../shared/lib/cn'
import { Button, Chip, ContextMenu, StatusDot, useToastStore, type ContextMenuItem } from '../../../shared/ui'

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

  const onlineCount = peers.filter((p) => p.online).length

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
      { label: '삭제', danger: true, divider: true, onClick: () => void removePeer(peer.id).then(loadPeers) },
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
    <section className="flex flex-col gap-5">
      <div className="flex items-center gap-4">
        <span className="text-[19px] font-bold">공유받은 호스트</span>
        {onlineCount > 0 && (
          <span className="flex items-center gap-1.75 text-[12.5px] text-fg2">
            <StatusDot status="running" />
            같은 네트워크에서 피어 {onlineCount}명 발견
          </span>
        )}
        <div className="flex-1" />
        <Button size="sm" onClick={() => setAddingByAddress(true)}>
          IP로 추가
        </Button>
      </div>

      {peers.length === 0 && <div className="text-fg2 text-[12.5px] py-2">발견된 공유 피어가 없습니다</div>}

      <div className="grid grid-cols-2 gap-4">
        {peers.map((peer) => (
          <div
            key={peer.id}
            className={cn(
              'flex flex-col gap-3.5 px-5.5 py-5 rounded-xl border bg-surface',
              peer.paired ? 'border-line' : 'border-dashed border-line justify-center',
              !peer.online && 'opacity-60',
            )}
            onContextMenu={(e) => peer.paired && openPeerMenu(e, peer)}
          >
            <div className="flex items-center gap-2.5">
              <span className="text-[14.5px] font-bold">{peer.name}</span>
              <Chip tone={peer.paired ? 'green' : 'neutral'}>{peer.paired ? '페어링됨' : '미페어링'}</Chip>
              <div className="flex-1" />
              {!peer.online && peer.lastSyncAt && (
                <span className="text-[10.5px] text-fg3">마지막 동기화 {new Date(peer.lastSyncAt * 1000).toLocaleString()}</span>
              )}
              {peer.paired ? (
                <span className="text-[11.5px] text-fg3">호스트 {peer.hosts.length}개 공유 중</span>
              ) : (
                <Button variant="outline-accent" onClick={() => setPairing({ id: peer.id, name: peer.name })}>
                  페어링
                </Button>
              )}
            </div>

            {peer.paired ? (
              <>
                <div className="flex flex-col gap-2">
                  {peer.hosts.length === 0 && <div className="text-fg2 text-[12.5px] py-1">공유된 호스트가 없습니다</div>}
                  {peer.hosts.map((h, index) => (
                    <div
                      key={`${h.address}:${h.port}`}
                      className="flex items-center gap-3 px-3.5 py-2.75 rounded-md bg-inputbg cursor-pointer"
                      onDoubleClick={() => setConnecting(h)}
                      onContextMenu={(e) => openHostMenu(e, peer.id, index, h)}
                    >
                      <span className="text-[13px] font-medium">{h.name}</span>
                      <span className="text-[11.5px] font-mono text-fg3">
                        {h.address}:{h.port}
                      </span>
                      <div className="flex-1" />
                      <Button
                        size="sm"
                        onClick={(e) => {
                          e.stopPropagation()
                          void importSharedHost(peer.id, index)
                            .then(() => useHostStore.getState().load())
                            .then(() => useToastStore.getState().push(`${h.name}을(를) 내 호스트로 가져왔습니다`, 'success'))
                            .catch((err) => useToastStore.getState().push(describeShareError(err), 'error'))
                        }}
                      >
                        내 호스트로 저장
                      </Button>
                      <Button
                        variant="primary"
                        size="sm"
                        onClick={(e) => {
                          e.stopPropagation()
                          setConnecting(h)
                        }}
                      >
                        연결
                      </Button>
                    </div>
                  ))}
                </div>
                <div className="text-[11.5px] text-fg3">연결 시 자격증명은 직접 입력합니다</div>
              </>
            ) : (
              <div className="text-[12.5px] text-fg2 leading-relaxed">
                이 피어의 공유 호스트를 보려면 페어링이 필요합니다. 상대 화면에 표시된 PIN을 입력하세요.
              </div>
            )}
          </div>
        ))}
      </div>

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
