import { useEffect, useMemo, useState, type MouseEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { useHostStore, type Host } from '../../../entities/host'
import { useSessionStore } from '../../../entities/session'
import { connectHost } from '../../../features/session-connect'
import { HostFormDialog, confirmAndDeleteHost } from '../../../features/host-crud'
import { Button, Chip, ContextMenu, IconButton, SearchInput, SegmentedControl, StatusDot, type ContextMenuItem, type DotStatus } from '../../../shared/ui'

interface SavedHostsGridProps {
  onConnect: (host: Host, sessionId: string) => void
}

type SortMode = 'recent' | 'name'

const STATUS_TEXT: Record<DotStatus, string> = {
  running: 'text-green',
  connecting: 'text-amber',
  error: 'text-red',
  idle: 'text-fg3',
}

export function SavedHostsGrid({ onConnect }: SavedHostsGridProps) {
  const { t } = useTranslation()
  const STATUS_LABEL: Record<DotStatus, string> = {
    running: t('home.savedHosts.statusRunning'),
    connecting: t('home.savedHosts.statusConnecting'),
    error: t('home.savedHosts.statusError'),
    idle: t('home.savedHosts.statusIdle'),
  }
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

  function statusFor(hostId: string): { status: DotStatus; error?: string } {
    const match = Object.values(sessions).find((s) => s.hostId === hostId && s.state !== 'closed')
    if (!match) return { status: 'idle' }
    if (match.state === 'running') return { status: 'running' }
    if (match.state === 'connecting') return { status: 'connecting' }
    if (match.state === 'error') return { status: 'error', error: match.error }
    return { status: 'idle' }
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
    <section className="flex flex-col gap-5">
      <div className="flex items-center gap-4">
        <span className="text-[19px] font-bold">{t('home.savedHosts.title')}</span>
        <span className="text-[13px] text-fg3">{t('home.savedHosts.count', { count: filtered.length })}</span>
        <div className="flex-1" />
        <SearchInput
          containerClassName="w-70 h-9.5"
          placeholder={t('home.savedHosts.searchPlaceholder')}
          value={query}
          onChange={(e) => setQuery(e.target.value)}
        />
        <SegmentedControl
          value={sortMode}
          onChange={setSortMode}
          options={[
            { value: 'recent', label: t('home.savedHosts.sortRecent') },
            { value: 'name', label: t('home.savedHosts.sortName') },
          ]}
        />
        <Button variant="primary" className="h-9.5" onClick={() => setDialog({})}>
          ＋ {t('common.addHost')}
        </Button>
      </div>

      <div className="grid grid-cols-3 gap-4">
        {filtered.map((host) => {
          const { status, error } = statusFor(host.id)
          return (
            <div
              key={host.id}
              className="group flex flex-col gap-3 px-5.5 py-5 rounded-xl border border-line bg-surface cursor-pointer text-left hover:border-accent"
              onDoubleClick={() => void handleConnect(host)}
              onContextMenu={(e) => openContextMenu(e, host)}
            >
              <div className="flex items-center gap-2.5 min-w-0">
                <span className="flex-1 min-w-0 overflow-hidden text-ellipsis whitespace-nowrap text-[15.5px] font-bold">
                  {host.name}
                </span>
                <span className={`flex-none flex items-center gap-1.5 text-[12px] ${STATUS_TEXT[status]}`}>
                  <StatusDot status={status} />
                  {STATUS_LABEL[status]}
                </span>
              </div>
              <div className="text-[12.5px] font-mono text-fg2 overflow-hidden text-ellipsis whitespace-nowrap">
                {host.username}@{host.address}:{host.port}
              </div>

              <div className="hidden group-hover:flex items-center gap-2">
                <Button variant="primary" size="sm" onClick={() => void handleConnect(host)}>
                  {t('common.connect')}
                </Button>
                <IconButton size={30} onClick={(e) => openContextMenu(e, host)} aria-label={t('home.savedHosts.moreActions')}>
                  ⋯
                </IconButton>
                <span className="text-[11.5px] text-fg3">{t('home.savedHosts.doubleClickHint')}</span>
              </div>
              <div className="flex group-hover:hidden gap-1.5">
                {status === 'error' && error ? (
                  <span className="text-[11.5px] text-red">{error}</span>
                ) : (
                  host.labels.map((label) => (
                    <Chip key={label} tone="neutral">
                      {label}
                    </Chip>
                  ))
                )}
              </div>
            </div>
          )
        })}

        <button
          className="flex flex-col gap-1.5 px-5.5 py-5 rounded-xl border border-dashed border-line bg-surface cursor-pointer items-center justify-center text-fg2 text-[13px] hover:text-fg"
          onClick={() => setDialog({})}
        >
          ＋ {t('common.addHost')}
        </button>
      </div>

      {filtered.length === 0 && <div className="text-fg2 text-[12px] py-2">{t('home.savedHosts.emptyState')}</div>}

      {menu && <ContextMenu x={menu.x} y={menu.y} items={contextMenuItems(menu.host)} onClose={() => setMenu(null)} />}
      {dialog && <HostFormDialog open hostId={dialog.hostId} cloneFrom={dialog.cloneFrom} onClose={() => setDialog(null)} />}
    </section>
  )
}
