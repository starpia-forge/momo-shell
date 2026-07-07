import { useEffect, useMemo, useState } from 'react'
import { useHostStore, type Host } from '../../../entities/host'
import { useSessionStore } from '../../../entities/session'
import { cn } from '../../../shared/lib/cn'
import { useSftpStore } from '../model/store'

interface HostPickerGridProps {
  onBack: () => void
  onRegister: () => void
}

type SortMode = 'recent' | 'name'
type DotStatus = 'running' | 'connecting' | 'error' | 'idle'

const DOT: Record<DotStatus, string> = {
  running: 'bg-green',
  connecting: 'bg-amber',
  error: 'bg-red',
  idle: 'bg-line',
}

/** R5/G2: the "연결" flow's host list -- visually mirrors the home page's
 * SavedHostsGrid (card grid, search, sort, status dot) but connects through
 * the SFTP store's own session (not connectHost()+onConnect), and omits the
 * edit/clone/delete context menu since this picker is connect-only. */
export function HostPickerGrid({ onBack, onRegister }: HostPickerGridProps) {
  const hosts = useHostStore((s) => s.hosts)
  const load = useHostStore((s) => s.load)
  const sessions = useSessionStore((s) => s.sessions)
  const connect = useSftpStore((s) => s.connect)
  const gateError = useSftpStore((s) => s.gateError)

  const [query, setQuery] = useState('')
  const [sortMode, setSortMode] = useState<SortMode>('recent')

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

  function handleConnect(host: Host) {
    void connect(host)
  }

  return (
    <div className="flex-1 min-w-0 overflow-y-auto p-4">
      <div className="flex items-center justify-between mb-2.5">
        <div className="flex items-center gap-2.5">
          <button
            className="border-none bg-transparent text-fg2 cursor-pointer text-[12px] hover:text-fg"
            onClick={onBack}
          >
            ← 돌아가기
          </button>
          <span className="text-[14px] font-semibold">내 호스트 ({filtered.length})</span>
        </div>
        <div className="flex items-center gap-2.5">
          <input
            className="px-2 py-1.5 rounded border border-line bg-surface text-fg text-[12px]"
            placeholder="🔍 검색"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
          <button
            className="border-none bg-transparent text-accent cursor-pointer text-[12px]"
            onClick={() => setSortMode((m) => (m === 'recent' ? 'name' : 'recent'))}
            title="정렬 방식 전환"
          >
            {sortMode === 'recent' ? '최근 연결 순' : '이름순'}
          </button>
        </div>
      </div>

      {gateError && <div className="text-[12px] text-red mb-2.5">{gateError}</div>}

      <div className="grid grid-cols-[repeat(auto-fill,minmax(220px,1fr))] gap-2.5">
        {filtered.map((host) => (
          <div
            key={host.id}
            className="flex flex-col gap-1.5 px-3 py-2.5 rounded-md border border-line bg-surface cursor-pointer text-left hover:border-accent"
            onDoubleClick={() => handleConnect(host)}
          >
            <div className="flex items-center gap-1.5 min-w-0">
              <span className={cn('flex-shrink-0 w-2 h-2 rounded-full', DOT[statusFor(host.id)])} />
              <span className="flex-1 min-w-0 overflow-hidden text-ellipsis whitespace-nowrap text-[13px]">{host.name}</span>
            </div>
            <div className="text-[11px] text-fg2 overflow-hidden text-ellipsis whitespace-nowrap">{host.address}</div>
            <button
              className="self-start border border-line bg-canvas text-accent rounded px-2 py-[3px] text-[11px] cursor-pointer"
              onClick={() => handleConnect(host)}
            >
              연결
            </button>
          </div>
        ))}

        <button
          className="flex flex-col gap-1.5 px-3 py-2.5 rounded-md border border-dashed border-line bg-surface cursor-pointer items-center justify-center text-fg2 text-[13px] hover:text-fg"
          onClick={onRegister}
        >
          + 새 호스트
        </button>
      </div>

      {filtered.length === 0 && <div className="text-fg2 text-[12px] py-2">호스트를 추가해 시작하세요</div>}
    </div>
  )
}
