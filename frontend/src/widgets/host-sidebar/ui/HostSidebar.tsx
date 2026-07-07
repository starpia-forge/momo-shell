import { useEffect, useMemo, useState, type MouseEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { useHostStore, type Host } from '../../../entities/host'
import { useSessionStore } from '../../../entities/session'
import { connectHost } from '../../../features/session-connect'
import { HostFormDialog, confirmAndDeleteHost } from '../../../features/host-crud'
import { cn } from '../../../shared/lib/cn'
import { ContextMenu, SearchInput, StatusDot, type ContextMenuItem, type DotStatus } from '../../../shared/ui'
import { SharedHostsSection } from './SharedHostsSection'

interface HostSidebarProps {
  onConnect: (host: Host, sessionId: string) => void
  /** A shared host connected without saving it as a local Host first
   * (features/shared-host-connect's "내 호스트로 저장" left unchecked). */
  onConnectShared: (name: string, address: string, sessionId: string) => void
  /** Highlights the row for the terminal tab currently in focus. */
  activeHostId?: string
}

type SortMode = 'recent' | 'name'

export function HostSidebar({ onConnect, onConnectShared, activeHostId }: HostSidebarProps) {
  const { t } = useTranslation()
  const hosts = useHostStore((s) => s.hosts)
  const load = useHostStore((s) => s.load)
  const sessions = useSessionStore((s) => s.sessions)

  const [query, setQuery] = useState('')
  const [sortMode, setSortMode] = useState<SortMode>('recent')
  const [menu, setMenu] = useState<{ x: number; y: number; host: Host } | null>(null)
  const [dialog, setDialog] = useState<{ hostId?: string; cloneFrom?: Host } | null>(null)

  useEffect(() => {
    void load()
  }, [load])

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase()
    let list = Object.values(hosts)
    if (q) {
      list = list.filter(
        (h) =>
          h.name.toLowerCase().includes(q) ||
          h.address.toLowerCase().includes(q) ||
          h.labels.some((l) => l.toLowerCase().includes(q))
      )
    }
    return [...list].sort((a, b) =>
      sortMode === 'name' ? a.name.localeCompare(b.name) : (b.lastConnectedAt ?? 0) - (a.lastConnectedAt ?? 0)
    )
  }, [hosts, query, sortMode])

  function statusFor(hostId: string): DotStatus {
    const match = Object.values(sessions).find((s) => s.hostId === hostId && s.state !== 'closed')
    if (!match) return 'idle'
    if (match.state === 'running') return 'running'
    if (match.state === 'connecting') return 'connecting'
    if (match.state === 'error') return 'error'
    return 'idle'
  }

  async function handleConnect(host: Host) {
    const sessionId = await connectHost(host.id)
    onConnect(host, sessionId)
  }

  function openContextMenu(e: MouseEvent, host: Host) {
    e.preventDefault()
    setMenu({ x: e.clientX, y: e.clientY, host })
  }

  function contextMenuItems(host: Host): ContextMenuItem[] {
    return [
      { label: t('common.edit'), onClick: () => setDialog({ hostId: host.id }) },
      { label: t('home.savedHosts.contextClone'), onClick: () => setDialog({ cloneFrom: host }) },
      { label: t('home.savedHosts.contextConnectNewTab'), onClick: () => void handleConnect(host) },
      { label: t('common.delete'), danger: true, divider: true, onClick: () => void confirmAndDeleteHost(host) },
    ]
  }

  return (
    <div className="flex flex-col h-full bg-surface border-r border-line text-fg text-[13px] p-3.5 gap-3">
      <SearchInput
        containerClassName="h-8.5"
        placeholder={t('hostSidebar.searchPlaceholder')}
        value={query}
        onChange={(e) => setQuery(e.target.value)}
      />

      <div className="flex-1 overflow-y-auto flex flex-col gap-0.5">
        <div className="flex items-center justify-between px-2.5 py-1.5">
          <span className="text-[11px] font-bold text-fg3 tracking-wide">{t('home.savedHosts.title')}</span>
          <button
            className="border-none bg-transparent text-fg3 cursor-pointer text-[10.5px] hover:text-fg2"
            onClick={() => setSortMode((m) => (m === 'recent' ? 'name' : 'recent'))}
            title={t('hostSidebar.sortToggle')}
          >
            {sortMode === 'recent' ? t('hostSidebar.sortRecent') : t('hostSidebar.sortName')}
          </button>
        </div>
        {filtered.map((host) => (
          <div
            key={host.id}
            className={cn(
              'flex items-center gap-2.25 px-2.5 py-2.25 rounded-md cursor-pointer',
              host.id === activeHostId ? 'bg-surface2 text-fg font-medium' : 'text-fg2 hover:bg-surface2',
            )}
            onDoubleClick={() => void handleConnect(host)}
            onKeyDown={(e) => e.key === 'Enter' && void handleConnect(host)}
            onContextMenu={(e) => openContextMenu(e, host)}
            tabIndex={0}
            role="button"
          >
            <StatusDot status={statusFor(host.id)} />
            <div className="min-w-0 flex-1 overflow-hidden text-ellipsis whitespace-nowrap">{host.name}</div>
          </div>
        ))}

        <SharedHostsSection onConnect={onConnect} onConnectShared={onConnectShared} />
      </div>

      <button
        className="h-9 rounded-md border border-dashed border-line bg-transparent text-fg2 cursor-pointer text-[12.5px] hover:border-accent hover:text-fg"
        onClick={() => setDialog({})}
      >
        ＋ {t('common.addHost')}
      </button>

      {menu && <ContextMenu x={menu.x} y={menu.y} items={contextMenuItems(menu.host)} onClose={() => setMenu(null)} />}

      {dialog && <HostFormDialog open hostId={dialog.hostId} cloneFrom={dialog.cloneFrom} onClose={() => setDialog(null)} />}
    </div>
  )
}
