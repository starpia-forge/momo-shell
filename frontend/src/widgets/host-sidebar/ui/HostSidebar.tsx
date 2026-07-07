import { useEffect, useMemo, useState, type MouseEvent } from 'react'
import { useHostStore, type Host } from '../../../entities/host'
import { useSessionStore } from '../../../entities/session'
import { connectHost } from '../../../features/session-connect'
import { HostFormDialog, confirmAndDeleteHost } from '../../../features/host-crud'
import { cn } from '../../../shared/lib/cn'
import { ContextMenu, type ContextMenuItem } from '../../../shared/ui'
import { SharedHostsSection } from './SharedHostsSection'

interface HostSidebarProps {
  onConnect: (host: Host, sessionId: string) => void
  /** A shared host connected without saving it as a local Host first
   * (features/shared-host-connect's "내 호스트로 저장" left unchecked). */
  onConnectShared: (name: string, address: string, sessionId: string) => void
}

type SortMode = 'recent' | 'name'
type DotStatus = 'running' | 'connecting' | 'error' | 'idle'

const DOT: Record<DotStatus, string> = {
  running: 'bg-green',
  connecting: 'bg-amber animate-pulse-dot',
  error: 'bg-red',
  idle: 'bg-line',
}

export function HostSidebar({ onConnect, onConnectShared }: HostSidebarProps) {
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
      { label: '연결', onClick: () => void handleConnect(host) },
      { label: '편집', onClick: () => setDialog({ hostId: host.id }) },
      { label: '복제', onClick: () => setDialog({ cloneFrom: host }) },
      { label: '삭제', danger: true, onClick: () => void confirmAndDeleteHost(host) },
    ]
  }

  return (
    <div className="flex flex-col h-full bg-surface border-r border-line text-fg text-[13px]">
      <div className="p-2">
        <input
          className="w-full px-2 py-1.5 rounded border border-line bg-canvas text-fg text-[12px]"
          placeholder="🔍 검색"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
        />
      </div>
      <div className="flex justify-between items-center px-2 py-1 text-fg2 text-[12px]">
        <span>내 호스트 ({filtered.length})</span>
        <button
          className="border-none bg-transparent text-accent cursor-pointer text-[12px]"
          onClick={() => setSortMode((m) => (m === 'recent' ? 'name' : 'recent'))}
          title="정렬 방식 전환"
        >
          {sortMode === 'recent' ? '최근 연결 순' : '이름순'}
        </button>
      </div>
      <div className="flex-1 overflow-y-auto">
        {filtered.map((host) => (
          <div
            key={host.id}
            className="flex items-center gap-2 px-2 py-1.5 cursor-pointer hover:bg-canvas"
            onDoubleClick={() => void handleConnect(host)}
            onKeyDown={(e) => e.key === 'Enter' && void handleConnect(host)}
            onContextMenu={(e) => openContextMenu(e, host)}
            tabIndex={0}
            role="button"
          >
            <span className={cn('flex-shrink-0 w-2 h-2 rounded-full', DOT[statusFor(host.id)])} />
            <div className="min-w-0">
              <div className="overflow-hidden text-ellipsis whitespace-nowrap">{host.name}</div>
              <div className="text-[11px] text-fg2 overflow-hidden text-ellipsis whitespace-nowrap">{host.address}</div>
            </div>
          </div>
        ))}
      </div>
      <button
        className="m-2 px-1.5 py-1.5 rounded border border-dashed border-line bg-transparent text-fg2 cursor-pointer text-[12px] hover:border-accent hover:text-fg"
        onClick={() => setDialog({})}
      >
        + 호스트 추가
      </button>

      <SharedHostsSection onConnect={onConnect} onConnectShared={onConnectShared} />

      {menu && <ContextMenu x={menu.x} y={menu.y} items={contextMenuItems(menu.host)} onClose={() => setMenu(null)} />}

      {dialog && <HostFormDialog open hostId={dialog.hostId} cloneFrom={dialog.cloneFrom} onClose={() => setDialog(null)} />}
    </div>
  )
}
