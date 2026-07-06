import { useEffect, useMemo, useState, type MouseEvent } from 'react'
import { useHostStore, type Host } from '../../../entities/host'
import { useSessionStore } from '../../../entities/session'
import { connectHost } from '../../../features/session-connect'
import { HostFormDialog, confirmAndDeleteHost } from '../../../features/host-crud'
import { ContextMenu, type ContextMenuItem } from '../../../shared/ui'

interface SavedHostsGridProps {
  onConnect: (host: Host, sessionId: string) => void
}

type SortMode = 'recent' | 'name'
type DotStatus = 'running' | 'connecting' | 'error' | 'idle'

export function SavedHostsGrid({ onConnect }: SavedHostsGridProps) {
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
    <section className="home-section">
      <div className="home-section__header">
        <span className="home-section__title">내 호스트 ({filtered.length})</span>
        <div className="home-section__actions">
          <input
            className="home-section__search"
            placeholder="🔍 검색"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
          <button
            className="home-section__link-btn"
            onClick={() => setSortMode((m) => (m === 'recent' ? 'name' : 'recent'))}
            title="정렬 방식 전환"
          >
            {sortMode === 'recent' ? '최근 연결 순' : '이름순'}
          </button>
        </div>
      </div>

      <div className="home-grid">
        {filtered.map((host) => (
          <div
            key={host.id}
            className="home-card"
            onDoubleClick={() => void handleConnect(host)}
            onContextMenu={(e) => openContextMenu(e, host)}
          >
            <div className="home-card__header">
              <span className={`home-card__dot home-card__dot--${statusFor(host.id)}`} />
              <span className="home-card__name">{host.name}</span>
            </div>
            <div className="home-card__address">{host.address}</div>
            <button className="home-card__connect" onClick={() => void handleConnect(host)}>
              연결
            </button>
          </div>
        ))}

        <button className="home-card home-card--add" onClick={() => setDialog({})}>
          + 새 호스트
        </button>
      </div>

      {filtered.length === 0 && (
        <div className="home-section__empty">호스트를 추가해 시작하세요</div>
      )}

      {menu && <ContextMenu x={menu.x} y={menu.y} items={contextMenuItems(menu.host)} onClose={() => setMenu(null)} />}
      {dialog && <HostFormDialog open hostId={dialog.hostId} cloneFrom={dialog.cloneFrom} onClose={() => setDialog(null)} />}
    </section>
  )
}
