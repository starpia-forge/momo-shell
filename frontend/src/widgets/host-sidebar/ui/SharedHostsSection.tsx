import { useEffect, useState, type MouseEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { usePeerStore, registerPeerUpdates, loadPeers, type PeerView } from '../../../entities/peer'
import { removePeer, fetchSharedHosts, importSharedHost, describeShareError, type SharedHost } from '../../../shared/api/share'
import { PinEntryDialog, DirectAddPeerDialog } from '../../../features/peer-pairing'
import { CredentialDialog } from '../../../features/shared-host-connect'
import { useHostStore, type Host } from '../../../entities/host'
import { cn } from '../../../shared/lib/cn'
import { Button, ContextMenu, useToastStore, type ContextMenuItem } from '../../../shared/ui'

interface SharedHostsSectionProps {
  onConnect: (host: Host, sessionId: string) => void
  onConnectShared: (name: string, address: string, sessionId: string) => void
}

export function SharedHostsSection({ onConnect, onConnectShared }: SharedHostsSectionProps) {
  const { t } = useTranslation()
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
        label: t('common.refresh'),
        onClick: () =>
          void fetchSharedHosts(peer.id)
            .then(loadPeers)
            .catch((err) => useToastStore.getState().push(describeShareError(err), 'error')),
      },
      { label: t('common.delete'), danger: true, onClick: () => void removePeer(peer.id).then(loadPeers) },
    ]
  }

  function openHostMenu(e: MouseEvent, peerId: string, index: number, host: SharedHost) {
    e.preventDefault()
    e.stopPropagation()
    setHostMenu({ x: e.clientX, y: e.clientY, peerId, index, host })
  }

  function hostMenuItems(peerId: string, index: number, host: SharedHost): ContextMenuItem[] {
    return [
      { label: t('common.connect'), onClick: () => setConnecting(host) },
      {
        label: t('home.sharedHosts.importAsMyHost'),
        onClick: () =>
          void importSharedHost(peerId, index)
            .then(() => useHostStore.getState().load())
            .then(() => useToastStore.getState().push(t('home.sharedHosts.importedToast', { name: host.name }), 'success'))
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
    <div className="flex-none flex flex-col">
      <div className="flex items-center justify-between px-2.5 py-1.5 mt-2">
        <span className="text-[11px] font-bold text-fg3 tracking-wide">{t('home.sharedHosts.title')}</span>
        <button className="border-none bg-transparent text-fg3 cursor-pointer text-[10.5px] hover:text-fg2" onClick={() => setAddingByAddress(true)}>
          {t('home.sharedHosts.addByAddress')}
        </button>
      </div>
      {peers.length > 0 && (
        <div className="overflow-y-auto flex flex-col gap-0.5">
          {peers.map((peer) => (
            <div key={peer.id}>
              <div
                className={cn(
                  'flex items-center gap-2.25 px-2.5 py-2.25 rounded-md cursor-pointer text-fg2 hover:bg-surface2',
                  !peer.online && 'opacity-60',
                )}
                onClick={() => peer.paired && toggle(peer.id)}
                onContextMenu={(e) => peer.paired && openPeerMenu(e, peer)}
              >
                <span className="flex-1 min-w-0 overflow-hidden text-ellipsis whitespace-nowrap text-[13px]">
                  {peer.name}
                  {peer.paired ? ` (${peer.hosts.length})` : ''}
                </span>
                {!peer.online && peer.lastSyncAt && (
                  <span className="flex-none text-[10px] text-fg3">
                    {t('home.sharedHosts.lastSync', { time: new Date(peer.lastSyncAt * 1000).toLocaleString() })}
                  </span>
                )}
                {!peer.paired && (
                  <Button
                    size="sm"
                    onClick={(e) => {
                      e.stopPropagation()
                      setPairing({ id: peer.id, name: peer.name })
                    }}
                  >
                    {t('home.sharedHosts.pair')}
                  </Button>
                )}
              </div>
              {peer.paired && expanded[peer.id] && (
                <ul className="m-0 py-0 pr-2.5 pb-1 pl-7 flex flex-col gap-0.5">
                  {peer.hosts.length === 0 && <li className="text-[11px] text-fg3 py-1">{t('home.sharedHosts.noHosts')}</li>}
                  {peer.hosts.map((h, index) => (
                    <li
                      key={`${h.address}:${h.port}`}
                      className="flex items-center gap-1.5 text-[13px] text-fg2 py-1"
                      onDoubleClick={() => setConnecting(h)}
                      onContextMenu={(e) => openHostMenu(e, peer.id, index, h)}
                    >
                      <span className="overflow-hidden text-ellipsis whitespace-nowrap flex-1 min-w-0">{h.name}</span>
                      <span className="flex-none text-[10px] text-fg3">{peer.name}</span>
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
