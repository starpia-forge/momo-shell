import { useEffect, useState, type MouseEvent } from 'react'
import { useTranslation } from 'react-i18next'
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
  const { t } = useTranslation()
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
        label: t('common.refresh'),
        onClick: () =>
          void fetchSharedHosts(peer.id)
            .then(loadPeers)
            .catch((err) => useToastStore.getState().push(describeShareError(err), 'error')),
      },
      { label: t('common.delete'), danger: true, divider: true, onClick: () => void removePeer(peer.id).then(loadPeers) },
    ]
  }

  function openHostMenu(e: MouseEvent, peerId: string, index: number, host: SharedHost) {
    e.preventDefault()
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
    <section className="flex flex-col gap-5">
      <div className="flex items-center gap-4">
        <span className="text-[19px] font-bold">{t('home.sharedHosts.title')}</span>
        {onlineCount > 0 && (
          <span className="flex items-center gap-1.75 text-[12.5px] text-fg2">
            <StatusDot status="running" />
            {t('home.sharedHosts.peersFound', { count: onlineCount })}
          </span>
        )}
        <div className="flex-1" />
        <Button size="sm" onClick={() => setAddingByAddress(true)}>
          {t('home.sharedHosts.addByAddress')}
        </Button>
      </div>

      {peers.length === 0 && <div className="text-fg2 text-[12.5px] py-2">{t('home.sharedHosts.emptyState')}</div>}

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
              <Chip tone={peer.paired ? 'green' : 'neutral'}>{peer.paired ? t('home.sharedHosts.paired') : t('home.sharedHosts.unpaired')}</Chip>
              <div className="flex-1" />
              {!peer.online && peer.lastSyncAt && (
                <span className="text-[10.5px] text-fg3">
                  {t('home.sharedHosts.lastSync', { time: new Date(peer.lastSyncAt * 1000).toLocaleString() })}
                </span>
              )}
              {peer.paired ? (
                <span className="text-[11.5px] text-fg3">{t('home.sharedHosts.hostsSharedCount', { count: peer.hosts.length })}</span>
              ) : (
                <Button variant="outline-accent" onClick={() => setPairing({ id: peer.id, name: peer.name })}>
                  {t('home.sharedHosts.pair')}
                </Button>
              )}
            </div>

            {peer.paired ? (
              <>
                <div className="flex flex-col gap-2">
                  {peer.hosts.length === 0 && <div className="text-fg2 text-[12.5px] py-1">{t('home.sharedHosts.noHosts')}</div>}
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
                            .then(() => useToastStore.getState().push(t('home.sharedHosts.importedToast', { name: h.name }), 'success'))
                            .catch((err) => useToastStore.getState().push(describeShareError(err), 'error'))
                        }}
                      >
                        {t('home.sharedHosts.saveAsMyHost')}
                      </Button>
                      <Button
                        variant="primary"
                        size="sm"
                        onClick={(e) => {
                          e.stopPropagation()
                          setConnecting(h)
                        }}
                      >
                        {t('common.connect')}
                      </Button>
                    </div>
                  ))}
                </div>
                <div className="text-[11.5px] text-fg3">{t('home.sharedHosts.connectHint')}</div>
              </>
            ) : (
              <div className="text-[12.5px] text-fg2 leading-relaxed">
                {t('home.sharedHosts.pairingRequired')}
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
